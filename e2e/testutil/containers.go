// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartMemgraph launches a Memgraph container and returns the bolt URI.
// The returned cleanup function terminates the container.
func StartMemgraph(ctx context.Context) (string, func(), error) {
	req := testcontainers.ContainerRequest{
		Image:        "memgraph/memgraph:latest",
		ExposedPorts: []string{"7687/tcp"},
		WaitingFor:   wait.ForListeningPort("7687/tcp").WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start memgraph container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get container host: %w", err)
	}
	port, err := container.MappedPort(ctx, "7687")
	if err != nil {
		_ = container.Terminate(ctx)
		return "", nil, fmt.Errorf("get mapped port: %w", err)
	}

	boltURI := fmt.Sprintf("bolt://%s:%s", host, port.Port())
	cleanup := func() { _ = container.Terminate(ctx) }
	return boltURI, cleanup, nil
}
