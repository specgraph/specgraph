// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/oauth2"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

const txCookieName = "specgraph_oidc_tx"

// cliCodeTTL bounds the lifetime of a one-time CLI login code (machine leg).
const cliCodeTTL = 60 * time.Second

// OIDCLoginConfig parametrizes the interactive login handlers.
type OIDCLoginConfig struct {
	Providers       []auth.LoginProvider
	Resolver        auth.Resolver
	WebAuth         storage.WebAuthStore
	BaseURL         string
	SessionTTL      time.Duration
	FlowTTL         time.Duration // default 5m
	Limiter         *ipRateLimiter
	CLILoginEnabled bool
}

type oidcLoginHandler struct {
	byID            map[string]auth.LoginProvider
	resolver        auth.Resolver
	webAuth         storage.WebAuthStore
	baseURL         string
	sessionTTL      time.Duration
	flowTTL         time.Duration
	cliLoginEnabled bool
}

// RegisterOIDCLoginHandlers wires /api/auth/oidc/{providers,start,callback}.
// Endpoints are public (no RequireAuth); start/callback are per-IP rate
// limited. No-op when no interactive providers are configured.
func RegisterOIDCLoginHandlers(mux *http.ServeMux, cfg OIDCLoginConfig) { //nolint:gocritic // hugeParam: cfg is one-shot startup wiring, not a hot path
	if len(cfg.Providers) == 0 {
		return
	}
	flowTTL := cfg.FlowTTL
	if flowTTL <= 0 {
		flowTTL = 5 * time.Minute
	}
	h := &oidcLoginHandler{
		byID:            map[string]auth.LoginProvider{},
		resolver:        cfg.Resolver,
		webAuth:         cfg.WebAuth,
		baseURL:         cfg.BaseURL,
		sessionTTL:      cfg.SessionTTL,
		flowTTL:         flowTTL,
		cliLoginEnabled: cfg.CLILoginEnabled,
	}
	for _, p := range cfg.Providers {
		h.byID[p.ID()] = p
	}

	limit := cfg.Limiter.wrap
	mux.HandleFunc("/api/auth/oidc/providers", h.handleProviders)
	mux.Handle("/api/auth/oidc/callback", limit(http.HandlerFunc(h.handleCallback)))
	mux.Handle("/api/auth/oidc/{provider}/start", limit(http.HandlerFunc(h.handleStart)))
	if cfg.CLILoginEnabled {
		mux.Handle("/api/auth/cli/exchange", limit(http.HandlerFunc(h.handleCLIExchange)))
	}
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
	cliCallback := r.URL.Query().Get("cli_callback")
	cliState := r.URL.Query().Get("cli_state")
	cliChallenge := r.URL.Query().Get("cli_challenge")
	if cliCallback != "" {
		if !h.cliLoginEnabled {
			writeJSONError(w, http.StatusForbidden, "cli_login_disabled")
			return
		}
		if _, ok := validateCLICallback(cliCallback); !ok {
			writeJSONError(w, http.StatusBadRequest, "invalid cli_callback")
			return
		}
		if cliChallenge == "" || cliState == "" {
			writeJSONError(w, http.StatusBadRequest, "cli_challenge and cli_state required")
			return
		}
		// Bound lengths to the storage CHECK constraints so oversized params
		// surface as 400 rather than a generic 503 from the failed INSERT.
		if len(cliCallback) > 512 || len(cliState) > 512 || len(cliChallenge) > 256 {
			writeJSONError(w, http.StatusBadRequest, "cli parameter too long")
			return
		}
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
		CLICallback: cliCallback, CLIState: cliState, CLIChallenge: cliChallenge,
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
	//nolint:gosec // G710: redirect target is the configured IdP authorize URL (from discovery) + opaque params, not user-controlled
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
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(flow.State)) != 1 {
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
	claims, err := p.Exchange(r.Context(), r.URL.Query().Get("code"), flow.CodeVerifier, flow.Nonce, redirectURI)
	if err != nil {
		slog.LogAttrs(r.Context(), slog.LevelWarn, "oidc: exchange failed", slog.String("provider", p.ID()), slog.Any("error", err))
		fail("exchange")
		return
	}
	// Resolve identity from verified claims (binding lookup / JIT), limiter bypassed.
	id, err := h.resolver.ResolveLogin(auth.WithInteractiveLogin(r.Context()), claims)
	if err != nil {
		if errors.Is(err, auth.ErrTransient) {
			fail("temporary")
			return
		}
		fail("unauthorized")
		return
	}
	// CLI flow: deliver a one-time code to the validated loopback, no cookie.
	if flow.CLICallback != "" {
		cb, ok := validateCLICallback(flow.CLICallback) // re-validate; never trust stored value into a redirect
		if !ok {
			fail("exchange")
			return
		}
		code, codeErr := randToken()
		if codeErr != nil {
			fail("temporary")
			return
		}
		sum := sha256.Sum256([]byte(code))
		if createErr := h.webAuth.CreateCLICode(r.Context(), sum[:], id.UserID, subjectOnly(id.Subject), flow.CLIChallenge, time.Now().Add(cliCodeTTL)); createErr != nil {
			if errors.Is(createErr, storage.ErrUserNotFound) {
				fail("unauthorized")
				return
			}
			fail("temporary")
			return
		}
		q := url.Values{"cli_state": {flow.CLIState}, "code": {code}}
		cb.RawQuery = q.Encode()
		http.Redirect(w, r, cb.String(), http.StatusFound) //nolint:gosec // G710: target validated to literal loopback via validateCLICallback
		return
	}

	// Web flow: mint the server session (unchanged).
	token, err := randSessionToken()
	if err != nil {
		fail("temporary")
		return
	}
	sum := sha256.Sum256([]byte(token))
	if _, err := h.webAuth.CreateSession(r.Context(), &storage.Session{
		TokenHash: sum[:], UserID: id.UserID, OIDCSubject: subjectOnly(id.Subject),
		Issuer:    id.Issuer,
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
	return &http.Cookie{ //nolint:gosec // G124: Secure is dynamic via r.TLS / X-Forwarded-Proto for dev/prod parity; HttpOnly+SameSite are set here
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
	c := h.txCookie("", r) //nolint:gosec // G124: cookie comes from txCookie() which sets HttpOnly/SameSite/dynamic Secure
	c.MaxAge = -1
	return c
}

// validateCLICallback enforces a strict loopback redirect target. Returns the
// parsed URL (query/fragment-free, path "/callback") or false.
func validateCLICallback(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" || u.Path != "/callback" ||
		!auth.IsLiteralLoopbackHost(u.Hostname()) {
		return nil, false
	}
	return u, true
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
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// randSessionToken returns the opaque spgr_ws_ session token.
func randSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return "spgr_ws_" + base64.RawURLEncoding.EncodeToString(b), nil
}

type cliExchangeRequest struct {
	Code        string `json:"code"`
	CLIVerifier string `json:"cli_verifier"`
}

type cliExchangeResponse struct {
	Token       string    `json:"token"`
	ExpiresAt   time.Time `json:"expires_at"`
	OIDCSubject string    `json:"oidc_subject"`
}

func (h *oidcLoginHandler) handleCLIExchange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req cliExchangeRequest
	// Bound the public endpoint's request body; {code, cli_verifier} is tiny.
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil || req.Code == "" || req.CLIVerifier == "" {
		writeJSONError(w, http.StatusBadRequest, "missing code or verifier")
		return
	}
	token, err := randSessionToken()
	if err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		return
	}
	tokenHash := sha256.Sum256([]byte(token))
	codeHash := sha256.Sum256([]byte(req.Code))
	gotChallenge := oauth2.S256ChallengeFromVerifier(req.CLIVerifier)
	expiresAt := time.Now().Add(h.sessionTTL)

	sess, err := h.webAuth.ExchangeCLICode(r.Context(), codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: expiresAt,
	}, gotChallenge)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrCLICodeNotFound), errors.Is(err, storage.ErrCLIChallengeMismatch):
			writeJSONError(w, http.StatusBadRequest, "invalid or expired code")
		case errors.Is(err, storage.ErrUserNotFound):
			writeJSONError(w, http.StatusForbidden, "account_unavailable")
		default:
			slog.LogAttrs(r.Context(), slog.LevelError, "oidc: cli exchange", slog.Any("error", err))
			writeJSONError(w, http.StatusServiceUnavailable, "temporary")
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(cliExchangeResponse{ //nolint:errcheck // best-effort write
		Token: token, ExpiresAt: sess.ExpiresAt, OIDCSubject: sess.OIDCSubject,
	})
}
