# Analytical Pass System Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace placeholder analytical pass execution with an agent-driven LLM system using a unified `AnalyticalFinding` type, a new `AnalyticalPassService` RPC, and embedded prompt templates.

**Architecture:** New `AnalyticalPassService` (proto + handler) provides prompt templates and tool manifests via `RunAnalyticalPass` RPC. Agent runs the LLM, stores results via `StoreFindings`. Unified `AnalyticalFinding` replaces five separate finding types. Old `CheckViolation` removed entirely.

**Tech Stack:** Go, ConnectRPC, Memgraph (Cypher), Protocol Buffers, Go embed

**Spec:** `docs/superpowers/specs/2026-03-20-analytical-pass-system-design.md`

---

## File Map

### New Files

| File | Responsibility |
|------|---------------|
| `proto/specgraph/v1/analytical_pass.proto` | `AnalyticalPassService` proto definition |
| `internal/storage/findings.go` | `AnalyticalFinding` domain type, `PassType`, `FindingsBackend` interface |
| `internal/storage/memgraph/findings.go` | Memgraph `StoreFindings`/`ListFindings` implementation |
| `internal/storage/memgraph/findings_test.go` | Integration tests for findings storage |
| `internal/server/analytical_pass_handler.go` | `AnalyticalPassHandler` ConnectRPC handler |
| `internal/server/analytical_pass_handler_test.go` | Handler unit tests |
| `internal/server/templates/constitution_check.md` | Embedded prompt template for constitution_check |

### Modified Files

| File | Change |
|------|--------|
| `proto/specgraph/v1/authoring.proto` | Remove 5 finding messages + finding fields from stage responses |
| `proto/specgraph/v1/constitution.proto` | Remove `CheckViolation` RPC + request/response messages |
| `internal/storage/authoring.go` | Remove 5 finding structs, remove `PassWriter`, move `StoreSafetyFlags` to `StageWriter` |
| `internal/storage/constitution.go` | Remove `CheckViolation` from `ConstitutionBackend` |
| `internal/storage/scoper.go` | Add `FindingsBackend` to `ScopedBackend` composition |
| `internal/storage/memgraph/authoring.go` | Remove 5 `Store*` methods + `allowedJSONProperties` entries |
| `internal/storage/memgraph/constitution.go` | Remove `CheckViolation` implementation |
| `internal/storage/memgraph/graph.go` | Add `HAS_FINDING` to `ListEdges` exclusion |
| `internal/server/authoring_handler.go` | Remove `runAnalyticalPasses`, remove finding fields from responses |
| `internal/server/constitution_handler.go` | Remove `CheckViolation` handler method |
| `internal/server/authoring_handler_test.go` | Update tests for removed fields |
| `cmd/specgraph/serve.go` | Register `AnalyticalPassService` |
| `cmd/specgraph/constitution.go` | Remove `check` subcommand |

---

## Chunk 1: Proto Changes and Domain Types

### Task 1: Remove old finding messages from authoring.proto

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto:198-244` (remove finding messages)
- Modify: `proto/specgraph/v1/authoring.proto:286-371` (remove finding fields from stage responses)

- [ ] **Step 1: Remove the 5 analytical pass finding messages**

In `proto/specgraph/v1/authoring.proto`, delete the entire "Analytical Pass Messages" section (lines 198-244): `RedTeamFinding`, `PeripheralVisionItem`, `ConsistencyIssue`, `SimplicityFinding`, `ConstitutionViolation`.

Also delete the orphaned enums that were only used by removed messages:

- `PeripheralDisposition` enum (line 49)
- `IssueKind` enum (line 91)

Keep `SafetyFlag` (line 248-256), `PromptTemplate` (line 260-268), `FindingSeverity` — those stay.

- [ ] **Step 2: Remove finding fields from SparkResponse**

In `SparkResponse` (line 286), remove field 3 (`constitution_violations`). Keep `output` (1), `safety_flags` (2), `next_prompts` (4).

- [ ] **Step 3: Remove finding fields from ShapeResponse**

In `ShapeResponse` (line 308), remove fields 2 (`peripheral_vision`) and 4 (`constitution_violations`). Keep `output` (1), `safety_flags` (3), `next_prompts` (5).

- [ ] **Step 4: Remove finding fields from SpecifyResponse**

In `SpecifyResponse` (line 332), remove fields 2 (`red_team`), 3 (`consistency_issues`), and 5 (`constitution_violations`). Keep `output` (1), `safety_flags` (4), `next_prompts` (6).

- [ ] **Step 5: Remove finding fields from DecomposeResponse**

In `DecomposeResponse` (line 358), remove fields 2 (`simplicity`), and 4 (`constitution_violations`). Keep `output` (1), `safety_flags` (3), `next_prompts` (5), `child_spec_slugs` (6).

### Task 2: Remove CheckViolation from constitution.proto

**Files:**

- Modify: `proto/specgraph/v1/constitution.proto:203-212` (remove messages)
- Modify: `proto/specgraph/v1/constitution.proto:236-237` (remove RPC)

- [ ] **Step 1: Remove CheckViolation messages**

Delete `CheckViolationRequest` (lines 203-207) and `CheckViolationResponse` (lines 209-212).

- [ ] **Step 2: Remove CheckViolation RPC**

In the `ConstitutionService` service block (line 231), delete line 237: `rpc CheckViolation(CheckViolationRequest) returns (CheckViolationResponse);`

- [ ] **Step 3: Remove orphaned Violation message and ViolationSeverity enum**

The `Violation` message and `ViolationSeverity` enum in `constitution.proto` were only used by `CheckViolationResponse`. Delete them as dead code. Also delete `Violation` struct and `ViolationSeverity` type from `internal/storage/constitution_domain.go` (lines 20-28, 109-115).

### Task 3: Create new analytical_pass.proto

**Files:**

- Create: `proto/specgraph/v1/analytical_pass.proto`

- [ ] **Step 1: Write the proto file**

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";
package specgraph.v1;

option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";

import "specgraph/v1/authoring.proto";

// AnalyticalPassService manages LLM-driven analytical passes for spec evaluation.
service AnalyticalPassService {
  // RunAnalyticalPass returns the prompt template, tool manifest, and initial
  // message needed to execute an analytical pass against a spec.
  rpc RunAnalyticalPass(RunAnalyticalPassRequest) returns (RunAnalyticalPassResponse);
  // StoreFindings persists findings produced by an analytical pass.
  rpc StoreFindings(StoreFindingsRequest) returns (StoreFindingsResponse);
  // ListFindings retrieves stored findings for a spec, optionally filtered by pass type.
  rpc ListFindings(ListFindingsRequest) returns (ListFindingsResponse);
}

message RunAnalyticalPassRequest {
  // Spec slug to evaluate.
  string slug = 1;
  // Analytical pass to run (e.g. "constitution_check").
  string pass_name = 2;
}

message RunAnalyticalPassResponse {
  // The pass being executed.
  string pass_name = 1;
  // Markdown persona prompt (system prompt for the LLM).
  string prompt_template = 2;
  // CLI commands/RPCs the agent should expose as LLM tools.
  repeated ToolReference tools = 3;
  // User-turn message to kick off the evaluation.
  string initial_message = 4;
  // Passes available but not auto-running for the current posture.
  repeated string offered_passes = 5;
  // The spec's current stage (informational for the agent).
  string stage = 6;
}

// ToolReference describes a CLI command or RPC the agent can expose as an LLM tool.
message ToolReference {
  // Short tool name (e.g. "show_spec").
  string name = 1;
  // CLI command template (e.g. "specgraph show {slug}").
  string command = 2;
  // Human-readable description of what this tool does.
  string description = 3;
}

// AnalyticalFinding is a finding produced by an analytical pass. Returned by ListFindings.
message AnalyticalFinding {
  // Server-generated ULID. Output-only.
  string id = 1;
  // Which pass produced this finding.
  string pass_type = 2;
  // How severe the finding is.
  FindingSeverity severity = 3;
  // Concise description of the finding.
  string summary = 4;
  // Full explanation and context.
  string detail = 5;
  // Which rule or principle was evaluated (optional).
  string constraint = 6;
  // Suggested remediation (optional).
  string resolution = 7;
  // Spec version when the finding was produced.
  int32 version = 8;
}

// AnalyticalFindingInput is the agent-submitted variant without server-generated fields.
message AnalyticalFindingInput {
  // How severe the finding is.
  FindingSeverity severity = 1;
  // Concise description of the finding.
  string summary = 2;
  // Full explanation and context.
  string detail = 3;
  // Which rule or principle was evaluated (optional).
  string constraint = 4;
  // Suggested remediation (optional).
  string resolution = 5;
}

message StoreFindingsRequest {
  // Spec slug the findings are about.
  string slug = 1;
  // Which pass produced these findings.
  string pass_type = 2;
  // The findings to store. Replaces all existing findings for this (slug, pass_type).
  repeated AnalyticalFindingInput findings = 3;
}

message StoreFindingsResponse {
  // Server-generated IDs for the stored findings.
  repeated string ids = 1;
}

message ListFindingsRequest {
  // Spec slug to list findings for.
  string slug = 1;
  // Optional: filter by pass type. Empty returns all findings for the spec.
  string pass_type = 2;
}

message ListFindingsResponse {
  // Findings matching the request criteria.
  repeated AnalyticalFinding findings = 1;
}
```

- [ ] **Step 2: Run `task proto` to generate Go code**

Run: `task proto`
Expected: New files generated in `gen/specgraph/v1/` for the analytical pass service.

- [ ] **Step 3: Verify the build compiles (it won't yet — callers need updating)**

Run: `go build ./gen/...`
Expected: Compiles. The generated code has no callers yet, so no errors.

- [ ] **Step 4: Commit**

```bash
jj commit -m "proto: add AnalyticalPassService, remove old finding messages and CheckViolation

New analytical_pass.proto defines RunAnalyticalPass, StoreFindings, and
ListFindings RPCs with unified AnalyticalFinding type.

Remove: RedTeamFinding, PeripheralVisionItem, ConsistencyIssue,
SimplicityFinding, ConstitutionViolation messages. Remove finding fields
from SparkResponse, ShapeResponse, SpecifyResponse, DecomposeResponse.
Remove CheckViolation RPC from ConstitutionService."
```

### Task 4: Create domain types and storage interfaces

**Files:**

- Create: `internal/storage/findings.go`
- Modify: `internal/storage/authoring.go:112-201` (remove old finding types and PassWriter)
- Modify: `internal/storage/constitution.go:24` (remove CheckViolation)
- Modify: `internal/storage/scoper.go:10-21` (add FindingsBackend)

- [ ] **Step 1: Create `internal/storage/findings.go`**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// PassType is a typed string for analytical pass identifiers.
type PassType string

// PassType values.
const (
	PassTypeConstitutionCheck PassType = "constitution_check"
	PassTypePeripheralVision  PassType = "peripheral_vision"
	PassTypeRedTeam           PassType = "red_team"
	PassTypeConsistencyCheck  PassType = "consistency_check"
	PassTypeSimplicityCheck   PassType = "simplicity_check"
)

// ValidPassType reports whether pt is a known pass type.
func ValidPassType(pt PassType) bool {
	switch pt {
	case PassTypeConstitutionCheck, PassTypePeripheralVision,
		PassTypeRedTeam, PassTypeConsistencyCheck, PassTypeSimplicityCheck:
		return true
	}
	return false
}

// AnalyticalFinding records a finding produced by an analytical pass.
type AnalyticalFinding struct {
	ID         string
	PassType   PassType
	Severity   FindingSeverity
	Summary    string
	Detail     string
	Constraint string
	Resolution string
	Version    int32
	CreatedAt  time.Time
}

// FindingsWriter stores analytical pass findings.
type FindingsWriter interface {
	// StoreFindings replaces all findings for the given (slug, passType) pair.
	StoreFindings(ctx context.Context, slug string, passType PassType, findings []AnalyticalFinding) error
}

// FindingsReader retrieves analytical pass findings.
type FindingsReader interface {
	// ListFindings returns findings for a spec. If passType is empty, returns all findings.
	ListFindings(ctx context.Context, slug string, passType PassType) ([]AnalyticalFinding, error)
}

// FindingsBackend combines finding read and write operations.
type FindingsBackend interface {
	FindingsWriter
	FindingsReader
}
```

- [ ] **Step 2: Remove old finding types from `internal/storage/authoring.go`**

Delete these structs and types (keep `FindingSeverity` and its constants, and `SafetyFlag`/`SafetyCategory` — those stay):

- `RedTeamFinding` (lines 122-126)
- `PeripheralDisposition` type and its constants (lines 128-136)
- `PeripheralVisionItem` (lines 139-142)
- `ConsistencyIssue` (lines 145-149)
- `SimplicityFinding` (lines 152-155)
- `ConstitutionViolation` (lines 168-172)

- [ ] **Step 3: Remove `PassWriter` interface, move `StoreSafetyFlags` to `StageWriter`**

Delete the `PassWriter` interface (lines 194-201). Add `StoreSafetyFlags` to `StageWriter` (line 185):

```go
type StageWriter interface {
	TransitionStage(ctx context.Context, slug string, from, to AuthoringStage) error
	StoreSparkOutput(ctx context.Context, slug string, output *SparkOutput) error
	StoreShapeOutput(ctx context.Context, slug string, output *ShapeOutput) error
	StoreSpecifyOutput(ctx context.Context, slug string, output *SpecifyOutput) error
	StoreDecomposeOutput(ctx context.Context, slug string, output *DecomposeOutput) ([]string, error)
	StoreSafetyFlags(ctx context.Context, slug string, flags []SafetyFlag) error
}
```

Update `AuthoringBackend` (line 214) to remove `PassWriter`:

```go
type AuthoringBackend interface {
	StageWriter
	AuthoringSpecLifecycle
}
```

- [ ] **Step 4: Remove `CheckViolation` from `ConstitutionBackend`**

In `internal/storage/constitution.go`, remove line 24 (`CheckViolation` method) from the interface. Result:

```go
type ConstitutionBackend interface {
	GetConstitution(ctx context.Context) (*Constitution, error)
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
}
```

- [ ] **Step 5: Add `FindingsBackend` to `ScopedBackend`**

In `internal/storage/scoper.go`, add `FindingsBackend` to the composition:

```go
type ScopedBackend interface {
	Backend
	GraphBackend
	DecisionBackend
	ClaimBackend
	ConstitutionBackend
	AuthoringBackend
	FindingsBackend
	ExecutionBackend
	LifecycleBackend
	SyncBackend
	ProjectBackend
}
```

- [ ] **Step 6: Commit**

```bash
jj commit -m "storage: add AnalyticalFinding domain type, remove old finding types

New findings.go defines PassType, AnalyticalFinding, and FindingsBackend.
PassWriter removed; StoreSafetyFlags moved to StageWriter.
CheckViolation removed from ConstitutionBackend.
FindingsBackend added to ScopedBackend composition."
```

---

## Chunk 2: Memgraph Implementation (Removals + New Findings Storage)

### Task 5: Remove old memgraph implementations

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:32-43,293-323` (remove Store* methods + allowlist entries)
- Modify: `internal/storage/memgraph/constitution.go:139-188` (remove CheckViolation)
- Modify: `internal/storage/memgraph/graph.go:114,118` (add HAS_FINDING exclusion)

- [ ] **Step 1: Remove old Store* methods from memgraph/authoring.go**

Delete lines 293-323 (all six `Store*` methods: `StoreRedTeamFindings`, `StorePeripheralVision`, `StoreConsistencyIssues`, `StoreSimplicityFindings`, `StoreConstitutionViolations`). Keep `StoreSafetyFlags` — it's still needed (but it's already on `StageWriter` now via `storeJSONProperty`).

Also remove the old finding property names from the `allowedJSONProperties` map (lines 32-43): delete entries for `"red_team_findings"`, `"peripheral_vision"`, `"consistency_issues"`, `"simplicity_findings"`, `"constitution_violations"`. Keep `"safety_flags"` and all stage output properties.

- [ ] **Step 2: Remove CheckViolation from memgraph/constitution.go**

Delete the `CheckViolation` method (lines 139-188).

- [ ] **Step 3: Add HAS_FINDING to ListEdges exclusion**

In `internal/storage/memgraph/graph.go`, update the unfiltered `ListEdges` queries (lines 112-120) to exclude `HAS_FINDING`:

```cypher
WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE" AND type(r) <> "HAS_FINDING"
```

Apply this to both the outgoing match (line 114) and the incoming match (line 118).

- [ ] **Step 4: Do NOT commit yet**

The build won't compile at this point — server handler still references removed types. Continue to Task 8 to clean up the handler before committing. Tasks 5 and 8 will share a single commit.

### Task 6: Write failing tests for findings storage

**Files:**

- Create: `internal/storage/memgraph/findings_test.go`

- [ ] **Step 1: Write integration tests**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func TestStoreFindings_CreatesNodesAndEdges(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	findings := []storage.AnalyticalFinding{
		{
			Severity:   storage.SeverityWarning,
			Summary:    "Uses forbidden framework",
			Detail:     "The spec references React which conflicts with the Vue.js requirement",
			Constraint: "tech.frameworks",
			Resolution: "Consider Vue.js as the frontend framework",
		},
		{
			Severity:   storage.SeverityCritical,
			Summary:    "Violates data locality principle",
			Detail:     "Spec proposes storing PII in a third-party service",
			Constraint: "principles.data-locality",
		},
	}

	err := testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, findings)
	require.NoError(t, err)

	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// Verify fields are stored correctly.
	require.Equal(t, storage.PassTypeConstitutionCheck, got[0].PassType)
	require.Equal(t, storage.SeverityWarning, got[0].Severity)
	require.Equal(t, "Uses forbidden framework", got[0].Summary)
	require.NotEmpty(t, got[0].ID, "ID should be server-generated")
	require.False(t, got[0].CreatedAt.IsZero(), "CreatedAt should be set")
}

func TestStoreFindings_ReplacesExistingFindings(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	// Store initial findings.
	initial := []storage.AnalyticalFinding{
		{Severity: storage.SeverityWarning, Summary: "old finding"},
	}
	err := testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, initial)
	require.NoError(t, err)

	// Replace with new findings.
	replacement := []storage.AnalyticalFinding{
		{Severity: storage.SeverityCritical, Summary: "new finding 1"},
		{Severity: storage.SeverityNote, Summary: "new finding 2"},
	}
	err = testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, replacement)
	require.NoError(t, err)

	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 2, "should have replaced, not appended")
	require.Equal(t, "new finding 1", got[0].Summary)
}

func TestStoreFindings_DifferentPassTypesAreIndependent(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	constitution := []storage.AnalyticalFinding{
		{Severity: storage.SeverityWarning, Summary: "constitution finding"},
	}
	redTeam := []storage.AnalyticalFinding{
		{Severity: storage.SeverityCritical, Summary: "red team finding"},
	}
	require.NoError(t, testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, constitution))
	require.NoError(t, testStore.StoreFindings(ctx, "test-spec", storage.PassTypeRedTeam, redTeam))

	// List by pass type returns only that type.
	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "constitution finding", got[0].Summary)

	// List all returns both.
	all, err := testStore.ListFindings(ctx, "test-spec", "")
	require.NoError(t, err)
	require.Len(t, all, 2)
}

func TestStoreFindings_RecordsSpecVersion(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	findings := []storage.AnalyticalFinding{
		{Severity: storage.SeverityNote, Summary: "versioned finding"},
	}
	err := testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, findings)
	require.NoError(t, err)

	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, int32(1), got[0].Version, "should record spec version at time of storage")
}

func TestStoreFindings_SpecNotFound(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)

	err := testStore.StoreFindings(ctx, "nonexistent", storage.PassTypeConstitutionCheck, nil)
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListFindings_EmptyResult(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestStoreFindings_EmptySliceDeletesExisting(t *testing.T) {
	clearDatabase(t)
	ctx := testCtx(t)
	createTestProject(t, ctx)
	createTestSpec(t, ctx, "test-spec")

	// Store findings, then replace with empty.
	initial := []storage.AnalyticalFinding{
		{Severity: storage.SeverityWarning, Summary: "will be deleted"},
	}
	require.NoError(t, testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, initial))
	require.NoError(t, testStore.StoreFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck, nil))

	got, err := testStore.ListFindings(ctx, "test-spec", storage.PassTypeConstitutionCheck)
	require.NoError(t, err)
	require.Empty(t, got, "empty store should clear existing findings")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags integration -run TestStoreFindings -v ./internal/storage/memgraph/`
Expected: FAIL — `StoreFindings` and `ListFindings` methods don't exist yet.

- [ ] **Step 3: Commit**

```bash
jj commit -m "test: add integration tests for findings storage

Tests cover: create, replace, filter by pass type, spec version recording,
empty results, empty-slice deletion, and spec-not-found error."
```

### Task 7: Implement findings storage in memgraph

**Files:**

- Create: `internal/storage/memgraph/findings.go`

- [ ] **Step 1: Implement StoreFindings and ListFindings**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// StoreFindings replaces all findings for the given (slug, passType) pair
// within a transaction. Finding nodes are linked to the Spec via HAS_FINDING edges.
func (s *Store) StoreFindings(ctx context.Context, slug string, passType storage.PassType, findings []storage.AnalyticalFinding) error {
	return s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Verify spec exists and get its version.
		spec, err := s.GetSpec(txCtx, slug)
		if err != nil {
			return fmt.Errorf("memgraph: store findings: %w", err)
		}

		// Delete existing findings for this pass type.
		deleteQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:HAS_FINDING]->(f:Finding {pass_type: $pass_type})
			DETACH DELETE f
		`
		deleteParams := mergeParams(s.projectParam(), map[string]any{
			"slug":      slug,
			"pass_type": string(passType),
		})
		if _, err := s.executeQuery(txCtx, deleteQuery, deleteParams); err != nil {
			return fmt.Errorf("memgraph: store findings delete: %w", err)
		}

		// Create new finding nodes.
		nowStr := time.Now().UTC().Format(sortableRFC3339Nano)
		for _, f := range findings {
			id := newID("fn")
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
			createParams := mergeParams(s.projectParam(), map[string]any{
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
			})
			if _, err := s.executeQuery(txCtx, createQuery, createParams); err != nil {
				return fmt.Errorf("memgraph: store finding %q: %w", id, err)
			}
		}
		return nil
	})
}

// ListFindings returns findings for a spec. If passType is empty, returns all findings.
func (s *Store) ListFindings(ctx context.Context, slug string, passType storage.PassType) ([]storage.AnalyticalFinding, error) {
	var query string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if passType != "" {
		query = `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:HAS_FINDING]->(f:Finding {pass_type: $pass_type})
			RETURN f.id AS id, f.pass_type AS pass_type, f.severity AS severity,
			       f.summary AS summary, f.detail AS detail,
			       f.constraint_ref AS constraint_ref, f.resolution AS resolution,
			       f.version AS version, f.created_at AS created_at
			ORDER BY f.created_at
		`
		params["pass_type"] = string(passType)
	} else {
		query = `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:HAS_FINDING]->(f:Finding)
			RETURN f.id AS id, f.pass_type AS pass_type, f.severity AS severity,
			       f.summary AS summary, f.detail AS detail,
			       f.constraint_ref AS constraint_ref, f.resolution AS resolution,
			       f.version AS version, f.created_at AS created_at
			ORDER BY f.created_at
		`
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list findings: %w", err)
	}

	findings := make([]storage.AnalyticalFinding, 0, len(records))
	for _, rec := range records {
		createdAt, _ := time.Parse(sortableRFC3339Nano, stringVal(rec.Values[8]))
		findings = append(findings, storage.AnalyticalFinding{
			ID:         stringVal(rec.Values[0]),
			PassType:   storage.PassType(stringVal(rec.Values[1])),
			Severity:   storage.FindingSeverity(stringVal(rec.Values[2])),
			Summary:    stringVal(rec.Values[3]),
			Detail:     stringVal(rec.Values[4]),
			Constraint: stringVal(rec.Values[5]),
			Resolution: stringVal(rec.Values[6]),
			Version:    int32(intVal(rec.Values[7])),
			CreatedAt:  createdAt,
		})
	}
	return findings, nil
}
```

**Implementation notes:**

- The Cypher uses `constraint_ref` (not `constraint`) as the property name to avoid potential reserved-word conflicts. The domain type maps it to `Constraint`.
- The implementation uses positional indexing (`rec.Values[N]`). Verify this matches the established pattern in `changelog.go` and `graph.go`. If the codebase uses `rec.Get("alias")`, use that instead.
- The `newID("fn")` prefix for findings should be verified against existing prefixes (`"cl"` for changelog). Check there's no collision.

- [ ] **Step 2: Run integration tests**

Run: `go test -tags integration -run TestStoreFindings -v ./internal/storage/memgraph/`
Expected: All tests PASS.

- [ ] **Step 3: Run ListFindings tests**

Run: `go test -tags integration -run TestListFindings -v ./internal/storage/memgraph/`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj commit -m "memgraph: implement StoreFindings and ListFindings

Finding nodes linked via HAS_FINDING edge. StoreFindings uses
RunInTransaction for atomic delete-then-create. ListFindings
supports filtering by pass type."
```

---

## Chunk 3: Handler Changes (Remove Old, Add New)

### Task 8: Clean up authoring handler

**Files:**

- Modify: `internal/server/authoring_handler.go:91-106,175-186,245-257,321-334,750-794`
- Modify: `internal/server/authoring_handler_test.go`

- [ ] **Step 1: Remove `runAnalyticalPasses` function**

Delete the entire `runAnalyticalPasses` function (lines 750-794).

- [ ] **Step 2: Remove analytical pass calls from Spark handler**

In the `Spark` handler (lines 37-106), remove:

- Line 91: `_, _, _, _, constitutionViolations, passErr := runAnalyticalPasses(...)` and the `passErr` check (lines 91-94)
- Line 103: `ConstitutionViolations: constitutionViolations,` from the response

The Spark response becomes:

```go
return connect.NewResponse(&specv1.SparkResponse{
	Output:      msg.Output,
	SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
	NextPrompts: authoring.PromptsToProto(authoring.StageShape),
}), nil
```

- [ ] **Step 3: Remove analytical pass calls from Shape handler**

In the `Shape` handler (lines 108-187), remove:

- Lines 175-179: `runAnalyticalPasses` call and error check
- Line 183: `PeripheralVision: peripheralVision,` from the response

The Shape response becomes:

```go
return connect.NewResponse(&specv1.ShapeResponse{
	Output:      msg.Output,
	SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
	NextPrompts: authoring.PromptsToProto(authoring.StageSpecify),
}), nil
```

- [ ] **Step 4: Remove analytical pass calls from Specify handler**

In the `Specify` handler (lines 189-258), remove:

- Lines 245-249: `runAnalyticalPasses` call and error check
- Lines 253-254: `RedTeam` and `ConsistencyIssues` from the response

The Specify response becomes:

```go
return connect.NewResponse(&specv1.SpecifyResponse{
	Output:      msg.Output,
	SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
	NextPrompts: authoring.PromptsToProto(authoring.StageDecompose),
}), nil
```

- [ ] **Step 5: Remove analytical pass calls from Decompose handler**

In the `Decompose` handler (lines 260-334), remove:

- Lines 321-324: `runAnalyticalPasses` call and error check
- Line 329: `Simplicity: simplicity,` from the response

The Decompose response becomes:

```go
return connect.NewResponse(&specv1.DecomposeResponse{
	Output:         msg.Output,
	SafetyFlags:    authoring.SafetyResultsToProto(safetyFlags),
	NextPrompts:    authoring.PromptsToProto(authoring.StageApproved),
	ChildSpecSlugs: childSlugs,
}), nil
```

- [ ] **Step 6: Update authoring handler tests**

In `internal/server/authoring_handler_test.go`:

- Remove `TestAuthoringHandler_Spark_ConstitutionViolationsReturned` (line 991)
- Remove `TestAuthoringHandler_Spark_ConstitutionViolations_UnspecifiedPosture` (line 1003)
- Remove `StoreConstitutionViolations` from `fakeAuthoringBackend` (line 88) and `authoringTestBackend` (line 232)
- Remove `storeConstitutionViolationsErr` field from fake (line 38)
- Update any other test assertions that reference removed response fields

- [ ] **Step 7: Remove `CheckViolation` handler from constitution handler**

In `internal/server/constitution_handler.go`, delete the `CheckViolation` method (lines 66-87). The `ConstitutionHandler` still satisfies the interface because the proto service no longer has `CheckViolation`.

- [ ] **Step 8: Remove `constitutionCheckCmd` from CLI**

In `cmd/specgraph/constitution.go`:

- Delete the `constitutionCheckCmd` variable and `runConstitutionCheck` function (lines 86-114)
- Remove the `constitutionCheckCmd` registration at line 326: `constitutionCmd.AddCommand(constitutionCheckCmd)`

- [ ] **Step 9: Remove converter functions for old violation types**

Search for `violationsToProto`, `constitutionViolationToProto`, and any proto↔domain converter functions for the removed types in `internal/server/convert.go` or the handler files. Delete them.

- [ ] **Step 10: Verify the build compiles**

Run: `go build ./...`
Expected: Compiles. All references to removed types should be gone.

- [ ] **Step 11: Run existing tests**

Run: `task check`
Expected: PASS (or only pre-existing failures unrelated to this change).

- [ ] **Step 12: Commit (covers Tasks 5 + 8)**

```bash
jj commit -m "server+memgraph: remove old analytical pass system

Remove runAnalyticalPasses and finding fields from stage responses.
Remove 5 Store* methods, CheckViolation handler+CLI+memgraph impl.
Add HAS_FINDING to ListEdges exclusion list.
Stage handlers no longer produce or return findings inline."
```

### Task 9: Create prompt template for constitution_check

**Files:**

- Create: `internal/server/templates/constitution_check.md`

- [ ] **Step 1: Create the templates directory and prompt file**

```markdown
# Constitution Compliance Reviewer

## Who You Are

You are a constitution compliance analyst for SpecGraph. Your role is to evaluate whether a specification aligns with the project's constitution — the layered ground truth that defines technology choices, principles, constraints, processes, and antipatterns.

You are thorough but fair. You consider exceptions and context. You flag genuine tensions, not theoretical ones. You distinguish between hard violations (breaking an explicit constraint) and soft tensions (bending a principle).

## Your Task

Evaluate the spec for compliance with the project constitution. Use the available tools to read both the spec and the constitution. Assess every section of the constitution against the spec content.

## Available Information

Use these tools to gather the information you need:

| Tool | What It Provides |
|------|-----------------|
| show_spec | The spec's full content: slug, intent, stage, and all stage outputs (spark, shape, specify, decompose) |
| show_constitution | The full project constitution: tech stack, principles, constraints, antipatterns, process, references |
| list_deps | Slugs of specs this one depends on |
| show_dep | Full content of a specific dependency (for cross-spec context) |

Start by reading both the spec and the constitution. Then systematically work through each constitution section.

## Evaluation Framework

For each section of the constitution, assess the spec:

1. **Tech Stack** — Does the spec align with primary language, allowed languages, frameworks, infrastructure, API standards, and data technologies? Does it reference or imply any forbidden technologies?
2. **Principles** — Does the spec respect each stated principle? Consider the rationale and exceptions. A principle with exceptions is not an absolute rule.
3. **Constraints** — Does the spec violate any explicit constraints? These are the hardest rules — violations here are typically critical.
4. **Antipatterns** — Does the spec's approach match any documented antipatterns? Reference the "instead" guidance.
5. **Process** — Does the spec meet process requirements appropriate for its current stage? (e.g., security review triggers, documentation requirements)

## Severity Guidelines

- **critical** — Direct violation of an explicit constraint or forbidden technology. Blocks spec advancement.
- **warning** — Tension with a principle, borderline antipattern match, or missing process step. Should be addressed but doesn't necessarily block.
- **note** — Worth flagging for awareness. Might become an issue if not considered. Does not block.

## Output Format

Return your findings as a JSON array. Each finding has these fields:

```json
[
  {
    "severity": "critical|warning|note",
    "summary": "One-line description of the finding",
    "detail": "Full explanation with context and reasoning",
    "constraint": "Which constitution section/rule was evaluated (e.g. 'principles.composition-over-inheritance')",
    "resolution": "What the spec author should consider changing"
  }
]
```

If the spec fully complies with the constitution, return an empty array: `[]`

## Important

- Read the actual constitution — don't assume what it contains.
- Consider the spec's current stage. A Spark-stage spec has only a seed and signal; don't flag missing details that come in later stages.
- When a principle has documented exceptions, check whether the spec falls within an exception before flagging a violation.
- Be specific in your findings. Reference the exact constitution section and the exact part of the spec that conflicts.

```text

- [ ] **Step 2: Commit**

```bash
jj commit -m "templates: add constitution_check prompt template

Markdown persona for the LLM-driven constitution compliance pass.
Covers evaluation framework, severity guidelines, and output format."
```

### Task 10: Implement AnalyticalPassHandler

**Files:**

- Create: `internal/server/analytical_pass_handler.go`

- [ ] **Step 1: Write the handler**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/specgraph/specgraph/internal/storage"
)

//go:embed templates/*.md
var templateFS embed.FS

// AnalyticalPassHandler implements the ConnectRPC AnalyticalPassService.
type AnalyticalPassHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.AnalyticalPassServiceHandler = (*AnalyticalPassHandler)(nil)

// RegisterAnalyticalPassService registers the AnalyticalPassService on the given mux.
func RegisterAnalyticalPassService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	if scoper == nil {
		panic("RegisterAnalyticalPassService: scoper must not be nil")
	}
	handler := &AnalyticalPassHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewAnalyticalPassServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// passToolManifest returns the CLI tools available to the LLM during a pass.
func passToolManifest(slug string) []*specv1.ToolReference {
	return []*specv1.ToolReference{
		{
			Name:        "show_spec",
			Command:     fmt.Sprintf("specgraph show %s --json", slug),
			Description: "Read the spec's full content including all stage outputs",
		},
		{
			Name:        "show_constitution",
			Command:     "specgraph constitution show --json",
			Description: "Read the full project constitution",
		},
		{
			Name:        "list_deps",
			Command:     fmt.Sprintf("specgraph edges %s --type DEPENDS_ON --json", slug),
			Description: "List specs this one depends on",
		},
		{
			Name:        "show_dep",
			Command:     "specgraph show {slug} --json",
			Description: "Read a specific dependency's full content (replace {slug} with the dependency slug)",
		},
	}
}

// loadTemplate reads a prompt template from the embedded filesystem.
func loadTemplate(passType storage.PassType) (string, error) {
	filename := fmt.Sprintf("templates/%s.md", string(passType))
	data, err := templateFS.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("template %q not found: %w", filename, err)
	}
	return string(data), nil
}

// RunAnalyticalPass returns the prompt template, tool manifest, and initial
// message needed to execute an analytical pass against a spec.
func (h *AnalyticalPassHandler) RunAnalyticalPass(ctx context.Context, req *connect.Request[specv1.RunAnalyticalPassRequest]) (*connect.Response[specv1.RunAnalyticalPassResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	passType := storage.PassType(msg.PassName)
	if !storage.ValidPassType(passType) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass type: %q", msg.PassName))
	}

	// Verify spec exists and get its stage.
	spec, err := store.GetSpec(ctx, msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("spec %q not found", msg.Slug))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get spec: %w", err))
	}

	// Load prompt template.
	template, err := loadTemplate(passType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("load template: %w", err))
	}

	// Determine offered passes (available but not auto-running).
	// NOTE: Defaults to PostureDrive. RunAnalyticalPassRequest does not
	// include posture; 0.2.0 limitation — add posture to request if needed.
	stage := authoring.Stage(spec.Stage)
	offered := authoring.OfferedPasses(stage, authoring.PostureDrive)

	return connect.NewResponse(&specv1.RunAnalyticalPassResponse{
		PassName:       msg.PassName,
		PromptTemplate: template,
		Tools:          passToolManifest(msg.Slug),
		InitialMessage: fmt.Sprintf("Evaluate spec `%s` for %s compliance.", msg.Slug, strings.ReplaceAll(msg.PassName, "_", " ")),
		OfferedPasses:  offered,
		Stage:          string(spec.Stage),
	}), nil
}

// StoreFindings persists findings produced by an analytical pass.
func (h *AnalyticalPassHandler) StoreFindings(ctx context.Context, req *connect.Request[specv1.StoreFindingsRequest]) (*connect.Response[specv1.StoreFindingsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	passType := storage.PassType(msg.PassType)
	if !storage.ValidPassType(passType) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass type: %q", msg.PassType))
	}

	findings := make([]storage.AnalyticalFinding, len(msg.Findings))
	for i, f := range msg.Findings {
		findings[i] = storage.AnalyticalFinding{
			Severity:   findingSeverityFromProto(f.Severity),
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
		}
	}

	if err := store.StoreFindings(ctx, msg.Slug, passType, findings); err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("spec %q not found", msg.Slug))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store findings: %w", err))
	}

	// Read back stored findings to return generated IDs.
	stored, err := store.ListFindings(ctx, msg.Slug, passType)
	if err != nil {
		h.logger.Error("store findings: read-back failed", slog.Any("error", err))
		return connect.NewResponse(&specv1.StoreFindingsResponse{}), nil
	}
	ids := make([]string, len(stored))
	for i, f := range stored {
		ids[i] = f.ID
	}
	return connect.NewResponse(&specv1.StoreFindingsResponse{Ids: ids}), nil
}

// ListFindings retrieves stored findings for a spec.
func (h *AnalyticalPassHandler) ListFindings(ctx context.Context, req *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	passType := storage.PassType(msg.PassType)
	if msg.PassType != "" && !storage.ValidPassType(passType) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass type: %q", msg.PassType))
	}

	findings, err := store.ListFindings(ctx, msg.Slug, passType)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list findings: %w", err))
	}

	protoFindings := make([]*specv1.AnalyticalFinding, len(findings))
	for i, f := range findings {
		protoFindings[i] = &specv1.AnalyticalFinding{
			Id:         f.ID,
			PassType:   string(f.PassType),
			Severity:   findingSeverityToProto(f.Severity),
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
			Version:    f.Version,
		}
	}
	return connect.NewResponse(&specv1.ListFindingsResponse{Findings: protoFindings}), nil
}

// findingSeverityFromProto converts proto FindingSeverity to domain FindingSeverity.
func findingSeverityFromProto(s specv1.FindingSeverity) storage.FindingSeverity {
	switch s {
	case specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL:
		return storage.SeverityCritical
	case specv1.FindingSeverity_FINDING_SEVERITY_WARNING:
		return storage.SeverityWarning
	case specv1.FindingSeverity_FINDING_SEVERITY_NOTE:
		return storage.SeverityNote
	default:
		return storage.SeverityNote
	}
}

// findingSeverityToProto converts domain FindingSeverity to proto FindingSeverity.
func findingSeverityToProto(s storage.FindingSeverity) specv1.FindingSeverity {
	switch s {
	case storage.SeverityCritical:
		return specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL
	case storage.SeverityWarning:
		return specv1.FindingSeverity_FINDING_SEVERITY_WARNING
	case storage.SeverityNote:
		return specv1.FindingSeverity_FINDING_SEVERITY_NOTE
	default:
		return specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
	}
}
```

Note: Check if `findingSeverityFromProto`/`findingSeverityToProto` already exist in `convert.go` or elsewhere. If so, reuse them instead of duplicating. The existing codebase may have `safetyFlagSeverityToProto` or similar helpers — follow the same pattern.

- [ ] **Step 2: Register the service in serve.go**

In `cmd/specgraph/serve.go`, after line 94 (`RegisterAuthoringService`), add:

```go
server.RegisterAnalyticalPassService(mux, store, opts)
```

- [ ] **Step 3: Verify the build compiles**

Run: `go build ./...`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
jj commit -m "server: add AnalyticalPassHandler with RunAnalyticalPass, StoreFindings, ListFindings

New AnalyticalPassService registered in serve.go. Embedded
constitution_check.md prompt template. Tool manifest provides
spec/constitution/dependency CLI commands for agent LLM tooling."
```

### Task 11: Write handler unit tests

**Files:**

- Create: `internal/server/analytical_pass_handler_test.go`

- [ ] **Step 1: Write unit tests**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

func setupAnalyticalPassServer(t *testing.T, backend storage.ScopedBackend) specgraphv1connect.AnalyticalPassServiceClient {
	t.Helper()
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterAnalyticalPassService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewAnalyticalPassServiceClient(http.DefaultClient, srv.URL)
}

func TestRunAnalyticalPass_ReturnsPromptAndTools(t *testing.T) {
	backend := newAuthoringTestBackend(t)
	sparkSpec(t, backend, "test-spec")
	client := setupAnalyticalPassServer(t, backend)

	resp, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "test-spec",
		PassName: "constitution_check",
	}))
	require.NoError(t, err)
	require.Equal(t, "constitution_check", resp.Msg.PassName)
	require.Contains(t, resp.Msg.PromptTemplate, "Constitution Compliance Reviewer")
	require.NotEmpty(t, resp.Msg.Tools)
	require.NotEmpty(t, resp.Msg.InitialMessage)
	require.Contains(t, resp.Msg.InitialMessage, "test-spec")
}

func TestRunAnalyticalPass_UnknownPassType(t *testing.T) {
	backend := newAuthoringTestBackend(t)
	sparkSpec(t, backend, "test-spec")
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "test-spec",
		PassName: "nonexistent_pass",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRunAnalyticalPass_SpecNotFound(t *testing.T) {
	backend := newAuthoringTestBackend(t)
	client := setupAnalyticalPassServer(t, backend)

	_, err := client.RunAnalyticalPass(context.Background(), connect.NewRequest(&specv1.RunAnalyticalPassRequest{
		Slug:     "nonexistent",
		PassName: "constitution_check",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestStoreAndListFindings(t *testing.T) {
	backend := newAuthoringTestBackend(t)
	sparkSpec(t, backend, "test-spec")
	client := setupAnalyticalPassServer(t, backend)

	storeResp, err := client.StoreFindings(context.Background(), connect.NewRequest(&specv1.StoreFindingsRequest{
		Slug:     "test-spec",
		PassType: "constitution_check",
		Findings: []*specv1.AnalyticalFindingInput{
			{
				Severity:   specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
				Summary:    "test finding",
				Detail:     "test detail",
				Constraint: "test constraint",
			},
		},
	}))
	require.NoError(t, err)
	require.Len(t, storeResp.Msg.Ids, 1)

	listResp, err := client.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{
		Slug:     "test-spec",
		PassType: "constitution_check",
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Findings, 1)
	require.Equal(t, "test finding", listResp.Msg.Findings[0].Summary)
}
```

**IMPORTANT:** The test snippets above use placeholder helpers (`newAuthoringTestBackend`, `sparkSpec`). These do NOT exist in the codebase. Before writing tests, study the existing pattern in `authoring_handler_test.go`:

- `fakeAuthoringBackend` (line 23) and `authoringTestBackend` (line 220) are the real patterns
- `testScoper` and `wrapTestProject` exist in `test_scoper_test.go`
- The test backend must implement `FindingsBackend` (with `StoreFindings`/`ListFindings` methods) since `ScopedBackend` requires it
- Rewrite the test setup to follow the existing `fakeBackend` + `stubBackend` pattern

- [ ] **Step 2: Run the tests**

Run: `go test -run TestRunAnalyticalPass -v ./internal/server/`
Expected: PASS.

Run: `go test -run TestStoreAndListFindings -v ./internal/server/`
Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run: `task check`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj commit -m "test: add AnalyticalPassHandler unit tests

Cover RunAnalyticalPass (success, unknown pass, spec not found),
StoreFindings, and ListFindings round-trip."
```

---

## Chunk 4: Cleanup and Verification

### Task 12: Remove stale converter functions and update test fakes

**Files:**

- Modify: `internal/server/convert.go` (if it has violation converters)
- Modify: `internal/server/authoring_handler_test.go` (update fakeBackend to implement FindingsBackend)
- Modify: `internal/server/test_scoper_test.go` (if stubBackend needs FindingsBackend)

- [ ] **Step 1: Find and remove stale converter functions**

Search for converter functions related to the removed types:

```bash
rg "violationsToProto|constitutionViolation|peripheralVision.*Proto|redTeam.*Proto|consistency.*Proto|simplicity.*Proto" internal/server/ --type go
```

Delete any functions that convert the old finding types to/from proto.

- [ ] **Step 2: Update test stub backends**

The `stubBackend` in `internal/server/test_scoper_test.go` needs `StoreFindings` and `ListFindings` stub methods to satisfy `ScopedBackend`. Add no-op implementations:

```go
func (stubBackend) StoreFindings(context.Context, string, storage.PassType, []storage.AnalyticalFinding) error {
	return nil
}
func (stubBackend) ListFindings(context.Context, string, storage.PassType) ([]storage.AnalyticalFinding, error) {
	return nil, nil
}
```

Remove stubs for the old methods: `StoreConstitutionViolations`, `StoreRedTeamFindings`, `StorePeripheralVision`, `StoreConsistencyIssues`, `StoreSimplicityFindings`, `CheckViolation`.

- [ ] **Step 3: Update the memgraph test helper if needed**

If `internal/storage/memgraph/authoring_test.go` has tests for the old `Store*` methods, remove them.

- [ ] **Step 4: Remove constitution test for CheckViolation**

If `internal/storage/memgraph/constitution_test.go` has tests for `CheckViolation`, remove them.

- [ ] **Step 5: Full build and test**

Run: `task check`
Expected: PASS — all code compiles, all tests pass.

- [ ] **Step 6: Run integration tests**

Run: `task pr-prep`
Expected: PASS (includes integration and e2e tests).

- [ ] **Step 7: Commit**

```bash
jj commit -m "cleanup: remove stale converters and update test fakes for FindingsBackend

Update all test backends to implement FindingsBackend.
Remove old finding type converters and CheckViolation test stubs."
```

### Task 13: Update bead and documentation

- [ ] **Step 1: Update the design doc status**

Change `**Status:** Draft` to `**Status:** Implemented` in `docs/superpowers/specs/2026-03-20-analytical-pass-system-design.md`.

- [ ] **Step 2: Update CLAUDE.md**

Add a note to the Gotchas section in `CLAUDE.md`:

```markdown
- **Analytical findings are graph nodes** — `HAS_FINDING` edge (Spec → Finding) is internal, like `HAS_CHANGE`. Not exposed via `AddEdge`/`RemoveEdge`. Pass-specific findings stored via `StoreFindings` RPC, not inline in stage responses.
```

- [ ] **Step 3: Close the bead**

```bash
bd close spgr-5pq --reason="Analytical pass infrastructure implemented: AnalyticalPassService with RunAnalyticalPass/StoreFindings/ListFindings RPCs, unified AnalyticalFinding type, embedded constitution_check prompt template. Old CheckViolation and five separate finding types removed."
```

- [ ] **Step 4: Final commit**

```bash
jj commit -m "docs: mark analytical pass design as implemented, update CLAUDE.md"
```
