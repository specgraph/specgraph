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
