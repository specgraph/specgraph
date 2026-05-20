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
