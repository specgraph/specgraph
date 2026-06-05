// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/server"
	"github.com/stretchr/testify/assert"
)

type mockSweeper struct {
	calls atomic.Int32
}

func (m *mockSweeper) ReleaseExpiredClaims(_ context.Context) (int, error) {
	m.calls.Add(1)
	return 0, nil
}

func TestSweeper_RunsOnInterval(t *testing.T) {
	mock := &mockSweeper{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server.StartSweeper(ctx, mock, 50*time.Millisecond)

	// Poll until the sweeper has run at least twice, with a hard timeout.
	deadline := time.After(500 * time.Millisecond)
	for mock.calls.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("sweeper ran %d times, expected at least 2", mock.calls.Load())
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

func TestSweeper_StopsOnCancel(t *testing.T) {
	mock := &mockSweeper{}
	ctx, cancel := context.WithCancel(context.Background())

	server.StartSweeper(ctx, mock, 50*time.Millisecond)

	// Wait for the first sweep to actually run, with a timeout.
	deadline := time.After(500 * time.Millisecond)
	for mock.calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("sweeper did not run within timeout")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	cancel()

	// Record call count after cancel and verify it doesn't increase.
	callsAfterCancel := mock.calls.Load()
	time.Sleep(150 * time.Millisecond)
	assert.Equal(t, callsAfterCancel, mock.calls.Load(), "sweeper should not run after cancel")
}
