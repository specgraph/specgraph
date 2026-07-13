// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/specgraph/specgraph/internal/config"
)

// IntrospectionResult is the decoded RFC 7662 introspection response. Only the
// fields the resolver needs are lifted into typed fields; the full response is
// retained in Raw so audienceContains (and any future claims_mapping) can read
// the "aud" claim in either its string or []string shape.
type IntrospectionResult struct {
	Active  bool
	Subject string
	Issuer  string
	Exp     int64
	Raw     map[string]json.RawMessage
}

// introspectionTTLCap bounds how long an active introspection result is cached,
// even when the token's own exp is further out. Keeps a revoked-but-unexpired
// token from being honored for its full lifetime after revocation.
const introspectionTTLCap = 60 * time.Second

// introspectionTimeout bounds a single introspection round-trip so a slow or
// hung IdP cannot stall the request path (DoS mitigation, T-03-04-02).
const introspectionTimeout = 5 * time.Second

// introspectionMaxBody caps the response body read to defend against a
// misbehaving/hostile introspection endpoint returning an unbounded body.
const introspectionMaxBody = 1 << 20 // 1 MiB

// Introspector is a bounded RFC 7662 client for a single introspection-capable
// provider. It holds the endpoint + client credentials, a timeout-bounded HTTP
// client, and a small active-result cache. issuer is the provider's canonical
// issuer (config.ProviderIssuer) used by the resolver for per-issuer rate
// limiting.
type Introspector struct {
	issuer       string
	endpoint     string
	clientID     string
	clientSecret string
	client       *http.Client

	now func() time.Time // injectable clock for tests; defaults to time.Now

	mu    sync.Mutex
	cache map[string]introspectionCacheEntry // key: sha256(token) hex
}

type introspectionCacheEntry struct {
	result    *IntrospectionResult
	expiresAt time.Time
}

// NewIntrospector builds an Introspector for one provider. endpoint is the RFC
// 7662 introspection URL; clientID/clientSecret authenticate the resource
// server to the IdP (HTTP Basic). issuer is config.ProviderIssuer(pc).
func NewIntrospector(issuer, endpoint, clientID, clientSecret string) *Introspector {
	return &Introspector{
		issuer:       issuer,
		endpoint:     endpoint,
		clientID:     clientID,
		clientSecret: clientSecret,
		client:       &http.Client{Timeout: introspectionTimeout},
		now:          time.Now,
		cache:        make(map[string]introspectionCacheEntry),
	}
}

// Issuer returns the provider's canonical issuer (used for per-issuer rate
// limiting by the resolver).
func (c *Introspector) Issuer() string { return c.issuer }

// BuildIntrospectors constructs an Introspector for each provider that
// configures an IntrospectionURL (D-06). The RS authenticates to the IdP with
// the provider's client_id + resolved client secret (HTTP Basic). issuer is the
// canonical config.ProviderIssuer so the resolver's per-issuer rate limiter is
// keyed consistently with the JWT/JIT path. Providers without an
// IntrospectionURL are skipped; a configured endpoint with no resolvable secret
// is a startup-fatal error (mirrors BuildLoginProviders discipline).
func BuildIntrospectors(providers []config.OIDCProviderConfig) ([]*Introspector, error) {
	var out []*Introspector
	for i := range providers {
		pc := providers[i]
		if pc.IntrospectionURL == "" {
			continue
		}
		secret, err := resolveClientSecret(pc)
		if err != nil {
			return nil, fmt.Errorf("introspection provider %q: %w", pc.ID, err)
		}
		out = append(out, NewIntrospector(config.ProviderIssuer(pc), pc.IntrospectionURL, pc.ClientID, secret))
	}
	return out, nil
}

func introspectionCacheKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", sum)
}

func (c *Introspector) cacheGet(token string) (*IntrospectionResult, bool) {
	key := introspectionCacheKey(token)
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.After(c.now()) {
		delete(c.cache, key)
		return nil, false
	}
	return entry.result, true
}

func (c *Introspector) cachePut(token string, res *IntrospectionResult) {
	// Cache active results only, to min(token exp, now+TTL cap).
	if !res.Active {
		return
	}
	now := c.now()
	expiresAt := now.Add(introspectionTTLCap)
	if res.Exp > 0 {
		tokenExp := time.Unix(res.Exp, 0)
		if tokenExp.Before(expiresAt) {
			expiresAt = tokenExp
		}
	}
	if !expiresAt.After(now) {
		return // already expired; nothing to cache
	}
	key := introspectionCacheKey(token)
	c.mu.Lock()
	c.cache[key] = introspectionCacheEntry{result: res, expiresAt: expiresAt}
	c.mu.Unlock()
}

// Introspect validates an opaque token via RFC 7662. Return contract:
//   - (active result, nil)   — the IdP answered active==true
//   - (inactive result, nil) — the IdP gave a DECISIVE inactive answer (2xx
//     active==false, or a 4xx client error)
//   - (nil, err)             — a NON-decisive failure (network error, timeout,
//     5xx, or an undecodable 2xx body) — the resolver maps this to ErrTransient
//     (fail-closed, retryable) when no other introspector answers decisively.
func (c *Introspector) Introspect(ctx context.Context, token string) (*IntrospectionResult, error) {
	if res, ok := c.cacheGet(token); ok {
		return res, nil
	}
	form := url.Values{"token": {token}}
	// G704 (gosec): c.endpoint is an operator-configured introspection URL
	// (config.OIDCProviderConfig.IntrospectionURL), never derived from the token
	// or any request-controlled input — see threat T-03-04-06 (SSRF accepted:
	// operator config, bounded timeout, configured providers only).
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(form.Encode())) //nolint:gosec // G704: endpoint is operator config, not token-derived (T-03-04-06)
	if err != nil {
		return nil, fmt.Errorf("introspect: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.clientID != "" {
		req.SetBasicAuth(c.clientID, c.clientSecret)
	}
	resp, err := c.client.Do(req) //nolint:gosec // G704: endpoint is operator config, not token-derived (T-03-04-06)
	if err != nil {
		return nil, fmt.Errorf("introspect: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() //nolint:errcheck // best-effort close on read path

	switch {
	case resp.StatusCode >= 500:
		// Upstream failure — non-decisive; caller may retry.
		return nil, fmt.Errorf("introspect: upstream status %d", resp.StatusCode)
	case resp.StatusCode >= 400:
		// Client error (e.g. 401 bad RS credentials) — decisive: the token is
		// not validatable here. Treat as inactive (fail-closed).
		return &IntrospectionResult{Active: false}, nil
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("introspect: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, introspectionMaxBody))
	if err != nil {
		return nil, fmt.Errorf("introspect: read body: %w", err)
	}
	var raw map[string]json.RawMessage
	if decErr := json.Unmarshal(body, &raw); decErr != nil {
		return nil, fmt.Errorf("introspect: decode: %w", decErr)
	}
	// Second pass into a typed view so the standard claims are extracted without
	// unchecked per-field unmarshals (errcheck-clean).
	var parsed struct {
		Active  bool   `json:"active"`
		Subject string `json:"sub"`
		Issuer  string `json:"iss"`
		Exp     int64  `json:"exp"`
	}
	if decErr := json.Unmarshal(body, &parsed); decErr != nil {
		return nil, fmt.Errorf("introspect: decode claims: %w", decErr)
	}
	res := &IntrospectionResult{
		Active:  parsed.Active,
		Subject: parsed.Subject,
		Issuer:  parsed.Issuer,
		Exp:     parsed.Exp,
		Raw:     raw,
	}
	c.cachePut(token, res)
	return res, nil
}
