// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package publish defines interfaces for publishing SpecGraph documents
// to external systems.
package publish

import (
	"context"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// Result describes the outcome of a publish or update operation.
type Result struct {
	Mappings []PageRef
}

// PageRef identifies a published page.
type PageRef struct {
	DocKind    render.DocumentKind
	DecisionID string
	PageID     string
	Version    int
	URL        string
}

// Status describes the current state of a spec's published documents.
type Status struct {
	SpecSlug    string
	PRD         *PageState
	SDD         *PageState
	ADRs        []PageState
	NewComments int
	LastSync    time.Time
}

// PageState describes a single published page.
type PageState struct {
	PageID      string
	State       string
	SpecVersion int32
	LastSync    time.Time
}

// Feedback represents an ingested external comment.
type Feedback struct {
	ExternalID string
	Author     string
	Body       string
	Timestamp  time.Time
	Kind       FeedbackKind
	Stage      string // routed authoring stage (inline only)
	IsQuestion bool
	ParentID   string
	SpecSlug   string
}

// FeedbackKind distinguishes inline vs footer comments.
type FeedbackKind string

const (
	// FeedbackInline is a comment anchored to a specific text range.
	FeedbackInline FeedbackKind = "inline"
	// FeedbackFooter is a general footer comment on the page.
	FeedbackFooter FeedbackKind = "footer"
)

// Publisher manages document lifecycle in an external system.
type Publisher interface {
	Name() string
	Publish(ctx context.Context, slug string, docs []render.Document) (Result, error)
	Update(ctx context.Context, slug string, docs []render.Document, changelog *specv1.ChangeLogEntry) (Result, error)
	Unpublish(ctx context.Context, slug string) error
	Status(ctx context.Context, slug string) (Status, error)
}

// FeedbackSource ingests external feedback back into SpecGraph.
type FeedbackSource interface {
	Poll(ctx context.Context, slug string) ([]Feedback, error)
}
