# Phase 4: Verification & Integration Reliability - Context

**Gathered:** 2026-07-10
**Status:** Ready for planning
**Mode:** `--auto` (decisions auto-selected to the recommended option; audit and adjust before/after planning)
**Scope update (2026-07-10):** After scouting, the user **descoped INTG-01** from
this phase — the Confluence poller is not in this repo (D-05). **Phase 4 now plans
DRFT-01 only.** INTG-01 removed from ROADMAP.md/REQUIREMENTS.md pending its owning repo.

<domain>
## Phase Boundary

Two small, independent reliability requirements migrated from `bd`/beads
(`spgr-vch`, `spgr-jwbj`). Neither is covered by the 177-doc intel corpus —
both were scoped directly from current repo state during this discussion (per
the STATE.md blocker note).

- **DRFT-01 (`spgr-vch`) — "Interface and verify drift detection":** maintainers
  can trust that drift signals are correct and queryable through a stable,
  documented interface (CLI / API / MCP), and that the interface is *verified*
  against real content-hash and `DEPENDS_ON`-edge scenarios by a test suite that
  confirms it flags true drift and does not false-positive on unrelated edits.
- **INTG-01 (`spgr-jwbj`) — "Fix Confluence comment polling pagination bug":**
  Confluence comment polling should walk every page of results so no comments
  are silently skipped when a thread's comment count spans multiple pages.

This is HOW-to-implement clarification on fixed roadmap scope. **No new drift
*dimensions* and no new integrations are introduced** — this is hardening and
verification of existing (DRFT-01) or externally-located (INTG-01) behavior.

**Critical scouting findings (drive every decision below):**

- **DRFT-01's interface already exists and is mature — this parallels Phase 1's
  REL-01/CFG-01 "already shipped" discovery.** The deps-based drift detector is
  fully wired end-to-end:
  - **Engine:** `internal/drift/drift.go` — `Engine.Check(ctx, slug, scope)`
    compares each `DEPENDS_ON` edge's `content_hash_at_link` against the
    upstream's live `ContentHash`; supports single-spec and full-graph
    (empty-slug) modes; only `done`-stage specs are eligible.
  - **API / MCP:** `LifecycleService.CheckDrift` + `AcknowledgeDrift`
    (`internal/server/lifecycle_handler.go`).
  - **CLI:** `specgraph drift [slug] --scope --json` and
    `specgraph drift acknowledge <slug> --note --upstream|--all`
    (`cmd/specgraph/lifecycle.go`), rendered via `render.DriftReport`.
  - **Docs:** `site/docs/concepts/drift.md` (full concept + CLI usage) and
    `site/docs/cli-reference.md` already document it.
  - **Tests:** `internal/drift/drift_test.go` (15 unit tests over a mock backend
    — deps drift, content-hash mismatch, empty-edge-hash, scope filtering, error
    paths) **plus** real-DB e2e coverage in `e2e/api/lifecycle_test.go`
    ("Drift detection" describe block: update upstream → drift appears) and
    `e2e/api/lifecycle_pipeline_test.go`.
  - The `interfaces` and `verify` drift **scopes are deliberate stubs** — they
    return "not yet implemented" and are documented as "Planned" in `drift.md`.
    They are NOT the subject of DRFT-01 (see D-01 for the naming-collision
    resolution); code-level drift is a separate v2 item (DRFT-02, `spgr-93k`).
  - **Likely outcome:** DRFT-01's roadmap SC#1 (queryable/documented interface)
    is already met; the net-new work is a targeted *verification* audit and
    gap-fill (SC#2), not a build. Research MUST confirm-or-refute this against
    the repo first (Phase-1 pattern).

- **INTG-01's target code does NOT exist in this repository.** An exhaustive
  scout (`rg -i "confluence|poll|comment|paginat"` across `*.go`) found **no
  Confluence adapter, no comment-polling loop, and no pagination code**. The
  only Confluence artifacts are documentation (`docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`
  — an unrelated *template* bridge doc) and a keyword entry in `cmd/specgraph/nudge.go`.
  The sync layer (`internal/sync/`) has only **beads** and **github** adapters.
  `cmd/specgraph/connector.go` is a *Postgres* connection helper, not a
  Confluence connector. **The `spgr-jwbj` pagination bug lives in a different
  system/repo** (a separate Confluence connector/ingestion tool). This is a
  hard blocker for actioning INTG-01 here — see D-05. In `--auto` this could not
  be resolved interactively; **the user must confirm where this code lives before
  INTG-01 can be planned.**

</domain>

<decisions>
## Implementation Decisions

### DRFT-01 — Interface & verify drift detection (`spgr-vch`)

- **D-01 (requirement interpretation — recommended default):** Read "Interface
  and verify drift detection" as **"expose drift detection through a stable,
  documented interface, and *verify* it with tests"** — NOT as "implement the
  stubbed `interfaces` and `verify` drift *scopes*." Rationale: (a) roadmap
  SC#2 explicitly names *content-hash* and *`DEPENDS_ON`-edge* scenarios, which
  is precisely the already-implemented `deps` scope; (b) SC#1 is about
  queryability, not new detection dimensions; (c) code-level / interface / verify
  drift is separately tracked as v2 DRFT-02 (`spgr-93k`). The scope-name
  collision (`--scope interfaces|verify`) is coincidental and must not pull this
  phase into building two new detection algorithms.
  - ⚠️ **This is the single most important gray area and it was auto-answered.**
    If the intent was actually to implement the `interfaces`/`verify` scopes,
    that is a materially larger phase — flag to the user before planning.

- **D-02 (scope = verification audit + gap-fill, not a rebuild):** Treat DRFT-01
  like Phase 1's REL-01/CFG-01: **first verify what already exists against the
  repo**, then close only the real gaps. Expected concrete deltas (research to
  confirm):
  1. **False-positive verification (SC#2 "doesn't false-positive on unrelated
     edits"):** add/confirm a test proving an edit to an *unrelated* spec (not an
     upstream dependency) produces **no** drift on the downstream — the current
     suite proves true-positive and empty-hash, but the explicit
     no-false-positive case should be nailed down.
  2. **Full-graph query verification:** confirm the empty-slug "check all specs"
     path is covered at the e2e/integration level (mixed drifted + clean +
     skipped-non-done specs, and the `SkippedCount` accounting).
  3. **Acknowledge round-trip:** confirm acknowledging (single upstream and
     `--all`) re-baselines `content_hash_at_link` and a subsequent check reports
     clean — end-to-end, not only via handler unit tests.
  4. **Interface-stability/documentation check:** confirm CLI + API + MCP
     surfaces are all documented (see D-04).
  - Prefer real-DB verification (testcontainers/e2e, the existing
    `e2e/api/lifecycle_test.go` pattern) over more mock-backend unit tests for
    SC#2, since "verified against real content-hash and `DEPENDS_ON`-edge
    scenarios" implies real storage behavior.

- **D-03 (stub scopes stay stubbed — explicitly OUT of scope):** Do **NOT**
  implement `--scope interfaces` or `--scope verify` in this phase. Leave them as
  documented "Planned / not yet implemented" stubs. If anything, planning may
  *tighten* their surface (clearer error, or keep as-is) but building real
  interface/verify detection belongs to DRFT-02 (v2). Keep the
  `driftscope.validScopes` ↔ `driftScopeToProtoMap` ↔ `driftScopeFromProtoMap`
  three-way SYNC intact if touched.

- **D-04 (documentation = confirm, lightly extend):** `site/docs/concepts/drift.md`
  and `cli-reference.md` already document the concept and CLI thoroughly. The
  doc is **CLI-centric**; SC#1 names "CLI/API/MCP." Recommended: verify the doc
  (or an adjacent one) makes clear the same drift check is reachable via the
  `LifecycleService.CheckDrift` API and the MCP surface — add a short
  API/MCP-access note if missing. No large doc rewrite.

### INTG-01 — Confluence comment polling pagination (`spgr-jwbj`)

- **D-05 (RESOLVED — INTG-01 descoped from Phase 4):** The Confluence
  comment-polling code is **not present in this repository** (confirmed by
  exhaustive scout). The user descoped INTG-01 from Phase 4 on 2026-07-10;
  ROADMAP.md and REQUIREMENTS.md updated accordingly. **Planning ignores INTG-01.**
  Remaining disposition (outside this phase): identify the repository/system that
  owns the `spgr-jwbj` Confluence poller and either re-home INTG-01 there or
  formally defer it. Do **NOT** scaffold a net-new Confluence connector in this
  repo — that is new-capability scope creep, not a pagination bug fix.

### the agent's Discretion (planner/researcher)
- Exact set of verification tests to add vs. confirm-already-covered for DRFT-01
  (D-02) — pick the minimal set that demonstrably closes SC#2, favoring the
  existing e2e Ginkgo patterns.
- Whether D-04's API/MCP-access note lands in `drift.md`, a new snippet, or the
  generated `cli-reference.md` — planner's call.
- Any test-helper/fixture shape for seeding drifted vs. clean vs. non-done specs
  in the full-graph verification.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### DRFT-01 — drift detection (grounding code + docs, all in-repo)
- `internal/drift/drift.go` — the `Engine`, `Backend` interface, `Check`/`checkSpec`
  (content-hash vs `content_hash_at_link` comparison), scope handling, and the
  `interfaces`/`verify` "not yet implemented" stub branches. **The core of DRFT-01.**
- `internal/drift/drift_test.go` — existing 15 unit tests (mock backend). The
  baseline coverage the verification audit (D-02) extends.
- `internal/driftscope/scope.go` — `validScopes` + `IsValid`; the SYNC contract
  with `cmd/specgraph/lifecycle.go`'s `driftScopeToProtoMap` and
  `internal/server/convert.go`'s `driftScopeFromProtoMap`.
- `internal/server/lifecycle_handler.go` — `DriftChecker`/`SpecLinter` interfaces,
  `CheckDrift` + `AcknowledgeDrift` (`LifecycleAcknowledgeDrift` → re-check) RPC
  handlers; `RegisterLifecycleService` wiring.
- `cmd/specgraph/lifecycle.go` — `specgraph drift` + `specgraph drift acknowledge`
  CLI (flags, scope mapping, exit-code logic), rendered via `render.DriftReport`.
- `e2e/api/lifecycle_test.go` ("Drift detection" describe) and
  `e2e/api/lifecycle_pipeline_test.go` — existing real-DB drift e2e; the pattern
  D-02's SC#2 verification should follow/extend.
- `internal/storage/postgres/lifecycle.go` + `internal/storage/postgres/graph.go`
  — `content_hash_at_link` on `DEPENDS_ON` edges, drift-relevant storage ops.
- `internal/constitution/hash/hash.go` — canonical content-hash (Murmur3-128)
  computation drift compares against.
- `site/docs/concepts/drift.md`, `site/docs/cli-reference.md` — existing
  user-facing drift documentation (D-04 audit target).
- **Repo convention (AGENTS.md / CLAUDE.md gotcha):** "`content_hash_at_link` on
  DEPENDS_ON edges" and "empty edge hash always triggers drift — baseline with
  `specgraph drift acknowledge <slug> --all`" — the exact behavior under test.

### INTG-01 — Confluence comment polling (`spgr-jwbj`)
- **No in-repo canonical code found** — this is the blocker (D-05). Reference
  only: `docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`
  (a *template*-bridge design, NOT the poller — confirms Confluence *ingestion*
  is out of scope for the design corpus).
- `internal/sync/adapter.go` + `internal/sync/{beads,github}.go` — the ONLY sync
  adapters that exist; shows there is no Confluence adapter to fix here.

### Prior-phase precedent
- `.planning/phases/01-release-build-tooling/01-CONTEXT.md` — the "requirement
  found already shipped; scope to the true delta" pattern DRFT-01 likely follows.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **Full drift stack already built** (`internal/drift`, `LifecycleService`,
  `specgraph drift` CLI, `render.DriftReport`) — DRFT-01 reuses/verifies this
  rather than building anything new.
- **e2e drift harness** (`e2e/api/lifecycle_test.go`) — real-DB Ginkgo/Gomega
  scaffolding for seeding specs, edges, upstream updates, and asserting drift;
  the natural home for SC#2 verification (false-positive + full-graph cases).
- **`drift_test.go` mock backend** — cheap unit-level scenario construction for
  any additional deterministic cases.

### Established Patterns
- **Three-way scope SYNC** (`driftscope.validScopes` ↔ CLI `driftScopeToProtoMap`
  ↔ server `driftScopeFromProtoMap`) — any scope touch must keep all three in
  sync (there are completeness/sync tests in `lifecycle_test.go`).
- **Only `done`-stage specs are drift-eligible**; single-spec on a non-done
  stage returns `ErrSpecIneligibleForDrift` → `FailedPrecondition`; all-specs
  mode counts the rest as `SkippedCount`. Verification must respect this.
- **Handler error sanitization** (AGENTS.md gotcha) — drift errors are sanitized
  before returning to clients; test assertions use error *codes*, not message
  strings.
- **Testcontainers for Postgres integration** (`pgvector/pgvector:pg18`) — the
  established real-DB verification path; requires Docker (`task pr-prep`).

### Integration Points
- DRFT-01 work stays within `internal/drift/`, its tests, `e2e/api/`, and
  `site/docs/` — no proto/schema changes expected (no new RPCs, no migration).
- INTG-01 has **no in-repo integration point** (D-05 blocker).

</code_context>

<specifics>
## Specific Ideas

- Mirror the Phase-1 discipline: **verify the requirement against the live repo
  before planning build work.** DRFT-01 is very likely mostly-satisfied; the
  value is proving SC#2 (no false-positives on unrelated edits) and confirming
  the interface is documented across CLI/API/MCP — not re-implementing drift.
- Do not let DRFT-01's `--scope interfaces|verify` scope *names* trick the phase
  into building code-level/interface drift detection (that is v2 DRFT-02).
- INTG-01: the honest state is "target code is not here." Prefer surfacing that
  to the user over fabricating a Confluence connector.

</specifics>

<deferred>
## Deferred Ideas

- **DRFT-02 (`spgr-93k`) — code-level drift detection** (watch for repo/code
  changes outside a spec) — the real content behind the `interfaces`/`verify`
  stub scopes. Explicitly v2; out of scope for Phase 4 (D-03).
- **Implementing the `--scope interfaces` and `--scope verify` detectors** —
  deferred with DRFT-02; kept as documented "Planned" stubs this phase.
- **Building a Confluence connector / ingestion pipeline** — a whole new
  integration, not a pagination bug fix. If INTG-01's poller truly does not
  exist in any owned repo, standing up Confluence ingestion is a separate,
  much larger effort (relates to EXPL-02 one-way Confluence export design).

None — discussion stayed within phase scope (auto mode).

</deferred>

---

*Phase: 4-Verification & Integration Reliability*
*Context gathered: 2026-07-10*
