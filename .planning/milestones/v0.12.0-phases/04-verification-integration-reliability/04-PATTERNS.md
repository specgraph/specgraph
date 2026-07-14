# Phase 4: Verification & Integration Reliability - Pattern Map

**Mapped:** 2026-07-10
**Files analyzed:** 3 (1 required e2e test, 1 optional integration test, 1 doc)
**Analogs found:** 3 / 3 (all exact — every planned change extends an existing sibling in the same file)

> Scope note: DRFT-01 only (INTG-01 descoped, D-05). This is a **verification +
> doc** phase over an already-shipped drift interface. There are **no net-new
> source files** — every change is an addition to an existing test file or doc,
> and each has an in-file analog to copy verbatim. Do **not** touch the drift
> engine, converters, scope tables, or proto (A2 / D-03).

---

## File Classification

| Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---------------|------|-----------|----------------|---------------|
| `e2e/api/lifecycle_test.go` | test (e2e / Ginkgo, `//go:build e2e`) | request-response (ConnectRPC over real DB) | Same file — the `Describe("Drift detection", ...)` block (lines 230–310) + `Describe("Drift detection (all specs)", ...)` (lines 388–397) | **exact** (extend sibling `Describe` blocks) |
| `internal/storage/postgres/lifecycle_test.go` *(optional)* | test (integration / testify, `//go:build integration`) | CRUD (direct storage calls, testcontainers) | Same file — `t.Run("AcknowledgeDrift_Basic", ...)` (lines 391–423) + `AcknowledgeDrift_AllUpstreams` (425–472) | **exact** (extend sibling `t.Run` subtests) |
| `site/docs/concepts/drift.md` | docs (Markdown, MkDocs) | N/A | Same file — `## CLI Usage` section (lines 54–88) | **exact** (add a sibling `##` section) |

**Planned additions (from RESEARCH Wave 0 Gaps):**
1. `e2e/api/lifecycle_test.go` — **no-false-positive-on-unrelated-edit** spec (REQUIRED, headline SC#2).
2. `e2e/api/lifecycle_test.go` — **full-graph mixed-state / `SkippedCount`** spec (REQUIRED) — *or* the integration alternative in item 4.
3. `e2e/api/lifecycle_test.go` — **per-upstream (`--upstream`) acknowledge round-trip** e2e (OPTIONAL mirror; storage-level already covered).
4. `internal/storage/postgres/lifecycle_test.go` — **full-graph `SkippedCount`** integration test (OPTIONAL alternative to 2, if e2e seeding is too collision-prone).
5. `site/docs/concepts/drift.md` — **API / MCP access note** (REQUIRED, D-04).

---

## Pattern Assignments

### `e2e/api/lifecycle_test.go` — add no-false-positive spec (test, request-response)

**Analog:** `e2e/api/lifecycle_test.go` → `Describe("Drift detection", ...)` (lines 230–310) and helpers in `e2e/api/helpers_test.go`.

**File header / build tag / imports** (lines 1–19) — new specs live in the same file, so **no new imports needed**; reuse the existing:
```go
//go:build e2e

package api_test

import (
	"context"
	"time"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)
```

**Clients + ctx are already wired** by the outer `Describe("Lifecycle", Ordered, ...)` `BeforeAll` (lines 35–40): `lifecycleClient`, `specClient`, `graphClient`, `ctx`. A new `Describe` block inside that container inherits them — **do not re-create clients**.

**Seed → advance-to-done → DEPENDS_ON edge pattern** (copy from lines 236–254):
```go
It("creates two specs, advances to done, and adds a dependency", func() {
	for _, slug := range []string{upstreamSlug, downstreamSlug} {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   slug,
			Intent: "Drift detection test spec " + slug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(advanceStage(ctx, slug, "done")).To(Succeed())   // helper: spark→…→done
	}
	// downstream DEPENDS_ON upstream — baselines content_hash_at_link
	_, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
		FromSlug: downstreamSlug,
		ToSlug:   upstreamSlug,
		EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	}))
	Expect(err).NotTo(HaveOccurred())
})
```

**Timestamp-skew mutate pattern** (copy from lines 256–267) — MANDATORY on any mutate-then-check drift path (Pitfall 1):
```go
time.Sleep(timestampSkew)   // const = 1200ms (line 25); guarantees updated_at ordering
_, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
	Slug:   someSlug,
	Intent: proto.String("Updated intent to change its content hash"),
}))
Expect(err).NotTo(HaveOccurred())
```

**Assert-drift with retry pattern** (copy from lines 269–286) — for the NO-drift assertion, invert it (assert items stay empty across the window). For the unrelated-edit test:
```go
// Seed THREE done specs: downstream DEPENDS_ON upstream; `unrelated` has NO edge.
// Mutate ONLY `unrelated` (with a real intent change → its ContentHash changes).
// Assert downstream stays clean.  [Pitfall 2: the edit must be truly unrelated AND change a hash]
resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
	Slug: downstreamSlug,
}))
Expect(err).NotTo(HaveOccurred())
if len(resp.Msg.Reports) > 0 {
	Expect(resp.Msg.Reports[0].Items).To(BeEmpty(),
		"editing an unrelated spec must NOT drift the downstream")
}
```
> Sanity guard (Pitfall 2 "warning sign"): the test should FAIL if the edge is deleted —
> i.e. keep the downstream→upstream edge in the seed so the path is genuinely exercised.

**Error assertions use connect codes, never message strings** (pattern throughout `Describe("Error paths", ...)`, e.g. lines 414, 436, 510, 581):
```go
Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
// codes in play: CodeFailedPrecondition (non-done), CodeNotFound, CodeInvalidArgument
```

**Ordering / collision caveat** (Anti-Pattern in RESEARCH, and lines 386–397): the existing
`Describe("Drift detection (all specs)")` runs AFTER a blanket ack and asserts **zero** drift
across ALL specs. A new full-graph test that seeds a *drifted* spec MUST use **uniquely-named
specs** and must not leave un-acked drift that the "(all specs)" block would then see. Safest:
place the new full-graph `Describe` so its drifted spec is either self-contained-asserted-then-acked,
or seed names that the later blanket assertion tolerates. Prefer the integration alternative (below)
if this ordering is fragile.

---

### `e2e/api/lifecycle_test.go` — add full-graph mixed-state / `SkippedCount` spec (test, request-response)

**Analog:** the all-specs check shape at lines 388–397 + the seed/advance helpers above.

**Empty-slug = all-specs; assert `SkippedCount`** (proto: `DriftCheckResponse.skipped_count`, field 2):
```go
// Seed: drifted-done (edge → mutated upstream), clean-done (edge → untouched upstream),
//       and at least one NON-done spec (leave at spark/approved) → counted as skipped.
resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{})) // empty slug
Expect(err).NotTo(HaveOccurred())
Expect(resp.Msg.GetSkippedCount()).To(BeNumerically(">=", int32(1)), "the non-done spec is skipped")
// Exactly the drifted spec appears with items; clean-done spec is filtered out (zero-item reports dropped).
```
> Semantics (Pitfall 4, `DriftReport` proto doc lines 65–77): all-specs mode **skips** non-done
> specs (`skipped_count = len(all) - len(done)`); reports with zero items are omitted from the
> response; a single-spec check on a non-done spec instead errors `FailedPrecondition`.

---

### `e2e/api/lifecycle_test.go` — (OPTIONAL) per-upstream acknowledge round-trip (test, request-response)

**Analog:** the ack + re-check pair at lines 288–309 (which uses `All: true`). For per-upstream, swap
`All` for `UpstreamSlug` (proto: `DriftAcknowledgeRequest.upstream_slug`, field 3):
```go
It("acknowledges a single upstream and re-checks clean", func() {
	resp, err := lifecycleClient.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug:         downstreamSlug,
		Note:         "Reviewed single upstream change",
		UpstreamSlug: upstreamSlug,   // per-upstream (mutually exclusive with All)
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Msg.Report.SpecSlug).To(Equal(downstreamSlug))

	// Re-check: after per-upstream ack, that edge's hash matches → no items.
	check, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{Slug: downstreamSlug}))
	Expect(err).NotTo(HaveOccurred())
	if len(check.Msg.Reports) > 0 {
		Expect(check.Msg.Reports[0].Items).To(BeEmpty())
	}
})
```

---

### `internal/storage/postgres/lifecycle_test.go` — (OPTIONAL) full-graph `SkippedCount` integration test (test, CRUD)

**Analog:** `t.Run("AcknowledgeDrift_Basic", ...)` (lines 391–423) and `AcknowledgeDrift_AllUpstreams` (425–472).

**Build tag / package / imports** (lines 1–15) — reuse in-file; **no new imports**:
```go
//go:build integration

package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)
```

**Per-subtest store + clear pattern** (copy from lines 392–394) — every `t.Run` re-seeds:
```go
store := newStore(t)
clearDatabase(t, store)
ctx := context.Background()
```

**Seed → edge → advance-to-done → mutate-upstream pattern** (copy from lines 396–410):
```go
_, err := store.CreateSpec(ctx, "ack-upstream", "Upstream", "p1", "medium",
	storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
require.NoError(t, err)
_, err = store.CreateSpec(ctx, "ack-drift", "Test spec", "p1", "medium",
	storage.SpecProvenanceAuthored, storage.SpecProvenanceDetail{}, nil, nil, nil, nil)
require.NoError(t, err)
_, err = store.AddEdge(ctx, "ack-drift", "ack-upstream", storage.EdgeTypeDependsOn) // baselines edge hash
require.NoError(t, err)
doneStage := "done"
_, err = store.UpdateSpec(ctx, "ack-drift", nil, &doneStage, nil, nil, nil)          // downstream → done (eligible)
require.NoError(t, err)
newIntent := "Changed upstream"
_, err = store.UpdateSpec(ctx, "ack-upstream", &newIntent, nil, nil, nil, nil)        // mutate upstream → drift
require.NoError(t, err)
```
> For a `SkippedCount` integration test, drive `drift.NewEngine(store, ...)` `Check(ctx, "", scope)`
> with an empty slug (see `internal/drift/drift_test.go` `TestCheckAllSpecs` for the engine-level
> shape), seeding a drifted-done + clean-done + one non-done spec, and assert the returned skipped
> count / reports. No timestamp-skew needed here — storage ops are ordered by call, and
> `content_hash` changes deterministically on `UpdateSpec` (no second-precision race like the e2e path).

**Edge-hash re-baseline assertion** (copy from lines 418–422):
```go
err = store.LifecycleAcknowledgeDrift(ctx, "ack-drift", "ack-upstream", "drift is intentional")
require.NoError(t, err)
deps, err := store.GetDependenciesWithEdgeData(ctx, "ack-drift")
require.NoError(t, err)
require.Len(t, deps, 1)
require.Equal(t, upstream.ContentHash, deps[0].ContentHashAtLink) // re-baselined
```

**Sentinel-error assertions** (copy from lines 485, 495): storage returns sentinels — assert with `ErrorIs`:
```go
require.ErrorIs(t, err, storage.ErrSpecIneligibleStage)  // non-done spec
require.ErrorIs(t, err, storage.ErrSpecNotFound)          // missing spec
```

---

### `site/docs/concepts/drift.md` — add API / MCP access note (docs)

**Analog:** the existing `## CLI Usage` section (lines 54–88) — mirror its heading level, fenced-block
style, and the `!!! info "Planned"` admonition idiom.

**Add a sibling `##` section** (D-04; recommended placement: right after `## CLI Usage`, before
`## Worked Example`). Keep it short — name the same-RPC-behind-all-surfaces fact:
```markdown
## Accessing Drift via API / MCP

The same drift check backing the CLI is reachable directly:

- **API (ConnectRPC):** `LifecycleService.CheckDrift` (single spec via `slug`, or
  all eligible specs when `slug` is empty) and `LifecycleService.AcknowledgeDrift`
  (`upstream_slug` for one edge, or `all: true` to re-baseline every drifted edge).
- **MCP:** the `drift` tool exposes the same operations via `check` and
  `acknowledge` actions.

All three surfaces (CLI, API, MCP) call the identical server-side detection —
content-hash comparison on `DEPENDS_ON` edges — so results are consistent
regardless of entry point.
```
> Keep the `!!! info "Planned"` note about `--scope interfaces|verify` intact (lines 81–87) — those
> stubs stay documented as Planned (D-03). No `cli-reference.md` regeneration required.

---

## Shared Patterns

### Project scoping (V4 Access Control)
**Source:** `e2e/api/helpers_test.go` lines 19–41, 71–73
**Apply to:** every e2e client — already handled by the shared client constructors; do not weaken.
```go
const e2eProject = "e2e-test"
// projectClient() injects the X-Specgraph-Project header via a RoundTripper:
req.Header.Set("X-Specgraph-Project", t.project)
func newLifecycleClient() specgraphv1connect.LifecycleServiceClient {
	return specgraphv1connect.NewLifecycleServiceClient(projectClient(), serverInfo.BaseURL)
}
```
Every new e2e spec uses the package-level `lifecycleClient`/`specClient`/`graphClient` created in the
outer `BeforeAll` — these already carry the project header. No per-test scoping code needed.

### Advance-to-done helper (don't hand-roll the funnel)
**Source:** `e2e/api/helpers_test.go` lines 83–201 (`advanceStage`), 203–229 (`claimAndComplete`)
**Apply to:** all e2e drift seeds (specs must be `done` to be drift-eligible).
```go
Expect(advanceStage(ctx, slug, "done")).To(Succeed())   // spark→shape→specify→decompose→approve→claim→complete
```
Valid targets: `"shape" | "specify" | "decompose" | "approved" | "in_progress" | "done"`.
Use `"in_progress"` or `"approved"` (NOT `"done"`) to seed the **skipped** (non-done) spec in the
full-graph test.

### Error assertions on connect codes, never strings (handler error sanitization)
**Source:** `e2e/api/lifecycle_test.go` `Describe("Error paths")` (lines 399–583); RESEARCH lines 124–126
**Apply to:** all e2e/handler-level negative assertions.
```go
Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
```
Storage-level (integration) negatives use sentinel `require.ErrorIs(t, err, storage.Err…)` instead.

### Timestamp-skew guard for mutate-then-check drift (e2e only)
**Source:** `e2e/api/lifecycle_test.go` line 25 (`const timestampSkew = 1200ms`), lines 256–286
**Apply to:** any e2e test that updates a spec then checks drift. `time.Sleep(timestampSkew)` before the
mutating `UpdateSpec`, and wrap the drift read in the 3-attempt retry loop (lines 272–285). The
integration/storage path does NOT need this (deterministic hash change per call).

### Three-way scope SYNC — DO NOT TOUCH (D-03 / Pitfall 3)
**Source:** `internal/driftscope/scope.go` (`validScopes`) ↔ `cmd/specgraph/lifecycle.go` (`driftScopeToProtoMap`)
↔ `internal/server/convert.go` (`driftScopeFromProtoMap`)
**Apply to:** N/A — verification phase must not modify scopes. If any scope table is touched, update all
three and keep `TestDriftScope*Map_Sync*` / `_Completeness` green.

---

## No Analog Found

None. Every planned change extends an existing sibling construct in the same file:

| File | Change | In-file analog |
|------|--------|----------------|
| `e2e/api/lifecycle_test.go` | no-false-positive spec | `Describe("Drift detection")` L230–310 |
| `e2e/api/lifecycle_test.go` | full-graph `SkippedCount` spec | `Describe("Drift detection (all specs)")` L388–397 |
| `e2e/api/lifecycle_test.go` | per-upstream ack (optional) | ack+re-check L288–309 |
| `internal/storage/postgres/lifecycle_test.go` | `SkippedCount` integration (optional) | `AcknowledgeDrift_Basic` L391–423 |
| `site/docs/concepts/drift.md` | API/MCP note | `## CLI Usage` L54–88 |

---

## Metadata

**Analog search scope:** `e2e/api/`, `internal/storage/postgres/`, `site/docs/concepts/`, `proto/specgraph/v1/`
**Files scanned (read in full):** `e2e/api/lifecycle_test.go`, `e2e/api/helpers_test.go`,
`internal/storage/postgres/lifecycle_test.go`, `site/docs/concepts/drift.md`, `proto/specgraph/v1/lifecycle.proto` (drift messages)
**Upstream inputs:** `04-CONTEXT.md`, `04-RESEARCH.md`
**Pattern extraction date:** 2026-07-10
