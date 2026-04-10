// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreatePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/wiki/api/v2/pages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		if body["title"] != "Test Page" {
			t.Errorf("title = %v", body["title"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"id":      "123",
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/123"},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{
		CloudID:      "test-cloud",
		BaseURL:      srv.URL,
		APIToken:     "test-token",
		UserEmail:    "test@example.com",
		SpaceKey:     "ENG",
		ParentPageID: "parent-1",
	})
	page, err := c.CreatePage(context.Background(), "Test Page", "parent-1", []byte(`{"type":"doc","version":1}`))
	if err != nil {
		t.Fatalf("CreatePage: %v", err)
	}
	if page.ID != "123" {
		t.Errorf("ID = %q", page.ID)
	}
}

func TestGetPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/123" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
			"id":      "123",
			"title":   "Test Page",
			"version": map[string]any{"number": 2},
		})
	}))
	defer srv.Close()

	c := NewClient(Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	page, err := c.GetPage(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Version != 2 {
		t.Errorf("Version = %d", page.Version)
	}
}
