---
phase: 4
slug: verification-integration-reliability
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-10
---

# Phase 4 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Scope: DRFT-01 only (INTG-01 descoped). This is a verification-audit + doc phase over an
> already-shipped drift interface — the net-new work is closing SC#2 test gaps, not building features.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify` (unit); Ginkgo/Gomega v2 (e2e); testcontainers `pgvector/pgvector:pg18` (integration/e2e) |
| **Config file** | `Taskfile.yml` test targets — no separate framework config |
| **Quick run command** | `task test` (unit; excludes `//go:build integration` and `//go:build e2e`) |
| **Full suite command** | `task pr-prep` (check → `task test:integration` → `task test:e2e`) |
| **Estimated runtime** | ~30s unit; several min for pr-prep (Docker) |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task pr-prep` (integration + e2e — where the new SC#2 real-DB tests run; requires Docker)
- **Before `/gsd-verify-work`:** `task pr-prep` must be green
- **Max feedback latency:** ~30s (unit); pr-prep at wave boundaries

---

## Per-Task Verification Map

> Task IDs are assigned by the planner (PLAN.md). Rows below are the requirement→test contract
> from 04-RESEARCH.md; the planner MUST map each net-new (❌/⚠️) row to a concrete task.

| Behavior (Requirement) | Threat Ref | Test Type | Automated Command | File Exists | Status |
|---------|------------|-----------|-------------------|-------------|--------|
| DRFT-01: True drift flagged when upstream changes | — | unit + e2e | `task test` / `task test:e2e:api` | ✅ (`drift_test.go` TestCheckDependencyDrift; e2e "Drift detection") | ✅ green |
| DRFT-01: **No false-positive on unrelated edit** (headline SC#2) | — | integration/e2e | `task test:e2e:api` | ❌ W0 — add | ⬜ pending |
| DRFT-01: Full-graph mixed drifted/clean/skipped + `SkippedCount` | — | e2e/integration | `task test:e2e:api` | ⚠️ unit-only (`TestCheckAllSpecs`); add real-DB | ⬜ pending |
| DRFT-01: Acknowledge `--all` round-trip → clean | Tampering | e2e | `task test:e2e:api` | ✅ (e2e ack-all + "no drift after") | ✅ green |
| DRFT-01: Acknowledge `--upstream` round-trip → clean via interface | Tampering | integration/e2e | `task test:e2e:api` | ⚠️ storage-only; optional e2e mirror | ⬜ pending |
| DRFT-01: Non-done single-spec → `FailedPrecondition` | — | unit + e2e | `task test` / `task test:e2e:api` | ✅ (`TestCheck_NonDoneStageBySlug`) | ✅ green |
| DRFT-01: Scope SYNC intact (validScopes ↔ proto maps) | — | unit | `task test` | ✅ (`TestDriftScope*Map_Sync*`, `_Completeness`) | ✅ green |
| DRFT-01: API/MCP surfaces documented (SC#1) | Info disclosure | manual/doc | review `site/docs/concepts/drift.md` | ⚠️ doc gap — add API/MCP note (D-04) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `e2e/api/lifecycle_test.go` — add a **no-false-positive-on-unrelated-edit** spec (three specs; edit the unrelated one; assert downstream stays clean). Covers the headline SC#2 requirement. `[VERIFIED gap]`
- [ ] Full-graph mixed-state coverage at real-DB level — extend `e2e/api/lifecycle_test.go` (or a `//go:build integration` test in `internal/storage/postgres`) asserting `SkippedCount >= 1` with a drifted-done + clean-done + non-done seed. `[VERIFIED gap]`
- [ ] (Optional) e2e mirror of per-upstream (`--upstream`) acknowledge round-trip through `CheckDrift`.
- [ ] Documentation: API/MCP access note in `site/docs/concepts/drift.md` (D-04).

*No new framework install required — Ginkgo/testcontainers/testify already present. Existing infrastructure covers all true-positive, error-path, scope-sync, and `--all` ack cases.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| API/MCP access documentation reads correctly | DRFT-01 SC#1 | Doc-quality is a human judgment | Review `site/docs/concepts/drift.md` renders and the API/MCP note is accurate against `LifecycleService.CheckDrift` + the MCP `drift` tool |

*All functional phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references (the three SC#2 gaps + doc note)
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s (unit); pr-prep at wave boundaries
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
