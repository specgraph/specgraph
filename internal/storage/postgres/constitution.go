// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/specgraph/specgraph/internal/storage"
)

// constitutionData is the intermediate struct marshaled into the JSONB data column.
// It excludes identity/version fields that are stored as explicit columns.
type constitutionData struct {
	Tech         *storage.TechStack    `json:"tech,omitempty"`
	Principles   []storage.Principle   `json:"principles,omitempty"`
	Process      *storage.ProcessConfig `json:"process,omitempty"`
	Constraints  []string              `json:"constraints,omitempty"`
	Antipatterns []storage.Antipattern `json:"antipatterns,omitempty"`
	References   []storage.Reference   `json:"references,omitempty"`
}

// GetConstitution returns the active constitution for the current project.
// Returns ErrConstitutionNotFound if none exists.
func (s *Store) GetConstitution(ctx context.Context) (*storage.Constitution, error) {
	var (
		id        string
		layer     string
		name      string
		version   int32
		dataJSON  []byte
		createdAt time.Time
		updatedAt time.Time
	)

	err := s.queryRow(ctx,
		`SELECT id, layer, name, version, data, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1
		 ORDER BY version DESC LIMIT 1`,
		s.project,
	).Scan(&id, &layer, &name, &version, &dataJSON, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get constitution: %w", err)
	}

	return constitutionFromRow(id, layer, name, version, dataJSON, createdAt, updatedAt)
}

// UpdateConstitution stores or replaces the constitution for the current project,
// incrementing its version on each update.
func (s *Store) UpdateConstitution(ctx context.Context, constitution *storage.Constitution) (*storage.Constitution, error) {
	now := s.now()

	payload := constitutionData{
		Tech:         constitution.Tech,
		Principles:   constitution.Principles,
		Process:      constitution.Process,
		Constraints:  constitution.Constraints,
		Antipatterns: constitution.Antipatterns,
		References:   constitution.References,
	}

	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("postgres: update constitution marshal: %w", err)
	}

	id := constitution.ID
	if id == "" {
		// Reuse existing constitution ID for this project so ON CONFLICT fires.
		var existingID string
		existErr := s.queryRow(ctx,
			`SELECT id FROM constitutions WHERE project_slug = $1 ORDER BY version DESC LIMIT 1`,
			s.project,
		).Scan(&existingID)
		if existErr != nil && !errors.Is(existErr, pgx.ErrNoRows) {
			return nil, fmt.Errorf("postgres: update constitution: lookup existing: %w", existErr)
		}
		if existingID != "" {
			id = existingID
		} else {
			id = newID("con")
		}
	}

	var (
		retID        string
		retLayer     string
		retName      string
		retVersion   int32
		retDataJSON  []byte
		retCreatedAt time.Time
		retUpdatedAt time.Time
	)

	err = s.queryRow(ctx,
		`INSERT INTO constitutions (id, project_slug, layer, name, version, data, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 1, $5, $6, $6)
		 ON CONFLICT (project_slug) DO UPDATE
		   SET id         = EXCLUDED.id,
		       layer      = EXCLUDED.layer,
		       name       = EXCLUDED.name,
		       data       = EXCLUDED.data,
		       version    = constitutions.version + 1,
		       updated_at = EXCLUDED.updated_at
		 RETURNING id, layer, name, version, data, created_at, updated_at`,
		id, s.project, string(constitution.Layer), constitution.Name, dataJSON, now,
	).Scan(&retID, &retLayer, &retName, &retVersion, &retDataJSON, &retCreatedAt, &retUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: update constitution: %w", err)
	}

	return constitutionFromRow(retID, retLayer, retName, retVersion, retDataJSON, retCreatedAt, retUpdatedAt)
}

// constitutionFromRow assembles a *storage.Constitution from scanned column values.
func constitutionFromRow(id, layer, name string, version int32, dataJSON []byte, createdAt, updatedAt time.Time) (*storage.Constitution, error) {
	var payload constitutionData
	if len(dataJSON) > 0 && string(dataJSON) != "{}" && string(dataJSON) != "null" {
		if err := json.Unmarshal(dataJSON, &payload); err != nil {
			return nil, fmt.Errorf("postgres: constitution unmarshal data: %w", err)
		}
	}

	c := &storage.Constitution{
		ID:           id,
		Layer:        storage.ConstitutionLayer(layer),
		Name:         name,
		Version:      version,
		Tech:         payload.Tech,
		Principles:   payload.Principles,
		Process:      payload.Process,
		Constraints:  payload.Constraints,
		Antipatterns: payload.Antipatterns,
		References:   payload.References,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	return c, nil
}
