// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package fetch

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/go-getter"
)

// defaultGetters is the registry of allowed getters used by Fetch.
// Production builds use restrictedGetters (no file://). Test builds
// override this via init() in getters_testfetch.go to add file://.
var defaultGetters = restrictedGetters

// restrictedGetters returns a getter registry with http/https/git only.
// If token is non-empty, the http/https getters inject Authorization
// for the GitHub host allow-list.
//
// Note: no separate "github" scheme key — go-getter's URL detectors
// rewrite github.com/org/repo shorthand to git::... before dispatch
// (verified in Task 0).
func restrictedGetters(token string) map[string]getter.Getter {
	httpClient := &http.Client{
		Timeout:       httpTimeout,
		CheckRedirect: stripAuthOnCrossHost,
		Transport: &tokenTransport{
			base:  http.DefaultTransport,
			token: token,
		},
	}
	httpGetter := &getter.HttpGetter{
		Client: httpClient,
	}
	return map[string]getter.Getter{
		"http":  httpGetter,
		"https": httpGetter,
		"git":   new(getter.GitGetter),
	}
}

// tokenTransport injects Authorization: Bearer <token> on requests whose
// host is in the GitHub allow-list. Tokens are never sent to other hosts.
type tokenTransport struct {
	base  http.RoundTripper
	token string
}

func (t *tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Use Hostname() (strips port) and let hostAllowed lower-case for
	// case-insensitive matching; URL.Host preserves both port and casing,
	// which would let raw.githubusercontent.com:443 and mixed-case hosts
	// bypass the allow-list.
	if t.token != "" && hostAllowed(r.URL.Hostname()) {
		// Clone so we don't mutate the caller's request.
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", "Bearer "+t.token)
	}
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, fmt.Errorf("round trip: %w", err)
	}
	return resp, nil
}

// hostAllowed returns true if the host is a known GitHub identity
// surface that should receive the token. Case-insensitive; caller
// should pass a hostname-only string (no port).
func hostAllowed(host string) bool {
	host = strings.ToLower(host)
	if host == "raw.githubusercontent.com" || host == "api.github.com" {
		return true
	}
	if strings.HasSuffix(host, ".githubusercontent.com") {
		return true
	}
	return false
}

// stripAuthOnCrossHost removes the Authorization header when a redirect
// crosses to a host outside the allow-list. net/http does NOT strip
// Authorization automatically across cross-host redirects.
func stripAuthOnCrossHost(req *http.Request, via []*http.Request) error {
	if len(via) == 0 {
		return nil
	}
	prev := via[len(via)-1]
	curHost := req.URL.Hostname()
	prevHost := prev.URL.Hostname()
	if !strings.EqualFold(curHost, prevHost) && !hostAllowed(curHost) {
		req.Header.Del("Authorization")
	}
	return nil
}
