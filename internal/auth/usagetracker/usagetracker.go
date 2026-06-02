// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package usagetracker provides asynchronous batched last_used_at updates
// for API keys. The auth Resolver enqueues touches via Manager.Touch; a
// background goroutine drains them into TouchLastUsedBackend (typically
// storage.UsersBackend) at a configurable interval.
package usagetracker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// TouchLastUsedBackend is the storage surface the Manager uses to persist
// touches. Satisfied by storage.UsersBackend.
type TouchLastUsedBackend interface {
	TouchLastUsed(ctx context.Context, keyID string) error
}

// Config parametrizes Manager.
type Config struct {
	BufferSize    int           // channel capacity; default 256
	FlushInterval time.Duration // drain interval; default 5s
}

// Manager batches Touch calls and persists them async.
type Manager struct {
	backend TouchLastUsedBackend
	ch      chan string
	done    chan struct{}
	wg      sync.WaitGroup
	dropped atomic.Uint64 // count of touches dropped on overflow (ops visibility)
}

// NewManager constructs a Manager and starts its drain goroutine.
func NewManager(backend TouchLastUsedBackend, cfg Config) *Manager {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 256
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	m := &Manager{
		backend: backend,
		ch:      make(chan string, cfg.BufferSize),
		done:    make(chan struct{}),
	}
	m.wg.Add(1)
	go m.drain(cfg.FlushInterval)
	return m
}

// Dropped returns the cumulative count of touches dropped due to channel
// overflow. Exposed for ops visibility (metrics export, doctor checks, a
// startup-time warning if non-zero). A chronically non-zero value means
// BufferSize or FlushInterval needs tuning for the request rate.
func (m *Manager) Dropped() uint64 {
	return m.dropped.Load()
}

// Touch enqueues a key ID for async last_used_at update. Non-blocking:
// drops on channel overflow, incrementing the dropped counter and logging
// a warning.
func (m *Manager) Touch(keyID string) {
	select {
	case m.ch <- keyID:
	default:
		m.dropped.Add(1)
		slog.Warn("usagetracker: touch dropped on overflow",
			"keyID", keyID, "total_dropped", m.dropped.Load())
	}
}

func (m *Manager) drain(interval time.Duration) {
	defer m.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.done:
			m.flushAll()
			return
		case <-ticker.C:
			m.flushAll()
		}
	}
}

func (m *Manager) flushAll() {
	// Drain the channel into a set first, then persist once per unique keyID.
	// TouchLastUsed is idempotent (sets last_used_at = now), so N touches of
	// the same key within one interval coalesce into a single SQL write.
	pending := make(map[string]struct{})
	for drained := false; !drained; {
		select {
		case id := <-m.ch:
			pending[id] = struct{}{}
		default:
			drained = true
		}
	}
	if len(pending) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for id := range pending {
		// Per-key error isolation: a failed key logs and the loop continues.
		if err := m.backend.TouchLastUsed(ctx, id); err != nil {
			slog.Warn("usagetracker: TouchLastUsed failed", "keyID", id, "error", err)
		}
	}
}

// Close stops the drain goroutine, flushing any in-flight items.
// Respects ctx cancellation: returns ctx.Err() if cancelled mid-drain.
//
// Caller contract: no further Touch calls may occur after Close returns. A
// Touch after the drain goroutine has exited is silently dropped (no panic,
// but the touch is lost and not counted in Dropped()). Last-used is
// best-effort, so this is acceptable — but the contract is explicit.
func (m *Manager) Close(ctx context.Context) error {
	close(m.done)
	doneCh := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err() //nolint:wrapcheck // ctx.Err() is the sentinel; not an external error to wrap
	}
}
