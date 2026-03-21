// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.FindingsBackend = (*Store)(nil)

// StoreFindings persists analytical pass findings for a spec.
// Existing findings for the given (slug, passType) are atomically replaced
// via RunInTransaction: delete-then-create ensures no stale findings remain.
func (s *Store) StoreFindings(ctx context.Context, slug string, passType storage.PassType, findings []storage.AnalyticalFinding) ([]string, error) {
	var ids []string
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify spec exists and capture its version.
		spec, txErr := s.GetSpec(txCtx, slug)
		if txErr != nil {
			return txErr
		}

		// Delete existing findings for this (slug, pass_type).
		deleteQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:HAS_FINDING]->(f:Finding {pass_type: $pass_type})
			DETACH DELETE f
		`
		_, txErr = s.executeQuery(txCtx, deleteQuery,
			mergeParams(s.projectParam(), map[string]any{
				"slug":      slug,
				"pass_type": string(passType),
			}))
		if txErr != nil {
			return fmt.Errorf("memgraph: delete findings: %w", txErr)
		}

		// Create new Finding nodes with HAS_FINDING edges.
		ids = make([]string, 0, len(findings))
		for i := range findings {
			f := &findings[i]
			id := newID("fn")
			nowStr := s.now()
			createQuery := `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
				CREATE (s)-[:HAS_FINDING]->(f:Finding {
					id: $id,
					pass_type: $pass_type,
					severity: $severity,
					summary: $summary,
					detail: $detail,
					constraint_ref: $constraint_ref,
					resolution: $resolution,
					version: $version,
					created_at: $created_at
				})
				RETURN f.id
			`
			_, txErr = s.executeQuery(txCtx, createQuery,
				mergeParams(s.projectParam(), map[string]any{
					"slug":           slug,
					"id":             id,
					"pass_type":      string(passType),
					"severity":       string(f.Severity),
					"summary":        f.Summary,
					"detail":         f.Detail,
					"constraint_ref": f.Constraint,
					"resolution":     f.Resolution,
					"version":        int64(spec.Version),
					"created_at":     nowStr,
				}))
			if txErr != nil {
				return fmt.Errorf("memgraph: create finding: %w", txErr)
			}
			ids = append(ids, id)
		}

		return nil
	})
	return ids, err
}

// ListFindings retrieves analytical pass findings for a spec.
// If passType is empty, all findings for the spec are returned.
// Results are ordered by created_at. Returns an empty slice (not nil) when
// no matches are found. Returns ErrSpecNotFound if the spec does not exist.
func (s *Store) ListFindings(ctx context.Context, slug string, passType storage.PassType) ([]storage.AnalyticalFinding, error) {
	// Verify spec exists before listing findings.
	if _, err := s.GetSpec(ctx, slug); err != nil {
		return nil, err
	}

	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	query := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:HAS_FINDING]->(f:Finding)`
	if passType != "" {
		query += ` WHERE f.pass_type = $pass_type`
		params["pass_type"] = string(passType)
	}
	query += `
		RETURN f.id, f.pass_type, f.severity, f.summary, f.detail,
		       f.constraint_ref, f.resolution, f.version, f.created_at
		ORDER BY f.created_at
	`

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list findings: %w", err)
	}

	findings := make([]storage.AnalyticalFinding, 0, len(records))
	for _, rec := range records {
		id, err := recordString(rec, 0, "f.id")
		if err != nil {
			return nil, err
		}
		pt, err := recordString(rec, 1, "f.pass_type")
		if err != nil {
			return nil, err
		}
		severity, err := recordString(rec, 2, "f.severity")
		if err != nil {
			return nil, err
		}
		summary, err := recordString(rec, 3, "f.summary")
		if err != nil {
			return nil, err
		}
		detail, err := recordStringOptional(rec, 4, "f.detail")
		if err != nil {
			return nil, err
		}
		constraintRef, err := recordStringOptional(rec, 5, "f.constraint_ref")
		if err != nil {
			return nil, err
		}
		resolution, err := recordStringOptional(rec, 6, "f.resolution")
		if err != nil {
			return nil, err
		}
		version, err := recordInt64(rec, 7, "f.version")
		if err != nil {
			return nil, err
		}
		createdAtStr, err := recordString(rec, 8, "f.created_at")
		if err != nil {
			return nil, err
		}
		createdAt, err := parseRFC3339("f.created_at", createdAtStr)
		if err != nil {
			return nil, err
		}

		findings = append(findings, storage.AnalyticalFinding{
			ID:         id,
			PassType:   storage.PassType(pt),
			Severity:   storage.FindingSeverity(severity),
			Summary:    summary,
			Detail:     detail,
			Constraint: constraintRef,
			Resolution: resolution,
			Version:    safeInt32(version),
			CreatedAt:  createdAt,
		})
	}

	return findings, nil
}
