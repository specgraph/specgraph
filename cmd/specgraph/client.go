// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/xdg"
)

// clientTransport injects the X-Specgraph-Project header and forwards the
// bearer token from context on every request.
type clientTransport struct {
	base    http.RoundTripper
	project string
}

func (t *clientTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if t.project != "" {
		req.Header.Set("X-Specgraph-Project", t.project)
	}
	if token, ok := auth.BearerTokenFromContext(req.Context()); ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return t.base.RoundTrip(req)
}

// staticTokenTransport injects a fixed bearer token into context for
// CLI commands where the token is resolved once at startup.
type staticTokenTransport struct {
	inner http.RoundTripper
	token string
}

func (t *staticTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		ctx := auth.WithBearerToken(req.Context(), t.token)
		req = req.WithContext(ctx)
	}
	return t.inner.RoundTrip(req)
}

func resolveBaseURL() (baseURL, project string, err error) {
	// Try new config system: requires .specgraph.yaml to exist in repo.
	root, findErr := config.FindProjectRoot(".")
	if findErr != nil && !errors.Is(findErr, config.ErrProjectNotFound) {
		return "", "", fmt.Errorf("find project root: %w", findErr)
	}
	if findErr == nil {
		projectCfg, projErr := config.LoadProject(root)
		if projErr != nil {
			return "", "", fmt.Errorf("load project config: %w", projErr)
		}
		globalCfg, globalErr := loadGlobalCfg()
		if globalErr != nil {
			return "", "", fmt.Errorf("load global config: %w", globalErr)
		}
		slug := projectCfg.Slug
		resolved := globalCfg.ResolveServer(slug, projectCfg.Server)
		return resolved, slug, nil
	}

	// No .specgraph.yaml found; fall back to old config.
	cfg, err := config.Load(legacyConfigPath())
	if err != nil {
		return "", "", err
	}
	if cfg.Server.Remote != "" {
		return cfg.Server.Remote, "", nil
	}
	scheme := "http"
	if cfg.Server.TLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, cfg.Server.Host, cfg.Server.Port), "", nil
}

// resolveAPIKey returns the API key to use for CLI requests.
// Precedence: SPECGRAPH_API_KEY env var > credentials file > empty (no auth).
func resolveAPIKey() string {
	if key := os.Getenv("SPECGRAPH_API_KEY"); key != "" {
		return key
	}
	key, err := auth.ReadDefaultKey(xdg.CredentialsFile())
	if err != nil {
		return ""
	}
	return key
}

func newHTTPClient(project string) *http.Client {
	return &http.Client{Transport: &clientTransport{
		base:    http.DefaultTransport,
		project: project,
	}}
}

// newAuthenticatedHTTPClient returns an HTTP client that injects a static
// bearer token (resolved once at startup) into context before the
// context-aware clientTransport picks it up. Used by CLI commands.
func newAuthenticatedHTTPClient(project string) *http.Client {
	return &http.Client{Transport: &staticTokenTransport{
		inner: &clientTransport{
			base:    http.DefaultTransport,
			project: project,
		},
		token: resolveAPIKey(),
	}}
}

// newClient creates a ConnectRPC client using the configured base URL.
func newClient[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C) (C, error) {
	baseURL, project, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	return ctor(newAuthenticatedHTTPClient(project), baseURL), nil
}

// newClientWithProject creates a ConnectRPC client using an explicit project
// slug for the X-Specgraph-Project header, overriding the auto-derived value.
func newClientWithProject[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C, project string) (C, error) {
	baseURL, derivedProject, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	if project != "" {
		derivedProject = project
	}
	return ctor(newAuthenticatedHTTPClient(derivedProject), baseURL), nil
}

func authoringClient() (specgraphv1connect.AuthoringServiceClient, error) {
	return newClient(specgraphv1connect.NewAuthoringServiceClient)
}
