// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package auth_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specgraphv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// capturingAuthorizer allows every request and records the Identity it was
// handed. It lets the full-stack tests assert that the Identity the resolver
// produced from a real JWT actually propagated through the interceptor to the
// authorization boundary (and thus to the handler).
type capturingAuthorizer struct {
	mu   sync.Mutex
	last *auth.Identity
}

func (c *capturingAuthorizer) Authorize(_ context.Context, id *auth.Identity, _ string, _ any) (auth.Decision, error) {
	c.mu.Lock()
	c.last = id
	c.mu.Unlock()
	return auth.Decision{Allowed: true, Reason: "test-allow"}, nil
}

func (c *capturingAuthorizer) lastIdentity() *auth.Identity {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last
}

// TestFullStack_JWT_JITProvisionsAndReachesHandler drives a real signed JWT for
// an unknown subject through NewAuthInterceptor backed by the real Postgres
// resolver + verifier. This closes the seam the unit/integration suites left
// open: header parsing → interceptor → real JIT resolve → identity at the
// authorization boundary. The existing full-stack identity tests only ever used
// API-key tokens, so the JWT branch never traversed the interceptor before.
func TestFullStack_JWT_JITProvisionsAndReachesHandler(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	_, resolver := authnTestStore(t, issuer, "aud-1")

	authorizer := &capturingAuthorizer{}
	srv, _, _ := newTestServer(t, resolver, authorizer)

	const subject = "fullstack-jit-subject"
	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "carol@example.com",
	})

	client := newSpecClientWithAuth(srv.URL, token)
	_, err := client.GetSpec(ctx, connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	require.NoError(t, err, "a valid JWT must authenticate through the full interceptor stack")

	id := authorizer.lastIdentity()
	require.NotNil(t, id, "the resolved identity must reach the authorizer")
	require.Equal(t, "oidc", id.Source)
	require.Equal(t, "oidc:"+subject, id.Subject)
	require.Equal(t, auth.RoleReader, id.EffectiveRole, "JIT provisions at the configured default role")
	require.Equal(t, "carol@example.com", id.Email)
}

// TestFullStack_JWT_ExistingBindingRoleReachesHandler verifies that a JWT whose
// (issuer, subject) is already bound resolves to the persisted user — including
// the user's role — through the interceptor to the authorization boundary.
func TestFullStack_JWT_ExistingBindingRoleReachesHandler(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	store, resolver := authnTestStore(t, issuer, "aud-1")

	const subject = "fullstack-bound-subject"
	_, err := store.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "Dave", Email: "dave@example.com", Role: "writer",
	}, &storage.OIDCBinding{
		Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "dave@example.com",
	})
	require.NoError(t, err)

	authorizer := &capturingAuthorizer{}
	srv, _, _ := newTestServer(t, resolver, authorizer)

	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "dave@example.com",
	})
	client := newSpecClientWithAuth(srv.URL, token)
	_, err = client.GetSpec(ctx, connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	require.NoError(t, err)

	id := authorizer.lastIdentity()
	require.NotNil(t, id)
	require.Equal(t, auth.Role("writer"), id.EffectiveRole, "the bound user's role must reach the authorizer")
}

// TestFullStack_JWT_BadAudienceRejected asserts a JWT with a wrong audience is
// rejected at the interceptor with CodeUnauthenticated and never reaches the
// authorizer.
func TestFullStack_JWT_BadAudienceRejected(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	_, resolver := authnTestStore(t, issuer, "aud-1")

	authorizer := &capturingAuthorizer{}
	srv, _, _ := newTestServer(t, resolver, authorizer)

	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": "x", "aud": "wrong-audience",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	client := newSpecClientWithAuth(srv.URL, token)
	_, err := client.GetSpec(ctx, connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Nil(t, authorizer.lastIdentity(), "a rejected token must never reach the authorizer")
}

// TestFullStack_JWT_UnknownIssuerRejected asserts a validly-structured JWT from
// an issuer with no configured verifier is rejected at the interceptor and
// never reaches the authorizer.
func TestFullStack_JWT_UnknownIssuerRejected(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	other := newOIDCTestIssuer(t) // a second issuer the resolver has no verifier for
	_, resolver := authnTestStore(t, issuer, "aud-1")

	authorizer := &capturingAuthorizer{}
	srv, _, _ := newTestServer(t, resolver, authorizer)

	// Token signed by `other` and claiming `other`'s issuer — the resolver has
	// no verifier registered for that issuer, so routing fails closed.
	token := other.mintToken(t, map[string]any{
		"iss": other.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	client := newSpecClientWithAuth(srv.URL, token)
	_, err := client.GetSpec(ctx, connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	require.Nil(t, authorizer.lastIdentity())
}
