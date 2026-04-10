// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/storage"
)

// FeedbackSource implements publish.FeedbackSource for Confluence.
type FeedbackSource struct {
	client *Client
	store  PublishStore
}

// NewFeedbackSource creates a Confluence feedback source.
func NewFeedbackSource(client *Client, store PublishStore) *FeedbackSource {
	return &FeedbackSource{client: client, store: store}
}

// Poll retrieves new comments from all published pages for a spec.
func (f *FeedbackSource) Poll(ctx context.Context, slug string) ([]publish.Feedback, error) {
	mappings, err := f.store.ListPageMappings(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("list mappings: %w", err)
	}
	var all []publish.Feedback
	for _, m := range mappings {
		// Footer comments
		footerComments, err := f.client.GetFooterComments(ctx, m.PageID)
		if err != nil {
			return nil, fmt.Errorf("get footer comments for page %s: %w", m.PageID, err)
		}
		for i := range footerComments {
			all = append(all, toFeedback(&footerComments[i], slug, publish.FeedbackFooter, ""))
		}
		// Inline comments
		inlineComments, err := f.client.GetInlineComments(ctx, m.PageID)
		if err != nil {
			return nil, fmt.Errorf("get inline comments for page %s: %w", m.PageID, err)
		}
		for i := range inlineComments {
			stage := routeInlineComment(&inlineComments[i], m)
			all = append(all, toFeedback(&inlineComments[i], slug, publish.FeedbackInline, stage))
		}
	}
	return all, nil
}

func toFeedback(c *CommentInfo, slug string, kind publish.FeedbackKind, stage string) publish.Feedback {
	ts, _ := time.Parse(time.RFC3339, c.CreatedAt) //nolint:errcheck // invalid timestamps default to zero time
	return publish.Feedback{
		ExternalID: c.ID,
		Author:     c.Author,
		Body:       c.Body,
		Timestamp:  ts,
		Kind:       kind,
		Stage:      stage,
		IsQuestion: strings.Contains(c.Body, "?"),
		ParentID:   c.ParentID,
		SpecSlug:   slug,
	}
}

// routeInlineComment maps an inline comment to an authoring stage
// based on the page's document kind. Anchor-based routing within
// document sections is not yet implemented.
func routeInlineComment(_ *CommentInfo, m *storage.PageMapping) string {
	switch m.DocKind {
	case storage.DocumentKindPRD:
		return "shape"
	case storage.DocumentKindSDD:
		return "specify"
	case storage.DocumentKindADR:
		return "decision"
	default:
		return ""
	}
}
