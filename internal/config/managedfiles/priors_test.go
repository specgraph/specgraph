// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import "testing"

func TestPriorsRegistry_CursorRulesMigrated(t *testing.T) {
	// PR D's vestigial cursor-rule priors must remain accessible
	// through priorsFor after the unification.
	hashes := priorsFor(".cursor/rules/specgraph.mdc")
	if len(hashes) == 0 {
		t.Error("expected at least one prior hash for .cursor/rules/specgraph.mdc")
	}
}

func TestPriorsRegistry_UnknownPathEmpty(t *testing.T) {
	if priors := priorsFor("nonexistent/path"); len(priors) != 0 {
		t.Errorf("expected empty slice for unknown path, got %v", priors)
	}
}
