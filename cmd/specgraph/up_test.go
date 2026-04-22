// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"strings"
	"testing"
)

// Hint extraction lets us test the service-mode branching without spinning
// up cobra flags or reading config from disk. Regression guard against
// accidentally inverting the isInstalled check or dropping the hint.

func TestServiceModeHint_InstalledReturnsEmpty(t *testing.T) {
	if got := serviceModeHint(true); got != "" {
		t.Errorf("installed should return empty hint, got %q", got)
	}
}

func TestServiceModeHint_NotInstalledDirectsUserToInstall(t *testing.T) {
	got := serviceModeHint(false)
	if got == "" {
		t.Fatal("not-installed should produce a hint, got empty")
	}
	if !strings.Contains(got, "specgraph install") {
		t.Errorf("hint should name the command to run, got %q", got)
	}
	if !strings.Contains(got, "container") {
		t.Errorf("hint should clarify container still starts, got %q", got)
	}
}
