// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"
)

//go:embed policies/*.cedar
var embeddedPolicyFS embed.FS

// EmbeddedPolicySource serves the built-in policies compiled into the
// binary. The built-in source MUST load successfully; a failure here means
// the binary was built wrong (the engine refuses to start — see
// NewCedarEngine).
type EmbeddedPolicySource struct{}

// NewEmbeddedPolicySource returns the built-in policy source.
func NewEmbeddedPolicySource() EmbeddedPolicySource { return EmbeddedPolicySource{} }

// Name implements PolicySource.
func (EmbeddedPolicySource) Name() string { return "embedded" }

// Load implements PolicySource. Reads every *.cedar file under policies/.
func (EmbeddedPolicySource) Load(_ context.Context) ([]PolicyDocument, error) {
	entries, err := fs.ReadDir(embeddedPolicyFS, "policies")
	if err != nil {
		return nil, fmt.Errorf("read embedded policies dir: %w", err)
	}
	docs := make([]PolicyDocument, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".cedar") {
			continue
		}
		b, readErr := embeddedPolicyFS.ReadFile("policies/" + e.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read embedded policy %s: %w", e.Name(), readErr)
		}
		docs = append(docs, PolicyDocument{Source: "embedded:" + e.Name(), Text: string(b)})
	}
	return docs, nil
}
