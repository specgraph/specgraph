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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"id":      fmt.Sprintf("page-%d", callCount),
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/1"},
		})
	}))
	defer srv.Close()

	client := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e", SpaceKey: "ENG", ParentPageID: "root"})
	store := &fakePublishStore{}
	pub := NewPublisher(client, store, Config{ParentPageID: "root", Labels: []string{"specgraph"}})

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
	pub := NewPublisher(nil, nil, Config{})
	if pub.Name() != "confluence" {
		t.Errorf("Name() = %q", pub.Name())
	}
}
