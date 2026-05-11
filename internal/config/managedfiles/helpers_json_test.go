// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"bytes"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	t.Run("alphabetical keys", func(t *testing.T) {
		in := []byte(`{"z":1,"a":2}`)
		out, err := canonicalize(in)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte("{\n  \"a\": 2,\n  \"z\": 1\n}\n")
		if !bytes.Equal(out, want) {
			t.Errorf("got %q want %q", out, want)
		}
	})
	t.Run("trailing newline", func(t *testing.T) {
		out, err := canonicalize([]byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if out[len(out)-1] != '\n' {
			t.Error("missing trailing newline")
		}
	})
	t.Run("invalid JSON returns error", func(t *testing.T) {
		if _, err := canonicalize([]byte(`{not json}`)); err == nil {
			t.Error("expected error")
		}
	})
}
