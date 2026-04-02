// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e || e2e_cli || e2e_agent

package testutil

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresImage is the container image used for PostgreSQL in e2e tests.
const PostgresImage = "pgvector/pgvector:pg18"

// StartPostgres launches a PostgreSQL container and returns the connection URL.
// The returned cleanup function terminates the container.
func StartPostgres(ctx context.Context) (string, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        PostgresImage,
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "specgraph",
			"POSTGRES_PASSWORD": "specgraph",
			"POSTGRES_DB":       "specgraph",
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp"),
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
		).WithDeadline(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get mapped port: %w", err)
	}

	connURL := fmt.Sprintf("postgres://specgraph:specgraph@%s:%s/specgraph?sslmode=disable", host, port.Port())

	// Retry connection to handle the window between port readiness and PG accepting queries.
	var connErr error
	for range 10 {
		pool, pErr := pgxpool.New(ctx, connURL)
		if pErr != nil {
			connErr = pErr
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if pingErr := pool.Ping(ctx); pingErr != nil {
			pool.Close()
			connErr = pingErr
			time.Sleep(500 * time.Millisecond)
			continue
		}
		pool.Close()
		connErr = nil
		break
	}
	if connErr != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("postgres connection retry exhausted: %w", connErr)
	}

	cleanup := func() {
		cleanCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := container.Terminate(cleanCtx); err != nil {
			fmt.Fprintf(os.Stderr, "testutil: container terminate error: %v\n", err)
		}
	}
	return connURL, cleanup, nil
}
