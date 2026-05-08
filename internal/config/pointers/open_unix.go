// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build !windows

package pointers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

// readFileNoFollow is os.ReadFile with O_NOFOLLOW. On a symlink it returns
// an error wrapping syscall.ELOOP; on file-not-found it returns an error
// satisfying errors.Is(_, fs.ErrNotExist).
func readFileNoFollow(path string) ([]byte, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only file
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// noFollowIsSymlink reports whether err arose from O_NOFOLLOW refusing
// to traverse a symlink. Used by callers to translate ELOOP into
// ErrSymlinkRejected.
func noFollowIsSymlink(err error) bool {
	return errors.Is(err, syscall.ELOOP)
}
