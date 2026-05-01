// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package confluence implements the publish.Publisher interface for Confluence.
package confluence

import "time"

// Config holds Confluence publishing configuration.
type Config struct {
	CloudID      string
	SpaceKey     string
	ParentPageID string
	BaseURL      string        // defaults to https://api.atlassian.com
	APIToken     string        // from CONFLUENCE_API_TOKEN
	UserEmail    string        // from CONFLUENCE_USER_EMAIL
	PollInterval time.Duration // default 15m
	Labels       []string
}
