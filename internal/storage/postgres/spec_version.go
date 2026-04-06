// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// GetSpecAtVersion reconstructs the spec state at a given version by walking
// changelog entries in reverse. Version 0 means latest.
// Returns ErrSpecNotFound if the slug doesn't exist.
// Returns ErrVersionNotFound if version exceeds current version.
func (s *Store) GetSpecAtVersion(ctx context.Context, slug string, version int32) (*storage.Spec, error) {
	// Get current spec — this verifies existence.
	current, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Version 0 or matching current version — return current spec as-is.
	if version == 0 || version == current.Version {
		return current, nil
	}

	// Version exceeds what exists.
	if version > current.Version {
		return nil, fmt.Errorf("postgres: get spec at version %d: %w", version, storage.ErrVersionNotFound)
	}

	// Fetch changelog entries AFTER the target version (ascending order).
	// These are the changes we need to undo to reconstruct state at `version`.
	entries, err := s.ListChanges(ctx, slug, storage.ChangeLogFilter{SinceVersion: version})
	if err != nil {
		return nil, fmt.Errorf("postgres: get spec at version: list changes: %w", err)
	}

	// Start with a copy of the current spec.
	spec := *current

	// Walk entries in REVERSE order (newest to oldest), applying each OldValue
	// to undo the change and reconstruct the state at the target version.
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		for _, change := range entry.Changes {
			applyOldValue(&spec, change.Field, change.OldValue)
		}
	}

	// Set version, stage, and content_hash from the changelog entry AT target version.
	// That entry is the first one in the entries list (lowest version > target),
	// minus one — but actually we need the entry AT target version, which is not
	// in the SinceVersion result. We reconstruct those from the undone state.
	// The version and stage are now consistent from the undo walk, but we need
	// to set Version explicitly.
	spec.Version = version

	// Find the changelog entry at exactly version `version` to get the correct
	// stage and content_hash at that point.
	allEntries, err := s.ListChanges(ctx, slug, storage.ChangeLogFilter{SinceVersion: version - 1})
	if err != nil {
		return nil, fmt.Errorf("postgres: get spec at version: list at-version entry: %w", err)
	}
	if len(allEntries) > 0 && allEntries[0].Version == version {
		spec.Stage = storage.SpecStage(allEntries[0].Stage)
		spec.ContentHash = allEntries[0].ContentHash
	}

	return &spec, nil
}

// applyOldValue applies a field's OldValue to undo a changelog change on spec.
func applyOldValue(spec *storage.Spec, field, oldValue string) {
	switch field {
	case "intent":
		spec.Intent = oldValue
	case "stage":
		spec.Stage = storage.SpecStage(oldValue)
	case "priority":
		spec.Priority = storage.SpecPriority(oldValue)
	case "complexity":
		spec.Complexity = storage.SpecComplexity(oldValue)
	case "superseded_by":
		spec.SupersededBy = oldValue
	case "supersedes":
		spec.Supersedes = oldValue
	case "notes":
		spec.Notes = oldValue
	}
}
