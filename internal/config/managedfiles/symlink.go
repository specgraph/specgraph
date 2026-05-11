// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rejectSymlinkComponents walks the components of relPath under projectDir
// and returns ErrSymlinkRejected on the first symlink encountered. Components
// that don't exist (e.g. a leaf file we haven't written yet) are allowed —
// init writes new files, so non-existence is normal at write time.
//
// Mirrors internal/config/pointers/sync.go's helper. Reproducing it here
// is cheaper than introducing a shared util package.
func rejectSymlinkComponents(projectDir, relPath string) error {
	cur := projectDir
	for _, part := range strings.Split(filepath.Clean(relPath), string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		cur = filepath.Join(cur, part)
		info, err := os.Lstat(cur)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("lstat %s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: %s", ErrSymlinkRejected, cur)
		}
	}
	return nil
}
