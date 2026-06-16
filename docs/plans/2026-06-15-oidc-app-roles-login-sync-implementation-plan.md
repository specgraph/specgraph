# OIDC App-Roles + Login-Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On interactive OIDC login, refresh a user's `DisplayName`/`Email` and re-derive their role from the issuer's `claims_mapping` (so app-role assignments — the fix for the #996 groups-overage problem — take effect on every login, not just at JIT creation).

**Architecture:** A new `applyLoginSync` method on `pgIdentityStore` runs in `resolveJWT`'s existing-binding branch, gated on `loginSyncEnabled && InteractiveLoginFromContext(ctx)`. It re-enforces the email allowlist, refreshes metadata with guards, and re-derives the role via a pure `resolveLoginRole` helper. Persistence is one atomic `UPDATE` on the `users` row (`UpdateUserOnLogin`). Failures are handled asymmetrically: demotions/allowlist-misses deny the login (fail closed), promotions/metadata-only failures are best-effort. A `sync_on_login` config flag (default true) gates the feature.

**Tech Stack:** Go, `github.com/coreos/go-oidc/v3`, pgx v5 (Postgres), koanf config, testify, testcontainers (Postgres integration tests).

**Design doc:** `docs/plans/2026-06-15-oidc-app-roles-login-sync-design.md`

---

## File Structure

| File | Responsibility | Change |
|------|----------------|--------|
| `internal/auth/oidc_verifier.go` | Add parsed `Name` claim to `OIDCClaims` | Modify |
| `internal/config/global.go` | Add `OIDCConfig.SyncOnLogin` field + default | Modify |
| `internal/storage/users.go` | Add `UpdateUserOnLogin` to `UsersBackend` interface | Modify |
| `internal/storage/postgres/users.go` | Implement `UpdateUserOnLogin` | Modify |
| `internal/storage/postgres/users_test.go` | Integration test for `UpdateUserOnLogin` | Modify |
| `internal/auth/usersbackend_stub_test.go` | Stub gains `UpdateUserOnLogin` (configurable) | Modify |
| `internal/server/usersbackend_stub_test.go` | Stub gains `UpdateUserOnLogin` | Modify |
| `internal/auth/loginsync.go` | `resolveLoginRole`, `isPromotion`, `applyLoginSync` | Create |
| `internal/auth/loginsync_internal_test.go` | White-box (`package auth`) tests for the helpers + `applyLoginSync` (local fakes) | Create |
| `internal/auth/oidc_verifier_internal_test.go` | White-box test for `nameFromClaims` | Create |
| `internal/auth/identitystore.go` | New `loginSyncEnabled` field + config + startup validation + hook in `resolveJWT` | Modify |
| `internal/auth/context.go` | Defensive comment on `WithInteractiveLogin` | Modify |
| `cmd/specgraph/serve.go` | Wire `LoginSyncEnabled` from config | Modify |
| `site/docs/guides/oidc-login.md` | Document app-roles, `sync_on_login`, warnings | Modify |

Conventions: every `.go` file needs the SPDX header (`task license:add` fixes). New packages need a `// Package` doc comment (not needed here — all in existing `auth`/`config`/`postgres` packages). Run `task check` before pushing.

---

## Task 1: Add parsed `Name` to `OIDCClaims`

**Files:**

- Modify: `internal/auth/oidc_verifier.go` (struct ~23-29; `Verify` ~78-112)
- Create: `internal/auth/oidc_verifier_internal_test.go` (white-box, `package auth`)

- [ ] **Step 1: Write the failing test**

Create `internal/auth/oidc_verifier_internal_test.go` as a **white-box** test
(`package auth`, mirroring the existing `internal/auth/clampedrole_internal_test.go`
convention) so it can call the unexported `nameFromClaims`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNameFromClaims(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(`"` + s + `"`) }
	cases := []struct {
		name   string
		claims map[string]json.RawMessage
		want   string
	}{
		{"name preferred", map[string]json.RawMessage{"name": raw("Ada Lovelace"), "preferred_username": raw("ada@x.io")}, "Ada Lovelace"},
		{"falls back to preferred_username", map[string]json.RawMessage{"preferred_username": raw("ada@x.io")}, "ada@x.io"},
		{"both absent", map[string]json.RawMessage{}, ""},
		{"empty name falls through", map[string]json.RawMessage{"name": raw(""), "preferred_username": raw("ada@x.io")}, "ada@x.io"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, nameFromClaims(tc.claims))
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestNameFromClaims`
Expected: FAIL — `undefined: nameFromClaims`.

- [ ] **Step 3: Add the `Name` field and `nameFromClaims` helper, populate in `Verify`**

In `internal/auth/oidc_verifier.go`, add `Name` to the struct:

```go
type OIDCClaims struct {
	Issuer  string
	Subject string
	Email   string
	Name    string // parsed display name: "name" claim, falling back to "preferred_username"
	Nonce   string
	Raw     map[string]json.RawMessage
}
```

Add the helper (place it near the bottom of the file, after `nonceMatches`):

```go
// nameFromClaims resolves a human-friendly display name from the claims,
// preferring the standard "name" claim and falling back to
// "preferred_username". Returns "" when neither is present or both are empty.
func nameFromClaims(raw map[string]json.RawMessage) string {
	for _, claim := range []string{"name", "preferred_username"} {
		rawVal, ok := raw[claim]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(rawVal, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}
```

In `Verify`, after the email-resolution loop and before `c.Nonce = idToken.Nonce`, add:

```go
	c.Name = nameFromClaims(raw)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -run TestNameFromClaims`
Expected: PASS.

- [ ] **Step 5: Verify the package still builds**

Run: `go build ./internal/auth/`
Expected: no output (success).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/oidc_verifier.go internal/auth/oidc_verifier_internal_test.go
git commit -s -m "feat(auth): parse Name claim into OIDCClaims for login metadata sync"
```

---

## Task 2: Add `sync_on_login` config field (default true)

**Files:**

- Modify: `internal/config/global.go` (`OIDCConfig` ~206-219; `globalDefaults` ~413-431)
- Test: `internal/config/global_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/config/global_test.go`:

```go
func TestSyncOnLogin_DefaultsTrue_WhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("auth:\n  oidc:\n    providers: []\n"), 0o600))

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	require.True(t, cfg.Auth.OIDC.SyncOnLogin, "absent sync_on_login must default to true")
}

func TestSyncOnLogin_ExplicitFalse_Honored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("auth:\n  oidc:\n    sync_on_login: false\n"), 0o600))

	cfg, err := config.LoadGlobalExplicit(path)
	require.NoError(t, err)
	require.False(t, cfg.Auth.OIDC.SyncOnLogin, "explicit false must survive the default merge")
}
```

Ensure `global_test.go` imports `os`, `path/filepath`, `testing`, `github.com/stretchr/testify/require`, and the `config` package (match the existing import style in that file; if it is `package config` white-box, drop the `config.` qualifier and use `LoadGlobalExplicit`/`cfg.Auth...` directly).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestSyncOnLogin`
Expected: FAIL — `cfg.Auth.OIDC.SyncOnLogin undefined`.

- [ ] **Step 3: Add the field and default**

In `internal/config/global.go`, add to `OIDCConfig` (after `CLILoginEnabled`):

```go
	// SyncOnLogin enables refreshing DisplayName/Email and re-evaluating the
	// role from token claims on each interactive login. Default true (set in
	// globalDefaults). MUST NOT be defaulted in applyPostLoad: a default-true
	// bool cannot be distinguished there from an explicit `false`.
	SyncOnLogin bool `yaml:"sync_on_login" koanf:"sync_on_login"`
```

In `globalDefaults()`, change the OIDC default to set both flags:

```go
		Auth: AuthConfig{
			OIDC: OIDCConfig{CLILoginEnabled: true, SyncOnLogin: true},
		},
```

Do **not** touch `applyPostLoad`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run TestSyncOnLogin`
Expected: PASS (both subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/config/global.go internal/config/global_test.go
git commit -s -m "feat(config): add auth.oidc.sync_on_login flag (default true)"
```

---

## Task 3: Add `UpdateUserOnLogin` storage method

**Files:**

- Modify: `internal/storage/users.go` (`UsersBackend` interface ~49-51)
- Modify: `internal/storage/postgres/users.go` (after `UpdateUserRole` ~225-237)
- Modify: `internal/storage/postgres/users_test.go` (integration)
- Modify: `internal/auth/usersbackend_stub_test.go` and `internal/server/usersbackend_stub_test.go`

- [ ] **Step 1: Add the interface method**

In `internal/storage/users.go`, after the `UpdateUserRole` declaration:

```go
	// UpdateUserOnLogin sets display_name, email, AND role on an active user in
	// a single UPDATE (deleted_at IS NULL guard, like UpdateUserRole). Used by
	// the OIDC login-sync path. Returns ErrUserNotFound if no active row matched.
	// Role validation is the caller's responsibility.
	UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error
```

- [ ] **Step 2: Run build to verify it fails (interface not satisfied)**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — `*AuthStore` (and the two test stubs) do not implement `storage.UsersBackend` (missing `UpdateUserOnLogin`).

- [ ] **Step 3: Implement on `AuthStore`**

In `internal/storage/postgres/users.go`, after `UpdateUserRole`:

```go
// UpdateUserOnLogin sets display_name, email, and role on an active user in a
// single statement. Returns ErrUserNotFound if no active user has the given ID.
func (s *AuthStore) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	const q = `
		UPDATE users SET display_name = $1, email = $2, role = $3
		WHERE id = $4::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, displayName, email, role, userID)
	if err != nil {
		return fmt.Errorf("update user on login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}
```

- [ ] **Step 4: Add to the auth test stub**

In `internal/auth/usersbackend_stub_test.go`, add a configurable hook field to the `usersBackendStub` struct (alongside `listUsers`):

```go
	updateUserOnLogin func(ctx context.Context, userID, displayName, email, role string) error
```

And the method (place near `UpdateUserRole`):

```go
func (s *usersBackendStub) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	if s.updateUserOnLogin != nil {
		return s.updateUserOnLogin(ctx, userID, displayName, email, role)
	}
	return errUnexpectedCall("UpdateUserOnLogin")
}
```

- [ ] **Step 5: Add to the server test stub**

In `internal/server/usersbackend_stub_test.go`, add the method mirroring its `UpdateUserRole` style (add an optional `updateUserOnLogin` func field if that stub uses field hooks; otherwise a no-op returning `nil`):

```go
func (s *usersBackendStub) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	if s.updateUserOnLogin != nil {
		return s.updateUserOnLogin(ctx, userID, displayName, email, role)
	}
	return nil
}
```

If that stub does not use func-field hooks, drop the `if` and just `return nil`. Add the `updateUserOnLogin` field to its struct only if you used the hook form.

- [ ] **Step 6: Verify build passes**

Run: `go build ./... && go vet ./internal/storage/... ./internal/auth/... ./internal/server/...`
Expected: no output (success).

- [ ] **Step 7: Write the integration test**

In `internal/storage/postgres/users_test.go` (build tag `//go:build integration`), add — model the setup on the existing `TestAuthStore_UpdateUserRole` (~269):

```go
func TestAuthStore_UpdateUserOnLogin(t *testing.T) {
	ctx := context.Background()
	auth := newTestAuthStore(t) // use whatever helper the existing tests use to get an *AuthStore
	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "old-sub", Email: "old@x.io", Role: "reader",
	}, nil)
	require.NoError(t, err)

	// Happy path: all three columns update.
	require.NoError(t, auth.UpdateUserOnLogin(ctx, u.ID, "Ada", "ada@x.io", "admin"))
	got, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "Ada", got.DisplayName)
	require.Equal(t, "ada@x.io", got.Email)
	require.Equal(t, "admin", got.Role)

	// Unknown user -> ErrUserNotFound.
	err = auth.UpdateUserOnLogin(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa", "x", "x@x.io", "reader")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Soft-deleted user -> ErrUserNotFound (active-row guard).
	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))
	err = auth.UpdateUserOnLogin(ctx, u.ID, "y", "y@x.io", "writer")
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}
```

Match the actual helper name used by neighbouring tests for constructing the `*AuthStore` (search the file for how `TestAuthStore_UpdateUserRole` obtains `auth`). Use that exact pattern instead of `newTestAuthStore` if it differs.

- [ ] **Step 8: Run the integration test (requires Docker)**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestAuthStore_UpdateUserOnLogin`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/storage/users.go internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/auth/usersbackend_stub_test.go internal/server/usersbackend_stub_test.go
git commit -s -m "feat(storage): add UpdateUserOnLogin for OIDC login-sync"
```

---

## Task 4: Pure helpers `resolveLoginRole` and `isPromotion`

**Files:**

- Create: `internal/auth/loginsync.go`
- Create: `internal/auth/loginsync_internal_test.go` (white-box, `package auth`)

- [ ] **Step 1: Write the failing tests**

Create `internal/auth/loginsync_internal_test.go`. It is white-box (`package auth`)
because it tests unexported `resolveLoginRole`/`isPromotion` (and, in Task 6,
`applyLoginSync` on the unexported `pgIdentityStore`). It defines its **own**
local fakes — the existing `usersBackendStub`/`noopTracker` live in `package
auth_test` and are not visible here:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// loginSyncFakeBackend satisfies storage.UsersBackend via an embedded (nil)
// interface; only UpdateUserOnLogin is exercised by applyLoginSync. Any other
// method call would nil-panic, which correctly flags an unexpected dependency.
type loginSyncFakeBackend struct {
	storage.UsersBackend
	updateUserOnLogin func(ctx context.Context, userID, displayName, email, role string) error
}

func (f loginSyncFakeBackend) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	return f.updateUserOnLogin(ctx, userID, displayName, email, role)
}

type loginSyncTracker struct{}

func (loginSyncTracker) Touch(string) {}

func rawArr(vals ...string) json.RawMessage {
	b, _ := json.Marshal(vals)
	return b
}

func TestResolveLoginRole(t *testing.T) {
	adminRule := []config.ClaimMapping{{Claim: "roles", Value: "specgraph.admin", Role: "admin"}}
	claimsAdmin := map[string]json.RawMessage{"roles": rawArr("specgraph.admin")}
	claimsNone := map[string]json.RawMessage{"roles": rawArr("specgraph.other")}

	cases := []struct {
		name        string
		mappings    []config.ClaimMapping
		claims      map[string]json.RawMessage
		current     string
		defaultRole string
		wantRole    string
		wantChanged bool
	}{
		{"rule1 no mappings -> unchanged", nil, claimsNone, "admin", "reader", "admin", false},
		{"rule1 empty slice -> unchanged", []config.ClaimMapping{}, claimsNone, "writer", "reader", "writer", false},
		{"rule2 match", adminRule, claimsAdmin, "reader", "reader", "admin", true},
		{"rule2 match no change -> changed=false", adminRule, claimsAdmin, "admin", "reader", "admin", false},
		{"rule3 no match -> default_role", adminRule, claimsNone, "admin", "reader", "reader", true},
		{"rule3 default unset -> reader", adminRule, claimsNone, "admin", "", "reader", true},
		{"rule3 numeric claim never matches", adminRule, map[string]json.RawMessage{"roles": json.RawMessage(`[1]`)}, "admin", "reader", "reader", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, changed := resolveLoginRole(tc.mappings, tc.claims, tc.current, tc.defaultRole)
			require.Equal(t, tc.wantRole, role)
			require.Equal(t, tc.wantChanged, changed)
		})
	}
}

func TestIsPromotion(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		{"reader", "writer", true},
		{"writer", "admin", true},
		{"reader", "admin", true},
		{"admin", "writer", false},     // demotion
		{"writer", "writer", false},    // equal
		{"reader", "auditor", false},   // builtin -> custom (incomparable)
		{"auditor", "admin", false},    // custom -> builtin
		{"auditor", "releaser", false}, // custom -> custom
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, isPromotion(tc.current, tc.next), "%s->%s", tc.current, tc.next)
	}
}
```

The `context`, `errors`, and `storage` imports are used by the Task-6 tests
appended to this same file; including them now keeps the file compiling once
Task 6 lands (they are unused until then — if `go vet` complains about unused
imports between tasks, add the Task-6 tests in the same change, or temporarily
drop the unused imports and re-add in Task 6).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run 'TestResolveLoginRole|TestIsPromotion'`
Expected: FAIL — `undefined: resolveLoginRole`, `undefined: isPromotion`.

- [ ] **Step 3: Implement the helpers**

Create `internal/auth/loginsync.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"encoding/json"

	"github.com/specgraph/specgraph/internal/config"
)

// resolveLoginRole computes the role for an interactive login from the issuer's
// claims_mapping and the verified claims. Returns (newRole, changed).
//
// Rules, evaluated in this exact order (the ordering is the correctness hinge —
// conflating "no mappings" with "no match" would mass-demote mapping-less
// providers):
//
//	1. len(mappings) == 0           -> currentRole unchanged.
//	2. a rule matches               -> that rule's role.
//	3. mappings exist, none match   -> defaultRole (or "reader" if unset).
func resolveLoginRole(mappings []config.ClaimMapping, claims map[string]json.RawMessage, currentRole, defaultRole string) (string, bool) {
	if len(mappings) == 0 {
		return currentRole, false // rule 1
	}
	if matched := applyClaimsMapping(claims, mappings); matched != "" {
		return matched, matched != currentRole // rule 2
	}
	floor := defaultRole // rule 3
	if floor == "" {
		floor = string(RoleReader)
	}
	return floor, floor != currentRole
}

// isPromotion reports true ONLY when both roles are ranked built-ins and the
// new built-in rank is strictly higher than the current. Every other change — a
// rank decrease, equal roles, or ANY transition involving a custom/unranked
// role — returns false, so the login-sync error model treats it as a potential
// demotion and fails closed. This mirrors clampedRole's fail-closed philosophy
// for incomparable custom roles.
func isPromotion(current, next string) bool {
	return roleLessThan(Role(current), Role(next))
}
```

(`applyClaimsMapping` and `roleLessThan` already exist in `identitystore.go`.)

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run 'TestResolveLoginRole|TestIsPromotion'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/loginsync.go internal/auth/loginsync_internal_test.go
git commit -s -m "feat(auth): add resolveLoginRole and isPromotion helpers"
```

---

## Task 5: Wire `loginSyncEnabled` + extend startup validation

**Files:**

- Modify: `internal/auth/identitystore.go` (struct ~37-52; `IdentityStoreConfig` ~54-110; validation ~115; assembly ~140-156)
- Modify: `cmd/specgraph/serve.go` (~162-173)
- Test: `internal/auth/identitystore_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/identitystore_test.go`:

```go
func TestNewIdentityStore_ValidatesMappingRoles_WhenLoginSyncOnAndJITOff(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:            &usersBackendStub{},
		Tracker:          &noopTracker{},
		JITEnabled:       false, // JIT off…
		LoginSyncEnabled: true,  // …but login-sync on
		KnownRoles:       map[auth.Role]bool{auth.RoleReader: true, auth.RoleWriter: true, auth.RoleAdmin: true},
		JITClaimsMapping: map[string][]config.ClaimMapping{
			"https://issuer": {{Claim: "roles", Value: "x", Role: "admln"}}, // typo
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown role")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestNewIdentityStore_ValidatesMappingRoles_WhenLoginSyncOnAndJITOff`
Expected: FAIL — `unknown field LoginSyncEnabled` (compile error) or no error returned.

- [ ] **Step 3: Add the field, config, validation gate, and assembly**

In `internal/auth/identitystore.go`:

Add to the `pgIdentityStore` struct (after `jitEmailAllowlist`):

```go
	loginSyncEnabled bool
```

Add to `IdentityStoreConfig` (after `JITEmailDomainAllowlist`):

```go
	// LoginSyncEnabled turns on metadata + role re-evaluation on interactive
	// login (see loginsync.go). When true, claims-mapping roles are validated
	// at startup even if JITEnabled is false.
	LoginSyncEnabled bool
```

Change the validation gate condition from:

```go
	if cfg.JITEnabled && len(cfg.KnownRoles) > 0 {
```

to:

```go
	if (cfg.JITEnabled || cfg.LoginSyncEnabled) && len(cfg.KnownRoles) > 0 {
```

Add to the returned struct literal (after `jitEmailAllowlist: allowlist,`):

```go
		loginSyncEnabled:   cfg.LoginSyncEnabled,
```

In `cmd/specgraph/serve.go`, add to the `auth.IdentityStoreConfig{...}` literal (after `JITEmailDomainAllowlist`):

```go
		LoginSyncEnabled:        cfg.Auth.OIDC.SyncOnLogin,
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -run TestNewIdentityStore_ValidatesMappingRoles_WhenLoginSyncOnAndJITOff`
Expected: PASS.

- [ ] **Step 5: Verify everything builds**

Run: `go build ./...`
Expected: no output.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/identitystore.go cmd/specgraph/serve.go internal/auth/identitystore_test.go
git commit -s -m "feat(auth): wire loginSyncEnabled and validate mapping roles when on"
```

---

## Task 6: Implement `applyLoginSync` and hook into `resolveJWT`

**Files:**

- Modify: `internal/auth/loginsync.go` (add the method)
- Modify: `internal/auth/identitystore.go` (`resolveJWT` existing-binding branch — after the soft-delete check, before the `return &Identity{` at ~459)
- Test: `internal/auth/loginsync_internal_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/auth/loginsync_internal_test.go` (white-box, `package auth`;
reuses the local `loginSyncFakeBackend`/`loginSyncTracker`/`rawArr` from Task 4):

```go
func newSyncStore(t *testing.T, fake loginSyncFakeBackend, mappings map[string][]config.ClaimMapping, allowlist []string) *pgIdentityStore {
	t.Helper()
	r, err := NewIdentityStore(IdentityStoreConfig{
		Users:                   fake,
		Tracker:                 loginSyncTracker{},
		LoginSyncEnabled:        true,
		KnownRoles:              map[Role]bool{RoleReader: true, RoleWriter: true, RoleAdmin: true},
		JITDefaultRole:          RoleReader,
		JITClaimsMapping:        mappings,
		JITEmailDomainAllowlist: allowlist,
	})
	require.NoError(t, err)
	return r.(*pgIdentityStore)
}

func TestApplyLoginSync_PromotesAndRefreshesMetadata(t *testing.T) {
	var gotRole, gotName, gotEmail string
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, dn, em, role string) error {
		gotName, gotEmail, gotRole = dn, em, role
		return nil
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{
		"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}},
	}, nil)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "old@x.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "new@x.io", Name: "Ada",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.admin")}}

	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
	require.Equal(t, "admin", string(out.Role))
	require.Equal(t, "admin", gotRole)
	require.Equal(t, "Ada", gotName) // DisplayName updated (stored was == subject)
	require.Equal(t, "new@x.io", gotEmail)
}

func TestApplyLoginSync_PreservesOperatorRename(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, dn, _, _ string) error {
		require.Equal(t, "Operator Set Name", dn) // unchanged because stored != subject
		return nil
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "x", Role: "admin"}}}, nil)
	user := &storage.User{ID: "u1", DisplayName: "Operator Set Name", Email: "e@x.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@x.io", Name: "Token Name",
		Raw: map[string]json.RawMessage{"roles": rawArr("x")}}
	_, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
}

func TestApplyLoginSync_NoOpSkipsWrite(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		t.Fatal("UpdateUserOnLogin must not be called on a no-op login")
		return nil
	}}
	s := newSyncStore(t, fake, nil, nil) // no mappings -> rule 1 unchanged
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "admin"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@x.io", Name: "", Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
	require.Equal(t, "admin", string(out.Role))
}

func TestApplyLoginSync_DemotionPersistFailure_Denies(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}}}, nil)
	// current admin, token no longer grants admin -> rule 3 demote to reader -> persist fails -> deny.
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "admin"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@x.io",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.other")}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.Error(t, err)
	require.Nil(t, out)
	require.ErrorIs(t, err, ErrTransient)
}

func TestApplyLoginSync_PromotionPersistFailure_BestEffort(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}}}, nil)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@x.io",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.admin")}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)                       // login proceeds
	require.Equal(t, "reader", string(out.Role)) // at the OLD lower role
}

func TestApplyLoginSync_MetadataOnlyFailure_BestEffort(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, nil, nil) // no mappings -> role unchanged (changed=false)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "old@x.io", Role: "admin"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "new@x.io", // email changed, role not
		Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err) // metadata-only write failure must NOT deny
	require.Equal(t, "admin", string(out.Role))
}

func TestApplyLoginSync_AllowlistMiss_Denies(t *testing.T) {
	s := newSyncStore(t, loginSyncFakeBackend{}, nil, []string{"allowed.io"})
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@blocked.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@blocked.io", Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.Nil(t, out)
}

func TestApplyLoginSync_AllowlistSkippedOnAbsentEmail(t *testing.T) {
	// no mappings + no metadata change -> no write expected, just must not deny.
	s := newSyncStore(t, loginSyncFakeBackend{}, nil, []string{"allowed.io"})
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@allowed.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "", Raw: map[string]json.RawMessage{}} // no email claim
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err) // absent email skips the allowlist re-check
	require.Equal(t, "reader", string(out.Role))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/auth/ -run TestApplyLoginSync`
Expected: FAIL — `s.applyLoginSync undefined`.

- [ ] **Step 3: Implement `applyLoginSync`**

Append to `internal/auth/loginsync.go` (add `"context"`, `"fmt"`, `"log/slog"`, and `storage "github.com/specgraph/specgraph/internal/storage"` to imports):

```go
// applyLoginSync re-enforces the email allowlist, refreshes profile metadata,
// and re-derives the role from the issuer just authenticated for an existing
// OIDC user on interactive login. Returns the user to surface as the resolved
// Identity, or an error that DENIES the login.
//
// Error model (classification is gated on `changed` FIRST):
//   - allowlist domain miss (present email)   -> deny (ErrUnauthenticated)
//   - changed && !isPromotion, persist fails  -> deny (ErrTransient), fail closed
//   - changed && isPromotion, persist fails   -> best-effort, old role
//   - !changed (metadata-only), persist fails -> best-effort
func (s *pgIdentityStore) applyLoginSync(ctx context.Context, claims *OIDCClaims, user *storage.User) (*storage.User, error) {
	// 1. Re-enforce the email-domain allowlist. Skip when the token carries no
	// email claim — an existing binding already passed at bind time, and some
	// providers intermittently omit email.
	if len(s.jitEmailAllowlist) > 0 && claims.Email != "" {
		domain := emailDomain(claims.Email)
		if domain == "" || !s.jitEmailAllowlist[domain] {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync refused — email domain not in allowlist",
				slog.String("issuer", claims.Issuer), slog.String("domain", domain))
			return nil, ErrUnauthenticated
		}
	}

	// 2. Compute the new role and metadata.
	newRole, changed := resolveLoginRole(s.jitClaimsMapping[claims.Issuer], claims.Raw, user.Role, string(s.jitDefaultRole))

	newDisplay := user.DisplayName
	if user.DisplayName == claims.Subject && claims.Name != "" {
		newDisplay = claims.Name // update only if never renamed by an operator
	}
	newEmail := user.Email
	if claims.Email != "" {
		newEmail = claims.Email
	}

	// 3. No-op skip: nothing to persist.
	if newDisplay == user.DisplayName && newEmail == user.Email && newRole == user.Role {
		return user, nil
	}

	// 4. Persist (single atomic UPDATE).
	if err := s.users.UpdateUserOnLogin(ctx, user.ID, newDisplay, newEmail, newRole); err != nil {
		// 5. Classify on `changed` first.
		if !changed {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync metadata update failed (proceeding)",
				slog.String("user_id", user.ID), slog.Any("error", err))
			return user, nil // best-effort
		}
		if isPromotion(user.Role, newRole) {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync promotion persist failed (proceeding at old role)",
				slog.String("user_id", user.ID), slog.String("old", user.Role), slog.String("new", newRole), slog.Any("error", err))
			return user, nil // best-effort, OLD (lower) role
		}
		slog.LogAttrs(ctx, slog.LevelError, "auth: login-sync demotion persist failed — denying login",
			slog.Bool("audit", true), slog.String("user_id", user.ID), slog.String("subject", claims.Subject),
			slog.String("issuer", claims.Issuer), slog.String("old", user.Role), slog.String("new", newRole), slog.Any("error", err))
		return nil, fmt.Errorf("%w: login-sync demotion persist failed", ErrTransient) // fail closed
	}

	// Success: mutate the returned user from the persisted values; audit role changes.
	if changed {
		slog.LogAttrs(ctx, slog.LevelInfo, "auth: login-sync role change",
			slog.Bool("audit", true), slog.String("user_id", user.ID), slog.String("subject", claims.Subject),
			slog.String("issuer", claims.Issuer), slog.String("old", user.Role), slog.String("new", newRole))
	}
	user.DisplayName = newDisplay
	user.Email = newEmail
	user.Role = newRole
	return user, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/auth/ -run TestApplyLoginSync`
Expected: PASS (all subtests).

- [ ] **Step 5: Hook into `resolveJWT`**

In `internal/auth/identitystore.go`, in the existing-binding branch, insert immediately **after** the `user.DeletedAt != nil` soft-delete block (right before `return &Identity{...}` at ~459):

```go
	if s.loginSyncEnabled && InteractiveLoginFromContext(ctx) {
		synced, syncErr := s.applyLoginSync(ctx, claims, user)
		if syncErr != nil {
			return nil, syncErr // deny: allowlist miss or failed demotion
		}
		user = synced
	}
```

- [ ] **Step 6: Verify the gate is correct by inspection + existing coverage**

The two-line gate (`if s.loginSyncEnabled && InteractiveLoginFromContext(ctx)`) is exercised end-to-end by the existing OIDC resolve integration tests, which mint real signed tokens (`internal/auth/identitystore_integration_test.go`, build tag `//go:build integration`). Add one assertion there: in the existing interactive-login test path, after a second login through the same binding, assert the user's role/metadata reflect the current token (sync ran); in a non-interactive bearer-resolve test, assert the role is unchanged (sync did not run). Reuse that file's existing token-minting helpers — do **not** write a new signed-token unit test in `loginsync_internal_test.go` (token verification needs the test JWKS harness those integration tests already set up).

Run: `go test -tags integration ./internal/auth/ -run Integration` (Docker/JWKS harness as the existing tests require)
Expected: PASS.

- [ ] **Step 7: Run the full auth package unit tests**

Run: `go test ./internal/auth/`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/auth/loginsync.go internal/auth/loginsync_internal_test.go internal/auth/identitystore.go internal/auth/identitystore_integration_test.go
git commit -s -m "feat(auth): apply login-sync (metadata + role) on interactive OIDC login"
```

---

## Task 7: Defensive comment on `WithInteractiveLogin`

**Files:**

- Modify: `internal/auth/context.go` (~71-85)

- [ ] **Step 1: Add the comment**

In `internal/auth/context.go`, expand the doc comment on `WithInteractiveLogin`:

```go
// WithInteractiveLogin marks the context as originating from the interactive
// OIDC browser/CLI login flow (the auth_oidc_handler callback). It is the SOLE
// gate distinguishing a login event from a per-request bearer JWT. Login-sync
// (role + metadata re-evaluation) fires only when this is set.
//
// SECURITY: do NOT call this on any per-request/bearer/API-key/MCP code path. A
// single misplaced caller would let an ordinary request mutate a user's role.
// The only production caller is internal/server/auth_oidc_handler.go.
func WithInteractiveLogin(ctx context.Context) context.Context {
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/auth/`
Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/context.go
git commit -s -m "docs(auth): warn WithInteractiveLogin is the sole login-sync gate"
```

---

## Task 8: Documentation

**Files:**

- Modify: `site/docs/guides/oidc-login.md`

- [ ] **Step 1: Add an "App roles and login-sync" section**

Append a section to `site/docs/guides/oidc-login.md` covering (prose, scaled to fit the existing doc style):

- **Prefer app roles over `groups`.** Entra caps inline group memberships (the "groups overage" pattern), so `claims_mapping` on `groups` silently fails for users in many groups. App roles are emitted inline in the `roles` claim and are never subject to overage. Target `claim: "roles"` with the app role's **Value** string.
- Example config block (copy the example from the design doc's "Example operator config").
- **`sync_on_login` (default `true`):** on each interactive login, `DisplayName`/`Email` are refreshed and the role is re-derived from the login issuer's `claims_mapping`. Set `false` to keep the legacy JIT-only behavior.
- **Demotion semantics:** if a mapping is configured and no rule matches, the user is set to `default_role` on next login through that issuer. Losing an app role demotes you.
- **Migration warning:** before enabling against existing deployments, re-target rules from `groups` to `roles`, verify every `Value` exactly (case-sensitive), assign app roles to admins, and keep the bootstrap admin key — otherwise a misconfig can demote all OIDC admins. To *revoke* via app roles keep ≥1 rule (deleting the whole block freezes roles).
- **Requirements/caveats:** `Value`s must be strings (numeric claims never match); order rules most-privileged-first (first match wins); the issuer **must be tenant-pinned** (not `common`/`organizations`).
- **MCP boundary:** role changes reach MCP/API-key usage only after the user next logs in interactively (web dashboard or `specgraph login`); app-key-paste web sessions never sync.

- [ ] **Step 2: Lint the docs**

Run: `task lint` (or `rumdl check site/docs/guides/oidc-login.md` if you only want the markdown linter)
Expected: no errors for the edited file.

- [ ] **Step 3: Commit**

```bash
git add site/docs/guides/oidc-login.md
git commit -s -m "docs(oidc): document app-roles and sync_on_login"
```

---

## Final verification

- [ ] **Step 1: Run the quality gate**

Run: `task check`
Expected: fmt/license/lint/build/unit-tests all pass.

- [ ] **Step 2: Run the Postgres integration test for the new storage method (Docker)**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestAuthStore_UpdateUserOnLogin`
Expected: PASS.

- [ ] **Step 3: Full pre-PR pipeline (Docker)**

Run: `task pr-prep`
Expected: check + integration + e2e all pass.

---

## Notes for the implementer

- **No DB migration is required** — `UpdateUserOnLogin` writes existing `users` columns (`display_name`, `email`, `role`). The single-issuer design deliberately avoids a new column.
- **`Role` is a `string` type** (`auth.go:8`): use `string(role)` / `Role(s)` to convert; there is no `.s()` method.
- **Test packaging:** unexported helpers (`nameFromClaims`, `resolveLoginRole`, `isPromotion`, `applyLoginSync`) are tested **white-box** in `*_internal_test.go` files (`package auth`), following the existing `clampedrole_internal_test.go` convention. These files define their **own** local fakes (`loginSyncFakeBackend`, `loginSyncTracker`) because the existing `usersBackendStub`/`noopTracker` live in `package auth_test` and aren't visible from `package auth`. The interface-growth stub updates in Task 3 are still required so `package auth_test` keeps compiling.
- **`applyClaimsMapping`, `matchClaimValue`, `roleLessThan`, `emailDomain`** already exist in `identitystore.go` — reuse, do not duplicate.
- **DCO:** every commit needs `Signed-off-by:` — use `git commit -s` (shown above) or `jj describe` with the trailer.
- **jj-colocated repo:** if using jj, use `jj --no-pager` and `-m`; never `git push` (use `jj bookmark set` + `jj git push --bookmark`).
