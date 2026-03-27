// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
)

// stubHealthHandler implements a minimal ServerServiceHandler for testing.
type stubHealthHandler struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (h *stubHealthHandler) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	return connect.NewResponse(&specv1.HealthResponse{Status: "ok"}), nil
}

func TestReadMaxBytes_RejectsOversizedBody(t *testing.T) {
	// Register a handler with a 4 MiB read limit.
	mux := http.NewServeMux()
	opts := connect.WithReadMaxBytes(4 << 20)
	path, handler := specgraphv1connect.NewServerServiceHandler(&stubHealthHandler{}, opts)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Send a request body larger than 4 MiB directly via HTTP POST to the
	// ConnectRPC endpoint. The handler should reject it.
	oversized := bytes.Repeat([]byte("x"), 5<<20) // 5 MiB
	resp, err := http.Post(
		srv.URL+path+"Health",
		"application/json",
		bytes.NewReader(oversized),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	// ConnectRPC returns 400 (resource_exhausted) for oversized bodies.
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("expected rejection for oversized body, got 200")
	}
}

func TestReadMaxBytes_AllowsNormalBody(t *testing.T) {
	// Same setup with 4 MiB limit.
	opts := connect.WithReadMaxBytes(4 << 20)
	mux := http.NewServeMux()
	server.RegisterHealthService(mux, opts)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, srv.URL)
	_, err := client.Health(context.Background(), connect.NewRequest(&specv1.HealthRequest{}))
	if err != nil {
		t.Fatalf("expected success for normal request, got: %v", err)
	}
}
