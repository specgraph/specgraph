---
phase: 04-verification-integration-reliability
verified: 2026-07-10T16:21:47Z
status: passed
score: 4/4 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 4: Verification & Integration Reliability — Verification Report

**Phase Goal:** Maintainers can trust that reported drift signals are correct and verifiable.
**Verified:** 2026-07-10T16:21:47Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

Phase goal decomposes into the two ROADMAP Success Criteria for DRFT-01. Both are met with behavioral (real-DB) evidence, not just presence:

- **SC#1** — drift status queryable through a stable, documented CLI/API/MCP interface.
- **SC#2** — the interface is verified against real content-hash + `DEPENDS_ON` scenarios; flags true drift and does NOT false-positive on unrelated edits.

INTG-01 was formally descoped (Confluence poller not in this repo, D-05) and is recorded as "Descoped" in ROADMAP.md and REQUIREMENTS.md. It is **not** a gap.

### Observable Truths

| #   | Truth (source) | Status | Evidence |
| --- | -------------- | ------ | -------- |
| 1 | No-false-positive: editing a NON-upstream spec produces no drift on a downstream done-spec, via `LifecycleService.CheckDrift` (04-01 / SC#2) | ✓ VERIFIED | `e2e/api/lifecycle_test.go:393` `Describe("Drift detection (no false-positive on unrelated edit)")` — seeds 3 done specs, wires only `nfp-downstream`→`nfp-upstream`, mutates only `nfp-unrelated`, asserts downstream Items empty. **Ran green vs real Postgres** (part of 5-Passed e2e run). |
| 2 | Full-graph CheckDrift (empty slug) over mixed seed reports `SkippedCount >= 1` and surfaces exactly the drifted spec (04-01 / SC#2) | ✓ VERIFIED | `internal/storage/postgres/lifecycle_test.go:499` `t.Run("CheckAllSpecs_MixedState_SkippedCount")` — drifted-done + clean-done + non-done seed; asserts `SkippedCount>=1`, `mix-drift-down` present with Items, `mix-clean-down` filtered. **PASS: TestLifecycle/CheckAllSpecs_MixedState_SkippedCount (0.12s)** vs real Postgres (pgvector:pg18). |
| 3 | Per-upstream `AcknowledgeDrift` re-baselines `content_hash_at_link`; subsequent CheckDrift reports clean, end-to-end (04-01 / SC#2) | ✓ VERIFIED | `e2e/api/lifecycle_test.go:451` `Describe("Drift detection (per-upstream acknowledge)")` — drift detected via 3-attempt retry, `AcknowledgeDrift{UpstreamSlug: ...}` (not `All`), re-check clean. **Ran green vs real Postgres** (part of 5-Passed e2e run). |
| 4 | Drift is documented as queryable across CLI, API (`CheckDrift`/`AcknowledgeDrift`), and MCP (`drift` tool) (04-02 / SC#1) | ✓ VERIFIED | `site/docs/concepts/drift.md:91` `## Accessing Drift via API / MCP` names both RPCs + the MCP `drift` tool `check`/`acknowledge` actions; states all three surfaces share content-hash detection on `DEPENDS_ON` edges. Named symbols confirmed to exist in code (see Key Links). Existing `!!! info "Planned"` note intact (line 81). |

**Score:** 4/4 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `e2e/api/lifecycle_test.go` — no-false-positive Describe | New block, 3-spec seed | ✓ VERIFIED | Lines 393–444; substantive, compiles under `e2e` tag, passes real DB. |
| `e2e/api/lifecycle_test.go` — per-upstream ack Describe | New block, `UpstreamSlug` path | ✓ VERIFIED | Lines 451–520; substantive, passes real DB. |
| `internal/storage/postgres/lifecycle_test.go` — SkippedCount subtest | New `t.Run` under `TestLifecycle` | ✓ VERIFIED | Lines 499–558; substantive, passes real DB, no import cycle. |
| `site/docs/concepts/drift.md` — API/MCP section | New `## Accessing Drift via API / MCP` | ✓ VERIFIED | Lines 91–103, positioned between `## CLI Usage` and `## Worked Example`. |

### Key Link Verification

| From | To | Via | Status | Details |
| ---- | -- | --- | ------ | ------- |
| Doc note | API handler | `LifecycleService.CheckDrift` / `AcknowledgeDrift` | ✓ WIRED | `internal/server/lifecycle_handler.go:140` (`CheckDrift`), `:178` (`AcknowledgeDrift`) exist. |
| Doc note | MCP tool | `drift` tool `check`/`acknowledge` | ✓ WIRED | `internal/mcp/tools_lifecycle.go:50` `Name: "drift"`, `:72` `case "check"`, `:74` `case "acknowledge"`. |
| Doc note | CLI | `specgraph drift` | ✓ WIRED | `cmd/specgraph/lifecycle.go:123` `driftCmd` (`Use: "drift [slug]"`). |
| Test | Engine detection | `content_hash_at_link` vs upstream `ContentHash` | ✓ WIRED | `internal/drift` unit tests pass; integration + e2e exercise the deps path end-to-end. |
| Test | done-eligibility / SkippedCount | non-done specs counted skipped in all-specs mode | ✓ WIRED | Integration subtest asserts `SkippedCount>=1` for `mix-skipped` (left at spark). |
| Test | Project scoping | `X-Specgraph-Project` header via shared e2e clients | ✓ WIRED | New specs reuse the package-level scoped clients; e2e run green. |
| Doc | Planned-stub note | `--scope interfaces\|verify` "Planned" admonition | ✓ WIRED | `site/docs/concepts/drift.md:81` unchanged (D-03 preserved). |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| e2e compile | `go vet -tags e2e ./e2e/api/...` | exit 0 | ✓ PASS |
| integration compile | `go vet -tags integration ./internal/storage/postgres/...` | exit 0 | ✓ PASS |
| drift/driftscope unit | `go test ./internal/drift/... ./internal/driftscope/...` | ok | ✓ PASS |
| Full-graph SkippedCount (real DB) | `go test -tags integration -count=1 -run 'TestLifecycle/CheckAllSpecs_MixedState_SkippedCount'` | PASS (0.12s) | ✓ PASS |
| No-false-positive + per-upstream ack (real DB) | `go test -tags e2e -count=1 -ginkgo.focus="no false-positive on unrelated edit\|per-upstream acknowledge"` | Ran 5 of 201 — **5 Passed \| 0 Failed** | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| DRFT-01 | 04-01, 04-02 | Interface and verify drift detection (`spgr-vch`) | ✓ SATISFIED | SC#1 (docs across CLI/API/MCP) + SC#2 (3 real-DB proofs) both met. REQUIREMENTS.md line 31 marked `[x]`, line 118 `Complete`. |
| INTG-01 | — (descoped) | Confluence poller pagination (`spgr-jwbj`) | N/A DESCOPED | Not in this repo (D-05). ROADMAP.md line 119 + REQUIREMENTS.md line 119 record "Descoped". Not a gap. |

### Anti-Patterns Found

None. The four commits (`3b00de73`, `44a4e025`, `7acd0f4a`, `b6a21e73`) are additions only to two test files and one doc file — no drift engine, converter, scope-table, proto, or migration change (D-01/D-03 discipline confirmed via `git show --stat`). The `if len(resp.Msg.Reports) > 0` guards in the e2e specs are intentional (zero-item reports are filtered server-side), not stubs; the assertion path is genuinely exercised because the DEPENDS_ON edge remains in each seed (Pitfall-2 guard).

### Human Verification Required

None blocking. (04-02-SUMMARY flagged doc *prose readability* as a soft human-judgment item. The structural/naming accuracy is objectively confirmed — every symbol named in the doc exists in code with matching semantics — so goal achievement does not depend on it. Optional editorial review of `site/docs/concepts/drift.md:91–103` only.)

### Gaps Summary

No gaps. Both Success Criteria are met with real-DB behavioral evidence:
- SC#1: drift is documented and reachable across CLI (`driftCmd`), API (`CheckDrift`/`AcknowledgeDrift`), and MCP (`drift` tool) — all three surfaces exist and are now named in `drift.md`.
- SC#2: three targeted real-DB tests prove true-drift detection, no-false-positive on unrelated edits, full-graph `SkippedCount` accounting, and per-upstream acknowledge re-baselining.

The phase goal — "Maintainers can trust that reported drift signals are correct and verifiable" — is achieved: signals are verifiable through a documented, stable, multi-surface interface and are proven correct by an executed real-DB test suite. D-01/D-03 discipline (no engine/proto/schema/scope-SYNC change) held.

---

_Verified: 2026-07-10T16:21:47Z_
_Verifier: the agent (gsd-verifier)_
