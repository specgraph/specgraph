// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build dev

package managedfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/term"
)

// devSourceRoot resolves the directory dev builds read sources from.
// SPECGRAPH_DEV_SOURCE_ROOT overrides the default of "./plugin".
func devSourceRoot() string {
	if v := os.Getenv("SPECGRAPH_DEV_SOURCE_ROOT"); v != "" {
		return v
	}
	return "./plugin"
}

// devBannerOnce ensures the dev banner stderr line is printed at most once
// per process. Loud-but-not-spammy.
var devBannerOnce sync.Once

func emitDevBanner() {
	devBannerOnce.Do(func() {
		// Same isatty gate as the drift-nudge — a dev binary invoked
		// non-interactively (CI, hooks) shouldn't smear the banner across
		// captured stderr.
		if !term.IsTerminal(int(os.Stderr.Fd())) {
			return
		}
		fmt.Fprintf(os.Stderr,
			"specgraph: DEV BUILD — embedded files read from disk at %s\n",
			devSourceRoot())
	})
}

// readSourceImpl reads from disk at <devSourceRoot>/<mf.Source>.
func readSourceImpl(mf ManagedFile) ([]byte, error) {
	emitDevBanner()
	path := filepath.Join(devSourceRoot(), mf.Source)
	b, err := os.ReadFile(path) //nolint:gosec // path comes from manifest under dev source root
	if err != nil {
		return nil, fmt.Errorf("read dev source %q: %w", path, err)
	}
	return b, nil
}
