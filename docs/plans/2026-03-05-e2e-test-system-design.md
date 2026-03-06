# E2E Test System Design

**Date:** 2026-03-05
**Status:** Approved

## Problem

The CLI surface has 19.5% unit test coverage, almost entirely from `init()` registrations and two helper functions. Every `run*` handler is at 0%. The existing E2E smoke test (`e2e/smoke_test.sh`) is a bash script covering only 3 of ~25 CLI commands. There is no way to prove the CLI works end-to-end.

## Design

### Two Test Tiers

Both use **Ginkgo v2 + Gomega** as the BDD test framework.

**Tier 1 — API E2E tests (`e2e/api/`):** The bulk of coverage. Testcontainers spins up Memgraph, Go test starts the specgraph HTTP server in-process. Tests exercise the full CLI surface by calling ConnectRPC endpoints through real client code. Fast iteration, no Docker image required for the server.

**Tier 2 — Docker mode tests (`e2e/docker/`):** Tests the `serve` command's `mode: docker` path — verifies it writes a compose file, starts Memgraph via Docker Compose, and tears it down cleanly. Smaller, focused suite that validates the Docker lifecycle specifically.

### Build Tags

All E2E files use `//go:build e2e` so `go test ./...` skips them. Dedicated Taskfile targets run them.

### Test Coverage Map

#### API Suite (`e2e/api/`)

```text
Describe("health")
  It("returns healthy when server is running")

Describe("specgraph init")
  It("creates config with defaults in non-interactive mode")
  It("scans codebase and generates constitution draft with --scan")
  It("rejects init when config already exists")

Describe("spec lifecycle", Ordered)
  It("creates a spec with slug, intent, and priority")
  It("lists specs and includes the created spec")
  It("shows spec details by slug")
  It("updates spec fields")

Describe("decision lifecycle", Ordered)
  It("creates a decision")
  It("lists decisions")
  It("shows decision by slug")

Describe("constitution", Ordered)
  It("shows the bootstrapped constitution")
  It("checks a spec against constitution constraints")
  It("emits constitution as YAML")

Describe("claim protocol", Ordered)
  It("claims an approved spec")
  It("rejects double-claim")
  It("unclaims a spec")

Describe("graph edges", Ordered)
  It("adds edges between specs")
  It("lists edges for a spec")
  It("removes an edge")

Describe("graph queries", Ordered)
  It("shows dependency tree with deps")
  It("shows specs ready to work on with ready")
  It("shows critical path for a spec")
  It("shows impact of changes to a spec")

Describe("authoring funnel", Ordered)
  It("sparks a new spec from an idea")
  It("shapes a sparked spec")
  It("specifies a shaped spec")
  It("decomposes a specified spec")
  It("approves a decomposed spec")

Describe("error handling")
  It("rejects commands with missing required args")
  It("returns error for nonexistent slugs")
  It("rejects claim on unapproved spec")
  It("detects edge cycles")
```

#### Docker Suite (`e2e/docker/`)

```text
Describe("serve with docker mode", Ordered)
  It("writes docker-compose.yaml to .specgraph/")
  It("starts Memgraph via compose and serves API")
  It("health check passes")
  It("tears down compose on shutdown")
```

### Test Helpers (`e2e/testutil/`)

- `StartMemgraph(ctx) -> (boltURI, cleanup)` — wraps testcontainers Memgraph setup
- `StartServer(ctx, boltURI) -> (baseURL, cleanup)` — starts in-process HTTP server on random port
- `NewCLI(configPath) -> runner` — wraps exec.Command for specgraph binary, captures stdout/stderr

### File Layout

```text
e2e/
  testutil/
    containers.go        # Memgraph testcontainer helper
    server.go            # In-process server startup
    cli.go               # CLI binary runner
  api/
    api_suite_test.go    # Ginkgo bootstrap, BeforeSuite/AfterSuite
    health_test.go       # health check
    init_test.go         # init, init --scan
    spec_test.go         # spec create/list/show/update
    decision_test.go     # decision create/list/show
    constitution_test.go # constitution show/check/emit
    claim_test.go        # claim/unclaim
    edge_test.go         # edge add/remove/list
    graph_test.go        # deps/ready/critical-path/impact
    authoring_test.go    # spark/shape/specify/decompose/approve
    errors_test.go       # missing args, nonexistent slugs, invalid states
  docker/
    docker_suite_test.go # Ginkgo bootstrap
    serve_test.go        # serve with mode: docker lifecycle
```

### Taskfile Additions

```yaml
test:e2e:api:
  desc: Run API E2E tests (requires Docker)
  cmds:
    - go test -tags e2e -v -timeout 5m ./e2e/api/...

test:e2e:docker:
  desc: Run Docker mode E2E tests (requires Docker)
  deps: [build]
  cmds:
    - go test -tags e2e -v -timeout 5m ./e2e/docker/...

test:e2e:
  desc: Run all E2E tests
  cmds:
    - task: test:e2e:api
    - task: test:e2e:docker
```

### Dependencies

- `github.com/onsi/ginkgo/v2` — BDD test framework
- `github.com/onsi/gomega` — matcher library
- `github.com/testcontainers/testcontainers-go` — already in go.mod

### Migration

The old `e2e/smoke_test.sh` is deleted and replaced by these Go tests.

### CI

Add `task test:e2e` as a GitHub Actions workflow step. GitHub-hosted runners provide Docker.
