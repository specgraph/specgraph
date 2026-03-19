// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"errors"
	"fmt"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/contenthash"
)

const (
	defaultChildPriority   = "p2"
	defaultChildComplexity = "medium"
)

// hashInputProperties lists the spec node properties that affect the content hash.
// Only these properties trigger a hash recomputation after storeJSONProperty succeeds.
var hashInputProperties = map[string]bool{
	"spark_output":     true,
	"shape_output":     true,
	"specify_output":   true,
	"decompose_output": true,
}

// allowedJSONProperties lists the spec node properties that storeJSONProperty may write.
var allowedJSONProperties = map[string]bool{
	"spark_output":            true,
	"shape_output":            true,
	"specify_output":          true,
	"decompose_output":        true,
	"red_team_findings":       true,
	"peripheral_vision":       true,
	"consistency_issues":      true,
	"simplicity_findings":     true,
	"safety_flags":            true,
	"constitution_violations": true,
}

// TransitionStage validates and applies a spec's stage transition.
// It first checks the transition is valid via authoring.ValidateTransition,
// then updates the spec's stage in the database. Returns ErrSpecNotFound if
// the spec doesn't exist, or ErrInvalidStageTransition if the spec is at
// a different stage than expected. Returns ErrSpecAlreadyApproved if from
// is the approved stage — approved is terminal and cannot be a source stage
// (AmendSpec handles the approved→X path with its own guard).
func (s *Store) TransitionStage(ctx context.Context, slug string, from, to storage.AuthoringStage) error {
	if from == storage.AuthoringStage(authoring.StageApproved) {
		return storage.ErrSpecAlreadyApproved
	}
	if err := authoring.ValidateTransition(authoring.Stage(from), authoring.Stage(to)); err != nil {
		return fmt.Errorf("memgraph: %w: %w", storage.ErrInvalidStageTransition, err)
	}
	nowStr := s.now()
	fromStr := string(from)
	toStr := string(to)
	// When transitioning to approved, also persist approved_at so the
	// timestamp is stored rather than computed at response time.
	setClause := "s.stage = $to, s.updated_at = $updated_at"
	if to == storage.AuthoringStage(authoring.StageApproved) {
		setClause += ", s.approved_at = $updated_at"
	}
	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		WHERE s.stage = $from OR ($from = "" AND (s.stage IS NULL OR s.stage = ""))
		SET %s
		RETURN s.slug
	`, setClause)
	records, err := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"slug": slug, "from": fromStr, "to": toStr, "updated_at": nowStr}))
	if err != nil {
		return fmt.Errorf("memgraph: transition stage: %w", err)
	}
	if len(records) == 0 {
		// Distinguish between "spec not found" and "spec at wrong stage".
		checkQuery := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.stage AS stage`
		checkRecords, checkErr := s.executeQuery(ctx, checkQuery,
			mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if checkErr != nil {
			return fmt.Errorf("memgraph: check spec stage: %w", checkErr)
		}
		if len(checkRecords) == 0 {
			return fmt.Errorf("memgraph: transition stage %q: %w", slug, storage.ErrSpecNotFound)
		}
		actualStage, ok := checkRecords[0].Get("stage")
		if !ok || actualStage == nil {
			actualStage = "<unknown>"
		}
		return fmt.Errorf("memgraph: spec %q at stage %v, expected %q: %w", slug, actualStage, from, storage.ErrInvalidStageTransition)
	}
	if err := s.recomputeContentHash(ctx, slug); err != nil {
		return err
	}

	// Create a checkpoint ChangeLog for the stage transition.
	updatedSpec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return err
	}
	deltas := []storage.FieldChange{{Field: "stage", OldValue: fromStr, NewValue: toStr}}
	clEntry := &storage.ChangeLogEntry{
		Version:     updatedSpec.Version,
		Stage:       updatedSpec.Stage,
		ContentHash: updatedSpec.ContentHash,
		Checkpoint:  true,
		Summary:     fmt.Sprintf("Stage transition: %s → %s", fromStr, toStr),
		Date:        updatedSpec.UpdatedAt,
	}
	return s.createChangeLog(ctx, slug, clEntry, deltas)
}

// StoreSparkOutput persists the spark stage output as JSON on the spec node.
func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *storage.SparkOutput) error {
	oldFields, oldHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return err
	}
	if err := s.storeJSONProperty(ctx, slug, "spark_output", output); err != nil {
		return err
	}
	return s.authoringOutputChangeLog(ctx, slug, "spark_output", oldFields, oldHash)
}

// StoreShapeOutput persists the shape stage output as JSON on the spec node.
// It also promotes any structured decisions to first-class Decision graph nodes
// with DECIDED_IN edges, enabling cross-referencing and impact analysis (ADR-003).
// Decision promotion is idempotent: CreateDecision is skipped if the decision
// already exists, but AddEdge is always called so a lost edge is recreated.
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	oldFields, oldHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return err
	}
	if txErr := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.storeJSONProperty(txCtx, slug, "shape_output", output); err != nil {
			return err
		}
		// Promote decisions to graph nodes with DECIDED_IN edges (spec→decision per ADR-003).
		for i, d := range output.Decisions {
			if d.Slug == "" {
				return fmt.Errorf("decision at index %d: slug is required", i)
			}
			if d.Title == "" {
				return fmt.Errorf("decision %q: title is required", d.Slug)
			}
			// Create decision node only if it does not already exist.
			_, getErr := s.GetDecision(txCtx, d.Slug)
			if errors.Is(getErr, storage.ErrDecisionNotFound) {
				if _, createErr := s.CreateDecision(txCtx, d.Slug, d.Title, d.Body, d.Rationale); createErr != nil {
					return fmt.Errorf("create decision %q: %w", d.Slug, createErr)
				}
			} else if getErr != nil {
				return fmt.Errorf("check decision %q existence: %w", d.Slug, getErr)
			}
			// Always ensure the DECIDED_IN edge exists (spec→decision). AddEdge uses
			// MERGE so calling it on an existing edge is safe.
			if _, err := s.AddEdge(txCtx, slug, d.Slug, storage.EdgeTypeDecidedIn); err != nil {
				return fmt.Errorf("add DECIDED_IN edge %q->%q: %w", slug, d.Slug, err)
			}
		}
		return nil
	}); txErr != nil {
		return txErr
	}
	return s.authoringOutputChangeLog(ctx, slug, "shape_output", oldFields, oldHash)
}

// StoreSpecifyOutput persists the specify stage output as JSON on the spec node.
func (s *Store) StoreSpecifyOutput(ctx context.Context, slug string, output *storage.SpecifyOutput) error {
	oldFields, oldHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return err
	}
	if err := s.storeJSONProperty(ctx, slug, "specify_output", output); err != nil {
		return err
	}
	return s.authoringOutputChangeLog(ctx, slug, "specify_output", oldFields, oldHash)
}

// StoreDecomposeOutput persists the decompose output and creates child spec nodes with edges.
// When called within a transaction (via RunInTransaction), partial failures roll back automatically.
// Child spec creation is idempotent: GetSpec is called first; if the child does not exist,
// CreateSpec is used to create it. MERGE is used only for the COMPOSES and DEPENDS_ON edges.
// It returns the slugs of the created (or already-existing) child specs.
func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	if err := storage.ValidateStrategy(output.Strategy); err != nil {
		return nil, fmt.Errorf("memgraph: invalid decomposition strategy: %w", err)
	}
	// Build and validate the set of slice IDs before writing anything to the
	// database so that validation failures do not leave a partial JSON blob.
	sliceIDs := make(map[string]bool, len(output.Slices))
	for _, sl := range output.Slices {
		if sl.ID == "" {
			return nil, fmt.Errorf("memgraph: decompose slice ID must not be empty")
		}
		if sliceIDs[sl.ID] {
			return nil, fmt.Errorf("memgraph: duplicate decompose slice ID %q", sl.ID)
		}
		sliceIDs[sl.ID] = true
	}
	// Capture old fields before the store to detect content hash changes.
	oldFields, oldHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return nil, err
	}
	if err := s.storeJSONProperty(ctx, slug, "decompose_output", output); err != nil {
		return nil, err
	}
	// Pass 1: create all child Spec nodes and COMPOSES edges.
	// This ensures every node exists before any DEPENDS_ON edge is attempted,
	// so out-of-order dependencies (slice B depends on slice C listed later)
	// are handled correctly.
	var childSlugs []string
	for _, sl := range output.Slices {
		childSlug := fmt.Sprintf("%s/%s", slug, sl.ID)
		// Check if child spec already exists (idempotency for retries).
		_, getErr := s.GetSpec(ctx, childSlug)
		if getErr != nil {
			if !errors.Is(getErr, storage.ErrSpecNotFound) {
				return nil, fmt.Errorf("memgraph: check child spec %q: %w", childSlug, getErr)
			}
			// Not found — proceed to create.
			if _, err := s.CreateSpec(ctx, childSlug, sl.Intent, defaultChildPriority, defaultChildComplexity); err != nil {
				return nil, fmt.Errorf("memgraph: create child spec %q: %w", childSlug, err)
			}
		}
		// If getErr == nil, child spec already exists (idempotent retry).
		composeQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(child:Spec {slug: $child_slug}),
			      (p)<-[:BELONGS_TO]-(parent:Spec {slug: $parent_slug})
			MERGE (child)-[:COMPOSES]->(parent)
		`
		_, err := s.executeQuery(ctx, composeQuery,
			mergeParams(s.projectParam(), map[string]any{"child_slug": childSlug, "parent_slug": slug}))
		if err != nil {
			return nil, fmt.Errorf("memgraph: merge COMPOSES edge: %w", err)
		}
		childSlugs = append(childSlugs, childSlug)
	}

	// Pass 2: create all DEPENDS_ON edges now that all child nodes exist.
	for _, sl := range output.Slices {
		childSlug := fmt.Sprintf("%s/%s", slug, sl.ID)
		for _, dep := range sl.DependsOn {
			if !sliceIDs[dep] {
				return nil, fmt.Errorf("memgraph: slice %q depends on unknown sibling %q", sl.ID, dep)
			}
			depSlug := fmt.Sprintf("%s/%s", slug, dep)
			depQuery := `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(from:Spec {slug: $from_slug}),
				      (p)<-[:BELONGS_TO]-(to:Spec {slug: $to_slug})
				MERGE (from)-[:DEPENDS_ON]->(to)
			`
			_, err := s.executeQuery(ctx, depQuery,
				mergeParams(s.projectParam(), map[string]any{"from_slug": childSlug, "to_slug": depSlug}))
			if err != nil {
				return nil, fmt.Errorf("memgraph: merge DEPENDS_ON edge: %w", err)
			}
		}
	}
	// Create a non-checkpoint ChangeLog for the decompose_output field change
	// on the parent spec. Child specs get their own changelogs via CreateSpec.
	if clErr := s.authoringOutputChangeLog(ctx, slug, "decompose_output", oldFields, oldHash); clErr != nil {
		return nil, clErr
	}
	return childSlugs, nil
}

// --- Analytical pass storage (thin wrappers over storeJSONProperty) ---

// StoreRedTeamFindings persists red team findings as JSON on the spec node.
func (s *Store) StoreRedTeamFindings(ctx context.Context, slug string, findings []storage.RedTeamFinding) error {
	return s.storeJSONProperty(ctx, slug, "red_team_findings", findings)
}

// StorePeripheralVision persists peripheral vision items as JSON on the spec node.
func (s *Store) StorePeripheralVision(ctx context.Context, slug string, items []storage.PeripheralVisionItem) error {
	return s.storeJSONProperty(ctx, slug, "peripheral_vision", items)
}

// StoreConsistencyIssues persists consistency issues as JSON on the spec node.
func (s *Store) StoreConsistencyIssues(ctx context.Context, slug string, issues []storage.ConsistencyIssue) error {
	return s.storeJSONProperty(ctx, slug, "consistency_issues", issues)
}

// StoreSimplicityFindings persists simplicity findings as JSON on the spec node.
func (s *Store) StoreSimplicityFindings(ctx context.Context, slug string, findings []storage.SimplicityFinding) error {
	return s.storeJSONProperty(ctx, slug, "simplicity_findings", findings)
}

// StoreSafetyFlags persists safety flags as JSON on the spec node.
func (s *Store) StoreSafetyFlags(ctx context.Context, slug string, flags []storage.SafetyFlag) error {
	return s.storeJSONProperty(ctx, slug, "safety_flags", flags)
}

// StoreConstitutionViolations persists constitution violations as JSON on the spec node.
func (s *Store) StoreConstitutionViolations(ctx context.Context, slug string, violations []storage.ConstitutionViolation) error {
	return s.storeJSONProperty(ctx, slug, "constitution_violations", violations)
}

// SupersedeSpec marks a spec as superseded and creates a SUPERSEDES edge to the replacement.
// This is the authoring-level supersession; for lifecycle-level supersession see LifecycleSupersedeSpec.
func (s *Store) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	// Validate both specs exist before the combined operation so callers get
	// a precise error identifying which slug was missing.
	if _, err := s.GetSpec(ctx, slug); err != nil {
		return fmt.Errorf("memgraph: supersede spec: old spec %q: %w", slug, err)
	}
	if _, err := s.GetSpec(ctx, supersededBy); err != nil {
		return fmt.Errorf("memgraph: supersede spec: new spec %q: %w", supersededBy, err)
	}
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(old:Spec {slug: $old_slug}),
		      (p)<-[:BELONGS_TO]-(new:Spec {slug: $new_slug})
		SET old.stage = "superseded", old.updated_at = $updated_at
		CREATE (new)-[:SUPERSEDES {reason: $reason}]->(old)
		RETURN old.slug
	`
	records, err := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"old_slug": slug, "new_slug": supersededBy, "reason": reason, "updated_at": nowStr}))
	if err != nil {
		return fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: supersede spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// AmendSpec moves a spec backward to an earlier stage, bumping its version.
// This is the authoring-level amendment; for lifecycle-level amendment see LifecycleAmendSpec.
func (s *Store) AmendSpec(ctx context.Context, slug, reason string, targetStage storage.AuthoringStage) (*storage.AmendResult, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("amend spec %q: get current: %w", slug, err)
	}
	if string(spec.Stage) == string(authoring.StageApproved) {
		return nil, storage.ErrSpecAlreadyApproved
	}
	if spec.Stage == storage.SpecStageSuperseded {
		return nil, fmt.Errorf("amend spec %q: %w", slug, storage.ErrSpecSuperseded)
	}
	if vErr := authoring.ValidateAmendTransition(authoring.Stage(spec.Stage), authoring.Stage(targetStage)); vErr != nil {
		return nil, fmt.Errorf("memgraph: amend: %w: %w", storage.ErrInvalidStageTransition, vErr)
	}
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		SET s.stage = $stage, s.amend_reason = $reason,
		    s.version = s.version + 1, s.updated_at = $updated_at
		RETURN s.slug, s.stage, s.version
	`
	records, err := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"slug": slug, "stage": string(targetStage), "reason": reason, "updated_at": nowStr}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	rec := records[0]
	retSlug, ok := rec.Get("s.slug")
	if !ok {
		return nil, fmt.Errorf("memgraph: amend spec %q: record missing s.slug", slug)
	}
	retStage, ok := rec.Get("s.stage")
	if !ok {
		return nil, fmt.Errorf("memgraph: amend spec %q: record missing s.stage", slug)
	}
	retVersion, ok := rec.Get("s.version")
	if !ok {
		return nil, fmt.Errorf("memgraph: amend spec %q: record missing s.version", slug)
	}
	result := &storage.AmendResult{
		Slug:  fmt.Sprintf("%v", retSlug),
		Stage: storage.AuthoringStage(fmt.Sprintf("%v", retStage)),
	}
	switch v := retVersion.(type) {
	case int64:
		result.Version = safeInt32(v)
	case nil:
		// Version not set on node — leave as 0.
	default:
		return nil, fmt.Errorf("memgraph: amend spec %q: unexpected version type %T", slug, retVersion)
	}
	if err := s.recomputeContentHash(ctx, slug); err != nil {
		return nil, err
	}
	return result, nil
}

// storeJSONProperty marshals data to JSON and stores it as a string property on the spec node.
// Property names must be interpolated into the Cypher query (parameterized property names are
// not supported by Cypher). The allowlist is the primary defense against Cypher injection;
// character validation provides a secondary check.
func (s *Store) storeJSONProperty(ctx context.Context, slug, property string, data any) error {
	if data == nil {
		return fmt.Errorf("memgraph: %s data must not be nil", property)
	}
	if !allowedJSONProperties[property] {
		return fmt.Errorf("memgraph: disallowed property name %q", property)
	}
	// Defense-in-depth: the allowlist above is the primary Cypher injection
	// guard. This character check is a secondary structural invariant — it
	// ensures that even if the allowlist were somehow bypassed (e.g., a
	// future refactor that populates the map from external input), no
	// property name containing special characters can reach fmt.Sprintf.
	for _, r := range property {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return fmt.Errorf("memgraph: unsafe property name character in %q", property)
		}
	}
	jsonStr, err := marshalJSON(data)
	if err != nil {
		return fmt.Errorf("memgraph: marshal %s: %w", property, err)
	}
	nowStr := s.now()
	// property is safe to interpolate: it passed the allowlist check and the
	// character-validation loop above, so it contains only [a-zA-Z0-9_].
	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		SET s.%s = $data, s.updated_at = $updated_at
		RETURN s.slug
	`, property)
	records, err := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"slug": slug, "data": jsonStr, "updated_at": nowStr}))
	if err != nil {
		return fmt.Errorf("memgraph: store %s: %w", property, err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: store %s for %q: %w", property, slug, storage.ErrSpecNotFound)
	}
	if hashInputProperties[property] {
		if err := s.recomputeContentHash(ctx, slug); err != nil {
			return err
		}
	}
	return nil
}

// authoringOutputChangeLog creates a non-checkpoint ChangeLog entry after a
// Store*Output method succeeds. It compares the old and new spec fields and
// only creates an entry if the content hash changed (i.e., the field was a
// hash-input property that actually changed the hash). This keeps analytical
// pass methods (StoreRedTeamFindings, etc.) free of changelog noise.
func (s *Store) authoringOutputChangeLog(ctx context.Context, slug, field string, oldFields storage.SpecFields, oldHash string) error {
	newFields, newHash, err := s.readSpecFields(ctx, slug)
	if err != nil {
		return err
	}
	if newHash == oldHash {
		return nil
	}
	deltas := storage.ComputeFieldDeltas(oldFields, newFields)
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return err
	}
	clEntry := &storage.ChangeLogEntry{
		Version:     spec.Version,
		Stage:       spec.Stage,
		ContentHash: newHash,
		Checkpoint:  false,
		Summary:     fmt.Sprintf("Updated %s", field),
		Date:        spec.UpdatedAt,
	}
	return s.createChangeLog(ctx, slug, clEntry, deltas)
}

// recomputeContentHash reads all hash-input fields from the spec node,
// computes a new Murmur3-128 content hash, and persists it. This method
// is called after any mutation that changes a hash-input field (stage,
// authoring outputs).
func (s *Store) recomputeContentHash(ctx context.Context, slug string) error {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		RETURN s.intent, s.stage, s.priority, s.complexity,
		       s.spark_output, s.shape_output, s.specify_output, s.decompose_output
	`
	records, err := s.executeQuery(ctx, query,
		mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return fmt.Errorf("memgraph: recompute content_hash: read fields: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("memgraph: recompute content_hash %q: %w", slug, storage.ErrSpecNotFound)
	}
	rec := records[0]
	getString := func(_ string, pos int) string {
		if pos >= len(rec.Values) || rec.Values[pos] == nil {
			return ""
		}
		if v, ok := rec.Values[pos].(string); ok {
			return v
		}
		return ""
	}
	intent := getString("s.intent", 0)
	stage := getString("s.stage", 1)
	priority := getString("s.priority", 2)
	complexity := getString("s.complexity", 3)

	authoringOutputs := make(map[string]string)
	for i, key := range []string{"spark_output", "shape_output", "specify_output", "decompose_output"} {
		if v := getString(key, 4+i); v != "" {
			authoringOutputs[key] = v
		}
	}

	ch := contenthash.Spec(intent, stage, priority, complexity, authoringOutputs)

	hashQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		SET s.content_hash = $content_hash
	`
	if _, err := s.executeQuery(ctx, hashQuery, mergeParams(s.projectParam(), map[string]any{
		"slug":         slug,
		"content_hash": ch,
	})); err != nil {
		return fmt.Errorf("memgraph: recompute content_hash: set: %w", err)
	}
	return nil
}
