// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"log/slog"
	"time"
)

// ClaimSweeper is the interface for releasing expired claims.
type ClaimSweeper interface {
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}

// WebAuthSweeper releases expired web sessions and login flows.
type WebAuthSweeper interface {
	DeleteExpiredSessions(ctx context.Context) (int64, error)
	DeleteExpiredLoginFlows(ctx context.Context) (int64, error)
}

// StartWebAuthSweeper launches a goroutine that periodically GCs expired web
// sessions and OIDC login flows. Stops when ctx is cancelled.
func StartWebAuthSweeper(ctx context.Context, store WebAuthSweeper, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := store.DeleteExpiredSessions(ctx); err != nil {
					slog.LogAttrs(ctx, slog.LevelError, "sweep expired sessions", slog.Any("error", err))
				} else if n > 0 {
					slog.LogAttrs(ctx, slog.LevelDebug, "swept expired sessions", slog.Int64("count", n))
				}
				if n, err := store.DeleteExpiredLoginFlows(ctx); err != nil {
					slog.LogAttrs(ctx, slog.LevelError, "sweep expired login flows", slog.Any("error", err))
				} else if n > 0 {
					slog.LogAttrs(ctx, slog.LevelDebug, "swept expired login flows", slog.Int64("count", n))
				}
			}
		}
	}()
}

// StartSweeper launches a background goroutine that periodically releases expired claims.
// It stops when the context is cancelled.
func StartSweeper(ctx context.Context, store ClaimSweeper, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				released, err := store.ReleaseExpiredClaims(ctx)
				if err != nil {
					slog.LogAttrs(ctx, slog.LevelError, "release expired claims", slog.Any("error", err))
					continue
				}
				if released > 0 {
					slog.LogAttrs(ctx, slog.LevelInfo, "released expired claims", slog.Int("count", released))
				}
			}
		}
	}()
}
