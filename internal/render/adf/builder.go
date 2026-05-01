// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package adf

import (
	"encoding/json"
	"fmt"
)

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
	b, err := json.Marshal(d.root)
	if err != nil {
		return nil, fmt.Errorf("marshal ADF document: %w", err)
	}
	return b, nil
}

// append adds a node to the document's content.
func (d *Document) append(n Node) *Document { //nolint:gocritic // fluent builder API; inline literals make pointer syntax harmful
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
func (d *Document) Raw(n Node) *Document { //nolint:gocritic // public builder API; pointer would break caller ergonomics
	return d.append(n)
}
