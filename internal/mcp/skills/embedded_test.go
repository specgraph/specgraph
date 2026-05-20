// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Expected names match the six embedded canonicals (relocated in commit 1).
var wantNames = []string{
	"specgraph-analytical-passes",
	"specgraph-authoring",
	"specgraph-conventions",
	"specgraph-drift",
	"specgraph-graph-query",
	"specgraph-troubleshooting",
}

func TestNewEmbedded_LoadsAllSixSkills(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	metas, err := src.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != len(wantNames) {
		t.Fatalf("got %d skills, want %d", len(metas), len(wantNames))
	}
	for i, m := range metas {
		if m.Name != wantNames[i] {
			t.Errorf("[%d] name = %q, want %q (List must be sorted)", i, m.Name, wantNames[i])
		}
		if m.Summary == "" {
			t.Errorf("[%d] %s has empty summary", i, m.Name)
		}
		wantURI := "specgraph://skills/" + m.Name
		if m.URI != wantURI {
			t.Errorf("[%d] URI = %q, want %q", i, m.URI, wantURI)
		}
	}
}

func TestEmbedded_Get_Known(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	sk, err := src.Get(context.Background(), "specgraph-authoring")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if sk.Name != "specgraph-authoring" {
		t.Errorf("Name = %q, want %q", sk.Name, "specgraph-authoring")
	}
	if !strings.Contains(string(sk.Body), "name: specgraph-authoring") {
		t.Errorf("body missing name line; got first 200 bytes: %q", string(sk.Body[:minInt(200, len(sk.Body))]))
	}
}

func TestEmbedded_Get_Unknown(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	_, err = src.Get(context.Background(), "no-such-skill")
	if err == nil {
		t.Fatal("expected error for unknown skill")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestSearch_TextMatchesAcrossFields(t *testing.T) {
	src, err := NewEmbedded()
	if err != nil {
		t.Fatalf("NewEmbedded: %v", err)
	}
	// "drift" appears in the drift skill's name and in other bodies.
	results, err := src.Search(context.Background(), "drift", SearchOptions{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one match for 'drift'")
	}
	var found bool
	for _, m := range results {
		if m.Name == "specgraph-drift" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected specgraph-drift in results; got %+v", results)
	}
}

func TestSearch_TextCaseInsensitive(t *testing.T) {
	src, _ := NewEmbedded()
	lower, _ := src.Search(context.Background(), "drift", SearchOptions{})
	upper, _ := src.Search(context.Background(), "DRIFT", SearchOptions{})
	if len(lower) != len(upper) {
		t.Errorf("case sensitivity: lower=%d, upper=%d", len(lower), len(upper))
	}
}

func TestSearch_RegexAnchors(t *testing.T) {
	src, _ := NewEmbedded()
	// \bdrift\b matches "drift" but not "drifted" — pins regex mode.
	results, err := src.Search(context.Background(), `\bdrift\b`,
		SearchOptions{Mode: SearchRegex})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Errorf("expected matches for \\bdrift\\b")
	}
}

func TestSearch_RegexInvalidReturnsErrInvalidQuery(t *testing.T) {
	src, _ := NewEmbedded()
	_, err := src.Search(context.Background(), `[unclosed`,
		SearchOptions{Mode: SearchRegex})
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("got %v, want ErrInvalidQuery", err)
	}
}

func TestSearch_FieldsRestriction(t *testing.T) {
	src, _ := NewEmbedded()
	// Restrict to FieldName: a query that matches body but not name
	// must return zero rows.
	results, err := src.Search(context.Background(), "funnel",
		SearchOptions{Fields: []SearchField{FieldName}})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	for _, m := range results {
		if !strings.Contains(strings.ToLower(m.Name), "funnel") {
			t.Errorf("FieldName restriction matched a non-name field: %s", m.Name)
		}
	}
}

func TestSearch_LimitClamps(t *testing.T) {
	src, _ := NewEmbedded()
	// A broad query that matches all six skills.
	results, err := src.Search(context.Background(), "spec",
		SearchOptions{Limit: 2})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) > 2 {
		t.Errorf("Limit=2 not honored; got %d rows", len(results))
	}
}

func TestSearch_StableOrder(t *testing.T) {
	src, _ := NewEmbedded()
	a, _ := src.Search(context.Background(), "spec", SearchOptions{})
	b, _ := src.Search(context.Background(), "spec", SearchOptions{})
	if len(a) != len(b) {
		t.Fatalf("len differs: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name {
			t.Errorf("[%d] order differs: %q vs %q", i, a[i].Name, b[i].Name)
		}
	}
}

func TestSearch_EmptyQueryReturnsErrInvalidQuery(t *testing.T) {
	src, _ := NewEmbedded()
	_, err := src.Search(context.Background(), "", SearchOptions{})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	if !errors.Is(err, ErrInvalidQuery) {
		t.Errorf("got %v, want ErrInvalidQuery", err)
	}
}
