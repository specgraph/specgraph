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

// buildCommentResponse returns a JSON-encoded Confluence comment list response
// with the given comments in the "results" array.
func buildCommentResponse(comments []map[string]any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": comments})
	}
}

// fakeInlineComment returns a minimal inline comment map for the test server.
func fakeInlineComment(id, body string) map[string]any {
	return map[string]any{
		"id":        id,
		"body":      map[string]any{"storage": map[string]any{"value": body}},
		"version":   map[string]any{"authorId": "user-x"},
		"createdAt": "2026-04-10T10:00:00Z",
		"parentId":  "",
		"properties": map[string]any{
			"inline-marker": map[string]any{"textSelection": "some text"},
		},
	}
}

func TestFeedbackPoll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
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

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
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

func TestFeedbackPollSDDPageRoutesInlineToSpecify(t *testing.T) {
	// The footer comments endpoint returns one comment; inline returns one.
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/sdd-page/footer-comments", buildCommentResponse(nil))
	mux.HandleFunc("/wiki/api/v2/pages/sdd-page/inline-comments", buildCommentResponse([]map[string]any{
		fakeInlineComment("ic-sdd", "This section is unclear"),
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-speccsdd": {
				SpecSlug: "test-spec",
				DocKind:  storage.DocumentKindSDD,
				PageID:   "sdd-page",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	// Expect 1 inline comment routed to "specify"
	var inlines []publish.Feedback
	for _, f := range feedback {
		if f.Kind == publish.FeedbackInline {
			inlines = append(inlines, f)
		}
	}
	if len(inlines) != 1 {
		t.Fatalf("inline feedbacks = %d, want 1", len(inlines))
	}
	if inlines[0].Stage != "specify" {
		t.Errorf("Stage = %q, want 'specify'", inlines[0].Stage)
	}
}

func TestFeedbackPollADRPageRoutesInlineToDecision(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/adr-page/footer-comments", buildCommentResponse(nil))
	mux.HandleFunc("/wiki/api/v2/pages/adr-page/inline-comments", buildCommentResponse([]map[string]any{
		fakeInlineComment("ic-adr", "Why not approach B?"),
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-specadrmy-decision": {
				SpecSlug:     "test-spec",
				DocKind:      storage.DocumentKindADR,
				DecisionSlug: "my-decision",
				PageID:       "adr-page",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	var inlines []publish.Feedback
	for _, f := range feedback {
		if f.Kind == publish.FeedbackInline {
			inlines = append(inlines, f)
		}
	}
	if len(inlines) != 1 {
		t.Fatalf("inline feedbacks = %d, want 1", len(inlines))
	}
	if inlines[0].Stage != "decision" {
		t.Errorf("Stage = %q, want 'decision'", inlines[0].Stage)
	}
}

func TestFeedbackPollBothInlineAndFooter(t *testing.T) {
	footerComment := map[string]any{
		"id":        "fc-1",
		"body":      map[string]any{"storage": map[string]any{"value": "Nice work"}},
		"version":   map[string]any{"authorId": "user-a"},
		"createdAt": "2026-04-10T10:00:00Z",
		"parentId":  "",
		"properties": map[string]any{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/prd-page/footer-comments", buildCommentResponse([]map[string]any{footerComment}))
	mux.HandleFunc("/wiki/api/v2/pages/prd-page/inline-comments", buildCommentResponse([]map[string]any{
		fakeInlineComment("ic-1", "Can you clarify this requirement?"),
	}))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-specprd": {
				SpecSlug: "test-spec",
				DocKind:  storage.DocumentKindPRD,
				PageID:   "prd-page",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(feedback) != 2 {
		t.Fatalf("feedback count = %d, want 2", len(feedback))
	}
	// Expect one footer and one inline
	var footers, inlines int
	for _, f := range feedback {
		switch f.Kind {
		case publish.FeedbackFooter:
			footers++
		case publish.FeedbackInline:
			inlines++
		}
	}
	if footers != 1 {
		t.Errorf("footer count = %d, want 1", footers)
	}
	if inlines != 1 {
		t.Errorf("inline count = %d, want 1", inlines)
	}
}

func TestFeedbackPollQuestionFlag(t *testing.T) {
	// A comment body containing "?" should set IsQuestion = true.
	mux := http.NewServeMux()
	mux.HandleFunc("/wiki/api/v2/pages/prd-page2/footer-comments", buildCommentResponse([]map[string]any{
		{
			"id":        "fc-q",
			"body":      map[string]any{"storage": map[string]any{"value": "Is this correct?"}},
			"version":   map[string]any{"authorId": "user-q"},
			"createdAt": "2026-04-10T11:00:00Z",
			"parentId":  "",
			"properties": map[string]any{},
		},
	}))
	mux.HandleFunc("/wiki/api/v2/pages/prd-page2/inline-comments", buildCommentResponse(nil))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	fakeStore := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"test-specprd2": {
				SpecSlug: "test-spec",
				DocKind:  storage.DocumentKindPRD,
				PageID:   "prd-page2",
			},
		},
	}
	fs := NewFeedbackSource(client, fakeStore)
	feedback, err := fs.Poll(context.Background(), "test-spec")
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if len(feedback) != 1 {
		t.Fatalf("feedback count = %d, want 1", len(feedback))
	}
	if !feedback[0].IsQuestion {
		t.Errorf("IsQuestion = false, want true for body containing '?'")
	}
}
