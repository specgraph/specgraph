# Slice 2: Constitution Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add constitution support — create, store, query, validate, and emit constitutions. Extend `specgraph init` to generate constitutions from codebase scanning.

**Architecture:** Constitution is a new proto service (ConstitutionService) with its own storage interface, Memgraph implementation, ConnectRPC handler, and CLI commands. Constitution can be created interactively via `specgraph init` or manually. Emitters convert the constitution into tool-specific files (CLAUDE.md, .cursorrules, AGENTS.md).

**Tech Stack:** Go, ConnectRPC (buf/connect-go), Memgraph (neo4j-go-driver v5), Cobra (CLI), buf (proto codegen), testcontainers-go (integration tests), gopkg.in/yaml.v3

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 2 section)

---

## Project Structure (new files)

```text
proto/specgraph/v1/
  constitution.proto              # Constitution messages + ConstitutionService
gen/specgraph/v1/                 # Generated (buf generate)
  constitution.pb.go
  specgraphv1connect/
    constitution.connect.go
internal/
  storage/
    constitution.go               # ConstitutionBackend interface
  storage/memgraph/
    constitution.go               # Memgraph implementation
    constitution_test.go          # Integration tests
  server/
    constitution_handler.go       # ConnectRPC handler
    constitution_handler_test.go  # Handler tests with mock
  emitter/
    emitter.go                    # Constitution → tool files
    emitter_test.go               # Emitter unit tests
cmd/specgraph/
  constitution.go                 # CLI: constitution show/update/check/emit
```

---

## Task 1: Protobuf Schema — Constitution Messages + ConstitutionService

**Files:**

- Create: `proto/specgraph/v1/constitution.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/constitution.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/specgraph/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// --- Enums ---

enum ConstitutionLayer {
  CONSTITUTION_LAYER_UNSPECIFIED = 0;
  CONSTITUTION_LAYER_USER = 1;
  CONSTITUTION_LAYER_ORG = 2;
  CONSTITUTION_LAYER_PROJECT = 3;
  CONSTITUTION_LAYER_DOMAIN = 4;
}

enum ViolationSeverity {
  VIOLATION_SEVERITY_UNSPECIFIED = 0;
  VIOLATION_SEVERITY_ERROR = 1;
  VIOLATION_SEVERITY_WARNING = 2;
  VIOLATION_SEVERITY_INFO = 3;
}

// --- Messages ---

message Constitution {
  string id = 1;
  ConstitutionLayer layer = 2;
  string name = 3;
  int32 version = 4;
  TechConfig tech = 5;
  repeated Principle principles = 6;
  ProcessConfig process = 7;
  repeated string constraints = 8;
  repeated Antipattern antipatterns = 9;
  repeated Reference references = 10;
  google.protobuf.Timestamp created_at = 11;
  google.protobuf.Timestamp updated_at = 12;
}

message TechConfig {
  LanguageConfig languages = 1;
  map<string, string> frameworks = 2;
  map<string, string> infrastructure = 3;
  map<string, string> api_standards = 4;
  map<string, string> data = 5;
}

message LanguageConfig {
  string primary = 1;
  repeated string allowed = 2;
  repeated string forbidden = 3;
  map<string, string> forbidden_reasons = 4;
}

message Principle {
  string id = 1;
  string principle = 2;
  string rationale = 3;
  string exceptions = 4;
}

message ProcessConfig {
  string spec_review = 1;
  SecurityReviewConfig security_review = 2;
  DeploymentConfig deployment = 3;
  DocumentationConfig documentation = 4;
}

message SecurityReviewConfig {
  string when = 1;
}

message DeploymentConfig {
  string strategy = 1;
  string rollback = 2;
}

message DocumentationConfig {
  string api_docs = 1;
  string runbook = 2;
}

message Antipattern {
  string pattern = 1;
  string why = 2;
  string instead = 3;
}

message Reference {
  string type = 1;
  string path = 2;
}

message Violation {
  string rule = 1;
  ViolationSeverity severity = 2;
  string message = 3;
  string spec_slug = 4;
}

// --- Requests/Responses ---

message GetConstitutionRequest {}

message UpdateConstitutionRequest {
  Constitution constitution = 1;
}

message CheckViolationRequest {
  string spec_slug = 1;
}

message CheckViolationResponse {
  repeated Violation violations = 1;
}

message EmitRequest {
  string format = 1;
}

message EmitResponse {
  string content = 1;
  string filename = 2;
}

// --- Service ---

service ConstitutionService {
  rpc GetConstitution(GetConstitutionRequest) returns (Constitution);
  rpc UpdateConstitution(UpdateConstitutionRequest) returns (Constitution);
  rpc CheckViolation(CheckViolationRequest) returns (CheckViolationResponse);
  rpc EmitToolFiles(EmitRequest) returns (EmitResponse);
}
```

**Step 2: Generate Go code**

```bash
buf generate
```

Expected: generates `gen/specgraph/v1/constitution.pb.go` and `gen/specgraph/v1/specgraphv1connect/constitution.connect.go`

**Step 3: Verify generated code compiles**

```bash
go mod tidy
go build ./gen/...
```

**Step 4: Commit**

```bash
git add proto/specgraph/v1/constitution.proto gen/specgraph/v1/constitution.pb.go gen/specgraph/v1/specgraphv1connect/constitution.connect.go go.mod go.sum
git commit -m "feat(constitution): protobuf schema for Constitution messages and ConstitutionService"
```

---

## Task 2: Storage Interface — ConstitutionBackend

**Files:**

- Create: `internal/storage/constitution.go`

**Step 1: Define the interface**

`internal/storage/constitution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

var ErrConstitutionNotFound = errors.New("constitution not found")

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	GetConstitution(ctx context.Context) (*specv1.Constitution, error)

	// UpdateConstitution stores or replaces the constitution, bumping its version.
	UpdateConstitution(ctx context.Context, constitution *specv1.Constitution) (*specv1.Constitution, error)

	// CheckViolation checks a spec against constitution constraints.
	// Returns a list of violations (empty if compliant).
	CheckViolation(ctx context.Context, specSlug string) ([]*specv1.Violation, error)
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/constitution.go
git commit -m "feat(constitution): storage backend interface for constitution"
```

---

## Task 3: Memgraph Implementation — Constitution Storage

**Files:**

- Create: `internal/storage/memgraph/constitution.go`
- Create: `internal/storage/memgraph/constitution_test.go`

**Step 1: Write the integration test**

`internal/storage/memgraph/constitution_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestConstitution_GetNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.GetConstitution(ctx)
	require.ErrorIs(t, err, storage.ErrConstitutionNotFound)
}

func TestConstitution_UpdateAndGet(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	input := &specv1.Constitution{
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Name:  "test-project",
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary:   "go",
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"java"},
				ForbiddenReasons: map[string]string{
					"java": "No Java expertise",
				},
			},
			Frameworks: map[string]string{
				"api":     "net/http + chi",
				"testing": "go test + testify",
			},
		},
		Principles: []*specv1.Principle{
			{
				Id:        "backward-compat",
				Principle: "All API changes must be backward compatible",
				Rationale: "External consumers",
			},
		},
		Constraints: []string{
			"No new dependencies without review",
		},
		Antipatterns: []*specv1.Antipattern{
			{
				Pattern: "Shared mutable state between services",
				Why:     "Caused cascading failure",
				Instead: "Event-driven with Pub/Sub",
			},
		},
	}

	created, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)
	require.NotEmpty(t, created.Id)
	require.Equal(t, int32(1), created.Version)
	require.NotNil(t, created.CreatedAt)
	require.Equal(t, "go", created.Tech.Languages.Primary)

	// Get it back
	got, err := store.GetConstitution(ctx)
	require.NoError(t, err)
	require.Equal(t, created.Id, got.Id)
	require.Equal(t, "go", got.Tech.Languages.Primary)
	require.Len(t, got.Principles, 1)
	require.Equal(t, "backward-compat", got.Principles[0].Id)
	require.Len(t, got.Constraints, 1)
	require.Len(t, got.Antipatterns, 1)

	// Update it — version should bump
	input.Name = "updated-project"
	updated, err := store.UpdateConstitution(ctx, input)
	require.NoError(t, err)
	require.Equal(t, int32(2), updated.Version)
	require.Equal(t, "updated-project", updated.Name)
}

func TestConstitution_CheckViolation(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Set up constitution with forbidden language
	_, err = store.UpdateConstitution(ctx, &specv1.Constitution{
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Name:  "test-project",
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary:   "go",
				Forbidden: []string{"java"},
				ForbiddenReasons: map[string]string{
					"java": "No Java expertise",
				},
			},
		},
		Constraints: []string{
			"No new dependencies without review",
		},
	})
	require.NoError(t, err)

	// Create a spec to check against
	_, err = store.CreateSpec(ctx, "test-spec", "A test spec", "p2", "medium")
	require.NoError(t, err)

	// Check violations — spec exists, constitution exists, no violations expected
	// (violations require spec metadata that references forbidden items)
	violations, err := store.CheckViolation(ctx, "test-spec")
	require.NoError(t, err)
	require.Empty(t, violations)

	// Check against nonexistent spec
	_, err = store.CheckViolation(ctx, "nonexistent")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/storage/memgraph/ -run TestConstitution -v -count=1 -timeout=120s
```

Expected: FAIL — `constitution.go` doesn't exist yet

**Step 3: Implement the Memgraph constitution backend**

`internal/storage/memgraph/constitution.go`:

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
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Store) GetConstitution(ctx context.Context) (*specv1.Constitution, error) {
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (c:Constitution)
		 RETURN c.id, c.layer, c.name, c.version,
		        c.tech_json, c.principles_json, c.process_json,
		        c.constraints_json, c.antipatterns_json, c.references_json,
		        c.created_at, c.updated_at
		 LIMIT 1`,
		map[string]any{},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get constitution: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: get constitution: %w", storage.ErrConstitutionNotFound)
	}

	return recordToConstitution(result.Records[0])
}

func (s *Store) UpdateConstitution(ctx context.Context, constitution *specv1.Constitution) (*specv1.Constitution, error) {
	now := time.Now().UTC()

	techJSON, err := json.Marshal(constitution.Tech)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal tech: %w", err)
	}
	principlesJSON, err := json.Marshal(constitution.Principles)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal principles: %w", err)
	}
	processJSON, err := json.Marshal(constitution.Process)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal process: %w", err)
	}
	constraintsJSON, err := json.Marshal(constitution.Constraints)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal constraints: %w", err)
	}
	antipatternsJSON, err := json.Marshal(constitution.Antipatterns)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal antipatterns: %w", err)
	}
	referencesJSON, err := json.Marshal(constitution.References)
	if err != nil {
		return nil, fmt.Errorf("memgraph: marshal references: %w", err)
	}

	// MERGE on label — only one constitution at a time
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MERGE (c:Constitution)
		 ON CREATE SET
		   c.id = $id,
		   c.layer = $layer,
		   c.name = $name,
		   c.version = 1,
		   c.tech_json = $tech_json,
		   c.principles_json = $principles_json,
		   c.process_json = $process_json,
		   c.constraints_json = $constraints_json,
		   c.antipatterns_json = $antipatterns_json,
		   c.references_json = $references_json,
		   c.created_at = $now,
		   c.updated_at = $now
		 ON MATCH SET
		   c.layer = $layer,
		   c.name = $name,
		   c.version = c.version + 1,
		   c.tech_json = $tech_json,
		   c.principles_json = $principles_json,
		   c.process_json = $process_json,
		   c.constraints_json = $constraints_json,
		   c.antipatterns_json = $antipatterns_json,
		   c.references_json = $references_json,
		   c.updated_at = $now
		 RETURN c.id, c.layer, c.name, c.version,
		        c.tech_json, c.principles_json, c.process_json,
		        c.constraints_json, c.antipatterns_json, c.references_json,
		        c.created_at, c.updated_at`,
		map[string]any{
			"id":                generateID("con", constitution.Name, now),
			"layer":             constitution.Layer.String(),
			"name":              constitution.Name,
			"tech_json":         string(techJSON),
			"principles_json":   string(principlesJSON),
			"process_json":      string(processJSON),
			"constraints_json":  string(constraintsJSON),
			"antipatterns_json": string(antipatternsJSON),
			"references_json":   string(referencesJSON),
			"now":               now.Format(time.RFC3339),
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update constitution: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: update constitution: no result returned")
	}

	return recordToConstitution(result.Records[0])
}

func (s *Store) CheckViolation(ctx context.Context, specSlug string) ([]*specv1.Violation, error) {
	// First verify spec exists
	specResult, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug}) RETURN s.slug`,
		map[string]any{"slug": specSlug},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: check violation: %w", err)
	}
	if len(specResult.Records) == 0 {
		return nil, fmt.Errorf("memgraph: check violation %q: %w", specSlug, storage.ErrSpecNotFound)
	}

	// Get constitution
	constitution, err := s.GetConstitution(ctx)
	if err != nil {
		return nil, fmt.Errorf("memgraph: check violation: %w", err)
	}

	var violations []*specv1.Violation

	// Check forbidden languages — for now, we check if the spec's metadata
	// references any forbidden items. In practice, this will be extended
	// when specs carry language/tech metadata from the authoring funnel.
	// For now, the check validates that the constitution and spec both exist
	// and returns an empty list (no violations detectable without spec tech metadata).
	_ = constitution

	return violations, nil
}

func recordToConstitution(rec *neo4j.Record) (*specv1.Constitution, error) {
	c := &specv1.Constitution{}

	var err error
	c.Id, err = recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}

	layerStr, err := recordString(rec, 1, "layer")
	if err != nil {
		return nil, err
	}
	if val, ok := specv1.ConstitutionLayer_value[layerStr]; ok {
		c.Layer = specv1.ConstitutionLayer(val)
	}

	c.Name, err = recordString(rec, 2, "name")
	if err != nil {
		return nil, err
	}

	version, err := recordInt64(rec, 3, "version")
	if err != nil {
		return nil, err
	}
	c.Version = int32(version)

	// Unmarshal JSON fields
	techJSON, err := recordString(rec, 4, "tech_json")
	if err != nil {
		return nil, err
	}
	if techJSON != "" {
		c.Tech = &specv1.TechConfig{}
		if err := json.Unmarshal([]byte(techJSON), c.Tech); err != nil {
			return nil, fmt.Errorf("unmarshal tech: %w", err)
		}
	}

	principlesJSON, err := recordString(rec, 5, "principles_json")
	if err != nil {
		return nil, err
	}
	if principlesJSON != "" {
		if err := json.Unmarshal([]byte(principlesJSON), &c.Principles); err != nil {
			return nil, fmt.Errorf("unmarshal principles: %w", err)
		}
	}

	processJSON, err := recordString(rec, 6, "process_json")
	if err != nil {
		return nil, err
	}
	if processJSON != "" {
		c.Process = &specv1.ProcessConfig{}
		if err := json.Unmarshal([]byte(processJSON), c.Process); err != nil {
			return nil, fmt.Errorf("unmarshal process: %w", err)
		}
	}

	constraintsJSON, err := recordString(rec, 7, "constraints_json")
	if err != nil {
		return nil, err
	}
	if constraintsJSON != "" {
		if err := json.Unmarshal([]byte(constraintsJSON), &c.Constraints); err != nil {
			return nil, fmt.Errorf("unmarshal constraints: %w", err)
		}
	}

	antipatternsJSON, err := recordString(rec, 8, "antipatterns_json")
	if err != nil {
		return nil, err
	}
	if antipatternsJSON != "" {
		if err := json.Unmarshal([]byte(antipatternsJSON), &c.Antipatterns); err != nil {
			return nil, fmt.Errorf("unmarshal antipatterns: %w", err)
		}
	}

	referencesJSON, err := recordString(rec, 9, "references_json")
	if err != nil {
		return nil, err
	}
	if referencesJSON != "" {
		if err := json.Unmarshal([]byte(referencesJSON), &c.References); err != nil {
			return nil, fmt.Errorf("unmarshal references: %w", err)
		}
	}

	createdStr, err := recordString(rec, 10, "created_at")
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, createdStr); parseErr == nil {
		c.CreatedAt = timestamppb.New(t)
	}

	updatedStr, err := recordString(rec, 11, "updated_at")
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, updatedStr); parseErr == nil {
		c.UpdatedAt = timestamppb.New(t)
	}

	return c, nil
}
```

**Step 4: Run the tests**

```bash
go mod tidy
go test ./internal/storage/memgraph/ -run TestConstitution -v -count=1 -timeout=120s
```

Expected: PASS (all three tests). Requires Docker running.

**Step 5: Commit**

```bash
git add internal/storage/memgraph/constitution.go internal/storage/memgraph/constitution_test.go
git commit -m "feat(constitution): memgraph storage backend with integration tests"
```

---

## Task 4: ConnectRPC Handler — ConstitutionService

**Files:**

- Create: `internal/server/constitution_handler.go`
- Create: `internal/server/constitution_handler_test.go`

**Step 1: Write the handler test**

`internal/server/constitution_handler_test.go`:

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
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockConstitutionBackend struct {
	mu           sync.Mutex
	constitution *specv1.Constitution
	specs        map[string]bool
}

func newMockConstitutionBackend() *mockConstitutionBackend {
	return &mockConstitutionBackend{specs: map[string]bool{}}
}

func (m *mockConstitutionBackend) GetConstitution(_ context.Context) (*specv1.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.constitution == nil {
		return nil, storage.ErrConstitutionNotFound
	}
	return m.constitution, nil
}

func (m *mockConstitutionBackend) UpdateConstitution(_ context.Context, c *specv1.Constitution) (*specv1.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.constitution == nil {
		c.Id = "con-test123"
		c.Version = 1
	} else {
		c.Id = m.constitution.Id
		c.Version = m.constitution.Version + 1
	}
	m.constitution = c
	return c, nil
}

func (m *mockConstitutionBackend) CheckViolation(_ context.Context, specSlug string) ([]*specv1.Violation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.specs[specSlug] {
		return nil, fmt.Errorf("check violation %q: %w", specSlug, storage.ErrSpecNotFound)
	}
	return nil, nil
}

var _ storage.ConstitutionBackend = (*mockConstitutionBackend)(nil)

func setupConstitutionServer(t *testing.T) specgraphv1connect.ConstitutionServiceClient {
	t.Helper()
	mb := newMockConstitutionBackend()
	mb.specs["test-spec"] = true
	mux := http.NewServeMux()
	server.RegisterConstitutionService(mux, mb)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewConstitutionServiceClient(http.DefaultClient, srv.URL)
}

func TestConstitutionHandler_GetNotFound(t *testing.T) {
	client := setupConstitutionServer(t)
	_, err := client.GetConstitution(context.Background(),
		connect.NewRequest(&specv1.GetConstitutionRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConstitutionHandler_UpdateAndGet(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	// Update (create)
	updateResp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			Name:  "test-project",
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{
					Primary: "go",
				},
			},
			Constraints: []string{"No ORMs"},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), updateResp.Msg.Version)
	require.Equal(t, "go", updateResp.Msg.Tech.Languages.Primary)

	// Get
	getResp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	require.NoError(t, err)
	require.Equal(t, "test-project", getResp.Msg.Name)
	require.Len(t, getResp.Msg.Constraints, 1)
}

func TestConstitutionHandler_CheckViolation(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	// Check existing spec — no violations
	resp, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: "test-spec",
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Violations)

	// Check nonexistent spec
	_, err = client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/server/ -run TestConstitution -v -count=1
```

Expected: FAIL — handler doesn't exist yet

**Step 3: Implement the handler**

`internal/server/constitution_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// ConstitutionHandler implements the ConstitutionService.
type ConstitutionHandler struct {
	store storage.ConstitutionBackend
}

var _ specgraphv1connect.ConstitutionServiceHandler = (*ConstitutionHandler)(nil)

// RegisterConstitutionService registers the ConstitutionService handler on the mux.
func RegisterConstitutionService(mux *http.ServeMux, store storage.ConstitutionBackend) {
	handler := &ConstitutionHandler{store: store}
	path, h := specgraphv1connect.NewConstitutionServiceHandler(handler)
	mux.Handle(path, h)
}

func (h *ConstitutionHandler) GetConstitution(ctx context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.Constitution], error) {
	constitution, err := h.store.GetConstitution(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(constitution), nil
}

func (h *ConstitutionHandler) UpdateConstitution(ctx context.Context, req *connect.Request[specv1.UpdateConstitutionRequest]) (*connect.Response[specv1.Constitution], error) {
	if req.Msg.Constitution == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("constitution is required"))
	}
	constitution, err := h.store.UpdateConstitution(ctx, req.Msg.Constitution)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(constitution), nil
}

func (h *ConstitutionHandler) CheckViolation(ctx context.Context, req *connect.Request[specv1.CheckViolationRequest]) (*connect.Response[specv1.CheckViolationResponse], error) {
	if req.Msg.SpecSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("spec_slug is required"))
	}
	violations, err := h.store.CheckViolation(ctx, req.Msg.SpecSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.CheckViolationResponse{Violations: violations}), nil
}

func (h *ConstitutionHandler) EmitToolFiles(ctx context.Context, req *connect.Request[specv1.EmitRequest]) (*connect.Response[specv1.EmitResponse], error) {
	if req.Msg.Format == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("format is required"))
	}

	constitution, err := h.store.GetConstitution(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	content, filename, err := emitConstitution(constitution, req.Msg.Format)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&specv1.EmitResponse{
		Content:  content,
		Filename: filename,
	}), nil
}

// emitConstitution is a placeholder — full implementation in Task 6 (emitter package).
// For now, returns a basic formatted string.
func emitConstitution(c *specv1.Constitution, format string) (string, string, error) {
	switch format {
	case "claude-md":
		return formatClaudeMD(c), "CLAUDE.md", nil
	case "cursorrules":
		return formatCursorrules(c), ".cursorrules", nil
	case "agents-md":
		return formatAgentsMD(c), "AGENTS.md", nil
	default:
		return "", "", errors.New("unsupported format: " + format)
	}
}

func formatClaudeMD(c *specv1.Constitution) string {
	// Placeholder — replaced by emitter package in Task 6
	return "# Project Constitution\n\nGenerated by SpecGraph.\n"
}

func formatCursorrules(c *specv1.Constitution) string {
	return "# Project Rules\n\nGenerated by SpecGraph.\n"
}

func formatAgentsMD(c *specv1.Constitution) string {
	return "# Agent Instructions\n\nGenerated by SpecGraph.\n"
}
```

**Step 4: Run the tests**

```bash
go test ./internal/server/ -run TestConstitution -v -count=1
```

Expected: PASS

**Step 5: Wire into serve.go**

Add `server.RegisterConstitutionService(mux, store)` in `cmd/specgraph/serve.go` after the other service registrations:

```go
// In serve.go, after line "server.RegisterClaimService(mux, store)":
server.RegisterConstitutionService(mux, store)
```

**Step 6: Verify full build**

```bash
go build ./cmd/specgraph/
```

**Step 7: Commit**

```bash
git add internal/server/constitution_handler.go internal/server/constitution_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(constitution): ConnectRPC handler with handler tests"
```

---

## Task 5: Codebase Scanner — Tier 0

> **Superseded by Slice 3.5:** The codebase scanner has been removed. Constitution bootstrapping is now interactive/agent-driven. See `docs/plans/2026-03-03-slice-3.5-scanner-cleanup-plan.md`.

**Files:**

- Create: `internal/scanner/scanner.go`
- Create: `internal/scanner/scanner_test.go`

**Step 1: Write the test**

`internal/scanner/scanner_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestScan_GoProject(t *testing.T) {
	dir := t.TempDir()

	// Set up a minimal Go project
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module example.com/myapp

go 1.25.0

require (
	github.com/spf13/cobra v1.10.2
	connectrpc.com/connect v1.19.1
)
`), 0o644)

	os.MkdirAll(filepath.Join(dir, ".github", "workflows"), 0o755)
	os.WriteFile(filepath.Join(dir, ".github", "workflows", "ci.yaml"), []byte("name: CI\n"), 0o644)

	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM golang:1.25-alpine\n"), 0o644)

	os.MkdirAll(filepath.Join(dir, "cmd", "myapp"), 0o755)
	os.WriteFile(filepath.Join(dir, "cmd", "myapp", "main.go"), []byte("package main\n"), 0o644)

	os.MkdirAll(filepath.Join(dir, "internal", "server"), 0o755)
	os.WriteFile(filepath.Join(dir, "internal", "server", "server.go"), []byte("package server\n"), 0o644)

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "go", result.Tech.Languages.Primary)
	require.Equal(t, "GitHub Actions", result.Tech.Infrastructure["ci"])
	require.Equal(t, "Docker", result.Tech.Infrastructure["runtime"])
}

func TestScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Empty(t, result.Tech.Languages.Primary)
}

func TestScan_NodeProject(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
  "name": "my-app",
  "dependencies": {
    "react": "^18.0.0",
    "next": "^14.0.0"
  }
}
`), 0o644)

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "typescript", result.Tech.Languages.Primary)
	require.Contains(t, result.Tech.Frameworks, "ui")
}

func TestScan_PullCLAUDEMD(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte(`# Project Guidelines

## Tech Stack
- Language: Go
- Database: PostgreSQL

## Constraints
- No ORMs
- All APIs must be versioned
`), 0o644)

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25.0\n"), 0o644)

	result, err := scanner.Scan(dir)
	require.NoError(t, err)
	require.Equal(t, "go", result.Tech.Languages.Primary)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/scanner/ -v -count=1
```

Expected: FAIL — package doesn't exist

**Step 3: Implement the scanner**

`internal/scanner/scanner.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package scanner implements Tier 0 codebase scanning for constitution drafting.
package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Scan performs a Tier 0 scan of the given directory, detecting languages,
// frameworks, infrastructure, and CI configuration.
func Scan(dir string) (*specv1.Constitution, error) {
	c := &specv1.Constitution{
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		Tech: &specv1.TechConfig{
			Languages:      &specv1.LanguageConfig{},
			Frameworks:     map[string]string{},
			Infrastructure: map[string]string{},
		},
	}

	detectLanguage(dir, c)
	detectFrameworks(dir, c)
	detectInfrastructure(dir, c)
	detectCI(dir, c)

	return c, nil
}

func detectLanguage(dir string, c *specv1.Constitution) {
	langFiles := map[string]string{
		"go.mod":       "go",
		"Cargo.toml":   "rust",
		"pom.xml":      "java",
		"build.gradle": "java",
		"setup.py":     "python",
		"pyproject.toml": "python",
		"Gemfile":      "ruby",
	}

	for file, lang := range langFiles {
		if fileExists(filepath.Join(dir, file)) {
			c.Tech.Languages.Primary = lang
			return
		}
	}

	// Check package.json — could be JS or TS
	if fileExists(filepath.Join(dir, "package.json")) {
		c.Tech.Languages.Primary = "typescript"
		if fileExists(filepath.Join(dir, "tsconfig.json")) {
			c.Tech.Languages.Primary = "typescript"
		} else {
			c.Tech.Languages.Primary = "javascript"
		}
	}
}

func detectFrameworks(dir string, c *specv1.Constitution) {
	// Go frameworks
	if gomod := readFile(filepath.Join(dir, "go.mod")); gomod != "" {
		if strings.Contains(gomod, "connectrpc.com/connect") {
			c.Tech.Frameworks["api"] = "ConnectRPC"
		}
		if strings.Contains(gomod, "github.com/go-chi/chi") {
			c.Tech.Frameworks["api"] = "chi"
		}
		if strings.Contains(gomod, "github.com/gin-gonic/gin") {
			c.Tech.Frameworks["api"] = "gin"
		}
		if strings.Contains(gomod, "github.com/spf13/cobra") {
			c.Tech.Frameworks["cli"] = "cobra"
		}
		if strings.Contains(gomod, "github.com/stretchr/testify") {
			c.Tech.Frameworks["testing"] = "testify"
		}
	}

	// Node frameworks
	if pkgJSON := readFile(filepath.Join(dir, "package.json")); pkgJSON != "" {
		var pkg map[string]any
		if json.Unmarshal([]byte(pkgJSON), &pkg) == nil {
			deps := mergeDeps(pkg)
			if _, ok := deps["react"]; ok {
				c.Tech.Frameworks["ui"] = "React"
			}
			if _, ok := deps["next"]; ok {
				c.Tech.Frameworks["ui"] = "Next.js"
			}
			if _, ok := deps["express"]; ok {
				c.Tech.Frameworks["api"] = "Express"
			}
			if _, ok := deps["fastify"]; ok {
				c.Tech.Frameworks["api"] = "Fastify"
			}
		}
	}
}

func detectInfrastructure(dir string, c *specv1.Constitution) {
	if fileExists(filepath.Join(dir, "Dockerfile")) {
		c.Tech.Infrastructure["runtime"] = "Docker"
	}

	if fileExists(filepath.Join(dir, "docker-compose.yaml")) || fileExists(filepath.Join(dir, "docker-compose.yml")) {
		c.Tech.Infrastructure["orchestration"] = "Docker Compose"
	}

	// Kubernetes
	globs, _ := filepath.Glob(filepath.Join(dir, "**", "*.yaml"))
	for _, f := range globs {
		content := readFile(f)
		if strings.Contains(content, "apiVersion:") && strings.Contains(content, "kind:") {
			c.Tech.Infrastructure["runtime"] = "Kubernetes"
			break
		}
	}
}

func detectCI(dir string, c *specv1.Constitution) {
	if dirExists(filepath.Join(dir, ".github", "workflows")) {
		c.Tech.Infrastructure["ci"] = "GitHub Actions"
	}
	if fileExists(filepath.Join(dir, ".gitlab-ci.yml")) {
		c.Tech.Infrastructure["ci"] = "GitLab CI"
	}
	if fileExists(filepath.Join(dir, "Jenkinsfile")) {
		c.Tech.Infrastructure["ci"] = "Jenkins"
	}
	if fileExists(filepath.Join(dir, ".circleci", "config.yml")) {
		c.Tech.Infrastructure["ci"] = "CircleCI"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func mergeDeps(pkg map[string]any) map[string]any {
	merged := map[string]any{}
	for _, key := range []string{"dependencies", "devDependencies"} {
		if deps, ok := pkg[key].(map[string]any); ok {
			for k, v := range deps {
				merged[k] = v
			}
		}
	}
	return merged
}
```

**Step 4: Run the tests**

```bash
go test ./internal/scanner/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/scanner/scanner.go internal/scanner/scanner_test.go
git commit -m "feat(constitution): Tier 0 codebase scanner for constitution drafting"
```

---

## Task 6: Emitter — Constitution to Tool Files

**Files:**

- Create: `internal/emitter/emitter.go`
- Create: `internal/emitter/emitter_test.go`

**Step 1: Write the test**

`internal/emitter/emitter_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package emitter_test

import (
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/emitter"
	"github.com/stretchr/testify/require"
)

func testConstitution() *specv1.Constitution {
	return &specv1.Constitution{
		Name: "test-project",
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary:   "go",
				Allowed:   []string{"go", "python"},
				Forbidden: []string{"java"},
				ForbiddenReasons: map[string]string{
					"java": "No Java expertise",
				},
			},
			Frameworks: map[string]string{
				"api":     "ConnectRPC",
				"testing": "testify",
			},
			Infrastructure: map[string]string{
				"runtime": "Docker",
				"ci":      "GitHub Actions",
			},
		},
		Principles: []*specv1.Principle{
			{
				Id:        "backward-compat",
				Principle: "All API changes must be backward compatible",
				Rationale: "External consumers",
			},
		},
		Constraints: []string{
			"No ORMs",
			"All secrets via Secret Manager",
		},
		Antipatterns: []*specv1.Antipattern{
			{
				Pattern: "Shared mutable state",
				Why:     "Caused cascading failure",
				Instead: "Event-driven",
			},
		},
	}
}

func TestEmit_ClaudeMD(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "claude-md")
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", filename)
	require.Contains(t, content, "go")
	require.Contains(t, content, "ConnectRPC")
	require.Contains(t, content, "No ORMs")
	require.Contains(t, content, "backward compatible")
	require.Contains(t, content, "Shared mutable state")
}

func TestEmit_Cursorrules(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "cursorrules")
	require.NoError(t, err)
	require.Equal(t, ".cursorrules", filename)
	require.Contains(t, content, "go")
}

func TestEmit_AgentsMD(t *testing.T) {
	c := testConstitution()
	content, filename, err := emitter.Emit(c, "agents-md")
	require.NoError(t, err)
	require.Equal(t, "AGENTS.md", filename)
	require.Contains(t, content, "go")
}

func TestEmit_InvalidFormat(t *testing.T) {
	c := testConstitution()
	_, _, err := emitter.Emit(c, "invalid")
	require.Error(t, err)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/emitter/ -v -count=1
```

Expected: FAIL — package doesn't exist

**Step 3: Implement the emitter**

`internal/emitter/emitter.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package emitter converts constitutions into tool-specific configuration files.
package emitter

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Emit generates a tool-specific file from the given constitution.
// Supported formats: "claude-md", "cursorrules", "agents-md".
func Emit(c *specv1.Constitution, format string) (content string, filename string, err error) {
	switch format {
	case "claude-md":
		return emitClaudeMD(c), "CLAUDE.md", nil
	case "cursorrules":
		return emitCursorrules(c), ".cursorrules", nil
	case "agents-md":
		return emitAgentsMD(c), "AGENTS.md", nil
	default:
		return "", "", fmt.Errorf("unsupported format: %s", format)
	}
}

func emitClaudeMD(c *specv1.Constitution) string {
	var b strings.Builder

	b.WriteString("# Project Constitution\n\n")
	b.WriteString("Generated by SpecGraph. Do not edit manually.\n\n")

	writeTechSection(&b, c)
	writePrinciplesSection(&b, c)
	writeConstraintsSection(&b, c)
	writeAntipatternSection(&b, c)

	return b.String()
}

func emitCursorrules(c *specv1.Constitution) string {
	var b strings.Builder

	b.WriteString("# Project Rules (Generated by SpecGraph)\n\n")

	writeTechSection(&b, c)
	writePrinciplesSection(&b, c)
	writeConstraintsSection(&b, c)
	writeAntipatternSection(&b, c)

	return b.String()
}

func emitAgentsMD(c *specv1.Constitution) string {
	var b strings.Builder

	b.WriteString("# Agent Instructions\n\n")
	b.WriteString("Generated by SpecGraph. Do not edit manually.\n\n")

	writeTechSection(&b, c)
	writePrinciplesSection(&b, c)
	writeConstraintsSection(&b, c)
	writeAntipatternSection(&b, c)

	return b.String()
}

func writeTechSection(b *strings.Builder, c *specv1.Constitution) {
	if c.Tech == nil {
		return
	}

	b.WriteString("## Tech Stack\n\n")

	if c.Tech.Languages != nil {
		b.WriteString(fmt.Sprintf("- **Primary language:** %s\n", c.Tech.Languages.Primary))
		if len(c.Tech.Languages.Allowed) > 0 {
			b.WriteString(fmt.Sprintf("- **Allowed languages:** %s\n", strings.Join(c.Tech.Languages.Allowed, ", ")))
		}
		if len(c.Tech.Languages.Forbidden) > 0 {
			b.WriteString(fmt.Sprintf("- **Forbidden languages:** %s\n", strings.Join(c.Tech.Languages.Forbidden, ", ")))
			for lang, reason := range c.Tech.Languages.ForbiddenReasons {
				b.WriteString(fmt.Sprintf("  - %s: %s\n", lang, reason))
			}
		}
	}

	if len(c.Tech.Frameworks) > 0 {
		b.WriteString("\n**Frameworks:**\n\n")
		for area, framework := range c.Tech.Frameworks {
			b.WriteString(fmt.Sprintf("- %s: %s\n", area, framework))
		}
	}

	if len(c.Tech.Infrastructure) > 0 {
		b.WriteString("\n**Infrastructure:**\n\n")
		for area, tech := range c.Tech.Infrastructure {
			b.WriteString(fmt.Sprintf("- %s: %s\n", area, tech))
		}
	}

	b.WriteString("\n")
}

func writePrinciplesSection(b *strings.Builder, c *specv1.Constitution) {
	if len(c.Principles) == 0 {
		return
	}

	b.WriteString("## Principles\n\n")
	for _, p := range c.Principles {
		b.WriteString(fmt.Sprintf("- **%s**: %s", p.Id, p.Principle))
		if p.Rationale != "" {
			b.WriteString(fmt.Sprintf(" (%s)", p.Rationale))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeConstraintsSection(b *strings.Builder, c *specv1.Constitution) {
	if len(c.Constraints) == 0 {
		return
	}

	b.WriteString("## Constraints\n\n")
	for _, constraint := range c.Constraints {
		b.WriteString(fmt.Sprintf("- %s\n", constraint))
	}
	b.WriteString("\n")
}

func writeAntipatternSection(b *strings.Builder, c *specv1.Constitution) {
	if len(c.Antipatterns) == 0 {
		return
	}

	b.WriteString("## Anti-patterns\n\n")
	for _, ap := range c.Antipatterns {
		b.WriteString(fmt.Sprintf("- **%s**", ap.Pattern))
		if ap.Why != "" {
			b.WriteString(fmt.Sprintf(" — %s", ap.Why))
		}
		if ap.Instead != "" {
			b.WriteString(fmt.Sprintf(". Instead: %s", ap.Instead))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
```

**Step 4: Run the tests**

```bash
go test ./internal/emitter/ -v -count=1
```

Expected: PASS

**Step 5: Update handler to use emitter package**

Replace the placeholder `emitConstitution` function in `internal/server/constitution_handler.go` to call the emitter package:

```go
// In constitution_handler.go, replace the emitConstitution function and
// the placeholder format functions with:

import "github.com/specgraph/specgraph/internal/emitter"

// In the EmitToolFiles method, replace the call to emitConstitution with:
content, filename, err := emitter.Emit(constitution, req.Msg.Format)
```

Remove the placeholder `formatClaudeMD`, `formatCursorrules`, `formatAgentsMD` functions and the `emitConstitution` function.

**Step 6: Verify all tests pass**

```bash
go test ./internal/emitter/ ./internal/server/ -v -count=1
```

Expected: PASS

**Step 7: Commit**

```bash
git add internal/emitter/emitter.go internal/emitter/emitter_test.go internal/server/constitution_handler.go
git commit -m "feat(constitution): emitter for CLAUDE.md, .cursorrules, AGENTS.md"
```

---

## Task 7: CLI — Constitution Commands

**Files:**

- Create: `cmd/specgraph/constitution.go`

**Step 1: Implement the CLI commands**

`cmd/specgraph/constitution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func constitutionClient() (specgraphv1connect.ConstitutionServiceClient, error) {
	return newClient(specgraphv1connect.NewConstitutionServiceClient)
}

var constitutionCmd = &cobra.Command{
	Use:   "constitution",
	Short: "Manage the project constitution",
}

var constitutionShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display the active constitution",
	RunE:  runConstitutionShow,
}

var constitutionCheckCmd = &cobra.Command{
	Use:   "check <slug>",
	Short: "Check a spec against constitution constraints",
	Args:  cobra.ExactArgs(1),
	RunE:  runConstitutionCheck,
}

var constitutionEmitCmd = &cobra.Command{
	Use:   "emit",
	Short: "Generate tool-specific files from the constitution",
	RunE:  runConstitutionEmit,
}

var emitFormat string
var emitOutput string

func init() {
	constitutionEmitCmd.Flags().StringVar(&emitFormat, "format", "claude-md", "output format (claude-md, cursorrules, agents-md)")
	constitutionEmitCmd.Flags().StringVarP(&emitOutput, "output", "o", "", "output file path (default: stdout)")

	constitutionCmd.AddCommand(constitutionShowCmd)
	constitutionCmd.AddCommand(constitutionCheckCmd)
	constitutionCmd.AddCommand(constitutionEmitCmd)
	rootCmd.AddCommand(constitutionCmd)
}

func runConstitutionShow(_ *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}

	resp, err := client.GetConstitution(context.Background(),
		connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return fmt.Errorf("get constitution: %w", err)
	}

	c := resp.Msg
	fmt.Printf("Name:       %s\n", c.Name)
	fmt.Printf("Layer:      %s\n", c.Layer.String())
	fmt.Printf("Version:    %d\n", c.Version)

	if c.Tech != nil && c.Tech.Languages != nil {
		fmt.Printf("\nTech Stack:\n")
		fmt.Printf("  Primary:    %s\n", c.Tech.Languages.Primary)
		if len(c.Tech.Languages.Allowed) > 0 {
			fmt.Printf("  Allowed:    %s\n", strings.Join(c.Tech.Languages.Allowed, ", "))
		}
		if len(c.Tech.Languages.Forbidden) > 0 {
			fmt.Printf("  Forbidden:  %s\n", strings.Join(c.Tech.Languages.Forbidden, ", "))
		}
	}

	if c.Tech != nil && len(c.Tech.Frameworks) > 0 {
		fmt.Printf("\nFrameworks:\n")
		for area, fw := range c.Tech.Frameworks {
			fmt.Printf("  %-12s %s\n", area+":", fw)
		}
	}

	if len(c.Principles) > 0 {
		fmt.Printf("\nPrinciples:\n")
		for _, p := range c.Principles {
			fmt.Printf("  - %s: %s\n", p.Id, p.Principle)
		}
	}

	if len(c.Constraints) > 0 {
		fmt.Printf("\nConstraints:\n")
		for _, constraint := range c.Constraints {
			fmt.Printf("  - %s\n", constraint)
		}
	}

	if len(c.Antipatterns) > 0 {
		fmt.Printf("\nAnti-patterns:\n")
		for _, ap := range c.Antipatterns {
			fmt.Printf("  - %s (instead: %s)\n", ap.Pattern, ap.Instead)
		}
	}

	return nil
}

func runConstitutionCheck(_ *cobra.Command, args []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}

	resp, err := client.CheckViolation(context.Background(),
		connect.NewRequest(&specv1.CheckViolationRequest{
			SpecSlug: args[0],
		}))
	if err != nil {
		return fmt.Errorf("check violation: %w", err)
	}

	if len(resp.Msg.Violations) == 0 {
		fmt.Println("No violations found.")
		return nil
	}

	for _, v := range resp.Msg.Violations {
		fmt.Printf("[%s] %s: %s\n", v.Severity.String(), v.Rule, v.Message)
	}
	return nil
}

func runConstitutionEmit(_ *cobra.Command, _ []string) error {
	client, err := constitutionClient()
	if err != nil {
		return err
	}

	resp, err := client.EmitToolFiles(context.Background(),
		connect.NewRequest(&specv1.EmitRequest{
			Format: emitFormat,
		}))
	if err != nil {
		return fmt.Errorf("emit: %w", err)
	}

	if emitOutput != "" {
		if err := os.WriteFile(emitOutput, []byte(resp.Msg.Content), 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Printf("Wrote %s to %s\n", resp.Msg.Filename, emitOutput)
		return nil
	}

	fmt.Print(resp.Msg.Content)
	return nil
}
```

**Step 2: Verify build**

```bash
go build ./cmd/specgraph/
./specgraph constitution --help
./specgraph constitution show --help
./specgraph constitution check --help
./specgraph constitution emit --help
```

Expected: help output for all commands

**Step 3: Commit**

```bash
git add cmd/specgraph/constitution.go
git commit -m "feat(constitution): CLI commands — show, check, emit"
```

---

## Task 8: Enhanced Init — Constitution Creation with Scanner

**Files:**

- Modify: `cmd/specgraph/init.go`

**Step 1: Enhance `specgraph init` to create a constitution**

After the existing config creation, add a constitution creation step that optionally scans the codebase:

> **Note:** The `--scan` flag was removed in Slice 3.5. Constitution bootstrapping is now interactive/agent-driven.

Add to `cmd/specgraph/init.go`:

- ~~Add a `--scan` flag~~ *(removed in Slice 3.5)*
- ~~After writing config, if `--scan` is set, run the scanner and display the draft constitution~~ *(removed in Slice 3.5)*
- If not scanning, prompt for minimal constitution fields (project name, primary language)
- Store the constitution by calling the ConstitutionService (requires server running) OR write a local `.specgraph/constitution.yaml` file

Since the init command runs before the server exists, store the constitution as a local YAML file that the server reads on startup:

```go
// Add to init.go:
// - Import scanner package
// - Add --scan flag
// - After config write, scan codebase if --scan set
// - Write constitution YAML to .specgraph/constitution.yaml
// - Or prompt for minimal fields if no scan
```

The full implementation extends `runInit` to:

1. After writing config, ask "Create a constitution? (y/n)"
2. If yes + `--scan`: run `scanner.Scan(".")`, display draft, confirm
3. If yes + no scan: prompt for project name and primary language
4. Write constitution as YAML to `.specgraph/constitution.yaml`

**Step 2: Add local constitution YAML support to config**

Add `ConstitutionPath` to `StorageConfig` in `internal/config/config.go`, defaulting to `.specgraph/constitution.yaml`. Add `LoadConstitution` and `WriteConstitution` functions that read/write the YAML file.

**Step 3: Verify build and test init flow**

```bash
go build ./cmd/specgraph/
./specgraph init --help
```

Expected: shows `--scan` flag

**Step 4: Commit**

```bash
git add cmd/specgraph/init.go internal/config/config.go
git commit -m "feat(constitution): enhanced init with constitution creation and --scan"
```

---

## Task 9: Integration — Wire Everything Together

**Files:**

- Modify: `cmd/specgraph/serve.go` (ensure ConstitutionService loads constitution from YAML on startup if no graph constitution exists)

**Step 1: Add constitution bootstrap to serve**

On server startup, check if a constitution exists in Memgraph. If not, check for `.specgraph/constitution.yaml` and load it.

**Step 2: Verify full integration**

```bash
go build ./cmd/specgraph/
go test ./... -count=1 -timeout=120s
```

Expected: all tests pass, binary builds

**Step 3: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -m "feat(constitution): bootstrap constitution from YAML on server startup"
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
./specgraph constitution --help
./specgraph constitution show --help
./specgraph constitution emit --help
./specgraph constitution check --help
```

Expected: all commands show help

**Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore(constitution): cleanup and final verification"
```

---

## Summary

| Task | What | Files | Test Type |
|------|------|-------|-----------|
| 1 | Proto schema | `proto/specgraph/v1/constitution.proto` | Compile |
| 2 | Storage interface | `internal/storage/constitution.go` | Compile |
| 3 | Memgraph backend | `internal/storage/memgraph/constitution.go` | Integration (testcontainers) |
| 4 | ConnectRPC handler | `internal/server/constitution_handler.go` | Unit (mock backend) |
| 5 | ~~Codebase scanner~~ *(superseded by Slice 3.5)* | ~~`internal/scanner/scanner.go`~~ | ~~Unit (temp dirs)~~ |
| 6 | Emitter | `internal/emitter/emitter.go` | Unit |
| 7 | CLI commands | `cmd/specgraph/constitution.go` | Build verification |
| 8 | Enhanced init | `cmd/specgraph/init.go` | Build verification |
| 9 | Wire together | `cmd/specgraph/serve.go` | Integration |
| 10 | Final verification | — | All tests + lint |
