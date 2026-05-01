// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// PublishState represents the state of a published document.
type PublishState string

const (
	// PublishStateDraft indicates the document has been created but not yet synced.
	PublishStateDraft PublishState = "draft"
	// PublishStateSynced indicates the document is up to date with Confluence.
	PublishStateSynced PublishState = "synced"
	// PublishStateError indicates the last publish attempt failed.
	PublishStateError PublishState = "error"
	// PublishStateUnpublished indicates the document has been removed from Confluence.
	PublishStateUnpublished PublishState = "unpublished"
)

// DocumentKind identifies the type of published document.
type DocumentKind string

const (
	// DocumentKindPRD identifies a Product Requirements Document.
	DocumentKindPRD DocumentKind = "prd"
	// DocumentKindSDD identifies a Software Design Document.
	DocumentKindSDD DocumentKind = "sdd"
	// DocumentKindADR identifies an Architectural Decision Record.
	DocumentKindADR DocumentKind = "adr"
)

// FeedbackKind identifies the type of Confluence comment.
type FeedbackKind string

const (
	// FeedbackKindInline is a comment anchored to a specific text range.
	FeedbackKindInline FeedbackKind = "inline"
	// FeedbackKindFooter is a general footer comment on the page.
	FeedbackKindFooter FeedbackKind = "footer"
)

// PageMapping tracks a published document's Confluence page.
type PageMapping struct {
	SpecSlug     string
	DocKind      DocumentKind
	DecisionSlug string // only for ADRs
	PageID       string
	PageVersion  int
	SpecVersion  int32
	State        PublishState
	ErrorMessage string
	LastSync     time.Time
	CreatedAt    time.Time
}

// FeedbackEntry represents an ingested Confluence comment.
type FeedbackEntry struct {
	ID         string
	ExternalID string // Confluence comment ID (dedup key)
	SpecSlug   string
	Author     string
	Body       string
	Timestamp  time.Time
	Kind       FeedbackKind
	Stage      string // routed authoring stage (inline only)
	IsQuestion bool
	ParentID   string // reply threading
	CreatedAt  time.Time
}

// PublishBackend manages page mappings and feedback entries.
type PublishBackend interface {
	UpsertPageMapping(ctx context.Context, m *PageMapping) (*PageMapping, error)
	GetPageMapping(ctx context.Context, specSlug string, kind DocumentKind, decisionSlug string) (*PageMapping, error)
	ListPageMappings(ctx context.Context, specSlug string) ([]*PageMapping, error)
	DeletePageMappings(ctx context.Context, specSlug string) (int, error)

	StoreFeedback(ctx context.Context, entry *FeedbackEntry) (*FeedbackEntry, error)
	ListFeedback(ctx context.Context, specSlug, sinceExternalID string) ([]*FeedbackEntry, error)
	CountNewFeedback(ctx context.Context, specSlug string) (int, error)
}
