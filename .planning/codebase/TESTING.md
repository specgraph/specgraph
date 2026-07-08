# Testing Patterns

**Analysis Date:** 2026-07-08

## Test Framework

**Runner:**
- Standard Go `testing` package for unit tests (majority of ~304 `*_test.go` files)
- Ginkgo/Gomega (BDD-style) for e2e suites: 168 files reference `ginkgo`/`gomega`, concentrated in `e2e/`

**Assertion Library:**
- `testify` (`assert`/`require`) used widely alongside plain `testing` in unit tests, e.g. `cmd/specgraph/constitution_test.go`
- Plain `t.Fatalf`/`t.Errorf` also common for simple checks, e.g. `internal/storage/change_event_test.go`
- Gomega matchers (`Expect(...).To(...)`) exclusively in `//go:build e2e` / `//go:build e2e_cli` files

**Run Commands:**
```bash
task test              # go test ./... — excludes integration and e2e build tags
task test:short        # short tests only
task pr-prep           # full pipeline: check → test:integration → test:e2e (requires Docker)
go test ./internal/storage/          # unit tests, no ellipsis (avoids pulling in postgres/ integration pkg)
go test -tags integration ./...      # Postgres integration suites (testcontainers)
go test -tags e2e ./e2e/api/...      # API e2e suite (Ginkgo)
go test -tags e2e_cli ./e2e/cli/...  # CLI e2e suite
```

## Test File Organization

**Location:**
- Unit tests co-located with source: `internal/storage/claim_domain.go` sits beside its test in the same package
- Integration tests co-located but gated with `//go:build integration`: `internal/storage/postgres/lifecycle_test.go`, `internal/storage/postgres/migration_007_test.go`
- E2E tests fully separated under `e2e/` by domain: `e2e/api/`, `e2e/cli/`, `e2e/agent/`, `e2e/docker/`, with shared helpers in `e2e/testutil/`

**Naming:**
- `Test<Subject>_<Scenario>` for Go-native tests, e.g. `TestConstitutionLayerStringToProto`, `TestAuthoringCmds_RequireSlug`
- Ginkgo `Describe("<feature>", func() { It("<behavior>", func() {...}) })` blocks for e2e

**Structure:**
```
internal/storage/
├── claim_domain.go
├── claim.go
└── change_event_test.go        # in-package unit test (package storage_test or storage)

internal/storage/postgres/
├── lifecycle_test.go            # //go:build integration
├── auth_helpers_test.go
├── migration_007_test.go
└── postgrestest/pool.go         # shared testcontainers pool helper

e2e/
├── api/                          # Ginkgo suite, //go:build e2e
│   ├── pipeline_test.go
│   └── skills_test.go
├── cli/                          # //go:build e2e_cli
│   └── cli_suite_test.go         # TestCLI entrypoint, RunSpecs
├── agent/
├── docker/                       # requires Docker-in-Docker, skipped in CI
└── testutil/                     # ServerInfo, CLIRunner, containers.go
```

## Test Structure

**Go-native table-driven pattern** (`cmd/specgraph/constitution_test.go:22`):
```go
func TestConstitutionLayerStringToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ConstitutionLayer
	}{
		{"user", specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER},
		{"", specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := constitutionLayerStringToProto(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

**Subtest grouping via `t.Run` for related scenarios** (`internal/storage/postgres/lifecycle_test.go:18`):
```go
t.Run("AmendSpec_HappyPath", func(t *testing.T) { ... })
t.Run("AmendSpec_RequiresReEntryStage", func(t *testing.T) { ... })
t.Run("AmendSpec_NotAmendable_Spark", func(t *testing.T) { ... })
```

**Ginkgo BDD structure** (`e2e/api/pipeline_test.go:20`):
```go
var _ = Describe("Full pipeline", Ordered, func() {
	var (
		specClient      specgraphv1connect.SpecServiceClient
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		httpClient := projectClientFor("pipeline-project")
		specClient = specgraphv1connect.NewSpecServiceClient(httpClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("sets project constitution", func() { ... })
})
```

**Suite entrypoint pattern** (`e2e/cli/cli_suite_test.go`):
```go
func TestCLI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CLI E2E Suite")
}
```

**Patterns:**
- `Ordered` Ginkgo containers used when steps in a pipeline depend on prior step state (e.g. full spec lifecycle through spark→shape→specify→decompose→claim→execute)
- Dedicated per-suite project/namespace to avoid cross-suite pollution (e.g. `pipeline-project` vs the shared e2e-test project)
- `t.Cleanup()` for resource teardown (pools, containers) rather than manual defer chains in table-driven tests

## Mocking

**Framework:** No mocking library (no gomock/mockery). Hand-written fakes implementing storage interfaces directly.

**Patterns** (`cmd/specgraph/logout_test.go:219`):
```go
// logoutFakeWA is a minimal storage.WebAuthStore for logout-revocation tests.
type logoutFakeWA struct {
	onRevoke func()
}

var _ storage.WebAuthStore = (*logoutFakeWA)(nil)

func (f *logoutFakeWA) RevokeSession(_ context.Context, tokenHash []byte) error {
	if f.onRevoke != nil {
		f.onRevoke()
	}
	return nil
}
// ... remaining interface methods stubbed with minimal behavior
```

**What to Mock:**
- Storage interfaces (`storage.WebAuthStore`, etc.) when testing handler/CLI logic in isolation
- Compile-time interface assertion (`var _ storage.WebAuthStore = (*logoutFakeWA)(nil)`) required alongside every fake to catch drift when the interface changes

**What NOT to Mock:**
- Postgres storage itself — integration tests (`//go:build integration`) use real Postgres via testcontainers (`pgvector/pgvector:pg18`) rather than mocking the DB layer
- ConnectRPC handlers in e2e tests run against a real server instance (`serverInfo.BaseURL`), not mocked clients

**Sentinel errors as mock/fake behavior contract:** Fakes and mock backends MUST return the same sentinel errors as real implementations (e.g. `storage.ErrSpecNotFound`) — not `fmt.Errorf()` — because handler code uses `errors.Is()` checks against them.

## Fixtures and Factories

**Test Data:**
- Inline struct literals per test rather than shared factory functions, e.g. `&storage.ChangeEvent{Slug: "spec-a", Version: 1}` built directly in test bodies
- Shared containerized Postgres pool via `SharedPool(t, ctx)` in `internal/storage/postgres/postgrestest/pool.go` — starts a testcontainer once per test binary using `sync.Once`, returns a fresh `pgxpool.Pool` per caller, and `t.Cleanup`s the pool

**Location:**
- `internal/storage/postgres/postgrestest/` — shared testcontainers bootstrap for storage/auth/server/bootstrap integration suites (exists because per-package external test packages can't reach one another's `TestMain`)
- `e2e/testutil/` — `ServerInfo`, `CLIRunner`, `containers.go` for e2e harness setup

## Coverage

**Requirements:** No explicit coverage threshold enforced in `task check`; coverage is not gated in CI beyond passing tests

**View Coverage:**
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Test Types

**Unit Tests:**
- Default `go test ./...` scope; run via `task test` / `task check`
- Excludes files behind `//go:build integration` and `//go:build e2e` tags automatically (unbuilt without the tag)

**Integration Tests:**
- `//go:build integration` tag, primarily `internal/storage/postgres/`
- Require Docker; use testcontainers-go with `pgvector/pgvector:pg18`, wait strategy `wait.ForLog("database system is ready").WithOccurrence(2)`
- Run via `task pr-prep` (not part of default `task check`)

**E2E Tests:**
- Ginkgo/Gomega, `go test -tags e2e`
- Split by concern: `e2e/api/` (ConnectRPC over HTTP), `e2e/cli/` (`e2e_cli` tag, drives the built binary via `testutil.CLIRunner`), `e2e/agent/`, `e2e/docker/` (Docker-in-Docker, skipped in CI)
- Full pipeline coverage (spark → shape → specify → decompose → claim → execute) exercised as ordered Ginkgo scenarios

## Common Patterns

**Interface conformance assertion (compile-time check on fakes):**
```go
var _ storage.WebAuthStore = (*logoutFakeWA)(nil)
```

**Error code assertions (not string matching) against sanitized handler errors:**
```go
// Handlers sanitize internal errors before returning to clients.
// Tests assert on connect.CodeNotFound / connect.CodeInternal, never on error text.
```

**No-op-on-uninitialized-context testing:**
```go
func TestStashChangeEvent_NoInit(_ *testing.T) {
	// Should not panic on un-initialized context.
	storage.StashChangeEvent(context.Background(), &storage.ChangeEvent{Slug: "x"})
}
```

---

*Testing analysis: 2026-07-08*
