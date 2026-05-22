// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package prime

import "github.com/specgraph/specgraph/internal/storage"

// Backend is the subset of storage methods the Composer reads from. It is
// modelled the same way as internal/export.Backend: a wide aggregate of
// the storage interfaces a single feature needs, so the package can be
// satisfied either by storage.ScopedBackend (production) or by a stub
// (tests).
type Backend interface {
	storage.Backend
	storage.GraphBackend
	storage.ConstitutionBackend
	storage.FindingsBackend
	storage.ExecutionBackend
	storage.DecisionBackend
	storage.SliceBackend
	storage.ClaimBackend
}
