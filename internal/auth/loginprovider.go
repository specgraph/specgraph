// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"errors"
	"fmt"
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
// nonce) and returns the raw token for the resolver to materialize identity.
type LoginProvider interface {
	ID() string
	DisplayName() string
	AuthCodeURL(state, nonce, codeChallenge, redirectURI string) string
	// Exchange swaps the authorization code for a nonce-verified raw id_token.
	Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (idToken string, err error)
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

func (p *oidcLoginProvider) Exchange(ctx context.Context, code, codeVerifier, nonce, redirectURI string) (string, error) {
	cfg := p.oauth2Config(redirectURI)
	tok, err := cfg.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
	if err != nil {
		return "", fmt.Errorf("oidc exchange: %w", err)
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok || rawID == "" {
		return "", errors.New("oidc exchange: no id_token in response")
	}
	if _, err := p.verifier.VerifyWithNonce(ctx, rawID, nonce); err != nil {
		return "", err
	}
	return rawID, nil
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
		if kind != "oidc" {
			return nil, fmt.Errorf("OIDC provider %s: unsupported kind %q (only \"oidc\")", pc.ID, kind)
		}
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
		out = append(out, &oidcLoginProvider{
			id:          pc.ID,
			displayName: display,
			authURL:     ep.AuthURL,
			tokenURL:    ep.TokenURL,
			clientID:    pc.ClientID,
			secret:      secret,
			scopes:      scopes,
			verifier:    verifier,
		})
	}
	return out, nil
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
