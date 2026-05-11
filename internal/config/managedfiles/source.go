// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

// readSource returns the canonical bytes for a ManagedFile.
//
// Behaviour by build tag:
//   - Default build: reads from the package-level canonicalSources embed.FS
//     populated via //go:embed directives in source_release.go. PR A leaves
//     it empty; PRs C/D/E add directives.
//   - `dev` build tag: reads from disk at SPECGRAPH_DEV_SOURCE_ROOT (default
//     "./plugin") so a developer can edit a source file and re-run init
//     without rebuilding the binary.
//
// Returns (nil, nil) when mf.Source is empty (JSON-key-merge files build
// their canonical programmatically, not from an embedded asset).
//
// Implementation lives in source_release.go (no build tag — default build)
// or source_dev.go (`dev` build tag).
func readSource(mf ManagedFile) ([]byte, error) {
	if mf.Source == "" {
		return nil, nil
	}
	return readSourceImpl(mf)
}
