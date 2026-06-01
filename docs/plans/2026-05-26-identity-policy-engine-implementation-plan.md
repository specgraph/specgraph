# Identity Policy Engine (Cedar) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt `cedar-policy/cedar-go` as SpecGraph's embedded authorization engine, replacing the `StaticTableAuthorizer` introduced by the Authn plan with a `CedarAuthorizer` that slots into the existing `auth.Authorizer` seam — so the interceptor diff is **zero**.

**Architecture:** Build-alongside-then-switch-then-delete. Cedar is wrapped behind a SpecGraph-owned `auth.PolicyEngine` interface; only the engine wrapper imports cedar-go. Policies load from an ordered list of `PolicySource`s (embedded built-ins + operator directories). A `(service, method) → stable action name` map decouples policy from RPC method names. The static `rpcPermissions` table and the whole `StaticTableAuthorizer` are deleted in one Phase-C task once `serve.go` is switched and nothing references them. Every task ends with a compiling, testable package at BOTH package and whole-project level.

**Tech Stack:** Go 1.26, ConnectRPC, `github.com/cedar-policy/cedar-go v1.7.0` (new dep), `embed` (built-in policies), `log/slog` (decision logging).

**Implements bead:** Implementation of approved design `spgr-rjrt.1` (`docs/plans/2026-05-26-identity-policy-engine-design.md`) under epic `spgr-rjrt`. Depends on the Authn plan (`spgr-n2rw`) being merged first — it builds on the `Authorizer`/`Decision` seam, the `Resolver`/`Identity` shape, and the `StaticTableAuthorizer` that this plan deletes.

---

## Verified cedar-go v1.7.0 API (the code below is written against this, not an imagined API)

```go
// github.com/cedar-policy/cedar-go
func NewPolicySetFromBytes(fileName string, document []byte) (*cedar.PolicySet, error)
func NewPolicySet() *cedar.PolicySet
func (p *cedar.PolicySet) Add(id cedar.PolicyID, policy *cedar.Policy) bool
func (p *cedar.PolicySet) All() iter.Seq2[cedar.PolicyID, *cedar.Policy] // range-over-func (Go 1.23+)
func Authorize(policies cedar.PolicyIterator, entities types.EntityGetter, req cedar.Request) (cedar.Decision, cedar.Diagnostic)
func NewEntityUID(typ cedar.EntityType, id cedar.String) cedar.EntityUID
func NewEntityUIDSet(args ...cedar.EntityUID) cedar.EntityUIDSet
func NewRecord(m cedar.RecordMap) cedar.Record

type Request   = types.Request    // struct{ Principal, Action, Resource types.EntityUID; Context types.Record }
type Decision  = types.Decision   // bool; cedar.Allow == Decision(true)
type Diagnostic = types.Diagnostic // struct{ Reasons []DiagnosticReason; Errors []DiagnosticError }
type PolicyID  = types.PolicyID   // string
type EntityMap = types.EntityMap  // map[EntityUID]Entity; implements EntityGetter
type Entity    = types.Entity     // struct{ UID EntityUID; Parents EntityUIDSet; Attributes Record; Tags Record }
type RecordMap = types.RecordMap  // map[String]Value
type String, Boolean, Long        // value types
```

Two API facts that shape the plan:

1. **No schema-based type checking in v1.** cedar-go's schema validation lives in the experimental `x/exp/schema` package. We rely on parse-time validation (`NewPolicySetFromBytes` returns an error on malformed policy text), not schema typechecking. The design's "static type checking on policies at load time" is therefore *partially* realized; full schema validation is a deliberate follow-up, gated on cedar-go promoting it out of `x/exp`.
2. **`Authorize` is context-free and infallible at the cedar layer** — it returns `(Decision, Diagnostic)`, no `error`. Our `PolicyEngine.Evaluate` wrapper owns `context` handling and the error channel (e.g. unconfigured action).

Note: the design (written 2026-05-26) calls cedar-go "younger than the Rust original." As of this plan it is **v1.7.0** — post-1.0 and actively maintained — so that caveat is stale. The wrapper indirection is retained anyway, for the swap-point value the design describes.

---

## Testing approach

Four categories (same framework as the Storage and Authn plans). Each behavior-introducing task tags `**Covers:**`.

- **Happy** — normal success per code path (reader reads, admin deletes, policy loads).
- **Invariants** — system-wide rules (explicit `forbid` beats any `permit`; the built-in source MUST load or the engine refuses construction; action name is decoupled from RPC method name; the interceptor diff is zero).
- **Boundaries** — edges (unconfigured procedure → error; malformed policy text → load error; missing operator dir → error; empty dir → no error; unknown role → deny by default).
- **E2E** — full flows through interceptor → `CedarAuthorizer` → `PolicyEngine` → cedar.

Unit tests use an inline `stubSource` (Task 6) so the engine is exercised independently of the embedded file. Integration tests (`//go:build integration`) drive the real interceptor with the embedded policies. All `go test` run-commands carry `-tags integration` where they exercise tagged suites; package-level cycles run plain `go test`.

---

## File Structure

**Create (Phase A — alongside the StaticTableAuthorizer, which still serves auth until Task 16):**

- `internal/auth/policysource.go` — `PolicyDocument` struct + `PolicySource` interface.
- `internal/auth/embedded_source.go` — `EmbeddedPolicySource` + `//go:embed policies/*.cedar`.
- `internal/auth/policies/base.cedar` — the built-in base policies (migrated `rpcPermissions`).
- `internal/auth/directory_source.go` — `DirectoryPolicySource`.
- `internal/auth/engine.go` — `PolicyEngine` interface, `EvalRequest`, `ResourceRef`, `PolicyDecision`, `cedarEngine` impl. **`engine.go` is the sole cedar-go importer** — `embedded_source.go`/`directory_source.go` import only `embed`/`os`/`io/fs`, never cedar. This keeps Cedar types out of every other file.
- `internal/auth/actions.go` — `procedureActions` map (`(service,method) → action name`), `ActionNames()`, `actionForProcedure`, `actionVerb`, `actionDomain`.
- `internal/auth/exempt.go` — `exemptProcedures` + `IsExempt`, **relocated** out of `permissions.go` so they survive `permissions.go`'s deletion.
- `internal/auth/cedar_authorizer.go` — `CedarAuthorizer` implementing `auth.Authorizer`; `NewCedarAuthorizer`.
- `internal/auth/known_roles.go` — `KnownRolesFrom([]string) map[Role]bool` (replaces the role-name set previously derived from `LoadRolePerms`).
- Test files for each of the above (`*_test.go`), plus `internal/auth/cedar_integration_test.go` (`//go:build integration`).

**Modify (Phase B — the switch):**

- `internal/config/global.go` — change `AuthConfig.Roles` from `map[string]RoleConfig` to `[]string`; delete the `RoleConfig` type; add `AuthConfig.Policies PolicyConfig{ ExtraDirs []string }`.
- `cmd/specgraph/serve.go` — replace the `LoadRolePerms` + `NewStaticTableAuthorizer` block with policy-source + `NewCedarEngine` + `NewCedarAuthorizer` construction; derive `KnownRoles` via `KnownRolesFrom`. **The `auth.NewAuthInterceptor(resolver, authorizer)` line is byte-for-byte unchanged.**

**Delete (Phase C — once nothing references them):**

- `internal/auth/static_authorizer.go` + `static_authorizer_test.go` (`StaticTableAuthorizer`, `NewStaticTableAuthorizer`, `DefaultRolePermissions`, `hasPermissionInternal`, `LoadRolePerms`).
- `internal/auth/permissions.go` (`rpcPermissions`, `RPCPermission`) — deleted in the SAME task as `static_authorizer.go`, because `StaticTableAuthorizer.Authorize` is `rpcPermissions`' only consumer; removing one without the other leaves an `unused` lint failure. `exemptProcedures`/`IsExempt` already moved to `exempt.go` in Task 12.
- The `rpcPermissions`-specific tests in `permissions_test.go` (the exempt tests move to `exempt_test.go` in Task 12).

**Do NOT touch:** `interceptor.go`, `middleware.go`, `resolver.go`, `identitystore.go`, `oidc_verifier.go`, `auth.go`, `store.go`, `context.go` — Cedar changes none of them. That invariance is the design's headline payoff and is asserted in Task 16 and Task 18.

---

## Symbol-lifetime sweep (the bug class that bit the Authn plan three times)

Go has one namespace per package and no overloading. Before finalizing, every NEW package-level identifier Cedar introduces was greppe�d against the post-Authn `internal/auth/*.go` surface. Results:

| New identifier | Collision? | Notes |
|---|---|---|
| `PolicyEngine`, `cedarEngine`, `NewCedarEngine` | none | — |
| `EvalRequest`, `ResourceRef`, `PolicyDecision` | none | `PolicyDecision` is deliberately **not** `Decision` — `auth.Decision{Allowed,Reason}` already exists (Authn). `CedarAuthorizer` returns the existing `auth.Decision`; the engine returns the richer `PolicyDecision`. |
| `PolicySource`, `PolicyDocument` | none | `PolicyDocument` (not `Policy`) chosen to avoid reader confusion with `cedar.Policy` in `engine.go`. |
| `EmbeddedPolicySource`, `DirectoryPolicySource`, `NewEmbeddedPolicySource`, `NewDirectoryPolicySource` | none | — |
| `CedarAuthorizer`, `NewCedarAuthorizer` | none | — |
| `procedureActions`, `ActionNames`, `actionForProcedure`, `actionVerb`, `actionDomain` | none | `rpcPermissions`/`RPCPermission` still exist until Task 17 but have different names. |
| `exemptProcedures`, `IsExempt` | **moved, not new** | Relocated from `permissions.go` → `exempt.go` in Task 12. The post-Authn interceptor (`interceptor.go`) calls `IsExempt`; it MUST keep resolving. |
| `KnownRolesFrom` | none | Replaces the role-name extraction `serve.go` did from `LoadRolePerms`. |
| `embeddedPolicyFS` (embed var) | none | — |

Survivor-after-deletion check (Task 17 deletes `static_authorizer.go` + `permissions.go`):

- `StaticTableAuthorizer` / `NewStaticTableAuthorizer` — only consumers: `serve.go` (switched in Task 16) and `static_authorizer_test.go` (deleted with the file). ✓
- `DefaultRolePermissions`, `LoadRolePerms` — only consumers: `serve.go` (switched) and the deleted test. (Authn's Phase C already removed `config_store.go`'s usage.) ✓
- `hasPermissionInternal` — only consumer: `StaticTableAuthorizer.Authorize` (deleted). (Authn's Phase C already removed the exported `HasPermission`.) ✓
- `rpcPermissions` / `RPCPermission` — only consumer: `StaticTableAuthorizer.Authorize` (deleted in the same task). The post-Authn interceptor uses `authorizer.Authorize`, **not** `RPCPermission`. ✓
- `IsExempt` / `exemptProcedures` — consumer `interceptor.go` survives; symbols relocated to `exempt.go` (Task 12) BEFORE `permissions.go` is deleted. ✓

---

## Task 1: Add the cedar-go dependency and pin the core API with a smoke test

Before building anything on cedar-go, prove the three calls the whole plan rests on — parse policy text, build entities, authorize — work exactly as the API docs claim. This test is throwaway scaffolding for confidence; it lives in a temporary file deleted at the end of the task.

**Files:**

- Modify: `go.mod`, `go.sum`
- Create (temporary): `internal/auth/cedar_smoke_test.go`

**Covers:** Happy (parse + authorize allow) + Invariant (cedar-go API shape is what the plan assumes).

- [ ] **Step 1: Add the dependency**

Run:

```bash
go get github.com/cedar-policy/cedar-go@v1.7.0
go mod tidy
```

Expected: `go.mod` gains `github.com/cedar-policy/cedar-go v1.7.0` as a direct dependency; `go.sum` updated.

- [ ] **Step 2: Write the smoke test**

Create `internal/auth/cedar_smoke_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/stretchr/testify/require"
)

// TestCedarSmoke pins the three cedar-go calls the plan depends on:
// parse policy text, build an EntityMap (incl. an action group via Parents),
// and Authorize. If cedar-go's API differs from the plan's assumptions, this
// fails first — before any real code is written against a wrong API.
func TestCedarSmoke(t *testing.T) {
	policy := `permit (
		principal,
		action in SpecGraph::Action::"read",
		resource
	) when { principal has role && principal.role == "reader" };`

	ps, err := cedar.NewPolicySetFromBytes("smoke:base.cedar", []byte(policy))
	require.NoError(t, err)

	readGroup := cedar.NewEntityUID("SpecGraph::Action", "read")
	specRead := cedar.NewEntityUID("SpecGraph::Action", "spec.read")
	principal := cedar.NewEntityUID("SpecGraph::User", "u1")
	resource := cedar.NewEntityUID("SpecGraph::Resource", "spec")

	// Construct Parents explicitly (even empty) rather than relying on the
	// zero value of EntityUIDSet — so a failure here is about the action-group
	// mechanism, not about whether Authorize tolerates a nil-backed set.
	entities := cedar.EntityMap{
		readGroup: {UID: readGroup, Parents: cedar.NewEntityUIDSet(), Attributes: cedar.NewRecord(nil)},
		specRead:  {UID: specRead, Parents: cedar.NewEntityUIDSet(readGroup), Attributes: cedar.NewRecord(nil)},
		principal: {
			UID:        principal,
			Parents:    cedar.NewEntityUIDSet(),
			Attributes: cedar.NewRecord(cedar.RecordMap{"role": cedar.String("reader")}),
		},
		resource: {UID: resource, Parents: cedar.NewEntityUIDSet(), Attributes: cedar.NewRecord(nil)},
	}

	req := cedar.Request{
		Principal: principal,
		Action:    specRead,
		Resource:  resource,
		Context:   cedar.NewRecord(nil),
	}

	dec, diag := cedar.Authorize(ps, entities, req)
	require.Equal(t, cedar.Allow, dec)
	require.NotEmpty(t, diag.Reasons, "an allow decision must cite the matching policy")

	// A writer-only request against the same read policy must NOT match.
	entities[principal] = cedar.Entity{
		UID:        principal,
		Attributes: cedar.NewRecord(cedar.RecordMap{"role": cedar.String("nobody")}),
	}
	dec, _ = cedar.Authorize(ps, entities, req)
	require.Equal(t, cedar.Deny, dec)
}
```

- [ ] **Step 2b: Verify failure first (sanity)**

Temporarily break the policy text (e.g. change `principal.role == "reader"` to `principal.role == "admin"`) and run:

Run: `cd internal/auth && go test -run TestCedarSmoke -v`

Expected: FAIL on `require.Equal(t, cedar.Allow, dec)` — confirms the test actually exercises evaluation. Then revert the edit.

- [ ] **Step 3: Run the smoke test**

Run: `cd internal/auth && go test -run TestCedarSmoke -v`

Expected: PASS. This is the proof the action-group-via-`Parents` mechanism works in cedar-go v1.7.0. **If this fails**, do not proceed — the action model assumption is wrong; revisit the design's entity model before writing `engine.go`.

- [ ] **Step 4: Delete the smoke test and verify the build**

```bash
rm internal/auth/cedar_smoke_test.go
go build ./... && go test ./...
```

Expected: PASS. (The dependency stays in `go.mod`; the throwaway test is gone — the real engine tests in later tasks supersede it.)

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -s -m "build(auth): add cedar-policy/cedar-go v1.7.0 dependency"
```

---

## Task 2: Define `PolicyDocument` and the `PolicySource` interface

The seam that lets policy storage evolve (embedded → directory → DB → URL) without touching the engine.

**Files:**

- Create: `internal/auth/policysource.go`

**Covers:** N/A (definitions only).

- [ ] **Step 1: Write the file**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// PolicyDocument is one unit of Cedar policy text plus a stable identifier
// for diagnostics and decision logs. Named PolicyDocument (not Policy) to
// avoid confusion with cedar.Policy inside engine.go.
type PolicyDocument struct {
	// Source identifies where this document came from, e.g.
	// "embedded:base.cedar" or "dir:/etc/specgraph/policies/extra.cedar".
	// Used as the filename argument to cedar.NewPolicySetFromBytes and as a
	// prefix on merged policy IDs so a decision log can name the origin.
	Source string
	// Text is the raw Cedar policy text (one or more policies).
	Text string
}

// PolicySource yields Cedar policy documents from some backing store.
// Implementations: EmbeddedPolicySource (built-ins), DirectoryPolicySource
// (operator files). DB- and URL-backed sources are deliberate follow-ups
// requiring no engine change — only a new PolicySource.
type PolicySource interface {
	// Load returns this source's policy documents. A source MAY return an
	// empty slice with no error (it simply contributes nothing).
	Load(ctx context.Context) ([]PolicyDocument, error)
	// Name returns a short identifier for diagnostics and decision logs.
	Name() string
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/policysource.go
git commit -s -m "feat(auth): define PolicyDocument and PolicySource interface"
```

---

## Task 3: Implement `EmbeddedPolicySource` with the migrated base policies

The built-in policies, compiled into the binary. This is also where the `rpcPermissions` table's behavior is re-expressed as three Cedar policies (verb action-groups).

**Files:**

- Create: `internal/auth/policies/base.cedar`
- Create: `internal/auth/embedded_source.go`
- Create: `internal/auth/embedded_source_test.go`

**Covers:** Happy (loads the embedded document) + Invariant (built-in policies are present and parseable).

- [ ] **Step 1: Write the base policy file**

Create `internal/auth/policies/base.cedar` (no SPDX header — `.cedar` is not a license-checked extension):

```
// SpecGraph base authorization policies.
//
// Migrated from the static rpcPermissions table. Roles gate VERBS via action
// groups: every concrete action (e.g. SpecGraph::Action::"spec.read") is a
// member of its verb group (SpecGraph::Action::"read") via the entity graph
// the engine builds at construction. New RPCs join a group by being added to
// internal/auth/actions.go — these policies do not change.
//
// Cedar semantics: default-deny; any matching permit allows; an explicit
// forbid (none here) would beat every permit. Discrete per-action and
// ownership policies for later stories layer on top via additional sources.

permit (
	principal,
	action in SpecGraph::Action::"read",
	resource
) when {
	principal has role &&
	(principal.role == "reader" || principal.role == "writer" || principal.role == "admin")
};

permit (
	principal,
	action in SpecGraph::Action::"write",
	resource
) when {
	principal has role &&
	(principal.role == "writer" || principal.role == "admin")
};

permit (
	principal,
	action in SpecGraph::Action::"delete",
	resource
) when {
	principal has role && principal.role == "admin"
};
```

> Semantics check against the old table: the only `delete` permission was `graph:delete` (RemoveEdge). Old writer = `{*:read, *:write}` (no delete); old admin = `{*:*}`. So under the migration: read → reader∪writer∪admin; write → writer∪admin; delete → admin only. Identical behavior.

- [ ] **Step 2: Write the embedded source**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed policies/*.cedar
var embeddedPolicyFS embed.FS

// EmbeddedPolicySource serves the built-in policies compiled into the
// binary. The built-in source MUST load successfully; a failure here means
// the binary was built wrong (the engine refuses to start — see
// NewCedarEngine).
type EmbeddedPolicySource struct{}

// NewEmbeddedPolicySource returns the built-in policy source.
func NewEmbeddedPolicySource() EmbeddedPolicySource { return EmbeddedPolicySource{} }

// Name implements PolicySource.
func (EmbeddedPolicySource) Name() string { return "embedded" }

// Load implements PolicySource. Reads every *.cedar file under policies/.
func (EmbeddedPolicySource) Load(_ context.Context) ([]PolicyDocument, error) {
	entries, err := fs.ReadDir(embeddedPolicyFS, "policies")
	if err != nil {
		return nil, fmt.Errorf("read embedded policies dir: %w", err)
	}
	docs := make([]PolicyDocument, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cedar") {
			continue
		}
		b, readErr := embeddedPolicyFS.ReadFile("policies/" + e.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read embedded policy %s: %w", e.Name(), readErr)
		}
		docs = append(docs, PolicyDocument{Source: "embedded:" + e.Name(), Text: string(b)})
	}
	return docs, nil
}
```

- [ ] **Step 3: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"strings"
	"testing"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestEmbeddedPolicySource_LoadsBasePolicies(t *testing.T) {
	docs, err := auth.NewEmbeddedPolicySource().Load(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, docs, "embedded base policies must be present")

	var combined strings.Builder
	for _, d := range docs {
		require.True(t, strings.HasPrefix(d.Source, "embedded:"), "source tag: %s", d.Source)
		combined.WriteString(d.Text)
		combined.WriteString("\n")
	}

	// The embedded text must parse as valid Cedar.
	_, err = cedar.NewPolicySetFromBytes("embedded:test", []byte(combined.String()))
	require.NoError(t, err, "embedded base policies must parse")

	// Behavior anchor: the three verb groups are referenced.
	text := combined.String()
	require.Contains(t, text, `action in SpecGraph::Action::"read"`)
	require.Contains(t, text, `action in SpecGraph::Action::"write"`)
	require.Contains(t, text, `action in SpecGraph::Action::"delete"`)
}

func TestEmbeddedPolicySource_Name(t *testing.T) {
	require.Equal(t, "embedded", auth.NewEmbeddedPolicySource().Name())
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestEmbeddedPolicySource -v`

Expected: PASS (impl from Step 2 is real; the embed directive picks up `policies/base.cedar`).

- [ ] **Step 5: Commit**

```bash
git add internal/auth/policies/base.cedar internal/auth/embedded_source.go internal/auth/embedded_source_test.go
git commit -s -m "feat(auth): embed base Cedar policies migrated from rpcPermissions"
```

---

## Task 4: Implement `DirectoryPolicySource`

Operator-supplied policy files (configured later via `auth.policies.extra_dirs`). Required-by-default semantics: a missing/unreadable dir is an error; an empty dir is fine.

**Files:**

- Create: `internal/auth/directory_source.go`
- Create: `internal/auth/directory_source_test.go`

**Covers:** Happy (reads `.cedar` files) + Boundary (missing dir → error; empty dir → no error, no docs; non-`.cedar` files ignored).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestDirectoryPolicySource_ReadsCedarFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "extra.cedar"),
		[]byte(`permit (principal, action, resource) when { principal has role && principal.role == "admin" };`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"),
		[]byte("ignored"), 0o600))

	docs, err := auth.NewDirectoryPolicySource(dir).Load(context.Background())
	require.NoError(t, err)
	require.Len(t, docs, 1, "only .cedar files are loaded")
	require.Contains(t, docs[0].Source, "extra.cedar")
}

func TestDirectoryPolicySource_MissingDirIsError(t *testing.T) {
	_, err := auth.NewDirectoryPolicySource("/no/such/dir/specgraph").Load(context.Background())
	require.Error(t, err)
}

func TestDirectoryPolicySource_EmptyDirIsOK(t *testing.T) {
	docs, err := auth.NewDirectoryPolicySource(t.TempDir()).Load(context.Background())
	require.NoError(t, err)
	require.Empty(t, docs)
}

func TestDirectoryPolicySource_Name(t *testing.T) {
	require.Equal(t, "dir:/etc/x", auth.NewDirectoryPolicySource("/etc/x").Name())
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestDirectoryPolicySource -v`

Expected: FAIL ("undefined: auth.NewDirectoryPolicySource").

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirectoryPolicySource serves Cedar policy files from a filesystem
// directory. Required-by-default: a missing or unreadable directory is an
// error (operators who configure a policy dir mean it). An existing but
// empty directory (no *.cedar files) returns no documents and no error.
type DirectoryPolicySource struct {
	dir string
}

// NewDirectoryPolicySource returns a source rooted at dir.
func NewDirectoryPolicySource(dir string) DirectoryPolicySource {
	return DirectoryPolicySource{dir: dir}
}

// Name implements PolicySource.
func (s DirectoryPolicySource) Name() string { return "dir:" + s.dir }

// Load implements PolicySource. Reads every *.cedar file in the directory
// (non-recursive; nested dirs are ignored).
func (s DirectoryPolicySource) Load(_ context.Context) ([]PolicyDocument, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read policy dir %s: %w", s.dir, err)
	}
	docs := make([]PolicyDocument, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cedar") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		b, readErr := os.ReadFile(path) //nolint:gosec // operator-configured policy dir
		if readErr != nil {
			return nil, fmt.Errorf("read policy %s: %w", path, readErr)
		}
		docs = append(docs, PolicyDocument{Source: "dir:" + path, Text: string(b)})
	}
	return docs, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestDirectoryPolicySource -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/directory_source.go internal/auth/directory_source_test.go
git commit -s -m "feat(auth): add DirectoryPolicySource for operator policy files"
```

---

## Task 5: Define the engine types — `PolicyEngine`, `EvalRequest`, `ResourceRef`, `PolicyDecision`

The SpecGraph-owned vocabulary the rest of the codebase uses. No cedar types here.

**Files:**

- Create: `internal/auth/engine.go` (types only in this task; impl in Tasks 6–10)

**Covers:** N/A (definitions only).

- [ ] **Step 1: Write the type declarations**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// PolicyEngine evaluates an authorization request against a loaded policy
// set. Cedar is wrapped behind this interface so the rest of the codebase
// never imports cedar-go directly. The only implementation is cedarEngine
// (this file); the only file importing cedar-go is engine.go.
type PolicyEngine interface {
	// Evaluate decides the request. Returns an error only for operational
	// failures (the engine could not reach a decision); a clean Deny is a
	// successful evaluation with PolicyDecision.Allowed == false.
	Evaluate(ctx context.Context, req EvalRequest) (PolicyDecision, error)
	// Reload re-reads every PolicySource and atomically swaps the active
	// policy set. Not wired to any signal in v1 (restart applies policy
	// changes); present for the future reload story and exercised by tests.
	Reload(ctx context.Context) error
}

// EvalRequest is a SpecGraph-shaped authorization question, mapped to a
// cedar.Request inside the engine.
type EvalRequest struct {
	// Identity is the resolved principal. Its EffectiveRole becomes the
	// Cedar principal's "role" attribute.
	Identity *Identity
	// Action is the stable action name (e.g. "spec.read"), already mapped
	// from the RPC procedure by the caller. Decoupled from method names.
	Action string
	// Resource describes what is being acted on. For the migration this is
	// a placeholder derived from the action's domain; stories #3/#4 populate
	// real resource attributes here.
	Resource ResourceRef
	// Context carries transient request attributes (e.g. project slug).
	// Empty for the migration; the Cedar context Record is built from it.
	Context map[string]string
}

// ResourceRef projects a resource into Cedar's (type, id, attributes) shape.
type ResourceRef struct {
	Type       string            // Cedar resource entity id namespace, e.g. "spec"
	ID         string            // resource id; "" → "unspecified"
	Attributes map[string]string // e.g. {"owner_user_id": "..."}; empty in migration
}

// PolicyDecision is the engine's full result: the allow/deny plus the
// policy IDs that drove it (for decision logs / future audit) and any
// evaluation errors Cedar surfaced.
type PolicyDecision struct {
	Allowed         bool
	MatchedPolicies []string // cedar policy IDs from Diagnostic.Reasons
	Errors          []string // stringified Diagnostic.Errors (rare; bad policy/attr)
}
```

- [ ] **Step 2: Verify compile**

Run: `cd internal/auth && go build ./...`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/engine.go
git commit -s -m "feat(auth): define PolicyEngine, EvalRequest, ResourceRef, PolicyDecision"
```

---

## Task 6: Implement `cedarEngine` construction and `Reload` (policy loading)

`NewCedarEngine` loads all sources into one merged `*cedar.PolicySet`, stored in an `atomic.Pointer` so the eval hot path is lock-free. The built-in source MUST yield at least one parseable policy.

**Files:**

- Modify: `internal/auth/engine.go`
- Create: `internal/auth/engine_test.go`

**Covers:** Happy (loads from a source) + Invariant (empty/unparseable built-in refuses construction) + Boundary (source Load error propagates) + (Reload swaps the set).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

// stubSource is an in-memory PolicySource for engine unit tests, so the
// engine is exercised independently of the embedded file.
type stubSource struct {
	name    string
	docs    []auth.PolicyDocument
	loadErr error
}

func (s stubSource) Name() string { return s.name }
func (s stubSource) Load(context.Context) ([]auth.PolicyDocument, error) {
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return s.docs, nil
}

// basePolicies is the three-policy verb-group set reused across engine tests.
const basePolicies = `
permit (principal, action in SpecGraph::Action::"read", resource)
when { principal has role && (principal.role == "reader" || principal.role == "writer" || principal.role == "admin") };
permit (principal, action in SpecGraph::Action::"write", resource)
when { principal has role && (principal.role == "writer" || principal.role == "admin") };
permit (principal, action in SpecGraph::Action::"delete", resource)
when { principal has role && principal.role == "admin" };
`

func baseSource() auth.PolicySource {
	return stubSource{name: "test", docs: []auth.PolicyDocument{{Source: "test:base.cedar", Text: basePolicies}}}
}

func testActions() []string { return []string{"spec.read", "spec.write", "graph.delete"} }

func TestNewCedarEngine_LoadsPolicies(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	require.NotNil(t, eng)
}

func TestNewCedarEngine_NoPoliciesIsError(t *testing.T) {
	empty := stubSource{name: "empty", docs: nil}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{empty}, testActions())
	require.Error(t, err, "no loaded policies must refuse construction")
}

func TestNewCedarEngine_BadPolicyTextIsError(t *testing.T) {
	bad := stubSource{name: "bad", docs: []auth.PolicyDocument{{Source: "bad:x.cedar", Text: "this is not cedar"}}}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{bad}, testActions())
	require.Error(t, err)
}

func TestNewCedarEngine_SourceLoadErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	failing := stubSource{name: "failing", loadErr: sentinel}
	_, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{failing}, testActions())
	require.ErrorIs(t, err, sentinel)
}

func TestCedarEngine_Reload(t *testing.T) {
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	require.NoError(t, eng.Reload(context.Background()))
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestNewCedarEngine|TestCedarEngine_Reload' -v`

Expected: FAIL ("undefined: auth.NewCedarEngine").

- [ ] **Step 3: Write the construction + loading impl**

Append to `internal/auth/engine.go`:

```go
import (
	"context"
	"fmt"
	"sync/atomic"

	cedar "github.com/cedar-policy/cedar-go"
)
```

> Replace the existing minimal import block (`"context"`) with the block above — `engine.go` now needs `fmt`, `sync/atomic`, and the cedar-go import.

```go
// cedarEngine is the sole PolicyEngine implementation and the sole file
// importing cedar-go. Policies live in an atomic.Pointer for a lock-free
// eval hot path; Reload builds a fresh set and swaps it.
type cedarEngine struct {
	sources        []PolicySource
	actionEntities cedar.EntityMap // precomputed action + verb-group entities
	policies       atomic.Pointer[cedar.PolicySet]
}

// NewCedarEngine loads every source into one merged policy set and
// precomputes the action-group entity graph from actionNames. The built-in
// source MUST contribute at least one parseable policy: a zero-policy result
// is a build error (the binary shipped without its base policies) and
// construction fails.
func NewCedarEngine(ctx context.Context, sources []PolicySource, actionNames []string) (*cedarEngine, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("cedar: NewCedarEngine: at least one PolicySource required")
	}
	actionEntities, err := buildActionEntities(actionNames)
	if err != nil {
		return nil, fmt.Errorf("cedar: build action entities: %w", err)
	}
	eng := &cedarEngine{sources: sources, actionEntities: actionEntities}
	if err := eng.Reload(ctx); err != nil {
		return nil, err
	}
	return eng, nil
}

// Reload re-reads all sources, parses + merges them, and atomically swaps
// the active policy set. Safe to call concurrently with Evaluate.
func (e *cedarEngine) Reload(ctx context.Context) error {
	set, err := loadPolicySet(ctx, e.sources)
	if err != nil {
		return err
	}
	count := 0
	for range set.All() {
		count++
	}
	if count == 0 {
		return fmt.Errorf("cedar: no policies loaded from %d source(s); refusing to start", len(e.sources))
	}
	e.policies.Store(set)
	return nil
}

// loadPolicySet parses each source's documents and merges them into one
// PolicySet. Policy IDs are prefixed with the document source so decision
// logs can name the origin and IDs never collide across documents.
func loadPolicySet(ctx context.Context, sources []PolicySource) (*cedar.PolicySet, error) {
	combined := cedar.NewPolicySet()
	for _, src := range sources {
		docs, err := src.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("cedar: load source %s: %w", src.Name(), err)
		}
		for _, doc := range docs {
			ps, parseErr := cedar.NewPolicySetFromBytes(doc.Source, []byte(doc.Text))
			if parseErr != nil {
				return nil, fmt.Errorf("cedar: parse %s: %w", doc.Source, parseErr)
			}
			for id, p := range ps.All() {
				// Prefix the per-document policy id with the source so ids are
				// globally unique and a decision log names the origin.
				// PolicySet.Add returns false if the id already exists; that
				// would mean a policy was silently dropped, so treat it as a
				// programming error rather than ignoring the result.
				mergedID := cedar.PolicyID(doc.Source + "#" + string(id))
				if !combined.Add(mergedID, p) {
					return nil, fmt.Errorf("cedar: duplicate policy id %q while merging %s", mergedID, doc.Source)
				}
			}
		}
	}
	return combined, nil
}
```

> `buildActionEntities` and `Evaluate` are added in Tasks 7 and 9. To keep this task compiling, add a temporary stub for `buildActionEntities` and `Evaluate` at the end of `engine.go`:
>
> ```go
> // Temporary stubs — real impls land in Tasks 7 and 9.
> func buildActionEntities(_ []string) (cedar.EntityMap, error) { return cedar.EntityMap{}, nil }
> func (e *cedarEngine) Evaluate(_ context.Context, _ EvalRequest) (PolicyDecision, error) {
> 	return PolicyDecision{}, fmt.Errorf("Evaluate not implemented")
> }
> ```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestNewCedarEngine|TestCedarEngine_Reload' -v`

Expected: PASS.

- [ ] **Step 5: Whole-project build + test**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/engine.go internal/auth/engine_test.go
git commit -s -m "feat(auth): cedarEngine construction + lock-free Reload"
```

---

## Task 7: Implement action-entity construction (verb groups)

Turn `["spec.read", "graph.delete", ...]` into the cedar entity graph: each concrete action parented to its verb group. This is what makes `action in SpecGraph::Action::"read"` resolve.

**Files:**

- Modify: `internal/auth/engine.go` (replace the `buildActionEntities` stub)
- Create: `internal/auth/engine_actions_test.go`

**Covers:** Happy (builds groups + concrete actions) + Boundary (unknown verb suffix → error).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"testing"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/stretchr/testify/require"
)

// In-package test (package auth, not auth_test) because buildActionEntities
// and actionVerb are unexported.

func TestBuildActionEntities_GroupsByVerb(t *testing.T) {
	ents, err := buildActionEntities([]string{"spec.read", "spec.write", "graph.delete"})
	require.NoError(t, err)

	readGroup := cedar.NewEntityUID("SpecGraph::Action", "read")
	specRead := cedar.NewEntityUID("SpecGraph::Action", "spec.read")

	require.Contains(t, ents, readGroup, "verb group entity must exist")
	require.Contains(t, ents, specRead, "concrete action entity must exist")

	specReadEnt := ents[specRead]
	require.True(t, specReadEnt.Parents.Contains(readGroup),
		"spec.read must be a member of the read group")
}

func TestBuildActionEntities_RejectsUnknownVerb(t *testing.T) {
	_, err := buildActionEntities([]string{"spec.frobnicate"})
	require.Error(t, err)
}
```

> `EntityUIDSet.Contains(EntityUID) bool` is part of cedar-go's `types.EntityUIDSet`. If the method name differs in v1.7.0, the Task 1 smoke test would not have caught it (it used `NewEntityUIDSet` only) — if `Contains` is undefined, substitute the membership check the API provides (iterate the set) and note it. The plan assumes `Contains` per the v1.7.0 `types` docs.

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestBuildActionEntities -v`

Expected: FAIL — the stub returns an empty map, so `require.Contains` fails.

- [ ] **Step 3: Replace the stub with the real impl**

**FIRST, locate and delete the Task 6 temporary stub** — Go has no overloading, so leaving it produces a "buildActionEntities redeclared" compile error. It is the two-line block at the bottom of `engine.go`:

```go
func buildActionEntities(_ []string) (cedar.EntityMap, error) { return cedar.EntityMap{}, nil }
```

(Leave the `Evaluate` stub for now — Task 9 removes it.) THEN add to `engine.go`:

```go
import "strings" // add to engine.go's import block
```

```go
// Cedar entity-type namespaces. Defined here (first use) and reused by the
// principal/resource helpers in Task 8 and Evaluate in Task 9.
const (
	entityTypeUser     = "SpecGraph::User"
	entityTypeResource = "SpecGraph::Resource"
	entityTypeAction   = "SpecGraph::Action"
)

// knownVerbs are the action suffixes that map to verb groups. The base
// policies gate roles per verb; an action whose suffix is not here cannot be
// authorized and is a programming error (caught at engine construction).
var knownVerbs = map[string]bool{"read": true, "write": true, "delete": true}

// actionVerb returns the verb suffix of an action name ("spec.read" -> "read").
func actionVerb(action string) (string, error) {
	idx := strings.LastIndex(action, ".")
	if idx < 0 || idx == len(action)-1 {
		return "", fmt.Errorf("action %q has no verb suffix", action)
	}
	verb := action[idx+1:]
	if !knownVerbs[verb] {
		return "", fmt.Errorf("action %q has unknown verb %q", action, verb)
	}
	return verb, nil
}

// actionDomain returns the domain prefix of an action name
// ("spec.read" -> "spec"). Used to derive the placeholder resource id.
func actionDomain(action string) string {
	if idx := strings.Index(action, "."); idx >= 0 {
		return action[:idx]
	}
	return action
}

// buildActionEntities turns action names into the cedar entity graph: each
// verb group (SpecGraph::Action::"read") and each concrete action
// (SpecGraph::Action::"spec.read") parented to its group. cedar resolves
// "action in <group>" through these Parents at Authorize time.
func buildActionEntities(actionNames []string) (cedar.EntityMap, error) {
	ents := cedar.EntityMap{}
	for _, name := range actionNames {
		verb, err := actionVerb(name)
		if err != nil {
			return nil, err
		}
		groupUID := cedar.NewEntityUID(entityTypeAction, cedar.String(verb))
		if _, ok := ents[groupUID]; !ok {
			ents[groupUID] = cedar.Entity{
				UID:        groupUID,
				Parents:    cedar.NewEntityUIDSet(),
				Attributes: cedar.NewRecord(nil),
			}
		}
		actionUID := cedar.NewEntityUID(entityTypeAction, cedar.String(name))
		ents[actionUID] = cedar.Entity{
			UID:        actionUID,
			Parents:    cedar.NewEntityUIDSet(groupUID),
			Attributes: cedar.NewRecord(nil),
		}
	}
	return ents, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestBuildActionEntities|TestNewCedarEngine|TestCedarEngine_Reload' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/engine.go internal/auth/engine_actions_test.go
git commit -s -m "feat(auth): build Cedar action-group entity graph from action names"
```

---

## Task 8: Implement principal and resource entity construction

Project the resolved `Identity` and the `ResourceRef` into cedar entities. Unexported helpers tested in-package.

**Files:**

- Modify: `internal/auth/engine.go`
- Modify: `internal/auth/engine_actions_test.go` (add cases — same in-package test file)

**Covers:** Happy (principal carries role/id/email; resource carries attributes) + Boundary (empty UserID falls back to Subject; empty resource id → "unspecified").

- [ ] **Step 1: Write the failing test**

Append to `internal/auth/engine_actions_test.go`:

```go
func TestPrincipalEntity_CarriesRoleAndID(t *testing.T) {
	id := &Identity{UserID: "u1", EffectiveRole: RoleWriter, Email: "a@example.com", Subject: "apikey:k1"}
	uid, ent := principalEntity(id)

	require.Equal(t, cedar.NewEntityUID("SpecGraph::User", "u1"), uid)
	role, ok := ent.Attributes.Get("role")
	require.True(t, ok)
	require.Equal(t, cedar.String("writer"), role)
	email, ok := ent.Attributes.Get("email")
	require.True(t, ok)
	require.Equal(t, cedar.String("a@example.com"), email)
}

func TestPrincipalEntity_FallsBackToSubject(t *testing.T) {
	id := &Identity{UserID: "", EffectiveRole: RoleReader, Subject: "apikey:k9"}
	uid, _ := principalEntity(id)
	require.Equal(t, cedar.NewEntityUID("SpecGraph::User", "apikey:k9"), uid)
}

func TestResourceEntity_Defaults(t *testing.T) {
	uid, ent := resourceEntity(ResourceRef{Type: "spec"})
	require.Equal(t, cedar.NewEntityUID("SpecGraph::Resource", "unspecified"), uid)
	_ = ent
}

func TestResourceEntity_CarriesAttributes(t *testing.T) {
	_, ent := resourceEntity(ResourceRef{Type: "apikey", ID: "key-1", Attributes: map[string]string{"owner_user_id": "u1"}})
	owner, ok := ent.Attributes.Get("owner_user_id")
	require.True(t, ok)
	require.Equal(t, cedar.String("u1"), owner)
}
```

> `cedar.Record.Get` has signature `Get(key cedar.String) (cedar.Value, bool)` in v1.7.0 — the key is `cedar.String`, NOT a plain `string`. The test calls above pass string *literals* (`"role"`, `"email"`), which assign to the `cedar.String` parameter as untyped constants, so they compile. A string *variable* would need an explicit `cedar.String(...)` conversion. Returned values are `cedar.Value`; compare against `cedar.String("...")`.

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestPrincipalEntity|TestResourceEntity' -v`

Expected: FAIL ("undefined: principalEntity").

- [ ] **Step 3: Write the impl**

Append to `internal/auth/engine.go`:

The entity-type constants (`entityTypeUser`, `entityTypeResource`, `entityTypeAction`) were defined in Task 7; reuse them here.

```go
// principalEntity projects the resolved Identity into a Cedar principal.
// principal.role is EffectiveRole (the authz-relevant, possibly-downgraded
// role). UserID is the entity id; legacy/edge identities without a UserID
// fall back to Subject so the principal still has a stable id.
func principalEntity(id *Identity) (cedar.EntityUID, cedar.Entity) {
	pid := id.UserID
	if pid == "" {
		pid = id.Subject
	}
	uid := cedar.NewEntityUID(entityTypeUser, cedar.String(pid))
	attrs := cedar.NewRecord(cedar.RecordMap{
		"role":  cedar.String(string(id.EffectiveRole)),
		"id":    cedar.String(id.UserID),
		"email": cedar.String(id.Email),
	})
	return uid, cedar.Entity{UID: uid, Parents: cedar.NewEntityUIDSet(), Attributes: attrs}
}

// resourceEntity projects a ResourceRef into a Cedar resource. For the
// migration the id is a placeholder ("unspecified"); stories #3/#4 pass real
// ids and attributes (e.g. owner_user_id) that ownership policies read.
func resourceEntity(r ResourceRef) (cedar.EntityUID, cedar.Entity) {
	id := r.ID
	if id == "" {
		id = "unspecified"
	}
	uid := cedar.NewEntityUID(entityTypeResource, cedar.String(id))
	rm := make(cedar.RecordMap, len(r.Attributes))
	for k, v := range r.Attributes {
		rm[cedar.String(k)] = cedar.String(v)
	}
	return uid, cedar.Entity{UID: uid, Parents: cedar.NewEntityUIDSet(), Attributes: cedar.NewRecord(rm)}
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestPrincipalEntity|TestResourceEntity|TestBuildActionEntities' -v`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/engine.go internal/auth/engine_actions_test.go
git commit -s -m "feat(auth): build Cedar principal and resource entities from Identity"
```

---

## Task 9: Implement `Evaluate` and pin the role × verb authorization matrix

The heart of the engine: build the cedar.Request, merge entities, call `cedar.Authorize`, map the result. This task's test is the contract for the entire action-group model.

**Files:**

- Modify: `internal/auth/engine.go` (replace the `Evaluate` stub)
- Create: `internal/auth/engine_evaluate_test.go`

**Covers:** Happy (each role's allowed verbs) + Invariant (role × verb matrix matches the old table exactly) + Boundary (unknown role → deny).

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
)

func newMatrixEngine(t *testing.T) auth.PolicyEngine {
	t.Helper()
	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)
	return eng
}

func evalRole(t *testing.T, eng auth.PolicyEngine, role auth.Role, action string) bool {
	t.Helper()
	dec, err := eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: role, Role: role},
		Action:   action,
		Resource: auth.ResourceRef{Type: "spec"},
	})
	require.NoError(t, err)
	return dec.Allowed
}

func TestEvaluate_RoleVerbMatrix(t *testing.T) {
	eng := newMatrixEngine(t)
	cases := []struct {
		role    auth.Role
		action  string
		allowed bool
	}{
		{auth.RoleReader, "spec.read", true},
		{auth.RoleReader, "spec.write", false},
		{auth.RoleReader, "graph.delete", false},
		{auth.RoleWriter, "spec.read", true},
		{auth.RoleWriter, "spec.write", true},
		{auth.RoleWriter, "graph.delete", false},
		{auth.RoleAdmin, "spec.read", true},
		{auth.RoleAdmin, "spec.write", true},
		{auth.RoleAdmin, "graph.delete", true},
	}
	for _, c := range cases {
		require.Equalf(t, c.allowed, evalRole(t, eng, c.role, c.action),
			"role=%s action=%s", c.role, c.action)
	}
}

func TestEvaluate_UnknownRoleDenied(t *testing.T) {
	eng := newMatrixEngine(t)
	require.False(t, evalRole(t, eng, auth.Role("auditor"), "spec.read"),
		"a role with no matching policy is denied by default")
}

func TestEvaluate_AllowCitesPolicy(t *testing.T) {
	eng := newMatrixEngine(t)
	dec, err := eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleAdmin, Role: auth.RoleAdmin},
		Action:   "graph.delete",
		Resource: auth.ResourceRef{Type: "graph"},
	})
	require.NoError(t, err)
	require.True(t, dec.Allowed)
	require.NotEmpty(t, dec.MatchedPolicies, "an allow must cite the matching policy id")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestEvaluate -v`

Expected: FAIL — `Evaluate` stub returns `"Evaluate not implemented"`.

- [ ] **Step 3: Write the real `Evaluate`**

**FIRST, locate and delete the Task 6 temporary `Evaluate` stub** — leaving it produces an "Evaluate redeclared" compile error. It is the block at the bottom of `engine.go`:

```go
func (e *cedarEngine) Evaluate(_ context.Context, _ EvalRequest) (PolicyDecision, error) {
	return PolicyDecision{}, fmt.Errorf("Evaluate not implemented")
}
```

(After this, no temporary stubs remain in `engine.go`.) THEN add the real impl (the import block already has `cedar`):

```go
// Evaluate maps the EvalRequest into a cedar.Request, merges the principal,
// resource, and precomputed action entities, and calls cedar.Authorize
// against the current policy set (loaded lock-free from the atomic pointer).
func (e *cedarEngine) Evaluate(_ context.Context, req EvalRequest) (PolicyDecision, error) {
	if req.Identity == nil {
		return PolicyDecision{}, fmt.Errorf("cedar: Evaluate: nil Identity")
	}
	ps := e.policies.Load()
	if ps == nil {
		return PolicyDecision{}, fmt.Errorf("cedar: Evaluate: no policy set loaded")
	}

	principalUID, principalEnt := principalEntity(req.Identity)
	resourceUID, resourceEnt := resourceEntity(req.Resource)
	actionUID := cedar.NewEntityUID(entityTypeAction, cedar.String(req.Action))

	// Merge precomputed action entities (immutable) with the per-request
	// principal and resource into a fresh EntityMap.
	entities := make(cedar.EntityMap, len(e.actionEntities)+2)
	for uid, ent := range e.actionEntities {
		entities[uid] = ent
	}
	entities[principalUID] = principalEnt
	entities[resourceUID] = resourceEnt

	ctxRecord := make(cedar.RecordMap, len(req.Context))
	for k, v := range req.Context {
		ctxRecord[cedar.String(k)] = cedar.String(v)
	}

	cedarReq := cedar.Request{
		Principal: principalUID,
		Action:    actionUID,
		Resource:  resourceUID,
		Context:   cedar.NewRecord(ctxRecord),
	}

	decision, diag := cedar.Authorize(ps, entities, cedarReq)

	matched := make([]string, 0, len(diag.Reasons))
	for _, r := range diag.Reasons {
		matched = append(matched, string(r.PolicyID))
	}
	var evalErrs []string
	for _, de := range diag.Errors {
		evalErrs = append(evalErrs, fmt.Sprintf("%v", de))
	}

	return PolicyDecision{
		Allowed:         decision == cedar.Allow,
		MatchedPolicies: matched,
		Errors:          evalErrs,
	}, nil
}
```

> `diag.Errors` elements are stringified with `%v` rather than accessing fields, so the code does not bet on `DiagnosticError`'s field names.

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestEvaluate -v`

Expected: PASS — the full role × verb matrix authorizes correctly through real cedar evaluation.

- [ ] **Step 5: Whole-project build + test**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/engine.go internal/auth/engine_evaluate_test.go
git commit -s -m "feat(auth): implement cedarEngine.Evaluate + pin role×verb matrix"
```

---

## Task 10: Add decision logging

Every evaluation emits a structured `slog.Debug` line — the natural source for audit-log story #1. Until that story lands a real sink, Debug is the trace-level observability.

**Files:**

- Modify: `internal/auth/engine.go` (add the log call in `Evaluate`)
- Create: `internal/auth/engine_logging_test.go`

**Covers:** Happy (a decision emits a structured log with action, principal, allowed, policies).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestEvaluate_EmitsDecisionLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)

	_, err = eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleAdmin, Subject: "apikey:k1"},
		Action:   "graph.delete",
		Resource: auth.ResourceRef{Type: "graph"},
	})
	require.NoError(t, err)

	out := buf.String()
	require.True(t, strings.Contains(out, "cedar decision"), "log: %s", out)
	require.Contains(t, out, "graph.delete")
	require.Contains(t, out, "allowed=true")
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestEvaluate_EmitsDecisionLog -v`

Expected: FAIL — no log emitted yet.

- [ ] **Step 3: Add the log call**

In `Evaluate` (engine.go), add `"log/slog"` to the import block, and immediately before the `return PolicyDecision{...}`:

```go
	slog.Debug("cedar decision",
		"action", req.Action,
		"principal", req.Identity.Subject,
		"role", string(req.Identity.EffectiveRole),
		"allowed", decision == cedar.Allow,
		"policies", matched,
	)
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestEvaluate -v`

Expected: PASS (logging test + matrix tests).

- [ ] **Step 5: Commit**

```bash
git add internal/auth/engine.go internal/auth/engine_logging_test.go
git commit -s -m "feat(auth): emit slog.Debug decision log per Cedar evaluation"
```

---

## Task 11: Build the `(service, method) → action name` map

The stable action vocabulary that replaces `rpcPermissions`. Same procedure keys; values are domain.verb action names decoupled from RPC method names.

**Files:**

- Create: `internal/auth/actions.go`
- Create: `internal/auth/actions_test.go`

**Covers:** Happy (procedure → action lookup) + Invariant (every action name parses to a known verb; action names are NOT method names) + Boundary (unconfigured procedure → not found).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

func TestActionForProcedure_KnownProcedure(t *testing.T) {
	action, ok := auth.ActionForProcedure(specgraphv1connect.SpecServiceGetSpecProcedure)
	require.True(t, ok)
	require.Equal(t, "spec.read", action)
}

func TestActionForProcedure_DeleteProcedure(t *testing.T) {
	action, ok := auth.ActionForProcedure(specgraphv1connect.GraphServiceRemoveEdgeProcedure)
	require.True(t, ok)
	require.Equal(t, "graph.delete", action)
}

func TestActionForProcedure_Unconfigured(t *testing.T) {
	_, ok := auth.ActionForProcedure("/no.such/Procedure")
	require.False(t, ok)
}

func TestActionNames_AllParseToKnownVerb(t *testing.T) {
	names := auth.ActionNames()
	require.NotEmpty(t, names)
	for _, n := range names {
		idx := strings.LastIndex(n, ".")
		require.Greater(t, idx, 0, "action %q must be domain.verb", n)
		verb := n[idx+1:]
		require.Contains(t, []string{"read", "write", "delete"}, verb, "action %q", n)
	}
}

func TestActionNames_DecoupledFromMethodNames(t *testing.T) {
	// The whole point: action names are domain.verb, never Service.Method.
	for _, n := range auth.ActionNames() {
		require.NotContains(t, n, "Service", "action %q leaks an RPC service name", n)
	}
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run 'TestActionForProcedure|TestActionNames' -v`

Expected: FAIL ("undefined: auth.ActionForProcedure").

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"sort"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// procedureActions maps each RPC procedure to a stable, RPC-method-decoupled
// action name (domain.verb). It replaces the rpcPermissions table: where
// rpcPermissions held "spec:read", this holds "spec.read", which the Cedar
// policies gate via the verb action-group. Renaming an RPC method changes
// only this map, not any policy.
var procedureActions = map[string]string{
	// SpecService
	specgraphv1connect.SpecServiceGetSpecProcedure:         "spec.read",
	specgraphv1connect.SpecServiceListSpecsProcedure:       "spec.read",
	specgraphv1connect.SpecServiceCreateSpecProcedure:      "spec.write",
	specgraphv1connect.SpecServiceUpdateSpecProcedure:      "spec.write",
	specgraphv1connect.SpecServiceListChangesProcedure:     "spec.read",
	specgraphv1connect.SpecServiceCompareVersionsProcedure: "spec.read",
	// DecisionService
	specgraphv1connect.DecisionServiceGetDecisionProcedure:    "decision.read",
	specgraphv1connect.DecisionServiceListDecisionsProcedure:  "decision.read",
	specgraphv1connect.DecisionServiceCreateDecisionProcedure: "decision.write",
	specgraphv1connect.DecisionServiceUpdateDecisionProcedure: "decision.write",
	// GraphService
	specgraphv1connect.GraphServiceGetFullGraphProcedure:      "graph.read",
	specgraphv1connect.GraphServiceGetDependenciesProcedure:   "graph.read",
	specgraphv1connect.GraphServiceGetTransitiveDepsProcedure: "graph.read",
	specgraphv1connect.GraphServiceGetImpactProcedure:         "graph.read",
	specgraphv1connect.GraphServiceGetReadyProcedure:          "graph.read",
	specgraphv1connect.GraphServiceGetCriticalPathProcedure:   "graph.read",
	specgraphv1connect.GraphServiceListEdgesProcedure:         "graph.read",
	specgraphv1connect.GraphServiceAddEdgeProcedure:           "graph.write",
	specgraphv1connect.GraphServiceRemoveEdgeProcedure:        "graph.delete",
	// ClaimService
	specgraphv1connect.ClaimServiceClaimSpecProcedure:   "claim.write",
	specgraphv1connect.ClaimServiceHeartbeatProcedure:   "claim.write",
	specgraphv1connect.ClaimServiceUnclaimSpecProcedure: "claim.write",
	// ConstitutionService
	specgraphv1connect.ConstitutionServiceGetConstitutionProcedure:          "constitution.read",
	specgraphv1connect.ConstitutionServiceUpdateConstitutionProcedure:       "constitution.write",
	specgraphv1connect.ConstitutionServiceEmitToolFilesProcedure:            "constitution.read",
	specgraphv1connect.ConstitutionServiceRefreshConstitutionLayerProcedure: "constitution.write",
	// AuthoringService
	specgraphv1connect.AuthoringServiceGetPromptsProcedure:         "authoring.read",
	specgraphv1connect.AuthoringServiceSparkProcedure:              "authoring.write",
	specgraphv1connect.AuthoringServiceShapeProcedure:              "authoring.write",
	specgraphv1connect.AuthoringServiceSpecifyProcedure:            "authoring.write",
	specgraphv1connect.AuthoringServiceDecomposeProcedure:          "authoring.write",
	specgraphv1connect.AuthoringServiceApproveProcedure:            "authoring.write",
	specgraphv1connect.AuthoringServiceAmendProcedure:              "authoring.write",
	specgraphv1connect.AuthoringServiceSupersedeProcedure:          "authoring.write",
	specgraphv1connect.AuthoringServiceRecordConversationProcedure: "authoring.write",
	specgraphv1connect.AuthoringServiceListConversationsProcedure:  "authoring.read",
	// ExecutionService
	specgraphv1connect.ExecutionServiceGenerateBundleProcedure:     "execution.read",
	specgraphv1connect.ExecutionServiceGetPrimeProcedure:           "execution.read",
	specgraphv1connect.ExecutionServiceGetExecutionEventsProcedure: "execution.read",
	specgraphv1connect.ExecutionServiceReportProgressProcedure:     "execution.write",
	specgraphv1connect.ExecutionServiceReportBlockerProcedure:      "execution.write",
	specgraphv1connect.ExecutionServiceReportCompletionProcedure:   "execution.write",
	// LifecycleService
	specgraphv1connect.LifecycleServiceCheckDriftProcedure:          "lifecycle.read",
	specgraphv1connect.LifecycleServiceLintProcedure:                "lifecycle.read",
	specgraphv1connect.LifecycleServiceAcknowledgeDriftProcedure:    "lifecycle.write",
	specgraphv1connect.LifecycleServiceTransitionAmendProcedure:     "lifecycle.write",
	specgraphv1connect.LifecycleServiceTransitionSupersedeProcedure: "lifecycle.write",
	specgraphv1connect.LifecycleServiceTransitionAbandonProcedure:   "lifecycle.write",
	// SyncService
	specgraphv1connect.SyncServiceGetSyncStatusProcedure: "sync.read",
	specgraphv1connect.SyncServiceSyncBeadsProcedure:     "sync.write",
	specgraphv1connect.SyncServiceSyncGitHubProcedure:    "sync.write",
	// AnalyticalPassService
	specgraphv1connect.AnalyticalPassServiceRunAnalyticalPassProcedure:   "analytical_pass.write",
	specgraphv1connect.AnalyticalPassServiceStoreFindingsProcedure:       "analytical_pass.write",
	specgraphv1connect.AnalyticalPassServiceListFindingsProcedure:        "analytical_pass.read",
	specgraphv1connect.AnalyticalPassServiceListProjectFindingsProcedure: "analytical_pass.read",
	// ExportService
	specgraphv1connect.ExportServiceExportProjectProcedure: "export.read",
	specgraphv1connect.ExportServiceImportProjectProcedure: "export.write",
	specgraphv1connect.ExportServiceVerifyExportProcedure:  "export.read",
	// SliceService
	specgraphv1connect.SliceServiceListSlicesProcedure:    "slice.read",
	specgraphv1connect.SliceServiceGetSliceProcedure:      "slice.read",
	specgraphv1connect.SliceServiceClaimSliceProcedure:    "slice.write",
	specgraphv1connect.SliceServiceCompleteSliceProcedure: "slice.write",
}

// ActionForProcedure returns the stable action name for an RPC procedure.
func ActionForProcedure(procedure string) (string, bool) {
	a, ok := procedureActions[procedure]
	return a, ok
}

// ActionNames returns the distinct action names, sorted. Passed to
// NewCedarEngine to build the action-group entity graph.
func ActionNames() []string {
	seen := make(map[string]bool, len(procedureActions))
	for _, a := range procedureActions {
		seen[a] = true
	}
	names := make([]string, 0, len(seen))
	for a := range seen {
		names = append(names, a)
	}
	sort.Strings(names)
	return names
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run 'TestActionForProcedure|TestActionNames' -v`

Expected: PASS.

- [ ] **Step 5: Sanity — every action name is buildable**

The action names feed `buildActionEntities`. Confirm they all carry a known verb:

Run: `cd internal/auth && go test -run TestActionNames_AllParseToKnownVerb -v`

Expected: PASS (every value ends in `.read`/`.write`/`.delete`).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/actions.go internal/auth/actions_test.go
git commit -s -m "feat(auth): add procedure→action map (replaces rpcPermissions)"
```

---

## Task 12: Relocate `IsExempt`/`exemptProcedures` to `exempt.go`

`permissions.go` is deleted in Phase C, but the post-Authn interceptor calls `IsExempt`. Move those two symbols to their own file FIRST so the interceptor keeps resolving after `permissions.go` is gone. This task changes no behavior — it relocates code.

**Files:**

- Create: `internal/auth/exempt.go`
- Create: `internal/auth/exempt_test.go`
- Modify: `internal/auth/permissions.go` (remove the relocated symbols)
- Modify: `internal/auth/permissions_test.go` (remove the relocated tests)

**Covers:** Happy (Health is exempt) + Boundary (a normal procedure is not exempt).

- [ ] **Step 1: Write `exempt.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// exemptProcedures lists procedures that bypass authentication AND
// authorization entirely (health checks). Consulted by the interceptor
// before any Resolver/Authorizer work. Relocated from permissions.go (which
// the Cedar plan deletes); IsExempt is the interceptor's only dependency on
// this file.
var exemptProcedures = map[string]bool{
	specgraphv1connect.ServerServiceHealthProcedure: true,
}

// IsExempt reports whether a procedure bypasses auth.
func IsExempt(procedure string) bool {
	return exemptProcedures[procedure]
}
```

- [ ] **Step 2: Write `exempt_test.go`**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

func TestIsExempt_Health(t *testing.T) {
	require.True(t, auth.IsExempt(specgraphv1connect.ServerServiceHealthProcedure))
}

func TestIsExempt_NormalProcedureNotExempt(t *testing.T) {
	require.False(t, auth.IsExempt(specgraphv1connect.SpecServiceGetSpecProcedure))
}
```

- [ ] **Step 3: Remove the relocated symbols from `permissions.go`**

Delete the `exemptProcedures` var and the `IsExempt` func from `internal/auth/permissions.go`. After this edit `permissions.go` contains only `rpcPermissions` and `RPCPermission`. `rpcPermissions` is still read by `StaticTableAuthorizer.Authorize` (which indexes the map directly), so it stays live until Task 17. `RPCPermission` (exported) may already be unreferenced post-Authn — that's fine: exported symbols don't trip the `unused` linter, and it's deleted with the file in Task 17. The `specgraphv1connect` import is still needed by `rpcPermissions`' keys — leave it.

- [ ] **Step 4: Move the exempt tests out of `permissions_test.go`**

Delete any `TestIsExempt*` / `exemptProcedures` tests from `permissions_test.go` (they now live in `exempt_test.go`). Leave the `rpcPermissions`/`RPCPermission` tests in place — they're deleted with the file in Task 17.

- [ ] **Step 5: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS — `IsExempt` resolves from `exempt.go`; the interceptor (unchanged) still compiles.

- [ ] **Step 6: Whole-project build + test**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/auth/exempt.go internal/auth/exempt_test.go internal/auth/permissions.go internal/auth/permissions_test.go
git commit -s -m "refactor(auth): relocate IsExempt/exemptProcedures to exempt.go"
```

---

## Task 13: Implement `CedarAuthorizer` (implements `auth.Authorizer`)

The drop-in replacement for `StaticTableAuthorizer`. Same interface, returns the same `auth.Decision`. This is what makes the interceptor diff zero.

**Files:**

- Create: `internal/auth/cedar_authorizer.go`
- Create: `internal/auth/cedar_authorizer_test.go`

**Covers:** Happy (allow returns `auth.Decision{Allowed:true}`) + Boundary (unconfigured procedure → error; deny → `Allowed:false` with reason) + Invariant (engine error → authorizer error).

- [ ] **Step 1: Write the failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

// fakeEngine lets the authorizer tests control decisions without real cedar.
type fakeEngine struct {
	dec auth.PolicyDecision
	err error
	got auth.EvalRequest
}

func (f *fakeEngine) Evaluate(_ context.Context, req auth.EvalRequest) (auth.PolicyDecision, error) {
	f.got = req
	return f.dec, f.err
}
func (f *fakeEngine) Reload(context.Context) error { return nil }

func TestCedarAuthorizer_Allow(t *testing.T) {
	eng := &fakeEngine{dec: auth.PolicyDecision{Allowed: true, MatchedPolicies: []string{"embedded:base.cedar#policy0"}}}
	a := auth.NewCedarAuthorizer(eng)
	id := &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader}

	d, err := a.Authorize(context.Background(), id, specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.NoError(t, err)
	require.True(t, d.Allowed)
	require.Contains(t, d.Reason, "cedar-allow")
	// The engine received the mapped action name, not the RPC method name.
	require.Equal(t, "spec.read", eng.got.Action)
}

func TestCedarAuthorizer_Deny(t *testing.T) {
	eng := &fakeEngine{dec: auth.PolicyDecision{Allowed: false}}
	a := auth.NewCedarAuthorizer(eng)
	id := &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader}

	d, err := a.Authorize(context.Background(), id, specgraphv1connect.SpecServiceCreateSpecProcedure, nil)
	require.NoError(t, err)
	require.False(t, d.Allowed)
	require.Contains(t, d.Reason, "cedar-deny")
}

func TestCedarAuthorizer_UnconfiguredProcedureIsError(t *testing.T) {
	a := auth.NewCedarAuthorizer(&fakeEngine{})
	_, err := a.Authorize(context.Background(), &auth.Identity{}, "/no.such/Procedure", nil)
	require.Error(t, err)
}

func TestCedarAuthorizer_EngineErrorPropagates(t *testing.T) {
	sentinel := errors.New("engine down")
	a := auth.NewCedarAuthorizer(&fakeEngine{err: sentinel})
	_, err := a.Authorize(context.Background(), &auth.Identity{EffectiveRole: auth.RoleReader},
		specgraphv1connect.SpecServiceGetSpecProcedure, nil)
	require.ErrorIs(t, err, sentinel)
}

// Compile-time assertion that CedarAuthorizer satisfies Authorizer.
var _ auth.Authorizer = (*auth.CedarAuthorizer)(nil)
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestCedarAuthorizer -v`

Expected: FAIL ("undefined: auth.NewCedarAuthorizer").

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"strings"
)

// CedarAuthorizer implements Authorizer by delegating to a PolicyEngine.
// It replaces StaticTableAuthorizer; because both satisfy Authorizer, the
// interceptor and serve.go wiring change only in which constructor is called
// — the interceptor's Authorize call site is byte-identical.
type CedarAuthorizer struct {
	engine PolicyEngine
}

// NewCedarAuthorizer wraps a PolicyEngine as an Authorizer.
func NewCedarAuthorizer(engine PolicyEngine) *CedarAuthorizer {
	return &CedarAuthorizer{engine: engine}
}

// Authorize maps the RPC procedure to a stable action name, builds an
// EvalRequest, and asks the engine. An unconfigured procedure is an error
// (mirrors StaticTableAuthorizer; the interceptor maps it to CodeInternal),
// which is deliberately distinct from a clean Deny.
//
// req (the unmarshaled request body) is accepted for the Authorizer
// interface and future ownership rules; the migration's role-only policies
// do not inspect it.
func (a *CedarAuthorizer) Authorize(ctx context.Context, id *Identity, procedure string, _ any) (Decision, error) {
	action, ok := ActionForProcedure(procedure)
	if !ok {
		return Decision{}, fmt.Errorf("cedar: unconfigured procedure %q", procedure)
	}
	domain := actionDomain(action)
	dec, err := a.engine.Evaluate(ctx, EvalRequest{
		Identity: id,
		Action:   action,
		Resource: ResourceRef{Type: domain, ID: domain},
	})
	if err != nil {
		return Decision{}, fmt.Errorf("cedar: evaluate %s: %w", action, err)
	}
	if dec.Allowed {
		return Decision{
			Allowed: true,
			Reason:  "cedar-allow:" + strings.Join(dec.MatchedPolicies, ","),
		}, nil
	}
	return Decision{
		Allowed: false,
		Reason:  "cedar-deny:" + action,
	}, nil
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestCedarAuthorizer -v`

Expected: PASS, including the `var _ auth.Authorizer = (*auth.CedarAuthorizer)(nil)` compile-time assertion.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/cedar_authorizer.go internal/auth/cedar_authorizer_test.go
git commit -s -m "feat(auth): add CedarAuthorizer implementing the Authorizer seam"
```

---

## Task 14: Add `KnownRolesFrom`

`serve.go` derives `KnownRoles` (for JIT role validation) from `LoadRolePerms` today. Cedar deletes `LoadRolePerms`, so the role-name set needs a new, perm-free home.

**Files:**

- Create: `internal/auth/known_roles.go`
- Create: `internal/auth/known_roles_test.go`

**Covers:** Happy (built-ins ∪ custom names) + Boundary (nil/empty custom → just built-ins).

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

func TestKnownRolesFrom_BuiltinsPlusCustom(t *testing.T) {
	known := auth.KnownRolesFrom([]string{"auditor", "releaser"})
	require.True(t, known[auth.RoleAdmin])
	require.True(t, known[auth.RoleWriter])
	require.True(t, known[auth.RoleReader])
	require.True(t, known[auth.Role("auditor")])
	require.True(t, known[auth.Role("releaser")])
	require.False(t, known[auth.Role("nope")])
}

func TestKnownRolesFrom_NilCustom(t *testing.T) {
	known := auth.KnownRolesFrom(nil)
	require.Len(t, known, 3)
	require.True(t, known[auth.RoleAdmin])
}
```

- [ ] **Step 2: Verify failure**

Run: `cd internal/auth && go test -run TestKnownRolesFrom -v`

Expected: FAIL ("undefined: auth.KnownRolesFrom").

- [ ] **Step 3: Write the impl**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// KnownRolesFrom returns the set of role names valid for assignment:
// the built-in roles plus any operator-defined custom role NAMES. Under
// Cedar, custom roles carry no permission list — their authorization is
// expressed as Cedar policies (e.g. in a DirectoryPolicySource). This set
// exists only so the resolver can reject JIT/claims-mapping references to
// roles that don't exist. A known role with no matching policy authorizes
// nothing (default-deny), which is the intended behavior.
func KnownRolesFrom(custom []string) map[Role]bool {
	known := map[Role]bool{
		RoleAdmin:  true,
		RoleWriter: true,
		RoleReader: true,
	}
	for _, name := range custom {
		if name != "" {
			known[Role(name)] = true
		}
	}
	return known
}
```

- [ ] **Step 4: Run the tests**

Run: `cd internal/auth && go build ./... && go test -run TestKnownRolesFrom -v`

Expected: PASS.

- [ ] **Step 5: Whole-project build + test**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./... && go test ./...`

Expected: PASS. All Phase-A pieces exist; `StaticTableAuthorizer` still serves auth (nothing switched yet).

- [ ] **Step 6: Commit**

```bash
git add internal/auth/known_roles.go internal/auth/known_roles_test.go
git commit -s -m "feat(auth): add KnownRolesFrom (built-ins ∪ custom role names)"
```

---

## Task 15: Reshape config — `Roles []string`, drop `RoleConfig`, add `Policies`

No vestigial cruft: the permission lists are gone entirely, not left as an ignored field. `cfg.Auth.Roles` becomes a list of role names; `auth.policies.extra_dirs` is added.

**Files:**

- Modify: `internal/config/global.go`
- Modify: `internal/config/global_test.go` (or the relevant config test file)

**Covers:** Happy (parse names + extra_dirs) + Boundary (old map-shaped `roles:` hard-fails at parse — deliberate, not silent).

- [ ] **Step 1: Write the failing test**

Add to the config test file (the package that loads `AuthConfig`):

```go
func TestAuthConfig_RolesAreNames(t *testing.T) {
	const y = `
auth:
  roles: [auditor, releaser]
  policies:
    extra_dirs: ["/etc/specgraph/policies"]
`
	var c config.GlobalConfig
	require.NoError(t, yaml.Unmarshal([]byte(y), &c))
	require.Equal(t, []string{"auditor", "releaser"}, c.Auth.Roles)
	require.Equal(t, []string{"/etc/specgraph/policies"}, c.Auth.Policies.ExtraDirs)
}

func TestAuthConfig_LegacyMapRolesRejected(t *testing.T) {
	// The old map-with-permissions shape must NOT silently parse — Cedar
	// removed permission lists, and a silent drop would strip authorization
	// without warning. A type mismatch (map vs sequence) is the honest fail.
	const y = `
auth:
  roles:
    auditor:
      permissions: ["spec:read"]
`
	var c config.GlobalConfig
	require.Error(t, yaml.Unmarshal([]byte(y), &c))
}
```

> Match the test's package, import path for `config`, the YAML library actually used (`gopkg.in/yaml.v3` or sigs.k8s.io/yaml — check the existing config tests), and the real top-level config type name (shown as `GlobalConfig` here — use whatever `global.go` declares). If config loading goes through a loader function rather than raw `yaml.Unmarshal`, call that loader instead.

- [ ] **Step 2: Verify failure**

Run: `cd internal/config && go test -run 'TestAuthConfig_RolesAreNames|TestAuthConfig_LegacyMapRolesRejected' -v`

Expected: FAIL — `c.Auth.Roles` is still `map[string]RoleConfig`; `c.Auth.Policies` doesn't exist.

- [ ] **Step 3: Reshape the config types**

In `internal/config/global.go`:

1. Change the `Roles` field in `AuthConfig` from:
   ```go
   Roles map[string]RoleConfig `yaml:"roles"`
   ```
   to:
   ```go
   Roles    []string     `yaml:"roles"`
   Policies PolicyConfig `yaml:"policies"`
   ```

2. Delete the `RoleConfig` type entirely:
   ```go
   // DELETE:
   // RoleConfig defines a custom role with explicit permissions.
   // type RoleConfig struct {
   // 	Permissions []string `yaml:"permissions"`
   // }
   ```

3. Add the `PolicyConfig` type near the other auth config types:
   ```go
   // PolicyConfig configures the Cedar authorization engine's policy
   // sources. Built-in policies are always loaded; ExtraDirs adds operator
   // policy directories (each *.cedar file becomes a DirectoryPolicySource).
   type PolicyConfig struct {
   	ExtraDirs []string `yaml:"extra_dirs"`
   }
   ```

- [ ] **Step 4: Fix any in-package references to `RoleConfig`**

Run: `grep -rn 'RoleConfig\|\.Permissions' internal/config/` and remove/adapt any validation or defaulting code that referenced the deleted type or the permissions field. (The cross-package consumer `serve.go` is fixed in Task 16; in-package config tests that built `RoleConfig` literals must be updated to the `[]string` shape now.)

- [ ] **Step 5: Run the tests**

Run: `cd internal/config && go build ./... && go test ./...`

Expected: PASS.

- [ ] **Step 6: Whole-project build**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./...`

Expected: **FAIL** — `serve.go` still references `cfg.Auth.Roles` as a map and `rc.Permissions`. That's expected; Task 16 fixes serve.go. Do NOT commit a broken whole-project build: this task and Task 16 are a coupled pair. If executing inline, proceed directly to Task 16 before the next push. If executing via subagents, run Tasks 15 and 16 in one batch.

- [ ] **Step 7: Commit (package-local)**

```bash
git add internal/config/global.go internal/config/global_test.go
git commit -s -m "feat(config): Roles->[]string, drop RoleConfig, add auth.policies.extra_dirs"
```

---

## Task 16: Switch `serve.go` to Cedar (the zero-diff-interceptor payoff)

Replace the `LoadRolePerms` + `NewStaticTableAuthorizer` block with policy-source + engine + `CedarAuthorizer` construction, and derive `KnownRoles` via `KnownRolesFrom`. The interceptor construction line is unchanged.

**Files:**

- Modify: `cmd/specgraph/serve.go`

**Covers:** E2E (whole project builds and the server wires Cedar) + Invariant (interceptor call site unchanged).

- [ ] **Step 1: Replace the role-perms + authorizer block**

In `cmd/specgraph/serve.go`, find the post-Authn block (Authn plan Task 29):

```go
// Role→permissions snapshot (built-ins ∪ cfg.Auth.Roles). Shared by the
// authorizer and used to derive KnownRoles for JIT validation.
rolePerms := auth.LoadRolePerms(cfg.Auth.Roles)
knownRoles := make(map[auth.Role]bool, len(rolePerms))
for r := range rolePerms {
	knownRoles[r] = true
}
```

Replace it with:

```go
// KnownRoles for JIT validation (built-ins ∪ custom role names). Under
// Cedar, custom roles carry no permission list; their authorization is
// expressed as Cedar policies, not YAML.
knownRoles := auth.KnownRolesFrom(cfg.Auth.Roles)
```

Then find:

```go
// Authorizer (static table for now; Cedar plan swaps).
authorizer := auth.NewStaticTableAuthorizer(rolePerms)
```

Replace it with:

```go
// Authorizer: Cedar policy engine. Built-in policies are always loaded;
// operators add directories via auth.policies.extra_dirs.
policySources := []auth.PolicySource{auth.NewEmbeddedPolicySource()}
for _, dir := range cfg.Auth.Policies.ExtraDirs {
	policySources = append(policySources, auth.NewDirectoryPolicySource(dir))
}
engine, err := auth.NewCedarEngine(ctx, policySources, auth.ActionNames())
if err != nil {
	return fmt.Errorf("policy engine: %w", err)
}
authorizer := auth.NewCedarAuthorizer(engine)
```

> Import check: this block uses `fmt.Errorf`. Confirm `cmd/specgraph/serve.go` already imports `"fmt"` — the post-Authn serve.go does (its OIDC-verifier and auth-store construction use `fmt.Errorf`), so no import change is expected. If a `go build` after Step 1 reports `undefined: fmt`, add `"fmt"` to the import block. `err` is already a declared variable in this scope (the post-Authn wiring assigns `resolver, err := ...`), so `engine, err := ...` reuses it via `:=` with the new `engine` on the left — no shadowing concern.

- [ ] **Step 2: Confirm the interceptor line is unchanged**

The construction line remains exactly:

```go
interceptor := auth.NewAuthInterceptor(resolver, authorizer)
```

> If the Authn cleanup task (30b) has not yet renamed `NewAuthInterceptorV2` back to `NewAuthInterceptor`, this line reads `auth.NewAuthInterceptorV2(...)` instead — leave whichever name the merged Authn code uses. Cedar changes neither the name nor the arguments; `authorizer` is now a `*CedarAuthorizer` instead of a `*StaticTableAuthorizer`, and both satisfy `auth.Authorizer`. **This non-change is the design's headline result.**

- [ ] **Step 3: Verify whole-project compile + test**

Run: `cd /Volumes/Code/github.com/specgraph-identity-plans && go build ./... && go test ./...`

Expected: PASS. (`StaticTableAuthorizer`, `LoadRolePerms`, `DefaultRolePermissions`, `rpcPermissions` are now unreferenced by non-test code, but still present — deleted in Task 17. They are exported or mutually-referenced, so `go build`/`go test` stay green.)

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -s -m "feat(serve): swap StaticTableAuthorizer for Cedar (interceptor diff is zero)"
```

---

## Task 17: Delete `StaticTableAuthorizer` and the static `rpcPermissions` table

Phase C. With `serve.go` switched, the static authorizer and its table are dead. Delete them together — `StaticTableAuthorizer.Authorize` is `rpcPermissions`' only consumer, so removing the file orphans the table and would trip the `unused` linter if `permissions.go` lingered.

**Files:**

- Delete: `internal/auth/static_authorizer.go`
- Delete: `internal/auth/static_authorizer_test.go`
- Delete: `internal/auth/permissions.go`
- Delete: `internal/auth/permissions_test.go` (only the `rpcPermissions`/`RPCPermission` tests remain there after Task 12; the exempt tests already moved to `exempt_test.go`)

**Covers:** Invariant (no surviving reference to the deleted symbols).

- [ ] **Step 1: Pre-deletion survivor grep**

```bash
grep -rn 'StaticTableAuthorizer\|NewStaticTableAuthorizer\|LoadRolePerms\|DefaultRolePermissions\|hasPermissionInternal\|rpcPermissions\|RPCPermission' \
  --include='*.go' internal/ cmd/
```

Expected: matches ONLY inside the four files being deleted in this task. If anything else matches (e.g. a stray reference in `serve.go` or a test), fix that first — a surviving consumer of a to-be-deleted symbol is the exact bug class this plan guards against.

- [ ] **Step 2: Delete the files**

```bash
rm internal/auth/static_authorizer.go internal/auth/static_authorizer_test.go \
   internal/auth/permissions.go internal/auth/permissions_test.go
```

- [ ] **Step 3: Verify compile + tests**

Run: `cd internal/auth && go build ./... && go test ./...`

Expected: PASS. `IsExempt` resolves from `exempt.go`; authorization runs through `CedarAuthorizer`.

- [ ] **Step 4: Post-deletion grep (confirm clean)**

```bash
grep -rn 'StaticTableAuthorizer\|LoadRolePerms\|DefaultRolePermissions\|hasPermissionInternal\|rpcPermissions\|RPCPermission' \
  --include='*.go' internal/ cmd/
```

Expected: NO matches anywhere.

- [ ] **Step 5: Whole-project build + test + lint**

Run:

```bash
cd /Volumes/Code/github.com/specgraph-identity-plans
go build ./... && go test ./...
task lint
```

Expected: PASS. `task lint` here catches `unused` (staticcheck) — confirms no orphaned unexported symbols remain.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -s -m "refactor(auth): delete StaticTableAuthorizer and static rpcPermissions table"
```

---

## Task 18: Integration test — interceptor → CedarAuthorizer → engine

End-to-end through the real interceptor with the real embedded policies. Proves the zero-diff seam works in situ and the embedded `base.cedar` authorizes correctly.

**Files:**

- Create: `internal/auth/cedar_integration_test.go`

**Covers:** E2E (full authz path: reader allowed read, denied write; admin allowed delete; exempt bypass).

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

	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
)

// newRealCedarAuthorizer builds the production authorizer: embedded policies,
// real engine, real action map.
func newRealCedarAuthorizer(t *testing.T) auth.Authorizer {
	t.Helper()
	engine, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{auth.NewEmbeddedPolicySource()}, auth.ActionNames())
	require.NoError(t, err)
	return auth.NewCedarAuthorizer(engine)
}

func TestIntegration_CedarAuthorizer_EmbeddedPolicies(t *testing.T) {
	a := newRealCedarAuthorizer(t)
	ctx := context.Background()

	cases := []struct {
		role      auth.Role
		procedure string
		allowed   bool
	}{
		{auth.RoleReader, specgraphv1connect.SpecServiceGetSpecProcedure, true},
		{auth.RoleReader, specgraphv1connect.SpecServiceCreateSpecProcedure, false},
		{auth.RoleReader, specgraphv1connect.GraphServiceRemoveEdgeProcedure, false},
		{auth.RoleWriter, specgraphv1connect.SpecServiceCreateSpecProcedure, true},
		{auth.RoleWriter, specgraphv1connect.GraphServiceRemoveEdgeProcedure, false},
		{auth.RoleAdmin, specgraphv1connect.GraphServiceRemoveEdgeProcedure, true},
	}
	for _, c := range cases {
		id := &auth.Identity{UserID: "u1", EffectiveRole: c.role, Role: c.role, Subject: "apikey:k1"}
		d, err := a.Authorize(ctx, id, c.procedure, nil)
		require.NoErrorf(t, err, "role=%s proc=%s", c.role, c.procedure)
		require.Equalf(t, c.allowed, d.Allowed, "role=%s proc=%s reason=%s", c.role, c.procedure, d.Reason)
	}
}

func TestIntegration_HealthIsExempt(t *testing.T) {
	// Exempt procedures never reach the authorizer; assert the interceptor's
	// gate directly.
	require.True(t, auth.IsExempt(specgraphv1connect.ServerServiceHealthProcedure))
}
```

- [ ] **Step 2: Run**

Run: `cd internal/auth && go test -tags integration -run 'TestIntegration_CedarAuthorizer|TestIntegration_HealthIsExempt' -v`

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/auth/cedar_integration_test.go
git commit -s -m "test(auth): integration test for Cedar authorizer with embedded policies"
```

---

## Task 19: Integration test — discrete policy layering (the design payoff)

Prove that a discrete, owner-based policy from a `DirectoryPolicySource` layers on top of the role-only base — granting access the base would deny — without any code change. This is the property stories #3/#4 depend on, and it exercises `DirectoryPolicySource` end-to-end.

**Files:**

- Modify: `internal/auth/cedar_integration_test.go`

**Covers:** E2E (composed sources; discrete permit broadens beyond the base) + Invariant (a layered permit is additive — base denials for others are unaffected).

- [ ] **Step 1: Write the test**

Append to `internal/auth/cedar_integration_test.go`:

```go
import (
	"os"
	"path/filepath"
)

// TestIntegration_DiscretePolicyLayering shows a directory-sourced ownership
// policy granting a reader access to a write action on their OWN resource —
// something the role-only base policy denies — while a non-owner reader is
// still denied. New behavior arrives as a new policy file, no code change.
func TestIntegration_DiscretePolicyLayering(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// Owner may rotate their own resource regardless of role.
	ownerPolicy := `permit (
		principal,
		action in SpecGraph::Action::"write",
		resource
	) when {
		resource has owner_user_id && principal has id && resource.owner_user_id == principal.id
	};`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ownership.cedar"), []byte(ownerPolicy), 0o600))

	engine, err := auth.NewCedarEngine(ctx,
		[]auth.PolicySource{auth.NewEmbeddedPolicySource(), auth.NewDirectoryPolicySource(dir)},
		auth.ActionNames())
	require.NoError(t, err)

	// Reader who owns the resource: base denies write, ownership permit allows.
	ownerDec, err := engine.Evaluate(ctx, auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleReader},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec", ID: "s1", Attributes: map[string]string{"owner_user_id": "u1"}},
	})
	require.NoError(t, err)
	require.True(t, ownerDec.Allowed, "owner reader should be allowed by the layered ownership policy")

	// Reader who does NOT own the resource: neither base nor ownership matches.
	otherDec, err := engine.Evaluate(ctx, auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u2", EffectiveRole: auth.RoleReader},
		Action:   "spec.write",
		Resource: auth.ResourceRef{Type: "spec", ID: "s1", Attributes: map[string]string{"owner_user_id": "u1"}},
	})
	require.NoError(t, err)
	require.False(t, otherDec.Allowed, "non-owner reader stays denied — layering is additive, not blanket")
}
```

- [ ] **Step 2: Run**

Run: `cd internal/auth && go test -tags integration -run TestIntegration_DiscretePolicyLayering -v`

Expected: PASS — the owner is allowed via the directory policy; the non-owner stays denied.

- [ ] **Step 3: Run the full auth suite (unit + integration)**

Run: `cd internal/auth && go test ./... && go test -tags integration ./...`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/auth/cedar_integration_test.go
git commit -s -m "test(auth): integration test for discrete ownership policy layering"
```

---

## Self-Review

**1. Spec coverage** (against `docs/plans/2026-05-26-identity-policy-engine-design.md`):

- [x] Cedar embedded, one binary, no sidecar — `engine.go` (`cedarEngine`), Tasks 1, 6, 9.
- [x] Wrapped behind SpecGraph-owned `PolicyEngine`; only the wrapper imports cedar-go — Task 5 (interface), Task 6/9 (impl); `engine.go` is the sole cedar-go importer (asserted by the File Structure and the "do not touch" list).
- [x] `EvalRequest` / `PolicyDecision` are SpecGraph types, not cedar types — Task 5.
- [x] `PolicySource` interface; `EmbeddedPolicySource` + `DirectoryPolicySource` ship; composable, ordered — Tasks 2, 3, 4, 6 (`loadPolicySet` merges in order).
- [x] Built-in source MUST load or refuse start — Task 6 (`Reload` zero-policy error; `NewCedarEngine` propagates).
- [x] Filesystem sources required-by-default — Task 4 (missing dir → error; empty dir → OK).
- [x] No hot-reload (restart applies changes); `Reload` present for the future story, lock-free hot path — Tasks 5, 6 (`atomic.Pointer`).
- [x] Entity model: principal `SpecGraph::User` with `role`/`id`/`email`; resource per-domain; actions namespaced + decoupled from method names — Tasks 7, 8, 9, 11.
- [x] Actions decoupled from RPC method names via `(service,method)→action` map — Task 11 (`procedureActions`, tested by `TestActionForProcedure*` and `TestActionNames_DecoupledFromMethodNames`).
- [x] Migration of `rpcPermissions` to policies + action declarations, in one landing — base policies (Task 3) + action map (Task 11); semantics-equivalence argued in Task 3 and pinned by Task 9 matrix + Task 18.
- [x] Static table deleted: `StaticTableAuthorizer`, `permissions.go`, `hasPermissionInternal`, `LoadRolePerms`, `DefaultRolePermissions` — Task 17 (with Task 12 preserving `IsExempt`).
- [x] Decision logging at `slog.Debug` until audit story #1 — Task 10.
- [x] `CedarAuthorizer` implements existing `Authorizer`; interceptor diff zero — Task 13 + Task 16 (Step 2 asserts the unchanged call site).
- [x] Self-service / ownership rules expressible without handler code — demonstrated by Task 19 (discrete ownership permit layered via `DirectoryPolicySource`).
- [x] Custom-role decision: names-only, no vestigial permission lists — Task 14 (`KnownRolesFrom`) + Task 15 (config reshape, `RoleConfig` deleted).

**2. Placeholder scan:** No `TBD`/`TODO`/"similar to above"/"add error handling" left. The two temporary stubs in Task 6 (`buildActionEntities`, `Evaluate`) are explicitly labeled temporary and are replaced with full real code in Tasks 7 and 9 respectively, each with its own failing-test-first cycle. The Task 1 smoke test is intentionally throwaway and deleted in its own Step 4. Every code step shows complete code; every run step shows the exact command and expected result.

**3. Type consistency:** `PolicyEngine`, `cedarEngine`, `NewCedarEngine`, `EvalRequest`, `ResourceRef`, `PolicyDecision`, `PolicyDocument`, `PolicySource`, `EmbeddedPolicySource`, `DirectoryPolicySource`, `CedarAuthorizer`, `NewCedarAuthorizer`, `ActionForProcedure`, `ActionNames`, `actionVerb`, `actionDomain`, `buildActionEntities`, `principalEntity`, `resourceEntity`, `KnownRolesFrom`, `IsExempt` are referenced identically across Tasks 1–19. `CedarAuthorizer.Authorize` returns the existing `auth.Decision` (compile-time assertion in Task 13); the engine returns `PolicyDecision` (distinct, no collision). Entity-type constants (`entityTypeUser`/`entityTypeResource`/`entityTypeAction`) defined in Task 8 and back-applied to Task 7's literals.

**4. Build-discipline check:** Every task ends with `go build ./... && go test ./...` green at the package level; Tasks 6, 9, 14, 16, 17 additionally run the whole-project build+test (and Task 17 runs `task lint`). The one deliberate exception is Task 15 → 16: Task 15 leaves the whole project non-building (serve.go still references the old config shape) and is explicitly flagged as a coupled pair with Task 16 — they execute in one batch / before the next push. Phase A (1–14) is purely additive: `StaticTableAuthorizer` continues to serve authorization the entire time. Phase B (15–16) switches `serve.go` with a byte-identical interceptor line. Phase C (17) deletes the now-dead static authorizer and table together, after a pre-deletion survivor grep and a post-deletion clean grep plus `task lint` to catch any orphaned unexported symbol. Integration tests (18–19) are additive `//go:build integration` files.

**5. Symbol-lifetime sweep:** Completed in the dedicated section above — every new identifier greppe�d against the post-Authn surface (no collisions; `PolicyDecision` and `PolicyDocument` chosen specifically to avoid `Decision`/`cedar.Policy` clashes), and every deleted symbol's consumers verified to be either switched (`serve.go`) or co-deleted (the four Task-17 files). The single survivor that the handoff's literal "delete permissions.go" would have broken — `IsExempt`/`exemptProcedures`, called by the interceptor — is relocated to `exempt.go` in Task 12 before `permissions.go` is deleted in Task 17.

---

## Execution

Two options:

1. **Subagent-Driven (recommended)** — fresh subagent per task. Tasks 1–14 are short, well-scoped TDD cycles ideal for a fleet. **Tasks 15–16 MUST run as one batch** (Task 15 alone leaves the whole project non-building, by design). Task 17 (deletion) and Tasks 18–19 (integration) are natural standalone units with checkpoint review.
2. **Inline Execution** — practical especially for the 15–16 config/serve coupling and the 17 deletion, which touch multiple files at once.

After execution, the Bootstrap & UX plan is the last of the four; it is unaffected at the design level by this plan (the entity model projects cleanly into Cedar). Stories #3 (project-scoped RBAC) and #4 (resource ownership) build on this foundation as separate epics — each adds resource attributes and discrete/group policies, with no new authorization code paths in handlers, exactly as Task 19 demonstrates.
