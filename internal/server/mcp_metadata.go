// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/reqctx"
)

// protectedResourceMetadataPath is the RFC 9728 well-known path where the
// Protected Resource Metadata document is published.
const protectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

// protectedResourceMetadata is the RFC 9728 §3.2 OAuth 2.0 Protected Resource
// Metadata document. `resource` MUST equal the canonical identifier clients
// fetch the document by (§3.3); `authorization_servers` lists every issuer a
// client may obtain a resource-bound token from.
type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// RegisterProtectedResourceMetadata mounts the public, unauthenticated RFC 9728
// Protected Resource Metadata endpoint at /.well-known/oauth-protected-resource.
// The document advertises the canonical resource URI and the authorization
// servers (issuers) an MCP client can use to obtain a resource-bound token. It
// is mounted like /api/auth/oidc/providers — no RequireAuth (discovery must be
// reachable pre-authentication); it exposes only issuer URLs already known to
// clients.
func RegisterProtectedResourceMetadata(mux *http.ServeMux, resourceURI string, authServers []string) {
	meta := protectedResourceMetadata{
		Resource:               resourceURI,
		AuthorizationServers:   authServers,
		BearerMethodsSupported: []string{"header"},
	}
	// The document is static for the process lifetime; marshal once.
	body, err := json.Marshal(meta)
	if err != nil {
		// Unreachable: the struct contains only strings/[]string. Fail closed
		// to an empty object rather than panicking during startup wiring.
		body = []byte("{}")
	}
	mux.HandleFunc(protectedResourceMetadataPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(body) //nolint:errcheck // best-effort write of a static document
	})
}

// RequireAuthWithChallenge returns HTTP middleware for the /mcp/ OAuth 2.1
// resource-server boundary. It mirrors auth.RequireAuth's authenticate/error
// mapping EXCEPT:
//
//  1. It marks the request context with auth.WithMCPRequest BEFORE authenticating,
//     so the resource-URI audience check (Plan 04) is path-scoped to /mcp/ only —
//     ordinary ConnectRPC / /api/* bearer callers are never subjected to it
//     (review HIGH #2, D-08).
//  2. On the unauthenticated (non-transient) branch it emits
//     `WWW-Authenticate: Bearer resource_metadata="<metadataURL>"` alongside the
//     401, so a standard MCP client auto-discovers the authorization server(s)
//     (MCP Authorization rev 2025-06-18, D-05.2).
//
// The challenge header and the WithMCPRequest marker are present ONLY here, never
// on auth.RequireAuth or the ConnectRPC interceptor, so static-credential
// (spgr_sk_/spgr_ws_) clients on /api/* keep receiving a plain 401 (D-08,
// Pitfall 4). A malformed/absent token is treated as unauthenticated (fail-closed
// 401 with challenge); there is no distinct 400 path (D-05.4).
func RequireAuthWithChallenge(resolver auth.Resolver, metadataURL string) func(http.Handler) http.Handler {
	challenge := fmt.Sprintf("Bearer resource_metadata=%q", metadataURL)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Mark the /mcp/ boundary before authenticating so downstream
			// resolveJWT (Plan 04) can path-scope the resource-URI audience check.
			ctx := auth.WithMCPRequest(r.Context())
			id, err := auth.Authenticate(ctx, resolver, r.Header)
			if err != nil {
				switch {
				case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
					return // client gone / deadline; nothing to write
				case errors.Is(err, auth.ErrTransient):
					http.Error(w, `{"error":"transient"}`, http.StatusServiceUnavailable)
				default:
					w.Header().Set("WWW-Authenticate", challenge)
					http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
				}
				return
			}
			if info := reqctx.FromContext(r.Context()); info != nil {
				info.Identity = id.Subject
			}
			next.ServeHTTP(w, r.WithContext(auth.WithIdentity(ctx, id)))
		})
	}
}
