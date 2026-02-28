package docker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

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

func ComposeDown(composeFile string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose down: %w", err)
	}
	return nil
}

func EnsureComposeFile(projectDir, backend string) (string, error) {
	sgDir := filepath.Join(projectDir, ".specgraph")
	if err := os.MkdirAll(sgDir, 0755); err != nil {
		return "", fmt.Errorf("create .specgraph dir: %w", err)
	}
	dest := filepath.Join(sgDir, "docker-compose.yaml")
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	template := memgraphComposeTemplate
	if backend == "postgres" {
		template = postgresComposeTemplate
	}
	if err := os.WriteFile(dest, []byte(template), 0644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return dest, nil
}

const memgraphComposeTemplate = `services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "${SPECGRAPH_PORT:-9090}:9090"
    depends_on:
      memgraph:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=memgraph
      - SPECGRAPH_STORAGE_BOLT_URI=bolt://memgraph:7687

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

const postgresComposeTemplate = `services:
  specgraph:
    image: ghcr.io/seanb4t/specgraph:latest
    ports:
      - "${SPECGRAPH_PORT:-9090}:9090"
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      - SPECGRAPH_STORAGE_BACKEND=postgres
      - SPECGRAPH_STORAGE_POSTGRES_DSN=postgres://specgraph:specgraph@postgres:5432/specgraph?sslmode=disable

  postgres:
    image: apache/age:latest
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    environment:
      - POSTGRES_USER=specgraph
      - POSTGRES_PASSWORD=specgraph
      - POSTGRES_DB=specgraph
    volumes:
      - specgraph-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U specgraph -d specgraph"]
      interval: 5s
      timeout: 10s
      retries: 5

volumes:
  specgraph-data:
`
