# Identity Bootstrap & UX — Plan 4b: Bootstrap Flows, Credentials File & Protections

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the first admin exist and be reachable: a shared DB-backed bootstrap helper (idempotent, race-safe) driven by `specgraph init` (local, direct-DB) and `serve.go` first-start (hosted, log-banner); a multi-server credentials file the CLI reads; and the operator-protection guards (force-flag on bootstrap soft-delete/purge, last-credential guard on `UnbindOIDC`).

**Architecture:** Second of two plans split from the "Identity Bootstrap & Operator UX" design (`docs/plans/2026-05-22-identity-bootstrap-ux-design.md`). **Depends on Plan 4a** (IdentityService RPCs, `auth.GenerateAPIKeySecret`/`FormatAPIKeyToken`, the `force`-less delete/purge/unbind handlers it leaves for 4b to harden). The bootstrap user is a **system identity** (`display_name = "admin"`, `bootstrap = true`, no OIDC binding) — never derived from environment. Two bootstrap paths share one `bootstrap.Ensure` helper; they differ only in whether they write the credentials file. Build-alongside discipline: every task ends green at package and whole-project level.

**Tech Stack:** Go 1.26, pgx v5 (via `UsersBackend` / `postgres.New`), `gopkg.in/yaml.v3` (credentials file), Cobra (CLI), ConnectRPC (protections in handlers).

**Implements bead:** Approved design `spgr-g90r` (Identity Bootstrap & UX) under epic `spgr-rjrt`, sub-plan 4b. Depends on Storage (`spgr-e82m`), Authn (`spgr-n2rw`), Cedar (`spgr-rjrt.1`), and **4a** being merged first.

---

## Dependency contract (consumed symbols)

- **4a / Authn** — `auth.GenerateAPIKeySecret() (secret, phcHash string, err error)`, `auth.FormatAPIKeyToken(prefix, secret string) string`, `auth.APIKeyTokenPrefix() string` (package `auth`). The IdentityService proto messages `SoftDeleteUserRequest`/`PurgeUserRequest`/`UnbindOIDCRequest` (4b adds fields). The `IdentityHandler` (4b adds guards to three methods). The `specgraph auth` CLI commands (4b adds `--force`/`--user` flags).
- **Storage** — `storage.UsersBackend` (`GetBootstrap`, `CreateHuman`, `CreateAPIKey`, `GetUserByID`, `SoftDeleteUser`, `PurgeUser`, `UnbindOIDC`, `ListOIDCBindings`, `ListAPIKeys`); domain types `storage.User`/`storage.APIKey`/`storage.OIDCBinding`; `storage.ListAPIKeysFilter`; sentinels `storage.ErrUserNotFound`, `storage.ErrBootstrapExists`. `postgres.New(ctx, url, opts...) (*postgres.Store, error)` and `(*Store).Pool()`; `postgres.NewAuth(ctx, pool) (*AuthStore, error)`. `CreateHuman` returns `storage.ErrBootstrapExists` when a bootstrap already exists (the `users_one_bootstrap` unique index).
- **CLI** — `resolveBaseURL() (baseURL, project string, err error)`, `newClient[C]`, `printJSON`, the shared `authJSON` flag var (4a), `cmd/specgraph/client.go`'s transport plumbing. `internal/xdg.CredentialsFile() string` (`~/.config/specgraph/credentials.yaml`).
- **Config** — `cfg.Server.Postgres.URL` (the connection string; defaulted to a localhost dev value by `loadGlobalCfg`, so rarely empty); `cfg.Server.Listen`.
- **Test harness** — the exported `postgrestest.SharedPool(t, ctx) *pgxpool.Pool` from `internal/storage/postgres/postgrestest`, created by **Storage plan Task 32**. (The in-package `sharedTestPool` is unexported `package postgres_test` and not importable cross-package; Task 32 exists precisely so `internal/bootstrap`, `internal/auth`, and `internal/server` integration tests share one harness.) Task 4 imports it.

**Note on the deleted `auth.ReadDefaultKey`:** the Authn plan deletes `internal/auth/bootstrap.go` (which held `ReadDefaultKey`, `CheckCredentialPermissions`, the old single-key `CredentialsFile`). The current `cmd/specgraph/client.go` `resolveAPIKey` calls `auth.ReadDefaultKey`. **4b takes ownership of all credential read/write** via the new `internal/credentials` package and rewrites `resolveAPIKey` (Task 2) — which also resolves that post-Authn dangling reference.

---

## Testing approach

Four categories; each behavior task tags `**Covers:**`.

- **Happy** — bootstrap creates the admin + key; credentials round-trip; guards allow with force.
- **Invariants** — bootstrap is idempotent and race-safe (one bootstrap row); the bootstrap user is never derived from environment; credentials file never clobbers unrelated server entries; plaintext written 0600.
- **Boundaries** — bootstrap when one already exists (no-op); credentials file missing/old-shape (empty, no error); guards refuse without force; unbind last credential refused.
- **E2E** — `init` → bootstrap → credentials file → CLI authenticates; protections through the real interceptor.

Unit tests use the 4a server-package `usersBackendStub` and a bootstrap-package stub. Integration tests (`//go:build integration`) use the testcontainer `AuthStore`.

---

## File Structure

**Create:**

- `internal/credentials/credentials.go` (+ `_test.go`) — `File` (multi-server), `Load`, `Save`, `TokenFor`, `Upsert`, `CheckPermissions`.
- `internal/bootstrap/bootstrap.go` (+ `_test.go`, + `_integration_test.go`) — `Ensure` (shared DB helper) + `Options`/`Result`.

**Modify:**

- `cmd/specgraph/client.go` — `resolveAPIKey` → use `credentials.TokenFor(serverURL)`; thread the resolved base URL through `newAuthenticatedHTTPClient`/`newClient`.
- `cmd/specgraph/init.go` — local bootstrap path (detect Postgres → `bootstrap.Ensure` + write credentials; else hint).
- `cmd/specgraph/serve.go` — hosted bootstrap path (first-start → `bootstrap.Ensure` → log banner).
- `proto/specgraph/v1/identity.proto` — add `force` to `SoftDeleteUserRequest`/`PurgeUserRequest`; add `user_id`+`force` to `UnbindOIDCRequest`. (`task proto`.)
- `internal/server/identity_handler.go` (+ `_test.go`) — bootstrap guard on `SoftDeleteUser`/`PurgeUser`; last-credential guard on `UnbindOIDC`.
- `cmd/specgraph/auth_user.go` / `auth_oidc.go` — `--force` flags (+ `--user` on unbind).

**Do NOT touch:** the Cedar engine, the resolver, the interceptor, the 4a converters/minting.

---

## Symbol-lifetime sweep

| Identifier | Package | Collision? |
|---|---|---|
| `credentials.File`, `ServerCreds`, `Load`, `Save`, `TokenFor`, `Upsert`, `CheckPermissions` | `credentials` (new) | none — new package. |
| `bootstrap.Ensure`, `Options`, `Result` | `bootstrap` (new) | none — new package. **Distinct from the deleted `auth.Bootstrap`** (different package, different signature). |
| `resolveAPIKey` (signature change), `bootstrapOnInit`, `initSkipBootstrap` | `main` (cmd) | `resolveAPIKey` already exists — Task 2 changes its signature + body and updates all three callers (`newClient`, `newClientWithProject`, `read_mcp_resource.go`). |
| `force`/`user_id` proto fields | generated | additive proto fields (new field numbers); backward-compatible. |
| `requireForceForBootstrap`, `containsBindingID` | `server` | new unexported helpers; no collision. |
| `userForce`, `oidcUnbindUser`, `oidcUnbindForce` (CLI flag vars) | `main` (cmd) | new; no collision. |

No deletions in this plan (the Authn plan already deleted `auth/bootstrap.go`). No survivor analysis needed beyond confirming `resolveAPIKey`'s new body compiles against `credentials`.

---

## Task 1: `internal/credentials` — multi-server credentials file

**Files:**

- Create: `internal/credentials/credentials.go`
- Create: `internal/credentials/credentials_test.go`

**Covers:** Happy (save → load → TokenFor) + Invariant (Upsert preserves other servers; Save is 0600) + Boundary (missing file → empty, no error; old `api_keys:` shape → empty servers, no error).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package credentials_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/credentials"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "credentials.yaml")
	f := credentials.File{}
	f.Upsert("http://localhost:8080", credentials.ServerCreds{Token: "spgr_sk_abc_secret", Label: "local"})
	require.NoError(t, credentials.Save(path, f))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm(), "credentials file must be 0600")

	loaded, err := credentials.Load(path)
	require.NoError(t, err)
	require.Equal(t, "spgr_sk_abc_secret", loaded.TokenFor("http://localhost:8080"))
}

func TestUpsertPreservesOtherServers(t *testing.T) {
	f := credentials.File{}
	f.Upsert("http://a", credentials.ServerCreds{Token: "tok-a"})
	f.Upsert("http://b", credentials.ServerCreds{Token: "tok-b"})
	f.Upsert("http://a", credentials.ServerCreds{Token: "tok-a2"}) // refresh a
	require.Equal(t, "tok-a2", f.TokenFor("http://a"))
	require.Equal(t, "tok-b", f.TokenFor("http://b"), "unrelated entry untouched")
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	f, err := credentials.Load(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	require.Empty(t, f.TokenFor("http://anything"))
}

func TestLoadOldShapeYieldsNoServers(t *testing.T) {
	// The pre-identity credentials.yaml used `api_keys:`. Lenient YAML parse
	// ignores it; TokenFor returns empty (operator re-bootstraps / creates a
	// key). This is the documented, release-noted migration.
	path := filepath.Join(t.TempDir(), "credentials.yaml")
	require.NoError(t, os.WriteFile(path, []byte("api_keys:\n  - key: spgr_sk_old\n    role: admin\n"), 0o600))
	f, err := credentials.Load(path)
	require.NoError(t, err)
	require.Empty(t, f.TokenFor("http://localhost:8080"))
}

func TestTokenForNormalizesTrailingSlash(t *testing.T) {
	f := credentials.File{}
	f.Upsert("http://localhost:8080", credentials.ServerCreds{Token: "tok"})
	require.Equal(t, "tok", f.TokenFor("http://localhost:8080/"), "trailing slash ignored")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/credentials && go test ./...`

Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package credentials manages the CLI's local credentials file — a
// per-operator artifact mapping server URLs to bearer tokens. It is read by
// the CLI for outgoing auth and written by the local bootstrap path. It is
// NOT read by the server (the server resolves credentials from the database).
package credentials

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// File is the on-disk credentials structure. It holds entries for multiple
// server URLs so a single CLI install can authenticate against several
// deployments without rewriting the file.
type File struct {
	Servers map[string]ServerCreds `yaml:"servers"`
}

// ServerCreds is one server's credential.
type ServerCreds struct {
	Token string `yaml:"token"`
	Label string `yaml:"label,omitempty"`
}

// normalize trims a trailing slash so "http://h:8080" and "http://h:8080/"
// resolve to the same entry.
func normalize(serverURL string) string {
	return strings.TrimRight(serverURL, "/")
}

// TokenFor returns the token for the given server URL, or "" if none.
func (f File) TokenFor(serverURL string) string {
	if f.Servers == nil {
		return ""
	}
	return f.Servers[normalize(serverURL)].Token
}

// Upsert sets (or refreshes) the entry for serverURL without disturbing
// other entries.
func (f *File) Upsert(serverURL string, creds ServerCreds) {
	if f.Servers == nil {
		f.Servers = make(map[string]ServerCreds)
	}
	f.Servers[normalize(serverURL)] = creds
}

// Load reads the credentials file. A missing file returns an empty File with
// no error. Unknown legacy fields (the old `api_keys:` shape) are ignored by
// the lenient YAML parse, yielding no servers.
func Load(path string) (File, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-owned path under XDG config
	if errors.Is(err, fs.ErrNotExist) {
		return File{}, nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read credentials %s: %w", path, err)
	}
	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("parse credentials %s: %w", path, err)
	}
	return f, nil
}

// Save atomically writes the credentials file with 0600 permissions
// (temp-file + rename so a partial write never replaces a good file).
func Save(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	header := "# SpecGraph CLI credentials. Per-operator; do not commit.\n"
	content := append([]byte(header), data...)

	tmp, err := os.CreateTemp(filepath.Dir(path), ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op if rename succeeded
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp credentials: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp credentials: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename credentials into place: %w", err)
	}
	return nil
}

// CheckPermissions returns a warning string if the file is more permissive
// than 0600, or "" if it's fine or absent.
func CheckPermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	// Warn only when group/other bits are set (a real leak). Using perm&0o077
	// — NOT perm&^0o600 — so stricter owner-only modes like 0o400/0o700 are
	// accepted rather than wrongly flagged.
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Sprintf("credentials file %s is group/other-accessible (%04o); tighten to 0600", path, perm)
	}
	return ""
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/credentials && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/credentials/credentials.go internal/credentials/credentials_test.go
git commit -s -m "feat(credentials): multi-server credentials file (load/save/upsert)"
```

---

## Task 2: Rewrite CLI `resolveAPIKey` to use the credentials package

**Files:**

- Modify: `cmd/specgraph/client.go`
- Modify: `cmd/specgraph/read_mcp_resource.go` (a second caller of `newAuthenticatedHTTPClient`)

**Covers:** Happy (token resolved for the active server URL) + Boundary (env var precedence; missing file → "").

> Context: the Authn plan deletes `auth.ReadDefaultKey`, which the current `resolveAPIKey` calls — so post-Authn this file does not compile until this task lands. 4b owns the fix.

- [ ] **Step 1: Identify the current code and ALL callers**

The current `resolveAPIKey()` (no args) returns `os.Getenv("SPECGRAPH_API_KEY")` or `auth.ReadDefaultKey(xdg.CredentialsFile())`. The signature change to `resolveAPIKey(serverURL string)` ripples through `newAuthenticatedHTTPClient`, which has **three** callers — grep to confirm before editing:

```bash
grep -rn 'newAuthenticatedHTTPClient\|resolveAPIKey' cmd/specgraph/
```

Expected callers: `client.go`'s `newClient` AND `newClientWithProject` (both build a client after resolving a base URL), plus `read_mcp_resource.go`. Each must pass the resolved `serverURL`. All three have a base URL in scope at the call site (or can call `resolveBaseURL()`).

- [ ] **Step 2: Rewrite credential resolution to be server-scoped**

Replace `resolveAPIKey()` with a server-URL-aware version and thread the URL through:

```go
// resolveAPIKey returns the bearer token for the given server URL.
// Precedence: SPECGRAPH_API_KEY env var > credentials file entry for
// serverURL > "" (no auth).
func resolveAPIKey(serverURL string) string {
	if key := os.Getenv("SPECGRAPH_API_KEY"); key != "" {
		return key
	}
	f, err := credentials.Load(xdg.CredentialsFile())
	if err != nil {
		return ""
	}
	return f.TokenFor(serverURL)
}
```

Update `newAuthenticatedHTTPClient` to take the server URL and pass it through:

```go
func newAuthenticatedHTTPClient(project, serverURL string) *http.Client {
	return &http.Client{Transport: &staticTokenTransport{
		inner: &clientTransport{
			base:    http.DefaultTransport,
			project: project,
		},
		token: resolveAPIKey(serverURL),
	}}
}
```

Update `newClient` to pass `baseURL` into `newAuthenticatedHTTPClient`:

```go
func newClient[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C) (C, error) {
	baseURL, project, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	return ctor(newAuthenticatedHTTPClient(project, baseURL), baseURL), nil
}
```

Update the **other two** call sites:

- `client.go`'s `newClientWithProject` (used by `constitution.go`): it resolves a base URL too — pass it as the second arg, e.g. `newAuthenticatedHTTPClient(derivedProject, baseURL)`. If it doesn't currently capture the base URL, add `baseURL, _, _ := resolveBaseURL()` (or thread the value it already computes).
- `cmd/specgraph/read_mcp_resource.go`: it calls `newAuthenticatedHTTPClient(project)` — change to `newAuthenticatedHTTPClient(project, serverURL)` where `serverURL` is the base URL it resolves (it already builds a client against a URL; pass that URL).

Add `internal/credentials` to client.go's imports. **Keep the `internal/auth` import** — `clientTransport.RoundTrip` still uses `auth.WithBearerToken`/`auth.ProjectFromContext` (unchanged; only `auth.ReadDefaultKey` is dropped). `newHTTPClient` (unauthenticated MCP calls) is separate — leave it.

- [ ] **Step 3: Verify build + tests**

Run: `go build ./... && go test ./...`

Expected: PASS (and the post-Authn dangling `auth.ReadDefaultKey` reference is gone).

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/client.go cmd/specgraph/read_mcp_resource.go
git commit -s -m "feat(cli): resolve API key per server URL from the credentials package"
```

---

## Task 3: `internal/bootstrap.Ensure` — the shared DB helper

**Files:**

- Create: `internal/bootstrap/bootstrap.go`
- Create: `internal/bootstrap/bootstrap_test.go`

**Covers:** Happy (creates admin + key, returns token) + Invariant (idempotent: existing bootstrap → no-op; system identity has display_name="admin", no OIDC binding) + Boundary (race: CreateHuman returns ErrBootstrapExists → re-fetch, no-op).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package bootstrap_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/bootstrap"
	"github.com/specgraph/specgraph/internal/storage"
)

// stub implements the narrow bootstrap.Backend (GetBootstrap, CreateHuman,
// CreateAPIKey) that bootstrap.Ensure depends on.
type stub struct {
	bootstrapUser *storage.User
	createHuman   func(*storage.User) (*storage.User, error)
	createdKey    *storage.APIKey
}

func (s *stub) GetBootstrap(context.Context) (*storage.User, error) {
	if s.bootstrapUser == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.bootstrapUser, nil
}
func (s *stub) CreateHuman(_ context.Context, u *storage.User, _ *storage.OIDCBinding) (*storage.User, error) {
	return s.createHuman(u)
}
func (s *stub) CreateAPIKey(_ context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	k.ID = "key-1"
	k.Prefix = "boot1234" // simulate storage-assigned prefix
	s.createdKey = k
	return k, nil
}

// stub satisfies bootstrap.Backend with exactly these three methods — no
// other UsersBackend methods are needed because Ensure depends on the narrow
// Backend interface (Step 3).

func TestEnsure_CreatesBootstrapAdmin(t *testing.T) {
	var created *storage.User
	s := &stub{createHuman: func(u *storage.User) (*storage.User, error) {
		require.Equal(t, "admin", u.DisplayName, "system identity, not env-derived")
		require.True(t, u.Bootstrap)
		require.Equal(t, storage.KindHuman, u.Kind)
		u.ID = "boot-user"
		created = u
		return u, nil
	}}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created)
	require.NotEmpty(t, res.Token, "a new bootstrap returns the plaintext token")
	require.True(t, strings.HasPrefix(res.Token, auth.APIKeyTokenPrefix()))
	require.Contains(t, res.Token, "boot1234", "token embeds the storage-assigned prefix")
	require.Equal(t, "admin", created.Role)
}

func TestEnsure_IdempotentWhenExists(t *testing.T) {
	s := &stub{bootstrapUser: &storage.User{ID: "boot-user", DisplayName: "admin", Bootstrap: true, Role: "admin"}}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "existing bootstrap is a no-op")
	require.Empty(t, res.Token, "no token minted when bootstrap already exists")
}

// raceStub models losing the create race: GetBootstrap misses first (so
// Ensure proceeds to CreateHuman), CreateHuman returns ErrBootstrapExists
// (the winner created it concurrently), and the re-fetch GetBootstrap returns
// the winner.
type raceStub struct {
	winner   *storage.User
	getCalls int
}

func (r *raceStub) GetBootstrap(context.Context) (*storage.User, error) {
	r.getCalls++
	if r.getCalls == 1 {
		return nil, storage.ErrUserNotFound
	}
	return r.winner, nil
}
func (r *raceStub) CreateHuman(context.Context, *storage.User, *storage.OIDCBinding) (*storage.User, error) {
	return nil, storage.ErrBootstrapExists
}
func (r *raceStub) CreateAPIKey(context.Context, *storage.APIKey) (*storage.APIKey, error) {
	panic("CreateAPIKey must not be called when the create race is lost")
}

func TestEnsure_RaceLosesGracefully(t *testing.T) {
	r := &raceStub{winner: &storage.User{ID: "winner", DisplayName: "admin", Bootstrap: true}}
	res, err := bootstrap.Ensure(context.Background(), r, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "race loser observes the winner's bootstrap")
	require.Equal(t, "winner", res.UserID)
	require.Empty(t, res.Token, "loser mints no key")
}
```

> Add a `strings` import (used by `TestEnsure_CreatesBootstrapAdmin`). Because `Ensure` takes the narrow `bootstrap.Backend` interface (3 methods — see Step 3), both `*stub` and `*raceStub` satisfy it with only those methods; no `unusedBackend` boilerplate is needed. The `errors` import is only needed if a test uses `errors.Is` — drop it otherwise.

- [ ] **Step 2: Verify failure**

Run: `cd internal/bootstrap && go test ./...`

Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Write the impl**

To keep the stub small and the dependency honest, `Ensure` depends on a **narrow interface** (only the methods it needs), which `storage.UsersBackend` satisfies structurally:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package bootstrap creates the first admin identity. It is the shared DB
// helper behind both bootstrap paths: `specgraph init` (local, direct DB) and
// the server's first start (hosted). The bootstrap user is a SYSTEM identity
// (display_name "admin", bootstrap=true, no OIDC binding) — never derived
// from the OS user, hostname, or any environmental signal.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the narrow slice of storage.UsersBackend that Ensure needs.
// storage.UsersBackend (and the postgres *AuthStore) satisfy it structurally.
type Backend interface {
	GetBootstrap(ctx context.Context) (*storage.User, error)
	CreateHuman(ctx context.Context, u *storage.User, binding *storage.OIDCBinding) (*storage.User, error)
	CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error)
}

// Options parametrizes Ensure.
type Options struct {
	// Role for the bootstrap admin. Defaults to "admin".
	Role string
}

// Result reports what Ensure did.
type Result struct {
	Created bool   // true if this call created the bootstrap user + key
	Token   string // plaintext token (only set when Created; show once)
	UserID  string // bootstrap user id (set whether created or pre-existing)
}

// Ensure creates the bootstrap admin + an admin API key if no bootstrap user
// exists, and is a no-op otherwise. Idempotent and race-safe: concurrent
// callers converge to one bootstrap row (CreateHuman returns
// ErrBootstrapExists for the loser, which re-reads the winner).
func Ensure(ctx context.Context, b Backend, opts Options) (Result, error) {
	role := opts.Role
	if role == "" {
		role = "admin"
	}

	// Idempotency check.
	if existing, err := b.GetBootstrap(ctx); err == nil {
		return Result{Created: false, UserID: existing.ID}, nil
	} else if !errors.Is(err, storage.ErrUserNotFound) {
		return Result{}, fmt.Errorf("check bootstrap: %w", err)
	}

	// Create the system admin (no OIDC binding — backstop identity).
	user, err := b.CreateHuman(ctx, &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "admin", // literal; NOT env-derived
		Role:        role,
		Bootstrap:   true,
	}, nil)
	if err != nil {
		// Lost a race: another caller created the bootstrap first.
		if errors.Is(err, storage.ErrBootstrapExists) {
			existing, getErr := b.GetBootstrap(ctx)
			if getErr != nil {
				return Result{}, fmt.Errorf("re-read bootstrap after race: %w", getErr)
			}
			return Result{Created: false, UserID: existing.ID}, nil
		}
		return Result{}, fmt.Errorf("create bootstrap user: %w", err)
	}

	// Mint an admin key. Storage owns the prefix (see 4a Task 7); build the
	// token from the storage-assigned prefix.
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return Result{}, fmt.Errorf("generate bootstrap key: %w", err)
	}
	key, err := b.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  user.ID,
		PHCHash: phc,
		Label:   "bootstrap admin key",
	})
	if err != nil {
		return Result{}, fmt.Errorf("create bootstrap key: %w", err)
	}
	return Result{
		Created: true,
		Token:   auth.FormatAPIKeyToken(key.Prefix, secret),
		UserID:  user.ID,
	}, nil
}
```

> Because `Ensure` takes the narrow `Backend`, the test stub only needs three methods (`GetBootstrap`, `CreateHuman`, `CreateAPIKey`) — no `unusedBackend` boilerplate. Simplify the Task-1 test stub accordingly (drop the panicking-base note; the three-method `stub`/`raceStub` already satisfy `Backend`).

- [ ] **Step 4: Run the tests**

Run: `cd internal/bootstrap && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/bootstrap/bootstrap.go internal/bootstrap/bootstrap_test.go
git commit -s -m "feat(bootstrap): idempotent, race-safe Ensure (system admin + key)"
```

---

## Task 4: Bootstrap integration test (idempotent + resolvable)

**Files:**

- Create: `internal/bootstrap/bootstrap_integration_test.go`

**Covers:** E2E (Ensure against real AuthStore; token resolves; second call is a no-op).

- [ ] **Step 1: Write the test**

```go
//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package bootstrap_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/bootstrap"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

func TestIntegration_EnsureIdempotentAndResolvable(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx) // EXPORTED cross-package harness — see note below
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })
	_, err = pool.Exec(ctx, `TRUNCATE users RESTART IDENTITY CASCADE`)
	require.NoError(t, err)

	res, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created)
	require.NotEmpty(t, res.Token)

	// The minted token resolves to the bootstrap admin.
	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)
	id, err := resolver.Resolve(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, res.UserID, id.UserID)
	require.Equal(t, auth.Role("admin"), id.Role)

	// Second call is a no-op.
	res2, err := bootstrap.Ensure(ctx, authStore, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res2.Created)
	require.Empty(t, res2.Token)
	require.Equal(t, res.UserID, res2.UserID)
}
```

> **Test pool helper:** `postgrestest.SharedPool(t, ctx)` is the exported cross-package harness created by **Storage plan Task 32** (the in-package `sharedTestPool` is `package postgres_test` and not importable here). The Authn and 4a integration tests use the same helper. This task depends on Task 32 having landed.

- [ ] **Step 2: Run**

Run: `cd internal/bootstrap && go test -tags integration -run TestIntegration_EnsureIdempotentAndResolvable -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/bootstrap/bootstrap_integration_test.go
git commit -s -m "test(bootstrap): integration test for Ensure (idempotent + resolvable)"
```

---

## Task 5: `specgraph init` local bootstrap path

**Files:**

- Modify: `cmd/specgraph/init.go`

**Covers:** Happy (DB reachable → bootstrap + write credentials) + Boundary (no DB URL / unreachable → hint, no error).

- [ ] **Step 1: Add the local-bootstrap step**

After `init` finishes its managed-file sync, add a bootstrap step. Add a helper `bootstrapOnInit` and call it near the end of `runInit` (before the final success message):

```go
// bootstrapOnInit runs the local-mode bootstrap when a Postgres connection is
// available: creates the first admin (idempotent) and writes the resulting
// token into the local credentials file for the resolved server URL. When no
// DB is configured or reachable, it prints a hint that the server's first
// start will handle bootstrap, and returns nil (init never fails on this).
func bootstrapOnInit(ctx context.Context, w io.Writer) error {
	cfg, err := loadGlobalCfg()
	if err != nil {
		return nil // config problems are surfaced elsewhere; don't block init
	}
	url := cfg.Server.Postgres.URL
	if url == "" {
		// Defensive: loadGlobalCfg() normally defaults this to a localhost dev
		// URL, so this branch rarely fires — the unreachable-DB branch below is
		// the real "no local Postgres" path.
		fmt.Fprintln(w, "  No Postgres URL configured; the server's first start will create the admin.")
		return nil
	}

	// Short dial timeout: a developer without local Postgres should not wait
	// long for `specgraph init`. 2s is enough for a healthy local DB; a miss
	// falls through to the hint quickly.
	connectCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	store, err := postgres.New(connectCtx, url, postgres.WithProject("_server"))
	if err != nil {
		fmt.Fprintf(w, "  Postgres not reachable (%v); the server's first start will create the admin.\n", err)
		return nil
	}
	// Close with a non-timed context: connectCtx's 2s deadline is for the dial
	// only and may already be expired by the time these deferred closes run.
	defer func() { _ = store.Close(context.Background()) }()

	authStore, err := postgres.NewAuth(connectCtx, store.Pool())
	if err != nil {
		fmt.Fprintf(w, "  Could not open identity store (%v); skipping local bootstrap.\n", err)
		return nil
	}
	defer func() { _ = authStore.Close(context.Background()) }()

	res, err := bootstrap.Ensure(connectCtx, authStore, bootstrap.Options{})
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	if !res.Created {
		fmt.Fprintln(w, "  Admin already exists; leaving credentials untouched.")
		return nil
	}

	// Write the token into the credentials file for the resolved server URL.
	serverURL, _, err := resolveBaseURL()
	if err != nil || serverURL == "" {
		// Fall back to the local listen address.
		serverURL = "http://" + cfg.Server.Listen
	}
	credPath := xdg.CredentialsFile()
	f, _ := credentials.Load(credPath)
	f.Upsert(serverURL, credentials.ServerCreds{Token: res.Token, Label: "bootstrap"})
	if err := credentials.Save(credPath, f); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	fmt.Fprintf(w, "\n  Created admin and saved its API key to %s (server %s).\n", credPath, serverURL)
	fmt.Fprintln(w, "  Rotate it after configuring OIDC: specgraph auth api-key rotate <id> --user <admin-id>")
	return nil
}
```

Wire it into `runInit`, guarded by a new `--skip-bootstrap` flag for operators who want managed-file-only init. **The current `runInit` signature is `func runInit(_ *cobra.Command, args []string) error` — rename the `_` to `cmd`** so `cmd.Context()`/`cmd.OutOrStdout()` resolve:

```go
	if !initSkipBootstrap {
		if err := bootstrapOnInit(cmd.Context(), cmd.OutOrStdout()); err != nil {
			return err
		}
	}
```

Add imports: `context`, `io`, `time`, `internal/bootstrap`, `internal/credentials`, `internal/storage/postgres`, `internal/xdg`. Register the flag in `init()`: `initCmd.Flags().BoolVar(&initSkipBootstrap, "skip-bootstrap", false, "skip local admin bootstrap (managed files only)")`, and declare `var initSkipBootstrap bool`.

> Match the merged `init.go`'s actual run-function name (it is `runInit` in the current code) and confirm `loadGlobalCfg()` exists (it's used by `resolveBaseURL`). `(*postgres.Store).Close(ctx)` and `(*postgres.AuthStore).Close(ctx)` both take a context (verified against the current/merged signatures).

- [ ] **Step 2: Verify build**

Run: `go build ./... && go run ./cmd/specgraph init --help`

Expected: build PASS; `--skip-bootstrap` listed.

- [ ] **Step 3: Verify local-bootstrap degrades gracefully without a DB**

Because `loadGlobalCfg()` defaults `Server.Postgres.URL` to a localhost dev value, `init` on a machine with no local Postgres takes the unreachable-DB branch: it prints "Postgres not reachable … the server's first start will create the admin" after a ≤2s dial and exits 0.

Run (in a temp dir, no Postgres running): `time go run ./cmd/specgraph init demo-project 2>&1 | grep -i 'not reachable\|create the admin'` — expect the hint, exit 0, and the run to finish within a couple seconds. (Operators who want to skip the dial entirely use `--skip-bootstrap`.)

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/init.go
git commit -s -m "feat(init): local-mode admin bootstrap when Postgres is reachable"
```

---

## Task 6: `serve.go` hosted bootstrap path (log banner)

**Files:**

- Modify: `cmd/specgraph/serve.go`

**Covers:** Happy (first start with no bootstrap → create + banner) + Invariant (no filesystem write in hosted path; idempotent on restart).

- [ ] **Step 1: Add the hosted-bootstrap step**

In the post-Authn `serve.go`, after `authStore` is constructed (the `postgres.NewAuth(...)` value) and before/around the existing `warnIfNoAuthOnPublicListen` call, add:

```go
	// Hosted bootstrap: create the first admin on first start. Unlike the
	// local path (specgraph init), the server does NOT write a credentials
	// file the operator can read — it surfaces the key in the log banner.
	if res, bErr := bootstrap.Ensure(ctx, authStore, bootstrap.Options{}); bErr != nil {
		return fmt.Errorf("bootstrap admin: %w", bErr)
	} else if res.Created {
		serverURL := "http://" + cfg.Server.Listen
		fmt.Fprintf(os.Stderr,
			"\n  SpecGraph created a bootstrap admin API key:\n\n    %s\n\n"+
				"  Server: %s\n"+
				"  Copy it into your local credentials (the CLI reads ~/.config/specgraph/credentials.yaml),\n"+
				"  then rotate it after configuring OIDC. It will not be shown again.\n\n",
			res.Token, serverURL)
	}
```

- [ ] **Step 2: Verify whole-project build**

Run: `go build ./...`

Expected: PASS. Add `internal/bootstrap` to serve.go's imports.

> Match the merged serve.go: `authStore` (the `*postgres.AuthStore`), `ctx`, `cfg.Server.Listen`, and the `fmt`/`os` imports (both already present). The hosted path runs unconditionally each start; `Ensure` is idempotent, so restarts after the first are silent no-ops.

- [ ] **Step 3: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): hosted admin bootstrap with log-banner on first start"
```

---

## Task 7: Proto — `force` flags + `user_id` on unbind

**Files:**

- Modify: `proto/specgraph/v1/identity.proto`

**Covers:** N/A (schema + codegen).

- [ ] **Step 1: Add the fields**

In `identity.proto`, update the three messages (4a left field numbers free):

```proto
message SoftDeleteUserRequest {
  string id = 1;
  bool force = 2; // required to soft-delete the bootstrap admin
}

message PurgeUserRequest {
  string id = 1;
  bool force = 2; // required to purge the bootstrap admin
}

message UnbindOIDCRequest {
  string binding_id = 1;
  string user_id = 2; // owner of the binding (required for the last-credential check)
  bool force = 3;     // required to remove a user's only credential
}
```

- [ ] **Step 2: Generate + build**

Run: `task proto && go build ./...`

Expected: generated getters `GetForce()`, `GetUserId()` appear; build PASS (4a handlers ignore the new fields until Tasks 8–9).

- [ ] **Step 3: Commit**

```bash
git add proto/specgraph/v1/identity.proto gen/specgraph/v1/identity.pb.go gen/specgraph/v1/specgraphv1connect/identity.connect.go
git commit -s -m "feat(proto): add force flags + unbind user_id for identity protections"
```

---

## Task 8: Bootstrap-protection guard on `SoftDeleteUser` + `PurgeUser`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (force allows; non-bootstrap unaffected) + Invariant (bootstrap user refused without force) + Boundary (NotFound from GetUserByID propagates).

- [ ] **Step 1: Write the failing test**

```go
func TestSoftDeleteUser_RefusesBootstrapWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Kind: storage.KindHuman, Bootstrap: true, Role: "admin"}, nil
		},
		// softDeleteUser intentionally unset — must NOT be called.
	}
	h := newTestIdentityHandler(stub)
	_, err := h.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "boot"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestSoftDeleteUser_AllowsBootstrapWithForce(t *testing.T) {
	deleted := false
	stub := &usersBackendStub{
		getUserByID:    func(_ context.Context, id string) (*storage.User, error) { return &storage.User{ID: id, Bootstrap: true}, nil },
		softDeleteUser: func(context.Context, string) error { deleted = true; return nil },
	}
	h := newTestIdentityHandler(stub)
	_, err := h.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "boot", Force: true}))
	require.NoError(t, err)
	require.True(t, deleted)
}

func TestSoftDeleteUser_NonBootstrapNoForceNeeded(t *testing.T) {
	deleted := false
	stub := &usersBackendStub{
		getUserByID:    func(_ context.Context, id string) (*storage.User, error) { return &storage.User{ID: id, Bootstrap: false}, nil },
		softDeleteUser: func(context.Context, string) error { deleted = true; return nil },
	}
	h := newTestIdentityHandler(stub)
	_, err := h.SoftDeleteUser(context.Background(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: "u1"}))
	require.NoError(t, err)
	require.True(t, deleted)
}

func TestPurgeUser_RefusesBootstrapWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return &storage.User{ID: id, Bootstrap: true}, nil
		},
	}
	h := newTestIdentityHandler(stub)
	_, err := h.PurgeUser(context.Background(), connect.NewRequest(&specv1.PurgeUserRequest{Id: "boot"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestSoftDeleteUser_RefusesBootstrap|TestPurgeUser_RefusesBootstrap' -v`

Expected: FAIL — 4a's handlers don't read `Force` and don't fetch the user; they soft-delete/purge unconditionally.

- [ ] **Step 3: Add the guard**

Add a shared helper and call it from both methods. In `identity_handler.go`:

```go
// requireForceForBootstrap fetches the target user and refuses a destructive
// operation on the bootstrap admin unless force is set. Returns a connect
// error ready to return, or nil to proceed.
func (h *IdentityHandler) requireForceForBootstrap(ctx context.Context, id string, force bool) error {
	u, err := h.users.GetUserByID(ctx, id)
	if err != nil {
		return h.identityError(ctx, err)
	}
	if u.Bootstrap && !force {
		return connect.NewError(connect.CodeFailedPrecondition,
			errors.New("refusing to delete the bootstrap admin without force; pass --force to confirm"))
	}
	return nil
}
```

Update `SoftDeleteUser` (insert the guard after the id check, before the backend call):

```go
	if err := h.requireForceForBootstrap(ctx, req.Msg.GetId(), req.Msg.GetForce()); err != nil {
		return nil, err
	}
```

Update `PurgeUser` identically (same guard line after its id check).

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestSoftDeleteUser|TestPurgeUser' -v`

Expected: PASS (new guard tests + the 4a tests, after the one stub fix below).

> **Cross-task note (ripple into 4a tests):** The guard adds a `GetUserByID` call before the backend mutation. Auditing 4a's three delete/purge tests:
> - `TestSoftDeleteUser_Happy` — its stub sets only `softDeleteUser`; the default `getUserByID` returns `ErrUserNotFound`, so the guard now returns `CodeNotFound` and the test fails. **Fix:** add `getUserByID: func(_ context.Context, id string) (*storage.User, error) { return &storage.User{ID: id, Bootstrap: false}, nil }` to its stub.
> - `TestSoftDeleteUser_RequiresID` — unaffected (the empty-id check runs before the guard).
> - `TestPurgeUser_NotFound` — still passes unchanged: the guard's `GetUserByID` hits the default `ErrUserNotFound` and returns `CodeNotFound`, which is exactly what the test asserts (the `purgeUser` stub is simply never reached).
> That single `TestSoftDeleteUser_Happy` stub addition is the only required edit.

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): bootstrap-protection guard on SoftDeleteUser and PurgeUser"
```

---

## Task 9: Last-credential guard on `UnbindOIDC`

**Files:**

- Modify: `internal/server/identity_handler.go`
- Modify: `internal/server/identity_handler_test.go`

**Covers:** Happy (force allows; user with other credentials unaffected) + Invariant (removing the only credential refused without force) + Boundary (missing user_id → InvalidArgument).

> Design: `UnbindOIDC` carries the last-credential protection; `RevokeAPIKey` deliberately does NOT (revoking the last key is a normal rotate-by-hand step, and OIDC remains). The handler counts the user's OIDC bindings + active API keys; if this unbind would leave zero credentials and `force` is unset, it refuses. The request carries `user_id` because there is no get-binding-by-id backend method.

- [ ] **Step 1: Write the failing test**

```go
func TestUnbindOIDC_RefusesLastCredentialWithoutForce(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(context.Context, string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}, nil // exactly one binding
		},
		listAPIKeys: func(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			return nil, nil // no active keys
		},
		// unbindOIDC unset — must NOT be called.
	}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1"}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestUnbindOIDC_AllowsLastWithForce(t *testing.T) {
	unbound := false
	stub := &usersBackendStub{
		listOIDCBindings: func(context.Context, string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}, nil
		},
		listAPIKeys: func(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error) { return nil, nil },
		unbindOIDC:  func(context.Context, string) error { unbound = true; return nil },
	}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1", Force: true}))
	require.NoError(t, err)
	require.True(t, unbound)
}

func TestUnbindOIDC_AllowsWhenOtherCredentialsExist(t *testing.T) {
	unbound := false
	stub := &usersBackendStub{
		listOIDCBindings: func(context.Context, string) ([]*storage.OIDCBinding, error) {
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}, {ID: "b2", UserID: "u1"}}, nil // two bindings
		},
		listAPIKeys: func(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error) { return nil, nil },
		unbindOIDC:  func(context.Context, string) error { unbound = true; return nil },
	}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1", UserId: "u1"}))
	require.NoError(t, err)
	require.True(t, unbound)
}

func TestUnbindOIDC_RequiresUserID(t *testing.T) {
	h := newTestIdentityHandler(&usersBackendStub{})
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "b1"}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// Ownership check: a credential-rich user_id must NOT let a caller remove a
// binding that doesn't belong to that user (closes the trust-the-caller gap).
func TestUnbindOIDC_RejectsBindingNotOwnedByUser(t *testing.T) {
	stub := &usersBackendStub{
		listOIDCBindings: func(context.Context, string) ([]*storage.OIDCBinding, error) {
			// u1 has two bindings, but NOT the one being removed ("other").
			return []*storage.OIDCBinding{{ID: "b1", UserID: "u1"}, {ID: "b2", UserID: "u1"}}, nil
		},
		// unbindOIDC unset — must NOT be called.
	}
	h := newTestIdentityHandler(stub)
	_, err := h.UnbindOIDC(context.Background(), connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: "other", UserId: "u1", Force: true}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "binding not owned by user → NotFound, even with force")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/server && go test -run 'TestUnbindOIDC' -v`

Expected: FAIL — 4a's `UnbindOIDC` unbinds unconditionally and doesn't require `user_id`.

- [ ] **Step 3: Replace the `UnbindOIDC` handler**

```go
// UnbindOIDC removes an OIDC binding, refusing to remove a user's ONLY
// credential unless force is set. It first verifies the binding belongs to
// user_id (there is no get-binding-by-id backend method, so the caller names
// the owner and we confirm it against the user's binding list — this prevents
// a credential-rich user_id from masking the removal of a different user's
// last binding). Then it counts the user's active credentials (bindings +
// non-revoked keys) and gates on force.
func (h *IdentityHandler) UnbindOIDC(ctx context.Context, req *connect.Request[specv1.UnbindOIDCRequest]) (*connect.Response[specv1.UnbindOIDCResponse], error) {
	msg := req.Msg
	if msg.GetBindingId() == "" || msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("binding_id and user_id are required"))
	}
	// Fetch the user's bindings once — used to verify ownership AND to count.
	bindings, err := h.users.ListOIDCBindings(ctx, msg.GetUserId())
	if err != nil {
		return nil, h.identityError(ctx, err)
	}
	if !containsBindingID(bindings, msg.GetBindingId()) {
		return nil, connect.NewError(connect.CodeNotFound,
			errors.New("binding not found for the given user"))
	}
	if !msg.GetForce() {
		keys, keyErr := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: msg.GetUserId(), IncludeRevoked: false})
		if keyErr != nil {
			return nil, h.identityError(ctx, keyErr)
		}
		if len(bindings)+len(keys) <= 1 {
			return nil, connect.NewError(connect.CodeFailedPrecondition,
				errors.New("refusing to remove the user's only credential without force; pass --force to confirm"))
		}
	}
	if err := h.users.UnbindOIDC(ctx, msg.GetBindingId()); err != nil {
		return nil, h.identityError(ctx, err)
	}
	return connect.NewResponse(&specv1.UnbindOIDCResponse{}), nil
}

// containsBindingID reports whether bindingID is among the user's bindings.
func containsBindingID(bindings []*storage.OIDCBinding, bindingID string) bool {
	for _, b := range bindings {
		if b.ID == bindingID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/server && go build ./... && go test -run 'TestUnbindOIDC|TestListOIDCBindings' -v`

Expected: PASS.

> **Cross-task note (ripple into 4a tests):** 4a's `TestUnbindOIDC_Happy` sends only `BindingId: "b1"`. The guard now (a) requires `UserId`, and (b) verifies the binding is in `ListOIDCBindings(user_id)`. Update that 4a test's request to `{BindingId: "b1", UserId: "u1", Force: true}` AND give its stub a `listOIDCBindings` returning `[]*storage.OIDCBinding{{ID: "b1", UserID: "u1"}}` so the ownership check passes (force then bypasses the count). 4a's `TestUnbindOIDC_NotFound` (binding "x", default stub `listOIDCBindings` returns nil) still yields `CodeNotFound` — now from the ownership check rather than the backend — so its assertion holds; add `UserId: "u1"` to its request so it passes the arg check. The `unbindOIDC` stub func in that test is simply never reached (fine).

- [ ] **Step 5: Commit**

```bash
git add internal/server/identity_handler.go internal/server/identity_handler_test.go
git commit -s -m "feat(server): last-credential guard on UnbindOIDC"
```

---

## Task 10: CLI `--force` / `--user` flags for protected operations

**Files:**

- Modify: `cmd/specgraph/auth_user.go`
- Modify: `cmd/specgraph/auth_oidc.go`

**Covers:** Command wiring (build + help smoke); guard behavior covered by Tasks 8–9 + the integration test (Task 11).

- [ ] **Step 1: Add `--force` to user delete/purge**

In `auth_user.go`, add a package-level `var userForce bool`, thread it into the delete/purge requests, and register the flag.

Update the delete command's request:

```go
		if _, err := client.SoftDeleteUser(cmd.Context(), connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: args[0], Force: userForce})); err != nil {
```

Update the purge command's request:

```go
		if _, err := client.PurgeUser(cmd.Context(), connect.NewRequest(&specv1.PurgeUserRequest{Id: args[0], Force: userForce})); err != nil {
```

In `auth_user.go`'s `init()`, add:

```go
	authUserDeleteCmd.Flags().BoolVar(&userForce, "force", false, "allow deleting the bootstrap admin")
	authUserPurgeCmd.Flags().BoolVar(&userForce, "force", false, "allow purging the bootstrap admin")
```

- [ ] **Step 2: Add `--user` + `--force` to oidc unbind**

In `auth_oidc.go`, add `var (oidcUnbindUser string; oidcUnbindForce bool)`, update the unbind command:

```go
var authOIDCUnbindCmd = &cobra.Command{
	Use:   "unbind <binding-id>",
	Short: "Remove an OIDC binding",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if oidcUnbindUser == "" {
			return fmt.Errorf("--user is required (owner of the binding)")
		}
		client, err := identityClient()
		if err != nil {
			return err
		}
		if _, err := client.UnbindOIDC(cmd.Context(), connect.NewRequest(&specv1.UnbindOIDCRequest{
			BindingId: args[0], UserId: oidcUnbindUser, Force: oidcUnbindForce,
		})); err != nil {
			return fmt.Errorf("unbind oidc: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Unbound %s\n", args[0])
		return nil
	},
}
```

In `auth_oidc.go`'s `init()`, add:

```go
	authOIDCUnbindCmd.Flags().StringVar(&oidcUnbindUser, "user", "", "owner of the binding (required)")
	authOIDCUnbindCmd.Flags().BoolVar(&oidcUnbindForce, "force", false, "allow removing the user's only credential")
```

- [ ] **Step 3: Verify build + help**

Run:

```bash
go build ./...
go run ./cmd/specgraph auth user delete --help
go run ./cmd/specgraph auth oidc unbind --help
```

Expected: build PASS; `--force` on delete/purge; `--user`/`--force` on unbind.

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/auth_user.go cmd/specgraph/auth_oidc.go
git commit -s -m "feat(cli): --force on user delete/purge; --user/--force on oidc unbind"
```

---

## Task 11: Integration test — protections through the interceptor

**Files:**

- Modify: `internal/server/identity_integration_test.go` (the 4a file)

**Covers:** E2E (bootstrap-delete refused without force, allowed with; last-credential unbind refused without force).

- [ ] **Step 1: Write the test**

Append to `internal/server/identity_integration_test.go` (reuses the 4a `buildIdentityTestServer`/`mintFor`/`tokenClient` helpers):

```go
func TestIntegration_BootstrapDeleteProtected(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true) // bootstrap admin
	client := tokenClient(baseURL, adminToken)

	// Find the bootstrap admin's id.
	boot, err := store.GetBootstrap(ctx)
	require.NoError(t, err)

	// Without force: refused.
	_, err = client.SoftDeleteUser(ctx, connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: boot.ID}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// With force: allowed.
	_, err = client.SoftDeleteUser(ctx, connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: boot.ID, Force: true}))
	require.NoError(t, err)
}

func TestIntegration_LastCredentialUnbindProtected(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true)
	client := tokenClient(baseURL, adminToken)

	// Create a Human with exactly one OIDC binding and no API keys.
	u, err := store.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "person", Role: "reader"},
		&storage.OIDCBinding{Issuer: "https://idp", Subject: "sub-1"})
	require.NoError(t, err)
	bindings, err := store.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)

	// Without force: refused (only credential).
	_, err = client.UnbindOIDC(ctx, connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: bindings[0].ID, UserId: u.ID}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// With force: allowed.
	_, err = client.UnbindOIDC(ctx, connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: bindings[0].ID, UserId: u.ID, Force: true}))
	require.NoError(t, err)
}
```

> `CreateHuman` with a binding: per the Storage plan, `CreateHuman(ctx, u, binding)` creates the Human + binding atomically. If the merged signature differs, adapt. The admin token's user is the bootstrap admin (created by `mintFor(...,"admin",true)`), so `GetBootstrap` finds it.

- [ ] **Step 2: Run**

Run: `cd internal/server && go test -tags integration -run 'TestIntegration_BootstrapDeleteProtected|TestIntegration_LastCredentialUnbindProtected' -v`

Expected: PASS.

- [ ] **Step 3: Full suites**

```bash
# from the repository root:
go build ./... && go test ./...
go test -tags integration ./internal/server/... ./internal/auth/... ./internal/bootstrap/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/server/identity_integration_test.go
git commit -s -m "test(server): integration tests for bootstrap + last-credential protections"
```

---

## Self-Review

**1. Spec coverage** (against `docs/plans/2026-05-22-identity-bootstrap-ux-design.md`):

- [x] Local-mode bootstrap via `specgraph init` (direct DB → create admin + write credentials): Tasks 3, 5.
- [x] Hosted bootstrap via server first-start (create admin + log banner, no file write): Tasks 3, 6.
- [x] Both paths idempotent + race-safe via a single shared helper: Task 3 (`bootstrap.Ensure`).
- [x] `init` detects which path from a working Postgres connection: Task 5 (`bootstrapOnInit`).
- [x] Credentials file is CLI-read-only, multi-server, doesn't clobber other entries: Tasks 1, 2, 5.
- [x] Bootstrap user is a system identity (`display_name="admin"`, no OIDC binding, never env-derived): Task 3 (asserted in `TestEnsure_CreatesBootstrapAdmin`).
- [x] Bootstrap soft-delete/purge force-flag protection: Tasks 7, 8, 10.
- [x] Last-credential `UnbindOIDC` protection; `RevokeAPIKey` deliberately NOT protected: Task 9 (and Task 9's docstring records the asymmetry; `RevokeAPIKey` is untouched). The guard also verifies the binding belongs to the named `user_id` (`containsBindingID`) so a credential-rich `user_id` can't mask removing another user's last binding — covered by `TestUnbindOIDC_RejectsBindingNotOwnedByUser`.
- [x] CLI under `specgraph auth` (4a) with `--force`/`--user` for protected ops: Task 10.
- [x] Credentials-file behavior change is release-noted: Task 1 (old `api_keys:` → empty; documented).

**2. Placeholder scan:** No TODO/TBD. The Task 3 test stub is sketched with a note, then Step 3 narrows `Ensure`'s dependency to a 3-method `Backend` interface so the stub needs no boilerplate — the note explicitly tells the executor to simplify. Every impl + test step shows complete code.

**3. Type consistency:** `credentials.File`/`ServerCreds`/`Load`/`Save`/`TokenFor`/`Upsert`, `bootstrap.Ensure`/`Options`/`Result`/`Backend`, `resolveAPIKey(serverURL)`, `requireForceForBootstrap`, `containsBindingID`, and the proto `GetForce()`/`GetUserId()` getters are referenced consistently. `bootstrap.Ensure` takes the narrow `Backend` interface (satisfied by `*postgres.AuthStore` and the test stubs). The token-assembly (`auth.GenerateAPIKeySecret` + `auth.FormatAPIKeyToken` over the storage-assigned prefix) matches 4a exactly.

**4. Build discipline:** Tasks 1, 3, 4 are isolated new packages. Task 2 fixes the post-Authn dangling reference (whole project builds after it) and updates all three `newAuthenticatedHTTPClient` callers (`newClient`, `newClientWithProject`, `read_mcp_resource.go`). Task 7 adds proto fields (backward-compatible; 4a handlers ignore them until 8–9). Tasks 8–9 add guards and explicitly flag the ripple into 4a's existing tests (the cross-task notes give the exact stub edits). Task 10 is CLI wiring. Every task ends with `go build ./... && go test ./...` green; Tasks 4, 11 add integration coverage. **Task 4 depends on Storage plan Task 32** (the exported `postgrestest.SharedPool` cross-package test-pool helper); see the Dependency contract. The `init` local-bootstrap (Task 5) uses a 2s dial timeout so the common no-local-DB `init` degrades quickly rather than stalling.

**5. Cross-plan ripple (called out, not hidden):** Adding the `GetUserByID` call in Task 8 and the credential-count in Task 9 changes what 4a's `TestSoftDeleteUser_Happy`/`TestPurgeUser_*`/`TestUnbindOIDC_Happy`/`_NotFound` stubs must provide. Tasks 8 and 9 each carry an explicit "Cross-task note" instructing the exact stub additions (`getUserByID` returning a non-bootstrap user; `UserId`+`Force` on the unbind happy/notfound tests). This is the one place 4b reaches back into 4a's tests; it is deliberate and enumerated.

**6. Symbol-lifetime sweep:** Completed above. Two new packages (`credentials`, `bootstrap`) — no collisions. `resolveAPIKey` signature change (not a new symbol). Additive proto fields. No deletions (the Authn plan already removed `auth/bootstrap.go`; Task 2 removes the last consumer of the deleted `auth.ReadDefaultKey`).

---

## Execution

1. **Subagent-Driven (recommended)** — Tasks 1, 3 (new packages) and 4 (integration) are clean isolated units. Task 2 (client.go) + Tasks 5–6 (init/serve wiring) are a small batch. Tasks 7–10 (proto + guards + CLI) form the protections batch; run 8–9 together since they touch the same handler + ripple into 4a tests. Task 11 is the capstone.
2. **Inline Execution** — practical for 7–10 (proto + guards + CLI) as one coordinated change.

**Epic completion:** 4b is the last of the four identity plans (Storage → Authn → Cedar → Bootstrap-UX-4a → 4b). With it merged, the epic delivers: database-backed identity, a single resolver, Cedar authorization, the operator RPC surface + CLI, and a working first-admin story for both local and hosted deployments. **Release notes MUST call out** the credentials-file schema change (old `api_keys:` no longer provisions server keys; operators use `specgraph auth api-key create` and the new multi-server `servers:` file) and the removal of YAML-driven key provisioning.
