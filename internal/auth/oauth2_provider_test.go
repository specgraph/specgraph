// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/config"
)

// oauth2StubServer wires a token endpoint plus GitHub-shaped /user and
// /user/emails endpoints so Exchange can be exercised end-to-end without a
// live IdP.
func oauth2StubServer(t *testing.T, userinfo, emails string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"test-access-token","token_type":"bearer"}`))
	})
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-access-token" {
			t.Errorf("userinfo Authorization header = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(userinfo))
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emails))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newOAuth2TestProvider(srv *httptest.Server) *oauth2LoginProvider {
	pc := config.OIDCProviderConfig{ID: "github", Kind: "oauth2"}
	return &oauth2LoginProvider{
		id:           "github",
		displayName:  "GitHub",
		authURL:      srv.URL + "/authorize",
		tokenURL:     srv.URL + "/token",
		userinfoURL:  srv.URL + "/user",
		emailsURL:    srv.URL + "/user/emails",
		clientID:     "client-1",
		secret:       "shh",
		scopes:       []string{"read:user", "user:email"},
		subjectField: "id",
		emailField:   "email",
		issuerID:     config.ProviderIssuer(pc),
		httpClient:   srv.Client(),
	}
}

// TestOAuth2Provider_Exchange_VerifiedEmailFallback proves the primary path:
// a numeric userinfo id is stringified into the stable Subject, Issuer is the
// configured ProviderIssuer value, and a null userinfo email falls back to the
// primary&&verified entry from /user/emails (D-02).
func TestOAuth2Provider_Exchange_VerifiedEmailFallback(t *testing.T) {
	srv := oauth2StubServer(t,
		`{"id":583231,"login":"octocat","name":"The Octocat","email":null}`,
		`[{"email":"other@example.com","primary":false,"verified":true},
		  {"email":"octo@example.com","primary":true,"verified":true}]`,
	)
	p := newOAuth2TestProvider(srv)

	claims, err := p.Exchange(context.Background(), "code-xyz", "verifier-abc", "", srv.URL+"/callback")
	require.NoError(t, err)
	require.Equal(t, "583231", claims.Subject, "numeric id must be stringified as the stable subject")
	require.Equal(t, config.ProviderIssuer(config.OIDCProviderConfig{ID: "github", Kind: "oauth2"}), claims.Issuer,
		"Issuer must equal the configured ProviderIssuer value")
	require.Equal(t, "octo@example.com", claims.Email, "must fall back to the primary&&verified email")
	require.Contains(t, claims.Raw, "login", "Raw must carry the userinfo fields for claims_mapping")
	require.Equal(t, "The Octocat", claims.Name)
}

// TestOAuth2Provider_Exchange_UnverifiedEmailBlank proves an unverified or
// absent verified-primary email yields a blank Email — never trusting an
// unverified address (D-02).
func TestOAuth2Provider_Exchange_UnverifiedEmailBlank(t *testing.T) {
	srv := oauth2StubServer(t,
		`{"id":583231,"login":"octocat","email":null}`,
		`[{"email":"unverified@example.com","primary":true,"verified":false}]`,
	)
	p := newOAuth2TestProvider(srv)

	claims, err := p.Exchange(context.Background(), "code-xyz", "verifier-abc", "", srv.URL+"/callback")
	require.NoError(t, err)
	require.Equal(t, "583231", claims.Subject)
	require.Empty(t, claims.Email, "an unverified email must be treated as blank")
}

// TestOAuth2Provider_Exchange_PopulatesName is a regression guard for D-05's
// "already correct" research finding: Exchange must populate OIDCClaims.Name
// from the userinfo "name" field via displayNameFromUserinfo, with no
// production change required in this plan.
func TestOAuth2Provider_Exchange_PopulatesName(t *testing.T) {
	srv := oauth2StubServer(t,
		`{"id":583231,"login":"octocat","name":"The Octocat","email":null}`,
		`[{"email":"octo@example.com","primary":true,"verified":true}]`,
	)
	p := newOAuth2TestProvider(srv)

	claims, err := p.Exchange(context.Background(), "code-xyz", "verifier-abc", "", srv.URL+"/callback")
	require.NoError(t, err)
	require.NotEmpty(t, claims.Name, "Exchange must populate Name from userinfo")
	require.Equal(t, "The Octocat", claims.Name)
}

// TestOAuth2Provider_AuthCodeURL proves the authorize URL carries state + PKCE
// S256 but NO nonce (there is no id_token to bind a nonce to).
func TestOAuth2Provider_AuthCodeURL(t *testing.T) {
	p := &oauth2LoginProvider{
		id:       "github",
		authURL:  "https://github.com/login/oauth/authorize",
		clientID: "client-1",
		scopes:   []string{"read:user", "user:email"},
	}
	got := p.AuthCodeURL("STATE", "NONCE", "CHALLENGE", "https://app/api/auth/oidc/callback")
	for _, want := range []string{
		"https://github.com/login/oauth/authorize?", "client_id=client-1", "state=STATE",
		"code_challenge=CHALLENGE", "code_challenge_method=S256", "response_type=code",
	} {
		require.Contains(t, got, want)
	}
	require.NotContains(t, got, "nonce", "oauth2 AuthCodeURL must not include a nonce")
}

// TestOAuth2Provider_Exchange_MissingSubjectFatal proves a userinfo response
// with no subject field fails closed.
func TestOAuth2Provider_Exchange_MissingSubjectFatal(t *testing.T) {
	srv := oauth2StubServer(t, `{"login":"octocat","email":"octo@example.com"}`, `[]`)
	p := newOAuth2TestProvider(srv)

	_, err := p.Exchange(context.Background(), "code-xyz", "verifier-abc", "", srv.URL+"/callback")
	require.Error(t, err, "missing subject field must fail closed")
}
