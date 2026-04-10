// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package confluence

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// checkBasicAuth asserts that r carries a valid Basic auth header for the given email and token.
func checkBasicAuth(t *testing.T, r *http.Request, email, token string) {
	t.Helper()
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		t.Errorf("Authorization header is missing")
		return
	}
	if !strings.HasPrefix(authHeader, "Basic ") {
		t.Errorf("Authorization header = %q, want Basic scheme", authHeader)
		return
	}
	encoded := strings.TrimPrefix(authHeader, "Basic ")
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Errorf("Authorization header base64 decode failed: %v", err)
		return
	}
	want := email + ":" + token
	if string(decoded) != want {
		t.Errorf("Authorization credentials = %q, want %q", string(decoded), want)
	}
}

func TestCreatePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/wiki/api/v2/pages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		checkBasicAuth(t, r, "test@example.com", "test-token")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Test Page" {
			t.Errorf("title = %v", body["title"])
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "123",
			"version": map[string]any{"number": 1},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/123"},
		})
	}))
	defer srv.Close()

	c := NewClient(&Config{
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
		checkBasicAuth(t, r, "e", "t")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "123",
			"title":   "Test Page",
			"version": map[string]any{"number": 2},
		})
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	page, err := c.GetPage(context.Background(), "123")
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if page.Version != 2 {
		t.Errorf("Version = %d", page.Version)
	}
}

func TestUpdatePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		if r.URL.Path != "/wiki/api/v2/pages/456" {
			t.Errorf("path = %s, want /wiki/api/v2/pages/456", r.URL.Path)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["title"] != "Updated Title" {
			t.Errorf("title = %v, want Updated Title", body["title"])
		}
		// version should be incremented
		ver := body["version"].(map[string]any)
		if ver["number"] != float64(3) { // version 2 + 1
			t.Errorf("version.number = %v, want 3", ver["number"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":      "456",
			"title":   "Updated Title",
			"version": map[string]any{"number": 3},
			"_links":  map[string]any{"webui": "/wiki/spaces/ENG/pages/456"},
		})
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	page, err := c.UpdatePage(context.Background(), "456", "Updated Title", 2, []byte(`{"type":"doc"}`))
	if err != nil {
		t.Fatalf("UpdatePage: %v", err)
	}
	if page.ID != "456" {
		t.Errorf("ID = %q, want 456", page.ID)
	}
	if page.Version != 3 {
		t.Errorf("Version = %d, want 3", page.Version)
	}
	if page.Title != "Updated Title" {
		t.Errorf("Title = %q, want Updated Title", page.Title)
	}
}

func TestDeletePage(t *testing.T) {
	deleted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s, want DELETE", r.Method)
		}
		if r.URL.Path != "/wiki/api/v2/pages/789" {
			t.Errorf("path = %s, want /wiki/api/v2/pages/789", r.URL.Path)
		}
		deleted = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	err := c.DeletePage(context.Background(), "789")
	if err != nil {
		t.Fatalf("DeletePage: %v", err)
	}
	if !deleted {
		t.Error("DeletePage: no DELETE request was received")
	}
}

func TestGetFooterComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/101/footer-comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id": "c1",
					"body": map[string]any{
						"storage": map[string]any{"value": "Great spec!"},
					},
					"version":    map[string]any{"authorId": "user-1"},
					"createdAt":  "2026-01-01T00:00:00Z",
					"parentId":   "",
					"properties": map[string]any{},
				},
				{
					"id": "c2",
					"body": map[string]any{
						"storage": map[string]any{"value": "Needs more detail."},
					},
					"version":    map[string]any{"authorId": "user-2"},
					"createdAt":  "2026-01-02T00:00:00Z",
					"parentId":   "c1",
					"properties": map[string]any{},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	comments, err := c.GetFooterComments(context.Background(), "101")
	if err != nil {
		t.Fatalf("GetFooterComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("comments = %d, want 2", len(comments))
	}
	if comments[0].ID != "c1" {
		t.Errorf("comments[0].ID = %q, want c1", comments[0].ID)
	}
	if comments[0].Body != "Great spec!" {
		t.Errorf("comments[0].Body = %q, want 'Great spec!'", comments[0].Body)
	}
	if comments[0].Author != "user-1" {
		t.Errorf("comments[0].Author = %q, want user-1", comments[0].Author)
	}
	if comments[1].ParentID != "c1" {
		t.Errorf("comments[1].ParentID = %q, want c1", comments[1].ParentID)
	}
}

func TestGetInlineComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wiki/api/v2/pages/202/inline-comments" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"id": "ic1",
					"body": map[string]any{
						"storage": map[string]any{"value": "Inline note here."},
					},
					"version":   map[string]any{"authorId": "user-3"},
					"createdAt": "2026-02-01T00:00:00Z",
					"parentId":  "",
					"properties": map[string]any{
						"inline-marker": map[string]any{"textSelection": "some text"},
					},
				},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	comments, err := c.GetInlineComments(context.Background(), "202")
	if err != nil {
		t.Fatalf("GetInlineComments: %v", err)
	}
	if len(comments) != 1 {
		t.Fatalf("comments = %d, want 1", len(comments))
	}
	if comments[0].ID != "ic1" {
		t.Errorf("comments[0].ID = %q, want ic1", comments[0].ID)
	}
	if comments[0].Body != "Inline note here." {
		t.Errorf("comments[0].Body = %q", comments[0].Body)
	}
	if comments[0].InlineProperties == nil {
		t.Error("InlineProperties is nil, want non-nil")
	}
}

func TestClientErrorHandling400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid request"}`))
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	_, err := c.GetPage(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
}

func TestClientErrorHandling500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"internal server error"}`))
	}))
	defer srv.Close()

	c := NewClient(&Config{BaseURL: srv.URL, APIToken: "t", UserEmail: "e"})
	_, err := c.CreatePage(context.Background(), "Test", "parent-1", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
}
