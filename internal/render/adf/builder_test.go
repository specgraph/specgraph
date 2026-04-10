// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

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

func TestOrderedList(t *testing.T) {
	doc := NewDocument().OrderedList([]string{"first", "second", "third"})
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	list := content[0].(map[string]any)
	if list["type"] != TypeOrderedList {
		t.Errorf("type = %v, want %q", list["type"], TypeOrderedList)
	}
	items := list["content"].([]any)
	if len(items) != 3 {
		t.Errorf("items count = %d, want 3", len(items))
	}
	// Verify structure of first item
	firstItem := items[0].(map[string]any)
	if firstItem["type"] != TypeListItem {
		t.Errorf("item type = %v, want %q", firstItem["type"], TypeListItem)
	}
	itemContent := firstItem["content"].([]any)
	para := itemContent[0].(map[string]any)
	if para["type"] != TypeParagraph {
		t.Errorf("paragraph type = %v, want %q", para["type"], TypeParagraph)
	}
	paraContent := para["content"].([]any)
	text := paraContent[0].(map[string]any)
	if text["text"] != "first" {
		t.Errorf("text = %v, want %q", text["text"], "first")
	}
}

func TestBlockquote(t *testing.T) {
	doc := NewDocument().Blockquote("This is a quote")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	blockquote := content[0].(map[string]any)
	if blockquote["type"] != TypeBlockquote {
		t.Errorf("type = %v, want %q", blockquote["type"], TypeBlockquote)
	}
	bqContent := blockquote["content"].([]any)
	para := bqContent[0].(map[string]any)
	if para["type"] != TypeParagraph {
		t.Errorf("paragraph type = %v, want %q", para["type"], TypeParagraph)
	}
	paraContent := para["content"].([]any)
	text := paraContent[0].(map[string]any)
	if text["text"] != "This is a quote" {
		t.Errorf("text = %v, want %q", text["text"], "This is a quote")
	}
}

func TestRaw(t *testing.T) {
	customNode := Node{
		Type: "custom",
		Attrs: map[string]any{"key": "value"},
	}
	doc := NewDocument().Raw(customNode)
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	custom := content[0].(map[string]any)
	if custom["type"] != "custom" {
		t.Errorf("type = %v, want %q", custom["type"], "custom")
	}
	attrs := custom["attrs"].(map[string]any)
	if attrs["key"] != "value" {
		t.Errorf("attrs[key] = %v, want %q", attrs["key"], "value")
	}
}

func TestParagraphNodes(t *testing.T) {
	node1 := TextNode("Hello", Bold())
	node2 := TextNode("world")
	doc := NewDocument().ParagraphNodes(node1, node2)
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	para := content[0].(map[string]any)
	if para["type"] != TypeParagraph {
		t.Errorf("type = %v, want %q", para["type"], TypeParagraph)
	}
	paraContent := para["content"].([]any)
	if len(paraContent) != 2 {
		t.Errorf("paragraph content length = %d, want 2", len(paraContent))
	}
	firstText := paraContent[0].(map[string]any)
	if firstText["text"] != "Hello" {
		t.Errorf("first text = %v, want %q", firstText["text"], "Hello")
	}
	marks := firstText["marks"].([]any)
	if len(marks) != 1 {
		t.Errorf("marks length = %d, want 1", len(marks))
	}
}

func TestTextNode(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		marks []Mark
		check func(t *testing.T, node Node)
	}{
		{
			name: "plain text",
			text: "plain",
			marks: []Mark{},
			check: func(t *testing.T, node Node) {
				if node.Type != TypeText {
					t.Errorf("type = %v, want %q", node.Type, TypeText)
				}
				if node.Text != "plain" {
					t.Errorf("text = %v, want %q", node.Text, "plain")
				}
				if len(node.Marks) != 0 {
					t.Errorf("marks = %v, want empty", node.Marks)
				}
			},
		},
		{
			name: "text with bold",
			text: "bold text",
			marks: []Mark{Bold()},
			check: func(t *testing.T, node Node) {
				if len(node.Marks) != 1 {
					t.Errorf("marks length = %d, want 1", len(node.Marks))
					return
				}
				if node.Marks[0].Type != MarkStrong {
					t.Errorf("mark type = %v, want %q", node.Marks[0].Type, MarkStrong)
				}
			},
		},
		{
			name: "text with multiple marks",
			text: "bold italic code",
			marks: []Mark{Bold(), Italic(), CodeMark()},
			check: func(t *testing.T, node Node) {
				if len(node.Marks) != 3 {
					t.Errorf("marks length = %d, want 3", len(node.Marks))
					return
				}
				if node.Marks[0].Type != MarkStrong {
					t.Errorf("mark[0] = %q, want %q", node.Marks[0].Type, MarkStrong)
				}
				if node.Marks[1].Type != MarkEm {
					t.Errorf("mark[1] = %q, want %q", node.Marks[1].Type, MarkEm)
				}
				if node.Marks[2].Type != MarkCode {
					t.Errorf("mark[2] = %q, want %q", node.Marks[2].Type, MarkCode)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := TextNode(tt.text, tt.marks...)
			tt.check(t, node)
		})
	}
}

func TestBold(t *testing.T) {
	mark := Bold()
	if mark.Type != MarkStrong {
		t.Errorf("type = %v, want %q", mark.Type, MarkStrong)
	}
	if mark.Attrs != nil {
		t.Errorf("attrs = %v, want nil", mark.Attrs)
	}
}

func TestItalic(t *testing.T) {
	mark := Italic()
	if mark.Type != MarkEm {
		t.Errorf("type = %v, want %q", mark.Type, MarkEm)
	}
	if mark.Attrs != nil {
		t.Errorf("attrs = %v, want nil", mark.Attrs)
	}
}

func TestCodeMark(t *testing.T) {
	mark := CodeMark()
	if mark.Type != MarkCode {
		t.Errorf("type = %v, want %q", mark.Type, MarkCode)
	}
	if mark.Attrs != nil {
		t.Errorf("attrs = %v, want nil", mark.Attrs)
	}
}

func TestLink(t *testing.T) {
	tests := []struct {
		href string
	}{
		{"https://example.com"},
		{"http://localhost:3000"},
		{"/relative/path"},
	}
	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			mark := Link(tt.href)
			if mark.Type != MarkLink {
				t.Errorf("type = %v, want %q", mark.Type, MarkLink)
			}
			if mark.Attrs == nil {
				t.Fatalf("attrs = nil, want non-nil")
			}
			href, ok := mark.Attrs["href"]
			if !ok {
				t.Errorf("href not in attrs")
			}
			if href != tt.href {
				t.Errorf("href = %v, want %v", href, tt.href)
			}
		})
	}
}

func TestCellNodes(t *testing.T) {
	para := Node{
		Type: TypeParagraph,
		Content: []Node{
			{Type: TypeText, Text: "Cell content"},
		},
	}
	cell := CellNodes(para)
	if cell.Type != TypeTableCell {
		t.Errorf("type = %v, want %q", cell.Type, TypeTableCell)
	}
	if len(cell.Content) != 1 {
		t.Errorf("content length = %d, want 1", len(cell.Content))
	}
	if cell.Content[0].Type != TypeParagraph {
		t.Errorf("content[0].type = %v, want %q", cell.Content[0].Type, TypeParagraph)
	}
}

func TestCodeBlockWithoutLanguage(t *testing.T) {
	doc := NewDocument().CodeBlock("", "const x = 42;")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	cb := content[0].(map[string]any)
	if cb["type"] != TypeCodeBlock {
		t.Errorf("type = %v, want %q", cb["type"], TypeCodeBlock)
	}
	// When no language is specified, attrs should be nil or not present
	attrs, hasAttrs := cb["attrs"]
	if hasAttrs && attrs != nil {
		attrMap := attrs.(map[string]any)
		if _, hasLanguage := attrMap["language"]; hasLanguage {
			t.Errorf("language should not be in attrs")
		}
	}
}

func TestRule(t *testing.T) {
	doc := NewDocument().Rule()
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	rule := content[0].(map[string]any)
	if rule["type"] != TypeRule {
		t.Errorf("type = %v, want %q", rule["type"], TypeRule)
	}
}

func TestEmptyOrderedList(t *testing.T) {
	doc := NewDocument().OrderedList([]string{})
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	list := content[0].(map[string]any)
	items, ok := list["content"].([]any)
	if !ok {
		// Empty list may have nil content
		items = []any{}
	}
	if len(items) != 0 {
		t.Errorf("items = %d, want 0", len(items))
	}
}

func TestTextNodeSerialization(t *testing.T) {
	node := TextNode("test", Bold(), Link("https://example.com"))
	b, _ := json.Marshal(node)
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["type"] != TypeText {
		t.Errorf("type = %v, want %q", m["type"], TypeText)
	}
	if m["text"] != "test" {
		t.Errorf("text = %v, want %q", m["text"], "test")
	}
	marks := m["marks"].([]any)
	if len(marks) != 2 {
		t.Errorf("marks = %d, want 2", len(marks))
	}
}
