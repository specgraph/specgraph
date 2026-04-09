// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"io/fs"
	"net/http"
	"strings"
)

// StaticHandler serves embedded static files with an SPA catch-all.
// Files matching actual paths are served directly. All other paths
// return index.html so SvelteKit client-side routing works.
func StaticHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try serving the file directly.
		// fs.FS requires clean paths without leading slash.
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		// Check if the file exists in the embedded FS
		f, err := fsys.Open(path)
		if err == nil {
			if closeErr := f.Close(); closeErr != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA catch-all: serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
