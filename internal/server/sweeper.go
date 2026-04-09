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

// StartSweeper launches a background goroutine that periodically releases expired claims.
// It stops when the context is cancelled.
func StartSweeper(ctx context.Context, store ClaimSweeper, interval time.Duration, logger *slog.Logger) {
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
					logger.Error("release expired claims", "error", err)
					continue
				}
				if released > 0 {
					logger.Info("released expired claims", "count", released)
				}
			}
		}
	}()
}
