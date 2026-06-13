// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"strings"
	"testing"

	"github.com/specgraph/specgraph/internal/config"
)

func TestBuildLoginProviders_SkipsNonInteractive(t *testing.T) {
	provs, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "verify-only", Issuer: "https://x", ClientID: "c"}, // interactive=false
	}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(provs) != 0 {
		t.Fatalf("non-interactive provider must not yield a login provider, got %d", len(provs))
	}
}

func TestBuildLoginProviders_MissingSecret(t *testing.T) {
	_, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "entra", Interactive: true, Issuer: "https://x", ClientID: "c"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("expected missing-secret error, got %v", err)
	}
}

func TestBuildLoginProviders_AudienceMismatch(t *testing.T) {
	t.Setenv("TEST_SECRET", "shh")
	_, err := BuildLoginProviders(nil, []config.OIDCProviderConfig{
		{ID: "entra", Interactive: true, Issuer: "https://x", ClientID: "c",
			Audience: "different", ClientSecretEnv: "TEST_SECRET"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "audience") {
		t.Fatalf("expected audience error, got %v", err)
	}
}

func TestResolveClientSecret(t *testing.T) {
	t.Setenv("MY_SECRET", "value")
	got, err := resolveClientSecret(config.OIDCProviderConfig{ClientSecretEnv: "MY_SECRET"})
	if err != nil || got != "value" {
		t.Fatalf("env resolution: got %q err %v", got, err)
	}
	got, err = resolveClientSecret(config.OIDCProviderConfig{ClientSecret: "plain"})
	if err != nil || got != "plain" {
		t.Fatalf("plaintext fallback: got %q err %v", got, err)
	}
	if _, err := resolveClientSecret(config.OIDCProviderConfig{ClientSecretEnv: "UNSET_VAR_XYZ"}); err == nil {
		t.Fatal("unset env var must error")
	}
}

func TestOIDCLoginProvider_AuthCodeURL(t *testing.T) {
	p := &oidcLoginProvider{
		id:          "entra",
		displayName: "Entra",
		authURL:     "https://idp/authorize",
		clientID:    "client-1",
		scopes:      []string{"openid", "email"},
	}
	got := p.AuthCodeURL("STATE", "NONCE", "CHALLENGE", "https://app/api/auth/oidc/callback")
	for _, want := range []string{
		"https://idp/authorize?", "client_id=client-1", "state=STATE",
		"nonce=NONCE", "code_challenge=CHALLENGE", "code_challenge_method=S256",
		"scope=openid+email", "response_type=code",
		"redirect_uri=https%3A%2F%2Fapp%2Fapi%2Fauth%2Foidc%2Fcallback",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("AuthCodeURL missing %q in %q", want, got)
		}
	}
}

func TestRedirectURI(t *testing.T) {
	if got := RedirectURI("https://app.example.com", false, "ignored", "", ""); got != "https://app.example.com/api/auth/oidc/callback" {
		t.Fatalf("override: %q", got)
	}
	if got := RedirectURI("", false, "internal:9090", "https", "app.example.com"); got != "https://app.example.com/api/auth/oidc/callback" {
		t.Fatalf("derived: %q", got)
	}
	if got := RedirectURI("", false, "localhost:9090", "", ""); got != "http://localhost:9090/api/auth/oidc/callback" {
		t.Fatalf("http: %q", got)
	}
}
