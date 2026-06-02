// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/time/rate"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// LastUsedTracker is the asynchronous TouchLastUsed surface consumed by
// pgIdentityStore after a successful API-key resolve. The interface is
// satisfied by usagetracker.Manager (Task 23) and by test stubs.
type LastUsedTracker interface {
	Touch(keyID string)
}

// pgIdentityStore is the Postgres-backed Resolver impl. Wraps UsersBackend
// + per-issuer OIDCVerifiers; enforces JIT rate-limit and allowlist.
type pgIdentityStore struct {
	users     storage.UsersBackend
	verifiers map[string]*OIDCVerifier // issuer -> verifier
	tracker   LastUsedTracker

	jitEnabled         bool
	jitDefaultRole     Role
	jitClaimsMapping   map[string][]config.ClaimMapping // issuer -> mappings
	jitRateLimiters    sync.Map                         //nolint:unused // forward-declared for Tasks 16–21; used by rateLimiterFor
	jitRateBurst       int
	jitRateRefillPerHr int
	jitEmailAllowlist  map[string]bool // domain -> true; empty = no allowlist

	now func() time.Time
}

// IdentityStoreConfig parametrizes pgIdentityStore at construction.
type IdentityStoreConfig struct {
	Users     storage.UsersBackend
	Verifiers []*OIDCVerifier
	Tracker   LastUsedTracker

	// KnownRoles is the set of role names valid for assignment. Validated
	// against at construction time: any JITDefaultRole or
	// JITClaimsMapping role string not in this set causes
	// NewIdentityStore to return an error. Typically derived from
	// (built-in roles ∪ cfg.Auth.Roles keys).
	//
	// When JITEnabled is true but KnownRoles is empty/nil, JIT role
	// validation is intentionally skipped — NewIdentityStore cannot tell a
	// valid role from a typo'd one. Callers SHOULD therefore pass the
	// known-role set (built-in ∪ configured custom roles) whenever JIT is
	// enabled, so operator typos (e.g. "reder" for "reader") are caught at
	// construction instead of silently minting users with an unmatched role.
	KnownRoles map[Role]bool

	JITEnabled              bool
	JITDefaultRole          Role
	JITClaimsMapping        map[string][]config.ClaimMapping
	JITRateBurstPerHour     int      // bucket capacity AND refill rate (1:1)
	JITEmailDomainAllowlist []string // empty = no allowlist

	Now func() time.Time // optional; defaults to time.Now
}

// NewIdentityStore constructs a pgIdentityStore from the supplied config.
// Verifiers are indexed by issuer; the allowlist is normalized to
// lowercase domains.
func NewIdentityStore(cfg IdentityStoreConfig) (Resolver, error) { //nolint:gocritic // hugeParam: cfg is read-only; pointer would require changing all call sites
	if cfg.Users == nil {
		return nil, errors.New("auth: NewIdentityStore: Users required")
	}
	if cfg.Tracker == nil {
		return nil, errors.New("auth: NewIdentityStore: Tracker required")
	}
	verifiers := make(map[string]*OIDCVerifier, len(cfg.Verifiers))
	for _, v := range cfg.Verifiers {
		if _, dup := verifiers[v.Issuer()]; dup {
			return nil, fmt.Errorf("auth: duplicate verifier for issuer %q", v.Issuer())
		}
		verifiers[v.Issuer()] = v
	}
	// Validate JIT-related role references against KnownRoles. Catches
	// operator typos (e.g. "reder" instead of "reader") that would create
	// users whose role can never match any rolePerms entry.
	//
	// The len(cfg.KnownRoles) > 0 guard is deliberate: when JIT is enabled
	// but no known-role set is supplied, validation is skipped rather than
	// failing closed. Callers SHOULD pass KnownRoles whenever JIT is enabled
	// (see the field doc) so these typos are caught here.
	if cfg.JITEnabled && len(cfg.KnownRoles) > 0 {
		if cfg.JITDefaultRole != "" && !cfg.KnownRoles[cfg.JITDefaultRole] {
			return nil, fmt.Errorf("auth: JITDefaultRole %q not in KnownRoles", cfg.JITDefaultRole)
		}
		for issuer, mappings := range cfg.JITClaimsMapping {
			for _, m := range mappings {
				if !cfg.KnownRoles[Role(m.Role)] {
					return nil, fmt.Errorf("auth: ClaimsMapping for issuer %q maps to unknown role %q", issuer, m.Role)
				}
			}
		}
	}

	allowlist := make(map[string]bool, len(cfg.JITEmailDomainAllowlist))
	for _, d := range cfg.JITEmailDomainAllowlist {
		allowlist[strings.ToLower(d)] = true
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	burst := cfg.JITRateBurstPerHour
	if burst <= 0 {
		burst = 100 // design default
	}
	return &pgIdentityStore{
		users:              cfg.Users,
		verifiers:          verifiers,
		tracker:            cfg.Tracker,
		jitEnabled:         cfg.JITEnabled,
		jitDefaultRole:     cfg.JITDefaultRole,
		jitClaimsMapping:   cfg.JITClaimsMapping,
		jitRateBurst:       burst,
		jitRateRefillPerHr: burst,
		jitEmailAllowlist:  allowlist,
		now:                now,
	}, nil
}

// Resolve dispatches on token shape and produces an Identity (or
// returns ErrUnauthenticated / ErrTransient).
func (s *pgIdentityStore) Resolve(ctx context.Context, token string) (*Identity, error) {
	if token == "" {
		return nil, ErrUnauthenticated
	}
	if isJWTShaped(token) {
		return s.resolveJWT(ctx, token)
	}
	return s.resolveAPIKey(ctx, token)
}

// isJWTShaped reports whether token looks like a JWS Compact Serialization
// (three dot-separated base64 segments). Non-strict — a token that LOOKS
// like a JWT but isn't will fail at the verify step, which also maps to
// ErrUnauthenticated.
//
// The spgr_sk_ prefix is excluded in addition to the dot-count check so a
// (hypothetical) API key whose secret happens to contain two dots is never
// misrouted to the OIDC/JWT path: an spgr_sk_-prefixed token always goes to
// the API-key resolver regardless of its dot count. Any future API-key
// prefix that could contain dots MUST be added to this guard, or such keys
// risk being treated as JWTs.
func isJWTShaped(token string) bool {
	return strings.Count(token, ".") == 2 && !strings.HasPrefix(token, "spgr_sk_")
}

const (
	apiKeyPrefix    = "spgr_sk_" //nolint:gosec // G101: token-format prefix, not a credential
	apiKeyPrefixLen = 8          // characters
	apiKeySecretLen = 32         // characters
)

// parseAPIKey splits a token of the form spgr_sk_<prefix>_<secret> into
// its components. Returns ("", "", false) for any malformed input.
func parseAPIKey(token string) (prefix, secret string, ok bool) {
	if !strings.HasPrefix(token, apiKeyPrefix) {
		return "", "", false
	}
	rest := token[len(apiKeyPrefix):]
	// Expect <8-char-prefix>_<32-char-secret>
	sep := strings.IndexByte(rest, '_')
	if sep != apiKeyPrefixLen {
		return "", "", false
	}
	prefix = rest[:sep]
	secret = rest[sep+1:]
	if len(secret) != apiKeySecretLen {
		return "", "", false
	}
	return prefix, secret, true
}

// argon2idVerify checks whether secret matches the stored PHC-encoded
// argon2id hash. Returns false on any parse or mismatch error (callers
// map all failures to ErrUnauthenticated).
//
// PHC format: $argon2id$v=19$m=<m>,t=<t>,p=<p>$<salt-b64>$<hash-b64>
func argon2idVerify(phc, secret string) bool {
	parts := strings.Split(phc, "$")
	// Expected: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	storedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(secret), salt, t, m, p, uint32(len(storedHash))) //nolint:gosec // G115: len(storedHash) is derived from PHC-encoded hash; value is always in [0, 2^31-1]
	return subtle.ConstantTimeCompare(storedHash, computed) == 1
}

// roleRank gives the privilege ordering of the built-in roles:
// reader < writer < admin. Custom/unranked roles are absent from the map.
var roleRank = map[Role]int{RoleReader: 1, RoleWriter: 2, RoleAdmin: 3}

// roleLessThan reports whether a is strictly less privileged than b.
// Built-in roles are linearly ordered: reader < writer < admin. Custom
// roles return false in either direction (see Storage design's roleLessThan).
func roleLessThan(a, b Role) bool {
	ra, oka := roleRank[a]
	rb, okb := roleRank[b]
	if !oka || !okb {
		return false
	}
	return ra < rb
}

// clampedRole returns the lesser of userRole and downgrade, but only for
// built-in roles. A downgrade on a custom/unranked role has no defined
// ordering, so it is silently a no-op: EffectiveRole equals userRole.
func clampedRole(userRole, downgrade Role) Role {
	if downgrade == "" {
		return userRole
	}
	if roleLessThan(downgrade, userRole) {
		return downgrade
	}
	return userRole
}

// resolveAPIKey implements Tasks 11–15: parse, verify, owner-load,
// EffectiveRole clamp, and fire-and-forget TouchLastUsed.
func (s *pgIdentityStore) resolveAPIKey(ctx context.Context, token string) (*Identity, error) {
	prefix, secret, ok := parseAPIKey(token)
	if !ok {
		return nil, ErrUnauthenticated
	}
	key, err := s.users.LookupAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, storage.ErrAPIKeyNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if !argon2idVerify(key.PHCHash, secret) {
		return nil, ErrUnauthenticated
	}
	if key.RevokedAt != nil || (key.ExpiresAt != nil && !key.ExpiresAt.After(s.now())) {
		return nil, ErrUnauthenticated
	}
	user, err := s.users.GetUserByID(ctx, key.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		// Security-observable: a valid credential for a soft-deleted user.
		// Worth a warn — could indicate a credential that should have been
		// revoked, or an offboarding gap.
		slog.Warn("auth: credential resolved to soft-deleted user (api-key)",
			"user_id", user.ID, "key_id", key.ID)
		return nil, ErrUnauthenticated
	}
	s.tracker.Touch(key.ID)
	return &Identity{
		UserID:        user.ID,
		Subject:       "apikey:" + key.ID,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: clampedRole(Role(user.Role), Role(key.RoleDowngrade)),
		Source:        "apikey",
	}, nil
}

// resolveJWT is implemented in Tasks 16–21.
func (s *pgIdentityStore) resolveJWT(_ context.Context, _ string) (*Identity, error) {
	return nil, ErrUnauthenticated
}

// HasAuth is implemented in Task 22.
func (s *pgIdentityStore) HasAuth(_ context.Context) (bool, error) {
	return false, errors.New("HasAuth not implemented")
}

// rateLimiterFor returns (or lazily creates) the per-issuer limiter.
// Forward-declared for Tasks 16–21 (JIT OIDC path). The jitRateLimiters
// field and this function are used once those tasks are implemented.
//
//nolint:unused // forward-declared for Tasks 16–21; referenced once JIT OIDC is wired
func (s *pgIdentityStore) rateLimiterFor(issuer string) *rate.Limiter {
	if l, ok := s.jitRateLimiters.Load(issuer); ok {
		return l.(*rate.Limiter) //nolint:errcheck // type assertion: sync.Map always stores *rate.Limiter
	}
	refill := rate.Every(time.Hour / time.Duration(s.jitRateRefillPerHr))
	l := rate.NewLimiter(refill, s.jitRateBurst)
	actual, _ := s.jitRateLimiters.LoadOrStore(issuer, l)
	return actual.(*rate.Limiter) //nolint:errcheck // type assertion: sync.Map always stores *rate.Limiter
}
