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
	PublishStateDraft       PublishState = "draft"
	PublishStateSynced      PublishState = "synced"
	PublishStateError       PublishState = "error"
	PublishStateUnpublished PublishState = "unpublished"
)

// DocumentKind identifies the type of published document.
type DocumentKind string

const (
	DocumentKindPRD DocumentKind = "prd"
	DocumentKindSDD DocumentKind = "sdd"
	DocumentKindADR DocumentKind = "adr"
)

// FeedbackKind identifies the type of Confluence comment.
type FeedbackKind string

const (
	FeedbackKindInline FeedbackKind = "inline"
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
	ListFeedback(ctx context.Context, specSlug string, sinceExternalID string) ([]*FeedbackEntry, error)
	CountNewFeedback(ctx context.Context, specSlug string) (int, error)
}
