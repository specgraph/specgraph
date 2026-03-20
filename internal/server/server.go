// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package server implements ConnectRPC service handlers for SpecGraph.
package server

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// NewMux creates an http.ServeMux with the SpecService handler registered.
// Callers register additional services (lifecycle, authoring, etc.) on the
// returned mux via their respective Register functions.
func NewMux(scoper storage.Scoper, opts ...connect.HandlerOption) *http.ServeMux {
	mux := http.NewServeMux()
	specHandler := NewSpecHandler(scoper)
	path, handler := specgraphv1connect.NewSpecServiceHandler(specHandler, opts...)
	mux.Handle(path, handler)
	return mux
}
