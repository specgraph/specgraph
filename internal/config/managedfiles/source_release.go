// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import (
	"embed"
	"fmt"
)

// canonicalSources holds managed-file source content embedded at build time.
// Sources are real files under embedded/<harness>/ — the canonical location.
// Where harness-convention demands a copy under plugin/<harness>/ (e.g. so
// OpenCode's tooling discovers the .ts alongside its package.json and
// SMOKE_TEST.md), that copy is a symlink BACK into embedded/. go:embed
// rejects symlinks in its patterns, but a regular file at the embed path
// with symlinks pointing INTO it from elsewhere is fine — the canonical
// remains a single file.
//
//go:embed embedded/opencode/specgraph.ts embedded/cursor/test-rule.mdc
var canonicalSources embed.FS

// readSourceImpl reads from the embedded sources tree.
//nolint:gocritic // ManagedFile is the framework's standard parameter shape; pointer would change the internal API
func readSourceImpl(mf ManagedFile) ([]byte, error) {
	b, err := canonicalSources.ReadFile(mf.Source)
	if err != nil {
		return nil, fmt.Errorf("read embedded source %q: %w", mf.Source, err)
	}
	return b, nil
}
