// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

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
