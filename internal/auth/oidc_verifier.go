// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/config"
)

// OIDCClaims is the verified claim payload returned by OIDCVerifier.Verify.
// Subject and Email are unmarshaled for convenience; Raw retains all claims
// for downstream ClaimsMapping evaluation at JIT time.
type OIDCClaims struct {
	Issuer  string
	Subject string
	Email   string
	Raw     map[string]json.RawMessage
}

// OIDCVerifier verifies JWTs against a single OIDC provider. It has no DB
// dependency and performs no role computation or Identity construction — the
// resolver materializes the Identity from claims; the verifier just validates
// signature/audience/expiry.
type OIDCVerifier struct {
	providerID string
	issuer     string
	verifier   *oidc.IDTokenVerifier
}

// NewOIDCVerifier discovers the OIDC provider configuration and builds a
// JWT verifier. ctx should carry a startup deadline (e.g., 10s).
func NewOIDCVerifier(ctx context.Context, cfg config.OIDCProviderConfig) (*OIDCVerifier, error) { //nolint:gocritic // hugeParam: cfg is read-only; pointer would require changing all call sites
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover OIDC provider %s: %w", cfg.ID, err)
	}
	audience := cfg.Audience
	if audience == "" {
		audience = cfg.ClientID
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: audience})
	return &OIDCVerifier{
		providerID: cfg.ID,
		issuer:     cfg.Issuer,
		verifier:   verifier,
	}, nil
}

// Issuer returns the OIDC issuer URL.
func (v *OIDCVerifier) Issuer() string { return v.issuer }

// ProviderID returns the configured provider ID (used in logs).
func (v *OIDCVerifier) ProviderID() string { return v.providerID }

// Verify validates the token's signature, audience, and expiry. On success
// returns the decoded claims; on failure returns a wrapped error.
//
// The resolver maps any error from this function to ErrUnauthenticated;
// callers should not try to distinguish verification failure modes.
func (v *OIDCVerifier) Verify(ctx context.Context, rawToken string) (*OIDCClaims, error) {
	idToken, err := v.verifier.Verify(ctx, rawToken)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelWarn, "auth: OIDC token verification failed",
			slog.String("provider", v.providerID), slog.Any("error", err))
		return nil, fmt.Errorf("oidc verify: %w", err)
	}
	var raw map[string]json.RawMessage
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("extract claims: %w", err)
	}
	c := &OIDCClaims{
		Issuer:  idToken.Issuer,
		Subject: idToken.Subject,
		Raw:     raw,
	}
	if rawEmail, ok := raw["email"]; ok {
		var email string
		if jsonErr := json.Unmarshal(rawEmail, &email); jsonErr == nil {
			c.Email = email
		}
	}
	return c, nil
}
