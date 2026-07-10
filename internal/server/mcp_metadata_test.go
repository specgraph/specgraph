// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

// transientResolver always fails with ErrTransient, to exercise the 503 branch.
type transientResolver struct{}

func (transientResolver) Resolve(_ context.Context, _ string) (*auth.Identity, error) {
	return nil, auth.ErrTransient
}

func (transientResolver) ResolveLogin(_ context.Context, _ *auth.OIDCClaims) (*auth.Identity, error) {
	return nil, auth.ErrTransient
}

func (transientResolver) HasAuth(_ context.Context) (bool, error) { return true, nil }

func TestProtectedResourceMetadata(t *testing.T) {
	const canonicalURI = "https://mcp.example.com/mcp"
	authServers := []string{"https://issuer.example.com", "oauth2:github"}

	mux := http.NewServeMux()
	RegisterProtectedResourceMetadata(mux, canonicalURI, authServers)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var got protectedResourceMetadata
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if got.Resource != canonicalURI {
		t.Errorf("resource = %q, want %q", got.Resource, canonicalURI)
	}
	if len(got.AuthorizationServers) != len(authServers) {
		t.Fatalf("authorization_servers = %v, want %v", got.AuthorizationServers, authServers)
	}
	for i, want := range authServers {
		if got.AuthorizationServers[i] != want {
			t.Errorf("authorization_servers[%d] = %q, want %q", i, got.AuthorizationServers[i], want)
		}
	}
	if len(got.BearerMethodsSupported) != 1 || got.BearerMethodsSupported[0] != "header" {
		t.Errorf("bearer_methods_supported = %v, want [header]", got.BearerMethodsSupported)
	}
}

func TestProtectedResourceMetadata_NonGET(t *testing.T) {
	mux := http.NewServeMux()
	RegisterProtectedResourceMetadata(mux, "https://mcp.example.com/mcp", nil)

	req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-protected-resource", http.NoBody)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestMCPChallenge_Unauthenticated(t *testing.T) {
	const metadataURL = "https://mcp.example.com/.well-known/oauth-protected-resource"
	wrapper := RequireAuthWithChallenge(&mockResolver{}, metadataURL)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/mcp/", http.NoBody) // no credential
	rec := httptest.NewRecorder()
	wrapper(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	challenge := rec.Header().Get("WWW-Authenticate")
	if !strings.HasPrefix(challenge, "Bearer ") {
		t.Errorf("WWW-Authenticate = %q, want Bearer scheme", challenge)
	}
	if !strings.Contains(challenge, `resource_metadata="`+metadataURL+`"`) {
		t.Errorf("WWW-Authenticate = %q, want resource_metadata pointing at %q", challenge, metadataURL)
	}
}

func TestMCPChallenge_Authenticated(t *testing.T) {
	wrapper := RequireAuthWithChallenge(&mockResolver{}, "https://mcp.example.com/.well-known/oauth-protected-resource")

	var (
		called     bool
		sawMCPMark bool
	)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		sawMCPMark = auth.MCPRequestFromContext(r.Context())
		if _, ok := auth.IdentityFromContext(r.Context()); !ok {
			t.Error("identity missing from downstream context")
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/mcp/", http.NoBody)
	req.Header.Set("Authorization", "Bearer valid-test-key")
	rec := httptest.NewRecorder()
	wrapper(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !called {
		t.Error("next handler was not called on the success path")
	}
	if !sawMCPMark {
		t.Error("downstream context missing WithMCPRequest marker")
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate = %q on success, want empty", got)
	}
}

func TestMCPChallenge_Transient(t *testing.T) {
	wrapper := RequireAuthWithChallenge(transientResolver{}, "https://mcp.example.com/.well-known/oauth-protected-resource")
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/mcp/", http.NoBody)
	req.Header.Set("Authorization", "Bearer whatever")
	rec := httptest.NewRecorder()
	wrapper(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got != "" {
		t.Errorf("WWW-Authenticate = %q on transient error, want empty (not a credential failure)", got)
	}
}
