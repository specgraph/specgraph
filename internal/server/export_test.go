// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"

	"github.com/specgraph/specgraph/internal/storage"
)

// ScopeStore is a test export of the unexported scopeStore function.
func ScopeStore(ctx context.Context, scoper storage.Scoper) (storage.ScopedBackend, error) {
	return scopeStore(ctx, scoper)
}
