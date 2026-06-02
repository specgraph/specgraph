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

// NewAuthInterceptor returns a ConnectRPC unary interceptor that authenticates
// requests using the provided IdentityStore. Exempt procedures (e.g., Health)
// bypass authentication entirely.
func NewAuthInterceptor(store IdentityStore) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure

			if IsExempt(procedure) {
				return next(ctx, req)
			}

			id, authErr := resolveIdentity(ctx, store, req.Header())
			if authErr != nil {
				slog.Warn("auth: authentication failed",
					"procedure", procedure,
					"error", authErr.Error(),
				)
				return nil, authErr
			}

			required, ok := RPCPermission(procedure)
			if !ok {
				slog.Error("auth: unconfigured RPC permission",
					"procedure", procedure,
				)
				return nil, connect.NewError(connect.CodeInternal, errors.New("unconfigured RPC permission"))
			}

			if !HasPermission(id.Permissions, required) {
				slog.Warn("auth: permission denied",
					"subject", id.Subject,
					"procedure", procedure,
					"required", required,
				)
				return nil, connect.NewError(connect.CodePermissionDenied, nil)
			}

			slog.Info("auth: authenticated",
				"subject", id.Subject,
				"procedure", procedure,
			)
			return next(WithIdentity(ctx, id), req)
		}
	}
}

func resolveIdentity(ctx context.Context, store IdentityStore, headers http.Header) (*Identity, error) {
	authHeader := headers.Get("Authorization")
	if authHeader != "" {
		scheme, token, ok := strings.Cut(authHeader, " ")
		token = strings.TrimSpace(token)
		if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, nil)
		}
		return resolveToken(ctx, store, token)
	}

	// Fallback: session cookie.
	r := &http.Request{Header: headers}
	cookie, err := r.Cookie("specgraph_session")
	if err == nil && cookie.Value != "" {
		return resolveToken(ctx, store, cookie.Value)
	}

	return nil, connect.NewError(connect.CodeUnauthenticated, nil)
}

func resolveToken(ctx context.Context, store IdentityStore, token string) (*Identity, error) {
	id, err := store.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, ErrUnknownKey) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return nil, connect.NewError(connect.CodeInternal, nil)
}

// NewAuthInterceptorV2 returns a ConnectRPC unary interceptor that
// authenticates and authorizes requests using the supplied Resolver and
// Authorizer. Exempt procedures (Health) bypass both.
//
// Named "V2" temporarily during the Phase B cutover. After serve.go
// switches to this constructor (Task 29) and the legacy NewAuthInterceptor
// is removed, this function will be renamed back to NewAuthInterceptor
// in the cleanup task.
func NewAuthInterceptorV2(resolver Resolver, authorizer Authorizer) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			procedure := req.Spec().Procedure
			if IsExempt(procedure) {
				return next(ctx, req)
			}

			id, err := authenticateV2(ctx, resolver, req.Header())
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

// authenticateV2 extracts the bearer token (Authorization header or cookie
// fallback) and resolves it. Returns ErrUnauthenticated on missing token.
// V2-suffixed during Phase A (legacy middleware.go has a different-signature
// `authenticate`); renamed to `authenticate` in Task 30b.
func authenticateV2(ctx context.Context, resolver Resolver, headers http.Header) (*Identity, error) {
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
