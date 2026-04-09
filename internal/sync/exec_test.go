// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/sync"
	"github.com/stretchr/testify/require"
)

func TestExecRunner_Echo(t *testing.T) {
	runner := sync.NewExecRunner()
	output, err := runner.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	require.Contains(t, string(output), "hello")
}

func TestExecRunner_NotFound(t *testing.T) {
	runner := sync.NewExecRunner()
	_, err := runner.Run(context.Background(), "nonexistent-binary-xyz")
	require.Error(t, err)
}

func TestExecRunner_ContextCancellation(t *testing.T) {
	runner := sync.NewExecRunner()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := runner.Run(ctx, "sleep", "60")
	require.Error(t, err)
}

func TestExecRunner_DefaultTimeout(t *testing.T) {
	// Exercises the !ok branch in exec.go where context.Background() (no deadline)
	// triggers the internal default timeout. Uses a fast command to verify the
	// code path without waiting for the actual 30s timeout.
	runner := sync.NewExecRunner()
	ctx := context.Background() // no deadline — triggers default timeout path
	out, err := runner.Run(ctx, "echo", "default-timeout-path")
	require.NoError(t, err)
	require.Contains(t, string(out), "default-timeout-path")
}

func TestExecRunner_Stderr(t *testing.T) {
	runner := sync.NewExecRunner()
	_, err := runner.Run(context.Background(), "sh", "-c", "echo error >&2; exit 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error")
}
