// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os/user"
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
				return nil, connect.NewError(connect.CodeInternal, nil)
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

	if authHeader == "" {
		if store.AllowUnauthenticated() {
			return localIdentity(), nil
		}
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// Parse "Bearer <token>" — scheme is case-insensitive per RFC 7235.
	scheme, token, ok := strings.Cut(authHeader, " ")
	token = strings.TrimSpace(token)
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}

	// ResolveAPIKey handles all token routing:
	// - API keys matched directly by ConfigStore
	// - JWT-shaped tokens delegated to OIDCStore via CompositeStore
	id, err := store.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	if errors.Is(err, ErrUnknownKey) {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	// Non-ErrUnknownKey failures (I/O, store outage) are internal errors.
	return nil, connect.NewError(connect.CodeInternal, nil)
}

func localIdentity() *Identity {
	username := "unknown"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	return &Identity{
		Subject:     "local:" + username,
		DisplayName: username,
		Role:        RoleAdmin,
		Permissions: map[string]bool{"*:*": true},
		Source:      "local",
	}
}
