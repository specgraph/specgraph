# Steel Thread Decomposition Strategy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `DECOMPOSITION_STRATEGY_STEEL_THREAD` to the decompose stage — a thin vertical slice that proves out the riskiest integration points, with all subsequent slices depending on it.

**Architecture:** New proto enum value (4), server-side topology validation (single root + reachability), domain constant, mapping updates in both directions, and skill/reference doc updates. No new messages or fields.

**Tech Stack:** Protobuf, Go (ConnectRPC handlers, testify), SpecGraph decompose skill (Markdown)

**Spec:** `docs/superpowers/specs/2026-04-03-steel-thread-decomposition-design.md`

---

## Task 1: Proto Enum + Code Generation

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto:49-57`
- Regenerate: `gen/specgraph/v1/authoring.pb.go` (via `task proto`)

- [ ] **Step 1: Add STEEL_THREAD enum value**

In `proto/specgraph/v1/authoring.proto`, add the new value after `SINGLE_UNIT`:

```protobuf
enum DecompositionStrategy {
  DECOMPOSITION_STRATEGY_UNSPECIFIED = 0;
  // Deliver end-to-end value in independently shippable vertical slices.
  DECOMPOSITION_STRATEGY_VERTICAL_SLICE = 1;
  // Split work by architectural layer (e.g. storage first, then API, then UI).
  DECOMPOSITION_STRATEGY_LAYER_CAKE = 2;
  // Deliver the entire spec as one unit; no decomposition.
  DECOMPOSITION_STRATEGY_SINGLE_UNIT = 3;
  // Thin vertical cut proving riskiest integration points first.
  // slices[0] is the thread (no dependsOn); all others reachable from it.
  DECOMPOSITION_STRATEGY_STEEL_THREAD = 4;
}
```

- [ ] **Step 2: Regenerate proto**

Run: `task proto`
Expected: Proto fingerprint updates, `gen/specgraph/v1/authoring.pb.go` regenerated with `DECOMPOSITION_STRATEGY_STEEL_THREAD = 4`.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Clean build (no compilation errors).

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "feat(proto): add DECOMPOSITION_STRATEGY_STEEL_THREAD enum value (spgr-47v)"
jj --no-pager new
```

---

### Task 2: Domain Constant + Mapping

**Files:**

- Modify: `internal/storage/authoring.go:79-94`
- Modify: `internal/server/authoring_handler.go:747-751`
- Modify: `internal/server/convert_authoring.go:158-163`

- [ ] **Step 1: Write the failing test — domain IsValid**

In `internal/server/convert_authoring_test.go`, find `TestDecompositionStrategyIsValid` (line ~290) and add after the existing assertions:

```go
assert.True(t, storage.StrategySteelThread.IsValid())
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestDecompositionStrategyIsValid -v`
Expected: FAIL — `storage.StrategySteelThread` undefined.

- [ ] **Step 3: Add domain constant**

In `internal/storage/authoring.go`, add the constant to the existing block (after `StrategySingleUnit`):

```go
const (
	StrategyVerticalSlice DecompositionStrategy = "vertical_slice"
	StrategyLayerCake     DecompositionStrategy = "layer_cake"
	StrategySingleUnit    DecompositionStrategy = "single_unit"
	StrategySteelThread   DecompositionStrategy = "steel_thread"
)
```

And update `IsValid` to include it:

```go
func (s DecompositionStrategy) IsValid() bool {
	switch s {
	case StrategyVerticalSlice, StrategyLayerCake, StrategySingleUnit, StrategySteelThread:
		return true
	default:
		return false
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server/ -run TestDecompositionStrategyIsValid -v`
Expected: PASS

- [ ] **Step 5: Write the failing test — proto-to-domain map**

In `internal/server/authoring_handler_test.go`, add a new test after `TestAuthoringHandler_Decompose_HappyPath` (line ~396):

```go
func TestAuthoringHandler_Decompose_SteelThread_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "broaden-a", Intent: "add feature A", DependsOn: []string{"thread"}},
			},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD, resp.Msg.Output.Strategy)
}
```

- [ ] **Step 6: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Decompose_SteelThread_HappyPath -v`
Expected: FAIL — `unknown decomposition strategy` because `decomposeStrategyMap` doesn't include the new value.

- [ ] **Step 7: Add proto-to-domain mapping**

In `internal/server/authoring_handler.go`, add to `decomposeStrategyMap` (line ~747):

```go
var decomposeStrategyMap = map[specv1.DecompositionStrategy]storage.DecompositionStrategy{
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE: storage.StrategyVerticalSlice,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE:     storage.StrategyLayerCake,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT:    storage.StrategySingleUnit,
	specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD:   storage.StrategySteelThread,
}
```

- [ ] **Step 8: Add domain-to-proto mapping**

In `internal/server/convert_authoring.go`, add to `decomposeStrategyStringToProtoMap` (line ~158):

```go
var decomposeStrategyStringToProtoMap = map[storage.DecompositionStrategy]specv1.DecompositionStrategy{
	storage.StrategyVerticalSlice: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
	storage.StrategyLayerCake:     specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE,
	storage.StrategySingleUnit:    specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
	storage.StrategySteelThread:   specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
}
```

- [ ] **Step 9: Write the failing test — domain-to-proto round-trip**

In `internal/server/convert_authoring_test.go`, add to the `TestDecomposeStrategyStringToProto` table (line ~233):

```go
{storage.StrategySteelThread, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD},
```

- [ ] **Step 10: Run all mapping tests**

Run: `go test ./internal/server/ -run "TestDecompositionStrategyIsValid|TestDecomposeStrategyStringToProto|TestAuthoringHandler_Decompose_SteelThread_HappyPath" -v`
Expected: All PASS.

- [ ] **Step 11: Commit**

```text
jj --no-pager describe -m "feat(storage,server): add StrategySteelThread domain constant and proto mappings (spgr-47v)"
jj --no-pager new
```

---

### Task 3: Steel Thread Topology Validation

**Files:**

- Modify: `internal/server/authoring_handler.go:271-297` (add validation call)
- Modify: `internal/server/authoring_handler.go` (add `validateSteelThread` function, near other validation helpers)
- Modify: `internal/server/authoring_handler_test.go` (add validation tests)

- [ ] **Step 1: Write the failing test — thread root has dependsOn**

In `internal/server/authoring_handler_test.go`, add:

```go
func TestAuthoringHandler_Decompose_SteelThread_RootHasDeps(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip", DependsOn: []string{"something"}},
				{Id: "broaden", Intent: "add feature", DependsOn: []string{"thread"}},
			},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "no dependencies")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Decompose_SteelThread_RootHasDeps -v`
Expected: FAIL — no validation rejects this yet, so the request succeeds.

- [ ] **Step 3: Write the failing test — disconnected slice**

In `internal/server/authoring_handler_test.go`, add:

```go
func TestAuthoringHandler_Decompose_SteelThread_DisconnectedSlice(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "connected", Intent: "depends on thread", DependsOn: []string{"thread"}},
				{Id: "island", Intent: "no path to thread"},
			},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "does not transitively depend on thread slice")
}
```

- [ ] **Step 4: Write the failing test — valid chained broadening**

In `internal/server/authoring_handler_test.go`, add:

```go
func TestAuthoringHandler_Decompose_SteelThread_ChainedBroadening(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "broaden-a", Intent: "first broadening", DependsOn: []string{"thread"}},
				{Id: "broaden-b", Intent: "depends on broaden-a", DependsOn: []string{"broaden-a"}},
			},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD, resp.Msg.Output.Strategy)
}
```

- [ ] **Step 5: Write the failing test — regression: non-steel-thread unaffected**

In `internal/server/authoring_handler_test.go`, add:

```go
func TestAuthoringHandler_Decompose_NonSteelThread_NoNewValidation(t *testing.T) {
	// A vertical-slice decomposition with a disconnected slice should still pass
	// (steel thread validation only applies to STEEL_THREAD strategy).
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "a", Intent: "independent slice A"},
				{Id: "b", Intent: "independent slice B"},
			},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}
```

- [ ] **Step 6: Run all new tests to confirm failure pattern**

Run: `go test ./internal/server/ -run "TestAuthoringHandler_Decompose_SteelThread_RootHasDeps|TestAuthoringHandler_Decompose_SteelThread_DisconnectedSlice|TestAuthoringHandler_Decompose_SteelThread_ChainedBroadening|TestAuthoringHandler_Decompose_NonSteelThread_NoNewValidation" -v`
Expected: `RootHasDeps` and `DisconnectedSlice` FAIL (no validation yet). `ChainedBroadening` and `NonSteelThread_NoNewValidation` PASS.

- [ ] **Step 7: Implement validateSteelThread**

In `internal/server/authoring_handler.go`, add near the other validation helpers (after `decomposeOutputToDomain`, around line 772):

```go
// validateSteelThread checks steel-thread topology constraints:
// 1. slices[0] (the thread) must have no dependsOn.
// 2. Every other slice must transitively reach slices[0].id.
func validateSteelThread(slices []*specv1.DecompositionSlice) error {
	if len(slices) == 0 {
		return nil // empty slices caught elsewhere
	}
	threadID := slices[0].GetId()
	if len(slices[0].GetDependsOn()) > 0 {
		return fmt.Errorf("steel thread strategy requires slices[0] to have no dependencies (it is the thread)")
	}

	// Build adjacency: child -> parents (dependsOn).
	deps := make(map[string][]string, len(slices))
	for _, s := range slices {
		deps[s.GetId()] = s.GetDependsOn()
	}

	// For each non-root slice, walk dependsOn transitively to check reachability.
	for _, s := range slices[1:] {
		if !reachesRoot(s.GetId(), threadID, deps) {
			return fmt.Errorf("slice %q does not transitively depend on thread slice %q", s.GetId(), threadID)
		}
	}
	return nil
}

// reachesRoot walks the dependsOn graph from start, returning true if threadID is reachable.
func reachesRoot(start, threadID string, deps map[string][]string) bool {
	visited := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == threadID {
			return true
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		queue = append(queue, deps[cur]...)
	}
	return false
}
```

- [ ] **Step 8: Wire validation into Decompose handler**

In `internal/server/authoring_handler.go`, inside `func (h *AuthoringHandler) Decompose(...)`, add the steel-thread validation after the existing `for` loop that checks slice intent lengths (after line ~293, before `decomposeOutputToDomain`):

```go
	if msg.Output.GetStrategy() == specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD {
		if err := validateSteelThread(msg.Output.GetSlices()); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
```

- [ ] **Step 9: Run all steel thread tests**

Run: `go test ./internal/server/ -run "TestAuthoringHandler_Decompose_SteelThread|TestAuthoringHandler_Decompose_NonSteelThread_NoNewValidation" -v`
Expected: All PASS.

- [ ] **Step 10: Run full server test suite for regressions**

Run: `go test ./internal/server/ -v -count=1`
Expected: All existing tests still PASS.

- [ ] **Step 11: Commit**

```text
jj --no-pager describe -m "feat(server): add steel thread topology validation for decompose (spgr-47v)"
jj --no-pager new
```

---

### Task 4: Reference Docs

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-decompose/references/decompose-output-format.md`

> **Note:** The original plan included creating `e2e/cli/testdata/decompose-output-steel-thread.json`,
> but this fixture was removed (no CLI test references it). The steel thread strategy is exercised
> via E2E API tests in `e2e/api/authoring_test.go` instead.

- [ ] **Step 1: Update decompose-output-format.md — Strategy Values**

In `plugin/specgraph/skills/specgraph-decompose/references/decompose-output-format.md`, add to the Strategy Values section (after the `SINGLE_UNIT` entry):

```markdown
- `DECOMPOSITION_STRATEGY_STEEL_THREAD` — First slice (slices[0]) is a thin vertical cut proving riskiest interfaces; all other slices depend on it and can parallelize
```

- [ ] **Step 2: Update decompose-output-format.md — Field Notes**

In the same file, add to the Field Notes section:

```markdown
- When using `STEEL_THREAD`, `slices[0]` is the thread slice and must have no `dependsOn`. All other slices must transitively depend on it.
```

- [ ] **Step 3: Commit**

```text
jj --no-pager describe -m "docs: add steel thread reference docs (spgr-47v)"
jj --no-pager new
```

---

### Task 5: Decompose Skill Update

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-decompose/SKILL.md`

- [ ] **Step 1: Add steel thread to strategy table**

In `plugin/specgraph/skills/specgraph-decompose/SKILL.md`, update the strategy table in the "1. Strategy" section (line ~44) to add the new row:

```markdown
| Strategy | When to recommend | Description |
|----------|-------------------|-------------|
| **Vertical slice** | User-facing features | Each slice delivers end-to-end value |
| **Horizontal layer** | Infrastructure work | Split by architecture tier (storage -> API -> UI) |
| **Steel thread** | Unproven interfaces, maximizing future parallelism | First slice proves riskiest integration points end-to-end; remaining slices broaden from it |
| **Single unit** | Small, self-contained work | Deliver the spec as-is without decomposition |
```

- [ ] **Step 2: Add steel thread guidance section**

In the same file, after the "3. Dependency Ordering" section (line ~68), add a new subsection:

```markdown
#### Steel Thread Guidance

When steel thread strategy is selected, guide the author through these steps:

1. **Identify risk:** Which integration points are least understood? Where could
   assumptions break? The thread slice must exercise these.
2. **Minimal cut:** Design the thread slice as the thinnest possible path through
   all layers. Its purpose is proving interfaces work, not delivering features.
3. **Verify contracts:** Thread slice `verify` criteria should focus on interface
   contracts -- "request round-trips through storage and back" not "all CRUD
   operations work."
4. **Fan-out:** Remaining slices broaden from the thread. Each adds depth or
   features using the now-proven interfaces. These can parallelize freely.

**Example:**

| Slice | Intent | Depends on |
|-------|--------|------------|
| `prove-roundtrip` | Proto-storage-handler-CLI round-trip for create | (none -- this is the thread) |
| `broaden-crud` | Add update and delete using proven interfaces | `prove-roundtrip` |
| `broaden-query` | Add list and filter operations | `prove-roundtrip` |

Note: `broaden-crud` and `broaden-query` can execute in parallel because they
both depend only on the thread, not on each other.
```

- [ ] **Step 3: Add quality heuristic for steel thread**

In the "Quality Heuristics" table (line ~79), add:

```markdown
| Steel thread slice has feature-level verify criteria | "The thread slice should prove interfaces, not deliver features. Can we narrow the verify criteria to contract validation?" |
```

- [ ] **Step 4: Commit**

```text
jj --no-pager describe -m "docs(skill): add steel thread guidance to decompose skill (spgr-47v)"
jj --no-pager new
```

---

### Task 6: E2E Test — Steel Thread Pipeline

**Files:**

- Modify: `e2e/api/authoring_test.go`

These tests use Ginkgo/Gomega and require Docker (`//go:build e2e`). They run via `task pr-prep`, not `task check`.

- [ ] **Step 1: Add steel thread Ordered container**

In `e2e/api/authoring_test.go`, add a new `Describe` block after the existing "Authoring funnel" container. This needs its own slug to avoid conflicts with the existing ordered tests:

```go
var _ = Describe("Authoring funnel — steel thread", Ordered, func() {
	const steelThreadSlug = "steel-thread-funnel-test"

	var (
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		authoringClient = newAuthoringClient()
		ctx = context.Background()
	})

	It("sparks a new spec", func() {
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: steelThreadSlug,
			Output: &specv1.SparkOutput{
				Seed:       "Steel thread E2E test idea",
				Signal:     "Testing steel thread decomposition",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
				KillTest:   "Test fails",
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("shapes the spec", func() {
		_, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: steelThreadSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn: []string{"interfaces"},
				Risks:   []string{"integration risk"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("specifies the spec", func() {
		_, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: steelThreadSlug,
			Output: &specv1.SpecifyOutput{
				Interfaces:     []*specv1.InterfaceSection{{Name: "API", Body: "test"}},
				VerifyCriteria: []*specv1.VerifyCriterion{{Description: "passes"}},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("decomposes with steel thread strategy", func() {
		resp, err := authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: steelThreadSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
				Slices: []*specv1.DecompositionSlice{
					{Id: "thread", Intent: "Prove roundtrip", Verify: []string{"roundtrip works"}},
					{Id: "broaden-a", Intent: "Add feature A", Verify: []string{"feature A works"}, DependsOn: []string{"thread"}},
					{Id: "broaden-b", Intent: "Add feature B", Verify: []string{"feature B works"}, DependsOn: []string{"thread"}},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Slices).To(HaveLen(3))
		Expect(resp.Msg.SliceSlugs).To(HaveLen(3))
		Expect(resp.Msg.SliceSlugs).To(ContainElement(HaveSuffix("/thread")))
	})

	It("approves the steel thread spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: steelThreadSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
	})

	It("rejects steel thread with disconnected slice", func() {
		const badSlug = "steel-thread-bad-test"
		// Advance to specify first
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug:   badSlug,
			Output: &specv1.SparkOutput{Seed: "bad steel thread", Signal: "test", ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_SMALL, KillTest: "fails"},
		}))
		Expect(err).NotTo(HaveOccurred())
		_, err = authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug:   badSlug,
			Output: &specv1.ShapeOutput{ScopeIn: []string{"test"}, Risks: []string{"none"}},
		}))
		Expect(err).NotTo(HaveOccurred())
		_, err = authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug:   badSlug,
			Output: &specv1.SpecifyOutput{Interfaces: []*specv1.InterfaceSection{{Name: "X", Body: "y"}}, VerifyCriteria: []*specv1.VerifyCriterion{{Description: "z"}}},
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: badSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
				Slices: []*specv1.DecompositionSlice{
					{Id: "thread", Intent: "Prove roundtrip"},
					{Id: "island", Intent: "No path to thread"},
				},
			},
		}))
		Expect(err).To(HaveOccurred())
		var connErr *connect.Error
		Expect(errors.As(err, &connErr)).To(BeTrue())
		Expect(connErr.Code()).To(Equal(connect.CodeInvalidArgument))
	})
})
```

Note: The negative test (`"rejects steel thread with disconnected slice"`) needs `"errors"` imported. Check whether `errors` is already imported in the file — if not, add it.

- [ ] **Step 2: Verify E2E compiles**

Run: `go build -tags e2e ./e2e/...`
Expected: Clean build.

- [ ] **Step 3: Commit**

```text
jj --no-pager describe -m "test(e2e): add steel thread pipeline and rejection tests (spgr-47v)"
jj --no-pager new
```

---

### Task 7: Quality Gate + Squash

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: All format checks, lint, build, and unit tests pass.

- [ ] **Step 2: Review the full change set**

Run: `jj --no-pager log --limit 10`
Verify there are 6 commits for Tasks 1-6, all on the `steel-thread` bookmark.

- [ ] **Step 3: Squash into a single commit**

```text
jj --no-pager squash --from 'all:roots(steel-thread..@)' --into 'roots(steel-thread..@)' -m "feat(authoring): add steel thread decomposition strategy (spgr-47v)

Add DECOMPOSITION_STRATEGY_STEEL_THREAD (enum value 4) to the decompose stage.
Steel thread is a thin vertical slice proving riskiest integration points first,
with all subsequent slices depending on it for maximum parallelism.

- Proto: new enum value in DecompositionStrategy
- Storage: StrategySteelThread domain constant
- Server: validateSteelThread() enforces single root + full reachability
- Skill: strategy selection guidance and thread slice authoring prompts
- Docs: reference format updated with new strategy and field notes
- E2E: pipeline test and negative validation test

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 4: Update bookmark and verify**

```text
jj --no-pager bookmark set steel-thread -r @
jj --no-pager log --limit 3
```
