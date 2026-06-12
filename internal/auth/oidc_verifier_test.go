// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// oidcTestIssuer spins up an httptest server serving OIDC discovery + JWKS.
// Tests mint JWTs signed with its private key.
type oidcTestIssuer struct {
	server *httptest.Server
	key    *rsa.PrivateKey
}

func newOIDCTestIssuer(t *testing.T) *oidcTestIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   srv.URL,
			"jwks_uri": srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "k1", Algorithm: string(jose.RS256), Use: "sig"}
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &oidcTestIssuer{server: srv, key: key}
}

func (p *oidcTestIssuer) mintToken(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"),
	)
	require.NoError(t, err)
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	require.NoError(t, err)
	return raw
}

func TestOIDCVerifier_VerifyValidToken(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss":   p.server.URL,
		"sub":   "user-123",
		"aud":   "aud-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": "alice@example.com",
	})
	claims, err := v.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "user-123", claims.Subject)
	require.Equal(t, "alice@example.com", claims.Email)
}

// TestOIDCVerifier_PreferredUsernameFallback proves that when an access token
// omits the email claim (the common Microsoft Entra v2.0 configuration), the
// verifier falls back to preferred_username, which carries the user's UPN in
// email format. Without this fallback, OIDCClaims.Email is empty and JIT user
// creation fails at the email-domain allowlist check.
func TestOIDCVerifier_PreferredUsernameFallback(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss":                p.server.URL,
		"sub":                "user-123",
		"aud":                "aud-1",
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"preferred_username": "bob@example.com",
	})
	claims, err := v.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "bob@example.com", claims.Email)
}

// TestOIDCVerifier_EmailTakesPrecedenceOverPreferredUsername proves that when
// both claims are present, the authoritative email claim wins. This preserves
// standard OIDC correctness while still handling Entra's email-less tokens.
func TestOIDCVerifier_EmailTakesPrecedenceOverPreferredUsername(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss":                p.server.URL,
		"sub":                "user-123",
		"aud":                "aud-1",
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"email":              "authoritative@example.com",
		"preferred_username": "fallback@example.com",
	})
	claims, err := v.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "authoritative@example.com", claims.Email)
}

// TestOIDCVerifier_EmptyEmailFallsThroughToPreferredUsername proves that an
// empty-string email claim does not shadow a usable preferred_username. Entra
// can emit an empty email when the directory mail attribute is unset.
func TestOIDCVerifier_EmptyEmailFallsThroughToPreferredUsername(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss":                p.server.URL,
		"sub":                "user-123",
		"aud":                "aud-1",
		"exp":                time.Now().Add(time.Hour).Unix(),
		"iat":                time.Now().Unix(),
		"email":              "",
		"preferred_username": "fallback@example.com",
	})
	claims, err := v.Verify(ctx, token)
	require.NoError(t, err)
	require.Equal(t, "fallback@example.com", claims.Email)
}

func TestOIDCVerifier_RejectsBadAudience(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "expected",
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "wrong",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = v.Verify(ctx, token)
	require.Error(t, err)
	require.ErrorContains(t, err, "expected audience")
}

func TestOIDCVerifier_RejectsExpired(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "aud-1",
		"exp": time.Now().Add(-time.Hour).Unix(), "iat": time.Now().Add(-2 * time.Hour).Unix(),
	})
	_, err = v.Verify(ctx, token)
	require.Error(t, err)
	require.ErrorContains(t, err, "token is expired")
}

func TestOIDCVerifier_RejectsMalformed(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	_, err = v.Verify(ctx, "not.a.jwt")
	require.Error(t, err)
	require.ErrorContains(t, err, "oidc verify: oidc: malformed jwt")
}

// TestOIDCVerifier_ExplicitAudienceOverridesClientID proves the audience
// precedence in NewOIDCVerifier: when cfg.Audience is set it is the expected
// audience, NOT cfg.ClientID. A token whose aud matches Audience verifies;
// one matching only ClientID is rejected. Guards against an aud/client_id swap.
func TestOIDCVerifier_ExplicitAudienceOverridesClientID(t *testing.T) {
	ctx := context.Background()
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "the-client", Audience: "explicit-aud",
	})
	require.NoError(t, err)

	// aud == Audience → verifies.
	okToken := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "explicit-aud",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = v.Verify(ctx, okToken)
	require.NoError(t, err, "token whose aud matches the explicit Audience must verify")

	// aud == ClientID (but != Audience) → rejected, proving Audience wins.
	clientIDToken := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "the-client",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = v.Verify(ctx, clientIDToken)
	require.Error(t, err, "ClientID must not be accepted when an explicit Audience is configured")
	require.ErrorContains(t, err, "expected audience")
}

// TestOIDCVerifier_DiscoveryFailure asserts NewOIDCVerifier surfaces a wrapped
// error (rather than a nil verifier) when the issuer's discovery endpoint is
// unreachable/absent. The error is wrapped with the provider ID for operator
// diagnosis at startup.
func TestOIDCVerifier_DiscoveryFailure(t *testing.T) {
	ctx := context.Background()
	// A bare server with no discovery handler returns 404 for
	// /.well-known/openid-configuration, so provider discovery fails.
	srv := httptest.NewServer(http.NewServeMux())
	t.Cleanup(srv.Close)

	v, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "broken-idp", Issuer: srv.URL, ClientID: "aud-1",
	})
	require.Error(t, err)
	require.Nil(t, v, "no verifier should be returned on discovery failure")
	require.ErrorContains(t, err, "discover OIDC provider broken-idp")
}
