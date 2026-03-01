// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"
	"net/http"

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
	return fmt.Sprintf("http://%s:%d", cfg.Server.Host, cfg.Server.Port), nil
}

func newHTTPClient() *http.Client {
	return http.DefaultClient
}
