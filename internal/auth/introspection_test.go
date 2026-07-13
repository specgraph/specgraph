// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// introspectionStub is an httptest RFC 7662 endpoint. handler controls the
// response; calls counts how many times the endpoint was hit (used to prove
// the spgr_sk_ guard keeps API keys off the IdP).
type introspectionStub struct {
	server *httptest.Server
	calls  atomic.Int64
}

func newIntrospectionStub(t *testing.T, handler func(token string) (status int, body map[string]any)) *introspectionStub {
	t.Helper()
	stub := &introspectionStub{}
	mux := http.NewServeMux()
	mux.HandleFunc("/introspect", func(w http.ResponseWriter, r *http.Request) {
		stub.calls.Add(1)
		_ = r.ParseForm()
		status, body := handler(r.Form.Get("token"))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	})
	stub.server = httptest.NewServer(mux)
	t.Cleanup(stub.server.Close)
	return stub
}

func (s *introspectionStub) url() string { return s.server.URL + "/introspect" }

// introspectionBindingStub returns a users backend resolving an existing OIDC
// binding to an active user, isolating resolveIntrospection from the JIT path.
func introspectionBindingStub() *usersBackendStub {
	return &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, iss, sub string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{ID: "b1", UserID: "u1", Issuer: iss, Subject: sub}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
}

const introspectResourceURI = "https://specgraph.example.com/mcp"

// TestIntrospection_ActiveResourceBound_Resolves proves an opaque token the IdP
// reports active==true with an aud containing the resource URI resolves to an
// identity (D-06).
func TestIntrospection_ActiveResourceBound_Resolves(t *testing.T) {
	stub := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{
			"active": true, "sub": "sub-1", "iss": "https://idp.example.com",
			"aud": []string{introspectResourceURI},
			"exp": time.Now().Add(time.Hour).Unix(),
		}
	})
	intro := auth.NewIntrospector("https://idp.example.com", stub.url(), "rs-client", "rs-secret")
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          introspectionBindingStub(),
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{intro},
	})
	require.NoError(t, err)

	id, err := store.Resolve(context.Background(), "opaque-access-token")
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "oidc:sub-1", id.Subject)
	require.Equal(t, int64(1), stub.calls.Load())
}

// TestIntrospection_Inactive_Rejected proves an inactive token → ErrUnauthenticated.
func TestIntrospection_Inactive_Rejected(t *testing.T) {
	stub := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{"active": false}
	})
	intro := auth.NewIntrospector("https://idp.example.com", stub.url(), "rs-client", "rs-secret")
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          introspectionBindingStub(),
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{intro},
	})
	require.NoError(t, err)

	_, err = store.Resolve(context.Background(), "opaque-inactive-token")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// TestIntrospection_ServerError_Transient proves a 5xx from the IdP → ErrTransient
// (fail-closed, retryable).
func TestIntrospection_ServerError_Transient(t *testing.T) {
	stub := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusInternalServerError, nil
	})
	intro := auth.NewIntrospector("https://idp.example.com", stub.url(), "rs-client", "rs-secret")
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          introspectionBindingStub(),
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{intro},
	})
	require.NoError(t, err)

	_, err = store.Resolve(context.Background(), "opaque-token")
	require.ErrorIs(t, err, auth.ErrTransient)
}

// TestIntrospection_WrongAudience_Rejected proves an active token whose aud does
// NOT contain the resource URI is rejected (confused-deputy, decisive → 401).
func TestIntrospection_WrongAudience_Rejected(t *testing.T) {
	stub := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{
			"active": true, "sub": "sub-1", "iss": "https://idp.example.com",
			"aud": []string{"some-other-resource"},
			"exp": time.Now().Add(time.Hour).Unix(),
		}
	})
	intro := auth.NewIntrospector("https://idp.example.com", stub.url(), "rs-client", "rs-secret")
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          introspectionBindingStub(),
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{intro},
	})
	require.NoError(t, err)

	_, err = store.Resolve(context.Background(), "opaque-wrong-aud-token")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// TestIntrospection_MultiIntrospector_FirstMatchWins proves that with multiple
// introspectors, the first returning an active+resource-aud result wins and the
// later introspector is not consulted.
func TestIntrospection_MultiIntrospector_FirstMatchWins(t *testing.T) {
	first := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{
			"active": true, "sub": "sub-first", "iss": "https://idp-a.example.com",
			"aud": []string{introspectResourceURI},
			"exp": time.Now().Add(time.Hour).Unix(),
		}
	})
	second := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{"active": true, "sub": "sub-second"}
	})
	introA := auth.NewIntrospector("https://idp-a.example.com", first.url(), "a", "a-secret")
	introB := auth.NewIntrospector("https://idp-b.example.com", second.url(), "b", "b-secret")
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          introspectionBindingStub(),
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{introA, introB},
	})
	require.NoError(t, err)

	id, err := store.Resolve(context.Background(), "opaque-token")
	require.NoError(t, err)
	require.Equal(t, "oidc:sub-first", id.Subject)
	require.Equal(t, int64(1), first.calls.Load(), "first introspector should be consulted")
	require.Equal(t, int64(0), second.calls.Load(), "second introspector must not be consulted after first match")
}

// TestIntrospection_APIKeyNeverIntrospected is the HIGH #3 / D-08 guard: an
// spgr_sk_-prefixed API key is routed to resolveAPIKey by the explicit prefix
// guard and must NEVER be POSTed to the introspection endpoint, even when
// introspectors are configured. The stub's request counter must stay zero.
func TestIntrospection_APIKeyNeverIntrospected(t *testing.T) {
	stub := newIntrospectionStub(t, func(_ string) (int, map[string]any) {
		return http.StatusOK, map[string]any{"active": true}
	})
	intro := auth.NewIntrospector("https://idp.example.com", stub.url(), "rs-client", "rs-secret")
	// No API key exists in the backend → resolveAPIKey rejects with 401. The
	// point is that the introspection endpoint is never called.
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          &usersBackendStub{},
		Tracker:        &noopTracker{},
		MCPResourceURI: introspectResourceURI,
		Introspectors:  []*auth.Introspector{intro},
	})
	require.NoError(t, err)

	apiKey := stubAPIKeyToken("prefix12")
	_, err = store.Resolve(context.Background(), apiKey)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
	require.Equal(t, int64(0), stub.calls.Load(),
		"an spgr_sk_ API key must never reach the introspection endpoint (HIGH #3, D-08)")
}
