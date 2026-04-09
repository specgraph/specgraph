// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

func TestCompositeStore_APIKey(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs, csErr := auth.NewCompositeStore(cfgStore, nil, "local")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	id, err := cs.ResolveAPIKey(context.Background(), "spgr_sk_test")
	if err != nil {
		t.Fatalf("ResolveAPIKey: %v", err)
	}
	if id.Subject != "apikey:k1" {
		t.Errorf("subject = %q, want apikey:k1", id.Subject)
	}
}

func TestCompositeStore_LocalMode_RejectsJWT(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	cs, csErr := auth.NewCompositeStore(cfgStore, nil, "local")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	_, err = cs.ResolveAPIKey(context.Background(), "header.payload.signature")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey", err)
	}
}

func TestCompositeStore_OIDCMode_RoutesJWT(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{
		ID: "test", Issuer: srv.URL, ClientID: "test-client",
		ClaimsMapping: []config.ClaimMapping{
			{Claim: "groups", Value: "writers", Role: "writer"},
		},
	}

	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs, csErr := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	token := signToken(t, key, map[string]interface{}{
		"iss": srv.URL, "aud": "test-client", "sub": "user-oidc",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"groups": []string{"writers"},
	})

	id, err := cs.ResolveAPIKey(ctx, token)
	if err != nil {
		t.Fatalf("ResolveAPIKey (JWT route): %v", err)
	}
	if id.Source != "oidc" {
		t.Errorf("source = %q, want oidc", id.Source)
	}
	if id.Role != auth.RoleWriter {
		t.Errorf("role = %q, want writer", id.Role)
	}
}

func TestCompositeStore_UnknownIssuer(t *testing.T) {
	srv, key := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{ID: "test", Issuer: srv.URL, ClientID: "test-client"}
	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs, csErr := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}

	// Token with a different issuer — will not match any provider
	token := signToken(t, key, map[string]interface{}{
		"iss": "https://unknown-issuer.example.com", "aud": "test-client", "sub": "user-unknown",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})

	_, err = cs.ResolveAPIKey(ctx, token)
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey (unknown issuer mapped)", err)
	}
}

func TestCompositeStore_HasAuth_WithOIDCOnly(t *testing.T) {
	srv, _ := mockOIDCServer(t)
	defer srv.Close()

	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}

	providerCfg := config.OIDCProviderConfig{ID: "test", Issuer: srv.URL, ClientID: "test-client"}
	ctx := oidc.InsecureIssuerURLContext(context.Background(), srv.URL)
	oidcStore, err := auth.NewOIDCStore(ctx, providerCfg, "reader", auth.DefaultRolePermissions)
	if err != nil {
		t.Fatalf("NewOIDCStore: %v", err)
	}

	cs, csErr := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	if !cs.HasAuth() {
		t.Error("HasAuth() = false, want true with OIDC providers")
	}
}

func TestCompositeStore_MalformedJWT_BadBase64(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs, csErr := auth.NewCompositeStore(cfgStore, nil, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	// JWT with invalid base64 in payload
	_, err = cs.ResolveAPIKey(context.Background(), "header.!!!invalid-base64!!!.signature")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey for bad base64", err)
	}
}

func TestCompositeStore_MalformedJWT_MissingIss(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs, csErr := auth.NewCompositeStore(cfgStore, nil, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	// Build a JWT-shaped token with a valid base64 payload that has no "iss" claim.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	payload, _ := json.Marshal(map[string]string{"sub": "test"})
	noIssToken := header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	_, err = cs.ResolveAPIKey(context.Background(), noIssToken)
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey for missing iss", err)
	}
}

func TestCompositeStore_NonJWTToken_OIDCMode(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs, csErr := auth.NewCompositeStore(cfgStore, nil, "oidc")
	if csErr != nil {
		t.Fatalf("NewCompositeStore: %v", csErr)
	}
	// Token without dots — not JWT-shaped
	_, err = cs.ResolveAPIKey(context.Background(), "plain-api-key-no-dots")
	if !errors.Is(err, auth.ErrUnknownKey) {
		t.Errorf("error = %v, want ErrUnknownKey for non-JWT token", err)
	}
}

func TestIdentityFromContext(t *testing.T) {
	id := &auth.Identity{
		Subject:     "test:user",
		DisplayName: "Test User",
		Role:        auth.RoleAdmin,
		Source:      "test",
	}

	ctx := auth.WithIdentity(context.Background(), id)
	got, ok := auth.IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext returned false")
	}
	if got.Subject != "test:user" {
		t.Errorf("subject = %q, want test:user", got.Subject)
	}
}

func TestIdentityFromContext_Missing(t *testing.T) {
	_, ok := auth.IdentityFromContext(context.Background())
	if ok {
		t.Error("IdentityFromContext returned true for empty context")
	}
}

func TestWithIdentity_Nil(t *testing.T) {
	ctx := auth.WithIdentity(context.Background(), nil)
	_, ok := auth.IdentityFromContext(ctx)
	if ok {
		t.Error("IdentityFromContext returned true after WithIdentity(nil)")
	}
}
