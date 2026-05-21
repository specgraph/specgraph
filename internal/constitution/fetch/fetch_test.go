// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build testfetch

package fetch_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/constitution/fetch"
)

func TestFetch_HappyPath_FileScheme(t *testing.T) {
	tmpDir := t.TempDir()
	fixturePath := filepath.Join(tmpDir, "constitution.yaml")
	fixture := []byte("name: test\nlayer: project\n")
	require.NoError(t, os.WriteFile(fixturePath, fixture, 0o644))

	url := "file://" + fixturePath
	result, err := fetch.Fetch(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, fixture, result.Body, "body should match fixture exactly")
	assert.Equal(t, url, result.ResolvedURL, "ResolvedURL stores user-supplied URL verbatim")
}

func TestFetch_UnsupportedScheme(t *testing.T) {
	// 'mailto' is not a registered getter.
	_, err := fetch.Fetch(context.Background(), "mailto:bob@example.com")
	require.Error(t, err)
}
