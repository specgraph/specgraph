// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package fetch retrieves constitution files from remote URLs using
// hashicorp/go-getter v1 with a restricted security posture:
//
//   - ClientModeFile only (no directory mode, no archive extraction)
//   - Restricted getter registry: http/https/git only in production;
//     file:// is added under the testfetch build tag for tests.
//   - Host-scoped SPECGRAPH_FETCH_GITHUB_TOKEN injection for the GitHub
//     raw / API hosts; never sent to other hosts.
//   - 10-second HTTP timeout.
//   - 1 MiB body cap enforced after fetch.
//   - URL credential sanitization: rejects userinfo and common
//     token-bearing query parameters at parse time.
//   - Decompressors disabled (no zip/tar auto-extraction).
package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-getter"
)

const (
	bodySizeCap = 1 << 20 // 1 MiB
	httpTimeout = 10 * time.Second
)

// Fetched holds the result of a remote constitution fetch.
type Fetched struct {
	// Body is the raw YAML/JSON content as bytes.
	Body []byte
	// ResolvedURL is the URL as the user supplied it (stored verbatim).
	ResolvedURL string
}

// Fetch retrieves a constitution file from the given URL.
//
// URL credential sanitization: rejects URLs with RFC 3986 userinfo
// (https://user[:pass]@host) and common token-bearing query parameters
// (token, access_token, api_key, password) with an InvalidArgument-style
// error that names SPECGRAPH_FETCH_GITHUB_TOKEN as the supported
// alternative.
//
// Auth: if SPECGRAPH_FETCH_GITHUB_TOKEN is set and the request host is
// in the GitHub allow-list (raw.githubusercontent.com, api.github.com,
// *.githubusercontent.com), Authorization: Bearer <token> is injected
// via the HTTP transport. The header is stripped on cross-host redirects
// to non-allow-list hosts. Token is never sent to other hosts.
func Fetch(ctx context.Context, rawURL string) (*Fetched, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "specgraph-fetch-")
	if err != nil {
		return nil, fmt.Errorf("create temp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() //nolint:errcheck // best-effort cleanup; OS reclaims on reboot

	dst := filepath.Join(tmpDir, "constitution")

	client := &getter.Client{
		Ctx:           ctx,
		Src:           rawURL,
		Dst:           dst,
		Mode:          getter.ClientModeFile,
		Getters:       defaultGetters(tokenFromEnv()),
		Decompressors: nil,
	}
	err = client.Get()
	if err != nil {
		return nil, fmt.Errorf("fetch %q: %w", rawURL, err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		return nil, fmt.Errorf("stat fetched file: %w", err)
	}
	if info.Size() > bodySizeCap {
		return nil, fmt.Errorf("fetched body exceeds %d bytes (got %d)", bodySizeCap, info.Size())
	}

	body, err := os.ReadFile(dst)
	if err != nil {
		return nil, fmt.Errorf("read fetched file: %w", err)
	}

	return &Fetched{Body: body, ResolvedURL: rawURL}, nil
}

// validateURL rejects URLs with embedded credentials per the spec's
// URL credential containment invariant (Section 14).
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.User != nil {
		return errors.New("URL contains embedded credentials; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access")
	}
	// Case-insensitive credential param check — variants like ACCESS_TOKEN
	// must also be rejected.
	blocked := map[string]struct{}{
		"token":        {},
		"access_token": {},
		"api_key":      {},
		"password":     {},
	}
	for k := range u.Query() {
		if _, ok := blocked[strings.ToLower(k)]; ok {
			return fmt.Errorf("URL query contains credential parameter %q; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access", k)
		}
	}
	return nil
}

// tokenFromEnv reads SPECGRAPH_FETCH_GITHUB_TOKEN. Returns "" if unset.
func tokenFromEnv() string {
	return os.Getenv("SPECGRAPH_FETCH_GITHUB_TOKEN")
}
