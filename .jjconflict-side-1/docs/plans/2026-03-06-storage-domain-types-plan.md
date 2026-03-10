# Storage Domain Types & Decision Promotion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove proto type coupling from Backend, DecisionBackend, and GraphBackend interfaces; promote ShapeOutput decisions to first-class Decision graph nodes with DECIDED_IN edges.

**Architecture:** Create domain types in `internal/storage/`, update Memgraph implementations to return domain types, add proto<->domain converters in handler layer. Follow the pattern already established by AuthoringBackend (which uses `storage.SparkOutput` etc.) and its converters in `authoring_handler.go` (`sparkOutputToDomain`, `shapeOutputToDomain`, etc.).

**Tech Stack:** Go 1.26, ConnectRPC, Memgraph (neo4j-go-driver v5), buf (proto codegen)

**Design Doc:** `docs/plans/2026-03-06-storage-domain-types-design.md`

---

## Task 1: Domain Spec Type

**Files:**

- Create: `internal/storage/spec_domain.go`
- Test: `internal/storage/spec_domain_test.go`

**Step 1: Write the domain type and constructor test**

`internal/storage/spec_domain_test.go`:

```go
package storage_test

import (
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewSpec(t *testing.T) {
	now := time.Now().UTC()
	spec := &storage.Spec{
		ID:         "spec-abc1234",
		Slug:       "login-api",
		Intent:     "REST endpoint for OAuth2",
		Stage:      "spark",
		Priority:   "p1",
		Complexity: "medium",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	assert.Equal(t, "login-api", spec.Slug)
	assert.Equal(t, int32(1), spec.Version)
	assert.False(t, spec.CreatedAt.IsZero())
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/storage/ -run TestNewSpec -v
```

Expected: FAIL — `storage.Spec` type doesn't exist

**Step 3: Write the domain type**

`internal/storage/spec_domain.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// Spec is the storage-layer domain type for specifications.
// Handlers convert between this type and the proto Spec message.
type Spec struct {
	ID         string
	Slug       string
	Intent     string
	Stage      string
	Priority   string
	Complexity string
	Version    int32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/storage/ -run TestNewSpec -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/spec_domain.go internal/storage/spec_domain_test.go
git commit -m "feat(storage): add domain Spec type"
```

---

## Task 2: Domain Decision and Edge Types

**Files:**

- Modify: `internal/storage/decision.go` — add domain types, keep interface using specv1 for now
- Modify: `internal/storage/graph.go` — add domain types, keep interface using specv1 for now

**Step 1: Add domain Decision type and DecisionStatus to `decision.go`**

Add below the existing error variables in `internal/storage/decision.go`:

```go
import "time"

// DecisionStatus represents the lifecycle state of a decision.
type DecisionStatus string

const (
	DecisionStatusProposed   DecisionStatus = "DECISION_STATUS_PROPOSED"
	DecisionStatusAccepted   DecisionStatus = "DECISION_STATUS_ACCEPTED"
	DecisionStatusSuperseded DecisionStatus = "DECISION_STATUS_SUPERSEDED"
	DecisionStatusDeprecated DecisionStatus = "DECISION_STATUS_DEPRECATED"
)

// Decision is the storage-layer domain type for architectural decisions.
type Decision struct {
	ID           string
	Slug         string
	Title        string
	Status       DecisionStatus
	Decision     string
	Rationale    string
	SupersededBy string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

**Step 2: Add domain Edge type and EdgeType to `graph.go`**

Add below NodeRef in `internal/storage/graph.go`:

```go
// EdgeType represents the kind of relationship between nodes.
type EdgeType string

const (
	EdgeTypeDependsOn EdgeType = "DEPENDS_ON"
	EdgeTypeBlocks    EdgeType = "BLOCKS"
	EdgeTypeComposes  EdgeType = "COMPOSES"
	EdgeTypeRelatesTo EdgeType = "RELATES_TO"
	EdgeTypeInforms   EdgeType = "INFORMS"
	EdgeTypeDecidedIn EdgeType = "DECIDED_IN"
)

// Edge represents a typed relationship between two graph nodes.
type Edge struct {
	FromID   string
	ToID     string
	EdgeType EdgeType
}
```

**Step 3: Verify it compiles**

```bash
go build ./internal/storage/
```

Expected: compiles cleanly

**Step 4: Commit**

```bash
git add internal/storage/decision.go internal/storage/graph.go
git commit -m "feat(storage): add domain Decision, Edge, and EdgeType types"
```

---

## Task 3: Proto Converters for Spec and Decision

**Files:**

- Create: `internal/server/convert.go`
- Create: `internal/server/convert_test.go`

**Step 1: Write converter tests**

`internal/server/convert_test.go`:

```go
package server

import (
	"testing"
	"time"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	spec := &storage.Spec{
		ID: "spec-abc", Slug: "login", Intent: "Login API",
		Stage: "spark", Priority: "p1", Complexity: "medium",
		Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	pb := specToProto(spec)
	assert.Equal(t, "spec-abc", pb.Id)
	assert.Equal(t, "login", pb.Slug)
	assert.Equal(t, int32(1), pb.Version)
	require.NotNil(t, pb.CreatedAt)
	assert.Equal(t, now.Unix(), pb.CreatedAt.AsTime().Unix())
}

func TestSpecsToProto(t *testing.T) {
	specs := []*storage.Spec{
		{ID: "a", Slug: "a"},
		{ID: "b", Slug: "b"},
	}
	pbs := specsToProto(specs)
	assert.Len(t, pbs, 2)
	assert.Equal(t, "a", pbs[0].Id)
}

func TestDecisionToProto(t *testing.T) {
	now := time.Date(2026, 3, 6, 12, 0, 0, 0, time.UTC)
	d := &storage.Decision{
		ID: "dec-abc", Slug: "use-memgraph", Title: "Use Memgraph",
		Status: storage.DecisionStatusAccepted, Decision: "We chose Memgraph",
		Rationale: "Graph-native", CreatedAt: now, UpdatedAt: now,
	}
	pb := decisionToProto(d)
	assert.Equal(t, "dec-abc", pb.Id)
	assert.Equal(t, "use-memgraph", pb.Slug)
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, pb.Status)
}

func TestDecisionStatusToProto(t *testing.T) {
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_PROPOSED, decisionStatusToProto(storage.DecisionStatusProposed))
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_ACCEPTED, decisionStatusToProto(storage.DecisionStatusAccepted))
	assert.Equal(t, specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED, decisionStatusToProto(storage.DecisionStatusSuperseded))
}

func TestDecisionStatusFromProto(t *testing.T) {
	assert.Equal(t, storage.DecisionStatusProposed, decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
	assert.Equal(t, storage.DecisionStatusAccepted, decisionStatusFromProto(specv1.DecisionStatus_DECISION_STATUS_ACCEPTED))
}

func TestEdgeToProto(t *testing.T) {
	e := &storage.Edge{FromID: "a", ToID: "b", EdgeType: storage.EdgeTypeDependsOn}
	pb := edgeToProto(e)
	assert.Equal(t, "a", pb.FromId)
	assert.Equal(t, specv1.EdgeType_EDGE_TYPE_DEPENDS_ON, pb.EdgeType)
}

func TestEdgeTypeFromProto(t *testing.T) {
	assert.Equal(t, storage.EdgeTypeDependsOn, edgeTypeFromProto(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
	assert.Equal(t, storage.EdgeTypeComposes, edgeTypeFromProto(specv1.EdgeType_EDGE_TYPE_COMPOSES))
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/server/ -run TestSpecToProto -v
```

Expected: FAIL — converter functions don't exist

**Step 3: Implement converters**

`internal/server/convert.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Spec ---

func specToProto(s *storage.Spec) *specv1.Spec {
	return &specv1.Spec{
		Id:         s.ID,
		Slug:       s.Slug,
		Intent:     s.Intent,
		Stage:      s.Stage,
		Priority:   s.Priority,
		Complexity: s.Complexity,
		Version:    s.Version,
		CreatedAt:  timestamppb.New(s.CreatedAt),
		UpdatedAt:  timestamppb.New(s.UpdatedAt),
	}
}

func specsToProto(specs []*storage.Spec) []*specv1.Spec {
	result := make([]*specv1.Spec, len(specs))
	for i, s := range specs {
		result[i] = specToProto(s)
	}
	return result
}

// --- Decision ---

var decisionStatusToProtoMap = map[storage.DecisionStatus]specv1.DecisionStatus{
	storage.DecisionStatusProposed:   specv1.DecisionStatus_DECISION_STATUS_PROPOSED,
	storage.DecisionStatusAccepted:   specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
	storage.DecisionStatusSuperseded: specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
	storage.DecisionStatusDeprecated: specv1.DecisionStatus_DECISION_STATUS_DEPRECATED,
}

var decisionStatusFromProtoMap = map[specv1.DecisionStatus]storage.DecisionStatus{
	specv1.DecisionStatus_DECISION_STATUS_PROPOSED:   storage.DecisionStatusProposed,
	specv1.DecisionStatus_DECISION_STATUS_ACCEPTED:   storage.DecisionStatusAccepted,
	specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED: storage.DecisionStatusSuperseded,
	specv1.DecisionStatus_DECISION_STATUS_DEPRECATED: storage.DecisionStatusDeprecated,
}

func decisionStatusToProto(s storage.DecisionStatus) specv1.DecisionStatus {
	if v, ok := decisionStatusToProtoMap[s]; ok {
		return v
	}
	return specv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED
}

func decisionStatusFromProto(s specv1.DecisionStatus) storage.DecisionStatus {
	if v, ok := decisionStatusFromProtoMap[s]; ok {
		return v
	}
	return storage.DecisionStatusProposed
}

func decisionToProto(d *storage.Decision) *specv1.Decision {
	return &specv1.Decision{
		Id:           d.ID,
		Slug:         d.Slug,
		Title:        d.Title,
		Status:       decisionStatusToProto(d.Status),
		Decision:     d.Decision,
		Rationale:    d.Rationale,
		SupersededBy: d.SupersededBy,
		CreatedAt:    timestamppb.New(d.CreatedAt),
		UpdatedAt:    timestamppb.New(d.UpdatedAt),
	}
}

func decisionsToProto(decisions []*storage.Decision) []*specv1.Decision {
	result := make([]*specv1.Decision, len(decisions))
	for i, d := range decisions {
		result[i] = decisionToProto(d)
	}
	return result
}

// --- Edge ---

var edgeTypeToProtoMap = map[storage.EdgeType]specv1.EdgeType{
	storage.EdgeTypeDependsOn: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
	storage.EdgeTypeBlocks:    specv1.EdgeType_EDGE_TYPE_BLOCKS,
	storage.EdgeTypeComposes:  specv1.EdgeType_EDGE_TYPE_COMPOSES,
	storage.EdgeTypeRelatesTo: specv1.EdgeType_EDGE_TYPE_RELATES_TO,
	storage.EdgeTypeInforms:   specv1.EdgeType_EDGE_TYPE_INFORMS,
}

var edgeTypeFromProtoMap = map[specv1.EdgeType]storage.EdgeType{
	specv1.EdgeType_EDGE_TYPE_DEPENDS_ON: storage.EdgeTypeDependsOn,
	specv1.EdgeType_EDGE_TYPE_BLOCKS:     storage.EdgeTypeBlocks,
	specv1.EdgeType_EDGE_TYPE_COMPOSES:   storage.EdgeTypeComposes,
	specv1.EdgeType_EDGE_TYPE_RELATES_TO: storage.EdgeTypeRelatesTo,
	specv1.EdgeType_EDGE_TYPE_INFORMS:    storage.EdgeTypeInforms,
}

func edgeTypeToProto(e storage.EdgeType) specv1.EdgeType {
	if v, ok := edgeTypeToProtoMap[e]; ok {
		return v
	}
	return specv1.EdgeType_EDGE_TYPE_UNSPECIFIED
}

func edgeTypeFromProto(e specv1.EdgeType) storage.EdgeType {
	if v, ok := edgeTypeFromProtoMap[e]; ok {
		return v
	}
	return storage.EdgeTypeDependsOn
}

func edgeToProto(e *storage.Edge) *specv1.Edge {
	return &specv1.Edge{
		FromId:   e.FromID,
		ToId:     e.ToID,
		EdgeType: edgeTypeToProto(e.EdgeType),
	}
}

func edgesToProto(edges []*storage.Edge) []*specv1.Edge {
	result := make([]*specv1.Edge, len(edges))
	for i, e := range edges {
		result[i] = edgeToProto(e)
	}
	return result
}
```

**Step 4: Run all converter tests**

```bash
go test ./internal/server/ -run "TestSpecToProto|TestSpecsToProto|TestDecisionToProto|TestDecisionStatus|TestEdgeToProto|TestEdgeType" -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/convert.go internal/server/convert_test.go
git commit -m "feat(server): add proto<->domain converters for Spec, Decision, Edge"
```

---

## Task 4: Swap Backend Interface to Domain Types

**Files:**

- Modify: `internal/storage/storage.go` — change return types
- Modify: `internal/storage/memgraph/memgraph.go` — change `recordToSpec` and method signatures
- Modify: `internal/server/spec_handler.go` — add converter calls

**Step 1: Update `storage.go` interface**

Remove the `specv1` import. Change all `*specv1.Spec` to `*Spec`:

```go
package storage

import "context"

type Backend interface {
	CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*Spec, error)
	GetSpec(ctx context.Context, slug string) (*Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*Spec, error)
	UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity *string) (*Spec, error)
	Close(ctx context.Context) error
}
```

**Step 2: Update `memgraph/memgraph.go`**

Change `recordToSpec` to return `*storage.Spec` instead of `*specv1.Spec`. Remove `timestamppb` usage — return `time.Time` directly:

```go
func recordToSpec(rec *neo4j.Record) (*storage.Spec, error) {
	// ... same field extraction as before ...
	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}
	return &storage.Spec{
		ID:         id,
		Slug:       slug,
		Intent:     intent,
		Stage:      stage,
		Priority:   priority,
		Complexity: complexity,
		Version:    safeInt32(version),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}, nil
}
```

Update all method signatures: `CreateSpec`, `GetSpec`, `ListSpecs`, `UpdateSpec` to return `*storage.Spec` / `[]*storage.Spec`.

Remove `specv1` and `timestamppb` imports from `memgraph.go` if no longer used.

**Step 3: Update `spec_handler.go`**

Add converter calls. For example:

```go
func (h *SpecHandler) CreateSpec(ctx context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.Spec], error) {
	// ... validation same as before ...
	spec, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Intent, priority, complexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(specToProto(spec)), nil
}
```

Apply the same pattern to `GetSpec`, `ListSpecs` (use `specsToProto`), and `UpdateSpec`.

**Step 4: Fix compilation errors**

The authoring handler also calls `h.backend.CreateSpec` — its return type reference needs updating. Check all callers:

```bash
rg "\.CreateSpec\|\.GetSpec\|\.ListSpecs\|\.UpdateSpec" internal/server/ internal/storage/
```

Update each to work with `*storage.Spec`.

**Step 5: Run tests**

```bash
go test ./internal/storage/ ./internal/server/ -v -count=1
```

Expected: PASS (unit tests). Integration tests need Docker so verify with `go build ./...` for compilation.

**Step 6: Commit**

```bash
git add internal/storage/storage.go internal/storage/memgraph/memgraph.go internal/server/spec_handler.go internal/server/authoring_handler.go
git commit -m "refactor(storage): swap Backend interface to domain Spec type"
```

---

## Task 5: Swap DecisionBackend Interface to Domain Types

**Files:**

- Modify: `internal/storage/decision.go` — change interface signatures
- Modify: `internal/storage/memgraph/decision.go` — change `recordToDecision` and method signatures
- Modify: `internal/server/decision_handler.go` — add converter calls

**Step 1: Update `decision.go` interface**

Remove `specv1` import. Change interface:

```go
type DecisionBackend interface {
	CreateDecision(ctx context.Context, slug, title, decision, rationale string) (*Decision, error)
	GetDecision(ctx context.Context, slug string) (*Decision, error)
	ListDecisions(ctx context.Context, status DecisionStatus, limit int) ([]*Decision, error)
	UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus, decision, rationale, supersededBy *string) (*Decision, error)
}
```

**Step 2: Update `memgraph/decision.go`**

Change `recordToDecision` to return `*storage.Decision`. Parse status string into `storage.DecisionStatus` instead of `specv1.DecisionStatus`:

```go
func recordToDecision(rec *neo4j.Record) (*storage.Decision, error) {
	// ... same field extraction ...
	return &storage.Decision{
		ID:           id,
		Slug:         slug,
		Title:        title,
		Status:       storage.DecisionStatus(statusStr),
		Decision:     decisionText,
		Rationale:    rationale,
		SupersededBy: supersededBy,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}
```

Update `CreateDecision` to store status as `storage.DecisionStatusProposed.String()` — note: status is stored as the string constant value (e.g., `"DECISION_STATUS_PROPOSED"`), which matches our `DecisionStatus` type values.

**Step 3: Update `decision_handler.go`**

Add converter calls. Convert `storage.Decision` to `specv1.Decision` at the RPC boundary:

```go
func (h *DecisionHandler) CreateDecision(ctx context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.Decision], error) {
	msg := req.Msg
	d, err := h.store.CreateDecision(ctx, msg.Slug, msg.Title, msg.Decision, msg.Rationale)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(decisionToProto(d)), nil
}
```

For `ListDecisions`, convert proto status to domain: `decisionStatusFromProto(msg.GetStatus())`.
For `UpdateDecision`, convert `msg.Status` proto pointer to `*storage.DecisionStatus`.

**Step 4: Run tests**

```bash
go test ./internal/server/ -v -count=1
go build ./...
```

Expected: PASS / clean build

**Step 5: Commit**

```bash
git add internal/storage/decision.go internal/storage/memgraph/decision.go internal/server/decision_handler.go
git commit -m "refactor(storage): swap DecisionBackend interface to domain types"
```

---

## Task 6: Swap GraphBackend Interface to Domain Types

**Files:**

- Modify: `internal/storage/graph.go` — change interface signatures
- Modify: `internal/storage/memgraph/graph.go` — update method signatures
- Modify: `internal/server/graph_handler.go` — add converter calls

**Step 1: Update `graph.go` interface**

Remove `specv1` import. Change interface to use domain types:

```go
type GraphBackend interface {
	AddEdge(ctx context.Context, fromSlug, toSlug string, edgeType EdgeType) (*Edge, error)
	RemoveEdge(ctx context.Context, fromSlug, toSlug string, edgeType EdgeType) error
	ListEdges(ctx context.Context, slug string, edgeType EdgeType) ([]*Edge, error)
	GetDependencies(ctx context.Context, slug string) ([]NodeRef, error)
	GetTransitiveDeps(ctx context.Context, slug string) ([]NodeRef, error)
	GetImpact(ctx context.Context, slug string) ([]NodeRef, error)
	GetReady(ctx context.Context) ([]NodeRef, error)
	GetCriticalPath(ctx context.Context, slug string) ([]NodeRef, error)
}
```

**Step 2: Update `memgraph/graph.go`**

Change edge type mapping from `specv1.EdgeType` to `storage.EdgeType`. Update `AddEdge`, `ListEdges`, `RemoveEdge` to use `storage.EdgeType` and `*storage.Edge`.

**Step 3: Update graph handler**

Add `edgeToProto`, `edgeTypeFromProto` calls at the RPC boundary (using converters from Task 3).

**Step 4: Run tests**

```bash
go build ./...
go test ./internal/server/ -v -count=1
```

Expected: clean build, tests pass

**Step 5: Commit**

```bash
git add internal/storage/graph.go internal/storage/memgraph/graph.go internal/server/graph_handler.go
git commit -m "refactor(storage): swap GraphBackend interface to domain types"
```

---

## Task 7: Remove specv1 Import from Storage Package

**Files:**

- Verify: `internal/storage/*.go` — no remaining `specv1` imports

**Step 1: Verify no proto imports remain in storage**

```bash
rg "specv1\|specgraphv1\|gen/specgraph" internal/storage/
```

Expected: no results (all proto references eliminated)

**Step 2: Verify clean build**

```bash
go build ./...
go test ./internal/storage/ ./internal/server/ -v -count=1
```

Expected: PASS

**Step 3: Commit (if any cleanup was needed)**

```bash
git add internal/storage/
git commit -m "refactor(storage): remove all proto imports from storage package"
```

---

## Task 8: Proto — Add DecisionInput to ShapeOutput

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto` — add DecisionInput, change ShapeOutput.decisions

**Step 1: Update the proto file**

Add `DecisionInput` message and change `ShapeOutput.decisions`:

```protobuf
message DecisionInput {
  string slug = 1;
  string title = 2;
  string decision = 3;
  string rationale = 4;
}

message ShapeOutput {
  repeated string scope_in = 1;
  repeated string scope_out = 2;
  repeated Approach approaches = 3;
  string chosen_approach = 4;
  repeated string risks = 5;
  repeated string success_must = 6;
  repeated string success_should = 7;
  repeated string success_wont = 8;
  repeated DecisionInput decisions = 9;  // was: repeated string
}
```

**Step 2: Regenerate proto**

```bash
task proto
```

**Step 3: Fix compilation — update `shapeOutputToDomain` in `authoring_handler.go`**

The `shapeOutputToDomain` function (line ~568) currently does `Decisions: p.GetDecisions()` which worked for `[]string`. Now it needs to convert `[]*specv1.DecisionInput` to `[]storage.DecisionInput`:

```go
func shapeOutputToDomain(p *specv1.ShapeOutput) *storage.ShapeOutput {
	approaches := make([]storage.Approach, len(p.GetApproaches()))
	for i, a := range p.GetApproaches() {
		approaches[i] = storage.Approach{
			Name:        a.GetName(),
			Description: a.GetDescription(),
			Tradeoffs:   a.GetTradeoffs(),
		}
	}
	decisions := make([]storage.DecisionInput, len(p.GetDecisions()))
	for i, d := range p.GetDecisions() {
		decisions[i] = storage.DecisionInput{
			Slug:      d.GetSlug(),
			Title:     d.GetTitle(),
			Decision:  d.GetDecision(),
			Rationale: d.GetRationale(),
		}
	}
	return &storage.ShapeOutput{
		// ... existing fields ...
		Decisions: decisions,
	}
}
```

**Step 4: Update domain ShapeOutput type in `authoring.go`**

Change the `Decisions` field from `[]string` to `[]DecisionInput` and add the `DecisionInput` type:

```go
type DecisionInput struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
}

type ShapeOutput struct {
	ScopeIn        []string         `json:"scope_in,omitempty"`
	ScopeOut       []string         `json:"scope_out,omitempty"`
	Approaches     []Approach       `json:"approaches,omitempty"`
	ChosenApproach string           `json:"chosen_approach,omitempty"`
	Risks          []string         `json:"risks,omitempty"`
	SuccessMust    []string         `json:"success_must,omitempty"`
	SuccessShould  []string         `json:"success_should,omitempty"`
	SuccessWont    []string         `json:"success_wont,omitempty"`
	Decisions      []DecisionInput  `json:"decisions,omitempty"`
}
```

Remove the `TODO(spgr-bpq)` comment from the Decisions field.

**Step 5: Fix any remaining compilation errors**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add proto/ gen/ internal/storage/authoring.go internal/server/authoring_handler.go
git commit -m "feat(proto): add DecisionInput, change ShapeOutput.decisions to structured type"
```

---

## Task 9: Decision Promotion in StoreShapeOutput

**Files:**

- Modify: `internal/storage/memgraph/authoring.go` — add decision creation + edge
- Modify: `internal/storage/memgraph/authoring_test.go` — add test

**Step 1: Write the integration test**

Add to `internal/storage/memgraph/authoring_test.go`:

```go
func TestStoreShapeOutput_CreatesDecisionNodes(t *testing.T) {
	store, cleanup := newStore(t)
	defer cleanup()

	ctx := context.Background()
	// Create a spec to attach decisions to
	_, err := store.CreateSpec(ctx, "shape-decisions-test", "test spec", "p1", "medium")
	require.NoError(t, err)

	// Store shape output with structured decisions
	shapeOut := &storage.ShapeOutput{
		ScopeIn: []string{"feature A"},
		Decisions: []storage.DecisionInput{
			{
				Slug:      "use-memgraph",
				Title:     "Use Memgraph",
				Decision:  "We chose Memgraph for graph storage",
				Rationale: "Native graph, Bolt protocol, good Go driver",
			},
		},
	}
	err = store.StoreShapeOutput(ctx, "shape-decisions-test", shapeOut)
	require.NoError(t, err)

	// Verify decision node was created
	decision, err := store.GetDecision(ctx, "use-memgraph")
	require.NoError(t, err)
	assert.Equal(t, "Use Memgraph", decision.Title)
	assert.Equal(t, "We chose Memgraph for graph storage", decision.Decision)
	assert.Equal(t, storage.DecisionStatusProposed, decision.Status)

	// Verify DECIDED_IN edge exists
	edges, err := store.ListEdges(ctx, "use-memgraph", storage.EdgeTypeDecidedIn)
	require.NoError(t, err)
	require.Len(t, edges, 1)
	assert.Equal(t, storage.EdgeTypeDecidedIn, edges[0].EdgeType)
}

func TestStoreShapeOutput_IdempotentDecisions(t *testing.T) {
	store, cleanup := newStore(t)
	defer cleanup()

	ctx := context.Background()
	_, err := store.CreateSpec(ctx, "idempotent-test", "test spec", "p1", "medium")
	require.NoError(t, err)

	shapeOut := &storage.ShapeOutput{
		ScopeIn: []string{"feature"},
		Decisions: []storage.DecisionInput{
			{Slug: "reuse-decision", Title: "Reuse", Decision: "Reuse it", Rationale: "Why not"},
		},
	}

	// Store twice — should not fail or create duplicate
	require.NoError(t, store.StoreShapeOutput(ctx, "idempotent-test", shapeOut))
	require.NoError(t, store.StoreShapeOutput(ctx, "idempotent-test", shapeOut))

	// Still just one decision node
	decision, err := store.GetDecision(ctx, "reuse-decision")
	require.NoError(t, err)
	assert.Equal(t, "Reuse", decision.Title)
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/storage/memgraph/ -run "TestStoreShapeOutput_Creates|TestStoreShapeOutput_Idempotent" -v -count=1 -timeout=120s
```

Expected: FAIL — no decision creation logic in StoreShapeOutput

**Step 3: Implement decision promotion**

Update `StoreShapeOutput` in `internal/storage/memgraph/authoring.go`:

```go
func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	if err := s.storeJSONProperty(ctx, slug, "shape_output", output); err != nil {
		return err
	}
	// Promote decisions to graph nodes with DECIDED_IN edges
	for _, d := range output.Decisions {
		if d.Slug == "" {
			continue
		}
		// Idempotent: skip if decision already exists
		if _, err := s.GetDecision(ctx, d.Slug); err == nil {
			continue
		}
		if _, err := s.CreateDecision(ctx, d.Slug, d.Title, d.Decision, d.Rationale); err != nil {
			return fmt.Errorf("create decision %q: %w", d.Slug, err)
		}
		if _, err := s.AddEdge(ctx, d.Slug, slug, storage.EdgeTypeDecidedIn); err != nil {
			return fmt.Errorf("add DECIDED_IN edge %q->%q: %w", d.Slug, slug, err)
		}
	}
	return nil
}
```

**Note:** `AddEdge` needs to support `EdgeTypeDecidedIn`. Check that the edge type mapping in `memgraph/graph.go` includes `DECIDED_IN`. If not, add it:

```go
// In the edge type → Cypher relationship name map:
storage.EdgeTypeDecidedIn: "DECIDED_IN",
```

**Step 4: Run integration tests**

```bash
go test ./internal/storage/memgraph/ -run "TestStoreShapeOutput" -v -count=1 -timeout=120s
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/storage/memgraph/authoring.go internal/storage/memgraph/authoring_test.go internal/storage/memgraph/graph.go
git commit -m "feat(storage): promote ShapeOutput decisions to Decision graph nodes"
```

---

## Task 10: Update E2E Tests

**Files:**

- Modify: `e2e/api/authoring_test.go` — update Shape test to use structured decisions
- Modify: `e2e/api/decision_test.go` — verify promoted decisions appear

**Step 1: Update Shape E2E test to include decisions**

In the "shapes a sparked spec" test, add decisions to the ShapeRequest:

```go
Output: &specv1.ShapeOutput{
	ScopeIn:  []string{"feature A"},
	ScopeOut: []string{"feature B"},
	Approaches: []*specv1.Approach{
		{Name: "approach-1", Description: "Do it this way"},
	},
	ChosenApproach: "approach-1",
	SuccessMust:    []string{"works correctly"},
	Decisions: []*specv1.DecisionInput{
		{
			Slug:      "e2e-decision-1",
			Title:     "Use approach 1",
			Decision:  "We chose approach 1",
			Rationale: "Simplest option",
		},
	},
},
```

**Step 2: Add a test verifying the decision was promoted**

After the shape test, add:

```go
It("promoted shape decisions to Decision nodes", func() {
	resp, err := decisionClient.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
		Slug: "e2e-decision-1",
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Msg.Title).To(Equal("Use approach 1"))
	Expect(resp.Msg.Decision).To(Equal("We chose approach 1"))
})
```

**Step 3: Run E2E tests**

```bash
task test:e2e:api
```

Expected: PASS

**Step 4: Commit**

```bash
git add e2e/
git commit -m "test(e2e): verify decision promotion from shape output"
```

---

## Task 11: Final Verification

**Step 1: Run full test suite**

```bash
task test
```

Expected: all tests pass

**Step 2: Run linters**

```bash
task lint
```

Expected: no issues

**Step 3: Verify no proto imports in storage**

```bash
rg "specv1\|specgraphv1\|gen/specgraph" internal/storage/
```

Expected: no results

**Step 4: Close beads**

```bash
bd close spgr-q5h --reason="Backend, DecisionBackend, and GraphBackend now use domain types. Proto conversion in handler layer."
bd close spgr-0zd --reason="ShapeOutput decisions auto-promoted to Decision graph nodes with DECIDED_IN edges at Shape time."
```

**Step 5: Final commit (if any cleanup)**

```bash
git add -A
git commit -m "refactor: complete storage domain types and decision promotion"
```
