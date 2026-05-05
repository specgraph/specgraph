// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 SpecGraph Contributors

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

const testAPIKey = "spgr_sk_test_key" //nolint:gosec // test fixture key, not a real credential

func TestReadMCPResource_Prime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	mcpHandler := newStubMCPHandler(t)
	mux := http.NewServeMux()
	mux.Handle("/mcp/", mcpHandler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	// Write both the new (client.default_server) and legacy (server.remote)
	// schemas — see test_helpers_test.go for rationale.
	body := fmt.Sprintf("client:\n  default_server: %s\nserver:\n  remote: %s\n", srv.URL, srv.URL)
	if err := os.WriteFile(cfgPath, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })

	t.Setenv("SPECGRAPH_API_KEY", testAPIKey)

	var out bytes.Buffer
	oldOut := rootCmd.OutOrStdout()
	oldErr := rootCmd.ErrOrStderr()
	t.Cleanup(func() {
		rootCmd.SetOut(oldOut)
		rootCmd.SetErr(oldErr)
		rootCmd.SetArgs(nil)
	})
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"read-mcp-resource", "specgraph://prime"})
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		t.Fatalf("execute: %v\nout: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "SpecGraph Session Prime") {
		t.Errorf("expected prime body to contain \"SpecGraph Session Prime\", got:\n%s", got)
	}
}

// newStubMCPHandler stands up a minimal MCP server using mcp-go's own server
// library, with a stubbed specgraph://prime resource. The handler is wrapped
// in auth.RequireAuth so the test also exercises the bearer-token round trip.
func newStubMCPHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "test", Key: testAPIKey, Name: "test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("auth store: %v", err)
	}

	srv := mcpserver.NewMCPServer(
		"specgraph-test", "0.0.0",
		mcpserver.WithResourceCapabilities(false, false),
	)
	srv.AddResource(
		mcp.Resource{
			URI:      "specgraph://prime",
			Name:     "prime",
			MIMEType: "text/markdown",
		},
		func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:      "specgraph://prime",
					MIMEType: "text/markdown",
					Text:     "# SpecGraph Session Prime\n\nstub body for CLI test\n",
				},
			}, nil
		},
	)
	httpSrv := mcpserver.NewStreamableHTTPServer(srv)
	return auth.RequireAuth(store)(http.StripPrefix("/mcp", httpSrv))
}
