# Show Stage Detail Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expand `specgraph show` to render full authoring stage detail (Spark/Shape/Specify/Decompose) inline.

**Architecture:** Extend the GetSpec → proto → render pipeline: add 4 proto fields to Spec, parse stage output JSON in `recordToSpec()`, create domain-to-proto converters, add render functions. No new RPCs or commands.

**Tech Stack:** Protobuf (buf), Go, ConnectRPC, Memgraph (Cypher)

**Spec:** `docs/superpowers/specs/2026-03-24-show-stage-detail-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `proto/specgraph/v1/spec.proto` | Modify | Add fields 17-20 (SparkOutput, ShapeOutput, SpecifyOutput, DecomposeOutput) |
| `internal/storage/spec_domain.go` | Modify | Add 4 pointer fields to Spec struct |
| `internal/storage/memgraph/memgraph.go` | Modify | Extend queries + `recordToSpecOffset()` to parse stage output JSON |
| `internal/server/convert.go` | Modify | Add 4 domain-to-proto converters; extend `specToProto()` |
| `internal/render/authoring.go` | Create | Stage section renderers (Spark, Shape, Specify, Decompose) |
| `internal/render/authoring_test.go` | Create | Unit tests for stage renderers |
| `internal/render/spec.go` | Modify | Call stage section renderers from `Spec()` |

---

## Chunk 1: Proto & Domain Types

### Task 1: Add stage output fields to proto Spec and domain Spec

**Files:**

- Modify: `proto/specgraph/v1/spec.proto:38` (after content_hash field)
- Modify: `internal/storage/spec_domain.go:151` (after ConversationLogs field)

- [ ] **Step 1: Add proto fields to Spec message**

In `proto/specgraph/v1/spec.proto`, after `conversation_logs` (field 16), add:

```protobuf
  SparkOutput spark_output = 17;         // populated when stage >= spark
  ShapeOutput shape_output = 18;         // populated when stage >= shape
  SpecifyOutput specify_output = 19;     // populated when stage >= specify
  DecomposeOutput decompose_output = 20; // populated when stage >= decompose
```

`spec.proto` already imports `authoring.proto` (for ConversationLog), so SparkOutput etc. are available. Prefix with `specgraph.v1.` if needed.

- [ ] **Step 2: Generate Go code**

Run: `task proto`

Expected: Clean generation.

- [ ] **Step 3: Add domain fields to storage.Spec**

In `internal/storage/spec_domain.go`, after the `ConversationLogs` field, add:

```go
	SparkOutput     *SparkOutput
	ShapeOutput     *ShapeOutput
	SpecifyOutput   *SpecifyOutput
	DecomposeOutput *DecomposeOutput
```

- [ ] **Step 4: Verify build**

Run: `go build ./internal/storage/...`

Expected: PASS

- [ ] **Step 5: Commit**

```text
feat(proto,storage): add stage output fields to Spec message and domain type (spgr-0dg)
```

---

## Chunk 2: Storage Layer — Query & Parse

### Task 2: Extend GetSpec, BatchGetSpecs, ListSpecs queries and recordToSpecOffset

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:270-273` (GetSpec query)
- Modify: `internal/storage/memgraph/memgraph.go:327-330` (BatchGetSpecs query)
- Modify: `internal/storage/memgraph/memgraph.go:365-368` (ListSpecs query)
- Modify: `internal/storage/memgraph/memgraph.go:697-785` (recordToSpecOffset)
- Modify: `internal/storage/memgraph/lifecycle.go:206-213` (SupersedeSpec query — returns TWO specs per row)
- Modify: `internal/storage/memgraph/lifecycle_unit_test.go` (SupersedeSpec test — update offset from 14 to 18)

- [ ] **Step 1: Write integration test for GetSpec returning stage outputs**

Append to an existing test file or create a new section in `memgraph_test.go`. The test creates a spec, stores a spark output, then verifies GetSpec returns it:

```go
func TestGetSpec_IncludesSparkOutput(t *testing.T) {
	clearDatabase(t)
	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-show-spark", "test intent", "p2", "medium")
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "test-show-spark", &storage.SparkOutput{
		Seed:       "Build a widget",
		Signal:     "High demand",
		ScopeSniff: "small",
		KillTest:   "No migration needed",
		Questions:  []string{"What throughput?"},
	})
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "test-show-spark")
	require.NoError(t, err)
	require.NotNil(t, spec.SparkOutput)
	assert.Equal(t, "Build a widget", spec.SparkOutput.Seed)
	assert.Equal(t, "High demand", spec.SparkOutput.Signal)
	assert.Equal(t, "small", spec.SparkOutput.ScopeSniff)
	assert.Len(t, spec.SparkOutput.Questions, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/storage/memgraph/ -run TestGetSpec_IncludesSparkOutput -v -count=1`

Expected: FAIL — `SparkOutput` is nil because the query doesn't return it.

- [ ] **Step 3: Add stage output columns to all three queries**

In `memgraph.go`, extend the RETURN clauses for `GetSpec` (line 270), `BatchGetSpecs` (line 327), and `ListSpecs` (line 365). Add after `s.content_hash`:

```text
, s.spark_output, s.shape_output, s.specify_output, s.decompose_output
```

These will be at positions offset+14 through offset+17.

- [ ] **Step 4: Parse stage output JSON in recordToSpecOffset**

After the `contentHash` extraction (line 764-767), add parsing for each stage output. Use `recordStringOptional` since these may be null/empty:

```go
	sparkJSON, err := recordStringOptional(rec, offset+14, "spark_output")
	if err != nil {
		return nil, err
	}
	shapeJSON, err := recordStringOptional(rec, offset+15, "shape_output")
	if err != nil {
		return nil, err
	}
	specifyJSON, err := recordStringOptional(rec, offset+16, "specify_output")
	if err != nil {
		return nil, err
	}
	decomposeJSON, err := recordStringOptional(rec, offset+17, "decompose_output")
	if err != nil {
		return nil, err
	}
```

Then unmarshal each non-empty string. Add a local helper (or use inline):

```go
	var sparkOutput *storage.SparkOutput
	if sparkJSON != "" {
		sparkOutput = &storage.SparkOutput{}
		if err := json.Unmarshal([]byte(sparkJSON), sparkOutput); err != nil {
			return nil, fmt.Errorf("memgraph: unmarshal spark_output: %w", err)
		}
	}
	// Repeat for shape, specify, decompose...
```

Add the 4 fields to the returned `storage.Spec` struct literal:

```go
	SparkOutput:     sparkOutput,
	ShapeOutput:     shapeOutput,
	SpecifyOutput:   specifyOutput,
	DecomposeOutput: decomposeOutput,
```

Add `"encoding/json"` to the imports if not already present.

- [ ] **Step 4b: Update SupersedeSpec query in lifecycle.go**

The `SupersedeSpec` query in `lifecycle.go:206-213` returns TWO specs per row using `recordToSpecOffset(rec, 0)` and `recordToSpecOffset(rec, 14)`. Since each spec now has 18 columns (was 14), update:

1. Add `old.spark_output, old.shape_output, old.specify_output, old.decompose_output` after `old.content_hash` in the RETURN clause
2. Add `new.spark_output, new.shape_output, new.specify_output, new.decompose_output` after `new.content_hash`
3. Change `recordToSpecOffset(rec, 14)` to `recordToSpecOffset(rec, 18)`
4. Update the unit test in `lifecycle_unit_test.go` — the test constructs a mock record with hard-coded 28 values (14 per spec). Extend to 36 values (18 per spec), placing 4 empty strings at positions 14-17 and 32-35 for the stage output fields. Update the second spec offset from 14 to 18.

- [ ] **Step 5: Run test**

Run: `go test -tags integration ./internal/storage/memgraph/ -run TestGetSpec_IncludesSparkOutput -v -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```text
feat(memgraph): return stage outputs from GetSpec/BatchGetSpecs/ListSpecs (spgr-0dg)
```

---

## Chunk 3: Server Converters

### Task 3: Add domain-to-proto converters for stage outputs

**Files:**

- Modify: `internal/server/convert.go`

The existing code has proto-to-domain converters in `authoring_handler.go` (`sparkOutputToDomain` at line 644, `shapeOutputToDomain` at 658, etc.). We need the **reverse** direction: domain → proto. These go in `convert.go` alongside `specToProto`.

- [ ] **Step 1: Add sparkOutputToProto**

```go
func sparkOutputToProto(o *storage.SparkOutput) *specv1.SparkOutput {
	if o == nil {
		return nil
	}
	return &specv1.SparkOutput{
		Seed:       o.Seed,
		Signal:     o.Signal,
		Questions:  o.Questions,
		ScopeSniff: scopeSniffStringToProto(o.ScopeSniff),
		KillTest:   o.KillTest,
	}
}
```

Note: `ScopeSniff` is stored as a string (e.g., "small") but the proto field is a `ScopeSniff` enum. Add a reverse mapping:

```go
var scopeSniffStringToProtoMap = map[string]specv1.ScopeSniff{
	"":       specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED,
	"tiny":   specv1.ScopeSniff_SCOPE_SNIFF_TINY,
	"small":  specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
	"medium": specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
	"large":  specv1.ScopeSniff_SCOPE_SNIFF_LARGE,
	"epic":   specv1.ScopeSniff_SCOPE_SNIFF_EPIC,
}

func scopeSniffStringToProto(s string) specv1.ScopeSniff {
	if v, ok := scopeSniffStringToProtoMap[s]; ok {
		return v
	}
	return specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED
}
```

- [ ] **Step 2: Add shapeOutputToProto**

```go
func shapeOutputToProto(o *storage.ShapeOutput) *specv1.ShapeOutput {
	if o == nil {
		return nil
	}
	approaches := make([]*specv1.Approach, len(o.Approaches))
	for i, a := range o.Approaches {
		approaches[i] = &specv1.Approach{
			Name:        a.Name,
			Description: a.Description,
			Tradeoffs:   a.Tradeoffs,
		}
	}
	return &specv1.ShapeOutput{
		ScopeIn:        o.ScopeIn,
		ScopeOut:       o.ScopeOut,
		Approaches:     approaches,
		ChosenApproach: o.ChosenApproach,
		Risks:          o.Risks,
		SuccessMust:    o.SuccessMust,
		SuccessShould:  o.SuccessShould,
		SuccessWont:    o.SuccessWont,
	}
}
```

Note: `Decisions` are stored as `[]DecisionInput` in the domain but the proto `ShapeOutput` has `repeated DecisionInput decisions`. Map them:

```go
	decisions := make([]*specv1.DecisionInput, len(o.Decisions))
	for i, d := range o.Decisions {
		decisions[i] = &specv1.DecisionInput{
			Slug:      d.Slug,
			Title:     d.Title,
			Decision:  d.Body,
			Rationale: d.Rationale,
		}
	}
```

Add `Decisions: decisions` to the returned struct.

- [ ] **Step 3: Add specifyOutputToProto**

```go
func specifyOutputToProto(o *storage.SpecifyOutput) *specv1.SpecifyOutput {
	if o == nil {
		return nil
	}
	interfaces := make([]*specv1.InterfaceSection, len(o.Interfaces))
	for i, iface := range o.Interfaces {
		interfaces[i] = &specv1.InterfaceSection{
			Name: iface.Name,
			Body: iface.Body,
		}
	}
	criteria := make([]*specv1.VerifyCriterion, len(o.VerifyCriteria))
	for i, vc := range o.VerifyCriteria {
		criteria[i] = &specv1.VerifyCriterion{
			Category:    vc.Category,
			Description: vc.Description,
		}
	}
	touches := make([]*specv1.FileTouch, len(o.Touches))
	for i, ft := range o.Touches {
		touches[i] = &specv1.FileTouch{
			Path:       ft.Path,
			Purpose:    ft.Purpose,
			ChangeType: ft.ChangeType,
		}
	}
	return &specv1.SpecifyOutput{
		Interfaces:     interfaces,
		VerifyCriteria: criteria,
		Invariants:     o.Invariants,
		Touches:        touches,
	}
}
```

- [ ] **Step 4: Add decomposeOutputToProto**

```go
func decomposeOutputToProto(o *storage.DecomposeOutput) *specv1.DecomposeOutput {
	if o == nil {
		return nil
	}
	slices := make([]*specv1.DecompositionSlice, len(o.Slices))
	for i, s := range o.Slices {
		slices[i] = &specv1.DecompositionSlice{
			Id:        s.ID,
			Intent:    s.Intent,
			Verify:    s.Verify,
			Touches:   s.Touches,
			DependsOn: s.DependsOn,
		}
	}
	return &specv1.DecomposeOutput{
		Strategy: decomposeStrategyStringToProto(o.Strategy),
		Slices:   slices,
	}
}

var decomposeStrategyStringToProtoMap = map[storage.DecompositionStrategy]specv1.DecompositionStrategy{
	storage.StrategyVerticalSlice: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
	storage.StrategyLayerCake:     specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE,
	storage.StrategySingleUnit:    specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
}

func decomposeStrategyStringToProto(s storage.DecompositionStrategy) specv1.DecompositionStrategy {
	if v, ok := decomposeStrategyStringToProtoMap[s]; ok {
		return v
	}
	return specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED
}
```

- [ ] **Step 5: Update specToProto to populate stage output fields**

In `specToProto()`, after the ConversationLogs block, add:

```go
	pb.SparkOutput = sparkOutputToProto(s.SparkOutput)
	pb.ShapeOutput = shapeOutputToProto(s.ShapeOutput)
	pb.SpecifyOutput = specifyOutputToProto(s.SpecifyOutput)
	pb.DecomposeOutput = decomposeOutputToProto(s.DecomposeOutput)
```

- [ ] **Step 6: Verify build**

Run: `go build ./...`

Expected: PASS

- [ ] **Step 7: Commit**

```text
feat(server): add domain-to-proto converters for stage outputs (spgr-0dg)
```

---

## Chunk 4: Render Functions & Integration

### Task 4: Create authoring stage renderers

**Files:**

- Create: `internal/render/authoring.go`
- Create: `internal/render/authoring_test.go`

- [ ] **Step 1: Write test for SparkSection**

Create `internal/render/authoring_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestSparkSection_Nil(t *testing.T) {
	assert.Empty(t, render.SparkSection(nil))
}

func TestSparkSection_Full(t *testing.T) {
	out := &specv1.SparkOutput{
		Seed:       "Build a widget factory",
		Signal:     "High customer demand",
		ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
		KillTest:   "No migration needed",
		Questions:  []string{"What throughput?", "Which types?"},
	}
	result := render.SparkSection(out)
	assert.Contains(t, result, "## Spark")
	assert.Contains(t, result, "Build a widget factory")
	assert.Contains(t, result, "High customer demand")
	assert.Contains(t, result, "medium")
	assert.Contains(t, result, "No migration needed")
	assert.Contains(t, result, "What throughput?")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestSparkSection -v`

Expected: FAIL — `SparkSection` not found.

- [ ] **Step 3: Implement SparkSection**

Create `internal/render/authoring.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// SparkSection renders the spark stage output as markdown.
func SparkSection(o *specv1.SparkOutput) string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Spark\n\n")

	if o.Seed != "" {
		fmt.Fprintf(&b, "> **Seed:** %s\n\n", o.Seed)
	}
	if o.Signal != "" {
		fmt.Fprintf(&b, "> **Signal:** %s\n\n", o.Signal)
	}
	if scope := scopeSniffString(o.ScopeSniff); scope != "" {
		fmt.Fprintf(&b, "**Scope Sniff:** %s\n", scope)
	}
	if o.KillTest != "" {
		fmt.Fprintf(&b, "**Kill Test:** %s\n", o.KillTest)
	}
	if len(o.Questions) > 0 {
		b.WriteString("\n**Questions:**\n")
		for _, q := range o.Questions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func scopeSniffString(s specv1.ScopeSniff) string {
	switch s {
	case specv1.ScopeSniff_SCOPE_SNIFF_TINY:
		return "tiny"
	case specv1.ScopeSniff_SCOPE_SNIFF_SMALL:
		return "small"
	case specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM:
		return "medium"
	case specv1.ScopeSniff_SCOPE_SNIFF_LARGE:
		return "large"
	case specv1.ScopeSniff_SCOPE_SNIFF_EPIC:
		return "epic"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/render/ -run TestSparkSection -v`

Expected: PASS

- [ ] **Step 5: Add ShapeSection tests and implementation**

Tests:

```go
func TestShapeSection_Nil(t *testing.T) {
	assert.Empty(t, render.ShapeSection(nil))
}

func TestShapeSection_Full(t *testing.T) {
	out := &specv1.ShapeOutput{
		ScopeIn:        []string{"API", "Storage"},
		ScopeOut:       []string{"Web UI"},
		ChosenApproach: "Plugin arch",
		Approaches: []*specv1.Approach{
			{Name: "Plugin arch", Description: "Modular plugins", Tradeoffs: []string{"Complex"}},
			{Name: "Monolith", Description: "Single service"},
		},
		Risks:         []string{"Performance under load"},
		SuccessMust:   []string{"CRUD via API"},
		SuccessShould: []string{"Batch ops"},
		SuccessWont:   []string{"Real-time collab"},
	}
	result := render.ShapeSection(out)
	assert.Contains(t, result, "## Shape")
	assert.Contains(t, result, "API")
	assert.Contains(t, result, "Web UI")
	assert.Contains(t, result, "Plugin arch")
	assert.Contains(t, result, "(chosen)")
	assert.Contains(t, result, "Performance under load")
	assert.Contains(t, result, "CRUD via API")
}
```

Implementation: `ShapeSection` renders scope in/out as bullet lists, approaches as H3 subsections with "(chosen)" marker, risks/success as bullet lists under H3 headings. Follow the same `strings.Builder` pattern.

- [ ] **Step 6: Add SpecifySection tests and implementation**

Tests:

```go
func TestSpecifySection_Nil(t *testing.T) {
	assert.Empty(t, render.SpecifySection(nil))
}

func TestSpecifySection_Full(t *testing.T) {
	out := &specv1.SpecifyOutput{
		Interfaces: []*specv1.InterfaceSection{
			{Name: "WidgetService", Body: "CreateWidget, GetWidget RPCs"},
		},
		VerifyCriteria: []*specv1.VerifyCriterion{
			{Category: "functional", Description: "Create returns valid ID"},
		},
		Invariants: []string{"Widget IDs are globally unique"},
		Touches: []*specv1.FileTouch{
			{Path: "internal/widget/service.go", Purpose: "Widget service", ChangeType: "create"},
		},
	}
	result := render.SpecifySection(out)
	assert.Contains(t, result, "## Specify")
	assert.Contains(t, result, "WidgetService")
	assert.Contains(t, result, "Create returns valid ID")
	assert.Contains(t, result, "Widget IDs are globally unique")
	assert.Contains(t, result, "internal/widget/service.go")
}
```

Implementation: interfaces as H3 subsections with body, verify criteria as a table (Category | Description), invariants as bullets, touches as a table (Path | Purpose | Action).

- [ ] **Step 7: Add DecomposeSection tests and implementation**

Tests:

```go
func TestDecomposeSection_Nil(t *testing.T) {
	assert.Empty(t, render.DecomposeSection(nil))
}

func TestDecomposeSection_Full(t *testing.T) {
	out := &specv1.DecomposeOutput{
		Strategy: "vertical_slice",
		Slices: []*specv1.DecompositionSlice{
			{
				Id:        "slice-1",
				Intent:    "Core widget CRUD",
				Verify:    []string{"Create returns 201"},
				DependsOn: []string{},
			},
			{
				Id:        "slice-2",
				Intent:    "Batch operations",
				DependsOn: []string{"slice-1"},
			},
		},
	}
	result := render.DecomposeSection(out)
	assert.Contains(t, result, "## Decompose")
	assert.Contains(t, result, "vertical_slice")
	assert.Contains(t, result, "slice-1")
	assert.Contains(t, result, "Core widget CRUD")
	assert.Contains(t, result, "slice-2")
	assert.Contains(t, result, "slice-1") // dependency reference
}
```

Implementation: strategy as blockquote, slices as H3 subsections with intent, verify criteria bullets, dependency list.

- [ ] **Step 8: Run all render tests**

Run: `go test ./internal/render/ -v`

Expected: All PASS (existing + new)

- [ ] **Step 9: Commit**

```text
feat(render): add authoring stage section renderers (spgr-0dg)
```

### Task 5: Integrate stage sections into render.Spec()

**Files:**

- Modify: `internal/render/spec.go:34`

- [ ] **Step 1: Add stage section calls to Spec()**

In `render.Spec()`, after `b.WriteString(section(2, "Notes", s.Notes))` (line 34), add:

```go
	b.WriteString(SparkSection(s.SparkOutput))
	b.WriteString(ShapeSection(s.ShapeOutput))
	b.WriteString(SpecifySection(s.SpecifyOutput))
	b.WriteString(DecomposeSection(s.DecomposeOutput))
	b.WriteString(ConversationLogList(s.ConversationLogs))
```

Note: `ConversationLogList` was previously not called from `Spec()` — add it now so conversations appear in show output too.

- [ ] **Step 2: Verify build**

Run: `go build ./...`

Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(render): integrate stage sections into Spec() output (spgr-0dg)
```

---

## Chunk 5: Quality Gates

### Task 6: Run full quality gates

- [ ] **Step 1: Run task check**

Run: `task check`

Expected: PASS

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`

Expected: PASS

- [ ] **Step 3: Fix any issues**

Address lint, license headers, formatting.

- [ ] **Step 4: Commit fixes if needed**

```text
fix: address lint and formatting issues (spgr-0dg)
```

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | Task 1 | Proto fields + domain type |
| 2 | Task 2 | Storage queries + JSON parsing |
| 3 | Task 3 | Domain-to-proto converters |
| 4 | Tasks 4-5 | Render functions + Spec() integration |
| 5 | Task 6 | Quality gates |

**Total:** 6 tasks, ~25 bite-sized steps.
