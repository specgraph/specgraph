# Confluence Publishing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish SpecGraph specs to Confluence as PRDs, SDDs, and ADRs in native ADF format, with auto-publish on stage transitions and bidirectional comment ingestion.

**Architecture:** Three-layer design. `internal/render/` defines a `Renderer` interface with `markdown/` and `adf/` sub-packages. `internal/publish/` defines `Publisher` and `FeedbackSource` interfaces with a Confluence implementation. The existing render package is refactored — entity-level renderers move to `markdown/`, the root becomes interface-only. New `PublishService` proto + ConnectRPC handler. CLI commands under `specgraph confluence`.

**Tech Stack:** Go, ConnectRPC, ADF JSON (Atlassian Document Format), Confluence REST API v2, `pgx/v5` for page mapping storage, standard `testing`.

**Convention:** All new `.go` files MUST include the standard SPDX license header as the first two lines:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```

Code blocks in this plan omit the header for brevity. Run `task license:add` as a safety net.

**Design Spec:** `docs/designs/2026-04-10-confluence-publishing-design.md`

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/render/render.go` | `Renderer` interface, `Document`, `DocumentKind` types |
| `internal/render/adf/adf.go` | Package doc + ADF node type constants |
| `internal/render/adf/builder.go` | Fluent ADF document builder |
| `internal/render/adf/builder_test.go` | Builder tests |
| `internal/render/adf/macros.go` | Confluence macro helpers (status, panel, expand, page-properties) |
| `internal/render/adf/macros_test.go` | Macro tests |
| `internal/render/adf/prd.go` | ADF PRD renderer (Spark + Shape) |
| `internal/render/adf/prd_test.go` | PRD renderer tests |
| `internal/render/adf/sdd.go` | ADF SDD renderer (Specify + Decompose) |
| `internal/render/adf/sdd_test.go` | SDD renderer tests |
| `internal/render/adf/adr.go` | ADF ADR renderer (MADR format) |
| `internal/render/adf/adr_test.go` | ADR renderer tests |
| `internal/render/markdown/prd.go` | Markdown PRD renderer |
| `internal/render/markdown/prd_test.go` | PRD renderer tests |
| `internal/render/markdown/sdd.go` | Markdown SDD renderer |
| `internal/render/markdown/sdd_test.go` | SDD renderer tests |
| `internal/render/markdown/adr.go` | Markdown ADR renderer (MADR format) |
| `internal/render/markdown/adr_test.go` | ADR renderer tests |
| `internal/publish/publish.go` | `Publisher`, `FeedbackSource` interfaces, types |
| `internal/publish/orchestrator.go` | Stage transition → render → publish wiring |
| `internal/publish/orchestrator_test.go` | Orchestrator tests |
| `internal/publish/confluence/client.go` | Confluence REST API client |
| `internal/publish/confluence/client_test.go` | Client tests (httptest) |
| `internal/publish/confluence/publisher.go` | Publisher impl — page tree management |
| `internal/publish/confluence/publisher_test.go` | Publisher tests |
| `internal/publish/confluence/feedback.go` | FeedbackSource impl — comment polling + routing |
| `internal/publish/confluence/feedback_test.go` | Feedback tests |
| `internal/publish/confluence/config.go` | Configuration types |
| `internal/storage/publish.go` | `PublishBackend` interface + domain types |
| `internal/storage/postgres/publish.go` | Postgres implementation |
| `internal/storage/postgres/publish_test.go` | Integration tests |
| `internal/storage/postgres/migrations/NNNN_publish_mappings.sql` | Migration DDL |
| `proto/specgraph/v1/publish.proto` | PublishService proto definition |
| `internal/server/publish_handler.go` | ConnectRPC handler |
| `internal/server/publish_handler_test.go` | Handler tests |
| `cmd/specgraph/confluence.go` | CLI commands |
| `cmd/specgraph/confluence_test.go` | CLI tests |

### Modified Files

| File | Change |
|------|--------|
| `internal/render/markdown.go` | Move to `internal/render/markdown/helpers.go`, change package to `markdown` |
| `internal/render/spec.go` | Move to `internal/render/markdown/spec.go`, change package to `markdown` |
| `internal/render/decision.go` | Move to `internal/render/markdown/decision.go`, change package to `markdown` |
| `internal/render/authoring.go` | Move to `internal/render/markdown/authoring.go`, change package to `markdown` |
| `internal/render/changelog.go` | Move to `internal/render/markdown/changelog.go` |
| `internal/render/constitution.go` | Move to `internal/render/markdown/constitution.go` |
| `internal/render/conversation.go` | Move to `internal/render/markdown/conversation.go` |
| `internal/render/drift.go` | Move to `internal/render/markdown/drift.go` |
| `internal/render/edge.go` | Move to `internal/render/markdown/edge.go` |
| `internal/render/findings.go` | Move to `internal/render/markdown/findings.go` |
| `internal/render/noderef.go` | Move to `internal/render/markdown/noderef.go` |
| `internal/render/pass.go` | Move to `internal/render/markdown/pass.go` |
| `internal/render/slice.go` | Move to `internal/render/markdown/slice.go` |
| All corresponding `*_test.go` files | Move to `internal/render/markdown/` |
| `cmd/specgraph/spec.go` | Update import `render` → `markdown` |
| `cmd/specgraph/decision.go` | Update import `render` → `markdown` |
| `cmd/specgraph/slice.go` | Update import `render` → `markdown` |
| `cmd/specgraph/edge.go` | Update import `render` → `markdown` |
| `cmd/specgraph/lifecycle.go` | Update import `render` → `markdown` |
| `cmd/specgraph/graph.go` | Update import `render` → `markdown` |
| `cmd/specgraph/pass.go` | Update import `render` → `markdown` |
| `cmd/specgraph/findings.go` | Update import `render` → `markdown` |
| `cmd/specgraph/changes.go` | Update import `render` → `markdown` |
| `cmd/specgraph/conversation.go` | Update import `render` → `markdown` |
| `cmd/specgraph/constitution.go` | Update import `render` → `markdown` |
| `cmd/specgraph/report.go` | Update import `render` → `markdown` |
| `cmd/specgraph/main.go` | Register `confluenceCmd` |
| `internal/config/config.go` | Add `Publish` field to `Config` |
| `internal/storage/storage.go` | Add `PublishBackend` to `ScopedBackend` |
| `internal/server/server.go` | Add `RegisterPublishService` |

---

## Phase 1: Foundation — Interfaces & Types

### Task 1: Renderer Interface + Document Types

**Files:**
- Create: `internal/render/render.go`

- [ ] **Step 1: Create the Renderer interface and Document types**

```go
// internal/render/render.go
package render

import (
	"context"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// DocumentKind identifies the type of rendered document.
type DocumentKind int

const (
	DocumentPRD DocumentKind = iota
	DocumentSDD
	DocumentADR
)

// String returns the human-readable name of the document kind.
func (k DocumentKind) String() string {
	switch k {
	case DocumentPRD:
		return "PRD"
	case DocumentSDD:
		return "SDD"
	case DocumentADR:
		return "ADR"
	default:
		return "unknown"
	}
}

// Document is a format-agnostic rendered document.
type Document struct {
	Kind       DocumentKind
	Title      string
	Body       []byte            // Rendered content (ADF JSON, Markdown, etc.)
	SpecSlug   string            // The spec this document belongs to
	DecisionID string            // For ADR only: the decision slug
	Metadata   map[string]string // Optional key-value pairs for the publisher
}

// Renderer transforms spec data into structured documents.
// Implementations are format-specific (ADF, Markdown, etc.).
type Renderer interface {
	RenderPRD(ctx context.Context, spec *specv1.Spec) (Document, error)
	RenderSDD(ctx context.Context, spec *specv1.Spec) (Document, error)
	RenderADR(ctx context.Context, decision *specv1.Decision) (Document, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Volumes/Code/github.com/.worktrees/specgraph/confluence-render && go build ./internal/render/`
Expected: Success (no tests yet — this is a pure types/interfaces file)

- [ ] **Step 3: Commit**

```bash
git add internal/render/render.go
git commit -s -m "feat(render): add Renderer interface and Document types

Foundation for multi-format document rendering (Markdown, ADF).
Defines DocumentKind (PRD, SDD, ADR) and format-agnostic Document container."
```

---

### Task 2: Render Package Refactor — Move entity renderers to markdown/

This is a mechanical refactor. All existing files in `internal/render/` (except the new `render.go`) move to `internal/render/markdown/`. Package name changes from `render` to `markdown`. All CLI callers update their import.

**Files:**
- Move: all `internal/render/*.go` except `render.go` → `internal/render/markdown/`
- Modify: all `cmd/specgraph/*.go` files that import `render`

- [ ] **Step 1: Create the markdown subdirectory and move files**

```bash
cd /Volumes/Code/github.com/.worktrees/specgraph/confluence-render
mkdir -p internal/render/markdown
# Move all Go files except render.go
for f in internal/render/*.go; do
  base=$(basename "$f")
  if [ "$base" != "render.go" ]; then
    git mv "$f" "internal/render/markdown/$base"
  fi
done
```

- [ ] **Step 2: Update package declaration in all moved files**

In every `internal/render/markdown/*.go` file, change `package render` to `package markdown`.

- [ ] **Step 3: Update the package doc comment**

In `internal/render/markdown/helpers.go` (formerly `markdown.go`), rename the file and update:

```go
// Package markdown renders protobuf types as markdown strings for CLI output.
package markdown
```

Note: rename `markdown.go` → `helpers.go` to avoid the confusing `markdown/markdown.go` path:

```bash
git mv internal/render/markdown/markdown.go internal/render/markdown/helpers.go
git mv internal/render/markdown/markdown_test.go internal/render/markdown/helpers_test.go
```

- [ ] **Step 4: Update all CLI callers**

Every file in `cmd/specgraph/` that imports `"github.com/specgraph/specgraph/internal/render"` must change to:

```go
import (
	"github.com/specgraph/specgraph/internal/render/markdown"
)
```

And all `render.Xxx` calls become `markdown.Xxx`. The affected files and their call sites:

**`cmd/specgraph/spec.go`:** `render.Spec` → `markdown.Spec`, `render.SpecList` → `markdown.SpecList`
**`cmd/specgraph/decision.go`:** `render.Decision` → `markdown.Decision`, `render.DecisionList` → `markdown.DecisionList`
**`cmd/specgraph/slice.go`:** `render.SliceDetail` → `markdown.SliceDetail`, `render.SliceList` → `markdown.SliceList`
**`cmd/specgraph/edge.go`:** `render.EdgeList` → `markdown.EdgeList`
**`cmd/specgraph/lifecycle.go`:** `render.DriftReport` → `markdown.DriftReport`
**`cmd/specgraph/graph.go`:** `render.NodeRefList` → `markdown.NodeRefList`
**`cmd/specgraph/pass.go`:** `render.AnalyticalPass` → `markdown.AnalyticalPass`
**`cmd/specgraph/findings.go`:** `render.Findings` → `markdown.Findings`
**`cmd/specgraph/changes.go`:** `render.Changes` → `markdown.Changes` (or `ChangesWithDiff`, `VersionComparison`)
**`cmd/specgraph/conversation.go`:** `render.ConversationLog` → `markdown.ConversationLog`, `render.ConversationLogList` → `markdown.ConversationLogList`
**`cmd/specgraph/constitution.go`:** `render.Constitution` → `markdown.Constitution`
**`cmd/specgraph/report.go`:** Check for any render imports and update.

- [ ] **Step 5: Verify build**

Run: `cd /Volumes/Code/github.com/.worktrees/specgraph/confluence-render && go build ./...`
Expected: Success — all imports resolve, no broken references.

- [ ] **Step 6: Run all tests**

Run: `task test`
Expected: All tests pass — this is a pure refactor, behavior unchanged.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -s -m "refactor(render): move entity renderers to render/markdown sub-package

Mechanical move to make room for render/adf and the Renderer interface.
All CLI callers updated: render.Xxx → markdown.Xxx. No behavior change."
```

---

## Phase 2: ADF Builder

### Task 3: ADF Node Types + Builder Core

**Files:**
- Create: `internal/render/adf/adf.go`
- Create: `internal/render/adf/builder.go`
- Create: `internal/render/adf/builder_test.go`

- [ ] **Step 1: Write the ADF type constants**

```go
// internal/render/adf/adf.go

// Package adf builds Atlassian Document Format (ADF) documents for Confluence.
package adf

// Node types in the ADF spec.
const (
	TypeDoc          = "doc"
	TypeParagraph    = "paragraph"
	TypeHeading      = "heading"
	TypeText         = "text"
	TypeBulletList   = "bulletList"
	TypeOrderedList  = "orderedList"
	TypeListItem     = "listItem"
	TypeTable        = "table"
	TypeTableRow     = "tableRow"
	TypeTableHeader  = "tableHeader"
	TypeTableCell    = "tableCell"
	TypeCodeBlock    = "codeBlock"
	TypeBlockquote   = "blockquote"
	TypePanel        = "panel"
	TypeExpand       = "expand"
	TypeRule         = "rule"
	TypeTaskList     = "taskList"
	TypeTaskItem     = "taskItem"
	TypeExtension    = "extension"
	TypeInlineCard   = "inlineCard"
	TypeStatus       = "status"
	TypeHardBreak    = "hardBreak"
)

// Mark types in the ADF spec.
const (
	MarkStrong    = "strong"
	MarkEm        = "em"
	MarkCode      = "code"
	MarkLink      = "link"
	MarkTextColor = "textColor"
)

// Panel types supported by Confluence.
const (
	PanelInfo    = "info"
	PanelNote    = "note"
	PanelWarning = "warning"
	PanelSuccess = "success"
	PanelError   = "error"
)
```

- [ ] **Step 2: Write failing tests for the builder**

```go
// internal/render/adf/builder_test.go
package adf

import (
	"encoding/json"
	"testing"
)

func TestNewDocument(t *testing.T) {
	doc := NewDocument()
	b, err := doc.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if m["type"] != TypeDoc {
		t.Errorf("type = %v, want %q", m["type"], TypeDoc)
	}
	if m["version"].(float64) != 1 {
		t.Errorf("version = %v, want 1", m["version"])
	}
}

func TestHeading(t *testing.T) {
	doc := NewDocument().Heading(1, "Title")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("content length = %d, want 1", len(content))
	}
	h := content[0].(map[string]any)
	if h["type"] != TypeHeading {
		t.Errorf("type = %v, want %q", h["type"], TypeHeading)
	}
	attrs := h["attrs"].(map[string]any)
	if attrs["level"].(float64) != 1 {
		t.Errorf("level = %v, want 1", attrs["level"])
	}
}

func TestParagraph(t *testing.T) {
	doc := NewDocument().Paragraph("Hello world")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	p := content[0].(map[string]any)
	if p["type"] != TypeParagraph {
		t.Errorf("type = %v, want %q", p["type"], TypeParagraph)
	}
	texts := p["content"].([]any)
	textNode := texts[0].(map[string]any)
	if textNode["text"] != "Hello world" {
		t.Errorf("text = %v, want %q", textNode["text"], "Hello world")
	}
}

func TestBulletList(t *testing.T) {
	doc := NewDocument().BulletList([]string{"item 1", "item 2"})
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	list := content[0].(map[string]any)
	if list["type"] != TypeBulletList {
		t.Errorf("type = %v, want %q", list["type"], TypeBulletList)
	}
	items := list["content"].([]any)
	if len(items) != 2 {
		t.Errorf("items = %d, want 2", len(items))
	}
}

func TestTable(t *testing.T) {
	doc := NewDocument().Table(
		Row(HeaderCell("Name"), HeaderCell("Value")),
		Row(Cell("foo"), Cell("bar")),
	)
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	table := content[0].(map[string]any)
	if table["type"] != TypeTable {
		t.Errorf("type = %v, want %q", table["type"], TypeTable)
	}
	rows := table["content"].([]any)
	if len(rows) != 2 {
		t.Errorf("rows = %d, want 2", len(rows))
	}
	// Header row
	headerRow := rows[0].(map[string]any)
	headerCells := headerRow["content"].([]any)
	firstHeader := headerCells[0].(map[string]any)
	if firstHeader["type"] != TypeTableHeader {
		t.Errorf("header cell type = %v, want %q", firstHeader["type"], TypeTableHeader)
	}
}

func TestCodeBlock(t *testing.T) {
	doc := NewDocument().CodeBlock("go", "func main() {}")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	cb := content[0].(map[string]any)
	if cb["type"] != TypeCodeBlock {
		t.Errorf("type = %v, want %q", cb["type"], TypeCodeBlock)
	}
	attrs := cb["attrs"].(map[string]any)
	if attrs["language"] != "go" {
		t.Errorf("language = %v, want %q", attrs["language"], "go")
	}
}

func TestChaining(t *testing.T) {
	doc := NewDocument().
		Heading(1, "Title").
		Paragraph("Body text").
		BulletList([]string{"a", "b"}).
		Rule()
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	if len(content) != 4 {
		t.Errorf("content length = %d, want 4", len(content))
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Volumes/Code/github.com/.worktrees/specgraph/confluence-render && go test ./internal/render/adf/`
Expected: FAIL — `NewDocument`, `Row`, `HeaderCell`, `Cell` undefined.

- [ ] **Step 4: Implement the builder**

```go
// internal/render/adf/builder.go
package adf

import "encoding/json"

// Node represents an ADF node.
type Node struct {
	Type    string         `json:"type"`
	Version *int           `json:"version,omitempty"`
	Content []Node         `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// Mark represents an ADF inline mark (bold, italic, link, etc.).
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// Document is a fluent builder for an ADF document.
type Document struct {
	root Node
}

// NewDocument creates a new ADF document.
func NewDocument() *Document {
	v := 1
	return &Document{
		root: Node{
			Type:    TypeDoc,
			Version: &v,
		},
	}
}

// JSON serializes the document to JSON bytes.
func (d *Document) JSON() ([]byte, error) {
	return json.Marshal(d.root)
}

// append adds a node to the document's content.
func (d *Document) append(n Node) *Document {
	d.root.Content = append(d.root.Content, n)
	return d
}

// Heading adds a heading node.
func (d *Document) Heading(level int, text string) *Document {
	return d.append(Node{
		Type:  TypeHeading,
		Attrs: map[string]any{"level": level},
		Content: []Node{
			{Type: TypeText, Text: text},
		},
	})
}

// Paragraph adds a paragraph with plain text.
func (d *Document) Paragraph(text string) *Document {
	return d.append(Node{
		Type: TypeParagraph,
		Content: []Node{
			{Type: TypeText, Text: text},
		},
	})
}

// ParagraphNodes adds a paragraph containing arbitrary inline nodes.
func (d *Document) ParagraphNodes(nodes ...Node) *Document {
	return d.append(Node{
		Type:    TypeParagraph,
		Content: nodes,
	})
}

// TextNode creates an inline text node with optional marks.
func TextNode(text string, marks ...Mark) Node {
	n := Node{Type: TypeText, Text: text}
	if len(marks) > 0 {
		n.Marks = marks
	}
	return n
}

// Bold creates a strong mark.
func Bold() Mark { return Mark{Type: MarkStrong} }

// Italic creates an em mark.
func Italic() Mark { return Mark{Type: MarkEm} }

// CodeMark creates a code mark.
func CodeMark() Mark { return Mark{Type: MarkCode} }

// Link creates a link mark.
func Link(href string) Mark {
	return Mark{Type: MarkLink, Attrs: map[string]any{"href": href}}
}

// BulletList adds a bullet list node.
func (d *Document) BulletList(items []string) *Document {
	listItems := make([]Node, len(items))
	for i, item := range items {
		listItems[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{
					Type: TypeParagraph,
					Content: []Node{
						{Type: TypeText, Text: item},
					},
				},
			},
		}
	}
	return d.append(Node{
		Type:    TypeBulletList,
		Content: listItems,
	})
}

// OrderedList adds a numbered list node.
func (d *Document) OrderedList(items []string) *Document {
	listItems := make([]Node, len(items))
	for i, item := range items {
		listItems[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{
					Type: TypeParagraph,
					Content: []Node{
						{Type: TypeText, Text: item},
					},
				},
			},
		}
	}
	return d.append(Node{
		Type:    TypeOrderedList,
		Content: listItems,
	})
}

// Table adds a table with the given rows.
func (d *Document) Table(rows ...Node) *Document {
	return d.append(Node{
		Type:    TypeTable,
		Content: rows,
	})
}

// Row creates a table row from cells.
func Row(cells ...Node) Node {
	return Node{
		Type:    TypeTableRow,
		Content: cells,
	}
}

// HeaderCell creates a table header cell with plain text.
func HeaderCell(text string) Node {
	return Node{
		Type: TypeTableHeader,
		Content: []Node{
			{
				Type: TypeParagraph,
				Content: []Node{
					{Type: TypeText, Text: text},
				},
			},
		},
	}
}

// Cell creates a table cell with plain text.
func Cell(text string) Node {
	return Node{
		Type: TypeTableCell,
		Content: []Node{
			{
				Type: TypeParagraph,
				Content: []Node{
					{Type: TypeText, Text: text},
				},
			},
		},
	}
}

// CellNodes creates a table cell containing arbitrary block nodes.
func CellNodes(nodes ...Node) Node {
	return Node{
		Type:    TypeTableCell,
		Content: nodes,
	}
}

// CodeBlock adds a code block with optional language.
func (d *Document) CodeBlock(language, code string) *Document {
	n := Node{
		Type: TypeCodeBlock,
		Content: []Node{
			{Type: TypeText, Text: code},
		},
	}
	if language != "" {
		n.Attrs = map[string]any{"language": language}
	}
	return d.append(n)
}

// Blockquote adds a blockquote node containing a paragraph.
func (d *Document) Blockquote(text string) *Document {
	return d.append(Node{
		Type: TypeBlockquote,
		Content: []Node{
			{
				Type: TypeParagraph,
				Content: []Node{
					{Type: TypeText, Text: text},
				},
			},
		},
	})
}

// Rule adds a horizontal rule.
func (d *Document) Rule() *Document {
	return d.append(Node{Type: TypeRule})
}

// Raw appends a pre-built node directly to the document.
func (d *Document) Raw(n Node) *Document {
	return d.append(n)
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/render/adf/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/render/adf/
git commit -s -m "feat(render/adf): add ADF builder with fluent API

Core builder for constructing Atlassian Document Format documents.
Supports headings, paragraphs, tables, lists, code blocks, blockquotes,
rules, and inline marks (bold, italic, code, link)."
```

---

### Task 4: ADF Macro Helpers

**Files:**
- Create: `internal/render/adf/macros.go`
- Create: `internal/render/adf/macros_test.go`

- [ ] **Step 1: Write failing tests for macros**

```go
// internal/render/adf/macros_test.go
package adf

import (
	"encoding/json"
	"testing"
)

func TestStatusMacro(t *testing.T) {
	node := StatusMacro("In Progress", "blue")
	b, _ := json.Marshal(node)
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["type"] != TypeStatus {
		t.Errorf("type = %v, want %q", m["type"], TypeStatus)
	}
	attrs := m["attrs"].(map[string]any)
	if attrs["text"] != "In Progress" {
		t.Errorf("text = %v, want %q", attrs["text"], "In Progress")
	}
	if attrs["color"] != "blue" {
		t.Errorf("color = %v, want %q", attrs["color"], "blue")
	}
}

func TestPanelMacro(t *testing.T) {
	doc := NewDocument()
	doc.Panel(PanelWarning, "This is a warning")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	panel := content[0].(map[string]any)
	if panel["type"] != TypePanel {
		t.Errorf("type = %v, want %q", panel["type"], TypePanel)
	}
	attrs := panel["attrs"].(map[string]any)
	if attrs["panelType"] != PanelWarning {
		t.Errorf("panelType = %v, want %q", attrs["panelType"], PanelWarning)
	}
}

func TestExpandMacro(t *testing.T) {
	doc := NewDocument()
	doc.Expand("Click to expand", "Hidden content here")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	expand := content[0].(map[string]any)
	if expand["type"] != TypeExpand {
		t.Errorf("type = %v, want %q", expand["type"], TypeExpand)
	}
	attrs := expand["attrs"].(map[string]any)
	if attrs["title"] != "Click to expand" {
		t.Errorf("title = %v, want %q", attrs["title"], "Click to expand")
	}
}

func TestPagePropertiesMacro(t *testing.T) {
	node := PageProperties([][2]string{
		{"Owner", "Sean"},
		{"Status", "Draft"},
	})
	b, _ := json.Marshal(node)
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["type"] != TypeExtension {
		t.Errorf("type = %v, want %q", m["type"], TypeExtension)
	}
	attrs := m["attrs"].(map[string]any)
	if attrs["extensionType"] != "com.atlassian.confluence.macro.core" {
		t.Errorf("extensionType = %v", attrs["extensionType"])
	}
	if attrs["extensionKey"] != "details" {
		t.Errorf("extensionKey = %v", attrs["extensionKey"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/adf/`
Expected: FAIL — `StatusMacro`, `Panel`, `Expand`, `PageProperties` undefined.

- [ ] **Step 3: Implement macros**

```go
// internal/render/adf/macros.go
package adf

// StatusMacro creates a Confluence status lozenge node.
// Valid colors: neutral, purple, blue, red, yellow, green.
func StatusMacro(text, color string) Node {
	return Node{
		Type: TypeStatus,
		Attrs: map[string]any{
			"text":  text,
			"color": color,
			"style": "",
		},
	}
}

// StageColor returns the Confluence status color for a spec stage.
func StageColor(stage string) string {
	switch stage {
	case "spark":
		return "purple"
	case "shape":
		return "blue"
	case "specify":
		return "blue"
	case "decompose":
		return "blue"
	case "approved":
		return "green"
	case "in_progress":
		return "yellow"
	case "done":
		return "green"
	case "abandoned":
		return "red"
	default:
		return "neutral"
	}
}

// DecisionStatusColor returns the Confluence status color for a decision status.
func DecisionStatusColor(status string) string {
	switch status {
	case "proposed":
		return "yellow"
	case "accepted":
		return "green"
	case "deprecated":
		return "red"
	case "superseded":
		return "neutral"
	default:
		return "neutral"
	}
}

// Panel adds a Confluence panel macro to the document.
// panelType is one of: info, note, warning, success, error.
func (d *Document) Panel(panelType, text string) *Document {
	return d.append(Node{
		Type:  TypePanel,
		Attrs: map[string]any{"panelType": panelType},
		Content: []Node{
			{
				Type: TypeParagraph,
				Content: []Node{
					{Type: TypeText, Text: text},
				},
			},
		},
	})
}

// PanelNodes adds a Confluence panel containing arbitrary block nodes.
func (d *Document) PanelNodes(panelType string, content ...Node) *Document {
	return d.append(Node{
		Type:    TypePanel,
		Attrs:   map[string]any{"panelType": panelType},
		Content: content,
	})
}

// Expand adds a Confluence expand/collapse macro to the document.
func (d *Document) Expand(title, text string) *Document {
	return d.append(Node{
		Type:  TypeExpand,
		Attrs: map[string]any{"title": title},
		Content: []Node{
			{
				Type: TypeParagraph,
				Content: []Node{
					{Type: TypeText, Text: text},
				},
			},
		},
	})
}

// ExpandNodes adds an expand macro containing arbitrary block nodes.
func (d *Document) ExpandNodes(title string, content ...Node) *Document {
	return d.append(Node{
		Type:    TypeExpand,
		Attrs:   map[string]any{"title": title},
		Content: content,
	})
}

// PageProperties creates a Confluence Page Properties macro (details macro)
// containing key-value metadata rows.
func PageProperties(pairs [][2]string) Node {
	rows := make([]Node, 0, len(pairs)+1)
	// Header row
	rows = append(rows, Row(HeaderCell("Property"), HeaderCell("Value")))
	for _, p := range pairs {
		rows = append(rows, Row(Cell(p[0]), Cell(p[1])))
	}
	return Node{
		Type: TypeExtension,
		Attrs: map[string]any{
			"extensionType": "com.atlassian.confluence.macro.core",
			"extensionKey":  "details",
			"layout":        "default",
		},
		Content: []Node{
			{
				Type:    TypeTable,
				Content: rows,
			},
		},
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/adf/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/adf/macros.go internal/render/adf/macros_test.go
git commit -s -m "feat(render/adf): add Confluence macro helpers

Status lozenges, panels, expand/collapse sections, and page-properties
macro. Includes stage-to-color mappings for status lozenges."
```

---

## Phase 3: Markdown Document Renderers

### Task 5: Markdown PRD Renderer

**Files:**
- Create: `internal/render/markdown/prd.go`
- Create: `internal/render/markdown/prd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/markdown/prd_test.go
package markdown

import (
	"context"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestRenderPRD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:   "auth-redesign",
		Intent: "Redesign authentication system",
		Stage:  "shape",
		SparkOutput: &specv1.SparkOutput{
			Seed:     "Auth is brittle and needs rework",
			Signal:   "Three incidents in two weeks",
			KillTest: "If compliance approves current system",
		},
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn:  []string{"OAuth2 refresh rotation", "Session management"},
			ScopeOut: []string{"SSO integration", "MFA"},
			Approaches: []*specv1.Approach{
				{Name: "Full rewrite", Description: "Start from scratch", Tradeoffs: []string{"Clean design", "High risk"}},
			},
			ChosenApproach: "Full rewrite",
			SuccessMust:    []string{"Token rotation works"},
			SuccessShould:  []string{"Session UI improved"},
			SuccessWont:    []string{"SSO support"},
			Risks:          []string{"Timeline risk"},
		},
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD() error: %v", err)
	}
	if doc.Kind != render.DocumentPRD {
		t.Errorf("Kind = %v, want DocumentPRD", doc.Kind)
	}
	if doc.SpecSlug != "auth-redesign" {
		t.Errorf("SpecSlug = %q, want %q", doc.SpecSlug, "auth-redesign")
	}
	body := string(doc.Body)
	if !strings.Contains(body, "# PRD: auth-redesign") {
		t.Error("missing PRD title")
	}
	if !strings.Contains(body, "Redesign authentication system") {
		t.Error("missing intent")
	}
	if !strings.Contains(body, "Auth is brittle") {
		t.Error("missing spark seed")
	}
	if !strings.Contains(body, "OAuth2 refresh rotation") {
		t.Error("missing scope in")
	}
	if !strings.Contains(body, "SSO integration") {
		t.Error("missing scope out")
	}
	if !strings.Contains(body, "MUST") {
		t.Error("missing success criteria")
	}
}

func TestRenderPRDNilSpec(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderPRD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/markdown/`
Expected: FAIL — `NewRenderer` undefined.

- [ ] **Step 3: Implement the Markdown PRD renderer**

```go
// internal/render/markdown/prd.go
package markdown

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// Renderer implements render.Renderer for Markdown output.
type Renderer struct{}

// NewRenderer creates a Markdown renderer.
func NewRenderer() *Renderer { return &Renderer{} }

// RenderPRD renders a spec's Spark + Shape outputs as a PRD document.
func (r *Renderer) RenderPRD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# PRD: %s\n\n", spec.Slug)
	b.WriteString(metadataTable([][2]string{
		{"Stage", spec.Stage},
		{"Priority", spec.Priority},
	}))

	if spec.Intent != "" {
		fmt.Fprintf(&b, "\n> %s\n\n", spec.Intent)
	}

	// Spark section
	if o := spec.SparkOutput; o != nil {
		if o.Seed != "" {
			b.WriteString(section(2, "Intent", o.Seed))
		}
		if o.Signal != "" || o.KillTest != "" {
			var ctx strings.Builder
			if o.Signal != "" {
				fmt.Fprintf(&ctx, "**Signal:** %s\n\n", o.Signal)
			}
			if o.KillTest != "" {
				fmt.Fprintf(&ctx, "**Kill Test:** %s\n", o.KillTest)
			}
			b.WriteString(section(2, "Context & Signal", ctx.String()))
		}
	}

	// Shape section
	if o := spec.ShapeOutput; o != nil {
		// Scope
		if len(o.ScopeIn) > 0 || len(o.ScopeOut) > 0 {
			var scope strings.Builder
			if len(o.ScopeIn) > 0 {
				scope.WriteString("**In:**\n")
				for _, s := range o.ScopeIn {
					fmt.Fprintf(&scope, "- %s\n", s)
				}
				scope.WriteString("\n")
			}
			if len(o.ScopeOut) > 0 {
				scope.WriteString("**Out:**\n")
				for _, s := range o.ScopeOut {
					fmt.Fprintf(&scope, "- %s\n", s)
				}
			}
			b.WriteString(section(2, "Scope", scope.String()))
		}
		// Approaches
		if len(o.Approaches) > 0 {
			var app strings.Builder
			for _, a := range o.Approaches {
				chosen := ""
				if a.Name == o.ChosenApproach {
					chosen = " (chosen)"
				}
				fmt.Fprintf(&app, "### %s%s\n\n", a.Name, chosen)
				if a.Description != "" {
					fmt.Fprintf(&app, "%s\n\n", a.Description)
				}
				for _, t := range a.Tradeoffs {
					fmt.Fprintf(&app, "- %s\n", t)
				}
				app.WriteString("\n")
			}
			b.WriteString(section(2, "Approaches", app.String()))
		}
		// Success criteria
		if len(o.SuccessMust) > 0 || len(o.SuccessShould) > 0 || len(o.SuccessWont) > 0 {
			var sc strings.Builder
			for _, s := range o.SuccessMust {
				fmt.Fprintf(&sc, "- **MUST:** %s\n", s)
			}
			for _, s := range o.SuccessShould {
				fmt.Fprintf(&sc, "- **SHOULD:** %s\n", s)
			}
			for _, s := range o.SuccessWont {
				fmt.Fprintf(&sc, "- **WON'T:** %s\n", s)
			}
			b.WriteString(section(2, "Success Criteria", sc.String()))
		}
		// Risks
		if len(o.Risks) > 0 {
			var risks strings.Builder
			for _, r := range o.Risks {
				fmt.Fprintf(&risks, "- %s\n", r)
			}
			b.WriteString(section(2, "Risks", risks.String()))
		}
	}

	return render.Document{
		Kind:     render.DocumentPRD,
		Title:    fmt.Sprintf("PRD: %s", spec.Slug),
		Body:     []byte(b.String()),
		SpecSlug: spec.Slug,
	}, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/markdown/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/markdown/prd.go internal/render/markdown/prd_test.go
git commit -s -m "feat(render/markdown): add PRD renderer

Renders Spark + Shape outputs as a markdown PRD document.
Implements render.Renderer.RenderPRD for CLI preview."
```

---

### Task 6: Markdown SDD Renderer

**Files:**
- Create: `internal/render/markdown/sdd.go`
- Create: `internal/render/markdown/sdd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/markdown/sdd_test.go
package markdown

import (
	"context"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestRenderSDD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "auth-redesign",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "TokenService", Body: "service TokenService { ... }"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "auth", Description: "Token refresh works under load"},
			},
			Invariants: []string{"Tokens expire within 15 minutes"},
			Touches: []*specv1.FileTouch{
				{Path: "internal/auth/token.go", Purpose: "Token rotation", ChangeType: "modify"},
			},
		},
		DecomposeOutput: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "slice-1", Intent: "Token rotation", Verify: []string{"Tokens rotate"}, Touches: []string{"internal/auth/"}},
			},
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD() error: %v", err)
	}
	if doc.Kind != render.DocumentSDD {
		t.Errorf("Kind = %v, want DocumentSDD", doc.Kind)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "# SDD: auth-redesign") {
		t.Error("missing SDD title")
	}
	if !strings.Contains(body, "TokenService") {
		t.Error("missing interface")
	}
	if !strings.Contains(body, "Token refresh works under load") {
		t.Error("missing verify criterion")
	}
	if !strings.Contains(body, "Tokens expire within 15 minutes") {
		t.Error("missing invariant")
	}
	if !strings.Contains(body, "vertical_slice") {
		t.Error("missing strategy")
	}
}

func TestRenderSDDNilSpec(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderSDD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/markdown/`
Expected: FAIL — `RenderSDD` not defined on `Renderer`.

- [ ] **Step 3: Implement SDD renderer**

```go
// internal/render/markdown/sdd.go
package markdown

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderSDD renders a spec's Specify + Decompose outputs as an SDD document.
func (r *Renderer) RenderSDD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# SDD: %s\n\n", spec.Slug)

	// Specify section
	if o := spec.SpecifyOutput; o != nil {
		if len(o.Interfaces) > 0 {
			b.WriteString("## Interface Contracts\n\n")
			for _, iface := range o.Interfaces {
				fmt.Fprintf(&b, "### %s\n\n", iface.Name)
				if iface.Body != "" {
					fmt.Fprintf(&b, "```\n%s\n```\n\n", iface.Body)
				}
			}
		}
		if len(o.VerifyCriteria) > 0 {
			rows := make([][]string, len(o.VerifyCriteria))
			for i, vc := range o.VerifyCriteria {
				rows[i] = []string{vc.Category, vc.Description}
			}
			b.WriteString(section(2, "Acceptance Criteria", itemTable([]string{"Category", "Description"}, rows)))
		}
		if len(o.Invariants) > 0 {
			var inv strings.Builder
			for _, s := range o.Invariants {
				fmt.Fprintf(&inv, "- %s\n", s)
			}
			b.WriteString(section(2, "Invariants", inv.String()))
		}
		if len(o.Touches) > 0 {
			rows := make([][]string, len(o.Touches))
			for i, ft := range o.Touches {
				rows[i] = []string{ft.Path, ft.Purpose, ft.ChangeType}
			}
			b.WriteString(section(2, "File Touches", itemTable([]string{"Path", "Purpose", "Action"}, rows)))
		}
	}

	// Decompose section
	if o := spec.DecomposeOutput; o != nil {
		if strategy := decompositionStrategyString(o.Strategy); strategy != "" {
			b.WriteString(section(2, "Decomposition Strategy", strategy))
		}
		if len(o.Slices) > 0 {
			b.WriteString("## Slices\n\n")
			rows := make([][]string, len(o.Slices))
			for i, s := range o.Slices {
				rows[i] = []string{
					s.Id,
					s.Intent,
					strings.Join(s.Verify, "; "),
					strings.Join(s.DependsOn, ", "),
				}
			}
			b.WriteString(itemTable([]string{"ID", "Intent", "Verify", "Depends On"}, rows))
		}
	}

	return render.Document{
		Kind:     render.DocumentSDD,
		Title:    fmt.Sprintf("SDD: %s", spec.Slug),
		Body:     []byte(b.String()),
		SpecSlug: spec.Slug,
	}, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/markdown/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/markdown/sdd.go internal/render/markdown/sdd_test.go
git commit -s -m "feat(render/markdown): add SDD renderer

Renders Specify + Decompose outputs as a markdown SDD document.
Implements render.Renderer.RenderSDD for CLI preview."
```

---

### Task 7: Markdown ADR Renderer (MADR Format)

**Files:**
- Create: `internal/render/markdown/adr.go`
- Create: `internal/render/markdown/adr_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/markdown/adr_test.go
package markdown

import (
	"context"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestRenderADR(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:       "use-pgx",
		Title:      "Use pgx/v5 as database driver",
		Status:     specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:   "We will use pgx/v5 directly instead of database/sql",
		Rationale:  "Native PostgreSQL features and better performance",
		Question:   "Which database driver should we use?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "database/sql + pq", Reason: "Missing native PostgreSQL type support"},
			{Option: "sqlx", Reason: "Extra abstraction layer not needed"},
		},
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR() error: %v", err)
	}
	if doc.Kind != render.DocumentADR {
		t.Errorf("Kind = %v, want DocumentADR", doc.Kind)
	}
	if doc.DecisionID != "use-pgx" {
		t.Errorf("DecisionID = %q, want %q", doc.DecisionID, "use-pgx")
	}
	body := string(doc.Body)
	if !strings.Contains(body, "# ADR: Use pgx/v5 as database driver") {
		t.Error("missing MADR title")
	}
	if !strings.Contains(body, "## Status") {
		t.Error("missing status section")
	}
	if !strings.Contains(body, "accepted") {
		t.Error("missing status value")
	}
	if !strings.Contains(body, "## Context") {
		t.Error("missing context section")
	}
	if !strings.Contains(body, "## Decision") {
		t.Error("missing decision section")
	}
	if !strings.Contains(body, "## Considered Options") {
		t.Error("missing considered options")
	}
	if !strings.Contains(body, "database/sql + pq") {
		t.Error("missing rejected alternative")
	}
	if !strings.Contains(body, "HIGH") {
		t.Error("missing confidence")
	}
}

func TestRenderADRNilDecision(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderADR(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil decision")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/markdown/`
Expected: FAIL — `RenderADR` not defined on `Renderer`.

- [ ] **Step 3: Implement ADR renderer**

```go
// internal/render/markdown/adr.go
package markdown

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderADR renders a decision as an ADR in MADR format.
func (r *Renderer) RenderADR(_ context.Context, d *specv1.Decision) (render.Document, error) {
	if d == nil {
		return render.Document{}, fmt.Errorf("decision is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# ADR: %s\n\n", d.Title)

	// Status
	b.WriteString(section(2, "Status", decisionStatusString(d.Status)))

	// Context
	if d.Question != "" {
		b.WriteString(section(2, "Context", d.Question))
	}

	// Decision
	if d.Decision != "" {
		body := d.Decision
		if d.Rationale != "" {
			body += fmt.Sprintf("\n\n**Rationale:** %s", d.Rationale)
		}
		b.WriteString(section(2, "Decision", body))
	}

	// Considered Options (MADR extension)
	if len(d.RejectedAlternatives) > 0 {
		// Include the chosen option as first row, then rejected
		rows := make([][]string, 0, len(d.RejectedAlternatives)+1)
		if d.Decision != "" {
			rows = append(rows, []string{d.Title, "Chosen", d.Rationale})
		}
		for _, ra := range d.RejectedAlternatives {
			rows = append(rows, []string{ra.Option, "Rejected", ra.Reason})
		}
		b.WriteString(section(2, "Considered Options", itemTable([]string{"Option", "Status", "Reason"}, rows)))
	}

	// Confidence (MADR extension)
	if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED {
		conf := decisionConfidenceName(d.Confidence)
		detail := fmt.Sprintf("**Confidence:** %s", conf)
		if d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
			detail += fmt.Sprintf("\n**Scope:** %s", decisionScopeName(d.Scope))
		}
		b.WriteString(section(2, "Confidence & Scope", detail))
	}

	return render.Document{
		Kind:       render.DocumentADR,
		Title:      fmt.Sprintf("ADR: %s", d.Title),
		Body:       []byte(b.String()),
		SpecSlug:   d.OriginSpec,
		DecisionID: d.Slug,
	}, nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/markdown/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/markdown/adr.go internal/render/markdown/adr_test.go
git commit -s -m "feat(render/markdown): add MADR-format ADR renderer

Renders Decision proto as a Markdown Any Decision Record (MADR).
Includes considered-options table and confidence/scope metadata."
```

---

## Phase 4: ADF Document Renderers

### Task 8: ADF PRD Renderer

**Files:**
- Create: `internal/render/adf/prd.go`
- Create: `internal/render/adf/prd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/adf/prd_test.go
package adf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestADFRenderPRD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug:   "auth-redesign",
		Intent: "Redesign authentication system",
		Stage:  "shape",
		SparkOutput: &specv1.SparkOutput{
			Seed:     "Auth is brittle",
			Signal:   "Three incidents",
			KillTest: "If compliance approves",
		},
		ShapeOutput: &specv1.ShapeOutput{
			ScopeIn:        []string{"OAuth2 refresh"},
			ScopeOut:       []string{"SSO"},
			SuccessMust:    []string{"Token rotation"},
			SuccessShould:  []string{"Better UX"},
			SuccessWont:    []string{"SSO support"},
			Risks:          []string{"Timeline"},
			Approaches: []*specv1.Approach{
				{Name: "Rewrite", Description: "From scratch", Tradeoffs: []string{"Clean", "Risky"}},
			},
			ChosenApproach: "Rewrite",
		},
	}
	doc, err := r.RenderPRD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderPRD() error: %v", err)
	}
	if doc.Kind != render.DocumentPRD {
		t.Errorf("Kind = %v, want DocumentPRD", doc.Kind)
	}
	if doc.SpecSlug != "auth-redesign" {
		t.Errorf("SpecSlug = %q", doc.SpecSlug)
	}
	// Verify it's valid JSON
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	if m["type"] != TypeDoc {
		t.Errorf("root type = %v, want %q", m["type"], TypeDoc)
	}
	// Verify it contains expected content
	body := string(doc.Body)
	if !strings.Contains(body, "auth-redesign") {
		t.Error("missing slug in ADF")
	}
	if !strings.Contains(body, "Auth is brittle") {
		t.Error("missing spark seed")
	}
	if !strings.Contains(body, "OAuth2 refresh") {
		t.Error("missing scope")
	}
	if !strings.Contains(body, "Token rotation") {
		t.Error("missing success criteria")
	}
}

func TestADFRenderPRDNil(t *testing.T) {
	r := NewRenderer()
	_, err := r.RenderPRD(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil spec")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/adf/`
Expected: FAIL — `NewRenderer` undefined.

- [ ] **Step 3: Implement ADF PRD renderer**

```go
// internal/render/adf/prd.go
package adf

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// Renderer implements render.Renderer for ADF output.
type Renderer struct{}

// NewRenderer creates an ADF renderer.
func NewRenderer() *Renderer { return &Renderer{} }

// RenderPRD renders a spec's Spark + Shape as an ADF PRD document.
func (r *Renderer) RenderPRD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}

	doc := NewDocument()

	// Title + status lozenge
	doc.Heading(1, fmt.Sprintf("PRD: %s", spec.Slug))
	doc.ParagraphNodes(
		TextNode("Stage: "),
		StatusMacro(spec.Stage, StageColor(spec.Stage)),
	)

	// Page properties
	props := [][2]string{
		{"Slug", spec.Slug},
		{"Stage", spec.Stage},
	}
	if spec.Priority != "" {
		props = append(props, [2]string{"Priority", spec.Priority})
	}
	doc.Raw(PageProperties(props))

	// Intent
	if spec.Intent != "" {
		doc.Blockquote(spec.Intent)
	}

	// Spark sections
	if o := spec.SparkOutput; o != nil {
		if o.Seed != "" {
			doc.Heading(2, "Intent")
			doc.Paragraph(o.Seed)
		}
		if o.Signal != "" || o.KillTest != "" {
			doc.Heading(2, "Context & Signal")
			if o.Signal != "" {
				doc.ParagraphNodes(TextNode("Signal: ", Bold()), TextNode(o.Signal))
			}
			if o.KillTest != "" {
				doc.Panel(PanelWarning, fmt.Sprintf("Kill Test: %s", o.KillTest))
			}
		}
	}

	// Shape sections
	if o := spec.ShapeOutput; o != nil {
		// Scope table
		if len(o.ScopeIn) > 0 || len(o.ScopeOut) > 0 {
			doc.Heading(2, "Scope")
			doc.Table(
				Row(HeaderCell("In"), HeaderCell("Out")),
				Row(
					CellNodes(listParagraph(o.ScopeIn)),
					CellNodes(listParagraph(o.ScopeOut)),
				),
			)
		}

		// Approaches
		if len(o.Approaches) > 0 {
			doc.Heading(2, "Approaches")
			for _, a := range o.Approaches {
				title := a.Name
				if a.Name == o.ChosenApproach {
					title += " (chosen)"
				}
				content := a.Description
				if len(a.Tradeoffs) > 0 {
					content += "\n\nTradeoffs: " + strings.Join(a.Tradeoffs, "; ")
				}
				if a.Name == o.ChosenApproach {
					doc.Heading(3, title)
					doc.Paragraph(content)
				} else {
					doc.Expand(title, content)
				}
			}
		}

		// Success criteria
		if len(o.SuccessMust) > 0 || len(o.SuccessShould) > 0 || len(o.SuccessWont) > 0 {
			doc.Heading(2, "Success Criteria")
			rows := make([]Node, 0)
			rows = append(rows, Row(HeaderCell("Priority"), HeaderCell("Criterion")))
			for _, s := range o.SuccessMust {
				rows = append(rows, Row(Cell("MUST"), Cell(s)))
			}
			for _, s := range o.SuccessShould {
				rows = append(rows, Row(Cell("SHOULD"), Cell(s)))
			}
			for _, s := range o.SuccessWont {
				rows = append(rows, Row(Cell("WON'T"), Cell(s)))
			}
			doc.Table(rows...)
		}

		// Risks
		if len(o.Risks) > 0 {
			doc.Heading(2, "Risks")
			doc.BulletList(o.Risks)
		}
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:     render.DocumentPRD,
		Title:    fmt.Sprintf("PRD: %s", spec.Slug),
		Body:     b,
		SpecSlug: spec.Slug,
	}, nil
}

// listParagraph creates a bullet list node from string items.
// Returns an empty paragraph if items is nil.
func listParagraph(items []string) Node {
	if len(items) == 0 {
		return Node{
			Type:    TypeParagraph,
			Content: []Node{{Type: TypeText, Text: "—"}},
		}
	}
	listItems := make([]Node, len(items))
	for i, item := range items {
		listItems[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{
					Type:    TypeParagraph,
					Content: []Node{{Type: TypeText, Text: item}},
				},
			},
		}
	}
	return Node{
		Type:    TypeBulletList,
		Content: listItems,
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/adf/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/adf/prd.go internal/render/adf/prd_test.go
git commit -s -m "feat(render/adf): add ADF PRD renderer

Renders Spark + Shape as Confluence-native ADF with status lozenges,
page-properties macro, scope table, expand macros for approaches,
MoSCoW criteria table, and risk panel."
```

---

### Task 9: ADF SDD Renderer

**Files:**
- Create: `internal/render/adf/sdd.go`
- Create: `internal/render/adf/sdd_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/adf/sdd_test.go
package adf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestADFRenderSDD(t *testing.T) {
	r := NewRenderer()
	spec := &specv1.Spec{
		Slug: "auth-redesign",
		SpecifyOutput: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "TokenService", Body: "service TokenService { ... }"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "auth", Description: "Token refresh works"},
			},
			Invariants: []string{"Tokens expire in 15min"},
			Touches: []*specv1.FileTouch{
				{Path: "internal/auth/token.go", Purpose: "Rotation", ChangeType: "modify"},
			},
		},
		DecomposeOutput: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "Token rotation", Verify: []string{"Rotates"}, DependsOn: []string{}},
			},
		},
	}
	doc, err := r.RenderSDD(context.Background(), spec)
	if err != nil {
		t.Fatalf("RenderSDD() error: %v", err)
	}
	if doc.Kind != render.DocumentSDD {
		t.Errorf("Kind = %v, want DocumentSDD", doc.Kind)
	}
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "TokenService") {
		t.Error("missing interface")
	}
	if !strings.Contains(body, "Token refresh works") {
		t.Error("missing verify criterion")
	}
	if !strings.Contains(body, "Tokens expire in 15min") {
		t.Error("missing invariant")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/adf/`
Expected: FAIL — `RenderSDD` not defined.

- [ ] **Step 3: Implement ADF SDD renderer**

```go
// internal/render/adf/sdd.go
package adf

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderSDD renders a spec's Specify + Decompose as an ADF SDD document.
func (r *Renderer) RenderSDD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}

	doc := NewDocument()
	doc.Heading(1, fmt.Sprintf("SDD: %s", spec.Slug))

	// Specify sections
	if o := spec.SpecifyOutput; o != nil {
		if len(o.Interfaces) > 0 {
			doc.Heading(2, "Interface Contracts")
			for _, iface := range o.Interfaces {
				doc.Heading(3, iface.Name)
				if iface.Body != "" {
					doc.CodeBlock("", iface.Body)
				}
			}
		}
		if len(o.VerifyCriteria) > 0 {
			doc.Heading(2, "Acceptance Criteria")
			rows := []Node{Row(HeaderCell("Category"), HeaderCell("Description"))}
			for _, vc := range o.VerifyCriteria {
				rows = append(rows, Row(Cell(vc.Category), Cell(vc.Description)))
			}
			doc.Table(rows...)
		}
		if len(o.Invariants) > 0 {
			doc.Heading(2, "Invariants")
			doc.PanelNodes(PanelWarning, Node{
				Type:    TypeBulletList,
				Content: textListItems(o.Invariants),
			})
		}
		if len(o.Touches) > 0 {
			doc.Heading(2, "File Touches")
			rows := []Node{Row(HeaderCell("Path"), HeaderCell("Purpose"), HeaderCell("Action"))}
			for _, ft := range o.Touches {
				rows = append(rows, Row(Cell(ft.Path), Cell(ft.Purpose), Cell(ft.ChangeType)))
			}
			doc.Table(rows...)
		}
	}

	// Decompose sections
	if o := spec.DecomposeOutput; o != nil {
		if strategy := decompositionStrategyName(o.Strategy); strategy != "" {
			doc.Heading(2, "Decomposition Strategy")
			doc.Panel(PanelInfo, strategy)
		}
		if len(o.Slices) > 0 {
			doc.Heading(2, "Slices")
			rows := []Node{Row(HeaderCell("ID"), HeaderCell("Intent"), HeaderCell("Verify"), HeaderCell("Depends On"))}
			for _, s := range o.Slices {
				rows = append(rows, Row(
					Cell(s.Id),
					Cell(s.Intent),
					Cell(strings.Join(s.Verify, "; ")),
					Cell(strings.Join(s.DependsOn, ", ")),
				))
			}
			doc.Table(rows...)
		}
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:     render.DocumentSDD,
		Title:    fmt.Sprintf("SDD: %s", spec.Slug),
		Body:     b,
		SpecSlug: spec.Slug,
	}, nil
}

// textListItems creates ADF list item nodes from strings.
func textListItems(items []string) []Node {
	nodes := make([]Node, len(items))
	for i, item := range items {
		nodes[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{Type: TypeParagraph, Content: []Node{{Type: TypeText, Text: item}}},
			},
		}
	}
	return nodes
}

func decompositionStrategyName(s specv1.DecompositionStrategy) string {
	switch s {
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE:
		return "vertical_slice"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE:
		return "layer_cake"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT:
		return "single_unit"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD:
		return "steel_thread"
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/adf/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/adf/sdd.go internal/render/adf/sdd_test.go
git commit -s -m "feat(render/adf): add ADF SDD renderer

Renders Specify + Decompose as ADF with code blocks for interfaces,
warning panels for invariants, and slice dependency tables."
```

---

### Task 10: ADF ADR Renderer (MADR)

**Files:**
- Create: `internal/render/adf/adr.go`
- Create: `internal/render/adf/adr_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/render/adf/adr_test.go
package adf

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestADFRenderADR(t *testing.T) {
	r := NewRenderer()
	dec := &specv1.Decision{
		Slug:       "use-pgx",
		Title:      "Use pgx/v5 as database driver",
		Status:     specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
		Decision:   "Use pgx/v5 directly",
		Rationale:  "Native PostgreSQL features",
		Question:   "Which database driver?",
		Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
		Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
		OriginSpec: "auth-redesign",
		RejectedAlternatives: []*specv1.RejectedAlternative{
			{Option: "database/sql + pq", Reason: "Missing native types"},
		},
	}
	doc, err := r.RenderADR(context.Background(), dec)
	if err != nil {
		t.Fatalf("RenderADR() error: %v", err)
	}
	if doc.Kind != render.DocumentADR {
		t.Errorf("Kind = %v, want DocumentADR", doc.Kind)
	}
	if doc.DecisionID != "use-pgx" {
		t.Errorf("DecisionID = %q", doc.DecisionID)
	}
	if doc.SpecSlug != "auth-redesign" {
		t.Errorf("SpecSlug = %q", doc.SpecSlug)
	}
	var m map[string]any
	if err := json.Unmarshal(doc.Body, &m); err != nil {
		t.Fatalf("invalid ADF JSON: %v", err)
	}
	body := string(doc.Body)
	if !strings.Contains(body, "Use pgx/v5 as database driver") {
		t.Error("missing title")
	}
	if !strings.Contains(body, "accepted") {
		t.Error("missing status")
	}
	if !strings.Contains(body, "Which database driver") {
		t.Error("missing context/question")
	}
	if !strings.Contains(body, "database/sql + pq") {
		t.Error("missing rejected alternative")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/render/adf/`
Expected: FAIL — `RenderADR` not defined.

- [ ] **Step 3: Implement ADF ADR renderer**

```go
// internal/render/adf/adr.go
package adf

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderADR renders a decision as an ADF document in MADR format.
func (r *Renderer) RenderADR(_ context.Context, d *specv1.Decision) (render.Document, error) {
	if d == nil {
		return render.Document{}, fmt.Errorf("decision is nil")
	}

	doc := NewDocument()

	// Title
	doc.Heading(1, fmt.Sprintf("ADR: %s", d.Title))

	// Status with lozenge
	statusStr := decisionStatusName(d.Status)
	doc.Heading(2, "Status")
	doc.ParagraphNodes(StatusMacro(statusStr, DecisionStatusColor(statusStr)))

	// Context (the question being answered)
	if d.Question != "" {
		doc.Heading(2, "Context")
		doc.Paragraph(d.Question)
	}

	// Decision
	if d.Decision != "" {
		doc.Heading(2, "Decision")
		doc.Paragraph(d.Decision)
		if d.Rationale != "" {
			doc.ParagraphNodes(TextNode("Rationale: ", Bold()), TextNode(d.Rationale))
		}
	}

	// Considered Options (MADR format)
	if len(d.RejectedAlternatives) > 0 || d.Decision != "" {
		doc.Heading(2, "Considered Options")
		rows := []Node{Row(HeaderCell("Option"), HeaderCell("Status"), HeaderCell("Reason"))}
		if d.Title != "" {
			rows = append(rows, Row(Cell(d.Title), Cell("Chosen"), Cell(d.Rationale)))
		}
		for _, ra := range d.RejectedAlternatives {
			rows = append(rows, Row(Cell(ra.Option), Cell("Rejected"), Cell(ra.Reason)))
		}
		doc.Table(rows...)
	}

	// Confidence & Scope
	if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED {
		doc.Heading(2, "Confidence & Scope")
		details := fmt.Sprintf("Confidence: %s", strings.TrimPrefix(d.Confidence.String(), "DECISION_CONFIDENCE_"))
		if d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
			details += fmt.Sprintf(" | Scope: %s", strings.TrimPrefix(d.Scope.String(), "DECISION_SCOPE_"))
		}
		doc.Panel(PanelInfo, details)
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:       render.DocumentADR,
		Title:      fmt.Sprintf("ADR: %s", d.Title),
		Body:       b,
		SpecSlug:   d.OriginSpec,
		DecisionID: d.Slug,
	}, nil
}

func decisionStatusName(s specv1.DecisionStatus) string {
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

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/adf/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/render/adf/adr.go internal/render/adf/adr_test.go
git commit -s -m "feat(render/adf): add MADR-format ADF ADR renderer

Renders Decision proto as ADF with status lozenges, context section,
considered-options table, and confidence/scope info panel."
```

---

## Phase 5: Proto, Config, and Storage

### Task 11: PublishService Proto Definition

**Files:**
- Create: `proto/specgraph/v1/publish.proto`

- [ ] **Step 1: Write the proto definition**

```protobuf
// proto/specgraph/v1/publish.proto
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// --- Enums ---

enum PublishState {
  PUBLISH_STATE_UNSPECIFIED = 0;
  PUBLISH_STATE_DRAFT = 1;
  PUBLISH_STATE_SYNCED = 2;
  PUBLISH_STATE_ERROR = 3;
  PUBLISH_STATE_UNPUBLISHED = 4;
}

enum DocumentKind {
  DOCUMENT_KIND_UNSPECIFIED = 0;
  DOCUMENT_KIND_PRD = 1;
  DOCUMENT_KIND_SDD = 2;
  DOCUMENT_KIND_ADR = 3;
}

enum FeedbackKind {
  FEEDBACK_KIND_UNSPECIFIED = 0;
  FEEDBACK_KIND_INLINE = 1;
  FEEDBACK_KIND_FOOTER = 2;
}

// --- Messages ---

message PageMapping {
  string spec_slug = 1;
  DocumentKind doc_kind = 2;
  string decision_slug = 3;      // only for ADR
  string page_id = 4;            // Confluence page ID
  int32 page_version = 5;        // last published Confluence version
  int32 spec_version = 6;        // spec version at last publish
  PublishState state = 7;
  string error_message = 8;
  google.protobuf.Timestamp last_sync = 9;
}

message PublishStatusEntry {
  string spec_slug = 1;
  PageMapping prd = 2;
  PageMapping sdd = 3;
  repeated PageMapping adrs = 4;
  int32 new_comments = 5;
  google.protobuf.Timestamp last_sync = 6;
}

message Feedback {
  string external_id = 1;
  string author = 2;
  string body = 3;
  google.protobuf.Timestamp timestamp = 4;
  FeedbackKind kind = 5;
  string stage = 6;              // routed authoring stage (inline only)
  bool is_question = 7;
  string parent_id = 8;          // reply threading
  string spec_slug = 9;
}

// --- Requests/Responses ---

message PublishRequest {
  string slug = 1;
}

message PublishResponse {
  repeated PageMapping mappings = 1;
}

message GetPublishStatusRequest {
  string slug = 1;               // optional: if empty, returns all
}

message GetPublishStatusResponse {
  repeated PublishStatusEntry entries = 1;
}

message SyncCommentsRequest {
  string slug = 1;               // optional: if empty, syncs all
}

message SyncCommentsResponse {
  repeated Feedback feedback = 1;
  int32 new_count = 2;
  int32 spec_count = 3;
}

message UnpublishRequest {
  string slug = 1;
}

message UnpublishResponse {
  int32 pages_removed = 1;
}

// --- Service ---

service PublishService {
  rpc Publish(PublishRequest) returns (PublishResponse);
  rpc GetPublishStatus(GetPublishStatusRequest) returns (GetPublishStatusResponse);
  rpc SyncComments(SyncCommentsRequest) returns (SyncCommentsResponse);
  rpc Unpublish(UnpublishRequest) returns (UnpublishResponse);
}
```

- [ ] **Step 2: Generate Go code**

Run: `task proto`
Expected: Success — new `gen/specgraph/v1/publish.pb.go` and `publish.connect.go` generated.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add proto/specgraph/v1/publish.proto gen/specgraph/v1/publish.pb.go gen/specgraph/v1/publish.connect.go
git commit -s -m "feat(proto): add PublishService for Confluence publishing

Defines Publish, GetPublishStatus, SyncComments, Unpublish RPCs.
Includes PageMapping, Feedback, and PublishStatusEntry messages."
```

---

### Task 12: Config — Add Publish Section

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add publish config types**

Add to `internal/config/config.go`:

```go
// PublishConfig describes publishing settings.
type PublishConfig struct {
	Confluence ConfluencePublishConfig `yaml:"confluence"`
}

// ConfluencePublishConfig describes Confluence-specific publishing settings.
type ConfluencePublishConfig struct {
	CloudID      string   `yaml:"cloud_id"`
	SpaceKey     string   `yaml:"space_key"`
	ParentPageID string   `yaml:"parent_page_id"`
	Auth         AuthMethodConfig `yaml:"auth"`
	PollInterval string   `yaml:"poll_interval"` // duration string, e.g. "15m"
	AutoPublish  bool     `yaml:"auto_publish"`
	Labels       []string `yaml:"labels"`
}

// AuthMethodConfig describes authentication for external services.
type AuthMethodConfig struct {
	Method string `yaml:"method"` // api_token | oauth2
}
```

Add `Publish PublishConfig` field to the `Config` struct:

```go
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Storage StorageConfig `yaml:"storage"`
	Sync    SyncConfig    `yaml:"sync"`
	Publish PublishConfig `yaml:"publish"`
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/config/config.go
git commit -s -m "feat(config): add publish.confluence config section

Supports cloud_id, space_key, parent_page_id, auth method,
poll_interval, auto_publish flag, and labels."
```

---

### Task 13: Storage — PublishBackend Interface + Domain Types

**Files:**
- Create: `internal/storage/publish.go`
- Modify: `internal/storage/storage.go` — add `PublishBackend` to `ScopedBackend`

- [ ] **Step 1: Define storage types and interface**

```go
// internal/storage/publish.go
package storage

import (
	"context"
	"time"
)

// PublishState represents the state of a published document.
type PublishState string

const (
	PublishStateDraft       PublishState = "draft"
	PublishStateSynced      PublishState = "synced"
	PublishStateError       PublishState = "error"
	PublishStateUnpublished PublishState = "unpublished"
)

// DocumentKind identifies the type of published document.
type DocumentKind string

const (
	DocumentKindPRD DocumentKind = "prd"
	DocumentKindSDD DocumentKind = "sdd"
	DocumentKindADR DocumentKind = "adr"
)

// FeedbackKind identifies the type of Confluence comment.
type FeedbackKind string

const (
	FeedbackKindInline FeedbackKind = "inline"
	FeedbackKindFooter FeedbackKind = "footer"
)

// PageMapping tracks a published document's Confluence page.
type PageMapping struct {
	SpecSlug     string
	DocKind      DocumentKind
	DecisionSlug string // only for ADRs
	PageID       string
	PageVersion  int
	SpecVersion  int32
	State        PublishState
	ErrorMessage string
	LastSync     time.Time
	CreatedAt    time.Time
}

// FeedbackEntry represents an ingested Confluence comment.
type FeedbackEntry struct {
	ID         string
	ExternalID string // Confluence comment ID (dedup key)
	SpecSlug   string
	Author     string
	Body       string
	Timestamp  time.Time
	Kind       FeedbackKind
	Stage      string // routed authoring stage (inline only)
	IsQuestion bool
	ParentID   string // reply threading
	CreatedAt  time.Time
}

// PublishBackend manages page mappings and feedback entries.
type PublishBackend interface {
	UpsertPageMapping(ctx context.Context, m *PageMapping) (*PageMapping, error)
	GetPageMapping(ctx context.Context, specSlug string, kind DocumentKind, decisionSlug string) (*PageMapping, error)
	ListPageMappings(ctx context.Context, specSlug string) ([]*PageMapping, error)
	DeletePageMappings(ctx context.Context, specSlug string) (int, error)

	StoreFeedback(ctx context.Context, entry *FeedbackEntry) (*FeedbackEntry, error)
	ListFeedback(ctx context.Context, specSlug string, sinceExternalID string) ([]*FeedbackEntry, error)
	CountNewFeedback(ctx context.Context, specSlug string) (int, error)
}
```

- [ ] **Step 2: Add PublishBackend to ScopedBackend**

In `internal/storage/storage.go`, add `PublishBackend` to the `ScopedBackend` interface:

```go
type ScopedBackend interface {
	Backend
	GraphBackend
	DecisionBackend
	ClaimBackend
	ConstitutionBackend
	AuthoringBackend
	FindingsBackend
	ExecutionBackend
	LifecycleBackend
	SyncBackend
	ProjectBackend
	SliceBackend
	ConversationBackend
	ChangeLogBackend
	SpecVersionBackend
	TransactionalBackend
	PublishBackend
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/`
Expected: PASS (interface only — Postgres impl comes next)

Note: `go build ./...` will fail because the Postgres backend doesn't implement `PublishBackend` yet. That's expected and will be fixed in the next task.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/publish.go internal/storage/storage.go
git commit -s -m "feat(storage): add PublishBackend interface for page mappings and feedback

Defines PageMapping (spec↔Confluence page tracking) and FeedbackEntry
(ingested comment) domain types. Added to ScopedBackend composition."
```

---

### Task 14: Postgres Migration + PublishBackend Implementation

**Files:**
- Create: `internal/storage/postgres/migrations/NNNN_publish_mappings.sql`
- Create: `internal/storage/postgres/publish.go`
- Create: `internal/storage/postgres/publish_test.go`

- [ ] **Step 1: Determine the next migration number**

```bash
ls internal/storage/postgres/migrations/ | tail -5
```

Use the next sequential number (format: `NNNN_publish_mappings.sql`).

- [ ] **Step 2: Write the migration**

```sql
-- internal/storage/postgres/migrations/NNNN_publish_mappings.sql
-- +goose Up
CREATE TABLE page_mappings (
    spec_slug      TEXT NOT NULL,
    doc_kind       TEXT NOT NULL CHECK (doc_kind IN ('prd', 'sdd', 'adr')),
    decision_slug  TEXT NOT NULL DEFAULT '',
    page_id        TEXT NOT NULL,
    page_version   INTEGER NOT NULL DEFAULT 0,
    spec_version   INTEGER NOT NULL DEFAULT 0,
    state          TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'synced', 'error', 'unpublished')),
    error_message  TEXT NOT NULL DEFAULT '',
    last_sync      TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (spec_slug, doc_kind, decision_slug)
);

CREATE TABLE feedback_entries (
    id             TEXT PRIMARY KEY,
    external_id    TEXT NOT NULL UNIQUE,
    spec_slug      TEXT NOT NULL,
    author         TEXT NOT NULL DEFAULT '',
    body           TEXT NOT NULL DEFAULT '',
    timestamp      TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind           TEXT NOT NULL CHECK (kind IN ('inline', 'footer')),
    stage          TEXT NOT NULL DEFAULT '',
    is_question    BOOLEAN NOT NULL DEFAULT false,
    parent_id      TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_feedback_spec_slug ON feedback_entries (spec_slug);
CREATE INDEX idx_feedback_external_id ON feedback_entries (external_id);

-- +goose Down
DROP TABLE IF EXISTS feedback_entries;
DROP TABLE IF EXISTS page_mappings;
```

- [ ] **Step 3: Write the Postgres implementation**

```go
// internal/storage/postgres/publish.go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/oklog/ulid/v2"
	"github.com/specgraph/specgraph/internal/storage"
)

func (b *Backend) UpsertPageMapping(ctx context.Context, m *storage.PageMapping) (*storage.PageMapping, error) {
	q := b.executeQuery(ctx)
	now := time.Now()
	err := q.QueryRow(ctx, `
		INSERT INTO page_mappings (spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (spec_slug, doc_kind, decision_slug)
		DO UPDATE SET page_id = $4, page_version = $5, spec_version = $6, state = $7, error_message = $8, last_sync = $9
		RETURNING last_sync, created_at`,
		m.SpecSlug, m.DocKind, m.DecisionSlug, m.PageID, m.PageVersion, m.SpecVersion, m.State, m.ErrorMessage, now, now,
	).Scan(&m.LastSync, &m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("upsert page mapping: %w", err)
	}
	return m, nil
}

func (b *Backend) GetPageMapping(ctx context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error) {
	q := b.executeQuery(ctx)
	var m storage.PageMapping
	err := q.QueryRow(ctx, `
		SELECT spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at
		FROM page_mappings
		WHERE spec_slug = $1 AND doc_kind = $2 AND decision_slug = $3`,
		specSlug, kind, decisionSlug,
	).Scan(&m.SpecSlug, &m.DocKind, &m.DecisionSlug, &m.PageID, &m.PageVersion, &m.SpecVersion, &m.State, &m.ErrorMessage, &m.LastSync, &m.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get page mapping: %w", err)
	}
	return &m, nil
}

func (b *Backend) ListPageMappings(ctx context.Context, specSlug string) ([]*storage.PageMapping, error) {
	q := b.executeQuery(ctx)
	query := `SELECT spec_slug, doc_kind, decision_slug, page_id, page_version, spec_version, state, error_message, last_sync, created_at FROM page_mappings`
	var args []any
	if specSlug != "" {
		query += ` WHERE spec_slug = $1`
		args = append(args, specSlug)
	}
	query += ` ORDER BY spec_slug, doc_kind, decision_slug`
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list page mappings: %w", err)
	}
	defer rows.Close()
	var mappings []*storage.PageMapping
	for rows.Next() {
		var m storage.PageMapping
		if err := rows.Scan(&m.SpecSlug, &m.DocKind, &m.DecisionSlug, &m.PageID, &m.PageVersion, &m.SpecVersion, &m.State, &m.ErrorMessage, &m.LastSync, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan page mapping: %w", err)
		}
		mappings = append(mappings, &m)
	}
	return mappings, rows.Err()
}

func (b *Backend) DeletePageMappings(ctx context.Context, specSlug string) (int, error) {
	q := b.executeQuery(ctx)
	tag, err := q.Exec(ctx, `DELETE FROM page_mappings WHERE spec_slug = $1`, specSlug)
	if err != nil {
		return 0, fmt.Errorf("delete page mappings: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (b *Backend) StoreFeedback(ctx context.Context, entry *storage.FeedbackEntry) (*storage.FeedbackEntry, error) {
	q := b.executeQuery(ctx)
	if entry.ID == "" {
		entry.ID = "fb-" + ulid.Make().String()
	}
	now := time.Now()
	_, err := q.Exec(ctx, `
		INSERT INTO feedback_entries (id, external_id, spec_slug, author, body, timestamp, kind, stage, is_question, parent_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (external_id) DO NOTHING`,
		entry.ID, entry.ExternalID, entry.SpecSlug, entry.Author, entry.Body, entry.Timestamp, entry.Kind, entry.Stage, entry.IsQuestion, entry.ParentID, now,
	)
	if err != nil {
		return nil, fmt.Errorf("store feedback: %w", err)
	}
	entry.CreatedAt = now
	return entry, nil
}

func (b *Backend) ListFeedback(ctx context.Context, specSlug string, sinceExternalID string) ([]*storage.FeedbackEntry, error) {
	q := b.executeQuery(ctx)
	query := `SELECT id, external_id, spec_slug, author, body, timestamp, kind, stage, is_question, parent_id, created_at
		FROM feedback_entries WHERE spec_slug = $1`
	args := []any{specSlug}
	if sinceExternalID != "" {
		query += ` AND created_at > (SELECT created_at FROM feedback_entries WHERE external_id = $2)`
		args = append(args, sinceExternalID)
	}
	query += ` ORDER BY timestamp`
	rows, err := q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list feedback: %w", err)
	}
	defer rows.Close()
	var entries []*storage.FeedbackEntry
	for rows.Next() {
		var e storage.FeedbackEntry
		if err := rows.Scan(&e.ID, &e.ExternalID, &e.SpecSlug, &e.Author, &e.Body, &e.Timestamp, &e.Kind, &e.Stage, &e.IsQuestion, &e.ParentID, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan feedback: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (b *Backend) CountNewFeedback(ctx context.Context, specSlug string) (int, error) {
	q := b.executeQuery(ctx)
	var count int
	err := q.QueryRow(ctx, `SELECT COUNT(*) FROM feedback_entries WHERE spec_slug = $1`, specSlug).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count feedback: %w", err)
	}
	return count, nil
}
```

- [ ] **Step 4: Write integration test**

```go
// internal/storage/postgres/publish_test.go
package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestUpsertAndGetPageMapping(t *testing.T) {
	ctx := context.Background()
	b := setupTestBackend(t)

	m := &storage.PageMapping{
		SpecSlug:    "test-spec",
		DocKind:     storage.DocumentKindPRD,
		PageID:      "12345",
		PageVersion: 1,
		SpecVersion: 1,
		State:       storage.PublishStateSynced,
	}
	got, err := b.UpsertPageMapping(ctx, m)
	if err != nil {
		t.Fatalf("UpsertPageMapping: %v", err)
	}
	if got.PageID != "12345" {
		t.Errorf("PageID = %q, want %q", got.PageID, "12345")
	}

	fetched, err := b.GetPageMapping(ctx, "test-spec", storage.DocumentKindPRD, "")
	if err != nil {
		t.Fatalf("GetPageMapping: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetPageMapping returned nil")
	}
	if fetched.PageID != "12345" {
		t.Errorf("fetched PageID = %q", fetched.PageID)
	}
}

func TestStoreFeedbackDedup(t *testing.T) {
	ctx := context.Background()
	b := setupTestBackend(t)

	entry := &storage.FeedbackEntry{
		ExternalID: "conf-comment-1",
		SpecSlug:   "test-spec",
		Author:     "alice",
		Body:       "Looks good",
		Kind:       storage.FeedbackKindFooter,
	}
	_, err := b.StoreFeedback(ctx, entry)
	if err != nil {
		t.Fatalf("StoreFeedback: %v", err)
	}
	// Store again — should be idempotent (ON CONFLICT DO NOTHING)
	_, err = b.StoreFeedback(ctx, &storage.FeedbackEntry{
		ExternalID: "conf-comment-1",
		SpecSlug:   "test-spec",
		Author:     "alice",
		Body:       "Looks good updated",
		Kind:       storage.FeedbackKindFooter,
	})
	if err != nil {
		t.Fatalf("StoreFeedback duplicate: %v", err)
	}
	entries, err := b.ListFeedback(ctx, "test-spec", "")
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}
```

Note: `setupTestBackend` is an existing test helper in the postgres package that creates a testcontainers-backed Postgres instance. These tests require Docker and the `integration` build tag.

- [ ] **Step 5: Run integration tests**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestUpsert -v`
Expected: PASS (requires Docker running)

- [ ] **Step 6: Commit**

```bash
git add internal/storage/postgres/migrations/ internal/storage/postgres/publish.go internal/storage/postgres/publish_test.go
git commit -s -m "feat(storage/postgres): add PublishBackend implementation

Page mapping table with composite PK (spec_slug, doc_kind, decision_slug).
Feedback entries table with external_id uniqueness for dedup.
Upsert semantics for page mappings, ON CONFLICT DO NOTHING for feedback."
```

---

## Phase 6: Confluence Client + Publisher

### Task 15: Publisher Interface + Types

**Files:**
- Create: `internal/publish/publish.go`

- [ ] **Step 1: Define publisher interfaces and types**

```go
// internal/publish/publish.go

// Package publish defines interfaces for publishing SpecGraph documents
// to external systems.
package publish

import (
	"context"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// PublishResult describes the outcome of a publish or update operation.
type PublishResult struct {
	Mappings []PageRef
}

// PageRef identifies a published page.
type PageRef struct {
	DocKind    render.DocumentKind
	DecisionID string
	PageID     string
	Version    int
	URL        string
}

// PublishStatus describes the current state of a spec's published documents.
type PublishStatus struct {
	SpecSlug   string
	PRD        *PageState
	SDD        *PageState
	ADRs       []PageState
	NewComments int
	LastSync   time.Time
}

// PageState describes a single published page.
type PageState struct {
	PageID      string
	State       string
	SpecVersion int32
	LastSync    time.Time
}

// Feedback represents an ingested external comment.
type Feedback struct {
	ExternalID string
	Author     string
	Body       string
	Timestamp  time.Time
	Kind       FeedbackKind
	Stage      string // routed authoring stage (inline only)
	IsQuestion bool
	ParentID   string
	SpecSlug   string
}

// FeedbackKind distinguishes inline vs footer comments.
type FeedbackKind string

const (
	FeedbackInline FeedbackKind = "inline"
	FeedbackFooter FeedbackKind = "footer"
)

// Publisher manages document lifecycle in an external system.
type Publisher interface {
	Name() string
	Publish(ctx context.Context, slug string, docs []render.Document) (PublishResult, error)
	Update(ctx context.Context, slug string, docs []render.Document, changelog *specv1.ChangeLogEntry) (PublishResult, error)
	Unpublish(ctx context.Context, slug string) error
	Status(ctx context.Context, slug string) (PublishStatus, error)
}

// FeedbackSource ingests external feedback back into SpecGraph.
type FeedbackSource interface {
	Poll(ctx context.Context, slug string) ([]Feedback, error)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/publish/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/publish/publish.go
git commit -s -m "feat(publish): add Publisher and FeedbackSource interfaces

Core abstractions for document publishing lifecycle. Publisher handles
create/update/unpublish/status. FeedbackSource handles comment polling."
```

---

### Task 16: Confluence REST API Client

**Files:**
- Create: `internal/publish/confluence/config.go`
- Create: `internal/publish/confluence/client.go`
- Create: `internal/publish/confluence/client_test.go`

- [ ] **Step 1: Define config types**

```go
// internal/publish/confluence/config.go

// Package confluence implements the publish.Publisher interface for Confluence.
package confluence

import "time"

// Config holds Confluence publishing configuration.
type Config struct {
	CloudID      string
	SpaceKey     string
	ParentPageID string
	BaseURL      string        // defaults to https://api.atlassian.com
	APIToken     string        // from CONFLUENCE_API_TOKEN
	UserEmail    string        // from CONFLUENCE_USER_EMAIL
	PollInterval time.Duration // default 15m
	Labels       []string
}
```

- [ ] **Step 2: Write failing client tests**

```go
// internal/publish/confluence/client_test.go
package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/wiki/api/v2/pages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Test Page" {
			t.Errorf("title = %v", body["title"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "123",
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/123"},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{
		CloudID:      "test-cloud",
		BaseURL:      srv.URL,
		APIToken:     "test-token",
		UserEmail:    "test@example.com",
		SpaceKey:     "ENG",
		ParentPageID: "parent-1",
	})
	page, err := c.CreatePage(context.Background(), "Test Page", "parent-1", []byte(`{"type":"doc","version":1}`))
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if page.ID != "123" {
		t.Errorf("ID = %q", page.ID)
	}
}

func TestGetPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":      "123",
			"title":   "Test Page",
			"version": map[string]any{"number": 2},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	page, err := c.GetPage(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Version != 2 {
		t.Errorf("Version = %d", page.Version)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/publish/confluence/`
Expected: FAIL — `NewClient`, `CreatePage`, `GetPage` undefined.

- [ ] **Step 4: Implement the client**

```go
// internal/publish/confluence/client.go
package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// PageInfo represents a Confluence page response.
type PageInfo struct {
	ID      string
	Title   string
	Version int
	WebURL  string
}

// CommentInfo represents a Confluence comment.
type CommentInfo struct {
	ID               string
	Body             string
	Author           string
	CreatedAt        string
	InlineProperties map[string]any // for inline comments: textSelection, etc.
	ParentID         string
}

// Client communicates with the Confluence REST API v2.
type Client struct {
	cfg    Config
	http   *http.Client
}

// NewClient creates a Confluence API client.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = fmt.Sprintf("https://api.atlassian.com/ex/confluence/%s", cfg.CloudID)
	}
	return &Client{cfg: cfg, http: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.UserEmail, c.cfg.APIToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("confluence API %s %s: %d %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// CreatePage creates a new Confluence page.
func (c *Client) CreatePage(ctx context.Context, title, parentID string, adfBody []byte) (*PageInfo, error) {
	payload := map[string]any{
		"spaceId":  c.cfg.SpaceKey,
		"title":    title,
		"parentId": parentID,
		"status":   "current",
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(adfBody),
		},
	}
	respBody, err := c.do(ctx, http.MethodPost, "/wiki/api/v2/pages", payload)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// UpdatePage updates an existing Confluence page.
func (c *Client) UpdatePage(ctx context.Context, pageID, title string, version int, adfBody []byte) (*PageInfo, error) {
	payload := map[string]any{
		"id":     pageID,
		"title":  title,
		"status": "current",
		"version": map[string]any{
			"number": version + 1,
		},
		"body": map[string]any{
			"representation": "atlas_doc_format",
			"value":          string(adfBody),
		},
	}
	respBody, err := c.do(ctx, http.MethodPut, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), payload)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// GetPage retrieves a Confluence page by ID.
func (c *Client) GetPage(ctx context.Context, pageID string) (*PageInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parsePageResponse(respBody)
}

// DeletePage deletes a Confluence page.
func (c *Client) DeletePage(ctx context.Context, pageID string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/wiki/api/v2/pages/%s", pageID), nil)
	return err
}

// GetInlineComments retrieves inline comments for a page.
func (c *Client) GetInlineComments(ctx context.Context, pageID string) ([]CommentInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s/inline-comments", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parseCommentsResponse(respBody)
}

// GetFooterComments retrieves footer comments for a page.
func (c *Client) GetFooterComments(ctx context.Context, pageID string) ([]CommentInfo, error) {
	respBody, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/wiki/api/v2/pages/%s/footer-comments", pageID), nil)
	if err != nil {
		return nil, err
	}
	return parseCommentsResponse(respBody)
}

func parsePageResponse(data []byte) (*PageInfo, error) {
	var resp struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number int `json:"number"`
		} `json:"version"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse page response: %w", err)
	}
	return &PageInfo{
		ID:      resp.ID,
		Title:   resp.Title,
		Version: resp.Version.Number,
		WebURL:  resp.Links.WebUI,
	}, nil
}

func parseCommentsResponse(data []byte) ([]CommentInfo, error) {
	var resp struct {
		Results []struct {
			ID   string `json:"id"`
			Body struct {
				Storage struct {
					Value string `json:"value"`
				} `json:"storage"`
			} `json:"body"`
			Version struct {
				AuthorID string `json:"authorId"`
			} `json:"version"`
			Properties struct {
				InlineMarker map[string]any `json:"inline-marker"`
			} `json:"properties"`
			CreatedAt string `json:"createdAt"`
			ParentID  string `json:"parentId"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse comments: %w", err)
	}
	comments := make([]CommentInfo, len(resp.Results))
	for i, r := range resp.Results {
		comments[i] = CommentInfo{
			ID:               r.ID,
			Body:             r.Body.Storage.Value,
			Author:           r.Version.AuthorID,
			CreatedAt:        r.CreatedAt,
			InlineProperties: r.Properties.InlineMarker,
			ParentID:         r.ParentID,
		}
	}
	return comments, nil
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/publish/confluence/`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/publish/confluence/
git commit -s -m "feat(publish/confluence): add Confluence REST API client

HTTP client for pages (CRUD) and comments (inline + footer).
Uses ADF body format via atlas_doc_format representation.
Basic auth with API token."
```

---

### Task 17: Confluence Publisher Implementation

**Files:**
- Create: `internal/publish/confluence/publisher.go`
- Create: `internal/publish/confluence/publisher_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/publish/confluence/publisher_test.go
package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/render"
)

func TestPublisherPublish(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/1"},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, Config{ParentPageID: "root", Labels: []string{"specgraph"}})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Publish(context.Background(), "test", docs)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(result.Mappings) != 1 {
		t.Errorf("mappings = %d, want 1", len(result.Mappings))
	}
	if store.upsertCount != 1 {
		t.Errorf("upsert count = %d, want 1", store.upsertCount)
	}
}

func TestPublisherName(t *testing.T) {
	pub := NewPublisher(nil, nil, Config{})
	if pub.Name() != "confluence" {
		t.Errorf("Name() = %q", pub.Name())
	}
}
```

This test uses a `fakePublishStore` — a minimal in-memory implementation of the storage methods the publisher needs. Define it in the test file:

```go
type fakePublishStore struct {
	upsertCount int
	mappings    map[string]*storage.PageMapping
}

func (f *fakePublishStore) UpsertPageMapping(_ context.Context, m *storage.PageMapping) (*storage.PageMapping, error) {
	f.upsertCount++
	if f.mappings == nil {
		f.mappings = make(map[string]*storage.PageMapping)
	}
	key := m.SpecSlug + string(m.DocKind) + m.DecisionSlug
	f.mappings[key] = m
	return m, nil
}

func (f *fakePublishStore) GetPageMapping(_ context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error) {
	if f.mappings == nil {
		return nil, nil
	}
	return f.mappings[specSlug+string(kind)+decisionSlug], nil
}

func (f *fakePublishStore) ListPageMappings(_ context.Context, specSlug string) ([]*storage.PageMapping, error) {
	var result []*storage.PageMapping
	for _, m := range f.mappings {
		if specSlug == "" || m.SpecSlug == specSlug {
			result = append(result, m)
		}
	}
	return result, nil
}

func (f *fakePublishStore) DeletePageMappings(_ context.Context, _ string) (int, error) {
	count := len(f.mappings)
	f.mappings = nil
	return count, nil
}
```

- [ ] **Step 2: Implement publisher**

```go
// internal/publish/confluence/publisher.go
package confluence

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/storage"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// PublishStore is the subset of storage.PublishBackend the publisher needs.
type PublishStore interface {
	UpsertPageMapping(ctx context.Context, m *storage.PageMapping) (*storage.PageMapping, error)
	GetPageMapping(ctx context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error)
	ListPageMappings(ctx context.Context, specSlug string) ([]*storage.PageMapping, error)
	DeletePageMappings(ctx context.Context, specSlug string) (int, error)
}

// Publisher implements publish.Publisher for Confluence.
type Publisher struct {
	client *Client
	store  PublishStore
	cfg    Config
}

// NewPublisher creates a Confluence publisher.
func NewPublisher(client *Client, store PublishStore, cfg Config) *Publisher {
	return &Publisher{client: client, store: store, cfg: cfg}
}

func (p *Publisher) Name() string { return "confluence" }

func (p *Publisher) Publish(ctx context.Context, slug string, docs []render.Document) (publish.PublishResult, error) {
	var result publish.PublishResult
	for _, doc := range docs {
		parentID := p.cfg.ParentPageID
		// SDDs and ADRs are children of the PRD page
		if doc.Kind != render.DocumentPRD {
			prdMapping, err := p.store.GetPageMapping(ctx, slug, storage.DocumentKindPRD, "")
			if err != nil {
				return result, fmt.Errorf("get PRD mapping: %w", err)
			}
			if prdMapping != nil {
				parentID = prdMapping.PageID
			}
		}

		page, err := p.client.CreatePage(ctx, doc.Title, parentID, doc.Body)
		if err != nil {
			return result, fmt.Errorf("create page %q: %w", doc.Title, err)
		}

		mapping := &storage.PageMapping{
			SpecSlug:     slug,
			DocKind:      storageDocKind(doc.Kind),
			DecisionSlug: doc.DecisionID,
			PageID:       page.ID,
			PageVersion:  page.Version,
			State:        storage.PublishStateSynced,
		}
		if _, err := p.store.UpsertPageMapping(ctx, mapping); err != nil {
			return result, fmt.Errorf("store mapping: %w", err)
		}

		result.Mappings = append(result.Mappings, publish.PageRef{
			DocKind:    doc.Kind,
			DecisionID: doc.DecisionID,
			PageID:     page.ID,
			Version:    page.Version,
			URL:        page.WebURL,
		})
	}
	return result, nil
}

func (p *Publisher) Update(ctx context.Context, slug string, docs []render.Document, _ *specv1.ChangeLogEntry) (publish.PublishResult, error) {
	var result publish.PublishResult
	for _, doc := range docs {
		existing, err := p.store.GetPageMapping(ctx, slug, storageDocKind(doc.Kind), doc.DecisionID)
		if err != nil {
			return result, fmt.Errorf("get mapping: %w", err)
		}
		if existing == nil {
			// Not yet published — create instead
			return p.Publish(ctx, slug, docs)
		}

		page, err := p.client.UpdatePage(ctx, existing.PageID, doc.Title, existing.PageVersion, doc.Body)
		if err != nil {
			return result, fmt.Errorf("update page %q: %w", doc.Title, err)
		}

		existing.PageVersion = page.Version
		existing.State = storage.PublishStateSynced
		if _, err := p.store.UpsertPageMapping(ctx, existing); err != nil {
			return result, fmt.Errorf("store mapping: %w", err)
		}

		result.Mappings = append(result.Mappings, publish.PageRef{
			DocKind: doc.Kind,
			PageID:  page.ID,
			Version: page.Version,
		})
	}
	return result, nil
}

func (p *Publisher) Unpublish(ctx context.Context, slug string) error {
	mappings, err := p.store.ListPageMappings(ctx, slug)
	if err != nil {
		return fmt.Errorf("list mappings: %w", err)
	}
	// Delete child pages first, then parent
	for i := len(mappings) - 1; i >= 0; i-- {
		if err := p.client.DeletePage(ctx, mappings[i].PageID); err != nil {
			return fmt.Errorf("delete page %s: %w", mappings[i].PageID, err)
		}
	}
	if _, err := p.store.DeletePageMappings(ctx, slug); err != nil {
		return fmt.Errorf("delete mappings: %w", err)
	}
	return nil
}

func (p *Publisher) Status(ctx context.Context, slug string) (publish.PublishStatus, error) {
	mappings, err := p.store.ListPageMappings(ctx, slug)
	if err != nil {
		return publish.PublishStatus{}, err
	}
	status := publish.PublishStatus{SpecSlug: slug}
	for _, m := range mappings {
		ps := &publish.PageState{
			PageID:      m.PageID,
			State:       string(m.State),
			SpecVersion: m.SpecVersion,
			LastSync:    m.LastSync,
		}
		switch m.DocKind {
		case storage.DocumentKindPRD:
			status.PRD = ps
		case storage.DocumentKindSDD:
			status.SDD = ps
		case storage.DocumentKindADR:
			status.ADRs = append(status.ADRs, *ps)
		}
		if m.LastSync.After(status.LastSync) {
			status.LastSync = m.LastSync
		}
	}
	return status, nil
}

func storageDocKind(k render.DocumentKind) storage.DocumentKind {
	switch k {
	case render.DocumentPRD:
		return storage.DocumentKindPRD
	case render.DocumentSDD:
		return storage.DocumentKindSDD
	case render.DocumentADR:
		return storage.DocumentKindADR
	default:
		return storage.DocumentKindPRD
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/publish/confluence/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/publish/confluence/publisher.go internal/publish/confluence/publisher_test.go
git commit -s -m "feat(publish/confluence): add Confluence publisher

Implements publish.Publisher with page tree management.
PRD as root page, SDD/ADRs as children. Tracks page mappings
in storage for idempotent updates and unpublish."
```

---

### Task 18: Confluence Comment Feedback Source

**Files:**
- Create: `internal/publish/confluence/feedback.go`
- Create: `internal/publish/confluence/feedback_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/publish/confluence/feedback_test.go
package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestFeedbackPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id":        "comment-1",
					"body":      map[string]any{"storage": map[string]any{"value": "Looks good"}},
					"version":   map[string]any{"authorId": "alice"},
					"createdAt": "2026-04-10T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-specprd": {
				SpecSlug: "test-spec",
				DocKind:  storage.DocumentKindPRD,
				PageID:   "page-1",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(feedback) == 0 {
		t.Fatal("expected feedback")
	}
	if feedback[0].ExternalID != "comment-1" {
		t.Errorf("ExternalID = %q", feedback[0].ExternalID)
	}
	if feedback[0].Kind != publish.FeedbackFooter {
		t.Errorf("Kind = %q, want footer", feedback[0].Kind)
	}
}
```

- [ ] **Step 2: Implement feedback source**

```go
// internal/publish/confluence/feedback.go
package confluence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/specgraph/specgraph/internal/publish"
)

// FeedbackSource implements publish.FeedbackSource for Confluence.
type FeedbackSource struct {
	client *Client
	store  PublishStore
}

// NewFeedbackSource creates a Confluence feedback source.
func NewFeedbackSource(client *Client, store PublishStore) *FeedbackSource {
	return &FeedbackSource{client: client, store: store}
}

// Poll retrieves new comments from all published pages for a spec.
func (f *FeedbackSource) Poll(ctx context.Context, slug string) ([]publish.Feedback, error) {
	mappings, err := f.store.ListPageMappings(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	var all []publish.Feedback
	for _, m := range mappings {
		// Footer comments
		footerComments, err := f.client.GetFooterComments(ctx, m.PageID)
		if err != nil {
			return nil, fmt.Errorf("get footer comments for page %s: %w", m.PageID, err)
		}
		for _, c := range footerComments {
			all = append(all, toFeedback(c, slug, publish.FeedbackFooter, ""))
		}
		// Inline comments
		inlineComments, err := f.client.GetInlineComments(ctx, m.PageID)
		if err != nil {
			return nil, fmt.Errorf("get inline comments for page %s: %w", m.PageID, err)
		}
		for _, c := range inlineComments {
			stage := routeInlineComment(c, m)
			all = append(all, toFeedback(c, slug, publish.FeedbackInline, stage))
		}
	}
	return all, nil
}

func toFeedback(c CommentInfo, slug string, kind publish.FeedbackKind, stage string) publish.Feedback {
	ts, _ := time.Parse(time.RFC3339, c.CreatedAt)
	return publish.Feedback{
		ExternalID: c.ID,
		Author:     c.Author,
		Body:       c.Body,
		Timestamp:  ts,
		Kind:       kind,
		Stage:      stage,
		IsQuestion: strings.Contains(c.Body, "?"),
		ParentID:   c.ParentID,
		SpecSlug:   slug,
	}
}

// routeInlineComment maps an inline comment to an authoring stage
// based on the page's document kind and the comment's anchor position.
func routeInlineComment(c CommentInfo, m *storage.PageMapping) string {
	switch m.DocKind {
	case storage.DocumentKindPRD:
		return routePRDComment(c)
	case storage.DocumentKindSDD:
		return routeSDDComment(c)
	case storage.DocumentKindADR:
		return "decision"
	default:
		return ""
	}
}

func routePRDComment(_ CommentInfo) string {
	// Default to shape stage for PRD inline comments.
	// Future: parse InlineProperties to determine section anchor.
	return "shape"
}

func routeSDDComment(_ CommentInfo) string {
	// Default to specify stage for SDD inline comments.
	return "specify"
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/publish/confluence/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/publish/confluence/feedback.go internal/publish/confluence/feedback_test.go
git commit -s -m "feat(publish/confluence): add comment feedback source

Polls inline and footer comments from published Confluence pages.
Routes inline comments to authoring stages based on page document kind.
Question heuristic for ? in comment body."
```

---

## Phase 7: Orchestrator, Handler, and CLI

### Task 19: Publish Orchestrator

**Files:**
- Create: `internal/publish/orchestrator.go`
- Create: `internal/publish/orchestrator_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/publish/orchestrator_test.go
package publish

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

type fakeRenderer struct{}

func (f *fakeRenderer) RenderPRD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentPRD, Title: "PRD", Body: []byte("{}")}, nil
}
func (f *fakeRenderer) RenderSDD(_ context.Context, _ *specv1.Spec) (render.Document, error) {
	return render.Document{Kind: render.DocumentSDD, Title: "SDD", Body: []byte("{}")}, nil
}
func (f *fakeRenderer) RenderADR(_ context.Context, _ *specv1.Decision) (render.Document, error) {
	return render.Document{Kind: render.DocumentADR, Title: "ADR", Body: []byte("{}")}, nil
}

type fakePublisher struct {
	published int
}

func (f *fakePublisher) Name() string { return "fake" }
func (f *fakePublisher) Publish(_ context.Context, _ string, docs []render.Document) (PublishResult, error) {
	f.published += len(docs)
	return PublishResult{}, nil
}
func (f *fakePublisher) Update(_ context.Context, _ string, docs []render.Document, _ *specv1.ChangeLogEntry) (PublishResult, error) {
	f.published += len(docs)
	return PublishResult{}, nil
}
func (f *fakePublisher) Unpublish(_ context.Context, _ string) error { return nil }
func (f *fakePublisher) Status(_ context.Context, _ string) (PublishStatus, error) {
	return PublishStatus{}, nil
}

func TestOrchestratorOnShapeComplete(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "shape"}
	err := orch.OnStageComplete(context.Background(), spec, "shape")
	if err != nil {
		t.Fatalf("OnStageComplete: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (PRD)", pub.published)
	}
}

func TestOrchestratorOnSpecifyComplete(t *testing.T) {
	pub := &fakePublisher{}
	orch := NewOrchestrator(&fakeRenderer{}, pub)
	spec := &specv1.Spec{Slug: "test", Stage: "specify"}
	err := orch.OnStageComplete(context.Background(), spec, "specify")
	if err != nil {
		t.Fatalf("OnStageComplete: %v", err)
	}
	if pub.published != 1 {
		t.Errorf("published = %d, want 1 (SDD)", pub.published)
	}
}
```

- [ ] **Step 2: Implement orchestrator**

```go
// internal/publish/orchestrator.go
package publish

import (
	"context"
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// Orchestrator coordinates rendering and publishing on spec events.
type Orchestrator struct {
	renderer  render.Renderer
	publisher Publisher
}

// NewOrchestrator creates a publish orchestrator.
func NewOrchestrator(r render.Renderer, p Publisher) *Orchestrator {
	return &Orchestrator{renderer: r, publisher: p}
}

// OnStageComplete is called when a spec completes an authoring stage.
func (o *Orchestrator) OnStageComplete(ctx context.Context, spec *specv1.Spec, stage string) error {
	switch stage {
	case "shape":
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		_, err = o.publisher.Publish(ctx, spec.Slug, []render.Document{doc})
		return err
	case "specify":
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		_, err = o.publisher.Publish(ctx, spec.Slug, []render.Document{doc})
		return err
	default:
		return nil
	}
}

// OnDecisionLinked is called when a decision is linked to a spec.
func (o *Orchestrator) OnDecisionLinked(ctx context.Context, specSlug string, decision *specv1.Decision) error {
	doc, err := o.renderer.RenderADR(ctx, decision)
	if err != nil {
		return fmt.Errorf("render ADR: %w", err)
	}
	_, err = o.publisher.Publish(ctx, specSlug, []render.Document{doc})
	return err
}

// OnSpecUpdated is called when a spec is updated (new version).
func (o *Orchestrator) OnSpecUpdated(ctx context.Context, spec *specv1.Spec, changelog *specv1.ChangeLogEntry) error {
	var docs []render.Document
	if spec.ShapeOutput != nil {
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		docs = append(docs, doc)
	}
	if spec.SpecifyOutput != nil {
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		docs = append(docs, doc)
	}
	if len(docs) == 0 {
		return nil
	}
	_, err := o.publisher.Update(ctx, spec.Slug, docs, changelog)
	return err
}

// PublishAll renders and publishes all available documents for a spec.
func (o *Orchestrator) PublishAll(ctx context.Context, spec *specv1.Spec, decisions []*specv1.Decision) error {
	var docs []render.Document
	if spec.ShapeOutput != nil {
		doc, err := o.renderer.RenderPRD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render PRD: %w", err)
		}
		docs = append(docs, doc)
	}
	if spec.SpecifyOutput != nil {
		doc, err := o.renderer.RenderSDD(ctx, spec)
		if err != nil {
			return fmt.Errorf("render SDD: %w", err)
		}
		docs = append(docs, doc)
	}
	if len(docs) > 0 {
		if _, err := o.publisher.Publish(ctx, spec.Slug, docs); err != nil {
			return err
		}
	}
	for _, d := range decisions {
		doc, err := o.renderer.RenderADR(ctx, d)
		if err != nil {
			return fmt.Errorf("render ADR %s: %w", d.Slug, err)
		}
		if _, err := o.publisher.Publish(ctx, spec.Slug, []render.Document{doc}); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/publish/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/publish/orchestrator.go internal/publish/orchestrator_test.go
git commit -s -m "feat(publish): add orchestrator for stage-driven publishing

Coordinates rendering and publishing on stage completions, decision
links, and spec updates. PublishAll for manual full-publish."
```

---

### Task 20: ConnectRPC Publish Handler

**Files:**
- Create: `internal/server/publish_handler.go`
- Create: `internal/server/publish_handler_test.go`

- [ ] **Step 1: Implement the handler**

Follow the existing handler pattern (e.g., `sync_handler.go`):

```go
// internal/server/publish_handler.go
package server

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	specgraphv1connect "github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/publish/confluence"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/render/adf"
	"github.com/specgraph/specgraph/internal/storage"
	"net/http"
)

// PublishHandler implements the PublishService ConnectRPC handler.
type PublishHandler struct {
	scoper      storage.Scoper
	orchestrator *publish.Orchestrator
	publisher   publish.Publisher
	feedback    publish.FeedbackSource
}

// RegisterPublishService registers the PublishService handler on the mux.
func RegisterPublishService(
	mux *http.ServeMux,
	scoper storage.Scoper,
	publisher publish.Publisher,
	feedback publish.FeedbackSource,
	opts ...connect.HandlerOption,
) {
	renderer := adf.NewRenderer()
	orch := publish.NewOrchestrator(renderer, publisher)
	h := &PublishHandler{
		scoper:       scoper,
		orchestrator: orch,
		publisher:    publisher,
		feedback:     feedback,
	}
	path, handler := specgraphv1connect.NewPublishServiceHandler(h, opts...)
	mux.Handle(path, handler)
}

func (h *PublishHandler) Publish(ctx context.Context, req *connect.Request[specv1.PublishRequest]) (*connect.Response[specv1.PublishResponse], error) {
	slug := req.Msg.GetSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	backend, err := h.scoper.Scope(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	spec, err := backend.GetSpec(ctx, slug)
	if err != nil {
		return nil, stageError("get spec", err)
	}
	// Convert storage.Spec to proto for renderer
	// This will use the existing converter pattern in the server package
	protoSpec := specToProto(spec)
	decisions, err := getLinkedDecisions(ctx, backend, slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.orchestrator.PublishAll(ctx, protoSpec, decisions); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("publish: %w", err))
	}
	// Return mappings
	mappings, err := backend.ListPageMappings(ctx, slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.PublishResponse{
		Mappings: pageMappingsToProto(mappings),
	}), nil
}

func (h *PublishHandler) GetPublishStatus(ctx context.Context, req *connect.Request[specv1.GetPublishStatusRequest]) (*connect.Response[specv1.GetPublishStatusResponse], error) {
	backend, err := h.scoper.Scope(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	mappings, err := backend.ListPageMappings(ctx, req.Msg.GetSlug())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	entries := groupMappingsToEntries(mappings)
	return connect.NewResponse(&specv1.GetPublishStatusResponse{
		Entries: entries,
	}), nil
}

func (h *PublishHandler) SyncComments(ctx context.Context, req *connect.Request[specv1.SyncCommentsRequest]) (*connect.Response[specv1.SyncCommentsResponse], error) {
	if h.feedback == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("no feedback source configured"))
	}
	slug := req.Msg.GetSlug()
	backend, err := h.scoper.Scope(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	feedback, err := h.feedback.Poll(ctx, slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("poll comments: %w", err))
	}
	var protoFeedback []*specv1.Feedback
	for _, f := range feedback {
		entry := &storage.FeedbackEntry{
			ExternalID: f.ExternalID,
			SpecSlug:   f.SpecSlug,
			Author:     f.Author,
			Body:       f.Body,
			Timestamp:  f.Timestamp,
			Kind:       storage.FeedbackKind(f.Kind),
			Stage:      f.Stage,
			IsQuestion: f.IsQuestion,
			ParentID:   f.ParentID,
		}
		if _, err := backend.StoreFeedback(ctx, entry); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		protoFeedback = append(protoFeedback, feedbackToProto(f))
	}
	return connect.NewResponse(&specv1.SyncCommentsResponse{
		Feedback: protoFeedback,
		NewCount: int32(len(feedback)),
	}), nil
}

func (h *PublishHandler) Unpublish(ctx context.Context, req *connect.Request[specv1.UnpublishRequest]) (*connect.Response[specv1.UnpublishResponse], error) {
	slug := req.Msg.GetSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	if err := h.publisher.Unpublish(ctx, slug); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("unpublish: %w", err))
	}
	return connect.NewResponse(&specv1.UnpublishResponse{}), nil
}

// Helper functions for proto conversion will follow existing patterns
// in the server package (specToProto, etc.)
```

Note: The `specToProto`, `getLinkedDecisions`, `pageMappingsToProto`, `groupMappingsToEntries`, and `feedbackToProto` converter functions follow the existing pattern in the server package where domain types are converted to/from proto types. These will be implemented alongside the handler, referencing the existing conversion patterns in files like `spec_handler.go`.

- [ ] **Step 2: Verify build**

Run: `go build ./internal/server/`
Expected: PASS (after implementing converter stubs)

- [ ] **Step 3: Commit**

```bash
git add internal/server/publish_handler.go
git commit -s -m "feat(server): add PublishService ConnectRPC handler

Implements Publish, GetPublishStatus, SyncComments, Unpublish RPCs.
Uses orchestrator for coordinated render+publish, stores feedback
entries via PublishBackend."
```

---

### Task 21: CLI Commands

**Files:**
- Create: `cmd/specgraph/confluence.go`
- Create: `cmd/specgraph/confluence_test.go`
- Modify: `cmd/specgraph/main.go` — register command

- [ ] **Step 1: Implement CLI commands**

```go
// cmd/specgraph/confluence.go
package main

import (
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	specgraphv1connect "github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/render/markdown"
	"github.com/spf13/cobra"
)

var confluenceCmd = &cobra.Command{
	Use:   "confluence",
	Short: "Manage Confluence publishing",
}

var (
	confluencePublishJSON        bool
	confluenceStatusJSON         bool
	confluenceSyncCommentsJSON   bool
	confluenceUnpublishJSON      bool
)

var confluencePublishCmd = &cobra.Command{
	Use:   "publish <slug>",
	Short: "Publish or re-publish a spec to Confluence",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfluencePublish,
}

var confluenceStatusCmd = &cobra.Command{
	Use:   "status [slug]",
	Short: "Show Confluence publish status",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfluenceStatus,
}

var confluenceSyncCommentsCmd = &cobra.Command{
	Use:   "sync-comments [slug]",
	Short: "Poll Confluence comments for published specs",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConfluenceSyncComments,
}

var confluenceUnpublishCmd = &cobra.Command{
	Use:   "unpublish <slug>",
	Short: "Remove published pages from Confluence",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfluenceUnpublish,
}

func init() {
	confluencePublishCmd.Flags().BoolVar(&confluencePublishJSON, "json", false, "Output as JSON")
	confluenceStatusCmd.Flags().BoolVar(&confluenceStatusJSON, "json", false, "Output as JSON")
	confluenceSyncCommentsCmd.Flags().BoolVar(&confluenceSyncCommentsJSON, "json", false, "Output as JSON")

	confluenceCmd.AddCommand(confluencePublishCmd)
	confluenceCmd.AddCommand(confluenceStatusCmd)
	confluenceCmd.AddCommand(confluenceSyncCommentsCmd)
	confluenceCmd.AddCommand(confluenceUnpublishCmd)
}

func publishClient() (specgraphv1connect.PublishServiceClient, error) {
	return newClient(specgraphv1connect.NewPublishServiceClient)
}

func runConfluencePublish(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	resp, err := client.Publish(cmd.Context(), connect.NewRequest(&specv1.PublishRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	if confluencePublishJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Printf("Published %d pages for %s\n", len(resp.Msg.Mappings), args[0])
	for _, m := range resp.Msg.Mappings {
		fmt.Printf("  %s: page %s (v%d)\n", m.DocKind, m.PageId, m.PageVersion)
	}
	return nil
}

func runConfluenceStatus(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}
	resp, err := client.GetPublishStatus(cmd.Context(), connect.NewRequest(&specv1.GetPublishStatusRequest{
		Slug: slug,
	}))
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if confluenceStatusJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	// Render as markdown table
	fmt.Print(renderPublishStatus(resp.Msg.Entries))
	return nil
}

func runConfluenceSyncComments(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}
	resp, err := client.SyncComments(cmd.Context(), connect.NewRequest(&specv1.SyncCommentsRequest{
		Slug: slug,
	}))
	if err != nil {
		return fmt.Errorf("sync comments: %w", err)
	}
	if confluenceSyncCommentsJSON {
		return printJSON(cmd.OutOrStdout(), resp.Msg)
	}
	fmt.Printf("Synced %d new comments\n", resp.Msg.NewCount)
	return nil
}

func runConfluenceUnpublish(cmd *cobra.Command, args []string) error {
	client, err := publishClient()
	if err != nil {
		return err
	}
	resp, err := client.Unpublish(cmd.Context(), connect.NewRequest(&specv1.UnpublishRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("unpublish: %w", err)
	}
	fmt.Printf("Removed %d pages for %s\n", resp.Msg.PagesRemoved, args[0])
	return nil
}

func renderPublishStatus(entries []*specv1.PublishStatusEntry) string {
	if len(entries) == 0 {
		return "No published specs.\n"
	}
	headers := []string{"Slug", "PRD", "SDD", "ADRs", "Last Sync", "Comments"}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		prd := "-"
		if e.Prd != nil {
			prd = stateString(e.Prd.State)
		}
		sdd := "-"
		if e.Sdd != nil {
			sdd = stateString(e.Sdd.State)
		}
		adrs := fmt.Sprintf("%d", len(e.Adrs))
		lastSync := "-"
		if e.LastSync != nil {
			lastSync = e.LastSync.AsTime().Format("2006-01-02 15:04")
		}
		comments := fmt.Sprintf("%d new", e.NewComments)
		rows[i] = []string{e.SpecSlug, prd, sdd, adrs, lastSync, comments}
	}
	return markdown.ItemTable(headers, rows)
}

func stateString(s specv1.PublishState) string {
	switch s {
	case specv1.PublishState_PUBLISH_STATE_SYNCED:
		return "synced"
	case specv1.PublishState_PUBLISH_STATE_DRAFT:
		return "draft"
	case specv1.PublishState_PUBLISH_STATE_ERROR:
		return "error"
	default:
		return "-"
	}
}
```

Note: `renderPublishStatus` will need `markdown.ItemTable` to be exported. The existing `itemTable` helper in `internal/render/markdown/helpers.go` is currently unexported — export it as `ItemTable` during the refactor task (Task 2).

- [ ] **Step 2: Register command in main.go**

Add to `cmd/specgraph/main.go`'s `init()`:

```go
rootCmd.AddCommand(confluenceCmd)
```

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/specgraph/confluence.go cmd/specgraph/main.go
git commit -s -m "feat(cli): add specgraph confluence commands

publish, status, sync-comments, unpublish commands with --json support.
Status renders as markdown table by default."
```

---

### Task 22: Final Integration — Wire Everything in Server Startup

**Files:**
- Modify: `internal/server/server.go` — add `RegisterPublishService`
- Modify: `cmd/specgraph/serve.go` — wire up Confluence publisher on server start

- [ ] **Step 1: Wire registration in serve.go**

Add Confluence publisher initialization when config has `publish.confluence` configured. Follow the existing pattern of conditionally registering services.

The publisher is only created if `cfg.Publish.Confluence.CloudID` is non-empty. Otherwise, `RegisterPublishService` receives nil for the publisher and feedback source, and the handler returns `CodeUnimplemented` for publish/unpublish RPCs while status still works.

- [ ] **Step 2: Run full build and tests**

Run: `task check`
Expected: PASS — all lint, build, and unit tests pass.

- [ ] **Step 3: Commit**

```bash
git add internal/server/server.go cmd/specgraph/serve.go
git commit -s -m "feat(server): wire PublishService on server startup

Conditionally creates Confluence publisher when publish.confluence
config is present. Registers PublishService handler on the mux."
```

---

### Task 23: Export ItemTable Helper

During Task 2 (render refactor), the `itemTable` helper needs to be exported as `ItemTable` so the CLI status renderer can use it.

**Files:**
- Modify: `internal/render/markdown/helpers.go`

- [ ] **Step 1: Export the helper**

Rename `itemTable` → `ItemTable` in `helpers.go`. Update all internal callers within the `markdown` package to use `ItemTable`.

- [ ] **Step 2: Run tests**

Run: `go test ./internal/render/markdown/`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/render/markdown/
git commit -s -m "refactor(render/markdown): export ItemTable helper

Needed by CLI status renderer for publish status output."
```

---

## Execution Notes

### Task Dependencies

```
Task 1 (interfaces)           ─┐
Task 2 (render refactor)       ├─→ Task 5-7 (markdown renderers)
Task 3-4 (ADF builder+macros) ─┘   Task 8-10 (ADF renderers)

Task 11 (proto) ──────────────────→ Task 20 (handler) → Task 21 (CLI)
Task 12 (config) ─────────────────→ Task 22 (wiring)
Task 13-14 (storage) ────────────→ Task 17 (publisher) → Task 19 (orchestrator)
Task 15 (publish interfaces) ──┬─→ Task 16 (client)
                               └─→ Task 17 (publisher) → Task 18 (feedback)
```

Tasks 1-4 and 11-15 have no interdependencies and can be parallelized.

### Build Tags

- Unit tests: `go test ./...` (default)
- Postgres integration tests (Task 14): `go test -tags integration ./internal/storage/postgres/`
- Full check: `task check`
