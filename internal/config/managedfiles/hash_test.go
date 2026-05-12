// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

func TestHashExcludingSentinelAfterFrontmatter(t *testing.T) {
	body := "---\ndescription: x\nalwaysApply: false\n---\n\n<!-- specgraph:init v=2 sha256=abc -->\n# Heading\n\nbody text\n"

	got, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected hash: same content with the sentinel line removed.
	want := hashBytes([]byte("---\ndescription: x\nalwaysApply: false\n---\n\n# Heading\n\nbody text\n"))
	if got != want {
		t.Errorf("hash mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_StableAcrossRevChanges(t *testing.T) {
	a := "---\ndescription: x\n---\n\n<!-- specgraph:init v=2 sha256=abc -->\nbody\n"
	b := "---\ndescription: x\n---\n\n<!-- specgraph:init v=2 sha256=abc rev=deadbeef -->\nbody\n"
	ha, errA := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(a))
	hb, errB := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(b))
	if errA != nil || errB != nil {
		t.Fatalf("hash errors: %v / %v", errA, errB)
	}
	if ha != hb {
		t.Errorf("hash differs across sentinel-only changes: %s vs %s", ha, hb)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_NoFrontmatter(t *testing.T) {
	body := "no frontmatter here\nbody\n"
	_, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if !errors.Is(err, ErrFrontmatterMissing) {
		t.Errorf("error = %v, want ErrFrontmatterMissing", err)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_NoSentinelOnBodyFirstLine(t *testing.T) {
	// First body line is a heading, not a sentinel. The function should
	// hash the file content unchanged (no line dropped) — the classifier
	// is responsible for treating the absence of a sentinel as Drifted.
	body := "---\ndescription: x\n---\n\n# Heading\nbody\n"
	got, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := hashBytes([]byte(body))
	if got != want {
		t.Errorf("hash mismatch:\n got=%s\nwant=%s", got, want)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_CorruptSentinel(t *testing.T) {
	// Sentinel with unsupported version triggers ParseSentinel's
	// ErrCorruptedSentinel — the hash function surfaces that error so
	// callers can classify the file as Drifted with the parse error in
	// Detail.
	body := "---\ndescription: x\n---\n\n<!-- specgraph:init v=99 sha256=abc -->\ncontent\n"
	_, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if !errors.Is(err, ErrCorruptedSentinel) {
		t.Errorf("error = %v, want ErrCorruptedSentinel", err)
	}
}

func TestHashExcludingSentinelAfterFrontmatter_EmptyBody(t *testing.T) {
	// Input with frontmatter and only a trailing blank line — splitFrontmatter
	// consumes the blank into `front`, leaving body empty. The function
	// should hash `front` unchanged.
	body := "---\ndescription: x\n---\n\n"
	got, err := HashExcludingSentinelAfterFrontmatter(CommentHTML, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := hashBytes([]byte(body))
	if got != want {
		t.Errorf("hash mismatch:\n got=%s\nwant=%s", got, want)
	}
}
