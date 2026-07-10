# Phase 4: Verification & Integration Reliability - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-10
**Phase:** 4-Verification & Integration Reliability
**Mode:** `--auto` (no interactive prompts; recommended option auto-selected per area and logged below)
**Areas discussed:** DRFT-01 requirement interpretation, DRFT-01 verification approach, DRFT-01 stub scopes, DRFT-01 interface documentation, INTG-01 code location

---

## DRFT-01 — Requirement interpretation ("Interface and verify")

| Option | Description | Selected |
|--------|-------------|----------|
| Expose + verify existing deps drift | Read "interface and verify" as: provide a stable documented interface (CLI/API/MCP) + test-verify the existing content-hash/DEPENDS_ON drift | ✓ |
| Implement `interfaces` + `verify` scopes | Build the two stubbed drift scopes as new detection algorithms | |

**Auto-selected:** Expose + verify existing deps drift (recommended default).
**Notes:** SC#2 explicitly names content-hash + DEPENDS_ON-edge scenarios (= the implemented `deps` scope); code-level/interface drift is separately tracked as v2 DRFT-02 (`spgr-93k`). The `--scope interfaces|verify` name collision is coincidental. ⚠️ Highest-impact gray area — auto-answered; confirm with user before planning if the intent was actually to build the stub scopes.

---

## DRFT-01 — Verification approach

| Option | Description | Selected |
|--------|-------------|----------|
| Audit + targeted gap-fill | Verify existing unit+e2e coverage against the repo, add only missing cases (false-positive, full-graph, ack round-trip) | ✓ |
| Full new test suite from scratch | Rebuild drift verification ignoring existing tests | |

**Auto-selected:** Audit + targeted gap-fill (recommended default).
**Notes:** Mirrors Phase 1 REL-01/CFG-01 "already shipped → scope to the true delta." Prefer real-DB e2e (existing `e2e/api/lifecycle_test.go` pattern) for SC#2, especially the "no false-positive on unrelated edits" case.

---

## DRFT-01 — Stub scopes (`interfaces` / `verify`)

| Option | Description | Selected |
|--------|-------------|----------|
| Keep stubbed, out of scope | Leave as documented "Planned / not yet implemented"; defer real detection to DRFT-02 | ✓ |
| Implement now | Build interface/verify detection this phase | |

**Auto-selected:** Keep stubbed, out of scope (recommended default).
**Notes:** Keep the three-way scope SYNC intact if touched (`driftscope.validScopes` ↔ `driftScopeToProtoMap` ↔ `driftScopeFromProtoMap`).

---

## DRFT-01 — Interface documentation

| Option | Description | Selected |
|--------|-------------|----------|
| Confirm + light extend | drift.md/cli-reference already document it; verify CLI/API/MCP all covered, add short API/MCP note if missing | ✓ |
| Large doc rewrite | Rewrite drift documentation | |

**Auto-selected:** Confirm + light extend (recommended default).
**Notes:** Existing doc is CLI-centric; SC#1 names CLI/API/MCP.

---

## INTG-01 — Confluence comment polling code location

| Option | Description | Selected |
|--------|-------------|----------|
| Blocker: locate code, confirm with user | Confirm code is absent in-repo, then surface to user to re-home or descope | ✓ |
| Scaffold a new Confluence connector | Build Confluence ingestion to "satisfy" the bug | |

**Auto-selected:** Blocker: locate code, confirm with user (recommended default).
**Notes:** Exhaustive scout found NO Confluence adapter/poller/pagination code in this repo — only unrelated design docs. `internal/sync/` has only beads + github adapters. The `spgr-jwbj` bug presupposes existing code that lives elsewhere. Building a connector would be scope creep, not a bug fix. ⚠️ Could not be resolved interactively under `--auto`; requires user input.

---

## the agent's Discretion

- Exact set of DRFT-01 verification tests to add vs. confirm-already-covered.
- Where the D-04 API/MCP-access documentation note lands.
- Test-helper/fixture shape for seeding drifted/clean/non-done specs in the full-graph verification.

## Deferred Ideas

- DRFT-02 (`spgr-93k`) — code-level drift detection (the real `interfaces`/`verify` content), v2.
- Implementing the `--scope interfaces` / `--scope verify` detectors — deferred with DRFT-02.
- Building a Confluence connector / ingestion pipeline — separate, much larger effort (cf. EXPL-02).
