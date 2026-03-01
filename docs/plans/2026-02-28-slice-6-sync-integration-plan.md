# Slice 6: Sync & Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable specs to flow outward to Beads, GitHub Issues, and tool-specific workspace files via pluggable sync adapters and a tool injection system.

**Architecture:** Sync is a new proto service (SyncService) with its own storage interface, Memgraph implementation, ConnectRPC handler, and CLI commands. Two concrete adapters (Beads, GitHub) implement a common `SyncAdapter` interface, shelling out to `bd` and `gh` CLIs respectively. Tool injection writes execution bundles and constitution subsets into workspace-specific formats (CLAUDE.md, .cursorrules, AGENTS.md). Sync state is tracked in Memgraph via `[:SYNCED_TO]` edges between `(:Spec)` and `(:ExternalRef)` nodes.

**Tech Stack:** Go, ConnectRPC (buf/connect-go), Memgraph (neo4j-go-driver v5), Cobra (CLI), buf (proto codegen), testcontainers-go (integration tests)

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 6 section)

---

## Project Structure (new files)

```text
proto/specgraph/v1/
  sync.proto                           # Sync messages + SyncService
gen/specgraph/v1/                      # Generated (buf generate)
  sync.pb.go
  specgraphv1connect/
    sync.connect.go
internal/
  storage/
    sync.go                            # SyncBackend interface
  storage/memgraph/
    sync.go                            # Memgraph implementation
    sync_test.go                       # Integration tests
  sync/
    adapter.go                         # SyncAdapter interface
    beads.go                           # Beads adapter (bd CLI)
    beads_test.go                      # Beads adapter tests
    github.go                          # GitHub adapter (gh CLI)
    github_test.go                     # GitHub adapter tests
  inject/
    inject.go                          # Tool injection logic
    inject_test.go                     # Injection tests
  server/
    sync_handler.go                    # ConnectRPC handler
    sync_handler_test.go               # Handler tests with mock
cmd/specgraph/
  sync.go                              # CLI: sync beads, sync github, sync status
  inject.go                            # CLI: inject <slug> --tool=<tool>
```

---

## Task 1: Protobuf Schema — Sync Messages + SyncService

**Files:**

- Create: `proto/specgraph/v1/sync.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/sync.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// --- Enums ---

enum SyncAdapter {
  SYNC_ADAPTER_UNSPECIFIED = 0;
  SYNC_ADAPTER_BEADS = 1;
  SYNC_ADAPTER_GITHUB = 2;
}

enum SyncState {
  SYNC_STATE_UNSPECIFIED = 0;
  SYNC_STATE_PENDING = 1;
  SYNC_STATE_SYNCED = 2;
  SYNC_STATE_CONFLICT = 3;
  SYNC_STATE_ERROR = 4;
}

enum InjectTool {
  INJECT_TOOL_UNSPECIFIED = 0;
  INJECT_TOOL_CLAUDE_CODE = 1;
  INJECT_TOOL_CURSOR = 2;
  INJECT_TOOL_AGENTS_MD = 3;
}

// --- Messages ---

message SyncMapping {
  string spec_id = 1;
  string spec_slug = 2;
  SyncAdapter adapter = 3;
  string external_id = 4;
  SyncState state = 5;
  string error_message = 6;
  google.protobuf.Timestamp last_sync = 7;
  google.protobuf.Timestamp created_at = 8;
}

message SyncResult {
  string spec_slug = 1;
  string external_id = 2;
  SyncState state = 3;
  string message = 4;
}

message SyncConfig {
  SyncAdapter adapter = 1;
  string filter_stage = 2;       // optional: only sync specs at this stage
  string filter_priority = 3;    // optional: only sync specs at this priority
  bool dry_run = 4;
}

// --- Requests/Responses ---

message SyncBeadsRequest {
  SyncConfig config = 1;
}

message SyncGitHubRequest {
  SyncConfig config = 1;
}

message SyncResponse {
  repeated SyncResult results = 1;
  int32 synced = 2;
  int32 skipped = 3;
  int32 errors = 4;
}

message SyncStatusRequest {
  SyncAdapter adapter = 1;       // optional: filter by adapter
  string spec_slug = 2;          // optional: filter by spec
}

message SyncStatusResponse {
  repeated SyncMapping mappings = 1;
}

message InjectRequest {
  string spec_slug = 1;
  InjectTool tool = 2;
  string output_dir = 3;         // optional: defaults to current directory
}

message InjectResponse {
  repeated string files_written = 1;
  string summary = 2;
}

// --- Service ---

service SyncService {
  rpc SyncBeads(SyncBeadsRequest) returns (SyncResponse);
  rpc SyncGitHub(SyncGitHubRequest) returns (SyncResponse);
  rpc GetSyncStatus(SyncStatusRequest) returns (SyncStatusResponse);
  rpc Inject(InjectRequest) returns (InjectResponse);
}
```

**Step 2: Generate Go code**

```bash
buf generate
```

Expected: generates `gen/specgraph/v1/sync.pb.go` and `gen/specgraph/v1/specgraphv1connect/sync.connect.go`

**Step 3: Verify generated code compiles**

```bash
go mod tidy
go build ./gen/...
```

**Step 4: Commit**

```bash
git add proto/specgraph/v1/sync.proto gen/specgraph/v1/sync.pb.go gen/specgraph/v1/specgraphv1connect/sync.connect.go go.mod go.sum
git commit -m "feat(sync): protobuf schema for Sync messages and SyncService"
```

---

## Task 2: Storage Interface — SyncBackend

**Files:**

- Create: `internal/storage/sync.go`

**Step 1: Define the interface**

`internal/storage/sync.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrSyncMappingNotFound is returned when a sync mapping does not exist.
var ErrSyncMappingNotFound = errors.New("sync mapping not found")

// ErrSyncMappingExists is returned when a sync mapping already exists for a spec+adapter pair.
var ErrSyncMappingExists = errors.New("sync mapping already exists")

// SyncBackend defines storage operations for sync state tracking.
type SyncBackend interface {
	// CreateSyncMapping stores a new sync mapping between a spec and an external reference.
	CreateSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter, externalID string) (*specv1.SyncMapping, error)

	// UpdateSyncState updates the sync state and last_sync timestamp for an existing mapping.
	UpdateSyncState(ctx context.Context, specSlug string, adapter specv1.SyncAdapter, state specv1.SyncState, errorMessage string) (*specv1.SyncMapping, error)

	// GetSyncMapping retrieves a sync mapping by spec slug and adapter.
	GetSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter) (*specv1.SyncMapping, error)

	// ListSyncMappings returns all sync mappings, optionally filtered by adapter or spec slug.
	ListSyncMappings(ctx context.Context, adapter specv1.SyncAdapter, specSlug string) ([]*specv1.SyncMapping, error)

	// DeleteSyncMapping removes a sync mapping.
	DeleteSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter) error
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/sync.go
git commit -m "feat(sync): storage backend interface for sync state"
```

---

## Task 3: Memgraph Implementation — Sync Storage

**Files:**

- Create: `internal/storage/memgraph/sync.go`
- Create: `internal/storage/memgraph/sync_test.go`

**Step 1: Write the integration test**

`internal/storage/memgraph/sync_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestSync_CreateMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec first
	_, err = store.CreateSpec(ctx, "sync-test-spec", "Test spec for sync", "p2", "medium")
	require.NoError(t, err)

	// Create sync mapping
	mapping, err := store.CreateSyncMapping(ctx, "sync-test-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "github-issue-42")
	require.NoError(t, err)
	require.Equal(t, "sync-test-spec", mapping.SpecSlug)
	require.Equal(t, specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, mapping.Adapter)
	require.Equal(t, "github-issue-42", mapping.ExternalId)
	require.Equal(t, specv1.SyncState_SYNC_STATE_SYNCED, mapping.State)
	require.NotNil(t, mapping.LastSync)
	require.NotNil(t, mapping.CreatedAt)
}

func TestSync_CreateMappingDuplicate(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "dup-spec", "Test spec", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "dup-spec", specv1.SyncAdapter_SYNC_ADAPTER_BEADS, "beads-abc123")
	require.NoError(t, err)

	// Duplicate should fail
	_, err = store.CreateSyncMapping(ctx, "dup-spec", specv1.SyncAdapter_SYNC_ADAPTER_BEADS, "beads-xyz789")
	require.ErrorIs(t, err, storage.ErrSyncMappingExists)
}

func TestSync_CreateMappingSpecNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSyncMapping(ctx, "nonexistent", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "gh-1")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestSync_UpdateState(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "state-spec", "Test spec", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "state-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "gh-99")
	require.NoError(t, err)

	// Update state to error
	updated, err := store.UpdateSyncState(ctx, "state-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, specv1.SyncState_SYNC_STATE_ERROR, "rate limit exceeded")
	require.NoError(t, err)
	require.Equal(t, specv1.SyncState_SYNC_STATE_ERROR, updated.State)
	require.Equal(t, "rate limit exceeded", updated.ErrorMessage)

	// Update state back to synced
	updated, err = store.UpdateSyncState(ctx, "state-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, specv1.SyncState_SYNC_STATE_SYNCED, "")
	require.NoError(t, err)
	require.Equal(t, specv1.SyncState_SYNC_STATE_SYNCED, updated.State)
	require.Empty(t, updated.ErrorMessage)
}

func TestSync_UpdateStateNotFound(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.UpdateSyncState(ctx, "no-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, specv1.SyncState_SYNC_STATE_SYNCED, "")
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}

func TestSync_GetMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "get-spec", "Test spec", "p1", "high")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "get-spec", specv1.SyncAdapter_SYNC_ADAPTER_BEADS, "beads-get123")
	require.NoError(t, err)

	got, err := store.GetSyncMapping(ctx, "get-spec", specv1.SyncAdapter_SYNC_ADAPTER_BEADS)
	require.NoError(t, err)
	require.Equal(t, "beads-get123", got.ExternalId)

	// Not found
	_, err = store.GetSyncMapping(ctx, "get-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)
}

func TestSync_ListMappings(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "list-a", "Spec A", "p2", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "list-b", "Spec B", "p2", "medium")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "list-a", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "gh-1")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-b", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "gh-2")
	require.NoError(t, err)
	_, err = store.CreateSyncMapping(ctx, "list-a", specv1.SyncAdapter_SYNC_ADAPTER_BEADS, "beads-1")
	require.NoError(t, err)

	// List all
	all, err := store.ListSyncMappings(ctx, specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED, "")
	require.NoError(t, err)
	require.Len(t, all, 3)

	// Filter by adapter
	ghOnly, err := store.ListSyncMappings(ctx, specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "")
	require.NoError(t, err)
	require.Len(t, ghOnly, 2)

	// Filter by spec slug
	specA, err := store.ListSyncMappings(ctx, specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED, "list-a")
	require.NoError(t, err)
	require.Len(t, specA, 2)

	// Filter by both
	specific, err := store.ListSyncMappings(ctx, specv1.SyncAdapter_SYNC_ADAPTER_BEADS, "list-a")
	require.NoError(t, err)
	require.Len(t, specific, 1)
	require.Equal(t, "beads-1", specific[0].ExternalId)
}

func TestSync_DeleteMapping(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "del-spec", "Spec to delete sync", "p2", "low")
	require.NoError(t, err)

	_, err = store.CreateSyncMapping(ctx, "del-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, "gh-del")
	require.NoError(t, err)

	err = store.DeleteSyncMapping(ctx, "del-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB)
	require.NoError(t, err)

	_, err = store.GetSyncMapping(ctx, "del-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB)
	require.ErrorIs(t, err, storage.ErrSyncMappingNotFound)

	// Delete nonexistent — should not error (idempotent)
	err = store.DeleteSyncMapping(ctx, "del-spec", specv1.SyncAdapter_SYNC_ADAPTER_GITHUB)
	require.NoError(t, err)
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/storage/memgraph/ -run TestSync -v -count=1 -timeout=120s
```

Expected: FAIL -- `sync.go` doesn't exist yet

**Step 3: Implement the Memgraph sync backend**

`internal/storage/memgraph/sync.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Store) CreateSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter, externalID string) (*specv1.SyncMapping, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	adapterStr := adapter.String()

	// Verify spec exists
	specResult, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug}) RETURN s.id`,
		map[string]any{"slug": specSlug},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}
	if len(specResult.Records) == 0 {
		return nil, fmt.Errorf("memgraph: create sync mapping %q: %w", specSlug, storage.ErrSpecNotFound)
	}

	specID, err := recordString(specResult.Records[0], 0, "id")
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}

	// Check for existing mapping
	existingResult, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 RETURN e.external_id`,
		map[string]any{"slug": specSlug, "adapter": adapterStr},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}
	if len(existingResult.Records) > 0 {
		return nil, fmt.Errorf("memgraph: create sync mapping %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingExists)
	}

	// Create ExternalRef node and SYNCED_TO edge
	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})
		 CREATE (e:ExternalRef {
		   external_id: $external_id,
		   adapter: $adapter,
		   created_at: $now
		 })
		 CREATE (s)-[r:SYNCED_TO {
		   adapter: $adapter,
		   external_id: $external_id,
		   state: $state,
		   error_message: "",
		   last_sync: $now,
		   created_at: $now
		 }]->(e)
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		map[string]any{
			"slug":        specSlug,
			"external_id": externalID,
			"adapter":     adapterStr,
			"state":       specv1.SyncState_SYNC_STATE_SYNCED.String(),
			"now":         nowStr,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create sync mapping: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: create sync mapping: no result returned")
	}

	return recordToSyncMapping(result.Records[0], specID)
}

func (s *Store) UpdateSyncState(ctx context.Context, specSlug string, adapter specv1.SyncAdapter, state specv1.SyncState, errorMessage string) (*specv1.SyncMapping, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)
	adapterStr := adapter.String()

	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 SET r.state = $state,
		     r.error_message = $error_message,
		     r.last_sync = $now
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		map[string]any{
			"slug":          specSlug,
			"adapter":       adapterStr,
			"state":         state.String(),
			"error_message": errorMessage,
			"now":           nowStr,
		},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: update sync state: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: update sync state %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingNotFound)
	}

	specID, _ := recordString(result.Records[0], 0, "id")
	return recordToSyncMapping(result.Records[0], specID)
}

func (s *Store) GetSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter) (*specv1.SyncMapping, error) {
	adapterStr := adapter.String()

	result, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at`,
		map[string]any{"slug": specSlug, "adapter": adapterStr},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get sync mapping: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: get sync mapping %q/%s: %w", specSlug, adapterStr, storage.ErrSyncMappingNotFound)
	}

	specID, _ := recordString(result.Records[0], 0, "id")
	return recordToSyncMapping(result.Records[0], specID)
}

func (s *Store) ListSyncMappings(ctx context.Context, adapter specv1.SyncAdapter, specSlug string) ([]*specv1.SyncMapping, error) {
	var conditions []string
	params := map[string]any{}

	if specSlug != "" {
		conditions = append(conditions, "s.slug = $slug")
		params["slug"] = specSlug
	}
	if adapter != specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED {
		conditions = append(conditions, "r.adapter = $adapter")
		params["adapter"] = adapter.String()
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(
		`MATCH (s:Spec)-[r:SYNCED_TO]->(e:ExternalRef)%s
		 RETURN s.id, s.slug, r.adapter, e.external_id, r.state,
		        r.error_message, r.last_sync, r.created_at
		 ORDER BY r.last_sync DESC`,
		where,
	)

	result, err := neo4j.ExecuteQuery(ctx, s.driver, query, params, neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list sync mappings: %w", err)
	}

	mappings := make([]*specv1.SyncMapping, 0, len(result.Records))
	for _, rec := range result.Records {
		specID, _ := recordString(rec, 0, "id")
		m, err := recordToSyncMapping(rec, specID)
		if err != nil {
			return nil, err
		}
		mappings = append(mappings, m)
	}
	return mappings, nil
}

func (s *Store) DeleteSyncMapping(ctx context.Context, specSlug string, adapter specv1.SyncAdapter) error {
	adapterStr := adapter.String()

	_, err := neo4j.ExecuteQuery(ctx, s.driver,
		`MATCH (s:Spec {slug: $slug})-[r:SYNCED_TO {adapter: $adapter}]->(e:ExternalRef)
		 DELETE r, e`,
		map[string]any{"slug": specSlug, "adapter": adapterStr},
		neo4j.EagerResultTransformer,
	)
	if err != nil {
		return fmt.Errorf("memgraph: delete sync mapping: %w", err)
	}
	return nil
}

func recordToSyncMapping(rec *neo4j.Record, specID string) (*specv1.SyncMapping, error) {
	m := &specv1.SyncMapping{
		SpecId: specID,
	}

	var err error
	m.SpecSlug, err = recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}

	adapterStr, err := recordString(rec, 2, "adapter")
	if err != nil {
		return nil, err
	}
	if val, ok := specv1.SyncAdapter_value[adapterStr]; ok {
		m.Adapter = specv1.SyncAdapter(val)
	}

	m.ExternalId, err = recordString(rec, 3, "external_id")
	if err != nil {
		return nil, err
	}

	stateStr, err := recordString(rec, 4, "state")
	if err != nil {
		return nil, err
	}
	if val, ok := specv1.SyncState_value[stateStr]; ok {
		m.State = specv1.SyncState(val)
	}

	m.ErrorMessage, err = recordString(rec, 5, "error_message")
	if err != nil {
		return nil, err
	}

	lastSyncStr, err := recordString(rec, 6, "last_sync")
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, lastSyncStr); parseErr == nil {
		m.LastSync = timestamppb.New(t)
	}

	createdStr, err := recordString(rec, 7, "created_at")
	if err != nil {
		return nil, err
	}
	if t, parseErr := time.Parse(time.RFC3339, createdStr); parseErr == nil {
		m.CreatedAt = timestamppb.New(t)
	}

	return m, nil
}
```

**Step 4: Run the tests**

```bash
go mod tidy
go test ./internal/storage/memgraph/ -run TestSync -v -count=1 -timeout=120s
```

Expected: PASS (all sync tests). Requires Docker running.

**Step 5: Commit**

```bash
git add internal/storage/memgraph/sync.go internal/storage/memgraph/sync_test.go
git commit -m "feat(sync): memgraph storage backend for sync mappings with integration tests"
```

---

## Task 4: SyncAdapter Interface + Beads Adapter

**Files:**

- Create: `internal/sync/adapter.go`
- Create: `internal/sync/beads.go`
- Create: `internal/sync/beads_test.go`

**Step 1: Define the SyncAdapter interface**

`internal/sync/adapter.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package sync implements sync adapters for pushing specs to external systems.
package sync

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrAdapterNotAvailable is returned when the adapter's CLI tool is not installed.
var ErrAdapterNotAvailable = errors.New("adapter CLI tool not available")

// ErrPushFailed is returned when pushing a spec to the external system fails.
var ErrPushFailed = errors.New("push failed")

// ErrPullFailed is returned when pulling status from the external system fails.
var ErrPullFailed = errors.New("pull failed")

// Adapter defines the interface for syncing specs with external systems.
type Adapter interface {
	// Name returns the adapter identifier (e.g., "beads", "github").
	Name() specv1.SyncAdapter

	// Available checks if the adapter's CLI tool is installed and accessible.
	Available() error

	// Push creates or updates an external representation of the spec.
	// Returns the external ID (e.g., "beads-abc123", "github-issue-42").
	Push(ctx context.Context, spec *specv1.Spec) (externalID string, err error)

	// Pull retrieves the current status of the external item.
	// Returns the external status string (e.g., "open", "closed", "in_progress").
	Pull(ctx context.Context, externalID string) (status string, err error)
}

// CommandRunner abstracts CLI command execution for testing.
type CommandRunner interface {
	// Run executes a command and returns its combined stdout/stderr output.
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

**Step 2: Write the Beads adapter test**

`internal/sync/beads_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"fmt"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/sync"
	"github.com/stretchr/testify/require"
)

type mockCommandRunner struct {
	outputs map[string][]byte
	errors  map[string]error
	calls   []string
}

func newMockRunner() *mockCommandRunner {
	return &mockCommandRunner{
		outputs: map[string][]byte{},
		errors:  map[string]error{},
	}
}

func (m *mockCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	m.calls = append(m.calls, key)

	if err, ok := m.errors[key]; ok {
		return nil, err
	}
	if out, ok := m.outputs[key]; ok {
		return out, nil
	}
	return []byte{}, nil
}

func (m *mockCommandRunner) setOutput(cmd string, output string) {
	m.outputs[cmd] = []byte(output)
}

func (m *mockCommandRunner) setError(cmd string, err error) {
	m.errors[cmd] = err
}

func TestBeadsAdapter_Name(t *testing.T) {
	runner := newMockRunner()
	adapter := sync.NewBeadsAdapter(runner)
	require.Equal(t, specv1.SyncAdapter_SYNC_ADAPTER_BEADS, adapter.Name())
}

func TestBeadsAdapter_Available(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("bd --version", "beads v0.1.0")
	adapter := sync.NewBeadsAdapter(runner)
	require.NoError(t, adapter.Available())
}

func TestBeadsAdapter_AvailableNotInstalled(t *testing.T) {
	runner := newMockRunner()
	runner.setError("bd --version", fmt.Errorf("exec: bd: not found"))
	adapter := sync.NewBeadsAdapter(runner)
	require.ErrorIs(t, adapter.Available(), sync.ErrAdapterNotAvailable)
}

func TestBeadsAdapter_Push(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("bd issue create --title [spec] test-push-spec --description Test intent --json", `{"id": "beads-abc123"}`)
	adapter := sync.NewBeadsAdapter(runner)

	spec := &specv1.Spec{
		Slug:   "test-push-spec",
		Intent: "Test intent",
		Stage:  "approved",
	}

	externalID, err := adapter.Push(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, "beads-abc123", externalID)
}

func TestBeadsAdapter_Pull(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("bd issue show beads-abc123 --json", `{"status": "in_progress"}`)
	adapter := sync.NewBeadsAdapter(runner)

	status, err := adapter.Pull(context.Background(), "beads-abc123")
	require.NoError(t, err)
	require.Equal(t, "in_progress", status)
}
```

**Step 3: Implement the Beads adapter**

`internal/sync/beads.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"encoding/json"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// BeadsAdapter syncs specs with the Beads issue tracker via the bd CLI.
type BeadsAdapter struct {
	runner CommandRunner
}

// NewBeadsAdapter creates a BeadsAdapter using the given command runner.
func NewBeadsAdapter(runner CommandRunner) *BeadsAdapter {
	return &BeadsAdapter{runner: runner}
}

func (a *BeadsAdapter) Name() specv1.SyncAdapter {
	return specv1.SyncAdapter_SYNC_ADAPTER_BEADS
}

func (a *BeadsAdapter) Available() error {
	_, err := a.runner.Run(context.Background(), "bd", "--version")
	if err != nil {
		return fmt.Errorf("%w: bd: %v", ErrAdapterNotAvailable, err)
	}
	return nil
}

func (a *BeadsAdapter) Push(ctx context.Context, spec *specv1.Spec) (string, error) {
	title := fmt.Sprintf("[spec] %s", spec.Slug)
	output, err := a.runner.Run(ctx, "bd", "issue", "create",
		"--title", title,
		"--description", spec.Intent,
		"--json",
	)
	if err != nil {
		return "", fmt.Errorf("%w: bd issue create: %v", ErrPushFailed, err)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("%w: parse bd output: %v", ErrPushFailed, err)
	}

	return result.ID, nil
}

func (a *BeadsAdapter) Pull(ctx context.Context, externalID string) (string, error) {
	output, err := a.runner.Run(ctx, "bd", "issue", "show", externalID, "--json")
	if err != nil {
		return "", fmt.Errorf("%w: bd issue show: %v", ErrPullFailed, err)
	}

	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("%w: parse bd output: %v", ErrPullFailed, err)
	}

	return result.Status, nil
}
```

**Step 4: Run the tests**

```bash
go test ./internal/sync/ -run TestBeads -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/sync/adapter.go internal/sync/beads.go internal/sync/beads_test.go
git commit -m "feat(sync): SyncAdapter interface and Beads adapter with bd CLI"
```

---

## Task 5: GitHub Adapter

**Files:**

- Create: `internal/sync/github.go`
- Create: `internal/sync/github_test.go`

**Step 1: Write the GitHub adapter test**

`internal/sync/github_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"fmt"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/sync"
	"github.com/stretchr/testify/require"
)

func TestGitHubAdapter_Name(t *testing.T) {
	runner := newMockRunner()
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")
	require.Equal(t, specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, adapter.Name())
}

func TestGitHubAdapter_Available(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("gh --version", "gh version 2.60.0")
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")
	require.NoError(t, adapter.Available())
}

func TestGitHubAdapter_AvailableNotInstalled(t *testing.T) {
	runner := newMockRunner()
	runner.setError("gh --version", fmt.Errorf("exec: gh: not found"))
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")
	require.ErrorIs(t, adapter.Available(), sync.ErrAdapterNotAvailable)
}

func TestGitHubAdapter_Push(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput(
		"gh issue create --repo owner/repo --title [spec] oauth-refresh --body ## Spec: oauth-refresh\n\n**Intent:** Implement OAuth refresh\n\n**Stage:** approved\n**Priority:** p1\n**Complexity:** medium --label specgraph,approved,p1 --json number",
		`{"number": 42}`,
	)
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")

	spec := &specv1.Spec{
		Slug:       "oauth-refresh",
		Intent:     "Implement OAuth refresh",
		Stage:      "approved",
		Priority:   "p1",
		Complexity: "medium",
	}

	externalID, err := adapter.Push(context.Background(), spec)
	require.NoError(t, err)
	require.Equal(t, "42", externalID)
}

func TestGitHubAdapter_Pull(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("gh issue view 42 --repo owner/repo --json state", `{"state": "OPEN"}`)
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")

	status, err := adapter.Pull(context.Background(), "42")
	require.NoError(t, err)
	require.Equal(t, "OPEN", status)
}

func TestGitHubAdapter_PullClosed(t *testing.T) {
	runner := newMockRunner()
	runner.setOutput("gh issue view 99 --repo owner/repo --json state", `{"state": "CLOSED"}`)
	adapter := sync.NewGitHubAdapter(runner, "owner/repo")

	status, err := adapter.Pull(context.Background(), "99")
	require.NoError(t, err)
	require.Equal(t, "CLOSED", status)
}
```

**Step 2: Implement the GitHub adapter**

`internal/sync/github.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// GitHubAdapter syncs specs with GitHub Issues via the gh CLI.
type GitHubAdapter struct {
	runner CommandRunner
	repo   string // "owner/repo" format
}

// NewGitHubAdapter creates a GitHubAdapter for the given repository.
func NewGitHubAdapter(runner CommandRunner, repo string) *GitHubAdapter {
	return &GitHubAdapter{runner: runner, repo: repo}
}

func (a *GitHubAdapter) Name() specv1.SyncAdapter {
	return specv1.SyncAdapter_SYNC_ADAPTER_GITHUB
}

func (a *GitHubAdapter) Available() error {
	_, err := a.runner.Run(context.Background(), "gh", "--version")
	if err != nil {
		return fmt.Errorf("%w: gh: %v", ErrAdapterNotAvailable, err)
	}
	return nil
}

func (a *GitHubAdapter) Push(ctx context.Context, spec *specv1.Spec) (string, error) {
	title := fmt.Sprintf("[spec] %s", spec.Slug)
	body := formatIssueBody(spec)
	labels := formatLabels(spec)

	output, err := a.runner.Run(ctx, "gh", "issue", "create",
		"--repo", a.repo,
		"--title", title,
		"--body", body,
		"--label", labels,
		"--json", "number",
	)
	if err != nil {
		return "", fmt.Errorf("%w: gh issue create: %v", ErrPushFailed, err)
	}

	var result struct {
		Number int `json:"number"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("%w: parse gh output: %v", ErrPushFailed, err)
	}

	return strconv.Itoa(result.Number), nil
}

func (a *GitHubAdapter) Pull(ctx context.Context, externalID string) (string, error) {
	output, err := a.runner.Run(ctx, "gh", "issue", "view", externalID,
		"--repo", a.repo,
		"--json", "state",
	)
	if err != nil {
		return "", fmt.Errorf("%w: gh issue view: %v", ErrPullFailed, err)
	}

	var result struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return "", fmt.Errorf("%w: parse gh output: %v", ErrPullFailed, err)
	}

	return result.State, nil
}

func formatIssueBody(spec *specv1.Spec) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Spec: %s\n\n", spec.Slug))
	b.WriteString(fmt.Sprintf("**Intent:** %s\n\n", spec.Intent))
	b.WriteString(fmt.Sprintf("**Stage:** %s\n", spec.Stage))
	b.WriteString(fmt.Sprintf("**Priority:** %s\n", spec.Priority))
	b.WriteString(fmt.Sprintf("**Complexity:** %s", spec.Complexity))
	return b.String()
}

func formatLabels(spec *specv1.Spec) string {
	labels := []string{"specgraph"}
	if spec.Stage != "" {
		labels = append(labels, spec.Stage)
	}
	if spec.Priority != "" {
		labels = append(labels, spec.Priority)
	}
	return strings.Join(labels, ",")
}
```

**Step 3: Run the tests**

```bash
go test ./internal/sync/ -run TestGitHub -v -count=1
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/sync/github.go internal/sync/github_test.go
git commit -m "feat(sync): GitHub adapter with gh CLI for issue sync"
```

---

## Task 6: Tool Injection

**Files:**

- Create: `internal/inject/inject.go`
- Create: `internal/inject/inject_test.go`

**Step 1: Write the injection test**

`internal/inject/inject_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package inject_test

import (
	"os"
	"path/filepath"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/inject"
	"github.com/stretchr/testify/require"
)

func testSpec() *specv1.Spec {
	return &specv1.Spec{
		Id:         "spec-abc123",
		Slug:       "oauth-refresh-flow",
		Intent:     "Implement OAuth refresh token rotation",
		Stage:      "approved",
		Priority:   "p1",
		Complexity: "medium",
		Version:    3,
	}
}

func testConstitution() *specv1.Constitution {
	return &specv1.Constitution{
		Name: "test-project",
		Tech: &specv1.TechConfig{
			Languages: &specv1.LanguageConfig{
				Primary: "go",
				Allowed: []string{"go"},
			},
			Frameworks: map[string]string{
				"api": "ConnectRPC",
			},
		},
		Constraints: []string{
			"No ORMs",
			"All APIs must be backward compatible",
		},
	}
}

func TestInject_ClaudeCode(t *testing.T) {
	dir := t.TempDir()

	files, err := inject.Inject(testSpec(), testConstitution(), specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "specs", "oauth-refresh-flow.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "oauth-refresh-flow")
	require.Contains(t, string(content), "OAuth refresh token rotation")
	require.Contains(t, string(content), "go")
	require.Contains(t, string(content), "No ORMs")
}

func TestInject_Cursor(t *testing.T) {
	dir := t.TempDir()

	files, err := inject.Inject(testSpec(), testConstitution(), specv1.InjectTool_INJECT_TOOL_CURSOR, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(dir, ".cursor", "rules", "specgraph-oauth-refresh-flow.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "oauth-refresh-flow")
	require.Contains(t, string(content), "go")
}

func TestInject_AgentsMD(t *testing.T) {
	dir := t.TempDir()

	files, err := inject.Inject(testSpec(), testConstitution(), specv1.InjectTool_INJECT_TOOL_AGENTS_MD, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "oauth-refresh-flow")
}

func TestInject_UnsupportedTool(t *testing.T) {
	dir := t.TempDir()

	_, err := inject.Inject(testSpec(), testConstitution(), specv1.InjectTool_INJECT_TOOL_UNSPECIFIED, dir)
	require.Error(t, err)
}

func TestInject_NilConstitution(t *testing.T) {
	dir := t.TempDir()

	files, err := inject.Inject(testSpec(), nil, specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE, dir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	content, err := os.ReadFile(filepath.Join(dir, ".claude", "specs", "oauth-refresh-flow.md"))
	require.NoError(t, err)
	require.Contains(t, string(content), "oauth-refresh-flow")
	require.NotContains(t, string(content), "No ORMs")
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/inject/ -v -count=1
```

Expected: FAIL -- package doesn't exist

**Step 3: Implement the injection logic**

`internal/inject/inject.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package inject writes spec execution context into tool-specific workspace files.
package inject

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// Inject writes an execution bundle for the given spec and constitution into
// the specified output directory, formatted for the target tool.
// Returns the list of files written.
func Inject(spec *specv1.Spec, constitution *specv1.Constitution, tool specv1.InjectTool, outputDir string) ([]string, error) {
	switch tool {
	case specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE:
		return injectClaudeCode(spec, constitution, outputDir)
	case specv1.InjectTool_INJECT_TOOL_CURSOR:
		return injectCursor(spec, constitution, outputDir)
	case specv1.InjectTool_INJECT_TOOL_AGENTS_MD:
		return injectAgentsMD(spec, constitution, outputDir)
	default:
		return nil, fmt.Errorf("unsupported inject tool: %s", tool.String())
	}
}

func injectClaudeCode(spec *specv1.Spec, constitution *specv1.Constitution, outputDir string) ([]string, error) {
	dir := filepath.Join(outputDir, ".claude", "specs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create specs dir: %w", err)
	}

	filename := filepath.Join(dir, spec.Slug+".md")
	content := formatSpecContext(spec, constitution, "claude-code")

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write spec context: %w", err)
	}

	return []string{filename}, nil
}

func injectCursor(spec *specv1.Spec, constitution *specv1.Constitution, outputDir string) ([]string, error) {
	dir := filepath.Join(outputDir, ".cursor", "rules")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cursor rules dir: %w", err)
	}

	filename := filepath.Join(dir, "specgraph-"+spec.Slug+".md")
	content := formatSpecContext(spec, constitution, "cursor")

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write cursor rule: %w", err)
	}

	return []string{filename}, nil
}

func injectAgentsMD(spec *specv1.Spec, constitution *specv1.Constitution, outputDir string) ([]string, error) {
	filename := filepath.Join(outputDir, "AGENTS.md")
	content := formatSpecContext(spec, constitution, "agents-md")

	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write AGENTS.md: %w", err)
	}

	return []string{filename}, nil
}

func formatSpecContext(spec *specv1.Spec, constitution *specv1.Constitution, format string) string {
	var b strings.Builder

	switch format {
	case "cursor":
		b.WriteString("---\n")
		b.WriteString(fmt.Sprintf("description: \"SpecGraph context for %s\"\n", spec.Slug))
		b.WriteString("alwaysApply: false\n")
		b.WriteString("---\n\n")
	}

	b.WriteString(fmt.Sprintf("# Spec: %s\n\n", spec.Slug))
	b.WriteString("Generated by SpecGraph. Do not edit manually.\n\n")

	b.WriteString("## Task\n\n")
	b.WriteString(fmt.Sprintf("**Intent:** %s\n\n", spec.Intent))
	b.WriteString(fmt.Sprintf("| Field | Value |\n"))
	b.WriteString(fmt.Sprintf("|-------|-------|\n"))
	b.WriteString(fmt.Sprintf("| ID | %s |\n", spec.Id))
	b.WriteString(fmt.Sprintf("| Stage | %s |\n", spec.Stage))
	b.WriteString(fmt.Sprintf("| Priority | %s |\n", spec.Priority))
	b.WriteString(fmt.Sprintf("| Complexity | %s |\n", spec.Complexity))
	b.WriteString(fmt.Sprintf("| Version | %d |\n", spec.Version))
	b.WriteString("\n")

	if constitution != nil {
		writeConstitutionSubset(&b, constitution)
	}

	return b.String()
}

func writeConstitutionSubset(b *strings.Builder, c *specv1.Constitution) {
	b.WriteString("## Project Constraints\n\n")

	if c.Tech != nil && c.Tech.Languages != nil {
		b.WriteString(fmt.Sprintf("**Primary language:** %s\n", c.Tech.Languages.Primary))
		if len(c.Tech.Languages.Allowed) > 0 {
			b.WriteString(fmt.Sprintf("**Allowed:** %s\n", strings.Join(c.Tech.Languages.Allowed, ", ")))
		}
		b.WriteString("\n")
	}

	if c.Tech != nil && len(c.Tech.Frameworks) > 0 {
		b.WriteString("**Frameworks:**\n\n")
		for area, fw := range c.Tech.Frameworks {
			b.WriteString(fmt.Sprintf("- %s: %s\n", area, fw))
		}
		b.WriteString("\n")
	}

	if len(c.Constraints) > 0 {
		b.WriteString("**Constraints:**\n\n")
		for _, constraint := range c.Constraints {
			b.WriteString(fmt.Sprintf("- %s\n", constraint))
		}
		b.WriteString("\n")
	}

	if len(c.Antipatterns) > 0 {
		b.WriteString("**Anti-patterns:**\n\n")
		for _, ap := range c.Antipatterns {
			b.WriteString(fmt.Sprintf("- %s", ap.Pattern))
			if ap.Instead != "" {
				b.WriteString(fmt.Sprintf(" (instead: %s)", ap.Instead))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}
}
```

**Step 4: Run the tests**

```bash
go test ./internal/inject/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/inject/inject.go internal/inject/inject_test.go
git commit -m "feat(sync): tool injection for CLAUDE.md, .cursorrules, AGENTS.md"
```

---

## Task 7: ConnectRPC Handler — SyncService

**Files:**

- Create: `internal/server/sync_handler.go`
- Create: `internal/server/sync_handler_test.go`

**Step 1: Write the handler test**

`internal/server/sync_handler_test.go`:

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

type mockSyncBackend struct {
	mu       sync.Mutex
	mappings map[string]*specv1.SyncMapping // key: "slug:adapter"
	specs    map[string]*specv1.Spec
}

func newMockSyncBackend() *mockSyncBackend {
	return &mockSyncBackend{
		mappings: map[string]*specv1.SyncMapping{},
		specs:    map[string]*specv1.Spec{},
	}
}

func (m *mockSyncBackend) key(slug string, adapter specv1.SyncAdapter) string {
	return fmt.Sprintf("%s:%s", slug, adapter.String())
}

func (m *mockSyncBackend) CreateSyncMapping(_ context.Context, specSlug string, adapter specv1.SyncAdapter, externalID string) (*specv1.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	if _, exists := m.mappings[k]; exists {
		return nil, storage.ErrSyncMappingExists
	}
	mapping := &specv1.SyncMapping{
		SpecSlug:   specSlug,
		Adapter:    adapter,
		ExternalId: externalID,
		State:      specv1.SyncState_SYNC_STATE_SYNCED,
	}
	m.mappings[k] = mapping
	return mapping, nil
}

func (m *mockSyncBackend) UpdateSyncState(_ context.Context, specSlug string, adapter specv1.SyncAdapter, state specv1.SyncState, errorMessage string) (*specv1.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	mapping, exists := m.mappings[k]
	if !exists {
		return nil, storage.ErrSyncMappingNotFound
	}
	mapping.State = state
	mapping.ErrorMessage = errorMessage
	return mapping, nil
}

func (m *mockSyncBackend) GetSyncMapping(_ context.Context, specSlug string, adapter specv1.SyncAdapter) (*specv1.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	mapping, exists := m.mappings[k]
	if !exists {
		return nil, storage.ErrSyncMappingNotFound
	}
	return mapping, nil
}

func (m *mockSyncBackend) ListSyncMappings(_ context.Context, adapter specv1.SyncAdapter, specSlug string) ([]*specv1.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*specv1.SyncMapping
	for _, mapping := range m.mappings {
		if adapter != specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED && mapping.Adapter != adapter {
			continue
		}
		if specSlug != "" && mapping.SpecSlug != specSlug {
			continue
		}
		result = append(result, mapping)
	}
	return result, nil
}

func (m *mockSyncBackend) DeleteSyncMapping(_ context.Context, specSlug string, adapter specv1.SyncAdapter) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	delete(m.mappings, k)
	return nil
}

type mockSpecGetter struct {
	specs map[string]*specv1.Spec
}

func (m *mockSpecGetter) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	return spec, nil
}

func (m *mockSpecGetter) ListSpecs(_ context.Context, stage, priority string, limit int) ([]*specv1.Spec, error) {
	var result []*specv1.Spec
	for _, spec := range m.specs {
		if stage != "" && spec.Stage != stage {
			continue
		}
		if priority != "" && spec.Priority != priority {
			continue
		}
		result = append(result, spec)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

var _ storage.SyncBackend = (*mockSyncBackend)(nil)

func setupSyncServer(t *testing.T) specgraphv1connect.SyncServiceClient {
	t.Helper()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecGetter{
		specs: map[string]*specv1.Spec{
			"test-spec": {
				Id:         "spec-test123",
				Slug:       "test-spec",
				Intent:     "Test spec for sync",
				Stage:      "approved",
				Priority:   "p2",
				Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, specStore, nil)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)
}

func TestSyncHandler_GetSyncStatus_Empty(t *testing.T) {
	client := setupSyncServer(t)
	resp, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Mappings)
}

func TestSyncHandler_GetSyncStatus_WithMappings(t *testing.T) {
	syncStore := newMockSyncBackend()
	syncStore.mappings["spec-a:SYNC_ADAPTER_GITHUB"] = &specv1.SyncMapping{
		SpecSlug:   "spec-a",
		Adapter:    specv1.SyncAdapter_SYNC_ADAPTER_GITHUB,
		ExternalId: "gh-1",
		State:      specv1.SyncState_SYNC_STATE_SYNCED,
	}

	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, &mockSpecGetter{specs: map[string]*specv1.Spec{}}, nil)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Mappings, 1)
	require.Equal(t, "gh-1", resp.Msg.Mappings[0].ExternalId)
}

func TestSyncHandler_Inject_SpecNotFound(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug: "nonexistent",
			Tool:     specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
```

**Step 2: Run the test to verify it fails**

```bash
go test ./internal/server/ -run TestSync -v -count=1
```

Expected: FAIL -- handler doesn't exist yet

**Step 3: Implement the handler**

`internal/server/sync_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/inject"
	"github.com/seanb4t/specgraph/internal/storage"
	syncpkg "github.com/seanb4t/specgraph/internal/sync"
)

// SpecGetter retrieves specs for sync operations.
type SpecGetter interface {
	GetSpec(ctx context.Context, slug string) (*specv1.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*specv1.Spec, error)
}

// SyncHandler implements the ConnectRPC SyncService.
type SyncHandler struct {
	syncStore storage.SyncBackend
	specStore SpecGetter
	adapters  map[specv1.SyncAdapter]syncpkg.Adapter
}

var _ specgraphv1connect.SyncServiceHandler = (*SyncHandler)(nil)

// RegisterSyncService registers the SyncService handler on the mux.
// constitutionStore can be nil if constitution injection is not needed.
func RegisterSyncService(mux *http.ServeMux, syncStore storage.SyncBackend, specStore SpecGetter, constitutionStore storage.ConstitutionBackend) {
	handler := &SyncHandler{
		syncStore: syncStore,
		specStore: specStore,
		adapters:  map[specv1.SyncAdapter]syncpkg.Adapter{},
	}
	path, h := specgraphv1connect.NewSyncServiceHandler(handler)
	mux.Handle(path, h)
}

// RegisterAdapter adds a sync adapter to the handler.
func (h *SyncHandler) RegisterAdapter(adapter syncpkg.Adapter) {
	h.adapters[adapter.Name()] = adapter
}

func (h *SyncHandler) SyncBeads(ctx context.Context, req *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	adapter, ok := h.adapters[specv1.SyncAdapter_SYNC_ADAPTER_BEADS]
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("beads adapter not configured"))
	}
	return h.syncWithAdapter(ctx, adapter, req.Msg.Config)
}

func (h *SyncHandler) SyncGitHub(ctx context.Context, req *connect.Request[specv1.SyncGitHubRequest]) (*connect.Response[specv1.SyncResponse], error) {
	adapter, ok := h.adapters[specv1.SyncAdapter_SYNC_ADAPTER_GITHUB]
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("github adapter not configured"))
	}
	return h.syncWithAdapter(ctx, adapter, req.Msg.Config)
}

func (h *SyncHandler) syncWithAdapter(ctx context.Context, adapter syncpkg.Adapter, config *specv1.SyncConfig) (*connect.Response[specv1.SyncResponse], error) {
	if err := adapter.Available(); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	stage := ""
	priority := ""
	dryRun := false
	if config != nil {
		stage = config.FilterStage
		priority = config.FilterPriority
		dryRun = config.DryRun
	}

	specs, err := h.specStore.ListSpecs(ctx, stage, priority, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	resp := &specv1.SyncResponse{}
	for _, spec := range specs {
		result := &specv1.SyncResult{SpecSlug: spec.Slug}

		// Check if already synced
		existing, err := h.syncStore.GetSyncMapping(ctx, spec.Slug, adapter.Name())
		if err == nil && existing != nil {
			result.ExternalId = existing.ExternalId
			result.State = specv1.SyncState_SYNC_STATE_SYNCED
			result.Message = "already synced"
			resp.Skipped++
			resp.Results = append(resp.Results, result)
			continue
		}

		if dryRun {
			result.State = specv1.SyncState_SYNC_STATE_PENDING
			result.Message = "dry run - would sync"
			resp.Results = append(resp.Results, result)
			continue
		}

		externalID, err := adapter.Push(ctx, spec)
		if err != nil {
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = err.Error()
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		_, err = h.syncStore.CreateSyncMapping(ctx, spec.Slug, adapter.Name(), externalID)
		if err != nil {
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = err.Error()
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		result.ExternalId = externalID
		result.State = specv1.SyncState_SYNC_STATE_SYNCED
		result.Message = "synced"
		resp.Synced++
		resp.Results = append(resp.Results, result)
	}

	return connect.NewResponse(resp), nil
}

func (h *SyncHandler) GetSyncStatus(ctx context.Context, req *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	mappings, err := h.syncStore.ListSyncMappings(ctx, req.Msg.Adapter, req.Msg.SpecSlug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.SyncStatusResponse{Mappings: mappings}), nil
}

func (h *SyncHandler) Inject(ctx context.Context, req *connect.Request[specv1.InjectRequest]) (*connect.Response[specv1.InjectResponse], error) {
	if req.Msg.SpecSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("spec_slug is required"))
	}
	if req.Msg.Tool == specv1.InjectTool_INJECT_TOOL_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tool is required"))
	}

	spec, err := h.specStore.GetSpec(ctx, req.Msg.SpecSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	outputDir := req.Msg.OutputDir
	if outputDir == "" {
		outputDir, _ = os.Getwd()
	}

	// Constitution is optional for injection
	var constitution *specv1.Constitution

	files, err := inject.Inject(spec, constitution, req.Msg.Tool, outputDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&specv1.InjectResponse{
		FilesWritten: files,
		Summary:      "Injected spec context for " + spec.Slug,
	}), nil
}
```

**Step 4: Run the tests**

```bash
go test ./internal/server/ -run TestSync -v -count=1
```

Expected: PASS

**Step 5: Wire into serve.go**

Add `server.RegisterSyncService(mux, store, store, store)` in `cmd/specgraph/serve.go` after the other service registrations:

```go
// In serve.go, after the other RegisterXxxService lines:
server.RegisterSyncService(mux, store, store, store)
```

**Step 6: Verify full build**

```bash
go build ./cmd/specgraph/
```

**Step 7: Commit**

```bash
git add internal/server/sync_handler.go internal/server/sync_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(sync): ConnectRPC SyncService handler with handler tests"
```

---

## Task 8: CLI Commands — sync + inject

**Files:**

- Create: `cmd/specgraph/sync.go`
- Create: `cmd/specgraph/inject.go`

**Step 1: Implement the sync CLI commands**

`cmd/specgraph/sync.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func syncClient() (specgraphv1connect.SyncServiceClient, error) {
	return newClient(specgraphv1connect.NewSyncServiceClient)
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync specs with external systems",
}

// --- sync beads ---

var syncBeadsCmd = &cobra.Command{
	Use:   "beads",
	Short: "Push approved specs to Beads as issues",
	RunE:  runSyncBeads,
}

var (
	syncFilterStage    string
	syncFilterPriority string
	syncDryRun         bool
)

func runSyncBeads(_ *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncBeads(context.Background(), connect.NewRequest(&specv1.SyncBeadsRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_BEADS,
			FilterStage:    syncFilterStage,
			FilterPriority: syncFilterPriority,
			DryRun:         syncDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync beads: %w", err)
	}

	printSyncResponse(resp.Msg)
	return nil
}

// --- sync github ---

var syncGitHubCmd = &cobra.Command{
	Use:   "github",
	Short: "Push specs as GitHub Issues",
	RunE:  runSyncGitHub,
}

func runSyncGitHub(_ *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	resp, err := client.SyncGitHub(context.Background(), connect.NewRequest(&specv1.SyncGitHubRequest{
		Config: &specv1.SyncConfig{
			Adapter:        specv1.SyncAdapter_SYNC_ADAPTER_GITHUB,
			FilterStage:    syncFilterStage,
			FilterPriority: syncFilterPriority,
			DryRun:         syncDryRun,
		},
	}))
	if err != nil {
		return fmt.Errorf("sync github: %w", err)
	}

	printSyncResponse(resp.Msg)
	return nil
}

// --- sync status ---

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync state for all specs",
	RunE:  runSyncStatus,
}

var (
	statusAdapter string
	statusSpec    string
)

func runSyncStatus(cmd *cobra.Command, _ []string) error {
	client, err := syncClient()
	if err != nil {
		return err
	}

	adapter := specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED
	switch statusAdapter {
	case "beads":
		adapter = specv1.SyncAdapter_SYNC_ADAPTER_BEADS
	case "github":
		adapter = specv1.SyncAdapter_SYNC_ADAPTER_GITHUB
	}

	resp, err := client.GetSyncStatus(context.Background(), connect.NewRequest(&specv1.SyncStatusRequest{
		Adapter:  adapter,
		SpecSlug: statusSpec,
	}))
	if err != nil {
		return fmt.Errorf("sync status: %w", err)
	}

	mappings := resp.Msg.Mappings
	if len(mappings) == 0 {
		fmt.Println("No sync mappings found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	tw := &tableWriter{w: w}
	tw.println("SPEC\tADAPTER\tEXTERNAL_ID\tSTATE\tLAST_SYNC")
	for _, m := range mappings {
		lastSync := ""
		if m.LastSync != nil {
			lastSync = m.LastSync.AsTime().Format("2006-01-02 15:04:05")
		}
		tw.printf("%s\t%s\t%s\t%s\t%s\n",
			m.SpecSlug,
			m.Adapter.String(),
			m.ExternalId,
			m.State.String(),
			lastSync,
		)
	}
	if tw.err != nil {
		return tw.err
	}
	return w.Flush()
}

func printSyncResponse(resp *specv1.SyncResponse) {
	fmt.Printf("Synced: %d  Skipped: %d  Errors: %d\n", resp.Synced, resp.Skipped, resp.Errors)
	for _, r := range resp.Results {
		stateIcon := " "
		switch r.State {
		case specv1.SyncState_SYNC_STATE_SYNCED:
			stateIcon = "+"
		case specv1.SyncState_SYNC_STATE_ERROR:
			stateIcon = "!"
		case specv1.SyncState_SYNC_STATE_PENDING:
			stateIcon = "~"
		}
		fmt.Printf("  [%s] %s", stateIcon, r.SpecSlug)
		if r.ExternalId != "" {
			fmt.Printf(" -> %s", r.ExternalId)
		}
		if r.Message != "" {
			fmt.Printf(" (%s)", r.Message)
		}
		fmt.Println()
	}
}

func init() {
	syncBeadsCmd.Flags().StringVar(&syncFilterStage, "stage", "", "only sync specs at this stage")
	syncBeadsCmd.Flags().StringVar(&syncFilterPriority, "priority", "", "only sync specs at this priority")
	syncBeadsCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would be synced without syncing")

	syncGitHubCmd.Flags().StringVar(&syncFilterStage, "stage", "", "only sync specs at this stage")
	syncGitHubCmd.Flags().StringVar(&syncFilterPriority, "priority", "", "only sync specs at this priority")
	syncGitHubCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "show what would be synced without syncing")

	syncStatusCmd.Flags().StringVar(&statusAdapter, "adapter", "", "filter by adapter (beads, github)")
	syncStatusCmd.Flags().StringVar(&statusSpec, "spec", "", "filter by spec slug")

	syncCmd.AddCommand(syncBeadsCmd)
	syncCmd.AddCommand(syncGitHubCmd)
	syncCmd.AddCommand(syncStatusCmd)
	rootCmd.AddCommand(syncCmd)
}
```

**Step 2: Implement the inject CLI command**

`cmd/specgraph/inject.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func injectClient() (specgraphv1connect.SyncServiceClient, error) {
	return newClient(specgraphv1connect.NewSyncServiceClient)
}

var injectCmd = &cobra.Command{
	Use:   "inject <slug>",
	Short: "Write spec context into workspace for a coding tool",
	Long:  "Inject spec execution context (bundle + constitution subset) into tool-specific workspace files.",
	Args:  cobra.ExactArgs(1),
	RunE:  runInject,
}

var (
	injectTool   string
	injectOutput string
)

func runInject(_ *cobra.Command, args []string) error {
	client, err := injectClient()
	if err != nil {
		return err
	}

	tool := specv1.InjectTool_INJECT_TOOL_UNSPECIFIED
	switch strings.ToLower(injectTool) {
	case "claude-code", "claude":
		tool = specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE
	case "cursor":
		tool = specv1.InjectTool_INJECT_TOOL_CURSOR
	case "agents-md", "agents":
		tool = specv1.InjectTool_INJECT_TOOL_AGENTS_MD
	default:
		return fmt.Errorf("unsupported tool: %s (supported: claude-code, cursor, agents-md)", injectTool)
	}

	resp, err := client.Inject(context.Background(), connect.NewRequest(&specv1.InjectRequest{
		SpecSlug:  args[0],
		Tool:      tool,
		OutputDir: injectOutput,
	}))
	if err != nil {
		return fmt.Errorf("inject: %w", err)
	}

	fmt.Println(resp.Msg.Summary)
	for _, f := range resp.Msg.FilesWritten {
		fmt.Printf("  -> %s\n", f)
	}
	return nil
}

func init() {
	injectCmd.Flags().StringVar(&injectTool, "tool", "claude-code", "target tool (claude-code, cursor, agents-md)")
	injectCmd.Flags().StringVarP(&injectOutput, "output", "o", "", "output directory (default: current directory)")
	rootCmd.AddCommand(injectCmd)
}
```

**Step 3: Verify build**

```bash
go build ./cmd/specgraph/
./specgraph sync --help
./specgraph sync beads --help
./specgraph sync github --help
./specgraph sync status --help
./specgraph inject --help
```

Expected: help output for all commands

**Step 4: Commit**

```bash
git add cmd/specgraph/sync.go cmd/specgraph/inject.go
git commit -m "feat(sync): CLI commands — sync beads, sync github, sync status, inject"
```

---

## Task 9: Command Runner — Real Implementation

**Files:**

- Create: `internal/sync/exec.go`
- Create: `internal/sync/exec_test.go`

**Step 1: Write the test**

`internal/sync/exec_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync_test

import (
	"context"
	"testing"

	"github.com/seanb4t/specgraph/internal/sync"
	"github.com/stretchr/testify/require"
)

func TestExecRunner_Echo(t *testing.T) {
	runner := sync.NewExecRunner()
	output, err := runner.Run(context.Background(), "echo", "hello")
	require.NoError(t, err)
	require.Contains(t, string(output), "hello")
}

func TestExecRunner_NotFound(t *testing.T) {
	runner := sync.NewExecRunner()
	_, err := runner.Run(context.Background(), "nonexistent-binary-xyz")
	require.Error(t, err)
}
```

**Step 2: Implement the exec runner**

`internal/sync/exec.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package sync

import (
	"context"
	"os/exec"
)

// ExecRunner implements CommandRunner using os/exec.
type ExecRunner struct{}

// NewExecRunner creates a new ExecRunner.
func NewExecRunner() *ExecRunner {
	return &ExecRunner{}
}

func (r *ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
```

**Step 3: Run the tests**

```bash
go test ./internal/sync/ -run TestExecRunner -v -count=1
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/sync/exec.go internal/sync/exec_test.go
git commit -m "feat(sync): real command runner implementation for adapter CLI execution"
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
./specgraph sync --help
./specgraph sync beads --help
./specgraph sync github --help
./specgraph sync status --help
./specgraph inject --help
```

Expected: all commands show help

**Step 4: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore(sync): cleanup and final verification"
```

---

## Summary

| Task | What | Files | Test Type |
|------|------|-------|-----------|
| 1 | Proto schema | `proto/specgraph/v1/sync.proto` | Compile |
| 2 | Storage interface | `internal/storage/sync.go` | Compile |
| 3 | Memgraph backend | `internal/storage/memgraph/sync.go` | Integration (testcontainers) |
| 4 | SyncAdapter + Beads | `internal/sync/adapter.go`, `internal/sync/beads.go` | Unit (mock runner) |
| 5 | GitHub adapter | `internal/sync/github.go` | Unit (mock runner) |
| 6 | Tool injection | `internal/inject/inject.go` | Unit (temp dirs) |
| 7 | ConnectRPC handler | `internal/server/sync_handler.go` | Unit (mock backend) |
| 8 | CLI commands | `cmd/specgraph/sync.go`, `cmd/specgraph/inject.go` | Build verification |
| 9 | Command runner | `internal/sync/exec.go` | Unit |
| 10 | Final verification | -- | All tests + lint |
