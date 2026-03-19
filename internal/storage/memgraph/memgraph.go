// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package memgraph implements storage backends using Memgraph via the Bolt protocol.
package memgraph

import (
	"context"
	"crypto/rand"
	"fmt"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/contenthash"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Compile-time interface assertions.
var (
	_ storage.AuthoringBackend = (*Store)(nil)
	_ storage.LifecycleBackend = (*Store)(nil)
	_ storage.ProjectBackend   = (*Store)(nil)
	_ storage.SyncBackend      = (*Store)(nil)
	_ storage.Scoper           = (*Store)(nil)
	_ storage.ScopedBackend    = (*Store)(nil)
)

// Store implements storage.Backend using Memgraph (Bolt protocol).
type Store struct {
	driver     neo4j.DriverWithContext
	nowFunc    func() time.Time // injectable clock; defaults to time.Now
	project    string           // project slug for graph namespacing
	ownsDriver bool             // true for stores created by New(); false for Scoped() stores
}

// Option configures a Store.
type Option func(*Store)

// WithClock overrides the default wall clock used for timestamps.
// Intended for testing — production callers should omit this option.
func WithClock(fn func() time.Time) Option {
	return func(s *Store) { s.nowFunc = fn }
}

// WithProject sets the project slug for graph namespacing.
// Required — New() returns an error if no project is set.
func WithProject(slug string) Option {
	return func(s *Store) { s.project = slug }
}

// New creates a new Memgraph-backed Store and verifies connectivity.
func New(ctx context.Context, boltURI string, opts ...Option) (*Store, error) {
	driver, err := neo4j.NewDriverWithContext(boltURI, neo4j.NoAuth())
	if err != nil {
		return nil, fmt.Errorf("memgraph: create driver: %w", err)
	}
	if connErr := driver.VerifyConnectivity(ctx); connErr != nil {
		driver.Close(ctx) //nolint:errcheck // best-effort cleanup on init failure
		return nil, fmt.Errorf("memgraph: verify connectivity: %w", connErr)
	}
	s := &Store{driver: driver, nowFunc: time.Now, ownsDriver: true}
	for _, o := range opts {
		o(s)
	}
	if s.project == "" {
		driver.Close(ctx) //nolint:errcheck // best-effort cleanup on init failure
		return nil, fmt.Errorf("memgraph: project slug required: use memgraph.WithProject(slug)")
	}
	if err := s.ensureIndexes(ctx); err != nil {
		driver.Close(ctx) //nolint:errcheck // best-effort cleanup on init failure
		return nil, fmt.Errorf("memgraph: ensure indexes: %w", err)
	}

	// Ensure the Project node exists (MERGE is idempotent).
	if err := s.ensureProjectNode(ctx); err != nil {
		driver.Close(ctx) //nolint:errcheck // best-effort cleanup on init failure
		return nil, err
	}
	return s, nil
}

// ensureIndexes creates graph indexes idempotently. Called once from New();
// Scoped() shares the driver and does not re-create indexes.
func (s *Store) ensureIndexes(ctx context.Context) error {
	indexes := []string{
		"CREATE INDEX ON :Project(slug)",
		"CREATE INDEX ON :Spec(slug)",
		"CREATE INDEX ON :Decision(slug)",
	}
	// Memgraph requires index DDL to run in individual auto-commit transactions,
	// not inside multi-statement transactions. Use a fresh session per statement.
	for _, stmt := range indexes {
		session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
		_, err := session.Run(ctx, stmt, nil)
		closeErr := session.Close(ctx)
		if err != nil && !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("create index %q: %w", stmt, err)
		}
		if closeErr != nil {
			return fmt.Errorf("close session after index %q: %w", stmt, closeErr)
		}
	}
	return s.EnsureChangeLogIndexes(ctx)
}

// ensureProjectNode creates the Project node via MERGE (idempotent).
func (s *Store) ensureProjectNode(ctx context.Context) error {
	_, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MERGE (p:Project {slug: $slug})
		 ON CREATE SET p.created_at = $now, p.updated_at = $now,
		               p.sync_adapters = [], p.github_repo = ""`,
		map[string]any{"slug": s.project, "now": s.now()},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("memgraph: ensure project node: %w", err)
	}
	return nil
}

// Scoped returns a new Store that shares this Store's driver but targets a different project.
// The Project node is ensured via MERGE on first use. Indexes are NOT re-created here —
// they were already created by the parent Store's New() call.
func (s *Store) Scoped(ctx context.Context, project string) (storage.ScopedBackend, error) {
	if project == "" {
		return nil, fmt.Errorf("memgraph: project slug required")
	}
	scoped := &Store{driver: s.driver, nowFunc: s.nowFunc, project: project}
	if err := scoped.ensureProjectNode(ctx); err != nil {
		return nil, err
	}
	return scoped, nil
}

const (
	defaultInitialStage = "spark"
	defaultLifecycle    = storage.SpecLifecycleTask
)

// CreateSpec stores a new spec node in Memgraph and returns it.
// All DB operations (CREATE node + ChangeLog) run within a single transaction.
func (s *Store) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	id := newID("spec")
	nowStr := s.now()
	ch := contenthash.Spec(intent, defaultInitialStage, priority, complexity, nil)

	query := `
		MATCH (p:Project {slug: $project})
		CREATE (p)<-[:BELONGS_TO]-(s:Spec {
			id: $id,
			slug: $slug,
			intent: $intent,
			stage: $stage,
			priority: $priority,
			complexity: $complexity,
			version: $version,
			created_at: $created_at,
			updated_at: $updated_at,
			lifecycle: $lifecycle,
			notes: $notes,
			content_hash: $content_hash
		})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash
	`
	params := mergeParams(s.projectParam(), map[string]any{
		"id":           id,
		"slug":         slug,
		"intent":       intent,
		"stage":        defaultInitialStage,
		"priority":     priority,
		"complexity":   complexity,
		"version":      int64(1),
		"created_at":   nowStr,
		"updated_at":   nowStr,
		"lifecycle":    string(defaultLifecycle),
		"notes":        "",
		"content_hash": ch,
	})

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		records, qErr := s.executeQuery(txCtx, query, params)
		if qErr != nil {
			return fmt.Errorf("memgraph: create spec: %w", qErr)
		}
		if len(records) == 0 {
			return fmt.Errorf("memgraph: create spec returned no records")
		}

		spec, parseErr := recordToSpec(records[0])
		if parseErr != nil {
			return parseErr
		}

		// Create an initial checkpoint ChangeLog entry for the newly created spec.
		allFields := storage.SpecFields{
			Intent:     intent,
			Stage:      defaultInitialStage,
			Priority:   priority,
			Complexity: complexity,
		}
		empty := storage.SpecFields{}
		deltas := storage.ComputeFieldDeltas(&empty, &allFields)
		clEntry := &storage.ChangeLogEntry{
			Version:     spec.Version,
			Stage:       spec.Stage,
			ContentHash: spec.ContentHash,
			Checkpoint:  true,
			Summary:     "Spec created",
			Date:        spec.CreatedAt,
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}
		result = spec
		return nil
	})
	return result, err
}

// GetSpec retrieves a spec by slug.
func (s *Store) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash
	`
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(records[0])
}

// BatchGetSpecs retrieves multiple specs by slug in a single query.
// Missing slugs are silently omitted from the result map.
func (s *Store) BatchGetSpecs(ctx context.Context, slugs []string) (map[string]*storage.Spec, error) {
	if len(slugs) == 0 {
		return map[string]*storage.Spec{}, nil
	}
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec) WHERE s.slug IN $slugs
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{"slugs": slugs}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: batch get specs: %w", err)
	}
	result := make(map[string]*storage.Spec, len(records))
	for _, rec := range records {
		spec, err := recordToSpec(rec)
		if err != nil {
			return nil, fmt.Errorf("memgraph: batch get specs: parse: %w", err)
		}
		result[spec.Slug] = spec
	}
	return result, nil
}

// ListSpecs returns specs matching the given filters.
func (s *Store) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error) {
	var clauses []string
	params := s.projectParam()

	if stage != "" {
		clauses = append(clauses, "s.stage = $stage")
		params["stage"] = stage
	}
	if priority != "" {
		clauses = append(clauses, "s.priority = $priority")
		params["priority"] = priority
	}

	query := "MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash`
	query += " ORDER BY s.created_at"
	if limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(limit)
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list specs: %w", err)
	}

	specs := make([]*storage.Spec, 0, len(records))
	for _, rec := range records {
		sp, err := recordToSpec(rec)
		if err != nil {
			return nil, err
		}
		specs = append(specs, sp)
	}
	return specs, nil
}

// UpdateSpec updates a spec by slug. Only non-nil fields are changed.
// All DB operations (read old fields, UPDATE, recompute hash, ChangeLog)
// run within a single transaction.
func (s *Store) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error) {
	var setClauses []string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if intent != nil {
		setClauses = append(setClauses, "s.intent = $intent")
		params["intent"] = *intent
	}
	if stage != nil {
		setClauses = append(setClauses, "s.stage = $stage")
		params["stage"] = *stage
	}
	if priority != nil {
		setClauses = append(setClauses, "s.priority = $priority")
		params["priority"] = *priority
	}
	if complexity != nil {
		setClauses = append(setClauses, "s.complexity = $complexity")
		params["complexity"] = *complexity
	}
	if notes != nil {
		setClauses = append(setClauses, "s.notes = $notes")
		params["notes"] = *notes
	}

	if len(setClauses) == 0 {
		return s.GetSpec(ctx, slug)
	}

	nowStr := s.now()
	setClauses = append(setClauses, "s.version = s.version + 1", "s.updated_at = $updated_at")
	params["updated_at"] = nowStr

	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		SET %s
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes,
		       s.notes, s.content_hash,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output
	`, strings.Join(setClauses, ", "))

	var result *storage.Spec
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Capture old field values before the update for changelog delta computation.
		oldFields, oldContentHash, _, _, rfErr := s.readSpecFields(txCtx, slug)
		if rfErr != nil {
			return rfErr
		}

		records, qErr := s.executeQuery(txCtx, query, params)
		if qErr != nil {
			return fmt.Errorf("memgraph: update spec: %w", qErr)
		}
		if len(records) == 0 {
			return fmt.Errorf("memgraph: spec %q: %w", slug, storage.ErrSpecNotFound)
		}

		// Recompute content_hash from the updated fields.
		rec := records[0]
		spec, parseErr := recordToSpec(rec)
		if parseErr != nil {
			return parseErr
		}
		authoringOutputs := make(map[string]string)
		for i, key := range []string{"spark_output", "shape_output", "specify_output", "decompose_output"} {
			val, aoErr := recordStringOptional(rec, 16+i, key)
			if aoErr != nil {
				return aoErr
			}
			if val != "" {
				authoringOutputs[key] = val
			}
		}
		ch := contenthash.Spec(spec.Intent, string(spec.Stage), string(spec.Priority), spec.Complexity, authoringOutputs)

		hashQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			SET s.content_hash = $content_hash
		`
		if _, hErr := s.executeQuery(txCtx, hashQuery, mergeParams(s.projectParam(), map[string]any{
			"slug":         slug,
			"content_hash": ch,
		})); hErr != nil {
			return fmt.Errorf("memgraph: update spec content_hash: %w", hErr)
		}
		spec.ContentHash = ch

		// Create a changelog entry only if the content hash changed (substantive update).
		if ch != oldContentHash {
			newFields := storage.SpecFields{
				Intent:          spec.Intent,
				Stage:           string(spec.Stage),
				Priority:        string(spec.Priority),
				Complexity:      spec.Complexity,
				SparkOutput:     authoringOutputs["spark_output"],
				ShapeOutput:     authoringOutputs["shape_output"],
				SpecifyOutput:   authoringOutputs["specify_output"],
				DecomposeOutput: authoringOutputs["decompose_output"],
			}
			deltas := storage.ComputeFieldDeltas(&oldFields, &newFields)
			clEntry := &storage.ChangeLogEntry{
				Version:     spec.Version,
				Stage:       spec.Stage,
				ContentHash: ch,
				Checkpoint:  false,
				Summary:     "Spec updated",
				Date:        spec.UpdatedAt,
			}
			if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
				return clErr
			}
		}

		if stage != nil && storage.SpecStage(*stage) == storage.SpecStageDone {
			if err := s.RefreshDependencyHashes(txCtx, slug); err != nil {
				return fmt.Errorf("memgraph: refresh dependency hashes after done transition: %w", err)
			}
		}

		result = spec
		return nil
	})
	return result, err
}

// readSpecFields reads the substantive fields, content hash, version, and
// updated_at of a spec. Used before/after updates to capture values for
// changelog delta computation without a separate GetSpec round-trip.
func (s *Store) readSpecFields(ctx context.Context, slug string) (fields storage.SpecFields, contentHash string, version int32, updatedAt string, err error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		RETURN s.intent, s.stage, s.priority, s.complexity,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output,
		       s.content_hash, s.version, s.updated_at
	`
	records, rErr := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if rErr != nil {
		err = fmt.Errorf("memgraph: read spec fields: %w", rErr)
		return
	}
	if len(records) == 0 {
		err = fmt.Errorf("memgraph: read spec fields %q: %w", slug, storage.ErrSpecNotFound)
		return
	}
	rec := records[0]
	getString := func(pos int) string {
		if pos >= len(rec.Values) || rec.Values[pos] == nil {
			return ""
		}
		if v, ok := rec.Values[pos].(string); ok {
			return v
		}
		return ""
	}
	fields = storage.SpecFields{
		Intent:          getString(0),
		Stage:           getString(1),
		Priority:        getString(2),
		Complexity:      getString(3),
		SparkOutput:     getString(4),
		ShapeOutput:     getString(5),
		SpecifyOutput:   getString(6),
		DecomposeOutput: getString(7),
	}
	contentHash = getString(8)
	if pos := 9; pos < len(rec.Values) && rec.Values[pos] != nil {
		if v, ok := rec.Values[pos].(int64); ok {
			version = int32(v) //nolint:gosec // version is always positive and small
		}
	}
	updatedAt = getString(10)
	return
}

// ClearAll removes all nodes and relationships from the graph.
// Intended for test cleanup only. Re-creates the Project node after clearing
// so the store remains usable for subsequent operations.
func (s *Store) ClearAll(ctx context.Context) error {
	_, err := s.executeQuery(ctx, "MATCH (n) DETACH DELETE n", nil)
	if err != nil {
		return fmt.Errorf("memgraph: clear all: %w", err)
	}
	return s.ensureProjectNode(ctx)
}

// Close releases the driver resources. Scoped stores (created via Scoped())
// share the parent's driver and must not close it — only the owning store
// (created via New()) closes the underlying driver.
func (s *Store) Close(ctx context.Context) error {
	if !s.ownsDriver {
		return nil
	}
	if err := s.driver.Close(ctx); err != nil {
		return fmt.Errorf("memgraph: close: %w", err)
	}
	return nil
}

// newID produces a prefixed ULID: prefix + "-" + ULID.
// ULIDs are 128-bit and lexicographically sortable by timestamp.
func newID(prefix string) string {
	return prefix + "-" + ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// sortableRFC3339Nano is a fixed-width RFC 3339 layout with zero-padded nanoseconds.
// Unlike time.RFC3339Nano (which trims trailing fractional zeros), this format
// ensures lexicographic string ordering matches chronological ordering — critical
// for Cypher ORDER BY and timestamp comparison operators on string-typed fields.
//
// Migration note: This replaces the previous time.RFC3339 format used for all
// timestamp fields. Existing Memgraph nodes may have timestamps in the old format
// (without nanoseconds). Both formats are ISO 8601 prefix-sortable, so mixed-format
// ORDER BY queries produce correct results. Reads fall back via parseRFC3339.
const sortableRFC3339Nano = "2006-01-02T15:04:05.000000000Z07:00"

// nowTime returns the current UTC time from the Store's clock.
func (s *Store) nowTime() time.Time {
	return s.nowFunc().UTC()
}

// now returns the current UTC time as a fixed-width RFC 3339 string to
// ensure lexicographic string ordering matches chronological ordering.
func (s *Store) now() string {
	return s.nowTime().Format(sortableRFC3339Nano)
}

// projectParam returns a map with the project slug for use in Cypher queries.
func (s *Store) projectParam() map[string]any {
	return map[string]any{"project": s.project}
}

// mergeParams combines base params with additional params.
func mergeParams(base, extra map[string]any) map[string]any {
	m := make(map[string]any, len(base)+len(extra))
	maps.Copy(m, base)
	maps.Copy(m, extra)
	return m
}

// parseRFC3339 parses an RFC3339 timestamp string from a memgraph record field.
func parseRFC3339(field, value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t, err = time.Parse(time.RFC3339, value)
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("memgraph: parse %s %q: %w", field, value, err)
	}
	return t, nil
}

// recordString extracts a string value from a neo4j record by position.
// It returns an error if the value is not a string, preventing silent data corruption.
func recordString(rec *neo4j.Record, pos int, field string) (string, error) {
	v, ok := rec.Values[pos].(string)
	if !ok {
		return "", fmt.Errorf("memgraph: field %q at position %d: expected string, got %T", field, pos, rec.Values[pos])
	}
	return v, nil
}

// recordInt64 extracts an int64 value from a neo4j record by position.
// It returns an error if the value is not an int64, preventing silent data corruption.
func recordInt64(rec *neo4j.Record, pos int, field string) (int64, error) {
	v, ok := rec.Values[pos].(int64)
	if !ok {
		return 0, fmt.Errorf("memgraph: field %q at position %d: expected int64, got %T", field, pos, rec.Values[pos])
	}
	return v, nil
}

// safeInt32 clamps an int64 to the int32 range, preventing overflow on conversion.
func safeInt32(v int64) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

func recordStringOptional(rec *neo4j.Record, pos int, field string) (string, error) {
	if pos >= len(rec.Values) {
		return "", fmt.Errorf("memgraph: field %q at position %d: missing", field, pos)
	}
	if rec.Values[pos] == nil {
		return "", nil
	}
	s, ok := rec.Values[pos].(string)
	if !ok {
		return "", fmt.Errorf("memgraph: field %q at position %d: expected string or nil, got %T", field, pos, rec.Values[pos])
	}
	return s, nil
}

// recordToSpecOffset converts a neo4j record to a *storage.Spec, reading field
// values starting at the given positional offset. This supports queries that
// return multiple spec records in a single row (e.g., SupersedeSpec returning
// both old and new specs).
func recordToSpecOffset(rec *neo4j.Record, offset int) (*storage.Spec, error) {
	id, err := recordString(rec, offset+0, "id")
	if err != nil {
		return nil, err
	}
	slug, err := recordString(rec, offset+1, "slug")
	if err != nil {
		return nil, err
	}
	intent, err := recordString(rec, offset+2, "intent")
	if err != nil {
		return nil, err
	}
	stage, err := recordString(rec, offset+3, "stage")
	if err != nil {
		return nil, err
	}
	priority, err := recordString(rec, offset+4, "priority")
	if err != nil {
		return nil, err
	}
	complexity, err := recordString(rec, offset+5, "complexity")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, offset+6, "version")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, offset+7, "created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordString(rec, offset+8, "updated_at")
	if err != nil {
		return nil, err
	}

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}

	lifecycleStr, err := recordStringOptional(rec, offset+9, "lifecycle")
	if err != nil {
		return nil, err
	}
	lifecycle := storage.SpecLifecycle(lifecycleStr)
	if lifecycle == "" {
		lifecycle = defaultLifecycle
	}
	supersededBy, err := recordStringOptional(rec, offset+10, "superseded_by")
	if err != nil {
		return nil, err
	}
	supersedes, err := recordStringOptional(rec, offset+11, "supersedes")
	if err != nil {
		return nil, err
	}
	notes, err := recordStringOptional(rec, offset+12, "notes")
	if err != nil {
		return nil, err
	}
	contentHash, err := recordStringOptional(rec, offset+13, "content_hash")
	if err != nil {
		return nil, err
	}

	return &storage.Spec{
		ID:          id,
		Slug:        slug,
		Intent:      intent,
		Stage:       storage.SpecStage(stage),
		Priority:    storage.SpecPriority(priority),
		Complexity:  complexity,
		Version:     safeInt32(version),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Lifecycle:   lifecycle,
		SupersededBy: supersededBy,
		Supersedes:  supersedes,
		Notes:       notes,
		ContentHash: contentHash,
	}, nil
}

// recordToSpec converts a neo4j record (with positional values) to a *storage.Spec.
func recordToSpec(rec *neo4j.Record) (*storage.Spec, error) {
	return recordToSpecOffset(rec, 0)
}
