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

func TestValidateInitMarkers(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"no markers", "no markers here\n", false},
		{"valid v=1 pair", "<!-- specgraph:init:start v=1 -->\nbody\n<!-- specgraph:init:end -->\n", false},
		{"valid v=2 pair", "<!-- specgraph:init:start v=2 sha256=abc123 -->\nbody\n<!-- specgraph:init:end -->\n", false},
		{"end before start", "<!-- specgraph:init:end -->\nbody\n<!-- specgraph:init:start v=1 -->\n", true},
		{"double start", "<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:start v=1 -->\n<!-- specgraph:init:end -->\n", true},
		{"start without end", "<!-- specgraph:init:start v=1 -->\nbody\n", true},
		{"end without start", "body\n<!-- specgraph:init:end -->\n", true},
		{"naked start no version", "<!-- specgraph:init:start -->\nbody\n<!-- specgraph:init:end -->\n", true},
		{"unknown version v=99", "<!-- specgraph:init:start v=99 -->\nbody\n<!-- specgraph:init:end -->\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInitMarkers("test.md", []byte(tc.input))
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateInitMarkers err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
