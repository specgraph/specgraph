# Self-Service MCP API-Key Provisioning (spgr-g7st) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a non-admin user mint, list, rotate, and revoke their **own** role-capped, expiring MCP API keys via CLI and web — without an admin and without the bootstrap admin key.

**Architecture:** Four new self-scoped `IdentityService` RPCs (`CreateMyAPIKey`/`ListMyAPIKeys`/`RotateMyAPIKey`/`RevokeMyAPIKey`) gated by a new Cedar `apikey.self` verb (permitted for any authenticated principal). Handlers derive the user strictly from the request context, reject `Source=="apikey"` callers (anti key-chaining), and floor every minted key's `role_downgrade` at the caller's `EffectiveRole` (anti-laundering). New owner-scoped, quota-guarded storage methods enforce ownership and a per-user active-key cap in-query. No DB migration.

**Tech Stack:** Go, ConnectRPC, protobuf (buf), Cedar (`cedar-go`), pgx v5 / PostgreSQL, cobra (CLI), SvelteKit (web). Tests: Go `testing`, testcontainers (postgres integration).

**Spec:** `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` (rev 5)

**Conventions (from AGENTS.md):**

- All `.go`/`.proto` files need `// SPDX-License-Identifier: Apache-2.0` + `// Copyright 2026 Sean Brandt` headers. New packages need a `// Package` doc comment.
- Never edit `gen/` by hand — edit `.proto` and run `task proto`.
- Assert on connect error **codes**, never message strings. Mock backends return storage sentinel errors (`errors.Is`-compatible).
- `task check` before pushing. Commit with `git commit -s` (DCO sign-off required).
- Use 4-backtick fences for nested code blocks in docs.

---

## File Map

| File | Responsibility | Action |
|------|----------------|--------|
| `proto/specgraph/v1/identity.proto` | 4 new RPCs + request/response messages | Modify |
| `internal/auth/role.go` | new exported `RoleMin` helper | Create |
| `internal/auth/engine.go` | add `"self"` to `knownVerbs` | Modify |
| `internal/auth/actions.go` | map 4 procedures → `apikey.self` | Modify |
| `internal/auth/actions_test.go` | mirror map + `self`-verb-only-on-apikey drift test | Modify |
| `internal/auth/policies/base.cedar` | `apikey.self` permit + handler-restriction comment | Modify |
| `internal/storage/users.go` | `UsersBackend`: 3 new methods + quota sentinel | Modify |
| `internal/storage/errors.go` | `ErrAPIKeyQuotaExceeded` sentinel | Modify |
| `internal/storage/postgres/users.go` | owner-scoped + quota-guarded impls | Modify |
| `internal/auth/usersbackend_stub_test.go` | add stub methods | Modify |
| `internal/server/usersbackend_stub_test.go` | add stub methods | Modify |
| `internal/server/identity_handler.go` | 4 handlers + defaults consts | Modify |
| `internal/server/identity_selfkey_test.go` | handler unit tests | Create |
| `internal/storage/postgres/users_selfkey_test.go` | storage integration tests | Create |
| `cmd/specgraph/auth_apikey.go` | self-mint CLI (no `--user`), session-cred precedence | Modify |
| `web/src/lib/auth.svelte.ts` / `web/src/routes/keys/+page.svelte` | MCP Keys panel | Create/Modify |

> Implement tasks in order. Tasks 1–8 produce the working, testable backend; Task 9 the CLI; Task 10 the web UI; Task 11 the postgres integration tests.

---

### Task 1: Proto — add the four self-scoped RPCs

**Files:**

- Modify: `proto/specgraph/v1/identity.proto`

- [ ] **Step 1: Update the service doc comment + add RPCs**

In `proto/specgraph/v1/identity.proto`, change the service comment (lines 12-15) and add four RPCs after the existing API-key block (after line 29):

```proto
// IdentityService manages users, service accounts, API keys, and OIDC
// bindings. It is a GLOBAL service (not project-scoped). Admin-management RPCs
// require the admin role (Cedar "manage" verb). Whoami and the *MyAPIKey RPCs
// are self-scoped (Cedar "self" verb): any authenticated principal may act,
// but only on their OWN identity/keys.
service IdentityService {
  rpc Whoami(WhoamiRequest) returns (WhoamiResponse);

  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc CreateServiceAccount(CreateServiceAccountRequest) returns (CreateServiceAccountResponse);
  rpc UpdateUserRole(UpdateUserRoleRequest) returns (UpdateUserRoleResponse);
  rpc SoftDeleteUser(SoftDeleteUserRequest) returns (SoftDeleteUserResponse);
  rpc PurgeUser(PurgeUserRequest) returns (PurgeUserResponse);

  rpc CreateAPIKey(CreateAPIKeyRequest) returns (CreateAPIKeyResponse);
  rpc RevokeAPIKey(RevokeAPIKeyRequest) returns (RevokeAPIKeyResponse);
  rpc RotateAPIKey(RotateAPIKeyRequest) returns (RotateAPIKeyResponse);
  rpc ListAPIKeys(ListAPIKeysRequest) returns (ListAPIKeysResponse);

  // Self-scoped API-key management (Cedar "apikey.self"). The caller's user_id
  // is taken from the authenticated context; these RPCs never accept a target
  // user. See spec 2026-06-16-spgr-g7st.
  rpc CreateMyAPIKey(CreateMyAPIKeyRequest) returns (CreateMyAPIKeyResponse);
  rpc ListMyAPIKeys(ListMyAPIKeysRequest) returns (ListMyAPIKeysResponse);
  rpc RotateMyAPIKey(RotateMyAPIKeyRequest) returns (RotateMyAPIKeyResponse);
  rpc RevokeMyAPIKey(RevokeMyAPIKeyRequest) returns (RevokeMyAPIKeyResponse);

  rpc ListOIDCBindings(ListOIDCBindingsRequest) returns (ListOIDCBindingsResponse);
  rpc UnbindOIDC(UnbindOIDCRequest) returns (UnbindOIDCResponse);
}
```

- [ ] **Step 2: Add the message definitions**

Append after the `ListAPIKeysResponse` message (after line 168):

```proto
// CreateMyAPIKey mints a key for the AUTHENTICATED caller. There is no user_id
// field by design. role_downgrade (if set) is further floored at the caller's
// effective role server-side. expires_at is optional; unset = default TTL, and
// any value is capped server-side.
message CreateMyAPIKeyRequest {
  string label = 1;
  string role_downgrade = 2;                 // empty = inherit caller's role (still floored)
  google.protobuf.Timestamp expires_at = 3;  // unset = default TTL; capped server-side
}
message CreateMyAPIKeyResponse {
  APIKey key = 1;
  string plaintext = 2;  // full token, shown ONCE
}

message ListMyAPIKeysRequest {
  bool include_revoked = 1;
  int32 limit = 2;
  int32 offset = 3;
}
message ListMyAPIKeysResponse { repeated APIKey keys = 1; }

message RotateMyAPIKeyRequest {
  string key_id = 1;
  google.protobuf.Timestamp expires_at = 2;  // unset = default TTL; capped server-side
}
message RotateMyAPIKeyResponse {
  APIKey key = 1;
  string plaintext = 2;
}

message RevokeMyAPIKeyRequest { string key_id = 1; }
message RevokeMyAPIKeyResponse {}
```

- [ ] **Step 3: Regenerate Go code**

Run: `task proto`
Expected: regenerates `gen/specgraph/v1/*.pb.go` and `*.connect.go`; new procedures `IdentityServiceCreateMyAPIKeyProcedure` etc. appear. No errors.

- [ ] **Step 4: Verify build**

Run: `go build ./gen/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add proto/specgraph/v1/identity.proto gen/
git commit -s -m "feat(proto): add self-scoped *MyAPIKey RPCs to IdentityService"
```

---

### Task 2: `auth.RoleMin` exported helper

**Files:**

- Create: `internal/auth/role.go`
- Test: `internal/auth/role_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/auth/role_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "testing"

func TestRoleMin(t *testing.T) {
	cases := []struct {
		name     string
		a, b     Role
		want     Role
	}{
		{"reader<writer", RoleReader, RoleWriter, RoleReader},
		{"writer<admin", RoleAdmin, RoleWriter, RoleWriter},
		{"equal admin", RoleAdmin, RoleAdmin, RoleAdmin},
		{"unranked a fails closed", Role("custom"), RoleAdmin, RoleReader},
		{"unranked b fails closed", RoleAdmin, Role("custom"), RoleReader},
		{"empty fails closed", Role(""), RoleAdmin, RoleReader},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := RoleMin(c.a, c.b); got != c.want {
				t.Fatalf("RoleMin(%q,%q)=%q want %q", c.a, c.b, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/auth/ -run TestRoleMin`
Expected: FAIL — `undefined: RoleMin`.

- [ ] **Step 3: Write the implementation**

Create `internal/auth/role.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// RoleMin returns the less-privileged of two built-in roles (reader < writer
// < admin). If EITHER role is not a ranked built-in (custom/empty), it fails
// CLOSED to RoleReader — mirroring clampedRole's fail-closed discipline. Used
// to floor a self-minted key's role_downgrade at the caller's effective role
// so a self-service key can never exceed the caller's authority at mint time.
func RoleMin(a, b Role) Role {
	ra, oka := roleRank[a]
	rb, okb := roleRank[b]
	if !oka || !okb {
		return RoleReader
	}
	if ra < rb {
		return a
	}
	return b
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/auth/ -run TestRoleMin`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/role.go internal/auth/role_test.go
git commit -s -m "feat(auth): add exported RoleMin role-floor helper"
```

---

### Task 3: Cedar `apikey.self` verb plumbing

**Files:**

- Modify: `internal/auth/engine.go` (knownVerbs, ~line 155)
- Modify: `internal/auth/actions.go` (map, ~line 104)
- Modify: `internal/auth/policies/base.cedar`
- Modify: `internal/auth/actions_test.go`

- [ ] **Step 1: Write the failing drift test**

Add to `internal/auth/actions_test.go`:

```go
func TestSelfVerbOnlyUsedByApikey(t *testing.T) {
	for procedure, action := range procedureActions {
		_, verb, found := strings.Cut(action, ".")
		_ = found
		if verb == "self" && !strings.HasPrefix(action, "apikey.") {
			t.Errorf("procedure %q uses the 'self' verb on non-apikey action %q; "+
				"the base.cedar 'self' permit is all-principals and must stay apikey-only",
				procedure, action)
		}
	}
}

func TestSelfApikeyProceduresMapped(t *testing.T) {
	want := []string{
		specgraphv1connect.IdentityServiceCreateMyAPIKeyProcedure,
		specgraphv1connect.IdentityServiceListMyAPIKeysProcedure,
		specgraphv1connect.IdentityServiceRotateMyAPIKeyProcedure,
		specgraphv1connect.IdentityServiceRevokeMyAPIKeyProcedure,
	}
	for _, p := range want {
		if got := procedureActions[p]; got != "apikey.self" {
			t.Errorf("procedure %q mapped to %q, want apikey.self", p, got)
		}
	}
}
```

Ensure `strings` and the `specgraphv1connect` import are present in the test file (add if missing).

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/auth/ -run 'TestSelfApikeyProceduresMapped|TestSelfVerbOnlyUsedByApikey'`
Expected: FAIL — procedures map to `""` (not `apikey.self`).

- [ ] **Step 3: Add `"self"` to knownVerbs**

In `internal/auth/engine.go` find `knownVerbs` (~line 155) and add `"self"`:

```go
var knownVerbs = map[string]bool{
	"read":   true,
	"write":  true,
	"delete": true,
	"manage": true,
	"self":   true,
}
```

(If `knownVerbs` is a slice/set literal of a different shape, add `"self"` consistently with the existing entries.)

- [ ] **Step 4: Map the four procedures**

In `internal/auth/actions.go`, in the `procedureActions` map after the `apikey.manage` block (line 104), add:

```go
	specgraphv1connect.IdentityServiceCreateMyAPIKeyProcedure: "apikey.self",
	specgraphv1connect.IdentityServiceListMyAPIKeysProcedure:  "apikey.self",
	specgraphv1connect.IdentityServiceRotateMyAPIKeyProcedure: "apikey.self",
	specgraphv1connect.IdentityServiceRevokeMyAPIKeyProcedure: "apikey.self",
```

- [ ] **Step 5: Add the Cedar permit**

In `internal/auth/policies/base.cedar`, append:

```cedar
// Self-scoped API-key management (IdentityService *MyAPIKey RPCs). Permitted
// for ANY authenticated principal — but the handler enforces the real policy
// that Cedar cannot express today (principalEntity exposes only role/id/email):
//   - rejects callers whose credential Source == "apikey" (anti key-chaining)
//   - floors the minted key's role_downgrade at the caller's effective role
//   - derives user_id from context (never the request)
// The "self" verb must stay apikey-only (TestSelfVerbOnlyUsedByApikey guards this).
permit (
	principal,
	action in SpecGraph::Action::"self",
	resource
) when {
	principal has role &&
	(principal.role == "reader" || principal.role == "writer" || principal.role == "admin")
};
```

- [ ] **Step 6: Run the auth tests**

Run: `go test ./internal/auth/`
Expected: PASS (drift tests, `TestActionNames_AllParseToKnownVerb`, and engine construction all green).

- [ ] **Step 7: Commit**

```bash
git add internal/auth/engine.go internal/auth/actions.go internal/auth/actions_test.go internal/auth/policies/base.cedar
git commit -s -m "feat(auth): add apikey.self Cedar verb for self-scoped key RPCs"
```

---

### Task 4: Storage — owner-scoped + quota-guarded methods

**Files:**

- Modify: `internal/storage/errors.go` (sentinel)
- Modify: `internal/storage/users.go` (interface)
- Modify: `internal/storage/postgres/users.go` (impl)
- Modify: `internal/auth/usersbackend_stub_test.go`, `internal/server/usersbackend_stub_test.go`

- [ ] **Step 1: Add the quota sentinel**

In `internal/storage/errors.go` add alongside the existing API-key sentinels:

```go
// ErrAPIKeyQuotaExceeded is returned when a user has reached their maximum
// number of active (non-revoked, non-expired) API keys.
var ErrAPIKeyQuotaExceeded = errors.New("api key quota exceeded")
```

- [ ] **Step 2: Extend the `UsersBackend` interface**

In `internal/storage/users.go`, in the `// --- API key CRUD ---` section, add:

```go
	// GetAPIKeyForUser returns the key with the given ID IFF it belongs to
	// userID (owner-scoped). Returns ErrAPIKeyNotFound if no such row exists
	// for that user (whether missing, foreign, or revoked). Used by the
	// self-service rotate path to read the old role_downgrade before flooring.
	GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*APIKey, error)

	// CreateAPIKeyWithQuota inserts a new key, enforcing maxActive active
	// (non-revoked, non-expired) keys for k.UserID inside one transaction
	// (parent users-row lock serializes concurrent mints). Returns
	// ErrAPIKeyQuotaExceeded if the user is at/over the cap, ErrUserNotFound
	// if the user is missing/soft-deleted. Behaves like CreateAPIKey otherwise
	// (prefix-collision retry).
	CreateAPIKeyWithQuota(ctx context.Context, k *APIKey, maxActive int) (*APIKey, error)

	// RevokeAPIKeyForUser revokes keyID IFF it belongs to userID. Idempotent:
	// re-revoking the user's own (already-revoked) key is a no-op success.
	// Returns ErrAPIKeyNotFound if no row matches the (id, user_id) pair.
	RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error

	// RotateAPIKeyForUser revokes oldKeyID and mints a successor IFF oldKeyID
	// belongs to userID and is active. Unlike RotateAPIKey, role_downgrade and
	// expires_at are set EXPLICITLY by the caller (never inherited): the
	// self-service handler floors role_downgrade and defaults/caps expiry. The
	// caller supplies phcHash (the new secret's hash). Label is inherited from
	// the old key. Returns ErrAPIKeyNotFound if no active row matches
	// (id, user_id).
	RotateAPIKeyForUser(ctx context.Context, userID, oldKeyID, phcHash, roleDowngrade string, expiresAt *time.Time) (*APIKey, error)
```

- [ ] **Step 3: Implement `GetAPIKeyForUser`**

In `internal/storage/postgres/users.go`, add after `RotateAPIKey` (after line 517):

```go
// GetAPIKeyForUser returns the key IFF it belongs to userID. Owner-scoped:
// missing/foreign/revoked all yield ErrAPIKeyNotFound (no enumeration leak).
func (s *AuthStore) GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*storage.APIKey, error) {
	var k storage.APIKey
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, prefix, coalesce(role_downgrade, ''), coalesce(label, ''),
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE id = $1::uuid AND user_id = $2::uuid`, keyID, userID).
		Scan(&k.ID, &k.UserID, &k.Prefix, &k.RoleDowngrade, &k.Label,
			&k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get api key for user: %w", err)
	}
	return &k, nil
}
```

- [ ] **Step 4: Implement `CreateAPIKeyWithQuota`**

Add:

```go
// CreateAPIKeyWithQuota enforces maxActive active keys per user in one tx.
// It locks the caller's users row (FOR UPDATE) to serialize concurrent mints —
// FOR UPDATE alone cannot block a phantom INSERT at READ COMMITTED, so the
// parent-row lock is the serialization point.
func (s *AuthStore) CreateAPIKeyWithQuota(ctx context.Context, k *storage.APIKey, maxActive int) (*storage.APIKey, error) {
	if k.UserID == "" {
		return nil, errors.New("CreateAPIKeyWithQuota: UserID required")
	}
	if k.PHCHash == "" {
		return nil, errors.New("CreateAPIKeyWithQuota: PHCHash required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	// Lock the parent user row; also enforces existence + not-soft-deleted.
	var locked string
	err = tx.QueryRow(ctx, `
		SELECT id FROM users WHERE id = $1::uuid AND deleted_at IS NULL FOR UPDATE`,
		k.UserID).Scan(&locked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock user: %w", err)
	}

	var active int
	if err = tx.QueryRow(ctx, `
		SELECT count(*) FROM api_keys
		WHERE user_id = $1::uuid AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > $2)`,
		k.UserID, s.now()).Scan(&active); err != nil {
		return nil, fmt.Errorf("count active keys: %w", err)
	}
	if active >= maxActive {
		return nil, storage.ErrAPIKeyQuotaExceeded
	}

	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, perr := s.genPrefix()
		if perr != nil {
			return nil, fmt.Errorf("generate prefix: %w", perr)
		}
		if _, spErr := tx.Exec(ctx, `SAVEPOINT mint_insert`); spErr != nil {
			return nil, fmt.Errorf("savepoint: %w", spErr)
		}
		var id string
		var createdAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			k.UserID, prefix, k.PHCHash, k.RoleDowngrade, k.Label, k.ExpiresAt).
			Scan(&id, &createdAt)
		if err == nil {
			if _, relErr := tx.Exec(ctx, `RELEASE SAVEPOINT mint_insert`); relErr != nil {
				return nil, fmt.Errorf("release savepoint: %w", relErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit tx: %w", commitErr)
			}
			k.ID = id
			k.Prefix = prefix
			k.CreatedAt = createdAt
			return k, nil
		}
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT mint_insert`); rbErr != nil {
			return nil, fmt.Errorf("rollback savepoint: %w", rbErr)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}
```

- [ ] **Step 5: Implement `RevokeAPIKeyForUser`**

Add:

```go
// RevokeAPIKeyForUser revokes keyID IFF owned by userID. COALESCE keeps an
// existing revoked_at (idempotent no-op for an already-revoked own key);
// RowsAffected()==0 means no row matched (id, user_id) → ErrAPIKeyNotFound.
func (s *AuthStore) RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = COALESCE(revoked_at, $1)
		WHERE id = $2::uuid AND user_id = $3::uuid`, s.now(), keyID, userID)
	if err != nil {
		return fmt.Errorf("revoke key for user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrAPIKeyNotFound
	}
	return nil
}
```

- [ ] **Step 6: Implement `RotateAPIKeyForUser`**

Add (mirrors `RotateAPIKey` but owner-scoped, explicit role_downgrade + expiry):

```go
// RotateAPIKeyForUser revokes oldKeyID and mints a successor IFF owned by
// userID and active. role_downgrade, expires_at, and the new secret hash
// (phcHash) are supplied explicitly; label is inherited. Owner mismatch /
// missing / revoked → ErrAPIKeyNotFound.
func (s *AuthStore) RotateAPIKeyForUser(ctx context.Context, userID, oldKeyID, phcHash, roleDowngrade string, expiresAt *time.Time) (*storage.APIKey, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // deferred rollback is a no-op after commit

	var oldLabel string
	err = tx.QueryRow(ctx, `
		SELECT coalesce(label, '') FROM api_keys
		WHERE id = $1::uuid AND user_id = $2::uuid AND revoked_at IS NULL`,
		oldKeyID, userID).Scan(&oldLabel)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("read old key: %w", err)
	}

	if _, err = tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND user_id = $3::uuid AND revoked_at IS NULL`,
		s.now(), oldKeyID, userID); err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}

	return s.insertRotatedKey(ctx, tx, userID, roleDowngrade, oldLabel, expiresAt, phcHash)
}
```

```go
func (s *AuthStore) insertRotatedKey(ctx context.Context, tx pgx.Tx, userID, roleDowngrade, label string, expiresAt *time.Time, phcHash string) (*storage.APIKey, error) {
	for attempt := 0; attempt < maxPrefixRetries; attempt++ {
		prefix, perr := s.genPrefix()
		if perr != nil {
			return nil, fmt.Errorf("generate prefix: %w", perr)
		}
		if _, spErr := tx.Exec(ctx, `SAVEPOINT rotate_insert`); spErr != nil {
			return nil, fmt.Errorf("savepoint: %w", spErr)
		}
		var id string
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			userID, prefix, phcHash, roleDowngrade, label, expiresAt).Scan(&id, &createdAt)
		if err == nil {
			if _, relErr := tx.Exec(ctx, `RELEASE SAVEPOINT rotate_insert`); relErr != nil {
				return nil, fmt.Errorf("release savepoint: %w", relErr)
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				return nil, fmt.Errorf("commit tx: %w", commitErr)
			}
			return &storage.APIKey{
				ID: id, UserID: userID, Prefix: prefix, PHCHash: phcHash,
				RoleDowngrade: roleDowngrade, Label: label, ExpiresAt: expiresAt, CreatedAt: createdAt,
			}, nil
		}
		if _, rbErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT rotate_insert`); rbErr != nil {
			return nil, fmt.Errorf("rollback savepoint: %w", rbErr)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert rotated key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}
```

Adjust the `insertRotatedKey` helper signature to match the call above (`phcHash` last param), and ensure `RotateAPIKeyForUser` returns its result.

- [ ] **Step 7: Add the three methods to BOTH stub files**

In `internal/server/usersbackend_stub_test.go` and `internal/auth/usersbackend_stub_test.go`, add func fields and methods. For `internal/server/usersbackend_stub_test.go` add fields:

```go
	getAPIKeyForUser     func(ctx context.Context, userID, keyID string) (*storage.APIKey, error)
	createAPIKeyWithQuota func(ctx context.Context, k *storage.APIKey, maxActive int) (*storage.APIKey, error)
	revokeAPIKeyForUser  func(ctx context.Context, userID, keyID string) error
	rotateAPIKeyForUser  func(ctx context.Context, userID, oldKeyID, phcHash, roleDowngrade string, expiresAt *time.Time) (*storage.APIKey, error)
```

and methods (mutating ones fail loud, the read defaults to NotFound):

```go
func (s *usersBackendStub) GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*storage.APIKey, error) {
	if s.getAPIKeyForUser != nil {
		return s.getAPIKeyForUser(ctx, userID, keyID)
	}
	return nil, storage.ErrAPIKeyNotFound
}
func (s *usersBackendStub) CreateAPIKeyWithQuota(ctx context.Context, k *storage.APIKey, maxActive int) (*storage.APIKey, error) {
	if s.createAPIKeyWithQuota != nil {
		return s.createAPIKeyWithQuota(ctx, k, maxActive)
	}
	return nil, errUnexpected("CreateAPIKeyWithQuota")
}
func (s *usersBackendStub) RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error {
	if s.revokeAPIKeyForUser != nil {
		return s.revokeAPIKeyForUser(ctx, userID, keyID)
	}
	return errUnexpected("RevokeAPIKeyForUser")
}
func (s *usersBackendStub) RotateAPIKeyForUser(ctx context.Context, userID, oldKeyID, phcHash, roleDowngrade string, expiresAt *time.Time) (*storage.APIKey, error) {
	if s.rotateAPIKeyForUser != nil {
		return s.rotateAPIKeyForUser(ctx, userID, oldKeyID, phcHash, roleDowngrade, expiresAt)
	}
	return nil, errUnexpected("RotateAPIKeyForUser")
}
```

Add the equivalent to `internal/auth/usersbackend_stub_test.go` (check its field/struct style and match it; add `"time"` import if needed).

- [ ] **Step 8: Verify build**

Run: `go build ./... && go vet ./internal/storage/... ./internal/server/... ./internal/auth/...`
Expected: PASS (both `var _ storage.UsersBackend` assertions satisfied).

- [ ] **Step 9: Commit**

```bash
git add internal/storage/ internal/auth/usersbackend_stub_test.go internal/server/usersbackend_stub_test.go
git commit -s -m "feat(storage): owner-scoped + quota-guarded self-service key methods"
```

---

### Task 5: Handler — `CreateMyAPIKey`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Test: `internal/server/identity_selfkey_test.go`

- [ ] **Step 1: Add defaults + shared helpers (no test yet)**

In `internal/server/identity_handler.go`, add package-level consts and helpers:

```go
// Self-service key policy defaults. Server-configurable later; constants for v1.
const (
	defaultSelfKeyTTL  = 90 * 24 * time.Hour
	maxSelfKeyTTL      = 365 * 24 * time.Hour
	maxActiveSelfKeys  = 10
)

// callerOrUnauth returns the authenticated identity or an Unauthenticated error.
func callerOrUnauth(ctx context.Context) (*auth.Identity, error) {
	id, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no identity in context"))
	}
	return id, nil
}

// requireSelfServiceCaller rejects API-key-sourced callers (anti key-chaining;
// also excludes service accounts, which authenticate only via API keys).
func requireSelfServiceCaller(id *auth.Identity) error {
	if id.Source == "apikey" {
		return connect.NewError(connect.CodePermissionDenied,
			errors.New("self-service key management requires an interactive login, not an API key"))
	}
	return nil
}

// selfKeyExpiry computes a mandatory, capped expiry. nil request → default TTL.
func (h *IdentityHandler) selfKeyExpiry(reqTS *timestamppb.Timestamp) (time.Time, error) {
	now := time.Now()
	if reqTS == nil {
		return now.Add(defaultSelfKeyTTL), nil
	}
	t := reqTS.AsTime()
	if !t.After(now) {
		return time.Time{}, connect.NewError(connect.CodeInvalidArgument, errors.New("expires_at must be in the future"))
	}
	if t.After(now.Add(maxSelfKeyTTL)) {
		return time.Time{}, connect.NewError(connect.CodeInvalidArgument,
			errors.New("expires_at exceeds the maximum self-service key lifetime (365d)"))
	}
	return t, nil
}
```

Add imports: `"time"`, `"google.golang.org/protobuf/types/known/timestamppb"`.

- [ ] **Step 2: Write the failing handler tests**

Create `internal/server/identity_selfkey_test.go` with cases (uses the existing test harness pattern — a stub + `auth.WithIdentity(ctx, ...)`):

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

func selfCtx(role, source string) context.Context {
	return auth.WithIdentity(context.Background(), &auth.Identity{
		UserID: "u1", Role: auth.Role(role), EffectiveRole: auth.Role(role), Source: source,
	})
}

func TestCreateMyAPIKey_RejectsApikeySource(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{}, logger: testLogger()}
	_, err := h.CreateMyAPIKey(selfCtx("writer", "apikey"),
		connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", err)
	}
}

func TestCreateMyAPIKey_FloorsRoleAndUserFromContext(t *testing.T) {
	var captured *storage.APIKey
	var capMax int
	h := &IdentityHandler{users: &usersBackendStub{
		createAPIKeyWithQuota: func(_ context.Context, k *storage.APIKey, maxActive int) (*storage.APIKey, error) {
			captured, capMax = k, maxActive
			k.ID, k.Prefix = "k1", "abcd1234"
			return k, nil
		},
	}, logger: testLogger()}
	// writer caller requests an (escalating) admin downgrade → must be floored to writer.
	_, err := h.CreateMyAPIKey(selfCtx("writer", "oidc"),
		connect.NewRequest(&specv1.CreateMyAPIKeyRequest{RoleDowngrade: "admin", Label: "mcp"}))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if captured.UserID != "u1" {
		t.Errorf("user_id from context wanted u1, got %q", captured.UserID)
	}
	if captured.RoleDowngrade != "writer" {
		t.Errorf("role_downgrade floored to writer expected, got %q", captured.RoleDowngrade)
	}
	if captured.ExpiresAt == nil || captured.ExpiresAt.Before(time.Now().Add(80*24*time.Hour)) {
		t.Errorf("expected default ~90d expiry, got %v", captured.ExpiresAt)
	}
	if capMax != maxActiveSelfKeys {
		t.Errorf("quota wanted %d, got %d", maxActiveSelfKeys, capMax)
	}
}

func TestCreateMyAPIKey_QuotaExceeded(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{
		createAPIKeyWithQuota: func(context.Context, *storage.APIKey, int) (*storage.APIKey, error) {
			return nil, storage.ErrAPIKeyQuotaExceeded
		},
	}, logger: testLogger()}
	_, err := h.CreateMyAPIKey(selfCtx("reader", "oidc"),
		connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("want ResourceExhausted, got %v", err)
	}
}
```

> If a `testLogger()` helper or `auth.WithIdentity` exported constructor does not exist, use the existing test pattern in `identity_handler_test.go` (check how it injects identity and a logger) and match it. `auth.WithIdentity` is the interceptor's context setter — confirm its exported name in `internal/auth/context.go`.

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/server/ -run TestCreateMyAPIKey`
Expected: FAIL — `CreateMyAPIKey` undefined.

- [ ] **Step 4: Implement `CreateMyAPIKey`**

Add to `internal/server/identity_handler.go`:

```go
// CreateMyAPIKey mints an API key for the authenticated caller. The user is
// taken from context (never the request). role_downgrade is floored at the
// caller's effective role; expiry is mandatory and capped; a quota limits
// active keys per user. The plaintext is returned exactly once.
func (h *IdentityHandler) CreateMyAPIKey(ctx context.Context, req *connect.Request[specv1.CreateMyAPIKeyRequest]) (*connect.Response[specv1.CreateMyAPIKeyResponse], error) {
	id, err := callerOrUnauth(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireSelfServiceCaller(id); err != nil {
		return nil, err
	}
	if d := req.Msg.GetRoleDowngrade(); d != "" && !auth.IsBuiltinRole(auth.Role(d)) {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("role_downgrade must be one of: reader, writer, admin"))
	}

	// Floor: inherit→caller's effective role; otherwise the lesser of the two.
	ceiling := id.EffectiveRole
	downgrade := ceiling
	if d := auth.Role(req.Msg.GetRoleDowngrade()); d != "" {
		downgrade = auth.RoleMin(d, ceiling)
	}

	expiresAt, err := h.selfKeyExpiry(req.Msg.GetExpiresAt())
	if err != nil {
		return nil, err
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateMyAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	created, err := h.users.CreateAPIKeyWithQuota(ctx, &storage.APIKey{
		UserID:        id.UserID,
		PHCHash:       phc,
		Label:         req.Msg.GetLabel(),
		RoleDowngrade: string(downgrade),
		ExpiresAt:     &expiresAt,
	}, maxActiveSelfKeys)
	if err != nil {
		if errors.Is(err, storage.ErrAPIKeyQuotaExceeded) {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				errors.New("active key quota reached; revoke an existing key first"))
		}
		return nil, h.identityError(ctx, err)
	}

	return connect.NewResponse(&specv1.CreateMyAPIKeyResponse{
		Key:       apiKeyToProto(created),
		Plaintext: auth.FormatAPIKeyToken(created.Prefix, secret),
	}), nil
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/server/ -run TestCreateMyAPIKey`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_selfkey_test.go
git commit -s -m "feat(server): CreateMyAPIKey self-service handler (floor + quota + caps)"
```

---

### Task 6: Handler — `ListMyAPIKeys`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Test: `internal/server/identity_selfkey_test.go`

- [ ] **Step 1: Write the failing test**

Append to `identity_selfkey_test.go`:

```go
func TestListMyAPIKeys_HardSetsUserID(t *testing.T) {
	var gotFilter storage.ListAPIKeysFilter
	h := &IdentityHandler{users: &usersBackendStub{
		listAPIKeys: func(_ context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			gotFilter = f
			return nil, nil
		},
	}, logger: testLogger()}
	_, err := h.ListMyAPIKeys(selfCtx("reader", "oidc"),
		connect.NewRequest(&specv1.ListMyAPIKeysRequest{IncludeRevoked: true}))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotFilter.UserID != "u1" {
		t.Fatalf("UserID must be hard-set from context to u1, got %q", gotFilter.UserID)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/server/ -run TestListMyAPIKeys`
Expected: FAIL — `ListMyAPIKeys` undefined.

- [ ] **Step 3: Implement**

```go
// ListMyAPIKeys lists the authenticated caller's own keys. UserID is taken
// from context; an empty UserID would list ALL users' keys, so this is a
// load-bearing invariant.
func (h *IdentityHandler) ListMyAPIKeys(ctx context.Context, req *connect.Request[specv1.ListMyAPIKeysRequest]) (*connect.Response[specv1.ListMyAPIKeysResponse], error) {
	id, err := callerOrUnauth(ctx)
	if err != nil {
		return nil, err
	}
	keys, err := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{
		UserID:         id.UserID, // never from the request
		IncludeRevoked: req.Msg.GetIncludeRevoked(),
		Limit:          int(req.Msg.GetLimit()),
		Offset:         int(req.Msg.GetOffset()),
	})
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	out := make([]*specv1.APIKey, 0, len(keys))
	for _, k := range keys {
		out = append(out, apiKeyToProto(k))
	}
	return connect.NewResponse(&specv1.ListMyAPIKeysResponse{Keys: out}), nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -run TestListMyAPIKeys`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_selfkey_test.go
git commit -s -m "feat(server): ListMyAPIKeys self-scoped handler"
```

---

### Task 7: Handler — `RevokeMyAPIKey`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Test: `internal/server/identity_selfkey_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRevokeMyAPIKey_OwnerScopedNotFound(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{
		revokeAPIKeyForUser: func(_ context.Context, userID, keyID string) error {
			if userID != "u1" {
				t.Fatalf("userID must come from context, got %q", userID)
			}
			return storage.ErrAPIKeyNotFound
		},
	}, logger: testLogger()}
	_, err := h.RevokeMyAPIKey(selfCtx("reader", "oidc"),
		connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{KeyId: "k9"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestRevokeMyAPIKey_MissingKeyID(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{}, logger: testLogger()}
	_, err := h.RevokeMyAPIKey(selfCtx("reader", "oidc"),
		connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/server/ -run TestRevokeMyAPIKey`
Expected: FAIL — `RevokeMyAPIKey` undefined.

- [ ] **Step 3: Implement**

```go
// RevokeMyAPIKey revokes one of the caller's own keys. Revoke does NOT require
// an interactive source (revoking a leaked credential should always be
// possible), but it is owner-scoped: a foreign/missing key id → NotFound.
func (h *IdentityHandler) RevokeMyAPIKey(ctx context.Context, req *connect.Request[specv1.RevokeMyAPIKeyRequest]) (*connect.Response[specv1.RevokeMyAPIKeyResponse], error) {
	id, err := callerOrUnauth(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}
	if err = h.users.RevokeAPIKeyForUser(ctx, id.UserID, req.Msg.GetKeyId()); err != nil {
		return nil, h.identityError(ctx, err) // ErrAPIKeyNotFound → NotFound
	}
	return connect.NewResponse(&specv1.RevokeMyAPIKeyResponse{}), nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/server/ -run TestRevokeMyAPIKey`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_selfkey_test.go
git commit -s -m "feat(server): RevokeMyAPIKey owner-scoped handler"
```

---

### Task 8: Handler — `RotateMyAPIKey`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Test: `internal/server/identity_selfkey_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestRotateMyAPIKey_RejectsApikeySource(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{}, logger: testLogger()}
	_, err := h.RotateMyAPIKey(selfCtx("writer", "apikey"),
		connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "k1"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("want PermissionDenied, got %v", err)
	}
}

func TestRotateMyAPIKey_ReflowsRoleAtCurrentEffective(t *testing.T) {
	var gotDowngrade string
	h := &IdentityHandler{users: &usersBackendStub{
		getAPIKeyForUser: func(_ context.Context, _, _ string) (*storage.APIKey, error) {
			return &storage.APIKey{ID: "k1", UserID: "u1", RoleDowngrade: "admin"}, nil // stale high cap
		},
		rotateAPIKeyForUser: func(_ context.Context, userID, oldKeyID, phc, downgrade string, _ *time.Time) (*storage.APIKey, error) {
			gotDowngrade = downgrade
			return &storage.APIKey{ID: "k2", UserID: userID, Prefix: "wxyz5678", RoleDowngrade: downgrade}, nil
		},
	}, logger: testLogger()}
	// caller is now only writer-effective; rotating a stale admin-cap key must re-floor to writer.
	_, err := h.RotateMyAPIKey(selfCtx("writer", "oidc"),
		connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "k1"}))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if gotDowngrade != "writer" {
		t.Fatalf("rotate must re-floor downgrade to writer, got %q", gotDowngrade)
	}
}

func TestRotateMyAPIKey_NotFoundForeign(t *testing.T) {
	h := &IdentityHandler{users: &usersBackendStub{
		getAPIKeyForUser: func(context.Context, string, string) (*storage.APIKey, error) {
			return nil, storage.ErrAPIKeyNotFound
		},
	}, logger: testLogger()}
	_, err := h.RotateMyAPIKey(selfCtx("writer", "oidc"),
		connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "k1"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("want NotFound, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/server/ -run TestRotateMyAPIKey`
Expected: FAIL — `RotateMyAPIKey` undefined.

- [ ] **Step 3: Implement**

```go
// RotateMyAPIKey mints a fresh secret for one of the caller's own keys and
// revokes the old one. Like create, it requires an interactive source, re-caps
// expiry, and RE-FLOORS role_downgrade at the caller's CURRENT effective role
// (so a since-demoted caller cannot rotate a stale higher-cap key back to life).
func (h *IdentityHandler) RotateMyAPIKey(ctx context.Context, req *connect.Request[specv1.RotateMyAPIKeyRequest]) (*connect.Response[specv1.RotateMyAPIKeyResponse], error) {
	id, err := callerOrUnauth(ctx)
	if err != nil {
		return nil, err
	}
	if err = requireSelfServiceCaller(id); err != nil {
		return nil, err
	}
	if req.Msg.GetKeyId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("key_id is required"))
	}

	old, err := h.users.GetAPIKeyForUser(ctx, id.UserID, req.Msg.GetKeyId())
	if err != nil {
		return nil, h.identityError(ctx, err) // ErrAPIKeyNotFound → NotFound
	}

	// Re-floor: base = old cap (or caller ceiling if old had none), then floor
	// at the caller's current effective role.
	ceiling := id.EffectiveRole
	base := ceiling
	if old.RoleDowngrade != "" {
		base = auth.Role(old.RoleDowngrade)
	}
	downgrade := auth.RoleMin(base, ceiling)

	expiresAt, err := h.selfKeyExpiry(req.Msg.GetExpiresAt())
	if err != nil {
		return nil, err
	}

	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		h.logger.ErrorContext(ctx, "RotateMyAPIKey: generate secret", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	rotated, err := h.users.RotateAPIKeyForUser(ctx, id.UserID, req.Msg.GetKeyId(), phc, string(downgrade), &expiresAt)
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.RotateMyAPIKeyResponse{
		Key:       apiKeyToProto(rotated),
		Plaintext: auth.FormatAPIKeyToken(rotated.Prefix, secret),
	}), nil
}
```

- [ ] **Step 4: Run all handler tests**

Run: `go test ./internal/server/ -run 'TestRotateMyAPIKey|TestCreateMyAPIKey|TestListMyAPIKeys|TestRevokeMyAPIKey'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_selfkey_test.go
git commit -s -m "feat(server): RotateMyAPIKey with role re-floor + expiry re-cap"
```

---

### Task 9: CLI — self-mint via `specgraph auth api-key create`

**Files:**

- Modify: `cmd/specgraph/auth_apikey.go`
- Modify: `cmd/specgraph/identity_client.go` (confirm/extend client surface)
- Test: `cmd/specgraph/auth_apikey_test.go`

**Behavior:** `create`/`list`/`revoke`/`rotate` self-mint when no `--user` is given (call `*MyAPIKey`); with `--user`, keep the admin path. Add `--ttl` (duration) on create/rotate. The self path must authenticate with the stored OIDC **session** (from `specgraph login`), so warn if `SPECGRAPH_API_KEY` is set (it takes precedence in `client.go` and will hard-fail the `Source=="apikey"` gate).

- [ ] **Step 1: Confirm the generated client exposes the new RPCs**

Run: `grep -n 'CreateMyAPIKey\|ListMyAPIKeys\|RotateMyAPIKey\|RevokeMyAPIKey' gen/specgraph/v1/specgraphv1connect/*.go`
Expected: client methods exist on `IdentityServiceClient`. If `cmd/specgraph/identity_client.go` wraps a narrower interface, add the four methods to that wrapper.

- [ ] **Step 2: Write a CLI test for the self-path warning + routing**

Add to `cmd/specgraph/auth_apikey_test.go` a test that, with no `--user` and `SPECGRAPH_API_KEY` set in the environment, `create` emits a warning to stderr mentioning `SPECGRAPH_API_KEY` and calls `CreateMyAPIKey` (use the existing stub-client harness in that test file — match `withStubIdentityClient`/`stubIdentityHandler` patterns from `auth_oidc_test.go`). Assert the stub's `CreateMyAPIKey` was invoked and `CreateAPIKey` was not.

> Match the existing CLI test harness exactly; if the stub client type lacks the new methods, extend it.

- [ ] **Step 3: Run to verify failure**

Run: `go test ./cmd/specgraph/ -run APIKey`
Expected: FAIL (self routing/warn not implemented).

- [ ] **Step 4: Implement the create command routing**

Replace the `authAPIKeyCreateCmd.RunE` body so that empty `--user` self-mints:

```go
RunE: func(cmd *cobra.Command, _ []string) error {
	expiresAt, err := selfTTLOrExpires(apiKeyCreateTTL, apiKeyCreateExpires)
	if err != nil {
		return err
	}
	client, err := identityClient()
	if err != nil {
		return err
	}
	if apiKeyCreateUser == "" {
		warnIfAPIKeyEnv(cmd.ErrOrStderr())
		resp, mErr := client.CreateMyAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateMyAPIKeyRequest{
			Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown, ExpiresAt: expiresAt,
		}))
		if mErr != nil {
			return fmt.Errorf("create api key: %w", mErr)
		}
		return printNewKey(cmd, resp.Msg.GetKey(), resp.Msg.GetPlaintext())
	}
	// admin path (unchanged)
	resp, err := client.CreateAPIKey(cmd.Context(), connect.NewRequest(&specv1.CreateAPIKeyRequest{
		UserId: apiKeyCreateUser, Label: apiKeyCreateLabel, RoleDowngrade: apiKeyCreateDown, ExpiresAt: expiresAt,
	}))
	if err != nil {
		return fmt.Errorf("create api key: %w", err)
	}
	return printNewKey(cmd, resp.Msg.GetKey(), resp.Msg.GetPlaintext())
},
```

Add helpers in the same file:

```go
// warnIfAPIKeyEnv warns that SPECGRAPH_API_KEY (which client.go prefers over the
// stored session) will cause self-mint to authenticate as that key and be
// rejected by the apikey-source gate.
func warnIfAPIKeyEnv(w io.Writer) {
	if os.Getenv("SPECGRAPH_API_KEY") != "" {
		fmt.Fprintln(w, "warning: SPECGRAPH_API_KEY is set and takes precedence over your login "+
			"session; self-service key minting requires an interactive login. "+
			"Unset SPECGRAPH_API_KEY (or run `specgraph login`) if this fails with permission denied.")
	}
}

// selfTTLOrExpires resolves a --ttl duration (e.g. 90d→use time) or an
// --expires-at RFC3339. Exactly one or neither; neither = server default.
func selfTTLOrExpires(ttl, expires string) (*timestamppb.Timestamp, error) {
	if ttl != "" && expires != "" {
		return nil, fmt.Errorf("use only one of --ttl or --expires-at")
	}
	if ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil {
			return nil, fmt.Errorf("invalid --ttl %q (e.g. 720h): %w", ttl, err)
		}
		return timestamppb.New(time.Now().Add(d)), nil
	}
	return parseExpiresAt(expires)
}

func printNewKey(cmd *cobra.Command, key *specv1.APIKey, plaintext string) error {
	if authJSON {
		return printJSON(cmd.OutOrStdout(), map[string]any{"key": key, "plaintext": plaintext})
	}
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Created API key %s (prefix %s).\n\n  %s\n\n", key.GetId(), key.GetPrefix(), plaintext)
	fmt.Fprintln(w, "Store this token now — it will not be shown again.")
	fmt.Fprintln(w, "Set it in your MCP harness environment as SPECGRAPH_API_KEY")
	fmt.Fprintln(w, "(prefer your shell profile or a secret manager over an inline `export`,")
	fmt.Fprintln(w, "which would leak the token into shell history).")
	return nil
}
```

Add the `--ttl` flag var `apiKeyCreateTTL` and register it; add imports `io`, `os`. Apply the same `--user`-routing pattern to `list` (→ `ListMyAPIKeys`), `revoke` (→ `RevokeMyAPIKey`), and `rotate` (→ `RotateMyAPIKey` with `--ttl`).

- [ ] **Step 5: Run tests**

Run: `go test ./cmd/specgraph/ -run APIKey`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/specgraph/auth_apikey.go cmd/specgraph/identity_client.go cmd/specgraph/auth_apikey_test.go
git commit -s -m "feat(cli): self-service api-key create/list/rotate/revoke (no --user)"
```

---

### Task 10: Web — MCP Keys dashboard panel

**Files:**

- Modify: `web/src/lib/auth.svelte.ts` (or a new `web/src/lib/keys.svelte.ts`) — RPC calls
- Create: `web/src/routes/keys/+page.svelte`
- Modify: `web/src/routes/+layout.svelte` (nav link)

**Behavior:** Authenticated-session panel: list own keys (label, created, last-used, expiry, status), Create → one-time reveal modal with copy + the `SPECGRAPH_API_KEY` instruction, Revoke, Rotate. Mutations are POST (ConnectRPC default), cookie-authenticated.

- [ ] **Step 1: Add client calls**

Follow the existing pattern in `web/src/lib/auth.svelte.ts` (how it calls `/api/auth/whoami` and ConnectRPC endpoints). Add typed wrappers `createMyAPIKey({label, roleDowngrade, expiresAt})`, `listMyAPIKeys({includeRevoked})`, `rotateMyAPIKey(keyId)`, `revokeMyAPIKey(keyId)` hitting the generated ConnectRPC `IdentityService` endpoints, sending credentials (cookie) as the existing read calls do.

- [ ] **Step 2: Create the panel**

Create `web/src/routes/keys/+page.svelte` following the existing dashboard route style (`web/src/routes/+page.svelte`): a table bound to `listMyAPIKeys`, a "Create MCP key" form (label, optional role-downgrade ≤ own role, optional expiry), and on create a modal showing the plaintext once with a copy button and the instruction text:
`Set this as SPECGRAPH_API_KEY in your MCP harness environment.` Add Revoke/Rotate buttons per row that call the wrappers and refresh the list.

- [ ] **Step 3: Add nav link**

In `web/src/routes/+layout.svelte`, add a "Keys" nav entry (shown only when authenticated, mirroring the existing auth-gated nav).

- [ ] **Step 4: Build the web bundle**

Run: `task build` (or the web build task — check `task --list`)
Expected: web assets compile; no TypeScript/Svelte errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/
git commit -s -m "feat(web): self-service MCP Keys dashboard panel"
```

---

### Task 11: Postgres integration tests (require Docker)

**Files:**

- Create: `internal/storage/postgres/users_selfkey_test.go` (build tag `//go:build integration`)

- [ ] **Step 1: Write integration tests**

Create `internal/storage/postgres/users_selfkey_test.go` with `//go:build integration`, using the existing testcontainers harness in this package (match the setup in the existing `*_test.go` files — `pgvector/pgvector:pg18`, `ForLog("database system is ready").WithOccurrence(2)`). Cover:

```go
// 1. CreateAPIKeyWithQuota enforces the cap.
//    - Insert maxActive=2 keys for a user → 3rd returns storage.ErrAPIKeyQuotaExceeded.
//    - A revoked key does NOT count toward the cap (revoke one → can mint again).
//    - An expired key does NOT count (insert with past expires_at → mint succeeds).
// 2. GetAPIKeyForUser: own key returns row; another user's id → ErrAPIKeyNotFound.
// 3. RevokeAPIKeyForUser: own key → nil and revoked_at set; re-revoke → nil (idempotent);
//    foreign id → ErrAPIKeyNotFound.
// 4. RotateAPIKeyForUser: own active key → new key with the passed role_downgrade
//    and expires_at (NOT inherited); old key revoked; foreign/revoked id → ErrAPIKeyNotFound.
```

Write each as a focused subtest with explicit `errors.Is` assertions on the sentinels.

- [ ] **Step 2: Run integration tests**

Run: `go test -tags integration ./internal/storage/postgres/ -run SelfKey`
Expected: PASS (Docker required).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/users_selfkey_test.go
git commit -s -m "test(storage): integration tests for self-service key storage"
```

---

### Task 12: Full verification + docs

- [ ] **Step 1: Update the service comment was done in Task 1 — verify no stale "all RPCs require admin" text remains**

Run: `grep -rn "require the admin role" proto/ internal/`
Expected: only accurate references remain.

- [ ] **Step 2: Run the full quality gate**

Run: `task check`
Expected: fmt, license, lint, build, unit tests all PASS. Fix any `revive` package-comment / `addlicense` header issues on new files.

- [ ] **Step 3: Run integration + e2e (Docker)**

Run: `task pr-prep`
Expected: PASS.

- [ ] **Step 4: Commit any fixups**

```bash
git add -A
git commit -s -m "chore(spgr-g7st): lint/format fixups for self-service keys"
```

---

## Self-Review Notes

- **Spec coverage:** authz model (Tasks 1,3,5–8) · `RoleMin` floor (Tasks 2,5,8) · `Source=="apikey"` gate (Tasks 5,8) · mandatory/capped expiry + default (Task 5,8) · quota via tx+lock (Tasks 4,11) · owner-scoped revoke/rotate + idempotency (Tasks 4,7,8,11) · `ListMyAPIKeys` hard-set UserID (Task 6) · CLI both surfaces + session precedence (Task 9) · web panel + one-time reveal (Task 10) · drift test + knownVerbs + stubs (Tasks 3,4) · no migration (confirmed — uses existing `api_keys` + `api_keys_active` index).
- **Deferred to spgr-tmqm (not in this plan):** MCP OAuth resource-server, short-lived tokens, harness-config rewrite, resource-ownership Cedar, CSRF hardening beyond SameSite, per-identity rate-limiting (spec §5 lists rate-limiting — **if you want it in this slice, add a task** reusing the JIT limiter pattern at `identitystore.go:643`; otherwise track as a follow-up bead).
- **Open config item:** defaults are consts (Task 5). Promoting `defaultSelfKeyTTL`/`maxSelfKeyTTL`/`maxActiveSelfKeys` to `.specgraph.yaml`/server config is a small follow-up; file a bead if desired.
