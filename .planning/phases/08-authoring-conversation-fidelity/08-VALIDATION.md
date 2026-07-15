---
phase: 8
slug: authoring-conversation-fidelity
status: verified
nyquist_compliant: true
wave_0_complete: true
created: 2026-07-15
audited: 2026-07-15
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go standard testing; Ginkgo/Gomega for e2e) |
| **Config file** | none — `Taskfile.yml` defines `task test`, `task test:integration`, `task test:e2e` |
| **Quick run command** | `task test` (unit; excludes `//go:build integration` and `//go:build e2e`) |
| **Full suite command** | `task pr-prep` (check → integration → e2e; requires Docker running) |
| **Estimated runtime** | ~30–60s unit; several minutes for integration/e2e (testcontainers `pgvector:pg18`) |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task check` (fmt → license → lint → build → unit)
- **Before `/gsd-verify-work`:** Full suite (`task pr-prep`) must be green — **requires Docker running**
- **Max feedback latency:** 60 seconds (unit tier)

---

## Per-Task Verification Map

> Populated by the planner against the final task IDs. Each task must map to an
> automated `task test` / `task test:integration` command or declare a Wave 0
> dependency. The integration + MCP-only e2e gate (Success Criteria 1–4) is the
> backstop that recording is protocol-enforced, not agent-discretionary.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-T1 | 01 | 1 | CONV-01 | T-08-01 | Proto field-3 comment reword; no new field surface | unit (proto) | `task proto && task proto:check && go build ./...` | ✅ existing | ✅ green |
| 08-01-T2 | 01 | 1 | CONV-01 | T-08-01/04 | Approve-accept validates + records under stage tx (atomic) | unit (tdd) | `go build ./... && go test ./internal/server/...` | ✅ existing | ✅ green |
| 08-01-T3 | 01 | 1 | CONV-01 | T-08-02/03/04 | Empty→InvalidArgument; approved-stage conversation retrievable | integration | `task test:integration` | ✅ existing | 🔷 CI-gated |
| 08-02-T1 | 02 | 1 | CONV-01 | T-08-05 | MCP approve threads required exchanges; JSON-injection-safe parse | unit (tdd) | `go build ./... && go test ./internal/mcp/...` | ✅ existing | ✅ green |
| 08-02-T2 | 02 | 1 | CONV-01 | T-08-06 | Standalone record action removed; list retained | unit | `go build ./... && go test ./internal/mcp/...` | ✅ existing | ✅ green |
| 08-02-T3 | 02 | 1 | CONV-01 | — | Skill teaches approve-requires-exchanges; no token drift | unit | `task skills:validate && go test ./internal/authoring/... -run TestContentProtoDrift` | ✅ existing | ✅ green |
| 08-03-T1 | 03 | 1 | CONV-01 | T-08-08/10 | Shared `--conversation` loader; synthetic placeholder deleted | unit | `test ! -f cmd/specgraph/authoring_cli_exchanges.go && task license:check && go build ./... && go vet ./cmd/specgraph/...` | ✅ existing | ✅ green |
| 08-03-T2 | 03 | 1 | CONV-01 | T-08-10 | Loader array/stdin/missing-flag error paths | unit | `go test ./cmd/specgraph/...` | ✅ existing | ✅ green |
| 08-04-T1 | 04 | 2 | CONV-01 | T-08-11 | MCP-only funnel supplies approve exchanges; per-stage non-empty | e2e | `go test -tags e2e ./e2e/api/... -run "MCP-only authoring"` | ✅ existing | 🔷 CI-gated |
| 08-04-T2 | 04 | 2 | CONV-01 | T-08-12 | Positive fidelity + negative missing-exchanges → InvalidArgument | e2e | `go test -tags e2e ./e2e/api/...` | ✅ existing | 🔷 CI-gated |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · 🔷 CI-gated (test authored + compile-verified via `go vet -tags {integration,e2e}`; runtime green deferred to Docker CI / `task pr-prep`)*

**Audit (2026-07-15):** 10/10 tasks have an authored test. 7 unit tasks run green locally (`go test`, `task skills:validate`, `TestContentProtoDrift`, `task proto:check`). The 3 Wave-0 integration/e2e tasks (08-01-T3, 08-04-T1, 08-04-T2) compile clean under their build tags and are runtime-gated behind Docker — tracked as UAT items in `08-UAT.md`. No MISSING gaps; no test generation required.

---

## Wave 0 Requirements

- [x] Integration/e2e test asserting every funnel stage (shape/specify/decompose/approve) records a non-empty conversation — the "missing conversation cannot silently pass" backstop (Success Criteria 3). *Authored: `e2e/api/mcp_only_conversation_test.go` (positive per-stage + negative missing-exchanges) and `internal/storage/postgres/conversation_test.go` (`TestListConversations_ApprovedStageRetrievable`). Compile-verified; runtime green pending Docker CI.*
- [x] Docker running precondition documented in the plan's `<verify>` for integration/e2e tasks.

*Existing unit infrastructure (`go test`) covers the wiring/validator/loader tasks.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Skill/description prose accurately reflects the enforced-recording contract | CONV-01 | Prose review is not machine-assertable beyond token-drift checks | Read updated `internal/authoring/content/stage-*.md` and skill descriptions; confirm no "record if you choose" discretionary language remains |

*All functional phase behaviors have automated (unit + integration/e2e) verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references *(Wave-0 integration/e2e tests authored during execution and compile-verified; `wave_0_complete: true`, runtime green tracked as UAT items pending Docker CI)*
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** strategy nyquist-compliant — every task carries a real `<automated>` command with no watch-mode flags or MISSING refs. All 10 tasks have an authored test: 7 unit tasks run green locally; 3 Wave-0 integration/e2e backstops (08-01-T3, 08-04-T1/T2) are authored + compile-verified with runtime confirmation deferred to Docker CI (`task pr-prep`).

---

## Validation Audit 2026-07-15

| Metric | Count |
|--------|-------|
| Gaps found (MISSING) | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Tasks green (local) | 7 |
| Tasks CI-gated (authored + compile-verified) | 3 |

State-A audit: all task tests already exist post-execution — no test generation required, auditor not spawned. Statuses updated from `⬜ pending` to reflect actual coverage.
