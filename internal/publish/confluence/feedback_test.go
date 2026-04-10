// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestFeedbackPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"results": []map[string]any{
				{
					"id":        "comment-1",
					"body":      map[string]any{"storage": map[string]any{"value": "Looks good"}},
					"version":   map[string]any{"authorId": "alice"},
					"createdAt": "2026-04-10T10:00:00Z",
				},
			},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-specprd": {
				SpecSlug: "test-spec",
				DocKind:  storage.DocumentKindPRD,
				PageID:   "page-1",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(feedback) == 0 {
		t.Fatal("expected feedback")
	}
	if feedback[0].ExternalID != "comment-1" {
		t.Errorf("ExternalID = %q", feedback[0].ExternalID)
	}
	if feedback[0].Kind != publish.FeedbackFooter {
		t.Errorf("Kind = %q, want footer", feedback[0].Kind)
	}
}
