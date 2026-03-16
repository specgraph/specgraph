// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/config"
	"github.com/seanb4t/specgraph/internal/xdg"
)

// projectTransport injects the X-Specgraph-Project header on every request.
type projectTransport struct {
	base    http.RoundTripper
	project string
}

func (t *projectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("X-Specgraph-Project", t.project)
	return t.base.RoundTrip(req)
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
		globalCfg, globalErr := config.LoadGlobal(xdg.ConfigFile())
		if globalErr != nil {
			return "", "", fmt.Errorf("load global config: %w", globalErr)
		}
		slug := projectCfg.Slug
		resolved := globalCfg.ResolveServer(slug, projectCfg.Server)
		return resolved, slug, nil
	}

	// No .specgraph.yaml found; fall back to old config.
	cfg, err := config.Load(cfgFile)
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

func newHTTPClient(project string) *http.Client {
	transport := http.DefaultTransport
	if project != "" {
		transport = &projectTransport{
			base:    http.DefaultTransport,
			project: project,
		}
	}
	return &http.Client{Transport: transport}
}

// newClient creates a ConnectRPC client using the configured base URL.
func newClient[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C) (C, error) {
	baseURL, project, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	return ctor(newHTTPClient(project), baseURL), nil
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
	return ctor(newHTTPClient(derivedProject), baseURL), nil
}

func authoringClient() (specgraphv1connect.AuthoringServiceClient, error) {
	return newClient(specgraphv1connect.NewAuthoringServiceClient)
}
