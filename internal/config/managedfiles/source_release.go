// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !dev

package managedfiles

import (
	"embed"
	"fmt"
)

// canonicalSources is populated via //go:embed directives added in PR C+
// when actual managed-file source content lands in the binary. PR A leaves
// the FS empty, which means readSourceImpl returns fs.ErrNotExist for any
// non-empty mf.Source.
//
// The empty embed is intentional: it lets the framework compile and tests
// run even before any //go:embed directive references real files. Adding
// the first directive happens in PR C alongside the OpenCode plugin TS.
var canonicalSources embed.FS

// readSourceImpl reads from the embedded sources tree.
func readSourceImpl(mf ManagedFile) ([]byte, error) {
	b, err := canonicalSources.ReadFile(mf.Source)
	if err != nil {
		return nil, fmt.Errorf("read embedded source %q: %w", mf.Source, err)
	}
	return b, nil
}
