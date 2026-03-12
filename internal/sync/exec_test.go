// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"testing"

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
