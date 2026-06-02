// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "context"

// PolicyDocument is one unit of Cedar policy text plus a stable identifier
// for diagnostics and decision logs. Named PolicyDocument (not Policy) to
// avoid confusion with cedar.Policy inside engine.go.
type PolicyDocument struct {
	// Source identifies where this document came from, e.g.
	// "embedded:base.cedar" or "dir:/etc/specgraph/policies/extra.cedar".
	// Used as the filename argument to cedar.NewPolicySetFromBytes and as a
	// prefix on merged policy IDs so a decision log can name the origin.
	Source string
	// Text is the raw Cedar policy text (one or more policies).
	Text string
}

// PolicySource yields Cedar policy documents from some backing store.
// Implementations: EmbeddedPolicySource (built-ins), DirectoryPolicySource
// (operator files). DB- and URL-backed sources are deliberate follow-ups
// requiring no engine change — only a new PolicySource.
type PolicySource interface {
	// Load returns this source's policy documents. A source MAY return an
	// empty slice with no error (it simply contributes nothing).
	Load(ctx context.Context) ([]PolicyDocument, error)
	// Name returns a short identifier for diagnostics and decision logs.
	Name() string
}
