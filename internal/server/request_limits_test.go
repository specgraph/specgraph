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

	// Build a syntactically valid ConnectRPC JSON request body that exceeds 4 MiB.
	// The padding field doesn't exist in HealthRequest, but ConnectRPC still reads
	// the full body before rejecting unknown fields, so the size limit triggers first.
	padding := bytes.Repeat([]byte("a"), 5<<20) // 5 MiB
	body := append([]byte(`{"padding":"`), padding...)
	body = append(body, []byte(`"}`)...)

	resp, err := http.Post(
		srv.URL+path+"Health",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	// ConnectRPC maps resource_exhausted to HTTP 429 (Too Many Requests).
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for oversized body, got %d: %s", resp.StatusCode, string(respBody))
	}
	if !bytes.Contains(respBody, []byte("resource_exhausted")) {
		t.Fatalf("expected resource_exhausted error code, got: %s", string(respBody))
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
