// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import (
	"embed"
	"fmt"
)

// canonicalSources holds managed-file source content embedded at build time.
// Source files are copied into embedded/ (go:embed cannot follow symlinks) and
// kept in sync via task managedfiles:sync (wired into task build). PR C adds
// the OpenCode plugin TS; PR D adds Cursor rules; PR E adds Claude rules.
//
//go:embed embedded/opencode/specgraph.ts
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
