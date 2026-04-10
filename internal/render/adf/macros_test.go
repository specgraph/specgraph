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
		{"done", "green"},
		{"abandoned", "red"},
		{"unknown", "neutral"},
	}
	for _, tt := range tests {
		if got := StageColor(tt.stage); got != tt.want {
			t.Errorf("StageColor(%q) = %q, want %q", tt.stage, got, tt.want)
		}
	}
}
