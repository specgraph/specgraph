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

func TestRenderSentinel_CommentHTML(t *testing.T) {
	got := RenderSentinel(CommentHTML, Sentinel{Version: 2, SHA256: "abc", Rev: "cef"})
	want := "<!-- specgraph:init v=2 sha256=abc rev=cef -->"
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

// TestParseSentinel_RejectsUnanchored guards the regex anchor: a body line
// containing "specgraph:init v=2 ..." mid-text (no comment prefix at line
// start) must NOT be parsed as a sentinel. Without the start-anchor and
// comment-prefix gate, a markdown rule body documenting the sentinel format
// would accidentally be treated as one.
func TestParseSentinel_RejectsUnanchored(t *testing.T) {
	cases := []string{
		"garbage specgraph:init v=2 sha256=abc",
		"prefix // specgraph:init v=2 sha256=abc",
		"see specgraph:init:start v=2 sha256=abc -->",
	}
	for _, line := range cases {
		got, err := ParseSentinel(CommentSlash, line)
		if err != nil {
			t.Errorf("line %q: unexpected error %v", line, err)
		}
		if got.Version != 0 {
			t.Errorf("line %q: expected non-sentinel, got %+v", line, got)
		}
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

func TestParseSentinel_AcceptsLegacyStartForm(t *testing.T) {
	// markdownblock.go writes "<!-- specgraph:init:start v=2 sha256=... -->"
	// inline. Confirm the parser still accepts that form so block-strategy
	// files written by older binaries remain readable.
	legacy := "<!-- specgraph:init:start v=2 sha256=abc -->"
	s, err := ParseSentinel(CommentHTML, legacy)
	if err != nil {
		t.Fatalf("parse legacy form: %v", err)
	}
	if s.Version != 2 || s.SHA256 != "abc" {
		t.Errorf("parsed sentinel = %+v, want {Version:2, SHA256:abc}", s)
	}
}
