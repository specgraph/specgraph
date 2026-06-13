// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

const txCookieName = "specgraph_oidc_tx"

// OIDCLoginConfig parametrizes the interactive login handlers.
type OIDCLoginConfig struct {
	Providers  []auth.LoginProvider
	Resolver   auth.Resolver
	WebAuth    storage.WebAuthStore
	BaseURL    string
	SessionTTL time.Duration
	FlowTTL    time.Duration // default 5m
	Limiter    *ipRateLimiter
}

type oidcLoginHandler struct {
	byID       map[string]auth.LoginProvider
	resolver   auth.Resolver
	webAuth    storage.WebAuthStore
	baseURL    string
	sessionTTL time.Duration
	flowTTL    time.Duration
}

// RegisterOIDCLoginHandlers wires /api/auth/oidc/{providers,start,callback}.
// Endpoints are public (no RequireAuth); start/callback are per-IP rate
// limited. No-op when no interactive providers are configured.
func RegisterOIDCLoginHandlers(mux *http.ServeMux, cfg OIDCLoginConfig) {
	if len(cfg.Providers) == 0 {
		return
	}
	flowTTL := cfg.FlowTTL
	if flowTTL <= 0 {
		flowTTL = 5 * time.Minute
	}
	h := &oidcLoginHandler{
		byID:       map[string]auth.LoginProvider{},
		resolver:   cfg.Resolver,
		webAuth:    cfg.WebAuth,
		baseURL:    cfg.BaseURL,
		sessionTTL: cfg.SessionTTL,
		flowTTL:    flowTTL,
	}
	for _, p := range cfg.Providers {
		h.byID[p.ID()] = p
	}

	limit := cfg.Limiter.wrap
	mux.HandleFunc("/api/auth/oidc/providers", h.handleProviders)
	mux.Handle("/api/auth/oidc/callback", limit(http.HandlerFunc(h.handleCallback)))
	mux.Handle("/api/auth/oidc/{provider}/start", limit(http.HandlerFunc(h.handleStart)))
}

type providerInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (h *oidcLoginHandler) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	infos := make([]providerInfo, 0, len(h.byID))
	for _, p := range h.byID {
		infos = append(infos, providerInfo{ID: p.ID(), DisplayName: p.DisplayName()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"providers": infos}) //nolint:errcheck // best-effort write
}

func (h *oidcLoginHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	p, ok := h.byID[r.PathValue("provider")]
	if !ok {
		writeJSONError(w, http.StatusNotFound, "unknown provider")
		return
	}
	state, err := randToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	nonce, err := randToken()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "internal")
		return
	}
	verifier := oauth2.GenerateVerifier()
	challenge := oauth2.S256ChallengeFromVerifier(verifier)

	flowID, err := h.webAuth.CreateLoginFlow(r.Context(), &storage.LoginFlow{
		State: state, Nonce: nonce, CodeVerifier: verifier, ProviderID: p.ID(),
		ExpiresAt: time.Now().Add(h.flowTTL),
	})
	if err != nil {
		slog.LogAttrs(r.Context(), slog.LevelError, "oidc: create login flow", slog.Any("error", err))
		writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		return
	}
	http.SetCookie(w, h.txCookie(flowID, r))

	redirectURI := auth.RedirectURI(h.baseURL, r.TLS != nil, r.Host,
		r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
	http.Redirect(w, r, p.AuthCodeURL(state, nonce, challenge, redirectURI), http.StatusFound)
}

func (h *oidcLoginHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// Always delete the tx cookie on the response.
	http.SetCookie(w, h.deleteTxCookie(r))

	fail := func(reason string) { http.Redirect(w, r, "/?auth_error="+reason, http.StatusFound) }

	c, err := r.Cookie(txCookieName)
	if err != nil || c.Value == "" {
		fail("expired")
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		fail("denied")
		return
	}
	flow, err := h.webAuth.ConsumeLoginFlow(r.Context(), c.Value)
	if err != nil {
		// Not-found/expired/bad-id → expired; a genuine DB error → temporary.
		if errors.Is(err, storage.ErrLoginFlowNotFound) {
			fail("expired")
		} else {
			fail("temporary")
		}
		return
	}
	if r.URL.Query().Get("state") != flow.State {
		fail("state")
		return
	}
	p, ok := h.byID[flow.ProviderID]
	if !ok {
		slog.LogAttrs(r.Context(), slog.LevelWarn, "oidc: provider removed mid-flow", slog.String("provider", flow.ProviderID))
		fail("exchange")
		return
	}
	redirectURI := auth.RedirectURI(h.baseURL, r.TLS != nil, r.Host,
		r.Header.Get("X-Forwarded-Proto"), r.Header.Get("X-Forwarded-Host"))
	idToken, err := p.Exchange(r.Context(), r.URL.Query().Get("code"), flow.CodeVerifier, flow.Nonce, redirectURI)
	if err != nil {
		slog.LogAttrs(r.Context(), slog.LevelWarn, "oidc: exchange failed", slog.String("provider", p.ID()), slog.Any("error", err))
		fail("exchange")
		return
	}
	// Resolve identity (triggers binding lookup / JIT), limiter bypassed.
	id, err := h.resolver.Resolve(auth.WithInteractiveLogin(r.Context()), idToken)
	if err != nil {
		if errors.Is(err, auth.ErrTransient) {
			fail("temporary")
			return
		}
		fail("unauthorized")
		return
	}
	// Mint the server session.
	token, err := randSessionToken()
	if err != nil {
		fail("temporary")
		return
	}
	sum := sha256.Sum256([]byte(token))
	if _, err := h.webAuth.CreateSession(r.Context(), &storage.Session{
		TokenHash: sum[:], UserID: id.UserID, OIDCSubject: subjectOnly(id.Subject),
		ExpiresAt: time.Now().Add(h.sessionTTL),
	}); err != nil {
		slog.LogAttrs(r.Context(), slog.LevelError, "oidc: create session", slog.Any("error", err))
		fail("temporary")
		return
	}
	establishSession(w, r, token)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *oidcLoginHandler) txCookie(value string, r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     txCookieName,
		Value:    value,
		Path:     "/api/auth/oidc",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		MaxAge:   300,
	}
}

func (h *oidcLoginHandler) deleteTxCookie(r *http.Request) *http.Cookie {
	c := h.txCookie("", r)
	c.MaxAge = -1
	return c
}

// subjectOnly strips the "oidc:" prefix the resolver adds to Identity.Subject.
func subjectOnly(subject string) string {
	const p = "oidc:"
	if len(subject) > len(p) && subject[:len(p)] == p {
		return subject[len(p):]
	}
	return subject
}

func randToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// randSessionToken returns the opaque spgr_ws_ session token.
func randSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "spgr_ws_" + base64.RawURLEncoding.EncodeToString(b), nil
}
