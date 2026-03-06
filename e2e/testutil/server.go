// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
)

// ServerInfo holds the running server's details.
type ServerInfo struct {
	BaseURL string
	Store   *memgraph.Store
}

// StartServer launches a specgraph HTTP server connected to the given Memgraph instance.
// Returns the base URL and a cleanup function that shuts down the server.
func StartServer(ctx context.Context, boltURI string) (*ServerInfo, func(), error) {
	store, err := memgraph.New(ctx, boltURI)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to memgraph: %w", err)
	}

	mux := server.NewMux(store)
	server.RegisterHealthService(mux)
	server.RegisterDecisionService(mux, store)
	server.RegisterGraphService(mux, store)
	server.RegisterClaimService(mux, store)
	server.RegisterConstitutionService(mux, store)
	server.RegisterAuthoringService(mux, store, store)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = store.Close(ctx)
		return nil, nil, fmt.Errorf("listen: %w", err)
	}

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }()

	baseURL := fmt.Sprintf("http://%s", listener.Addr().String())
	cleanup := func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		_ = store.Close(ctx)
	}
	return &ServerInfo{BaseURL: baseURL, Store: store}, cleanup, nil
}
