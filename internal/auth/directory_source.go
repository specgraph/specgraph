// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirectoryPolicySource serves Cedar policy files from a filesystem
// directory. Required-by-default: a missing or unreadable directory is an
// error (operators who configure a policy dir mean it). An existing but
// empty directory (no *.cedar files) returns no documents and no error.
type DirectoryPolicySource struct {
	dir string
}

// NewDirectoryPolicySource returns a source rooted at dir.
func NewDirectoryPolicySource(dir string) DirectoryPolicySource {
	return DirectoryPolicySource{dir: dir}
}

// Name implements PolicySource.
func (s DirectoryPolicySource) Name() string { return "dir:" + s.dir }

// Load implements PolicySource. Reads every *.cedar file in the directory
// (non-recursive; nested dirs are ignored).
func (s DirectoryPolicySource) Load(_ context.Context) ([]PolicyDocument, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("read policy dir %s: %w", s.dir, err)
	}
	docs := make([]PolicyDocument, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cedar") {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, fmt.Errorf("read policy %s: %w", path, readErr)
		}
		docs = append(docs, PolicyDocument{Source: "dir:" + path, Text: string(b)})
	}
	return docs, nil
}
