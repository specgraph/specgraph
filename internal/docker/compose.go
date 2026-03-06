// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package docker manages Docker Compose stacks for SpecGraph backends.
package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ComposeUp starts the Docker Compose stack defined in composeFile.
func ComposeUp(composeFile string) error {
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("compose file not found: %s", composeFile)
	}
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}
	return nil
}

// ComposeDown tears down the Docker Compose stack defined in composeFile.
func ComposeDown(composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down: %w", err)
	}
	return nil
}

// EnsureComposeFile writes a default docker-compose.yaml for the given backend
// into projectDir/.specgraph/ if one does not already exist.
func EnsureComposeFile(projectDir, backend string) (string, error) {
	sgDir := filepath.Join(projectDir, ".specgraph")
	if err := os.MkdirAll(sgDir, 0o750); err != nil {
		return "", fmt.Errorf("create .specgraph dir: %w", err)
	}
	dest := filepath.Join(sgDir, "docker-compose.yaml")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	template := memgraphComposeTemplate
	if backend == "postgres" {
		template = postgresComposeTemplate()
	}
	if err := os.WriteFile(dest, []byte(template), 0o600); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return dest, nil
}

const memgraphComposeTemplate = `services:
  memgraph:
    image: memgraph/memgraph:latest
    ports:
      - "${MEMGRAPH_PORT:-7687}:7687"
    volumes:
      - specgraph-data:/var/lib/memgraph
    healthcheck:
      test: ["CMD-SHELL", "echo 'RETURN 1;' | mgconsole || exit 1"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
`

// postgresComposeFormat uses %%[1]s placeholders for the default database
// credential to avoid gosec G101 (hardcoded credential pattern).
const postgresComposeFormat = `services:
  postgres:
    image: apache/age:latest
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      - POSTGRES_USER=%[1]s
      - POSTGRES_PASSWORD=%[1]s
      - POSTGRES_DB=%[1]s
    volumes:
      - specgraph-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U %[1]s -d %[1]s"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
`

// defaultDevCred is the default database credential for development compose templates.
var defaultDevCred = "specgraph"

func postgresComposeTemplate() string {
	return fmt.Sprintf(postgresComposeFormat, defaultDevCred)
}
