// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
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
// pgIdentityStore after a successful API-key resolve.
//
// The canonical implementation is *usagetracker.Manager. Tests may use
// other implementations (stubs, no-ops) that satisfy the interface.
type LastUsedTracker interface {
	Touch(keyID string)
}

// pgIdentityStore is the Postgres-backed Resolver impl. Wraps UsersBackend
// + per-issuer OIDCVerifiers; enforces JIT rate-limit and allowlist.
type pgIdentityStore struct {
	users     storage.UsersBackend
	webAuth   storage.WebAuthStore
	verifiers map[string]*OIDCVerifier // issuer -> verifier
	tracker   LastUsedTracker

	jitEnabled         bool
	jitDefaultRole     Role
	jitClaimsMapping   map[string][]config.ClaimMapping // issuer -> mappings
	jitRateLimiters    sync.Map                         // per-issuer token-bucket limiters; keyed by issuer string
	jitRateBurst       int
	jitRateRefillPerHr int
	jitEmailAllowlist  map[string]bool // domain -> true; empty = no allowlist
	loginSyncEnabled   bool

	now func() time.Time
}

// IdentityStoreConfig parametrizes pgIdentityStore at construction.
type IdentityStoreConfig struct {
	Users     storage.UsersBackend
	Verifiers []*OIDCVerifier
	Tracker   LastUsedTracker

	// WebAuth persists/looks up interactive-login web sessions. Optional: when
	// nil, spgr_ws_-prefixed tokens resolve to ErrUnauthenticated.
	WebAuth storage.WebAuthStore

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

	// LoginSyncEnabled turns on metadata + role re-evaluation on interactive
	// login (see loginsync.go). When true, claims-mapping roles are validated
	// at startup even if JITEnabled is false.
	LoginSyncEnabled bool

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
		if v == nil {
			return nil, errors.New("auth: NewIdentityStore: nil verifier in cfg.Verifiers")
		}
		if _, dup := verifiers[v.Issuer()]; dup {
			return nil, fmt.Errorf("auth: duplicate verifier for issuer %q", v.Issuer())
		}
		verifiers[v.Issuer()] = v
	}
	// Validate JIT-related role references against KnownRoles. Catches
	// operator typos (e.g. "reder" instead of "reader") that would create
	// users whose role can never match any rolePerms entry. Login-sync
	// consumes the same JITClaimsMapping/JITDefaultRole, so validation also
	// runs when login-sync is enabled even if JIT is off.
	//
	// The len(cfg.KnownRoles) > 0 guard is deliberate: when JIT or login-sync
	// is enabled but no known-role set is supplied, validation is skipped
	// rather than failing closed. Callers SHOULD pass KnownRoles whenever JIT
	// or login-sync is enabled (see the field docs) so these typos are caught
	// here.
	if (cfg.JITEnabled || cfg.LoginSyncEnabled) && len(cfg.KnownRoles) > 0 {
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
		webAuth:            cfg.WebAuth,
		verifiers:          verifiers,
		tracker:            cfg.Tracker,
		jitEnabled:         cfg.JITEnabled,
		jitDefaultRole:     cfg.JITDefaultRole,
		jitClaimsMapping:   cfg.JITClaimsMapping,
		jitRateBurst:       burst,
		jitRateRefillPerHr: burst,
		jitEmailAllowlist:  allowlist,
		loginSyncEnabled:   cfg.LoginSyncEnabled,
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
	if strings.HasPrefix(token, sessionTokenPrefix) {
		return s.resolveSession(ctx, token)
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
// the API-key resolver regardless of its dot count. The spgr_ws_ session
// prefix is excluded for the same reason. Any future credential prefix that
// could contain dots MUST be added to this guard, or such tokens risk being
// treated as JWTs.
func isJWTShaped(token string) bool {
	return strings.Count(token, ".") == 2 &&
		!strings.HasPrefix(token, apiKeyPrefix) &&
		!strings.HasPrefix(token, sessionTokenPrefix)
}

const (
	apiKeyPrefix    = "spgr_sk_" //nolint:gosec // G101: token-format prefix, not a credential
	apiKeyPrefixLen = 8          // characters
	apiKeySecretLen = 32         // characters
)

const sessionTokenPrefix = "spgr_ws_" //nolint:gosec // G101: token-format prefix, not a credential

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

// IsBuiltinRole reports whether r is one of the ranked built-in roles
// (reader, writer, admin). It is the single source of truth for "ranked" —
// callers validating a RoleDowngrade target (a cap, which must be orderable)
// use it rather than duplicating the role list.
func IsBuiltinRole(r Role) bool {
	_, ok := roleRank[r]
	return ok
}

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

// clampedRole computes a key's EffectiveRole from its owner's role and the
// key's RoleDowngrade cap. An empty downgrade is no cap (the key is its owner's
// role). When both roles are ranked built-ins it returns the lesser, so a cap
// never escalates. Otherwise — a cap is set but a custom/unranked role on
// either side makes the pair incomparable — it fails CLOSED to the
// most-restrictive built-in (reader) rather than silently keeping the owner's
// fuller role (the spgr-rjrt.9 fail-open bug). RoleDowngrade is validated to a
// built-in at key creation; the floor still contains legacy keys and
// custom-role owners. See
// docs/superpowers/specs/2026-06-04-spgr-rjrt-9-role-downgrade-failclosed-design.md.
func clampedRole(userRole, downgrade Role) Role {
	if downgrade == "" {
		return userRole
	}
	_, ownerRanked := roleRank[userRole]
	_, downgradeRanked := roleRank[downgrade]
	if !ownerRanked || !downgradeRanked {
		return RoleReader
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
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: credential resolved to soft-deleted user (api-key)",
			slog.String("user_id", user.ID), slog.String("key_id", key.ID))
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

// resolveSession resolves an opaque web-session token (spgr_ws_...). Mirrors
// resolveAPIKey's error discipline: not-found/revoked/expired/soft-deleted →
// ErrUnauthenticated; any other backend error → ErrTransient.
func (s *pgIdentityStore) resolveSession(ctx context.Context, token string) (*Identity, error) {
	if s.webAuth == nil {
		return nil, ErrUnauthenticated
	}
	sum := sha256.Sum256([]byte(token))
	sess, err := s.webAuth.LookupSessionByHash(ctx, sum[:])
	if err != nil {
		if errors.Is(err, storage.ErrSessionNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if !sess.IsActive(s.now()) {
		return nil, ErrUnauthenticated
	}
	user, err := s.users.GetUserByID(ctx, sess.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: session resolved to soft-deleted user",
			slog.String("user_id", user.ID), slog.String("subject", sess.OIDCSubject))
		return nil, ErrUnauthenticated
	}
	return &Identity{
		UserID:        user.ID,
		Subject:       "oidc:" + sess.OIDCSubject,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: Role(user.Role),
		Source:        "oidc",
	}, nil
}

// peekIssuer extracts the iss claim from an unverified JWT payload. Used
// only to route to the correct OIDCVerifier; the verifier subsequently
// validates signature+audience+expiry.
func peekIssuer(token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}
	if claims.Issuer == "" {
		return "", errors.New("missing iss claim")
	}
	return claims.Issuer, nil
}

// resolveJWT implements Tasks 16–21: issuer peek, verifier routing,
// binding lookup, owner load, soft-delete check, and JIT provisioning.
func (s *pgIdentityStore) resolveJWT(ctx context.Context, token string) (*Identity, error) {
	issuer, err := peekIssuer(token)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	verifier, ok := s.verifiers[issuer]
	if !ok {
		return nil, ErrUnauthenticated
	}
	claims, err := verifier.Verify(ctx, token)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	// Defense-in-depth: the unverified peek issuer (used only for routing)
	// must equal the verified issuer. go-oidc already binds verification to
	// the configured issuer, so a mismatch should be impossible — but
	// asserting it closes the door on a token that claims iss:A in its
	// (unverified) payload while being validly signed under verifier A's
	// configured issuer differing from the embedded claim.
	if claims.Issuer != issuer {
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: JWT issuer mismatch between peek and verified claim",
			slog.String("peek", issuer), slog.String("verified", claims.Issuer))
		return nil, ErrUnauthenticated
	}
	// Binding lookup + user load. JIT path fires on binding miss (Task 18).
	binding, err := s.users.LookupOIDCBinding(ctx, claims.Issuer, claims.Subject)
	if err != nil {
		if !errors.Is(err, storage.ErrOIDCBindingNotFound) {
			return nil, fmt.Errorf("%w: %w", ErrTransient, err)
		}
		// Binding miss → JIT path.
		if !s.jitEnabled {
			return nil, ErrUnauthenticated
		}
		return s.jitResolve(ctx, claims)
	}
	user, err := s.users.GetUserByID(ctx, binding.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		// Security-observable: a valid OIDC binding for a soft-deleted user.
		// The binding wasn't unbound at offboarding; the deleted_at gate
		// catches it, but log so operators can notice the gap.
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: token resolved to soft-deleted user (oidc)",
			slog.String("user_id", user.ID), slog.String("subject", claims.Subject))
		return nil, ErrUnauthenticated
	}
	return &Identity{
		UserID:        user.ID,
		Subject:       "oidc:" + claims.Subject,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: Role(user.Role), // OIDC has no per-key downgrade
		Source:        "oidc",
	}, nil
}

// jitResolve creates a new Human + OIDC binding for an unknown subject.
// Gate order (Tasks 18–21):
//
//  1. Email-domain allowlist (Task 20) — cheap, no budget spent; refused tokens
//     never reach the rate limiter.
//  2. Per-issuer rate-limit gate (Task 19) — bounds eligible creation attempts.
//  3. ClaimsMapping role derivation (Task 21) — only fires at JIT creation; not
//     on subsequent sign-ins, which resolve via the stored binding.
//  4. JITCreateHuman (Task 18) — atomic user + binding creation.
func (s *pgIdentityStore) jitResolve(ctx context.Context, claims *OIDCClaims) (*Identity, error) {
	// (1) Allowlist gate — refused tokens never consume a rate-limit token.
	if len(s.jitEmailAllowlist) > 0 {
		domain := emailDomain(claims.Email)
		if domain == "" {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: JIT refused — empty email claim with non-empty allowlist",
				slog.String("issuer", claims.Issuer))
			return nil, ErrUnauthenticated
		}
		if !s.jitEmailAllowlist[domain] {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: JIT refused — email domain not in allowlist",
				slog.String("issuer", claims.Issuer), slog.String("domain", domain))
			return nil, ErrUnauthenticated
		}
	}

	// (2) Per-issuer rate-limit gate — bounds eligible creation attempts.
	// The token is consumed here, BEFORE JITCreateHuman: a transient create
	// failure still spends a token. This is deliberate — it dampens retry
	// storms during DB degradation rather than letting failed attempts retry
	// against the backend for free.
	//
	// Skipped for interactive logins (user-driven, IdP+PKCE+nonce verified);
	// the limiter targets unsolicited bearer-JWT JIT. The email-domain
	// allowlist (step 1, above) still applies.
	if !InteractiveLoginFromContext(ctx) {
		limiter := s.rateLimiterFor(claims.Issuer)
		if !limiter.Allow() {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: JIT rate-limit exceeded",
				slog.String("issuer", claims.Issuer), slog.String("subject", claims.Subject))
			return nil, ErrUnauthenticated
		}
	}

	// (3) ClaimsMapping: derive role from token claims at JIT time only.
	// Subsequent sign-ins resolve via the binding and use the DB-persisted role.
	role := s.jitDefaultRole
	if role == "" {
		role = RoleReader
	}
	if mappings, ok := s.jitClaimsMapping[claims.Issuer]; ok {
		if mapped := applyClaimsMapping(claims.Raw, mappings); mapped != "" {
			role = Role(mapped)
		}
	}

	// (4) Atomically create user + binding.
	u := &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: claims.Subject, // operator can rename later
		Email:       claims.Email,
		Role:        string(role),
	}
	b := &storage.OIDCBinding{
		Issuer:      claims.Issuer,
		Subject:     claims.Subject,
		EmailAtBind: claims.Email,
	}
	user, _, err := s.users.JITCreateHuman(ctx, u, b)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	return &Identity{
		UserID:        user.ID,
		Subject:       "oidc:" + claims.Subject,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: Role(user.Role),
		Source:        "oidc",
	}, nil
}

// emailDomain extracts the lowercase domain part from an email address.
// Returns "" if the address has no '@', or if '@' is the last character.
func emailDomain(email string) string {
	i := strings.LastIndexByte(email, '@')
	if i < 0 || i == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[i+1:])
}

// applyClaimsMapping evaluates the mapping rules in order. Returns the
// first matching role, or "" if no rule matches.
func applyClaimsMapping(claims map[string]json.RawMessage, rules []config.ClaimMapping) string {
	for _, m := range rules {
		raw, ok := claims[m.Claim]
		if !ok {
			continue
		}
		if matchClaimValue(raw, m.Value) {
			return m.Role
		}
	}
	return ""
}

// matchClaimValue checks whether a claim value matches the target.
// Supports string claims and string-array claims.
func matchClaimValue(raw json.RawMessage, target string) bool {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str == target
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		for _, v := range arr {
			if v == target {
				return true
			}
		}
	}
	return false
}

// HasAuth reports whether any non-bootstrap, non-deleted human user exists.
// Used by warnIfNoAuthOnPublicListen at startup to decide whether to warn
// operators that no credentials are configured.
//
// Single-page safety: HasAuth scans only the first ListUsers page (default
// limit 100) of active humans and post-filters out the bootstrap user. One
// page is sufficient because the storage layer enforces AT MOST ONE active
// bootstrap user (the users_one_bootstrap partial unique index). A 100-row
// page of humans therefore contains at most one bootstrap row, so if any
// non-bootstrap human exists at all, at least one is guaranteed to appear on
// the first page. This is correct by construction, not a truncation bug.
func (s *pgIdentityStore) HasAuth(ctx context.Context) (bool, error) {
	users, err := s.users.ListUsers(ctx, storage.ListUsersFilter{
		Kind: storage.KindHuman,
		// Note: ListUsers does not filter by Bootstrap directly; we filter
		// post-fetch since the bootstrap rows are rare and small in count.
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrTransient, err)
	}
	for _, u := range users {
		if !u.Bootstrap {
			return true, nil
		}
	}
	return false, nil
}

// rateLimiterFor returns (or lazily creates) the per-issuer token-bucket
// limiter. Called by jitResolve (Task 19) to bound JIT creation attempts
// per OIDC issuer.
func (s *pgIdentityStore) rateLimiterFor(issuer string) *rate.Limiter {
	if l, ok := s.jitRateLimiters.Load(issuer); ok {
		return l.(*rate.Limiter) //nolint:errcheck // type assertion: sync.Map always stores *rate.Limiter
	}
	refill := rate.Every(time.Hour / time.Duration(s.jitRateRefillPerHr))
	l := rate.NewLimiter(refill, s.jitRateBurst)
	actual, _ := s.jitRateLimiters.LoadOrStore(issuer, l)
	return actual.(*rate.Limiter) //nolint:errcheck // type assertion: sync.Map always stores *rate.Limiter
}
