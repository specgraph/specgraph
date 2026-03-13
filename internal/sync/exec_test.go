// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/sync"
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

func TestExecRunner_Stderr(t *testing.T) {
	runner := sync.NewExecRunner()
	_, err := runner.Run(context.Background(), "sh", "-c", "echo error >&2; exit 1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "error")
}
