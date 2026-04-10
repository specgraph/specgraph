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
