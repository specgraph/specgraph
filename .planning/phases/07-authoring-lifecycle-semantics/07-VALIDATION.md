---
phase: 7
slug: authoring-lifecycle-semantics
status: approved
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-14
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (unit) + Ginkgo/Gomega (e2e, `-tags e2e`) + testcontainers (postgres integration) |
| **Config file** | Taskfile.yml (task test / task test:integration / task test:e2e) |
| **Quick run command** | `task test` |
| **Full suite command** | `task pr-prep` (check → integration → e2e; requires Docker) |
| **Estimated runtime** | ~60–180 seconds (unit); integration/e2e longer (Docker) |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task test:integration` (postgres storage waves) / `task test:e2e` (MCP surface waves)
- **Before `/gsd-verify-work`:** Full suite must be green (`task pr-prep`)
- **Max feedback latency:** 180 seconds (unit); Docker suites out-of-band

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 07-01-T1 | 07-01 | 1 | LIFE-01 | — | Proto field addition (non-colliding field 3); no wire-surface change | build/proto | `grep -q 'string reason = 3;' proto/specgraph/v1/lifecycle.proto && task proto && grep -q 'Reason' gen/specgraph/v1/lifecycle.pb.go && go build ./...` | ✅ existing (lifecycle.proto, gen/) | ⬜ pending |
| 07-01-T2 | 07-01 | 1 | LIFE-01 | T-07-03, T-07-04 | Reason threaded via parameterized pgx inside RunInTransaction; errors sanitized (assert connect codes) | unit | `go build ./... && go vet ./... && go test ./internal/storage/... ./internal/server/...` | ✅ existing (lifecycle_handler_test.go) | ⬜ pending |
| 07-01-T3 | 07-01 | 1 | LIFE-01 | T-07-02 | CLI `--reason` optional flag; no new authz surface | build/smoke | `go build ./... && go run ./cmd/specgraph supersede --help 2>&1 \| grep -q -- '--reason'` | ✅ existing (cmd/specgraph/lifecycle.go) | ⬜ pending |
| 07-02-T1 | 07-02 | 2 | LIFE-01 | T-07-01 | Stale-lease removal: claims row + CLAIMED_BY edge deleted in same amend tx (txCtx, parameterized pgx) | build | `go build ./... && go vet ./...` | ✅ existing (postgres/lifecycle.go) | ⬜ pending |
| 07-02-T2 | 07-02 | 2 | LIFE-01, LIFE-02 | T-07-01, T-07-06 | Integration proof of claim-release, re-entry landing, done-only supersede, supersede reason | integration | `go build -tags integration ./internal/storage/postgres/ && task test:integration` | 🆕 `TestLifecycleAmend_ReleasesClaim` added in this task (postgres/lifecycle_test.go) | ⬜ pending |
| 07-03-T1 | 07-03 | 3 | LIFE-01, LIFE-02 | T-07-04 | `IsValidReEntryStage` allowlist enforced in TransitionAmend handler + LifecycleAmendSpec storage; approved/in_progress/review/done rejected with CodeInvalidArgument (error code, not message); explicit `IsValidReEntryStage(approved)==false` assertion + extended `AmendSpec_InvalidReEntryStage` storage integration case (approved/in_progress/review) guard against a footgun/revert | unit + integration | `go build ./... && go vet ./... && go test ./internal/storage/... ./internal/server/...` (unit); `task test:integration` at wave boundary for extended `AmendSpec_InvalidReEntryStage` | 🆕 `IsValidReEntryStage` + handler rejection test + extended `AmendSpec_InvalidReEntryStage` added in this task (spec_domain.go, spec_domain_test.go, lifecycle_handler.go, lifecycle_handler_test.go, postgres/lifecycle.go, postgres/lifecycle_test.go) | ⬜ pending |
| 07-03-T2 | 07-03 | 3 | LIFE-01, LIFE-02 | T-07-05, T-07-04, T-07-03 | Reroute inherits handler preconditions; tool passes re_entry_stage through to the single (now-correct) gate | build/grep | `go build ./... && rg -q 're_entry_stage' internal/mcp/tools_authoring.go && rg -q 'new_slug' internal/mcp/tools_authoring.go && rg -q 'Lifecycle.TransitionAmend' internal/mcp/tools_authoring.go` | ✅ existing (mcp/tools_authoring.go) | ⬜ pending |
| 07-03-T3 | 07-03 | 3 | LIFE-01, LIFE-02 | T-07-03 | Tool tests assert res.IsError / connect codes (not message strings); mock returns sentinels | unit | `go build ./... && go vet ./... && go test ./internal/mcp/...` | ✅ existing (mcp/tools_authoring_test.go) | ⬜ pending |
| 07-04-T1 | 07-04 | 4 | LIFE-01 | T-07-08 | RPC removed from wire surface; no non-test prod caller of Authoring.Amend/Supersede remains; remaining funnel RPCs still scope-enforced; build stays green | build/unit | `! rg -q 'Authoring\.(Amend\|Supersede)\(' internal/ --glob '!*_test.go' && task proto && go build ./... && go vet ./... && go test ./internal/server/... ./internal/mcp/...` | ✅ existing (authoring.proto, gen/) | ⬜ pending |
| 07-04-T2 | 07-04 | 4 | LIFE-01 | T-07-07 | Divergent second amend/supersede impl deleted (no re-divergence); absence greps + green check | build/check | `task proto && go build ./... && go vet ./... && task check` | ✅ existing (authoring_handler.go, storage/*, authoring.proto) | ⬜ pending |
| 07-05-T1 | 07-05 | 5 | LIFE-01, LIFE-02 | T-07-09 | Skills teach land-one-before + constrained params so MCP agent cannot reproduce #899 no-op | skills-validate | `rg -q 're_entry_stage' internal/mcp/skills/embedded/specgraph-authoring/SKILL.md && rg -q 'new_slug' internal/mcp/skills/embedded/specgraph-authoring/SKILL.md && task skills:validate` | ✅ existing (specgraph-authoring/troubleshooting SKILL.md) | ⬜ pending |
| 07-05-T2 | 07-05 | 5 | LIFE-01, LIFE-02 | T-07-09, T-07-03 | MCP-only e2e proves in-flight amend + re-author and done→supersede across distinct specs + both rejection cases; asserts res.IsError | e2e | `go build -tags e2e ./e2e/api/ && task test:e2e` | 🆕 `e2e/api/mcp_only_lifecycle_test.go` created in this task | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing test infrastructure covers all phase requirements. New tests are created inside their own plans (not as separate Wave 0 scaffolding):

- `TestSpecStage_IsValidReEntryStage` (asserting `IsValidReEntryStage(SpecStageApproved)==false`) + a TransitionAmend handler rejection case (approved/done → CodeInvalidArgument) + an extended `AmendSpec_InvalidReEntryStage` storage integration case (approved/in_progress/review → `ErrInvalidReEntryStage`) — created in 07-03 Task 1 (extends existing `spec_domain_test.go`, `lifecycle_handler_test.go`, and `postgres/lifecycle_test.go`).
- `TestLifecycleAmend_ReleasesClaim` — created in 07-02 Task 2 (postgres storage integration; extends existing `lifecycle_test.go` harness).
- `e2e/api/mcp_only_lifecycle_test.go` — created in 07-05 Task 2 (MCP-only Ginkgo e2e; modeled on existing `mcp_only_authoring_test.go`).

All other requirements are covered by existing unit / integration / e2e infrastructure (lifecycle_handler_test.go, tools_authoring_test.go, postgres/lifecycle_test.go, Taskfile targets). No MISSING `<automated>` references — every task carries a runnable command.

---

## Manual-Only Verifications

All phase behaviors have automated verification (MCP-only e2e + storage integration + unit). No manual-only verifications.

*Note: 07-05 Task 2 uses `task test:e2e` (>30s) as its per-task verify — this is inherent to the D-10 MCP-only e2e gate (Ginkgo suite requiring Docker). Accepted per Nyquist 8b: the D-10 acceptance contract is an end-to-end MCP-driven sequence that has no faster automated equivalent.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none MISSING; two new tests created in-plan)
- [x] No watch-mode flags
- [x] Feedback latency < 180s (unit); Docker integration/e2e out-of-band per sampling policy
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved
