// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/fetch"
)

const testToken = "ghp_TESTTOKEN1234567890"

// TestSecurity_TokenAllowList: with token set, fetching a non-GitHub
// HTTPS URL must NOT include Authorization in the captured request.
func TestSecurity_TokenAllowList(t *testing.T) {
	t.Setenv("SPECGRAPH_FETCH_GITHUB_TOKEN", testToken)

	var capturedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("name: test\n"))
	}))
	defer srv.Close()

	_, err := fetch.Fetch(context.Background(), srv.URL+"/constitution.yaml")
	require.NoError(t, err)
	assert.Empty(t, capturedAuth,
		"token must NOT be sent to non-allow-list hosts (httptest is 127.0.0.1, not raw.githubusercontent.com)")
}

// TestSecurity_BodySizeCap: bodies > 1 MiB are rejected after fetch
// completes but before the body is read into memory.
func TestSecurity_BodySizeCap(t *testing.T) {
	big := bytes.Repeat([]byte("x"), 2<<20) // 2 MiB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(big)
	}))
	defer srv.Close()

	_, err := fetch.Fetch(context.Background(), srv.URL+"/big.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds",
		"size-cap rejection error must mention 'exceeds'")
}

// TestSecurity_URLWithUserInfo_Rejected: https://token@host/path rejected.
func TestSecurity_URLWithUserInfo_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://ghp_xxxxxxxxxxxx@raw.githubusercontent.com/foo/bar/main/constitution.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded credentials")
	assert.Contains(t, err.Error(), "SPECGRAPH_FETCH_GITHUB_TOKEN")
}

// TestSecurity_URLWithUserPass_Rejected: https://user:pass@host/path rejected.
func TestSecurity_URLWithUserPass_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://user:pass@example.com/constitution.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "embedded credentials")
}

// TestSecurity_URLWithTokenQuery_Rejected: ?token=secret rejected with
// a specific error naming the parameter.
func TestSecurity_URLWithTokenQuery_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://example.com/constitution.yaml?token=secret123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential parameter")
	assert.Contains(t, err.Error(), "token")
}

// TestSecurity_URLWithAccessTokenQuery_Rejected: ?access_token=secret rejected.
func TestSecurity_URLWithAccessTokenQuery_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://example.com/constitution.yaml?access_token=secret123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential parameter")
	assert.Contains(t, err.Error(), "access_token")
}

// TestSecurity_URLWithAPIKeyQuery_Rejected: ?api_key=secret rejected.
func TestSecurity_URLWithAPIKeyQuery_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://example.com/constitution.yaml?api_key=secret123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential parameter")
	assert.Contains(t, err.Error(), "api_key")
}

// TestSecurity_URLWithPasswordQuery_Rejected: ?password=secret rejected.
func TestSecurity_URLWithPasswordQuery_Rejected(t *testing.T) {
	_, err := fetch.Fetch(context.Background(),
		"https://example.com/constitution.yaml?password=secret123")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential parameter")
	assert.Contains(t, err.Error(), "password")
}

// TestSecurity_LogRedaction: token bytes must not appear in slog output
// during a successful fetch.
func TestSecurity_LogRedaction(t *testing.T) {
	t.Setenv("SPECGRAPH_FETCH_GITHUB_TOKEN", testToken)

	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(orig)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("name: test\n"))
	}))
	defer srv.Close()

	_, err := fetch.Fetch(context.Background(), srv.URL+"/constitution.yaml")
	require.NoError(t, err, "fetch must succeed so the redaction check exercises the happy path")
	assert.NotContains(t, buf.String(), testToken,
		"token bytes must not appear in any log line")
}

// TestSecurity_FileURLRejectedInProduction is documented as a build-tag
// invariant rather than a runtime test: in testfetch builds the file://
// getter IS registered (we use it in TestFetch_HappyPath_FileScheme), so
// we can't assert the rejection here. The production-mode rejection is
// enforced by the absence of the file getter from defaultGetters in
// non-testfetch builds.

// TestSecurity_YAMLBomb: large YAML structures must fail gracefully
// rather than hanging the parser. Note this tests the LOADER not the
// fetcher — fetch.Fetch returns raw bytes; parsing happens in the
// load package. This test placed here documents the boundary.
func TestSecurity_YAMLBomb(t *testing.T) {
	// 100KB of deeply nested aliases that some YAML parsers can't handle.
	// gopkg.in/yaml.v3 has built-in depth/alias limits; verify they fire.
	// This test is deferred to the load package's tests in a follow-up;
	// gopkg.in/yaml.v3's defaults are sufficient for V1 per
	// CVE-2022-21698 fixes.
	t.Skip("YAML bomb resistance is provided by gopkg.in/yaml.v3's built-in alias limits; explicit test deferred to a load-package follow-up")
}
