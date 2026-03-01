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

// TestScan_GoProject_IgnoresCLAUDEMD verifies that non-manifest files like
// CLAUDE.md are not misidentified as infrastructure or framework config.
// The scanner should detect the Go language from go.mod while ignoring CLAUDE.md.
func TestScan_GoProject_IgnoresCLAUDEMD(t *testing.T) {
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

// TestScan_JavaScriptProject verifies a package.json without tsconfig.json is
// detected as JavaScript (not TypeScript).
func TestScan_JavaScriptProject(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "my-js-app",
  "dependencies": {
    "express": "^4.18.0"
  }
}
`), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "javascript", result.Tech.Languages.Primary)
	require.Equal(t, "Express", result.Tech.Frameworks["api"])
}

// TestScan_DockerComposeYml verifies docker-compose.yml (dot-yml extension) is
// detected in addition to docker-compose.yaml.
func TestScan_DockerComposeYml(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(`version: "3"
services:
  app:
    image: myapp
`), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "Docker Compose", result.Tech.Infrastructure["orchestration"])
}

// TestScan_MultipleCITools verifies that when multiple CI tool indicators are
// present, the last-matched CI tool wins (current last-writer-wins behavior).
func TestScan_MultipleCITools(t *testing.T) {
	dir := t.TempDir()

	// Both GitHub Actions and CircleCI present
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yaml"), []byte("name: CI\n"), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".circleci"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".circleci", "config.yml"), []byte("version: 2\n"), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	// detectCI checks in order: GitHub Actions, GitLab CI, Jenkins, CircleCI.
	// Last writer wins, so CircleCI overwrites GitHub Actions.
	require.Equal(t, "CircleCI", result.Tech.Infrastructure["ci"])
}

// TestScan_KubernetesInSubdirectory verifies that Kubernetes manifests nested
// in subdirectories are detected via WalkDir traversal.
func TestScan_KubernetesInSubdirectory(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "deploy", "k8s"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "deploy", "k8s", "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
`), 0o600))

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "Kubernetes", result.Tech.Infrastructure["runtime"])
}
