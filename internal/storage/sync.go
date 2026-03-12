// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"
)

// Sync-specific sentinel errors.
var (
	ErrSyncMappingNotFound = errors.New("sync mapping not found")
	ErrSyncMappingExists   = errors.New("sync mapping already exists for this spec and adapter")
)

// SyncAdapterType identifies the external system a spec is synced to.
type SyncAdapterType string

// SyncAdapterType values.
const (
	SyncAdapterBeads  SyncAdapterType = "beads"
	SyncAdapterGitHub SyncAdapterType = "github"
)

// SyncStateType tracks the sync status of a mapping.
type SyncStateType string

// SyncStateType values.
const (
	SyncStatePending  SyncStateType = "pending"
	SyncStateSynced   SyncStateType = "synced"
	SyncStateConflict SyncStateType = "conflict"
	SyncStateError    SyncStateType = "error"
)

// InjectToolType identifies the target tool for injection.
type InjectToolType string

// InjectToolType values.
const (
	InjectToolClaudeCode InjectToolType = "claude-code"
	InjectToolCursor     InjectToolType = "cursor"
	InjectToolAgentsMD   InjectToolType = "agents-md"
)

// SyncMapping represents the relationship between a spec and an external reference.
type SyncMapping struct {
	SpecID       string
	SpecSlug     string
	Adapter      SyncAdapterType
	ExternalID   string
	State        SyncStateType
	ErrorMessage string
	LastSync     time.Time
	CreatedAt    time.Time
}

// SyncResult captures the outcome of a single sync operation.
type SyncResult struct {
	SpecSlug   string
	ExternalID string
	State      SyncStateType
	Message    string
}

// SyncBackend defines storage operations for sync state tracking.
type SyncBackend interface {
	// CreateSyncMapping stores a new sync mapping between a spec and an external reference.
	// Returns ErrSpecNotFound if the spec does not exist, and ErrSyncMappingExists
	// if a mapping already exists for this spec+adapter pair.
	CreateSyncMapping(ctx context.Context, specSlug string, adapter SyncAdapterType, externalID string) (*SyncMapping, error)

	// UpdateSyncState updates the sync state and last_sync timestamp for an existing mapping.
	// Returns ErrSyncMappingNotFound if no mapping exists for this spec+adapter pair.
	UpdateSyncState(ctx context.Context, specSlug string, adapter SyncAdapterType, state SyncStateType, errorMessage string) (*SyncMapping, error)

	// GetSyncMapping retrieves a sync mapping by spec slug and adapter.
	// Returns ErrSyncMappingNotFound if no mapping exists.
	GetSyncMapping(ctx context.Context, specSlug string, adapter SyncAdapterType) (*SyncMapping, error)

	// ListSyncMappings returns all sync mappings, optionally filtered by adapter or spec slug.
	// Pass empty strings to skip filtering.
	ListSyncMappings(ctx context.Context, adapter SyncAdapterType, specSlug string) ([]*SyncMapping, error)

	// DeleteSyncMapping removes a sync mapping.
	// Idempotent: deleting a non-existent mapping is a no-op and returns nil.
	DeleteSyncMapping(ctx context.Context, specSlug string, adapter SyncAdapterType) error
}
