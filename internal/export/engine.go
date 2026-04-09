// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package export

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the subset of storage needed for export/import operations.
type Backend interface {
	storage.Backend
	storage.AuthoringBackend
	storage.GraphBackend
	storage.DecisionBackend
	storage.ConstitutionBackend
	storage.FindingsBackend
	storage.ChangeLogBackend
	storage.ConversationBackend
	storage.ExecutionBackend
	storage.SyncBackend
	storage.SliceBackend
	storage.ProjectBackend
}

// Engine performs project export and import operations.
type Engine struct {
	backend    Backend
	signingKey string
	version    string
}

// NewEngine creates an Engine that reads from backend and signs with the given key.
// An empty signingKey disables signature generation.
func NewEngine(backend Backend, signingKey, version string) *Engine {
	return &Engine{backend: backend, signingKey: signingKey, version: version}
}

// Export collects all project entities and returns a signed JSON document.
func (e *Engine) Export(ctx context.Context, projectSlug string) ([]byte, error) {
	doc, err := e.collect(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("export collect: %w", err)
	}

	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		return nil, fmt.Errorf("export marshal data: %w", err)
	}

	if e.signingKey != "" {
		mac := hmac.New(sha256.New, []byte(e.signingKey))
		mac.Write(dataBytes)
		doc.Signature = &Signature{
			Algorithm: "hmac-sha256",
			Digest:    hex.EncodeToString(mac.Sum(nil)),
		}
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("export marshal: %w", err)
	}
	return out, nil
}

// collect reads all entities from the backend and assembles a Document.
func (e *Engine) collect(ctx context.Context, projectSlug string) (*Document, error) {
	doc := &Document{
		SchemaVersion:    CurrentSchemaVersion,
		ExportedAt:       time.Now().UTC(),
		SpecGraphVersion: e.version,
		ProjectSlug:      projectSlug,
	}

	// Project
	proj, err := e.backend.GetProject(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	doc.Data.Project = proj

	// Constitution (optional — may not exist)
	constitution, err := e.backend.GetConstitution(ctx)
	if err == nil {
		doc.Data.Constitution = constitution
	}

	// Specs — list summaries, then fetch full data for each
	specSummaries, err := e.backend.ListSpecs(ctx, "", "", 0)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	specs := make([]*storage.Spec, 0, len(specSummaries))
	for _, s := range specSummaries {
		full, getErr := e.backend.GetSpec(ctx, s.Slug)
		if getErr != nil {
			return nil, fmt.Errorf("get spec %q: %w", s.Slug, getErr)
		}
		specs = append(specs, full)
	}
	doc.Data.Specs = specs

	// Decisions
	decisions, err := e.backend.ListDecisions(ctx, "", 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}
	doc.Data.Decisions = decisions

	// Slices — per spec
	var allSlices []*storage.Slice
	for _, s := range specs {
		slices, sliceErr := e.backend.ListSlices(ctx, s.Slug)
		if sliceErr != nil {
			return nil, fmt.Errorf("list slices for %q: %w", s.Slug, sliceErr)
		}
		allSlices = append(allSlices, slices...)
	}
	doc.Data.Slices = allSlices

	// Edges — from full graph
	fg, err := e.backend.GetFullGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("get full graph: %w", err)
	}
	edges := make([]Edge, 0, len(fg.Edges))
	for _, ge := range fg.Edges {
		edges = append(edges, Edge{
			FromSlug:          ge.FromID,
			ToSlug:            ge.ToID,
			Type:              string(ge.EdgeType),
			ContentHashAtLink: ge.ContentHashAtLink,
		})
	}
	doc.Data.Edges = edges

	// Findings
	findings, err := e.backend.ListAllFindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}
	doc.Data.Findings = findings

	// ChangeLogs
	changeLogs, err := e.backend.ListAllChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list changes: %w", err)
	}
	doc.Data.ChangeLogs = changeLogs

	// Conversations
	conversations, err := e.backend.ListAllConversations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	doc.Data.Conversations = conversations

	// SyncMappings
	mappings, err := e.backend.ListSyncMappings(ctx, "", "")
	if err != nil {
		return nil, fmt.Errorf("list sync mappings: %w", err)
	}
	doc.Data.SyncMappings = mappings

	// ExecutionEvents — per spec
	var allEvents []*storage.ExecutionEvent
	for _, s := range specs {
		events, evErr := e.backend.GetExecutionEvents(ctx, s.Slug, 0)
		if evErr != nil {
			return nil, fmt.Errorf("get execution events for %q: %w", s.Slug, evErr)
		}
		allEvents = append(allEvents, events...)
	}
	doc.Data.ExecutionEvents = allEvents

	return doc, nil
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

// ImportResult summarises what was created during an import.
type ImportResult struct {
	Project       int
	Constitution  int
	Specs         int
	Decisions     int
	Slices        int
	Edges         int
	Findings      int
	ChangeLogs    int
	Conversations int
	SyncMappings  int
	ExecEvents    int
	Warnings      []string
}

// Import validates and writes an exported JSON document into the backend.
// If force is true and data already exists, WipeProjectData is called first.
// If requireSig is true, a missing signature is an error.
func (e *Engine) Import(ctx context.Context, data []byte, force, requireSig bool) (*ImportResult, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("import unmarshal: %w", err)
	}

	if doc.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported schema version %d (max %d)", doc.SchemaVersion, CurrentSchemaVersion)
	}

	if err := e.verifySignature(data, &doc, requireSig); err != nil {
		return nil, fmt.Errorf("import signature: %w", err)
	}

	if err := validateRefs(&doc); err != nil {
		return nil, fmt.Errorf("import referential integrity: %w", err)
	}

	// Check for existing data — ListSpecs with limit=1.
	existing, err := e.backend.ListSpecs(ctx, "", "", 1)
	if err != nil {
		return nil, fmt.Errorf("import check existing: %w", err)
	}
	if len(existing) > 0 && !force {
		return nil, errors.New("project already contains data; use force to overwrite")
	}
	if len(existing) > 0 {
		if wipeErr := e.backend.WipeProjectData(ctx); wipeErr != nil {
			return nil, fmt.Errorf("import wipe: %w", wipeErr)
		}
	}

	return e.writeEntities(ctx, &doc)
}

// rawEnvelope extracts the "data" field as raw bytes from the original JSON,
// avoiding re-marshaling which could reorder map keys nondeterministically.
type rawEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// verifySignature checks the HMAC signature on a document.
// It uses the raw bytes from the original JSON to avoid nondeterministic re-marshaling.
func (e *Engine) verifySignature(raw []byte, doc *Document, requireSig bool) error {
	if doc.Signature == nil {
		if requireSig {
			return errors.New("signature required but not present")
		}
		return nil
	}

	// Signature present but engine has no key — can't verify.
	if e.signingKey == "" {
		return nil
	}

	// Extract the original "data" bytes from raw JSON to preserve exact byte ordering.
	var env rawEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("extract data bytes for verification: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(e.signingKey))
	mac.Write(env.Data)
	expected := mac.Sum(nil)

	got, err := hex.DecodeString(doc.Signature.Digest)
	if err != nil {
		return fmt.Errorf("decode signature digest: %w", err)
	}

	if !hmac.Equal(expected, got) {
		return errors.New("HMAC signature mismatch")
	}
	return nil
}

// validateRefs checks referential integrity of the document.
func validateRefs(doc *Document) error {
	slugs := make(map[string]bool, len(doc.Data.Specs)+len(doc.Data.Decisions))
	for _, s := range doc.Data.Specs {
		slugs[s.Slug] = true
	}
	for _, d := range doc.Data.Decisions {
		slugs[d.Slug] = true
	}

	var errs []string

	for i, edge := range doc.Data.Edges {
		if !slugs[edge.FromSlug] {
			errs = append(errs, fmt.Sprintf("edge[%d]: from_slug %q not found", i, edge.FromSlug))
		}
		if !slugs[edge.ToSlug] {
			errs = append(errs, fmt.Sprintf("edge[%d]: to_slug %q not found", i, edge.ToSlug))
		}
	}

	specSlugs := make(map[string]bool, len(doc.Data.Specs))
	for _, s := range doc.Data.Specs {
		specSlugs[s.Slug] = true
	}

	for i, sl := range doc.Data.Slices {
		if !specSlugs[sl.ParentSlug] {
			errs = append(errs, fmt.Sprintf("slice[%d]: parent_slug %q not found", i, sl.ParentSlug))
		}
	}
	for i, f := range doc.Data.Findings {
		if !specSlugs[f.SpecSlug] {
			errs = append(errs, fmt.Sprintf("finding[%d]: spec_slug %q not found", i, f.SpecSlug))
		}
	}
	for i, cl := range doc.Data.ChangeLogs {
		if !specSlugs[cl.SpecSlug] {
			errs = append(errs, fmt.Sprintf("changelog[%d]: spec_slug %q not found", i, cl.SpecSlug))
		}
	}
	for i, c := range doc.Data.Conversations {
		if !specSlugs[c.SpecSlug] {
			errs = append(errs, fmt.Sprintf("conversation[%d]: spec_slug %q not found", i, c.SpecSlug))
		}
	}
	for i, m := range doc.Data.SyncMappings {
		if !specSlugs[m.SpecSlug] {
			errs = append(errs, fmt.Sprintf("sync_mapping[%d]: spec_slug %q not found", i, m.SpecSlug))
		}
	}
	for i, ev := range doc.Data.ExecutionEvents {
		if !specSlugs[ev.SpecSlug] {
			errs = append(errs, fmt.Sprintf("execution_event[%d]: spec_slug %q not found", i, ev.SpecSlug))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("broken references:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// stageOrder maps spec stages to their funnel position for restoration.
var stageOrder = map[storage.SpecStage]int{
	storage.SpecStageSpark:      0,
	storage.SpecStageShape:      1,
	storage.SpecStageSpecify:    2,
	storage.SpecStageDecompose:  3,
	storage.SpecStageApproved:   4,
	storage.SpecStageInProgress: 5,
	storage.SpecStageReview:     6,
	storage.SpecStageDone:       7,
	storage.SpecStageAmended:    8,
	storage.SpecStageSuperseded: 9,
	storage.SpecStageAbandoned:  10,
}

// writeEntities creates all entities in dependency order.
func (e *Engine) writeEntities(ctx context.Context, doc *Document) (*ImportResult, error) {
	res := &ImportResult{}

	// 1. Project
	if doc.ProjectSlug != "" {
		if _, err := e.backend.EnsureProject(ctx, doc.ProjectSlug); err != nil {
			return nil, fmt.Errorf("ensure project: %w", err)
		}
		res.Project = 1
	}

	// 2. Constitution
	if doc.Data.Constitution != nil {
		if _, err := e.backend.UpdateConstitution(ctx, doc.Data.Constitution); err != nil {
			return nil, fmt.Errorf("update constitution: %w", err)
		}
		res.Constitution = 1
	}

	// 3. Specs — create then restore stage via Store*Output + TransitionStage
	for _, spec := range doc.Data.Specs {
		if _, err := e.backend.CreateSpec(ctx, spec.Slug, spec.Intent, string(spec.Priority), string(spec.Complexity)); err != nil {
			return nil, fmt.Errorf("create spec %q: %w", spec.Slug, err)
		}

		// Replay authoring outputs to advance stage.
		if spec.SparkOutput != nil {
			if err := e.backend.StoreSparkOutput(ctx, spec.Slug, spec.SparkOutput); err != nil {
				return nil, fmt.Errorf("store spark output %q: %w", spec.Slug, err)
			}
		}
		if spec.ShapeOutput != nil {
			if err := e.backend.StoreShapeOutput(ctx, spec.Slug, spec.ShapeOutput); err != nil {
				return nil, fmt.Errorf("store shape output %q: %w", spec.Slug, err)
			}
		}
		if spec.SpecifyOutput != nil {
			if err := e.backend.StoreSpecifyOutput(ctx, spec.Slug, spec.SpecifyOutput); err != nil {
				return nil, fmt.Errorf("store specify output %q: %w", spec.Slug, err)
			}
		}
		if spec.DecomposeOutput != nil {
			if _, err := e.backend.StoreDecomposeOutput(ctx, spec.Slug, spec.DecomposeOutput); err != nil {
				return nil, fmt.Errorf("store decompose output %q: %w", spec.Slug, err)
			}
		}

		// After Store*Output calls, the spec is at the stage following the last stored output.
		// For stages beyond decompose (approved, in_progress, review, done),
		// we need explicit TransitionStage calls stepping through the funnel.
		targetOrd := stageOrder[spec.Stage]
		decomposeOrd := stageOrder[storage.SpecStageDecompose]
		if targetOrd > decomposeOrd {
			// Step through the post-decompose funnel.
			// TransitionStage(from, to) requires the from to be the current stage.
			chain := []storage.SpecStage{
				storage.SpecStageDecompose,
				storage.SpecStageApproved,
				storage.SpecStageInProgress,
				storage.SpecStageReview,
				storage.SpecStageDone,
			}
			for i := 0; i < len(chain)-1; i++ {
				from, to := chain[i], chain[i+1]
				if stageOrder[to] > targetOrd {
					break
				}
				if err := e.backend.TransitionStage(ctx, spec.Slug, from, to); err != nil {
					return nil, fmt.Errorf("transition %q from %s to %s: %w", spec.Slug, from, to, err)
				}
			}
			// Terminal lifecycle stages (amended, superseded, abandoned) transition from done.
			switch spec.Stage {
			case storage.SpecStageAmended, storage.SpecStageSuperseded, storage.SpecStageAbandoned:
				if err := e.backend.TransitionStage(ctx, spec.Slug, storage.SpecStageDone, spec.Stage); err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("could not set terminal stage %s for %q: %v", spec.Stage, spec.Slug, err))
				}
			}
		}

		// Restore lifecycle and notes via UpdateSpec.
		if spec.Lifecycle != "" || spec.Notes != "" || spec.SupersededBy != "" || spec.Supersedes != "" {
			var lifecycle, notes *string
			if spec.Lifecycle != "" {
				l := string(spec.Lifecycle)
				lifecycle = &l
			}
			if spec.Notes != "" {
				notes = &spec.Notes
			}
			// UpdateSpec only supports intent, stage, priority, complexity, notes.
			// lifecycle/supersededBy/supersedes are set via other paths.
			if notes != nil {
				if _, err := e.backend.UpdateSpec(ctx, spec.Slug, nil, nil, nil, nil, notes); err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("update notes for %q: %v", spec.Slug, err))
				}
			}
			_ = lifecycle // lifecycle is set implicitly by storage layer
		}

		res.Specs++
	}

	// 4. Decisions
	for _, dec := range doc.Data.Decisions {
		if _, err := e.backend.CreateDecision(ctx, dec.Slug, dec.Title, dec.Body, dec.Rationale,
			dec.Question, dec.RejectedAlternatives, dec.Confidence,
			dec.Tags, dec.Scope, dec.OriginSpec, dec.OriginStage); err != nil {
			return nil, fmt.Errorf("create decision %q: %w", dec.Slug, err)
		}
		// Restore status if not the default (proposed).
		if dec.Status != "" && dec.Status != storage.DecisionStatusProposed {
			status := dec.Status
			var supersededBy *string
			if dec.SupersededBy != "" {
				supersededBy = &dec.SupersededBy
			}
			if _, err := e.backend.UpdateDecision(ctx, dec.Slug, 0, nil, &status, nil, nil, supersededBy,
				nil, nil, nil, nil, nil, nil, nil); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("update decision status %q: %v", dec.Slug, err))
			}
		}
		res.Decisions++
	}

	// 5. Slices — created after specs exist
	for _, sl := range doc.Data.Slices {
		if err := e.backend.CreateSlice(ctx, sl); err != nil {
			return nil, fmt.Errorf("create slice %q: %w", sl.Slug, err)
		}
		// Restore claimed/done status.
		if sl.Status == storage.SliceStatusClaimed && sl.AssignedTo != "" {
			if _, err := e.backend.ClaimSlice(ctx, sl.Slug, sl.AssignedTo); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("claim slice %q: %v", sl.Slug, err))
			}
		}
		if sl.Status == storage.SliceStatusDone {
			if sl.AssignedTo != "" {
				// Must claim before completing.
				if _, err := e.backend.ClaimSlice(ctx, sl.Slug, sl.AssignedTo); err != nil {
					res.Warnings = append(res.Warnings, fmt.Sprintf("claim slice %q before complete: %v", sl.Slug, err))
				}
			}
			if _, err := e.backend.CompleteSlice(ctx, sl.Slug); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("complete slice %q: %v", sl.Slug, err))
			}
		}
		res.Slices++
	}

	// 6. Edges
	for _, edge := range doc.Data.Edges {
		edgeType := storage.EdgeType(edge.Type)
		if _, err := e.backend.AddEdge(ctx, edge.FromSlug, edge.ToSlug, edgeType); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("add edge %s->%s (%s): %v", edge.FromSlug, edge.ToSlug, edge.Type, err))
			continue
		}
		res.Edges++
	}

	// 7. Findings — group by (spec_slug, pass_type)
	type findingKey struct {
		slug     string
		passType storage.PassType
	}
	findingsMap := make(map[findingKey][]storage.AnalyticalFindingInput)
	for _, f := range doc.Data.Findings {
		key := findingKey{slug: f.SpecSlug, passType: f.PassType}
		findingsMap[key] = append(findingsMap[key], storage.AnalyticalFindingInput{
			Severity:   f.Severity,
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
		})
	}
	for key, inputs := range findingsMap {
		if _, err := e.backend.StoreFindings(ctx, key.slug, key.passType, inputs); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("store findings for %q/%s: %v", key.slug, key.passType, err))
			continue
		}
		res.Findings += len(inputs)
	}

	// 8. ChangeLogs — auto-created by storage mutations; track count only
	res.ChangeLogs = len(doc.Data.ChangeLogs)

	// 9. Conversations
	for _, conv := range doc.Data.Conversations {
		if _, err := e.backend.RecordConversation(ctx, conv.SpecSlug, *conv); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("record conversation for %q: %v", conv.SpecSlug, err))
			continue
		}
		res.Conversations++
	}

	// 10. SyncMappings
	for _, m := range doc.Data.SyncMappings {
		if _, err := e.backend.CreateSyncMapping(ctx, m.SpecSlug, m.Adapter, m.ExternalID); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("create sync mapping %q/%s: %v", m.SpecSlug, m.Adapter, err))
			continue
		}
		res.SyncMappings++
	}

	// 11. ExecutionEvents — use RecordProgress for all types (RecordCompletion transitions stage)
	for _, ev := range doc.Data.ExecutionEvents {
		switch ev.Type {
		case storage.ExecutionEventTypeBlocker:
			if err := e.backend.RecordBlocker(ctx, ev.SpecSlug, ev.Agent, ev.Message); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("record blocker %q: %v", ev.SpecSlug, err))
				continue
			}
		default:
			// Progress and completion events both use RecordProgress to avoid stage transitions.
			if err := e.backend.RecordProgress(ctx, ev.SpecSlug, ev.Agent, ev.Message); err != nil {
				res.Warnings = append(res.Warnings, fmt.Sprintf("record execution event %q: %v", ev.SpecSlug, err))
				continue
			}
		}
		res.ExecEvents++
	}

	return res, nil
}

// ---------------------------------------------------------------------------
// Verify
// ---------------------------------------------------------------------------

// EntityDiff summarises the comparison for one entity type.
type EntityDiff struct {
	EntityType string
	Matched    int
	Missing    int // in provided but not in current
	Extra      int // in current but not in provided
}

// VerifyResult holds the comparison between a provided document and the current state.
type VerifyResult struct {
	ProjectSlug string
	Diffs       []EntityDiff
	OK          bool // true when all diffs show zero missing and zero extra
}

// Verify re-exports the project and compares entity-by-entity with the provided document.
func (e *Engine) Verify(ctx context.Context, data []byte, projectSlug string) (*VerifyResult, error) {
	var provided Document
	if err := json.Unmarshal(data, &provided); err != nil {
		return nil, fmt.Errorf("verify unmarshal: %w", err)
	}

	slug := projectSlug
	if slug == "" {
		slug = provided.ProjectSlug
	}
	if slug == "" {
		return nil, errors.New("project slug required for verify")
	}

	current, err := e.collect(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("verify collect: %w", err)
	}

	vr := &VerifyResult{ProjectSlug: slug, OK: true}

	appendDiff := func(d EntityDiff) {
		vr.Diffs = append(vr.Diffs, d)
		if d.Missing > 0 || d.Extra > 0 {
			vr.OK = false
		}
	}

	// Specs
	appendDiff(compareBySlug("specs", provided.Data.Specs, current.Data.Specs, func(s *storage.Spec) string { return s.Slug }))

	// Decisions
	appendDiff(compareBySlug("decisions", provided.Data.Decisions, current.Data.Decisions, func(d *storage.Decision) string { return d.Slug }))

	// Slices
	appendDiff(compareBySlug("slices", provided.Data.Slices, current.Data.Slices, func(s *storage.Slice) string { return s.Slug }))

	// Edges
	appendDiff(compareEdges(provided.Data.Edges, current.Data.Edges))

	// Findings
	appendDiff(compareBySlug("findings", provided.Data.Findings, current.Data.Findings, func(f *storage.AnalyticalFinding) string {
		return fmt.Sprintf("%s/%s/%s", f.SpecSlug, f.PassType, f.Summary)
	}))

	// ChangeLogs
	appendDiff(compareBySlug("changelogs", provided.Data.ChangeLogs, current.Data.ChangeLogs, func(c *storage.ChangeLogEntry) string {
		return fmt.Sprintf("%s/v%d/%s", c.SpecSlug, c.Version, c.Stage)
	}))

	// Conversations
	appendDiff(compareBySlug("conversations", provided.Data.Conversations, current.Data.Conversations, func(c *storage.ConversationLogEntry) string {
		return fmt.Sprintf("%s/%s/v%d", c.SpecSlug, c.Stage, c.Version)
	}))

	// SyncMappings
	appendDiff(compareBySlug("sync_mappings", provided.Data.SyncMappings, current.Data.SyncMappings, func(m *storage.SyncMapping) string {
		return fmt.Sprintf("%s/%s/%s", m.SpecSlug, m.Adapter, m.ExternalID)
	}))

	// ExecutionEvents
	appendDiff(compareBySlug("execution_events", provided.Data.ExecutionEvents, current.Data.ExecutionEvents, func(ev *storage.ExecutionEvent) string {
		return fmt.Sprintf("%s/%s/%s/%s", ev.SpecSlug, ev.Agent, ev.Type, ev.Message)
	}))

	return vr, nil
}

// compareBySlug builds maps by key and counts matched/missing/extra.
func compareBySlug[T any](entityType string, provided, current []*T, keyFn func(*T) string) EntityDiff {
	provMap := make(map[string]bool, len(provided))
	for _, item := range provided {
		provMap[keyFn(item)] = true
	}
	curMap := make(map[string]bool, len(current))
	for _, item := range current {
		curMap[keyFn(item)] = true
	}

	diff := EntityDiff{EntityType: entityType}
	for k := range provMap {
		if curMap[k] {
			diff.Matched++
		} else {
			diff.Missing++
		}
	}
	for k := range curMap {
		if !provMap[k] {
			diff.Extra++
		}
	}
	return diff
}

// compareEdges uses a composite key of from+to+type.
func compareEdges(provided, current []Edge) EntityDiff {
	key := func(e Edge) string {
		return e.FromSlug + "|" + e.ToSlug + "|" + e.Type
	}

	provMap := make(map[string]bool, len(provided))
	for _, e := range provided {
		provMap[key(e)] = true
	}
	curMap := make(map[string]bool, len(current))
	for _, e := range current {
		curMap[key(e)] = true
	}

	diff := EntityDiff{EntityType: "edges"}
	for k := range provMap {
		if curMap[k] {
			diff.Matched++
		} else {
			diff.Missing++
		}
	}
	for k := range curMap {
		if !provMap[k] {
			diff.Extra++
		}
	}
	return diff
}
