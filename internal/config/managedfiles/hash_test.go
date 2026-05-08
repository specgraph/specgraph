// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestHashExcludingSentinel_NoSentinel(t *testing.T) {
	body := "package foo\n\nfunc Bar() {}\n"
	got := HashExcludingSentinel(CommentSlash, []byte(body))
	want := hashOf(body)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestHashExcludingSentinel_WithSlashSentinel(t *testing.T) {
	body := "package foo\n\nfunc Bar() {}\n"
	withSentinel := "// specgraph:init v=2 sha256=abc rev=def\n" + body
	got := HashExcludingSentinel(CommentSlash, []byte(withSentinel))
	if got != hashOf(body) {
		t.Errorf("hash should equal body hash, got %s", got)
	}
}

func TestHashExcludingSentinel_StableAcrossRevChanges(t *testing.T) {
	body := "echo hi\n"
	a := "# specgraph:init v=2 sha256=abc rev=AAA\n" + body
	b := "# specgraph:init v=2 sha256=abc rev=BBB\n" + body
	if HashExcludingSentinel(CommentHash, []byte(a)) != HashExcludingSentinel(CommentHash, []byte(b)) {
		t.Error("hash differed across rev-only changes; should be stable")
	}
}

func TestHashExcludingSentinel_HTMLBlock(t *testing.T) {
	body := "# Title\n\nbody\n"
	withSentinel := "<!-- specgraph:init:start v=2 sha256=abc -->\n" + body + "<!-- specgraph:init:end -->\n"
	got := HashExcludingSentinel(CommentHTML, []byte(withSentinel))
	// For HTML/Markdown-block, BOTH the start AND end markers are dropped
	// before hashing, leaving the inner content.
	if got != hashOf(body) {
		t.Errorf("got %s, want %s", got, hashOf(body))
	}
}

func TestHashExcludingSentinel_NoneStrategy(t *testing.T) {
	body := `{"foo":"bar"}`
	// CommentNone (JSON files): no sentinel logic; hash the bytes as-is.
	got := HashExcludingSentinel(CommentNone, []byte(body))
	if got != hashOf(body) {
		t.Errorf("got %s, want %s", got, hashOf(body))
	}
}

// hashOf is a test helper: returns the hex sha256 of a string.
func hashOf(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
