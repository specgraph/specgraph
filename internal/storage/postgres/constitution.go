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
	Tech         *storage.TechStack     `json:"tech,omitempty"`
	Principles   []storage.Principle    `json:"principles,omitempty"`
	Process      *storage.ProcessConfig `json:"process,omitempty"`
	Constraints  []string               `json:"constraints,omitempty"`
	Antipatterns []storage.Antipattern  `json:"antipatterns,omitempty"`
	References   []storage.Reference    `json:"references,omitempty"`
}

// GetConstitution returns the active constitution for the current project.
// Returns ErrConstitutionNotFound if none exists.
func (s *Store) GetConstitution(ctx context.Context) (*storage.Constitution, error) {
	var (
		id         string
		layer      string
		name       string
		version    int32
		dataJSON   []byte
		sourceURL  string
		sourceHash string
		createdAt  time.Time
		updatedAt  time.Time
	)

	err := s.queryRow(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1
		 ORDER BY version DESC LIMIT 1`,
		s.project,
	).Scan(&id, &layer, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get constitution: %w", err)
	}

	return constitutionFromRow(id, layer, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
}

// GetConstitutionLayer returns a single layer's raw constitution data.
// Returns ErrConstitutionNotFound if the layer does not exist.
func (s *Store) GetConstitutionLayer(ctx context.Context, layer storage.ConstitutionLayer) (*storage.Constitution, error) {
	var (
		id         string
		layerStr   string
		name       string
		version    int32
		dataJSON   []byte
		sourceURL  string
		sourceHash string
		createdAt  time.Time
		updatedAt  time.Time
	)

	err := s.queryRow(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1 AND layer = $2`,
		s.project, string(layer),
	).Scan(&id, &layerStr, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: %w", storage.ErrConstitutionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get constitution layer: %w", err)
	}

	return constitutionFromRow(id, layerStr, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
}

// GetAllLayers returns all constitution layers for the project,
// ordered by precedence (user, org, project, domain).
// Returns an empty slice (not error) if no layers exist.
func (s *Store) GetAllLayers(ctx context.Context) ([]*storage.Constitution, error) {
	rows, err := s.query(ctx,
		`SELECT id, layer, name, version, data, source_url, source_hash, created_at, updated_at
		 FROM constitutions WHERE project_slug = $1
		 ORDER BY CASE layer
		   WHEN 'user'    THEN 1
		   WHEN 'org'     THEN 2
		   WHEN 'project' THEN 3
		   WHEN 'domain'  THEN 4
		   ELSE 5
		 END`,
		s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: get all constitution layers: %w", err)
	}
	defer rows.Close()

	var results []*storage.Constitution
	for rows.Next() {
		var (
			id         string
			layer      string
			name       string
			version    int32
			dataJSON   []byte
			sourceURL  string
			sourceHash string
			createdAt  time.Time
			updatedAt  time.Time
		)
		if err := rows.Scan(&id, &layer, &name, &version, &dataJSON, &sourceURL, &sourceHash, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("postgres: get all constitution layers scan: %w", err)
		}
		c, err := constitutionFromRow(id, layer, name, version, dataJSON, sourceURL, sourceHash, createdAt, updatedAt)
		if err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: get all constitution layers rows: %w", err)
	}

	if results == nil {
		results = []*storage.Constitution{}
	}

	return results, nil
}

// UpdateConstitution stores or replaces the constitution for the current project,
// incrementing its version on each update.
func (s *Store) UpdateConstitution(ctx context.Context, constitution *storage.Constitution) (*storage.Constitution, error) {
	now := s.now()

	layer := string(constitution.Layer)
	if layer == "" {
		layer = string(storage.ConstitutionLayerProject)
	}

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
		// Reuse existing constitution ID for this project+layer so ON CONFLICT fires.
		var existingID string
		existErr := s.queryRow(ctx,
			`SELECT id FROM constitutions WHERE project_slug = $1 AND layer = $2`,
			s.project, layer,
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
		retID         string
		retLayer      string
		retName       string
		retVersion    int32
		retDataJSON   []byte
		retSourceURL  string
		retSourceHash string
		retCreatedAt  time.Time
		retUpdatedAt  time.Time
	)

	err = s.queryRow(ctx,
		`INSERT INTO constitutions (id, project_slug, layer, name, version, data, source_url, source_hash, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 1, $5, $6, $7, $8, $8)
		 ON CONFLICT (project_slug, layer) DO UPDATE
		   SET id         = EXCLUDED.id,
		       layer      = EXCLUDED.layer,
		       name       = EXCLUDED.name,
		       data       = EXCLUDED.data,
		       source_url  = EXCLUDED.source_url,
		       source_hash = EXCLUDED.source_hash,
		       version    = constitutions.version + 1,
		       updated_at = EXCLUDED.updated_at
		 RETURNING id, layer, name, version, data, source_url, source_hash, created_at, updated_at`,
		id, s.project, layer, constitution.Name, dataJSON, constitution.SourceURL, constitution.SourceHash, now,
	).Scan(&retID, &retLayer, &retName, &retVersion, &retDataJSON, &retSourceURL, &retSourceHash, &retCreatedAt, &retUpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("postgres: update constitution: %w", err)
	}

	return constitutionFromRow(retID, retLayer, retName, retVersion, retDataJSON, retSourceURL, retSourceHash, retCreatedAt, retUpdatedAt)
}

// constitutionFromRow assembles a *storage.Constitution from scanned column values.
func constitutionFromRow(id, layer, name string, version int32, dataJSON []byte, sourceURL, sourceHash string, createdAt, updatedAt time.Time) (*storage.Constitution, error) {
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
		SourceURL:    sourceURL,
		SourceHash:   sourceHash,
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
