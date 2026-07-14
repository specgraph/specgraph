---
phase: 04-verification-integration-reliability
reviewed: 2026-07-10T00:00:00Z
depth: deep
files_reviewed: 3
files_reviewed_list:
  - e2e/api/lifecycle_test.go
  - internal/storage/postgres/lifecycle_test.go
  - site/docs/concepts/drift.md
findings:
  critical: 0
  warning: 0
  info: 2
  total: 2
status: clean
---

# Phase 04: Code Review Report

**Reviewed:** 2026-07-10
**Depth:** deep (cross-referenced tests against real source: `internal/drift/drift.go`, `internal/server/lifecycle_handler.go`, `internal/mcp/tools_lifecycle.go`)
**Files Reviewed:** 3
**Status:** clean

## Summary

This is a TEST + DOCS phase (DRFT-01 verification-audit). No production code changed — verified via `git show --stat` on all four commits (`3b00de73`, `44a4e025`, `7acd0f4a`, `b6a21e73`): only the three files above were touched. No drift engine, proto, schema, or scope-SYNC changes leaked in.

The submitted work is sound. Every claim in the new tests and docs was traced to real source and holds up:

- **Doc accuracy (`drift.md`):** `LifecycleService.CheckDrift` / `AcknowledgeDrift` (confirmed against `lifecycle_handler.go` + `lifecycle.connect.go`) and the MCP `drift` tool with `check`/`acknowledge` actions plus `slug`/`upstream_slug`/`all` params (confirmed against `tools_lifecycle.go:48-100`) are all named accurately. The claim that all three surfaces share identical content-hash detection on `DEPENDS_ON` edges is correct — CLI, ConnectRPC handler, and MCP tool all funnel through `drift.Engine.Check`. The existing `!!! info "Planned"` interfaces/verify stub note (D-03) is preserved (drift.md:81-88), and the new section is correctly placed between "CLI Usage" and "Worked Example."

- **No-false-positive e2e test:** genuinely wires only `nfp-downstream → nfp-upstream` (an upstream that is never mutated) and edits `nfp-unrelated` (which has no downstream edge). Since the downstream's only dependency is untouched, a false-positive drift would surface as a non-empty `Reports[0].Items` and fail the `To(BeEmpty())` assertion. The failure mode the test targets is genuinely caught.

- **Per-upstream acknowledge round-trip e2e test:** correctly uses `UpstreamSlug` (not `All`), applies the `timestampSkew` + 3-attempt retry idiom to handle second-precision timestamp races, proves drift is detected, then proves per-upstream `AcknowledgeDrift` re-baselines `content_hash_at_link` so a re-check reports clean.

- **Full-graph mixed-state SkippedCount integration test:** seeds a real mixed state — a drifted-done pair (upstream mutated *after* the done-transition refreshed the edge baseline, so the hash genuinely diverges), a clean-done pair (upstream untouched), and a non-done spark spec. `drift.NewEngine(store, nil).Check(ctx, "", "deps")` yields `SkippedCount == 3` (5 specs − 2 done), satisfying `>= 1`, and asserts exactly the drifted downstream surfaces while the zero-item clean-done report is filtered out. Ordering is correct: done-transition refreshes `content_hash_at_link` to the upstream's then-current hash *before* the upstream mutation, per the storage contract in AGENTS.md.

**Test hygiene (all pass):**
- Build tags correct: `//go:build e2e` and `//go:build integration`.
- SPDX Apache-2.0 headers intact on both modified `.go` files.
- Isolation handled: the integration test calls `clearDatabase(t, store)` to avoid colliding with the existing blanket-zero "Drift detection (all specs)" assertion; the e2e specs use uniquely-named `nfp-*`/`pua-*` slugs (distinct from the primary `lifecycle-drift-*` block) and fully resolve any drift they create, leaving no residual for the "(all specs)" block that runs after them in the `Ordered` container.
- No error-string assertions introduced — the new tests are happy-path drift checks asserting `NotTo(HaveOccurred())`, not error-code paths, so the "assert on codes not messages" rule is not implicated.
- Timestamp-skew / hash-race handling present (`time.Sleep(timestampSkew)` before mutate; retry loop on detect).

The two findings below are non-blocking clarity/robustness observations. Neither changes the verdict.

## Info

### IN-01: Negative-path assertions pass vacuously without a positive control

**File:** `e2e/api/lifecycle_test.go:428-437` (no-false-positive check), also `:503-518` (pua re-check-clean)
**Issue:** Both "expect clean" assertions are guarded by `if len(resp.Msg.Reports) > 0 { ... }`. For a single-slug `CheckDrift`, `drift.Engine.Check` filters out zero-item reports (`drift.go:117`), so in the correct (passing) case `Reports` is empty and the inner `To(BeEmpty())` never executes. The test still correctly *fails* if a false positive occurs (a spurious drift item makes `Reports` non-empty → `To(BeEmpty())` fails), so the target failure mode is covered. However, the no-false-positive test has no positive control proving `CheckDrift` actually ran against `nfp-downstream` — a hypothetical regression that made `CheckDrift` always return empty `Reports` would pass this test silently. That machinery is exercised by the `pua-*` and primary `lifecycle-drift-*` blocks, so overall coverage is not lost.
**Fix:** Prefer the stronger, clearer form `Expect(resp.Msg.Reports).To(BeEmpty())` for the single-slug no-drift case (valid because zero-item reports are filtered out), which positively asserts the empty result instead of skipping the check.

### IN-02: "Sanity guard" seed comment is slightly misleading

**File:** `e2e/api/lifecycle_test.go:411-413` (nfp seed comment)
**Issue:** The comment claims keeping the `nfp-downstream → nfp-upstream` edge "means the test would fail if the drift path were not genuinely exercised (Pitfall 2 sanity guard)." Because `nfp-upstream` is never mutated, this edge can never produce a drift item regardless of whether the drift path runs — so it is not actually a sanity guard against the check being skipped. The edge's real purpose is to give the downstream a legitimate (clean) dependency so the test proves an *unrelated* edit doesn't drift it.
**Fix:** Reword the comment to describe the edge's true role (a clean baseline dependency), or drop the "sanity guard" framing to avoid implying a positive-control property the seed doesn't provide.

---

_Reviewed: 2026-07-10_
_Reviewer: the agent (gsd-code-reviewer)_
_Depth: deep_
