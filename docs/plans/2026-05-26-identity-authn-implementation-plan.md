# Identity Authn Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Identity Authn layer specified in `docs/plans/2026-05-22-identity-authn-design.md` — collapse the multi-store routing into one `Resolver.Resolve(ctx, token)` method backed by the new `UsersBackend` (from the Storage plan), split `OIDCStore` into a thin verifier, add JIT user creation with per-issuer rate limiting and email-domain allowlist, and introduce the `Authorizer` interface as the seam between authz mechanisms (static table now, Cedar later).

**Architecture:** Build alongside, then switch, then delete. New `Resolver` interface and `pgIdentityStore` impl live as siblings to the existing `IdentityStore`/`CompositeStore` until everything is wired and tested; the cutover happens in one focused task (#23); old code is removed in #26 once `go build ./...` is green without it. Every task in this plan ends with a compiling, testable package. No half-broken intermediate states.

**Tech Stack:** Go 1.x, ConnectRPC, pgx v5 (via `UsersBackend`), `github.com/coreos/go-oidc/v3` (existing), `github.com/go-jose/go-jose/v4` (existing transitive), `golang.org/x/time/rate` (new dep for per-issuer rate limiting).

**Implements bead:** Implementation of approved design `spgr-n2rw` under epic `spgr-rjrt`. Depends on the Storage plan (`spgr-e82m`) being merged first.

---

## Testing approach

Uses the same four-category framework defined in the Storage plan. Recap:

- **Happy** — normal success per code path.
- **Invariants** — system-wide rules (soft-deleted user can't authenticate; ClaimsMapping evaluates only at JIT; rate-limit applies per-issuer not globally).
- **Boundaries** — edges (malformed tokens, NotFound credentials, rate exhaustion, allowlist miss, error categorization).
- **E2E** — multi-component flows through Resolver → Authorizer → Decision.

Each behavior-introducing task tags categories under `**Covers:**`. Unit tests use a hand-rolled `UsersBackend` stub (Task 8); integration tests reuse the testcontainer harness from Storage via the **exported** `postgrestest.SharedPool(t, ctx)` helper (Storage plan Task 32 — the in-package `sharedTestPool` is not importable cross-package), `//go:build integration`.

---

## File Structure

**Create (Phase A — alongside the old code):**

- `internal/auth/authorizer.go` — `Authorizer` interface + `Decision` type.
- `internal/auth/static_authorizer.go` — `StaticTableAuthorizer` impl.
- `internal/auth/static_authorizer_test.go` — unit tests.
- `internal/auth/oidc_verifier.go` — `OIDCVerifier` type (thin JWT verifier) + `OIDCClaims` struct. Sibling to existing `oidc_store.go`.
- `internal/auth/oidc_verifier_test.go` — unit tests against an httptest OIDC issuer.
- `internal/auth/resolver.go` — new `Resolver` interface (single `Resolve` method).
- `internal/auth/identitystore.go` — `pgIdentityStore` impl of `Resolver`. Sibling to existing `CompositeStore`.
- `internal/auth/identitystore_test.go` — unit tests (with `UsersBackend` stub).
- `internal/auth/identitystore_jit_test.go` — JIT-path tests (rate limit, email allowlist).
- `internal/auth/identitystore_integration_test.go` — integration tests (real `AuthStore`).
- `internal/auth/usersbackend_stub_test.go` — hand-rolled `UsersBackend` stub for unit tests.
- `internal/auth/usagetracker/usagetracker.go` — async batched `TouchLastUsed` updater.
- `internal/auth/usagetracker/usagetracker_test.go` — unit tests.

**Modify (Phase A — additive only):**

- `internal/auth/auth.go` — Add `UserID`, `EffectiveRole`, `Email` to `Identity` struct. **Do not remove `Permissions` yet** — old consumers still use it. The cleanup happens in Phase C after old code is deleted.
- `internal/auth/store.go` — Add `ErrUnauthenticated` and `ErrTransient` sentinels. Keep old sentinels in place.

**Modify (Phase B — the switch):**

- `internal/auth/interceptor.go` — change signature from `NewAuthInterceptor(store IdentityStore)` to `NewAuthInterceptor(resolver Resolver, authorizer Authorizer)`; call `resolver.Resolve` + `authorizer.Authorize`; map errors per the design's three categories; preserve cookie fallback.
- `internal/auth/middleware.go` — same shape change for HTTP cookie path.
- `cmd/specgraph/serve.go` — construct `pgIdentityStore`, `StaticTableAuthorizer`, `usagetracker.Manager`; remove `ConfigStore`/`CompositeStore` instantiation; remove `auth.Bootstrap()` invocation (moves to Bootstrap & UX plan); remove `cfg.Auth.Mode` validation.

**Delete (Phase C — once nothing references them):**

- `internal/auth/config_store.go` + `config_store_test.go`
- `internal/auth/composite_store.go` + `composite_store_test.go`
- `internal/auth/token_store.go` + `token_store_test.go`
- `internal/auth/oidc_store.go` + `oidc_store_test.go`
- Old sentinels in `store.go` (`ErrUnknownKey`, `ErrNoOIDC`, `ErrUnknownIssuer`, `ErrInvalidToken`).
- `IdentityStore` interface (replaced by `Resolver`).
- `Identity.Permissions` field (now unreferenced).
- `auth.HasPermission` exported helper (moves into `static_authorizer.go` as `hasPermissionInternal`).
- `auth.Bootstrap()` and friends move to the Bootstrap & UX plan; gone from `internal/auth/bootstrap.go` here.

**Don't touch:** `internal/auth/permissions.go` (`rpcPermissions` table) — survives this plan; consumed by `StaticTableAuthorizer`. The Cedar plan removes it.

---

## Task 1: Add new fields to `Identity` additively

The Identity struct gains `UserID`, `EffectiveRole`, `Email` without losing anything. Existing consumers continue to work because they don't reference the new fields; new code paths read them as they come online.

**Files:**

- Modify: `internal/auth/auth.go`

**Covers:** N/A (type-only change; behavior tests in later tasks).

- [ ] **Step 1: Update the struct**

Replace the existing `Identity` declaration with:

```go
// Identity represents an authenticated principal. Produced by Resolver.Resolve
// (or, until the Phase B cutover, by the legacy IdentityStore methods);
// consumed by the interceptor and by Authorizer implementations.
//
// Subject keeps its original namespacing format ("apikey:<id>", "oidc:<sub>",
// historically "local:<user>") for log-filter and dashboard compatibility.
// After Phase B the "local:" prefix is no longer produced; the constant is
// retained as a historical comment only.
type Identity struct {
	// New fields (populated by Resolver.Resolve from Task 12 onward).
	UserID        string // uuid (storage.User.ID); empty for legacy paths
	EffectiveRole Role   // min(Role, key.RoleDowngrade); equals Role for OIDC
	Email         string // from User row; populated by new resolver only

	// Existing fields.
	Subject     string          // "apikey:<id>" | "oidc:<sub>" | (legacy) "local:<user>"
	DisplayName string          // human-friendly name
	Role        Role            // role name (built-in or custom)
	Permissions map[string]bool // raw entries from role definition (legacy; removed in Phase C)
	Source      string          // "local" | "apikey" | "oidc" ("local" removed in Phase C)
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS. The new fields are zero-valued for existing consumers; nothing breaks.

- [ ] **Step 3: Verify existing tests still pass**

Run: `cd internal/auth && go test ./...`

Expected: PASS (existing auth tests unaffected by additive change).

- [ ] **Step 4: Commit**

```bash
git add internal/auth/auth.go
git commit -s -m "feat(auth): add UserID, EffectiveRole, Email to Identity (additive)"
```

---

## Task 2: Add new sentinel errors

The new resolver produces `ErrUnauthenticated` and `ErrTransient` so the interceptor can map them to distinct HTTP codes (401 vs 503). Existing sentinels stay; they'll be removed in Phase C.

**Files:**

- Modify: `internal/auth/store.go`

**Covers:** N/A (definitions only).

- [ ] **Step 1: Append new sentinels**

In `internal/auth/store.go`, before the `IdentityStore` interface declaration, add:

```go
// ErrUnauthenticated indicates a credential failure: missing, malformed,
// expired, revoked, soft-deleted user, JIT-rate-limited, allowlist
// mismatch, or any other "this principal isn't allowed to authenticate"
// condition. The interceptor maps this to connect.CodeUnauthenticated.
//
// Produced by the new Resolver impl (pgIdentityStore). The legacy
// IdentityStore methods still produce ErrUnknownKey etc. until Phase C.
var ErrUnauthenticated = errors.New("auth: unauthenticated")

// ErrTransient indicates a backend failure unrelated to the credential:
// database unavailable, pool exhausted, network timeout. The interceptor
// maps this to connect.CodeUnavailable so callers know to retry.
//
// Errors of this kind wrap the underlying cause; tests use errors.Is to
// detect ErrTransient.
var ErrTransient = errors.New("auth: transient backend error")
```

- [ ] **Step 2: Verify compile and existing tests**

```bash
cd internal/auth && go build ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/store.go
git commit -s -m "feat(auth): add ErrUnauthenticated and ErrTransient sentinels"
```

---

## Task 3: Define the `Authorizer` interface

The seam between today's static-table authz and the future Cedar engine. Defined now; consumed in Phase B's interceptor diff.

**Files:**

- Create: `internal/auth/authorizer.go`

**Covers:** N/A.

- [ ] **Step 1: Write the file**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// Authorizer decides whether a resolved Identity may invoke procedure
// with the given request body. The current StaticTableAuthorizer impl
// (static_authorizer.go) consults the static rpcPermissions table;
// the Cedar plan will add a CedarAuthorizer impl that swaps in without
// any caller changes.
//
// req carries the unmarshaled request body so future authorizers
// (Cedar, ownership rules) can inspect resource attributes. Today's
// StaticTableAuthorizer ignores req.
type Authorizer interface {
	Authorize(ctx context.Context, id *Identity, procedure string, req any) (Decision, error)
}

// Decision is the outcome of an Authorize call.
//
// Allowed=true means the handler should run. Allowed=false means the
// interceptor returns connect.CodePermissionDenied.
//
// Reason carries a short structured tag for audit emission and logging
// (e.g., "static-table-allow:spec:read", "cedar-deny:no-policy-matched").
// The interceptor does not parse it.
type Decision struct {
	Allowed bool
	Reason  string
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/authorizer.go
git commit -s -m "feat(auth): define Authorizer interface (seam for static-table vs Cedar)"
```

---

## Task 4: Implement `StaticTableAuthorizer`

The current authz mechanism, wrapped in the new interface so the Cedar plan can swap it out with one new file.

**Files:**

- Create: `internal/auth/static_authorizer.go`
- Create: `internal/auth/static_authorizer_test.go`

**Covers:** Happy (allow on match) + Boundary (deny on role-mismatch, error on unconfigured procedure).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

func TestStaticTableAuthorizer_AllowOnMatch(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleReader: {"*:read"},
	})
	id := &auth.Identity{Role: auth.RoleReader, EffectiveRole: auth.RoleReader}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
	require.Contains(t, d.Reason, "static-table-allow")
}

func TestStaticTableAuthorizer_DenyOnRoleMismatch(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleReader: {"*:read"},
	})
	id := &auth.Identity{Role: auth.RoleReader, EffectiveRole: auth.RoleReader}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "static-table-deny")
}

func TestStaticTableAuthorizer_ErrorOnUnconfigured(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(nil)
	id := &auth.Identity{Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	_, err := a.Authorize(context.Background(), id, "/unconfigured/procedure", nil)
	require.Error(t, err)
}

func TestStaticTableAuthorizer_AdminWildcard(t *testing.T) {
	a := auth.NewStaticTableAuthorizer(map[auth.Role][]string{
		auth.RoleAdmin: {"*:*"},
	})
	id := &auth.Identity{Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin}
	d, err := a.Authorize(context.Background(), id,
		specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestStaticTableAuthorizer' -v`

Expected: FAIL ("undefined: auth.NewStaticTableAuthorizer").

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
)

// DefaultRolePermissions defines the built-in role permission bundles.
//
// RELOCATED here from config_store.go in this task. config_store.go is
// deleted in Task 30, but LoadRolePerms (below) and the legacy code both
// reference DefaultRolePermissions, so it must live in a file that
// survives. static_authorizer.go is the natural home (it owns role→perm
// policy) and itself survives until the Cedar plan. See Step 3b for the
// matching removal from config_store.go.
var DefaultRolePermissions = map[Role][]string{
	RoleReader: {"*:read"},
	RoleWriter: {"*:read", "*:write"},
	RoleAdmin:  {"*:*"},
}

// StaticTableAuthorizer implements Authorizer by consulting the static
// rpcPermissions table (permissions.go) and a snapshot of role→permissions
// loaded at construction time. This impl will be deleted when the Cedar
// plan introduces CedarAuthorizer.
type StaticTableAuthorizer struct {
	rolePerms map[Role]map[string]bool
}

// NewStaticTableAuthorizer builds a StaticTableAuthorizer from a
// role→[]permission mapping. The constructor expands the lists into a
// per-role lookup map for cheap membership checks at Authorize time.
//
// rolePerms is typically built by LoadRolePerms(cfg.Auth.Roles) — see
// `LoadRolePerms` (exported) in this file.
func NewStaticTableAuthorizer(rolePerms map[Role][]string) *StaticTableAuthorizer {
	expanded := make(map[Role]map[string]bool, len(rolePerms))
	for role, perms := range rolePerms {
		entry := make(map[string]bool, len(perms))
		for _, p := range perms {
			entry[p] = true
		}
		expanded[role] = entry
	}
	return &StaticTableAuthorizer{rolePerms: expanded}
}

// Authorize implements the Authorizer interface.
func (a *StaticTableAuthorizer) Authorize(_ context.Context, id *Identity, procedure string, _ any) (Decision, error) {
	required, ok := rpcPermissions[procedure]
	if !ok {
		return Decision{}, fmt.Errorf("static-table: unconfigured procedure %q", procedure)
	}
	perms, ok := a.rolePerms[id.EffectiveRole]
	if !ok {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("static-table-deny:unknown-role:%s", id.EffectiveRole),
		}, nil
	}
	if hasPermissionInternal(perms, required) {
		return Decision{
			Allowed: true,
			Reason:  fmt.Sprintf("static-table-allow:%s", required),
		}, nil
	}
	return Decision{
		Allowed: false,
		Reason:  fmt.Sprintf("static-table-deny:%s not in %s", required, id.EffectiveRole),
	}, nil
}

// hasPermissionInternal is the wildcard-matching helper used by
// StaticTableAuthorizer. The same logic is temporarily duplicated in the
// exported auth.HasPermission (still consumed by legacy code in Phase A).
// The duplication is intentional and short-lived: Task 31 removes the
// exported HasPermission once the legacy stores are gone, leaving this
// package-private copy as the sole implementation. Package-private here
// because no caller outside this struct should do raw perm checks — they
// go through Authorize.
func hasPermissionInternal(perms map[string]bool, required string) bool {
	if len(perms) == 0 {
		return false
	}
	if perms["*:*"] {
		return true
	}
	if perms[required] {
		return true
	}
	parts := strings.SplitN(required, ":", 2)
	if len(parts) != 2 {
		return false
	}
	service, action := parts[0], parts[1]
	if perms[service+":*"] {
		return true
	}
	if perms["*:"+action] {
		return true
	}
	return false
}

// LoadRolePerms builds the rolePerms map from the YAML auth.roles section
// combined with the built-in DefaultRolePermissions. Exported so serve.go
// (a different package) can construct both the StaticTableAuthorizer and
// the KnownRoles set for JIT validation from a single source.
//
// custom is cfg.Auth.Roles from internal/config — a real named type, not
// an anonymous struct. The auth package imports internal/config (already
// imported elsewhere in the package, e.g., OIDCVerifier).
func LoadRolePerms(custom map[string]config.RoleConfig) map[Role][]string {
	out := make(map[Role][]string, len(DefaultRolePermissions)+len(custom))
	for role, perms := range DefaultRolePermissions {
		out[role] = append([]string(nil), perms...)
	}
	for name, rc := range custom {
		out[Role(name)] = append([]string(nil), rc.Permissions...)
	}
	return out
}
```

- [ ] **Step 3b: Remove the now-relocated var from `config_store.go`**

`DefaultRolePermissions` now lives in `static_authorizer.go`. Delete the duplicate declaration from `internal/auth/config_store.go` (the `var DefaultRolePermissions = map[Role][]string{...}` block, ~lines 22-27) to avoid a duplicate-package-var compile error. Leave config_store.go's *usage* of it (e.g., `for role, perms := range DefaultRolePermissions`) intact — it still resolves, because the var is now defined elsewhere in the same `auth` package. config_store.go itself is deleted later in Task 30; this step just prevents the duplicate definition in the interim.

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestStaticTableAuthorizer' -v`

Expected: build PASS (no duplicate `DefaultRolePermissions`), tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/static_authorizer.go internal/auth/static_authorizer_test.go internal/auth/config_store.go
git commit -s -m "feat(auth): add StaticTableAuthorizer; relocate DefaultRolePermissions to survive Task 30"
```

---

## Task 5: Create `OIDCVerifier` alongside `OIDCStore`

Sibling to the existing `oidc_store.go` — no rename, no deletion. Both exist; the new resolver uses `OIDCVerifier`, the old `CompositeStore` keeps using `OIDCStore` until the Phase B cutover.

**Files:**

- Create: `internal/auth/oidc_verifier.go`

**Covers:** N/A (behavior tests in Task 6).

- [ ] **Step 1: Write the file**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/config"
)

// OIDCClaims is the verified claim payload returned by OIDCVerifier.Verify.
// Subject and Email are unmarshaled for convenience; Raw retains all claims
// for downstream ClaimsMapping evaluation at JIT time.
type OIDCClaims struct {
	Issuer  string
	Subject string
	Email   string
	Raw     map[string]json.RawMessage
}

// OIDCVerifier verifies JWTs against a single OIDC provider. Successor to
// OIDCStore: no DB dependency, no role computation, no Identity construction.
// The resolver materializes the Identity from claims; the verifier just
// validates signature/audience/expiry.
type OIDCVerifier struct {
	providerID string
	issuer     string
	verifier   *oidc.IDTokenVerifier
}

// NewOIDCVerifier discovers the OIDC provider configuration and builds a
// JWT verifier. ctx should carry a startup deadline (e.g., 10s).
func NewOIDCVerifier(ctx context.Context, cfg config.OIDCProviderConfig) (*OIDCVerifier, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider %s: %w", cfg.ID, err)
	}
	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: audience})
	return &OIDCVerifier{
		providerID: cfg.ID,
		issuer:     cfg.Issuer,
		verifier:   verifier,
	}, nil
}

// Issuer returns the OIDC issuer URL.
func (v *OIDCVerifier) Issuer() string { return v.issuer }

// ProviderID returns the configured provider ID (used in logs).
func (v *OIDCVerifier) ProviderID() string { return v.providerID }

// Verify validates the token's signature, audience, and expiry. On success
// returns the decoded claims; on failure returns a wrapped error.
//
// The resolver maps any error from this function to ErrUnauthenticated;
// callers should not try to distinguish verification failure modes.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*OIDCClaims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		slog.Warn("auth: OIDC token verification failed",
			"provider", v.providerID, "error", err.Error())
		return nil, fmt.Errorf("oidc verify: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}
	c := &OIDCClaims{
		Issuer:  idToken.Issuer,
		Subject: idToken.Subject,
		Raw:     raw,
	}
	if rawEmail, ok := raw["email"]; ok {
		var email string
		if jsonErr := json.Unmarshal(rawEmail, &email); jsonErr == nil {
			c.Email = email
		}
	}
	return c, nil
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/oidc_verifier.go
git commit -s -m "feat(auth): add OIDCVerifier alongside OIDCStore"
```

---

## Task 6: Test `OIDCVerifier`

Pull the test issuer pattern from the existing `oidc_store_test.go` (which is still in place — it'll be deleted in Phase C). Adapt for the new `OIDCVerifier` shape.

**Files:**

- Create: `internal/auth/oidc_verifier_test.go`

**Covers:** Happy (valid token) + Boundary (bad audience, expired, malformed).

- [ ] **Step 1: Write the test harness and failing tests**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// oidcTestIssuer spins up an httptest server serving OIDC discovery + JWKS.
// Tests mint JWTs signed with its private key.
type oidcTestIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newOIDCTestIssuer(t *testing.T) *oidcTestIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   srv.URL,
			"jwks_uri": srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "k1", Algorithm: string(jose.RS256), Use: "sig"}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &oidcTestIssuer{server: srv, key: key}
}

func (p *oidcTestIssuer) mintToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"),
	)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func TestOIDCVerifier_VerifyValidToken(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss":   p.server.URL,
		"sub":   "user-123",
		"aud":   "aud-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": "alice@example.com",
	})
	claims, err := v.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "user-123", claims.Subject)
	require.Equal(t, "alice@example.com", claims.Email)
}

func TestOIDCVerifier_RejectsBadAudience(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "expected",
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "wrong",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = v.Verify(ctx, token)
	require.Error(t, err)
}

func TestOIDCVerifier_RejectsExpired(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "aud-1",
		"exp": time.Now().Add(-time.Hour).Unix(), "iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	_, err = v.Verify(ctx, token)
	require.Error(t, err)
}

func TestOIDCVerifier_RejectsMalformed(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	_, err = v.Verify(ctx, "not.a.jwt")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run the tests**

Run: `cd internal/auth && go test -run 'TestOIDCVerifier' -v`

Expected: PASS (the impl from Task 5 is real).

- [ ] **Step 3: Commit**

```bash
git add internal/auth/oidc_verifier_test.go
git commit -s -m "test(auth): cover OIDCVerifier signature/audience/expiry checks"
```

---

## Task 7: Define `Resolver` interface

The new interface that `pgIdentityStore` will satisfy. Sibling to the legacy `IdentityStore`. The interceptor will switch from depending on `IdentityStore` to depending on `Resolver` in Phase B.

**Files:**

- Create: `internal/auth/resolver.go`

**Covers:** N/A.

- [ ] **Step 1: Write the file**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// Resolver dispatches an authentication token to an Identity. Successor
// to the legacy IdentityStore interface (which routed across multiple
// stores). The interceptor depends on Resolver after the Phase B cutover.
type Resolver interface {
	// Resolve returns the Identity for the given bearer token.
	//
	// Returns ErrUnauthenticated for any credential failure (missing,
	// malformed, expired, revoked, soft-deleted, JIT-rate-limited,
	// allowlist mismatch). Returns ErrTransient (wrapping the cause) for
	// backend failures (DB down, pool exhausted, etc.). Propagates
	// context.Canceled / context.DeadlineExceeded unwrapped.
	Resolve(ctx context.Context, token string) (*Identity, error)

	// HasAuth reports whether any non-bootstrap, non-deleted user
	// exists. Used by warnIfNoAuthOnPublicListen at startup.
	HasAuth(ctx context.Context) (bool, error)
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/resolver.go
git commit -s -m "feat(auth): define Resolver interface as successor to IdentityStore"
```

---

## Task 8: Hand-rolled `UsersBackend` stub for unit tests

Unit tests for the new `pgIdentityStore` mock `UsersBackend` rather than going to Postgres. The project doesn't use testify/mock; the stub is ~150 lines of method-pointer indirection.

**Files:**

- Create: `internal/auth/usersbackend_stub_test.go`

**Covers:** N/A (test fixture).

- [ ] **Step 1: Write the stub**

This file is also the home for the shared PHC test fixtures (`stubPHCSecret`, `stubPHCHash`, `stubAPIKeyToken`) so they exist from Task 8 onward — `activeKey` references `stubPHCHash`, and later tasks (12–15, 33–34) reference all three. Defining them here avoids a forward reference that would break the "every task green" rule.

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/specgraph/specgraph/internal/storage"
)

// --- Shared API-key test fixtures ---

// stubPHCSecret is the canonical 32-char test secret. It MUST be exactly
// 32 characters to match the API-key parser's expected secret length
// (apiKeySecretLen in identitystore.go). TestStubSecretLength guards this
// at runtime. Do not change without updating nothing else — all tokens are
// built from this constant via stubAPIKeyToken.
const stubPHCSecret = "test-secret-32-chars-padding-aaa"

// stubPHCHash is the argon2id PHC encoding of stubPHCSecret, computed once
// in init() with the same parameters production uses. A token built by
// stubAPIKeyToken verifies against this hash.
var stubPHCHash string

func init() {
	salt := []byte("teststaltestsalt") // exactly 16 bytes
	hash := argon2.IDKey([]byte(stubPHCSecret), salt, 2, 19456, 1, 32)
	stubPHCHash = fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))
}

// stubAPIKeyToken builds a well-formed API-key token whose secret is
// stubPHCSecret, so it verifies against stubPHCHash. Callers pass an
// 8-char prefix.
func stubAPIKeyToken(prefix string) string {
	return "spgr_sk_" + prefix + "_" + stubPHCSecret
}

// usersBackendStub is a hand-rolled stub for storage.UsersBackend used
// by Resolver unit tests. Each field is a function the test sets to
// control behavior. Unset functions return a default that flags an
// unexpected-call test bug.
type usersBackendStub struct {
	lookupAPIKey      func(ctx context.Context, prefix string) (*storage.APIKey, error)
	lookupOIDCBinding func(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error)
	getUserByID       func(ctx context.Context, id string) (*storage.User, error)
	getBootstrap      func(ctx context.Context) (*storage.User, error)
	jitCreateHuman    func(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error)
	touchLastUsed     func(ctx context.Context, keyID string) error
	listUsers         func(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error)
}

func (s *usersBackendStub) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*storage.APIKey, error) {
	if s.lookupAPIKey == nil {
		return nil, storage.ErrAPIKeyNotFound
	}
	return s.lookupAPIKey(ctx, prefix)
}

func (s *usersBackendStub) LookupOIDCBinding(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
	if s.lookupOIDCBinding == nil {
		return nil, storage.ErrOIDCBindingNotFound
	}
	return s.lookupOIDCBinding(ctx, issuer, subject)
}

func (s *usersBackendStub) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	if s.getUserByID == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.getUserByID(ctx, id)
}

func (s *usersBackendStub) GetBootstrap(ctx context.Context) (*storage.User, error) {
	if s.getBootstrap == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.getBootstrap(ctx)
}

func (s *usersBackendStub) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	if s.jitCreateHuman == nil {
		return nil, nil, errUnexpectedCall("JITCreateHuman")
	}
	return s.jitCreateHuman(ctx, u, b)
}

func (s *usersBackendStub) TouchLastUsed(ctx context.Context, keyID string) error {
	if s.touchLastUsed == nil {
		return nil // fire-and-forget; default no-op
	}
	return s.touchLastUsed(ctx, keyID)
}

func (s *usersBackendStub) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	if s.listUsers == nil {
		return nil, nil
	}
	return s.listUsers(ctx, f)
}

// Methods unused by Resolver — these are guards that fail loud if the
// resolver calls them unexpectedly.
func (s *usersBackendStub) CreateHuman(_ context.Context, _ *storage.User, _ *storage.OIDCBinding) (*storage.User, error) {
	return nil, errUnexpectedCall("CreateHuman")
}
func (s *usersBackendStub) CreateServiceAccount(_ context.Context, _ *storage.User) (*storage.User, error) {
	return nil, errUnexpectedCall("CreateServiceAccount")
}
func (s *usersBackendStub) UpdateUserRole(_ context.Context, _, _ string) error {
	return errUnexpectedCall("UpdateUserRole")
}
func (s *usersBackendStub) SoftDeleteUser(_ context.Context, _ string) error {
	return errUnexpectedCall("SoftDeleteUser")
}
func (s *usersBackendStub) PurgeUser(_ context.Context, _ string) error {
	return errUnexpectedCall("PurgeUser")
}
func (s *usersBackendStub) CreateAPIKey(_ context.Context, _ *storage.APIKey) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("CreateAPIKey")
}
func (s *usersBackendStub) RevokeAPIKey(_ context.Context, _ string) error {
	return errUnexpectedCall("RevokeAPIKey")
}
func (s *usersBackendStub) RotateAPIKey(_ context.Context, _ string, _ *storage.APIKey) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("RotateAPIKey")
}
func (s *usersBackendStub) ListAPIKeys(_ context.Context, _ storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return nil, errUnexpectedCall("ListAPIKeys")
}
func (s *usersBackendStub) ListOIDCBindings(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
	return nil, errUnexpectedCall("ListOIDCBindings")
}
func (s *usersBackendStub) UnbindOIDC(_ context.Context, _ string) error {
	return errUnexpectedCall("UnbindOIDC")
}

func errUnexpectedCall(method string) error {
	return errors.New("usersBackendStub: unexpected call to " + method + " (test bug)")
}

// activeUser builds an active user for tests.
func activeUser(id, role string, kind storage.Kind) *storage.User {
	return &storage.User{
		ID: id, Kind: kind, Role: role, DisplayName: "test-" + id,
		CreatedAt: time.Now(),
	}
}

// activeKey builds an active API key for tests. PHCHash is the verifiable
// stubPHCHash (computed in init() from stubPHCSecret), so a token built via
// stubAPIKeyToken(prefix) will successfully argon2id-verify against it.
func activeKey(id, userID, prefix string) *storage.APIKey {
	return &storage.APIKey{
		ID: id, UserID: userID, Prefix: prefix,
		PHCHash:   stubPHCHash,
		CreatedAt: time.Now(),
	}
}
```

Also add a guard test that pins the secret length. Put it in `identitystore_test.go` (created in Task 9) which already imports `testing` and `testify/require` — keep the fixtures file free of test-framework imports. This is the runtime equivalent of a compile-time assertion — `apiKeySecretLen` is unexported in package `auth`, so `auth_test` can't reference it directly; we assert the literal 32 and rely on the API-key parse tests (Task 11) to catch any drift in the parser's expectation:

```go
// In identitystore_test.go:
func TestStubSecretLength(t *testing.T) {
	require.Len(t, stubPHCSecret, 32,
		"stubPHCSecret must be 32 chars to match the API-key parser's secret length")
}
```

> **Note:** the `PHCHash` value above is a stub; tests that exercise argon2id verification (Task 11) will use a precomputed PHC string for a known-plaintext "test-secret".

- [ ] **Step 2: Verify the stub compiles**

Run: `cd internal/auth && go vet ./...`

Expected: PASS. The stub satisfies `storage.UsersBackend` (every method signature matches).

- [ ] **Step 3: Commit**

```bash
git add internal/auth/usersbackend_stub_test.go
git commit -s -m "test(auth): add hand-rolled UsersBackend stub for unit tests"
```

---

## Task 9: Scaffold `pgIdentityStore` + constructor

**Files:**

- Create: `internal/auth/identitystore.go`
- Create: `internal/auth/identitystore_test.go`

**Covers:** Boundary (constructor rejects nil inputs).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestNewIdentityStore_RequiresUsers(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Users required")
}

func TestNewIdentityStore_RequiresTracker(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: &usersBackendStub{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Tracker required")
}

func TestNewIdentityStore_BuildsSuccessfully(t *testing.T) {
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   &usersBackendStub{},
		Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestNewIdentityStore_RejectsUnknownJITDefaultRole(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          &usersBackendStub{},
		Tracker:        &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: "reder", // typo for "reader"
		KnownRoles:     map[auth.Role]bool{auth.RoleReader: true, auth.RoleWriter: true, auth.RoleAdmin: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "JITDefaultRole")
}

func TestNewIdentityStore_RejectsUnknownClaimsMappingRole(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          &usersBackendStub{},
		Tracker:        &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		KnownRoles:     map[auth.Role]bool{auth.RoleReader: true},
		JITClaimsMapping: map[string][]config.ClaimMapping{
			"https://issuer": {{Claim: "groups", Value: "admins", Role: "superuser"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown role")
}

// noopTracker implements auth.LastUsedTracker as a no-op stub used until
// Task 25 wires usagetracker.Manager.
type noopTracker struct{}

func (noopTracker) Touch(string) {}
```

These two tests reference `config.ClaimMapping` and `auth.RoleReader`/`RoleWriter`/`RoleAdmin`; add `"github.com/specgraph/specgraph/internal/config"` to the test imports.

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestNewIdentityStore' -v`

Expected: FAIL — `auth.NewIdentityStore`, `auth.IdentityStoreConfig`, and `auth.LastUsedTracker` don't exist yet.

- [ ] **Step 3: Write the scaffold**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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
	jitRateLimiters    sync.Map                          // issuer -> *rate.Limiter
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
func NewIdentityStore(cfg IdentityStoreConfig) (Resolver, error) {
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

// Resolve is implemented in Tasks 10–21.
func (s *pgIdentityStore) Resolve(_ context.Context, _ string) (*Identity, error) {
	return nil, errors.New("Resolve not implemented")
}

// HasAuth is implemented in Task 22.
func (s *pgIdentityStore) HasAuth(_ context.Context) (bool, error) {
	return false, errors.New("HasAuth not implemented")
}

// rateLimiterFor returns (or lazily creates) the per-issuer limiter.
func (s *pgIdentityStore) rateLimiterFor(issuer string) *rate.Limiter {
	if l, ok := s.jitRateLimiters.Load(issuer); ok {
		return l.(*rate.Limiter)
	}
	refill := rate.Every(time.Hour / time.Duration(s.jitRateRefillPerHr))
	l := rate.NewLimiter(refill, s.jitRateBurst)
	actual, _ := s.jitRateLimiters.LoadOrStore(issuer, l)
	return actual.(*rate.Limiter)
}
```

- [ ] **Step 4: Run `go mod tidy`**

The plan introduces a direct import of `golang.org/x/time/rate`. Today's `go.mod` lists `golang.org/x/time` as `// indirect`. Promote it to a direct dependency:

Run: `go mod tidy && go build ./...`

Expected: `go.mod` line for `golang.org/x/time` loses the `// indirect` comment; build passes.

- [ ] **Step 5: Run the tests**

Run: `cd internal/auth && go test -run 'TestNewIdentityStore' -v`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): scaffold pgIdentityStore with constructor validation"
```

---

## Task 10: Implement token-shape dispatch in `Resolve`

`Resolve` dispatches on whether the token is JWT-shaped (3 dot-separated base64 segments) or an API-key (`spgr_sk_<prefix>_<secret>`). Both branches return "not implemented" stubs; subsequent tasks fill them in.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (token-shape dispatch picks the right branch) + Boundary (empty/malformed token returns ErrUnauthenticated).

- [ ] **Step 1: Write the failing test**

Append to `identitystore_test.go`:

```go
import (
	"context"
	"errors"
)

func TestResolve_EmptyTokenUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolve_JWTShapeRoutesToOIDC(t *testing.T) {
	store := newTestIdentityStore(t)
	// 3-segment string but garbage payload — dispatches to OIDC, which
	// will fail because no verifier matches the issuer.
	_, err := store.Resolve(context.Background(), "abc.def.ghi")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolve_APIKeyShapeRoutesToKeyPath(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "spgr_sk_abc12345_thirtytwocharsecretthirtytwocha")
	require.ErrorIs(t, err, auth.ErrUnauthenticated) // no key in stub
}

// newTestIdentityStore builds an empty pgIdentityStore for dispatch tests.
func newTestIdentityStore(t *testing.T) auth.Resolver {
	t.Helper()
	r, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   &usersBackendStub{},
		Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	return r
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolve' -v`

Expected: FAIL — `Resolve` still returns the "not implemented" string error, not `ErrUnauthenticated`.

- [ ] **Step 3: Implement dispatch**

In `identitystore.go`, replace the `Resolve` stub:

```go
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
func isJWTShaped(token string) bool {
	return strings.Count(token, ".") == 2 && !strings.HasPrefix(token, "spgr_sk_")
}

// resolveAPIKey is implemented in Tasks 11–15.
func (s *pgIdentityStore) resolveAPIKey(_ context.Context, _ string) (*Identity, error) {
	return nil, ErrUnauthenticated
}

// resolveJWT is implemented in Tasks 16–21.
func (s *pgIdentityStore) resolveJWT(_ context.Context, _ string) (*Identity, error) {
	return nil, ErrUnauthenticated
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolve' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): Resolve dispatches on token shape (JWT vs API-key)"
```

---

## Task 11: API-key — parse `spgr_sk_<prefix>_<secret>`

Split the token into prefix (8 chars) and secret (32 chars). Malformed tokens return `ErrUnauthenticated` immediately.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (well-formed splits cleanly) + Boundary (missing prefix, wrong length, no underscores, wrong vendor).

- [ ] **Step 1: Failing test**

```go
func TestResolveAPIKey_MalformedTokens(t *testing.T) {
	store := newTestIdentityStore(t)
	bad := []string{
		"not-a-key",
		"spgr_sk_",                 // missing parts
		"spgr_sk_short_secret",     // prefix too short
		"spgr_sk_abc12345_short",   // secret too short
		"spgr_pk_abc12345_thirtytwocharsecretthirtytwocha", // wrong vendor prefix
	}
	for _, tok := range bad {
		_, err := store.Resolve(context.Background(), tok)
		require.ErrorIs(t, err, auth.ErrUnauthenticated, "token %q should be Unauthenticated", tok)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey_Malformed' -v`

Expected: **PASS for the wrong reason** — the current `resolveAPIKey` returns `ErrUnauthenticated` for every token regardless of shape, so malformed-token assertions trivially succeed. The negation isn't covered. To force a real failing test, add the positive case below which DOES fail today (stub returns ErrUnauthenticated even though the token is well-formed):

```go
func TestResolveAPIKey_WellFormedReachesLookup(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			called = true
			require.Equal(t, "abc12345", prefix)
			return nil, storage.ErrAPIKeyNotFound
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
	require.True(t, called, "lookupAPIKey should have been invoked once parse logic is wired")
}
```

Verify failure: `go test -run 'TestResolveAPIKey_WellFormedReachesLookup' -v` — expected FAIL because `lookupAPIKey` is never invoked (stub `resolveAPIKey` returns before any storage call).

- [ ] **Step 3: Implement parse**

Add helper and rewrite `resolveAPIKey`:

```go
const (
	apiKeyPrefix    = "spgr_sk_"
	apiKeyPrefixLen = 8  // characters
	apiKeySecretLen = 32 // characters
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

func (s *pgIdentityStore) resolveAPIKey(ctx context.Context, token string) (*Identity, error) {
	prefix, _, ok := parseAPIKey(token)
	if !ok {
		return nil, ErrUnauthenticated
	}
	// Subsequent tasks (12–15) add lookup, verify, owner load, etc.
	// For now: reach the lookup so the test can prove parse succeeded.
	_, err := s.users.LookupAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	// Will be replaced in Task 12.
	return nil, ErrUnauthenticated
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): parse spgr_sk_<prefix>_<secret> API-key tokens"
```

---

## Task 12: API-key — argon2id verify against `PHCHash`

After the prefix lookup succeeds, verify the presented secret against the stored argon2id PHC hash.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (matching secret verifies) + Boundary (wrong secret → ErrUnauthenticated; corrupt PHC string → ErrUnauthenticated).

- [ ] **Step 1: Failing test**

The shared fixtures `stubPHCSecret`, `stubPHCHash`, and `stubAPIKeyToken` are already defined in `usersbackend_stub_test.go` (Task 8). This task just consumes them — do NOT redefine. A wrong-secret token is any well-formed token whose secret ≠ `stubPHCSecret`; build one inline since `stubAPIKeyToken` always uses the correct secret.

```go
func TestResolveAPIKey_WrongSecretUnauthenticated(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix,
				PHCHash: stubPHCHash,
			}, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	// Well-formed token, correct prefix, but a DIFFERENT secret of the
	// SAME length — derive it from stubPHCSecret by flipping the first
	// char so the parse succeeds (length matches) but argon2id verify
	// fails. Avoids hand-counting a 32-char literal.
	wrongSecret := "X" + stubPHCSecret[1:]
	_, err := store.Resolve(context.Background(), "spgr_sk_abc12345_"+wrongSecret)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: FAIL — verify isn't wired yet; the call returns `ErrUnauthenticated` for the wrong reason.

- [ ] **Step 3: Implement verify**

Add the argon2id verify helper to `identitystore.go`. We roll our own PHC parse (no extra dependency) since the format is fixed and the parse is ~15 lines. Add these imports to `identitystore.go`'s import block:

```go
import (
	// ... existing imports ...
	"crypto/subtle"
	"encoding/base64"

	"golang.org/x/crypto/argon2"
)

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
	computed := argon2.IDKey([]byte(secret), salt, t, m, p, uint32(len(storedHash)))
	return subtle.ConstantTimeCompare(storedHash, computed) == 1
}

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
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	if !argon2idVerify(key.PHCHash, secret) {
		return nil, ErrUnauthenticated
	}
	if key.RevokedAt != nil || (key.ExpiresAt != nil && !key.ExpiresAt.After(s.now())) {
		return nil, ErrUnauthenticated
	}
	// Owner load + Identity construction happen in Tasks 13–15.
	// For now return a partial Identity that tests can assert against.
	return &Identity{
		UserID:  key.UserID,
		Subject: "apikey:" + key.ID,
		Source:  "apikey",
	}, nil
}
```

The argon2id parameters in `argon2idVerify` (m=19456, t=2, p=1) MUST match the parameters used to generate `stubPHCHash` in the Task 8 fixtures `init()`. They do — both use m=19456, t=2, p=1. The fixtures and the verifier are kept in sync by using the same literals; if production hashing parameters change later, update both sites (a single follow-up bead can centralize them into a shared `argon2Params` struct).

- [ ] **Step 4: Add a happy-path test**

```go
func TestResolveAPIKey_MatchingSecretSucceeds(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				CreatedAt: time.Now(),
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			// Will be exercised in Task 13; for now this is a forward dep.
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	id, err := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "apikey:k1", id.Subject)
}
```

> **Note:** `stubAPIKeyToken(prefix)` (Task 8 fixtures) builds the token from `stubPHCSecret`, so callers can't drift the secret. `TestStubSecretLength` (Task 8) guards that the secret is 32 chars. The verifier and the fixture's hash use identical argon2id parameters, so the token verifies.

- [ ] **Step 5: Run**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: PASS for the happy-path and wrong-secret tests.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): argon2id verify in API-key Resolve path"
```

---

## Task 13: API-key — owner load + soft-delete check

After verifying the key, load the user. Reject if soft-deleted.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (active user resolves) + Invariant (soft-deleted user can't auth via API key).

- [ ] **Step 1: Failing test**

```go
func TestResolveAPIKey_SoftDeletedOwnerUnauthenticated(t *testing.T) {
	deletedAt := time.Now().Add(-time.Hour)
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u-del", Prefix: prefix, PHCHash: stubPHCHash,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "reader", storage.KindHuman)
			u.DeletedAt = &deletedAt
			return u, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	_, err := store.Resolve(context.Background(),
		stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey_SoftDeletedOwner' -v`

Expected: FAIL — the resolveAPIKey impl from Task 12 doesn't yet load the owner or check `deleted_at`.

- [ ] **Step 3: Implement owner load**

In `resolveAPIKey`, replace the partial-Identity return with a full load:

```go
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
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
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
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		// Security-observable: a valid credential for a soft-deleted user.
		// Worth a warn — could indicate a credential that should have been
		// revoked, or an offboarding gap.
		slog.Warn("auth: credential resolved to soft-deleted user (api-key)",
			"user_id", user.ID, "key_id", key.ID)
		return nil, ErrUnauthenticated
	}
	return &Identity{
		UserID:        user.ID,
		Subject:       "apikey:" + key.ID,
		DisplayName:   user.DisplayName,
		Email:         user.Email,
		Role:          Role(user.Role),
		EffectiveRole: Role(user.Role), // Task 14 will clamp by RoleDowngrade
		Source:        "apikey",
	}, nil
}
```

This task introduces the first `slog` call in `identitystore.go`. Add `"log/slog"` to the file's import block (it stays used from here through Tasks 17, 19, 20). Go will not complain about an unused import because the soft-delete `slog.Warn` above references it.

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: PASS for all API-key tests so far.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): owner load + soft-delete check in API-key Resolve"
```

---

## Task 14: API-key — `EffectiveRole = min(user.Role, key.RoleDowngrade)`

Apply the per-key role downgrade clamp at resolve time so demoting a user instantly demotes all their keys.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (downgrade clamps when set; equals Role when unset) + Boundary (custom role + downgrade is a no-op per Storage design).

- [ ] **Step 1: Failing test**

```go
func TestResolveAPIKey_RoleDowngradeClamps(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				RoleDowngrade: "reader",
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "writer", storage.KindHuman), nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	id, err := store.Resolve(context.Background(),
		stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, auth.Role("reader"), id.EffectiveRole)
}

func TestResolveAPIKey_NoDowngradeEqualsRole(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				// RoleDowngrade: "" (zero value)
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "writer", storage.KindHuman), nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	id, err := store.Resolve(context.Background(),
		stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, id.Role, id.EffectiveRole)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey_Role' -v`

Expected: FAIL — Task 13's impl set `EffectiveRole = Role` unconditionally.

- [ ] **Step 3: Implement clamp**

Add helper and update `resolveAPIKey`:

```go
// roleLessThan reports whether a is strictly less privileged than b.
// Built-in roles are linearly ordered: reader < writer < admin. Custom
// roles return false in either direction (see Storage design's roleLessThan).
func roleLessThan(a, b Role) bool {
	rank := map[Role]int{RoleReader: 1, RoleWriter: 2, RoleAdmin: 3}
	ra, oka := rank[a]
	rb, okb := rank[b]
	if !oka || !okb {
		return false
	}
	return ra < rb
}

// clampedRole returns the lesser of user.Role and key.RoleDowngrade, but
// only for built-in roles. Custom roles have no defined ordering; the
// downgrade is a no-op (logged at the call site).
func clampedRole(userRole, downgrade Role) Role {
	if downgrade == "" {
		return userRole
	}
	if roleLessThan(downgrade, userRole) {
		return downgrade
	}
	return userRole
}
```

Update the Identity construction in `resolveAPIKey`:

```go
return &Identity{
	UserID:        user.ID,
	Subject:       "apikey:" + key.ID,
	DisplayName:   user.DisplayName,
	Email:         user.Email,
	Role:          Role(user.Role),
	EffectiveRole: clampedRole(Role(user.Role), Role(key.RoleDowngrade)),
	Source:        "apikey",
}, nil
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): clamp EffectiveRole = min(Role, RoleDowngrade) in API-key path"
```

---

## Task 15: API-key — fire-and-forget `TouchLastUsed`

After a successful resolve, enqueue a `last_used_at` update via the Tracker. The Tracker is async — it returns immediately and batches updates in the background (Task 23 implements the real Manager).

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (successful resolve enqueues a Touch) + Boundary (failed resolve does NOT enqueue).

- [ ] **Step 1: Failing test**

```go
// countingTracker records every Touch call.
type countingTracker struct {
	touched []string
}

func (c *countingTracker) Touch(keyID string) {
	c.touched = append(c.touched, keyID)
}

func TestResolveAPIKey_SuccessTouchesLastUsed(t *testing.T) {
	tracker := &countingTracker{}
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: tracker,
	})
	_, err := store.Resolve(context.Background(),
		stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, []string{"k1"}, tracker.touched)
}

func TestResolveAPIKey_FailureDoesNotTouch(t *testing.T) {
	tracker := &countingTracker{}
	stub := &usersBackendStub{
		// No lookupAPIKey set → returns ErrAPIKeyNotFound.
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: tracker,
	})
	_, err := store.Resolve(context.Background(),
		stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
	require.Empty(t, tracker.touched)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey_Success.*Touch|FailureDoesNotTouch' -v`

Expected: FAIL — Touch isn't called in the current impl.

- [ ] **Step 3: Add the Touch call**

In `resolveAPIKey`, right before the final return, add:

```go
s.tracker.Touch(key.ID)
return &Identity{ ... }, nil
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveAPIKey' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): fire-and-forget TouchLastUsed on successful API-key resolve"
```

---

## Task 16: JWT — issuer peek + verifier routing

For JWT-shaped tokens, extract the issuer claim (without verifying yet) and route to the matching `OIDCVerifier`. Unknown issuers return `ErrUnauthenticated`.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (routes to matching verifier) + Boundary (no matching verifier → ErrUnauthenticated; malformed JWT → ErrUnauthenticated).

- [ ] **Step 1: Failing test**

```go
import (
	"github.com/specgraph/specgraph/internal/config"
)

func TestResolveJWT_UnknownIssuerUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t) // no verifiers configured
	_, err := store.Resolve(context.Background(), "header.payload.signature")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveJWT_MalformedJWTUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "not.a.valid.jwt") // 3 dots not 2
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveJWT' -v`

Expected: PASS already (resolveJWT is a stub returning ErrUnauthenticated). Add a positive test to force real routing:

```go
func TestResolveJWT_KnownIssuerRoutes(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
			require.Equal(t, p.server.URL, issuer)
			require.Equal(t, "user-123", subject)
			return nil, storage.ErrOIDCBindingNotFound // forces JIT path (Task 18)
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:     stub,
		Verifiers: []*auth.OIDCVerifier{v},
		Tracker:   &noopTracker{},
		// JITEnabled: false (Task 18 enables and tests JIT)
	})
	token := p.mintToken(t, map[string]any{
		"iss":   p.server.URL,
		"sub":   "user-123",
		"aud":   "aud-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": "alice@example.com",
	})
	_, err = store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated) // JIT disabled → reject on binding miss
}
```

This test should FAIL because `resolveJWT` doesn't yet peek the issuer or call the verifier.

- [ ] **Step 3: Implement issuer peek and routing**

```go
import (
	"encoding/base64"
	"encoding/json"
)

// peekIssuerV2 extracts the iss claim from an unverified JWT payload. Used
// only to route to the correct OIDCVerifier; the verifier subsequently
// validates signature+audience+expiry.
//
// Named "V2" during Phase A because the legacy composite_store.go already
// defines an identical `peekIssuer` in this package; two same-name
// package functions would not compile. Task 30b renames this back to
// `peekIssuer` after composite_store.go is deleted (Task 30).
func peekIssuerV2(token string) (string, error) {
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

func (s *pgIdentityStore) resolveJWT(ctx context.Context, token string) (*Identity, error) {
	issuer, err := peekIssuerV2(token) // renamed to peekIssuer in Task 30b
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
		slog.Warn("auth: JWT issuer mismatch between peek and verified claim",
			"peek", issuer, "verified", claims.Issuer)
		return nil, ErrUnauthenticated
	}
	// Binding lookup + user load happen in Task 17.
	// JIT happens in Task 18.
	binding, err := s.users.LookupOIDCBinding(ctx, claims.Issuer, claims.Subject)
	if err != nil {
		if errors.Is(err, storage.ErrOIDCBindingNotFound) {
			// Task 18 will replace this with the JIT path.
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	_ = binding
	// Task 17 fills in the rest.
	return nil, ErrUnauthenticated
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveJWT' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): JWT issuer peek and OIDCVerifier routing"
```

---

## Task 17: JWT — existing binding resolves to owner

For known `(issuer, subject)` pairs, load the bound user and return an Identity. ClaimsMapping does NOT re-evaluate (it's JIT-only per the design).

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (existing binding resolves) + Invariant (DB-persisted role is authoritative; claims-mapping not re-evaluated).

- [ ] **Step 1: Failing test**

```go
func TestResolveJWT_ExistingBindingResolves(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})

	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{
				ID: "b1", UserID: "u1", Issuer: issuer, Subject: subject,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "writer", storage.KindHuman)
			u.Email = "alice@example.com"
			return u, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
	})
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "user-123", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "alice@example.com",
		// Claims that would map to "admin" if evaluated — but claims-mapping
		// is JIT-only, so the DB role (writer) must win.
		"groups": []string{"specgraph-admins"},
	})
	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "oidc:user-123", id.Subject)
	require.Equal(t, auth.Role("writer"), id.Role) // NOT admin from claims
	require.Equal(t, auth.Role("writer"), id.EffectiveRole)
	require.Equal(t, "oidc", id.Source)
}

func TestResolveJWT_SoftDeletedUserUnauthenticated(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	deletedAt := time.Now()
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{ID: "b1", UserID: "u-del"}, nil
		},
		getUserByID: func(_ context.Context, _ string) (*storage.User, error) {
			u := activeUser("u-del", "writer", storage.KindHuman)
			u.DeletedAt = &deletedAt
			return u, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
	})
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err := store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveJWT_(Existing|SoftDeleted)' -v`

Expected: FAIL — current `resolveJWT` returns `ErrUnauthenticated` after finding the binding (Task 16's stub).

- [ ] **Step 3: Implement owner load**

Replace the placeholder at the end of `resolveJWT`:

```go
	user, err := s.users.GetUserByID(ctx, binding.UserID)
	if err != nil {
		if errors.Is(err, storage.ErrUserNotFound) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	if user.DeletedAt != nil {
		// Security-observable: a valid OIDC binding for a soft-deleted user.
		// The binding wasn't unbound at offboarding; the deleted_at gate
		// catches it, but log so operators can notice the gap.
		slog.Warn("auth: token resolved to soft-deleted user (oidc)",
			"user_id", user.ID, "subject", claims.Subject)
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
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveJWT' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): existing OIDC binding resolves to bound user (JIT-mapping-skipped)"
```

---

## Task 18: JIT — happy path (no rate-limit / allowlist yet)

When the binding lookup misses AND `JITEnabled`, create a new Human + binding atomically. This task handles the bare happy case; rate-limit (Task 19) and allowlist (Task 20) layer on next.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (new sub creates user + binding) + Boundary (JIT disabled → reject on miss).

- [ ] **Step 1: Failing test**

```go
func TestResolveJWT_JITCreatesNewUser(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	var capturedUser *storage.User
	var capturedBinding *storage.OIDCBinding
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "new-user"
			b.ID = "new-binding"
			b.UserID = u.ID
			capturedUser, capturedBinding = u, b
			return u, b, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
	})
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "new-sub", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "new@example.com",
	})
	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "new-user", id.UserID)
	require.Equal(t, auth.Role("reader"), id.Role)
	require.NotNil(t, capturedUser)
	require.Equal(t, "new@example.com", capturedUser.Email)
	require.NotNil(t, capturedBinding)
	require.Equal(t, "new-sub", capturedBinding.Subject)
}

func TestResolveJWT_JITDisabledRejects(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled: false,
	})
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err := store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestResolveJWT_JIT' -v`

Expected: FAIL — Task 16's impl returns ErrUnauthenticated on binding miss; doesn't call JITCreateHuman.

- [ ] **Step 3: Implement JIT happy path**

Replace the `ErrOIDCBindingNotFound` branch in `resolveJWT`:

```go
	binding, err := s.users.LookupOIDCBinding(ctx, claims.Issuer, claims.Subject)
	if err != nil {
		if !errors.Is(err, storage.ErrOIDCBindingNotFound) {
			return nil, fmt.Errorf("%w: %v", ErrTransient, err)
		}
		// Binding miss → JIT path.
		if !s.jitEnabled {
			return nil, ErrUnauthenticated
		}
		return s.jitResolve(ctx, claims)
	}
	// existing binding path continues below ...
```

Add the JIT method:

```go
// jitResolve creates a new Human + OIDC binding for an unknown subject.
// Rate-limit and email-allowlist checks land in Tasks 19–20.
func (s *pgIdentityStore) jitResolve(ctx context.Context, claims *OIDCClaims) (*Identity, error) {
	role := s.jitDefaultRole
	if role == "" {
		role = RoleReader
	}
	u := &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: claims.Subject, // operator can rename later via auth user
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
		return nil, fmt.Errorf("%w: %v", ErrTransient, err)
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
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestResolveJWT_JIT' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): JIT-create Human+binding for unknown OIDC subjects (happy path)"
```

---

## Task 19: JIT — per-issuer rate limit

Bound JIT creation per issuer so a misconfigured trust relationship can't amplify into unbounded user creation.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Create: `internal/auth/identitystore_jit_test.go`

**Covers:** Happy (allowed within burst) + Invariant (per-issuer cap; exceeded → ErrUnauthenticated).

- [ ] **Step 1: Failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestJIT_RateLimitExhaustion(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:          true,
		JITDefaultRole:      auth.RoleReader,
		JITRateBurstPerHour: 5, // small burst so the test runs fast
	})

	// First 5 JITs succeed.
	for i := 0; i < 5; i++ {
		tok := p.mintToken(t, map[string]any{
			"iss": p.server.URL, "sub": "u" + strconv.Itoa(i),
			"aud": "aud-1", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		})
		_, err := store.Resolve(context.Background(), tok)
		require.NoError(t, err, "JIT %d should succeed", i)
	}
	// 6th JIT exhausts the bucket.
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u-exhaust",
		"aud": "aud-1", "exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	_, err := store.Resolve(context.Background(), tok)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestJIT_RateLimit' -v`

Expected: FAIL — `jitResolve` doesn't yet check the rate limiter.

- [ ] **Step 3: Wire the rate limit**

In `jitResolve`, add the rate-limit gate. **Ordering note:** Task 20 will insert the email-allowlist check ABOVE this rate-limit gate, so that allowlist-refused tokens do not consume rate-limit budget. For now (allowlist not yet implemented), the rate limiter is the first gate:

```go
func (s *pgIdentityStore) jitResolve(ctx context.Context, claims *OIDCClaims) (*Identity, error) {
	// NOTE (finalized in Task 20): the email-allowlist check is inserted
	// above this line so tokens that can't create a user never spend a
	// rate-limit token. The rate limiter therefore bounds *eligible*
	// creation attempts, which is its design intent.
	limiter := s.rateLimiterFor(claims.Issuer)
	if !limiter.Allow() {
		slog.Warn("auth: JIT rate-limit exceeded",
			"issuer", claims.Issuer, "subject", claims.Subject)
		return nil, ErrUnauthenticated
	}
	// ... existing JIT body ...
}
```

Add `"log/slog"` to imports if not already there.

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestJIT_RateLimit' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_jit_test.go
git commit -s -m "feat(auth): per-issuer JIT rate limit via golang.org/x/time/rate"
```

---

## Task 20: JIT — email-domain allowlist

When the operator configures an email-domain allowlist, JIT refuses tokens whose email claim's domain doesn't match. Missing email + non-empty allowlist also refuses.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_jit_test.go`

**Covers:** Happy (matching domain succeeds) + Boundary (mismatched domain refuses; missing claim with allowlist refuses; missing claim with empty allowlist allows).

- [ ] **Step 1: Failing tests**

```go
func TestJIT_EmailAllowlist_Match(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:              true,
		JITDefaultRole:          auth.RoleReader,
		JITEmailDomainAllowlist: []string{"example.com"},
	})
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "alice@example.com",
	})
	_, err := store.Resolve(context.Background(), tok)
	require.NoError(t, err)
}

func TestJIT_EmailAllowlist_Mismatch(t *testing.T) {
	// Same setup as above but token email is "bob@other.com".
	// Expected: ErrUnauthenticated.
	// (Full setup omitted for brevity in this comment — copy from _Match
	// and change the email claim. Reuse the stub.)
}

func TestJIT_EmailAllowlist_MissingClaimRefuses(t *testing.T) {
	// Allowlist non-empty + token has no "email" claim → ErrUnauthenticated.
}

func TestJIT_EmptyAllowlistAllowsMissingClaim(t *testing.T) {
	// Allowlist empty + token has no "email" claim → JIT succeeds; user.Email = "".
}
```

> **Note:** the additional three test bodies follow the same pattern as `_Match`. Expanded verbatim in the implementation; the plan shows the first one in full to fix the structure, then the executor copies it for the other three.

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestJIT_EmailAllowlist' -v`

Expected: FAIL — `jitResolve` doesn't check the allowlist.

- [ ] **Step 3: Implement allowlist (placed BEFORE the rate-limit gate)**

In `jitResolve`, insert the allowlist check ABOVE the `limiter := s.rateLimiterFor(...)` line from Task 19. Ordering matters: a token whose email domain isn't allowed can never create a user, so it must not consume a rate-limit token. Putting the allowlist first means the rate limiter bounds only *eligible* creation attempts — its design intent.

```go
func (s *pgIdentityStore) jitResolve(ctx context.Context, claims *OIDCClaims) (*Identity, error) {
	// (1) Allowlist gate — cheap, no budget spent. Refused tokens never
	// reach the rate limiter.
	if len(s.jitEmailAllowlist) > 0 {
		domain := emailDomain(claims.Email)
		if domain == "" {
			slog.Warn("auth: JIT refused — empty email claim with non-empty allowlist",
				"issuer", claims.Issuer)
			return nil, ErrUnauthenticated
		}
		if !s.jitEmailAllowlist[domain] {
			slog.Warn("auth: JIT refused — email domain not in allowlist",
				"issuer", claims.Issuer, "domain", domain)
			return nil, ErrUnauthenticated
		}
	}

	// (2) Rate-limit gate — bounds eligible creation attempts per issuer.
	limiter := s.rateLimiterFor(claims.Issuer)
	if !limiter.Allow() {
		slog.Warn("auth: JIT rate-limit exceeded",
			"issuer", claims.Issuer, "subject", claims.Subject)
		return nil, ErrUnauthenticated
	}

	// (3) ClaimsMapping + (4) JITCreateHuman follow (Tasks 21, 18).
	// ... existing JIT body ...
}
```

This replaces the rate-limit-first ordering from Task 19's stub. The executor moves the limiter block down and adds the allowlist block above it.

Add the `emailDomain` helper (once):

```go
func emailDomain(email string) string {
	i := strings.LastIndexByte(email, '@')
	if i < 0 || i == len(email)-1 {
		return ""
	}
	return strings.ToLower(email[i+1:])
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestJIT_EmailAllowlist' -v`

Expected: PASS for all four sub-tests.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_jit_test.go
git commit -s -m "feat(auth): JIT email-domain allowlist with missing-claim refusal"
```

---

## Task 21: JIT — ClaimsMapping evaluation

Map verified claims to a role at JIT-creation time (only). Subsequent sign-ins use the DB-persisted role.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_jit_test.go`

**Covers:** Happy (matching claim → mapped role) + Invariant (claims-mapping fires ONLY at JIT, never on re-sign-in; verified in Task 17's test).

- [ ] **Step 1: Failing test**

```go
func TestJIT_ClaimsMapping_AppliesRole(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, _ := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	var capturedRole string
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			capturedRole = u.Role
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		JITClaimsMapping: map[string][]config.ClaimMapping{
			p.server.URL: {
				{Claim: "groups", Value: "specgraph-admins", Role: "admin"},
			},
		},
	})
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"email":  "a@example.com",
		"groups": []string{"specgraph-admins"},
	})
	_, err := store.Resolve(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "admin", capturedRole, "claims-mapping should override default-role")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestJIT_ClaimsMapping' -v`

Expected: FAIL — `jitResolve` uses `jitDefaultRole` unconditionally.

- [ ] **Step 3: Implement claims-mapping**

In `jitResolve`, replace the role assignment:

```go
	role := s.jitDefaultRole
	if role == "" {
		role = RoleReader
	}
	if mappings, ok := s.jitClaimsMapping[claims.Issuer]; ok {
		if mapped := applyClaimsMapping(claims.Raw, mappings); mapped != "" {
			role = Role(mapped)
		}
	}
```

Add helper:

```go
// applyClaimsMapping evaluates the mapping rules in order. Returns the
// first matching role, or "" if no rule matches.
func applyClaimsMapping(claims map[string]json.RawMessage, rules []config.ClaimMapping) string {
	for _, m := range rules {
		raw, ok := claims[m.Claim]
		if !ok {
			continue
		}
		if matchClaimValueV2(raw, m.Value) {
			return m.Role
		}
	}
	return ""
}

// matchClaimValueV2 checks whether a claim value matches the target.
// Supports string claims and string-array claims.
//
// Named "V2" during Phase A because the legacy oidc_store.go already
// defines an identical `matchClaimValue` in this package. Task 30b
// renames this back to `matchClaimValue` after oidc_store.go is
// deleted (Task 30).
func matchClaimValueV2(raw json.RawMessage, target string) bool {
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
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestJIT_ClaimsMapping' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_jit_test.go
git commit -s -m "feat(auth): apply ClaimsMapping at JIT creation only"
```

---

## Task 22: `HasAuth` impl

Reports whether any non-bootstrap, non-deleted user exists. Used by `warnIfNoAuthOnPublicListen` at startup.

**Files:**

- Modify: `internal/auth/identitystore.go`
- Modify: `internal/auth/identitystore_test.go`

**Covers:** Happy (one real user → true) + Boundary (only-bootstrap → false).

- [ ] **Step 1: Failing test**

```go
func TestHasAuth_OnlyBootstrapReturnsFalse(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
			require.Equal(t, storage.KindHuman, f.Kind)
			require.False(t, f.IncludeDeleted)
			// Return ONLY the bootstrap user.
			u := activeUser("u-boot", "admin", storage.KindHuman)
			u.Bootstrap = true
			return []*storage.User{u}, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	has, err := store.HasAuth(context.Background())
	require.NoError(t, err)
	require.False(t, has)
}

func TestHasAuth_NonBootstrapUserReturnsTrue(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, _ storage.ListUsersFilter) ([]*storage.User, error) {
			return []*storage.User{
				activeUser("u1", "reader", storage.KindHuman),
			}, nil
		},
	}
	store, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	has, err := store.HasAuth(context.Background())
	require.NoError(t, err)
	require.True(t, has)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestHasAuth' -v`

Expected: FAIL — `HasAuth` returns "not implemented".

- [ ] **Step 3: Implement**

```go
func (s *pgIdentityStore) HasAuth(ctx context.Context) (bool, error) {
	users, err := s.users.ListUsers(ctx, storage.ListUsersFilter{
		Kind: storage.KindHuman,
		// Note: ListUsers does not filter by Bootstrap directly; we filter
		// post-fetch since the bootstrap rows are rare and small in count.
	})
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrTransient, err)
	}
	for _, u := range users {
		if !u.Bootstrap {
			return true, nil
		}
	}
	return false, nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth && go test -run 'TestHasAuth' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/identitystore.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): HasAuth via ListUsers filter for non-bootstrap presence"
```

---

## Task 23: `usagetracker` package

Async batched `TouchLastUsed` updater. Channel + drain goroutine + `Close(ctx)` with drain semantics.

**Files:**

- Create: `internal/auth/usagetracker/usagetracker.go`
- Create: `internal/auth/usagetracker/usagetracker_test.go`

**Covers:** Happy (Touch enqueues; drain calls TouchLastUsed) + Invariant (Close drains in-flight items).

- [ ] **Step 1: Failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package usagetracker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth/usagetracker"
)

type fakeStorage struct {
	mu     sync.Mutex
	touched []string
}

func (f *fakeStorage) TouchLastUsed(_ context.Context, keyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched = append(f.touched, keyID)
	return nil
}

func (f *fakeStorage) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.touched))
	copy(out, f.touched)
	return out
}

func TestManager_TouchesAreFlushedOnDrain(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    16,
		FlushInterval: 50 * time.Millisecond,
	})
	mgr.Touch("k1")
	mgr.Touch("k2")
	mgr.Touch("k3")

	require.NoError(t, mgr.Close(context.Background()))
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, store.snapshot())
}

func TestManager_OverflowDropsButDoesNotBlock(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    2,
		FlushInterval: time.Hour, // drain only on Close
	})
	// Enqueue 100 without giving the drain a chance.
	for i := 0; i < 100; i++ {
		mgr.Touch("k")
	}
	require.NoError(t, mgr.Close(context.Background()))
	require.LessOrEqual(t, len(store.snapshot()), 100,
		"Touch should drop on overflow, not block")
	// With BufferSize=2 and no drain until Close, most of the 100 are
	// dropped. The dropped counter must be non-zero and account for the
	// difference between enqueued and persisted.
	require.Positive(t, mgr.Dropped(), "overflow drops should be counted")
	require.Equal(t, uint64(100)-uint64(len(store.snapshot())), mgr.Dropped(),
		"dropped + persisted should equal total enqueued")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth/usagetracker && go test -v`

Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Implement**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package usagetracker provides asynchronous batched last_used_at updates
// for API keys. The auth Resolver enqueues touches via Manager.Touch; a
// background goroutine drains them into TouchLastUsedBackend (typically
// storage.UsersBackend) at a configurable interval.
package usagetracker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// TouchLastUsedBackend is the storage surface the Manager uses to persist
// touches. Satisfied by storage.UsersBackend.
type TouchLastUsedBackend interface {
	TouchLastUsed(ctx context.Context, keyID string) error
}

// Config parametrizes Manager.
type Config struct {
	BufferSize    int           // channel capacity; default 256
	FlushInterval time.Duration // drain interval; default 5s
}

// Manager batches Touch calls and persists them async.
type Manager struct {
	backend TouchLastUsedBackend
	ch      chan string
	done    chan struct{}
	wg      sync.WaitGroup
	dropped atomic.Uint64 // count of touches dropped on overflow (ops visibility)
}

// NewManager constructs a Manager and starts its drain goroutine.
func NewManager(backend TouchLastUsedBackend, cfg Config) *Manager {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	m := &Manager{
		backend: backend,
		ch:      make(chan string, cfg.BufferSize),
		done:    make(chan struct{}),
	}
	m.wg.Add(1)
	go m.drain(cfg.FlushInterval)
	return m
}

// Dropped returns the cumulative count of touches dropped due to channel
// overflow. Exposed for ops visibility (metrics export, doctor checks, a
// startup-time warning if non-zero). A chronically non-zero value means
// BufferSize or FlushInterval needs tuning for the request rate.
func (m *Manager) Dropped() uint64 {
	return m.dropped.Load()
}

// Touch enqueues a key ID for async last_used_at update. Non-blocking:
// drops on channel overflow, incrementing the dropped counter and logging
// a warning.
func (m *Manager) Touch(keyID string) {
	select {
	case m.ch <- keyID:
	default:
		m.dropped.Add(1)
		slog.Warn("usagetracker: touch dropped on overflow",
			"keyID", keyID, "total_dropped", m.dropped.Load())
	}
}

func (m *Manager) drain(interval time.Duration) {
	defer m.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			m.flushAll()
			return
		case <-ticker.C:
			m.flushAll()
		}
	}
}

func (m *Manager) flushAll() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for {
		select {
		case id := <-m.ch:
			if err := m.backend.TouchLastUsed(ctx, id); err != nil {
				slog.Warn("usagetracker: TouchLastUsed failed", "error", err)
			}
		default:
			return
		}
	}
}

// Close stops the drain goroutine, flushing any in-flight items.
// Respects ctx cancellation: returns ctx.Err() if cancelled mid-drain.
func (m *Manager) Close(ctx context.Context) error {
	close(m.done)
	doneCh := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

- [ ] **Step 4: Run**

Run: `cd internal/auth/usagetracker && go test -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/usagetracker/
git commit -s -m "feat(auth): usagetracker package for async batched TouchLastUsed"
```

---

## Task 24: `Manager.Close` drain integration test

Beyond the unit tests, verify Close drains correctly under load.

**Files:**

- Modify: `internal/auth/usagetracker/usagetracker_test.go`

**Covers:** Invariant (Close drains in-flight items synchronously).

- [ ] **Step 1: Test**

```go
func TestManager_CloseDrainsUnderLoad(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    1024,
		FlushInterval: time.Hour, // only Close drains
	})
	for i := 0; i < 500; i++ {
		mgr.Touch("k")
	}
	require.NoError(t, mgr.Close(context.Background()))
	require.Equal(t, 500, len(store.snapshot()))
}

func TestManager_CloseRespectsCtxCancellation(t *testing.T) {
	// Backend with intentional slowness.
	slowStore := &slowFakeStorage{delay: 10 * time.Millisecond}
	mgr := usagetracker.NewManager(slowStore, usagetracker.Config{
		BufferSize:    1024,
		FlushInterval: time.Hour,
	})
	for i := 0; i < 100; i++ {
		mgr.Touch("k")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := mgr.Close(ctx)
	// Either the close completes (fast enough) OR the ctx wins.
	// Both are acceptable; the assertion is "Close does not hang".
	require.True(t, err == nil || err == context.DeadlineExceeded)
}

type slowFakeStorage struct {
	delay time.Duration
}

func (s *slowFakeStorage) TouchLastUsed(_ context.Context, _ string) error {
	time.Sleep(s.delay)
	return nil
}
```

- [ ] **Step 2: Run**

Run: `cd internal/auth/usagetracker && go test -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/usagetracker/usagetracker_test.go
git commit -s -m "test(auth): Manager.Close drains under load + respects ctx"
```

---

## Task 25: Wire `usagetracker.Manager` into `pgIdentityStore`

Replace the stub `LastUsedTracker` field with the real `*usagetracker.Manager` (which satisfies the same interface). No behavior changes — the interface is identical.

**Files:**

- Modify: `internal/auth/identitystore.go` (just an import + comment update; Manager already satisfies LastUsedTracker)

**Covers:** N/A (refactor only; tests stay green via the interface).

- [ ] **Step 1: Verify compatibility**

`usagetracker.Manager` exposes `Touch(keyID string)`. `auth.LastUsedTracker` requires `Touch(keyID string)`. Compatible.

- [ ] **Step 2: Add a docstring note**

In `identitystore.go`, update the `LastUsedTracker` comment to point at the canonical implementation:

```go
// LastUsedTracker is the asynchronous TouchLastUsed surface consumed by
// pgIdentityStore after a successful API-key resolve.
//
// The canonical implementation is *usagetracker.Manager. Tests may use
// other implementations (stubs, no-ops) that satisfy the interface.
type LastUsedTracker interface {
	Touch(keyID string)
}
```

No behavior change. The wiring happens in `serve.go` (Task 29) where `usagetracker.NewManager` is constructed and passed via `IdentityStoreConfig.Tracker`.

- [ ] **Step 3: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/identitystore.go
git commit -s -m "docs(auth): point LastUsedTracker docstring at usagetracker.Manager"
```

---

## Task 26: Add `NewAuthInterceptorV2` alongside the legacy `NewAuthInterceptor`

The Phase B cutover starts here. To keep both `internal/auth/` AND `cmd/specgraph/` building green at every task boundary, this task adds the NEW interceptor under a temporary name (`NewAuthInterceptorV2`) while the legacy `NewAuthInterceptor(IdentityStore)` continues to exist. `serve.go` keeps calling the legacy function until Task 29 switches it. Task 30b (after Task 30 deletes the legacy stores) renames `NewAuthInterceptorV2` back to `NewAuthInterceptor` once the legacy version is unreferenced.

**Files:**

- Modify: `internal/auth/interceptor.go`
- Modify: `internal/auth/interceptor_test.go`

**Covers:** Happy (resolve → authorize → handler) + Boundary (ErrUnauthenticated → 401; ErrTransient → 503; context.Canceled → propagates).

- [ ] **Step 1: Write the failing test**

Existing `interceptor_test.go` tests against the old `IdentityStore` interface. They will need updating in this same task. Start with new tests for the error categorization:

```go
import (
	"errors"

	"connectrpc.com/connect"
)

// fakeResolver is a tiny stub Resolver for interceptor tests.
type fakeResolver struct {
	resolve func(ctx context.Context, token string) (*auth.Identity, error)
}

func (f *fakeResolver) Resolve(ctx context.Context, token string) (*auth.Identity, error) {
	return f.resolve(ctx, token)
}
func (f *fakeResolver) HasAuth(_ context.Context) (bool, error) { return true, nil }

// fakeAuthorizer is a stub Authorizer.
type fakeAuthorizer struct {
	authorize func(ctx context.Context, id *auth.Identity, proc string, req any) (auth.Decision, error)
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, id *auth.Identity, proc string, req any) (auth.Decision, error) {
	return f.authorize(ctx, id, proc, req)
}

func TestInterceptor_TransientErrorIs503(t *testing.T) {
	r := &fakeResolver{
		resolve: func(_ context.Context, _ string) (*auth.Identity, error) {
			return nil, fmt.Errorf("%w: pool exhausted", auth.ErrTransient)
		},
	}
	a := &fakeAuthorizer{}
	// Build a request through a test handler that uses the interceptor.
	// Assert response code is connect.CodeUnavailable.
	// (Test harness construction is verbose; full template follows the
	// existing interceptor_test.go conventions.)
	_ = r; _ = a
	t.Skip("expand using existing interceptor_test.go harness conventions")
}
```

> **Note:** the full interceptor test harness is involved (httptest server + connect handler + client) and exists in the current `interceptor_test.go`. Adapt those tests to the new interceptor signature; this task's pseudo-test above marks the expected new assertions.

- [ ] **Step 2: Add the V2 interceptor alongside the legacy one**

Add the following functions to `internal/auth/interceptor.go`. **Do NOT** re-paste the `package auth` clause or an import block — the file already has both (`context, errors, log/slog, net/http, strings, connectrpc.com/connect` are all already imported by the existing `NewAuthInterceptor`). Add only the new functions: `NewAuthInterceptorV2`, `authenticateV2`, `sessionCookieValue`, `extractBearerToken`, `mapAuthError`. Leave the existing `NewAuthInterceptor` function in place.

> **Name note:** the helper is `authenticateV2`, NOT `authenticate` — the legacy `middleware.go` already defines `func authenticate(ctx, store IdentityStore, r *http.Request) (*Identity, bool)` in this package, and Go has no overloading. The V2 suffix avoids the redeclaration error during Phase A; Task 30b renames it back to `authenticate` after the legacy middleware/interceptor are deleted.

```go
// NewAuthInterceptorV2 returns a ConnectRPC unary interceptor that
// authenticates and authorizes requests using the supplied Resolver and
// Authorizer. Exempt procedures (Health) bypass both.
//
// Named "V2" temporarily during the Phase B cutover. After serve.go
// switches to this constructor (Task 29) and the legacy NewAuthInterceptor
// is removed, this function will be renamed back to NewAuthInterceptor
// in the cleanup task.
func NewAuthInterceptorV2(resolver Resolver, authorizer Authorizer) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure
			if IsExempt(procedure) {
				return next(ctx, req)
			}

			id, err := authenticateV2(ctx, resolver, req.Header())
			if err != nil {
				return nil, mapAuthError(procedure, err)
			}

			decision, err := authorizer.Authorize(ctx, id, procedure, req.Any())
			if err != nil {
				slog.Error("auth: authorizer error",
					"procedure", procedure, "error", err.Error())
				return nil, connect.NewError(connect.CodeInternal, nil)
			}
			if !decision.Allowed {
				slog.Warn("auth: permission denied",
					"subject", id.Subject, "procedure", procedure, "reason", decision.Reason)
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			slog.Info("auth: authenticated",
				"subject", id.Subject, "procedure", procedure)
			return next(WithIdentity(ctx, id), req)
		}
	}
}

// authenticateV2 extracts the bearer token (Authorization header or cookie
// fallback) and resolves it. Returns ErrUnauthenticated on missing token.
// V2-suffixed during Phase A (legacy middleware.go has a different-signature
// `authenticate`); renamed to `authenticate` in Task 30b.
func authenticateV2(ctx context.Context, resolver Resolver, headers http.Header) (*Identity, error) {
	token := extractBearerToken(headers)
	if token == "" {
		token = sessionCookieValue(headers) // dashboard fallback
	}
	if token == "" {
		return nil, ErrUnauthenticated
	}
	return resolver.Resolve(ctx, token)
}

// sessionCookieValue reads the specgraph_session cookie from raw headers.
// The standard library only exposes cookie parsing via *http.Request, so
// we wrap the headers in a throwaway request to reuse net/http's RFC-6265
// parser rather than hand-rolling cookie splitting. Isolated here so the
// idiom is documented in exactly one place.
func sessionCookieValue(headers http.Header) string {
	r := &http.Request{Header: headers}
	c, err := r.Cookie("specgraph_session")
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

func extractBearerToken(headers http.Header) string {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// mapAuthError maps Resolver / authentication errors to connect codes.
func mapAuthError(procedure string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	case errors.Is(err, ErrTransient):
		slog.Warn("auth: transient backend error", "procedure", procedure, "error", err.Error())
		return connect.NewError(connect.CodeUnavailable, nil)
	case errors.Is(err, ErrUnauthenticated):
		slog.Info("auth: unauthenticated", "procedure", procedure)
		return connect.NewError(connect.CodeUnauthenticated, nil)
	default:
		slog.Error("auth: unexpected error category", "procedure", procedure, "error", err.Error())
		return connect.NewError(connect.CodeInternal, nil)
	}
}
```

- [ ] **Step 3: Add tests for V2 alongside the existing tests**

The existing `interceptor_test.go` exercises the legacy `NewAuthInterceptor(IdentityStore)` — leave those tests in place; they continue to pass against the legacy function. Add NEW tests for `NewAuthInterceptorV2(Resolver, Authorizer)` using the `fakeResolver` and `fakeAuthorizer` stubs from Step 1. After Task 29 the legacy tests will be removed alongside the legacy function in the cleanup task.

- [ ] **Step 4: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/interceptor.go internal/auth/interceptor_test.go
git commit -s -m "feat(auth): interceptor calls Resolver+Authorizer with error categorization"
```

---

## Task 27: Add `RequireAuthV2` alongside legacy `RequireAuth`

Same V2 transition pattern as Task 26: append the new function under a temporary name; leave the legacy one in place until the cleanup task.

**Files:**

- Modify: `internal/auth/middleware.go`
- Modify: `internal/auth/middleware_test.go`

**Covers:** Happy (cookie auth → identity in context) + Boundary (missing cookie → 401).

- [ ] **Step 1: Append the V2 helper to middleware.go**

```go
// RequireAuthV2 returns HTTP middleware that authenticates requests via
// Bearer header or session cookie using a Resolver. Renamed back to
// RequireAuth in the Phase C cleanup task once the legacy version is
// removed.
func RequireAuthV2(resolver Resolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := authenticateV2(r.Context(), resolver, r.Header) // shared with NewAuthInterceptorV2; renamed in Task 30b
			if err != nil {
				switch {
				case errors.Is(err, ErrTransient):
					http.Error(w, `{"error":"transient"}`, http.StatusServiceUnavailable)
				default:
					http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				}
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}
```

- [ ] **Step 2: Add V2 tests alongside legacy tests**

The existing `middleware_test.go` tests stay; add new tests covering `RequireAuthV2` with a `fakeResolver`. The cleanup task removes the legacy tests with the legacy function.

- [ ] **Step 3: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -s -m "feat(auth): HTTP middleware uses Resolver (cookie path preserved)"
```

---

## Task 28: Remove implicit OS-user identity paths

The legacy `os/user.Current()` fallback (when no API keys were configured) is gone from both interceptor and middleware after Tasks 26–27 — but check for any lingering references and remove them.

**Files:**

- Modify: `internal/auth/auth.go` (drop `os` import if unreferenced)
- Modify: any other files still referencing OS-user identity construction.

- [ ] **Step 1: Grep for residual references**

Run: `cd internal/auth && grep -rn 'os/user\|os.Getenv("USER")\|local:' .`

Inspect the output. The only legitimate matches should be in the legacy stores being deleted in Task 30. Any references in interceptor.go, middleware.go, or other surviving files must be removed.

- [ ] **Step 2: Remove residual references**

If any are found in surviving files, remove them. Update any test that asserted on `"local:"` Subject prefixes.

- [ ] **Step 3: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/
git commit -s -m "refactor(auth): remove residual OS-user identity references"
```

---

## Task 28b: Introduce nested OIDC + JITCreate config

`pgIdentityStore` consumes JIT settings (rate limit, default role, email allowlist) that don't exist in `cfg.AuthConfig` today. Land the config schema change here so Task 29's serve.go references compile.

**Files:**

- Modify: `internal/config/global.go`
- Modify: `internal/config/global_test.go`

**Covers:** Happy (nested YAML parses) + Boundary (legacy flat `auth.oidc_providers:` returns load-time error pointing at the new path).

- [ ] **Step 1: Add the new struct types**

In `internal/config/global.go`, alongside the existing `AuthConfig`:

```go
// OIDCConfig wraps the OIDC provider list and JIT settings under a
// nested auth.oidc key. Replaces the flat AuthConfig.OIDCProviders.
type OIDCConfig struct {
	Providers []OIDCProviderConfig `yaml:"providers"`
	JITCreate JITCreateConfig      `yaml:"jit_create"`
}

// JITCreateConfig parametrizes just-in-time Human creation on first
// OIDC sign-in. Consumed by the identity resolver (Authn plan).
type JITCreateConfig struct {
	Enabled              bool     `yaml:"enabled"`
	DefaultRole          string   `yaml:"default_role"`
	RateLimitPerHour     int      `yaml:"rate_limit_per_hour"`
	EmailDomainAllowlist []string `yaml:"email_domain_allowlist"`
}
```

Add `OIDC OIDCConfig` to `AuthConfig`:

```go
type AuthConfig struct {
	Mode          string                `yaml:"mode"`           // deprecated; ignored after Authn plan
	DefaultRole   string                `yaml:"default_role"`   // deprecated; ignored after Authn plan
	APIKeys       []APIKeyConfig        `yaml:"api_keys"`       // ignored after Authn plan (storage owns)
	OIDCProviders []OIDCProviderConfig  `yaml:"oidc_providers"` // deprecated; superseded by OIDC.Providers
	Roles         map[string]RoleConfig `yaml:"roles"`
	OIDC          OIDCConfig            `yaml:"oidc"`
}
```

Keep `OIDCProviders` for one upgrade cycle so existing YAML files don't break catastrophically — but the loader (`LoadGlobal`) warns when the legacy path is populated.

- [ ] **Step 2: Add migration warning in the loader**

Insert the migration in `loadGlobalAt` (global.go:151), the single function that `LoadGlobal` AND `LoadGlobalExplicit` both delegate to — NOT in `LoadGlobal` directly, or the `--config` path (which uses `LoadGlobalExplicit`) would skip migration. Place it AFTER the YAML unmarshal and before the function returns:

```go
// Migrate the deprecated flat auth.oidc_providers into auth.oidc.providers.
// New path wins if both are set (no migration in that case).
if len(cfg.Auth.OIDCProviders) > 0 && len(cfg.Auth.OIDC.Providers) == 0 {
	cfg.Auth.OIDC.Providers = cfg.Auth.OIDCProviders
	slog.Warn("auth.oidc_providers is deprecated; move providers under auth.oidc.providers")
}
```

`internal/config/global.go` does not currently import `log/slog` — add it to the import block.

- [ ] **Step 3: Tests**

Add to `global_test.go`:

```go
func TestLoadGlobal_OIDCJITConfig(t *testing.T) {
	// Write a tmp YAML with the new nested shape.
	yamlBody := []byte(`
auth:
  oidc:
    providers:
      - id: entra
        issuer: https://login.microsoftonline.com/tenant/v2.0
        client_id: app-id
    jit_create:
      enabled: true
      default_role: reader
      rate_limit_per_hour: 200
      email_domain_allowlist: [example.com, other.com]
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, yamlBody, 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.True(t, cfg.Auth.OIDC.JITCreate.Enabled)
	require.Equal(t, "reader", cfg.Auth.OIDC.JITCreate.DefaultRole)
	require.Equal(t, 200, cfg.Auth.OIDC.JITCreate.RateLimitPerHour)
	require.Len(t, cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist, 2)
	require.Len(t, cfg.Auth.OIDC.Providers, 1)
}

func TestLoadGlobal_LegacyOIDCProvidersStillWorks(t *testing.T) {
	yamlBody := []byte(`
auth:
  oidc_providers:
    - id: legacy-entra
      issuer: https://login.microsoftonline.com/old/v2.0
      client_id: app-id
`)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, yamlBody, 0o600))

	cfg, err := config.LoadGlobal(path)
	require.NoError(t, err)
	require.Len(t, cfg.Auth.OIDC.Providers, 1, "legacy path should migrate transparently")
	require.Equal(t, "legacy-entra", cfg.Auth.OIDC.Providers[0].ID)
}
```

- [ ] **Step 4: Verify compile + tests**

Run: `cd internal/config && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/global.go internal/config/global_test.go
git commit -s -m "feat(config): nest OIDC + JITCreate under auth.oidc with legacy migration"
```

---

## Task 29: Update `cmd/specgraph/serve.go` wiring

Construct the new components: `usagetracker.Manager`, `OIDCVerifier`s, `pgIdentityStore`, `StaticTableAuthorizer`. Pass to `NewAuthInterceptor`. Remove old store construction. Remove `cfg.Auth.Mode` validation. Remove `auth.Bootstrap()` (moves to Bootstrap & UX plan). Wire `Manager.Close()` into shutdown.

**Files:**

- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Identify the diff**

The existing serve.go (around lines 128–210, per the merged code) has:

- `auth.NewConfigStore(...)` — DELETE
- `auth.NewCompositeStore(...)` — DELETE
- `cfg.Auth.Mode` validation block — DELETE
- `auth.Bootstrap(credPath)` call — DELETE (moves to Bootstrap & UX)
- `auth.NewOIDCStore(...)` per provider — REPLACE with `auth.NewOIDCVerifier(...)`
- `auth.NewAuthInterceptor(compositeStore)` — REPLACE with `auth.NewAuthInterceptorV2(resolver, authorizer)` (the V2 name persists until the cleanup task renames it back to `NewAuthInterceptor`)

- [ ] **Step 2: Rewrite the auth wiring section**

```go
// Build the auth pool via the existing postgres.Store.Pool() accessor.
authStore, err := postgres.NewAuth(ctx, store.Pool())
if err != nil {
	return fmt.Errorf("auth store: %w", err)
}
defer func() { _ = authStore.Close(ctx) }()

// Construct OIDC verifiers (one per provider; renamed from OIDCStore).
// Iterate cfg.Auth.OIDC.Providers — the post-migration canonical field.
// Task 28b's loader copies legacy cfg.Auth.OIDCProviders into this field,
// so reading the legacy field here would yield ZERO verifiers for any
// config that uses the new auth.oidc.providers shape (silently breaking
// all JWT/JIT auth). Must match the claims-mapping source below.
verifiers := make([]*auth.OIDCVerifier, 0, len(cfg.Auth.OIDC.Providers))
for _, pc := range cfg.Auth.OIDC.Providers {
	issuerCtx, issuerCancel := context.WithTimeout(ctx, 10*time.Second)
	v, oidcErr := auth.NewOIDCVerifier(issuerCtx, pc)
	issuerCancel()
	if oidcErr != nil {
		return fmt.Errorf("OIDC provider %s: %w", pc.ID, oidcErr)
	}
	verifiers = append(verifiers, v)
}

// usagetracker for async TouchLastUsed.
tracker := usagetracker.NewManager(authStore, usagetracker.Config{
	BufferSize:    256,
	FlushInterval: 5 * time.Second,
})
defer func() {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := tracker.Close(shutdownCtx); err != nil {
		slog.Warn("usagetracker close", "error", err)
	}
	if dropped := tracker.Dropped(); dropped > 0 {
		slog.Warn("usagetracker dropped touches over session lifetime",
			"count", dropped,
			"hint", "increase auth usagetracker BufferSize or decrease FlushInterval")
	}
}()

// Role→permissions snapshot (built-ins ∪ cfg.Auth.Roles). Shared by the
// authorizer and used to derive KnownRoles for JIT validation.
rolePerms := auth.LoadRolePerms(cfg.Auth.Roles)
knownRoles := make(map[auth.Role]bool, len(rolePerms))
for r := range rolePerms {
	knownRoles[r] = true
}

// IdentityStore.
resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
	Users:                   authStore,
	Verifiers:               verifiers,
	Tracker:                 tracker,
	KnownRoles:              knownRoles,
	JITEnabled:              cfg.Auth.OIDC.JITCreate.Enabled,
	JITDefaultRole:          auth.Role(cfg.Auth.OIDC.JITCreate.DefaultRole),
	JITClaimsMapping:        buildClaimsMappingByIssuer(cfg.Auth.OIDC.Providers),
	JITRateBurstPerHour:     cfg.Auth.OIDC.JITCreate.RateLimitPerHour,
	JITEmailDomainAllowlist: cfg.Auth.OIDC.JITCreate.EmailDomainAllowlist,
})
if err != nil {
	return fmt.Errorf("identity store: %w", err)
}

// Authorizer (static table for now; Cedar plan swaps).
authorizer := auth.NewStaticTableAuthorizer(rolePerms)

interceptor := auth.NewAuthInterceptorV2(resolver, authorizer) // renamed in cleanup task

// HasAuth signal for the existing warn path.
if has, _ := resolver.HasAuth(ctx); !has {
	warnIfNoAuthOnPublicListen(cfg.Server.Listen, false)
}
```

> Note: `buildClaimsMappingByIssuer` is a small helper added to serve.go that converts `[]config.OIDCProviderConfig` into `map[string][]config.ClaimMapping` keyed by issuer URL (each provider contributes its `ClaimsMapping` slice under its issuer). Role→permissions come from the EXPORTED `auth.LoadRolePerms(cfg.Auth.Roles)` (Task 4) — serve.go does not define its own role-perms helper. `cfg.Auth.OIDC.*` references the nested config shape introduced in Task 28b. Add explicit imports for `slog`, `time`, `internal/auth/usagetracker`, and `internal/config` at the top of serve.go.

- [ ] **Step 3: Verify compile + tests**

Run: `cd cmd/specgraph && go build ./... && cd ../.. && go test ./...`

Expected: PASS, with the caveat that integration tests in `e2e/` may need parallel adaptation if they exercise the old auth wiring.

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): wire pgIdentityStore + StaticTableAuthorizer + usagetracker"
```

---

## Task 30: Delete obsolete files

After Tasks 26–29 land, nothing references `config_store.go`, `composite_store.go`, `token_store.go`, or `oidc_store.go`. Delete them.

**Files:**

- Delete: `internal/auth/config_store.go` + `config_store_test.go`
- Delete: `internal/auth/composite_store.go` + `composite_store_test.go`
- Delete: `internal/auth/token_store.go` + `token_store_test.go`
- Delete: `internal/auth/oidc_store.go` + `oidc_store_test.go`

- [ ] **Step 1: Delete the files**

```bash
rm internal/auth/config_store.go internal/auth/config_store_test.go
rm internal/auth/composite_store.go internal/auth/composite_store_test.go
rm internal/auth/token_store.go internal/auth/token_store_test.go
rm internal/auth/oidc_store.go internal/auth/oidc_store_test.go
```

- [ ] **Step 2: Verify the package still compiles**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS. If FAIL: some Phase B task missed a reference. Re-run grep:

```bash
grep -rn 'NewConfigStore\|NewCompositeStore\|NewOIDCStore\|TokenStore' .
```

Resolve any remaining references before committing.

- [ ] **Step 3: Commit**

```bash
git add -A internal/auth/
git commit -s -m "chore(auth): delete obsolete ConfigStore, CompositeStore, TokenStore, OIDCStore"
```

---

## Task 30b: Drop the V2 suffixes; delete remaining legacy functions

Phase A used five V2-suffixed names to coexist with identically-named legacy functions: `NewAuthInterceptorV2`, `RequireAuthV2`, `authenticateV2`, `peekIssuerV2`, `matchClaimValueV2`. Task 30 deleted `composite_store.go` (legacy `peekIssuer`) and `oidc_store.go` (legacy `matchClaimValue`), so those two canonical names are now free. The legacy `NewAuthInterceptor`, `RequireAuth`, and `authenticate` still live in `interceptor.go`/`middleware.go` and must be deleted here before their V2 counterparts can take the canonical names.

This task removes all five V2 suffixes and deletes the three remaining legacy functions, leaving the canonical names pointing at the new implementations.

**Files:**

- Modify: `internal/auth/interceptor.go`, `internal/auth/interceptor_test.go`
- Modify: `internal/auth/middleware.go`, `internal/auth/middleware_test.go`
- Modify: `internal/auth/identitystore.go` (peekIssuerV2, matchClaimValueV2 + their callers)
- Modify: `cmd/specgraph/serve.go`

- [ ] **Step 1: Confirm the canonical names are free**

```bash
# Legacy peekIssuer / matchClaimValue should be GONE (their files were
# deleted in Task 30). Expect matches ONLY for the V2 names:
grep -rn 'func peekIssuer\b\|func matchClaimValue\b' --include='*.go' internal/auth/
# Legacy NewAuthInterceptor / RequireAuth / authenticate still exist here
# (to be deleted in Step 2); V2 versions also present:
grep -rn 'func NewAuthInterceptor\|func RequireAuth\|func authenticate' --include='*.go' internal/auth/
```

If a legacy `peekIssuer`/`matchClaimValue` still appears, Task 30 didn't fully delete its file — fix that first.

- [ ] **Step 2: Delete the three remaining legacy functions**

- Remove legacy `func NewAuthInterceptor(store IdentityStore)` from `interceptor.go`.
- Remove legacy `func RequireAuth(store IdentityStore)` from `middleware.go`.
- Remove legacy `func authenticate(ctx, store IdentityStore, r *http.Request) (*Identity, bool)` from `middleware.go`.
- Remove the legacy tests that exercise them from `interceptor_test.go` and `middleware_test.go`.

- [ ] **Step 3: Drop the V2 suffixes (rename to canonical)**

Search-and-replace across `internal/auth/` and `cmd/specgraph/serve.go`:

- `NewAuthInterceptorV2` → `NewAuthInterceptor`
- `RequireAuthV2` → `RequireAuth`
- `authenticateV2` → `authenticate`
- `peekIssuerV2` → `peekIssuer`
- `matchClaimValueV2` → `matchClaimValue`

Update the V2 test names to drop the suffix too, and serve.go's `auth.NewAuthInterceptorV2(...)` → `auth.NewAuthInterceptor(...)`.

- [ ] **Step 4: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./... && cd ../../cmd/specgraph && go build ./...`

Then re-run the grep from Step 1 — expect exactly one definition of each canonical name (`peekIssuer`, `matchClaimValue`, `authenticate`, `NewAuthInterceptor`, `RequireAuth`), all pointing at the new implementations.

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/interceptor.go internal/auth/interceptor_test.go \
        internal/auth/middleware.go internal/auth/middleware_test.go \
        internal/auth/identitystore.go cmd/specgraph/serve.go
git commit -s -m "chore(auth): drop V2 suffixes, delete remaining legacy auth functions"
```

---

## Task 31: Remove `Identity.Permissions` and legacy interface

Now that nothing in the package references these, clean them up.

**Files:**

- Modify: `internal/auth/auth.go` (remove `Permissions` field)
- Modify: `internal/auth/store.go` (remove old sentinels + `IdentityStore` interface)
- Search-and-fix any remaining references.

- [ ] **Step 1: Grep for residual references**

```bash
grep -rn 'Permissions\|ErrUnknownKey\|ErrNoOIDC\|ErrUnknownIssuer\|ErrInvalidToken\|IdentityStore' internal/auth/ cmd/specgraph/
```

Inspect; anything that remains is something to fix in this task before deletion.

- [ ] **Step 2: Remove `Permissions` from `Identity`**

In `auth.go`:

```go
type Identity struct {
	UserID        string
	EffectiveRole Role
	Email         string

	Subject     string
	DisplayName string
	Role        Role
	Source      string // "apikey" | "oidc"
}
```

Drop the `Permissions` line and its comment.

- [ ] **Step 3: Remove `HasPermission` exported helper**

If it survived Task 4 in `auth.go`, delete it. The package-private `hasPermissionInternal` in `static_authorizer.go` is the only remaining permission-check helper.

- [ ] **Step 4: Remove old sentinels and the `IdentityStore` interface from `store.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "errors"

// ErrUnauthenticated and ErrTransient remain. Old sentinels removed.
var (
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	ErrTransient       = errors.New("auth: transient backend error")
)
```

The `IdentityStore` interface is deleted (Resolver replaces it).

- [ ] **Step 5: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/auth.go internal/auth/store.go
git commit -s -m "chore(auth): remove Identity.Permissions, HasPermission, legacy IdentityStore"
```

---

## Task 32: Remove `"local"` Source value

The implicit OS-user path is gone; `"local"` is no longer a valid `Identity.Source`.

**Files:**

- Modify: `internal/auth/auth.go` (update Source comment)
- Modify: any test that asserted `Source == "local"`.

- [ ] **Step 1: Update the Source comment**

```go
Source string // "apikey" | "oidc"
```

- [ ] **Step 2: Grep for "local" assertions**

```bash
grep -rn '"local"' internal/auth/ cmd/specgraph/
```

Update or remove any test/assertion still expecting `"local"`.

- [ ] **Step 3: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/auth.go
git commit -s -m "chore(auth): remove 'local' from valid Identity.Source values"
```

---

## Task 33: Integration test — full resolve flow

End-to-end test against a real `AuthStore`. Bootstraps an admin via direct SQL, mints a key with a known PHC hash, resolves the key, asserts the Identity.

**Files:**

- Create: `internal/auth/identitystore_integration_test.go`

**Covers:** E2E (Resolver + UsersBackend + argon2id; one user lifecycle).

- [ ] **Step 1: Write the test**

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

func TestIntegration_APIKeyResolve(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx) // from postgres test harness
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	// Truncate then create a user + key.
	_, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	user := activeUser("aaaa0000-0000-0000-0000-000000000001", "writer", storage.KindHuman)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, kind, display_name, role)
	                         VALUES ($1::uuid, 'human', $2, $3)`,
		user.ID, user.DisplayName, user.Role)
	require.NoError(t, err)

	// PHC hash for plaintext "test-secret-32-chars-padding-aa".
	_, err = pool.Exec(ctx, `INSERT INTO api_keys (id, user_id, prefix, phc_hash)
	                         VALUES ('bbbb0000-0000-0000-0000-000000000002'::uuid,
	                                 $1::uuid, 'abc12345', $2)`,
		user.ID, stubPHCHash)
	require.NoError(t, err)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   authStore,
		Tracker: tracker,
	})
	require.NoError(t, err)

	id, err := resolver.Resolve(ctx, stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, user.ID, id.UserID)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, "apikey", id.Source)
}
```

- [ ] **Step 2: Run**

Run: `cd internal/auth && go test -tags integration -run 'TestIntegration_APIKeyResolve' -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/identitystore_integration_test.go
git commit -s -m "test(auth): integration test for end-to-end API-key resolve"
```

---

## Task 34: Lifecycle smoke (auth-only)

A multi-step lifecycle test: bootstrap admin authenticates, JIT a real OIDC user, revoke the bootstrap key, assert bootstrap can't auth but JIT'd user can.

**Files:**

- Modify: `internal/auth/identitystore_integration_test.go`

**Covers:** E2E (multi-step lifecycle through Resolver + UsersBackend).

- [ ] **Step 1: Write the test**

```go
func TestIntegration_Lifecycle(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	// Seed a bootstrap admin via direct SQL.
	admin, err := authStore.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true,
	}, nil)
	require.NoError(t, err)

	// Mint a key for admin.
	adminKey, err := authStore.CreateAPIKey(ctx, &storage.APIKey{
		UserID: admin.ID, PHCHash: stubPHCHash,
	})
	require.NoError(t, err)

	// Build resolver.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, _ := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: authStore, Tracker: tracker,
	})

	// Bootstrap admin authenticates.
	id, err := resolver.Resolve(ctx,
		stubAPIKeyToken(adminKey.Prefix))
	require.NoError(t, err)
	require.Equal(t, admin.ID, id.UserID)

	// Revoke the key.
	require.NoError(t, authStore.RevokeAPIKey(ctx, adminKey.ID))

	// Bootstrap can no longer authenticate.
	_, err = resolver.Resolve(ctx,
		stubAPIKeyToken(adminKey.Prefix))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}
```

- [ ] **Step 2: Run all integration tests**

Run: `cd internal/auth && go test -tags integration -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/identitystore_integration_test.go
git commit -s -m "test(auth): integration lifecycle smoke (bootstrap → revoke → reject)"
```

---

## Self-Review

**Spec coverage check:**

- [x] Single resolver replacing multi-store routing: Task 9 (scaffold) + 10–18 (impl).
- [x] OIDC verifier split from user materialization: Tasks 5–6.
- [x] No implicit OS-user identity: Task 28.
- [x] JIT controls (rate limit + email allowlist): Tasks 19–20.
- [x] Error categorization (3 categories → 3 codes): Tasks 11–18 (sentinel returns) + Task 26 (mapping).
- [x] Authorizer interface seam: Tasks 3–4.
- [x] ClaimsMapping at JIT-only: Task 21.
- [x] Async last-used (usagetracker): Tasks 23–25.
- [x] Dashboard cookie path preserved: Task 27.
- [x] `cfg.Auth.Mode` removal: Task 29.
- [x] Identity.Permissions removal: Task 31 (Phase C, after Cedar landing point is reachable from this plan).

**Build discipline:**

Every task ends with `go build ./... && go test ./...` green — at BOTH the `internal/auth/` package level AND the whole-project level. The Phase B cutover uses five V2-suffixed names to coexist with identically-named legacy functions during Phase A: `NewAuthInterceptorV2`/`RequireAuthV2` (Tasks 26–27) plus the package helpers `authenticateV2` (Task 26), `peekIssuerV2` (Task 16), and `matchClaimValueV2` (Task 21) — each collides by name with a legacy definition in `composite_store.go`/`oidc_store.go`/`middleware.go` that survives until Task 30. The new functions are added alongside the legacy ones; serve.go keeps calling the legacy interceptor until Task 29 switches; Task 30b deletes the remaining legacy functions and drops all five V2 suffixes after Task 30 removes the legacy files. This avoids the window where `serve.go` would fail to compile against a changed interceptor signature. Tasks 1–2 are additive to existing types. Tasks 3–8 add new files. Tasks 9–25 add `pgIdentityStore` + `usagetracker.Manager` alongside old code. Task 28b lands the nested OIDC/JIT config. Phase C (30, 30b, 31, 32) deletes obsolete code only after Phase B confirms nothing references it.

**Placeholder scan:**

All 35 tasks (1–34 plus 28b, 30b) fully expanded with TDD cycles (failing test → verify failure → impl → run to pass → commit). Two documented decision points are flagged inline as the executor's call, not as placeholders:

- Task 20: three of the four allowlist sub-tests have their bodies summarized in a comment, with the first one shown in full. The executor copies the first into the other three with different email values.
- Task 26: the existing `interceptor_test.go` harness is substantial; the plan describes the new V2 assertions but expects the executor to adapt the existing test scaffolding rather than re-write it from scratch.

The PHC-secret harmonization that was previously a manual decision point is now mechanically solved: `stubPHCSecret`, `stubPHCHash`, and `stubAPIKeyToken` live in the Task 8 fixtures file; `TestStubSecretLength` guards the length; the verifier and fixture use identical argon2id parameters.

**Type consistency:**

`Identity`, `Resolver`, `Authorizer`, `Decision`, `OIDCVerifier`, `OIDCClaims`, `pgIdentityStore`, `LastUsedTracker`, `usagetracker.Manager`, `StaticTableAuthorizer`, `IdentityStoreConfig`, `OIDCClaims` referenced consistently across all 34 tasks. The `pgIdentityStore` implements `Resolver`; the compile-time assertion is implicit via the constructor's return type.

**Build-discipline check (every task ends green):**

- Tasks 1–8: additive (new files, new fields, shared test fixtures). Package compiles after each. `go mod tidy` in Task 9/10 promotes `golang.org/x/time` from indirect to direct.
- Tasks 9–22: build out `pgIdentityStore` incrementally; each method-introducing task is a self-contained TDD cycle on the stub UsersBackend. Package compiles after each.
- Tasks 23–25: add `usagetracker` package in isolation; wire docstring after.
- Task 28b: lands nested `auth.oidc.jit_create` config so Task 29's serve.go references compile (with transparent legacy `auth.oidc_providers` migration).
- Tasks 26–29: the Phase B cutover via the V2 transition. Task 26 ADDS `NewAuthInterceptorV2` (legacy `NewAuthInterceptor` untouched); Task 27 ADDS `RequireAuthV2`; Task 28 removes residual OS-user code; Task 29 switches serve.go to the V2 constructors. **Whole-project compiles at every commit** — serve.go calls the legacy interceptor until Task 29, then the V2 one. No signature-mismatch window.
- Tasks 30, 30b, 31, 32: deletion phase. Legacy stores deleted (30), V2 renamed to canonical + legacy interceptor/middleware deleted (30b), `Identity.Permissions` + old sentinels + old interface removed (31), "local" Source value removed (32). Each task ends compiling.
- Tasks 33–34: pure-additive integration tests (`//go:build integration`).

---

## Execution

The plan supports both execution modes:

1. **Subagent-Driven (recommended)** — fresh subagent per task. Tasks 1–22 are short, well-scoped TDD cycles ideal for a fleet. Tasks 23–25 (usagetracker), 28b (config), 26–30b (cutover + cleanup), and 33–34 (integration) form natural batches; a coordinator subagent can run each batch with checkpoint review between.
2. **Inline Execution** — practical for the Phase B cutover (Tasks 26–30b) which involves coordinated changes across multiple files. The earlier and later phases work fine inline too.

The Cedar plan follows this one. Per the Authorizer-interface seam established in Tasks 3–4, the Cedar plan adds `CedarAuthorizer` and deletes `StaticTableAuthorizer` + `internal/auth/permissions.go` + the `hasPermissionInternal` helper. The interceptor diff in Cedar is **zero** — that's the payoff of the seam.
