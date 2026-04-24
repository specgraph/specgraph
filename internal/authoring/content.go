// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"embed"
	"fmt"
)

//go:embed content/*.md
var contentFS embed.FS

// Content returns the bytes of the named embedded content file.
// name is relative to the content/ directory (e.g. "persona.md").
func Content(name string) ([]byte, error) {
	data, err := contentFS.ReadFile("content/" + name)
	if err != nil {
		return nil, fmt.Errorf("read embedded content: %w", err)
	}
	return data, nil
}
