// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package reqctx carries request-scoped log fields that are produced by
// different layers (HTTP edge middleware, Connect interceptor, auth) and
// consumed by the access-log middleware. It is a leaf package (imports only
// context) so both internal/server and internal/auth can use it without an
// import cycle.
package reqctx

import "context"

// RequestInfo holds fields the access-log line cannot otherwise obtain at the
// HTTP edge: the RPC procedure, the connect outcome code, and the
// authenticated principal. Fields are exported because separate packages set
// them. A pointer is seeded into the context by NewContext; inner layers
// mutate it in place, and the seeder reads it after the handler returns.
type RequestInfo struct {
	Procedure string // RPC procedure, e.g. /specgraph.v1.GraphService/AddEdge
	Code      string // connect code string: "ok", "invalid_argument", ...
	Identity  string // authenticated principal (Identity.Subject), or ""
}

type ctxKey struct{}

// NewContext returns a context carrying a fresh *RequestInfo plus that pointer.
func NewContext(ctx context.Context) (context.Context, *RequestInfo) {
	info := &RequestInfo{}
	return context.WithValue(ctx, ctxKey{}, info), info
}

// FromContext returns the *RequestInfo carried by ctx, or nil if none.
func FromContext(ctx context.Context) *RequestInfo {
	info, ok := ctx.Value(ctxKey{}).(*RequestInfo)
	if !ok {
		return nil
	}
	return info
}
