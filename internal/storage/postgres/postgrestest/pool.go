// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

// Package postgrestest provides an importable testcontainers Postgres pool
// shared across the identity integration suites (storage, auth, server,
// bootstrap). It exists because the per-package external test packages cannot
// reach one another's TestMain; this package centralizes the container
// bootstrap behind an exported SharedPool.
package postgrestest

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	once     sync.Once
	sharedCS string
	startErr error
)

// SharedPool starts the shared testcontainer Postgres on first call (per test
// binary) and returns a fresh pool against it. The pool is closed via
// t.Cleanup; the container lives for the process lifetime.
//
// Callers are responsible for running any schema migrations they need (e.g.
// postgres.New runs spec migrations; postgres.NewAuth runs auth migrations).
func SharedPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	once.Do(func() {
		sharedCS, startErr = startContainer(ctx)
	})
	require.NoError(t, startErr, "start shared test container")

	pool, err := pgxpool.New(ctx, sharedCS)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// ConnString returns the connection string for the shared container, starting
// it if not already running. Useful for in-package tests that open pools
// directly (e.g. postgres_test.go's newStore).
func ConnString(ctx context.Context) (string, error) {
	once.Do(func() {
		sharedCS, startErr = startContainer(ctx)
	})
	return sharedCS, startErr
}

// startContainer starts the pgvector/pgvector:pg18 testcontainer and returns
// the connection string. This is a verbatim relocation of the container-start
// body from postgres_test.go's TestMain: same image, same wait strategy, same
// credentials.
func startContainer(ctx context.Context) (string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg18",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", fmt.Errorf("start postgres container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", fmt.Errorf("get container host: %w", err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", fmt.Errorf("get mapped port: %w", err)
	}

	return fmt.Sprintf("postgres://test:test@%s:%s/testdb", host, port.Port()), nil
}
