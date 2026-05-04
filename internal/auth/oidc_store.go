// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/config"
)

// OIDCStore verifies JWTs against a single OIDC provider.
type OIDCStore struct {
	providerID    string
	issuer        string
	verifier      *oidc.IDTokenVerifier
	claimsMapping []config.ClaimMapping
	defaultRole   string
	rolePerms     map[Role][]string
}

// NewOIDCStore creates an OIDCStore by discovering the provider's OIDC configuration.
// The context should include a 10-second deadline for startup.
// For test issuers using HTTP, pass oidc.InsecureIssuerURLContext.
func NewOIDCStore(ctx context.Context, cfg config.OIDCProviderConfig, defaultRole string, rolePerms map[Role][]string) (*OIDCStore, error) { //nolint:gocritic // hugeParam: cfg is read-only; pointer would require changing all call sites
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider %s: %w", cfg.ID, err)
	}

	if _, ok := rolePerms[Role(defaultRole)]; !ok {
		return nil, fmt.Errorf("OIDC provider %s: default_role %q not found in configured roles", cfg.ID, defaultRole)
	}
	for _, m := range cfg.ClaimsMapping {
		if _, ok := rolePerms[Role(m.Role)]; !ok {
			return nil, fmt.Errorf("OIDC provider %s: claims_mapping role %q not found in configured roles", cfg.ID, m.Role)
		}
	}

	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: audience,
	})

	return &OIDCStore{
		providerID:    cfg.ID,
		issuer:        cfg.Issuer,
		verifier:      verifier,
		claimsMapping: cfg.ClaimsMapping,
		defaultRole:   defaultRole,
		rolePerms:     rolePerms,
	}, nil
}

// Issuer returns the OIDC issuer URL for this store.
func (s *OIDCStore) Issuer() string {
	return s.issuer
}

// ResolveJWT verifies the token and maps claims to an Identity.
func (s *OIDCStore) ResolveJWT(ctx context.Context, rawToken string) (*Identity, error) {
	idToken, err := s.verifier.Verify(ctx, rawToken)
	if err != nil {
		slog.Warn("auth: OIDC token verification failed",
			"provider", s.providerID,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	// Extract all claims as raw JSON for flexible mapping.
	var claims map[string]json.RawMessage
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}

	role := s.mapClaims(claims)

	perms, ok := s.rolePerms[Role(role)]
	if !ok {
		slog.Warn("auth: OIDC mapped role not found, falling back to default",
			"provider", s.providerID,
			"role", role,
			"default_role", s.defaultRole,
		)
		role = s.defaultRole
		perms = s.rolePerms[Role(role)]
	}

	permMap := make(map[string]bool, len(perms))
	for _, p := range perms {
		permMap[p] = true
	}

	slog.Debug("auth: OIDC authenticated",
		"provider", s.providerID,
		"subject", idToken.Subject,
		"role", role,
	)

	return &Identity{
		Subject:     "oidc:" + idToken.Subject,
		DisplayName: idToken.Subject,
		Role:        Role(role),
		Permissions: permMap,
		Source:      "oidc",
	}, nil
}

// mapClaims evaluates claims_mapping rules in order, returning the first matching role.
// Falls back to defaultRole if no mapping matches.
func (s *OIDCStore) mapClaims(claims map[string]json.RawMessage) string {
	for _, m := range s.claimsMapping {
		raw, ok := claims[m.Claim]
		if !ok {
			continue
		}
		if matchClaimValue(raw, m.Value) {
			return m.Role
		}
	}
	return s.defaultRole
}

// matchClaimValue checks if the raw JSON claim value matches the target.
// Supports string claims ("value") and string array claims (["a","b"]).
func matchClaimValue(raw json.RawMessage, target string) bool {
	// Try string first.
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str == target
	}

	// Try string array.
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return slices.Contains(arr, target)
	}

	return false
}
