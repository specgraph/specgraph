// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package web provides the embedded static files for the SpecGraph UI.
package web

import "embed"

// Build contains the SvelteKit production build output.
// When the web UI hasn't been built yet, this will be empty.
//
//go:embed all:build
var Build embed.FS
