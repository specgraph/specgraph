// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/storage"
)

type fakePublishStore struct {
	upsertCount int
	mappings    map[string]*storage.PageMapping
}

func (f *fakePublishStore) UpsertPageMapping(_ context.Context, m *storage.PageMapping) (*storage.PageMapping, error) {
	f.upsertCount++
	if f.mappings == nil {
		f.mappings = make(map[string]*storage.PageMapping)
	}
	key := m.SpecSlug + string(m.DocKind) + m.DecisionSlug
	f.mappings[key] = m
	return m, nil
}

func (f *fakePublishStore) GetPageMapping(_ context.Context, specSlug string, kind storage.DocumentKind, decisionSlug string) (*storage.PageMapping, error) {
	if f.mappings == nil {
		return nil, nil
	}
	return f.mappings[specSlug+string(kind)+decisionSlug], nil
}

func (f *fakePublishStore) ListPageMappings(_ context.Context, specSlug string) ([]*storage.PageMapping, error) {
	var result []*storage.PageMapping
	for _, m := range f.mappings {
		if specSlug == "" || m.SpecSlug == specSlug {
			result = append(result, m)
		}
	}
	return result, nil
}

func (f *fakePublishStore) DeletePageMappings(_ context.Context, _ string) (int, error) {
	count := len(f.mappings)
	f.mappings = nil
	return count, nil
}

func TestPublisherPublish(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/1"},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root", Labels: []string{"specgraph"}})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Publish(context.Background(), "test", docs)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if len(result.Mappings) != 1 {
		t.Errorf("mappings = %d, want 1", len(result.Mappings))
	}
	if store.upsertCount != 1 {
		t.Errorf("upsert count = %d, want 1", store.upsertCount)
	}
}

func TestPublisherName(t *testing.T) {
	pub := NewPublisher(nil, nil, &Config{})
	if pub.Name() != "confluence" {
		t.Errorf("Name() = %q", pub.Name())
	}
}

func TestPublisherUpdateExistingPage(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "page-42",
			"version": map[string]any{"number": 2},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/42"},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG"})
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindPRD,
				PageID:      "page-42",
				PageVersion: 1,
				State:       storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Update(context.Background(), "test", docs, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if len(result.Mappings) != 1 {
		t.Errorf("mappings = %d, want 1", len(result.Mappings))
	}
	if result.Mappings[0].PageID != "page-42" {
		t.Errorf("PageID = %q, want page-42", result.Mappings[0].PageID)
	}
	if result.Mappings[0].Version != 2 {
		t.Errorf("Version = %d, want 2", result.Mappings[0].Version)
	}
	// 1 PUT (UpdatePage) + 1 UpsertPageMapping
	if callCount != 1 {
		t.Errorf("HTTP calls = %d, want 1 (UpdatePage only)", callCount)
	}
}

func TestPublisherUpdateFallsBackToPublish(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/1"},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	// Empty store — no existing mapping
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Update(context.Background(), "test", docs, nil)
	if err != nil {
		t.Fatalf("Update (fallback): %v", err)
	}
	if len(result.Mappings) != 1 {
		t.Errorf("mappings = %d, want 1", len(result.Mappings))
	}
	// Should have called CreatePage (fallback to Publish)
	if callCount < 1 {
		t.Errorf("HTTP calls = %d, want >= 1 (CreatePage fallback)", callCount)
	}
}

func TestPublisherUnpublish(t *testing.T) {
	deletedIDs := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			// extract page id from path
			deletedIDs = append(deletedIDs, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG"})
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug: "test",
				DocKind:  storage.DocumentKindPRD,
				PageID:   "page-prd",
				State:    storage.PublishStateSynced,
			},
			"testsdd": {
				SpecSlug: "test",
				DocKind:  storage.DocumentKindSDD,
				PageID:   "page-sdd",
				State:    storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(client, store, &Config{})

	err := pub.Unpublish(context.Background(), "test")
	if err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	// Both pages should have been deleted
	if len(deletedIDs) != 2 {
		t.Errorf("deleted pages = %d, want 2", len(deletedIDs))
	}
	// Mappings should be cleared
	if store.mappings != nil {
		t.Errorf("mappings not cleared after Unpublish")
	}
}

func TestPublisherStatusWithMappings(t *testing.T) {
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindPRD,
				PageID:      "page-prd",
				PageVersion: 3,
				State:       storage.PublishStateSynced,
			},
			"testsdd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindSDD,
				PageID:      "page-sdd",
				PageVersion: 2,
				State:       storage.PublishStateSynced,
			},
			"testadrmy-decision": {
				SpecSlug:     "test",
				DocKind:      storage.DocumentKindADR,
				DecisionSlug: "my-decision",
				PageID:       "page-adr",
				PageVersion:  1,
				State:        storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(nil, store, &Config{})

	status, err := pub.Status(context.Background(), "test")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.SpecSlug != "test" {
		t.Errorf("SpecSlug = %q, want test", status.SpecSlug)
	}
	if status.PRD == nil {
		t.Fatal("PRD status is nil, want non-nil")
	}
	if status.PRD.PageID != "page-prd" {
		t.Errorf("PRD.PageID = %q, want page-prd", status.PRD.PageID)
	}
	if status.SDD == nil {
		t.Fatal("SDD status is nil, want non-nil")
	}
	if status.SDD.PageID != "page-sdd" {
		t.Errorf("SDD.PageID = %q, want page-sdd", status.SDD.PageID)
	}
	if len(status.ADRs) != 1 {
		t.Errorf("ADRs = %d, want 1", len(status.ADRs))
	}
	if status.ADRs[0].PageID != "page-adr" {
		t.Errorf("ADRs[0].PageID = %q, want page-adr", status.ADRs[0].PageID)
	}
}

func TestPublisherStatusEmpty(t *testing.T) {
	store := &fakePublishStore{}
	pub := NewPublisher(nil, store, &Config{})

	status, err := pub.Status(context.Background(), "no-such-spec")
	if err != nil {
		t.Fatalf("Status (empty): %v", err)
	}
	if status.PRD != nil {
		t.Errorf("PRD = %v, want nil", status.PRD)
	}
	if status.SDD != nil {
		t.Errorf("SDD = %v, want nil", status.SDD)
	}
	if len(status.ADRs) != 0 {
		t.Errorf("ADRs = %d, want 0", len(status.ADRs))
	}
}

func TestPublisherPublishMultipleDocs(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method != http.MethodPost {
			t.Errorf("call %d: method = %s, want POST", callCount, r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": fmt.Sprintf("/wiki/spaces/ENG/pages/%d", callCount)},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	// Publish PRD first, then SDD (SDD should be parented under PRD page).
	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
		{Kind: render.DocumentSDD, Title: "SDD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Publish(context.Background(), "test", docs)
	if err != nil {
		t.Fatalf("Publish(multiple docs): %v", err)
	}
	if len(result.Mappings) != 2 {
		t.Errorf("mappings = %d, want 2", len(result.Mappings))
	}
	if store.upsertCount != 2 {
		t.Errorf("upsert count = %d, want 2", store.upsertCount)
	}
	// First mapping should be PRD kind
	if result.Mappings[0].DocKind != render.DocumentPRD {
		t.Errorf("Mappings[0].DocKind = %v, want PRD", result.Mappings[0].DocKind)
	}
	// Second mapping should be SDD kind
	if result.Mappings[1].DocKind != render.DocumentSDD {
		t.Errorf("Mappings[1].DocKind = %v, want SDD", result.Mappings[1].DocKind)
	}
}

func TestPublisherUpdateMixedExistence(t *testing.T) {
	callCount := 0
	methods := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		methods = append(methods, r.Method)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 2},
			"_links":  map[string]any{"webui": fmt.Sprintf("/wiki/spaces/ENG/pages/%d", callCount)},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	// PRD mapping exists; SDD does not.
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindPRD,
				PageID:      "page-existing",
				PageVersion: 1,
				State:       storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
		{Kind: render.DocumentSDD, Title: "SDD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	result, err := pub.Update(context.Background(), "test", docs, nil)
	if err != nil {
		t.Fatalf("Update(mixed existence): %v", err)
	}
	if len(result.Mappings) != 2 {
		t.Errorf("mappings = %d, want 2", len(result.Mappings))
	}
	// PRD existed → PUT; SDD is new → POST.
	putCount := 0
	postCount := 0
	for _, m := range methods {
		switch m {
		case http.MethodPut:
			putCount++
		case http.MethodPost:
			postCount++
		}
	}
	if putCount < 1 {
		t.Errorf("expected at least 1 PUT for existing PRD page, got %d PUT calls (methods=%v)", putCount, methods)
	}
	if postCount < 1 {
		t.Errorf("expected at least 1 POST for new SDD page, got %d POST calls (methods=%v)", postCount, methods)
	}
}

func TestPublisherStatusWithADRs(t *testing.T) {
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindPRD,
				PageID:      "page-prd",
				PageVersion: 1,
				State:       storage.PublishStateSynced,
			},
			"testadr-adr-001": {
				SpecSlug:     "test",
				DocKind:      storage.DocumentKindADR,
				DecisionSlug: "adr-001",
				PageID:       "page-adr-1",
				PageVersion:  1,
				State:        storage.PublishStateSynced,
			},
			"testadr-adr-002": {
				SpecSlug:     "test",
				DocKind:      storage.DocumentKindADR,
				DecisionSlug: "adr-002",
				PageID:       "page-adr-2",
				PageVersion:  1,
				State:        storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(nil, store, &Config{})

	status, err := pub.Status(context.Background(), "test")
	if err != nil {
		t.Fatalf("Status (with ADRs): %v", err)
	}
	if len(status.ADRs) != 2 {
		t.Errorf("ADRs = %d, want 2", len(status.ADRs))
	}
}

func TestPublisherPublishError(t *testing.T) {
	// Server returns 500 — CreatePage should fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal server error"}`))
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	_, err := pub.Publish(context.Background(), "test", docs)
	if err == nil {
		t.Fatal("Publish: expected error on server 500, got nil")
	}
}

func TestPublisherUpdateError(t *testing.T) {
	// Server always returns 500 — UpdatePage should fail when the page exists.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal server error"}`))
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG"})
	// Pre-seed a mapping so the publisher takes the update-existing-page path.
	store := &fakePublishStore{
		mappings: map[string]*storage.PageMapping{
			"testprd": {
				SpecSlug:    "test",
				DocKind:     storage.DocumentKindPRD,
				PageID:      "page-existing",
				PageVersion: 1,
				State:       storage.PublishStateSynced,
			},
		},
	}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	_, err := pub.Update(context.Background(), "test", docs, nil)
	if err == nil {
		t.Fatal("Update: expected error on server 500 when page exists, got nil")
	}
}

// failingPublishStore returns an error from UpsertPageMapping while other methods succeed.
type failingPublishStore struct {
	fakePublishStore
	upsertErr error
}

func (f *failingPublishStore) UpsertPageMapping(_ context.Context, _ *storage.PageMapping) (*storage.PageMapping, error) {
	return nil, f.upsertErr
}

func TestPublisherPublishStoreError(t *testing.T) {
	// Confluence API call succeeds but the storage upsert fails.
	// This tests that orphaned pages are caught — Publish must return an error.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": fmt.Sprintf("/wiki/spaces/ENG/pages/%d", callCount)},
		})
	}))
	defer srv.Close()

	client := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &failingPublishStore{
		upsertErr: fmt.Errorf("storage unavailable"),
	}
	pub := NewPublisher(client, store, &Config{ParentPageID: "root"})

	docs := []render.Document{
		{Kind: render.DocumentPRD, Title: "PRD: test", Body: []byte(`{}`), SpecSlug: "test"},
	}
	_, err := pub.Publish(context.Background(), "test", docs)
	if err == nil {
		t.Fatal("Publish: expected error when UpsertPageMapping fails, got nil (page would be orphaned)")
	}
	// The Confluence API should have been called (page created) even though storage failed.
	if callCount == 0 {
		t.Error("expected Confluence CreatePage to be called before the storage error")
	}
}
