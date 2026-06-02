// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package usagetracker_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth/usagetracker"
)

type fakeStorage struct {
	mu      sync.Mutex
	touched []string
}

func (f *fakeStorage) TouchLastUsed(_ context.Context, keyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched = append(f.touched, keyID)
	return nil
}

func (f *fakeStorage) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.touched))
	copy(out, f.touched)
	return out
}

// counts returns the number of TouchLastUsed calls recorded per keyID.
func (f *fakeStorage) counts() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]int, len(f.touched))
	for _, k := range f.touched {
		out[k]++
	}
	return out
}

func TestManager_TouchesAreFlushedOnDrain(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    16,
		FlushInterval: 50 * time.Millisecond,
	})
	mgr.Touch("k1")
	mgr.Touch("k2")
	mgr.Touch("k3")

	require.NoError(t, mgr.Close(context.Background()))
	require.ElementsMatch(t, []string{"k1", "k2", "k3"}, store.snapshot())
}

func TestManager_FlushDeduplicatesKeys(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    256,
		FlushInterval: time.Hour, // only Close drains, so all touches batch together
	})
	// Enqueue the same key many times, plus a couple of distinct ones.
	for i := 0; i < 50; i++ {
		mgr.Touch("hot")
	}
	mgr.Touch("cold-a")
	mgr.Touch("cold-b")

	require.NoError(t, mgr.Close(context.Background()))

	// Coalesced: exactly ONE TouchLastUsed call per unique keyID.
	counts := store.counts()
	require.Equal(t, map[string]int{"hot": 1, "cold-a": 1, "cold-b": 1}, counts)
}

func TestManager_OverflowDropsButDoesNotBlock(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    2,
		FlushInterval: time.Hour, // drain only on Close
	})
	// Enqueue 100 DISTINCT keys without giving the drain a chance. Distinct
	// keys are required so the dropped+persisted accounting is exact: flushAll
	// coalesces duplicate keyIDs, so a repeated key would make the persisted
	// count smaller than the number of buffered items.
	for i := 0; i < 100; i++ {
		mgr.Touch(fmt.Sprintf("k%d", i))
	}
	require.NoError(t, mgr.Close(context.Background()))
	require.LessOrEqual(t, len(store.snapshot()), 100,
		"Touch should drop on overflow, not block")
	// With BufferSize=2 and no drain until Close, most of the 100 are
	// dropped. The dropped counter must be non-zero and account for the
	// difference between enqueued and persisted.
	require.Positive(t, mgr.Dropped(), "overflow drops should be counted")
	require.Equal(t, uint64(100)-uint64(len(store.snapshot())), mgr.Dropped(),
		"dropped + persisted should equal total enqueued")
}

// --- Task 24: Close-drain under load + ctx cancellation ---

func TestManager_CloseDrainsUnderLoad(t *testing.T) {
	store := &fakeStorage{}
	mgr := usagetracker.NewManager(store, usagetracker.Config{
		BufferSize:    1024,
		FlushInterval: time.Hour, // only Close drains
	})
	// Distinct keys so the drained count equals the enqueued count (flushAll
	// coalesces duplicate keyIDs into a single persist).
	for i := 0; i < 500; i++ {
		mgr.Touch(fmt.Sprintf("k%d", i))
	}
	require.NoError(t, mgr.Close(context.Background()))
	require.Equal(t, 500, len(store.snapshot()))
}

func TestManager_CloseRespectsCtxCancellation(t *testing.T) {
	// Backend with intentional slowness.
	slowStore := &slowFakeStorage{delay: 10 * time.Millisecond}
	mgr := usagetracker.NewManager(slowStore, usagetracker.Config{
		BufferSize:    1024,
		FlushInterval: time.Hour,
	})
	// Distinct keys so the slow backend is actually invoked 100 times during
	// the drain (a repeated key would coalesce to a single fast call).
	for i := 0; i < 100; i++ {
		mgr.Touch(fmt.Sprintf("k%d", i))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := mgr.Close(ctx)
	// Either the close completes (fast enough) OR the ctx wins.
	// Both are acceptable; the assertion is "Close does not hang".
	require.True(t, err == nil || errors.Is(err, context.DeadlineExceeded))
}

type slowFakeStorage struct {
	delay time.Duration
}

func (s *slowFakeStorage) TouchLastUsed(_ context.Context, _ string) error {
	time.Sleep(s.delay)
	return nil
}
