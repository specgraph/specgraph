# Slice 4: Execution Bundles & Prime Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate execution bundles for approved specs, serve prime orientation context to executing agents, and accept progress/blocker/completion callbacks that automatically transition spec status.

**Architecture:** Execution is a new proto service (ExecutionService) with five RPCs: GenerateBundle assembles a lean YAML bundle from the spec, its decisions, and the constitution; GetPrime composes agent orientation; ReportProgress/ReportBlocker/ReportCompletion accept agent callbacks that validate claim ownership and transition spec status. A background lease sweeper goroutine in `serve` releases expired claims.

**Tech Stack:** Go, ConnectRPC, Memgraph, Cobra, buf, testcontainers-go, gopkg.in/yaml.v3

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 4 section)

---

## Project Structure (new files)

```text
proto/specgraph/v1/
  execution.proto                 # Execution messages + ExecutionService
gen/specgraph/v1/                 # Generated (buf generate)
  execution.pb.go
  specgraphv1connect/
    execution.connect.go
internal/
  storage/
    execution.go                  # ExecutionBackend interface
  storage/memgraph/
    execution.go                  # Memgraph implementation
    execution_test.go             # Integration tests
    sweeper.go                    # Lease sweeper queries
    sweeper_test.go               # Sweeper integration tests
  server/
    execution_handler.go          # ConnectRPC handler
    execution_handler_test.go     # Handler tests with mock
    sweeper.go                    # Background lease sweeper goroutine
    sweeper_test.go               # Sweeper goroutine tests
cmd/specgraph/
  bundle.go                       # CLI: bundle <slug>
  progress.go                     # CLI: progress <slug>
```

---

## Task 1: Protobuf Schema — Execution Messages + ExecutionService

**Files:**

- Create: `proto/specgraph/v1/execution.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/execution.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";
import "specgraph/v1/spec.proto";
import "specgraph/v1/decision.proto";

// --- Enums ---

enum ExecutionEventType {
  EXECUTION_EVENT_TYPE_UNSPECIFIED = 0;
  EXECUTION_EVENT_TYPE_PROGRESS = 1;
  EXECUTION_EVENT_TYPE_BLOCKER = 2;
  EXECUTION_EVENT_TYPE_COMPLETION = 3;
}

// --- Messages ---

message CallbackConfig {
  string endpoint = 1;      // base URL, e.g. "https://specgraph.internal:9090"
  string prime = 2;          // prime path, e.g. "/specgraph.v1.ExecutionService/GetPrime"
  string progress = 3;       // progress callback path
  string blocker = 4;        // blocker callback path
  string completion = 5;     // completion callback path
}

message Bundle {
  int32 version = 1;         // bundle format version, always 1
  Spec spec = 2;             // full spec snapshot
  repeated Decision decisions = 3;  // resolved decisions for this spec
  string bootstrap = 4;      // human-readable bootstrap instructions
  CallbackConfig callbacks = 5;
  string bundle_yaml = 6;    // pre-rendered lean YAML for file output
}

message PrimeResponse {
  string constitution_summary = 1;   // tech stack, constraints, conventions
  string project_context = 2;       // Constitution summary, architecture patterns
  repeated Decision decisions = 3;   // resolved decisions with rationale
  string coding_conventions = 4;     // from constitution
  string callback_docs = 5;          // how to report progress/blockers/completion
}

message ExecutionEvent {
  string id = 1;
  string spec_slug = 2;
  string agent = 3;
  ExecutionEventType type = 4;
  string message = 5;
  google.protobuf.Timestamp created_at = 6;
}

// --- Requests/Responses ---

message GenerateBundleRequest {
  string slug = 1;
  string endpoint = 2;      // callback endpoint base URL (optional, defaults to server address)
}

message GetPrimeRequest {
  string slug = 1;
}

message ReportProgressRequest {
  string slug = 1;
  string agent = 2;
  string message = 3;
}

message ReportProgressResponse {
  bool acknowledged = 1;
}

message ReportBlockerRequest {
  string slug = 1;
  string agent = 2;
  string description = 3;
}

message ReportBlockerResponse {
  bool acknowledged = 1;
}

message ReportCompletionRequest {
  string slug = 1;
  string agent = 2;
}

message ReportCompletionResponse {
  bool acknowledged = 1;
  string new_stage = 2;     // the stage the spec transitioned to (e.g. "done")
}

message GetExecutionEventsRequest {
  string slug = 1;
  int32 limit = 2;           // optional, defaults to 50
}

message GetExecutionEventsResponse {
  repeated ExecutionEvent events = 1;
}

// --- Service ---

service ExecutionService {
  rpc GenerateBundle(GenerateBundleRequest) returns (Bundle);
  rpc GetPrime(GetPrimeRequest) returns (PrimeResponse);
  rpc ReportProgress(ReportProgressRequest) returns (ReportProgressResponse);
  rpc ReportBlocker(ReportBlockerRequest) returns (ReportBlockerResponse);
  rpc ReportCompletion(ReportCompletionRequest) returns (ReportCompletionResponse);
  rpc GetExecutionEvents(GetExecutionEventsRequest) returns (GetExecutionEventsResponse);
}
```

**Step 2: Generate Go code**

```bash
buf generate
```

Expected: generates `gen/specgraph/v1/execution.pb.go` and `gen/specgraph/v1/specgraphv1connect/execution.connect.go`

**Step 3: Verify generated code compiles**

```bash
go mod tidy
go build ./gen/...
```

**Step 4: Commit**

```bash
git add proto/specgraph/v1/execution.proto gen/specgraph/v1/execution.pb.go gen/specgraph/v1/specgraphv1connect/execution.connect.go go.mod go.sum
git commit -m "feat(execution): protobuf schema for ExecutionService and bundle messages"
```

---

## Task 2: Storage Interface — ExecutionBackend

**Files:**

- Create: `internal/storage/execution.go`

**Step 1: Define the interface**

`internal/storage/execution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrSpecNotApproved is returned when a bundle is requested for a spec not in an executable stage.
var ErrSpecNotApproved = errors.New("spec is not in an approved or in_progress stage")

// ErrAgentNotClaimOwner is returned when an agent reports an event but does not hold the claim.
var ErrAgentNotClaimOwner = errors.New("agent does not hold the claim for this spec")

// ExecutionBackend defines storage operations for execution bundles and agent callbacks.
type ExecutionBackend interface {
	// GenerateBundle assembles a bundle from the spec, its decisions, and the constitution.
	// Returns ErrSpecNotFound if the spec does not exist.
	// Returns ErrSpecNotApproved if the spec is not in approved or in_progress stage.
	GenerateBundle(ctx context.Context, slug string) (*specv1.Bundle, error)

	// RecordProgress stores a progress event from an executing agent.
	// Returns ErrAgentNotClaimOwner if the agent does not hold the claim.
	RecordProgress(ctx context.Context, slug, agent, message string) error

	// RecordBlocker stores a blocker event from an executing agent.
	// Returns ErrAgentNotClaimOwner if the agent does not hold the claim.
	RecordBlocker(ctx context.Context, slug, agent, description string) error

	// RecordCompletion stores a completion event and transitions spec to done.
	// Returns ErrAgentNotClaimOwner if the agent does not hold the claim.
	RecordCompletion(ctx context.Context, slug, agent string) error

	// GetExecutionEvents returns execution events for a spec, ordered by time descending.
	GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*specv1.ExecutionEvent, error)

	// GetPrimeData returns the data needed to compose a prime response.
	// Returns the constitution summary, decisions for this spec, and spec metadata.
	GetPrimeData(ctx context.Context, slug string) (*PrimeData, error)

	// ReleaseExpiredClaims finds and releases all CLAIMED_BY relationships past their lease.
	// Returns the number of claims released.
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}

// PrimeData holds the raw data needed to compose a PrimeResponse.
type PrimeData struct {
	Spec         *specv1.Spec
	Decisions    []*specv1.Decision
	Constitution *specv1.Constitution
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/execution.go
git commit -m "feat(execution): storage backend interface for execution bundles and callbacks"
```

---

## Task 3: Memgraph Implementation — Execution Storage

**Files:**

- Create: `internal/storage/memgraph/execution.go`
- Create: `internal/storage/memgraph/execution_test.go`

**Step 1: Write the integration test**

`internal/storage/memgraph/execution_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestExecution_GenerateBundle_NotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GenerateBundle(ctx, "nonexistent")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestExecution_GenerateBundle_NotApproved(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec in spark stage (not approved)
	_, err = store.CreateSpec(ctx, "draft-spec", "A draft spec", "p2", "medium")
	require.NoError(t, err)

	_, err = store.GenerateBundle(ctx, "draft-spec")
	require.ErrorIs(t, err, storage.ErrSpecNotApproved)
}

func TestExecution_GenerateBundle_Approved(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec and move it to approved
	_, err = store.CreateSpec(ctx, "approved-spec", "An approved spec", "p1", "medium")
	require.NoError(t, err)
	stage := "approved"
	_, err = store.UpdateSpec(ctx, "approved-spec", nil, &stage, nil, nil)
	require.NoError(t, err)

	// Create a decision linked to the spec
	_, err = store.CreateDecision(ctx, "dec-test", "Test Decision", "Use approach A", "Simpler")
	require.NoError(t, err)

	// Link decision to spec via DECIDED_IN edge
	_, err = store.AddEdge(ctx, "approved-spec", "dec-test", 6) // DECIDED_IN
	require.NoError(t, err)

	bundle, err := store.GenerateBundle(ctx, "approved-spec")
	require.NoError(t, err)
	require.NotNil(t, bundle.Spec)
	require.Equal(t, "approved-spec", bundle.Spec.Slug)
	require.Equal(t, int32(1), bundle.Version)
	require.NotEmpty(t, bundle.Bootstrap)
	require.Len(t, bundle.Decisions, 1)
	require.Equal(t, "dec-test", bundle.Decisions[0].Slug)
}

func TestExecution_Callbacks_NoClaimOwner(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "exec-spec", "A spec", "p1", "medium")
	require.NoError(t, err)

	// No claim — should fail
	err = store.RecordProgress(ctx, "exec-spec", "agent-1", "working on it")
	require.ErrorIs(t, err, storage.ErrAgentNotClaimOwner)
}

func TestExecution_Callbacks_WithClaim(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create spec at in_progress stage and claim it
	_, err = store.CreateSpec(ctx, "exec-spec", "A spec", "p1", "medium")
	require.NoError(t, err)
	stage := "in_progress"
	_, err = store.UpdateSpec(ctx, "exec-spec", nil, &stage, nil, nil)
	require.NoError(t, err)
	_, err = store.ClaimSpec(ctx, "exec-spec", "agent-1", 15*time.Minute)
	require.NoError(t, err)

	// Progress
	err = store.RecordProgress(ctx, "exec-spec", "agent-1", "working on it")
	require.NoError(t, err)

	// Blocker
	err = store.RecordBlocker(ctx, "exec-spec", "agent-1", "need clarification")
	require.NoError(t, err)

	// Get events
	events, err := store.GetExecutionEvents(ctx, "exec-spec", 50)
	require.NoError(t, err)
	require.Len(t, events, 2)

	// Completion — transitions spec to done
	err = store.RecordCompletion(ctx, "exec-spec", "agent-1")
	require.NoError(t, err)

	// Verify spec is now done
	spec, err := store.GetSpec(ctx, "exec-spec")
	require.NoError(t, err)
	require.Equal(t, "done", spec.Stage)

	// Events now include completion
	events, err = store.GetExecutionEvents(ctx, "exec-spec", 50)
	require.NoError(t, err)
	require.Len(t, events, 3)
}

func TestExecution_GetPrimeData(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Set up spec, decision, constitution
	_, err = store.CreateSpec(ctx, "prime-spec", "A spec for prime", "p1", "medium")
	require.NoError(t, err)
	stage := "approved"
	_, err = store.UpdateSpec(ctx, "prime-spec", nil, &stage, nil, nil)
	require.NoError(t, err)

	_, err = store.CreateDecision(ctx, "dec-prime", "Prime Decision", "Use B", "Better fit")
	require.NoError(t, err)
	_, err = store.AddEdge(ctx, "prime-spec", "dec-prime", 6) // DECIDED_IN
	require.NoError(t, err)

	data, err := store.GetPrimeData(ctx, "prime-spec")
	require.NoError(t, err)
	require.NotNil(t, data.Spec)
	require.Equal(t, "prime-spec", data.Spec.Slug)
	require.Len(t, data.Decisions, 1)
	// Constitution may be nil if none stored — that's OK
}

func TestExecution_ReleaseExpiredClaims(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "expired-spec", "A spec", "p1", "medium")
	require.NoError(t, err)

	// Claim with very short lease
	_, err = store.ClaimSpec(ctx, "expired-spec", "agent-1", 1*time.Millisecond)
	require.NoError(t, err)

	// Wait for lease to expire
	time.Sleep(10 * time.Millisecond)

	released, err := store.ReleaseExpiredClaims(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, released)

	// Should be able to claim again
	_, err = store.ClaimSpec(ctx, "expired-spec", "agent-2", 15*time.Minute)
	require.NoError(t, err)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/storage/memgraph/ -run TestExecution -v -count=1 -timeout=120s
```

Expected: FAIL — `execution.go` doesn't exist yet

**Step 3: Implement the Memgraph execution backend**

`internal/storage/memgraph/execution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// executableStages are stages where a bundle can be generated.
var executableStages = map[string]bool{
	"approved":    true,
	"in_progress": true,
}

func (s *Store) GenerateBundle(ctx context.Context, slug string) (*specv1.Bundle, error) {
	// Get spec
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err // already wraps ErrSpecNotFound
	}

	if !executableStages[spec.Stage] {
		return nil, fmt.Errorf("memgraph: generate bundle %q (stage %q): %w", slug, spec.Stage, storage.ErrSpecNotApproved)
	}

	// Get decisions linked via DECIDED_IN
	decisions, err := s.getSpecDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle decisions: %w", err)
	}

	bootstrap := composeBundleBootstrap(slug)

	return &specv1.Bundle{
		Version:   1,
		Spec:      spec,
		Decisions: decisions,
		Bootstrap: bootstrap,
		Callbacks: &specv1.CallbackConfig{
			Prime:      fmt.Sprintf("/specgraph.v1.ExecutionService/GetPrime"),
			Progress:   fmt.Sprintf("/specgraph.v1.ExecutionService/ReportProgress"),
			Blocker:    fmt.Sprintf("/specgraph.v1.ExecutionService/ReportBlocker"),
			Completion: fmt.Sprintf("/specgraph.v1.ExecutionService/ReportCompletion"),
		},
	}, nil
}

func (s *Store) getSpecDecisions(ctx context.Context, slug string) ([]*specv1.Decision, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[:DECIDED_IN]->(d:Decision)
		 RETURN d.id, d.slug, d.title, d.status, d.decision, d.rationale,
		        d.version, d.created_at, d.updated_at`,
		map[string]any{"slug": slug},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get spec decisions: %w", err)
	}

	decisions := make([]*specv1.Decision, 0, len(result.Records))
	for _, rec := range result.Records {
		d, err := recordToDecision(rec)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

func composeBundleBootstrap(slug string) string {
	return fmt.Sprintf(`You are executing a SpecGraph specification. This bundle contains a fully
designed work unit — the intent, interface contract, verification criteria,
and key decisions have already been made. Your job is to implement it.

## Before you start
Call the prime endpoint to get project context, coding conventions,
relevant files, and available operations.

The prime response will tell you:
- Project tech stack and conventions
- Which files to modify and patterns to follow
- How to report progress, blockers, and completion
- Resolved decisions (don't re-decide these)

## Workflow
1. Read the spec below (intent, interface, verify, invariants)
2. Call prime to get project context and callback operations
3. Implement per the spec and conventions
4. Report progress as you go
5. Verify against all criteria in the spec
6. Report completion when all verify criteria pass

## Rules
- Do NOT make design decisions — they're already made (see decisions)
- Do NOT deviate from the interface contract
- If something is unclear or blocked, report a blocker — don't guess
- Follow the project conventions from the prime response
`)
}

func (s *Store) verifyClaimOwner(ctx context.Context, slug, agent string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[c:CLAIMED_BY]->(a:Agent {name: $agent})
		 WHERE c.lease_expires > $now
		 RETURN c.lease_expires`,
		map[string]any{"slug": slug, "agent": agent, "now": now},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("memgraph: verify claim owner: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: verify claim %q by %q: %w", slug, agent, storage.ErrAgentNotClaimOwner)
	}
	return nil
}

func (s *Store) recordExecutionEvent(ctx context.Context, slug, agent string, eventType specv1.ExecutionEventType, message string) error {
	now := time.Now().UTC()
	id := generateID("evt", slug+"-"+agent, now)

	_, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})
		 CREATE (e:ExecutionEvent {
			id: $id,
			spec_slug: $slug,
			agent: $agent,
			type: $type,
			message: $message,
			created_at: $created_at
		 })
		 CREATE (s)-[:HAS_EVENT]->(e)`,
		map[string]any{
			"id":         id,
			"slug":       slug,
			"agent":      agent,
			"type":       eventType.String(),
			"message":    message,
			"created_at": now.Format(time.RFC3339),
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("memgraph: record execution event: %w", err)
	}
	return nil
}

func (s *Store) RecordProgress(ctx context.Context, slug, agent, message string) error {
	if err := s.verifyClaimOwner(ctx, slug, agent); err != nil {
		return err
	}
	return s.recordExecutionEvent(ctx, slug, agent, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS, message)
}

func (s *Store) RecordBlocker(ctx context.Context, slug, agent, description string) error {
	if err := s.verifyClaimOwner(ctx, slug, agent); err != nil {
		return err
	}
	return s.recordExecutionEvent(ctx, slug, agent, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER, description)
}

func (s *Store) RecordCompletion(ctx context.Context, slug, agent string) error {
	if err := s.verifyClaimOwner(ctx, slug, agent); err != nil {
		return err
	}

	if err := s.recordExecutionEvent(ctx, slug, agent, specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION, "completed"); err != nil {
		return err
	}

	// Transition spec to done
	stage := "done"
	_, err := s.UpdateSpec(ctx, slug, nil, &stage, nil, nil)
	if err != nil {
		return fmt.Errorf("memgraph: complete spec: %w", err)
	}

	// Release the claim
	_ = s.UnclaimSpec(ctx, slug, agent)

	return nil
}

func (s *Store) GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*specv1.ExecutionEvent, error) {
	if limit <= 0 {
		limit = 50
	}

	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[:HAS_EVENT]->(e:ExecutionEvent)
		 RETURN e.id, e.spec_slug, e.agent, e.type, e.message, e.created_at
		 ORDER BY e.created_at DESC
		 LIMIT $limit`,
		map[string]any{"slug": slug, "limit": int64(limit)},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get execution events: %w", err)
	}

	events := make([]*specv1.ExecutionEvent, 0, len(result.Records))
	for _, rec := range result.Records {
		evt, err := recordToExecutionEvent(rec)
		if err != nil {
			return nil, err
		}
		events = append(events, evt)
	}
	return events, nil
}

func (s *Store) GetPrimeData(ctx context.Context, slug string) (*storage.PrimeData, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}

	decisions, err := s.getSpecDecisions(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Constitution may not exist — that's OK for prime
	var constitution *specv1.Constitution
	c, err := s.GetConstitution(ctx)
	if err == nil {
		constitution = c
	}

	return &storage.PrimeData{
		Spec:         spec,
		Decisions:    decisions,
		Constitution: constitution,
	}, nil
}

func (s *Store) ReleaseExpiredClaims(ctx context.Context) (int, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec)-[c:CLAIMED_BY]->(a:Agent)
		 WHERE c.lease_expires <= $now
		 DELETE c
		 RETURN count(c) AS released`,
		map[string]any{"now": now},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return 0, fmt.Errorf("memgraph: release expired claims: %w", err)
	}
	if len(result.Records) == 0 {
		return 0, nil
	}

	released, err := recordInt64(result.Records[0], 0, "released")
	if err != nil {
		return 0, err
	}
	return int(released), nil
}

func recordToExecutionEvent(rec *neo4j.Record) (*specv1.ExecutionEvent, error) {
	evt := &specv1.ExecutionEvent{}

	var err error
	evt.Id, err = recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}

	evt.SpecSlug, err = recordString(rec, 1, "spec_slug")
	if err != nil {
		return nil, err
	}

	evt.Agent, err = recordString(rec, 2, "agent")
	if err != nil {
		return nil, err
	}

	typeStr, err := recordString(rec, 3, "type")
	if err != nil {
		return nil, err
	}
	if val, ok := specv1.ExecutionEventType_value[typeStr]; ok {
		evt.Type = specv1.ExecutionEventType(val)
	}

	evt.Message, err = recordString(rec, 4, "message")
	if err != nil {
		return nil, err
	}

	createdStr, err := recordString(rec, 5, "created_at")
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, createdStr); parseErr == nil {
		evt.CreatedAt = timestamppb.New(t)
	}

	return evt, nil
}
```

**Step 4: Run the tests**

```bash
go mod tidy
go test ./internal/storage/memgraph/ -run TestExecution -v -count=1 -timeout=120s
```

Expected: PASS (all tests). Requires Docker running.

**Step 5: Commit**

```bash
git add internal/storage/memgraph/execution.go internal/storage/memgraph/execution_test.go
git commit -m "feat(execution): memgraph storage backend with integration tests"
```

---

## Task 4: ConnectRPC Handler — ExecutionService

**Files:**

- Create: `internal/server/execution_handler.go`
- Create: `internal/server/execution_handler_test.go`

**Step 1: Write the handler test**

`internal/server/execution_handler_test.go`:

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
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockExecutionBackend struct {
	mu     sync.Mutex
	specs  map[string]*specv1.Spec
	events []*specv1.ExecutionEvent
	claims map[string]string // slug -> agent
}

func newMockExecutionBackend() *mockExecutionBackend {
	return &mockExecutionBackend{
		specs:  map[string]*specv1.Spec{},
		events: []*specv1.ExecutionEvent{},
		claims: map[string]string{},
	}
}

func (m *mockExecutionBackend) GenerateBundle(_ context.Context, slug string) (*specv1.Bundle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("mock: spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	if spec.Stage != "approved" && spec.Stage != "in_progress" {
		return nil, fmt.Errorf("mock: spec %q: %w", slug, storage.ErrSpecNotApproved)
	}
	return &specv1.Bundle{
		Version:   1,
		Spec:      spec,
		Bootstrap: "test bootstrap",
		Callbacks: &specv1.CallbackConfig{
			Prime:      "/prime",
			Progress:   "/progress",
			Blocker:    "/blocker",
			Completion: "/completion",
		},
	}, nil
}

func (m *mockExecutionBackend) RecordProgress(_ context.Context, slug, agent, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claims[slug] != agent {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.events = append(m.events, &specv1.ExecutionEvent{
		SpecSlug: slug, Agent: agent, Message: message,
		Type: specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS,
	})
	return nil
}

func (m *mockExecutionBackend) RecordBlocker(_ context.Context, slug, agent, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claims[slug] != agent {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.events = append(m.events, &specv1.ExecutionEvent{
		SpecSlug: slug, Agent: agent, Message: description,
		Type: specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER,
	})
	return nil
}

func (m *mockExecutionBackend) RecordCompletion(_ context.Context, slug, agent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.claims[slug] != agent {
		return fmt.Errorf("mock: %w", storage.ErrAgentNotClaimOwner)
	}
	m.events = append(m.events, &specv1.ExecutionEvent{
		SpecSlug: slug, Agent: agent, Message: "completed",
		Type: specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION,
	})
	m.specs[slug].Stage = "done"
	delete(m.claims, slug)
	return nil
}

func (m *mockExecutionBackend) GetExecutionEvents(_ context.Context, slug string, limit int) ([]*specv1.ExecutionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*specv1.ExecutionEvent
	for _, e := range m.events {
		if e.SpecSlug == slug {
			result = append(result, e)
		}
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (m *mockExecutionBackend) GetPrimeData(_ context.Context, slug string) (*storage.PrimeData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("mock: spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return &storage.PrimeData{
		Spec:      spec,
		Decisions: []*specv1.Decision{},
	}, nil
}

func (m *mockExecutionBackend) ReleaseExpiredClaims(_ context.Context) (int, error) {
	return 0, nil
}

var _ storage.ExecutionBackend = (*mockExecutionBackend)(nil)

func setupExecutionServer(t *testing.T) (specgraphv1connect.ExecutionServiceClient, *mockExecutionBackend) {
	t.Helper()
	mb := newMockExecutionBackend()
	mb.specs["test-spec"] = &specv1.Spec{
		Id: "spec-test", Slug: "test-spec", Intent: "Test",
		Stage: "approved", Priority: "p1", Complexity: "medium",
		Version: 1, CreatedAt: timestamppb.Now(), UpdatedAt: timestamppb.Now(),
	}
	mb.claims["test-spec"] = "agent-1"
	mux := http.NewServeMux()
	server.RegisterExecutionService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewExecutionServiceClient(http.DefaultClient, srv.URL), mb
}

func TestExecutionHandler_GenerateBundle(t *testing.T) {
	client, _ := setupExecutionServer(t)
	ctx := context.Background()

	resp, err := client.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug: "test-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Version)
	require.Equal(t, "test-spec", resp.Msg.Spec.Slug)
	require.NotEmpty(t, resp.Msg.Bootstrap)
}

func TestExecutionHandler_GenerateBundle_NotFound(t *testing.T) {
	client, _ := setupExecutionServer(t)
	_, err := client.GenerateBundle(context.Background(),
		connect.NewRequest(&specv1.GenerateBundleRequest{Slug: "nonexistent"}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestExecutionHandler_GetPrime(t *testing.T) {
	client, _ := setupExecutionServer(t)
	resp, err := client.GetPrime(context.Background(),
		connect.NewRequest(&specv1.GetPrimeRequest{Slug: "test-spec"}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.CallbackDocs)
}

func TestExecutionHandler_ReportProgress(t *testing.T) {
	client, _ := setupExecutionServer(t)
	resp, err := client.ReportProgress(context.Background(),
		connect.NewRequest(&specv1.ReportProgressRequest{
			Slug: "test-spec", Agent: "agent-1", Message: "implementing feature",
		}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Acknowledged)
}

func TestExecutionHandler_ReportProgress_NoClaim(t *testing.T) {
	client, _ := setupExecutionServer(t)
	_, err := client.ReportProgress(context.Background(),
		connect.NewRequest(&specv1.ReportProgressRequest{
			Slug: "test-spec", Agent: "agent-2", Message: "trying",
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestExecutionHandler_ReportCompletion(t *testing.T) {
	client, mb := setupExecutionServer(t)
	resp, err := client.ReportCompletion(context.Background(),
		connect.NewRequest(&specv1.ReportCompletionRequest{
			Slug: "test-spec", Agent: "agent-1",
		}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Acknowledged)
	require.Equal(t, "done", resp.Msg.NewStage)
	require.Equal(t, "done", mb.specs["test-spec"].Stage)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/server/ -run TestExecution -v -count=1
```

Expected: FAIL — handler doesn't exist yet

**Step 3: Implement the handler**

`internal/server/execution_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// ExecutionHandler implements the ExecutionService.
type ExecutionHandler struct {
	store storage.ExecutionBackend
}

var _ specgraphv1connect.ExecutionServiceHandler = (*ExecutionHandler)(nil)

// RegisterExecutionService registers the ExecutionService handler on the mux.
func RegisterExecutionService(mux *http.ServeMux, store storage.ExecutionBackend) {
	handler := &ExecutionHandler{store: store}
	path, h := specgraphv1connect.NewExecutionServiceHandler(handler)
	mux.Handle(path, h)
}

func (h *ExecutionHandler) GenerateBundle(ctx context.Context, req *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.Bundle], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}

	bundle, err := h.store.GenerateBundle(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSpecNotApproved) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Set endpoint if provided
	if req.Msg.Endpoint != "" && bundle.Callbacks != nil {
		bundle.Callbacks.Endpoint = req.Msg.Endpoint
	}

	// Render bundle YAML
	bundle.BundleYaml = renderBundleYAML(bundle)

	return connect.NewResponse(bundle), nil
}

func (h *ExecutionHandler) GetPrime(ctx context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}

	data, err := h.store.GetPrimeData(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &specv1.PrimeResponse{
		Decisions:    data.Decisions,
		CallbackDocs: composeCallbackDocs(),
	}

	if data.Constitution != nil {
		resp.ConstitutionSummary = composeConstitutionSummary(data.Constitution)
		resp.CodingConventions = composeCodingConventions(data.Constitution)
	}

	return connect.NewResponse(resp), nil
}

func (h *ExecutionHandler) ReportProgress(ctx context.Context, req *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	if req.Msg.Slug == "" || req.Msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug and agent are required"))
	}

	err := h.store.RecordProgress(ctx, req.Msg.Slug, req.Msg.Agent, req.Msg.Message)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotClaimOwner) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ReportProgressResponse{Acknowledged: true}), nil
}

func (h *ExecutionHandler) ReportBlocker(ctx context.Context, req *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	if req.Msg.Slug == "" || req.Msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug and agent are required"))
	}

	err := h.store.RecordBlocker(ctx, req.Msg.Slug, req.Msg.Agent, req.Msg.Description)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotClaimOwner) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ReportBlockerResponse{Acknowledged: true}), nil
}

func (h *ExecutionHandler) ReportCompletion(ctx context.Context, req *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	if req.Msg.Slug == "" || req.Msg.Agent == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug and agent are required"))
	}

	err := h.store.RecordCompletion(ctx, req.Msg.Slug, req.Msg.Agent)
	if err != nil {
		if errors.Is(err, storage.ErrAgentNotClaimOwner) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ReportCompletionResponse{
		Acknowledged: true,
		NewStage:     "done",
	}), nil
}

func (h *ExecutionHandler) GetExecutionEvents(ctx context.Context, req *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	if req.Msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}

	limit := int(req.Msg.Limit)
	if limit <= 0 {
		limit = 50
	}

	events, err := h.store.GetExecutionEvents(ctx, req.Msg.Slug, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetExecutionEventsResponse{Events: events}), nil
}

func renderBundleYAML(bundle *specv1.Bundle) string {
	var b strings.Builder

	b.WriteString("# Generated by: specgraph bundle\n\n")
	b.WriteString(fmt.Sprintf("# -- Bootstrap --\n"))
	b.WriteString(fmt.Sprintf("bootstrap: |\n"))
	for _, line := range strings.Split(bundle.Bootstrap, "\n") {
		if line == "" {
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}
	b.WriteString("\n")

	b.WriteString("# -- Spec --\n")
	b.WriteString("spec:\n")
	b.WriteString(fmt.Sprintf("  id: %q\n", bundle.Spec.Id))
	b.WriteString(fmt.Sprintf("  slug: %q\n", bundle.Spec.Slug))
	b.WriteString(fmt.Sprintf("  intent: %q\n", bundle.Spec.Intent))
	b.WriteString(fmt.Sprintf("  stage: %s\n", bundle.Spec.Stage))
	b.WriteString(fmt.Sprintf("  priority: %s\n", bundle.Spec.Priority))
	b.WriteString(fmt.Sprintf("  complexity: %s\n", bundle.Spec.Complexity))
	b.WriteString("\n")

	if len(bundle.Decisions) > 0 {
		b.WriteString("# -- Decisions --\n")
		b.WriteString("decisions:\n")
		for _, d := range bundle.Decisions {
			b.WriteString(fmt.Sprintf("  - slug: %q\n", d.Slug))
			b.WriteString(fmt.Sprintf("    title: %q\n", d.Title))
			b.WriteString(fmt.Sprintf("    decision: %q\n", d.Decision))
			b.WriteString(fmt.Sprintf("    rationale: %q\n", d.Rationale))
		}
		b.WriteString("\n")
	}

	if bundle.Callbacks != nil {
		b.WriteString("# -- Callbacks --\n")
		b.WriteString("callbacks:\n")
		if bundle.Callbacks.Endpoint != "" {
			b.WriteString(fmt.Sprintf("  endpoint: %q\n", bundle.Callbacks.Endpoint))
		}
		b.WriteString(fmt.Sprintf("  prime: %q\n", bundle.Callbacks.Prime))
		b.WriteString(fmt.Sprintf("  progress: %q\n", bundle.Callbacks.Progress))
		b.WriteString(fmt.Sprintf("  blocker: %q\n", bundle.Callbacks.Blocker))
		b.WriteString(fmt.Sprintf("  completion: %q\n", bundle.Callbacks.Completion))
	}

	return b.String()
}

func composeConstitutionSummary(c *specv1.Constitution) string {
	var b strings.Builder

	b.WriteString("# Project Constitution\n\n")

	if c.Tech != nil && c.Tech.Languages != nil {
		b.WriteString(fmt.Sprintf("**Primary Language:** %s\n", c.Tech.Languages.Primary))
		if len(c.Tech.Languages.Allowed) > 0 {
			b.WriteString(fmt.Sprintf("**Allowed:** %s\n", strings.Join(c.Tech.Languages.Allowed, ", ")))
		}
		if len(c.Tech.Languages.Forbidden) > 0 {
			b.WriteString(fmt.Sprintf("**Forbidden:** %s\n", strings.Join(c.Tech.Languages.Forbidden, ", ")))
		}
	}

	if c.Tech != nil && len(c.Tech.Frameworks) > 0 {
		b.WriteString("\n**Frameworks:**\n")
		for area, fw := range c.Tech.Frameworks {
			b.WriteString(fmt.Sprintf("- %s: %s\n", area, fw))
		}
	}

	if len(c.Constraints) > 0 {
		b.WriteString("\n**Constraints:**\n")
		for _, constraint := range c.Constraints {
			b.WriteString(fmt.Sprintf("- %s\n", constraint))
		}
	}

	return b.String()
}

func composeCodingConventions(c *specv1.Constitution) string {
	var b strings.Builder

	b.WriteString("# Coding Conventions\n\n")

	if len(c.Principles) > 0 {
		b.WriteString("**Principles:**\n")
		for _, p := range c.Principles {
			b.WriteString(fmt.Sprintf("- %s: %s\n", p.Id, p.Principle))
		}
		b.WriteString("\n")
	}

	if len(c.Antipatterns) > 0 {
		b.WriteString("**Anti-patterns (avoid these):**\n")
		for _, ap := range c.Antipatterns {
			b.WriteString(fmt.Sprintf("- %s — %s. Instead: %s\n", ap.Pattern, ap.Why, ap.Instead))
		}
	}

	return b.String()
}

func composeCallbackDocs() string {
	return `# Callback Operations

Report your progress to SpecGraph as you work. All callbacks require your agent ID
and the spec slug. You must hold an active claim on the spec.

## ReportProgress
Call when you've made meaningful progress (e.g., completed a subtask, passed a test).
POST /specgraph.v1.ExecutionService/ReportProgress
Body: {"slug": "<spec-slug>", "agent": "<your-agent-id>", "message": "<what you did>"}

## ReportBlocker
Call when you're stuck and need human intervention.
POST /specgraph.v1.ExecutionService/ReportBlocker
Body: {"slug": "<spec-slug>", "agent": "<your-agent-id>", "description": "<what's blocking you>"}

## ReportCompletion
Call when all verification criteria pass and the work is done.
POST /specgraph.v1.ExecutionService/ReportCompletion
Body: {"slug": "<spec-slug>", "agent": "<your-agent-id>"}
This transitions the spec to "done" and releases your claim.
`
}
```

**Step 4: Run the tests**

```bash
go test ./internal/server/ -run TestExecution -v -count=1
```

Expected: PASS

**Step 5: Wire into serve.go**

Add `server.RegisterExecutionService(mux, store)` in `cmd/specgraph/serve.go` after the other service registrations:

```go
// In serve.go, after line "server.RegisterClaimService(mux, store)":
server.RegisterExecutionService(mux, store)
```

**Step 6: Verify full build**

```bash
go build ./cmd/specgraph/
```

**Step 7: Commit**

```bash
git add internal/server/execution_handler.go internal/server/execution_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(execution): ConnectRPC handler with handler tests"
```

---

## Task 5: Bundle YAML Rendering

**Files:**

- Modify: `internal/server/execution_handler.go` (already created in Task 4 with `renderBundleYAML`)

This task verifies that the bundle YAML rendering is correct by adding a dedicated test.

**Step 1: Add bundle YAML rendering test**

Add to `internal/server/execution_handler_test.go`:

```go
func TestExecutionHandler_GenerateBundle_HasYAML(t *testing.T) {
	client, _ := setupExecutionServer(t)
	ctx := context.Background()

	resp, err := client.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug:     "test-spec",
		Endpoint: "https://specgraph.example.com:9090",
	}))
	require.NoError(t, err)
	require.Contains(t, resp.Msg.BundleYaml, "bootstrap:")
	require.Contains(t, resp.Msg.BundleYaml, "spec:")
	require.Contains(t, resp.Msg.BundleYaml, `slug: "test-spec"`)
	require.Contains(t, resp.Msg.BundleYaml, "callbacks:")
	require.Contains(t, resp.Msg.BundleYaml, `endpoint: "https://specgraph.example.com:9090"`)
}
```

**Step 2: Run the test**

```bash
go test ./internal/server/ -run TestExecutionHandler_GenerateBundle_HasYAML -v -count=1
```

Expected: PASS

**Step 3: Commit**

```bash
git add internal/server/execution_handler_test.go
git commit -m "test(execution): verify bundle YAML rendering with endpoint"
```

---

## Task 6: Lease Sweeper — Background Goroutine

**Files:**

- Create: `internal/server/sweeper.go`
- Create: `internal/server/sweeper_test.go`

**Step 1: Write the sweeper test**

`internal/server/sweeper_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockSweeperBackend struct {
	mu       sync.Mutex
	released int
	calls    int
}

func (m *mockSweeperBackend) ReleaseExpiredClaims(_ context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.released += 1
	return 1, nil
}

var _ server.ClaimSweeper = (*mockSweeperBackend)(nil)

func TestSweeper_RunsOnInterval(t *testing.T) {
	mb := &mockSweeperBackend{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start sweeper with 50ms interval
	server.StartSweeper(ctx, mb, 50*time.Millisecond)

	// Wait enough time for at least 2 sweeps
	time.Sleep(150 * time.Millisecond)
	cancel()

	mb.mu.Lock()
	defer mb.mu.Unlock()
	require.GreaterOrEqual(t, mb.calls, 2)
}

func TestSweeper_StopsOnContextCancel(t *testing.T) {
	mb := &mockSweeperBackend{}

	ctx, cancel := context.WithCancel(context.Background())
	server.StartSweeper(ctx, mb, 50*time.Millisecond)

	// Let it run briefly
	time.Sleep(80 * time.Millisecond)
	cancel()

	// Wait a bit more
	time.Sleep(100 * time.Millisecond)

	mb.mu.Lock()
	callsAtCancel := mb.calls
	mb.mu.Unlock()

	// Should not increase significantly after cancel
	time.Sleep(100 * time.Millisecond)
	mb.mu.Lock()
	callsAfterWait := mb.calls
	mb.mu.Unlock()

	require.InDelta(t, callsAtCancel, callsAfterWait, 1)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/server/ -run TestSweeper -v -count=1
```

Expected: FAIL — `sweeper.go` doesn't exist yet

**Step 3: Implement the sweeper**

`internal/server/sweeper.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"
	"os"
	"time"
)

// ClaimSweeper is the interface needed by the lease sweeper.
type ClaimSweeper interface {
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}

// StartSweeper starts a background goroutine that periodically releases expired claims.
// It stops when the context is cancelled.
func StartSweeper(ctx context.Context, store ClaimSweeper, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				released, err := store.ReleaseExpiredClaims(ctx)
				if err != nil {
					fmt.Fprintf(os.Stderr, "sweeper: release expired claims: %v\n", err)
					continue
				}
				if released > 0 {
					fmt.Printf("sweeper: released %d expired claim(s)\n", released)
				}
			}
		}
	}()
}
```

**Step 4: Run the tests**

```bash
go test ./internal/server/ -run TestSweeper -v -count=1
```

Expected: PASS

**Step 5: Wire into serve.go**

Add the sweeper start in `cmd/specgraph/serve.go` after creating the server, before `ListenAndServe`:

```go
// After creating the http.Server and before fmt.Printf("SpecGraph server running..."):
server.StartSweeper(ctx, store, 60*time.Second)
```

**Step 6: Commit**

```bash
git add internal/server/sweeper.go internal/server/sweeper_test.go cmd/specgraph/serve.go
git commit -m "feat(execution): background lease sweeper for expired claims"
```

---

## Task 7: CLI — bundle command

**Files:**

- Create: `cmd/specgraph/bundle.go`

**Step 1: Implement the CLI command**

`cmd/specgraph/bundle.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func executionClient() (specgraphv1connect.ExecutionServiceClient, error) {
	return newClient(specgraphv1connect.NewExecutionServiceClient)
}

var bundleCmd = &cobra.Command{
	Use:   "bundle <slug>",
	Short: "Generate an execution bundle for a spec",
	Long:  "Generates a lean YAML execution bundle containing the spec snapshot, resolved decisions, bootstrap instructions, and callback configuration.",
	Args:  cobra.ExactArgs(1),
	RunE:  runBundle,
}

var (
	bundleOutput   string
	bundleEndpoint string
)

func init() {
	bundleCmd.Flags().StringVarP(&bundleOutput, "output", "o", "", "output file path (default: stdout)")
	bundleCmd.Flags().StringVar(&bundleEndpoint, "endpoint", "", "callback endpoint base URL (overrides server address)")
	rootCmd.AddCommand(bundleCmd)
}

func runBundle(_ *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}

	resp, err := client.GenerateBundle(context.Background(),
		connect.NewRequest(&specv1.GenerateBundleRequest{
			Slug:     args[0],
			Endpoint: bundleEndpoint,
		}))
	if err != nil {
		return fmt.Errorf("generate bundle: %w", err)
	}

	yaml := resp.Msg.BundleYaml
	if yaml == "" {
		return fmt.Errorf("server returned empty bundle YAML")
	}

	if bundleOutput != "" {
		if err := os.WriteFile(bundleOutput, []byte(yaml), 0o644); err != nil {
			return fmt.Errorf("write bundle: %w", err)
		}
		fmt.Printf("Bundle written to %s\n", bundleOutput)
		return nil
	}

	fmt.Print(yaml)
	return nil
}
```

**Step 2: Verify build**

```bash
go build ./cmd/specgraph/
./specgraph bundle --help
```

Expected:

```text
Generate an execution bundle for a spec

Usage:
  specgraph bundle <slug> [flags]

Flags:
      --endpoint string   callback endpoint base URL (overrides server address)
  -h, --help              help for bundle
  -o, --output string     output file path (default: stdout)
```

**Step 3: Commit**

```bash
git add cmd/specgraph/bundle.go
git commit -m "feat(execution): CLI bundle command for generating execution bundles"
```

---

## Task 8: CLI — progress command

**Files:**

- Create: `cmd/specgraph/progress.go`

**Step 1: Implement the CLI command**

`cmd/specgraph/progress.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var progressCmd = &cobra.Command{
	Use:   "progress <slug>",
	Short: "Show execution progress for a spec",
	Long:  "Displays execution events (progress, blockers, completion) reported by agents for the given spec.",
	Args:  cobra.ExactArgs(1),
	RunE:  runProgress,
}

var progressLimit int

func init() {
	progressCmd.Flags().IntVar(&progressLimit, "limit", 20, "maximum number of events to show")
	rootCmd.AddCommand(progressCmd)
}

func runProgress(_ *cobra.Command, args []string) error {
	client, err := executionClient()
	if err != nil {
		return err
	}

	resp, err := client.GetExecutionEvents(context.Background(),
		connect.NewRequest(&specv1.GetExecutionEventsRequest{
			Slug:  args[0],
			Limit: int32(progressLimit),
		}))
	if err != nil {
		return fmt.Errorf("get execution events: %w", err)
	}

	events := resp.Msg.Events
	if len(events) == 0 {
		fmt.Println("No execution events found.")
		return nil
	}

	fmt.Printf("Execution events for %s:\n\n", args[0])
	for _, evt := range events {
		ts := "unknown"
		if evt.CreatedAt != nil {
			ts = evt.CreatedAt.AsTime().Format(time.RFC3339)
		}
		typeLabel := eventTypeLabel(evt.Type)
		fmt.Printf("  [%s] %s (%s) — %s\n", ts, typeLabel, evt.Agent, evt.Message)
	}

	return nil
}

func eventTypeLabel(t specv1.ExecutionEventType) string {
	switch t {
	case specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS:
		return "PROGRESS"
	case specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER:
		return "BLOCKER"
	case specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION:
		return "COMPLETE"
	default:
		return "UNKNOWN"
	}
}
```

**Step 2: Verify build**

```bash
go build ./cmd/specgraph/
./specgraph progress --help
```

Expected:

```text
Displays execution events (progress, blockers, completion) reported by agents for the given spec.

Usage:
  specgraph progress <slug> [flags]

Flags:
  -h, --help        help for progress
      --limit int   maximum number of events to show (default 20)
```

**Step 3: Commit**

```bash
git add cmd/specgraph/progress.go
git commit -m "feat(execution): CLI progress command for viewing execution events"
```

---

## Task 9: Integration — Wire Everything Together

**Files:**

- Modify: `cmd/specgraph/serve.go`

**Step 1: Verify serve.go has all registrations and sweeper**

Ensure `cmd/specgraph/serve.go` includes:

```go
server.RegisterExecutionService(mux, store)
server.StartSweeper(ctx, store, 60*time.Second)
```

Both should already be added from Tasks 4 and 6.

**Step 2: Verify full integration**

```bash
go build ./cmd/specgraph/
go test ./... -count=1 -timeout=120s
```

Expected: all tests pass, binary builds

**Step 3: Test CLI help for all new commands**

```bash
./specgraph --help
./specgraph bundle --help
./specgraph progress --help
```

Expected: `bundle` and `progress` appear in the command list

**Step 4: Commit if any changes needed**

```bash
git add cmd/specgraph/serve.go
git commit -m "feat(execution): wire execution service and sweeper into server startup"
```

---

## Task 10: Final Verification and Cleanup

**Step 1: Run all tests**

```bash
go test ./... -count=1 -timeout=120s
```

Expected: all tests pass

**Step 2: Run linter**

```bash
golangci-lint run ./...
```

Expected: no issues (fix any that appear)

**Step 3: Verify full CLI works**

```bash
go build -o specgraph ./cmd/specgraph
./specgraph --help
./specgraph bundle --help
./specgraph progress --help
```

Expected: all commands show help

**Step 4: Run buf lint**

```bash
buf lint
```

Expected: no issues with the new `execution.proto`

**Step 5: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore(execution): cleanup and final verification"
```

---

## Summary

| Task | What | Files | Test Type |
|------|------|-------|-----------|
| 1 | Proto schema | `proto/specgraph/v1/execution.proto` | Compile |
| 2 | Storage interface | `internal/storage/execution.go` | Compile |
| 3 | Memgraph backend | `internal/storage/memgraph/execution.go` | Integration (testcontainers) |
| 4 | ConnectRPC handler | `internal/server/execution_handler.go` | Unit (mock backend) |
| 5 | Bundle YAML rendering | `internal/server/execution_handler.go` (verify) | Unit |
| 6 | Lease sweeper | `internal/server/sweeper.go` | Unit (timer-based) |
| 7 | CLI: bundle | `cmd/specgraph/bundle.go` | Build verification |
| 8 | CLI: progress | `cmd/specgraph/progress.go` | Build verification |
| 9 | Wire together | `cmd/specgraph/serve.go` | Integration |
| 10 | Final verification | -- | All tests + lint |
