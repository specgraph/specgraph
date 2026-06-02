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

			decision, err := authorizer.Authorize(ctx, id, procedure, req.Any())
			if err != nil {
				slog.Error("auth: authorizer error",
					"procedure", procedure, "error", err.Error())
				return nil, connect.NewError(connect.CodeInternal, nil)
			}
			if !decision.Allowed {
				slog.Warn("auth: permission denied",
					"subject", id.Subject, "procedure", procedure, "reason", decision.Reason)
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			slog.Info("auth: authenticated",
				"subject", id.Subject, "procedure", procedure)
			return next(WithIdentity(ctx, id), req)
		}
	}
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
		slog.Debug("auth: request canceled", "procedure", procedure)
		return connect.NewError(connect.CodeCanceled, nil)
	case errors.Is(err, context.DeadlineExceeded):
		slog.Debug("auth: request deadline exceeded", "procedure", procedure)
		return connect.NewError(connect.CodeDeadlineExceeded, nil)
	case errors.Is(err, ErrTransient):
		slog.Warn("auth: transient backend error", "procedure", procedure, "error", err.Error())
		return connect.NewError(connect.CodeUnavailable, nil)
	case errors.Is(err, ErrUnauthenticated):
		slog.Info("auth: unauthenticated", "procedure", procedure)
		return connect.NewError(connect.CodeUnauthenticated, nil)
	default:
		slog.Error("auth: unexpected error category", "procedure", procedure, "error", err.Error())
		return connect.NewError(connect.CodeInternal, nil)
	}
}
