// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
)

// CreateSyncMapping implements storage.SyncBackend.
func (s *Store) CreateSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, externalID string) (*storage.SyncMapping, error) {
	nowStr := s.now()
	adapterStr := string(adapter)

	// Verify spec exists
	specRecords, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.id`,
		mergeParams(s.projectParam(), map[string]any{"slug": specSlug}),
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}
	if len(specRecords) == 0 {
		return nil, fmt.Errorf("memgraph: create sync mapping %q: %w", specSlug, storage.ErrSpecNotFound)
	}

	specID, err := recordString(specRecords[0], 0, "id")
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}

	// Atomic MERGE to avoid TOCTOU race on concurrent sync.
	// MERGE on ExternalRef ensures at most one node per (external_id, adapter) combo
	// even under concurrent writes.
	records, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		 OPTIONAL MATCH (s)-[existing:SYNCED_TO {adapter: $adapter}]->(:ExternalRef)
		 WITH p, s, existing
		 WHERE existing IS NULL
		 MERGE (p)<-[:BELONGS_TO]-(e:ExternalRef {external_id: $external_id, adapter: $adapter})
		 ON CREATE SET e.created_at = $now
		 CREATE (s)-[r:SYNCED_TO {
		   adapter: $adapter,
		   external_id: $external_id,
		   state: $state,
		   error_message: "",
		   last_sync: $now,
		   created_at: $now
		 }]->(e)
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		mergeParams(s.projectParam(), map[string]any{
			"slug":        specSlug,
			"external_id": externalID,
			"adapter":     adapterStr,
			"state":       string(storage.SyncStateSynced),
			"now":         nowStr,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: create sync mapping %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingExists)
	}

	return recordToSyncMapping(records[0], specID)
}

// UpdateSyncState implements storage.SyncBackend.
func (s *Store) UpdateSyncState(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, state storage.SyncStateType, errorMessage string) (*storage.SyncMapping, error) {
	nowStr := s.now()
	adapterStr := string(adapter)

	records, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 SET r.state = $state,
		     r.error_message = $error_message,
		     r.last_sync = $now
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		mergeParams(s.projectParam(), map[string]any{
			"slug":          specSlug,
			"adapter":       adapterStr,
			"state":         string(state),
			"error_message": errorMessage,
			"now":           nowStr,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update sync state: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: update sync state %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingNotFound)
	}

	specID, err := recordString(records[0], 0, "id")
	if err != nil {
		return nil, fmt.Errorf("memgraph: update sync state: %w", err)
	}
	return recordToSyncMapping(records[0], specID)
}

// GetSyncMapping implements storage.SyncBackend.
func (s *Store) GetSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType) (*storage.SyncMapping, error) {
	adapterStr := string(adapter)

	records, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		mergeParams(s.projectParam(), map[string]any{"slug": specSlug, "adapter": adapterStr}),
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get sync mapping: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: get sync mapping %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingNotFound)
	}

	specID, err := recordString(records[0], 0, "id")
	if err != nil {
		return nil, fmt.Errorf("memgraph: get sync mapping: %w", err)
	}
	return recordToSyncMapping(records[0], specID)
}

// ListSyncMappings implements storage.SyncBackend.
func (s *Store) ListSyncMappings(ctx context.Context, adapter storage.SyncAdapterType, specSlug string) ([]*storage.SyncMapping, error) {
	// SECURITY: All conditions MUST use Cypher parameter placeholders ($param).
	// Values are passed via the params map, not interpolated into the query string.
	// The fmt.Sprintf below only injects the WHERE clause structure (hardcoded
	// condition strings), never user input. Do NOT add dynamic field names or
	// user-supplied strings to the conditions slice.
	var conditions []string
	params := s.projectParam()

	if specSlug != "" {
		conditions = append(conditions, "s.slug = $slug")
		params["slug"] = specSlug
	}
	if adapter != "" {
		conditions = append(conditions, "r.adapter = $adapter")
		params["adapter"] = string(adapter)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Safe: where clause contains only hardcoded condition strings; values are parameterized.
	query := fmt.Sprintf(
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)-[r:SYNCED_TO]->(e:ExternalRef)%s
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at
		 ORDER BY r.last_sync DESC`,
		where,
	)

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list sync mappings: %w", err)
	}

	mappings := make([]*storage.SyncMapping, 0, len(records))
	for i, rec := range records {
		specID, err := recordString(rec, 0, "id")
		if err != nil {
			return nil, fmt.Errorf("memgraph: list sync mappings: %w", err)
		}
		m, mErr := recordToSyncMapping(rec, specID)
		if mErr != nil {
			return nil, fmt.Errorf("scan sync mapping at index %d: %w", i, mErr)
		}
		mappings = append(mappings, m)
	}
	return mappings, nil
}

// DeleteSyncMapping implements storage.SyncBackend.
func (s *Store) DeleteSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType) error {
	adapterStr := string(adapter)

	// Delete the relationship first, then clean up the orphaned ExternalRef
	// in a separate query. Memgraph 2.4 doesn't support anonymous node
	// patterns or EXISTS subqueries in WHERE after WITH+DELETE.
	_, err := s.executeQuery(ctx,
		`MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 DELETE r`,
		mergeParams(s.projectParam(), map[string]any{"slug": specSlug, "adapter": adapterStr}),
	)
	if err != nil {
		return fmt.Errorf("memgraph: delete sync mapping: %w", err)
	}
	// Remove ExternalRef nodes with no remaining incoming SYNCED_TO edges.
	// Uses OPTIONAL MATCH + WHERE r IS NULL because Memgraph 2.4 doesn't
	// support anonymous node patterns in WHERE predicates.
	_, err = s.executeQuery(ctx,
		`MATCH (e:ExternalRef)
		 OPTIONAL MATCH (other)-[r:SYNCED_TO]->(e)
		 WITH e, r
		 WHERE r IS NULL
		 DETACH DELETE e`,
		nil,
	)
	if err != nil {
		return fmt.Errorf("memgraph: delete sync mapping: %w", err)
	}
	// Idempotent: deleting a non-existent mapping is a no-op.
	return nil
}

func recordToSyncMapping(rec *neo4j.Record, specID string) (*storage.SyncMapping, error) {
	m := &storage.SyncMapping{
		SpecID: specID,
	}

	var err error
	m.SpecSlug, err = recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}

	adapterStr, err := recordString(rec, 2, "adapter")
	if err != nil {
		return nil, err
	}
	m.Adapter = storage.SyncAdapterType(adapterStr)

	m.ExternalID, err = recordString(rec, 3, "external_id")
	if err != nil {
		return nil, err
	}

	stateStr, err := recordString(rec, 4, "state")
	if err != nil {
		return nil, err
	}
	m.State = storage.SyncStateType(stateStr)

	m.ErrorMessage, err = recordString(rec, 5, "error_message")
	if err != nil {
		return nil, err
	}

	lastSyncStr, err := recordString(rec, 6, "last_sync")
	if err != nil {
		return nil, err
	}
	m.LastSync, err = parseRFC3339("last_sync", lastSyncStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse last_sync: %w", err)
	}

	createdStr, err := recordString(rec, 7, "created_at")
	if err != nil {
		return nil, err
	}
	m.CreatedAt, err = parseRFC3339("created_at", createdStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse created_at: %w", err)
	}

	return m, nil
}
