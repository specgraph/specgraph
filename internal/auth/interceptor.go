// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"

	"github.com/specgraph/specgraph/internal/reqctx"
)

// NewAuthInterceptor returns a ConnectRPC unary interceptor that
// authenticates and authorizes requests using the supplied Resolver and
// Authorizer. Exempt procedures (Health) bypass both.
func NewAuthInterceptor(resolver Resolver, authorizer Authorizer) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure
			if IsExempt(procedure) {
				return next(ctx, req)
			}

			id, err := authenticate(ctx, resolver, req.Header())
			if err != nil {
				return nil, mapAuthError(procedure, err)
			}

			if info := reqctx.FromContext(ctx); info != nil {
				info.Identity = id.Subject
			}

			decision, err := authorizer.Authorize(ctx, id, procedure, req.Any())
			if err != nil {
				slog.LogAttrs(ctx, slog.LevelError, "auth: authorizer error",
					slog.String("procedure", procedure), slog.Any("error", err))
				return nil, connect.NewError(connect.CodeInternal, nil)
			}
			if !decision.Allowed {
				slog.LogAttrs(ctx, slog.LevelWarn, "auth: permission denied",
					slog.String("subject", id.Subject), slog.String("procedure", procedure), slog.String("reason", decision.Reason))
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			slog.LogAttrs(ctx, slog.LevelInfo, "auth: authenticated",
				slog.String("subject", id.Subject), slog.String("procedure", procedure))
			return next(WithIdentity(ctx, id), req)
		}
	}
}

// Authenticate extracts the bearer token (Authorization header or session-cookie
// fallback) from headers and resolves it via the resolver, applying the same
// missing-token → ErrUnauthenticated discipline as RequireAuth. It is exported so
// the server-package /mcp/ challenge wrapper (RequireAuthWithChallenge) reuses the
// exact token-extraction + resolution path rather than duplicating it, keeping the
// bare-401 and challenge-401 paths byte-identical in how they authenticate.
func Authenticate(ctx context.Context, resolver Resolver, headers http.Header) (*Identity, error) {
	return authenticate(ctx, resolver, headers)
}

// authenticate extracts the bearer token (Authorization header or cookie
// fallback) and resolves it. Returns ErrUnauthenticated on missing token.
func authenticate(ctx context.Context, resolver Resolver, headers http.Header) (*Identity, error) {
	token := extractBearerToken(headers)
	if token == "" {
		token = sessionCookieValue(headers) // dashboard fallback
	}
	if token == "" {
		return nil, ErrUnauthenticated
	}
	return resolver.Resolve(ctx, token) //nolint:wrapcheck // Resolver errors (ErrUnauthenticated, ErrTransient) are defined in this package; wrapping adds no context
}

// sessionCookieValue reads the specgraph_session cookie from raw headers.
// The standard library only exposes cookie parsing via *http.Request, so
// we wrap the headers in a throwaway request to reuse net/http's RFC-6265
// parser rather than hand-rolling cookie splitting. Isolated here so the
// idiom is documented in exactly one place.
func sessionCookieValue(headers http.Header) string {
	r := &http.Request{Header: headers}
	c, err := r.Cookie("specgraph_session")
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

// extractBearerToken extracts the token from an Authorization: Bearer header.
func extractBearerToken(headers http.Header) string {
	authHeader := headers.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	scheme, token, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}

// mapAuthError maps Resolver / authentication errors to connect error codes.
// ErrUnauthenticated → CodeUnauthenticated; ErrTransient → CodeUnavailable;
// context errors → CodeCanceled / CodeDeadlineExceeded; else → CodeInternal.
func mapAuthError(procedure string, err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "auth: request canceled",
			slog.String("procedure", procedure))
		return connect.NewError(connect.CodeCanceled, nil)
	case errors.Is(err, context.DeadlineExceeded):
		slog.LogAttrs(context.Background(), slog.LevelDebug, "auth: request deadline exceeded",
			slog.String("procedure", procedure))
		return connect.NewError(connect.CodeDeadlineExceeded, nil)
	case errors.Is(err, ErrTransient):
		slog.LogAttrs(context.Background(), slog.LevelWarn, "auth: transient backend error",
			slog.String("procedure", procedure), slog.Any("error", err))
		return connect.NewError(connect.CodeUnavailable, nil)
	case errors.Is(err, ErrUnauthenticated):
		slog.LogAttrs(context.Background(), slog.LevelInfo, "auth: unauthenticated",
			slog.String("procedure", procedure))
		return connect.NewError(connect.CodeUnauthenticated, nil)
	default:
		slog.LogAttrs(context.Background(), slog.LevelError, "auth: unexpected error category",
			slog.String("procedure", procedure), slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, nil)
	}
}
