// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/config"
)

func resolveBaseURL() (string, error) {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return "", err
	}
	if cfg.Server.Remote != "" {
		return cfg.Server.Remote, nil
	}
	scheme := "http"
	if cfg.Server.TLS {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, cfg.Server.Host, cfg.Server.Port), nil
}

func newHTTPClient() *http.Client {
	return http.DefaultClient
}

// newClient creates a ConnectRPC client using the configured base URL.
func newClient[C any](ctor func(httpClient connect.HTTPClient, baseURL string, opts ...connect.ClientOption) C) (C, error) {
	baseURL, err := resolveBaseURL()
	if err != nil {
		var zero C
		return zero, err
	}
	return ctor(newHTTPClient(), baseURL), nil
}

func authoringClient() (specgraphv1connect.AuthoringServiceClient, error) {
	return newClient(specgraphv1connect.NewAuthoringServiceClient)
}
