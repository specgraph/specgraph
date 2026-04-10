// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"context"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// DocumentKind identifies the type of rendered document.
type DocumentKind int

const (
	DocumentPRD DocumentKind = iota
	DocumentSDD
	DocumentADR
)

// String returns the human-readable name of the document kind.
func (k DocumentKind) String() string {
	switch k {
	case DocumentPRD:
		return "PRD"
	case DocumentSDD:
		return "SDD"
	case DocumentADR:
		return "ADR"
	default:
		return "unknown"
	}
}

// Document is a format-agnostic rendered document.
type Document struct {
	Kind       DocumentKind
	Title      string
	Body       []byte            // Rendered content (ADF JSON, Markdown, etc.)
	SpecSlug   string            // The spec this document belongs to
	DecisionID string            // For ADR only: the decision slug
	Metadata   map[string]string // Optional key-value pairs for the publisher
}

// Renderer transforms spec data into structured documents.
// Implementations are format-specific (ADF, Markdown, etc.).
type Renderer interface {
	RenderPRD(ctx context.Context, spec *specv1.Spec) (Document, error)
	RenderSDD(ctx context.Context, spec *specv1.Spec) (Document, error)
	RenderADR(ctx context.Context, decision *specv1.Decision) (Document, error)
}
