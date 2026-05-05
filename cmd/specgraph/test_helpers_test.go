// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// startFakeServer is a generic helper that creates an httptest.Server backed by
// a ConnectRPC service handler and points cfgFile at it. All generated handler
// constructors share the func(H, ...connect.HandlerOption) (string, http.Handler)
// signature, so H is inferred from the handler value.
func startFakeServer[H any](t *testing.T, h H, register func(H, ...connect.HandlerOption) (string, http.Handler)) {
	t.Helper()
	mux := http.NewServeMux()
	path, hnd := register(h)
	mux.Handle(path, hnd)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	// Write BOTH the current and legacy global-config schemas to the same
	// file. resolveBaseURL takes one of two paths depending on whether a
	// .specgraph.yaml is found upstack:
	//   - new path: reads client.default_server via loadGlobalCfg.
	//   - legacy path (no .specgraph.yaml): reads server.remote via
	//     config.Load(legacyConfigPath()).
	// Tests that Chdir into a temp dir (e.g. constitution-emit path-traversal)
	// follow the legacy path; tests that run from the worktree root follow
	// the new path (this repo ships a .specgraph.yaml at the root). Writing
	// both keys keeps both paths pointed at srv.URL.
	cfgYAML := fmt.Sprintf("client:\n  default_server: %s\nserver:\n  remote: %s\n", srv.URL, srv.URL)
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o600))
	old := cfgFile
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = old })
}

// --- Per-service typed wrappers ---
// Go generics can't infer that a concrete struct implements a service handler
// interface. These wrappers fix the type parameter explicitly.

func startFakeSpecServer(t *testing.T, h specgraphv1connect.SpecServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.SpecServiceHandler](t, h, specgraphv1connect.NewSpecServiceHandler)
}

func startFakeGraphServer(t *testing.T, h specgraphv1connect.GraphServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.GraphServiceHandler](t, h, specgraphv1connect.NewGraphServiceHandler)
}

func startFakeAuthoringServer(t *testing.T, h specgraphv1connect.AuthoringServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.AuthoringServiceHandler](t, h, specgraphv1connect.NewAuthoringServiceHandler)
}

func startFakeExecutionServer(t *testing.T, h specgraphv1connect.ExecutionServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.ExecutionServiceHandler](t, h, specgraphv1connect.NewExecutionServiceHandler)
}

func startFakeServerServiceServer(t *testing.T, h specgraphv1connect.ServerServiceHandler) {
	t.Helper()
	startFakeServer[specgraphv1connect.ServerServiceHandler](t, h, specgraphv1connect.NewServerServiceHandler)
}

// newCmdWithCtx creates a cobra.Command with a background context set.
// Needed for tests that call functions using cmd.Context() (conversation, report).
func newCmdWithCtx() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

// writeJSONFile creates a temporary JSON file with the given content and returns its path.
func writeJSONFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "input.json")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
