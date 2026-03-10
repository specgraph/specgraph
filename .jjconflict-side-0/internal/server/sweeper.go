// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"os"
	"time"
)

// ClaimSweeper is the interface for releasing expired claims.
type ClaimSweeper interface {
	ReleaseExpiredClaims(ctx context.Context) (int, error)
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
					fmt.Fprintf(os.Stderr, "sweeper: release expired claims: %v\n", err)
					continue
				}
				if released > 0 {
					fmt.Printf("sweeper: released %d expired claim(s)\n", released)
				}
			}
		}
	}()
}
