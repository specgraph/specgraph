# ConversationLog Graph Nodes Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Store authoring conversation exchanges as graph nodes linked to specs, enabling audit trails and richer CLI/web UI detail views.

**Architecture:** New `ConversationLog` node type with `AUTHORED_VIA`, `CONTINUES`, and `EXPLAINS` edges. Storage follows the ChangeLog pattern (internal edges, JSON-serialized exchanges, project-scoped queries). New `RecordConversation`/`ListConversations` RPCs on AuthoringService. CLI `conversation record`/`list` commands. Render function for markdown output.

**Tech Stack:** Protobuf (buf), Go, ConnectRPC, Memgraph (Cypher), Cobra CLI

**Spec:** `docs/superpowers/specs/2026-03-24-conversation-log-graph-nodes-design.md`

---

## File Structure

| File | Action | Responsibility |
|------|--------|---------------|
| `proto/specgraph/v1/authoring.proto` | Modify | Add ConversationExchange, ConversationLog messages; RecordConversation, ListConversations RPCs |
| `proto/specgraph/v1/spec.proto` | Modify | Add `conversation_logs` field 16 to Spec message |
| `internal/storage/conversation.go` | Create | Domain types (ConversationLogEntry, ConversationExchange) and ConversationBackend interface |
| `internal/storage/memgraph/conversation.go` | Create | Memgraph implementation: RecordConversation, ListConversations, EnsureConversationLogIndexes |
| `internal/storage/memgraph/conversation_test.go` | Create | Integration tests for conversation storage |
| `internal/storage/memgraph/memgraph.go` | Modify | Add ConversationBackend interface assertion, call EnsureConversationLogIndexes |
| `internal/storage/memgraph/graph.go` | Modify | Add AUTHORED_VIA, CONTINUES, EXPLAINS to internal edge exclusion list |
| `internal/storage/spec_domain.go` | Modify | Add ConversationLogs field to Spec struct |
| `internal/server/authoring_handler.go` | Modify | Add RecordConversation, ListConversations RPC handlers |
| `internal/server/authoring_handler_test.go` | Modify | Add handler tests with fake backend |
| `internal/server/convert.go` | Modify | Add conversationLogToProto, conversationLogFromProto converters |
| `internal/render/conversation.go` | Create | Markdown rendering for ConversationLog |
| `cmd/specgraph/conversation.go` | Create | CLI `conversation record` and `conversation list` commands |
| `cmd/specgraph/serve.go` | No change | AuthoringService already registered; new RPCs are auto-included |

---

## Chunk 1: Proto Messages & Code Generation

### Task 1: Add ConversationExchange and ConversationLog proto messages

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto:411` (after AuthoringService definition)
- Modify: `proto/specgraph/v1/spec.proto:38` (after content_hash field)

- [ ] **Step 1: Add proto messages to authoring.proto**

Add after the AuthoringService definition (line 411):

```protobuf
// --- Conversation Log ---

// ConversationExchange represents a single probe/response pair from an authoring session.
message ConversationExchange {
  string role = 1;           // "probe" or "response"
  string content = 2;        // the text of the exchange
  string stage = 3;          // authoring stage (spark, shape, specify, decompose, approve)
  int32 sequence = 4;        // pairs probes with their responses (same sequence = same pair)
  bool decision_point = 5;   // true if user made a judgment call between alternatives
}

// ConversationLog captures the authoring conversation for a single stage completion.
message ConversationLog {
  string id = 1;                              // cvl-prefixed ULID
  string stage = 2;                           // authoring stage
  int32 version = 3;                          // spec version at capture time
  bool is_amend = 4;                          // true if this was an amend re-entry
  repeated ConversationExchange exchanges = 5;
  int32 exchange_count = 6;                   // number of exchanges
  google.protobuf.Timestamp date = 7;         // creation timestamp
}

message RecordConversationRequest {
  string slug = 1;                            // spec slug
  string stage = 2;                           // authoring stage
  repeated ConversationExchange exchanges = 3;
  bool is_amend = 4;
}

message RecordConversationResponse {
  ConversationLog conversation_log = 1;
}

message ListConversationsRequest {
  string slug = 1;
  string stage = 2;  // optional filter; empty = all stages
}

message ListConversationsResponse {
  repeated ConversationLog conversation_logs = 1;
}
```

- [ ] **Step 2: Add RPCs to AuthoringService**

In the `service AuthoringService` block (line 394-411), add before the closing brace:

```protobuf
  // RecordConversation stores authoring conversation exchanges for a spec stage.
  rpc RecordConversation(RecordConversationRequest) returns (RecordConversationResponse);
  // ListConversations returns conversation logs for a spec, in narrative order.
  rpc ListConversations(ListConversationsRequest) returns (ListConversationsResponse);
```

- [ ] **Step 3: Add conversation_logs field to Spec message**

In `proto/specgraph/v1/spec.proto`, after `content_hash` (field 15), add:

```protobuf
  repeated specgraph.v1.ConversationLog conversation_logs = 16; // authoring conversation audit trail
```

Note: This requires importing `authoring.proto` in `spec.proto`. Add to imports:

```protobuf
import "specgraph/v1/authoring.proto";
```

If this creates a circular import (spec.proto ← authoring.proto already imports spec.proto), move the ConversationExchange and ConversationLog messages to `spec.proto` instead and import them from authoring.proto. Check import graph before generating.

- [ ] **Step 4: Generate Go code**

Run: `task proto`

Expected: clean generation, no errors. Verify new files in `gen/specgraph/v1/`.

- [ ] **Step 5: Verify build**

Run: `go build ./...`

Expected: Build may fail because AuthoringServiceHandler interface now has two new methods. That's expected — we'll implement them in Task 4.

- [ ] **Step 6: Commit**

```text
feat(proto): add ConversationLog messages and RPCs (spgr-9mz)
```

---

## Chunk 2: Domain Types & Storage Interface

### Task 2: Create conversation domain types and storage interface

**Files:**

- Create: `internal/storage/conversation.go`

- [ ] **Step 1: Create the conversation storage file**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// ConversationExchange represents a single probe/response from an authoring session.
type ConversationExchange struct {
	Role          string // "probe" or "response"
	Content       string
	Stage         string
	Sequence      int32
	DecisionPoint bool
}

// ConversationLogEntry records the authoring conversation for a single stage completion.
type ConversationLogEntry struct {
	ID            string
	Stage         SpecStage
	Version       int32
	IsAmend       bool
	Exchanges     []ConversationExchange
	ExchangeCount int32
	Date          time.Time
}

// ConversationBackend defines storage operations for conversation logs.
type ConversationBackend interface {
	// RecordConversation stores a conversation log for a spec stage.
	// Links to the most recent ChangeLog via EXPLAINS edge (if one exists).
	// Extends the CONTINUES chain from the previous ConversationLog (if one exists).
	// Returns ErrSpecNotFound if the spec slug does not exist.
	RecordConversation(ctx context.Context, slug string, entry ConversationLogEntry) (*ConversationLogEntry, error)

	// ListConversations returns conversation logs for a spec in narrative chain order.
	// If stage is non-empty, filters to that stage only.
	// Returns an empty slice (not an error) if no conversation logs exist.
	// Returns ErrSpecNotFound if the spec slug does not exist.
	ListConversations(ctx context.Context, slug string, stage string) ([]*ConversationLogEntry, error)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/storage/...`

Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(storage): add ConversationLogEntry domain types and ConversationBackend interface (spgr-9mz)
```

### Task 3: Add ConversationLogs field to Spec domain type

**Files:**

- Modify: `internal/storage/spec_domain.go:150` (after ContentHash field)

- [ ] **Step 1: Add field to Spec struct**

In `internal/storage/spec_domain.go`, add after the `ContentHash` field (line 150):

```go
	ConversationLogs []*ConversationLogEntry // authoring conversation audit trail (populated by GetSpec)
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/storage/...`

Expected: PASS

- [ ] **Step 3: Commit**

```text
feat(storage): add ConversationLogs field to Spec domain type (spgr-9mz)
```

---

## Chunk 3: Memgraph Implementation

### Task 4: Implement Memgraph conversation storage

**Files:**

- Create: `internal/storage/memgraph/conversation.go`
- Modify: `internal/storage/memgraph/memgraph.go:26` (interface assertion), `memgraph.go:126` (index call)

- [ ] **Step 1: Write integration test for RecordConversation (happy path)**

Create `internal/storage/memgraph/conversation_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build integration

package memgraph_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordConversation_CreatesNode(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	// Create a spec first (this also creates a ChangeLog entry).
	_, err = store.CreateSpec(ctx, "test-conv", "test intent", "p2", "medium")
	require.NoError(t, err)

	entry := storage.ConversationLogEntry{
		Stage:   storage.SpecStageSpark,
		IsAmend: false,
		Exchanges: []storage.ConversationExchange{
			{Role: "probe", Content: "What is the seed idea?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "Build a widget factory", Stage: "spark", Sequence: 1, DecisionPoint: true},
		},
		ExchangeCount: 2,
	}

	result, err := store.RecordConversation(ctx, "test-conv", entry)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, storage.SpecStageSpark, result.Stage)
	assert.Equal(t, int32(1), result.Version)
	assert.Equal(t, int32(2), result.ExchangeCount)
	assert.False(t, result.IsAmend)
	assert.NotZero(t, result.Date)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `task test:integration -- -run TestRecordConversation_CreatesNode -v`

Expected: FAIL — `RecordConversation` method not found on Store.

- [ ] **Step 3: Create conversation.go with RecordConversation implementation**

Create `internal/storage/memgraph/conversation.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.ConversationBackend = (*Store)(nil)

// conversationExchangeJSON is the JSON-serializable representation of an exchange.
type conversationExchangeJSON struct {
	Role          string `json:"role"`
	Content       string `json:"content"`
	Stage         string `json:"stage"`
	Sequence      int32  `json:"sequence"`
	DecisionPoint bool   `json:"decision_point,omitempty"`
}

// marshalExchanges serializes exchanges to a JSON string for storage.
func marshalExchanges(exchanges []storage.ConversationExchange) (string, error) {
	items := make([]conversationExchangeJSON, len(exchanges))
	for i, e := range exchanges {
		items[i] = conversationExchangeJSON{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal exchanges: %w", err)
	}
	return string(b), nil
}

// unmarshalExchanges deserializes exchanges from a JSON string.
func unmarshalExchanges(raw string) ([]storage.ConversationExchange, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []conversationExchangeJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("unmarshal exchanges: %w", err)
	}
	result := make([]storage.ConversationExchange, len(items))
	for i, item := range items {
		result[i] = storage.ConversationExchange{
			Role:          item.Role,
			Content:       item.Content,
			Stage:         item.Stage,
			Sequence:      item.Sequence,
			DecisionPoint: item.DecisionPoint,
		}
	}
	return result, nil
}

// RecordConversation stores a conversation log for a spec stage.
func (s *Store) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	var result *storage.ConversationLogEntry

	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// 1. Verify spec exists and get current version.
		specQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			RETURN s.version AS version
		`
		specRecords, specErr := s.executeQuery(txCtx, specQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if specErr != nil {
			return fmt.Errorf("memgraph: record conversation: verify spec: %w", specErr)
		}
		if len(specRecords) == 0 {
			return storage.ErrSpecNotFound
		}
		version, vErr := recordInt64(specRecords[0], 0, "version")
		if vErr != nil {
			return vErr
		}

		// 2. Find the most recent ChangeLog for this stage+version (for EXPLAINS edge).
		changeLogQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:HAS_CHANGE]->(cl:ChangeLog)
			WHERE cl.stage = $stage AND cl.version = $version
			RETURN cl.id AS cl_id
			ORDER BY cl.date DESC
			LIMIT 1
		`
		clParams := mergeParams(s.projectParam(), map[string]any{
			"slug":    slug,
			"stage":   string(entry.Stage),
			"version": version,
		})
		clRecords, clErr := s.executeQuery(txCtx, changeLogQuery, clParams)
		if clErr != nil {
			return fmt.Errorf("memgraph: record conversation: find changelog: %w", clErr)
		}
		var changeLogID string
		if len(clRecords) > 0 {
			changeLogID, _ = recordString(clRecords[0], 0, "cl_id")
		}

		// 3. Find the current CONTINUES chain tail.
		tailQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:AUTHORED_VIA]->(first:ConversationLog)
			OPTIONAL MATCH (first)-[:CONTINUES*0..10]->(tail:ConversationLog)
			WHERE NOT (tail)-[:CONTINUES]->(:ConversationLog)
			RETURN tail.id AS tail_id
		`
		tailRecords, tailErr := s.executeQuery(txCtx, tailQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if tailErr != nil {
			return fmt.Errorf("memgraph: record conversation: find tail: %w", tailErr)
		}
		var tailID string
		if len(tailRecords) > 0 {
			tailID, _ = recordString(tailRecords[0], 0, "tail_id")
		}

		// 4. Create the ConversationLog node.
		id := newID("cvl")
		dateStr := s.now()
		exchangesJSON, mErr := marshalExchanges(entry.Exchanges)
		if mErr != nil {
			return mErr
		}

		createQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			CREATE (cvl:ConversationLog {
				id: $id,
				stage: $stage,
				version: $version,
				is_amend: $is_amend,
				exchanges_json: $exchanges_json,
				exchange_count: $exchange_count,
				date: $date
			})
			RETURN cvl.id AS id
		`
		createParams := mergeParams(s.projectParam(), map[string]any{
			"slug":           slug,
			"id":             id,
			"stage":          string(entry.Stage),
			"version":        version,
			"is_amend":       entry.IsAmend,
			"exchanges_json": exchangesJSON,
			"exchange_count": int64(entry.ExchangeCount),
			"date":           dateStr,
		})
		createRecords, createErr := s.executeQuery(txCtx, createQuery, createParams)
		if createErr != nil {
			return fmt.Errorf("memgraph: record conversation: create node: %w", createErr)
		}
		if len(createRecords) == 0 {
			return fmt.Errorf("memgraph: record conversation: no rows returned from CREATE")
		}

		// 5a. AUTHORED_VIA edge (only if this is the first ConversationLog for this spec).
		if tailID == "" {
			edgeQuery := `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}),
				      (cvl:ConversationLog {id: $cvl_id})
				CREATE (s)-[:AUTHORED_VIA]->(cvl)
			`
			_, edgeErr := s.executeQuery(txCtx, edgeQuery, mergeParams(s.projectParam(), map[string]any{
				"slug":   slug,
				"cvl_id": id,
			}))
			if edgeErr != nil {
				return fmt.Errorf("memgraph: record conversation: create AUTHORED_VIA: %w", edgeErr)
			}
		}

		// 5b. CONTINUES edge (from previous tail to this node).
		if tailID != "" {
			contQuery := `
				MATCH (prev:ConversationLog {id: $tail_id}),
				      (cvl:ConversationLog {id: $cvl_id})
				CREATE (prev)-[:CONTINUES]->(cvl)
			`
			_, contErr := s.executeQuery(txCtx, contQuery, map[string]any{
				"tail_id": tailID,
				"cvl_id":  id,
			})
			if contErr != nil {
				return fmt.Errorf("memgraph: record conversation: create CONTINUES: %w", contErr)
			}
		}

		// 5c. EXPLAINS edge (to the matching ChangeLog, if found).
		if changeLogID != "" {
			explQuery := `
				MATCH (cvl:ConversationLog {id: $cvl_id}),
				      (cl:ChangeLog {id: $cl_id})
				CREATE (cvl)-[:EXPLAINS]->(cl)
			`
			_, explErr := s.executeQuery(txCtx, explQuery, map[string]any{
				"cvl_id": id,
				"cl_id":  changeLogID,
			})
			if explErr != nil {
				return fmt.Errorf("memgraph: record conversation: create EXPLAINS: %w", explErr)
			}
		}

		result = &storage.ConversationLogEntry{
			ID:            id,
			Stage:         entry.Stage,
			Version:       int32(version),
			IsAmend:       entry.IsAmend,
			Exchanges:     entry.Exchanges,
			ExchangeCount: entry.ExchangeCount,
			Date:          s.nowTime(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListConversations returns conversation logs for a spec in narrative chain order.
func (s *Store) ListConversations(ctx context.Context, slug string, stage string) ([]*storage.ConversationLogEntry, error) {
	// Verify spec exists.
	checkQuery := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.slug`
	checkRecords, err := s.executeQuery(ctx, checkQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: list conversations: %w", err)
	}
	if len(checkRecords) == 0 {
		return nil, storage.ErrSpecNotFound
	}

	// Fetch conversation logs in chain order.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		      -[:AUTHORED_VIA]->(first:ConversationLog)
		OPTIONAL MATCH path = (first)-[:CONTINUES*0..10]->(log)
		RETURN log.id AS id,
		       log.stage AS stage,
		       log.version AS version,
		       log.is_amend AS is_amend,
		       log.exchanges_json AS exchanges_json,
		       log.exchange_count AS exchange_count,
		       log.date AS date
		ORDER BY length(path)
	`
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, qErr := s.executeQuery(ctx, query, params)
	if qErr != nil {
		return nil, fmt.Errorf("memgraph: list conversations: %w", qErr)
	}

	var entries []*storage.ConversationLogEntry
	for _, rec := range records {
		e, pErr := recordToConversationLogEntry(rec)
		if pErr != nil {
			return nil, pErr
		}
		if stage != "" && string(e.Stage) != stage {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// recordToConversationLogEntry parses a neo4j record into a ConversationLogEntry.
func recordToConversationLogEntry(rec *neo4j.Record) (*storage.ConversationLogEntry, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	stageStr, err := recordString(rec, 1, "stage")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 2, "version")
	if err != nil {
		return nil, err
	}
	isAmend, ok := rec.Values[3].(bool)
	if !ok {
		return nil, fmt.Errorf("memgraph: conversation log: expected bool for is_amend, got %T", rec.Values[3])
	}
	exchangesJSON, err := recordString(rec, 4, "exchanges_json")
	if err != nil {
		return nil, err
	}
	exchangeCount, err := recordInt64(rec, 5, "exchange_count")
	if err != nil {
		return nil, err
	}
	dateStr, err := recordString(rec, 6, "date")
	if err != nil {
		return nil, err
	}

	exchanges, uErr := unmarshalExchanges(exchangesJSON)
	if uErr != nil {
		return nil, uErr
	}
	date, tErr := parseRFC3339("date", dateStr)
	if tErr != nil {
		return nil, tErr
	}

	return &storage.ConversationLogEntry{
		ID:            id,
		Stage:         storage.SpecStage(stageStr),
		Version:       int32(version),
		IsAmend:       isAmend,
		Exchanges:     exchanges,
		ExchangeCount: int32(exchangeCount),
		Date:          date,
	}, nil
}

// EnsureConversationLogIndexes creates indexes on ConversationLog nodes.
// Called from ensureIndexes during Store initialization.
func (s *Store) EnsureConversationLogIndexes(ctx context.Context) error {
	indexes := []string{
		"CREATE INDEX ON :ConversationLog(id)",
		"CREATE INDEX ON :ConversationLog(date)",
	}
	for _, stmt := range indexes {
		session := s.driver.NewSession(ctx, neo4j.SessionConfig{})
		_, runErr := session.Run(ctx, stmt, nil)
		closeErr := session.Close(ctx)
		if runErr != nil && !strings.Contains(runErr.Error(), "already exists") {
			if closeErr != nil {
				return errors.Join(
					fmt.Errorf("create conversation log index %q: %w", stmt, runErr),
					fmt.Errorf("close session: %w", closeErr),
				)
			}
			return fmt.Errorf("create conversation log index %q: %w", stmt, runErr)
		}
		if closeErr != nil {
			return fmt.Errorf("close session after index %q: %w", stmt, closeErr)
		}
	}
	return nil
}
```

Note: Use `parseRFC3339("date", dateStr)` (defined in `memgraph.go:602`) instead of creating a new parser — it handles both RFC3339 and RFC3339Nano with fallback. Use `s.now()` for formatted date strings and `s.nowTime()` for `time.Time` values (defined in `memgraph.go:578-586`).

- [ ] **Step 4: Add interface assertion and index call to memgraph.go**

In `internal/storage/memgraph/memgraph.go`:

1. Add to the `var` block at line 25-32:

```go
	_ storage.ConversationBackend = (*Store)(nil)
```

2. After `s.EnsureChangeLogIndexes(ctx)` call at line 126, add:

```go
	if err := s.EnsureConversationLogIndexes(ctx); err != nil {
		return err
	}
```

Adjust the existing line so both calls chain properly:

```go
	if err := s.EnsureChangeLogIndexes(ctx); err != nil {
		return err
	}
	return s.EnsureConversationLogIndexes(ctx)
```

- [ ] **Step 5: Run integration test**

Run: `task test:integration -- -run TestRecordConversation_CreatesNode -v`

Expected: PASS

- [ ] **Step 6: Commit**

```text
feat(memgraph): implement ConversationLog storage with RecordConversation (spgr-9mz)
```

### Task 5: Integration tests for conversation storage edge cases

**Files:**

- Modify: `internal/storage/memgraph/conversation_test.go`

- [ ] **Step 1: Write test for ListConversations chain order**

Append to `conversation_test.go`:

```go
func TestListConversations_ReturnsChainOrder(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-list", "test intent", "p2", "medium")
	require.NoError(t, err)

	// Record spark conversation.
	_, err = store.RecordConversation(ctx, "test-conv-list", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	// Transition to shape and record shape conversation.
	err = store.TransitionStage(ctx, "test-conv-list", "spark", "shape")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-list", storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-list", "")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	assert.Equal(t, storage.SpecStageSpark, entries[0].Stage)
	assert.Equal(t, storage.SpecStageShape, entries[1].Stage)
}
```

- [ ] **Step 2: Write test for nonexistent spec**

```go
func TestRecordConversation_NonexistentSpec(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.RecordConversation(ctx, "nonexistent", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}

func TestListConversations_NonexistentSpec(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.ListConversations(ctx, "nonexistent", "")
	require.ErrorIs(t, err, storage.ErrSpecNotFound)
}
```

- [ ] **Step 3: Write test for stage filter**

```go
func TestListConversations_StageFilter(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-filter", "test", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "test-conv-filter", "spark", "shape")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-conv-filter", storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-filter", "shape")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, storage.SpecStageShape, entries[0].Stage)
}
```

- [ ] **Step 4: Write test for no conversations returns empty slice**

```go
func TestListConversations_EmptyReturnsNil(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-empty", "test", "p2", "medium")
	require.NoError(t, err)

	entries, err := store.ListConversations(ctx, "test-conv-empty", "")
	require.NoError(t, err)
	assert.Empty(t, entries)
}
```

- [ ] **Step 5: Write test for EXPLAINS edge**

```go
func TestRecordConversation_CreatesExplainsEdge(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-conv-explains", "test", "p2", "medium")
	require.NoError(t, err)

	convResult, err := store.RecordConversation(ctx, "test-conv-explains", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	// Verify EXPLAINS edge exists via ListChanges + conversation version correlation.
	// The EXPLAINS edge should point to the spark checkpoint ChangeLog.
	changes, err := store.ListChanges(ctx, "test-conv-explains", storage.ChangeLogFilter{})
	require.NoError(t, err)
	require.NotEmpty(t, changes)

	// The ConversationLog version should match a ChangeLog's version.
	assert.Equal(t, changes[0].Version, convResult.Version,
		"ConversationLog version should match the ChangeLog it explains")
}
```

Note: If `executeQuery` is not exported, use the neo4j driver directly for this edge verification. Check how other tests verify internal edges.

- [ ] **Step 6: Run all conversation tests**

Run: `task test:integration -- -run TestRecordConversation -v && task test:integration -- -run TestListConversations -v`

Expected: All PASS

- [ ] **Step 7: Commit**

```text
test(memgraph): add conversation log integration tests for edge cases (spgr-9mz)
```

### Task 6: Update edge exclusion list

**Files:**

- Modify: `internal/storage/memgraph/graph.go:115,119`

- [ ] **Step 1: Add new edge types to exclusion list**

In `internal/storage/memgraph/graph.go`, update the WHERE clauses at lines 115 and 119. Change:

```text
WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE" AND type(r) <> "HAS_FINDING"
```

To:

```text
WHERE type(r) <> "BELONGS_TO" AND type(r) <> "HAS_CHANGE" AND type(r) <> "HAS_FINDING" AND type(r) <> "AUTHORED_VIA" AND type(r) <> "CONTINUES" AND type(r) <> "EXPLAINS"
```

Apply to ALL THREE locations:

- Outgoing edges (line 115)
- Incoming edges (line 119)
- `GetFullGraph` (line 307) — same pattern, also excludes internal edges

- [ ] **Step 2: Update the comment**

Change the comment at line 111:

```go
		// Exclude internal infrastructure edges (BELONGS_TO, HAS_CHANGE, HAS_FINDING,
		// AUTHORED_VIA, CONTINUES, EXPLAINS) from unfiltered listing — these are not
		// user-facing edge types.
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/memgraph/...`

Expected: PASS

- [ ] **Step 4: Commit**

```text
fix(memgraph): exclude conversation log edges from user-facing edge queries (spgr-9mz)
```

---

### Task 6b: Augment GetSpec to populate ConversationLogs

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (in `GetSpec` method)

The design spec requires `GetSpec` to return conversation logs inline. Without this, the `Spec.ConversationLogs` field is always empty and neither CLI show nor web UI can display conversations.

- [ ] **Step 1: Write integration test for GetSpec with conversation logs**

Append to `conversation_test.go`:

```go
func TestGetSpec_IncludesConversationLogs(t *testing.T) {
	clearDatabase(t)

	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "test-getspec-conv", "test intent", "p2", "medium")
	require.NoError(t, err)

	_, err = store.RecordConversation(ctx, "test-getspec-conv", storage.ConversationLogEntry{
		Stage:         storage.SpecStageSpark,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1}},
		ExchangeCount: 1,
	})
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "test-getspec-conv")
	require.NoError(t, err)
	require.Len(t, spec.ConversationLogs, 1)
	assert.Equal(t, storage.SpecStageSpark, spec.ConversationLogs[0].Stage)
	assert.Len(t, spec.ConversationLogs[0].Exchanges, 1)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `task test:integration -- -run TestGetSpec_IncludesConversationLogs -v`

Expected: FAIL — `ConversationLogs` is nil.

- [ ] **Step 3: Add conversation log query to GetSpec**

In `internal/storage/memgraph/memgraph.go`, after the main `GetSpec` query returns the spec, add a second query to fetch conversation logs:

```go
	// Fetch conversation logs in chain order (if any exist).
	convQuery := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		      -[:AUTHORED_VIA]->(first:ConversationLog)
		OPTIONAL MATCH path = (first)-[:CONTINUES*0..10]->(log)
		RETURN log.id AS id,
		       log.stage AS stage,
		       log.version AS version,
		       log.is_amend AS is_amend,
		       log.exchanges_json AS exchanges_json,
		       log.exchange_count AS exchange_count,
		       log.date AS date
		ORDER BY length(path)
	`
	convRecords, convErr := s.executeQuery(ctx, convQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if convErr != nil {
		return nil, fmt.Errorf("memgraph: get spec conversation logs: %w", convErr)
	}
	for _, rec := range convRecords {
		entry, pErr := recordToConversationLogEntry(rec)
		if pErr != nil {
			return nil, pErr
		}
		spec.ConversationLogs = append(spec.ConversationLogs, entry)
	}
```

Add this after the existing `GetSpec` populates the `Spec` struct but before it returns.

- [ ] **Step 4: Run test**

Run: `task test:integration -- -run TestGetSpec_IncludesConversationLogs -v`

Expected: PASS

- [ ] **Step 5: Commit**

```text
feat(memgraph): populate ConversationLogs in GetSpec response (spgr-9mz)
```

---

## Chunk 4: Server Handler & Converters

### Task 7: Add proto-to-domain converters for ConversationLog

**Files:**

- Modify: `internal/server/convert.go`

- [ ] **Step 1: Add converter functions**

Append to `internal/server/convert.go`:

```go
// conversationLogToProto converts a storage ConversationLogEntry to a proto ConversationLog.
func conversationLogToProto(entry *storage.ConversationLogEntry) *specv1.ConversationLog {
	if entry == nil {
		return nil
	}
	exchanges := make([]*specv1.ConversationExchange, len(entry.Exchanges))
	for i, e := range entry.Exchanges {
		exchanges[i] = &specv1.ConversationExchange{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	return &specv1.ConversationLog{
		Id:            entry.ID,
		Stage:         string(entry.Stage),
		Version:       entry.Version,
		IsAmend:       entry.IsAmend,
		Exchanges:     exchanges,
		ExchangeCount: entry.ExchangeCount,
		Date:          timeToProto(entry.Date),
	}
}

// conversationExchangesFromProto converts proto exchanges to storage domain types.
func conversationExchangesFromProto(exchanges []*specv1.ConversationExchange) []storage.ConversationExchange {
	result := make([]storage.ConversationExchange, len(exchanges))
	for i, e := range exchanges {
		result[i] = storage.ConversationExchange{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	return result
}
```

- [ ] **Step 2: Update specToProto to include conversation_logs**

In the `specToProto` function, add after the existing field mappings:

```go
	if spec.ConversationLogs != nil {
		logs := make([]*specv1.ConversationLog, len(spec.ConversationLogs))
		for i, entry := range spec.ConversationLogs {
			logs[i] = conversationLogToProto(entry)
		}
		protoSpec.ConversationLogs = logs
	}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/server/...`

Expected: May fail if AuthoringServiceHandler methods aren't implemented yet. That's OK — proceed to Task 8.

- [ ] **Step 4: Commit**

```text
feat(server): add ConversationLog proto-domain converters (spgr-9mz)
```

### Task 8: Implement RecordConversation and ListConversations RPC handlers

**Files:**

- Modify: `internal/server/authoring_handler.go`

- [ ] **Step 1: Add RecordConversation handler**

Add to `AuthoringHandler` (after existing RPC methods):

```go
// RecordConversation stores authoring conversation exchanges for a spec stage.
func (h *AuthoringHandler) RecordConversation(
	ctx context.Context,
	req *connect.Request[specv1.RecordConversationRequest],
) (*connect.Response[specv1.RecordConversationResponse], error) {
	slug := req.Msg.Slug
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}
	stage := req.Msg.Stage
	if stage == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("stage is required"))
	}
	if len(req.Msg.Exchanges) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at least one exchange is required"))
	}
	if len(req.Msg.Exchanges) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("exchanges exceed maximum of %d", maxElements))
	}

	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, h.stageError(err)
	}
	convBackend, ok := store.(storage.ConversationBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("storage backend does not support conversation logs"))
	}

	entry := storage.ConversationLogEntry{
		Stage:         storage.SpecStage(stage),
		IsAmend:       req.Msg.IsAmend,
		Exchanges:     conversationExchangesFromProto(req.Msg.Exchanges),
		ExchangeCount: int32(len(req.Msg.Exchanges)),
	}

	result, recErr := convBackend.RecordConversation(ctx, slug, entry)
	if recErr != nil {
		return nil, h.stageError("RecordConversation", slug, recErr)
	}

	return connect.NewResponse(&specv1.RecordConversationResponse{
		ConversationLog: conversationLogToProto(result),
	}), nil
}
```

- [ ] **Step 2: Add ListConversations handler**

```go
// ListConversations returns conversation logs for a spec in narrative order.
func (h *AuthoringHandler) ListConversations(
	ctx context.Context,
	req *connect.Request[specv1.ListConversationsRequest],
) (*connect.Response[specv1.ListConversationsResponse], error) {
	slug := req.Msg.Slug
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("slug is required"))
	}

	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, h.stageError(err)
	}
	convBackend, ok := store.(storage.ConversationBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("storage backend does not support conversation logs"))
	}

	entries, listErr := convBackend.ListConversations(ctx, slug, req.Msg.Stage)
	if listErr != nil {
		return nil, h.stageError("ListConversations", slug, listErr)
	}

	logs := make([]*specv1.ConversationLog, len(entries))
	for i, e := range entries {
		logs[i] = conversationLogToProto(e)
	}

	return connect.NewResponse(&specv1.ListConversationsResponse{
		ConversationLogs: logs,
	}), nil
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

Expected: PASS — all AuthoringServiceHandler methods now implemented.

- [ ] **Step 4: Commit**

```text
feat(server): implement RecordConversation and ListConversations RPC handlers (spgr-9mz)
```

### Task 9: Handler unit tests

**Files:**

- Modify: `internal/server/authoring_handler_test.go`

- [ ] **Step 1: Add ConversationBackend to fake backend**

Add to the fake backend types in the test file:

```go
type fakeConversationBackend struct {
	recordErr error
	listErr   error
	entries   []*storage.ConversationLogEntry
}

func (f *fakeConversationBackend) RecordConversation(_ context.Context, _ string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	if f.recordErr != nil {
		return nil, f.recordErr
	}
	result := &storage.ConversationLogEntry{
		ID:            "cvl-test",
		Stage:         entry.Stage,
		Version:       1,
		IsAmend:       entry.IsAmend,
		Exchanges:     entry.Exchanges,
		ExchangeCount: entry.ExchangeCount,
		Date:          time.Now(),
	}
	return result, nil
}

func (f *fakeConversationBackend) ListConversations(_ context.Context, _ string, _ string) ([]*storage.ConversationLogEntry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.entries, nil
}
```

Then compose this into the scoped backend type used by tests. The exact composition depends on how the existing fakes implement `ScopedBackend` — follow the same pattern used for `fakeAuthoringBackend`. The key is that the scoped backend returned by the test scoper must also implement `storage.ConversationBackend`.

- [ ] **Step 2: Write handler tests**

```go
func TestAuthoringHandler_RecordConversation(t *testing.T) {
	// Setup with conversation-capable backend
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{})

	resp, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:  "test-spec",
		Stage: "spark",
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "What's the seed?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "Build widgets", Stage: "spark", Sequence: 1, DecisionPoint: true},
		},
	}))
	require.NoError(t, err)
	assert.Equal(t, "cvl-test", resp.Msg.ConversationLog.Id)
	assert.Equal(t, "spark", resp.Msg.ConversationLog.Stage)
	assert.Len(t, resp.Msg.ConversationLog.Exchanges, 2)
}

func TestAuthoringHandler_RecordConversation_MissingSlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Stage:     "spark",
		Exchanges: []*specv1.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_RecordConversation_SpecNotFound(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{recordErr: storage.ErrSpecNotFound})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:      "nonexistent",
		Stage:     "spark",
		Exchanges: []*specv1.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestAuthoringHandler_ListConversations(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		entries: []*storage.ConversationLogEntry{
			{ID: "cvl-1", Stage: storage.SpecStageSpark, Version: 1, ExchangeCount: 2},
		},
	})

	resp, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug: "test-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.ConversationLogs, 1)
	assert.Equal(t, "cvl-1", resp.Msg.ConversationLogs[0].Id)
}
```

Note: The exact fake composition pattern must match the existing test infrastructure. Adapt the `fakeConvBackend` to compose with existing `fakeBackend`/`fakeAuthoringBackend` as needed.

- [ ] **Step 3: Run handler tests**

Run: `go test ./internal/server/ -run TestAuthoringHandler_RecordConversation -v && go test ./internal/server/ -run TestAuthoringHandler_ListConversations -v`

Expected: PASS

- [ ] **Step 4: Commit**

```text
test(server): add RecordConversation and ListConversations handler tests (spgr-9mz)
```

---

## Chunk 5: CLI Commands & Render

### Task 10: Create render function for ConversationLog

**Files:**

- Create: `internal/render/conversation.go`

- [ ] **Step 1: Create conversation renderer**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// ConversationLog renders a single conversation log as markdown.
func ConversationLog(log *specv1.ConversationLog) string {
	if log == nil || len(log.Exchanges) == 0 {
		return ""
	}
	var b strings.Builder

	amendLabel := ""
	if log.IsAmend {
		amendLabel = ", amend"
	}
	fmt.Fprintf(&b, "### Authoring Conversation (%s, v%d%s)\n\n", log.Stage, log.Version, amendLabel)

	// Group exchanges by sequence number (probe + response pairs).
	type pair struct {
		probe    *specv1.ConversationExchange
		response *specv1.ConversationExchange
	}
	pairs := make(map[int32]*pair)
	var order []int32
	for _, e := range log.Exchanges {
		p, ok := pairs[e.Sequence]
		if !ok {
			p = &pair{}
			pairs[e.Sequence] = p
			order = append(order, e.Sequence)
		}
		if e.Role == "probe" {
			p.probe = e
		} else {
			p.response = e
		}
	}

	for _, seq := range order {
		p := pairs[seq]
		decisionTag := ""
		if p.response != nil && p.response.DecisionPoint {
			decisionTag = " (decision)"
		}
		fmt.Fprintf(&b, "**[%d]%s**\n", seq, decisionTag)
		if p.probe != nil {
			fmt.Fprintf(&b, "> **Probe:** %s\n", p.probe.Content)
		}
		if p.response != nil {
			fmt.Fprintf(&b, "> **User:** %s\n", p.response.Content)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ConversationLogList renders multiple conversation logs in narrative order.
func ConversationLogList(logs []*specv1.ConversationLog) string {
	if len(logs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, log := range logs {
		b.WriteString(ConversationLog(log))
	}
	return b.String()
}
```

- [ ] **Step 2: Write unit test for renderer**

Create `internal/render/conversation_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestConversationLog_RendersPairs(t *testing.T) {
	log := &specv1.ConversationLog{
		Stage:   "shape",
		Version: 3,
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "What should be in scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "API and storage, not CLI", Stage: "shape", Sequence: 1, DecisionPoint: true},
		},
	}
	output := render.ConversationLog(log)
	assert.Contains(t, output, "Authoring Conversation (shape, v3)")
	assert.Contains(t, output, "(decision)")
	assert.Contains(t, output, "What should be in scope?")
	assert.Contains(t, output, "API and storage, not CLI")
}

func TestConversationLog_Nil(t *testing.T) {
	assert.Empty(t, render.ConversationLog(nil))
}

func TestConversationLogList_Empty(t *testing.T) {
	assert.Empty(t, render.ConversationLogList(nil))
}

func TestConversationLog_AmendLabel(t *testing.T) {
	log := &specv1.ConversationLog{
		Stage:   "shape",
		Version: 5,
		IsAmend: true,
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "Revised scope?", Stage: "shape", Sequence: 1},
		},
	}
	output := render.ConversationLog(log)
	assert.True(t, strings.Contains(output, "amend"))
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/render/ -run TestConversationLog -v`

Expected: PASS

- [ ] **Step 4: Commit**

```text
feat(render): add ConversationLog markdown renderer with tests (spgr-9mz)
```

### Task 11: Create CLI conversation commands

**Files:**

- Create: `cmd/specgraph/conversation.go`

- [ ] **Step 1: Create conversation.go CLI file**

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/spf13/cobra"
)

var conversationCmd = &cobra.Command{
	Use:   "conversation",
	Short: "Manage authoring conversation logs",
}

var conversationRecordCmd = &cobra.Command{
	Use:   "record <slug>",
	Short: "Record authoring conversation exchanges for a spec stage",
	Args:  cobra.ExactArgs(1),
	RunE:  runConversationRecord,
}

var conversationListCmd = &cobra.Command{
	Use:   "list <slug>",
	Short: "List authoring conversation logs for a spec",
	Args:  cobra.ExactArgs(1),
	RunE:  runConversationList,
}

var (
	convRecordStage    string
	convRecordJSONFile string
	convRecordIsAmend  bool
	convListStage      string
)

func init() {
	conversationRecordCmd.Flags().StringVar(&convRecordStage, "stage", "", "authoring stage (spark, shape, specify, decompose, approve)")
	conversationRecordCmd.Flags().StringVar(&convRecordJSONFile, "json-file", "", "path to JSON file containing conversation exchanges")
	conversationRecordCmd.Flags().BoolVar(&convRecordIsAmend, "amend", false, "mark as amend re-entry")
	_ = conversationRecordCmd.MarkFlagRequired("stage")
	_ = conversationRecordCmd.MarkFlagRequired("json-file")

	conversationListCmd.Flags().StringVar(&convListStage, "stage", "", "filter by authoring stage")

	conversationCmd.AddCommand(conversationRecordCmd)
	conversationCmd.AddCommand(conversationListCmd)
	rootCmd.AddCommand(conversationCmd)
}

// conversationRecordInput is the JSON structure for conversation record input.
type conversationRecordInput struct {
	Exchanges []struct {
		Role          string `json:"role"`
		Content       string `json:"content"`
		Stage         string `json:"stage"`
		Sequence      int32  `json:"sequence"`
		DecisionPoint bool   `json:"decision_point,omitempty"`
	} `json:"exchanges"`
}

func runConversationRecord(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}

	var input conversationRecordInput
	if loadErr := loadJSONFileRaw(convRecordJSONFile, &input); loadErr != nil {
		return fmt.Errorf("conversation record: %w", loadErr)
	}

	exchanges := make([]*specv1.ConversationExchange, len(input.Exchanges))
	for i, e := range input.Exchanges {
		exchanges[i] = &specv1.ConversationExchange{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}

	resp, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:      args[0],
		Stage:     convRecordStage,
		Exchanges: exchanges,
		IsAmend:   convRecordIsAmend,
	}))
	if err != nil {
		return fmt.Errorf("conversation record: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Msg)
	}
	fmt.Printf("Recorded conversation: %s (stage=%s, exchanges=%d)\n",
		resp.Msg.ConversationLog.Id,
		resp.Msg.ConversationLog.Stage,
		resp.Msg.ConversationLog.ExchangeCount)
	return nil
}

func runConversationList(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}

	resp, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  args[0],
		Stage: convListStage,
	}))
	if err != nil {
		return fmt.Errorf("conversation list: %w", err)
	}

	if jsonOutput {
		return printJSON(resp.Msg)
	}
	output := render.ConversationLogList(resp.Msg.ConversationLogs)
	if output == "" {
		fmt.Println("No conversation logs found.")
		return nil
	}
	fmt.Print(output)
	return nil
}
```

Note: `loadJSONFileRaw` is needed to load into a plain struct (not a proto message). Check if `cmd/specgraph/util.go` has a non-proto JSON loader. If not, add one:

```go
func loadJSONFileRaw(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./cmd/specgraph/...`

Expected: PASS

- [ ] **Step 3: Verify CLI help**

Run: `go run ./cmd/specgraph conversation --help`

Expected: Shows `record` and `list` subcommands.

- [ ] **Step 4: Commit**

```text
feat(cli): add conversation record and list commands (spgr-9mz)
```

---

## Chunk 6: Quality Gates & Skill Integration Notes

### Task 12: Run full quality gates

**Files:** None (verification only)

- [ ] **Step 1: Run task check**

Run: `task check`

Expected: PASS — fmt, lint, build, unit tests all green.

- [ ] **Step 2: Run task pr-prep**

Run: `task pr-prep`

Expected: PASS — includes integration and e2e tests.

- [ ] **Step 3: Fix any issues found**

Address lint warnings, missing license headers, formatting issues.

- [ ] **Step 4: Commit fixes if needed**

```text
fix: address lint and formatting issues (spgr-9mz)
```

### Task 13: Document skill integration (reference only, not code)

This task is a reference for the skill SKILL.md updates that will happen in a follow-up. The skill changes are:

Each authoring skill (spark, shape, specify, decompose, approve) gets an instruction block added after the stage completion section:

> After calling `specgraph <stage> <slug> --json-file ...`, record the authoring conversation:
>
> 1. For each probe you asked and the user's substantive response, create an exchange pair.
> 2. Mark exchanges where the user chose between alternatives as `decision_point: true`.
> 3. Skip meta-conversation ("does that make sense?", "yes, continue").
> 4. Write exchanges to JSON and call: `specgraph conversation record <slug> --stage <stage> --json-file /tmp/conv-<stage>.json`

The JSON file format:

```json
{
  "exchanges": [
    {"role": "probe", "content": "...", "stage": "<stage>", "sequence": 1},
    {"role": "response", "content": "...", "stage": "<stage>", "sequence": 1, "decision_point": true}
  ]
}
```

**This task is informational only — skill SKILL.md modifications should be done in a separate change to keep this PR focused on the backend infrastructure.**

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | Task 1 | Proto messages & codegen |
| 2 | Tasks 2-3 | Domain types & storage interface |
| 3 | Tasks 4-6b | Memgraph implementation, integration tests, edge exclusion, GetSpec augmentation |
| 4 | Tasks 7-9 | Server handlers, converters, handler tests |
| 5 | Tasks 10-11 | Render package with unit tests, CLI commands |
| 6 | Tasks 12-13 | Quality gates, skill integration docs |

**Total:** 14 tasks, ~35 bite-sized steps.
