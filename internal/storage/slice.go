// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// Slice represents a decomposition unit — a discrete work item produced by
// the Decompose authoring stage. Slices are first-class graph vertices.
type Slice struct {
	ID         string
	Slug       string      // parent-slug/slice-id (full slug)
	ParentSlug string      // parent spec slug
	SliceID    string      // local ID within decomposition
	Intent     string      // what this slice accomplishes
	Verify     []string    // conditions for completion
	Touches    []string    // files/packages modified
	DependsOn  []string    // full sibling slice slugs (resolved at creation)
	Status     SliceStatus // open, claimed, done
	AssignedTo string      // who claimed it (empty if unclaimed)
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// SliceStatus represents the lifecycle state of a decomposition slice.
type SliceStatus string

const (
	// SliceStatusOpen is the initial state of a slice.
	SliceStatusOpen SliceStatus = "open"
	// SliceStatusClaimed indicates someone is working on the slice.
	SliceStatusClaimed SliceStatus = "claimed"
	// SliceStatusDone indicates the slice is complete.
	SliceStatusDone SliceStatus = "done"
)

// SliceBackend provides CRUD operations for decomposition slices.
type SliceBackend interface {
	// CreateSlice persists a new Slice node in the graph with BELONGS_TO and COMPOSES edges.
	CreateSlice(ctx context.Context, slice *Slice) error
	// ListSlices returns all slices for a parent spec, ordered by creation time.
	ListSlices(ctx context.Context, parentSlug string) ([]*Slice, error)
	// GetSlice returns a single slice by its full slug.
	GetSlice(ctx context.Context, slug string) (*Slice, error)
	// ClaimSlice transitions a slice to claimed status and records the assignee.
	ClaimSlice(ctx context.Context, slug, assignee string) (*Slice, error)
	// CompleteSlice transitions a slice to done status.
	CompleteSlice(ctx context.Context, slug string) (*Slice, error)
}
