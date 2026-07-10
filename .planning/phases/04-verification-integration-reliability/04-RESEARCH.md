# Phase 4: Verification & Integration Reliability - Research

**Researched:** 2026-07-10
**Domain:** Drift-detection verification (Go / ConnectRPC / pgx / testcontainers / Ginkgo e2e)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01 (requirement interpretation):** Read "Interface and verify drift detection" as
  **"expose drift detection through a stable, documented interface, and *verify* it with
  tests"** — NOT as "implement the stubbed `interfaces`/`verify` drift *scopes*." SC#2 names
  content-hash and `DEPENDS_ON`-edge scenarios = the already-implemented `deps` scope. The
  `--scope interfaces|verify` name collision is coincidental. Code-level/interface/verify drift
  is separately tracked as v2 DRFT-02 (`spgr-93k`). ⚠️ Single most important gray area, auto-answered
  — if intent was actually to build the `interfaces`/`verify` scopes, that is a materially larger
  phase; flag to user before planning.

- **D-02 (scope = verification audit + gap-fill, not a rebuild):** Verify what exists against the
  repo first (Phase-1 REL-01/CFG-01 pattern), then close only the real gaps. Expected concrete deltas:
  1. False-positive verification (SC#2 "doesn't false-positive on unrelated edits") — prove an edit
     to an *unrelated* spec produces **no** drift on the downstream.
  2. Full-graph query verification — empty-slug "check all specs" at e2e/integration (mixed
     drifted + clean + skipped-non-done + `SkippedCount`).
  3. Acknowledge round-trip — acknowledging (single upstream and `--all`) re-baselines
     `content_hash_at_link` and a subsequent check reports clean, end-to-end.
  4. Interface-stability/documentation check — CLI + API + MCP surfaces all documented (D-04).
  - **Prefer real-DB verification (testcontainers/e2e)** over more mock-backend unit tests for SC#2.

- **D-03 (stub scopes stay stubbed — explicitly OUT of scope):** Do NOT implement `--scope
  interfaces` or `--scope verify`. Leave them as documented "Planned / not yet implemented" stubs.
  Keep the `driftscope.validScopes` ↔ `driftScopeToProtoMap` ↔ `driftScopeFromProtoMap` three-way
  SYNC intact if touched.

- **D-04 (documentation = confirm, lightly extend):** `drift.md` + `cli-reference.md` already
  document concept + CLI thoroughly, but the doc is **CLI-centric**; SC#1 names "CLI/API/MCP."
  Verify (or add) a short note that the same drift check is reachable via `LifecycleService.CheckDrift`
  API and the MCP `drift` tool. No large doc rewrite.

- **D-05 (RESOLVED — INTG-01 descoped from Phase 4):** Confluence poller code is not in this repo.
  INTG-01 removed from ROADMAP/REQUIREMENTS. **Planning ignores INTG-01.** Do NOT scaffold a
  Confluence connector here.

### the agent's Discretion (planner/researcher)

- Exact set of verification tests to add vs. confirm-already-covered for DRFT-01 (D-02) — pick the
  minimal set that demonstrably closes SC#2, favoring existing e2e Ginkgo patterns.
- Whether D-04's API/MCP-access note lands in `drift.md`, a new snippet, or generated `cli-reference.md`.
- Any test-helper/fixture shape for seeding drifted vs. clean vs. non-done specs in full-graph verification.

### Deferred Ideas (OUT OF SCOPE)

- **DRFT-02 (`spgr-93k`) — code-level drift detection** — the real content behind the
  `interfaces`/`verify` stub scopes. Explicitly v2 (D-03).
- **Implementing `--scope interfaces` / `--scope verify` detectors** — deferred with DRFT-02.
- **Building a Confluence connector / ingestion pipeline** — a whole new integration, not a bug fix.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DRFT-01 | Interface and verify drift detection (`spgr-vch`) | SC#1 (queryable/documented interface) is **already met** across CLI/API/MCP — see "Is SC#1 already met?" below. SC#2 (verified true-positive + no-false-positive) is **partially met**; the true delta is three targeted verification tests (no-false-positive on unrelated edit, full-graph mixed-state, per-upstream ack round-trip) plus a short API/MCP doc note. No proto/schema/RPC change required. |
</phase_requirements>

## Summary

The dependency-based drift stack is **fully built and shipped** end-to-end — engine
(`internal/drift/drift.go`), API/MCP (`LifecycleService.CheckDrift`/`AcknowledgeDrift` + MCP `drift`
tool), CLI (`specgraph drift [slug] --scope --json` + `specgraph drift acknowledge`), converters,
storage (`content_hash_at_link` on `DEPENDS_ON` edges), and user-facing docs. This exactly mirrors
Phase 1's REL-01/CFG-01 "already shipped" discovery. **DRFT-01 SC#1 (queryable/documented interface)
is already satisfied.** The net-new work is a **verification audit + minimal gap-fill for SC#2**, not
a build. `[VERIFIED: repo inspection]`

The drift mechanism is a pure **content-hash comparison**: each `DEPENDS_ON` edge stores
`content_hash_at_link` (the upstream's `ContentHash` at baseline time); `Engine.checkSpec` flags drift
when `upstream.ContentHash != dep.ContentHashAtLink`. Only `done`-stage specs are eligible; single-spec
checks on a non-done spec return `ErrSpecIneligibleForDrift` → `FailedPrecondition`; all-specs (empty
slug) mode counts non-done specs as `SkippedCount`. The `interfaces`/`verify` scopes are deliberate
"not yet implemented" stubs (v2 DRFT-02) and MUST stay stubbed per D-03. `[VERIFIED: repo inspection]`

Existing test coverage is strong on true-positive and error paths but has **three concrete SC#2 gaps**:
(1) no test proving an edit to an *unrelated* spec does NOT false-positive a downstream — the headline
SC#2 requirement; (2) no e2e/integration full-graph check asserting mixed drifted+clean+skipped state
with `SkippedCount`; (3) per-upstream (`--upstream`) acknowledge round-trip is only proven at the
storage layer, not through the full `CheckDrift → ack → CheckDrift` interface. Plus a one-line doc note
for D-04. `[VERIFIED: repo inspection]`

**Primary recommendation:** Treat this as a **verification-confirmation + doc-only-delta** phase.
Add ~2–3 e2e/integration verification tests (favor the existing `e2e/api/lifecycle_test.go` Ginkgo
pattern) that close the SC#2 gaps, add a short API/MCP access note to `drift.md`, and change nothing in
the drift engine, converters, scope tables, or proto. **No `buf-regen` / proto change is needed** (no
new RPCs, messages, or enum values). `[VERIFIED: repo inspection]`

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Drift detection algorithm (hash compare) | API/Backend (`internal/drift`) | — | Pure server-side graph traversal; compares stored edge hash vs live spec hash |
| Drift storage (`content_hash_at_link`) | Database/Storage (`internal/storage/postgres`) | — | Edge property persisted on `DEPENDS_ON`; refreshed on completion/amend/ack |
| Drift query surface (CLI) | CLI (`cmd/specgraph/lifecycle.go`) | API | Thin ConnectRPC client over `CheckDrift`/`AcknowledgeDrift` |
| Drift query surface (API) | API/Backend (`internal/server/lifecycle_handler.go`) | — | `LifecycleService.CheckDrift`/`AcknowledgeDrift` ConnectRPC handlers |
| Drift query surface (MCP) | API/Backend (`internal/mcp/tools_lifecycle.go`) | API | MCP `drift` tool proxies the same `LifecycleService` RPCs |
| Verification (this phase's real work) | Tests (`internal/drift`, `internal/storage/postgres`, `e2e/api`) | — | SC#2 is a testing/verification deliverable, not a code-behavior change |
| Documentation of the interface | Docs (`site/docs/concepts/drift.md`) | — | SC#1's "CLI/API/MCP" note lives here |

## Project Constraints (from AGENTS.md / CLAUDE.md)

Actionable directives the planner MUST honor (same authority as locked decisions):

- **Quality gates:** run `task check` before every push (fmt:check → license:check → lint → build →
  unit tests, excludes postgres integration/Docker). Run `task pr-prep` before opening/updating a PR
  (check → `test:integration` → `test:e2e`; **requires Docker**). Do NOT rely solely on lefthook pre-commit.
- **License headers required** on all `.go` files: `SPDX-License-Identifier: Apache-2.0` (+ copyright).
  Run `task license:add` to fix. New test files need the header.
- **DCO required** — every commit needs a `Signed-off-by:` trailer (`git commit -s` or `jj describe` trailer).
- **`revive` requires package comments** — only relevant if a *new* package is created (none expected here).
- **jj-colocated repo** — use `jj --no-pager`; never `git push` (use `jj bookmark set` + `jj git push --bookmark`);
  use `jj workspace add` not `git worktree`; pass `-m` to squash/describe/commit/new.
- **Handler error sanitization** — drift errors are sanitized before returning to clients; test assertions
  MUST use error **codes** (`connect.CodeFailedPrecondition`, `connect.CodeNotFound`,
  `connect.CodeInvalidArgument`), not message strings.
- **Mock backends must use sentinel errors** — when handler code uses `errors.Is()` (e.g.
  `storage.ErrSpecNotFound`, `storage.ErrSpecIneligibleForDrift`), fakes must return those sentinels.
- **All multi-query write paths use `RunInTransaction`** (ADR-004) — already honored by
  `LifecycleAcknowledgeDrift`; do not regress if touched.
- **Postgres integration tests require Docker** — testcontainers `pgvector/pgvector:pg18`, wait strategy
  `ForLog("database system is ready").WithOccurrence(2)`.
- **Claude Code hooks:** `task lint` runs after Bash; edits to `gen/` are blocked (edit `.proto` sources).
  No proto edits expected in this phase.

## Standard Stack

No new dependencies. This phase reuses the existing test + runtime stack. `[VERIFIED: repo inspection]`

### Core (existing, reused)
| Library / Tool | Version | Purpose | Why Standard |
|----------------|---------|---------|--------------|
| `github.com/stretchr/testify` | (in go.mod) | Unit assertions (`require`/`assert`) | Convention across `internal/drift`, `internal/server`, `cmd` unit tests |
| `github.com/onsi/ginkgo/v2` + `github.com/onsi/gomega` | (in go.mod) | e2e BDD suites (`//go:build e2e`) | Established `e2e/api/` pattern; real-DB verification |
| `testcontainers-go` (via `e2e/testutil`, `internal/storage/postgres`) | (in go.mod) | Spin up real Postgres for integration/e2e | `pgvector/pgvector:pg18`; the blessed real-DB path |
| `connectrpc.com/connect` | (in go.mod) | RPC client/handler | The API surface under test |

**Installation:** none — `task tools` already provisions everything; no `npm`/`pip`/`cargo` in scope.

## Package Legitimacy Audit

**Not applicable.** This phase installs **no external packages**. All libraries used by the planned
verification tests already exist in `go.mod` and are exercised by the current test suites. No
`npm`/`pip`/`cargo`/`go get` step is expected. `[VERIFIED: repo inspection]`

## Architecture Patterns

### System Architecture Diagram (drift query + verification flow)

```
                         ┌─────────────────────────────────────────────┐
   CLI                   │  cmd/specgraph/lifecycle.go                  │
   `specgraph drift ...` │  runDrift → driftScopeToProto → CheckDrift   │
                         └───────────────┬─────────────────────────────┘
   MCP                                   │
   `drift` tool  ───────────────────────┤  (all three surfaces call the SAME RPC)
   internal/mcp/tools_lifecycle.go       │
                                         ▼
   API                          ┌──────────────────────────────────────┐
   LifecycleService.CheckDrift  │ internal/server/lifecycle_handler.go  │
   / AcknowledgeDrift           │  scopeStore → driftScopeFromProto →   │
                                │  DriftChecker.Check(slug, scope)      │
                                └───────────────┬──────────────────────┘
                                                ▼
                                ┌──────────────────────────────────────┐
                                │ internal/drift/drift.go  Engine.Check │
                                │  slug=="" → ListSpecs(done)+ListSpecs │
                                │            (all) → SkippedCount        │
                                │  per spec → checkSpec:                 │
                                │    GetDependenciesWithEdgeData(slug)   │
                                │    for each dep: GetSpec(upstream)     │
                                │    drift if upstream.ContentHash !=    │
                                │             dep.ContentHashAtLink      │
                                └───────────────┬──────────────────────┘
                                                ▼
                                ┌──────────────────────────────────────┐
                                │ internal/storage/postgres             │
                                │  DEPENDS_ON edge.content_hash_at_link │
                                │  (set on AddEdge / decompose / update;│
                                │   refreshed on completion, amend,     │
                                │   drift acknowledge)                  │
                                └──────────────────────────────────────┘
```

Acknowledge path: `AcknowledgeDrift` handler → `store.LifecycleAcknowledgeDrift` (RunInTransaction:
refresh edge hash to upstream's current `content_hash`, write changelog) → **re-runs `Check`** to return
the now-clean report. `[VERIFIED: repo inspection]`

### Pattern 1: e2e drift verification (Ginkgo, real DB) — the primary SC#2 vehicle
**What:** An `Ordered` `Describe` block that creates specs via `specClient.CreateSpec`, advances them
with the `advanceStage(ctx, slug, "done")` helper, wires a `DEPENDS_ON` edge via
`graphClient.AddEdge`, mutates upstream with `specClient.UpdateSpec`, and asserts on
`lifecycleClient.CheckDrift`.
**When to use:** All SC#2 cases D-02 flags as "prefer real-DB."
**Example (existing true-positive, the template to extend):**
```go
// Source: e2e/api/lifecycle_test.go "Drift detection" (lines 230–310)
It("updates upstream to trigger drift", func() {
    time.Sleep(timestampSkew)            // 1200ms — guarantee timestamp ordering
    _, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
        Slug:   upstreamSlug,
        Intent: proto.String("Updated upstream intent to trigger drift"),
    }))
    Expect(err).NotTo(HaveOccurred())
})
It("detects drift on downstream spec", func() {
    // retry up to 3x for second-precision timestamp races
    ...
    resp, _ := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{Slug: downstreamSlug}))
    // assert resp.Msg.Reports[0].Items non-empty
})
```

### Pattern 2: integration test (testcontainers, storage layer) — for round-trip re-baseline
**What:** `//go:build integration`, `package postgres_test`, `newStore(t)` + `clearDatabase(t, store)`,
direct `store.*` calls; assert `GetDependenciesWithEdgeData(...)[i].ContentHashAtLink`.
**When to use:** Storage-level re-baseline assertions (per-upstream and `--all`). Already partly covered.
**Example:**
```go
// Source: internal/storage/postgres/lifecycle_test.go "AcknowledgeDrift_Basic" (lines 391–423)
err = store.LifecycleAcknowledgeDrift(ctx, "ack-drift", "ack-upstream", "drift is intentional")
require.NoError(t, err)
deps, _ := store.GetDependenciesWithEdgeData(ctx, "ack-drift")
require.Equal(t, upstream.ContentHash, deps[0].ContentHashAtLink)
```

### Pattern 3: unit test (mock backend, no DB) — for cheap deterministic scenarios
**What:** `package drift_test`, no build tag, `mockDriftBackend` with `specs`/`depsWithEdge` maps,
`drift.NewEngine(backend, nil)`.
**When to use:** Deterministic engine-logic cases; runs in `task test`. Good as a fast mirror of the
no-false-positive case, but D-02 wants the real-DB version to be the authoritative SC#2 proof.

### Anti-Patterns to Avoid
- **Implementing the `interfaces`/`verify` scopes** — explicitly out of scope (D-01/D-03). The scope
  names are a coincidence.
- **Adding a new RPC / proto field / migration** — unnecessary; the interface already exists. Any proto
  edit triggers `buf-regen`, blocked `gen/` edits, and scope creep.
- **Asserting on sanitized error message strings** — assert on `connect.CodeOf(err)` instead.
- **Reusing another `Describe` block's specs in a new full-graph test** — the existing "Drift detection
  (all specs)" block runs after a blanket ack and expects zero drift. A new full-graph test must seed its
  own uniquely-named specs (or be ordered carefully) so it does not collide with that expectation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Real Postgres for a drift test | A local pg install / SQL mock | `e2e/testutil.StartPostgres` + testcontainers `pgvector/pg18` | Established, CI-consistent, Docker-managed |
| Advancing a spec to `done` in e2e | Manual RPC sequence per test | `advanceStage(ctx, slug, "done")` helper | Handles spark→shape→specify→decompose→approve→claim→complete |
| Claim+complete to done | Hand-rolled claim/report calls | `claimAndComplete(ctx, slug)` helper | Same reason |
| Content hashing | Any new hash | existing spec `ContentHash` / `internal/constitution/hash` (Murmur3-128) | Drift compares the canonical hash already computed by storage |
| Scope enum mapping | New map | `driftscope.validScopes` + `driftScopeToProtoMap` + `driftScopeFromProtoMap` | Three-way SYNC already tested; do not add a fourth |

**Key insight:** Everything drift needs already exists. The phase's job is to *prove* it, not extend it.

## Runtime State Inventory

> This is a verification/test/doc phase, not a rename/refactor/migration. No runtime-state migration is
> implied. For completeness against D-02's "verify against the live repo" mandate:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `content_hash_at_link` text column on `edges` (verified present: `e2e/api/schema_validation_test.go` asserts the column is `text`). Populated for `DEPENDS_ON` edges. | None — read/verify only |
| Live service config | None — drift is in-process; no external service holds drift state | None |
| OS-registered state | None | None |
| Secrets/env vars | None referenced by drift | None |
| Build artifacts | None — no proto change, so `gen/` unchanged; no package rename | None |

**Nothing to migrate:** verified by inspection of `internal/drift`, `internal/storage/postgres`, and the
schema-validation e2e test.

## Common Pitfalls

### Pitfall 1: Content-hash / timestamp second-precision races in drift e2e tests
**What goes wrong:** A test updates an upstream then immediately checks drift and sees no drift.
**Why it happens:** The existing e2e comments note drift is sensitive to `updated_at` at second
precision (the spec `ContentHash` changes on update, but timestamp-adjacent writes can land in the same
second window). `[CITED: e2e/api/lifecycle_test.go lines 21–25, 256–285]`
**How to avoid:** Replicate the existing guard — sleep `timestampSkew` (1200ms) before the mutating
update, and wrap the drift assertion in the 3-attempt retry loop already used in the "detects drift"
spec. Any new mutate-then-check drift test MUST follow this pattern.
**Warning signs:** Intermittent CI flakes on a freshly-added drift spec.

### Pitfall 2: The "unrelated edit" must actually be unrelated AND actually change a hash
**What goes wrong:** A no-false-positive test that (a) edits a spec that is secretly upstream, or (b)
edits a spec whose hash does not change, proves nothing.
**Why it happens:** To be a meaningful SC#2 proof the test needs: downstream `DEPENDS_ON` upstream; a
**third** spec `unrelated` with no edge to downstream; edit `unrelated` (with a real intent change so its
`ContentHash` changes); assert `CheckDrift(downstream)` returns **no** items AND (ideally) that
`CheckDrift(unrelated)` also stays clean (it has no upstreams).
**How to avoid:** Seed three distinct done specs; only wire the downstream→upstream edge; mutate the
third; assert clean downstream.
**Warning signs:** Test passes even if you delete the edge — means it wasn't testing the edge path.

### Pitfall 3: Breaking the three-way scope SYNC
**What goes wrong:** Touching one scope table without the others fails `TestDriftScopeToProtoMap_Completeness`,
`TestDriftScopeToProtoMap_SyncWithDriftscope`, or `TestDriftScopeFromProtoMap_SyncWithDriftscope`.
**Why it happens:** `driftscope.validScopes` (engine) ↔ `driftScopeToProtoMap` (CLI) ↔
`driftScopeFromProtoMap` (server) must all agree. `[VERIFIED: repo inspection]`
**How to avoid:** D-03 says don't touch scopes. If anything is touched, update all three + keep the tests.
**Warning signs:** A sync/completeness test failing in `task check`.

### Pitfall 4: `done`-stage eligibility & SkippedCount semantics
**What goes wrong:** A full-graph test asserts on the wrong report/skip counts.
**Why it happens:** Single-spec check on a non-done spec → `ErrSpecIneligibleForDrift` →
`FailedPrecondition`. All-specs mode silently **skips** non-done specs and reports `SkippedCount =
len(allSpecs) - len(doneSpecs)`. Reports with zero items are filtered out of the response.
`[VERIFIED: internal/drift/drift.go lines 74–121]`
**How to avoid:** In a mixed-state full-graph test, seed at least one drifted-done, one clean-done, and
one non-done spec, and assert `SkippedCount >= 1` plus exactly the drifted spec appears in `Reports`.

### Pitfall 5 (documentation accuracy, optional): misleading "timestamps" comment
The e2e comment "Drift detection compares `updated_at` timestamps" (lines 257–259) is **imprecise** — the
engine compares `ContentHash`, not timestamps directly. Optional: correct the comment while adding tests.
Not required by SC#1/SC#2.

## Code Examples

### Verify SC#1 surfaces exist (reference, no new code)
```go
// API — internal/server/lifecycle_handler.go:140  CheckDrift(...) — DriftCheckRequest{Slug, Scope}
// MCP — internal/mcp/tools_lifecycle.go:50  driftTool "drift" (actions: check, acknowledge)
// CLI — cmd/specgraph/lifecycle.go:123  driftCmd  "specgraph drift [slug] --scope --json"
```

### Full-graph mixed-state assertion shape (planned new e2e/integration test)
```go
// pattern derived from internal/drift/drift_test.go TestCheckAllSpecs + e2e helpers
// seed: drifted-done (edge to mutated upstream), clean-done (edge to untouched upstream), one non-done
resp, _ := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{})) // empty slug = all
Expect(resp.Msg.GetSkippedCount()).To(BeNumerically(">=", int32(1))) // the non-done spec
// exactly the drifted spec appears with items; clean spec is filtered out (zero items)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| (n/a — drift shipped incrementally) | Content-hash drift on `DEPENDS_ON` edges | commit `b0752f1a` (#43) | The mechanism SC#2 verifies |
| all-specs drift with no skip accounting | `SkippedCount` on all-specs check | commit `6d739e1d` (#810, `spgr-col`) | Full-graph test must assert `SkippedCount` |

**Deprecated/outdated:** none relevant. The `interfaces`/`verify` scopes are **planned stubs**, not
deprecated — leave as-is (D-03).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | D-01's interpretation (verify-the-`deps`-interface, not build the stub scopes) is the intended reading of DRFT-01. | User Constraints / Summary | HIGH — if the intent was to build `interfaces`/`verify` detectors, this is a materially larger phase. CONTEXT.md flags this as the key auto-answered gray area; confirm with user before planning. |
| A2 | No proto/RPC/migration change is needed. | Summary / Anti-Patterns | MEDIUM — if a new field (e.g. structured drift metadata) is later desired, `buf-regen` re-enters scope. Current SC#1/SC#2 need none. |

**Note:** A1 is a restatement of CONTEXT.md D-01's own ⚠️ — surface it to the user at plan-review.

## Open Questions (RESOLVED)

1. **Should the no-false-positive proof be e2e, integration, or both?**
   - What we know: D-02 says "prefer real-DB verification" for SC#2. e2e (`e2e/api/lifecycle_test.go`)
     is the richest real-DB harness; a storage-level integration test is cheaper but doesn't exercise
     the RPC surface.
   - What's unclear: whether one e2e test suffices or a mirrored unit test is also wanted for fast `task test`.
   - Recommendation: **one e2e test** (authoritative, exercises CLI/API/MCP-equivalent path) + optionally a
     cheap `drift_test.go` unit mirror. Planner's discretion (D-02 discretion clause).
   - **RESOLVED:** 04-01-PLAN.md — Task 1 delivers the e2e no-false-positive spec; Task 2 delivers the
     full-graph `SkippedCount` proof at the integration layer (isolated `clearDatabase` per subtest) to
     avoid colliding with the existing "Drift detection (all specs)" blanket-zero assertion.

2. **Where does the D-04 API/MCP note land?**
   - What we know: `site/docs/concepts/drift.md` is CLI-only; `site/docs/architecture.md:51` already says
     LifecycleService does drift; the MCP `drift` tool exists but is undocumented in drift.md.
   - Recommendation: add a short "## Accessing drift via API / MCP" subsection to `drift.md` naming
     `LifecycleService.CheckDrift`/`AcknowledgeDrift` and the MCP `drift` tool (actions `check`,
     `acknowledge`). No `cli-reference.md` regeneration needed. Planner's call (D-04 discretion).
   - **RESOLVED:** 04-02-PLAN.md — adds the "Accessing Drift via API / MCP" section to `drift.md`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker | `task test:integration`, `task test:e2e` (testcontainers) | ✓ (assumed; standard dev/CI) | — | None — real-DB SC#2 tests cannot run without it; unit (`task test`) still runs |
| Go toolchain | build + all tests | ✓ | per go.mod | None |
| `task` (Taskfile.dev) | quality gates | ✓ | — | Raw `go test -tags ...` |

**Missing dependencies with no fallback:** Docker is required to *run* the SC#2 real-DB verification
(integration/e2e). If Docker is unavailable in the execution environment, the phase can still author the
tests and run `task test`, but the D-02-preferred real-DB proof must run under `task pr-prep` where Docker
is present. Planner should place the real-DB verification behind the `pr-prep`/CI gate, not the unit gate.

## Validation Architecture

> `workflow.nyquist_validation` is not set in `.planning/config.json` → treated as **enabled**.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` (unit), Ginkgo/Gomega v2 (e2e), testcontainers (integration/e2e) |
| Config file | `Taskfile.yml` (test targets); no separate framework config |
| Quick run command | `task test` (unit; excludes `//go:build integration` and `//go:build e2e`) |
| Full suite command | `task pr-prep` (check → `task test:integration` → `task test:e2e`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DRFT-01 | True drift flagged when upstream changes | unit + e2e | `task test` / `task test:e2e:api` | ✅ (`drift_test.go` TestCheckDependencyDrift; e2e "Drift detection") |
| DRFT-01 | **No false-positive on unrelated edit** | integration/e2e | `task test:e2e:api` (`go test -tags e2e ./e2e/api/...`) | ❌ **Wave 0 — add** |
| DRFT-01 | Full-graph mixed drifted/clean/skipped + `SkippedCount` | unit ✅ / e2e ❌ | `task test:e2e:api` | ⚠️ unit only (`TestCheckAllSpecs`); **add e2e/integration** |
| DRFT-01 | Acknowledge `--all` round-trip → clean | e2e | `task test:e2e:api` | ✅ (e2e "acknowledges drift for all upstreams" + "returns no drift after") |
| DRFT-01 | Acknowledge `--upstream` (single) round-trip → clean via interface | integration ✅ / e2e ❌ | `task test:e2e:api` | ⚠️ storage-only (`AcknowledgeDrift_Basic`); **optional e2e mirror** |
| DRFT-01 | Non-done single-spec → `FailedPrecondition` | unit + e2e | `task test` / `task test:e2e:api` | ✅ (`TestCheck_NonDoneStageBySlug`; e2e error paths) |
| DRFT-01 | Scope SYNC intact | unit | `task test` | ✅ (`TestDriftScope*Map_Sync*`, `_Completeness`) |
| DRFT-01 | API/MCP surfaces documented | manual/doc | review `site/docs/concepts/drift.md` | ⚠️ **doc gap — add API/MCP note (D-04)** |

### Sampling Rate
- **Per task commit:** `task test` (unit — fast, no Docker).
- **Per wave merge / pre-PR:** `task pr-prep` (integration + e2e, Docker) — where the new SC#2 real-DB tests run.
- **Phase gate:** `task pr-prep` green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `e2e/api/lifecycle_test.go` — add a **no-false-positive-on-unrelated-edit** spec (three specs; edit
      the unrelated one; assert downstream clean). Covers the headline SC#2 requirement. `[VERIFIED gap]`
- [ ] Full-graph mixed-state coverage at real-DB level — extend `e2e/api/lifecycle_test.go` (or an
      `//go:build integration` test in `internal/storage/postgres`) asserting `SkippedCount >= 1` with a
      drifted-done + clean-done + non-done seed. `[VERIFIED gap]`
- [ ] (Optional) e2e mirror of per-upstream (`--upstream`) acknowledge round-trip through `CheckDrift`.
- [ ] Documentation: API/MCP access note in `site/docs/concepts/drift.md` (D-04).
- [ ] No new framework install required — Ginkgo/testcontainers/testify already present.

*(Existing infrastructure covers all true-positive, error-path, scope-sync, and `--all` ack cases; only
the three items above are net-new.)*

## Security Domain

> `security_enforcement` is not set in `.planning/config.json` → treated as **enabled**. This phase is
> verification + docs over an existing read-mostly feature; no new attack surface is introduced.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Unchanged — drift RPCs sit behind the existing auth stack (Phases 2–3) |
| V3 Session Management | no | Unchanged |
| V4 Access Control | yes (existing) | Requests are project-scoped via `scopeStore(ctx, h.scoper)` in every lifecycle handler; tests use `X-Specgraph-Project` header. No change. |
| V5 Input Validation | yes (existing) | `validateSlug` on `Slug`/`UpstreamSlug`; note length capped at `maxFieldLen`; scope validated via enum map. No change. |
| V6 Cryptography | no | Content hash (Murmur3-128) is a change-detection fingerprint, **not** a security primitive — do not treat as such. |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cross-project drift read/ack | Information disclosure / Tampering | Existing project scoping (`scopeStore`); do not weaken it when adding tests |
| Error message leakage | Information disclosure | Existing `sanitizeDriftError` + code-based error mapping; assert on codes in tests |
| Injection via slug/upstream_slug | Tampering | Existing `validateSlug`; parameterized pgx queries in `LifecycleAcknowledgeDrift` |

**No new security controls required.** The phase must not regress the existing project-scoping or
error-sanitization guarantees; the added tests should keep asserting on `connect` error codes.

## Sources

### Primary (HIGH confidence — direct repo inspection this session)
- `internal/drift/drift.go`, `internal/drift/drift_test.go` — engine + 15 unit tests
- `internal/server/lifecycle_handler.go`, `internal/server/convert_drift.go`, `internal/server/convert_test.go`
- `cmd/specgraph/lifecycle.go`, `cmd/specgraph/lifecycle_test.go` — CLI + scope-sync/completeness tests
- `internal/mcp/tools_lifecycle.go`, `internal/mcp/tools_lifecycle_test.go` — MCP `drift` tool
- `internal/driftscope/scope.go` — three-way scope SYNC contract
- `internal/storage/postgres/lifecycle.go` (`LifecycleAcknowledgeDrift`), `internal/storage/postgres/lifecycle_test.go`
  (integration ack tests), `internal/storage/postgres/graph_test.go`, `execution_test.go`
- `e2e/api/lifecycle_test.go`, `e2e/api/lifecycle_pipeline_test.go`, `e2e/api/helpers_test.go`,
  `e2e/api/api_suite_test.go`, `e2e/api/schema_validation_test.go`
- `internal/constitution/hash/hash.go` — Murmur3-128 content hash
- `site/docs/concepts/drift.md`, `site/docs/cli-reference.md`, `site/docs/architecture.md` — user-facing docs
- `Taskfile.yml` test targets; `git log` on `internal/drift/`
- `.planning/phases/04-verification-integration-reliability/04-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `.planning/STATE.md`

### Secondary (MEDIUM confidence)
- CLAUDE.md / AGENTS.md project gotchas (quality gates, jj, testcontainers, error sanitization)

### Tertiary (LOW confidence)
- None — no external/web sources needed; this is an internal verification phase.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new deps; all test frameworks already in `go.mod` and in active use.
- Architecture / current-state: HIGH — every claim verified against repo source this session.
- SC#2 gap analysis: HIGH — gaps confirmed by reading the full existing unit/integration/e2e suites.
- D-01 interpretation risk (A1): flagged — the only open decision; needs user confirmation (per CONTEXT.md).

**Research date:** 2026-07-10
**Valid until:** ~2026-08-09 (stable internal code; refresh if the drift engine or e2e harness changes)
