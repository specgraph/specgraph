// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import "net/http"

// CORSMiddleware adds CORS headers for development mode.
// Only enabled when origin is non-empty (set via --cors-origin flag).
func CORSMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Connect-Protocol-Version, X-Specgraph-Project")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
