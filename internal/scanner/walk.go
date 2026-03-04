// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// walkGoFiles walks Go files under root, calling fn for each successfully parsed file.
// It skips vendor, node_modules, dotfiles, and optionally test files.
// Unparseable files are collected in the returned SkippedFile slice.
func walkGoFiles(root string, skipTests bool, fset *token.FileSet, fn func(path, relPath string, f *ast.File) error) ([]SkippedFile, error) {
	var skipped []SkippedFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			skipped = append(skipped, SkippedFile{Path: path, Reason: err.Error()})
			return nil //nolint:nilerr // intentional: skip unreadable entries gracefully
		}
		if d.IsDir() {
			if skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if skipTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relPath, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("scanner: relative path for %s: %w", path, relErr)
		}
		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			skipped = append(skipped, SkippedFile{Path: path, Reason: parseErr.Error()})
			return nil //nolint:nilerr // parse error recorded in skipped slice
		}
		return fn(path, relPath, f)
	})
	if err != nil {
		return skipped, fmt.Errorf("scanner: walk %s: %w", root, err)
	}
	return skipped, nil
}
