// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"testing"
)

func TestRenderSentinel_Slash(t *testing.T) {
	got := RenderSentinel(CommentSlash, Sentinel{Version: 2, SHA256: "abc123", Rev: "cef1ec3a"})
	want := "// specgraph:init v=2 sha256=abc123 rev=cef1ec3a"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSentinel_Hash(t *testing.T) {
	got := RenderSentinel(CommentHash, Sentinel{Version: 2, SHA256: "abc"})
	want := "# specgraph:init v=2 sha256=abc"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSentinel_HTMLBlockStart(t *testing.T) {
	got := RenderSentinel(CommentHTML, Sentinel{Version: 2, SHA256: "abc", Rev: "cef"})
	want := "<!-- specgraph:init:start v=2 sha256=abc rev=cef -->"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderSentinel_None_Empty(t *testing.T) {
	got := RenderSentinel(CommentNone, Sentinel{Version: 2, SHA256: "abc"})
	if got != "" {
		t.Errorf("CommentNone should produce empty string, got %q", got)
	}
}

func TestParseSentinel_Slash_v2(t *testing.T) {
	got, err := ParseSentinel(CommentSlash, "// specgraph:init v=2 sha256=abc123 rev=cef")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 2 || got.SHA256 != "abc123" || got.Rev != "cef" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSentinel_v1(t *testing.T) {
	// v=1 is recognized for upgrade path (no sha256 field).
	got, err := ParseSentinel(CommentHTML, "<!-- specgraph:init:start v=1 -->")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 1 || got.SHA256 != "" {
		t.Errorf("got %+v", got)
	}
}

func TestParseSentinel_v3_Rejected(t *testing.T) {
	_, err := ParseSentinel(CommentSlash, "// specgraph:init v=3 sha256=abc")
	if !errors.Is(err, ErrCorruptedSentinel) {
		t.Errorf("want ErrCorruptedSentinel, got %v", err)
	}
}

func TestParseSentinel_NotASentinel(t *testing.T) {
	got, err := ParseSentinel(CommentSlash, "// just a regular comment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Version != 0 {
		t.Errorf("expected zero Sentinel for non-sentinel line, got %+v", got)
	}
}

func TestRenderParseRoundTrip(t *testing.T) {
	for _, syntax := range []CommentSyntax{CommentSlash, CommentHash, CommentHTML} {
		original := Sentinel{Version: 2, SHA256: "deadbeef", Rev: "abc1234"}
		line := RenderSentinel(syntax, original)
		parsed, err := ParseSentinel(syntax, line)
		if err != nil {
			t.Fatalf("syntax %v: parse error: %v", syntax, err)
		}
		if parsed != original {
			t.Errorf("syntax %v: round-trip mismatch: got %+v, want %+v", syntax, parsed, original)
		}
	}
}
