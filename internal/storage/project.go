// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"
)

// ErrProjectNotFound is returned when no project exists with the given slug.
var ErrProjectNotFound = errors.New("project not found")

// Project represents a registered project in the spec graph.
type Project struct {
	Slug         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SyncAdapters []string // e.g. ["beads", "github"]
	GitHubRepo   string   // owner/repo for GitHub adapter (optional)
}

// ProjectBackend defines storage operations for project management.
type ProjectBackend interface {
	// GetProject returns a project by slug.
	GetProject(ctx context.Context, slug string) (*Project, error)

	// EnsureProject creates a project if it doesn't exist, or returns the existing one.
	EnsureProject(ctx context.Context, slug string) (*Project, error)

	// UpdateProject updates mutable project fields.
	UpdateProject(ctx context.Context, slug string, adapters []string, ghRepo string) (*Project, error)

	// ListProjects returns all registered projects.
	ListProjects(ctx context.Context) ([]*Project, error)

	// WipeProjectData deletes all entities belonging to the project (specs,
	// decisions, slices, findings, changelogs, conversations, sync mappings,
	// execution events, constitution) and all edges between them. The Project
	// node itself is preserved.
	WipeProjectData(ctx context.Context) error
}
