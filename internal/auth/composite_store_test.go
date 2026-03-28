// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
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

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
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

	cs := auth.NewCompositeStore(cfgStore, nil, "local")
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

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
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

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")

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

func TestCompositeStore_AllowUnauthenticated_MixedMode(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs := auth.NewCompositeStore(cfgStore, nil, "mixed")
	if !cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = false, want true in mixed mode")
	}
}

func TestCompositeStore_AllowUnauthenticated_OIDCMode(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs := auth.NewCompositeStore(cfgStore, nil, "oidc")
	if cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = true, want false in oidc mode")
	}
}

func TestCompositeStore_AllowUnauthenticated_LocalNoKeys(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	if !cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = false, want true in local mode with no keys")
	}
}

func TestCompositeStore_AllowUnauthenticated_LocalWithKeys(t *testing.T) {
	cfgStore, err := auth.NewConfigStore(config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_test", Name: "Test", Role: "admin"},
		},
	}, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	cs := auth.NewCompositeStore(cfgStore, nil, "local")
	if cs.AllowUnauthenticated() {
		t.Error("AllowUnauthenticated() = true, want false in local mode with keys")
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

	cs := auth.NewCompositeStore(cfgStore, []*auth.OIDCStore{oidcStore}, "oidc")
	if !cs.HasAuth() {
		t.Error("HasAuth() = false, want true with OIDC providers")
	}
}
