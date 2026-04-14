// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"net/http"
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
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
			assert.Equal(t, "Bearer ctx-token-123", got) //nolint:gosec // test assertion, not a real credential
			return &http.Response{StatusCode: http.StatusOK}, nil
		}),
		project: "test-project",
	}

	ctx := auth.WithBearerToken(t.Context(), "ctx-token-123") //nolint:gosec // test fixture, not a real credential
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

	ctx := auth.WithBearerToken(t.Context(), "some-token") //nolint:gosec // test fixture, not a real credential
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
		assert.Equal(t, "static-key", token) //nolint:gosec // test assertion, not a real credential
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	transport := &staticTokenTransport{inner: inner, token: "static-key"} //nolint:gosec // test fixture, not a real credential
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
		token: "chained-key", //nolint:gosec // test fixture, not a real credential
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://localhost/test", nil)
	require.NoError(t, err)

	_, err = transport.RoundTrip(req)
	require.NoError(t, err)

	assert.Equal(t, "Bearer chained-key", capturedAuth) //nolint:gosec // test assertion, not a real credential
}
