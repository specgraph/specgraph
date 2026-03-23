# Markdown CLI Output Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace tab-writer text output with markdown rendering across all 11 CLI read commands, add `--json` flags, and create a new `findings list` command.

**Architecture:** New `internal/render/` package with one function per entity type. Functions accept proto types and return markdown strings. CLI commands call render functions for default output and protojson for `--json`. A shared `printJSON` helper in `cmd/specgraph/output.go` centralizes JSON marshaling.

**Tech Stack:** Go, ConnectRPC proto types, `strings.Builder` for markdown assembly, `protojson` for JSON output, standard `testing` for render tests.

**Convention:** All new `.go` files MUST include the standard SPDX license header as the first two lines:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt
```

Code blocks in this plan omit the header for brevity. Run `task license:add` as a safety net.

**Output convention:** All CLI commands switch from `cmd.OutOrStdout()` to `fmt.Print` (writing to `os.Stdout` directly). This is intentional — render functions return strings, and no tests use cobra's `SetOut()`. This simplifies the code without breaking anything.

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/render/markdown.go` | Shared helpers: `metadataTable`, `itemTable`, `section` |
| `internal/render/spec.go` | `Spec(s)` and `SpecList(specs)` |
| `internal/render/edge.go` | `EdgeList(slug, edges)` |
| `internal/render/decision.go` | `Decision(d)` and `DecisionList(ds)` |
| `internal/render/constitution.go` | `Constitution(c)` |
| `internal/render/drift.go` | `DriftReport(reports)` |
| `internal/render/findings.go` | `Findings(findings)` |
| `internal/render/noderef.go` | `NodeRefList(title, refs)` |
| `internal/render/markdown_test.go` | Tests for shared helpers |
| `internal/render/spec_test.go` | Tests for Spec/SpecList |
| `internal/render/edge_test.go` | Tests for EdgeList |
| `internal/render/decision_test.go` | Tests for Decision/DecisionList |
| `internal/render/constitution_test.go` | Tests for Constitution |
| `internal/render/drift_test.go` | Tests for DriftReport |
| `internal/render/findings_test.go` | Tests for Findings |
| `internal/render/noderef_test.go` | Tests for NodeRefList |
| `cmd/specgraph/output.go` | `printJSON(msg)` shared helper, replaces inline protojson in spec.go |
| `cmd/specgraph/findings.go` | New `findings list` command |

### Modified Files

| File | Change |
|------|--------|
| `cmd/specgraph/spec.go` | Replace text rendering with `render.Spec`/`render.SpecList`; `--format` → `--json` flag |
| `cmd/specgraph/edge.go` | Replace tabwriter with `render.EdgeList`; add `--json` flag |
| `cmd/specgraph/decision.go` | Replace text/tabwriter with `render.Decision`/`render.DecisionList`; add `--json` flags |
| `cmd/specgraph/constitution.go` | Replace text rendering with `render.Constitution`; add `--json` flag |
| `cmd/specgraph/lifecycle.go` | Replace drift text with `render.DriftReport`; add `--json` flag |
| `cmd/specgraph/graph.go` | Replace `printNodeRefs` with `render.NodeRefList`; add `--json` flags |
| `cmd/specgraph/table.go` | May become removable — check after all commands migrated |

### Unchanged (Write Commands)

These keep their current one-line confirmation output: `create`, `update`, `spark`, `shape`, `specify`, `decompose`, `approve`, `claim`, `report`, `edge add`, `edge remove`, `constitution emit`, `constitution import`, `drift acknowledge`, `lint`.

---

## Chunk 1: Render Package Foundation + Spec

### Task 1: Shared markdown helpers

**Files:**

- Create: `internal/render/markdown.go`
- Create: `internal/render/markdown_test.go`

- [ ] **Step 1: Write tests for shared helpers**

```go
// internal/render/markdown_test.go
package render

import (
	"strings"
	"testing"
)

func TestMetadataTable(t *testing.T) {
	got := metadataTable([][2]string{
		{"Stage", "specify"},
		{"Priority", "p1"},
	})
	if !strings.Contains(got, "| Field | Value |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| Stage | specify |") {
		t.Error("missing row")
	}
}

func TestMetadataTableEmpty(t *testing.T) {
	got := metadataTable(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestItemTable(t *testing.T) {
	got := itemTable(
		[]string{"Slug", "Stage"},
		[][]string{{"login-api", "specify"}, {"webhook", "shape"}},
	)
	if !strings.Contains(got, "| Slug | Stage |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| login-api | specify |") {
		t.Error("missing row")
	}
}

func TestItemTableEmpty(t *testing.T) {
	got := itemTable([]string{"A"}, nil)
	if got != "" {
		t.Errorf("expected empty string for no rows, got %q", got)
	}
}

func TestSection(t *testing.T) {
	got := section(2, "Details", "Some body text.")
	if !strings.Contains(got, "## Details") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "Some body text.") {
		t.Error("missing body")
	}
}

func TestSectionEmptyBody(t *testing.T) {
	got := section(2, "Empty", "")
	if got != "" {
		t.Errorf("expected empty string for empty body, got %q", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestMetadata -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Implement shared helpers**

```go
// internal/render/markdown.go
package render

import (
	"fmt"
	"strings"
)

// metadataTable renders a two-column | Field | Value | markdown table.
// Returns empty string if pairs is empty.
func metadataTable(pairs [][2]string) string {
	if len(pairs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	for _, p := range pairs {
		fmt.Fprintf(&b, "| %s | %s |\n", p[0], p[1])
	}
	return b.String()
}

// itemTable renders a multi-column markdown table.
// Returns empty string if rows is empty.
func itemTable(headers []string, rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "| %s |\n", strings.Join(headers, " | "))
	seps := make([]string, len(headers))
	for i := range seps {
		seps[i] = "---"
	}
	fmt.Fprintf(&b, "| %s |\n", strings.Join(seps, " | "))
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s |\n", strings.Join(row, " | "))
	}
	return b.String()
}

// section renders a markdown heading with body. Returns empty string if body is empty.
func section(level int, title, body string) string {
	if body == "" {
		return ""
	}
	prefix := strings.Repeat("#", level)
	return fmt.Sprintf("%s %s\n\n%s\n", prefix, title, body)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add shared markdown helpers (metadataTable, itemTable, section)"
```

### Task 2: render.Spec and render.SpecList

**Files:**

- Create: `internal/render/spec.go`
- Create: `internal/render/spec_test.go`

- [ ] **Step 1: Write tests for Spec**

```go
// internal/render/spec_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestSpec(t *testing.T) {
	s := &specv1.Spec{
		Slug:       "login-api",
		Intent:     "Implement OAuth2 login flow",
		Stage:      "specify",
		Priority:   "p1",
		Complexity: "medium",
		Version:    3,
		Lifecycle:  specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK,
	}
	got := Spec(s)
	if !strings.Contains(got, "# login-api") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "> Implement OAuth2 login flow") {
		t.Error("missing blockquote intent")
	}
	if !strings.Contains(got, "| Stage | specify |") {
		t.Error("missing stage row")
	}
	if !strings.Contains(got, "| Priority | p1 |") {
		t.Error("missing priority row")
	}
	if !strings.Contains(got, "| Lifecycle | task |") {
		t.Error("missing lifecycle row")
	}
}

func TestSpecWithNotes(t *testing.T) {
	s := &specv1.Spec{
		Slug:    "test-spec",
		Intent:  "test",
		Stage:   "spark",
		Version: 1,
		Notes:   "Some context notes",
	}
	got := Spec(s)
	if !strings.Contains(got, "## Notes") {
		t.Error("missing notes section")
	}
	if !strings.Contains(got, "Some context notes") {
		t.Error("missing notes content")
	}
}

func TestSpecNil(t *testing.T) {
	got := Spec(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestSpecList(t *testing.T) {
	specs := []*specv1.Spec{
		{Slug: "login-api", Stage: "specify", Priority: "p1", Intent: "OAuth2 login"},
		{Slug: "webhook", Stage: "shape", Priority: "p2", Intent: "Webhooks"},
	}
	got := SpecList(specs)
	if !strings.Contains(got, "| Slug | Stage | Priority | Intent |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| login-api | specify | p1 | OAuth2 login |") {
		t.Error("missing first row")
	}
}

func TestSpecListEmpty(t *testing.T) {
	got := SpecList(nil)
	if !strings.Contains(got, "No specs found.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestSpec -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement Spec and SpecList**

```go
// internal/render/spec.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Spec renders a single spec as markdown. Returns empty string for nil.
func Spec(s *specv1.Spec) string {
	if s == nil {
		return ""
	}
	var b strings.Builder

	// Heading + intent blockquote
	fmt.Fprintf(&b, "# %s\n\n", s.Slug)
	if s.Intent != "" {
		fmt.Fprintf(&b, "> %s\n\n", s.Intent)
	}

	// Metadata table
	pairs := [][2]string{
		{"Stage", s.Stage},
		{"Priority", s.Priority},
		{"Complexity", s.Complexity},
		{"Version", fmt.Sprintf("%d", s.Version)},
		{"Lifecycle", lifecycleString(s.Lifecycle)},
	}
	b.WriteString(metadataTable(pairs))

	// Notes section (conditional)
	b.WriteString(section(2, "Notes", s.Notes))

	return b.String()
}

// SpecList renders a list of specs as a markdown table.
func SpecList(specs []*specv1.Spec) string {
	if len(specs) == 0 {
		return "No specs found.\n"
	}
	headers := []string{"Slug", "Stage", "Priority", "Intent"}
	rows := make([][]string, len(specs))
	for i, s := range specs {
		rows[i] = []string{s.Slug, s.Stage, s.Priority, s.Intent}
	}
	return itemTable(headers, rows)
}

func lifecycleString(lc specv1.SpecLifecycle) string {
	switch lc {
	case specv1.SpecLifecycle_SPEC_LIFECYCLE_TASK:
		return "task"
	case specv1.SpecLifecycle_SPEC_LIFECYCLE_LIVING:
		return "living"
	default:
		return "task"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add Spec and SpecList markdown renderers"
```

### Task 3: Shared printJSON helper + wire spec commands

**Files:**

- Create: `cmd/specgraph/output.go`
- Modify: `cmd/specgraph/spec.go`

- [ ] **Step 1: Create printJSON helper**

```go
// cmd/specgraph/output.go
package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printJSON marshals a proto message to pretty-printed JSON on stdout.
func printJSON(msg proto.Message) error {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if _, err := os.Stdout.Write(data); err != nil {
		return err
	}
	fmt.Println()
	return nil
}
```

- [ ] **Step 2: Update spec.go — replace `--format` with `--json`, wire render functions**

In `cmd/specgraph/spec.go`:

1. Remove `showFormat` variable and `--format` flag registration.
2. Add `showJSON` and `listJSON` boolean flags.
3. Replace `runShow` text rendering with `render.Spec`.
4. Replace `runList` tabwriter rendering with `render.SpecList`.
5. Remove `protojson` and `tabwriter` imports, add `render` import.

The updated `runList`:

```go
func runList(_ *cobra.Command, _ []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.ListSpecs(context.Background(), connect.NewRequest(&specv1.ListSpecsRequest{
		Stage:    listStage,
		Priority: listPriority,
	}))
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	if listJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.SpecList(resp.Msg.Specs))
	return nil
}
```

The updated `runShow`:

```go
func runShow(_ *cobra.Command, args []string) error {
	client, err := specClient()
	if err != nil {
		return err
	}
	resp, err := client.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	if showJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.Spec(resp.Msg.GetSpec()))
	return nil
}
```

The updated init (flag section only):

```go
	listCmd.Flags().StringVar(&listStage, "stage", "", "filter by stage")
	listCmd.Flags().StringVar(&listPriority, "priority", "", "filter by priority")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(listCmd)

	showCmd.Flags().BoolVar(&showJSON, "json", false, "output as JSON")
	rootCmd.AddCommand(showCmd)
```

- [ ] **Step 3: Verify build passes**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 4: Commit**

```text
jj commit -m "feat(cli): replace spec text output with markdown rendering, --format→--json"
```

---

## Chunk 2: Edge, Decision, NodeRef Renderers

### Task 4: render.EdgeList

**Files:**

- Create: `internal/render/edge.go`
- Create: `internal/render/edge_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/edge_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestEdgeList(t *testing.T) {
	edges := []*specv1.Edge{
		{FromId: "login-api", ToId: "token-storage", EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON},
		{FromId: "api-gateway", ToId: "login-api", EdgeType: specv1.EdgeType_EDGE_TYPE_BLOCKS},
	}
	got := EdgeList("login-api", edges)
	if !strings.Contains(got, "## Edges for login-api") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Type | Direction | Target |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| DEPENDS_ON | outgoing | token-storage |") {
		t.Error("missing outgoing edge")
	}
	if !strings.Contains(got, "| BLOCKS | incoming | api-gateway |") {
		t.Error("missing incoming edge")
	}
}

func TestEdgeListEmpty(t *testing.T) {
	got := EdgeList("test", nil)
	if !strings.Contains(got, "No edges found.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestEdge -v`
Expected: FAIL

- [ ] **Step 3: Implement EdgeList**

```go
// internal/render/edge.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// EdgeList renders edges for a slug as a markdown table with direction.
func EdgeList(slug string, edges []*specv1.Edge) string {
	if len(edges) == 0 {
		return "No edges found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Edges for %s\n\n", slug)

	headers := []string{"Type", "Direction", "Target"}
	rows := make([][]string, len(edges))
	for i, e := range edges {
		dir, target := edgeDirection(slug, e)
		rows[i] = []string{edgeTypeName(e.EdgeType), dir, target}
	}
	b.WriteString(itemTable(headers, rows))
	return b.String()
}

func edgeDirection(slug string, e *specv1.Edge) (direction, target string) {
	if e.FromId == slug {
		return "outgoing", e.ToId
	}
	return "incoming", e.FromId
}

func edgeTypeName(et specv1.EdgeType) string {
	s := et.String()
	return strings.TrimPrefix(s, "EDGE_TYPE_")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestEdge -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add EdgeList markdown renderer"
```

### Task 5: render.Decision and render.DecisionList

**Files:**

- Create: `internal/render/decision.go`
- Create: `internal/render/decision_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/decision_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestDecision(t *testing.T) {
	d := &specv1.Decision{
		Slug:     "use-rotating-tokens",
		Title:    "Use rotating refresh tokens",
		Status:   specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision: "Use rotating refresh tokens with family revocation.",
		Rationale: "Security audit requires rotation.",
	}
	got := Decision(d)
	if !strings.Contains(got, "# use-rotating-tokens") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "> Use rotating refresh tokens") {
		t.Error("missing title blockquote")
	}
	if !strings.Contains(got, "| Status | accepted |") {
		t.Error("missing status")
	}
	if !strings.Contains(got, "**Decision:**") {
		t.Error("missing decision section")
	}
	if !strings.Contains(got, "**Rationale:**") {
		t.Error("missing rationale section")
	}
}

func TestDecisionSuperseded(t *testing.T) {
	d := &specv1.Decision{
		Slug:         "old-auth",
		Status:       specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
		SupersededBy: "new-auth",
	}
	got := Decision(d)
	if !strings.Contains(got, "| Superseded By | new-auth |") {
		t.Error("missing superseded_by")
	}
}

func TestDecisionNil(t *testing.T) {
	got := Decision(nil)
	if got != "" {
		t.Errorf("expected empty for nil, got %q", got)
	}
}

func TestDecisionList(t *testing.T) {
	ds := []*specv1.Decision{
		{Slug: "use-memgraph", Status: specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, Title: "Use Memgraph"},
		{Slug: "old-db", Status: specv1.DecisionStatus_DECISION_STATUS_DEPRECATED, Title: "Use Postgres"},
	}
	got := DecisionList(ds)
	if !strings.Contains(got, "| Slug | Status | Title |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| use-memgraph | accepted | Use Memgraph |") {
		t.Error("missing first row")
	}
}

func TestDecisionListEmpty(t *testing.T) {
	got := DecisionList(nil)
	if !strings.Contains(got, "No decisions found.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestDecision -v`
Expected: FAIL

- [ ] **Step 3: Implement Decision and DecisionList**

```go
// internal/render/decision.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Decision renders a single decision as markdown.
func Decision(d *specv1.Decision) string {
	if d == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", d.Slug)
	if d.Title != "" {
		fmt.Fprintf(&b, "> %s\n\n", d.Title)
	}

	pairs := [][2]string{
		{"Status", decisionStatusString(d.Status)},
	}
	if d.SupersededBy != "" {
		pairs = append(pairs, [2]string{"Superseded By", d.SupersededBy})
	}
	b.WriteString(metadataTable(pairs))

	if d.Decision != "" {
		fmt.Fprintf(&b, "\n**Decision:** %s\n", d.Decision)
	}
	if d.Rationale != "" {
		fmt.Fprintf(&b, "\n**Rationale:** %s\n", d.Rationale)
	}

	return b.String()
}

// DecisionList renders a list of decisions as a markdown table.
func DecisionList(ds []*specv1.Decision) string {
	if len(ds) == 0 {
		return "No decisions found.\n"
	}
	headers := []string{"Slug", "Status", "Title"}
	rows := make([][]string, len(ds))
	for i, d := range ds {
		rows[i] = []string{d.Slug, decisionStatusString(d.Status), d.Title}
	}
	return itemTable(headers, rows)
}

func decisionStatusString(s specv1.DecisionStatus) string {
	switch s {
	case specv1.DecisionStatus_DECISION_STATUS_PROPOSED:
		return "proposed"
	case specv1.DecisionStatus_DECISION_STATUS_ACCEPTED:
		return "accepted"
	case specv1.DecisionStatus_DECISION_STATUS_DEPRECATED:
		return "deprecated"
	case specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED:
		return "superseded"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestDecision -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add Decision and DecisionList markdown renderers"
```

### Task 6: render.NodeRefList

**Files:**

- Create: `internal/render/noderef.go`
- Create: `internal/render/noderef_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/noderef_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestNodeRefList(t *testing.T) {
	refs := []*specv1.NodeRef{
		{Slug: "token-storage", Stage: "approved"},
		{Slug: "crypto-utils", Stage: "done"},
	}
	got := NodeRefList("Dependencies", refs)
	if !strings.Contains(got, "## Dependencies") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Slug | Stage |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "| token-storage | approved |") {
		t.Error("missing first row")
	}
}

func TestNodeRefListEmpty(t *testing.T) {
	got := NodeRefList("Dependencies", nil)
	if !strings.Contains(got, "None.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestNodeRef -v`
Expected: FAIL

- [ ] **Step 3: Implement NodeRefList**

```go
// internal/render/noderef.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// NodeRefList renders a titled list of node references as a markdown table.
func NodeRefList(title string, refs []*specv1.NodeRef) string {
	if len(refs) == 0 {
		return "None.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", title)

	headers := []string{"Slug", "Stage"}
	rows := make([][]string, len(refs))
	for i, r := range refs {
		rows[i] = []string{r.Slug, r.Stage}
	}
	b.WriteString(itemTable(headers, rows))
	return b.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestNodeRef -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add NodeRefList markdown renderer"
```

---

## Chunk 3: Constitution, Drift, Findings Renderers

### Task 7: render.Constitution

**Files:**

- Create: `internal/render/constitution.go`
- Create: `internal/render/constitution_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/constitution_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestConstitution(t *testing.T) {
	c := &specv1.Constitution{
		Name:    "SpecGraph",
		Layer:   specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Version: 2,
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{Primary: "Go"},
		},
		Principles: []*specv1.Principle{
			{Statement: "Specs are graph nodes"},
		},
		Constraints: []string{"No ORM usage"},
		Antipatterns: []*specv1.Antipattern{
			{Pattern: "God objects", Why: "Violates SRP"},
		},
		References: []*specv1.Reference{
			{ReferenceType: specv1.ReferenceType_REFERENCE_TYPE_ADR, Path: "docs/adr/002-content-hash.md"},
		},
	}
	got := Constitution(c)
	if !strings.Contains(got, "# SpecGraph") {
		t.Error("missing heading")
	}
	if !strings.Contains(got, "| Layer | project |") {
		t.Error("missing layer")
	}
	if !strings.Contains(got, "| Primary Language | Go |") {
		t.Error("missing tech")
	}
	if !strings.Contains(got, "## Principles") {
		t.Error("missing principles section")
	}
	if !strings.Contains(got, "- Specs are graph nodes") {
		t.Error("missing principle")
	}
	if !strings.Contains(got, "## Constraints") {
		t.Error("missing constraints section")
	}
	if !strings.Contains(got, "## Anti-patterns") {
		t.Error("missing antipatterns section")
	}
	if !strings.Contains(got, "- **God objects**: Violates SRP") {
		t.Error("missing antipattern")
	}
	if !strings.Contains(got, "## References") {
		t.Error("missing references section")
	}
	if !strings.Contains(got, "[ADR] docs/adr/002-content-hash.md") {
		t.Error("missing reference")
	}
}

func TestConstitutionNil(t *testing.T) {
	got := Constitution(nil)
	if !strings.Contains(got, "No constitution found.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestConstitution -v`
Expected: FAIL

- [ ] **Step 3: Implement Constitution**

```go
// internal/render/constitution.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Constitution renders a constitution as markdown.
func Constitution(c *specv1.Constitution) string {
	if c == nil {
		return "No constitution found.\n"
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", c.GetName())

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

	// Principles
	if ps := c.GetPrinciples(); len(ps) > 0 {
		b.WriteString("\n## Principles\n\n")
		for _, p := range ps {
			fmt.Fprintf(&b, "- %s\n", p.GetStatement())
		}
	}

	// Constraints
	if cs := c.GetConstraints(); len(cs) > 0 {
		b.WriteString("\n## Constraints\n\n")
		for _, ct := range cs {
			fmt.Fprintf(&b, "- %s\n", ct)
		}
	}

	// Anti-patterns
	if aps := c.GetAntipatterns(); len(aps) > 0 {
		b.WriteString("\n## Anti-patterns\n\n")
		for _, ap := range aps {
			fmt.Fprintf(&b, "- **%s**: %s\n", ap.GetPattern(), ap.GetWhy())
		}
	}

	// References
	if refs := c.GetReferences(); len(refs) > 0 {
		b.WriteString("\n## References\n\n")
		for _, ref := range refs {
			fmt.Fprintf(&b, "- [%s] %s\n", referenceTypeName(ref.GetReferenceType()), ref.GetPath())
		}
	}

	return b.String()
}

func referenceTypeName(rt specv1.ReferenceType) string {
	switch rt {
	case specv1.ReferenceType_REFERENCE_TYPE_ADR:
		return "ADR"
	case specv1.ReferenceType_REFERENCE_TYPE_SPEC:
		return "Spec"
	case specv1.ReferenceType_REFERENCE_TYPE_DOC:
		return "Doc"
	case specv1.ReferenceType_REFERENCE_TYPE_URL:
		return "URL"
	default:
		return "Ref"
	}
}

func constitutionLayerString(l specv1.ConstitutionLayer) string {
	switch l {
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:
		return "user"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:
		return "org"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT:
		return "project"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:
		return "domain"
	default:
		return "unspecified"
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestConstitution -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add Constitution markdown renderer"
```

### Task 8: render.DriftReport

**Files:**

- Create: `internal/render/drift.go`
- Create: `internal/render/drift_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/drift_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestDriftReport(t *testing.T) {
	reports := []*specv1.DriftReport{
		{
			SpecSlug: "login-api",
			Items: []*specv1.DriftItem{
				{
					Type:         specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
					Severity:     specv1.DriftSeverity_DRIFT_SEVERITY_HIGH,
					Description:  "upstream token-storage changed",
					UpstreamSlug: "token-storage",
				},
			},
		},
	}
	got := DriftReport(reports)
	if !strings.Contains(got, "## login-api") {
		t.Error("missing spec heading")
	}
	if !strings.Contains(got, "| DEPENDENCY | HIGH | upstream token-storage changed | token-storage |") {
		t.Error("missing drift item row")
	}
}

func TestDriftReportWithError(t *testing.T) {
	reports := []*specv1.DriftReport{
		{SpecSlug: "broken", ErrorMessage: "storage unavailable"},
	}
	got := DriftReport(reports)
	if !strings.Contains(got, "**Error:** storage unavailable") {
		t.Error("missing error message")
	}
}

func TestDriftReportEmpty(t *testing.T) {
	got := DriftReport(nil)
	if !strings.Contains(got, "No drift detected.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestDrift -v`
Expected: FAIL

- [ ] **Step 3: Implement DriftReport**

```go
// internal/render/drift.go
package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// DriftReport renders drift reports as markdown grouped by spec.
func DriftReport(reports []*specv1.DriftReport) string {
	if len(reports) == 0 {
		return "No drift detected.\n"
	}

	// Filter to reports that have items or errors.
	var hasContent bool
	for _, r := range reports {
		if len(r.GetItems()) > 0 || r.GetErrorMessage() != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return "No drift detected.\n"
	}

	var b strings.Builder
	for _, r := range reports {
		if len(r.GetItems()) == 0 && r.GetErrorMessage() == "" {
			continue
		}
		fmt.Fprintf(&b, "## %s\n\n", r.GetSpecSlug())

		if items := r.GetItems(); len(items) > 0 {
			headers := []string{"Type", "Severity", "Description", "Upstream"}
			rows := make([][]string, len(items))
			for i, item := range items {
				rows[i] = []string{
					driftTypeName(item.GetType()),
					driftSeverityName(item.GetSeverity()),
					item.GetDescription(),
					item.GetUpstreamSlug(),
				}
			}
			b.WriteString(itemTable(headers, rows))
		}

		if errMsg := r.GetErrorMessage(); errMsg != "" {
			fmt.Fprintf(&b, "\n**Error:** %s\n", errMsg)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func driftTypeName(dt specv1.DriftType) string {
	s := dt.String()
	return strings.TrimPrefix(s, "DRIFT_TYPE_")
}

func driftSeverityName(ds specv1.DriftSeverity) string {
	s := ds.String()
	return strings.TrimPrefix(s, "DRIFT_SEVERITY_")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestDrift -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add DriftReport markdown renderer"
```

### Task 9: render.Findings

**Files:**

- Create: `internal/render/findings.go`
- Create: `internal/render/findings_test.go`

- [ ] **Step 1: Write tests**

```go
// internal/render/findings_test.go
package render

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestFindings(t *testing.T) {
	fs := []*specv1.AnalyticalFinding{
		{
			PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
			Summary:  "Missing constraint coverage",
			Detail:   "Spec does not address constraint C3.",
		},
		{
			PassType: specv1.PassType_PASS_TYPE_RED_TEAM,
			Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
			Summary:  "Edge case unhandled",
		},
	}
	got := Findings(fs)
	if !strings.Contains(got, "| Pass | Severity | Summary |") {
		t.Error("missing header")
	}
	if !strings.Contains(got, "CONSTITUTION_CHECK") {
		t.Error("missing pass type")
	}
	if !strings.Contains(got, "CRITICAL") {
		t.Error("missing severity")
	}
}

func TestFindingsEmpty(t *testing.T) {
	got := Findings(nil)
	if !strings.Contains(got, "No findings.") {
		t.Error("expected empty message")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/ -run TestFindings -v`
Expected: FAIL

- [ ] **Step 3: Implement Findings**

```go
// internal/render/findings.go
package render

import (
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Findings renders analytical findings as a markdown table.
func Findings(fs []*specv1.AnalyticalFinding) string {
	if len(fs) == 0 {
		return "No findings.\n"
	}
	headers := []string{"Pass", "Severity", "Summary"}
	rows := make([][]string, len(fs))
	for i, f := range fs {
		rows[i] = []string{
			passTypeName(f.GetPassType()),
			findingSeverityName(f.GetSeverity()),
			f.GetSummary(),
		}
	}
	return itemTable(headers, rows)
}

func passTypeName(pt specv1.PassType) string {
	s := pt.String()
	return strings.TrimPrefix(s, "PASS_TYPE_")
}

func findingSeverityName(fs specv1.FindingSeverity) string {
	s := fs.String()
	return strings.TrimPrefix(s, "FINDING_SEVERITY_")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -run TestFindings -v`
Expected: PASS

- [ ] **Step 5: Commit**

```text
jj commit -m "feat(render): add Findings markdown renderer"
```

---

## Chunk 4: Wire All CLI Commands

### Task 10: Wire edge list command

**Files:**

- Modify: `cmd/specgraph/edge.go`

- [ ] **Step 1: Update runEdgeList to use render.EdgeList**

Replace the `runEdgeList` function body after getting the response. Add `--json` flag to `edgeListCmd`. The slug is `args[0]`.

Changes:

1. Add `edgeListJSON` bool var
2. Register `--json` flag in init
3. Replace tabwriter rendering with `render.EdgeList(args[0], edges)`
4. Add JSON path with `printJSON(resp.Msg)`
5. Remove `tabwriter` import if no longer used in file (check `edgeAddCmd` — it uses `fmt.Printf`, not tabwriter)

Updated rendering section of `runEdgeList`:

```go
	if edgeListJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.EdgeList(args[0], edges))
	return nil
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): replace edge list text with markdown, add --json"
```

### Task 11: Wire decision commands

**Files:**

- Modify: `cmd/specgraph/decision.go`

- [ ] **Step 1: Update decision show and list**

Changes:

1. Add `decisionShowJSON` and `decisionListJSON` bool vars
2. Register `--json` flags in init for both subcommands
3. Replace `runDecisionShow` text output with `render.Decision(d)`
4. Replace `runDecisionList` tabwriter output with `render.DecisionList(decisions)`
5. Remove `tabwriter` import

Updated `runDecisionShow`:

```go
	if decisionShowJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.Decision(resp.Msg.GetDecision()))
	return nil
```

Updated `runDecisionList`:

```go
	if decisionListJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.DecisionList(resp.Msg.Decisions))
	return nil
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): replace decision text with markdown, add --json"
```

### Task 12: Wire constitution show command

**Files:**

- Modify: `cmd/specgraph/constitution.go`

- [ ] **Step 1: Update constitution show**

Changes:

1. Add `constitutionShowJSON` bool var
2. Register `--json` flag for `constitutionShowCmd`
3. Replace `runConstitutionShow` text output with `render.Constitution(c)`
4. Remove nil check + "No constitution found" (render.Constitution handles nil)
5. Add JSON path

Updated `runConstitutionShow`:

```go
func runConstitutionShow(_ *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}
	resp, err := client.GetConstitution(context.Background(), connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return fmt.Errorf("get constitution: %w", err)
	}
	if constitutionShowJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.Constitution(resp.Msg.Constitution))
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): replace constitution show text with markdown, add --json"
```

### Task 13: Wire graph commands (deps, ready, critical-path, impact)

**Files:**

- Modify: `cmd/specgraph/graph.go`

- [ ] **Step 1: Update all graph commands to use render.NodeRefList**

Changes:

1. Add `depsJSON`, `readyJSON`, `criticalPathJSON`, `impactJSON` bool vars
2. Register `--json` flags in init for all four commands
3. Replace `printNodeRefs` calls with `render.NodeRefList`
4. Delete `printNodeRefs` function entirely
5. Remove `tabwriter` import

Updated pattern for each command (example: `runDeps`):

```go
func runDeps(_ *cobra.Command, args []string) error {
	client, err := graphClient()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if depsTransitive {
		resp, tdErr := client.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{Slug: args[0]}))
		if tdErr != nil {
			return fmt.Errorf("get transitive deps: %w", tdErr)
		}
		if depsJSON {
			return printJSON(resp.Msg)
		}
		fmt.Print(render.NodeRefList("Dependencies (transitive)", resp.Msg.Dependencies))
		return nil
	}

	resp, err := client.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{Slug: args[0]}))
	if err != nil {
		return fmt.Errorf("get dependencies: %w", err)
	}
	if depsJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.NodeRefList("Dependencies", resp.Msg.Dependencies))
	return nil
}
```

The other three commands follow the same structure. Key render calls:

```go
// runReady:
fmt.Print(render.NodeRefList("Ready Specs", resp.Msg.Ready))

// runCriticalPath:
fmt.Print(render.NodeRefList("Critical Path", resp.Msg.Path))

// runImpact:
fmt.Print(render.NodeRefList("Impacted Specs", resp.Msg.Impacted))
```

Each adds its own `*JSON` bool var and `--json` flag in init, with `printJSON(resp.Msg)` for the JSON path.

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): replace graph commands text with markdown, add --json, remove printNodeRefs"
```

### Task 14: Wire drift command

**Files:**

- Modify: `cmd/specgraph/lifecycle.go`

- [ ] **Step 1: Update runDrift to use render.DriftReport**

Changes:

1. Add `driftJSON` bool var
2. Register `--json` flag for `driftCmd`
3. Replace inline drift rendering with `render.DriftReport`
4. Keep error/drift exit code logic — check reports for items/errors after rendering

The drift command has special behavior: it needs to return errors for exit codes. The render function handles display; the CLI still needs to inspect reports for exit logic.

Updated `runDrift`:

```go
func runDrift(_ *cobra.Command, args []string) error {
	scope, err := driftScopeToProto(driftScope)
	if err != nil {
		return err
	}
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	req := &specv1.DriftCheckRequest{Scope: scope}
	if len(args) > 0 {
		req.Slug = args[0]
	}
	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("drift check: %w", err)
	}
	if driftJSON {
		return printJSON(resp.Msg)
	}

	reports := resp.Msg.GetReports()
	fmt.Print(render.DriftReport(reports))

	// Exit code logic: check for errors and drift in reports.
	var hasErrors, hasDrift bool
	for _, r := range reports {
		if r.GetErrorMessage() != "" {
			hasErrors = true
		}
		if len(r.GetItems()) > 0 {
			hasDrift = true
		}
	}
	if hasErrors {
		return fmt.Errorf("drift check completed with errors")
	}
	if hasDrift {
		return fmt.Errorf("drift detected")
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): replace drift text with markdown, add --json"
```

---

## Chunk 5: Findings Command + Cleanup

### Task 15: Create findings list command

**Files:**

- Create: `cmd/specgraph/findings.go`

- [ ] **Step 1: Create the findings command file**

```go
// cmd/specgraph/findings.go
package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

func analyticalPassClient() (specgraphv1connect.AnalyticalPassServiceClient, error) {
	return newClient(specgraphv1connect.NewAnalyticalPassServiceClient)
}

var findingsCmd = &cobra.Command{
	Use:   "findings",
	Short: "Manage analytical findings",
}

var findingsListCmd = &cobra.Command{
	Use:   "list <slug>",
	Short: "List findings for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindingsList,
}

var (
	findingsListPassType string
	findingsListJSON     bool
)

func init() {
	findingsListCmd.Flags().StringVar(&findingsListPassType, "pass-type", "", "filter by pass type")
	findingsListCmd.Flags().BoolVar(&findingsListJSON, "json", false, "output as JSON")
	findingsCmd.AddCommand(findingsListCmd)
	rootCmd.AddCommand(findingsCmd)
}

// passTypeMap maps friendly CLI names to proto enum values,
// following the same pattern as driftScopeToProtoMap in lifecycle.go.
var passTypeMap = map[string]specv1.PassType{
	"constitution-check": specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
	"red-team":           specv1.PassType_PASS_TYPE_RED_TEAM,
	"peripheral-vision":  specv1.PassType_PASS_TYPE_PERIPHERAL_VISION,
	"consistency":        specv1.PassType_PASS_TYPE_CONSISTENCY,
	"simplicity":         specv1.PassType_PASS_TYPE_SIMPLICITY,
}

func runFindingsList(_ *cobra.Command, args []string) error {
	client, err := analyticalPassClient()
	if err != nil {
		return err
	}

	req := &specv1.ListFindingsRequest{Slug: args[0]}
	if findingsListPassType != "" {
		pt, ok := passTypeMap[findingsListPassType]
		if !ok {
			return fmt.Errorf("unknown pass type %q; valid: constitution-check, red-team, peripheral-vision, consistency, simplicity", findingsListPassType)
		}
		req.PassType = pt
	}

	resp, err := client.ListFindings(context.Background(), connect.NewRequest(req))
	if err != nil {
		return fmt.Errorf("list findings: %w", err)
	}
	if findingsListJSON {
		return printJSON(resp.Msg)
	}
	fmt.Print(render.Findings(resp.Msg.Findings))
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: success

- [ ] **Step 3: Commit**

```text
jj commit -m "feat(cli): add findings list command with markdown output"
```

### Task 16: Verify table.go status

**Files:**

- Verify: `cmd/specgraph/table.go`

`table.go` is still used by `sync.go` and `prime.go` (write/operational commands not touched by this plan). It stays.

- [ ] **Step 1: Confirm tableWriter still has references**

Run: `grep -r "tableWriter" cmd/specgraph/ --include="*.go" | grep -v table.go`
Expected: references in `sync.go` and/or `prime.go` — confirms `table.go` must be retained. No changes needed.

### Task 17: Run full quality gate

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: all checks pass (fmt, lint, build, unit tests)

- [ ] **Step 2: Add license headers to all new files**

Run: `task license:add`

This adds SPDX headers to all new files created in Tasks 1-9 and 15. Verify with:
`grep -rL "SPDX-License-Identifier" internal/render/ cmd/specgraph/output.go cmd/specgraph/findings.go`

- [ ] **Step 3: Fix any lint issues**

Address golangci-lint findings. Common issues:

- Unused imports (tabwriter, protojson in spec.go)

- [ ] **Step 4: Run task pr-prep**

Run: `task pr-prep`
Expected: all checks pass including integration and e2e tests

- [ ] **Step 5: Commit any fixes**

```text
jj commit -m "fix: address lint and formatting issues"
```

---

## Notes for Implementation

### JSON output convention

All `--json` flags use `printJSON(resp.Msg)` which outputs the full response wrapper (e.g., `GetSpecResponse` with `spec` field). This preserves the response structure and is consistent with how `protojson` works — consumers get the complete typed response.

### Empty collection messages

The design spec's markdown format examples show tables for populated data. For empty collections, render functions return a simple message (`"No specs found.\n"`, `"None.\n"`, etc.) matching the existing CLI behavior.

### render.Spec degradation

Per the design spec, `render.Spec` currently renders only metadata (stage, priority, complexity, version, lifecycle, notes). Authoring output sections (Spark, Shape, Specify, Decompose) will be added when `GetSpecResponse` is extended to include authoring data — that's a separate future task.

### Exit code preservation

The `drift` command returns `fmt.Errorf("drift detected")` for non-zero exit when drift exists. This behavior is preserved — `render.DriftReport` handles display, the CLI function handles exit logic.

### Edge direction

`render.EdgeList` derives direction by comparing `fromId` against the queried slug. `FromId == slug` → outgoing, otherwise incoming. The edge type name strips the `EDGE_TYPE_` prefix for cleaner display.
