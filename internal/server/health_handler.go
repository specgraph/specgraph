// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Version is set at build time via ldflags.
var Version = "dev"

// HealthHandler implements the ConnectRPC ServerService.
type HealthHandler struct{}

var _ specgraphv1connect.ServerServiceHandler = (*HealthHandler)(nil)

// Health handles the Health RPC, returning server status and version.
func (h *HealthHandler) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	return connect.NewResponse(&specv1.HealthResponse{
		Status:     "ok",
		Version:    Version,
		ServerTime: timestamppb.Now(),
	}), nil
}

// RegisterHealthService registers the ServerService on the given mux.
func RegisterHealthService(mux *http.ServeMux, opts ...connect.HandlerOption) {
	path, handler := specgraphv1connect.NewServerServiceHandler(&HealthHandler{}, opts...)
	mux.Handle(path, handler)
}
