// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

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
