// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package browser

import "testing"

func TestOpenCommand(t *testing.T) {
	t.Parallel()
	name, args := openCommand("darwin", "https://example.com")
	if name != "open" || len(args) != 1 || args[0] != "https://example.com" {
		t.Fatalf("darwin: got %s %v", name, args)
	}
	name, _ = openCommand("linux", "https://example.com")
	if name != "xdg-open" {
		t.Fatalf("linux: got %s", name)
	}
	name, args = openCommand("windows", "https://example.com")
	if name != "rundll32" || args[0] != "url.dll,FileProtocolHandler" {
		t.Fatalf("windows: got %s %v", name, args)
	}
}
