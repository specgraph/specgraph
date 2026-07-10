// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/config"
)

// githubOAuth2Config returns a fully-populated oauth2 provider config for the
// BuildLoginProviders tests; individual tests mutate a copy to exercise the
// startup-fatal validation branches.
func githubOAuth2Config() config.OIDCProviderConfig {
	return config.OIDCProviderConfig{ //nolint:gosec // G101: ClientSecretEnv is an env var name, not a credential
		ID: "github", Kind: "oauth2", Interactive: true, ClientID: "c",
		ClientSecretEnv: "GH_SECRET",
		AuthURL:         "https://github.com/login/oauth/authorize",
		TokenURL:        "https://github.com/login/oauth/access_token",
		UserinfoURL:     "https://api.github.com/user",
		EmailsURL:       "https://api.github.com/user/emails",
		SubjectField:    "id",
		Scopes:          []string{"read:user", "user:email"},
	}
}

func TestBuildLoginProviders_OAuth2_Constructed(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	provs, err := BuildLoginProviders(context.Background(),
		[]config.OIDCProviderConfig{githubOAuth2Config()}, "")
	require.NoError(t, err)
	require.Len(t, provs, 1)
	require.Equal(t, "github", provs[0].ID())
	require.Equal(t, "github", provs[0].DisplayName(), "display defaults to id when unset")
	if _, ok := provs[0].(*oauth2LoginProvider); !ok {
		t.Fatalf("expected *oauth2LoginProvider, got %T", provs[0])
	}
}

func TestBuildLoginProviders_OAuth2_MissingUserinfoURL(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	pc := githubOAuth2Config()
	pc.UserinfoURL = ""
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{pc}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "userinfo_url", "missing userinfo_url must be startup-fatal")
}

func TestBuildLoginProviders_OAuth2_MissingSubjectField(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	pc := githubOAuth2Config()
	pc.SubjectField = ""
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{pc}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "subject_field")
}

func TestBuildLoginProviders_OAuth2_EmailsURLWithoutEmailScope(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	pc := githubOAuth2Config()
	pc.Scopes = []string{"read:user"} // EmailsURL set but no email scope
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{pc}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "email scope",
		"emails_url without an email scope must be startup-fatal")
}

func TestBuildLoginProviders_OAuth2_IssuerFromProviderIssuer(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	pc := githubOAuth2Config()
	provs, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{pc}, "")
	require.NoError(t, err)
	op, ok := provs[0].(*oauth2LoginProvider)
	require.True(t, ok)
	require.Equal(t, config.ProviderIssuer(pc), op.issuerID,
		"issuerID must be set from config.ProviderIssuer (single canonical issuer, HIGH #1)")
}

func TestBuildLoginProviders_UnsupportedKind(t *testing.T) {
	t.Setenv("GH_SECRET", "shh")
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{
		{ID: "weird", Kind: "saml", Interactive: true, ClientID: "c", ClientSecretEnv: "GH_SECRET"},
	}, "")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported kind")
}

// TestOAuth2ClaimsMapping_KeyedByProviderIssuer proves D-04 reuse AND the
// review HIGH #1 runtime-key alignment: the userinfo Raw drives a
// claims_mapping role match when the mapping map is keyed by
// config.ProviderIssuer(pc) — the SAME value the oauth2 provider stamps onto
// claims.Issuer, which the runtime role lookup (jitClaimsMapping[claims.Issuer])
// keys by. No mapping code changes are needed.
func TestOAuth2ClaimsMapping_KeyedByProviderIssuer(t *testing.T) {
	pc := config.OIDCProviderConfig{ID: "github", Kind: "oauth2"}
	issuer := config.ProviderIssuer(pc)

	// The startup builder keys the map by ProviderIssuer; the runtime looks up
	// by claims.Issuer. Both are `issuer`, so the lookup hits.
	mappingByIssuer := map[string][]config.ClaimMapping{
		issuer: {{Claim: "role", Value: "platform-admin", Role: "admin"}},
	}
	userinfo := map[string]json.RawMessage{
		"id":   json.RawMessage(`583231`),
		"role": json.RawMessage(`"platform-admin"`),
	}

	claimsIssuer := issuer // stamped by oauth2LoginProvider.Exchange
	role := applyClaimsMapping(userinfo, mappingByIssuer[claimsIssuer])
	require.Equal(t, "admin", role, "userinfo Raw must drive a claims_mapping role keyed by ProviderIssuer")
}

func TestBuildLoginProviders_SkipsNonInteractive(t *testing.T) {
	provs, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{
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
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{
		{ID: "entra", Interactive: true, Issuer: "https://x", ClientID: "c"},
	}, "")
	if err == nil || !strings.Contains(err.Error(), "client secret") {
		t.Fatalf("expected missing-secret error, got %v", err)
	}
}

func TestBuildLoginProviders_AudienceMismatch(t *testing.T) {
	t.Setenv("TEST_SECRET", "shh")
	_, err := BuildLoginProviders(context.Background(), []config.OIDCProviderConfig{
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
	if _, err := resolveClientSecret(config.OIDCProviderConfig{ClientSecretEnv: "UNSET_VAR_XYZ"}); err == nil { //nolint:gosec // G101: env var name, not a credential
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
