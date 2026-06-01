# Identity Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the Identity Storage layer specified in `docs/plans/2026-05-22-identity-storage-design.md` — Postgres-backed `UsersBackend` for global identity tables, sibling to but independent of the per-project `*Store`.

**Architecture:** A separate `*AuthStore` type in `internal/storage/postgres/` shares the existing pgxpool but skips the project-scoping requirement. Single `UsersBackend` interface in `internal/storage/users.go` (package `storage`). Single `User` domain struct with a `Kind` enum (Human / ServiceAccount). Goose migrations live in a separate directory with a separate version table to avoid collision with project migrations. Tests share the existing testcontainers pattern; the Postgres container is reused across project and auth stores.

**Tech Stack:** Go 1.x, pgx v5 (pgxpool), goose v3, testcontainers-go, argon2id (`golang.org/x/crypto/argon2`).

**Implements bead:** Implementation of approved design `spgr-e82m` under epic `spgr-rjrt`.

---

## Testing approach

Tests in this plan fall into four categories. Each task tags which categories it covers (under **Covers:** near the test step). When you find yourself wanting to add a new test mid-task, decide which category it belongs to — that frames whether it goes with the current task or belongs in the sweep tasks at the end.

### Happy

The normal success path: input is well-formed, preconditions are met, the operation does its primary thing. **Every task that introduces a method has at least one happy test.** If you can't articulate the happy case in one sentence, the method's contract isn't clear yet — stop and clarify before writing tests.

Example: "CreateHuman with a valid User struct returns the inserted row with a populated ID."

### Invariants

Properties that must always hold regardless of input: structural rules enforced by the system. Invariants come from the design (e.g., "at most one bootstrap admin"); they're not method-specific — they're system-wide. Often invariants need their own dedicated tests rather than fitting inside a happy-path test.

Examples:
- *Bootstrap uniqueness* — at most one user has `bootstrap=true AND deleted_at IS NULL`. Tested via concurrent CreateHuman calls in Task 27.
- *(issuer, subject) globally unique* — two providers with the same `sub` for different people can't collide. Tested via concurrent JIT calls in Task 28.
- *Soft-delete cascades to key revocation in one tx* — never an intermediate state where the user is deleted but their keys still pass auth. Tested in Task 16.
- *ServiceAccounts have no OIDC bindings* — convention enforced at the write API surface. Tested in Task 30 (sweep).

When in doubt: if the property is "always true regardless of which method you called," it's an invariant.

### Boundaries

Edges of the input space and behavior space: NotFound, idempotent re-calls, missing-required-fields, pagination edges (limit=0, very large offset, empty result), empty-vs-null distinctions, very-long strings. Boundary tests catch the "what if" cases that happy tests skip.

Examples:
- *NotFound* — every Lookup/Get method returns the appropriate sentinel error.
- *Idempotent re-call* — calling Revoke / SoftDelete / Purge / Unbind twice is not an error.
- *Empty filter* — ListUsers with no filter returns all (non-deleted) rows; an empty filter is not "no rows."
- *Default limit* — `ListUsersFilter{Limit: 0}` uses the default (100), not literal zero.

### E2E (integration here)

Multi-method flows that exercise the storage layer as a whole rather than one method at a time. For the Storage plan, "E2E" means a single test that walks through bootstrap → JIT-create-real-user → mint-key → rotate-key → promote-role → soft-delete-bootstrap and asserts the final state is coherent. True system-E2E (CLI → RPC → storage → response) is out of scope for this plan; it lives in the Authn and Bootstrap & UX plans.

There's one E2E test in this plan (Task 29). Add more if real flows surface during implementation that aren't already covered by category-1–3 tests.

### How to use these categories

When you write a task's failing test (TDD step 1), start with **Happy**. If implementing the happy path reveals that you need to also assert an invariant or a boundary, either add it to the same task (if it's small and method-local) or note it for the sweep tasks at the end (if it's cross-cutting). Don't let a single test cover all four categories — it makes the assertion clutter the intent.

The sweep tasks (Task 30 invariants, Task 31 boundaries) exist explicitly for the cross-cutting properties that don't belong to any single method.

---

## File Structure

**Create:**

- `internal/storage/users.go` — `UsersBackend` interface (package `storage`).
- `internal/storage/users_domain.go` — `User`, `OIDCBinding`, `APIKey` domain types + `Kind` enum + sentinel errors specific to this domain.
- `internal/storage/postgres/auth.go` — `AuthStore` type + `NewAuth` constructor + goose migration runner + pool accessor.
- `internal/storage/postgres/users.go` — methods on `*AuthStore` implementing `UsersBackend` (resolve-path queries, CRUD).
- `internal/storage/postgres/auth_migrations/001_initial.sql` — schema for `users`, `oidc_bindings`, `api_keys`, indexes.
- `internal/storage/postgres/auth_migrations/embed.go` — goose `embed.FS` registration for the auth migrations.
- `internal/storage/postgres/users_test.go` — integration tests using testcontainers.

**Modify:**

- `internal/storage/postgres/postgres.go` — add `(s *Store) Pool() *pgxpool.Pool` accessor (~5 lines).

**No changes to:** existing `*Store` semantics or behavior; existing migrations directory; existing interceptor or auth resolver (those land in the Authn plan).

---

## Task 1: Define the `Kind` enum and `User` domain type

**Files:**

- Create: `internal/storage/users_domain.go`

- [ ] **Step 1: Write the User type and Kind enum**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"time"
)

// Kind discriminates a User row as a Human (OIDC-backed person) or a
// ServiceAccount (machine identity owned by a Human). The kind constrains
// which credentials and ownership relationships are valid (see UsersBackend
// invariants); it does NOT change lifecycle semantics — soft-delete behaves
// identically for both kinds.
type Kind string

const (
	KindHuman          Kind = "human"
	KindServiceAccount Kind = "service_account"
)

// User is the identity row for a Human or ServiceAccount. Single table with
// a Kind discriminator; lifecycle is uniform across kinds.
//
// Invariants enforced at the schema or store layer (see UsersBackend):
//   - ServiceAccounts MUST have OwnerUserID set; Humans MUST NOT.
//   - At most one User has Bootstrap=true AND DeletedAt=nil.
//   - ServiceAccounts MUST NOT have rows in oidc_bindings (store-layer).
type User struct {
	ID           string     // uuid
	Kind         Kind
	DisplayName  string
	Email        string     // nullable; empty when not set
	Role         string     // role assignment; built-in or custom (validated against config)
	OwnerUserID  string     // ServiceAccount owner; empty for Humans
	Bootstrap    bool       // true for the system bootstrap admin
	CreatedAt    time.Time
	DeletedAt    *time.Time // nil = active
}

// IsHuman reports whether the user is a Human kind.
func (u *User) IsHuman() bool { return u.Kind == KindHuman }

// IsServiceAccount reports whether the user is a ServiceAccount kind.
func (u *User) IsServiceAccount() bool { return u.Kind == KindServiceAccount }

// IsActive reports whether the user has not been soft-deleted.
func (u *User) IsActive() bool { return u.DeletedAt == nil }
```

- [ ] **Step 2: Verify the package compiles**

Run: `cd internal/storage && go build ./...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/users_domain.go
git commit -s -m "feat(storage): add User domain type with Kind discriminator"
```

---

## Task 2: Define `OIDCBinding` and `APIKey` domain types

**Files:**

- Modify: `internal/storage/users_domain.go`

- [ ] **Step 1: Append the binding and key types**

```go
// OIDCBinding links a User to an external OIDC subject. The (Issuer, Subject)
// pair is globally unique across providers to prevent same-sub collisions
// between IdPs.
type OIDCBinding struct {
	ID          string
	UserID      string
	Issuer      string
	Subject     string
	EmailAtBind string // captured at JIT bind; nullable
	CreatedAt   time.Time
}

// APIKey is an issued credential owned by a single User (Human or
// ServiceAccount). Plaintext is never persisted; only the argon2id PHC
// hash. The prefix is queryable for O(log N) lookup; the secret is
// constant-time-verified against PHCHash.
//
// RoleDowngrade, when set, caps the EffectiveRole at request time to at most
// the named role (subject to the partial-ordering rules in the design).
type APIKey struct {
	ID            string
	UserID        string
	Prefix        string
	PHCHash       string     // argon2id PHC-format string
	RoleDowngrade string     // empty = no downgrade
	Label         string
	ExpiresAt     *time.Time // nil = no expiry (used by bootstrap key)
	LastUsedAt    *time.Time // nil = never used
	RevokedAt     *time.Time // nil = active
	CreatedAt     time.Time
}

// IsActive reports whether the key is neither revoked nor expired (as of now).
func (k *APIKey) IsActive(now time.Time) bool {
	if k.RevokedAt != nil {
		return false
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return false
	}
	return true
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/storage && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/users_domain.go
git commit -s -m "feat(storage): add OIDCBinding and APIKey domain types"
```

---

## Task 3: Add domain-specific sentinel errors

**Files:**

- Modify: `internal/storage/users_domain.go`

- [ ] **Step 1: Append sentinel errors**

```go
import (
	"errors"
	"time"
)

// Sentinel errors returned by UsersBackend. Callers use errors.Is to detect.
var (
	ErrUserNotFound        = errors.New("storage: user not found")
	ErrAPIKeyNotFound      = errors.New("storage: api key not found")
	ErrOIDCBindingNotFound = errors.New("storage: oidc binding not found")
	ErrBootstrapExists     = errors.New("storage: bootstrap user already exists")
	ErrAPIKeyPrefixExists  = errors.New("storage: api key prefix collision")
)
```

> Note: the existing `internal/storage/errors.go` holds cross-domain errors. These are domain-specific to identity, so they live alongside the domain types per the project convention (see `internal/storage/decision.go` for the precedent — sentinel errors next to domain types).

- [ ] **Step 2: Verify compile**

Run: `cd internal/storage && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/users_domain.go
git commit -s -m "feat(storage): add sentinel errors for identity storage"
```

---

## Task 4: Define `UsersBackend` interface

**Files:**

- Create: `internal/storage/users.go`

- [ ] **Step 1: Write the interface**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package storage defines storage backend interfaces. UsersBackend is the
// identity-domain interface — global, not project-scoped, intentionally
// distinct from ScopedBackend.
package storage

import (
	"context"
	"time"
)

// UsersBackend defines storage operations for the identity domain (users,
// API keys, OIDC bindings). It is the canonical cross-domain seam for any
// other domain that needs to ask identity questions (story #3, #4).
//
// Implementations MUST honor the invariants stated on User and the design
// document (docs/plans/2026-05-22-identity-storage-design.md).
type UsersBackend interface {
	// --- resolve-path queries (hot) ---

	// LookupAPIKeyByPrefix returns the key with the given prefix. Returns
	// ErrAPIKeyNotFound on miss. Does not verify the secret; callers must.
	LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKey, error)

	// LookupOIDCBinding returns the binding for the (issuer, subject) pair.
	// Returns ErrOIDCBindingNotFound on miss.
	LookupOIDCBinding(ctx context.Context, issuer, subject string) (*OIDCBinding, error)

	// GetUserByID returns the user with the given ID, including soft-deleted.
	// Callers gate on User.IsActive() per their semantics. Returns
	// ErrUserNotFound on miss.
	GetUserByID(ctx context.Context, id string) (*User, error)

	// GetBootstrap returns the active bootstrap user, or ErrUserNotFound if
	// none exists. Used by bootstrap path to detect idempotency.
	GetBootstrap(ctx context.Context) (*User, error)

	// --- user CRUD ---

	// CreateHuman inserts a Human row. The OIDCBinding is created in the
	// same transaction; pass binding=nil for admin-created Humans (rare).
	// Returns ErrBootstrapExists if u.Bootstrap is true AND a bootstrap
	// already exists.
	CreateHuman(ctx context.Context, u *User, binding *OIDCBinding) (*User, error)

	// CreateServiceAccount inserts a ServiceAccount row. u.OwnerUserID
	// must reference an existing active Human.
	CreateServiceAccount(ctx context.Context, u *User) (*User, error)

	// UpdateUserRole sets the role on an active user. Role validation
	// against the YAML config is the caller's responsibility.
	UpdateUserRole(ctx context.Context, userID, role string) error

	// SoftDeleteUser sets deleted_at and revokes all active keys in one tx.
	// Idempotent (re-deleting already-deleted user is a no-op).
	SoftDeleteUser(ctx context.Context, userID string) error

	// PurgeUser hard-deletes the user; cascades through bindings and keys.
	PurgeUser(ctx context.Context, userID string) error

	// ListUsers returns users matching the filter, optionally including
	// soft-deleted rows. Pagination is offset/limit; impl may add cursor
	// later.
	ListUsers(ctx context.Context, filter ListUsersFilter) ([]*User, error)

	// --- API key CRUD ---

	// CreateAPIKey inserts a new key. Retries on prefix-uniqueness violation
	// up to 3 times before returning ErrAPIKeyPrefixExists.
	CreateAPIKey(ctx context.Context, k *APIKey) (*APIKey, error)

	// RevokeAPIKey marks the key revoked. Idempotent on already-revoked keys.
	RevokeAPIKey(ctx context.Context, keyID string) error

	// RotateAPIKey revokes the old key and creates a new one with the same
	// metadata (label, role_downgrade, expires_at) in one transaction.
	// Returns the new key.
	RotateAPIKey(ctx context.Context, oldKeyID string, newKey *APIKey) (*APIKey, error)

	// ListAPIKeys returns keys for the given user; pass userID="" to list
	// across all users (admin operation). Excludes revoked keys unless
	// IncludeRevoked is set.
	ListAPIKeys(ctx context.Context, filter ListAPIKeysFilter) ([]*APIKey, error)

	// TouchLastUsed sets last_used_at = now() for the key. Fire-and-forget
	// from caller's perspective; impl is fast and ignores errors silently
	// (but returns them for tests).
	TouchLastUsed(ctx context.Context, keyID string) error

	// --- OIDC binding CRUD ---

	// JITCreateHuman creates a Human + its OIDCBinding atomically. Used by
	// the OIDC resolver when a binding lookup misses. On (issuer, subject)
	// uniqueness violation, returns the existing user via a re-lookup
	// (race-safe).
	JITCreateHuman(ctx context.Context, u *User, binding *OIDCBinding) (*User, *OIDCBinding, error)

	// ListOIDCBindings returns bindings for the given user.
	ListOIDCBindings(ctx context.Context, userID string) ([]*OIDCBinding, error)

	// UnbindOIDC removes the given binding. Last-credential protection is
	// the caller's responsibility (handler-level policy, not storage).
	UnbindOIDC(ctx context.Context, bindingID string) error
}

// ListUsersFilter narrows ListUsers results.
type ListUsersFilter struct {
	Kind            Kind   // empty = all kinds
	Role            string // empty = all roles
	IncludeDeleted  bool
	CreatedAfter    *time.Time
	Limit           int    // 0 = default 100
	Offset          int
}

// ListAPIKeysFilter narrows ListAPIKeys results.
type ListAPIKeysFilter struct {
	UserID         string // empty = all users (admin)
	IncludeRevoked bool
	Limit          int
	Offset         int
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/storage && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/users.go
git commit -s -m "feat(storage): add UsersBackend interface"
```

---

## Task 5: Add `Pool()` accessor on `*Store`

**Files:**

- Modify: `internal/storage/postgres/postgres.go`

- [ ] **Step 1: Add the accessor**

Locate the existing `*Store` type definition and add:

```go
// Pool returns the underlying pgxpool.Pool. Intended for the AuthStore
// constructor to share the database connection without owning a second
// pool. NOT a general-purpose escape hatch — code outside the auth and
// project storage layers should never reach into the pool directly.
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}
```

> Cross-reference: see `internal/storage/postgres/postgres.go:30` (or wherever `*Store` is defined) for the existing struct that holds `pool *pgxpool.Pool`. The accessor is a 3-liner.

- [ ] **Step 2: Verify compile**

Run: `cd internal/storage/postgres && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/postgres.go
git commit -s -m "feat(postgres): expose pgxpool via Store.Pool() for AuthStore sharing"
```

---

## Task 6: Author the initial auth migration

Mirrors the existing `internal/storage/postgres/migrate.go` pattern: SQL files live in a sibling subdirectory (`auth_migrations/`) of the `postgres` package and are embedded by the `postgres` package itself via `//go:embed`. No separate subpackage. This matches how the existing `migrations/` directory is wired (see `migrate.go:17`).

**Files:**

- Create: `internal/storage/postgres/auth_migrations/001_initial.sql`

- [ ] **Step 1: Write the migration**

```sql
-- SPDX-License-Identifier: Apache-2.0
-- Copyright 2026 Sean Brandt

-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind            text NOT NULL CHECK (kind IN ('human','service_account')),
    display_name    text NOT NULL CHECK (length(display_name) <= 255),
    email           text NOT NULL DEFAULT '' CHECK (length(email) <= 320),
    role            text NOT NULL CHECK (length(role) <= 64),
    owner_user_id   uuid REFERENCES users(id),
    bootstrap       boolean NOT NULL DEFAULT false,
    created_at      timestamptz NOT NULL DEFAULT now(),
    deleted_at      timestamptz,

    CHECK (kind = 'service_account' OR owner_user_id IS NULL),
    CHECK (kind = 'human'           OR owner_user_id IS NOT NULL)
);

CREATE UNIQUE INDEX users_one_bootstrap ON users(bootstrap)
    WHERE bootstrap = true AND deleted_at IS NULL;

CREATE TABLE oidc_bindings (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    issuer          text NOT NULL CHECK (length(issuer) <= 512),
    subject         text NOT NULL CHECK (length(subject) <= 255),
    email_at_bind   text NOT NULL DEFAULT '' CHECK (length(email_at_bind) <= 320),
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (issuer, subject)
);

CREATE TABLE api_keys (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prefix          text NOT NULL UNIQUE CHECK (length(prefix) >= 8 AND length(prefix) <= 16),
    phc_hash        text NOT NULL CHECK (length(phc_hash) >= 32 AND length(phc_hash) <= 256),
    role_downgrade  text NOT NULL DEFAULT '' CHECK (length(role_downgrade) <= 64),
    label           text NOT NULL DEFAULT '' CHECK (length(label) <= 128),
    expires_at      timestamptz,
    last_used_at    timestamptz,
    revoked_at      timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_keys_active ON api_keys(user_id)
    WHERE revoked_at IS NULL;

-- +goose Down

DROP INDEX IF EXISTS api_keys_active;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS oidc_bindings;
DROP INDEX IF EXISTS users_one_bootstrap;
DROP TABLE IF EXISTS users;
```

Length `CHECK`s pin the entropy floor for `prefix` (≥8 chars; the design's "blind-enumeration impractical" claim depends on this) and provide defense-in-depth caps against malicious or buggy callers shoving multi-MB strings into `subject`, `email`, etc. The `phc_hash` lower bound rejects obviously-corrupt entries; argon2id PHC strings are ~95–120 chars typically, leaving comfortable margin.

- [ ] **Step 2: Verify the migration is well-formed**

The SQL file embeds into the `postgres` package via `//go:embed` declared in `auth.go` (next task) — there is no separate `embed.go` needed (this is how the existing `migrate.go` does it too: see line 17 with `//go:embed migrations/*.sql`). No build step here yet; the SQL is exercised by the migration runner in Task 7 and the test harness in Task 8.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/auth_migrations/
git commit -s -m "feat(postgres): add identity storage migrations"
```

---

## Task 7: Create `AuthStore` scaffold with `NewAuth` constructor

**Files:**

- Create: `internal/storage/postgres/auth.go`

**Lifecycle contract pinned by this task:**

- `postgres.New` owns the `*pgxpool.Pool`. `postgres.NewAuth` borrows it.
- Caller (typically `cmd/specgraph/serve.go`) must close `*AuthStore` BEFORE closing `*Store`. `*AuthStore.Close()` is a no-op today, but the ordering rule lets future flush-on-shutdown work (e.g., draining a usagetracker queue) slot in without breaking shutdowns.
- Both `New` and `NewAuth` use goose, which sets package-global state (`goose.SetBaseFS`, `goose.SetTableName`). **They MUST NOT be called concurrently.** The expected call order is `New(...)` first → its migrations run → returns → `NewAuth(store.Pool(), ...)` → its migrations run. This is the natural call chain in `serve.go`; the comment on `NewAuth` will spell it out so future contributors don't try to parallelize startup.
- Clock override (`WithAuthClock`) affects only mutation timestamps that the Go code passes explicitly (e.g., `deleted_at` in `SoftDeleteUser`, `revoked_at` in `RevokeAPIKey`). Insert timestamps come from the SQL `DEFAULT now()` clause and bypass the override. Tests that need to fake time across the board should pin all timestamps from outside, or alternatively the impl can be refactored to pass `s.now()` everywhere (deferred follow-up).

- [ ] **Step 1: Write the AuthStore type and constructor**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"embed"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/specgraph/specgraph/internal/storage"
)

//go:embed auth_migrations/*.sql
var authMigrations embed.FS

// Compile-time assertion that *AuthStore implements UsersBackend.
// Mirrors the convention used by *Store for ConstitutionBackend etc.
// Note: this assertion forces every UsersBackend method to exist on
// *AuthStore. Stubs for every method are declared further down in this
// file to satisfy the assertion; real implementations land in tasks 9–26
// which replace the stubs one at a time.
var _ storage.UsersBackend = (*AuthStore)(nil)

// AuthStore is the Postgres implementation of UsersBackend. It is a sibling
// to *Store: shares the database pool, holds no project scope, owns the
// identity tables exclusively.
//
// Pool ownership: *Store owns the pool; *AuthStore borrows it. Callers
// MUST close *AuthStore before *Store at shutdown. Today AuthStore.Close
// is a no-op for the pool, but this ordering rule lets future flush-on-
// shutdown work (usagetracker drain etc.) slot in cleanly.
//
// Migration safety: NewAuth runs auth migrations using a separate goose
// version table (goose_db_version_auth) to avoid colliding with project
// migrations. Goose mutates package-global state (BaseFS, TableName,
// Dialect) so NewAuth and the existing postgres.New MUST NOT be called
// concurrently. The expected pattern is sequential startup: New first,
// then NewAuth, both single-threaded.
type AuthStore struct {
	pool    *pgxpool.Pool
	nowFunc func() time.Time
}

// AuthOption configures an AuthStore.
type AuthOption func(*AuthStore)

// WithAuthClock overrides the wall clock used for explicit mutation
// timestamps (deleted_at, revoked_at). Test-only. Does NOT affect insert
// timestamps that come from SQL DEFAULT now().
func WithAuthClock(fn func() time.Time) AuthOption {
	return func(s *AuthStore) { s.nowFunc = fn }
}

// WithAuthKeyPrefixGenerator overrides the API-key prefix generator.
// Test-only — used to force collision scenarios in CreateAPIKey tests.
// Production code does not call WithAuthKeyPrefixGenerator.
func WithAuthKeyPrefixGenerator(fn func() (string, error)) AuthOption {
	return func(s *AuthStore) { s.genPrefix = fn }
}

// NewAuth constructs an AuthStore wrapping the given pool. The caller
// retains ownership of the pool; AuthStore.Close is a no-op for the pool.
// Auth migrations run inline using a dedicated goose version table.
//
// MUST be called after postgres.New and never concurrently with it
// (goose uses package-global state). See type docstring.
func NewAuth(ctx context.Context, pool *pgxpool.Pool, opts ...AuthOption) (*AuthStore, error) {
	if pool == nil {
		return nil, errors.New("postgres: NewAuth: pool must not be nil")
	}
	s := &AuthStore{
		pool:      pool,
		nowFunc:   time.Now,
		genPrefix: defaultGenerateKeyPrefix,
	}
	for _, o := range opts {
		o(s)
	}

	if err := s.runAuthMigrations(ctx); err != nil {
		return nil, fmt.Errorf("postgres: auth migrations: %w", err)
	}
	return s, nil
}

// runAuthMigrations runs the embedded auth migrations using a dedicated
// goose version table. Goose state is package-global; do not call
// concurrently with the existing runMigrations from migrate.go.
func (s *AuthStore) runAuthMigrations(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(s.pool)
	// stdlib.OpenDBFromPool wraps the pool in a *sql.DB facade; closing
	// the *sql.DB does NOT close the underlying pgxpool. Verified via pgx
	// docs (jackc/pgx#1023) — pool ownership stays with the original
	// caller.
	defer func() { _ = db.Close() }()

	goose.SetBaseFS(authMigrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	goose.SetTableName("goose_db_version_auth")
	defer goose.SetTableName("goose_db_version") // restore default for any subsequent caller

	if err := goose.UpContext(ctx, db, "auth_migrations"); err != nil {
		return fmt.Errorf("run auth migrations: %w", err)
	}
	return nil
}

// Close releases AuthStore resources. Today this is a no-op (the pool is
// borrowed; no other resources held). Future flush-on-shutdown work
// (usagetracker drain etc.) will hook here. Callers MUST call Close
// before closing the underlying *Store.
func (s *AuthStore) Close(_ context.Context) error {
	return nil
}

// now returns the wall clock time used for explicit mutation timestamps.
func (s *AuthStore) now() time.Time { return s.nowFunc() }
```

- [ ] **Step 2: Add all method stubs**

The compile-time assertion `var _ storage.UsersBackend = (*AuthStore)(nil)` requires every interface method to exist on `*AuthStore`. Add all 17 stubs at the bottom of `auth.go` BEFORE running the compile in Step 3:

```go
// --- UsersBackend method stubs ---
// These satisfy the compile-time assertion above; real implementations land
// in tasks 9–26, which replace each stub with a SQL-backed method one at a
// time. Stubs return errors.New("not implemented") so that accidental use
// in tests fails loudly with a recognizable message rather than returning
// a zero value.

func (s *AuthStore) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*storage.APIKey, error) {
	return nil, errors.New("LookupAPIKeyByPrefix not implemented")
}

func (s *AuthStore) LookupOIDCBinding(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
	return nil, errors.New("LookupOIDCBinding not implemented")
}

func (s *AuthStore) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	return nil, errors.New("GetUserByID not implemented")
}

func (s *AuthStore) GetBootstrap(ctx context.Context) (*storage.User, error) {
	return nil, errors.New("GetBootstrap not implemented")
}

func (s *AuthStore) CreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, error) {
	return nil, errors.New("CreateHuman not implemented")
}

func (s *AuthStore) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	return nil, errors.New("CreateServiceAccount not implemented")
}

func (s *AuthStore) UpdateUserRole(ctx context.Context, userID, role string) error {
	return errors.New("UpdateUserRole not implemented")
}

func (s *AuthStore) SoftDeleteUser(ctx context.Context, userID string) error {
	return errors.New("SoftDeleteUser not implemented")
}

func (s *AuthStore) PurgeUser(ctx context.Context, userID string) error {
	return errors.New("PurgeUser not implemented")
}

func (s *AuthStore) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	return nil, errors.New("ListUsers not implemented")
}

func (s *AuthStore) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	return nil, errors.New("CreateAPIKey not implemented")
}

func (s *AuthStore) RevokeAPIKey(ctx context.Context, keyID string) error {
	return errors.New("RevokeAPIKey not implemented")
}

func (s *AuthStore) RotateAPIKey(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	return nil, errors.New("RotateAPIKey not implemented")
}

func (s *AuthStore) ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return nil, errors.New("ListAPIKeys not implemented")
}

func (s *AuthStore) TouchLastUsed(ctx context.Context, keyID string) error {
	return errors.New("TouchLastUsed not implemented")
}

func (s *AuthStore) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	return nil, nil, errors.New("JITCreateHuman not implemented")
}

func (s *AuthStore) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	return nil, errors.New("ListOIDCBindings not implemented")
}

func (s *AuthStore) UnbindOIDC(ctx context.Context, bindingID string) error {
	return errors.New("UnbindOIDC not implemented")
}
```

Also add the prefix-generator default at the bottom (used by `CreateAPIKey` in Task 19):

```go
// defaultGenerateKeyPrefix produces 8 random URL-safe base32 characters.
// Overridable via WithAuthKeyPrefixGenerator. Per-instance (not package-
// global) so parallel tests do not race.
func defaultGenerateKeyPrefix() (string, error) {
	const prefixLen = 8
	buf := make([]byte, 5) // 5 bytes -> 8 base32 chars
	if _, err := cryptorand.Read(buf); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)[:prefixLen], nil
}
```

Imports to add at this step: `cryptorand "crypto/rand"`, `"encoding/base32"`.

Add a `genPrefix` field to the `AuthStore` struct definition (initialized in `NewAuth`):

```go
type AuthStore struct {
	pool      *pgxpool.Pool
	nowFunc   func() time.Time
	genPrefix func() (string, error)
}
```

- [ ] **Step 3: Verify compile**

Run: `cd internal/storage/postgres && go build ./...`
Expected: success. The compile-time assertion is satisfied by the stubs.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/auth.go internal/storage/postgres/auth_migrations/
git commit -s -m "feat(postgres): add AuthStore scaffold with goose migration runner"
```

---

## Task 8: Wire testcontainers harness for AuthStore

**Files:**

- Create: `internal/storage/postgres/auth_helpers_test.go`
- Create: `internal/storage/postgres/users_test.go`

**Covers:** Happy (the harness produces a working AuthStore against a fresh DB).

**Important — build tag:** existing tests in this package (`postgres_test.go`, `constitution_test.go`, etc.) are gated with `//go:build integration` so they only run under `task test:integration` (testcontainers requires Docker). Both new files in this task MUST carry the same tag, or they will fail to compile under `go test` (no `connString`) AND silently skip the testcontainer.

- [ ] **Step 1: Write the helpers file**

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres"
)

// sharedTestPool returns a fresh pgxpool against the testcontainers Postgres
// started in postgres_test.go's TestMain. The pool is closed via t.Cleanup;
// callers do not need to manage lifecycle.
//
// Each call opens a new pool. This is the same pattern clearDatabase uses
// (see postgres_test.go:108). The container is shared; the pools are not.
func sharedTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(ctx, connString)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// authTestSetup constructs an AuthStore against the test container's
// Postgres. It uses sharedTestPool internally and registers AuthStore.Close
// via t.Cleanup. The returned store has the auth migrations applied.
func authTestSetup(t *testing.T) *postgres.AuthStore {
	t.Helper()
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)

	auth, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = auth.Close(ctx) })

	return auth
}

// truncateAuthTables wipes identity tables between tests. FK CASCADE on
// oidc_bindings.user_id and api_keys.user_id handles cleanup of child rows.
func truncateAuthTables(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
}
```

- [ ] **Step 2: Write the test file with the migration smoke test**

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage/postgres"
)

// TestAuthStore_NewAuth_RunsMigrations asserts the constructor brings the
// schema up.
func TestAuthStore_NewAuth_RunsMigrations(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	auth, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	defer auth.Close(ctx)

	// Tables exist?
	var exists bool
	row := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_name = 'users')`)
	require.NoError(t, row.Scan(&exists))
	require.True(t, exists, "users table should exist after migrations")
}
```

- [ ] **Step 3: Run the test**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_NewAuth_RunsMigrations -v`
Expected: PASS (Docker required — the testcontainer comes up via the existing `TestMain` in `postgres_test.go`).

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/auth_helpers_test.go internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): add AuthStore test harness sharing the existing testcontainer"
```

---

## Task 9: Implement `LookupAPIKeyByPrefix` (resolve-path)

**Files:**

- Modify: `internal/storage/postgres/users.go` (create file on first method)
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (lookup by known prefix) + Boundary (unknown prefix returns `ErrAPIKeyNotFound`).

> **Cross-spec boundary:** this method does NOT verify the PHC hash — that's the resolver's job in the Authn plan. A corrupted `phc_hash` value (e.g., the column was tampered with) surfaces during `argon2id.Verify` at the Authn layer, which maps it to `ErrUnauthenticated`. Storage's contract is: "return the row as stored." Length CHECKs on `phc_hash` (Task 6) reject obvious truncation at write time; mid-string corruption is the verifier's concern.

- [ ] **Step 1: Write the failing test**

Append to `users_test.go`:

```go
func TestAuthStore_LookupAPIKeyByPrefix(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Seed: a Human + one APIKey via direct SQL (CRUD methods come later).
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, bootstrap)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'reader', false)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (id, user_id, prefix, phc_hash, label)
		VALUES ('00000000-0000-0000-0000-000000000002'::uuid,
		        '00000000-0000-0000-0000-000000000001'::uuid,
		        'abc12345', 'phc-stub', 'test-key')`)
	require.NoError(t, err)

	key, err := auth.LookupAPIKeyByPrefix(ctx, "abc12345")
	require.NoError(t, err)
	require.Equal(t, "abc12345", key.Prefix)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", key.UserID)
	require.Equal(t, "test-key", key.Label)

	// Miss returns ErrAPIKeyNotFound.
	_, err = auth.LookupAPIKeyByPrefix(ctx, "no-such-prefix")
	require.ErrorIs(t, err, storage.ErrAPIKeyNotFound)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_LookupAPIKeyByPrefix -v`
Expected: FAIL with `"not implemented"` (the stub).

- [ ] **Step 3: Write the implementation**

Create `internal/storage/postgres/users.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// LookupAPIKeyByPrefix returns the api_keys row whose prefix matches.
func (s *AuthStore) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*storage.APIKey, error) {
	const q = `
		SELECT id, user_id, prefix, phc_hash, role_downgrade, label,
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys
		WHERE prefix = $1`
	row := s.pool.QueryRow(ctx, q, prefix)

	var k storage.APIKey
	err := row.Scan(
		&k.ID, &k.UserID, &k.Prefix, &k.PHCHash, &k.RoleDowngrade, &k.Label,
		&k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrAPIKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}
```

Remove the stub for `LookupAPIKeyByPrefix` from `auth.go`.

- [ ] **Step 4: Run the test**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_LookupAPIKeyByPrefix -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement LookupAPIKeyByPrefix"
```

---

## Task 10: Implement `LookupOIDCBinding` (resolve-path)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (lookup by known issuer+subject) + Boundary (unknown returns `ErrOIDCBindingNotFound`).

- [ ] **Step 1: Write the failing test**

```go
func TestAuthStore_LookupOIDCBinding(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'reader')`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO oidc_bindings (id, user_id, issuer, subject)
		VALUES ('00000000-0000-0000-0000-000000000003'::uuid,
		        '00000000-0000-0000-0000-000000000001'::uuid,
		        'https://login.microsoftonline.com/tenant/v2.0',
		        'sub-12345')`)
	require.NoError(t, err)

	b, err := auth.LookupOIDCBinding(ctx, "https://login.microsoftonline.com/tenant/v2.0", "sub-12345")
	require.NoError(t, err)
	require.Equal(t, "00000000-0000-0000-0000-000000000001", b.UserID)

	_, err = auth.LookupOIDCBinding(ctx, "https://github.com", "sub-12345")
	require.ErrorIs(t, err, storage.ErrOIDCBindingNotFound)
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_LookupOIDCBinding -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

Append to `users.go`:

```go
// LookupOIDCBinding returns the binding for (issuer, subject).
func (s *AuthStore) LookupOIDCBinding(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
	const q = `
		SELECT id, user_id, issuer, subject, email_at_bind, created_at
		FROM oidc_bindings
		WHERE issuer = $1 AND subject = $2`
	row := s.pool.QueryRow(ctx, q, issuer, subject)

	var b storage.OIDCBinding
	err := row.Scan(&b.ID, &b.UserID, &b.Issuer, &b.Subject, &b.EmailAtBind, &b.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrOIDCBindingNotFound
	}
	if err != nil {
		return nil, err
	}
	return &b, nil
}
```

Remove the stub from `auth.go`.

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_LookupOIDCBinding -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement LookupOIDCBinding"
```

---

## Task 11: Implement `GetUserByID` (resolve-path)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (active user) + Boundary (NotFound) + Invariant (soft-deleted users are returned with `DeletedAt` set so callers can gate; the data shape never hides a deleted row).

- [ ] **Step 1: Write the failing test**

```go
func TestAuthStore_GetUserByID(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, email, role, bootstrap, created_at, deleted_at)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'alice', 'alice@example.com', 'reader',
		        false, $1, NULL),
		       ('00000000-0000-0000-0000-000000000002'::uuid,
		        'human', 'bob', '', 'writer', false, $1, $2)`,
		now, now.Add(time.Hour))
	require.NoError(t, err)

	// Active user.
	u, err := auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	require.Equal(t, "alice", u.DisplayName)
	require.True(t, u.IsActive())

	// Soft-deleted user — still returned (caller gates).
	u, err = auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000002")
	require.NoError(t, err)
	require.False(t, u.IsActive())
	require.NotNil(t, u.DeletedAt)

	// Miss.
	_, err = auth.GetUserByID(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// ServiceAccount round-trip: owner_user_id is populated, scan handles
	// the nullable-FK column via coalesce. Seed via direct SQL (Task 14
	// implements CreateServiceAccount; this test verifies the read path
	// independently).
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, owner_user_id)
		VALUES ('00000000-0000-0000-0000-000000000003'::uuid,
		        'service_account', 'ci-bot', 'writer',
		        '00000000-0000-0000-0000-000000000001'::uuid)`)
	require.NoError(t, err)
	sa, err := auth.GetUserByID(ctx, "00000000-0000-0000-0000-000000000003")
	require.NoError(t, err)
	require.True(t, sa.IsServiceAccount())
	require.Equal(t, "00000000-0000-0000-0000-000000000001", sa.OwnerUserID)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_GetUserByID -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// GetUserByID returns the user row (including soft-deleted).
func (s *AuthStore) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	const q = `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users
		WHERE id = $1::uuid`
	row := s.pool.QueryRow(ctx, q, id)

	var u storage.User
	var kindStr string
	err := row.Scan(
		&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
		&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Kind = storage.Kind(kindStr)
	return &u, nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_GetUserByID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement GetUserByID"
```

---

## Task 12: Implement `GetBootstrap`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (existing bootstrap returned) + Boundary (no bootstrap returns `ErrUserNotFound`).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_GetBootstrap(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// No bootstrap yet.
	_, err := auth.GetBootstrap(ctx)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Insert active bootstrap.
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, kind, display_name, role, bootstrap)
		VALUES ('00000000-0000-0000-0000-000000000001'::uuid,
		        'human', 'admin', 'admin', true)`)
	require.NoError(t, err)

	u, err := auth.GetBootstrap(ctx)
	require.NoError(t, err)
	require.True(t, u.Bootstrap)
	require.Equal(t, "admin", u.DisplayName)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_GetBootstrap -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// GetBootstrap returns the active bootstrap admin, or ErrUserNotFound.
func (s *AuthStore) GetBootstrap(ctx context.Context) (*storage.User, error) {
	const q = `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users
		WHERE bootstrap = true AND deleted_at IS NULL
		LIMIT 1`
	row := s.pool.QueryRow(ctx, q)

	var u storage.User
	var kindStr string
	err := row.Scan(
		&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
		&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	u.Kind = storage.Kind(kindStr)
	return &u, nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_GetBootstrap -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement GetBootstrap"
```

---

## Task 13: Implement `CreateHuman` (transactional with optional binding)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (insert Human; insert Human+binding atomically) + Invariant (bootstrap uniqueness — second bootstrap insert returns `ErrBootstrapExists`; concurrent verification is in Task 27).

- [ ] **Step 1: Write the failing test**

```go
func TestAuthStore_CreateHuman(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u := &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "alice",
		Email:       "alice@example.com",
		Role:        "reader",
	}
	created, err := auth.CreateHuman(ctx, u, nil)
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "alice", created.DisplayName)
	require.True(t, created.IsHuman())
	require.True(t, created.IsActive())

	// With binding atomically.
	u2 := &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "reader"}
	b := &storage.OIDCBinding{
		Issuer: "https://login.microsoftonline.com/t/v2.0", Subject: "sub-bob",
		EmailAtBind: "bob@example.com",
	}
	created2, err := auth.CreateHuman(ctx, u2, b)
	require.NoError(t, err)

	// Verify the binding via direct SQL — ListOIDCBindings is implemented
	// later in Task 25, so we don't depend on it here.
	var bindingCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oidc_bindings
		WHERE user_id = $1::uuid AND subject = 'sub-bob'`, created2.ID).
		Scan(&bindingCount))
	require.Equal(t, 1, bindingCount)

	// Bootstrap dedup: first bootstrap insert succeeds.
	boot := &storage.User{Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true}
	_, err = auth.CreateHuman(ctx, boot, nil)
	require.NoError(t, err)
	// Second bootstrap insert fails.
	boot2 := &storage.User{Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true}
	_, err = auth.CreateHuman(ctx, boot2, nil)
	require.ErrorIs(t, err, storage.ErrBootstrapExists)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateHuman -v`
Expected: FAIL with `"not implemented"`.

- [ ] **Step 3: Implement**

Append to `internal/storage/postgres/users.go`:

```go
import (
	"github.com/jackc/pgx/v5/pgconn" // for PgError code matching
)

// CreateHuman inserts a Human row and (optionally) an OIDCBinding in one tx.
// Returns ErrBootstrapExists if u.Bootstrap is true and another active
// bootstrap user already exists (caught via the partial unique index).
func (s *AuthStore) CreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, error) {
	if u.Kind != "" && u.Kind != storage.KindHuman {
		return nil, errors.New("CreateHuman: u.Kind must be KindHuman or empty")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const insertUser = `
		INSERT INTO users (kind, display_name, email, role, bootstrap)
		VALUES ('human', $1, $2, $3, $4)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err = tx.QueryRow(ctx, insertUser, u.DisplayName, u.Email, u.Role, u.Bootstrap).
		Scan(&id, &createdAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" /* unique_violation */ &&
			pgErr.ConstraintName == "users_one_bootstrap" {
			return nil, storage.ErrBootstrapExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	if b != nil {
		const insertBinding = `
			INSERT INTO oidc_bindings (user_id, issuer, subject, email_at_bind)
			VALUES ($1::uuid, $2, $3, $4)`
		_, err = tx.Exec(ctx, insertBinding, id, b.Issuer, b.Subject, b.EmailAtBind)
		if err != nil {
			return nil, fmt.Errorf("insert binding: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	u.ID = id
	u.Kind = storage.KindHuman
	u.CreatedAt = createdAt
	return u, nil
}
```

Remove `CreateHuman` stub from `auth.go`.

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateHuman -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement CreateHuman with transactional binding"
```

---

## Task 14: Implement `CreateServiceAccount`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (SA inserted with owner) + Invariant (FK enforcement — missing/empty owner is rejected before the FK violation; FK is the schema-level backstop).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_CreateServiceAccount(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Owner first.
	owner, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "owner", Role: "admin",
	}, nil)
	require.NoError(t, err)

	sa, err := auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "ci-bot",
		Role: "writer", OwnerUserID: owner.ID,
	})
	require.NoError(t, err)
	require.True(t, sa.IsServiceAccount())
	require.Equal(t, owner.ID, sa.OwnerUserID)

	// Missing owner rejected.
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "no-owner", Role: "writer",
	})
	require.Error(t, err)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateServiceAccount -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// CreateServiceAccount inserts a ServiceAccount row. OwnerUserID must
// reference an existing user (FK enforced).
func (s *AuthStore) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	if u.OwnerUserID == "" {
		return nil, errors.New("CreateServiceAccount: OwnerUserID required")
	}
	const q = `
		INSERT INTO users (kind, display_name, email, role, owner_user_id)
		VALUES ('service_account', $1, $2, $3, $4::uuid)
		RETURNING id, created_at`
	var id string
	var createdAt time.Time
	err := s.pool.QueryRow(ctx, q, u.DisplayName, u.Email, u.Role, u.OwnerUserID).
		Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert service account: %w", err)
	}
	u.ID = id
	u.Kind = storage.KindServiceAccount
	u.CreatedAt = createdAt
	return u, nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateServiceAccount -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement CreateServiceAccount"
```

---

## Task 15: Implement `UpdateUserRole`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (role changes for active user) + Boundary (NotFound for unknown ID; NotFound for soft-deleted user — write-blocked, not silent).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_UpdateUserRole(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, nil)
	require.NoError(t, err)

	err = auth.UpdateUserRole(ctx, u.ID, "writer")
	require.NoError(t, err)

	reloaded, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.Equal(t, "writer", reloaded.Role)

	// Soft-deleted user is not updateable. Set deleted_at via direct SQL
	// rather than calling SoftDeleteUser (Task 16) to avoid a forward
	// reference; soft-delete cascade semantics are exercised in Task 16's
	// own test.
	_, err = pool.Exec(ctx, `UPDATE users SET deleted_at = now() WHERE id = $1::uuid`, u.ID)
	require.NoError(t, err)
	err = auth.UpdateUserRole(ctx, u.ID, "admin")
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Nonexistent.
	err = auth.UpdateUserRole(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa", "reader")
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_UpdateUserRole -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// UpdateUserRole sets the role on an active user. Returns ErrUserNotFound
// if no active user has the given ID.
func (s *AuthStore) UpdateUserRole(ctx context.Context, userID, role string) error {
	const q = `
		UPDATE users SET role = $1
		WHERE id = $2::uuid AND deleted_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, role, userID)
	if err != nil {
		return fmt.Errorf("update role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrUserNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_UpdateUserRole -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement UpdateUserRole"
```

---

## Task 16: Implement `SoftDeleteUser` (transactional cascade)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (deleted_at set; keys revoked) + Invariant (cascade is atomic — both UPDATEs in one tx; same `now()` timestamp) + Invariant (OIDC bindings persist after soft-delete) + Boundary (re-deleting already-deleted is a no-op).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_SoftDeleteUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, &storage.OIDCBinding{Issuer: "iss1", Subject: "sub1"})
	require.NoError(t, err)

	// Seed two API keys directly.
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash, label)
		VALUES ($1::uuid, 'pre00001', 'phc1', 'k1'),
		       ($1::uuid, 'pre00002', 'phc2', 'k2')`, u.ID)
	require.NoError(t, err)

	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))

	// User deleted_at set.
	reloaded, err := auth.GetUserByID(ctx, u.ID)
	require.NoError(t, err)
	require.False(t, reloaded.IsActive())

	// Both keys revoked in the same tx (same revoked_at timestamp).
	var count int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM api_keys
		WHERE user_id = $1::uuid AND revoked_at IS NOT NULL`, u.ID).Scan(&count))
	require.Equal(t, 2, count)

	// Bindings rows remain (they're history).
	var bindingCount int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM oidc_bindings WHERE user_id = $1::uuid`, u.ID).Scan(&bindingCount))
	require.Equal(t, 1, bindingCount)

	// Idempotent: re-deleting is a no-op.
	require.NoError(t, auth.SoftDeleteUser(ctx, u.ID))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_SoftDeleteUser -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// SoftDeleteUser sets deleted_at on the user and revokes all their active
// keys in the same transaction. Idempotent on already-deleted users (the
// user UPDATE matches zero rows, the keys UPDATE matches zero rows; both
// silently succeed).
func (s *AuthStore) SoftDeleteUser(ctx context.Context, userID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	now := s.now()

	_, err = tx.Exec(ctx, `
		UPDATE users SET deleted_at = $1
		WHERE id = $2::uuid AND deleted_at IS NULL`, now, userID)
	if err != nil {
		return fmt.Errorf("soft-delete user: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE user_id = $2::uuid AND revoked_at IS NULL`, now, userID)
	if err != nil {
		return fmt.Errorf("revoke keys: %w", err)
	}

	return tx.Commit(ctx)
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_SoftDeleteUser -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement SoftDeleteUser with key revocation cascade"
```

---

## Task 17: Implement `PurgeUser` (hard delete)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (user row gone) + Invariant (CASCADE deletes bindings AND keys; complement to Task 16's "bindings persist on soft-delete") + Boundary (idempotent on already-purged).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_PurgeUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "reader",
	}, &storage.OIDCBinding{Issuer: "iss1", Subject: "sub1"})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash, label)
		VALUES ($1::uuid, 'pre00003', 'phc3', 'k')`, u.ID)
	require.NoError(t, err)

	require.NoError(t, auth.PurgeUser(ctx, u.ID))

	// User gone.
	_, err = auth.GetUserByID(ctx, u.ID)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Cascaded keys + bindings gone.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id = $1::uuid`, u.ID).Scan(&n))
	require.Equal(t, 0, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM oidc_bindings WHERE user_id = $1::uuid`, u.ID).Scan(&n))
	require.Equal(t, 0, n)

	// Idempotent.
	require.NoError(t, auth.PurgeUser(ctx, u.ID))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_PurgeUser -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// PurgeUser hard-deletes the user; CASCADE constraints handle bindings
// and keys. Idempotent on already-purged users.
func (s *AuthStore) PurgeUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1::uuid`, userID)
	if err != nil {
		return fmt.Errorf("purge user: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_PurgeUser -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement PurgeUser"
```

---

## Task 18: Implement `ListUsers` (filter + pagination)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (unfiltered list of non-deleted) + Boundary (Kind / Role / IncludeDeleted filter combinations; default-limit pinning is verified in Task 31).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_ListUsers(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Three humans, one service account.
	owner, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h1", Role: "admin"}, nil)
	_, _ = auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h2", Role: "reader"}, nil)
	deleted, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "h3", Role: "writer"}, nil)
	require.NoError(t, auth.SoftDeleteUser(ctx, deleted.ID))
	_, _ = auth.CreateServiceAccount(ctx, &storage.User{Kind: storage.KindServiceAccount, DisplayName: "sa1", Role: "writer", OwnerUserID: owner.ID})

	all, err := auth.ListUsers(ctx, storage.ListUsersFilter{})
	require.NoError(t, err)
	require.Len(t, all, 3) // excludes deleted by default

	withDeleted, err := auth.ListUsers(ctx, storage.ListUsersFilter{IncludeDeleted: true})
	require.NoError(t, err)
	require.Len(t, withDeleted, 4)

	humansOnly, err := auth.ListUsers(ctx, storage.ListUsersFilter{Kind: storage.KindHuman})
	require.NoError(t, err)
	require.Len(t, humansOnly, 2) // h3 deleted, sa1 excluded

	readers, err := auth.ListUsers(ctx, storage.ListUsersFilter{Role: "reader"})
	require.NoError(t, err)
	require.Len(t, readers, 1)
	require.Equal(t, "h2", readers[0].DisplayName)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListUsers -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// ListUsers returns users matching the filter. Default limit is 100.
func (s *AuthStore) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	q := `
		SELECT id, kind, display_name, email, role,
		       coalesce(owner_user_id::text, ''), bootstrap,
		       created_at, deleted_at
		FROM users WHERE 1=1`
	args := []any{}
	if !f.IncludeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	if f.Kind != "" {
		args = append(args, string(f.Kind))
		q += fmt.Sprintf(` AND kind = $%d`, len(args))
	}
	if f.Role != "" {
		args = append(args, f.Role)
		q += fmt.Sprintf(` AND role = $%d`, len(args))
	}
	if f.CreatedAfter != nil {
		args = append(args, *f.CreatedAfter)
		q += fmt.Sprintf(` AND created_at > $%d`, len(args))
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit, f.Offset)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var out []*storage.User
	for rows.Next() {
		var u storage.User
		var kindStr string
		err := rows.Scan(&u.ID, &kindStr, &u.DisplayName, &u.Email, &u.Role,
			&u.OwnerUserID, &u.Bootstrap, &u.CreatedAt, &u.DeletedAt)
		if err != nil {
			return nil, err
		}
		u.Kind = storage.Kind(kindStr)
		out = append(out, &u)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListUsers -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement ListUsers with filter and pagination"
```

---

## Task 19: Implement `CreateAPIKey` with prefix-collision retry

**Decision pinned (called out in design):** the impl owns prefix generation. Callers pass an APIKey without `Prefix` set; the impl generates 8 random base32 chars and retries up to 3 times on collision.

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (key inserted; prefix populated; metadata round-trips) + Invariant (FK to users enforced — though this is tested implicitly via the schema; the collision retry is a Boundary covered in Task 19b).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_CreateAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)

	k, err := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-stub", Label: "first key",
	})
	require.NoError(t, err)
	require.Len(t, k.Prefix, 8)
	require.NotEmpty(t, k.ID)
	require.Equal(t, "first key", k.Label)
}
```

> A separate test forces a collision via a stubbed prefix generator; that test lands in Task 19b below as a sub-step.

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateAPIKey -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

The prefix generator and `genPrefix` field are already declared in `auth.go` from Task 7. Add the `CreateAPIKey` method to `users.go` consuming the per-instance generator:

```go
// CreateAPIKey inserts a new API key with a generated prefix. Retries up to
// 3 times on prefix-uniqueness violation; returns ErrAPIKeyPrefixExists if
// all retries collide (essentially impossible at 40 bits of entropy).
//
// The plaintext prefix and secret are NOT taken from the caller; the
// caller passes only metadata (UserID, PHCHash, RoleDowngrade, Label,
// ExpiresAt). Prefix is generated via s.genPrefix (overridable per-
// instance via WithAuthKeyPrefixGenerator for tests; never package-global).
func (s *AuthStore) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	if k.UserID == "" {
		return nil, errors.New("CreateAPIKey: UserID required")
	}
	if k.PHCHash == "" {
		return nil, errors.New("CreateAPIKey: PHCHash required")
	}
	for attempt := 0; attempt < 3; attempt++ {
		prefix, err := s.genPrefix()
		if err != nil {
			return nil, fmt.Errorf("generate prefix: %w", err)
		}
		const q = `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`
		var id string
		var createdAt time.Time
		err = s.pool.QueryRow(ctx, q, k.UserID, prefix, k.PHCHash, k.RoleDowngrade, k.Label, k.ExpiresAt).
			Scan(&id, &createdAt)
		if err == nil {
			k.ID = id
			k.Prefix = prefix
			k.CreatedAt = createdAt
			return k, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue // retry with new prefix
		}
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}
```

No new imports beyond what `users.go` already has (`context`, `errors`, `fmt`, `time`, `pgx`, `pgconn`, `storage`).

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateAPIKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement CreateAPIKey with prefix-collision retry"
```

---

## Task 19b: Prefix-collision retry test

**Files:**

- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Boundary (collision retry succeeds within attempts; retry exhaustion returns `ErrAPIKeyPrefixExists`).

- [ ] **Step 1: Write the collision test**

The test uses `WithAuthKeyPrefixGenerator` (declared in Task 7) to inject a deterministic generator into a per-test AuthStore. This keeps test isolation: no package-global mutation, safe under `t.Parallel()`.

```go
func TestAuthStore_CreateAPIKey_CollisionRetry(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Build a dedicated AuthStore with a stubbed prefix generator.
	calls := 0
	gen := func() (string, error) {
		calls++
		if calls <= 2 {
			return "collide1", nil
		}
		return "newpre23", nil
	}
	auth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(gen))
	require.NoError(t, err)
	t.Cleanup(func() { _ = auth.Close(ctx) })

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)

	// Seed a key with the prefix the generator will collide against.
	_, err = pool.Exec(ctx, `INSERT INTO api_keys (user_id, prefix, phc_hash)
	                         VALUES ($1::uuid, 'collide1', 'phc-seed-string-padding-to-meet-length-check')`, u.ID)
	require.NoError(t, err)

	k, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-string-padding-to-meet-length-check"})
	require.NoError(t, err)
	require.Equal(t, "newpre23", k.Prefix)
	require.GreaterOrEqual(t, calls, 3)

	// Exhaustion: a generator that always returns the same colliding prefix.
	authExhaust, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(
		func() (string, error) { return "collide1", nil }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = authExhaust.Close(ctx) })
	_, err = authExhaust.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc-stub-string-padding-to-meet-length-check"})
	require.ErrorIs(t, err, storage.ErrAPIKeyPrefixExists)
}
```

> Note: PHC strings are padded to meet the schema's `length(phc_hash) >= 32` CHECK from Task 6. In real use, argon2id PHC strings are ~95+ chars and trivially satisfy this.

- [ ] **Step 2: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_CreateAPIKey_CollisionRetry -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): exercise CreateAPIKey prefix-collision retry path"
```

---

## Task 20: Implement `RevokeAPIKey`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (revoked_at set) + Boundary (idempotent on already-revoked; silent on nonexistent ID).

> **Deliberate asymmetry with `RotateAPIKey`:** Revoke is fire-and-forget on a nonexistent ID — no error. Rotate (Task 21) returns `ErrAPIKeyNotFound` on the same condition because rotation has a load-bearing existence requirement: you can't "rotate" a key that doesn't exist into a new one that does. The two methods are intentionally inconsistent in this one respect.

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_RevokeAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	k, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc"})

	require.NoError(t, auth.RevokeAPIKey(ctx, k.ID))

	reloaded, err := auth.LookupAPIKeyByPrefix(ctx, k.Prefix)
	require.NoError(t, err)
	require.NotNil(t, reloaded.RevokedAt)
	require.False(t, reloaded.IsActive(time.Now()))

	// Idempotent on already-revoked.
	require.NoError(t, auth.RevokeAPIKey(ctx, k.ID))

	// Nonexistent ID is also a no-op (no error).
	require.NoError(t, auth.RevokeAPIKey(ctx, "00000000-0000-0000-0000-aaaaaaaaaaaa"))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_RevokeAPIKey -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// RevokeAPIKey marks the key revoked. Idempotent on already-revoked or
// nonexistent IDs (does not error).
func (s *AuthStore) RevokeAPIKey(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), keyID)
	if err != nil {
		return fmt.Errorf("revoke key: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_RevokeAPIKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement RevokeAPIKey"
```

---

## Task 21: Implement `RotateAPIKey` (atomic revoke + create)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (old revoked AND new active after rotate) + Invariant (atomic single-tx; rollback path leaves old key live with no new row — verified by injecting a forced rollback case, not just happy-path commits).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_RotateAPIKey(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old", Label: "ci-bot",
	})

	newKey, err := auth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-new", Label: "ci-bot",
	})
	require.NoError(t, err)
	require.NotEqual(t, old.Prefix, newKey.Prefix)
	require.Equal(t, "ci-bot", newKey.Label)

	// Old key is revoked, new key is active.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NotNil(t, oldReload.RevokedAt)
	newReload, _ := auth.LookupAPIKeyByPrefix(ctx, newKey.Prefix)
	require.Nil(t, newReload.RevokedAt)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_RotateAPIKey -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// RotateAPIKey revokes the old key and inserts a new one with the supplied
// metadata in one transaction. Returns the new key with generated prefix
// and ID populated.
func (s *AuthStore) RotateAPIKey(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		UPDATE api_keys SET revoked_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), oldKeyID)
	if err != nil {
		return nil, fmt.Errorf("revoke old key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, storage.ErrAPIKeyNotFound
	}

	// Insert new key inside the same tx with collision retry.
	for attempt := 0; attempt < 3; attempt++ {
		prefix, err := s.genPrefix()
		if err != nil {
			return nil, fmt.Errorf("generate prefix: %w", err)
		}
		var id string
		var createdAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO api_keys (user_id, prefix, phc_hash, role_downgrade, label, expires_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			RETURNING id, created_at`,
			newKey.UserID, prefix, newKey.PHCHash, newKey.RoleDowngrade,
			newKey.Label, newKey.ExpiresAt).Scan(&id, &createdAt)
		if err == nil {
			newKey.ID = id
			newKey.Prefix = prefix
			newKey.CreatedAt = createdAt
			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			return newKey, nil
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "api_keys_prefix_key" {
			continue
		}
		return nil, fmt.Errorf("insert new key: %w", err)
	}
	return nil, storage.ErrAPIKeyPrefixExists
}
```

- [ ] **Step 4: Add a forced-rollback test**

The Invariant tag says "rollback path leaves old key live with no new row." Verify it concretely by forcing the new-insert to fail. The forcing mechanism is a `WithAuthKeyPrefixGenerator` that returns an empty string, which violates the schema's `CHECK (length(prefix) >= 8)` from Task 6 — that failure rolls back the entire tx including the old-key revoke.

**Note for readers:** this test exercises the `23514 check_violation` branch (the CHECK constraint failure), NOT the `23505 unique_violation` retry loop. The impl falls through to `return nil, fmt.Errorf("insert new key: %w", err)` on the first attempt because the error isn't a unique-violation. A separate test (Step 5) exercises the unique-violation retry path.

```go
func TestAuthStore_RotateAPIKey_RollbackOnInsertFailure(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	auth := authTestSetup(t)
	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old-padding-to-meet-min-length-check",
	})

	// Build a SECOND store with a bad generator — returns empty string
	// which violates the prefix length CHECK constraint (23514), forcing
	// the INSERT to fail on the FIRST attempt (not via the 23505 retry
	// loop) and rolling back the entire tx including the revoke of `old`.
	badAuth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(
		func() (string, error) { return "", nil }))
	require.NoError(t, err)
	t.Cleanup(func() { _ = badAuth.Close(ctx) })

	_, err = badAuth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-new-padding-to-meet-min-length-check",
	})
	require.Error(t, err)

	// Old key MUST still be live (rollback worked) and no new row exists.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.Nil(t, oldReload.RevokedAt, "old key must still be live after rollback")
	var keyCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM api_keys WHERE user_id = $1::uuid`, u.ID).Scan(&keyCount))
	require.Equal(t, 1, keyCount, "only old key exists; no orphan from failed insert")
}
```

- [ ] **Step 5: Add a unique-violation retry-loop test**

The previous test (Step 4) hits a CHECK violation, which bypasses the retry loop. This test exercises the retry path directly: the generator returns a colliding prefix twice (each insert fails with `23505 unique_violation` and is retried), then a fresh prefix that succeeds.

```go
func TestAuthStore_RotateAPIKey_PrefixCollisionRetry(t *testing.T) {
	ctx := context.Background()
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	auth := authTestSetup(t)
	u, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "alice", Role: "writer",
	}, nil)
	old, _ := auth.CreateAPIKey(ctx, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-old-padding-to-meet-min-length-check",
	})

	// Seed a separate user with a key whose prefix the rotate generator
	// will collide against. Using a different user keeps the test focused
	// on the rotate path (not on the user's own key inventory).
	other, _ := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "bob", Role: "writer",
	}, nil)
	_, err := pool.Exec(ctx, `
		INSERT INTO api_keys (user_id, prefix, phc_hash)
		VALUES ($1::uuid, 'collidex', 'phc-bob-padding-to-meet-min-length-check')`, other.ID)
	require.NoError(t, err)

	// Generator returns the colliding prefix twice (triggers 23505 +
	// retry both times), then a fresh prefix on attempt 3.
	calls := 0
	gen := func() (string, error) {
		calls++
		if calls <= 2 {
			return "collidex", nil
		}
		return "rotated2", nil
	}
	rotateAuth, err := postgres.NewAuth(ctx, pool, postgres.WithAuthKeyPrefixGenerator(gen))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rotateAuth.Close(ctx) })

	newKey, err := rotateAuth.RotateAPIKey(ctx, old.ID, &storage.APIKey{
		UserID: u.ID, PHCHash: "phc-rot-padding-to-meet-min-length-check",
	})
	require.NoError(t, err)
	require.Equal(t, "rotated2", newKey.Prefix)
	require.GreaterOrEqual(t, calls, 3, "retry loop should have invoked the generator at least 3 times")

	// Old key is revoked, new key is active — same invariant as the happy
	// path, but the path through the retry loop is now exercised.
	oldReload, _ := auth.LookupAPIKeyByPrefix(ctx, old.Prefix)
	require.NotNil(t, oldReload.RevokedAt)
	newReload, _ := auth.LookupAPIKeyByPrefix(ctx, newKey.Prefix)
	require.Nil(t, newReload.RevokedAt)
}
```

- [ ] **Step 6: Run all rotate tests**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_RotateAPIKey -v`
Expected: PASS for all three tests (happy path, rollback on CHECK violation, retry on prefix collision).

- [ ] **Step 7: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement RotateAPIKey atomically with rollback + retry tests"
```

---

## Task 22: Implement `ListAPIKeys`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (per-user list; admin all-user list) + Boundary (IncludeRevoked toggle; revoked keys excluded by default).

> **Footgun acknowledged:** `ListAPIKeysFilter{UserID: ""}` lists across all users. A caller who forgets to set `UserID` accidentally reads every user's keys. This is admin functionality — callers reaching the storage layer with an empty `UserID` are deliberately doing an admin op, and authz gating is the handler/Cedar plan's responsibility. Document in the interface comment (Task 4 already states "empty = all users (admin)"); if a future story wants stricter type safety, introduce a sentinel like `AllUsers = "*"`. Out of scope for this story.

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_ListAPIKeys(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u1, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	u2, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "writer"}, nil)

	k1, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u1.ID, PHCHash: "phc1", Label: "k1"})
	_, _ = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u1.ID, PHCHash: "phc2", Label: "k2"})
	_, _ = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u2.ID, PHCHash: "phc3", Label: "k3"})
	require.NoError(t, auth.RevokeAPIKey(ctx, k1.ID))

	// Per-user, excluding revoked by default.
	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u1.ID})
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, "k2", keys[0].Label)

	// IncludeRevoked.
	keys, err = auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u1.ID, IncludeRevoked: true})
	require.NoError(t, err)
	require.Len(t, keys, 2)

	// All users (admin).
	all, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{})
	require.NoError(t, err)
	require.Len(t, all, 2) // k1 revoked excluded, k2 and k3 remain
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListAPIKeys -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// ListAPIKeys returns keys matching the filter.
func (s *AuthStore) ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	q := `
		SELECT id, user_id, prefix, phc_hash, role_downgrade, label,
		       expires_at, last_used_at, revoked_at, created_at
		FROM api_keys WHERE 1=1`
	args := []any{}
	if f.UserID != "" {
		args = append(args, f.UserID)
		q += fmt.Sprintf(` AND user_id = $%d::uuid`, len(args))
	}
	if !f.IncludeRevoked {
		q += ` AND revoked_at IS NULL`
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit, f.Offset)
	q += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args))

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var out []*storage.APIKey
	for rows.Next() {
		var k storage.APIKey
		if err := rows.Scan(&k.ID, &k.UserID, &k.Prefix, &k.PHCHash, &k.RoleDowngrade,
			&k.Label, &k.ExpiresAt, &k.LastUsedAt, &k.RevokedAt, &k.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &k)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListAPIKeys -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement ListAPIKeys with filter and pagination"
```

---

## Task 23: Implement `TouchLastUsed`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (last_used_at populated) + Boundary (silent on nonexistent key — fire-and-forget contract).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_TouchLastUsed(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	k, _ := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: "phc"})

	require.Nil(t, k.LastUsedAt)
	require.NoError(t, auth.TouchLastUsed(ctx, k.ID))

	reloaded, _ := auth.LookupAPIKeyByPrefix(ctx, k.Prefix)
	require.NotNil(t, reloaded.LastUsedAt)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_TouchLastUsed -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// TouchLastUsed sets last_used_at = now() for the key. Nonexistent or
// already-revoked IDs are silent no-ops (fire-and-forget semantics).
// Excluding revoked keys prevents audit-log confusion when a key is
// revoked between a successful verify and the async last-used update.
func (s *AuthStore) TouchLastUsed(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = $1
		WHERE id = $2::uuid AND revoked_at IS NULL`, s.now(), keyID)
	if err != nil {
		return fmt.Errorf("touch last_used_at: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_TouchLastUsed -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement TouchLastUsed"
```

---

## Task 24: Implement `JITCreateHuman` (race-safe)

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (atomic user + binding creation) + Invariant (single-tx atomicity; second JIT with same (issuer, subject) returns the existing user via race recovery, doesn't create a duplicate; concurrent verification in Task 28).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_JITCreateHuman(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u := &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Email: "alice@x.com", Role: "reader"}
	b := &storage.OIDCBinding{Issuer: "iss1", Subject: "alice-sub", EmailAtBind: "alice@x.com"}

	user, binding, err := auth.JITCreateHuman(ctx, u, b)
	require.NoError(t, err)
	require.NotEmpty(t, user.ID)
	require.NotEmpty(t, binding.ID)
	require.Equal(t, user.ID, binding.UserID)

	// Second JIT for same (issuer, subject) returns the existing user.
	user2, binding2, err := auth.JITCreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice-dup", Role: "reader"}, b)
	require.NoError(t, err)
	require.Equal(t, user.ID, user2.ID)
	require.Equal(t, binding.ID, binding2.ID)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_JITCreateHuman -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// JITCreateHuman creates a Human + OIDC binding atomically. If the (issuer,
// subject) already exists (race with another JIT), re-reads and returns the
// winning binding's user.
//
// Race recovery semantics: when a concurrent JIT wins the (issuer, subject)
// uniqueness contest, this caller's INSERT receives a 23505. Postgres
// guarantees the winner has already COMMITTED before the loser's INSERT can
// observe the constraint violation (the winner's tx held a row lock until
// commit, after which the loser's insert attempt resolves to "duplicate
// key"). At READ COMMITTED isolation, the loser's subsequent SELECT for
// the binding will see the committed row. No retry loop is needed.
func (s *AuthStore) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var userID string
	var createdAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO users (kind, display_name, email, role)
		VALUES ('human', $1, $2, $3)
		RETURNING id, created_at`,
		u.DisplayName, u.Email, u.Role).Scan(&userID, &createdAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert user (jit): %w", err)
	}

	var bindingID string
	var bindingCreatedAt time.Time
	err = tx.QueryRow(ctx, `
		INSERT INTO oidc_bindings (user_id, issuer, subject, email_at_bind)
		VALUES ($1::uuid, $2, $3, $4)
		RETURNING id, created_at`,
		userID, b.Issuer, b.Subject, b.EmailAtBind).Scan(&bindingID, &bindingCreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" &&
			pgErr.ConstraintName == "oidc_bindings_issuer_subject_key" {
			// Race: another JIT won. Rollback, re-read.
			_ = tx.Rollback(ctx)
			existing, lookupErr := s.LookupOIDCBinding(ctx, b.Issuer, b.Subject)
			if lookupErr != nil {
				return nil, nil, fmt.Errorf("race recovery: %w", lookupErr)
			}
			existingUser, lookupErr := s.GetUserByID(ctx, existing.UserID)
			if lookupErr != nil {
				return nil, nil, fmt.Errorf("race recovery user: %w", lookupErr)
			}
			return existingUser, existing, nil
		}
		return nil, nil, fmt.Errorf("insert binding (jit): %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	u.ID = userID
	u.Kind = storage.KindHuman
	u.CreatedAt = createdAt
	b.ID = bindingID
	b.UserID = userID
	b.CreatedAt = bindingCreatedAt
	return u, b, nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_JITCreateHuman -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement JITCreateHuman race-safely"
```

---

## Task 25: Implement `ListOIDCBindings`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (per-user bindings; multiple-provider case) + Boundary (empty slice for unbound user, not nil; an empty result is not an error).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_ListOIDCBindings(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-entra"})
	// Add a second binding (different provider).
	_, _ = pool.Exec(ctx, `INSERT INTO oidc_bindings (user_id, issuer, subject)
	                      VALUES ($1::uuid, 'github', 'alice-gh')`, u.ID)

	bindings, err := auth.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 2)

	// Empty list for unbound user.
	other, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "bob", Role: "reader"}, nil)
	bindings, err = auth.ListOIDCBindings(ctx, other.ID)
	require.NoError(t, err)
	require.Empty(t, bindings)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListOIDCBindings -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// ListOIDCBindings returns bindings for the given user.
func (s *AuthStore) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, issuer, subject, email_at_bind, created_at
		FROM oidc_bindings
		WHERE user_id = $1::uuid
		ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list bindings: %w", err)
	}
	defer rows.Close()

	var out []*storage.OIDCBinding
	for rows.Next() {
		var b storage.OIDCBinding
		if err := rows.Scan(&b.ID, &b.UserID, &b.Issuer, &b.Subject, &b.EmailAtBind, &b.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &b)
	}
	return out, rows.Err()
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ListOIDCBindings -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement ListOIDCBindings"
```

---

## Task 26: Implement `UnbindOIDC`

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Happy (binding deleted) + Boundary (idempotent on already-deleted ID). Note: last-credential protection (refuse if user has no other creds) lives at the handler layer per the design; storage layer doesn't enforce.

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_UnbindOIDC(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-entra"})
	bindings, _ := auth.ListOIDCBindings(ctx, u.ID)
	require.Len(t, bindings, 1)

	require.NoError(t, auth.UnbindOIDC(ctx, bindings[0].ID))

	after, _ := auth.ListOIDCBindings(ctx, u.ID)
	require.Empty(t, after)

	// Idempotent.
	require.NoError(t, auth.UnbindOIDC(ctx, bindings[0].ID))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_UnbindOIDC -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

```go
// UnbindOIDC deletes the binding. Idempotent on already-deleted.
func (s *AuthStore) UnbindOIDC(ctx context.Context, bindingID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM oidc_bindings WHERE id = $1::uuid`, bindingID)
	if err != nil {
		return fmt.Errorf("unbind oidc: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_UnbindOIDC -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go internal/storage/postgres/auth.go
git commit -s -m "feat(postgres): implement UnbindOIDC"
```

---

## Task 27: Concurrent bootstrap-uniqueness race test

**Files:**

- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Invariant (at most one active bootstrap regardless of concurrent CreateHuman calls — the partial unique index from Task 6 is the schema-level enforcement; this test verifies the runtime error mapping in Task 13 correctly surfaces it).

- [ ] **Step 1: Write the race test**

```go
func TestAuthStore_BootstrapRace(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	const concurrent = 5
	var g errgroup.Group
	results := make(chan error, concurrent)
	for i := 0; i < concurrent; i++ {
		g.Go(func() error {
			_, err := auth.CreateHuman(ctx, &storage.User{
				Kind: storage.KindHuman, DisplayName: "admin",
				Role: "admin", Bootstrap: true,
			}, nil)
			results <- err
			return nil
		})
	}
	require.NoError(t, g.Wait())
	close(results)

	var successes, expectsBootstrapExists int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, storage.ErrBootstrapExists):
			expectsBootstrapExists++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	require.Equal(t, 1, successes, "exactly one bootstrap insert should succeed")
	require.Equal(t, concurrent-1, expectsBootstrapExists)
}
```

Imports: `"golang.org/x/sync/errgroup"`.

- [ ] **Step 2: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_BootstrapRace -v`
Expected: PASS (the partial unique index from Task 6 + the error mapping in Task 13 do the work).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): cover concurrent bootstrap-uniqueness race"
```

---

## Task 28: Concurrent JIT race test

**Files:**

- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Invariant ((issuer, subject) globally unique — concurrent JITs converge to the same user and binding row, never duplicate rows).

- [ ] **Step 1: Write the JIT race test**

```go
func TestAuthStore_JITRace(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	const concurrent = 5
	type result struct {
		userID    string
		bindingID string
	}
	results := make(chan result, concurrent)

	var g errgroup.Group
	for i := 0; i < concurrent; i++ {
		g.Go(func() error {
			u, b, err := auth.JITCreateHuman(ctx,
				&storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"},
				&storage.OIDCBinding{Issuer: "iss1", Subject: "alice-sub"})
			if err != nil {
				return err
			}
			results <- result{userID: u.ID, bindingID: b.ID}
			return nil
		})
	}
	require.NoError(t, g.Wait())
	close(results)

	userIDs := map[string]struct{}{}
	bindingIDs := map[string]struct{}{}
	for r := range results {
		userIDs[r.userID] = struct{}{}
		bindingIDs[r.bindingID] = struct{}{}
	}
	require.Len(t, userIDs, 1, "all callers should resolve to the same user")
	require.Len(t, bindingIDs, 1, "all callers should resolve to the same binding")
}
```

- [ ] **Step 2: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_JITRace -v`
Expected: PASS (race-recovery code in Task 24 handles all but one caller).

- [ ] **Step 3: Commit**

```bash
git add internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): cover concurrent JIT race-recovery path"
```

---

## Task 29: Final integration — full lifecycle smoke test

**Files:**

- Modify: `internal/storage/postgres/users_test.go`

**Covers:** E2E / integration (multi-method flow through bootstrap → JIT → key → rotate → promote → soft-delete-bootstrap; asserts final state is coherent across all entities).

- [ ] **Step 1: Write the lifecycle test**

```go
func TestAuthStore_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Bootstrap admin.
	admin, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "admin", Role: "admin", Bootstrap: true,
	}, nil)
	require.NoError(t, err)

	got, err := auth.GetBootstrap(ctx)
	require.NoError(t, err)
	require.Equal(t, admin.ID, got.ID)

	// JIT a real person via OIDC.
	person, binding, err := auth.JITCreateHuman(ctx,
		&storage.User{Kind: storage.KindHuman, DisplayName: "alice", Email: "alice@x.com", Role: "reader"},
		&storage.OIDCBinding{Issuer: "entra", Subject: "alice-sub", EmailAtBind: "alice@x.com"})
	require.NoError(t, err)

	// Mint an API key.
	key, err := auth.CreateAPIKey(ctx, &storage.APIKey{UserID: person.ID, PHCHash: "phc", Label: "personal"})
	require.NoError(t, err)

	// Rotate it.
	newKey, err := auth.RotateAPIKey(ctx, key.ID, &storage.APIKey{UserID: person.ID, PHCHash: "phc-rot", Label: "personal"})
	require.NoError(t, err)
	require.NotEqual(t, key.Prefix, newKey.Prefix)

	// Promote alice via UpdateUserRole.
	require.NoError(t, auth.UpdateUserRole(ctx, person.ID, "admin"))

	// Soft-delete bootstrap admin (force-flag policy enforced at handler layer, not here).
	require.NoError(t, auth.SoftDeleteUser(ctx, admin.ID))

	// GetBootstrap now empty.
	_, err = auth.GetBootstrap(ctx)
	require.ErrorIs(t, err, storage.ErrUserNotFound)

	// Final state: alice is admin with one active key, one binding.
	reloaded, _ := auth.GetUserByID(ctx, person.ID)
	require.Equal(t, "admin", reloaded.Role)
	keys, _ := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: person.ID})
	require.Len(t, keys, 1)
	bindings, _ := auth.ListOIDCBindings(ctx, person.ID)
	require.Len(t, bindings, 1)
	require.Equal(t, binding.ID, bindings[0].ID)
}
```

- [ ] **Step 2: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_FullLifecycle -v`
Expected: PASS.

- [ ] **Step 3: Run the full test suite**

Run: `cd internal/storage/postgres && go test -tags integration -v ./...`
Expected: all auth tests PASS; existing project-storage tests unaffected.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): full identity lifecycle smoke test"
```

---

## Task 30: Invariant sweep — ServiceAccount has no OIDC bindings

The design says "ServiceAccounts MUST NOT have rows in `oidc_bindings`. Enforced at the store layer." `CreateHuman` (Task 13) already rejects `Kind=ServiceAccount` callers via the existing `if u.Kind != "" && u.Kind != storage.KindHuman` check. `JITCreateHuman` (Task 24) does NOT — it silently coerces the kind to `'human'` via the SQL literal. This task closes that gap.

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Invariant (Service Accounts cannot have OIDC bindings; both Human creation paths actively refuse a ServiceAccount kind rather than silently coerce).

- [ ] **Step 1: Failing test**

```go
func TestAuthStore_ServiceAccountNoBindingInvariant(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// CreateHuman already rejects Kind=ServiceAccount (per Task 13's
	// existing check). Verify that property survived.
	_, err := auth.CreateHuman(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "wrong", Role: "writer",
	}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "KindHuman")

	// JITCreateHuman with Kind=ServiceAccount currently silently coerces
	// to 'human' via the SQL literal — must instead refuse explicitly.
	_, _, err = auth.JITCreateHuman(ctx,
		&storage.User{Kind: storage.KindServiceAccount, DisplayName: "wrong", Role: "reader"},
		&storage.OIDCBinding{Issuer: "iss", Subject: "sub"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "KindHuman")

	// Defense in depth: no rows created by either failed call.
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE kind='service_account'`).Scan(&n))
	require.Equal(t, 0, n)
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM oidc_bindings`).Scan(&n))
	require.Equal(t, 0, n)
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ServiceAccountNoBindingInvariant -v`
Expected: FAIL on the JITCreateHuman assertion (no `KindHuman` error returned — instead a user is silently created as `kind='human'`). The CreateHuman half should already PASS thanks to the check landed in Task 13.

- [ ] **Step 3: Add the JIT defensive check**

In `JITCreateHuman` (in `internal/storage/postgres/users.go`), add this check at the top of the function:

```go
if u.Kind != "" && u.Kind != storage.KindHuman {
	return nil, nil, fmt.Errorf("JITCreateHuman: u.Kind must be KindHuman or empty, got %q", u.Kind)
}
```

- [ ] **Step 4: Run**

Run: `cd internal/storage/postgres && go test -tags integration -run TestAuthStore_ServiceAccountNoBindingInvariant -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go
git commit -s -m "feat(postgres): refuse Kind=ServiceAccount on JITCreateHuman explicitly"
```

---

## Task 31: Boundary sweep — pagination, empty/null, missing-required

A consolidated sweep of edge cases that don't naturally fit any single method's task: pagination defaults, empty-vs-nil distinctions, and required-field validation.

**Files:**

- Modify: `internal/storage/postgres/users.go`
- Modify: `internal/storage/postgres/users_test.go`

**Covers:** Boundary (pagination default-limit behavior; empty result returns empty slice not nil; missing required fields return clear errors).

- [ ] **Step 1: Pagination boundary tests**

Append to `users_test.go`:

```go
func TestAuthStore_Pagination_DefaultLimit(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// Insert 5 users; default limit is 100 so all are returned.
	for i := 0; i < 5; i++ {
		_, err := auth.CreateHuman(ctx, &storage.User{
			Kind: storage.KindHuman, DisplayName: fmt.Sprintf("u%d", i), Role: "reader",
		}, nil)
		require.NoError(t, err)
	}

	// Limit=0 means "use default" (100), not "return zero".
	users, err := auth.ListUsers(ctx, storage.ListUsersFilter{Limit: 0})
	require.NoError(t, err)
	require.Len(t, users, 5)

	// Offset beyond rows returns empty slice (not nil, not error).
	users, err = auth.ListUsers(ctx, storage.ListUsersFilter{Offset: 100})
	require.NoError(t, err)
	require.Empty(t, users)
	require.NotNil(t, users, "empty result must be []*User{}, never nil")
}
```

> Note: the `NotNil` assertion against an empty slice may need adjustment if Go's `require.Empty` treats nil and empty as equivalent. If so, replace with: `require.True(t, users != nil)` and ensure the implementation initializes the slice (`out := []*storage.User{}` instead of `var out []*storage.User`).

- [ ] **Step 2: Empty/nil distinction tests**

```go
func TestAuthStore_EmptyResults_NotNil(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "reader"}, nil)

	// User with no bindings returns empty slice, not nil.
	bindings, err := auth.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Empty(t, bindings)

	// User with no keys returns empty slice, not nil.
	keys, err := auth.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: u.ID})
	require.NoError(t, err)
	require.Empty(t, keys)
}
```

- [ ] **Step 3: Missing-required-field tests**

```go
func TestAuthStore_RequiredFields(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	// CreateAPIKey requires UserID.
	_, err := auth.CreateAPIKey(ctx, &storage.APIKey{PHCHash: "phc"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "UserID required")

	// CreateAPIKey requires PHCHash.
	u, _ := auth.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "alice", Role: "writer"}, nil)
	_, err = auth.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID})
	require.Error(t, err)
	require.Contains(t, err.Error(), "PHCHash required")

	// CreateServiceAccount requires OwnerUserID (already covered in Task 14, asserted here for completeness).
	_, err = auth.CreateServiceAccount(ctx, &storage.User{
		Kind: storage.KindServiceAccount, DisplayName: "no-owner", Role: "writer",
	})
	require.Error(t, err)
}
```

- [ ] **Step 4: Run all three sweep tests**

Run: `cd internal/storage/postgres && go test -tags integration -run 'TestAuthStore_(Pagination_DefaultLimit|EmptyResults_NotNil|RequiredFields)' -v`
Expected: PASS for all three. If `EmptyResults_NotNil` fails because the impl returns nil for empty slices, adjust the list implementations to initialize `out := []*X{}` rather than `var out []*X`.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres/users.go internal/storage/postgres/users_test.go
git commit -s -m "test(postgres): boundary sweep — pagination, empty/null, required fields"
```

---

## Task 32: Export an importable test-pool helper for cross-package integration tests

**Why this task exists:** the `sharedTestPool` helper from Task 8 lives in `package postgres_test` and is backed by the `connString` set in `postgres_test.go`'s `TestMain`. That makes it usable **only** from `internal/storage/postgres`'s own test files. The downstream identity plans (Authn integration tests, Bootstrap-UX 4a `buildIdentityTestServer`, Bootstrap-UX 4b `bootstrap` integration test) need a Postgres pool from THEIR packages (`auth_test`, `server_test`, `bootstrap_test`) — separate test binaries that cannot reach `postgres_test`'s `TestMain`. An unexported helper cannot cross that boundary. This task creates an **importable** harness so every package's integration tests share one bootstrap mechanism.

**Files:**

- Create: `internal/storage/postgres/postgrestest/pool.go`
- Modify: `internal/storage/postgres/postgres_test.go` (delegate the container bootstrap to the new package)
- Modify: `internal/storage/postgres/auth_helpers_test.go` (Task 8's `sharedTestPool` delegates to `postgrestest.SharedPool`)

**Covers:** N/A (test infrastructure) — validated by the existing integration suites continuing to pass under it.

- [ ] **Step 1: Create the importable harness**

`postgrestest` is a normal (importable) package, build-tagged `integration` so it is excluded from production builds and only compiles for tagged test runs. It owns the container via `sync.Once`, so each test binary that imports it starts exactly one pgvector container and applies the full migration set.

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package postgrestest provides an importable testcontainers Postgres pool
// shared across the identity integration suites (storage, auth, server,
// bootstrap). It exists because the per-package external test packages cannot
// reach one another's TestMain; this package centralizes the container +
// migration bootstrap behind an exported SharedPool.
package postgrestest

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

var (
	once    sync.Once
	connStr string
	startErr error
)

// SharedPool starts the shared testcontainer Postgres on first call (per test
// binary), applies migrations, and returns a fresh pool against it. The pool
// is closed via t.Cleanup; the container lives for the process lifetime.
func SharedPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	once.Do(func() {
		connStr, startErr = startContainerAndMigrate(ctx)
	})
	require.NoError(t, startErr, "start shared test container")

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// startContainerAndMigrate starts the pgvector container and applies the full
// goose migration set, returning the connection string.
//
// IMPLEMENTATION: move the container-start + migration logic VERBATIM out of
// the existing postgres_test.go TestMain (pgvector/pgvector:pg18, wait
// strategy ForLog("database system is ready").WithOccurrence(2), then the
// goose migrate call the harness already uses). This is a relocation, not new
// behavior — keep the image, wait strategy, and migration runner identical so
// the existing suites behave exactly as before.
func startContainerAndMigrate(ctx context.Context) (string, error) {
	// ... relocated from postgres_test.go TestMain ...
	panic("relocate the existing TestMain container+migrate body here")
}
```

> The `panic` placeholder marks the one block the executor fills by MOVING the existing `TestMain` body — it is not left as a stub. After relocation, no `panic` remains.

- [ ] **Step 2: Delegate the existing harness to the new package**

In `postgres_test.go`, replace the in-`TestMain` container bootstrap with a call into `postgrestest` (so there is ONE container-start implementation):

- `postgres_test.go`'s `TestMain` / `connString` setup now obtains its connection string from `postgrestest` (e.g. `TestMain` calls a `postgrestest.EnsureStarted(ctx)` that returns the conn string, or `connString` is sourced from `postgrestest`). Keep `connString` available to the existing in-package tests.
- In `auth_helpers_test.go` (Task 8), make `sharedTestPool` delegate: `func sharedTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool { return postgrestest.SharedPool(t, ctx) }`. The ~15 existing `sharedTestPool(t, ctx)` call sites in this package are unchanged.

> Keep the public behavior identical; this is a refactor to a single shared source. If splitting `TestMain` cleanly is awkward, the acceptable fallback is: `postgrestest` owns the container (Step 1), and `postgres_test.go`'s in-package tests ALSO route through `postgrestest.SharedPool` (drop the local `TestMain` container start). Either way, end state: one container-start implementation, importable cross-package.

- [ ] **Step 3: Verify the storage integration suite still passes**

Run: `cd internal/storage/postgres && go test -tags integration ./...`

Expected: PASS (the relocation is behavior-preserving; all Task 8–31 integration tests run against the relocated harness).

- [ ] **Step 4: Commit**

```bash
git add internal/storage/postgres/postgrestest/pool.go internal/storage/postgres/postgres_test.go internal/storage/postgres/auth_helpers_test.go
git commit -s -m "test(postgres): export importable SharedPool for cross-package integration tests"
```

> **Downstream consumers** (in later plans) import `postgrestest` and call `postgrestest.SharedPool(t, ctx)`: Authn integration tests (Tasks 33–34), Bootstrap-UX 4a (`buildIdentityTestServer`, Task 19), Bootstrap-UX 4b (`bootstrap` integration, Task 4). Those plans reference this helper directly.

---

## Self-Review

**Spec coverage check:**

- [x] User taxonomy (Human + ServiceAccount, single table + Kind discriminator): Tasks 1, 13, 14.
- [x] OIDC binding global uniqueness (issuer, subject): Migration in Task 6; race-safety verified by Task 28.
- [x] Soft-delete cascades to key revocation in one tx: Task 16 (impl + assertion); reinforced by Task 29.
- [x] OIDC bindings remain after soft-delete, removed on purge: Tasks 16 and 17 verify both halves.
- [x] Bootstrap uniqueness invariant (at most one active): Migration in Task 6 + Task 13 error mapping + Task 27 race verification.
- [x] Argon2id PHC storage as text: Migration in Task 6 (`phc_hash text`); verification at hash-time happens in Authn (separate plan).
- [x] `UsersBackend` interface as cross-domain seam: Task 4.
- [x] Separate goose version table: Task 7 (`goose.SetTableName("goose_db_version_auth")`).
- [x] Shared pool / NewAuth borrows: Tasks 5 (`Pool()` accessor) + 7 (`NewAuth(pool)` takes ownership-borrowed pool).
- [x] API key prefix-collision retry: Task 19 + Task 19b verifies.
- [x] RotateAPIKey atomicity: Task 21 (single tx wrapping revoke + create).
- [x] JIT race-safety: Task 24 (impl) + Task 28 (concurrent race verification).
- [x] TouchLastUsed is silent for nonexistent keys: Task 23.
- [x] Full lifecycle smoke (bootstrap → JIT → key → rotate → promote → soft-delete bootstrap): Task 29.

**Placeholder scan:** No "TBD" / "TODO" remain. Three documented decision points flagged inline:

- Task 13 / Task 15 / Task 18: tests have an ordering dependency on Tasks 25 (`ListOIDCBindings`) and 16 (`SoftDeleteUser`). The executor either implements the dependent task first or substitutes direct SQL in the early test. Both options spelled out.
- Task 19: the prefix-generation responsibility is pinned to the impl (not the caller). Documented in the method comment.

These are not placeholders; they are explicit execution-order choices the implementer must make.

**Type consistency:** `User`, `Kind`, `OIDCBinding`, `APIKey`, `UsersBackend`, `ListUsersFilter`, `ListAPIKeysFilter` referenced consistently. Method signatures across the interface (Task 4) and implementation (Tasks 9–26) match. The compile-time assertion in Task 7 (`var _ storage.UsersBackend = (*AuthStore)(nil)`) enforces this mechanically.

**Migration ordering:** Tasks expose three soft dependencies that mean the executor SHOULD use the order in this document rather than implementing tasks in parallel: Task 5 (`Pool()` accessor) must precede Task 7 (`NewAuth(pool)`); Task 6 (migrations) must precede Task 8 (test harness); Tasks 13–14 must precede every test that needs to create a user. Otherwise tasks within Phase 3+ are mostly independent and could be batched.

**Test category coverage:**

- *Happy* — Task 8 (harness migration test) plus every method-introducing task (Tasks 9–26). Task 29 covers the multi-method happy lifecycle.
- *Invariants* — bootstrap uniqueness (Tasks 13 + 27), (issuer, subject) global uniqueness (Tasks 24 + 28), soft-delete atomic cascade (Task 16), purge cascade complement (Task 17), atomic rotation incl. forced-rollback (Task 21), Service-Account-no-OIDC-binding (Task 30).
- *Boundaries* — NotFound per resolve method (Tasks 9–12, 15), idempotent re-calls (Tasks 16, 17, 20, 26), prefix collision retry (Task 19b), pagination defaults + empty result + missing required fields (Task 31).
- *E2E* — full storage lifecycle (Task 29). System-level E2E (CLI → RPC → storage) is out of scope for this plan.

If during execution a category gap surfaces (e.g., a new invariant emerges from impl experience), add a new test to the appropriate sweep task (30 or 31) rather than retrofitting the existing per-method task. Keep test categorization legible.

**Adversarial review applied (2026-05-26):** subagent review surfaced 3 Critical, 9 High, 8 Medium, 5 Low findings. All addressed in this revision. Notable structural fixes:

- All test files carry `//go:build integration`; harness lives in a separate `auth_helpers_test.go`; `sharedTestPool` is concretely spec'd as a pool-per-test helper.
- Task 7 inlines all 17 method stubs so the compile-time assertion is satisfied immediately.
- `WithAuthKeyPrefixGenerator` replaces a package-global generator; tests are now parallel-safe.
- Auth migrations live alongside the project migrations directory (sibling `auth_migrations/`) embedded by the `postgres` package itself, matching the existing `migrate.go` pattern; `goose.SetTableName` package-global concern is documented (sequential startup expected).
- Schema CHECKs constrain prefix length (≥8 chars, the entropy floor the design depends on), email length, role length, etc.
- `TouchLastUsed` excludes revoked keys.
- Task 30 reframed to only address `JITCreateHuman` (CreateHuman's check landed in Task 13).
- All `go test` commands carry `-tags integration`.

---

## Execution

After this plan is approved:

1. **Subagent-Driven (recommended)** — fresh subagent per task, two-stage review between tasks.
2. **Inline Execution** — execute tasks in the same session via the executing-plans skill, batch execution with checkpoints.

The plan is bounded (~28 tasks, mostly TDD cycles); either execution mode fits.
