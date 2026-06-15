// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestIsLiteralLoopbackHost(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"127.0.0.1":          true,
		"::1":                true,
		"localhost":          false,
		"127.0.0.1.evil.com": false,
		"127.0.0.1.":         false,
		"0.0.0.0":            false,
		"2130706433":         false,
		"::ffff:127.0.0.1":   false,
		"":                   false,
		"127.0.0.2":          false,
	}
	for host, want := range cases {
		if got := auth.IsLiteralLoopbackHost(host); got != want {
			t.Errorf("IsLiteralLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}
