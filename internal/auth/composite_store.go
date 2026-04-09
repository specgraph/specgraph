// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// CompositeStore routes authentication to ConfigStore (API keys) or OIDCStore (JWTs).
// It implements IdentityStore and encapsulates all routing logic.
type CompositeStore struct {
	config    *ConfigStore
	oidc      []*OIDCStore
	issuerMap map[string]*OIDCStore
	mode      string
}

// NewCompositeStore creates a CompositeStore wrapping the given stores.
// mode must be "local", "oidc", or "mixed".
// Returns an error if multiple providers share the same issuer URL.
func NewCompositeStore(config *ConfigStore, oidc []*OIDCStore, mode string) (*CompositeStore, error) {
	issuerMap := make(map[string]*OIDCStore, len(oidc))
	for _, s := range oidc {
		if existing, ok := issuerMap[s.Issuer()]; ok {
			return nil, fmt.Errorf("duplicate OIDC issuer %q: providers %q and %q share the same issuer URL",
				s.Issuer(), existing.providerID, s.providerID)
		}
		issuerMap[s.Issuer()] = s
	}
	return &CompositeStore{
		config:    config,
		oidc:      oidc,
		issuerMap: issuerMap,
		mode:      mode,
	}, nil
}

// ResolveAPIKey tries the ConfigStore first. On ErrUnknownKey, if the token
// is JWT-shaped and mode allows OIDC, delegates to ResolveJWT.
func (s *CompositeStore) ResolveAPIKey(ctx context.Context, token string) (*Identity, error) {
	id, err := s.config.ResolveAPIKey(ctx, token)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, ErrUnknownKey) {
		return nil, err
	}

	// In local mode, no OIDC routing.
	if s.mode == "local" {
		return nil, ErrUnknownKey
	}

	// Check if token looks like a JWT (3 dot-separated segments).
	if strings.Count(token, ".") != 2 {
		return nil, ErrUnknownKey
	}

	id, jwtErr := s.ResolveJWT(ctx, token)
	if jwtErr != nil {
		// Map OIDC-specific errors to ErrUnknownKey for the interceptor (→ 401).
		if errors.Is(jwtErr, ErrUnknownIssuer) || errors.Is(jwtErr, ErrInvalidToken) {
			return nil, ErrUnknownKey
		}
		return nil, jwtErr
	}
	return id, nil
}

// ResolveJWT peeks at the unverified issuer claim and routes to the matching OIDCStore.
func (s *CompositeStore) ResolveJWT(ctx context.Context, token string) (*Identity, error) {
	issuer, err := peekIssuer(token)
	if err != nil {
		slog.Warn("auth: malformed JWT, cannot extract issuer", "error", err.Error())
		return nil, ErrInvalidToken
	}

	store, ok := s.issuerMap[issuer]
	if !ok {
		slog.Warn("auth: unknown JWT issuer", "issuer", issuer)
		return nil, ErrUnknownIssuer
	}

	return store.ResolveJWT(ctx, token)
}

// HasAuth reports whether any authentication mechanism is configured.
func (s *CompositeStore) HasAuth() bool {
	return s.config.HasAuth() || len(s.oidc) > 0
}

// peekIssuer extracts the "iss" claim from the JWT payload without verification.
// This is safe because it's only used for routing — the actual verification
// happens in the OIDCStore after routing.
func peekIssuer(token string) (string, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return "", errors.New("not a JWT")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode payload: %w", err)
	}

	var claims struct {
		Issuer string `json:"iss"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("unmarshal payload: %w", err)
	}
	if claims.Issuer == "" {
		return "", errors.New("missing iss claim")
	}
	return claims.Issuer, nil
}
