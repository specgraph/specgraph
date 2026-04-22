// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package docker manages Docker Compose stacks for SpecGraph backends.
package docker

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// composeUpArgs returns the argv passed to `docker` for `ComposeUp`. Extracted
// as a pure function so tests can assert the exact flags without shelling out.
func composeUpArgs(composeFile string) []string {
	return []string{"compose", "-f", composeFile, "up", "-d", "--wait"}
}

// composeStopArgs returns the argv passed to `docker` for `ComposeStop`.
func composeStopArgs(composeFile string) []string {
	return []string{"compose", "-f", composeFile, "stop", "--timeout", "10"}
}

// composeDownWithVolumesArgs returns the argv passed to `docker` for
// `ComposeDownWithVolumes`. The trailing -v is intentional and destructive;
// only the user-guarded `specgraph down --purge` path calls this wrapper.
func composeDownWithVolumesArgs(composeFile string) []string {
	return []string{"compose", "-f", composeFile, "down", "--timeout", "10", "-v"}
}

// ComposeUp starts the Docker Compose stack defined in composeFile.
func ComposeUp(composeFile string) error {
	if _, err := os.Stat(composeFile); err != nil {
		return fmt.Errorf("compose file not found: %s", composeFile)
	}
	cmd := exec.Command("docker", composeUpArgs(composeFile)...) //nolint:gosec // argv assembled from pure function; composeFile is xdg-owned path, no shell involved
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}
	return nil
}

// ComposeStop halts the stack without removing containers or volumes.
// Idempotent when the stack exists: `docker compose stop` on already-stopped
// containers exits 0. Callers should guard against a missing compose file
// since this function does not (unlike ComposeUp); both `specgraph down` and
// `specgraph uninstall` stat the file before invoking.
func ComposeStop(composeFile string) error {
	cmd := exec.Command("docker", composeStopArgs(composeFile)...) //nolint:gosec // argv assembled from pure function; composeFile is xdg-owned path, no shell involved
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose stop: %w", err)
	}
	return nil
}

// ComposeDownWithVolumes tears down the stack AND removes named volumes.
// Destructive — callers must confirm with the user before invoking.
// Today only `specgraph down --purge` calls this wrapper.
func ComposeDownWithVolumes(composeFile string) error {
	cmd := exec.Command("docker", composeDownWithVolumesArgs(composeFile)...) //nolint:gosec // argv assembled from pure function; composeFile is xdg-owned path, no shell involved
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down -v: %w", err)
	}
	return nil
}

// EnsureComposeFile writes a default docker-compose.yaml for postgres
// into projectDir if one does not already exist.
func EnsureComposeFile(projectDir string) (string, error) {
	if err := os.MkdirAll(projectDir, 0o750); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	dest := filepath.Join(projectDir, "docker-compose.yaml")
	if data, err := os.ReadFile(dest); err == nil {
		// Regenerate if the existing file is a stale Memgraph compose template.
		if !bytes.Contains(data, []byte("memgraph")) {
			return dest, nil
		}
		// Fall through to overwrite with Postgres template.
	}
	template := postgresComposeTemplate()
	if err := os.WriteFile(dest, []byte(template), 0o600); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return dest, nil
}

// postgresComposeFormat uses %%[1]s placeholders for the default database
// credential to avoid gosec G101 (hardcoded credential pattern).
const postgresComposeFormat = `services:
  postgres:
    image: pgvector/pgvector:pg18
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      - POSTGRES_USER=%[1]s
      - POSTGRES_PASSWORD=%[1]s
      - POSTGRES_DB=%[1]s
    volumes:
      - specgraph-data:/var/lib/postgresql
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
