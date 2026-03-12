// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"
)

// BeadsAdapter syncs specs to the Beads task system via the bd CLI.
type BeadsAdapter struct {
	runner CommandRunner
}

// NewBeadsAdapter creates a BeadsAdapter with the given command runner.
func NewBeadsAdapter(runner CommandRunner) *BeadsAdapter {
	return &BeadsAdapter{runner: runner}
}

// Name returns the adapter type identifier.
func (b *BeadsAdapter) Name() storage.SyncAdapterType {
	return storage.SyncAdapterBeads
}

// Available checks whether the bd CLI is installed and reachable.
func (b *BeadsAdapter) Available() error {
	_, err := b.runner.Run(context.Background(), "bd", "--version")
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAdapterNotAvailable, err)
	}
	return nil
}

// beadsCreateResponse captures the JSON output from bd create.
type beadsCreateResponse struct {
	ID string `json:"id"`
}

// beadsShowResponse captures the JSON output from bd show.
type beadsShowResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Push creates a bead from the given spec using the bd CLI.
func (b *BeadsAdapter) Push(ctx context.Context, spec *storage.Spec) (string, error) {
	if spec.Slug == "" {
		return "", fmt.Errorf("%w: spec slug is required", ErrPushFailed)
	}
	title := fmt.Sprintf("[spec] %s", spec.Slug)
	out, err := b.runner.Run(ctx, "bd", "create",
		"--title", title,
		"--description", spec.Intent,
		"--type", "task",
		"--json",
	)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPushFailed, err)
	}

	var resp beadsCreateResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("%w: failed to parse response: %w", ErrPushFailed, err)
	}
	if resp.ID == "" {
		return "", fmt.Errorf("%w: missing bead id in response", ErrPushFailed)
	}

	return resp.ID, nil
}

// Pull retrieves the status of a bead by its external ID.
func (b *BeadsAdapter) Pull(ctx context.Context, externalID string) (string, error) {
	out, err := b.runner.Run(ctx, "bd", "show", externalID, "--json")
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPullFailed, err)
	}

	var resp beadsShowResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", fmt.Errorf("%w: failed to parse response: %w", ErrPullFailed, err)
	}
	if resp.Status == "" {
		return "", fmt.Errorf("%w: missing bead status in response", ErrPullFailed)
	}

	return resp.Status, nil
}
