// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package server implements ConnectRPC service handlers for SpecGraph.
package server

import (
	"net/http"

	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// NewMux creates an http.ServeMux with the SpecService handler registered.
func NewMux(backend storage.Backend) *http.ServeMux {
	mux := http.NewServeMux()
	specHandler := NewSpecHandler(backend)
	path, handler := specgraphv1connect.NewSpecServiceHandler(specHandler)
	mux.Handle(path, handler)
	return mux
}
