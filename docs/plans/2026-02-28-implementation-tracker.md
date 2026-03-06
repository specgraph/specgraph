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

**Plan:** `docs/plans/2026-02-28-slice-4-execution-bundles-plan.md`
**Status:** Not started
**Depends on:** Slice 3

- [ ] Task 1: Protobuf schema -- execution.proto
- [ ] Task 2: Storage interface -- ExecutionBackend
- [ ] Task 3: Memgraph implementation
- [ ] Task 4: ConnectRPC handler -- ExecutionService
- [ ] Task 5: Bundle YAML rendering
- [ ] Task 6: Lease sweeper -- background goroutine
- [ ] Task 7: CLI -- bundle command
- [ ] Task 8: CLI -- progress command
- [ ] Task 9: Integration -- wire everything together
- [ ] Task 10: Final verification and cleanup

**Slice Verification Gate:**

- [ ] `go test ./... -count=1 -timeout=120s` -- all pass
- [ ] `golangci-lint run ./...` -- no issues
- [ ] `buf lint` -- no proto issues
- [ ] `go build -o specgraph ./cmd/specgraph` -- clean build
- [ ] CLI commands work: `specgraph bundle|progress --help`
- [ ] Bundle YAML renders with constitution context, spec details, and decisions
- [ ] Lease sweeper releases expired claims

---

## Slice 5: Spec Lifecycle

**Plan:** `docs/plans/2026-02-28-slice-5-spec-lifecycle-plan.md`
**Status:** Not started
**Depends on:** Slice 4

- [ ] Task 1: Protobuf schema -- spec extensions + LifecycleService
- [ ] Task 2: Update Memgraph spec storage for new fields
- [ ] Task 3: Storage interface -- LifecycleBackend
- [ ] Task 4: Memgraph implementation -- lifecycle storage
- [ ] Task 5: Spec JSON Schema
- [ ] Task 6: Linter engine
- [ ] Task 7: Drift detection engine
- [ ] Task 8: ConnectRPC handler -- LifecycleService
- [ ] Task 9: CLI commands -- lifecycle operations
- [ ] Task 10: End-to-end integration test

**Slice Verification Gate:**

- [ ] `go test ./... -count=1 -timeout=120s` -- all pass
- [ ] `go test ./test/e2e/ -run TestLifecycleE2E -v -count=1 -timeout=60s` -- E2E pass
- [ ] `golangci-lint run ./...` -- no issues
- [ ] `buf lint` -- no proto issues
- [ ] `go build -o specgraph ./cmd/specgraph` -- clean build
- [ ] CLI commands work: `specgraph amend|supersede|abandon|lint|drift --help`
- [ ] Full lifecycle flow works: create -> done -> amend -> supersede -> abandon
- [ ] Linter validates specs against JSON Schema
- [ ] Drift engine detects spec-vs-code mismatches

---

## Slice 6: Sync and Integration

**Plan:** `docs/plans/2026-02-28-slice-6-sync-integration-plan.md`
**Status:** Not started
**Depends on:** Slice 5

- [ ] Task 1: Protobuf schema -- sync.proto
- [ ] Task 2: Storage interface -- SyncBackend
- [ ] Task 3: Memgraph implementation
- [ ] Task 4: SyncAdapter interface + Beads adapter
- [ ] Task 5: GitHub adapter
- [ ] Task 6: Tool injection
- [ ] Task 7: ConnectRPC handler -- SyncService
- [ ] Task 8: CLI commands -- sync + inject
- [ ] Task 9: Command runner -- real implementation
- [ ] Task 10: Final verification and cleanup

**Slice Verification Gate:**

- [ ] `go test ./... -count=1 -timeout=120s` -- all pass
- [ ] `golangci-lint run ./...` -- no issues
- [ ] `buf lint` -- no proto issues
- [ ] `go build -o specgraph ./cmd/specgraph` -- clean build
- [ ] CLI commands work: `specgraph sync beads|github|status --help` and `specgraph inject --help`
- [ ] Beads adapter creates/closes issues from specs
- [ ] GitHub adapter creates issues/PRs from specs
- [ ] Tool injection writes CLAUDE.md, .cursor/rules, AGENTS.md from constitution

---

## Slice 7: Claude Code Plugin

**Plan:** `docs/plans/2026-02-28-slice-7-claude-code-plugin-plan.md`
**Status:** Not started
**Depends on:** Slice 6

- [ ] Task 1: Plugin manifest -- plugin.json
- [ ] Task 2: Meta-skill -- SpecGraph overview
- [ ] Task 3: Authoring skills -- spark, shape, specify, decompose, approve
- [ ] Task 4: Query skills -- list, show, deps, ready
- [ ] Task 5: Bundle skill
- [ ] Task 6: SessionStart hook
- [ ] Task 7: PostToolUse progress hook (optional)
- [ ] Task 8: Plugin packaging and installation
- [ ] Task 9: Wire hook command into CLI and integration test
- [ ] Task 10: CLI init --tool=claude-code support

**Slice Verification Gate:**

- [ ] `go test ./... -count=1 -timeout=120s` -- all pass
- [ ] `golangci-lint run ./...` -- no issues
- [ ] `go build -o specgraph ./cmd/specgraph` -- clean build
- [ ] Plugin structure valid: `plugin.json` parses, all skill/hook paths resolve
- [ ] `specgraph init --tool=claude-code` installs plugin files
- [ ] Skills trigger correctly: `/specgraph`, `/specgraph-spark`, `/specgraph-list`, `/specgraph-bundle`
- [ ] SessionStart hook auto-loads constitution context
- [ ] Full workflow: `specgraph init --tool=claude-code` -> `/specgraph-spark` -> `/specgraph-approve` -> `/specgraph-bundle`

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
