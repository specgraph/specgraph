// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// oauth2LoginProvider drives the interactive Authorization-Code flow for a
// plain OAuth2 identity provider that does not issue an id_token (e.g. GitHub).
// It reuses oauth2.Config + PKCE for the front channel and the code exchange,
// but replaces id_token verification with a userinfo GET and (when the primary
// userinfo email is private) a primary&&verified email fetch (D-02). The
// resulting *OIDCClaims flows through the exact same materializeIdentity →
// binding → JIT → claims_mapping machinery the OIDC provider uses (D-04).
//
// issuerID is the single canonical issuer for this provider — populated by
// BuildLoginProviders from config.ProviderIssuer(pc) so claims.Issuer, the
// (issuer,subject) binding, and the startup claims-mapping key cannot diverge
// (review HIGH #1, D-09). It is never re-derived here.
type oauth2LoginProvider struct {
	id           string
	displayName  string
	authURL      string
	tokenURL     string
	userinfoURL  string
	emailsURL    string
	clientID     string
	secret       string
	scopes       []string
	subjectField string
	emailField   string
	issuerID     string
	httpClient   *http.Client
}

func (p *oauth2LoginProvider) ID() string          { return p.id }
func (p *oauth2LoginProvider) DisplayName() string { return p.displayName }

func (p *oauth2LoginProvider) oauth2Config(redirectURI string) oauth2.Config {
	return oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.secret,
		Endpoint:     oauth2.Endpoint{AuthURL: p.authURL, TokenURL: p.tokenURL},
		RedirectURL:  redirectURI,
		Scopes:       p.scopes,
	}
}

// AuthCodeURL mirrors the oidc provider's authorize URL but WITHOUT an OIDC
// nonce param — there is no id_token to bind a nonce to, so state + PKCE S256
// remain the CSRF/interception defenses. The nonce arg is accepted for
// interface parity and deliberately ignored.
func (p *oauth2LoginProvider) AuthCodeURL(state, _ /*nonce*/, codeChallenge, redirectURI string) string {
	cfg := p.oauth2Config(redirectURI)
	return cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

// Exchange swaps the authorization code for an access token, fetches the
// userinfo profile, and materializes verified *OIDCClaims. The subject is the
// stable id selector (never a renameable username); the email is only trusted
// when it is a primary&&verified address (D-02). All outbound calls use the
// bounded-timeout client; any non-2xx or malformed body fails closed (mapped to
// ErrUnauthenticated upstream).
func (p *oauth2LoginProvider) Exchange(ctx context.Context, code, codeVerifier, _ /*nonce*/, redirectURI string) (*OIDCClaims, error) {
	cfg := p.oauth2Config(redirectURI)
	// Route the token exchange through the bounded client.
	exCtx := context.WithValue(ctx, oauth2.HTTPClient, p.httpClient)
	tok, err := cfg.Exchange(exCtx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange: %w", err)
	}
	userinfo, err := p.fetchUserinfo(ctx, tok.AccessToken)
	if err != nil {
		return nil, err
	}

	subject, err := selectStringField(userinfo, p.subjectField)
	if err != nil {
		return nil, fmt.Errorf("oauth2 exchange: subject: %w", err)
	}
	if subject == "" {
		return nil, errors.New("oauth2 exchange: empty subject")
	}

	email := ""
	if p.emailField != "" {
		// Best-effort: a private/null userinfo email yields "" (no error).
		email, _ = selectStringField(userinfo, p.emailField)
	}
	if email == "" && p.emailsURL != "" {
		email, err = p.fetchPrimaryVerifiedEmail(ctx, tok.AccessToken)
		if err != nil {
			return nil, err
		}
	}

	return &OIDCClaims{
		Issuer:  p.issuerID,
		Subject: subject,
		Email:   email,
		Name:    displayNameFromUserinfo(userinfo),
		Raw:     userinfo,
	}, nil
}

// fetchUserinfo GETs the userinfo endpoint with the bearer access token and
// decodes the JSON profile into a raw claim map (the shape claims_mapping
// consumes). Non-2xx or malformed bodies fail closed.
func (p *oauth2LoginProvider) fetchUserinfo(ctx context.Context, accessToken string) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := p.getJSON(ctx, p.userinfoURL, accessToken, &m); err != nil {
		return nil, fmt.Errorf("oauth2 userinfo: %w", err)
	}
	return m, nil
}

// fetchPrimaryVerifiedEmail GETs the secondary emails endpoint and returns the
// entry that is both primary and verified. When none qualifies, it returns a
// blank email (not an error) — an unverified address is never trusted (D-02).
// A transport/non-2xx/malformed failure fails closed.
func (p *oauth2LoginProvider) fetchPrimaryVerifiedEmail(ctx context.Context, accessToken string) (string, error) {
	var entries []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := p.getJSON(ctx, p.emailsURL, accessToken, &entries); err != nil {
		return "", fmt.Errorf("oauth2 emails: %w", err)
	}
	for _, e := range entries {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}

// getJSON performs a bounded, bearer-authenticated GET and decodes the JSON
// body into out. It fails closed on transport error, non-2xx status, or a
// malformed body.
func (p *oauth2LoginProvider) getJSON(ctx context.Context, url, accessToken string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

// selectStringField reads field from a userinfo map, stringifying a JSON
// number (e.g. GitHub's numeric id) or returning a JSON string as-is. A JSON
// null yields ("", nil) so a private email falls through to the verified-email
// fallback. An absent field or a non-scalar value is an error.
func selectStringField(userinfo map[string]json.RawMessage, field string) (string, error) {
	raw, ok := userinfo[field]
	if !ok {
		return "", fmt.Errorf("field %q absent", field)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String(), nil
	}
	return "", fmt.Errorf("field %q is not a string or number", field)
}

// displayNameFromUserinfo resolves a human-friendly display name from the
// userinfo body, preferring "name" and falling back to "login". Returns "" when
// neither is a non-empty string.
func displayNameFromUserinfo(userinfo map[string]json.RawMessage) string {
	for _, field := range []string{"name", "login"} {
		raw, ok := userinfo[field]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s
		}
	}
	return ""
}
