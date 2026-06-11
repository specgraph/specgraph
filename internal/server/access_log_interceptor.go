// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/reqctx"
)

// connectCode returns the access-log code string for an RPC outcome.
// connect.CodeOf(nil) is Unknown, so nil is special-cased to "ok" first; a
// plain (non-connect) handler error yields "unknown" -> ERROR, the desired
// unexpected-failure signal.
func connectCode(err error) string {
	if err == nil {
		return "ok"
	}
	return connect.CodeOf(err).String()
}

// AccessLogInterceptor enriches the reqctx carrier with the RPC procedure and
// outcome code. It never logs - the edge AccessLog middleware emits the line.
// It MUST be registered OUTERMOST (first in the interceptor list) so it runs
// even when an inner interceptor (auth) rejects without calling next, and so
// it observes the final outcome code.
func AccessLogInterceptor() connect.Interceptor {
	return accessLogInterceptor{}
}

type accessLogInterceptor struct{}

func (accessLogInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		resp, err := next(ctx, req)
		if info := reqctx.FromContext(ctx); info != nil {
			info.Procedure = req.Spec().Procedure
			info.Code = connectCode(err)
		}
		return resp, err
	}
}

func (accessLogInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next // server process only; no client enrichment
}

func (accessLogInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		err := next(ctx, conn)
		if info := reqctx.FromContext(ctx); info != nil {
			info.Procedure = conn.Spec().Procedure
			info.Code = connectCode(err)
		}
		return err
	}
}
