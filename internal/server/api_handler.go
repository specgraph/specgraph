// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"encoding/json"
	"net/http"

	"github.com/specgraph/specgraph/internal/storage"
)

// RegisterAPIHandlers registers lightweight HTTP endpoints for the UI.
func RegisterAPIHandlers(mux *http.ServeMux, scoper storage.Scoper) {
	mux.HandleFunc("/api/projects", func(w http.ResponseWriter, r *http.Request) {
		store, err := scoper.Scoped(r.Context(), "_server")
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		projects, err := store.ListProjects(r.Context())
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		slugs := make([]string, 0, len(projects))
		for _, p := range projects {
			if p.Slug != "_server" {
				slugs = append(slugs, p.Slug)
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"projects": slugs}) //nolint:errcheck // best-effort write to http.ResponseWriter
	})
}
