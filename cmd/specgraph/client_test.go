// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// roundTripFunc is an adapter to allow the use of ordinary functions as
// http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientTransport_ProjectHeader(t *testing.T) {
	transport := &clientTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got := req.Header.Get("X-Specgraph-Project")
			assert.Equal(t, "my-project", got)
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "my-project",
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestClientTransport_EmptyProject(t *testing.T) {
	transport := &clientTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got := req.Header.Get("X-Specgraph-Project")
			assert.Empty(t, got)
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "",
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)
}

func TestClientTransport_TokenFromContext(t *testing.T) {
	transport := &clientTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got := req.Header.Get("Authorization")
			assert.Equal(t, "Bearer ctx-token-123", got)
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "test-project",
	}

	ctx := auth.WithBearerToken(t.Context(), "ctx-token-123")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)
}

func TestClientTransport_NoToken(t *testing.T) {
	transport := &clientTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got := req.Header.Get("Authorization")
			assert.Empty(t, got)
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "test-project",
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)
}

func TestClientTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	var capturedProject string
	transport := &clientTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			capturedProject = req.Header.Get("X-Specgraph-Project")
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "injected",
	}

	ctx := auth.WithBearerToken(t.Context(), "some-token")
	orig, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)
	orig.Header.Set("X-Custom", "original")

	_, err = transport.RoundTrip(orig)
	require.NoError(t, err)

	assert.Equal(t, "injected", capturedProject)
	// Original request header should not be modified.
	assert.Empty(t, orig.Header.Get("X-Specgraph-Project"))
}

func TestStaticTokenTransport_InjectsToken(t *testing.T) {
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		token, ok := auth.BearerTokenFromContext(req.Context())
		assert.True(t, ok, "expected bearer token in context")
		assert.Equal(t, "static-key", token)
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	transport := &staticTokenTransport{inner: inner, token: "static-key"}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)
}

func TestStaticTokenTransport_EmptyTokenSkipsInjection(t *testing.T) {
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		_, ok := auth.BearerTokenFromContext(req.Context())
		assert.False(t, ok, "expected no bearer token in context")
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	transport := &staticTokenTransport{inner: inner, token: ""}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)
}

func TestStaticTokenTransport_ChainedWithClientTransport(t *testing.T) {
	// Verify the full chain: staticTokenTransport puts token in context,
	// clientTransport reads it and sets the Authorization header.
	var capturedAuth string
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		capturedAuth = req.Header.Get("Authorization")
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	transport := &staticTokenTransport{
		inner: &clientTransport{base: base, project: "proj"},
		token: "chained-key",
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer chained-key", capturedAuth)
}

// TestSelfMint_SessionPrecedence covers the Finding-D credential-precedence fix
// used by the self-service `auth api-key` commands: the session-preferring
// resolver must return the stored login session and ignore SPECGRAPH_API_KEY,
// and setting the env key while a self command runs must emit a stderr warning —
// while the default env-first resolveAPIKey stays unchanged for admin/other
// commands.
func TestSelfMint_SessionPrecedence(t *testing.T) {
	const serverURL = "https://specgraph.example.com"

	// Point the credentials file at a temp XDG config dir with a stored session.
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	credPath := filepath.Join(cfgHome, "specgraph", "credentials.yaml")
	f := &credentials.File{}
	//nolint:gosec // test fixture token, not a real credential
	f.Upsert(serverURL, credentials.ServerCreds{Token: "spgr_ws_session_abc", Label: "oidc:alice"})
	require.NoError(t, f.Save(credPath))

	t.Run("session resolver ignores SPECGRAPH_API_KEY", func(t *testing.T) {
		t.Setenv("SPECGRAPH_API_KEY", "spgr_sk_envkey_should_be_ignored")

		// Session-preferring path returns the stored session, not the env key.
		assert.Equal(t, "spgr_ws_session_abc", resolveSessionCredential(serverURL))
		// The default env-first resolver is unchanged: it still prefers the env key.
		assert.Equal(t, "spgr_sk_envkey_should_be_ignored", resolveAPIKey(serverURL))
	})

	t.Run("warns to stderr when env key is set", func(t *testing.T) {
		t.Setenv("SPECGRAPH_API_KEY", "spgr_sk_envkey_should_be_ignored")
		var buf bytes.Buffer
		warnIfEnvKeyIgnored(&buf)
		assert.Contains(t, buf.String(), "SPECGRAPH_API_KEY")
		assert.Contains(t, buf.String(), "ignored")
	})

	t.Run("no warning when env key is unset", func(t *testing.T) {
		t.Setenv("SPECGRAPH_API_KEY", "")
		var buf bytes.Buffer
		warnIfEnvKeyIgnored(&buf)
		assert.Empty(t, buf.String())
	})
}
