# SpecGraph Implementation Tracker

> **For Claude:** Use this document to track overall progress across all slices. Mark checkboxes as tasks and slices are completed. Each slice's implementation plan lives in a separate file.

**Goal:** Complete the SpecGraph implementation through 7 vertical slices, each delivering a usable end-to-end capability.

**Dependency chain:** Slice 1 → Slice 2 → Slice 3 → Slice 3.5 → Slice 4 → Slice 5 → Slice 6 → Slice 7

---

## Slice 1: Vertical Slice (Foundation)

**Plan:** `docs/plans/2026-02-28-vertical-slice-plan.md`
**Status:** Complete (commits `50b504c`, `9fd18e5`)

- [x] Protobuf schema (5 services)
- [x] ConnectRPC server with all handlers
- [x] Memgraph storage backend with integration tests
- [x] CLI client (create, update, list, show, decision, claim, edge, deps, ready, critical-path, impact, health)
- [x] Config (docker/external/remote), Docker Compose
- [x] `specgraph init`
- [x] Dockerfile, E2E smoke test
- [x] Code quality (lefthook, golangci-lint)

**Verification:** All tests pass, CLI builds, E2E smoke test passes.

---

## Slice 2: Constitution

**Plan:** `docs/plans/2026-02-28-slice-2-constitution-plan.md`
**Status:** Complete (PR #7)

- [x] Task 1: Protobuf schema -- constitution.proto
- [x] Task 2: Storage interface -- ConstitutionBackend
- [x] Task 3: Memgraph implementation
- [x] Task 4: ConnectRPC handler -- ConstitutionService
- [x] ~~Task 5: Codebase scanner -- Tier 0~~ (Superseded — removed in Slice 3.5)
- [x] Task 6: Emitter -- constitution to tool files
- [x] Task 7: CLI -- constitution commands
- [x] Task 8: Enhanced init -- constitution creation
- [x] Task 9: Integration -- wire everything together
- [x] Task 10: Final verification and cleanup

**Slice Verification Gate:**

- [x] `go test ./... -count=1 -timeout=120s` -- all pass
- [x] `golangci-lint run ./...` -- no issues
- [x] `buf lint` -- no proto issues
- [x] `go build -o specgraph ./cmd/specgraph` -- clean build
- [x] CLI commands work: `specgraph constitution show|emit|check --help`
- [x] Integration: constitution is loaded and served via ConstitutionService

---

## Slice 3: Authoring Funnel

**Plan:** `docs/plans/2026-02-28-slice-3-authoring-funnel-plan.md`
**Status:** Complete (PR #8)
**Depends on:** Slice 2

- [x] Task 1: Protobuf schema -- authoring.proto
- [x] Task 2: Stage validation and transition rules
- [x] Task 3: Posture types and detection heuristic
- [x] Task 4: Prompt templates
- [x] Task 5: Safety net checks
- [x] Task 6: Analytical pass runner
- [x] Task 7: AuthoringBackend interface + Memgraph implementation
- [x] Task 8: ConnectRPC AuthoringService handler
- [x] Task 9: CLI authoring commands
- [x] ~~Task 10: Codebase scanner Tier 1 and Tier 2~~ (Superseded — removed in Slice 3.5)
- [x] Task 11: Wire AuthoringService into server
- [x] Task 12: End-to-end verification

**Slice Verification Gate:**

- [x] `go test ./internal/authoring/... -v -count=1` -- all pass (stages, posture, prompts, safety, passes)
- [x] `go test ./internal/storage/memgraph/... -v -count=1 -timeout=120s` -- all pass (including authoring)
- [x] `go test ./internal/server/... -v -count=1` -- all pass (AuthoringHandler)
- [x] `golangci-lint run ./...` -- no issues
- [x] `buf lint` -- no proto issues
- [x] `go build ./...` -- clean build
- [x] CLI commands work: `specgraph spark|shape|specify|decompose|approve --help`
- [x] Stage transitions enforce correct ordering (forward + backward amend)
- [x] Safety net flags security/data-loss concerns regardless of posture

---

## Slice 3.5: Scanner Removal & Documentation Cleanup

**Plan:** `docs/plans/2026-03-03-slice-3.5-scanner-cleanup-plan.md`
**Status:** Complete (PR #22)
**Depends on:** Slice 3

**Tasks:**

- [x] Task 1: Delete scanner package
- [x] Task 2: Remove --scan CLI flag and E2E test
- [x] Task 3: Remove scanner from Docker compose template
- [x] Task 4: Update init.go to remove scanner import and scan logic
- [x] Task 5: Update CI workflow (remove scanner test glob)
- [x] Task 6: Reframe plan docs (scanner references → agent-driven context)
- [x] Task 7: Update site docs (how-it-works, concepts, getting-started)
- [x] Task 8: Update CLAUDE.md gotchas
- [x] Task 9: Clean up go.mod (remove unused scanner dependencies)
- [x] Task 10: Final verification

---

## Slice 4: Execution Bundles and Prime

**Plan:** `docs/plans/2026-03-07-domain-types-and-slice4-plan.md`
**Status:** Complete
**Depends on:** Slice 3

- [x] Task 1: Protobuf schema -- execution.proto
- [x] Task 2: Storage interface -- ExecutionBackend (domain types)
- [x] Task 3: Memgraph implementation (domain types)
- [x] Task 4: ConnectRPC handler -- ExecutionService
- [x] Task 5: Bundle YAML rendering
- [x] Task 6: Lease sweeper -- background goroutine
- [x] Task 7: CLI -- bundle command
- [x] Task 8: CLI -- progress command
- [x] Task 9: Integration -- wire everything together
- [x] Task 10: Final verification and cleanup

**Additional work (domain types refactor):**

- [x] Refactor ConstitutionBackend to domain types
- [x] Refactor ClaimBackend to domain types
- [x] Proto↔domain converters in internal/server/convert.go
- [x] Zero proto imports in internal/storage/

**Slice Verification Gate:**

- [x] `go test ./... -count=1 -timeout=120s` -- all pass
- [x] `golangci-lint run ./...` -- no issues
- [x] `buf lint` -- pre-existing naming suggestions only (no new issues)
- [x] `go build -o specgraph ./cmd/specgraph` -- clean build
- [x] CLI commands work: `specgraph bundle|progress --help`
- [x] Bundle YAML renders with constitution context, spec details, and decisions
- [x] Lease sweeper releases expired claims

---

## Slice 5: Spec Lifecycle

**Plan:** `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md` (supersedes `2026-02-28-slice-5-spec-lifecycle-plan.md`)
**Status:** Complete (PR #27)
**Depends on:** Slice 4

- [x] Task 1: Protobuf schema -- spec extensions + LifecycleService
- [x] Task 2: Domain types -- extend Spec + add lifecycle types
- [x] Task 3: Update Memgraph spec storage for new fields
- [x] Task 4: Memgraph implementation -- lifecycle storage
- [x] Task 5: Spec JSON Schema + schema validator
- [x] Task 6: Linter engine
- [x] Task 7: Drift detection engine
- [x] Task 8: Proto↔domain converters + ConnectRPC handler
- [x] Task 9: CLI commands -- lifecycle operations
- [x] Task 10: End-to-end integration test

**Slice Verification Gate:**

- [x] `go test ./... -count=1 -timeout=120s` -- all pass
- [x] `go test ./test/e2e/ -run TestLifecycleE2E -v -count=1 -timeout=60s` -- E2E pass
- [x] `golangci-lint run ./...` -- no issues
- [x] `buf lint` -- no proto issues
- [x] `go build -o specgraph ./cmd/specgraph` -- clean build
- [x] CLI commands work: `specgraph amend|supersede|abandon|lint|drift --help`
- [x] Full lifecycle flow works: create -> done -> amend -> supersede -> abandon
- [x] Linter validates specs against JSON Schema
- [x] Drift engine detects spec-vs-code mismatches

---

## Slice 6: Sync and Integration

**Plan:** `docs/plans/2026-02-28-slice-6-sync-integration-plan.md`
**Status:** Complete (PR #30)
**Depends on:** Slice 5

- [x] Task 1: Protobuf schema -- sync.proto
- [x] Task 2: Storage interface -- SyncBackend
- [x] Task 3: Memgraph implementation
- [x] Task 4: SyncAdapter interface + Beads adapter
- [x] Task 5: GitHub adapter
- [x] Task 6: Tool injection
- [x] Task 7: ConnectRPC handler -- SyncService
- [x] Task 8: CLI commands -- sync + inject
- [x] Task 9: Command runner -- real implementation
- [x] Task 10: Final verification and cleanup

**Slice Verification Gate:**

- [x] `go test ./... -count=1 -timeout=120s` -- all pass
- [x] `golangci-lint run ./...` -- no issues
- [x] `buf lint` -- no proto issues
- [x] `go build -o specgraph ./cmd/specgraph` -- clean build
- [x] CLI commands work: `specgraph sync beads|github|status --help` and `specgraph inject --help`
- [x] Beads adapter creates/closes issues from specs
- [x] GitHub adapter creates issues/PRs from specs
- [x] Tool injection writes CLAUDE.md, .cursor/rules, AGENTS.md from constitution

---

## Slice 7: Global Daemon & Claude Code Plugin

**Design:** `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md` (supersedes `2026-02-28-slice-7-claude-code-plugin-plan.md`)
**Plan:** `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-plan.md`
**Status:** Complete
**Depends on:** Slice 6

**Phase A: Global Daemon Infrastructure**

- [x] XDG path resolution package (internal/xdg)
- [x] Global config schema with server/client split (internal/config/global.go)
- [x] Per-repo .specgraph.yaml reader with slug auto-derivation (internal/config/project.go)
- [x] Project domain type and ProjectBackend interface (internal/storage/project.go)
- [x] Memgraph Project CRUD implementation (internal/storage/memgraph/project.go)
- [x] WithProject option and Scoped method on Store
- [x] BELONGS_TO edges on all Cypher queries (9 files updated)
- [x] Service manager package -- launchd/systemd (internal/service)
- [x] specgraph up / down commands
- [x] specgraph prime command (session initialization)
- [x] specgraph init rework (global daemon model)
- [x] specgraph constitution import subcommand
- [x] Client wiring (new config + project header + old config fallback)
- [x] serve.go updated (global config, WithProject, project middleware)
- [x] X-Specgraph-Project RPC context middleware

**Phase B: Claude Code Plugin**

- [x] plugin.json manifest (11 skills, SessionStart hook)
- [x] SessionStart hook (session-start.sh → specgraph prime)
- [x] Meta-skill (overview/router)
- [x] Authoring skills (spark, shape, specify, decompose, approve)
- [x] Query skills (list, show, deps, ready)
- [x] Bundle skill

**Slice Verification Gate:**

- [x] `task check` -- all pass (fmt, license, lint, build, unit tests)
- [x] `go build -o specgraph ./cmd/specgraph` -- clean build
- [x] Plugin structure valid: `plugin.json` parses, all 11 skill paths resolve
- [x] CLI commands: `specgraph up|down|prime|init|constitution import` all work
- [ ] Integration tests with Docker -- `task test:integration` passes
- [ ] E2E full authoring funnel -- `task pr-prep` passes end-to-end

---

## Overall Verification

After all slices are complete:

- [ ] `go test ./... -count=1 -timeout=300s` -- full test suite passes
- [ ] `golangci-lint run ./...` -- zero issues
- [ ] `buf lint` -- clean proto
- [ ] `go build -o specgraph ./cmd/specgraph && ./specgraph --help` -- all commands listed
- [ ] Docker build: `docker build -t specgraph .` -- succeeds
- [ ] E2E: start server, run full authoring funnel, generate bundle, verify lifecycle
- [ ] Claude Code: install plugin, run `/specgraph`, verify skills work against running server
