// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectTransport_InjectsHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("X-Specgraph-Project"))) //nolint:gosec // test echo handler; taint is from controlled test input
	}))
	defer ts.Close()

	client := &http.Client{Transport: &clientTransport{
		base:    http.DefaultTransport,
		project: "test-proj",
	}}
	resp, err := client.Get(ts.URL) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "test-proj", string(body))
}

func TestProjectTransport_EmptyProject(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.Header.Get("X-Specgraph-Project"))) //nolint:gosec // test echo handler; taint is from controlled test input
	}))
	defer ts.Close()

	client := &http.Client{Transport: &clientTransport{
		base:    http.DefaultTransport,
		project: "",
	}}
	resp, err := client.Get(ts.URL) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	// Empty project slug — header is set but empty.
	assert.Equal(t, "", string(body))
}

func TestProjectTransport_DoesNotMutateOriginalRequest(t *testing.T) {
	var captured http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	orig, err := http.NewRequest(http.MethodGet, ts.URL, nil) //nolint:noctx // test helper
	require.NoError(t, err)
	orig.Header.Set("X-Custom", "original")

	client := &http.Client{Transport: &clientTransport{
		base:    http.DefaultTransport,
		project: "injected",
	}}
	resp, err := client.Do(orig) //nolint:bodyclose // test helper
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "injected", captured.Get("X-Specgraph-Project"))
	// Original request header should not be modified.
	assert.Empty(t, orig.Header.Get("X-Specgraph-Project"))
}

func TestClientTransport_InjectsBearerToken(t *testing.T) {
	var captured http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Transport: &clientTransport{ //nolint:gosec // test fixture, not a real credential
		base:        http.DefaultTransport,
		bearerToken: "spgr_sk_testkey",
	}}
	resp, err := client.Get(ts.URL) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "Bearer spgr_sk_testkey", captured.Get("Authorization"))
}

func TestClientTransport_NoBearerWhenEmpty(t *testing.T) {
	var captured http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := &http.Client{Transport: &clientTransport{
		base: http.DefaultTransport,
	}}
	resp, err := client.Get(ts.URL) //nolint:noctx // test helper
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Empty(t, captured.Get("Authorization"))
}
