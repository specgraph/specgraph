// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// ChangeLogEntry records a single material change to a spec or decision.
type ChangeLogEntry struct {
	ID          string
	SpecSlug    string // populated by ListAllChanges for export
	Version     int32
	Stage       string // spec authoring stage or decision status
	ContentHash string
	Checkpoint  bool
	Summary     string
	Reason      string
	Changes     []FieldChange
	Date        time.Time
}

// ChangeLogFilter controls which changelog entries are returned.
type ChangeLogFilter struct {
	CheckpointsOnly bool
	SinceVersion    int32
	Limit           int // 0 means no limit (return all matching entries)
}

// ChangeLogBackend defines storage operations for changelog entries.
type ChangeLogBackend interface {
	// ListChanges returns changelog entries for a spec, ordered by version.
	// Returns an empty slice (not an error) if the spec has no changelog entries.
	// Returns ErrSpecNotFound if the spec slug does not exist.
	ListChanges(ctx context.Context, slug string, opts ChangeLogFilter) ([]*ChangeLogEntry, error)

	// ListAllChanges returns all changelog entries across all specs, with SpecSlug populated.
	ListAllChanges(ctx context.Context) ([]*ChangeLogEntry, error)
}
