# Slice 5: Spec Lifecycle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement spec lifecycle operations (amend, supersede, abandon), drift detection, JSON Schema validation, and a spec linter.

**Architecture:** Lifecycle is a new proto service (LifecycleService) with its own storage interface, Memgraph implementation, ConnectRPC handler, and CLI commands. The drift engine compares spec metadata against graph state (dependency versions, interface fields). The linter validates specs against a JSON Schema and runs graph-consistency checks. Drift and lint results are stored as properties on spec nodes and returned via RPC.

**Tech Stack:** Go, ConnectRPC, Memgraph, Cobra, buf, testcontainers-go

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 5 section)

---

## Project Structure (new files)

```text
proto/specgraph/v1/
  lifecycle.proto                       # Lifecycle messages + LifecycleService
gen/specgraph/v1/                       # Generated (buf generate)
  lifecycle.pb.go
  specgraphv1connect/
    lifecycle.connect.go
internal/
  storage/
    lifecycle.go                        # LifecycleBackend interface
  storage/memgraph/
    lifecycle.go                        # Memgraph implementation
    lifecycle_test.go                   # Integration tests
  server/
    lifecycle_handler.go                # ConnectRPC handler
    lifecycle_handler_test.go           # Handler tests with mock
  linter/
    linter.go                           # Spec linter engine
    linter_test.go                      # Linter unit tests
  linter/
    schema.go                           # JSON Schema loader + validator
    schema_test.go                      # Schema validation tests
  drift/
    drift.go                            # Drift detection engine
    drift_test.go                       # Drift unit tests
cmd/specgraph/
  lifecycle.go                          # CLI: amend, supersede, abandon, drift, lint
spec.schema.json                        # Spec JSON Schema (project root)
```

---

## Task 1: Protobuf Schema — Spec Extensions + LifecycleService

**Files:**

- Modify: `proto/specgraph/v1/spec.proto`
- Modify: `proto/specgraph/v1/graph.proto`
- Create: `proto/specgraph/v1/lifecycle.proto`

**Step 1: Extend spec.proto with lifecycle fields**

Add `lifecycle`, `superseded_by`, `supersedes`, and `history` fields to the existing `Spec` message in `proto/specgraph/v1/spec.proto`:

```protobuf
// Add after the existing fields in message Spec:

  string lifecycle = 10;       // task | living (default: task)
  string superseded_by = 11;   // slug of replacement spec, if superseded
  string supersedes = 12;      // slug of spec this replaced
  repeated HistoryEntry history = 13;

// Add new message after Spec:

message HistoryEntry {
  int32 version = 1;
  string stage = 2;
  string summary = 3;
  string reason = 4;            // for amendments — why the change
  google.protobuf.Timestamp date = 5;
}
```

**Step 2: Add SUPERSEDES edge type to graph.proto**

Add to the `EdgeType` enum in `proto/specgraph/v1/graph.proto`:

```protobuf
  EDGE_TYPE_SUPERSEDES = 6;    // A supersedes B (A is the replacement for B)
```

**Step 3: Create lifecycle.proto**

`proto/specgraph/v1/lifecycle.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "specgraph/v1/spec.proto";

// --- Enums ---

enum DriftType {
  DRIFT_TYPE_UNSPECIFIED = 0;
  DRIFT_TYPE_INTERFACE = 1;       // spec interface field vs stored data
  DRIFT_TYPE_VERIFY = 2;          // test_path tests no longer exist/pass
  DRIFT_TYPE_DEPENDENCY = 3;      // upstream spec version changed
}

enum DriftSeverity {
  DRIFT_SEVERITY_UNSPECIFIED = 0;
  DRIFT_SEVERITY_HIGH = 1;
  DRIFT_SEVERITY_MEDIUM = 2;
  DRIFT_SEVERITY_LOW = 3;
  DRIFT_SEVERITY_INFO = 4;
}

enum LintSeverity {
  LINT_SEVERITY_UNSPECIFIED = 0;
  LINT_SEVERITY_ERROR = 1;
  LINT_SEVERITY_WARNING = 2;
  LINT_SEVERITY_INFO = 3;
}

// --- Drift Messages ---

message DriftItem {
  DriftType type = 1;
  DriftSeverity severity = 2;
  string description = 3;
  string spec_slug = 4;
  string upstream_slug = 5;       // for dependency drift: which upstream changed
  int32 expected_version = 6;     // version the dependent was last updated against
  int32 actual_version = 7;       // current upstream version
}

message DriftReport {
  string spec_slug = 1;
  repeated DriftItem items = 2;
  bool acknowledged = 3;
  string acknowledge_note = 4;
}

// --- Lint Messages ---

message LintViolation {
  string rule = 1;
  LintSeverity severity = 2;
  string message = 3;
  string location = 4;           // field path or edge reference
}

message LintResult {
  string spec_slug = 1;
  repeated LintViolation violations = 2;
  bool passed = 3;               // true if no error-severity violations
}

// --- Requests/Responses ---

message AmendRequest {
  string slug = 1;
  string reason = 2;
  string re_entry_stage = 3;     // shape | specify (default: shape)
}

message SupersedeRequest {
  string slug = 1;
  string new_slug = 2;
}

message SupersedeResponse {
  Spec old_spec = 1;
  Spec new_spec = 2;
}

message AbandonRequest {
  string slug = 1;
  string reason = 2;
}

message DriftCheckRequest {
  string slug = 1;               // optional — empty means check all
  string scope = 2;              // optional: interfaces | verify | deps
}

message DriftCheckResponse {
  repeated DriftReport reports = 1;
}

message DriftAcknowledgeRequest {
  string slug = 1;
  string note = 2;
}

message LintRequest {
  string slug = 1;               // optional — empty means lint all
}

message LintResponse {
  repeated LintResult results = 1;
}

// --- Service ---

service LifecycleService {
  rpc Amend(AmendRequest) returns (Spec);
  rpc Supersede(SupersedeRequest) returns (SupersedeResponse);
  rpc Abandon(AbandonRequest) returns (Spec);
  rpc CheckDrift(DriftCheckRequest) returns (DriftCheckResponse);
  rpc AcknowledgeDrift(DriftAcknowledgeRequest) returns (DriftReport);
  rpc Lint(LintRequest) returns (LintResponse);
}
```

**Step 4: Generate Go code**

```bash
buf generate
```

Expected: generates `gen/specgraph/v1/lifecycle.pb.go` and `gen/specgraph/v1/specgraphv1connect/lifecycle.connect.go`, plus updates to `spec.pb.go` and `graph.pb.go`.

**Step 5: Verify generated code compiles**

```bash
go mod tidy
go build ./gen/...
```

**Step 6: Commit**

```bash
git add proto/specgraph/v1/lifecycle.proto proto/specgraph/v1/spec.proto proto/specgraph/v1/graph.proto gen/ go.mod go.sum
git commit -m "feat(lifecycle): protobuf schema for lifecycle, drift, and lint messages"
```

---

## Task 2: Update Memgraph Spec Storage for New Fields

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go`

The existing `CreateSpec`, `GetSpec`, `ListSpecs`, and `UpdateSpec` methods must handle the new Spec fields: `lifecycle`, `superseded_by`, `supersedes`, `history`. The `recordToSpec` function needs updating.

**Step 1: Update CreateSpec to include lifecycle field**

In `internal/storage/memgraph/memgraph.go`, update the `CreateSpec` function to include `lifecycle` (default `"task"`) and `history_json` (empty array `"[]"`) in the CREATE query and params. Update the RETURN clause to include the new fields.

**Step 2: Update recordToSpec to handle new fields**

Update `recordToSpec` to extract `lifecycle` (position 9), `superseded_by` (position 10), `supersedes` (position 11), and `history_json` (position 12) from records. `superseded_by` and `supersedes` may be empty strings. `history_json` unmarshals to `[]*specv1.HistoryEntry`.

Note: Memgraph stores `null` as empty string for optional string fields. Use a helper that returns `""` for null values without erroring.

**Step 3: Update all queries that RETURN spec fields**

All `RETURN` clauses in `CreateSpec`, `GetSpec`, `ListSpecs`, `UpdateSpec` must be updated to include:

```text
s.lifecycle, s.superseded_by, s.supersedes, s.history_json
```

**Step 4: Update UpdateSpec to allow setting new fields**

UpdateSpec does not need to set lifecycle/superseded_by/supersedes directly — those transitions happen in the lifecycle storage layer. But the returned spec must include the new fields.

**Step 5: Verify existing tests still pass**

```bash
go test ./internal/storage/memgraph/ -run TestCreateAndGetSpec -v -count=1 -timeout=120s
go test ./internal/storage/memgraph/ -run TestListSpecs -v -count=1 -timeout=120s
go test ./internal/storage/memgraph/ -run TestUpdateSpec -v -count=1 -timeout=120s
```

**Step 6: Update handler tests and graph edge mapping**

Update `edgeTypeToRel` in `internal/storage/memgraph/graph.go` to include `EDGE_TYPE_SUPERSEDES`:

```go
specv1.EdgeType_EDGE_TYPE_SUPERSEDES: "SUPERSEDES",
```

**Step 7: Verify full build**

```bash
go build ./...
```

**Step 8: Commit**

```bash
git add internal/storage/memgraph/memgraph.go internal/storage/memgraph/graph.go
git commit -m "feat(lifecycle): extend spec storage with lifecycle, history, and supersedes fields"
```

---

## Task 3: Storage Interface — LifecycleBackend

**Files:**

- Create: `internal/storage/lifecycle.go`

**Step 1: Define the interface**

`internal/storage/lifecycle.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrSpecNotDone is returned when amend is called on a spec that is not in done stage.
var ErrSpecNotDone = errors.New("spec must be in done stage to amend")

// ErrSpecTerminal is returned when a lifecycle operation targets a terminal spec.
var ErrSpecTerminal = errors.New("spec is in a terminal state (superseded or abandoned)")

// ErrNewSpecNotFound is returned when the replacement spec for supersede does not exist.
var ErrNewSpecNotFound = errors.New("replacement spec not found")

// ErrDriftNotFound is returned when no drift report exists for the given spec.
var ErrDriftNotFound = errors.New("drift report not found")

// LifecycleBackend defines storage operations for spec lifecycle transitions.
type LifecycleBackend interface {
	// AmendSpec transitions a done spec to amended, bumps version, records reason
	// in history, and sets re-entry stage (shape or specify).
	AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*specv1.Spec, error)

	// SupersedeSpec marks the old spec as superseded, creates a SUPERSEDES edge,
	// and returns both the old and new specs.
	SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*specv1.Spec, *specv1.Spec, error)

	// AbandonSpec transitions a spec to abandoned with a reason. Terminal.
	AbandonSpec(ctx context.Context, slug, reason string) (*specv1.Spec, error)

	// CheckDrift runs drift detection for a single spec or all done/living specs.
	// If slug is empty, checks all eligible specs.
	CheckDrift(ctx context.Context, slug, scope string) ([]*specv1.DriftReport, error)

	// AcknowledgeDrift marks drift as intentional with a note.
	AcknowledgeDrift(ctx context.Context, slug, note string) (*specv1.DriftReport, error)

	// GetDependentsForDriftNotification returns slugs of specs that depend on the given spec.
	GetDependentsForDriftNotification(ctx context.Context, slug string) ([]string, error)

	// SetDriftFlag sets a drift notification flag on a dependent spec.
	SetDriftFlag(ctx context.Context, slug, flag, upstreamSlug string) error
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/lifecycle.go
git commit -m "feat(lifecycle): storage backend interface for lifecycle operations"
```

---

## Task 4: Memgraph Implementation — Lifecycle Storage

**Files:**

- Create: `internal/storage/memgraph/lifecycle.go`
- Create: `internal/storage/memgraph/lifecycle_test.go`

**Step 1: Write the integration tests**

`internal/storage/memgraph/lifecycle_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestAmendSpec_HappyPath(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create spec and transition to done
	_, err = store.CreateSpec(ctx, "amend-me", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "amend-me", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Amend it
	amended, err := store.AmendSpec(ctx, "amend-me", "Mobile needs offline refresh", "shape")
	require.NoError(t, err)
	require.Equal(t, "amended", amended.Stage)
	require.Equal(t, int32(3), amended.Version) // create=1, update=2, amend=3
	require.NotEmpty(t, amended.History)
	require.Equal(t, "Mobile needs offline refresh", amended.History[0].Reason)
}

func TestAmendSpec_NotDone(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "not-done", "Test spec", "p1", "medium")
	require.NoError(t, err)

	_, err = store.AmendSpec(ctx, "not-done", "Reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotDone)
}

func TestAmendSpec_NotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.AmendSpec(ctx, "nonexistent", "Reason", "shape")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_HappyPath(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create old spec (done) and new spec
	_, err = store.CreateSpec(ctx, "old-spec", "Old approach", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "old-spec", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "new-spec", "New approach", "p1", "medium")
	require.NoError(t, err)

	// Supersede
	oldSpec, newSpec, err := store.SupersedeSpec(ctx, "old-spec", "new-spec")
	require.NoError(t, err)
	require.Equal(t, "superseded", oldSpec.Stage)
	require.Equal(t, "new-spec", oldSpec.SupersededBy)
	require.Equal(t, "old-spec", newSpec.Supersedes)
}

func TestSupersedeSpec_OldNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "new-spec", "New approach", "p1", "medium")
	require.NoError(t, err)

	_, _, err = store.SupersedeSpec(ctx, "nonexistent", "new-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSupersedeSpec_NewNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "old-spec", "Old approach", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "old-spec", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	_, _, err = store.SupersedeSpec(ctx, "old-spec", "nonexistent")
	require.ErrorIs(t, err, storage.ErrNewSpecNotFound)
}

func TestAbandonSpec_HappyPath(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "abandon-me", "Test spec", "p2", "low")
	require.NoError(t, err)

	abandoned, err := store.AbandonSpec(ctx, "abandon-me", "Requirements changed")
	require.NoError(t, err)
	require.Equal(t, "abandoned", abandoned.Stage)
	require.NotEmpty(t, abandoned.History)
	require.Equal(t, "Requirements changed", abandoned.History[0].Reason)
}

func TestAbandonSpec_Terminal(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "terminal-spec", "Test spec", "p2", "low")
	require.NoError(t, err)

	// Abandon first time
	_, err = store.AbandonSpec(ctx, "terminal-spec", "First reason")
	require.NoError(t, err)

	// Abandon again should fail — already terminal
	_, err = store.AbandonSpec(ctx, "terminal-spec", "Second reason")
	require.ErrorIs(t, err, storage.ErrSpecTerminal)
}

func TestCheckDrift_DependencyDrift(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create upstream and downstream specs
	_, err = store.CreateSpec(ctx, "upstream", "Upstream API", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream", "Downstream consumer", "p1", "medium")
	require.NoError(t, err)

	// Add dependency edge: downstream DEPENDS_ON upstream
	_, err = store.AddEdge(ctx, "downstream", "upstream", 1) // EDGE_TYPE_DEPENDS_ON
	require.NoError(t, err)

	// Mark both as done
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "upstream", nil, &doneStage, nil, nil)
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "downstream", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Now amend upstream — this should cause dependency drift for downstream
	_, err = store.AmendSpec(ctx, "upstream", "Interface changed", "shape")
	require.NoError(t, err)

	// Check drift for downstream
	reports, err := store.CheckDrift(ctx, "downstream", "")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	// Should find dependency drift because upstream version changed
	hasDependencyDrift := false
	for _, item := range reports[0].Items {
		if item.Type == 3 { // DRIFT_TYPE_DEPENDENCY
			hasDependencyDrift = true
		}
	}
	require.True(t, hasDependencyDrift, "expected dependency drift item")
}

func TestAcknowledgeDrift(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "ack-spec", "Test spec", "p1", "medium")
	require.NoError(t, err)
	doneStage := "done"
	_, err = store.UpdateSpec(ctx, "ack-spec", nil, &doneStage, nil, nil)
	require.NoError(t, err)

	// Acknowledge drift
	report, err := store.AcknowledgeDrift(ctx, "ack-spec", "Intentional divergence")
	require.NoError(t, err)
	require.True(t, report.Acknowledged)
	require.Equal(t, "Intentional divergence", report.AcknowledgeNote)
}
```

**Step 2: Run the tests to verify they fail**

```bash
go test ./internal/storage/memgraph/ -run "TestAmend|TestSupersede|TestAbandon|TestCheckDrift|TestAcknowledgeDrift" -v -count=1 -timeout=120s
```

Expected: FAIL — `lifecycle.go` doesn't exist yet

**Step 3: Implement the Memgraph lifecycle backend**

`internal/storage/memgraph/lifecycle.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AmendSpec transitions a done spec to amended, bumps version, records reason in history.
func (s *Store) AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*specv1.Spec, error) {
	if reEntryStage == "" {
		reEntryStage = "shape"
	}

	// First, get the current spec to validate state and read history
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}

	if spec.Stage != "done" {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotDone)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Build new history entry
	entry := &specv1.HistoryEntry{
		Version: spec.Version + 1,
		Stage:   "amended",
		Summary: fmt.Sprintf("Amended: %s", reason),
		Reason:  reason,
		Date:    timestamppb.New(now),
	}
	newHistory := append(spec.History, entry)
	historyJSON, err := json.Marshal(newHistory)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal history: %w", err)
	}

	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage,
		    s.version = s.version + 1,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	params := map[string]any{
		"slug":         slug,
		"stage":        "amended",
		"updated_at":   nowStr,
		"history_json": string(historyJSON),
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(result.Records[0])
}

// SupersedeSpec marks old spec as superseded and creates SUPERSEDES edge.
func (s *Store) SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*specv1.Spec, *specv1.Spec, error) {
	// Validate old spec exists and is not already terminal
	oldSpec, err := s.GetSpec(ctx, oldSlug)
	if err != nil {
		return nil, nil, err
	}
	if oldSpec.Stage == "superseded" || oldSpec.Stage == "abandoned" {
		return nil, nil, fmt.Errorf("memgraph: supersede spec %q: %w", oldSlug, storage.ErrSpecTerminal)
	}

	// Validate new spec exists
	_, err = s.GetSpec(ctx, newSlug)
	if err != nil {
		if isSpecNotFound(err) {
			return nil, nil, fmt.Errorf("memgraph: supersede spec: new spec %q: %w", newSlug, storage.ErrNewSpecNotFound)
		}
		return nil, nil, err
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Build history entry for old spec
	entry := &specv1.HistoryEntry{
		Version: oldSpec.Version + 1,
		Stage:   "superseded",
		Summary: fmt.Sprintf("Superseded by %s", newSlug),
		Date:    timestamppb.New(now),
	}
	newHistory := append(oldSpec.History, entry)
	historyJSON, err := json.Marshal(newHistory)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: marshal history: %w", err)
	}

	// Update old spec, update new spec, create SUPERSEDES edge — all in one query
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		SET old.stage = 'superseded',
		    old.superseded_by = $new_slug,
		    old.version = old.version + 1,
		    old.updated_at = $updated_at,
		    old.history_json = $history_json
		SET new.supersedes = $old_slug,
		    new.updated_at = $updated_at
		MERGE (new)-[:SUPERSEDES]->(old)
		RETURN old.id, old.slug, old.intent, old.stage, old.priority, old.complexity,
		       old.version, old.created_at, old.updated_at,
		       old.lifecycle, old.superseded_by, old.supersedes, old.history_json,
		       new.id, new.slug, new.intent, new.stage, new.priority, new.complexity,
		       new.version, new.created_at, new.updated_at,
		       new.lifecycle, new.superseded_by, new.supersedes, new.history_json
	`
	params := map[string]any{
		"old_slug":     oldSlug,
		"new_slug":     newSlug,
		"updated_at":   nowStr,
		"history_json": string(historyJSON),
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, nil, fmt.Errorf("memgraph: supersede spec: no result returned")
	}

	rec := result.Records[0]

	// Parse old spec from positions 0-12
	updatedOld, err := recordToSpecAtOffset(rec, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: parse old spec: %w", err)
	}

	// Parse new spec from positions 13-25
	updatedNew, err := recordToSpecAtOffset(rec, 13)
	if err != nil {
		return nil, nil, fmt.Errorf("memgraph: parse new spec: %w", err)
	}

	return updatedOld, updatedNew, nil
}

// AbandonSpec transitions a spec to abandoned. Terminal.
func (s *Store) AbandonSpec(ctx context.Context, slug, reason string) (*specv1.Spec, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if spec.Stage == "superseded" || spec.Stage == "abandoned" {
		return nil, fmt.Errorf("memgraph: abandon spec %q: %w", slug, storage.ErrSpecTerminal)
	}

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	entry := &specv1.HistoryEntry{
		Version: spec.Version + 1,
		Stage:   "abandoned",
		Summary: fmt.Sprintf("Abandoned: %s", reason),
		Reason:  reason,
		Date:    timestamppb.New(now),
	}
	newHistory := append(spec.History, entry)
	historyJSON, err := json.Marshal(newHistory)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal history: %w", err)
	}

	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = 'abandoned',
		    s.version = s.version + 1,
		    s.updated_at = $updated_at,
		    s.history_json = $history_json
		RETURN s.id, s.slug, s.intent, s.stage, s.priority, s.complexity,
		       s.version, s.created_at, s.updated_at,
		       s.lifecycle, s.superseded_by, s.supersedes, s.history_json
	`
	params := map[string]any{
		"slug":         slug,
		"updated_at":   nowStr,
		"history_json": string(historyJSON),
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: abandon spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: abandon spec %q: %w", slug, storage.ErrSpecNotFound)
	}

	return recordToSpec(result.Records[0])
}

// CheckDrift runs drift detection. If slug is empty, checks all done/living specs.
func (s *Store) CheckDrift(ctx context.Context, slug, scope string) ([]*specv1.DriftReport, error) {
	var specs []*specv1.Spec

	if slug != "" {
		spec, err := s.GetSpec(ctx, slug)
		if err != nil {
			return nil, err
		}
		specs = []*specv1.Spec{spec}
	} else {
		// Get all done or living specs
		doneSpecs, err := s.ListSpecs(ctx, "done", "", 0)
		if err != nil {
			return nil, fmt.Errorf("memgraph: check drift: list done: %w", err)
		}
		specs = append(specs, doneSpecs...)

		// Also get amended specs (they were done, now being revised)
		amendedSpecs, err := s.ListSpecs(ctx, "amended", "", 0)
		if err != nil {
			return nil, fmt.Errorf("memgraph: check drift: list amended: %w", err)
		}
		specs = append(specs, amendedSpecs...)
	}

	var reports []*specv1.DriftReport
	for _, spec := range specs {
		report := &specv1.DriftReport{
			SpecSlug: spec.Slug,
		}

		// Dependency drift: check if upstream specs have changed since last update
		if scope == "" || scope == "deps" {
			depItems, err := s.checkDependencyDrift(ctx, spec)
			if err != nil {
				return nil, fmt.Errorf("memgraph: check drift for %q: %w", spec.Slug, err)
			}
			report.Items = append(report.Items, depItems...)
		}

		// Only include reports that have drift items
		if len(report.Items) > 0 {
			reports = append(reports, report)
		}
	}

	return reports, nil
}

// checkDependencyDrift checks if any upstream dependencies have been modified
// since this spec was last updated.
func (s *Store) checkDependencyDrift(ctx context.Context, spec *specv1.Spec) ([]*specv1.DriftItem, error) {
	// Query for all specs this spec depends on, where the upstream version
	// is higher than when this spec was last updated
	query := `
		MATCH (s:Spec {slug: $slug})-[:DEPENDS_ON]->(upstream:Spec)
		WHERE upstream.updated_at > s.updated_at
		RETURN upstream.slug, upstream.version
	`
	params := map[string]any{"slug": spec.Slug}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: dependency drift: %w", err)
	}

	var items []*specv1.DriftItem
	for _, rec := range result.Records {
		upstreamSlug, err := recordString(rec, 0, "upstream_slug")
		if err != nil {
			return nil, err
		}
		upstreamVersion, err := recordInt64(rec, 1, "upstream_version")
		if err != nil {
			return nil, err
		}

		items = append(items, &specv1.DriftItem{
			Type:           specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
			Severity:       specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
			Description:    fmt.Sprintf("Upstream spec %q changed (now v%d) since %q was last updated", upstreamSlug, upstreamVersion, spec.Slug),
			SpecSlug:       spec.Slug,
			UpstreamSlug:   upstreamSlug,
			ActualVersion:  int32(upstreamVersion),
		})
	}

	return items, nil
}

// AcknowledgeDrift marks drift as intentional with a note.
func (s *Store) AcknowledgeDrift(ctx context.Context, slug, note string) (*specv1.DriftReport, error) {
	// Verify spec exists
	_, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.drift_acknowledged = true,
		    s.drift_acknowledge_note = $note,
		    s.drift_acknowledge_at = $now
		RETURN s.slug
	`
	params := map[string]any{
		"slug": slug,
		"note": note,
		"now":  now,
	}

	_, err = neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: acknowledge drift: %w", err)
	}

	return &specv1.DriftReport{
		SpecSlug:        slug,
		Acknowledged:    true,
		AcknowledgeNote: note,
	}, nil
}

// GetDependentsForDriftNotification returns slugs of specs that depend on the given spec.
func (s *Store) GetDependentsForDriftNotification(ctx context.Context, slug string) ([]string, error) {
	query := `
		MATCH (dependent:Spec)-[:DEPENDS_ON]->(s:Spec {slug: $slug})
		RETURN dependent.slug
	`
	params := map[string]any{"slug": slug}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get dependents: %w", err)
	}

	var slugs []string
	for _, rec := range result.Records {
		depSlug, err := recordString(rec, 0, "slug")
		if err != nil {
			return nil, err
		}
		slugs = append(slugs, depSlug)
	}
	return slugs, nil
}

// SetDriftFlag sets a drift notification flag on a dependent spec.
func (s *Store) SetDriftFlag(ctx context.Context, slug, flag, upstreamSlug string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.drift_flag = $flag,
		    s.drift_upstream = $upstream,
		    s.drift_flagged_at = $now
		RETURN s.slug
	`
	params := map[string]any{
		"slug":     slug,
		"flag":     flag,
		"upstream": upstreamSlug,
		"now":      now,
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: set drift flag: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: set drift flag: spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

// recordToSpecAtOffset parses a spec from a record starting at a given column offset.
// Used when a query returns multiple specs in a single row.
func recordToSpecAtOffset(rec *neo4j.Record, offset int) (*specv1.Spec, error) {
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

	spec := &specv1.Spec{
		Id:         id,
		Slug:       slug,
		Intent:     intent,
		Stage:      stage,
		Priority:   priority,
		Complexity: complexity,
		Version:    int32(version),
		CreatedAt:  timestamppb.New(createdAt),
		UpdatedAt:  timestamppb.New(updatedAt),
	}

	// Parse optional string fields (may be null/empty in Memgraph)
	if lifecycle, err := recordStringOptional(rec, offset+9); err == nil {
		spec.Lifecycle = lifecycle
	}
	if supersededBy, err := recordStringOptional(rec, offset+10); err == nil {
		spec.SupersededBy = supersededBy
	}
	if supersedes, err := recordStringOptional(rec, offset+11); err == nil {
		spec.Supersedes = supersedes
	}
	if historyJSON, err := recordStringOptional(rec, offset+12); err == nil && historyJSON != "" {
		if err := json.Unmarshal([]byte(historyJSON), &spec.History); err != nil {
			return nil, fmt.Errorf("unmarshal history: %w", err)
		}
	}

	return spec, nil
}

// recordStringOptional extracts a string value from a record, returning "" for nil/null.
func recordStringOptional(rec *neo4j.Record, pos int) (string, error) {
	if rec.Values[pos] == nil {
		return "", nil
	}
	v, ok := rec.Values[pos].(string)
	if !ok {
		return "", fmt.Errorf("memgraph: field at position %d: expected string, got %T", pos, rec.Values[pos])
	}
	return v, nil
}

// isSpecNotFound checks if an error wraps storage.ErrSpecNotFound.
func isSpecNotFound(err error) bool {
	return err != nil && (err == storage.ErrSpecNotFound || fmt.Sprintf("%v", err) != "" && containsErr(err, storage.ErrSpecNotFound))
}

func containsErr(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		unwrapped, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = unwrapped.Unwrap()
	}
	return false
}
```

Note: The `isSpecNotFound` helper should use `errors.Is` from the standard library. Replace the custom implementation with:

```go
import "errors"

func isSpecNotFound(err error) bool {
	return errors.Is(err, storage.ErrSpecNotFound)
}
```

**Step 4: Run the tests**

```bash
go mod tidy
go test ./internal/storage/memgraph/ -run "TestAmend|TestSupersede|TestAbandon|TestCheckDrift|TestAcknowledgeDrift" -v -count=1 -timeout=120s
```

Expected: PASS (all lifecycle tests). Requires Docker running.

**Step 5: Commit**

```bash
git add internal/storage/memgraph/lifecycle.go internal/storage/memgraph/lifecycle_test.go
git commit -m "feat(lifecycle): memgraph lifecycle storage with integration tests"
```

---

## Task 5: Spec JSON Schema

**Files:**

- Create: `spec.schema.json`
- Create: `internal/linter/schema.go`
- Create: `internal/linter/schema_test.go`

**Step 1: Write the JSON Schema**

`spec.schema.json` (project root — will be embedded at `.specgraph/spec.schema.json` by init):

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://specgraph.dev/spec.schema.json",
  "title": "SpecGraph Spec",
  "description": "JSON Schema for SpecGraph spec objects. Supports progressive validation.",
  "type": "object",
  "required": ["slug", "intent", "stage"],
  "properties": {
    "id": {
      "type": "string",
      "pattern": "^spec-[a-f0-9]{7}$"
    },
    "slug": {
      "type": "string",
      "pattern": "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
      "minLength": 2,
      "maxLength": 128
    },
    "intent": {
      "type": "string",
      "minLength": 1,
      "maxLength": 1000
    },
    "stage": {
      "type": "string",
      "enum": ["spark", "shape", "specify", "decompose", "approved", "in_progress", "review", "done", "amended", "superseded", "abandoned"]
    },
    "priority": {
      "type": "string",
      "enum": ["p0", "p1", "p2", "p3"]
    },
    "complexity": {
      "type": "string",
      "enum": ["low", "medium", "high"]
    },
    "lifecycle": {
      "type": "string",
      "enum": ["task", "living"],
      "default": "task"
    },
    "version": {
      "type": "integer",
      "minimum": 1
    },
    "superseded_by": {
      "type": "string"
    },
    "supersedes": {
      "type": "string"
    },
    "depends_on": {
      "type": "array",
      "items": { "type": "string" }
    },
    "blocks": {
      "type": "array",
      "items": { "type": "string" }
    },
    "interface": {
      "type": "string",
      "description": "The spec's interface contract — API endpoints, data models, etc."
    },
    "verify": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["criterion"],
        "properties": {
          "criterion": { "type": "string" },
          "test_path": { "type": "string" }
        }
      }
    },
    "invariants": {
      "type": "array",
      "items": { "type": "string" }
    },
    "history": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["version", "stage", "summary"],
        "properties": {
          "version": { "type": "integer" },
          "stage": { "type": "string" },
          "summary": { "type": "string" },
          "reason": { "type": "string" },
          "date": { "type": "string", "format": "date-time" }
        }
      }
    }
  },
  "if": {
    "properties": {
      "stage": { "const": "superseded" }
    },
    "required": ["stage"]
  },
  "then": {
    "required": ["superseded_by"]
  }
}
```

**Step 2: Write the schema loader and validator**

`internal/linter/schema.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package linter validates specs against JSON Schema and graph-consistency rules.
package linter

import (
	_ "embed"
	"encoding/json"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

//go:embed ../../spec.schema.json
var specSchemaJSON []byte

// ValidateSchema validates a spec's JSON representation against the embedded schema.
// Returns lint violations for schema errors.
func ValidateSchema(spec *specv1.Spec) []*specv1.LintViolation {
	var violations []*specv1.LintViolation

	// Required fields
	if spec.Slug == "" {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.required",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  "slug is required",
			Location: "slug",
		})
	}
	if spec.Intent == "" {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.required",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  "intent is required",
			Location: "intent",
		})
	}
	if spec.Stage == "" {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.required",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  "stage is required",
			Location: "stage",
		})
	}

	// Validate stage enum
	validStages := map[string]bool{
		"spark": true, "shape": true, "specify": true, "decompose": true,
		"approved": true, "in_progress": true, "review": true, "done": true,
		"amended": true, "superseded": true, "abandoned": true,
	}
	if spec.Stage != "" && !validStages[spec.Stage] {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.enum",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("invalid stage %q", spec.Stage),
			Location: "stage",
		})
	}

	// Validate priority enum
	validPriorities := map[string]bool{"p0": true, "p1": true, "p2": true, "p3": true, "": true}
	if !validPriorities[spec.Priority] {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.enum",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("invalid priority %q", spec.Priority),
			Location: "priority",
		})
	}

	// Validate complexity enum
	validComplexities := map[string]bool{"low": true, "medium": true, "high": true, "": true}
	if !validComplexities[spec.Complexity] {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.enum",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("invalid complexity %q", spec.Complexity),
			Location: "complexity",
		})
	}

	// Validate lifecycle enum
	validLifecycles := map[string]bool{"task": true, "living": true, "": true}
	if !validLifecycles[spec.Lifecycle] {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.enum",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("invalid lifecycle %q", spec.Lifecycle),
			Location: "lifecycle",
		})
	}

	// Conditional: superseded specs must have superseded_by
	if spec.Stage == "superseded" && spec.SupersededBy == "" {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.conditional",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  "superseded spec must have superseded_by field set",
			Location: "superseded_by",
		})
	}

	// Version must be >= 1
	if spec.Version < 1 {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "schema.minimum",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("version must be >= 1, got %d", spec.Version),
			Location: "version",
		})
	}

	return violations
}

// SpecToJSON converts a spec to its JSON representation for schema validation.
func SpecToJSON(spec *specv1.Spec) ([]byte, error) {
	return json.Marshal(spec)
}
```

**Step 3: Write schema validation tests**

`internal/linter/schema_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/stretchr/testify/require"
)

func TestValidateSchema_ValidSpec(t *testing.T) {
	spec := &specv1.Spec{
		Slug:    "login-api",
		Intent:  "Implement login API",
		Stage:   "spark",
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func TestValidateSchema_MissingRequired(t *testing.T) {
	spec := &specv1.Spec{}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 3) // slug, intent, stage
	for _, v := range violations {
		require.Equal(t, "schema.required", v.Rule)
		require.Equal(t, specv1.LintSeverity_LINT_SEVERITY_ERROR, v.Severity)
	}
}

func TestValidateSchema_InvalidStage(t *testing.T) {
	spec := &specv1.Spec{
		Slug:    "test",
		Intent:  "Test",
		Stage:   "invalid-stage",
		Version: 1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.enum", violations[0].Rule)
}

func TestValidateSchema_SupersededWithoutBy(t *testing.T) {
	spec := &specv1.Spec{
		Slug:    "old-spec",
		Intent:  "Old approach",
		Stage:   "superseded",
		Version: 2,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.conditional", violations[0].Rule)
	require.Contains(t, violations[0].Message, "superseded_by")
}

func TestValidateSchema_SupersededWithBy(t *testing.T) {
	spec := &specv1.Spec{
		Slug:         "old-spec",
		Intent:       "Old approach",
		Stage:        "superseded",
		SupersededBy: "new-spec",
		Version:      2,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}

func TestValidateSchema_InvalidPriority(t *testing.T) {
	spec := &specv1.Spec{
		Slug:     "test",
		Intent:   "Test",
		Stage:    "spark",
		Priority: "critical",
		Version:  1,
	}
	violations := linter.ValidateSchema(spec)
	require.Len(t, violations, 1)
	require.Equal(t, "schema.enum", violations[0].Rule)
	require.Contains(t, violations[0].Message, "priority")
}

func TestValidateSchema_LivingLifecycle(t *testing.T) {
	spec := &specv1.Spec{
		Slug:      "user-api-contract",
		Intent:    "Public contract for User API",
		Stage:     "approved",
		Lifecycle: "living",
		Version:   1,
	}
	violations := linter.ValidateSchema(spec)
	require.Empty(t, violations)
}
```

**Step 4: Run the tests**

```bash
go test ./internal/linter/ -run TestValidateSchema -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add spec.schema.json internal/linter/schema.go internal/linter/schema_test.go
git commit -m "feat(lifecycle): spec JSON Schema with progressive validation"
```

---

## Task 6: Linter Engine

**Files:**

- Create: `internal/linter/linter.go`
- Create: `internal/linter/linter_test.go`

**Step 1: Write the linter tests**

`internal/linter/linter_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter_test

import (
	"context"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/linter"
	"github.com/stretchr/testify/require"
)

type mockLintBackend struct {
	specs map[string]*specv1.Spec
	edges map[string][]string // slug -> depends_on slugs
}

func (m *mockLintBackend) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q not found", slug)
	}
	return spec, nil
}

func (m *mockLintBackend) ListSpecs(_ context.Context, stage, _ string, _ int) ([]*specv1.Spec, error) {
	var result []*specv1.Spec
	for _, s := range m.specs {
		if stage == "" || s.Stage == stage {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockLintBackend) GetDependencies(_ context.Context, slug string) ([]string, error) {
	return m.edges[slug], nil
}

func TestLint_SchemaViolation(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*specv1.Spec{
			"bad-spec": {
				Slug:    "bad-spec",
				Intent:  "",
				Stage:   "spark",
				Version: 1,
			},
		},
		edges: map[string][]string{},
	}

	results, err := linter.Lint(context.Background(), backend, "bad-spec")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.False(t, results[0].Passed)

	hasSchemaViolation := false
	for _, v := range results[0].Violations {
		if v.Rule == "schema.required" {
			hasSchemaViolation = true
		}
	}
	require.True(t, hasSchemaViolation)
}

func TestLint_DanglingDependency(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*specv1.Spec{
			"my-spec": {
				Slug:    "my-spec",
				Intent:  "Test spec",
				Stage:   "approved",
				Version: 1,
			},
		},
		edges: map[string][]string{
			"my-spec": {"nonexistent-dep"},
		},
	}

	results, err := linter.Lint(context.Background(), backend, "my-spec")
	require.NoError(t, err)
	require.Len(t, results, 1)

	hasDanglingRef := false
	for _, v := range results[0].Violations {
		if v.Rule == "edge.dangling_ref" {
			hasDanglingRef = true
		}
	}
	require.True(t, hasDanglingRef)
}

func TestLint_CycleDetection(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*specv1.Spec{
			"spec-a": {Slug: "spec-a", Intent: "A", Stage: "approved", Version: 1},
			"spec-b": {Slug: "spec-b", Intent: "B", Stage: "approved", Version: 1},
		},
		edges: map[string][]string{
			"spec-a": {"spec-b"},
			"spec-b": {"spec-a"},
		},
	}

	results, err := linter.Lint(context.Background(), backend, "spec-a")
	require.NoError(t, err)
	require.Len(t, results, 1)

	hasCycle := false
	for _, v := range results[0].Violations {
		if v.Rule == "graph.cycle" {
			hasCycle = true
		}
	}
	require.True(t, hasCycle)
}

func TestLint_ValidSpec(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*specv1.Spec{
			"good-spec": {
				Slug:    "good-spec",
				Intent:  "A well-formed spec",
				Stage:   "approved",
				Version: 1,
			},
			"upstream": {
				Slug:    "upstream",
				Intent:  "Upstream dep",
				Stage:   "done",
				Version: 1,
			},
		},
		edges: map[string][]string{
			"good-spec": {"upstream"},
		},
	}

	results, err := linter.Lint(context.Background(), backend, "good-spec")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.True(t, results[0].Passed)
	require.Empty(t, results[0].Violations)
}

func TestLint_AllSpecs(t *testing.T) {
	backend := &mockLintBackend{
		specs: map[string]*specv1.Spec{
			"spec-a": {Slug: "spec-a", Intent: "A", Stage: "approved", Version: 1},
			"spec-b": {Slug: "spec-b", Intent: "B", Stage: "done", Version: 1},
		},
		edges: map[string][]string{},
	}

	results, err := linter.Lint(context.Background(), backend, "")
	require.NoError(t, err)
	require.Len(t, results, 2)
}
```

Note: Add `"fmt"` to the test file imports.

**Step 2: Run the tests to verify they fail**

```bash
go test ./internal/linter/ -run "TestLint_" -v -count=1
```

Expected: FAIL — `linter.go` doesn't exist yet

**Step 3: Implement the linter**

`internal/linter/linter.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter

import (
	"context"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// LintBackend is the subset of storage needed by the linter.
type LintBackend interface {
	GetSpec(ctx context.Context, slug string) (*specv1.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]string, error)
}

// Lint validates one or all specs. If slug is empty, lints all specs.
func Lint(ctx context.Context, backend LintBackend, slug string) ([]*specv1.LintResult, error) {
	var specs []*specv1.Spec

	if slug != "" {
		spec, err := backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("lint: get spec %q: %w", slug, err)
		}
		specs = []*specv1.Spec{spec}
	} else {
		allSpecs, err := backend.ListSpecs(ctx, "", "", 0)
		if err != nil {
			return nil, fmt.Errorf("lint: list specs: %w", err)
		}
		specs = allSpecs
	}

	var results []*specv1.LintResult
	for _, spec := range specs {
		result := lintSpec(ctx, backend, spec)
		results = append(results, result)
	}

	return results, nil
}

// lintSpec runs all lint rules against a single spec.
func lintSpec(ctx context.Context, backend LintBackend, spec *specv1.Spec) *specv1.LintResult {
	var violations []*specv1.LintViolation

	// Rule 1: Schema validation
	schemaViolations := ValidateSchema(spec)
	violations = append(violations, schemaViolations...)

	// Rule 2: Edge consistency — no dangling depends_on references
	edgeViolations := checkEdgeConsistency(ctx, backend, spec)
	violations = append(violations, edgeViolations...)

	// Rule 3: Cycle detection
	cycleViolations := checkCycles(ctx, backend, spec)
	violations = append(violations, cycleViolations...)

	// Determine pass/fail — passed if no error-severity violations
	passed := true
	for _, v := range violations {
		if v.Severity == specv1.LintSeverity_LINT_SEVERITY_ERROR {
			passed = false
			break
		}
	}

	return &specv1.LintResult{
		SpecSlug:   spec.Slug,
		Violations: violations,
		Passed:     passed,
	}
}

// checkEdgeConsistency validates that all dependency references resolve to existing specs.
func checkEdgeConsistency(ctx context.Context, backend LintBackend, spec *specv1.Spec) []*specv1.LintViolation {
	var violations []*specv1.LintViolation

	deps, err := backend.GetDependencies(ctx, spec.Slug)
	if err != nil {
		return violations // skip if we can't get dependencies
	}

	for _, depSlug := range deps {
		_, err := backend.GetSpec(ctx, depSlug)
		if err != nil {
			violations = append(violations, &specv1.LintViolation{
				Rule:     "edge.dangling_ref",
				Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
				Message:  fmt.Sprintf("depends_on references %q which does not exist", depSlug),
				Location: fmt.Sprintf("depends_on[%s]", depSlug),
			})
		}
	}

	return violations
}

// checkCycles detects cycles in the dependency graph starting from this spec.
func checkCycles(ctx context.Context, backend LintBackend, spec *specv1.Spec) []*specv1.LintViolation {
	var violations []*specv1.LintViolation

	visited := map[string]bool{}
	inStack := map[string]bool{}

	if hasCycle(ctx, backend, spec.Slug, visited, inStack) {
		violations = append(violations, &specv1.LintViolation{
			Rule:     "graph.cycle",
			Severity: specv1.LintSeverity_LINT_SEVERITY_ERROR,
			Message:  fmt.Sprintf("dependency cycle detected involving %q", spec.Slug),
			Location: "depends_on",
		})
	}

	return violations
}

// hasCycle performs DFS cycle detection.
func hasCycle(ctx context.Context, backend LintBackend, slug string, visited, inStack map[string]bool) bool {
	if inStack[slug] {
		return true
	}
	if visited[slug] {
		return false
	}

	visited[slug] = true
	inStack[slug] = true

	deps, err := backend.GetDependencies(ctx, slug)
	if err != nil {
		return false
	}

	for _, dep := range deps {
		if hasCycle(ctx, backend, dep, visited, inStack) {
			return true
		}
	}

	inStack[slug] = false
	return false
}
```

**Step 4: Run the tests**

```bash
go test ./internal/linter/ -run "TestLint_" -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/linter/linter.go internal/linter/linter_test.go
git commit -m "feat(lifecycle): spec linter with schema, edge, and cycle checks"
```

---

## Task 7: Drift Detection Engine

**Files:**

- Create: `internal/drift/drift.go`
- Create: `internal/drift/drift_test.go`

**Step 1: Write the drift engine tests**

`internal/drift/drift_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package drift_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/drift"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockDriftBackend struct {
	specs map[string]*specv1.Spec
	deps  map[string][]string // slug -> upstream slugs
}

func (m *mockDriftBackend) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q not found", slug)
	}
	return spec, nil
}

func (m *mockDriftBackend) ListSpecs(_ context.Context, stage, _ string, _ int) ([]*specv1.Spec, error) {
	var result []*specv1.Spec
	for _, s := range m.specs {
		if stage == "" || s.Stage == stage {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockDriftBackend) GetDependencies(_ context.Context, slug string) ([]string, error) {
	return m.deps[slug], nil
}

func TestCheckDependencyDrift(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-24 * time.Hour)

	backend := &mockDriftBackend{
		specs: map[string]*specv1.Spec{
			"upstream": {
				Slug:      "upstream",
				Intent:    "API contract",
				Stage:     "done",
				Version:   3,
				UpdatedAt: timestamppb.New(now), // updated recently
			},
			"downstream": {
				Slug:      "downstream",
				Intent:    "Consumer",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(old), // updated a day ago
			},
		},
		deps: map[string][]string{
			"downstream": {"upstream"},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Len(t, reports, 1)
	require.Equal(t, "downstream", reports[0].SpecSlug)

	hasDependencyDrift := false
	for _, item := range reports[0].Items {
		if item.Type == specv1.DriftType_DRIFT_TYPE_DEPENDENCY {
			hasDependencyDrift = true
			require.Equal(t, "upstream", item.UpstreamSlug)
			require.Equal(t, int32(3), item.ActualVersion)
		}
	}
	require.True(t, hasDependencyDrift)
}

func TestCheckDependencyDrift_NoDrift(t *testing.T) {
	now := time.Now().UTC()

	backend := &mockDriftBackend{
		specs: map[string]*specv1.Spec{
			"upstream": {
				Slug:      "upstream",
				Intent:    "API contract",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(now.Add(-48 * time.Hour)),
			},
			"downstream": {
				Slug:      "downstream",
				Intent:    "Consumer",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(now), // updated after upstream
			},
		},
		deps: map[string][]string{
			"downstream": {"upstream"},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "downstream", "")
	require.NoError(t, err)
	require.Empty(t, reports) // no drift
}

func TestCheckAllSpecs(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-24 * time.Hour)

	backend := &mockDriftBackend{
		specs: map[string]*specv1.Spec{
			"upstream": {
				Slug:      "upstream",
				Intent:    "API",
				Stage:     "done",
				Version:   2,
				UpdatedAt: timestamppb.New(now),
			},
			"downstream": {
				Slug:      "downstream",
				Intent:    "Consumer",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(old),
			},
			"independent": {
				Slug:      "independent",
				Intent:    "No deps",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(now),
			},
		},
		deps: map[string][]string{
			"downstream": {"upstream"},
		},
	}

	engine := drift.NewEngine(backend)
	reports, err := engine.Check(context.Background(), "", "")
	require.NoError(t, err)
	// Only downstream should have drift
	require.Len(t, reports, 1)
	require.Equal(t, "downstream", reports[0].SpecSlug)
}

func TestCheckDrift_ScopeFilter(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-24 * time.Hour)

	backend := &mockDriftBackend{
		specs: map[string]*specv1.Spec{
			"upstream": {
				Slug:      "upstream",
				Stage:     "done",
				Version:   2,
				UpdatedAt: timestamppb.New(now),
			},
			"downstream": {
				Slug:      "downstream",
				Stage:     "done",
				Version:   1,
				UpdatedAt: timestamppb.New(old),
			},
		},
		deps: map[string][]string{
			"downstream": {"upstream"},
		},
	}

	engine := drift.NewEngine(backend)

	// Scope=interfaces should find no drift (we don't have interface drift implementation yet)
	reports, err := engine.Check(context.Background(), "downstream", "interfaces")
	require.NoError(t, err)
	require.Empty(t, reports)

	// Scope=deps should find drift
	reports, err = engine.Check(context.Background(), "downstream", "deps")
	require.NoError(t, err)
	require.Len(t, reports, 1)
}
```

**Step 2: Run tests to verify failure**

```bash
go test ./internal/drift/ -run "TestCheck" -v -count=1
```

Expected: FAIL — package doesn't exist

**Step 3: Implement the drift engine**

`internal/drift/drift.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package drift implements drift detection for specs.
package drift

import (
	"context"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// DriftBackend is the subset of storage needed by the drift engine.
type DriftBackend interface {
	GetSpec(ctx context.Context, slug string) (*specv1.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]string, error)
}

// Engine performs drift detection across specs.
type Engine struct {
	backend DriftBackend
}

// NewEngine creates a new drift detection engine.
func NewEngine(backend DriftBackend) *Engine {
	return &Engine{backend: backend}
}

// Check runs drift detection. If slug is empty, checks all done/living specs.
// Scope filters: "" (all), "interfaces", "verify", "deps".
func (e *Engine) Check(ctx context.Context, slug, scope string) ([]*specv1.DriftReport, error) {
	var specs []*specv1.Spec

	if slug != "" {
		spec, err := e.backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("drift: get spec %q: %w", slug, err)
		}
		specs = []*specv1.Spec{spec}
	} else {
		doneSpecs, err := e.backend.ListSpecs(ctx, "done", "", 0)
		if err != nil {
			return nil, fmt.Errorf("drift: list done specs: %w", err)
		}
		specs = append(specs, doneSpecs...)

		amendedSpecs, err := e.backend.ListSpecs(ctx, "amended", "", 0)
		if err != nil {
			return nil, fmt.Errorf("drift: list amended specs: %w", err)
		}
		specs = append(specs, amendedSpecs...)
	}

	var reports []*specv1.DriftReport
	for _, spec := range specs {
		report := &specv1.DriftReport{SpecSlug: spec.Slug}

		// Dependency drift
		if scope == "" || scope == "deps" {
			items, err := e.checkDependencyDrift(ctx, spec)
			if err != nil {
				return nil, fmt.Errorf("drift: dependency check for %q: %w", spec.Slug, err)
			}
			report.Items = append(report.Items, items...)
		}

		// Interface drift (placeholder — compares stored interface field)
		if scope == "" || scope == "interfaces" {
			items := e.checkInterfaceDrift(spec)
			report.Items = append(report.Items, items...)
		}

		// Verify drift (placeholder — checks test_path existence)
		if scope == "" || scope == "verify" {
			items := e.checkVerifyDrift(spec)
			report.Items = append(report.Items, items...)
		}

		if len(report.Items) > 0 {
			reports = append(reports, report)
		}
	}

	return reports, nil
}

// checkDependencyDrift checks if upstream specs have been updated since this spec was last updated.
func (e *Engine) checkDependencyDrift(ctx context.Context, spec *specv1.Spec) ([]*specv1.DriftItem, error) {
	deps, err := e.backend.GetDependencies(ctx, spec.Slug)
	if err != nil {
		return nil, err
	}

	var items []*specv1.DriftItem
	for _, depSlug := range deps {
		upstream, err := e.backend.GetSpec(ctx, depSlug)
		if err != nil {
			continue // skip missing deps — linter catches those
		}

		// If upstream was updated after this spec, there's dependency drift
		if spec.UpdatedAt != nil && upstream.UpdatedAt != nil {
			if upstream.UpdatedAt.AsTime().After(spec.UpdatedAt.AsTime()) {
				items = append(items, &specv1.DriftItem{
					Type:          specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
					Severity:      specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
					Description:   fmt.Sprintf("upstream %q changed (v%d) since %q was last updated", depSlug, upstream.Version, spec.Slug),
					SpecSlug:      spec.Slug,
					UpstreamSlug:  depSlug,
					ActualVersion: upstream.Version,
				})
			}
		}
	}

	return items, nil
}

// checkInterfaceDrift is a placeholder for interface drift detection.
// Full implementation requires comparing spec's interface field against stored data.
// Code-level analysis is deferred (ADR-001 scope).
func (e *Engine) checkInterfaceDrift(_ *specv1.Spec) []*specv1.DriftItem {
	// TODO: Implement when specs carry structured interface fields.
	// For now, interface drift is not detectable without code analysis.
	return nil
}

// checkVerifyDrift is a placeholder for verify drift detection.
// Full implementation requires checking test_path existence on disk.
func (e *Engine) checkVerifyDrift(_ *specv1.Spec) []*specv1.DriftItem {
	// TODO: Implement when specs carry verify criteria with test_path.
	// For now, verify drift requires file system access not available in storage layer.
	return nil
}
```

**Step 4: Run the tests**

```bash
go test ./internal/drift/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/drift/drift.go internal/drift/drift_test.go
git commit -m "feat(lifecycle): drift detection engine with dependency drift checks"
```

---

## Task 8: ConnectRPC Handler — LifecycleService

**Files:**

- Create: `internal/server/lifecycle_handler.go`
- Create: `internal/server/lifecycle_handler_test.go`

**Step 1: Write the handler tests**

`internal/server/lifecycle_handler_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockLifecycleBackend struct {
	mu    sync.Mutex
	specs map[string]*specv1.Spec
}

func newMockLifecycleBackend() *mockLifecycleBackend {
	return &mockLifecycleBackend{
		specs: map[string]*specv1.Spec{
			"done-spec": {
				Slug: "done-spec", Intent: "Test", Stage: "done", Version: 2,
			},
			"active-spec": {
				Slug: "active-spec", Intent: "In progress", Stage: "in_progress", Version: 1,
			},
			"new-spec": {
				Slug: "new-spec", Intent: "Replacement", Stage: "spark", Version: 1,
			},
		},
	}
}

func (m *mockLifecycleBackend) AmendSpec(_ context.Context, slug, reason, reEntryStage string) (*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	if spec.Stage != "done" {
		return nil, storage.ErrSpecNotDone
	}
	spec.Stage = "amended"
	spec.Version++
	spec.History = append(spec.History, &specv1.HistoryEntry{
		Version: spec.Version,
		Stage:   "amended",
		Reason:  reason,
	})
	return spec, nil
}

func (m *mockLifecycleBackend) SupersedeSpec(_ context.Context, oldSlug, newSlug string) (*specv1.Spec, *specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	oldSpec, ok := m.specs[oldSlug]
	if !ok {
		return nil, nil, storage.ErrSpecNotFound
	}
	newSpec, ok := m.specs[newSlug]
	if !ok {
		return nil, nil, storage.ErrNewSpecNotFound
	}
	oldSpec.Stage = "superseded"
	oldSpec.SupersededBy = newSlug
	newSpec.Supersedes = oldSlug
	return oldSpec, newSpec, nil
}

func (m *mockLifecycleBackend) AbandonSpec(_ context.Context, slug, reason string) (*specv1.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	if spec.Stage == "superseded" || spec.Stage == "abandoned" {
		return nil, storage.ErrSpecTerminal
	}
	spec.Stage = "abandoned"
	spec.History = append(spec.History, &specv1.HistoryEntry{
		Version: spec.Version + 1,
		Stage:   "abandoned",
		Reason:  reason,
	})
	return spec, nil
}

func (m *mockLifecycleBackend) CheckDrift(_ context.Context, slug, _ string) ([]*specv1.DriftReport, error) {
	return []*specv1.DriftReport{
		{SpecSlug: slug, Items: []*specv1.DriftItem{
			{Type: specv1.DriftType_DRIFT_TYPE_DEPENDENCY, Severity: specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM},
		}},
	}, nil
}

func (m *mockLifecycleBackend) AcknowledgeDrift(_ context.Context, slug, note string) (*specv1.DriftReport, error) {
	return &specv1.DriftReport{SpecSlug: slug, Acknowledged: true, AcknowledgeNote: note}, nil
}

func (m *mockLifecycleBackend) GetDependentsForDriftNotification(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockLifecycleBackend) SetDriftFlag(_ context.Context, _, _, _ string) error {
	return nil
}

var _ storage.LifecycleBackend = (*mockLifecycleBackend)(nil)

func setupLifecycleServer(t *testing.T) specgraphv1connect.LifecycleServiceClient {
	t.Helper()
	mb := newMockLifecycleBackend()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)
}

func TestLifecycleHandler_Amend(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	resp, err := client.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{
		Slug:   "done-spec",
		Reason: "Mobile needs offline refresh",
	}))
	require.NoError(t, err)
	require.Equal(t, "amended", resp.Msg.Stage)
}

func TestLifecycleHandler_Amend_NotDone(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	_, err := client.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{
		Slug:   "active-spec",
		Reason: "Should fail",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
}

func TestLifecycleHandler_Amend_NotFound(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	_, err := client.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{
		Slug:   "nonexistent",
		Reason: "Should fail",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestLifecycleHandler_Supersede(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	resp, err := client.Supersede(ctx, connect.NewRequest(&specv1.SupersedeRequest{
		Slug:    "done-spec",
		NewSlug: "new-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, "superseded", resp.Msg.OldSpec.Stage)
	require.Equal(t, "new-spec", resp.Msg.OldSpec.SupersededBy)
	require.Equal(t, "done-spec", resp.Msg.NewSpec.Supersedes)
}

func TestLifecycleHandler_Abandon(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	resp, err := client.Abandon(ctx, connect.NewRequest(&specv1.AbandonRequest{
		Slug:   "active-spec",
		Reason: "Requirements changed",
	}))
	require.NoError(t, err)
	require.Equal(t, "abandoned", resp.Msg.Stage)
}

func TestLifecycleHandler_CheckDrift(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	resp, err := client.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "done-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 1)
}

func TestLifecycleHandler_AcknowledgeDrift(t *testing.T) {
	client := setupLifecycleServer(t)
	ctx := context.Background()

	resp, err := client.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "done-spec",
		Note: "Intentional divergence",
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Acknowledged)
	require.Equal(t, "Intentional divergence", resp.Msg.AcknowledgeNote)
}
```

**Step 2: Run tests to verify failure**

```bash
go test ./internal/server/ -run TestLifecycle -v -count=1
```

Expected: FAIL — handler doesn't exist yet

**Step 3: Implement the handler**

`internal/server/lifecycle_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// LifecycleHandler implements the LifecycleService.
type LifecycleHandler struct {
	store storage.LifecycleBackend
}

var _ specgraphv1connect.LifecycleServiceHandler = (*LifecycleHandler)(nil)

// RegisterLifecycleService registers the LifecycleService handler on the mux.
func RegisterLifecycleService(mux *http.ServeMux, store storage.LifecycleBackend) {
	handler := &LifecycleHandler{store: store}
	path, h := specgraphv1connect.NewLifecycleServiceHandler(handler)
	mux.Handle(path, h)
}

func (h *LifecycleHandler) Amend(ctx context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.Spec], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if req.Msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required"))
	}

	spec, err := h.store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, req.Msg.ReEntryStage)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSpecNotDone) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *LifecycleHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if req.Msg.NewSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_slug is required"))
	}

	oldSpec, newSpec, err := h.store.SupersedeSpec(ctx, req.Msg.Slug, req.Msg.NewSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrNewSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSpecTerminal) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.SupersedeResponse{
		OldSpec: oldSpec,
		NewSpec: newSpec,
	}), nil
}

func (h *LifecycleHandler) Abandon(ctx context.Context, req *connect.Request[specv1.AbandonRequest]) (*connect.Response[specv1.Spec], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	if req.Msg.Reason == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("reason is required"))
	}

	spec, err := h.store.AbandonSpec(ctx, req.Msg.Slug, req.Msg.Reason)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSpecTerminal) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *LifecycleHandler) CheckDrift(ctx context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	reports, err := h.store.CheckDrift(ctx, req.Msg.Slug, req.Msg.Scope)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.DriftCheckResponse{Reports: reports}), nil
}

func (h *LifecycleHandler) AcknowledgeDrift(ctx context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftReport], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}

	report, err := h.store.AcknowledgeDrift(ctx, req.Msg.Slug, req.Msg.Note)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(report), nil
}

func (h *LifecycleHandler) Lint(ctx context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	// The linter requires access to the full storage backend for graph queries.
	// For now, return an unimplemented error — the lint RPC is wired but requires
	// a LintBackend adapter that combines LifecycleBackend + Backend + GraphBackend.
	// The CLI will call the linter directly (in-process) rather than via RPC for now.
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("lint via RPC not yet implemented — use CLI"))
}
```

**Step 4: Run the tests**

```bash
go test ./internal/server/ -run TestLifecycle -v -count=1
```

Expected: PASS

**Step 5: Wire into serve.go**

Add `server.RegisterLifecycleService(mux, store)` in `cmd/specgraph/serve.go` after the other service registrations:

```go
// In serve.go, after line "server.RegisterClaimService(mux, store)":
server.RegisterLifecycleService(mux, store)
```

**Step 6: Verify full build**

```bash
go build ./cmd/specgraph/
```

**Step 7: Commit**

```bash
git add internal/server/lifecycle_handler.go internal/server/lifecycle_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(lifecycle): ConnectRPC handler with handler tests"
```

---

## Task 9: CLI Commands — Lifecycle Operations

**Files:**

- Create: `cmd/specgraph/lifecycle.go`

**Step 1: Implement the CLI commands**

`cmd/specgraph/lifecycle.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func lifecycleClient() (specgraphv1connect.LifecycleServiceClient, error) {
	return newClient(specgraphv1connect.NewLifecycleServiceClient)
}

// --- amend ---

var amendCmd = &cobra.Command{
	Use:   "amend <slug>",
	Short: "Reopen a done spec for amendment",
	Long:  "Transitions a done spec to amended status, recording the reason. Re-enters the authoring funnel at shape (default) or specify.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAmend,
}

var (
	amendReason     string
	amendReEntry    string
)

func runAmend(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:         args[0],
		Reason:       amendReason,
		ReEntryStage: amendReEntry,
	}))
	if err != nil {
		return fmt.Errorf("amend: %w", err)
	}
	s := resp.Msg
	fmt.Printf("Amended: %s (version %d, stage → %s)\n", s.Slug, s.Version, s.Stage)
	fmt.Printf("Reason:  %s\n", amendReason)
	return nil
}

// --- supersede ---

var supersedeCmd = &cobra.Command{
	Use:   "supersede <slug>",
	Short: "Replace a spec with a new one",
	Long:  "Marks the old spec as superseded and links it to the replacement spec.",
	Args:  cobra.ExactArgs(1),
	RunE:  runSupersede,
}

var supersedeWith string

func runSupersede(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:    args[0],
		NewSlug: supersedeWith,
	}))
	if err != nil {
		return fmt.Errorf("supersede: %w", err)
	}
	fmt.Printf("Superseded: %s → %s\n", resp.Msg.OldSpec.Slug, resp.Msg.NewSpec.Slug)
	fmt.Printf("Old spec:   %s (stage → %s)\n", resp.Msg.OldSpec.Slug, resp.Msg.OldSpec.Stage)
	fmt.Printf("New spec:   %s (supersedes → %s)\n", resp.Msg.NewSpec.Slug, resp.Msg.NewSpec.Supersedes)
	return nil
}

// --- abandon ---

var abandonCmd = &cobra.Command{
	Use:   "abandon <slug>",
	Short: "Abandon a spec (terminal)",
	Long:  "Marks a spec as abandoned with a reason. This is a terminal state.",
	Args:  cobra.ExactArgs(1),
	RunE:  runAbandon,
}

var abandonReason string

func runAbandon(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.Abandon(context.Background(), connect.NewRequest(&specv1.AbandonRequest{
		Slug:   args[0],
		Reason: abandonReason,
	}))
	if err != nil {
		return fmt.Errorf("abandon: %w", err)
	}
	fmt.Printf("Abandoned: %s (version %d)\n", resp.Msg.Slug, resp.Msg.Version)
	fmt.Printf("Reason:    %s\n", abandonReason)
	return nil
}

// --- drift ---

var driftCmd = &cobra.Command{
	Use:   "drift [slug]",
	Short: "Check for drift in done/living specs",
	Long:  "Runs drift detection across specs. If a slug is provided, checks only that spec.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDrift,
}

var driftScope string

func runDrift(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}

	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug:  slug,
		Scope: driftScope,
	}))
	if err != nil {
		return fmt.Errorf("drift: %w", err)
	}

	reports := resp.Msg.Reports
	if len(reports) == 0 {
		fmt.Println("No drift detected.")
		return nil
	}

	for _, report := range reports {
		fmt.Printf("\n=== %s ===\n", report.SpecSlug)
		if report.Acknowledged {
			fmt.Printf("  (acknowledged: %s)\n", report.AcknowledgeNote)
		}
		for _, item := range report.Items {
			fmt.Printf("  [%s] %s: %s\n",
				item.Severity.String(),
				item.Type.String(),
				item.Description,
			)
		}
	}
	return nil
}

// --- drift acknowledge ---

var driftAcknowledgeCmd = &cobra.Command{
	Use:   "acknowledge <slug>",
	Short: "Acknowledge drift as intentional",
	Args:  cobra.ExactArgs(1),
	RunE:  runDriftAcknowledge,
}

var driftAckNote string

func runDriftAcknowledge(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}
	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: args[0],
		Note: driftAckNote,
	}))
	if err != nil {
		return fmt.Errorf("drift acknowledge: %w", err)
	}
	fmt.Printf("Acknowledged drift for %s: %s\n", resp.Msg.SpecSlug, resp.Msg.AcknowledgeNote)
	return nil
}

// --- lint ---

var lintCmd = &cobra.Command{
	Use:   "lint [slug]",
	Short: "Validate spec structure and consistency",
	Long:  "Runs the spec linter. Checks schema, edge consistency, constitution compliance, and cycle detection.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLint,
}

func runLint(_ *cobra.Command, args []string) error {
	client, err := lifecycleClient()
	if err != nil {
		return err
	}

	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: slug,
	}))
	if err != nil {
		return fmt.Errorf("lint: %w", err)
	}

	results := resp.Msg.Results
	if len(results) == 0 {
		fmt.Println("No specs to lint.")
		return nil
	}

	allPassed := true
	for _, result := range results {
		if result.Passed {
			fmt.Printf("PASS  %s\n", result.SpecSlug)
		} else {
			allPassed = false
			fmt.Printf("FAIL  %s\n", result.SpecSlug)
		}
		for _, v := range result.Violations {
			fmt.Printf("  [%s] %s: %s (%s)\n",
				v.Severity.String(),
				v.Rule,
				v.Message,
				v.Location,
			)
		}
	}

	if !allPassed {
		return fmt.Errorf("lint failed")
	}
	return nil
}

func init() {
	amendCmd.Flags().StringVar(&amendReason, "reason", "", "reason for amendment (required)")
	amendCmd.Flags().StringVar(&amendReEntry, "re-entry", "shape", "re-entry stage (shape or specify)")
	cobra.CheckErr(amendCmd.MarkFlagRequired("reason"))
	rootCmd.AddCommand(amendCmd)

	supersedeCmd.Flags().StringVar(&supersedeWith, "with", "", "slug of replacement spec (required)")
	cobra.CheckErr(supersedeCmd.MarkFlagRequired("with"))
	rootCmd.AddCommand(supersedeCmd)

	abandonCmd.Flags().StringVar(&abandonReason, "reason", "", "reason for abandonment (required)")
	cobra.CheckErr(abandonCmd.MarkFlagRequired("reason"))
	rootCmd.AddCommand(abandonCmd)

	driftCmd.Flags().StringVar(&driftScope, "scope", "", "scope filter: interfaces, verify, deps")
	driftAcknowledgeCmd.Flags().StringVar(&driftAckNote, "note", "", "acknowledgement note (required)")
	cobra.CheckErr(driftAcknowledgeCmd.MarkFlagRequired("note"))
	driftCmd.AddCommand(driftAcknowledgeCmd)
	rootCmd.AddCommand(driftCmd)

	rootCmd.AddCommand(lintCmd)
}
```

**Step 2: Verify build**

```bash
go build ./cmd/specgraph/
```

**Step 3: Verify CLI help**

```bash
./specgraph amend --help
./specgraph supersede --help
./specgraph abandon --help
./specgraph drift --help
./specgraph drift acknowledge --help
./specgraph lint --help
```

**Step 4: Commit**

```bash
git add cmd/specgraph/lifecycle.go
git commit -m "feat(lifecycle): CLI commands for amend, supersede, abandon, drift, lint"
```

---

## Task 10: End-to-End Integration Test

**Files:**

- Create: `test/e2e/lifecycle_test.go`

This task verifies the full lifecycle flow: create spec -> move to done -> amend -> supersede -> abandon, plus drift detection and lint, all through the running server.

**Step 1: Write the E2E test**

`test/e2e/lifecycle_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package e2e_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecycleE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	// This test requires a running SpecGraph server.
	// Use the existing E2E test pattern from the project.
	ctx := context.Background()

	// Create a spec
	out, err := exec.CommandContext(ctx, "specgraph", "create", "e2e-lifecycle",
		"--intent", "E2E lifecycle test").CombinedOutput()
	require.NoError(t, err, "create: %s", out)

	// Move to done
	out, err = exec.CommandContext(ctx, "specgraph", "update", "e2e-lifecycle",
		"--stage", "done").CombinedOutput()
	require.NoError(t, err, "update to done: %s", out)

	// Amend
	out, err = exec.CommandContext(ctx, "specgraph", "amend", "e2e-lifecycle",
		"--reason", "E2E amendment test").CombinedOutput()
	require.NoError(t, err, "amend: %s", out)

	// Verify show reflects amendment
	out, err = exec.CommandContext(ctx, "specgraph", "show", "e2e-lifecycle").CombinedOutput()
	require.NoError(t, err, "show: %s", out)
	require.Contains(t, string(out), "amended")

	// Create replacement spec for supersede test
	out, err = exec.CommandContext(ctx, "specgraph", "create", "e2e-lifecycle-v2",
		"--intent", "E2E replacement").CombinedOutput()
	require.NoError(t, err, "create replacement: %s", out)

	// Move original back to done to test supersede
	out, err = exec.CommandContext(ctx, "specgraph", "update", "e2e-lifecycle",
		"--stage", "done").CombinedOutput()
	require.NoError(t, err, "update to done again: %s", out)

	// Supersede
	out, err = exec.CommandContext(ctx, "specgraph", "supersede", "e2e-lifecycle",
		"--with", "e2e-lifecycle-v2").CombinedOutput()
	require.NoError(t, err, "supersede: %s", out)

	// Create a spec to test abandon
	out, err = exec.CommandContext(ctx, "specgraph", "create", "e2e-abandon",
		"--intent", "E2E abandon test").CombinedOutput()
	require.NoError(t, err, "create for abandon: %s", out)

	// Abandon
	out, err = exec.CommandContext(ctx, "specgraph", "abandon", "e2e-abandon",
		"--reason", "Requirements dropped").CombinedOutput()
	require.NoError(t, err, "abandon: %s", out)

	// Drift check (should not error even with no drift)
	out, err = exec.CommandContext(ctx, "specgraph", "drift").CombinedOutput()
	require.NoError(t, err, "drift: %s", out)
}
```

**Step 2: Run the test (requires running server)**

```bash
# Start server in another terminal: specgraph serve
go test ./test/e2e/ -run TestLifecycleE2E -v -count=1 -timeout=60s
```

Expected: PASS (when server is running)

**Step 3: Commit**

```bash
git add test/e2e/lifecycle_test.go
git commit -m "test(lifecycle): end-to-end lifecycle flow test"
```

---

## Summary

| Task | Description | Files | Test Command |
|------|-------------|-------|-------------|
| 1 | Proto schema | lifecycle.proto, spec.proto, graph.proto | `buf generate && go build ./gen/...` |
| 2 | Spec storage extensions | memgraph.go, graph.go | `go test ./internal/storage/memgraph/ -run TestCreateAndGetSpec -v` |
| 3 | LifecycleBackend interface | lifecycle.go | `go build ./internal/storage/` |
| 4 | Memgraph lifecycle storage | lifecycle.go, lifecycle_test.go | `go test ./internal/storage/memgraph/ -run "TestAmend\|TestSupersede\|TestAbandon" -v` |
| 5 | JSON Schema + validator | spec.schema.json, schema.go, schema_test.go | `go test ./internal/linter/ -run TestValidateSchema -v` |
| 6 | Linter engine | linter.go, linter_test.go | `go test ./internal/linter/ -run "TestLint_" -v` |
| 7 | Drift engine | drift.go, drift_test.go | `go test ./internal/drift/ -v` |
| 8 | ConnectRPC handler | lifecycle_handler.go, lifecycle_handler_test.go | `go test ./internal/server/ -run TestLifecycle -v` |
| 9 | CLI commands | lifecycle.go | `go build ./cmd/specgraph/` |
| 10 | E2E test | lifecycle_test.go | `go test ./test/e2e/ -run TestLifecycleE2E -v` |

**Total: 10 tasks**

### Dependencies Between Tasks

```text
Task 1 (proto)
    │
    ├──→ Task 2 (spec storage extensions)
    │        │
    │        └──→ Task 4 (memgraph lifecycle storage)
    │                 │
    │                 └──→ Task 8 (handler) ──→ Task 9 (CLI) ──→ Task 10 (E2E)
    │
    ├──→ Task 3 (interface) ──→ Task 4
    │
    ├──→ Task 5 (JSON Schema + validator) ──→ Task 6 (linter) ──→ Task 8
    │
    └──→ Task 7 (drift engine) ──→ Task 8
```

Tasks 5, 6, 7 can be developed in parallel with Tasks 2-4 after Task 1.
