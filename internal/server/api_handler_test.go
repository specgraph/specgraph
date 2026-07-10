// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
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

// apiTestResolver accepts exactly one valid token and rejects everything else.
type apiTestResolver struct {
	validToken string
	identity   *auth.Identity
}

func (r *apiTestResolver) Resolve(_ context.Context, token string) (*auth.Identity, error) {
	if token == r.validToken {
		return r.identity, nil
	}
	return nil, auth.ErrUnauthenticated
}

func (r *apiTestResolver) ResolveLogin(_ context.Context, _ *auth.OIDCClaims) (*auth.Identity, error) {
	return r.identity, nil
}

func (r *apiTestResolver) HasAuth(_ context.Context) (bool, error) { return true, nil }

// noAuthResolver always returns ErrUnauthenticated.
type noAuthResolver struct{}

func (r *noAuthResolver) Resolve(_ context.Context, _ string) (*auth.Identity, error) {
	return nil, auth.ErrUnauthenticated
}

func (r *noAuthResolver) ResolveLogin(_ context.Context, _ *auth.OIDCClaims) (*auth.Identity, error) {
	return nil, auth.ErrUnauthenticated
}

func (r *noAuthResolver) HasAuth(_ context.Context) (bool, error) { return false, nil }

func TestAPIHandler_AuthRequired_NoToken_Returns401(t *testing.T) {
	resolver := &apiTestResolver{ //nolint:gosec // G101: test fixture struct; validToken is a test placeholder, not a real credential
		validToken: "spgr_sk_test",
		identity: &auth.Identity{
			Subject:       "apikey:k1",
			Role:          auth.RoleAdmin,
			EffectiveRole: auth.RoleAdmin,
			Source:        "apikey",
		},
	}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(resolver))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestAPIHandler_AuthRequired_ValidToken_Returns200(t *testing.T) {
	resolver := &apiTestResolver{ //nolint:gosec // G101: test fixture struct; validToken is a test placeholder, not a real credential
		validToken: "spgr_sk_test",
		identity: &auth.Identity{
			Subject:       "apikey:k1",
			Role:          auth.RoleAdmin,
			EffectiveRole: auth.RoleAdmin,
			Source:        "apikey",
		},
	}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(resolver))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	req.Header.Set("Authorization", "Bearer spgr_sk_test")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestAPIHandler_NoKeys_Returns401(t *testing.T) {
	resolver := &noAuthResolver{}

	mux := http.NewServeMux()
	server.RegisterAPIHandlers(mux, &fakeScoper{}, auth.RequireAuth(resolver))

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (no auth bypass)", rec.Code)
	}
}
