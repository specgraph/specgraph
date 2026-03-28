// SPDX-License-Identifier: MIT
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

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// mockOIDCServer starts a httptest.Server that serves OIDC discovery + JWKS.
func mockOIDCServer(t *testing.T) (*httptest.Server, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		jwk := jose.JSONWebKey{Key: &key.PublicKey, KeyID: "test-key-1", Algorithm: "RS256", Use: "sig"}
		jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{jwk}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks) //nolint:errcheck // test helper
	})

	srv := httptest.NewServer(mux)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		disc := map[string]interface{}{
			"issuer":                                srv.URL,
			"jwks_uri":                              srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(disc) //nolint:errcheck // test helper
	})

	return srv, key
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims map[string]interface{}) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithHeader("kid", "test-key-1"),
	)
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	raw, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func TestOIDCStore_ValidToken(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID:       "test",
		Issuer:   srv.URL,
		ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss":    srv.URL,
		"aud":    "test-client",
		"sub":    "user-123",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"groups": []string{"admins", "users"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Subject != "oidc:user-123" {
		t.Errorf("subject = %q, want oidc:user-123", id.Subject)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin", id.Role)
	}
	if id.Source != "oidc" {
		t.Errorf("source = %q, want oidc", id.Source)
	}
}

func TestOIDCStore_DefaultRole(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID: "test", Issuer: srv.URL, ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "user-456",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"groups": []string{"users"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleReader {
		t.Errorf("role = %q, want reader (default)", id.Role)
	}
}

func TestOIDCStore_ExpiredToken(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{ID: "test", Issuer: srv.URL, ClientID: "test-client"}
	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "user-789",
		"exp": time.Now().Add(-time.Hour).Unix(), "iat": time.Now().Add(-2 * time.Hour).Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestOIDCStore_WrongAudience(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{ID: "test", Issuer: srv.URL, ClientID: "correct-client"}
	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "wrong-client", "sub": "user-000",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
}

func TestOIDCStore_StringClaim(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID: "test", Issuer: srv.URL, ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "repository_owner", Value: "specgraph", Role: "writer"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "repo-actor",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"repository_owner": "specgraph",
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleWriter {
		t.Errorf("role = %q, want writer", id.Role)
	}
}

func TestOIDCStore_FirstMatchWins(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	providerCfg := config.OIDCProviderConfig{
		ID: "test", Issuer: srv.URL, ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "admins", Role: "admin"},
			{Claim: "groups", Value: "admins", Role: "reader"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "user-first",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"groups": []string{"admins"},
	})

	id, err := store.ResolveJWT(ctx, token)
	if err != nil {
		t.Fatalf("ResolveJWT: %v", err)
	}
	if id.Role != auth.RoleAdmin {
		t.Errorf("role = %q, want admin (first match)", id.Role)
	}
}

func TestOIDCStore_BadSignature(t *testing.T) {
	srv, _ := mockOIDCServer(t)
	defer srv.Close()

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{ID: "test", Issuer: srv.URL, ClientID: "test-client"}
	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	store, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	token := signToken(t, otherKey, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "user-bad-sig",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})

	_, err = store.ResolveJWT(ctx, token)
	if err == nil {
		t.Fatal("expected error for bad signature")
	}
}
