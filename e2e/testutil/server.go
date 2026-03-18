// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e || e2e_cli || e2e_agent

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"connectrpc.com/connect"

	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
)

// ServerInfo holds the running server's details.
type ServerInfo struct {
	BaseURL    string
	Store      *memgraph.Store
	ConfigPath string // path to a temp config file pointing at this server
}

// StartServer launches a specgraph HTTP server connected to the given Memgraph instance.
// Returns the base URL and a cleanup function that shuts down the server.
func StartServer(ctx context.Context, boltURI string, opts ...connect.HandlerOption) (*ServerInfo, func(), error) {
	var store *memgraph.Store
	var err error
	for range 10 {
		store, err = memgraph.New(ctx, boltURI, memgraph.WithProject("e2e-test"))
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect to memgraph: %w", err)
	}

	mux := server.NewMux(store, opts...)
	server.RegisterHealthService(mux, opts...)
	server.RegisterDecisionService(mux, store, opts...)
	server.RegisterGraphService(mux, store, opts...)
	server.RegisterClaimService(mux, store, opts...)
	server.RegisterConstitutionService(mux, store, opts...)
	server.RegisterAuthoringService(mux, store, opts...)
	server.RegisterExecutionService(mux, store, opts...)
	driftEngine := drift.NewEngine(store, nil)
	lintEngine := linter.NewEngine(store, nil)
	server.RegisterLifecycleService(mux, store, driftEngine, lintEngine, nil, opts...)
	server.RegisterSyncService(mux, store, "", opts...)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = store.Close(ctx)
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: server.ProjectMiddleware(mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "testutil: server error: %v\n", err)
		}
	}()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	cleanup := func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			fmt.Fprintf(os.Stderr, "testutil: server shutdown error: %v\n", err)
		}
		if err := store.Close(shutCtx); err != nil {
			fmt.Fprintf(os.Stderr, "testutil: store close error: %v\n", err)
		}
	}
	// Write a temp config file pointing the CLI at this server.
	// Used by E2E tests that shell out to the specgraph binary.
	cfgDir, err := os.MkdirTemp("", "specgraph-e2e-config-*")
	if err != nil {
		_ = srv.Close()
		_ = store.Close(ctx)
		return nil, nil, fmt.Errorf("create temp config dir: %w", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.yaml")
	cfgContent := fmt.Sprintf("server:\n  remote: %s\n", baseURL)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o600); err != nil {
		_ = srv.Close()
		_ = store.Close(ctx)
		os.RemoveAll(cfgDir) //nolint:errcheck
		return nil, nil, fmt.Errorf("write temp config: %w", err)
	}

	origCleanup := cleanup
	cleanup = func() {
		origCleanup()
		os.RemoveAll(cfgDir) //nolint:errcheck
	}
	return &ServerInfo{BaseURL: baseURL, Store: store, ConfigPath: cfgPath}, cleanup, nil
}
