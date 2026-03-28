// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "context"

// ScopedBackend combines all storage interfaces into a single type.
// Returned by Scoper.Scoped() for per-request project-scoped access.
type ScopedBackend interface {
	Backend
	GraphBackend
	DecisionBackend
	ClaimBackend
	ConstitutionBackend
	AuthoringBackend
	FindingsBackend
	ExecutionBackend
	LifecycleBackend
	SyncBackend
	ProjectBackend
	SliceBackend
	ConversationBackend
	ChangeLogBackend
}

// Scoper creates project-scoped storage instances.
type Scoper interface {
	// Scoped returns a ScopedBackend bound to the given project.
	// The returned backend shares the underlying connection but scopes
	// all queries to the specified project.
	Scoped(ctx context.Context, project string) (ScopedBackend, error)
}
