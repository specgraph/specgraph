# Slice 5: Spec Lifecycle — Revised Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement spec lifecycle operations (amend, supersede, abandon), drift detection, JSON Schema validation, and a spec linter — using domain types throughout the storage layer.

**Architecture:** Lifecycle is a new proto service (LifecycleService) with its own storage interface, Memgraph implementation, ConnectRPC handler, and CLI commands. The drift engine and linter operate on domain types and are invoked from the handler, which converts results to proto. All storage interfaces use domain types (`*storage.Spec`, etc.) — never proto types. Proto↔domain conversion happens exclusively in `internal/server/convert.go`.

**Tech Stack:** Go, ConnectRPC, Memgraph (Cypher), Cobra, buf, testcontainers-go

**Prior plan:** `docs/plans/2026-02-28-slice-5-spec-lifecycle-plan.md` — superseded by this plan due to domain types violation.

**Key correction:** The prior plan passed `*specv1.Spec` and `*specv1.DriftReport` through storage interfaces. This plan uses domain types throughout, matching the pattern established in Slice 4 (ADR: zero proto imports in `internal/storage/`).

---

## Project Structure (new/modified files)

```text
proto/specgraph/v1/
  lifecycle.proto                        # NEW: Lifecycle messages + LifecycleService
  spec.proto                             # MODIFY: add lifecycle, superseded_by, supersedes, history fields
  graph.proto                            # MODIFY: add EDGE_TYPE_SUPERSEDES
internal/
  storage/
    spec_domain.go                       # MODIFY: add done/amended/abandoned stages, lifecycle fields to Spec
    lifecycle.go                         # NEW: LifecycleBackend interface + domain types
    graph.go                             # MODIFY: add EdgeTypeSupersedes
  storage/memgraph/
    memgraph.go                          # MODIFY: extend CreateSpec/GetSpec/etc for new fields
    lifecycle.go                         # NEW: Memgraph LifecycleBackend implementation
    lifecycle_test.go                    # NEW: integration tests
    graph.go                             # MODIFY: add SUPERSEDES edge mapping
  server/
    convert.go                           # MODIFY: add lifecycle domain↔proto converters
    lifecycle_handler.go                 # NEW: ConnectRPC handler
    lifecycle_handler_test.go            # NEW: handler tests with mock
    server.go                            # MODIFY: register LifecycleService
  linter/
    linter.go                            # NEW: Spec linter engine (domain types)
    linter_test.go                       # NEW: linter unit tests
    schema.go                            # NEW: JSON Schema validation (domain types)
    schema_test.go                       # NEW: schema validation tests
  drift/
    drift.go                             # NEW: Drift detection engine (domain types)
    drift_test.go                        # NEW: drift unit tests
cmd/specgraph/
  lifecycle.go                           # NEW: CLI commands
  serve.go                               # MODIFY: register LifecycleService
spec.schema.json                         # NEW: Spec JSON Schema
```

---

## Task 1: Protobuf Schema — Spec Extensions + LifecycleService

**Files:**

- Modify: `proto/specgraph/v1/spec.proto`
- Modify: `proto/specgraph/v1/graph.proto`
- Create: `proto/specgraph/v1/lifecycle.proto`

**Step 1: Extend spec.proto with lifecycle fields**

Add after the existing fields in message Spec:

```protobuf
  string lifecycle = 10;       // task | living (default: task)
  string superseded_by = 11;   // slug of replacement spec, if superseded
  string supersedes = 12;      // slug of spec this replaced
  repeated HistoryEntry history = 13;
```

Add new message after Spec:

```protobuf
message HistoryEntry {
  int32 version = 1;
  string stage = 2;
  string summary = 3;
  string reason = 4;
  google.protobuf.Timestamp date = 5;
}
```

Import `google/protobuf/timestamp.proto` if not already imported.

**Step 2: Add SUPERSEDES edge type to graph.proto**

Add to the `EdgeType` enum:

```protobuf
  EDGE_TYPE_SUPERSEDES = 7;
```

Note: Use 7 (not 6) — check the current max enum value in the file and use the next available.

**Step 3: Create lifecycle.proto**

Create `proto/specgraph/v1/lifecycle.proto` with the content from the prior plan (lines 92-216). This includes:

- Enums: `DriftType`, `DriftSeverity`, `LintSeverity`
- Messages: `DriftItem`, `DriftReport`, `LintViolation`, `LintResult`
- Request/Response messages: `AmendRequest`, `SupersedeRequest/Response`, `AbandonRequest`, `DriftCheckRequest/Response`, `DriftAcknowledgeRequest`, `LintRequest/Response`
- Service: `LifecycleService` with Amend, Supersede, Abandon, CheckDrift, AcknowledgeDrift, Lint

**Step 4: Generate Go code**

Run: `task proto`

Expected: generates `gen/specgraph/v1/lifecycle.pb.go` and `gen/specgraph/v1/specgraphv1connect/lifecycle.connect.go`, plus updates to `spec.pb.go` and `graph.pb.go`.

**Step 5: Verify build**

Run: `go build ./...`

**Step 6: Commit**

```text
feat(lifecycle): protobuf schema for lifecycle, drift, and lint messages
```

---

## Task 2: Domain Types — Extend Spec + Add Lifecycle Types

**Files:**

- Modify: `internal/storage/spec_domain.go`
- Modify: `internal/storage/graph.go`
- Create: `internal/storage/lifecycle.go`

This task establishes the domain types and storage interface BEFORE any Memgraph implementation. The key principle: **zero proto imports in `internal/storage/`**.

**Step 1: Extend SpecStage with new stages**

In `internal/storage/spec_domain.go`, add new stage constants and update `IsValid`:

```go
const (
	SpecStageSpark      SpecStage = "spark"
	SpecStageShape      SpecStage = "shape"
	SpecStageSpecify    SpecStage = "specify"
	SpecStageDecompose  SpecStage = "decompose"
	SpecStageApproved   SpecStage = "approved"
	SpecStageInProgress SpecStage = "in_progress"
	SpecStageReview     SpecStage = "review"
	SpecStageDone       SpecStage = "done"
	SpecStageAmended    SpecStage = "amended"
	SpecStageSuperseded SpecStage = "superseded"
	SpecStageAbandoned  SpecStage = "abandoned"
)
```

Update `IsValid()` to include all new stages.

**Step 2: Extend the Spec domain type**

Add lifecycle fields to the `Spec` struct:

```go
type Spec struct {
	ID           string
	Slug         string
	Intent       string
	Stage        SpecStage
	Priority     SpecPriority
	Complexity   string
	Version      int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Lifecycle    string       // "task" (default) or "living"
	SupersededBy string       // slug of replacement spec
	Supersedes   string       // slug of spec this replaced
	History      []HistoryEntry
}
```

Add `HistoryEntry` domain type:

```go
// HistoryEntry records a lifecycle event on a spec.
type HistoryEntry struct {
	Version int32
	Stage   string
	Summary string
	Reason  string
	Date    time.Time
}
```

**Step 3: Add SUPERSEDES edge type to graph.go**

In `internal/storage/graph.go`, add:

```go
EdgeTypeSupersedes EdgeType = "SUPERSEDES"
```

Update `IsValid()` to include `EdgeTypeSupersedes`.

**Step 4: Create lifecycle domain types and interface**

Create `internal/storage/lifecycle.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// Lifecycle-specific sentinel errors.
var (
	ErrSpecNotDone    = errors.New("spec must be in done stage to amend")
	ErrSpecTerminal   = errors.New("spec is in a terminal state (superseded or abandoned)")
	ErrNewSpecNotFound = errors.New("replacement spec not found")
)

// DriftType identifies the category of drift detected.
type DriftType string

const (
	DriftTypeDependency DriftType = "dependency"
	DriftTypeInterface  DriftType = "interface"
	DriftTypeVerify     DriftType = "verify"
)

// DriftSeverity indicates drift urgency.
type DriftSeverity string

const (
	DriftSeverityHigh   DriftSeverity = "high"
	DriftSeverityMedium DriftSeverity = "medium"
	DriftSeverityLow    DriftSeverity = "low"
	DriftSeverityInfo   DriftSeverity = "info"
)

// DriftItem is a single drift finding.
type DriftItem struct {
	Type            DriftType
	Severity        DriftSeverity
	Description     string
	SpecSlug        string
	UpstreamSlug    string
	ExpectedVersion int32
	ActualVersion   int32
}

// DriftReport aggregates drift items for a spec.
type DriftReport struct {
	SpecSlug        string
	Items           []DriftItem
	Acknowledged    bool
	AcknowledgeNote string
}

// LintSeverity indicates lint violation urgency.
type LintSeverity string

const (
	LintSeverityError   LintSeverity = "error"
	LintSeverityWarning LintSeverity = "warning"
	LintSeverityInfo    LintSeverity = "info"
)

// LintViolation is a single lint finding.
type LintViolation struct {
	Rule     string
	Severity LintSeverity
	Message  string
	Location string
}

// LintResult holds lint results for a single spec.
type LintResult struct {
	SpecSlug   string
	Violations []LintViolation
	Passed     bool
}

// LifecycleBackend defines storage operations for spec lifecycle transitions.
type LifecycleBackend interface {
	// AmendSpec transitions a done spec back into authoring.
	// Returns the updated spec.
	AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*Spec, error)

	// SupersedeSpec marks old spec superseded and links to new.
	// Returns both updated specs.
	SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*Spec, *Spec, error)

	// AbandonSpec transitions a spec to abandoned (terminal).
	AbandonSpec(ctx context.Context, slug, reason string) (*Spec, error)

	// CheckDrift runs drift detection for a single spec or all eligible specs.
	CheckDrift(ctx context.Context, slug, scope string) ([]DriftReport, error)

	// AcknowledgeDrift marks drift as intentional.
	AcknowledgeDrift(ctx context.Context, slug, note string) (*DriftReport, error)
}
```

**Step 5: Verify build**

Run: `go build ./internal/storage/...`

**Step 6: Commit**

```text
feat(lifecycle): domain types and storage interface for lifecycle operations
```

---

## Task 3: Update Memgraph Spec Storage for New Fields

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go`
- Modify: `internal/storage/memgraph/graph.go`

**Step 1: Update CreateSpec query**

Add `lifecycle` (default `"task"`) and `history_json` (default `"[]"`) to the CREATE query params and RETURN clause.

**Step 2: Update recordToSpec**

Extend `recordToSpec` to extract new fields from positions after the existing ones:

- `lifecycle` (string, may be null → default "task")
- `superseded_by` (string, may be null → "")
- `supersedes` (string, may be null → "")
- `history_json` (string, JSON array → unmarshal to `[]storage.HistoryEntry`)

Add a `recordStringOptional` helper that returns "" for nil values.

Important: History is stored as JSON string on the Memgraph node. Unmarshal to `[]storage.HistoryEntry` (domain type), not proto.

**Step 3: Update ALL RETURN clauses**

Every query in `memgraph.go` that returns spec fields must add:

```text
s.lifecycle, s.superseded_by, s.supersedes, s.history_json
```

This includes: `CreateSpec`, `GetSpec`, `ListSpecs`, `UpdateSpec`.

**Step 4: Add SUPERSEDES edge mapping**

In `internal/storage/memgraph/graph.go`, add to the edge type mapping:

```go
storage.EdgeTypeSupersedes: "SUPERSEDES",
```

**Step 5: Verify existing tests still pass**

Run: `go test ./internal/storage/memgraph/ -run "TestCreate|TestList|TestUpdate|TestGet" -v -count=1 -timeout=120s`

Expected: PASS (all existing tests, Docker required)

**Step 6: Commit**

```text
feat(lifecycle): extend memgraph spec storage with lifecycle, history, and supersedes fields
```

---

## Task 4: Memgraph Implementation — Lifecycle Storage

**Files:**

- Create: `internal/storage/memgraph/lifecycle.go`
- Create: `internal/storage/memgraph/lifecycle_test.go`

**Step 1: Write the integration tests**

Tests follow the existing pattern from `memgraph_test.go` using `setupMemgraph(t)` and `newStore()`.

Key test cases:

- `TestAmendSpec_HappyPath`: Create spec → update to done → amend → verify stage="amended", version bumped, history entry has reason
- `TestAmendSpec_NotDone`: Create spec (spark) → amend → expect `storage.ErrSpecNotDone`
- `TestAmendSpec_NotFound`: Amend nonexistent → expect `storage.ErrSpecNotFound`
- `TestSupersedeSpec_HappyPath`: Create old (done) + new → supersede → verify old.Stage="superseded", old.SupersededBy="new", new.Supersedes="old", SUPERSEDES edge exists
- `TestSupersedeSpec_OldNotFound`: Supersede nonexistent → expect `storage.ErrSpecNotFound`
- `TestSupersedeSpec_NewNotFound`: Supersede with nonexistent new → expect `storage.ErrNewSpecNotFound`
- `TestAbandonSpec_HappyPath`: Create spec → abandon → verify stage="abandoned", history entry
- `TestAbandonSpec_Terminal`: Abandon already-abandoned spec → expect `storage.ErrSpecTerminal`
- `TestCheckDrift_DependencyDrift`: Create upstream + downstream with DEPENDS_ON edge, both done, amend upstream → check drift on downstream → expect dependency drift item
- `TestAcknowledgeDrift`: Create done spec → acknowledge drift → verify acknowledged=true

**Critical: All test assertions use domain types, not proto types.**

Example test:

```go
func TestAmendSpec_HappyPath(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-me", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	amended, err := store.AmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
	require.NoError(t, err)
	require.Equal(t, storage.SpecStage("amended"), amended.Stage)
	require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3
	require.NotEmpty(t, amended.History)
	require.Equal(t, "Mobile needs offline refresh", amended.History[0].Reason)
}
```

**Step 2: Run tests to verify failure**

Run: `go test ./internal/storage/memgraph/ -run "TestAmend|TestSupersede|TestAbandon|TestCheckDrift|TestAcknowledgeDrift" -v -count=1 -timeout=120s`

Expected: FAIL — `lifecycle.go` doesn't exist

**Step 3: Implement Memgraph lifecycle backend**

Create `internal/storage/memgraph/lifecycle.go` implementing `storage.LifecycleBackend`:

**AmendSpec:**

1. `GetSpec` to validate exists and stage is "done"
2. Read existing history, append new `storage.HistoryEntry`
3. Marshal history to JSON
4. Cypher SET stage="amended", bump version, set history_json, updated_at
5. Parse result with `recordToSpec` → returns `*storage.Spec`

**SupersedeSpec:**

1. Get both specs, validate old is not terminal, new exists
2. Build history entry for old spec
3. Single Cypher query: SET old.stage='superseded', old.superseded_by=$new, new.supersedes=$old, MERGE SUPERSEDES edge
4. RETURN both specs' fields in single row
5. Parse with `recordToSpecAtOffset` at positions 0 and 13 → returns `*storage.Spec, *storage.Spec`

**AbandonSpec:**

1. Get spec, validate not terminal
2. Build history entry
3. SET stage='abandoned', bump version, set history_json
4. Return `*storage.Spec`

**CheckDrift:**

1. If slug provided, get single spec; otherwise list "done" + "amended" specs
2. For each spec, query upstream deps where upstream.updated_at > spec.updated_at
3. Build `[]storage.DriftItem` with domain types
4. Return `[]storage.DriftReport`

**AcknowledgeDrift:**

1. Verify spec exists
2. SET drift_acknowledged=true, drift_acknowledge_note=$note
3. Return `*storage.DriftReport`

**Step 4: Run tests**

Run: `go test ./internal/storage/memgraph/ -run "TestAmend|TestSupersede|TestAbandon|TestCheckDrift|TestAcknowledgeDrift" -v -count=1 -timeout=120s`

Expected: PASS

**Step 5: Commit**

```text
feat(lifecycle): memgraph lifecycle storage with integration tests
```

---

## Task 5: Spec JSON Schema + Schema Validator

**Files:**

- Create: `spec.schema.json` (project root)
- Create: `internal/linter/schema.go`
- Create: `internal/linter/schema_test.go`

**Step 1: Write the JSON Schema**

Create `spec.schema.json` at project root with the schema from the prior plan (lines 1190-1292). Required fields: slug, intent, stage. Enum validation for stage, priority, complexity, lifecycle. Conditional: superseded stage requires superseded_by.

**Step 2: Write the schema validator**

Create `internal/linter/schema.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package linter validates specs against JSON Schema and graph-consistency rules.
package linter

import (
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"
)

// ValidateSchema validates a spec against the spec schema rules.
// Returns lint violations using domain types.
func ValidateSchema(spec *storage.Spec) []storage.LintViolation {
	var violations []storage.LintViolation

	// Required fields
	if spec.Slug == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "slug is required", Location: "slug",
		})
	}
	if spec.Intent == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "intent is required", Location: "intent",
		})
	}
	if spec.Stage == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.required", Severity: storage.LintSeverityError,
			Message: "stage is required", Location: "stage",
		})
	}

	// Validate stage enum
	if spec.Stage != "" && !spec.Stage.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid stage %q", spec.Stage),
			Location: "stage",
		})
	}

	// Validate priority enum
	if spec.Priority != "" && !spec.Priority.IsValid() {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid priority %q", spec.Priority),
			Location: "priority",
		})
	}

	// Validate lifecycle enum
	validLifecycles := map[string]bool{"task": true, "living": true, "": true}
	if !validLifecycles[spec.Lifecycle] {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.enum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("invalid lifecycle %q", spec.Lifecycle),
			Location: "lifecycle",
		})
	}

	// Conditional: superseded spec must have superseded_by
	if spec.Stage == storage.SpecStageSuperseded && spec.SupersededBy == "" {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.conditional", Severity: storage.LintSeverityError,
			Message:  "superseded spec must have superseded_by field set",
			Location: "superseded_by",
		})
	}

	// Version minimum
	if spec.Version < 1 {
		violations = append(violations, storage.LintViolation{
			Rule: "schema.minimum", Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("version must be >= 1, got %d", spec.Version),
			Location: "version",
		})
	}

	return violations
}
```

**Key difference from prior plan:** Uses `*storage.Spec` and `storage.LintViolation` domain types, not proto types.

**Step 3: Write schema tests**

Create `internal/linter/schema_test.go` with test cases:

- `TestValidateSchema_ValidSpec` — valid spec → no violations
- `TestValidateSchema_MissingRequired` — empty spec → 3 violations (slug, intent, stage)
- `TestValidateSchema_InvalidStage` — invalid stage enum → 1 violation
- `TestValidateSchema_SupersededWithoutBy` — superseded without superseded_by → 1 conditional violation
- `TestValidateSchema_SupersededWithBy` — superseded with superseded_by → no violations
- `TestValidateSchema_InvalidPriority` — invalid priority → 1 violation
- `TestValidateSchema_LivingLifecycle` — lifecycle="living" → no violations

All tests use `*storage.Spec` and `storage.LintViolation`.

**Step 4: Run tests**

Run: `go test ./internal/linter/ -run TestValidateSchema -v -count=1`

Expected: PASS

**Step 5: Commit**

```text
feat(lifecycle): spec JSON Schema with progressive validation
```

---

## Task 6: Linter Engine

**Files:**

- Create: `internal/linter/linter.go`
- Create: `internal/linter/linter_test.go`

**Step 1: Define LintBackend interface**

In `internal/linter/linter.go`, define a minimal interface the linter needs:

```go
// LintBackend is the subset of storage needed by the linter.
type LintBackend interface {
	GetSpec(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error)
}
```

Note: Uses `storage.NodeRef` from `GetDependencies` (the existing `GraphBackend.GetDependencies` returns `[]NodeRef`). The linter needs the dep slugs to check for dangling refs and cycles.

**Step 2: Implement the linter**

```go
// Lint validates one or all specs.
func Lint(ctx context.Context, backend LintBackend, slug string) ([]storage.LintResult, error)
```

Lint rules:

1. **Schema validation** — calls `ValidateSchema(spec)`
2. **Edge consistency** — for each dep slug from `GetDependencies`, verify spec exists via `GetSpec`
3. **Cycle detection** — DFS from spec through dependency graph

Returns `[]storage.LintResult` with domain types.

**Step 3: Write linter tests with mock backend**

Create `internal/linter/linter_test.go` with a `mockLintBackend` that implements `LintBackend` using `*storage.Spec` and `[]storage.NodeRef`:

- `TestLint_SchemaViolation` — spec with empty intent → schema violation
- `TestLint_DanglingDependency` — dep references nonexistent slug → edge.dangling_ref violation
- `TestLint_CycleDetection` — A depends on B, B depends on A → graph.cycle violation
- `TestLint_ValidSpec` — well-formed spec with valid deps → passed=true, no violations
- `TestLint_AllSpecs` — empty slug → lints all specs

**Step 4: Run tests**

Run: `go test ./internal/linter/ -run "TestLint_" -v -count=1`

Expected: PASS

**Step 5: Commit**

```text
feat(lifecycle): spec linter with schema, edge, and cycle checks
```

---

## Task 7: Drift Detection Engine

**Files:**

- Create: `internal/drift/drift.go`
- Create: `internal/drift/drift_test.go`

**Step 1: Define DriftBackend interface**

```go
// DriftBackend is the subset of storage needed by the drift engine.
type DriftBackend interface {
	GetSpec(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error)
}
```

**Step 2: Implement the drift engine**

```go
type Engine struct {
	backend DriftBackend
}

func NewEngine(backend DriftBackend) *Engine

// Check runs drift detection. Returns []storage.DriftReport.
func (e *Engine) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error)
```

Drift checks:

- **Dependency drift** (`scope="" || scope=="deps"`): For each dep, compare upstream.UpdatedAt > spec.UpdatedAt → `storage.DriftItem` with `DriftTypeDependency`
- **Interface drift** (`scope="" || scope=="interfaces"`): Placeholder — returns nil (requires structured interface fields, deferred)
- **Verify drift** (`scope="" || scope=="verify"`): Placeholder — returns nil (requires filesystem access, deferred)

All types use `storage.DriftReport`, `storage.DriftItem`, etc.

**Step 3: Write drift tests with mock backend**

Create `internal/drift/drift_test.go` with `mockDriftBackend` using `*storage.Spec`:

- `TestCheckDependencyDrift` — upstream updated after downstream → drift found
- `TestCheckDependencyDrift_NoDrift` — downstream updated after upstream → no drift
- `TestCheckAllSpecs` — empty slug → checks all done/amended specs, only drifted ones reported
- `TestCheckDrift_ScopeFilter` — scope="interfaces" → no drift; scope="deps" → drift found

**Step 4: Run tests**

Run: `go test ./internal/drift/ -v -count=1`

Expected: PASS

**Step 5: Commit**

```text
feat(lifecycle): drift detection engine with dependency drift checks
```

---

## Task 8: Proto↔Domain Converters + ConnectRPC Handler

**Files:**

- Modify: `internal/server/convert.go`
- Create: `internal/server/lifecycle_handler.go`
- Create: `internal/server/lifecycle_handler_test.go`
- Modify: `internal/server/server.go`

**Step 1: Add lifecycle converters to convert.go**

Add proto↔domain conversion functions:

```go
// specToProto — MODIFY existing function to include new fields:
// spec.Lifecycle, spec.SupersededBy, spec.Supersedes, historyToProto(spec.History)

func historyToProto(entries []storage.HistoryEntry) []*specv1.HistoryEntry { ... }
func historyFromProto(entries []*specv1.HistoryEntry) []storage.HistoryEntry { ... }

// Drift domain↔proto
var driftTypeToProtoMap = map[storage.DriftType]specv1.DriftType{...}
var driftSeverityToProtoMap = map[storage.DriftSeverity]specv1.DriftSeverity{...}

func driftReportToProto(r *storage.DriftReport) *specv1.DriftReport { ... }
func driftReportsToProto(reports []storage.DriftReport) []*specv1.DriftReport { ... }

// Lint domain↔proto
var lintSeverityToProtoMap = map[storage.LintSeverity]specv1.LintSeverity{...}

func lintResultToProto(r *storage.LintResult) *specv1.LintResult { ... }
func lintResultsToProto(results []storage.LintResult) []*specv1.LintResult { ... }

// Edge type maps — add:
storage.EdgeTypeSupersedes: specv1.EdgeType_EDGE_TYPE_SUPERSEDES,
```

**Step 2: Write handler tests**

Create `internal/server/lifecycle_handler_test.go` with a `fakeLifecycleBackend`:

```go
type fakeLifecycleBackend struct {
	amendErr      error
	supersedeErr  error
	abandonErr    error
	checkDriftErr error
	ackDriftErr   error
}
```

Returns domain types (not proto). Handler converts to proto.

Test cases:

- `TestLifecycleHandler_Amend` — happy path → stage="amended"
- `TestLifecycleHandler_Amend_NotDone` → `connect.CodeFailedPrecondition`
- `TestLifecycleHandler_Amend_NotFound` → `connect.CodeNotFound`
- `TestLifecycleHandler_Supersede` — happy path → old.Stage="superseded"
- `TestLifecycleHandler_Abandon` — happy path → stage="abandoned"
- `TestLifecycleHandler_CheckDrift` — returns drift reports
- `TestLifecycleHandler_AcknowledgeDrift` — returns acknowledged report

**Step 3: Implement the handler**

Create `internal/server/lifecycle_handler.go`:

```go
type LifecycleHandler struct {
	store   storage.LifecycleBackend
	linter  *linter.Engine  // optional, for Lint RPC
	drifter *drift.Engine   // optional, for CheckDrift RPC
}
```

Each method:

1. Validates input (slug required, etc.)
2. Calls storage backend (returns domain types)
3. Converts domain → proto using convert.go functions
4. Returns proto response

Error mapping:

- `storage.ErrSpecNotFound` → `connect.CodeNotFound`
- `storage.ErrSpecNotDone` → `connect.CodeFailedPrecondition`
- `storage.ErrSpecTerminal` → `connect.CodeFailedPrecondition`
- `storage.ErrNewSpecNotFound` → `connect.CodeNotFound`

For `Lint` RPC: if linter is wired, run `linter.Lint()` and convert results. Otherwise return `CodeUnimplemented`.

**Step 4: Register in server.go**

Add `RegisterLifecycleService(mux, store)` function and wire in `NewMux` or equivalent registration point.

**Step 5: Run tests**

Run: `go test ./internal/server/ -run TestLifecycle -v -count=1`

Expected: PASS

**Step 6: Commit**

```text
feat(lifecycle): ConnectRPC handler with domain→proto conversion and handler tests
```

---

## Task 9: CLI Commands — Lifecycle Operations

**Files:**

- Create: `cmd/specgraph/lifecycle.go`
- Modify: `cmd/specgraph/serve.go`

**Step 1: Implement CLI commands**

Create `cmd/specgraph/lifecycle.go` with commands following existing patterns from `spark.go`, `decision.go`:

Commands:

- `amend <slug> --reason "..." [--re-entry shape|specify]`
- `supersede <slug> --with <new-slug>`
- `abandon <slug> --reason "..."`
- `drift [slug] [--scope interfaces|verify|deps]`
- `drift acknowledge <slug> --note "..."`
- `lint [slug]`

Each command:

1. Creates `lifecycleClient()` using existing `newClient` pattern
2. Sends ConnectRPC request
3. Formats and prints response

**Step 2: Wire LifecycleService into serve.go**

In `cmd/specgraph/serve.go`, add:

```go
server.RegisterLifecycleService(mux, store)
```

**Step 3: Verify build**

Run: `go build ./cmd/specgraph/`

**Step 4: Verify CLI help**

Run: `./specgraph amend --help && ./specgraph supersede --help && ./specgraph abandon --help && ./specgraph drift --help && ./specgraph lint --help`

**Step 5: Commit**

```text
feat(lifecycle): CLI commands for amend, supersede, abandon, drift, lint
```

---

## Task 10: End-to-End Integration Test

**Files:**

- Create: `e2e/api/lifecycle_test.go` (follows existing E2E pattern in `e2e/`)

**Step 1: Write E2E test**

Follow existing E2E test pattern from the project. Test the full lifecycle flow via CLI or ConnectRPC client:

1. Create spec → move to done → amend → verify amended state
2. Create replacement spec → supersede original → verify superseded state
3. Create spec → abandon → verify abandoned state
4. Set up upstream/downstream deps → trigger drift → verify drift detected
5. Acknowledge drift → verify acknowledged
6. Run lint on valid spec → verify passed

**Step 2: Run E2E test**

Run: `go test ./e2e/api/ -run TestLifecycleE2E -v -count=1 -timeout=60s` (requires Docker for Memgraph)

Expected: PASS

**Step 3: Commit**

```text
test(lifecycle): end-to-end lifecycle flow test
```

---

## Task Dependencies

```text
Task 1 (proto)
    │
    ├──→ Task 2 (domain types + interface)
    │        │
    │        └──→ Task 3 (memgraph spec extensions)
    │                 │
    │                 └──→ Task 4 (memgraph lifecycle impl)
    │                          │
    │                          └──→ Task 8 (handler) ──→ Task 9 (CLI) ──→ Task 10 (E2E)
    │
    ├──→ Task 5 (JSON Schema + validator) ──→ Task 6 (linter) ──→ Task 8
    │
    └──→ Task 7 (drift engine) ──→ Task 8
```

Tasks 5-7 can run in parallel with Tasks 3-4 after Tasks 1-2.

## Verification Gate

After all tasks:

- [ ] `go test ./... -count=1 -timeout=120s` — all pass
- [ ] `go test ./internal/storage/memgraph/ -run "TestAmend|TestSupersede|TestAbandon|TestCheckDrift" -v -count=1 -timeout=120s` — lifecycle integration tests pass
- [ ] `golangci-lint run ./...` — no issues
- [ ] `buf lint` — no proto issues
- [ ] `go build -o specgraph ./cmd/specgraph` — clean build
- [ ] CLI commands work: `specgraph amend|supersede|abandon|lint|drift --help`
- [ ] Full lifecycle flow: create → done → amend → supersede → abandon
- [ ] Linter validates specs against JSON Schema
- [ ] Drift engine detects spec-vs-dependency version mismatches
