// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"errors"
	"testing"
)

func TestSplitFrontmatter(t *testing.T) {
	t.Run("valid frontmatter", func(t *testing.T) {
		in := []byte("---\ndescription: hi\nalwaysApply: true\n---\n\nbody text\n")
		front, body, err := splitFrontmatter(in)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		wantFront := []byte("---\ndescription: hi\nalwaysApply: true\n---\n\n")
		wantBody := []byte("body text\n")
		if !bytes.Equal(front, wantFront) {
			t.Errorf("front mismatch:\n  got %q\n want %q", front, wantFront)
		}
		if !bytes.Equal(body, wantBody) {
			t.Errorf("body mismatch:\n  got %q\n want %q", body, wantBody)
		}
	})
	t.Run("missing frontmatter", func(t *testing.T) {
		_, _, err := splitFrontmatter([]byte("body text\n"))
		if !errors.Is(err, ErrFrontmatterMissing) {
			t.Fatalf("want ErrFrontmatterMissing, got %v", err)
		}
	})
	t.Run("unclosed frontmatter", func(t *testing.T) {
		_, _, err := splitFrontmatter([]byte("---\ndescription: hi\nbody\n"))
		if !errors.Is(err, ErrFrontmatterMissing) {
			t.Fatalf("want ErrFrontmatterMissing, got %v", err)
		}
	})
}

func TestSafeSlugPattern(t *testing.T) {
	good := []string{"a", "abc", "a.b", "a-b", "a_b", "a1.2_3-4"}
	bad := []string{"", "-a", ".a", "a b", "a/b"}
	for _, s := range good {
		if !safeSlugPattern.MatchString(s) {
			t.Errorf("expected %q to match", s)
		}
	}
	for _, s := range bad {
		if safeSlugPattern.MatchString(s) {
			t.Errorf("expected %q NOT to match", s)
		}
	}
}
