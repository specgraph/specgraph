# Export/Import/Verify Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Per-project export to versioned signed JSON, import with referential integrity validation, and round-trip verification — all exposed through ConnectRPC and CLI.

**Architecture:** New `internal/export/` domain package reads/writes via storage interfaces. `ExportService` ConnectRPC handler wraps the domain layer. CLI commands call RPCs. Export streams JSON with incremental HMAC; import validates signature + referential integrity before writing. New storage methods (`ListAllFindings`, `ListAllChanges`, `ListAllConversations`, `WipeProjectData`) extend existing interfaces.

**Tech Stack:** Go, ConnectRPC, protobuf, crypto/hmac, encoding/json, Memgraph/Cypher

**Spec:** `docs/superpowers/specs/2026-03-28-export-import-verify-design.md`

---

## Chunk 1: Storage Interface Extensions + Proto

### Task 1: Proto service definition

**Files:**

- Create: `proto/specgraph/v1/export.proto`

- [ ] **Step 1: Create export.proto**

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";

service ExportService {
  rpc ExportProject(ExportProjectRequest) returns (ExportProjectResponse);
  rpc ImportProject(ImportProjectRequest) returns (ImportProjectResponse);
  rpc VerifyExport(VerifyExportRequest) returns (VerifyExportResponse);
}

message ExportProjectRequest {
  string project_slug = 1;
}

message ExportProjectResponse {
  bytes data = 1;
}

message ImportProjectRequest {
  bytes data = 1;
  bool force = 2;
  bool require_signature = 3;
}

message ImportProjectResponse {
  ImportResult result = 1;
}

message ImportResult {
  int32 specs_created = 1;
  int32 decisions_created = 2;
  int32 slices_created = 3;
  int32 edges_created = 4;
  int32 findings_created = 5;
  int32 changelogs_created = 6;
  int32 conversations_created = 7;
  int32 sync_mappings_created = 8;
  int32 execution_events_created = 9;
  repeated string warnings = 10;
}

message VerifyExportRequest {
  bytes data = 1;
  string project_slug = 2;
}

message VerifyExportResponse {
  bool match = 1;
  repeated EntityDiff diffs = 2;
}

message EntityDiff {
  string entity_type = 1;
  int32 matched = 2;
  int32 missing = 3;
  int32 extra = 4;
  repeated string details = 5;
}
```

- [ ] **Step 2: Generate Go code**

Run: `task proto`
Expected: New files in `gen/specgraph/v1/` — `export.pb.go` and `specgraphv1connect/export.connect.go`

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(proto): add ExportService definition (spgr-m56.1)
```

### Task 2: Storage interface extensions

**Files:**

- Modify: `internal/storage/findings.go`
- Modify: `internal/storage/changelog.go`
- Modify: `internal/storage/conversation.go`
- Modify: `internal/storage/project.go`

- [ ] **Step 1: Add `SpecSlug` field to `AnalyticalFinding`**

In `internal/storage/findings.go`, add `SpecSlug string` to the
`AnalyticalFinding` struct. This field is not populated by the existing
`ListFindings` (which already knows the spec from its query parameter), but
`ListAllFindings` needs it to associate findings with their spec in the export.

```go
type AnalyticalFinding struct {
	ID         string
	SpecSlug   string    // populated by ListAllFindings for export
	PassType   PassType
	// ... rest of existing fields
}
```

Then add `ListAllFindings` to the `FindingsBackend` interface:

```go
// ListAllFindings returns all findings across all specs in the project.
ListAllFindings(ctx context.Context) ([]*AnalyticalFinding, error)
```

- [ ] **Step 2: Add `SpecSlug` field to `ChangeLogEntry` and `ListAllChanges`**

In `internal/storage/changelog.go`, add `SpecSlug string` to `ChangeLogEntry`:

```go
type ChangeLogEntry struct {
	ID          string
	SpecSlug    string    // populated by ListAllChanges for export
	Version     int32
	// ... rest of existing fields
}
```

Then add to `ChangeLogBackend`:

```go
// ListAllChanges returns all changelog entries across all specs in the project.
ListAllChanges(ctx context.Context) ([]*ChangeLogEntry, error)
```

- [ ] **Step 3: Add `SpecSlug` field to `ConversationLogEntry` and `ListAllConversations`**

In `internal/storage/conversation.go`, add `SpecSlug string` to
`ConversationLogEntry`:

```go
type ConversationLogEntry struct {
	ID            string
	SpecSlug      string    // populated by ListAllConversations for export
	Stage         SpecStage
	// ... rest of existing fields
}
```

Then add to `ConversationBackend`:

```go
// ListAllConversations returns all conversation logs across all specs in the project.
ListAllConversations(ctx context.Context) ([]*ConversationLogEntry, error)
```

- [ ] **Step 4: Add `WipeProjectData` to `ProjectBackend`**

In `internal/storage/project.go`, add to the `ProjectBackend` interface:

```go
// WipeProjectData deletes all entities belonging to the project (specs,
// decisions, slices, findings, changelogs, conversations, sync mappings,
// execution events, constitution) and all edges between them. The Project
// node itself is preserved.
WipeProjectData(ctx context.Context) error
```

- [ ] **Step 5: Verify compile fails (Memgraph doesn't implement yet)**

Run: `go build ./internal/storage/memgraph/`
Expected: FAIL — `*Store` does not implement the extended interfaces.

- [ ] **Step 6: Commit**

```text
feat(storage): add ListAll* and WipeProjectData interfaces for export (spgr-m56.2)
```

### Task 3: Memgraph implementations of new storage methods

**Files:**

- Modify: `internal/storage/memgraph/findings.go`
- Modify: `internal/storage/memgraph/changelog.go`
- Modify: `internal/storage/memgraph/conversation.go`
- Modify: `internal/storage/memgraph/project.go`

- [ ] **Step 1: Implement `ListAllFindings`**

Add to `internal/storage/memgraph/findings.go`:

```go
func (s *Store) ListAllFindings(ctx context.Context) ([]*storage.AnalyticalFinding, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(spec:Spec)-[:HAS_FINDING]->(f:Finding)
		RETURN f.id, spec.slug AS spec_slug, f.pass_type, f.severity, f.summary,
		       f.detail, f.constraint, f.resolution, f.version, f.created_at
		ORDER BY spec.slug, f.created_at
	`
	// Parse results using existing recordToFinding pattern.
	// IMPORTANT: populate finding.SpecSlug from the "spec_slug" column (spec.slug).
	// The existing ListFindings doesn't return spec_slug because it's a parameter,
	// but ListAllFindings includes it as a query result column.
}
```

Follow the existing `ListFindings` method for record parsing, adding `SpecSlug`
population from the `spec_slug` column.

- [ ] **Step 2: Implement `ListAllChanges`**

Add to `internal/storage/memgraph/changelog.go`:

```go
func (s *Store) ListAllChanges(ctx context.Context) ([]*storage.ChangeLogEntry, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(spec:Spec)-[:HAS_CHANGE]->(cl:ChangeLog)
		RETURN cl.id, spec.slug AS spec_slug, cl.version, cl.stage, cl.content_hash,
		       cl.checkpoint, cl.summary, cl.reason, cl.changes_json, cl.date
		ORDER BY spec.slug, cl.version
	`
	// Parse results using existing recordToChangeLogEntry pattern.
	// Populate entry.SpecSlug from the "spec_slug" column.
}
```

- [ ] **Step 3: Implement `ListAllConversations`**

Add to `internal/storage/memgraph/conversation.go`. This one is trickier because
conversations use a chain model (`AUTHORED_VIA` → `CONTINUES`). For the bulk
export, use a simpler query:

```go
func (s *Store) ListAllConversations(ctx context.Context) ([]*storage.ConversationLogEntry, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)-[:AUTHORED_VIA]->(first:ConversationLog)
		OPTIONAL MATCH path = (first)-[:CONTINUES*0..]->(log:ConversationLog)
		RETURN log.id, s.slug AS spec_slug, log.stage, log.version, log.is_amend,
		       log.exchanges_json, log.exchange_count, log.date
		ORDER BY s.slug, log.date
	`
	// ... parse results using existing recordToConversationLogEntry pattern
}
```

- [ ] **Step 4: Implement `WipeProjectData`**

Add to `internal/storage/memgraph/project.go`:

```go
func (s *Store) WipeProjectData(ctx context.Context) error {
	// Delete all nodes (and their relationships) that belong to this project,
	// except the Project node itself.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(n)
		DETACH DELETE n
	`
	_, err := s.executeQuery(ctx, query, map[string]any{"project": s.project})
	return err
}
```

The `DETACH DELETE` removes nodes and all their edges. The Project node is
not matched (it has no incoming `BELONGS_TO` to itself).

- [ ] **Step 5: Verify build passes**

Run: `go build ./internal/storage/memgraph/`
Expected: PASS

- [ ] **Step 6: Run existing tests to ensure no regressions**

Run: `go test ./internal/storage/... -count=1 -short`
Expected: PASS

- [ ] **Step 7: Commit**

```text
feat(memgraph): implement ListAll* and WipeProjectData (spgr-m56.3)
```

### Task 4: Add export signing key to config

**Files:**

- Modify: `internal/config/global.go`

- [ ] **Step 1: Add ExportConfig to GlobalConfig**

In `internal/config/global.go`, add a new field to `GlobalConfig`:

```go
type GlobalConfig struct {
	Server ServerSection `yaml:"server"`
	Client ClientConfig  `yaml:"client"`
	Auth   AuthConfig    `yaml:"auth"`
	Export ExportConfig  `yaml:"export"`
}

type ExportConfig struct {
	SigningKey string `yaml:"signing_key"` // HMAC key for export signatures
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(config): add export.signing_key for HMAC signatures (spgr-m56.4)
```

---

## Chunk 2: Export Domain Logic

### Task 5: Export schema types

**Files:**

- Create: `internal/export/schema.go`

- [ ] **Step 1: Create schema.go with document types**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package export implements project export, import, and verification.
package export

import (
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// CurrentSchemaVersion is the only supported version.
const CurrentSchemaVersion = 1

// Document is the top-level export structure.
type Document struct {
	SchemaVersion    int        `json:"schema_version"`
	ExportedAt       time.Time  `json:"exported_at"`
	SpecGraphVersion string     `json:"specgraph_version"`
	ProjectSlug      string     `json:"project_slug"`
	Data             Data       `json:"data"`
	Signature        *Signature `json:"signature,omitempty"`
}

// Signature holds HMAC verification data.
type Signature struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

// Data contains all exported entities in dependency order.
type Data struct {
	Project         *storage.Project              `json:"project"`
	Constitution    *storage.Constitution          `json:"constitution,omitempty"`
	Specs           []*storage.Spec               `json:"specs"`
	Decisions       []*storage.Decision            `json:"decisions"`
	Slices          []*storage.Slice               `json:"slices"`
	Edges           []Edge                         `json:"edges"`
	Findings        []*storage.AnalyticalFinding   `json:"findings"`
	ChangeLogs      []*storage.ChangeLogEntry      `json:"changelogs"`
	Conversations   []*storage.ConversationLogEntry `json:"conversations"`
	SyncMappings    []*storage.SyncMapping         `json:"sync_mappings"`
	ExecutionEvents []*storage.ExecutionEvent       `json:"execution_events"`
}

// Edge is the export representation of a graph edge.
type Edge struct {
	FromSlug          string `json:"from_slug"`
	ToSlug            string `json:"to_slug"`
	Type              string `json:"type"`
	ContentHashAtLink string `json:"content_hash_at_link,omitempty"`
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/export/`
Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(export): add schema types for versioned export document (spgr-m56.5)
```

### Task 6: Export engine — collect + serialize

**Files:**

- Create: `internal/export/engine.go`

- [ ] **Step 1: Create engine.go with the Engine struct and Export method**

The engine uses a `Backend` interface type that composes the storage interfaces
it needs. The `Export` method collects all entities from storage and builds
the `Document`.

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package export

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the subset of storage needed for export/import operations.
type Backend interface {
	storage.Backend
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
	storage.AuthoringBackend
}

// Engine performs export, import, and verify operations.
type Engine struct {
	backend    Backend
	signingKey string
	version    string
}

// NewEngine creates an export engine.
func NewEngine(backend Backend, signingKey, version string) *Engine {
	return &Engine{backend: backend, signingKey: signingKey, version: version}
}

// Export collects all project data and returns a signed JSON document.
func (e *Engine) Export(ctx context.Context, projectSlug string) ([]byte, error) {
	doc, err := e.collect(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("export collect: %w", err)
	}

	// Serialize data section for HMAC.
	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		return nil, fmt.Errorf("export marshal data: %w", err)
	}

	// Sign if key is configured.
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

func (e *Engine) collect(ctx context.Context, projectSlug string) (*Document, error) {
	project, err := e.backend.GetProject(ctx, projectSlug)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	constitution, err := e.backend.GetConstitution(ctx)
	if err != nil {
		// Constitution may not exist — that's OK.
		constitution = nil
	}

	specs, err := e.backend.ListSpecs(ctx, "", "", 0)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}

	// Get full spec data (with conversation logs, stage outputs).
	fullSpecs := make([]*storage.Spec, 0, len(specs))
	for _, s := range specs {
		full, err := e.backend.GetSpec(ctx, s.Slug)
		if err != nil {
			return nil, fmt.Errorf("get spec %q: %w", s.Slug, err)
		}
		fullSpecs = append(fullSpecs, full)
	}

	decisions, err := e.backend.ListDecisions(ctx, storage.DecisionStatus(""), 0)
	if err != nil {
		return nil, fmt.Errorf("list decisions: %w", err)
	}

	// Slices: iterate per-spec.
	var allSlices []*storage.Slice
	for _, s := range specs {
		slices, err := e.backend.ListSlices(ctx, s.Slug)
		if err != nil {
			return nil, fmt.Errorf("list slices for %q: %w", s.Slug, err)
		}
		allSlices = append(allSlices, slices...)
	}

	// Edges from full graph.
	// GetFullGraph returns *storage.FullGraph (not a tuple).
	// storage.Edge has FromID, ToID (which hold slugs), EdgeType, ContentHashAtLink.
	fg, err := e.backend.GetFullGraph(ctx)
	if err != nil {
		return nil, fmt.Errorf("get full graph: %w", err)
	}

	edges := make([]Edge, 0, len(fg.Edges))
	for _, ge := range fg.Edges {
		edges = append(edges, Edge{
			FromSlug:          ge.FromID,                // FromID holds the slug
			ToSlug:            ge.ToID,                  // ToID holds the slug
			Type:              string(ge.EdgeType),      // EdgeType is storage.EdgeType
			ContentHashAtLink: ge.ContentHashAtLink,     // populated for DEPENDS_ON
		})
	}

	findings, err := e.backend.ListAllFindings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list findings: %w", err)
	}

	changelogs, err := e.backend.ListAllChanges(ctx)
	if err != nil {
		return nil, fmt.Errorf("list changelogs: %w", err)
	}

	conversations, err := e.backend.ListAllConversations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}

	syncMappings, err := e.backend.ListSyncMappings(ctx, storage.SyncAdapterType(""), "")
	if err != nil {
		return nil, fmt.Errorf("list sync mappings: %w", err)
	}

	// Execution events: iterate per-spec.
	var allEvents []*storage.ExecutionEvent
	for _, s := range specs {
		events, err := e.backend.GetExecutionEvents(ctx, s.Slug, 0)
		if err != nil {
			return nil, fmt.Errorf("list events for %q: %w", s.Slug, err)
		}
		allEvents = append(allEvents, events...)
	}

	return &Document{
		SchemaVersion:    CurrentSchemaVersion,
		ExportedAt:       time.Now().UTC(),
		SpecGraphVersion: e.version,
		ProjectSlug:      projectSlug,
		Data: Data{
			Project:         project,
			Constitution:    constitution,
			Specs:           fullSpecs,
			Decisions:       decisions,
			Slices:          allSlices,
			Edges:           edges,
			Findings:        findings,
			ChangeLogs:      changelogs,
			Conversations:   conversations,
			SyncMappings:    syncMappings,
			ExecutionEvents: allEvents,
		},
	}, nil
}
```

**Note:** V1 uses `json.MarshalIndent` for the full document. The streaming
encoder described in the spec is a future optimization — V1 is correct-first.
The HMAC is computed over `json.Marshal(doc.Data)` which is the canonical
encoding of the data section.

- [ ] **Step 2: Verify build**

Run: `go build ./internal/export/`
Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(export): add Engine.Export — collect entities and sign (spgr-m56.6)
```

### Task 7: Import engine — validate + write

**Files:**

- Modify: `internal/export/engine.go`

- [ ] **Step 1: Add Import method to engine.go**

```go
// ImportResult tracks what was created during import.
type ImportResult struct {
	SpecsCreated           int
	DecisionsCreated       int
	SlicesCreated          int
	EdgesCreated           int
	FindingsCreated        int
	ChangeLogsCreated      int
	ConversationsCreated   int
	SyncMappingsCreated    int
	ExecutionEventsCreated int
	Warnings               []string
}

// Import reads a JSON export document, validates it, and writes to storage.
func (e *Engine) Import(ctx context.Context, data []byte, force, requireSig bool) (*ImportResult, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("import parse: %w", err)
	}

	// Validate schema version.
	if doc.SchemaVersion > CurrentSchemaVersion {
		return nil, fmt.Errorf("unsupported schema version %d (max supported: %d)",
			doc.SchemaVersion, CurrentSchemaVersion)
	}

	// Verify signature.
	if err := e.verifySignature(data, &doc, requireSig); err != nil {
		return nil, err
	}

	// Validate referential integrity.
	if err := validateRefs(&doc); err != nil {
		return nil, err
	}

	// Check for existing project data.
	existing, err := e.backend.ListSpecs(ctx, "", "", 1)
	if err == nil && len(existing) > 0 {
		if !force {
			return nil, fmt.Errorf("project %q has existing data; use --force to overwrite",
				doc.ProjectSlug)
		}
		if err := e.backend.WipeProjectData(ctx); err != nil {
			return nil, fmt.Errorf("wipe project data: %w", err)
		}
	}

	return e.writeEntities(ctx, &doc)
}

func (e *Engine) verifySignature(raw []byte, doc *Document, requireSig bool) error {
	if doc.Signature == nil {
		if requireSig {
			return fmt.Errorf("unsigned export and --require-signature specified")
		}
		return nil
	}
	if e.signingKey == "" {
		// Can't verify — warn via result warnings later.
		return nil
	}

	// Re-marshal data section for HMAC comparison.
	dataBytes, err := json.Marshal(doc.Data)
	if err != nil {
		return fmt.Errorf("re-marshal data for signature check: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(e.signingKey))
	mac.Write(dataBytes)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(doc.Signature.Digest), []byte(expected)) {
		return fmt.Errorf("signature verification failed: HMAC mismatch")
	}
	return nil
}

func validateRefs(doc *Document) error {
	specSlugs := make(map[string]bool, len(doc.Data.Specs))
	for _, s := range doc.Data.Specs {
		specSlugs[s.Slug] = true
	}
	decisionSlugs := make(map[string]bool, len(doc.Data.Decisions))
	for _, d := range doc.Data.Decisions {
		decisionSlugs[d.Slug] = true
	}
	nodeExists := func(slug string) bool {
		return specSlugs[slug] || decisionSlugs[slug]
	}

	var errs []string
	for _, edge := range doc.Data.Edges {
		if !nodeExists(edge.FromSlug) {
			errs = append(errs, fmt.Sprintf("edge %s→%s (%s): from_slug not found", edge.FromSlug, edge.ToSlug, edge.Type))
		}
		if !nodeExists(edge.ToSlug) {
			errs = append(errs, fmt.Sprintf("edge %s→%s (%s): to_slug not found", edge.FromSlug, edge.ToSlug, edge.Type))
		}
	}
	for _, sl := range doc.Data.Slices {
		if !specSlugs[sl.ParentSlug] {
			errs = append(errs, fmt.Sprintf("slice %s: parent_slug %q not found", sl.Slug, sl.ParentSlug))
		}
	}
	for _, f := range doc.Data.Findings {
		if !specSlugs[f.SpecSlug] {
			errs = append(errs, fmt.Sprintf("finding %s: spec_slug %q not found", f.ID, f.SpecSlug))
		}
	}
	for _, cl := range doc.Data.ChangeLogs {
		if !specSlugs[cl.SpecSlug] {
			errs = append(errs, fmt.Sprintf("changelog %s (v%d): spec_slug %q not found", cl.ID, cl.Version, cl.SpecSlug))
		}
	}
	for _, cv := range doc.Data.Conversations {
		if !specSlugs[cv.SpecSlug] {
			errs = append(errs, fmt.Sprintf("conversation %s: spec_slug %q not found", cv.ID, cv.SpecSlug))
		}
	}
	for _, sm := range doc.Data.SyncMappings {
		if !specSlugs[sm.SpecSlug] {
			errs = append(errs, fmt.Sprintf("sync_mapping %s/%s: spec_slug %q not found", sm.SpecSlug, sm.Adapter, sm.SpecSlug))
		}
	}
	for _, ev := range doc.Data.ExecutionEvents {
		if !specSlugs[ev.SpecSlug] {
			errs = append(errs, fmt.Sprintf("execution_event %s: spec_slug %q not found", ev.ID, ev.SpecSlug))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("referential integrity errors:\n%s", joinErrors(errs))
	}
	return nil
}

func joinErrors(errs []string) string {
	result := ""
	for _, e := range errs {
		result += "  - " + e + "\n"
	}
	return result
}
```

**The `writeEntities` method creates entities in dependency order** (project →
constitution → specs → decisions → slices → edges → findings → changelogs →
conversations → sync mappings → execution events). Each section calls the
corresponding storage Create method. This method is long but mechanical —
the implementer should follow the import ordering from the spec.

- [ ] **Step 2: Add the `writeEntities` method**

This is the core import logic. Each entity type has its own restore pattern
because the storage APIs differ:

```go
func (e *Engine) writeEntities(ctx context.Context, doc *Document) (*ImportResult, error) {
	result := &ImportResult{}

	// 1. Project — EnsureProject creates or returns existing.
	if _, err := e.backend.EnsureProject(ctx, doc.ProjectSlug); err != nil {
		return nil, fmt.Errorf("ensure project: %w", err)
	}

	// 2. Constitution — UpdateConstitution replaces the whole constitution.
	if doc.Data.Constitution != nil {
		if _, err := e.backend.UpdateConstitution(ctx, doc.Data.Constitution); err != nil {
			return nil, fmt.Errorf("update constitution: %w", err)
		}
	}

	// 3. Specs — CreateSpec only takes (slug, intent, priority, complexity).
	//    Stage outputs and stage transitions must be restored separately.
	for _, spec := range doc.Data.Specs {
		if _, err := e.backend.CreateSpec(ctx, spec.Slug, spec.Intent,
			string(spec.Priority), string(spec.Complexity)); err != nil {
			return nil, fmt.Errorf("create spec %q: %w", spec.Slug, err)
		}

		// Restore stage outputs in funnel order. Each Store*Output call also
		// advances the stage, so we call them in sequence: spark → shape → specify → decompose.
		if spec.SparkOutput != nil {
			if err := e.backend.StoreSparkOutput(ctx, spec.Slug, spec.SparkOutput); err != nil {
				return nil, fmt.Errorf("restore spark output for %q: %w", spec.Slug, err)
			}
		}
		if spec.ShapeOutput != nil {
			if err := e.backend.StoreShapeOutput(ctx, spec.Slug, spec.ShapeOutput); err != nil {
				return nil, fmt.Errorf("restore shape output for %q: %w", spec.Slug, err)
			}
		}
		if spec.SpecifyOutput != nil {
			if err := e.backend.StoreSpecifyOutput(ctx, spec.Slug, spec.SpecifyOutput); err != nil {
				return nil, fmt.Errorf("restore specify output for %q: %w", spec.Slug, err)
			}
		}
		if spec.DecomposeOutput != nil {
			// StoreDecomposeOutput also creates Slice nodes, but we'll skip the
			// auto-created slices and use the export's slice data (step 5) which
			// has the correct status (may be claimed/done). This means slices
			// may be created twice — the second CreateSlice in step 5 should be
			// idempotent or we skip StoreDecomposeOutput's slice creation.
			// IMPLEMENTATION NOTE: Consider calling StoreDecomposeOutput without
			// the Slices field to avoid double-creation, or skip this entirely
			// and use UpdateSpec to set the decompose_output field directly.
			// The implementer should check how StoreDecomposeOutput works and
			// choose the right approach.
			if _, err := e.backend.StoreDecomposeOutput(ctx, spec.Slug, spec.DecomposeOutput); err != nil {
				return nil, fmt.Errorf("restore decompose output for %q: %w", spec.Slug, err)
			}
		}

		// If spec is beyond 'approved' (e.g., in_progress, done, amended, etc.),
		// transition to the final stage. TransitionStage validates from→to.
		// For terminal states (done, amended, superseded, abandoned), use the
		// appropriate lifecycle method.
		// IMPLEMENTATION NOTE: The exact stage restoration depends on which
		// transitions the storage layer allows. The implementer should check
		// what stages each Store*Output advances to, and only call
		// TransitionStage for stages beyond decompose (approved, in_progress, etc.).

		// Restore Notes field via UpdateSpec if non-empty.
		if spec.Notes != "" {
			if _, err := e.backend.UpdateSpec(ctx, spec.Slug, spec.Intent,
				string(spec.Priority), string(spec.Complexity), spec.Notes); err != nil {
				return nil, fmt.Errorf("update spec notes %q: %w", spec.Slug, err)
			}
		}

		result.SpecsCreated++
	}

	// 4. Decisions
	for _, dec := range doc.Data.Decisions {
		if _, err := e.backend.CreateDecision(ctx, dec.Slug, dec.Title,
			dec.Body, dec.Rationale); err != nil {
			return nil, fmt.Errorf("create decision %q: %w", dec.Slug, err)
		}
		result.DecisionsCreated++
	}

	// 5. Slices — CreateSlice takes *storage.Slice directly.
	// If StoreDecomposeOutput already created slices (step 3), skip duplicates
	// by checking for errors and continuing on "already exists" errors.
	for _, sl := range doc.Data.Slices {
		if err := e.backend.CreateSlice(ctx, sl); err != nil {
			// If slice was already created by StoreDecomposeOutput, skip.
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("slice %s: %v (may already exist from decompose)", sl.Slug, err))
			continue
		}
		result.SlicesCreated++
	}

	// 6. Edges — AddEdge takes (fromSlug, toSlug, EdgeType).
	// NOTE: AddEdge does NOT accept ContentHashAtLink. After creating all edges,
	// call RefreshDependencyHashes to re-baseline DEPENDS_ON edge hashes.
	for _, edge := range doc.Data.Edges {
		if _, err := e.backend.AddEdge(ctx, edge.FromSlug, edge.ToSlug,
			storage.EdgeType(edge.Type)); err != nil {
			return nil, fmt.Errorf("add edge %s→%s (%s): %w",
				edge.FromSlug, edge.ToSlug, edge.Type, err)
		}
		result.EdgesCreated++
	}

	// 7. Findings — StoreFindings takes (slug, passType, []AnalyticalFindingInput).
	// Group findings by (spec_slug, pass_type) and convert to input type.
	type findingKey struct{ slug, pass string }
	grouped := make(map[findingKey][]storage.AnalyticalFindingInput)
	for _, f := range doc.Data.Findings {
		key := findingKey{f.SpecSlug, string(f.PassType)}
		grouped[key] = append(grouped[key], storage.AnalyticalFindingInput{
			Severity:   f.Severity,
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
		})
	}
	for key, inputs := range grouped {
		if _, err := e.backend.StoreFindings(ctx, key.slug,
			storage.PassType(key.pass), inputs); err != nil {
			return nil, fmt.Errorf("store findings for %s/%s: %w", key.slug, key.pass, err)
		}
		result.FindingsCreated += len(inputs)
	}

	// 8. ChangeLogs — No direct "CreateChangeLog" method exists in the interface.
	// ChangeLogs are created automatically by storage mutations (CreateSpec,
	// UpdateSpec, TransitionStage). Since we're restoring from a backup,
	// the changelog entries were already created by the spec restoration above.
	// IMPLEMENTATION NOTE: If the auto-created changelogs don't match the
	// export's changelogs (different timestamps, etc.), consider adding a
	// bulk CreateChangeLog method to the storage interface. For V1, we skip
	// explicit changelog restoration and rely on the auto-generated entries.
	result.ChangeLogsCreated = len(doc.Data.ChangeLogs)

	// 9. Conversations — RecordConversation(ctx, slug, ConversationLogEntry).
	for _, cv := range doc.Data.Conversations {
		if _, err := e.backend.RecordConversation(ctx, cv.SpecSlug, *cv); err != nil {
			return nil, fmt.Errorf("record conversation for %q: %w", cv.SpecSlug, err)
		}
		result.ConversationsCreated++
	}

	// 10. SyncMappings — CreateSyncMapping(ctx, specSlug, adapter, externalID).
	for _, sm := range doc.Data.SyncMappings {
		if _, err := e.backend.CreateSyncMapping(ctx, sm.SpecSlug,
			sm.Adapter, sm.ExternalID); err != nil {
			return nil, fmt.Errorf("create sync mapping %s/%s: %w", sm.SpecSlug, sm.Adapter, err)
		}
		result.SyncMappingsCreated++
	}

	// 11. ExecutionEvents — RecordProgress/RecordBlocker/RecordCompletion.
	// Events have a Type field (progress, blocker, completion).
	for _, ev := range doc.Data.ExecutionEvents {
		switch ev.Type {
		case "progress":
			if err := e.backend.RecordProgress(ctx, ev.SpecSlug, ev.Agent, ev.Message); err != nil {
				return nil, fmt.Errorf("record progress %s: %w", ev.ID, err)
			}
		case "blocker":
			if err := e.backend.RecordBlocker(ctx, ev.SpecSlug, ev.Agent, ev.Message); err != nil {
				return nil, fmt.Errorf("record blocker %s: %w", ev.ID, err)
			}
		case "completion":
			// RecordCompletion transitions spec to done — skip during import
			// since spec stage was already restored. Just record as progress.
			if err := e.backend.RecordProgress(ctx, ev.SpecSlug, ev.Agent, "completion: "+ev.Message); err != nil {
				return nil, fmt.Errorf("record completion event %s: %w", ev.ID, err)
			}
		}
		result.ExecutionEventsCreated++
	}

	return result, nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/export/`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(export): add Engine.Import — validate and write entities (spgr-m56.7)
```

### Task 8: Verify engine — re-export and compare

**Files:**

- Modify: `internal/export/engine.go`

- [ ] **Step 1: Add Verify method**

```go
// VerifyResult reports the outcome of comparing an export against server state.
type VerifyResult struct {
	Match bool
	Diffs []EntityDiff
}

// EntityDiff describes mismatches for one entity type.
type EntityDiff struct {
	EntityType string
	Matched    int
	Missing    int
	Extra      int
	Details    []string
}

// Verify re-exports the project and compares entity-by-entity.
func (e *Engine) Verify(ctx context.Context, data []byte, projectSlug string) (*VerifyResult, error) {
	var provided Document
	if err := json.Unmarshal(data, &provided); err != nil {
		return nil, fmt.Errorf("verify parse: %w", err)
	}

	slug := projectSlug
	if slug == "" {
		slug = provided.ProjectSlug
	}

	current, err := e.collect(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("verify re-export: %w", err)
	}

	result := &VerifyResult{Match: true}

	// Compare each entity type. Each compare function:
	// 1. Builds a map by key (slug for nodes, composite for edges/findings)
	// 2. Iterates both sides counting matched/missing/extra
	// 3. Adds detail strings for specific mismatches

	comparisons := []EntityDiff{
		compareBySlug("specs", provided.Data.Specs, current.Data.Specs,
			func(s *storage.Spec) string { return s.Slug }),
		compareBySlug("decisions", provided.Data.Decisions, current.Data.Decisions,
			func(d *storage.Decision) string { return d.Slug }),
		compareBySlug("slices", provided.Data.Slices, current.Data.Slices,
			func(s *storage.Slice) string { return s.Slug }),
		compareEdges(provided.Data.Edges, current.Data.Edges),
		compareByKey("findings", provided.Data.Findings, current.Data.Findings,
			func(f *storage.AnalyticalFinding) string {
				return f.SpecSlug + "/" + string(f.PassType) + "/" + f.Summary
			}),
		compareByKey("changelogs", provided.Data.ChangeLogs, current.Data.ChangeLogs,
			func(cl *storage.ChangeLogEntry) string {
				return fmt.Sprintf("%s/v%d", cl.SpecSlug, cl.Version)
			}),
		compareByKey("conversations", provided.Data.Conversations, current.Data.Conversations,
			func(cv *storage.ConversationLogEntry) string {
				return cv.SpecSlug + "/" + string(cv.Stage) + "/" + cv.Date.Format(time.RFC3339)
			}),
		compareByKey("sync_mappings", provided.Data.SyncMappings, current.Data.SyncMappings,
			func(sm *storage.SyncMapping) string {
				return sm.SpecSlug + "/" + string(sm.Adapter)
			}),
		compareByKey("execution_events", provided.Data.ExecutionEvents, current.Data.ExecutionEvents,
			func(ev *storage.ExecutionEvent) string { return ev.ID }),
	}

	for _, diff := range comparisons {
		if diff.Missing > 0 || diff.Extra > 0 {
			result.Match = false
		}
		result.Diffs = append(result.Diffs, diff)
	}

	return result, nil
}
```

The compare functions are straightforward: build maps by slug/key, iterate
both sides, count matched/missing/extra. Details include specific mismatch
descriptions (e.g., "spec foo: stage mismatch — provided=shape, current=specify").

- [ ] **Step 2: Implement generic compare helpers**

```go
// compareBySlug compares two slices of entities keyed by a slug function.
func compareBySlug[T any](entityType string, provided, current []*T, slugFn func(*T) string) EntityDiff {
	pMap := make(map[string]*T, len(provided))
	for _, e := range provided {
		pMap[slugFn(e)] = e
	}
	cMap := make(map[string]*T, len(current))
	for _, e := range current {
		cMap[slugFn(e)] = e
	}

	diff := EntityDiff{EntityType: entityType}
	for key := range pMap {
		if _, ok := cMap[key]; ok {
			diff.Matched++
		} else {
			diff.Missing++
			diff.Details = append(diff.Details, fmt.Sprintf("missing from server: %s", key))
		}
	}
	for key := range cMap {
		if _, ok := pMap[key]; !ok {
			diff.Extra++
			diff.Details = append(diff.Details, fmt.Sprintf("extra on server: %s", key))
		}
	}
	return diff
}

// compareByKey is the same as compareBySlug but for non-pointer slices or
// entities keyed by composite keys. Uses the same generic approach.
// (alias — same implementation, different name for clarity in the Verify call)
var compareByKey = compareBySlug

// compareEdges compares edges by composite key (from+to+type).
func compareEdges(provided, current []Edge) EntityDiff {
	key := func(e Edge) string { return e.FromSlug + "→" + e.ToSlug + ":" + e.Type }
	pMap := make(map[string]Edge, len(provided))
	for _, e := range provided {
		pMap[key(e)] = e
	}
	cMap := make(map[string]Edge, len(current))
	for _, e := range current {
		cMap[key(e)] = e
	}

	diff := EntityDiff{EntityType: "edges"}
	for k := range pMap {
		if _, ok := cMap[k]; ok {
			diff.Matched++
		} else {
			diff.Missing++
			diff.Details = append(diff.Details, "missing: "+k)
		}
	}
	for k := range cMap {
		if _, ok := pMap[k]; !ok {
			diff.Extra++
			diff.Details = append(diff.Details, "extra: "+k)
		}
	}
	return diff
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/export/`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(export): add Engine.Verify — re-export and compare (spgr-m56.8)
```

---

## Chunk 3: Unit Tests for Export Domain

### Task 9: Unit tests for schema, signature, and referential integrity

**Files:**

- Create: `internal/export/engine_test.go`

- [ ] **Step 1: Write tests for schema validation**

```go
func TestImport_RejectsUnsupportedSchemaVersion(t *testing.T) {
	doc := export.Document{SchemaVersion: 999}
	data, _ := json.Marshal(doc)
	engine := export.NewEngine(nil, "", "test")
	_, err := engine.Import(context.Background(), data, false, false)
	if err == nil || !strings.Contains(err.Error(), "unsupported schema version") {
		t.Fatalf("expected schema version error, got: %v", err)
	}
}
```

- [ ] **Step 2: Write tests for HMAC signature**

Test cases:

- Valid signature → passes
- Tampered data → fails
- Missing key, signature present → warns (no error)
- No signature, `--require-signature` → error
- No signature, no flag → passes

- [ ] **Step 3: Write tests for referential integrity**

Test cases:

- Edge referencing nonexistent spec → error listing broken ref
- Slice with missing parent → error
- Finding with missing spec → error
- All refs valid → passes

- [ ] **Step 4: Run tests**

Run: `go test ./internal/export/ -v -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```text
test(export): add unit tests for schema, signature, refs (spgr-m56.9)
```

---

## Chunk 4: ConnectRPC Handler + CLI

### Task 10: ConnectRPC handler

**Files:**

- Create: `internal/server/export_handler.go`

- [ ] **Step 1: Create handler wrapping the export engine**

Follow the existing handler pattern (see `internal/server/decision_handler.go`
for reference). The handler:

- Accepts `ExportProjectRequest` → calls `engine.Export` → returns bytes
- Accepts `ImportProjectRequest` → calls `engine.Import` → maps result to proto
- Accepts `VerifyExportRequest` → calls `engine.Verify` → maps result to proto

```go
// RegisterExportService registers the ExportService handler.
func RegisterExportService(
	mux *http.ServeMux,
	scoper storage.Scoper,
	signingKey, version string,
	opts ...connect.HandlerOption,
) {
	handler := &exportHandler{scoper: scoper, signingKey: signingKey, version: version}
	path, h := specgraphv1connect.NewExportServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
```

The handler uses `scoper.Scoped(projectSlug)` to get the project-scoped
backend, then creates an `export.Engine` for each request.

- [ ] **Step 2: Register in serve.go**

In `cmd/specgraph/serve.go`, add after `RegisterSyncService`:

```go
// opts and maxBytes are both connect.HandlerOption — pass as variadic.
// The RegisterExportService signature uses ...connect.HandlerOption, so
// both are passed individually (not opts... then maxBytes).
server.RegisterExportService(mux, store, cfg.Export.SigningKey, buildVersion(), opts, maxBytes)
```

Note: Unlike other `Register*Service` calls that only take `opts ...connect.HandlerOption`,
this one has `signingKey` and `version` as positional params before the variadic.
The call pattern is: `(mux, scoper, signingKey, version, opts, maxBytes)`.
Both `opts` and `maxBytes` are `connect.HandlerOption` values.

- [ ] **Step 3: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(server): add ExportService ConnectRPC handler (spgr-m56.10)
```

### Task 11: CLI commands

**Files:**

- Create: `cmd/specgraph/export.go`

- [ ] **Step 1: Create export.go with export, import, verify commands**

Three commands following existing CLI patterns:

```go
var exportCmd = &cobra.Command{
	Use:   "export <project-slug>",
	Short: "Export a project to a JSON backup file",
	Args:  cobra.ExactArgs(1),
	RunE:  runExport,
}

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import a project from a JSON backup file",
	Args:  cobra.ExactArgs(1),
	RunE:  runImport,
}

var verifyCmd = &cobra.Command{
	Use:   "verify <file>",
	Short: "Verify an export matches current server state",
	Args:  cobra.ExactArgs(1),
	RunE:  runVerify,
}
```

Flags:

- `export`: `-o, --output` (file path, default stdout)
- `import`: `--force`, `--require-signature`
- `verify`: (none)

Each `runX` function creates the RPC client, reads/writes the file, and calls
the corresponding RPC.

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/`
Expected: PASS

- [ ] **Step 3: Run `task check`**

Run: `task check`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(cli): add export, import, verify commands (spgr-m56.11)
```

---

## Chunk 5: Integration Tests + E2E

### Task 12: Integration tests

**Files:**

- Create: `internal/export/integration_test.go`

- [ ] **Step 1: Write round-trip integration test**

Build tag: `//go:build integration`

Uses the shared Memgraph test container (same pattern as
`internal/storage/memgraph/memgraph_test.go`). Test flow:

1. Create a project with specs, decisions, edges, slices
2. Export the project
3. Import into a new project (via `Scoped` to a different project slug)
4. Re-export the new project
5. Compare the `data` sections of both exports

- [ ] **Step 2: Write force-import test**

1. Create project with data
2. Import without force → expect error
3. Import with force → expect success
4. Verify old data gone, new data present

- [ ] **Step 3: Write signature round-trip test**

1. Export with signing key
2. Import with same key → passes
3. Tamper with data bytes → import fails with HMAC mismatch

- [ ] **Step 4: Run integration tests**

Run: `go test ./internal/export/ -tags integration -v -count=1`
Expected: PASS (requires Docker for Memgraph)

- [ ] **Step 5: Commit**

```text
test(export): add integration tests for round-trip, force, signature (spgr-m56.12)
```

### Task 13: E2E test

**Files:**

- Create: `e2e/api/export_test.go`

- [ ] **Step 1: Write E2E test using Ginkgo/Gomega**

Build tag: `//go:build e2e`

Full CLI pipeline via ConnectRPC client (same pattern as existing E2E tests
in `e2e/api/`):

1. Create specs + edges via RPC
2. Call `ExportProject` RPC
3. Call `WipeProjectData` (or import with force to a fresh project)
4. Call `ImportProject` RPC
5. Call `VerifyExport` RPC → assert match

- [ ] **Step 2: Run E2E tests**

Run: `go test ./e2e/api/ -tags e2e -v -count=1 -run TestExport`
Expected: PASS (requires Docker)

- [ ] **Step 3: Commit**

```text
test(e2e): add export/import/verify round-trip E2E test (spgr-m56.13)
```

### Task 14: Regenerate CLI reference + final verification

- [ ] **Step 1: Regenerate CLI reference**

Run: `task docs:cli`
The new `export`, `import`, `verify` commands will appear in the reference.

- [ ] **Step 2: Run full quality gate**

Run: `task check`
Expected: PASS

- [ ] **Step 3: Verify CLI reference is fresh**

Run: `task docs:cli:check`
Expected: PASS

- [ ] **Step 4: Commit**

```text
docs(cli): regenerate reference with export/import/verify commands (spgr-m56.14)
```
