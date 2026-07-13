// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/config"
)

// LoginProvider drives the interactive OAuth2 Authorization Code flow for one
// provider. The oidc implementation verifies the returned id_token (incl.
// nonce) and returns the normalized, verified claims for the resolver to
// materialize identity via Resolver.ResolveLogin.
type LoginProvider interface {
	ID() string
	DisplayName() string
	AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string
	// Exchange swaps the authorization code for verified, normalized claims.
	// The oidc provider verifies the id_token (signature + nonce) and returns
	// the parsed *OIDCClaims; a future oauth2 provider returns claims built
	// from the userinfo response with a synthetic issuer.
	Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (claims *OIDCClaims, err error)
}

type oidcLoginProvider struct {
	id          string
	displayName string
	authURL     string
	tokenURL    string
	clientID    string
	secret      string
	scopes      []string
	verifier    *OIDCVerifier
}

func (p *oidcLoginProvider) ID() string          { return p.id }
func (p *oidcLoginProvider) DisplayName() string { return p.displayName }

func (p *oidcLoginProvider) AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string {
	cfg := p.oauth2Config(redirectURI)
	return cfg.AuthCodeURL(state,
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
	)
}

func (p *oidcLoginProvider) Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (*OIDCClaims, error) {
	cfg := p.oauth2Config(redirectURI)
	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return nil, errors.New("oidc exchange: no id_token in response")
	}
	claims, err := p.verifier.VerifyWithNonce(ctx, rawID, nonce)
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (p *oidcLoginProvider) oauth2Config(redirectURI string) oauth2.Config {
	return oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.secret,
		Endpoint:     oauth2.Endpoint{AuthURL: p.authURL, TokenURL: p.tokenURL},
		RedirectURL:  redirectURI,
		Scopes:       p.scopes,
	}
}

// resolveClientSecret returns the provider's client secret from
// ClientSecretEnv (preferred) or the plaintext ClientSecret fallback.
func resolveClientSecret(pc config.OIDCProviderConfig) (string, error) { //nolint:gocritic // hugeParam: pc is read-only; matches NewOIDCVerifier convention
	if pc.ClientSecretEnv != "" {
		v := os.Getenv(pc.ClientSecretEnv)
		if v == "" {
			return "", fmt.Errorf("client secret env var %q is unset", pc.ClientSecretEnv)
		}
		return v, nil
	}
	if pc.ClientSecret != "" {
		return pc.ClientSecret, nil
	}
	return "", errors.New("no client secret (set client_secret_env or client_secret)")
}

// defaultScopes is applied when a provider configures none.
var defaultScopes = []string{"openid", "email", "profile"}

// BuildLoginProviders discovers and constructs a LoginProvider for each
// interactive OIDC provider. Non-interactive providers are skipped. All
// failures (unknown kind, missing secret, bad audience, discovery failure)
// are fatal — the caller treats a non-nil error as a startup abort.
//
// The trailing string param is currently unused (reserved for a base URL hint);
// keep the signature stable for the serve.go call site.
func BuildLoginProviders(ctx context.Context, providers []config.OIDCProviderConfig, _ string) ([]LoginProvider, error) {
	var out []LoginProvider
	for _, pc := range providers { //nolint:gocritic // rangeValCopy: provider list is small and startup-only
		if !pc.Interactive {
			continue
		}
		kind := pc.Kind
		if kind == "" {
			kind = "oidc"
		}
		switch kind {
		case "oidc":
			prov, err := buildOIDCProvider(ctx, pc)
			if err != nil {
				return nil, err
			}
			out = append(out, prov)
		case "oauth2":
			prov, err := buildOAuth2Provider(pc)
			if err != nil {
				return nil, err
			}
			out = append(out, prov)
		default:
			return nil, fmt.Errorf("OIDC provider %s: unsupported kind %q (only \"oidc\" or \"oauth2\")", pc.ID, kind)
		}
	}
	return out, nil
}

// buildOIDCProvider constructs the standard OIDC login provider: it validates
// the audience, resolves the client secret, performs a single bounded discovery
// (issuer + endpoints + JWKS verifier), and returns an oidcLoginProvider.
func buildOIDCProvider(ctx context.Context, pc config.OIDCProviderConfig) (*oidcLoginProvider, error) { //nolint:gocritic // hugeParam: pc is read-only; matches NewOIDCVerifier convention
	if pc.Audience != "" && pc.Audience != pc.ClientID {
		return nil, fmt.Errorf("OIDC provider %s: interactive audience must be empty or equal client_id", pc.ID)
	}
	secret, err := resolveClientSecret(pc)
	if err != nil {
		return nil, fmt.Errorf("OIDC provider %s: %w", pc.ID, err)
	}
	// Single bounded discovery: NewOIDCVerifier discovers the issuer and
	// exposes Endpoint(), so we don't round-trip .well-known twice. The
	// timeout guards against a hung IdP blocking startup.
	dctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	verifier, err := NewOIDCVerifier(dctx, pc)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("OIDC provider %s discovery: %w", pc.ID, err)
	}
	scopes := pc.Scopes
	if len(scopes) == 0 {
		scopes = defaultScopes
	}
	scopes = ensureOpenID(scopes)
	display := pc.DisplayName
	if display == "" {
		display = pc.ID
	}
	ep := verifier.Endpoint()
	return &oidcLoginProvider{
		id:          pc.ID,
		displayName: display,
		authURL:     ep.AuthURL,
		tokenURL:    ep.TokenURL,
		clientID:    pc.ClientID,
		secret:      secret,
		scopes:      scopes,
		verifier:    verifier,
	}, nil
}

// buildOAuth2Provider constructs a non-OIDC oauth2 login provider (D-01). It
// skips OIDC discovery/id_token verification entirely; instead it validates the
// operator-supplied endpoints + selectors as startup-fatal (matching the oidc
// discipline) and materializes identity from the userinfo endpoint. The
// provider's issuerID is config.ProviderIssuer(pc) — the SINGLE canonical
// issuer shared by the runtime claims.Issuer, the (issuer,subject) binding, and
// the startup claims-mapping key, so they cannot diverge (review HIGH #1, D-09).
func buildOAuth2Provider(pc config.OIDCProviderConfig) (*oauth2LoginProvider, error) { //nolint:gocritic // hugeParam: pc is read-only; matches NewOIDCVerifier convention
	secret, err := resolveClientSecret(pc)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 provider %s: %w", pc.ID, err)
	}
	if pc.AuthURL == "" {
		return nil, fmt.Errorf("OAuth2 provider %s: auth_url is required", pc.ID)
	}
	if pc.TokenURL == "" {
		return nil, fmt.Errorf("OAuth2 provider %s: token_url is required", pc.ID)
	}
	if pc.UserinfoURL == "" {
		return nil, fmt.Errorf("OAuth2 provider %s: userinfo_url is required", pc.ID)
	}
	if pc.SubjectField == "" {
		return nil, fmt.Errorf("OAuth2 provider %s: subject_field is required", pc.ID)
	}
	// When a secondary emails endpoint is configured, the verified-email
	// fallback (D-02) can only succeed if an email scope was granted. Fail
	// closed at startup rather than silently returning blank emails forever.
	if pc.EmailsURL != "" && !hasEmailScope(pc.Scopes) {
		return nil, fmt.Errorf("OAuth2 provider %s: emails_url set but scopes lack an email scope (e.g. user:email)", pc.ID)
	}
	emailField := pc.EmailField
	if emailField == "" {
		emailField = "email"
	}
	display := pc.DisplayName
	if display == "" {
		display = pc.ID
	}
	return &oauth2LoginProvider{
		id:           pc.ID,
		displayName:  display,
		authURL:      pc.AuthURL,
		tokenURL:     pc.TokenURL,
		userinfoURL:  pc.UserinfoURL,
		emailsURL:    pc.EmailsURL,
		clientID:     pc.ClientID,
		secret:       secret,
		scopes:       pc.Scopes,
		subjectField: pc.SubjectField,
		emailField:   emailField,
		issuerID:     config.ProviderIssuer(pc),
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// hasEmailScope reports whether any configured scope grants email access
// (e.g. GitHub's "user:email" or a plain "email" scope).
func hasEmailScope(scopes []string) bool {
	for _, s := range scopes {
		if strings.Contains(s, "email") {
			return true
		}
	}
	return false
}

// ensureOpenID guarantees the openid scope is present.
func ensureOpenID(scopes []string) []string {
	for _, s := range scopes {
		if s == "openid" {
			return scopes
		}
	}
	return append([]string{"openid"}, scopes...)
}

// RedirectURI builds the OIDC callback URL from the request, honoring an
// explicit base override (auth.oidc.base_url) when set.
func RedirectURI(baseURL string, tls bool, host, fwdProto, fwdHost string) string {
	if baseURL != "" {
		return strings.TrimRight(baseURL, "/") + "/api/auth/oidc/callback"
	}
	scheme := "http"
	if tls || fwdProto == "https" {
		scheme = "https"
	}
	h := host
	if fwdHost != "" {
		h = fwdHost
	}
	u := url.URL{Scheme: scheme, Host: h, Path: "/api/auth/oidc/callback"}
	return u.String()
}
