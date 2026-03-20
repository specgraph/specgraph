// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package sync implements sync adapters for pushing specs to external systems.
package sync

import (
	"context"
	"errors"

	"github.com/specgraph/specgraph/internal/storage"
)

// Sentinel errors for sync adapters.
var (
	ErrAdapterNotAvailable = errors.New("adapter CLI tool not available")
	errPushFailed          = errors.New("push failed")
	errPullFailed          = errors.New("pull failed")
)

// Adapter defines the interface for syncing specs to external systems.
type Adapter interface {
	// Name returns the adapter type identifier.
	Name() storage.SyncAdapterType

	// Available checks whether the adapter's CLI tool is installed and reachable.
	Available(ctx context.Context) error

	// Push creates or updates an external work item from the given spec.
	// Returns the external system's ID for the created item.
	Push(ctx context.Context, spec *storage.Spec) (externalID string, err error)

	// Pull retrieves the current status of an external work item.
	Pull(ctx context.Context, externalID string) (status string, err error)
}

// CommandRunner abstracts shell command execution for testability.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
