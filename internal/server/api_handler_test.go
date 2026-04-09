// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
)

// fakeScoper returns a stubBackend that lists no projects (empty result set).
type fakeScoper struct{}

func (f *fakeScoper) Scoped(_ context.Context, _ string) (storage.ScopedBackend, error) {
	return &fakeProjectBackend{}, nil
}

func (f *fakeScoper) Subscribe(_ storage.ChangeSubscriber) {}

type fakeProjectBackend struct {
	stubBackend
}

func (f *fakeProjectBackend) ListProjects(_ context.Context) ([]*storage.Project, error) {
	return nil, nil
}

func TestAPIHandler_AuthRequired_NoToken_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(store))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAPIHandler_AuthRequired_ValidToken_Returns200(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Admin", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(store))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAPIHandler_NoKeys_Returns401(t *testing.T) {
	store, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(store))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no auth bypass)", rec.Code)
	}
}
