// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestScan_GoProject(t *testing.T) {
	dir := t.TempDir()

	// Set up a minimal Go project
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/myapp

go 1.25.0

require (
	github.com/spf13/cobra v1.10.2
	connectrpc.com/connect v1.19.1
)
`), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yaml"), []byte("name: CI\n"), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM golang:1.25-alpine\n"), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "myapp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "myapp", "main.go"), []byte("package main\n"), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "server"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal", "server", "server.go"), []byte("package server\n"), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "go", result.Tech.Languages.Primary)
	require.Equal(t, "GitHub Actions", result.Tech.Infrastructure["ci"])
	require.Equal(t, "Docker", result.Tech.Infrastructure["runtime"])
}

func TestScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Empty(t, result.Tech.Languages.Primary)
}

func TestScan_NodeProject(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "my-app",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^14.0.0"
  }
}
`), 0o600))

	// Add tsconfig.json so the scanner detects TypeScript (not JavaScript)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{}`), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "typescript", result.Tech.Languages.Primary)
	require.Contains(t, result.Tech.Frameworks, "ui")
}

func TestScan_PullCLAUDEMD(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(`# Project Guidelines

## Tech Stack
- Language: Go
- Database: PostgreSQL

## Constraints
- No ORMs
- All APIs must be versioned
`), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25.0\n"), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "go", result.Tech.Languages.Primary)
}
