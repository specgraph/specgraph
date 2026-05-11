// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// Manifest returns the list of ManagedFiles filtered by the given
// harnesses. Order is stable across calls — callers may rely on it for
// deterministic output (e.g. doctor's report).
//
// PR A returns an empty slice; PRs B/C/D/E populate it as they register
// managed files for each harness.
func Manifest(harnesses []Harness) []ManagedFile {
	all := allManagedFiles()
	enabled := harnessSet(harnesses)
	out := make([]ManagedFile, 0, len(all))
	for _, mf := range all {
		if enabled[mf.Harness] {
			out = append(out, mf)
		}
	}
	return out
}

// allManagedFiles is the framework's full set, unfiltered by harness.
// PR A returns nil; PRs B+ append entries.
func allManagedFiles() []ManagedFile {
	return nil
}

func harnessSet(harnesses []Harness) map[Harness]bool {
	out := make(map[Harness]bool, len(harnesses))
	for _, h := range harnesses {
		out[h] = true
	}
	return out
}
