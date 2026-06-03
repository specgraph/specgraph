// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"testing"
	"time"
)

// TestParseExpiresAt covers the three cases for the --expires-at flag value:
// empty (no expiry override), a valid RFC3339 timestamp, and a malformed one.
func TestParseExpiresAt(t *testing.T) {
	t.Run("empty yields nil with no error", func(t *testing.T) {
		ts, err := parseExpiresAt("")
		if err != nil {
			t.Fatalf("empty input must not error, got %v", err)
		}
		if ts != nil {
			t.Errorf("empty input must yield nil timestamp, got %v", ts)
		}
	})

	t.Run("valid RFC3339 parses", func(t *testing.T) {
		want := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
		ts, err := parseExpiresAt(want.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("valid RFC3339 must parse, got %v", err)
		}
		if ts == nil {
			t.Fatal("valid RFC3339 must yield a non-nil timestamp")
		}
		if got := ts.AsTime().UTC(); !got.Equal(want) {
			t.Errorf("parsed time = %v, want %v", got, want)
		}
	})

	t.Run("malformed yields an error", func(t *testing.T) {
		ts, err := parseExpiresAt("not-a-timestamp")
		if err == nil {
			t.Fatal("malformed input must return an error")
		}
		if ts != nil {
			t.Errorf("malformed input must yield nil timestamp, got %v", ts)
		}
	})
}
