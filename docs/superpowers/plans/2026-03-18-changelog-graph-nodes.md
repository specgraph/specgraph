# ChangeLog Graph Nodes Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `history_json` with ChangeLog graph nodes that capture field-level deltas on every material mutation, with checkpoints at stage transitions.

**Architecture:** ChangeLog is a new Memgraph node type linked to Spec via `HAS_CHANGE` edges. Every mutation that changes `content_hash` atomically creates a ChangeLog node with field deltas. The existing `history_json` property, `HistoryEntry` proto message, and all related marshaling code are removed.

**Tech Stack:** Go, Memgraph (Cypher), protobuf, ConnectRPC, testcontainers

**Spec:** `docs/superpowers/specs/2026-03-18-changelog-graph-nodes-design.md`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Modify | `proto/specgraph/v1/spec.proto` | Remove `HistoryEntry`, add `FieldChange`, reserve field 13 |
| Regen | `gen/specgraph/v1/spec.pb.go` | Regenerated proto code |
| Modify | `internal/storage/spec_domain.go` | Remove `HistoryEntry`/`History`, add `ChangeLogEntry`/`FieldChange` |
| Create | `internal/storage/changelog.go` | `ChangeLogBackend` interface and `ChangeLogFilter` |
| Create | `internal/storage/fieldchange.go` | `ComputeFieldDeltas` function for diffing old vs new spec fields |
| Modify | `internal/storage/memgraph/memgraph.go` | Remove `history_json` from queries/scanning, update `CreateSpec`/`UpdateSpec` to create ChangeLog nodes |
| Modify | `internal/storage/memgraph/lifecycle.go` | Remove `marshalHistory`/`appendHistory`/`maxHistoryEntries`, update lifecycle ops to create ChangeLog nodes |
| Create | `internal/storage/memgraph/changelog.go` | `ListChanges` implementation, ChangeLog Cypher queries, index creation |
| Modify | `internal/storage/memgraph/authoring.go` | Update `TransitionStage`/`Store*Output` to create ChangeLog nodes |
| Modify | `internal/server/convert.go` | Remove `historyToProto`, remove History from `specToProto` |
| Modify | `internal/server/convert_test.go` | Remove `TestHistoryToProto`, update `TestSpecToProto` History assertions |
| Modify | `internal/server/lifecycle_handler_test.go` | Remove History assertions (lines 140-142, 157-158) |
| Modify | `e2e/api/lifecycle_test.go` | Remove `.GetHistory()` assertions |
| Modify | `e2e/api/lifecycle_pipeline_test.go` | Remove `.GetHistory()` assertions (lines 78, 80, 217, 219) |
| Remove tests | `internal/storage/memgraph/lifecycle_unit_test.go` | Remove `TestMarshalHistory_*`, `TestUnmarshalHistory_*`, `TestAppendHistory_*` |
| Create | `internal/storage/fieldchange_test.go` | Unit tests for `ComputeFieldDeltas` |
| Create | `internal/storage/memgraph/changelog_test.go` | Integration tests for ChangeLog creation and querying |
| Modify | `site/docs/concepts/specs.md` | Add "Change Tracking" section |
| Modify | `site/docs/concepts/authoring.md` | Note checkpoint ChangeLog at stage transitions |
| Modify | `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md` | Forward-reference to ChangeLog |
| Modify | `CLAUDE.md` | Add ChangeLog to architecture table, HAS_CHANGE gotcha |

---

## Chunk 1: Proto and Domain Types

### Task 1: Remove HistoryEntry and Add FieldChange in Proto

**Files:**

- Modify: `proto/specgraph/v1/spec.proto:35,40-46`

- [ ] **Step 1: Edit spec.proto**

In `proto/specgraph/v1/spec.proto`:

1. Remove the `repeated HistoryEntry history = 13;` field from `Spec` message (line 35)
2. Add `reserved 13; reserved "history";` in its place
3. Remove the entire `HistoryEntry` message (lines 40-46)
4. Add the `FieldChange` message after the `Spec` message:

```protobuf
message FieldChange {
  string field     = 1;
  string old_value = 2;
  string new_value = 3;
}
```

- [ ] **Step 2: Regenerate proto code**

Run: `task proto`
Expected: Success, `gen/specgraph/v1/spec.pb.go` updated

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Compilation errors in files referencing `HistoryEntry` — this is expected and will be fixed in subsequent tasks.

- [ ] **Step 4: Commit**

```text
feat(proto): replace HistoryEntry with FieldChange message

Remove HistoryEntry message and history field from Spec,
reserving field 13. Add FieldChange message for changelog
field-level deltas.
```

### Task 2: Update Domain Types

**Files:**

- Modify: `internal/storage/spec_domain.go:136-163`
- Create: `internal/storage/changelog.go`
- Create: `internal/storage/fieldchange.go`
- Create: `internal/storage/fieldchange_test.go`

- [ ] **Step 1: Write the failing test for ComputeFieldDeltas**

Create `internal/storage/fieldchange_test.go`:

```go
package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestComputeFieldDeltas_NoChanges(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	new_ := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(old, new_)
	assert.Empty(t, deltas)
}

func TestComputeFieldDeltas_IntentChanged(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	new_ := storage.SpecFields{Intent: "OAuth2 login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(old, new_)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "intent", deltas[0].Field)
	assert.Equal(t, "login", deltas[0].OldValue)
	assert.Equal(t, "OAuth2 login", deltas[0].NewValue)
}

func TestComputeFieldDeltas_MultipleChanges(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "low"}
	new_ := storage.SpecFields{Intent: "OAuth2 login", Stage: "shape", Priority: "p1", Complexity: "low"}
	deltas := storage.ComputeFieldDeltas(old, new_)
	assert.Len(t, deltas, 3)
	fields := make(map[string]bool)
	for _, d := range deltas {
		fields[d.Field] = true
	}
	assert.True(t, fields["intent"])
	assert.True(t, fields["stage"])
	assert.True(t, fields["priority"])
}

func TestComputeFieldDeltas_AuthoringOutputChanged(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark"}
	new_ := storage.SpecFields{Intent: "login", Stage: "spark", SparkOutput: `{"goals":["fast"]}`}
	deltas := storage.ComputeFieldDeltas(old, new_)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "spark_output", deltas[0].Field)
	assert.Equal(t, "", deltas[0].OldValue)
	assert.Equal(t, `{"goals":["fast"]}`, deltas[0].NewValue)
}

func TestComputeFieldDeltas_StageTransitionOnly(t *testing.T) {
	old := storage.SpecFields{Intent: "login", Stage: "spark", Priority: "p2", Complexity: "medium"}
	new_ := storage.SpecFields{Intent: "login", Stage: "shape", Priority: "p2", Complexity: "medium"}
	deltas := storage.ComputeFieldDeltas(old, new_)
	assert.Len(t, deltas, 1)
	assert.Equal(t, "stage", deltas[0].Field)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestComputeFieldDeltas -v`
Expected: FAIL — `ComputeFieldDeltas` not defined

- [ ] **Step 3: Implement FieldChange, SpecFields, and ComputeFieldDeltas**

Create `internal/storage/fieldchange.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

// FieldChange records a single field-level change in a spec mutation.
type FieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// SpecFields holds the substantive fields of a spec for delta computation.
// These are the fields included in the content hash.
type SpecFields struct {
	Intent         string
	Stage          string
	Priority       string
	Complexity     string
	SparkOutput    string
	ShapeOutput    string
	SpecifyOutput  string
	DecomposeOutput string
}

// ComputeFieldDeltas compares two SpecFields and returns a slice of FieldChange
// for every field that differs. Only fields where the value changed are included.
func ComputeFieldDeltas(old, new_ SpecFields) []FieldChange {
	var deltas []FieldChange
	pairs := []struct {
		field    string
		oldVal   string
		newVal   string
	}{
		{"intent", old.Intent, new_.Intent},
		{"stage", old.Stage, new_.Stage},
		{"priority", old.Priority, new_.Priority},
		{"complexity", old.Complexity, new_.Complexity},
		{"spark_output", old.SparkOutput, new_.SparkOutput},
		{"shape_output", old.ShapeOutput, new_.ShapeOutput},
		{"specify_output", old.SpecifyOutput, new_.SpecifyOutput},
		{"decompose_output", old.DecomposeOutput, new_.DecomposeOutput},
	}
	for _, p := range pairs {
		if p.oldVal != p.newVal {
			deltas = append(deltas, FieldChange{Field: p.field, OldValue: p.oldVal, NewValue: p.newVal})
		}
	}
	return deltas
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/ -run TestComputeFieldDeltas -v`
Expected: PASS

- [ ] **Step 5: Create ChangeLogEntry and ChangeLogBackend**

Create `internal/storage/changelog.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// ChangeLogEntry records a single material change to a spec.
type ChangeLogEntry struct {
	ID          string
	Version     int32
	Stage       SpecStage
	ContentHash string
	Checkpoint  bool
	Summary     string
	Reason      string
	Changes     []FieldChange
	Date        time.Time
}

// ChangeLogFilter controls which changelog entries are returned.
type ChangeLogFilter struct {
	CheckpointsOnly bool
	SinceVersion    int32
	Limit           int // 0 means no limit (return all matching entries)
}

// ChangeLogBackend defines storage operations for changelog entries.
type ChangeLogBackend interface {
	// ListChanges returns changelog entries for a spec, ordered by version.
	// Returns an empty slice (not an error) if the spec has no changelog entries.
	// Returns ErrSpecNotFound if the spec slug does not exist.
	ListChanges(ctx context.Context, slug string, opts ChangeLogFilter) ([]*ChangeLogEntry, error)
}
```

- [ ] **Step 6: Remove HistoryEntry from spec_domain.go**

In `internal/storage/spec_domain.go`:

1. Remove the `HistoryEntry` struct (lines 157-163)
2. Remove the `History []HistoryEntry` field from `Spec` struct (line 149)
3. Remove the `Date time.Time` field from HistoryEntry (already covered by removing the struct)

- [ ] **Step 7: Verify build compiles (expect errors in memgraph/ and server/)**

Run: `go build ./internal/storage/...`
Expected: PASS (storage package itself compiles)

Run: `go build ./...`
Expected: FAIL in memgraph and server packages — fixed in next tasks

- [ ] **Step 8: Commit**

```text
feat(storage): add ChangeLogEntry, FieldChange types and ComputeFieldDeltas

Introduce domain types for changelog tracking. Remove HistoryEntry
struct and History field from Spec. Add ChangeLogBackend interface
for querying changelog entries.
```

---

## Chunk 2: Memgraph Migration — Remove history_json

### Task 3: Remove History Marshaling and Old Tests

**Files:**

- Modify: `internal/storage/memgraph/lifecycle.go:15-60`
- Remove tests: `internal/storage/memgraph/lifecycle_unit_test.go` (history-related tests)
- Modify: `internal/storage/memgraph/memgraph.go:529-563` (historyEntryJSON, unmarshalHistory)

- [ ] **Step 1: Remove history marshaling from lifecycle.go**

In `internal/storage/memgraph/lifecycle.go`:

1. Remove `maxHistoryEntries` constant (line 17)
2. Remove `appendHistory` function (lines 32-37)
3. Remove `marshalHistory` function (lines 41-60)
4. Remove `"encoding/json"` from imports if no longer used

- [ ] **Step 2: Remove historyEntryJSON and unmarshalHistory from memgraph.go**

In `internal/storage/memgraph/memgraph.go`:

1. Remove `historyEntryJSON` struct (lines 529-535)
2. Remove `unmarshalHistory` function (lines 539-563)

- [ ] **Step 3: Remove history-related unit tests**

In `internal/storage/memgraph/lifecycle_unit_test.go`, remove these test functions:

- `TestMarshalHistory_TrimsOldEntries`
- `TestMarshalHistory_ExactLimit`
- `TestUnmarshalHistory_InvalidJSON`
- `TestUnmarshalHistory_EmptyAndNil`
- `TestAppendHistory_TrimsOldestWhenFull`
- `TestUnmarshalHistory_UnparseableDate`
- `TestUnmarshalHistory_UnknownStageAccepted`

If no tests remain in the file, delete the file entirely.

- [ ] **Step 4: Verify remaining code compiles**

Run: `go build ./internal/storage/memgraph/...`
Expected: FAIL — still references to `history_json` in queries and `spec.History` in lifecycle ops. Fixed in next tasks.

- [ ] **Step 5: Commit**

```text
refactor(memgraph): remove history marshaling code and tests

Remove marshalHistory, unmarshalHistory, appendHistory,
historyEntryJSON, maxHistoryEntries and all related unit tests.
```

### Task 4: Remove history_json from Spec Queries and Scanning

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:148-201,204-260,300-387,569-682`

This task removes `history_json` from all Cypher queries and the `recordToSpecOffset` scanner function. The scanner currently reads `history_json` at a specific column offset — removing it shifts all subsequent column offsets.

- [ ] **Step 1: Remove history_json from CreateSpec**

In `CreateSpec` (line 148):

1. Remove `history_json: $history_json,` from CREATE clause (line 166)
2. Remove `s.history_json,` from RETURN clause (line 172)
3. Remove `"history_json": "[]",` from params (line 187)

- [ ] **Step 2: Remove history_json from GetSpec query**

In `GetSpec` (line 204): Remove `s.history_json,` from RETURN clause

- [ ] **Step 3: Remove history_json from ListSpecs query**

Find and remove `s.history_json,` from the ListSpecs RETURN clause

- [ ] **Step 4: Remove history_json from UpdateSpec query**

In `UpdateSpec` (line 301): Remove `s.history_json,` from RETURN clause (line 339)

- [ ] **Step 5: Update recordToSpecOffset scanner**

In `recordToSpecOffset` (line 569), the function reads columns by positional offset. `history_json` is at offset+12. Remove this column and shift all subsequent offsets down by 1:

1. Remove the `historyJSON` variable and its `unmarshalHistory` call
2. Remove the `History` field assignment on the returned Spec
3. Adjust offset indices for columns that followed `history_json` (`drift_acknowledged`, `drift_acknowledge_note`, `notes`, `content_hash`)

- [ ] **Step 6: Remove history_json from all remaining queries**

Search for ALL references to `history_json` in Cypher queries — not just `s.history_json` but also `old.history_json`, `new.history_json`, and any other aliases used in lifecycle operations (supersede uses `old`/`new` aliases, not `s`).

Run: `rg "history_json" internal/storage/memgraph/ --type go`

Remove from SET clauses, RETURN clauses, and params maps.

- [ ] **Step 7: Verify build**

Run: `go build ./internal/storage/memgraph/...`
Expected: FAIL — lifecycle.go still references `spec.History` and `appendHistory`. Fixed next.

- [ ] **Step 8: Commit**

```text
refactor(memgraph): remove history_json from all Cypher queries

Remove history_json from CreateSpec, GetSpec, ListSpecs, UpdateSpec
and all lifecycle operation queries. Update recordToSpecOffset
column offsets.
```

### Task 5: Update Lifecycle Operations to Remove History References

**Files:**

- Modify: `internal/storage/memgraph/lifecycle.go:110-240+`

- [ ] **Step 1: Update LifecycleAmendSpec**

In `LifecycleAmendSpec` (line 110):

1. Remove the `storage.HistoryEntry` creation (lines 127-133)
2. Remove the `appendHistory` call (line 134)
3. Remove `s.history_json = $history_json` from SET clause (line 148)
4. Remove `s.history_json` from RETURN clause (line 151)
5. Remove `"history_json": historyJSON` from params (line 162)

The function should now just do the stage transition and version bump — ChangeLog creation is added in a later task.

- [ ] **Step 2: Update LifecycleSupersedeSpec**

In `LifecycleSupersedeSpec` (line 198):

1. Remove `oldEntry` HistoryEntry creation and `appendHistory` call for old spec
2. Remove `newEntry` HistoryEntry creation and `appendHistory` call for new spec
3. Remove `history_json` from SET and RETURN clauses for both specs
4. Remove `history_json` params for both specs

- [ ] **Step 3: Update LifecycleAbandonSpec**

Same pattern: remove HistoryEntry creation, appendHistory call, and history_json from queries.

- [ ] **Step 4: Verify build compiles**

Run: `go build ./internal/storage/memgraph/...`
Expected: PASS — all history_json references removed from memgraph package

- [ ] **Step 5: Verify build for server package**

Run: `go build ./...`
Expected: FAIL in `internal/server/convert.go` — `historyToProto` references removed types. Fixed next.

- [ ] **Step 6: Commit**

```text
refactor(memgraph): remove history from lifecycle operations

Remove HistoryEntry creation, appendHistory calls, and
history_json from amend, supersede, and abandon operations.
```

### Task 6: Remove History from Server Converters and Tests

**Files:**

- Modify: `internal/server/convert.go:43,465-480`

- [ ] **Step 1: Remove historyToProto function**

In `internal/server/convert.go`:

1. Remove `historyToProto` function (lines 465-480)
2. Remove the `History: historyToProto(s.History)` line in `specToProto` (line 43)
3. Remove the `History` field assignment entirely (proto Spec no longer has it)

- [ ] **Step 2: Verify full build passes**

Run: `go build ./...`
Expected: PASS — all `HistoryEntry`/`history_json` references removed

- [ ] **Step 3: Run existing unit tests**

Run: `go test ./... -short`
Expected: PASS (some tests may need updates if they assert on History field — fix any failures)

- [ ] **Step 4: Commit**

```text
refactor(server): remove history converter

Remove historyToProto and History field from specToProto
converter. Proto Spec message no longer includes history.
```

---

## Chunk 3: ChangeLog Write Path

### Task 7: Create ChangeLog Memgraph Implementation

**Files:**

- Create: `internal/storage/memgraph/changelog.go`
- Create: `internal/storage/memgraph/changelog_test.go`

- [ ] **Step 1: Write failing integration test for ChangeLog creation via CreateSpec**

Create `internal/storage/memgraph/changelog_test.go`:

```go
//go:build integration

package memgraph_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSpec_CreatesChangeLogEntry(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-changelog", "test intent", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-changelog", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, int32(1), entry.Version)
	assert.Equal(t, storage.SpecStageSpark, entry.Stage)
	assert.True(t, entry.Checkpoint, "initial creation should be a checkpoint")
	assert.NotEmpty(t, entry.ContentHash)
	assert.NotEmpty(t, entry.ID)
}
```

Note: `setupTestStore` should follow the existing pattern in `memgraph_test.go` for creating a test store with testcontainers. Check existing integration tests for the exact helper signature.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestCreateSpec_CreatesChangeLogEntry -v`
Expected: FAIL — `ListChanges` not implemented

- [ ] **Step 3: Implement changelog.go**

Create `internal/storage/memgraph/changelog.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// changeLogEntryJSON is the JSON serialization format for field changes.
type changeLogEntryJSON struct {
	Field    string `json:"field"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

// marshalFieldChanges serializes a slice of FieldChange to JSON.
func marshalFieldChanges(changes []storage.FieldChange) (string, error) {
	if len(changes) == 0 {
		return "[]", nil
	}
	entries := make([]changeLogEntryJSON, len(changes))
	for i, c := range changes {
		entries[i] = changeLogEntryJSON{
			Field:    c.Field,
			OldValue: c.OldValue,
			NewValue: c.NewValue,
		}
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", fmt.Errorf("memgraph: marshal field changes: %w", err)
	}
	return string(data), nil
}

// unmarshalFieldChanges deserializes a JSON string into a slice of FieldChange.
func unmarshalFieldChanges(raw string) ([]storage.FieldChange, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var entries []changeLogEntryJSON
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("memgraph: unmarshal field changes: %w", err)
	}
	changes := make([]storage.FieldChange, len(entries))
	for i, e := range entries {
		changes[i] = storage.FieldChange{
			Field:    e.Field,
			OldValue: e.OldValue,
			NewValue: e.NewValue,
		}
	}
	return changes, nil
}

// createChangeLog creates a ChangeLog node linked to the given spec via HAS_CHANGE.
func (s *Store) createChangeLog(ctx context.Context, slug string, entry *storage.ChangeLogEntry, changes []storage.FieldChange) error {
	changesJSON, err := marshalFieldChanges(changes)
	if err != nil {
		return err
	}

	id := newID("cl")
	nowStr := entry.Date.UTC().Format(sortableRFC3339Nano)

	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		CREATE (s)-[:HAS_CHANGE]->(:ChangeLog {
			id: $id,
			version: $version,
			stage: $stage,
			content_hash: $content_hash,
			checkpoint: $checkpoint,
			summary: $summary,
			reason: $reason,
			changes_json: $changes_json,
			date: $date
		})
	`
	_, err = s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"slug":         slug,
		"id":           id,
		"version":      int64(entry.Version),
		"stage":        string(entry.Stage),
		"content_hash": entry.ContentHash,
		"checkpoint":   entry.Checkpoint,
		"summary":      entry.Summary,
		"reason":       entry.Reason,
		"changes_json": changesJSON,
		"date":         nowStr,
	}))
	if err != nil {
		return fmt.Errorf("memgraph: create changelog for %q: %w", slug, err)
	}
	return nil
}

// ListChanges returns changelog entries for a spec, ordered by version.
func (s *Store) ListChanges(ctx context.Context, slug string, opts storage.ChangeLogFilter) ([]*storage.ChangeLogEntry, error) {
	// First verify the spec exists.
	checkQuery := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.slug`
	checkRecords, err := s.executeQuery(ctx, checkQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: list changes: %w", err)
	}
	if len(checkRecords) == 0 {
		return nil, fmt.Errorf("memgraph: list changes %q: %w", slug, storage.ErrSpecNotFound)
	}

	// Build the changelog query with optional filters.
	whereClause := ""
	if opts.CheckpointsOnly {
		whereClause = " AND c.checkpoint = true"
	}
	if opts.SinceVersion > 0 {
		whereClause += fmt.Sprintf(" AND c.version > %d", opts.SinceVersion)
	}

	limitClause := ""
	if opts.Limit > 0 {
		limitClause = fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})-[:HAS_CHANGE]->(c:ChangeLog)
		WHERE true%s
		RETURN c.id, c.version, c.stage, c.content_hash, c.checkpoint,
		       c.summary, c.reason, c.changes_json, c.date
		ORDER BY c.version%s
	`, whereClause, limitClause)

	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: list changes: %w", err)
	}

	entries := make([]*storage.ChangeLogEntry, 0, len(records))
	for _, rec := range records {
		entry, recErr := recordToChangeLogEntry(rec)
		if recErr != nil {
			return nil, recErr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// recordToChangeLogEntry converts a Memgraph record to a ChangeLogEntry.
func recordToChangeLogEntry(rec Record) (*storage.ChangeLogEntry, error) {
	id, err := recordString(rec, 0, "c.id")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 1, "c.version")
	if err != nil {
		return nil, err
	}
	stage, err := recordString(rec, 2, "c.stage")
	if err != nil {
		return nil, err
	}
	contentHash, err := recordString(rec, 3, "c.content_hash")
	if err != nil {
		return nil, err
	}
	checkpoint, err := recordBool(rec, 4, "c.checkpoint")
	if err != nil {
		return nil, err
	}
	summary, err := recordStringOptional(rec, 5, "c.summary")
	if err != nil {
		return nil, err
	}
	reason, err := recordStringOptional(rec, 6, "c.reason")
	if err != nil {
		return nil, err
	}
	changesJSON, err := recordStringOptional(rec, 7, "c.changes_json")
	if err != nil {
		return nil, err
	}
	dateStr, err := recordString(rec, 8, "c.date")
	if err != nil {
		return nil, err
	}

	date, err := time.Parse(sortableRFC3339Nano, dateStr)
	if err != nil {
		return nil, fmt.Errorf("memgraph: parse changelog date: %w", err)
	}

	changes, err := unmarshalFieldChanges(changesJSON)
	if err != nil {
		return nil, err
	}

	return &storage.ChangeLogEntry{
		ID:          id,
		Version:     int32(version),
		Stage:       storage.SpecStage(stage),
		ContentHash: contentHash,
		Checkpoint:  checkpoint,
		Summary:     summary,
		Reason:      reason,
		Changes:     changes,
		Date:        date,
	}, nil
}

// EnsureChangeLogIndexes creates label-property indexes on ChangeLog nodes.
// Call this during store initialization.
func (s *Store) EnsureChangeLogIndexes(ctx context.Context) error {
	indexes := []string{
		"CREATE INDEX ON :ChangeLog(version);",
		"CREATE INDEX ON :ChangeLog(date);",
	}
	for _, idx := range indexes {
		if _, err := s.executeQuery(ctx, idx, nil); err != nil {
			return fmt.Errorf("memgraph: create changelog index: %w", err)
		}
	}
	return nil
}
```

Note: The `Record`, `recordString`, `recordInt64`, `recordStringOptional` helpers already exist in `memgraph.go`. However, `recordBool` does NOT exist — only `recordBoolOptional` exists. Create a `recordBool` helper following the same pattern as `recordString` (returns error if null), since `checkpoint` is a required field. Alternatively, use `recordBoolOptional` and default `false` for null, but the explicit helper is cleaner.

**Note on write path:** The spec shows an idealized single Cypher query that updates the Spec and creates the ChangeLog atomically. This plan uses two separate queries (update spec, then `createChangeLog`) wrapped in the same Memgraph session. This is safe because Memgraph sessions are single-writer and SpecGraph is single-writer. The two-query approach is simpler to implement and maintain.

- [ ] **Step 4: Run integration test**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestCreateSpec_CreatesChangeLogEntry -v`
Expected: FAIL — `CreateSpec` doesn't create ChangeLog nodes yet. That's Task 8.

- [ ] **Step 5: Commit**

```text
feat(memgraph): add ChangeLog storage implementation

Add createChangeLog, ListChanges, field change marshaling,
and ChangeLog index creation. ListChanges supports filtering
by checkpoint, version, and limit.
```

### Task 8: Wire ChangeLog into CreateSpec and UpdateSpec

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go:148-201,300-387`

- [ ] **Step 1: Update CreateSpec to create initial ChangeLog**

In `CreateSpec`, after the spec is created successfully, call `createChangeLog`:

```go
// After records[0] check, before return:
spec, err := recordToSpec(records[0])
if err != nil {
    return nil, err
}

// Create initial changelog (checkpoint = true for creation).
allFields := storage.SpecFields{
    Intent:     intent,
    Stage:      defaultInitialStage,
    Priority:   priority,
    Complexity: complexity,
}
deltas := storage.ComputeFieldDeltas(storage.SpecFields{}, allFields)
clEntry := &storage.ChangeLogEntry{
    Version:     spec.Version,
    Stage:       spec.Stage,
    ContentHash: spec.ContentHash,
    Checkpoint:  true,
    Summary:     "Spec created",
    Date:        spec.CreatedAt,
}
if err := s.createChangeLog(ctx, slug, clEntry, deltas); err != nil {
    return nil, err
}
return spec, nil
```

- [ ] **Step 2: Run integration test**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestCreateSpec_CreatesChangeLogEntry -v`
Expected: PASS

- [ ] **Step 3: Write failing test for UpdateSpec changelog**

Add to `changelog_test.go`:

```go
func TestUpdateSpec_CreatesChangeLogEntry(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-update-cl", "initial intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "updated intent"
	_, err = store.UpdateSpec(ctx, "test-update-cl", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-update-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2) // creation + update

	update := entries[1]
	assert.Equal(t, int32(2), update.Version)
	assert.False(t, update.Checkpoint)
	assert.Len(t, update.Changes, 1)
	assert.Equal(t, "intent", update.Changes[0].Field)
	assert.Equal(t, "initial intent", update.Changes[0].OldValue)
	assert.Equal(t, "updated intent", update.Changes[0].NewValue)
}

func TestUpdateSpec_NoChangeLogOnNoOp(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-noop-cl", "same intent", "p2", "medium")
	require.NoError(t, err)

	// Update notes only (not in content hash) — no changelog expected.
	notes := "just a note"
	_, err = store.UpdateSpec(ctx, "test-noop-cl", nil, nil, nil, nil, &notes)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-noop-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, entries, 1) // only creation, no update changelog
}
```

- [ ] **Step 4: Update UpdateSpec to create ChangeLog on material change**

In `UpdateSpec`, the function already reads old values, computes a new hash, and writes it. Add ChangeLog creation between hash computation and return:

1. Before the mutation query, read the current spec via `GetSpec` to capture old field values
2. After the hash is computed, compare old hash vs new hash
3. If different, compute field deltas using domain objects (not raw record offsets) and create ChangeLog

```go
// Before the main UPDATE query, read old state:
oldSpec, err := s.GetSpec(ctx, slug)
if err != nil {
    return nil, err
}

// Capture old authoring outputs by reading the spec's stored JSON properties.
// Use a helper query or extend GetSpec to include authoring outputs.
oldFields := specToFields(oldSpec, oldOutputs)

// ... existing mutation + hash computation ...

// After hash computation, before return:
if ch != oldSpec.ContentHash {
    // Read the updated spec to build newFields from domain objects.
    // Do NOT use raw record column offsets — they are brittle after schema changes.
    newFields := specToFields(spec, newOutputs)
    deltas := storage.ComputeFieldDeltas(oldFields, newFields)
    clEntry := &storage.ChangeLogEntry{
        Version:     spec.Version,
        Stage:       spec.Stage,
        ContentHash: ch,
        Checkpoint:  false,
        Summary:     "Spec updated",
        Date:        spec.UpdatedAt,
    }
    if err := s.createChangeLog(ctx, slug, clEntry, deltas); err != nil {
        return nil, err
    }
}
```

Create a `specToFields` helper in `changelog.go` that builds `storage.SpecFields` from a `*storage.Spec` and authoring output strings. This avoids hardcoding column offsets that shift when fields are added/removed.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestUpdateSpec_Creates -v`
Run: `go test ./internal/storage/memgraph/ -tags integration -run TestUpdateSpec_NoChangeLog -v`
Expected: PASS

- [ ] **Step 6: Commit**

```text
feat(memgraph): create ChangeLog nodes on CreateSpec and UpdateSpec

CreateSpec creates a checkpoint ChangeLog with all initial fields.
UpdateSpec creates a ChangeLog with field deltas only when the
content hash changes (no-op for non-substantive updates like notes).
```

### Task 9: Wire ChangeLog into Authoring Operations

**Files:**

- Modify: `internal/storage/memgraph/authoring.go:51-96,99-143`

- [ ] **Step 1: Write failing test for TransitionStage changelog**

Add to `changelog_test.go`:

```go
func TestTransitionStage_CreatesCheckpointChangeLog(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-transition-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "test-transition-cl", "spark", "shape")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-transition-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	transition := entries[1]
	assert.True(t, transition.Checkpoint, "stage transition should be checkpoint")
	assert.Equal(t, storage.SpecStageShape, transition.Stage)
}
```

- [ ] **Step 2: Update TransitionStage to create ChangeLog**

In `TransitionStage` (authoring.go:51), after `recomputeContentHash`:

1. Read the spec (post-transition) to get the new hash and version
2. Create a checkpoint ChangeLog with the stage change delta

```go
// After recomputeContentHash succeeds:
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
```

- [ ] **Step 3: Write failing test for StoreSparkOutput changelog**

```go
func TestStoreSparkOutput_CreatesChangeLog(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-spark-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "test-spark-cl", &storage.SparkOutput{Goals: []string{"fast"}})
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-spark-cl", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	sparkEntry := entries[1]
	assert.False(t, sparkEntry.Checkpoint)
	assert.Len(t, sparkEntry.Changes, 1)
	assert.Equal(t, "spark_output", sparkEntry.Changes[0].Field)
}
```

- [ ] **Step 4: Update StoreSparkOutput, StoreShapeOutput, StoreSpecifyOutput**

These all call `storeJSONProperty` which then calls `recomputeContentHash`. The ChangeLog creation needs to happen around `recomputeContentHash`. The cleanest approach: read old spec before, read new spec after, compare hashes, create ChangeLog if different.

Modify `storeJSONProperty` or create a wrapper that:

1. Reads old spec fields (including the relevant authoring output)
2. Calls the existing `storeJSONProperty`
3. Reads new spec
4. If hash changed, computes deltas and creates ChangeLog

- [ ] **Step 5: Handle StoreDecomposeOutput separately**

`StoreDecomposeOutput` is different from the other `Store*Output` methods — it creates child specs and `COMPOSES`/`DEPENDS_ON` edges in addition to storing the output. Wire ChangeLog creation for:

1. The parent spec's `decompose_output` field change (non-checkpoint)
2. Each child spec creation (checkpoint, same as `CreateSpec`)

The parent spec ChangeLog should include only the `decompose_output` field delta.

**Note on analytical passes:** Methods like `StoreRedTeamFindings`, `StorePeripheralVision`, `StoreConsistencyIssues`, `StoreSimplicityFindings`, `StoreSafetyFlags`, `StoreConstitutionViolations` also go through `storeJSONProperty` but are NOT in `hashInputProperties` — they do not change `content_hash`. No ChangeLog is created for these, which is correct (they are analysis artifacts, not substantive spec changes).

- [ ] **Step 6: Run all changelog tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestTransitionStage_Creates -v`
Run: `go test ./internal/storage/memgraph/ -tags integration -run TestStoreSparkOutput_Creates -v`
Expected: PASS

- [ ] **Step 7: Commit**

```text
feat(memgraph): create ChangeLog nodes on stage transitions and authoring outputs

TransitionStage creates checkpoint ChangeLog entries.
Store*Output creates non-checkpoint ChangeLog entries with
field deltas for the authoring output field.
```

### Task 10: Wire ChangeLog into Lifecycle Operations

**Files:**

- Modify: `internal/storage/memgraph/lifecycle.go:110-240+`

- [ ] **Step 1: Write failing test for LifecycleAmendSpec changelog**

```go
func TestLifecycleAmendSpec_CreatesCheckpointChangeLog(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-amend-cl", "intent", "p2", "medium")
	require.NoError(t, err)

	// Advance through authoring stages to done.
	// Follow the existing pattern in lifecycle_test.go:
	for _, transition := range []struct{ from, to string }{
		{"spark", "shape"},
		{"shape", "specify"},
		{"specify", "decompose"},
		{"decompose", "approved"},
		{"approved", "in_progress"},
		{"in_progress", "review"},
		{"review", "done"},
	} {
		err = store.TransitionStage(ctx, "test-amend-cl",
			storage.AuthoringStage(transition.from),
			storage.AuthoringStage(transition.to))
		require.NoError(t, err)
	}

	_, err = store.LifecycleAmendSpec(ctx, "test-amend-cl", "needs rework", "shape")
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-amend-cl", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	last := entries[len(entries)-1]
	assert.True(t, last.Checkpoint)
	assert.Contains(t, last.Summary, "Amended")
}
```

- [ ] **Step 2: Update LifecycleAmendSpec**

After the spec is amended and `recomputeContentHash` completes:

```go
updatedSpec, err := s.GetSpec(ctx, slug)
if err != nil {
    return nil, err
}
deltas := []storage.FieldChange{{Field: "stage", OldValue: string(storage.SpecStageDone), NewValue: string(targetStage)}}
clEntry := &storage.ChangeLogEntry{
    Version:     updatedSpec.Version,
    Stage:       updatedSpec.Stage,
    ContentHash: updatedSpec.ContentHash,
    Checkpoint:  true,
    Summary:     amendSummary(targetStage),
    Reason:      reason,
    Date:        updatedSpec.UpdatedAt,
}
if err := s.createChangeLog(ctx, slug, clEntry, deltas); err != nil {
    return nil, err
}
```

- [ ] **Step 3: Update LifecycleSupersedeSpec**

Supersede operates on TWO specs. After the operation completes:

1. Create checkpoint ChangeLog on **old spec**: stage → superseded, reason = "Superseded by {newSlug}"
2. Create checkpoint ChangeLog on **new spec**: version bump, reason = "Supersedes {oldSlug}"

```go
// For old spec:
oldDeltas := []storage.FieldChange{
    {Field: "stage", OldValue: string(oldCheck.Stage), NewValue: string(storage.SpecStageSuperseded)},
    {Field: "superseded_by", OldValue: "", NewValue: newSlug},
}
oldCLEntry := &storage.ChangeLogEntry{
    Version: oldVersion, Stage: storage.SpecStageSuperseded,
    ContentHash: updatedOld.ContentHash, Checkpoint: true,
    Summary: "Spec superseded", Reason: fmt.Sprintf("Superseded by %s", newSlug),
    Date: updatedOld.UpdatedAt,
}
if err := s.createChangeLog(ctx, oldSlug, oldCLEntry, oldDeltas); err != nil {
    return nil, nil, err
}

// For new spec:
newDeltas := []storage.FieldChange{
    {Field: "supersedes", OldValue: "", NewValue: oldSlug},
}
newCLEntry := &storage.ChangeLogEntry{
    Version: newVersion, Stage: updatedNew.Stage,
    ContentHash: updatedNew.ContentHash, Checkpoint: true,
    Summary: "Supersedes predecessor", Reason: fmt.Sprintf("Supersedes %s", oldSlug),
    Date: updatedNew.UpdatedAt,
}
if err := s.createChangeLog(ctx, newSlug, newCLEntry, newDeltas); err != nil {
    return nil, nil, err
}
```

- [ ] **Step 4: Update LifecycleAbandonSpec**

After the operation completes, create a checkpoint ChangeLog on the abandoned spec:

```go
deltas := []storage.FieldChange{
    {Field: "stage", OldValue: string(spec.Stage), NewValue: string(storage.SpecStageAbandoned)},
}
clEntry := &storage.ChangeLogEntry{
    Version: updatedSpec.Version, Stage: storage.SpecStageAbandoned,
    ContentHash: updatedSpec.ContentHash, Checkpoint: true,
    Summary: "Spec abandoned", Reason: reason,
    Date: updatedSpec.UpdatedAt,
}
if err := s.createChangeLog(ctx, slug, clEntry, deltas); err != nil {
    return nil, err
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestLifecycle -v`
Expected: PASS

- [ ] **Step 6: Commit**

```text
feat(memgraph): create ChangeLog nodes on lifecycle operations

Amend, supersede, and abandon operations create checkpoint
ChangeLog entries with stage transition deltas and reasons.
```

---

## Chunk 4: Read Path, Tests, and Documentation

### Task 11: Add ListChanges Filter Tests

**Files:**

- Modify: `internal/storage/memgraph/changelog_test.go`

- [ ] **Step 1: Write integration tests for ListChanges filters**

```go
func TestListChanges_CheckpointsOnly(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-filter-cp", "intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "updated"
	_, err = store.UpdateSpec(ctx, "test-filter-cp", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	// All entries
	all, err := store.ListChanges(ctx, "test-filter-cp", storage.ChangeLogFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Checkpoints only
	cps, err := store.ListChanges(ctx, "test-filter-cp", storage.ChangeLogFilter{CheckpointsOnly: true})
	require.NoError(t, err)
	assert.Len(t, cps, 1) // only creation
}

func TestListChanges_SinceVersion(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-filter-ver", "intent", "p2", "medium")
	require.NoError(t, err)

	newIntent := "v2"
	_, err = store.UpdateSpec(ctx, "test-filter-ver", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)

	entries, err := store.ListChanges(ctx, "test-filter-ver", storage.ChangeLogFilter{SinceVersion: 1})
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, int32(2), entries[0].Version)
}

func TestListChanges_Limit(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.CreateSpec(ctx, "test-filter-lim", "intent", "p2", "medium")
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		v := fmt.Sprintf("intent-%d", i)
		_, err = store.UpdateSpec(ctx, "test-filter-lim", &v, nil, nil, nil, nil)
		require.NoError(t, err)
	}

	entries, err := store.ListChanges(ctx, "test-filter-lim", storage.ChangeLogFilter{Limit: 3})
	require.NoError(t, err)
	assert.Len(t, entries, 3)
	assert.Equal(t, int32(1), entries[0].Version) // ordered by version ASC
}

func TestListChanges_SpecNotFound(t *testing.T) {
	ctx, store := setupTestStore(t)
	_, err := store.ListChanges(ctx, "nonexistent", storage.ChangeLogFilter{})
	require.Error(t, err)
	assert.ErrorIs(t, err, storage.ErrSpecNotFound)
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./internal/storage/memgraph/ -tags integration -run TestListChanges -v`
Expected: PASS

- [ ] **Step 3: Commit**

```text
test(memgraph): add integration tests for ListChanges filters

Test checkpoint-only, since-version, limit, and not-found
error behaviors.
```

### Task 12: Add ChangeLog Index Creation to Store Init

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (store initialization)

- [ ] **Step 1: Find where existing indexes are created**

Search for `CREATE INDEX` in memgraph.go to find the initialization pattern.

- [ ] **Step 2: Add EnsureChangeLogIndexes to store initialization**

Call `EnsureChangeLogIndexes` during store creation, following the same pattern as existing index creation.

- [ ] **Step 3: Run full integration test suite**

Run: `go test ./internal/storage/memgraph/ -tags integration -v`
Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(memgraph): add ChangeLog indexes on version and date
```

### Task 13: Update Existing Integration Tests

**Files:**

- Modify: various test files in `internal/storage/memgraph/`

- [ ] **Step 1: Find all test assertions on spec.History across the FULL codebase**

Run: `rg "spec\.History|\.GetHistory\(\)|historyToProto" --type go -C 2`

Known locations (verify these still exist):

- `internal/server/convert_test.go` — `TestHistoryToProto`, History assertions in `TestSpecToProto`
- `internal/server/lifecycle_handler_test.go` — History assertions (lines ~140-142, ~157-158)
- `e2e/api/lifecycle_test.go` — `.GetHistory()` assertions (lines ~73-75, ~140, ~187-189)
- `e2e/api/lifecycle_pipeline_test.go` — `.GetHistory()` assertions (lines ~78, 80, 217, 219)
- `internal/storage/memgraph/` — any remaining History assertions in integration tests

- [ ] **Step 2: Update internal/server/convert_test.go**

1. Remove `TestHistoryToProto` test function entirely
2. In `TestSpecToProto`, remove the `History` field assertion

- [ ] **Step 3: Update internal/server/lifecycle_handler_test.go**

Remove all `.History` assertions from lifecycle handler tests.

- [ ] **Step 4: Update e2e/api/lifecycle_test.go**

Remove all `.GetHistory()` assertions from e2e lifecycle tests. The proto `Spec` message no longer has a `history` field, so `.GetHistory()` won't compile.

- [ ] **Step 5: Remove any remaining History assertions in memgraph tests**

Check `internal/storage/memgraph/` integration tests for any `spec.History` assertions and remove them.

- [ ] **Step 6: Run full test suite**

Run: `go test ./... -short`
Run: `go test ./internal/storage/memgraph/ -tags integration -v`
Run: `go test ./e2e/... -tags e2e -v` (if Docker available)
Expected: PASS

- [ ] **Step 7: Commit**

```text
test: remove History/HistoryEntry assertions across all test files

Update server converter tests, lifecycle handler tests, and
e2e lifecycle tests to remove references to the removed
History field and HistoryEntry type.
```

### Task 14: Run Full Quality Gates

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: PASS (fmt, lint, build, unit tests)

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`
Expected: PASS (includes integration and e2e tests)

- [ ] **Step 3: Fix any failures and commit fixes**

### Task 15: Documentation Updates

**Files:**

- Modify: `site/docs/concepts/specs.md`
- Modify: `site/docs/concepts/authoring.md`
- Modify: `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add Change Tracking section to specs.md**

Add a "Change Tracking" section explaining ChangeLog nodes, content hash, field deltas, and checkpoints.

- [ ] **Step 2: Add checkpoint note to authoring.md**

In the "Why structured outputs?" section, add a note that stage transitions create checkpoint ChangeLog nodes.

- [ ] **Step 3: Add forward-reference to ADR-002**

Add a note that content hash is now consumed by ChangeLog nodes for field-level change tracking.

- [ ] **Step 4: Update CLAUDE.md**

1. Add `internal/storage/memgraph/changelog.go` row to Architecture table: "ChangeLog node operations (create, list, index)"
2. Add gotcha: "`HAS_CHANGE` edge is internal-only — not in `EdgeType` enum, not exposed via `AddEdge`/`RemoveEdge` RPCs"

- [ ] **Step 5: Commit**

```text
docs: add change tracking documentation

Update specs.md with Change Tracking section, authoring.md with
checkpoint note, ADR-002 with forward-reference, and CLAUDE.md
with architecture and gotcha entries.
```

### Task 16: Close Beads

- [ ] **Step 1: Close related beads**

```bash
bd close spgr-1p6 --reason "Implemented: ChangeLog graph nodes with field deltas"
```

- [ ] **Step 2: Pull beads updates**

```bash
bd dolt pull
```
