//go:build integration

// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package postgres_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
)

func TestUpsertAndGetPageMapping(t *testing.T) {
	ctx := context.Background()
	b := newStore(t)

	m := &storage.PageMapping{
		SpecSlug:    "test-spec",
		DocKind:     storage.DocumentKindPRD,
		PageID:      "12345",
		PageVersion: 1,
		SpecVersion: 1,
		State:       storage.PublishStateSynced,
	}
	got, err := b.UpsertPageMapping(ctx, m)
	if err != nil {
		t.Fatalf("UpsertPageMapping: %v", err)
	}
	if got.PageID != "12345" {
		t.Errorf("PageID = %q, want %q", got.PageID, "12345")
	}

	fetched, err := b.GetPageMapping(ctx, "test-spec", storage.DocumentKindPRD, "")
	if err != nil {
		t.Fatalf("GetPageMapping: %v", err)
	}
	if fetched == nil {
		t.Fatal("GetPageMapping returned nil")
	}
	if fetched.PageID != "12345" {
		t.Errorf("fetched PageID = %q", fetched.PageID)
	}
}

func TestStoreFeedbackDedup(t *testing.T) {
	ctx := context.Background()
	b := newStore(t)

	entry := &storage.FeedbackEntry{
		ExternalID: "conf-comment-1",
		SpecSlug:   "test-spec",
		Author:     "alice",
		Body:       "Looks good",
		Kind:       storage.FeedbackKindFooter,
	}
	_, err := b.StoreFeedback(ctx, entry)
	if err != nil {
		t.Fatalf("StoreFeedback: %v", err)
	}
	_, err = b.StoreFeedback(ctx, &storage.FeedbackEntry{
		ExternalID: "conf-comment-1",
		SpecSlug:   "test-spec",
		Author:     "alice",
		Body:       "Looks good updated",
		Kind:       storage.FeedbackKindFooter,
	})
	if err != nil {
		t.Fatalf("StoreFeedback duplicate: %v", err)
	}
	entries, err := b.ListFeedback(ctx, "test-spec", "")
	if err != nil {
		t.Fatalf("ListFeedback: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("entries = %d, want 1", len(entries))
	}
}
