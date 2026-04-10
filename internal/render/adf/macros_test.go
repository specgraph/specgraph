// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

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

func TestStageColor(t *testing.T) {
	tests := []struct {
		stage string
		want  string
	}{
		{"spark", "purple"},
		{"shape", "blue"},
		{"specify", "blue"},
		{"decompose", "blue"},
		{"approved", "green"},
		{"in_progress", "yellow"},
		{"done", "green"},
		{"abandoned", "red"},
		{"unknown", "neutral"},
		{"", "neutral"},
	}
	for _, tt := range tests {
		if got := StageColor(tt.stage); got != tt.want {
			t.Errorf("StageColor(%q) = %q, want %q", tt.stage, got, tt.want)
		}
	}
}

func TestDecisionStatusColor(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"proposed", "yellow"},
		{"accepted", "green"},
		{"deprecated", "red"},
		{"superseded", "neutral"},
		{"unknown", "neutral"},
		{"", "neutral"},
	}
	for _, tt := range tests {
		if got := DecisionStatusColor(tt.status); got != tt.want {
			t.Errorf("DecisionStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestPanelNodes(t *testing.T) {
	heading := Node{
		Type: TypeHeading,
		Attrs: map[string]any{"level": 2},
		Content: []Node{
			{Type: TypeText, Text: "Panel Title"},
		},
	}
	para := Node{
		Type: TypeParagraph,
		Content: []Node{
			{Type: TypeText, Text: "Panel content"},
		},
	}
	doc := NewDocument().PanelNodes(PanelInfo, heading, para)
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	panel := content[0].(map[string]any)
	if panel["type"] != TypePanel {
		t.Errorf("type = %v, want %q", panel["type"], TypePanel)
	}
	attrs := panel["attrs"].(map[string]any)
	if attrs["panelType"] != PanelInfo {
		t.Errorf("panelType = %v, want %q", attrs["panelType"], PanelInfo)
	}
	panelContent := panel["content"].([]any)
	if len(panelContent) != 2 {
		t.Errorf("panel content length = %d, want 2", len(panelContent))
	}
	firstNode := panelContent[0].(map[string]any)
	if firstNode["type"] != TypeHeading {
		t.Errorf("first content type = %v, want %q", firstNode["type"], TypeHeading)
	}
}

func TestExpandNodes(t *testing.T) {
	content1 := Node{
		Type: TypeParagraph,
		Content: []Node{
			{Type: TypeText, Text: "First paragraph"},
		},
	}
	content2 := Node{
		Type: TypeParagraph,
		Content: []Node{
			{Type: TypeText, Text: "Second paragraph"},
		},
	}
	doc := NewDocument().ExpandNodes("Expand me", content1, content2)
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	docContent := m["content"].([]any)
	expand := docContent[0].(map[string]any)
	if expand["type"] != TypeExpand {
		t.Errorf("type = %v, want %q", expand["type"], TypeExpand)
	}
	attrs := expand["attrs"].(map[string]any)
	if attrs["title"] != "Expand me" {
		t.Errorf("title = %v, want %q", attrs["title"], "Expand me")
	}
	expandContent := expand["content"].([]any)
	if len(expandContent) != 2 {
		t.Errorf("expand content length = %d, want 2", len(expandContent))
	}
}

func TestPanelNotesType(t *testing.T) {
	doc := NewDocument().Panel(PanelNote, "This is a note")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	panel := content[0].(map[string]any)
	attrs := panel["attrs"].(map[string]any)
	if attrs["panelType"] != PanelNote {
		t.Errorf("panelType = %v, want %q", attrs["panelType"], PanelNote)
	}
}

func TestPanelErrorType(t *testing.T) {
	doc := NewDocument().Panel(PanelError, "This is an error")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	panel := content[0].(map[string]any)
	attrs := panel["attrs"].(map[string]any)
	if attrs["panelType"] != PanelError {
		t.Errorf("panelType = %v, want %q", attrs["panelType"], PanelError)
	}
}

func TestPanelSuccessType(t *testing.T) {
	doc := NewDocument().Panel(PanelSuccess, "This is a success message")
	b, _ := doc.JSON()
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	panel := content[0].(map[string]any)
	attrs := panel["attrs"].(map[string]any)
	if attrs["panelType"] != PanelSuccess {
		t.Errorf("panelType = %v, want %q", attrs["panelType"], PanelSuccess)
	}
}

func TestStatusMacroWithDifferentColors(t *testing.T) {
	colors := []string{"neutral", "purple", "blue", "red", "yellow", "green"}
	for _, color := range colors {
		t.Run(color, func(t *testing.T) {
			node := StatusMacro("Status", color)
			b, _ := json.Marshal(node)
			var m map[string]any
			json.Unmarshal(b, &m)
			attrs := m["attrs"].(map[string]any)
			if attrs["color"] != color {
				t.Errorf("color = %v, want %v", attrs["color"], color)
			}
		})
	}
}

func TestPagePropertiesEmpty(t *testing.T) {
	node := PageProperties([][2]string{})
	b, _ := json.Marshal(node)
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["type"] != TypeExtension {
		t.Errorf("type = %v, want %q", m["type"], TypeExtension)
	}
	content := m["content"].([]any)
	table := content[0].(map[string]any)
	if table["type"] != TypeTable {
		t.Errorf("table type = %v, want %q", table["type"], TypeTable)
	}
	rows := table["content"].([]any)
	// Should have at least header row
	if len(rows) < 1 {
		t.Errorf("rows = %d, want at least 1", len(rows))
	}
	headerRow := rows[0].(map[string]any)
	headerCells := headerRow["content"].([]any)
	if len(headerCells) != 2 {
		t.Errorf("header cells = %d, want 2", len(headerCells))
	}
}

func TestPagePropertiesMultipleRows(t *testing.T) {
	pairs := [][2]string{
		{"Owner", "John Doe"},
		{"Status", "Active"},
		{"Created", "2026-04-10"},
	}
	node := PageProperties(pairs)
	b, _ := json.Marshal(node)
	var m map[string]any
	json.Unmarshal(b, &m)
	content := m["content"].([]any)
	table := content[0].(map[string]any)
	rows := table["content"].([]any)
	// Header + 3 data rows
	if len(rows) != 4 {
		t.Errorf("rows = %d, want 4", len(rows))
	}
	secondRow := rows[1].(map[string]any)
	cells := secondRow["content"].([]any)
	if len(cells) != 2 {
		t.Errorf("cells = %d, want 2", len(cells))
	}
	firstCell := cells[0].(map[string]any)
	cellContent := firstCell["content"].([]any)
	cellPara := cellContent[0].(map[string]any)
	cellParaContent := cellPara["content"].([]any)
	cellText := cellParaContent[0].(map[string]any)
	if cellText["text"] != "Owner" {
		t.Errorf("text = %v, want %q", cellText["text"], "Owner")
	}
}

func TestDecisionStatusColorCaseSensitive(t *testing.T) {
	// Test that color matching is case-sensitive
	tests := []struct {
		status string
		want   string
	}{
		{"proposed", "yellow"},
		{"PROPOSED", "neutral"}, // Should not match uppercase
		{"Proposed", "neutral"},  // Should not match mixed case
	}
	for _, tt := range tests {
		if got := DecisionStatusColor(tt.status); got != tt.want {
			t.Errorf("DecisionStatusColor(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStageColorCaseSensitive(t *testing.T) {
	// Test that color matching is case-sensitive
	tests := []struct {
		stage string
		want  string
	}{
		{"spark", "purple"},
		{"SPARK", "neutral"}, // Should not match uppercase
		{"Spark", "neutral"},  // Should not match mixed case
	}
	for _, tt := range tests {
		if got := StageColor(tt.stage); got != tt.want {
			t.Errorf("StageColor(%q) = %q, want %q", tt.stage, got, tt.want)
		}
	}
}
