// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// audienceBindingStore builds an identity store whose backend resolves an
// existing OIDC binding to an active user, wired to the given issuer verifier
// and MCP resource URI. It isolates the resource-URI audience assertion under
// test from the JIT/login-sync paths.
func audienceBindingStore(t *testing.T, v *auth.OIDCVerifier, resourceURI string) auth.Resolver {
	t.Helper()
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, iss, sub string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{ID: "b1", UserID: "u1", Issuer: iss, Subject: sub}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          stub,
		Verifiers:      []*auth.OIDCVerifier{v},
		Tracker:        &noopTracker{},
		MCPResourceURI: resourceURI,
	})
	require.NoError(t, err)
	return store
}

const testResourceURI = "https://specgraph.example.com/mcp"

// TestAudienceBinding_MCPMarked_ResourceBoundAccepted proves an MCP-path JWT
// whose aud contains BOTH the client_id (verifier audience) AND the canonical
// resource URI resolves (RFC 8707, D-05.3).
func TestAudienceBinding_MCPMarked_ResourceBoundAccepted(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	store := audienceBindingStore(t, v, testResourceURI)

	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "sub-1",
		"aud": []string{"aud-1", testResourceURI},
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	ctx := auth.WithMCPRequest(context.Background())
	id, err := store.Resolve(ctx, tok)
	require.NoError(t, err, "resource-URI-bound MCP token must resolve")
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, p.server.URL, id.Issuer)
}

// TestAudienceBinding_MCPMarked_ClientIDOnlyRejected proves an MCP-path JWT
// bound only to client_id (aud omits the resource URI) is rejected — the
// confused-deputy case the RFC 8707 check exists to catch.
func TestAudienceBinding_MCPMarked_ClientIDOnlyRejected(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	store := audienceBindingStore(t, v, testResourceURI)

	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "sub-1", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	ctx := auth.WithMCPRequest(context.Background())
	_, err = store.Resolve(ctx, tok)
	require.ErrorIs(t, err, auth.ErrUnauthenticated,
		"MCP token bound only to client_id (no resource URI in aud) must be rejected")
}

// TestAudienceBinding_NonMCPMarked_ClientIDOnlyResolves is the HIGH #2
// regression guard: with a resource URI configured but the request NOT marked
// WithMCPRequest (a ConnectRPC JWT caller), a client_id-only token STILL
// resolves — the resource-URI check must be path-scoped to /mcp/ only.
func TestAudienceBinding_NonMCPMarked_ClientIDOnlyResolves(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	store := audienceBindingStore(t, v, testResourceURI)

	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "sub-1", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	// No WithMCPRequest marker → ConnectRPC-style caller.
	id, err := store.Resolve(context.Background(), tok)
	require.NoError(t, err,
		"a non-MCP (ConnectRPC) JWT with aud=client_id must still resolve even when an MCP resource URI is configured")
	require.Equal(t, "u1", id.UserID)
}

// TestAudienceBinding_EmptyConfig_Additive proves that with no MCP resource URI
// configured, a client_id-only token resolves even on an MCP-marked request —
// the check is fully additive (D-08).
func TestAudienceBinding_EmptyConfig_Additive(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	store := audienceBindingStore(t, v, "") // no resource URI

	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "sub-1", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	ctx := auth.WithMCPRequest(context.Background())
	id, err := store.Resolve(ctx, tok)
	require.NoError(t, err, "with no MCP resource URI configured the audience check must not fire")
	require.Equal(t, "u1", id.UserID)
}
