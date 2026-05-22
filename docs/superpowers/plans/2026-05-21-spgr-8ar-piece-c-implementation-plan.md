# spgr-8ar Piece C — Surface Provenance in `constitution show`

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `--show-provenance` flag to `specgraph constitution show` that annotates each constitution field with the layer that set it. Without the flag, output is byte-identical to today.

**Architecture:** New `ConstitutionWithProvenance(c, prov)` text renderer alongside the existing `Constitution(c)`. CLI branches on the flag. JSON mode emits the `provenance` array only when the flag is set (clear `resp.Msg.Provenance` on the proto before `printJSON` if `--show-provenance` is not set — protojson omits empty fields).

**Tech Stack:** Go, existing renderer package, existing ConnectRPC. No new dependencies.

**Reference spec:** Section 9 + invariant 11 (CLI byte-stability) + invariant 12 (Renderer behavior).

---

## File Structure

### Files created

| Path | Responsibility |
|---|---|
| `internal/render/constitution_test.go` | Golden-file byte-stability test for `Constitution(c)`; assertions for `ConstitutionWithProvenance` |

### Files modified

| Path | Change |
|---|---|
| `internal/render/constitution.go` | Add `ConstitutionWithProvenance` sibling function |
| `cmd/specgraph/constitution.go` | Add `--show-provenance` flag; wire it through to text + JSON modes |

---

## Task 1: Add `ConstitutionWithProvenance` text renderer + tests

**Files:**

- Modify: `internal/render/constitution.go`
- Create: `internal/render/constitution_test.go`

The new function annotates each rendered field with `(set by: <layer>)` based on a `[]*specv1.ProvenanceEntry` map. When the slice is nil or empty, behavior is identical to the existing `Constitution(c)` (the renderer invariant from Section 14).

Provenance paths follow the format documented in `internal/constitution/merge`:

- `principles[<id>]` — for keyed principle list
- `antipatterns[<pattern>]` — for keyed antipattern list
- `references[<path>]` — for keyed reference list
- `constraints[<value>]` — for constraint string list
- `tech_config.languages.primary` — for nested scalar
- `tech_config.languages.allowed[<value>]` — for nested string list

Build a `map[string]string` lookup keyed by path → layer-string for fast access during rendering.

### Implementation sketch

In `internal/render/constitution.go`, append:

```go
// ConstitutionWithProvenance renders a constitution as markdown,
// annotating each field with the layer that set its value when
// provenance is available.
//
// Invariant (Section 14 of the spgr-8ar design): when provenance is nil
// or empty, output is byte-identical to Constitution(c).
func ConstitutionWithProvenance(c *specv1.Constitution, provenance []*specv1.ProvenanceEntry) string {
	if len(provenance) == 0 {
		return Constitution(c)
	}
	if c == nil {
		return "No constitution found.\n"
	}

	// Build a path → layer-string lookup.
	provByPath := make(map[string]string, len(provenance))
	for _, e := range provenance {
		provByPath[e.GetPath()] = constitutionLayerString(e.GetLayer())
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", c.GetName())

	// Metadata table (no provenance annotations — these are document
	// identity, not per-field configuration).
	pairs := [][2]string{
		{"Layer", constitutionLayerString(c.GetLayer())},
		{"Version", fmt.Sprintf("%d", c.GetVersion())},
	}
	if tech := c.GetTech(); tech != nil {
		if langs := tech.GetLanguages(); langs != nil && langs.GetPrimary() != "" {
			pairs = append(pairs, [2]string{"Primary Language", langs.GetPrimary()})
		}
	}
	b.WriteString(metadataTable(pairs))

	if ps := c.GetPrinciples(); len(ps) > 0 {
		b.WriteString("\n## Principles\n\n")
		for _, p := range ps {
			fmt.Fprint(&b, formatItem("- "+p.GetStatement(), provByPath["principles["+p.GetId()+"]"]))
		}
	}

	if cs := c.GetConstraints(); len(cs) > 0 {
		b.WriteString("\n## Constraints\n\n")
		for _, ct := range cs {
			fmt.Fprint(&b, formatItem("- "+ct, provByPath["constraints["+ct+"]"]))
		}
	}

	if aps := c.GetAntipatterns(); len(aps) > 0 {
		b.WriteString("\n## Anti-patterns\n\n")
		for _, ap := range aps {
			line := fmt.Sprintf("- **%s**: %s", ap.GetPattern(), ap.GetWhy())
			fmt.Fprint(&b, formatItem(line, provByPath["antipatterns["+ap.GetPattern()+"]"]))
		}
	}

	if refs := c.GetReferences(); len(refs) > 0 {
		b.WriteString("\n## References\n\n")
		for _, ref := range refs {
			line := fmt.Sprintf("- [%s] %s", referenceTypeName(ref.GetReferenceType()), ref.GetPath())
			fmt.Fprint(&b, formatItem(line, provByPath["references["+ref.GetPath()+"]"]))
		}
	}

	return b.String()
}

// formatItem appends the (set by: layer) annotation to a markdown list
// item when layer is non-empty. Falls back to a trailing newline.
func formatItem(line, layer string) string {
	if layer == "" {
		return line + "\n"
	}
	return line + "  (set by: " + layer + ")\n"
}
```

### Tests

Create `internal/render/constitution_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestConstitutionWithProvenance_NilProvenance_EquivalentToConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name:        "test",
		Layer:       specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version:     1,
		Constraints: []string{"never use eval"},
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "Prefer explicit"},
		},
	}
	a := Constitution(c)
	b := ConstitutionWithProvenance(c, nil)
	assert.Equal(t, a, b, "with nil provenance, output must be byte-identical to Constitution(c)")
}

func TestConstitutionWithProvenance_EmptyProvenance_EquivalentToConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name: "test",
		Principles: []*specv1.Principle{{Id: "p1", Statement: "Prefer explicit"}},
	}
	a := Constitution(c)
	b := ConstitutionWithProvenance(c, []*specv1.ProvenanceEntry{})
	assert.Equal(t, a, b, "with empty provenance, output must be byte-identical to Constitution(c)")
}

func TestConstitutionWithProvenance_AnnotatesPrinciples(t *testing.T) {
	c := &specv1.Constitution{
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "First"},
			{Id: "p2", Statement: "Second"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		{Path: "principles[p2]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- First  (set by: project)")
	assert.Contains(t, out, "- Second  (set by: org)")
}

func TestConstitutionWithProvenance_PartialProvenance_OnlyAnnotatesPresent(t *testing.T) {
	c := &specv1.Constitution{
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "First"},
			{Id: "p2", Statement: "Second"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
		// p2 deliberately missing from provenance
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- First  (set by: project)")
	assert.Contains(t, out, "- Second\n")
	assert.NotContains(t, out, "- Second  (set by:")
}

func TestConstitutionWithProvenance_Constraints(t *testing.T) {
	c := &specv1.Constitution{
		Constraints: []string{"never use eval"},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "constraints[never use eval]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- never use eval  (set by: domain)")
}

func TestConstitutionWithProvenance_Antipatterns(t *testing.T) {
	c := &specv1.Constitution{
		Antipatterns: []*specv1.Antipattern{
			{Pattern: "bad-pat", Why: "because"},
		},
	}
	prov := []*specv1.ProvenanceEntry{
		{Path: "antipatterns[bad-pat]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG},
	}
	out := ConstitutionWithProvenance(c, prov)
	assert.Contains(t, out, "- **bad-pat**: because  (set by: org)")
}

func TestConstitutionWithProvenance_NilConstitution(t *testing.T) {
	out := ConstitutionWithProvenance(nil, []*specv1.ProvenanceEntry{
		{Path: "principles[p1]", Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT},
	})
	assert.Equal(t, "No constitution found.\n", out, "nil constitution falls through to the 'not found' message")
}

// Golden-file byte-stability test for the legacy Constitution renderer.
// If this test fails, downstream scripts/diffs depending on today's
// output format will break. The exact byte output is locked in.
func TestConstitution_LegacyGolden(t *testing.T) {
	c := &specv1.Constitution{
		Name:    "golden",
		Layer:   specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version: 1,
		Principles: []*specv1.Principle{
			{Id: "p1", Statement: "Be explicit"},
		},
		Constraints: []string{"no-eval"},
	}
	out := Constitution(c)
	expected := strings.TrimLeft(`
# golden

` /* metadata table renders inline; capture exact bytes below */, "\n")
	// The actual expected value is filled in after the first run. Capture
	// the canonical bytes and pin them here. Skipped until pinned.
	if expected == "# golden\n\n" {
		t.Logf("captured legacy output:\n%s", out)
		t.Skip("pin the expected value after the first run")
	}
	assert.Equal(t, expected, out)
}
```

### Steps

- [ ] **Step 1: Read existing `internal/render/constitution.go`** to confirm function signatures and helpers.

- [ ] **Step 2: Append `ConstitutionWithProvenance` + `formatItem`** to `internal/render/constitution.go`.

- [ ] **Step 3: Create `internal/render/constitution_test.go`** with the 8 tests above.

- [ ] **Step 4: Run tests**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-8ar-piece-c
go test ./internal/render/ -v -count=1
```

Expected: 7 PASS + 1 SKIP (`TestConstitution_LegacyGolden`).

- [ ] **Step 5: Pin the golden value** — copy the captured legacy output from the SKIP message into the `expected` string in the test. Run again — all 8 PASS.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe @ -m "feat(render): ConstitutionWithProvenance renderer (spgr-8ar piece C)

New ConstitutionWithProvenance(c, []*ProvenanceEntry) renderer
annotates each constitution field with '(set by: <layer>)' based on
the provenance entries. Falls through to the existing Constitution(c)
when provenance is nil or empty (Section 14 renderer invariant).

Also adds a golden-file test pinning Constitution(c)'s output bytes
so scripts depending on today's format are protected against
accidental drift.

Part of spgr-8ar Piece C.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new @ -m "(working)"
```

---

## Task 2: Wire `--show-provenance` flag on `constitution show`

**Files:**

- Modify: `cmd/specgraph/constitution.go`

Current `runConstitutionShow`:

```go
if constitutionShowJSON {
    return printJSON(cmd.OutOrStdout(), resp.Msg)
}
fmt.Print(render.Constitution(resp.Msg.Constitution))
return nil
```

Change to:

```go
if constitutionShowJSON {
    if !constitutionShowProvenance {
        // Without --show-provenance, omit the provenance field from
        // JSON output for byte-stability with today's behavior.
        // protojson omits empty fields by default.
        resp.Msg.Provenance = nil
    }
    return printJSON(cmd.OutOrStdout(), resp.Msg)
}
if constitutionShowProvenance {
    fmt.Print(render.ConstitutionWithProvenance(resp.Msg.Constitution, resp.Msg.Provenance))
} else {
    fmt.Print(render.Constitution(resp.Msg.Constitution))
}
return nil
```

Add the flag:

```go
var constitutionShowProvenance bool

func init() {
    // ... existing flags ...
    constitutionShowCmd.Flags().BoolVar(&constitutionShowProvenance, "show-provenance", false,
        "annotate each field with the layer that set it (text mode); include provenance array (JSON mode)")
}
```

### Steps

- [ ] **Step 1: Add `constitutionShowProvenance` package var** at the top of the file alongside other show flags.

- [ ] **Step 2: Modify `runConstitutionShow`** to branch on the flag for both text and JSON.

- [ ] **Step 3: Register the flag** in `init()`.

- [ ] **Step 4: Build + run CLI tests**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-8ar-piece-c
go build ./...
go test ./cmd/specgraph/... -count=1
```

Expected: clean build, all CLI tests pass.

- [ ] **Step 5: Smoke-test help**

```bash
go build -o /tmp/specgraph-c ./cmd/specgraph
/tmp/specgraph-c constitution show --help 2>&1 | head -15
```

Expected: `--show-provenance` listed.

- [ ] **Step 6: Commit**

```bash
jj --no-pager describe @ -m "feat(cli): constitution show --show-provenance

Adds --show-provenance flag to 'specgraph constitution show'.

Text mode: render constitution via ConstitutionWithProvenance which
annotates each field with '(set by: <layer>)'.

JSON mode: include the 'provenance' array in the output. Without the
flag, the provenance field is cleared from the response before
JSON-marshaling (protojson omits empty fields), preserving today's
byte-stable JSON output for scripts that don't opt in.

Part of spgr-8ar Piece C.

Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>"

jj --no-pager new @ -m "(working)"
```

---

## Task 3: Quality gates + PR

**Files:** none

- [ ] **Step 1: `task check`**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-8ar-piece-c
task check 2>&1 | tail -5
```

Expected: green.

- [ ] **Step 2: `task test:integration`**

```bash
task test:integration 2>&1 | tail -5
```

Expected: green.

- [ ] **Step 3: Direct API e2e**

```bash
go test -tags e2e ./e2e/api/ -count=1 2>&1 | tail -5
```

Expected: green. (Skip `task test:e2e:ui` if it hits the same Docker TLS issue as Piece B; CI will validate.)

- [ ] **Step 4: Update bd + push**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph
bd update spgr-tf1z --notes "Piece C complete. ~3 commits (plan + renderer + CLI). task check green."
bd dolt push
```

- [ ] **Step 5: Push bookmark + open PR**

```bash
cd /Users/SeBrandt/Code/github.com/specgraph-8ar-piece-c
jj --no-pager bookmark set spgr-8ar-piece-c -r @-
jj --no-pager git push --bookmark spgr-8ar-piece-c

cd /Users/SeBrandt/Code/github.com/specgraph
gh auth switch -u seanb4t -h github.com
gh pr create --head spgr-8ar-piece-c --base main \
  --title "spgr-8ar PR C: surface provenance in constitution show" \
  --body "..."
```

PR body summary:

- `ConstitutionWithProvenance` renderer with `(set by: <layer>)` annotations
- `--show-provenance` flag on `constitution show`
- Renderer invariant: empty/nil provenance → byte-identical to today
- Golden-file test pins today's output bytes
- Closes spgr-tf1z

---

## Self-Review

- All renderer tests pass; golden value pinned
- CLI text mode renders annotations when flag set; legacy output when flag unset
- CLI JSON mode includes provenance only when flag set
- `task check` green
- DCO trailer uses `4678+seanb4t@users.noreply.github.com`
- No `git commit` — only `jj describe`

## Plan complete

3 implementation tasks (plus the plan commit). Smaller than B; aim is a clean polish PR. Expected ~3 commits total.
