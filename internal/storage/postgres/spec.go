// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/contenthash"
)

const (
	defaultInitialStage = "spark"
	defaultLifecycle    = storage.SpecLifecycleTask
)

// CreateSpec stores a new spec in Postgres and returns it.
// All DB operations run within a single transaction.
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	now := s.now()
	ch := contenthash.Spec(intent, defaultInitialStage, priority, complexity, nil)

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Guard: slug must not already exist in this project.
		var exists int
		err := s.queryRow(txCtx,
			`SELECT 1 FROM specs WHERE slug = $1 AND project_slug = $2`,
			slug, s.project,
		).Scan(&exists)
		if err == nil {
			return fmt.Errorf("slug %q: %w", slug, storage.ErrSpecAlreadyExists)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgres: check existing spec: %w", err)
		}

		// Insert spec row.
		specID := newID("spec")
		row := s.queryRow(txCtx,
			`INSERT INTO specs
				(id, slug, project_slug, intent, stage, priority, complexity, lifecycle, notes,
				 content_hash, version, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '', $9, 1, $10, $10)
			 RETURNING id, slug, project_slug, intent, stage, priority, complexity, lifecycle,
			           superseded_by, supersedes, notes, content_hash, version,
			           spark_output, shape_output, specify_output, decompose_output,
			           created_at, updated_at`,
			specID, slug, s.project, intent, defaultInitialStage, priority, complexity,
			string(defaultLifecycle), ch, now,
		)

		spec, err := scanSpec(row)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("slug %q: %w", slug, storage.ErrSpecAlreadyExists)
			}
			return fmt.Errorf("postgres: create spec: scan: %w", err)
		}

		// Initial changelog entry.
		allFields := storage.SpecFields{
			Intent:     intent,
			Stage:      defaultInitialStage,
			Priority:   priority,
			Complexity: complexity,
		}
		deltas := storage.ComputeFieldDeltas(&storage.SpecFields{}, &allFields)
		clEntry := &storage.ChangeLogEntry{
			Version:     spec.Version,
			Stage:       string(spec.Stage),
			ContentHash: spec.ContentHash,
			Checkpoint:  true,
			Summary:     "Spec created",
			Date:        spec.CreatedAt,
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}

		// BELONGS_TO edge: spec → project.
		_, err = s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'BELONGS_TO', $3)`,
			slug, s.project, s.project,
		)
		if err != nil {
			return fmt.Errorf("postgres: create BELONGS_TO edge: %w", err)
		}

		result = spec
		return nil
	})
	return result, err
}

// GetSpec retrieves a spec by slug within the store's project.
// Returns storage.ErrSpecNotFound if no match.
func (s *Store) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	row := s.queryRow(ctx,
		`SELECT s.id, s.slug, s.project_slug, s.intent, s.stage, s.priority, s.complexity,
		        s.lifecycle, s.superseded_by, s.supersedes, s.notes, s.content_hash, s.version,
		        s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		        s.created_at, s.updated_at,
		        (SELECT count(*) FROM conversation_logs
		         WHERE spec_slug = s.slug AND project_slug = s.project_slug) AS conversation_count
		 FROM specs s
		 WHERE s.slug = $1 AND s.project_slug = $2`,
		slug, s.project,
	)

	spec, err := scanSpecWithCount(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get spec %q: %w", slug, err)
	}

	convLogs, convErr := s.ListConversations(ctx, slug, "")
	if convErr != nil {
		return nil, fmt.Errorf("postgres: get spec %q: load conversations: %w", slug, convErr)
	}
	spec.ConversationLogs = convLogs

	return spec, nil
}

// scanSpec reads a Spec from a single pgx.Row without the conversation_count column.
func scanSpec(row pgx.Row) (*storage.Spec, error) {
	var (
		id              string
		slug            string
		projectSlug     string
		intent          string
		stage           string
		priority        string
		complexity      string
		lifecycle       string
		supersededBy    string
		supersedes      string
		notes           string
		contentHash     string
		version         int32
		sparkOutput     *storage.SparkOutput
		shapeOutput     *storage.ShapeOutput
		specifyOutput   *storage.SpecifyOutput
		decomposeOutput *storage.DecomposeOutput
		createdAt       time.Time
		updatedAt       time.Time
	)
	if err := row.Scan(
		&id, &slug, &projectSlug, &intent, &stage, &priority, &complexity,
		&lifecycle, &supersededBy, &supersedes, &notes, &contentHash, &version,
		&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput,
		&createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("postgres: scan spec: %w", err)
	}
	return buildSpec(id, slug, intent, stage, priority, complexity, lifecycle,
		supersededBy, supersedes, notes, contentHash, version,
		sparkOutput, shapeOutput, specifyOutput, decomposeOutput,
		createdAt, updatedAt, 0), nil
}

// scanSpecWithCount reads a Spec from a row that includes a trailing conversation_count column.
func scanSpecWithCount(row pgx.Row) (*storage.Spec, error) {
	var (
		id              string
		slug            string
		projectSlug     string
		intent          string
		stage           string
		priority        string
		complexity      string
		lifecycle       string
		supersededBy    string
		supersedes      string
		notes           string
		contentHash     string
		version         int32
		sparkOutput     *storage.SparkOutput
		shapeOutput     *storage.ShapeOutput
		specifyOutput   *storage.SpecifyOutput
		decomposeOutput *storage.DecomposeOutput
		createdAt       time.Time
		updatedAt       time.Time
		convCount       int
	)
	if err := row.Scan(
		&id, &slug, &projectSlug, &intent, &stage, &priority, &complexity,
		&lifecycle, &supersededBy, &supersedes, &notes, &contentHash, &version,
		&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput,
		&createdAt, &updatedAt,
		&convCount,
	); err != nil {
		return nil, fmt.Errorf("postgres: scan spec with count: %w", err)
	}
	return buildSpec(id, slug, intent, stage, priority, complexity, lifecycle,
		supersededBy, supersedes, notes, contentHash, version,
		sparkOutput, shapeOutput, specifyOutput, decomposeOutput,
		createdAt, updatedAt, convCount), nil
}

func buildSpec(
	id, slug, intent, stage, priority, complexity, lifecycle,
	supersededBy, supersedes, notes, contentHash string,
	version int32,
	sparkOutput *storage.SparkOutput,
	shapeOutput *storage.ShapeOutput,
	specifyOutput *storage.SpecifyOutput,
	decomposeOutput *storage.DecomposeOutput,
	createdAt, updatedAt time.Time,
	convCount int,
) *storage.Spec {
	return &storage.Spec{
		ID:                id,
		Slug:              slug,
		Intent:            intent,
		Stage:             storage.SpecStage(stage),
		Priority:          storage.SpecPriority(priority),
		Complexity:        storage.SpecComplexity(complexity),
		Version:           version,
		Lifecycle:         storage.SpecLifecycle(lifecycle),
		SupersededBy:      supersededBy,
		Supersedes:        supersedes,
		Notes:             notes,
		ContentHash:       contentHash,
		SparkOutput:       sparkOutput,
		ShapeOutput:       shapeOutput,
		SpecifyOutput:     specifyOutput,
		DecomposeOutput:   decomposeOutput,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		ConversationCount: convCount,
	}
}

// ListSpecs returns specs matching the given filters within the store's project.
// Empty filter values mean "no filter". limit=0 means no limit.
func (s *Store) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error) {
	rows, err := s.query(ctx,
		`SELECT s.id, s.slug, s.project_slug, s.intent, s.stage, s.priority, s.complexity,
		        s.lifecycle, s.superseded_by, s.supersedes, s.notes, s.content_hash, s.version,
		        s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		        s.created_at, s.updated_at,
		        (SELECT count(*) FROM conversation_logs cl
		         WHERE cl.spec_slug = s.slug AND cl.project_slug = s.project_slug) AS conversation_count
		 FROM specs s
		 WHERE s.project_slug = $1
		   AND ($2 = '' OR s.stage = $2)
		   AND ($3 = '' OR s.priority = $3)
		 ORDER BY s.created_at DESC
		 LIMIT CASE WHEN $4 > 0 THEN $4 END`,
		s.project, stage, priority, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: list specs: %w", err)
	}
	defer rows.Close()

	var specs []*storage.Spec
	for rows.Next() {
		var (
			id              string
			slug            string
			projectSlug     string
			intent          string
			stageVal        string
			priorityVal     string
			complexity      string
			lifecycle       string
			supersededBy    string
			supersedes      string
			notes           string
			contentHash     string
			version         int32
			sparkOutput     *storage.SparkOutput
			shapeOutput     *storage.ShapeOutput
			specifyOutput   *storage.SpecifyOutput
			decomposeOutput *storage.DecomposeOutput
			createdAt       time.Time
			updatedAt       time.Time
			convCount       int
		)
		if err := rows.Scan(
			&id, &slug, &projectSlug, &intent, &stageVal, &priorityVal, &complexity,
			&lifecycle, &supersededBy, &supersedes, &notes, &contentHash, &version,
			&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput,
			&createdAt, &updatedAt,
			&convCount,
		); err != nil {
			return nil, fmt.Errorf("postgres: list specs: scan: %w", err)
		}
		specs = append(specs, buildSpec(id, slug, intent, stageVal, priorityVal, complexity, lifecycle,
			supersededBy, supersedes, notes, contentHash, version,
			sparkOutput, shapeOutput, specifyOutput, decomposeOutput,
			createdAt, updatedAt, convCount))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list specs: rows: %w", err)
	}
	if specs == nil {
		specs = []*storage.Spec{}
	}
	return specs, nil
}

// BatchGetSpecs retrieves multiple specs by slug in a single query.
// Missing slugs are silently omitted from the result map.
// This is a concrete method on *Store (not in Backend interface) for batch optimization.
func (s *Store) BatchGetSpecs(ctx context.Context, slugs []string) (map[string]*storage.Spec, error) {
	if len(slugs) == 0 {
		return map[string]*storage.Spec{}, nil
	}

	rows, err := s.query(ctx,
		`SELECT s.id, s.slug, s.project_slug, s.intent, s.stage, s.priority, s.complexity,
		        s.lifecycle, s.superseded_by, s.supersedes, s.notes, s.content_hash, s.version,
		        s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		        s.created_at, s.updated_at,
		        0 AS conversation_count
		 FROM specs s
		 WHERE s.slug = ANY($1) AND s.project_slug = $2`,
		slugs, s.project,
	)
	if err != nil {
		return nil, fmt.Errorf("postgres: batch get specs: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*storage.Spec, len(slugs))
	for rows.Next() {
		var (
			id              string
			slug            string
			projectSlug     string
			intent          string
			stage           string
			priority        string
			complexity      string
			lifecycle       string
			supersededBy    string
			supersedes      string
			notes           string
			contentHash     string
			version         int32
			sparkOutput     *storage.SparkOutput
			shapeOutput     *storage.ShapeOutput
			specifyOutput   *storage.SpecifyOutput
			decomposeOutput *storage.DecomposeOutput
			createdAt       time.Time
			updatedAt       time.Time
			convCount       int
		)
		if err := rows.Scan(
			&id, &slug, &projectSlug, &intent, &stage, &priority, &complexity,
			&lifecycle, &supersededBy, &supersedes, &notes, &contentHash, &version,
			&sparkOutput, &shapeOutput, &specifyOutput, &decomposeOutput,
			&createdAt, &updatedAt,
			&convCount,
		); err != nil {
			return nil, fmt.Errorf("postgres: batch get specs: scan: %w", err)
		}
		result[slug] = buildSpec(id, slug, intent, stage, priority, complexity, lifecycle,
			supersededBy, supersedes, notes, contentHash, version,
			sparkOutput, shapeOutput, specifyOutput, decomposeOutput,
			createdAt, updatedAt, convCount)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: batch get specs: rows: %w", err)
	}
	return result, nil
}

// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
// Returns the updated spec with bumped version and updated timestamp.
// If no fields are provided, returns the current spec unchanged.
// Wraps all DB operations in a transaction with a version guard to detect concurrent modifications.
func (s *Store) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error) {
	if intent == nil && stage == nil && priority == nil && complexity == nil && notes == nil {
		return s.GetSpec(ctx, slug)
	}

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Read current spec for version guard and field delta computation.
		current, err := s.GetSpec(txCtx, slug)
		if err != nil {
			return err
		}

		// Resolve new field values (fall back to current if nil).
		newIntent := current.Intent
		if intent != nil {
			newIntent = *intent
		}
		newStage := string(current.Stage)
		if stage != nil {
			newStage = *stage
		}
		newPriority := string(current.Priority)
		if priority != nil {
			newPriority = *priority
		}
		newComplexity := string(current.Complexity)
		if complexity != nil {
			newComplexity = *complexity
		}
		newNotes := current.Notes
		if notes != nil {
			newNotes = *notes
		}
		// Recompute content hash with new field values.
		// Authoring outputs are not mutated by UpdateSpec, so pass nil.
		ch := contenthash.Spec(newIntent, newStage, newPriority, newComplexity, nil)

		// Build dynamic SET clause using positional args.
		var setClauses []string
		var args []any
		idx := 1

		setClauses = append(setClauses, "version = version + 1")

		now := s.now()
		setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", idx))
		args = append(args, now)
		idx++

		setClauses = append(setClauses, fmt.Sprintf("content_hash = $%d", idx))
		args = append(args, ch)
		idx++

		if intent != nil {
			setClauses = append(setClauses, fmt.Sprintf("intent = $%d", idx))
			args = append(args, *intent)
			idx++
		}
		if stage != nil {
			setClauses = append(setClauses, fmt.Sprintf("stage = $%d", idx))
			args = append(args, *stage)
			idx++
		}
		if priority != nil {
			setClauses = append(setClauses, fmt.Sprintf("priority = $%d", idx))
			args = append(args, *priority)
			idx++
		}
		if complexity != nil {
			setClauses = append(setClauses, fmt.Sprintf("complexity = $%d", idx))
			args = append(args, *complexity)
			idx++
		}
		if notes != nil {
			setClauses = append(setClauses, fmt.Sprintf("notes = $%d", idx))
			args = append(args, *notes)
			idx++
		}

		// Version guard: WHERE slug=$N AND project_slug=$N AND version=$N
		slugArgIdx := idx
		projArgIdx := idx + 1
		verArgIdx := idx + 2
		args = append(args, slug, s.project, current.Version)

		sql := fmt.Sprintf(
			"UPDATE specs SET %s WHERE slug = $%d AND project_slug = $%d AND version = $%d",
			strings.Join(setClauses, ", "),
			slugArgIdx, projArgIdx, verArgIdx,
		)

		tag, execErr := s.exec(txCtx, sql, args...)
		if execErr != nil {
			return fmt.Errorf("postgres: update spec: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgres: spec %q: %w", slug, storage.ErrConcurrentModification)
		}

		// Create changelog entry only if content hash changed (substantive update).
		if ch != current.ContentHash {
			oldFields := &storage.SpecFields{
				Intent:     current.Intent,
				Stage:      string(current.Stage),
				Priority:   string(current.Priority),
				Complexity: string(current.Complexity),
				Notes:      current.Notes,
			}
			newFields := &storage.SpecFields{
				Intent:     newIntent,
				Stage:      newStage,
				Priority:   newPriority,
				Complexity: newComplexity,
				Notes:      newNotes,
			}
			deltas := storage.ComputeFieldDeltas(oldFields, newFields)
			clEntry := &storage.ChangeLogEntry{
				Version:     current.Version + 1,
				Stage:       newStage,
				ContentHash: ch,
				Checkpoint:  false,
				Summary:     "Spec updated",
				Date:        now,
			}
			if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
				return clErr
			}
		}

		// When transitioning to done, refresh content_hash_at_link on all inbound
		// DEPENDS_ON edges so downstream specs see the new baseline immediately.
		if stage != nil && *stage == "done" {
			if refreshErr := s.refreshInboundDependencyHashes(txCtx, slug); refreshErr != nil {
				return fmt.Errorf("postgres: update spec: %w", refreshErr)
			}
		}

		updated, getErr := s.GetSpec(txCtx, slug)
		if getErr != nil {
			return getErr
		}
		result = updated
		return nil
	})
	return result, err
}
