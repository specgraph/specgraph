# Postgres Storage Backend Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Memgraph/Cypher storage backend with a pure Postgres/SQL implementation while keeping all 17 storage interfaces unchanged.

**Architecture:** Create `internal/storage/postgres/` as a drop-in replacement for `internal/storage/memgraph/`. Port file-by-file, translating Cypher queries to SQL (recursive CTEs for graph traversals). Use pgx v5 native driver, goose for migrations, testcontainers-go for integration testing.

**Tech Stack:** pgx v5 (`jackc/pgx/v5`), pgxpool, goose v3 (`pressly/goose/v3`), pgvector (`pgvector/pgvector`), testcontainers-go Postgres module.

**Spec:** `docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md`

**Bead:** spgr-khy (epic)

---

## File Structure

### New Files (in `internal/storage/postgres/`)

| File | Responsibility | Approx Lines |
|------|---------------|--------------|
| `postgres.go` | Store struct, New(), Close(), Scoped(), options, helpers (newID, now) | ~300 |
| `tx.go` | RunInTransaction, txKey, query/exec helpers, change event dispatch | ~120 |
| `migrations/001_initial_schema.sql` | All CREATE TABLE/INDEX/EXTENSION DDL | ~150 |
| `migrate.go` | Embedded migrations via goose + embed.FS | ~40 |
| `graph.go` | AddEdge, RemoveEdge, ListEdges, GetDependencies, recursive CTEs | ~350 |
| `project.go` | EnsureProject, GetProject, UpdateProject, ListProjects, WipeProjectData | ~150 |
| `decision.go` | CreateDecision, GetDecision, ListDecisions, UpdateDecision | ~350 |
| `changelog.go` | createChangeLog, ListChanges, ListAllChanges | ~200 |
| `lifecycle.go` | LifecycleAmendSpec, LifecycleSupersedeSpec, LifecycleAbandonSpec, LifecycleAcknowledgeDrift | ~350 |
| `authoring.go` | TransitionStage, Store*Output, StoreSafetyFlags, SupersedeSpec, AmendSpec | ~400 |
| `conversation.go` | RecordConversation, ListConversations, ListAllConversations | ~250 |
| `claim.go` | ClaimSpec, UnclaimSpec, Heartbeat | ~150 |
| `constitution.go` | GetConstitution, UpdateConstitution | ~180 |
| `execution.go` | GenerateBundle, RecordProgress/Blocker/Completion, GetExecutionEvents, GetPrimeData, ReleaseExpiredClaims | ~300 |
| `findings.go` | StoreFindings, ListFindings, ListAllFindings | ~180 |
| `slice.go` | CreateSlice, ListSlices, GetSlice, ClaimSlice, CompleteSlice | ~200 |
| `sync.go` | CreateSyncMapping, UpdateSyncState, GetSyncMapping, ListSyncMappings, DeleteSyncMapping | ~200 |

### New Test Files (in `internal/storage/postgres/`)

Mirror Memgraph test files with `//go:build integration` tag. Each implementation file gets a corresponding `*_test.go`.

### Modified Files

| File | Change |
|------|--------|
| `go.mod` | Add pgx v5, pgxpool, goose v3, pgvector-go |
| `internal/docker/compose.go` | Change `apache/age:latest` to `pgvector/pgvector:pg16` |
| `e2e/testutil/containers.go` | Add `StartPostgres()` function |
| `e2e/testutil/server.go` | Switch from `memgraph.New()` to `postgres.New()` |
| `cmd/specgraph/` | Add `--pg-url` / `SPECGRAPH_PG_URL` |
| `Taskfile.yml` | Update `test:integration` to include postgres package |

### Deleted Files (post-migration cleanup)

`internal/storage/memgraph/` (all files) after Postgres passes all integration + e2e tests.

---

## Chunk 1: Foundation (Tasks 1-4)

Schema migration, Store struct, transaction threading, project CRUD. This chunk produces
a Store that can connect, run migrations, manage projects, and execute transactions.

### Task 1: Schema Migration and Dependencies

**Files:**

- Create: `internal/storage/postgres/migrations/001_initial_schema.sql`
- Create: `internal/storage/postgres/migrate.go`
- Modify: `go.mod` (add dependencies)

- [ ] **Step 1: Add Go dependencies**

Run: `go get github.com/jackc/pgx/v5 github.com/pressly/goose/v3 github.com/pgvector/pgvector-go && go mod tidy`

- [ ] **Step 2: Create the initial schema migration**

Create `internal/storage/postgres/migrations/001_initial_schema.sql` with the full DDL
from the spec. Include `-- +goose Up` and `-- +goose Down` directives. The Up section
creates all 12 tables, indexes, and the vector extension. The Down section drops them
in reverse dependency order.

See `docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md` section
"Schema Design" for the complete DDL. Note: `projects` table has `sync_adapters TEXT[]`
and `github_repo TEXT` (not `name`), and `specs` table has `safety_flags JSONB`.

- [ ] **Step 3: Create the migration runner**

Create `internal/storage/postgres/migrate.go`:
- Package comment for `postgres`
- `//go:embed migrations/*.sql` for embedded FS
- `runMigrations(connString string) error` opens a `database/sql` connection via
  `pgx/v5/stdlib`, sets goose dialect to postgres, runs `goose.Up()`
- License header required

- [ ] **Step 4: Verify migration compiles**

Run: `go build ./internal/storage/postgres/...`
Expected: Build succeeds.

- [ ] **Step 5: Commit**

```
feat(storage): add Postgres schema migration and goose runner (spgr-khy)
```

---

### Task 2: Store Struct, Options, New, Close, Scoped

**Files:**

- Create: `internal/storage/postgres/postgres.go`
- Create: `internal/storage/postgres/postgres_test.go`

**Reference:** Read `internal/storage/memgraph/memgraph.go:42-170` for the Store struct,
options pattern, New constructor, Close, Scoped methods.

- [ ] **Step 1: Write test for New and Close**

Create `internal/storage/postgres/postgres_test.go` with `//go:build integration` tag.
Use testcontainers-go Postgres module with `pgvector/pgvector:pg16` image. Wait for
`"database system is ready to accept connections"` with `WithOccurrence(2)`.

Create `setupTestDB(t)` helper using `sync.Once` for shared container.
Create `newStore(t, opts...)` helper that creates a store with `WithProject("test")`.
Test: `New()` connects, runs migrations, returns store. `Close()` succeeds.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestNew -v -count=1`
Expected: FAIL (postgres.New does not exist).

- [ ] **Step 3: Implement Store struct and constructor**

Create `internal/storage/postgres/postgres.go`. Port from memgraph.go:

```go
type Store struct {
    pool       *pgxpool.Pool
    nowFunc    func() time.Time
    sliceOps   storage.SliceBackend
    project    string
    ownsPool   bool
    shared     *sharedState
}
```

Implement:
- `Option` type + `WithClock`, `WithProject`, `WithSliceOps`
- `New(ctx, connString, opts...)` -- creates pgxpool, runs migrations, ensures project row
- `Close(ctx)` -- closes pool if ownsPool
- `Scoped(ctx, project)` -- returns new Store sharing pool
- `Subscribe(sub)` -- appends to shared.subscribers
- `newID(prefix)` -- ULID generation (copy from memgraph.go)
- Compile-time interface assertions matching memgraph.go:26-34

Key difference: `ensureProject()` uses
`INSERT INTO projects (slug) VALUES ($1) ON CONFLICT DO NOTHING`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -tags integration ./internal/storage/postgres/ -run TestNew -v -count=1`
Expected: PASS

- [ ] **Step 5: Add test for Scoped**

Test: Scoped returns non-nil ScopedBackend for a different project.

- [ ] **Step 6: Run test, verify pass**

- [ ] **Step 7: Commit**

```
feat(storage): add Postgres Store struct with New/Close/Scoped (spgr-khy)
```

---

### Task 3: Transaction Threading and Query Helpers

**Files:**

- Create: `internal/storage/postgres/tx.go`
- Modify: `internal/storage/postgres/postgres_test.go` (add tx tests)

**Reference:** Read `internal/storage/memgraph/tx.go` (158 lines) for the complete
transaction pattern including txKey, context threading, executeQuery routing, and
change event dispatch.

- [ ] **Step 1: Write tests for RunInTransaction**

Test commit on success: wrap an INSERT in RunInTransaction, verify row exists after.
Test rollback on error: wrap an INSERT that returns error, verify row does not exist.
Test nested tx reuse: call RunInTransaction inside RunInTransaction, verify only one
commit happens.
Test change event dispatch: register a ChangeSubscriber, stash an event inside tx,
verify OnSpecChanged is called after commit.

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Implement tx.go**

Port from memgraph tx.go:
- `txKey struct{}` context key
- `txToContext(ctx, pgx.Tx)` / `txFromContext(ctx) (pgx.Tx, bool)`
- `RunInTransaction(ctx, fn)`: check for existing tx (nested = reuse); otherwise
  `pool.Begin()`, `storage.InitChangeEvents()`, run fn, commit on success / rollback
  on error, `dispatchChangeEvents()` after commit
- `query(ctx, sql, args...) (pgx.Rows, error)`: routes to tx or pool
- `exec(ctx, sql, args...) (pgconn.CommandTag, error)`: routes to tx or pool
- `queryRow(ctx, sql, args...) pgx.Row`: routes to tx or pool
- `dispatchChangeEvents(ctx)`: drains stashed events, notifies subscribers with panic
  recovery per subscriber

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Commit**

```
feat(storage): add Postgres transaction threading and query helpers (spgr-khy)
```

---

### Task 4: Project CRUD and WipeProjectData

**Files:**

- Create: `internal/storage/postgres/project.go`
- Modify: `internal/storage/postgres/postgres_test.go` (add project tests)

**Reference:** Read `internal/storage/memgraph/project.go` (168 lines) and
`internal/storage/memgraph/project_test.go` (85 lines).

- [ ] **Step 1: Write tests for all project methods**

Port from memgraph project_test.go:
- EnsureProject creates row, second call idempotent
- GetProject returns project
- UpdateProject sets fields
- ListProjects returns all
- WipeProjectData deletes all nodes/edges, preserves project row

- [ ] **Step 2: Run tests, verify they fail**

- [ ] **Step 3: Implement project.go**

- EnsureProject: `INSERT ... ON CONFLICT DO NOTHING` + fallback SELECT
- GetProject: `SELECT FROM projects WHERE slug = $1`
- UpdateProject: `UPDATE projects SET ... WHERE slug = $1`
- ListProjects: `SELECT FROM projects ORDER BY slug`
- WipeProjectData: DELETE cascade in FK-safe order:
  edges, sync_mappings, execution_events, claims, findings, conversation_logs,
  changelog_entries, constitutions, slices, decisions, specs.
  Project row preserved.

- [ ] **Step 4: Run tests, verify pass**

- [ ] **Step 5: Commit**

```
feat(storage): add Postgres project CRUD and WipeProjectData (spgr-khy)
```

---

## Chunk 2: Core Spec CRUD and Changelog (Tasks 5-7)

CreateSpec, GetSpec, ListSpecs, UpdateSpec, and the changelog system that underpins
every mutation. This chunk produces spec CRUD with version tracking and content hashing.

### Task 5: Changelog Infrastructure and CreateSpec/GetSpec

**Files:**

- Create: `internal/storage/postgres/changelog.go`
- Modify: `internal/storage/postgres/postgres.go` (add spec CRUD methods)
- Modify: `internal/storage/postgres/postgres_test.go` (add spec tests)

**Reference:** Read `internal/storage/memgraph/changelog.go` (364 lines) for
createChangeLog, marshalFieldChanges, unmarshalFieldChanges. Read
`internal/storage/memgraph/memgraph.go:200-350` for CreateSpec, GetSpec.

- [ ] **Step 1: Implement changelog.go** (internal helper, tested via spec CRUD)

- `createChangeLog(ctx, slug, entry, changes)`: INSERT into changelog_entries,
  INSERT HAS_CHANGE edge, stash change event
- `marshalFieldChanges` / `unmarshalFieldChanges`: JSON marshal for `[]FieldChange`
- `scanChangeLogEntry(row)`: scan helper for reading changelog rows

- [ ] **Step 2: Write test for CreateSpec**

Port from memgraph_test.go. Key: returns spec with version=1, stage=spark, generated
content_hash. Second create with same slug returns ErrSlugExists.

- [ ] **Step 3: Implement CreateSpec**

INSERT into specs + initial ChangeLog entry, all in RunInTransaction. Uses
`contenthash.Spec()` for hash computation (same as memgraph).

- [ ] **Step 4: Run test, verify pass**

- [ ] **Step 5: Write test for GetSpec**

Returns spec with all fields populated. ConversationLogs populated if any exist.

- [ ] **Step 6: Implement GetSpec**

SELECT spec + subquery for conversation_count. Loads conversation logs (stub initially,
filled in Task 12).

- [ ] **Step 7: Run test, verify pass**

- [ ] **Step 8: Commit**

```
feat(storage): add Postgres CreateSpec, GetSpec with changelog (spgr-khy)
```

---

### Task 6: ListSpecs, BatchGetSpecs, UpdateSpec

**Files:**

- Modify: `internal/storage/postgres/postgres.go`
- Modify: `internal/storage/postgres/postgres_test.go`

**Reference:** Read `internal/storage/memgraph/memgraph.go:350-550` for ListSpecs,
UpdateSpec, and the content hash recompute + conditional changelog pattern.

- [ ] **Step 1: Write tests for ListSpecs** (stage filter, priority filter, limit, conversation_count)

- [ ] **Step 2: Implement ListSpecs** with dynamic WHERE clauses and conversation_count subquery

- [ ] **Step 3: Run test, verify pass**

- [ ] **Step 4: Write tests for UpdateSpec** (partial update, version bump, hash recompute, changelog only if changed, ErrConcurrentModification on stale version)

- [ ] **Step 5: Implement UpdateSpec**

Dynamic SET clause construction for non-nil fields. Version guard:
`WHERE slug = $1 AND project_slug = $2 AND version = $3`. Check RowsAffected.
Recompute content hash. Create ChangeLog only if hash changed.

- [ ] **Step 6: Run test, verify pass**

- [ ] **Step 7: Write test and implement BatchGetSpecs** (`WHERE slug = ANY($1)`)

- [ ] **Step 8: Run test, verify pass**

- [ ] **Step 9: Commit**

```
feat(storage): add Postgres ListSpecs, BatchGetSpecs, UpdateSpec (spgr-khy)
```

---

### Task 7: ListChanges and ListAllChanges

**Files:**

- Modify: `internal/storage/postgres/changelog.go`
- Create: `internal/storage/postgres/changelog_test.go`

**Reference:** Read `internal/storage/memgraph/changelog_test.go` (496 lines, the
second-largest test file) for the full test suite including checkpoint filtering,
since_version, limit, and ErrSpecNotFound.

- [ ] **Step 1: Write tests** for all ListChanges filter combinations + ListAllChanges

- [ ] **Step 2: Implement ListChanges and ListAllChanges**

ListChanges: verify spec exists first, then SELECT with optional checkpoint/version/limit
filters. ListAllChanges: SELECT all with spec_slug populated via JOIN or direct column.

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres changelog ListChanges/ListAllChanges (spgr-khy)
```

---

## Chunk 3: Graph Operations (Task 8)

Edge CRUD and all recursive CTE graph traversals. Translates Cypher variable-length
path queries to SQL.

### Task 8: Graph Backend

**Files:**

- Create: `internal/storage/postgres/graph.go`
- Create: `internal/storage/postgres/graph_test.go`

**Reference:** Read `internal/storage/memgraph/graph.go` (376 lines) for every method
signature and the Cypher queries. Read the spec "Graph Query Patterns" section for the
SQL translations.

- [ ] **Step 1: Write tests for AddEdge, RemoveEdge, ListEdges**

AddEdge: creates row, duplicate is idempotent (ON CONFLICT DO NOTHING), DEPENDS_ON
stores content_hash_at_link. RemoveEdge: deletes. ListEdges: bidirectional, excludes
internal edge types.

- [ ] **Step 2: Implement AddEdge, RemoveEdge, ListEdges**

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Write tests for GetDependencies, GetDependenciesWithEdgeData, RefreshDependencyHashes**

- [ ] **Step 5: Implement these three methods**

GetDependencies: DEPENDS_ON targets UNION BLOCKS sources.
GetDependenciesWithEdgeData: includes content_hash_at_link + upstream content_hash.
RefreshDependencyHashes: UPDATE edges FROM (specs UNION decisions).

- [ ] **Step 6: Run tests, verify pass**

- [ ] **Step 7: Write tests for GetTransitiveDeps, GetImpact**

Create chain A->B->C->D. Verify transitive from A = {B,C,D}, impact on D = {A,B,C}.

- [ ] **Step 8: Implement using recursive CTEs with CYCLE clause**

See spec for exact SQL. Resolve node types via LEFT JOINs to specs/decisions/slices.

- [ ] **Step 9: Run tests, verify pass**

- [ ] **Step 10: Write tests for GetReady, GetCriticalPath, GetFullGraph**

- [ ] **Step 11: Implement GetReady** (NOT EXISTS anti-joins), **GetCriticalPath** (recursive
CTE with manual path array + unnest WITH ORDINALITY), **GetFullGraph** (nodes + edges query)

- [ ] **Step 12: Run tests, verify pass**

- [ ] **Step 13: Commit**

```
feat(storage): add Postgres graph operations with recursive CTEs (spgr-khy)
```

---

## Chunk 4: Decision, Lifecycle, Authoring (Tasks 9-11)

The most complex business logic. These files have heavy cross-dependencies.

### Task 9: Decision CRUD

**Files:**

- Create: `internal/storage/postgres/decision.go`
- Create: `internal/storage/postgres/decision_test.go`

**Reference:** Read `internal/storage/memgraph/decision.go` (567 lines) and
`internal/storage/memgraph/decision_test.go` (249 lines). Note the status normalization
(legacy proto enum names), tags as TEXT[], rejected_alternatives as JSONB, and
version-guarded UpdateDecision with conditional ChangeLog.

- [ ] **Step 1: Write tests** for CreateDecision (all ADR-003 fields), GetDecision,
ListDecisions (status filter), UpdateDecision (version guard, conditional changelog)

- [ ] **Step 2: Implement decision.go**

tags stored as TEXT[] (pgx native []string). rejected_alternatives as JSONB.
Status normalization handles DECISION_STATUS_PROPOSED -> proposed.
UpdateDecision: dynamic SET, version guard, content hash recompute, changelog if changed.

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres decision CRUD with version guards (spgr-khy)
```

---

### Task 10: Lifecycle Operations

**Files:**

- Create: `internal/storage/postgres/lifecycle.go`
- Create: `internal/storage/postgres/lifecycle_test.go`

**Reference:** Read `internal/storage/memgraph/lifecycle.go` (502 lines) and
`internal/storage/memgraph/lifecycle_test.go` (938 lines, the largest test file).

**Dependencies:** Requires Task 5 (changelog.go) and Task 8 (graph.go) for
`RefreshDependencyHashes` used by `LifecycleAcknowledgeDrift`.

- [ ] **Step 1: Write tests for LifecycleAmendSpec** (from done only, version guard,
checkpoint changelog, hash recompute)

- [ ] **Step 2: Implement LifecycleAmendSpec**

- [ ] **Step 3: Run test, verify pass**

- [ ] **Step 4: Write tests for LifecycleSupersedeSpec** (atomic on both specs, SUPERSEDES edge, both get checkpoint changelogs)

- [ ] **Step 5: Implement LifecycleSupersedeSpec**

- [ ] **Step 6: Run test, verify pass**

- [ ] **Step 7: Write tests for LifecycleAbandonSpec and LifecycleAcknowledgeDrift**

- [ ] **Step 8: Implement both** (AcknowledgeDrift calls RefreshDependencyHashes + non-checkpoint changelog)

- [ ] **Step 9: Run tests, verify pass**

- [ ] **Step 10: Commit**

```
feat(storage): add Postgres lifecycle operations (spgr-khy)
```

---

### Task 11: Authoring Operations

**Files:**

- Create: `internal/storage/postgres/authoring.go`
- Create: `internal/storage/postgres/authoring_test.go`

**Reference:** Read `internal/storage/memgraph/authoring.go` (531 lines) and
`internal/storage/memgraph/authoring_test.go` (682 lines). Note StoreShapeOutput
promotes decisions to graph nodes. StoreDecomposeOutput creates Slice nodes with
two-pass dependency resolution.

**Dependencies:** Requires Tasks 8 (graph - AddEdge), 9 (decision - CreateDecision,
GetDecision), and 17 (slice - CreateSlice) to be complete.

- [ ] **Step 1: Write tests for TransitionStage**

- [ ] **Step 2: Implement TransitionStage** (validate transition, update stage, recompute hash, checkpoint changelog)

- [ ] **Step 3: Run test, verify pass**

- [ ] **Step 4: Write tests for StoreSparkOutput, StoreShapeOutput, StoreSpecifyOutput**

StoreShapeOutput: promotes decisions (CreateDecision if not exists, DECIDED_IN edge).
StoreSafetyFlags: stores `[]SafetyFlag` as JSONB in `safety_flags` column.

- [ ] **Step 5: Implement Store*Output and StoreSafetyFlags** (JSONB columns, pgx auto-marshals structs)

- [ ] **Step 6: Run tests, verify pass**

- [ ] **Step 7: Write tests for StoreDecomposeOutput** (creates slices, COMPOSES/DEPENDS_ON edges)

- [ ] **Step 8: Implement StoreDecomposeOutput** (uses SliceBackend, two-pass for out-of-order deps)

- [ ] **Step 9: Run test, verify pass**

- [ ] **Step 10: Write tests for SupersedeSpec, AmendSpec (authoring-level)**

- [ ] **Step 11: Implement SupersedeSpec, AmendSpec**

- [ ] **Step 12: Run tests, verify pass**

- [ ] **Step 13: Commit**

```
feat(storage): add Postgres authoring operations (spgr-khy)
```

---

## Chunk 5: Satellite Entities (Tasks 12-17)

Conversation logs, claims, constitution, execution, findings, slices, sync. Mostly
straightforward CRUD. Tasks 12-17 can be parallelized (all depend only on Tasks 1-4).

### Task 12: Conversation Logs

**Files:**

- Create: `internal/storage/postgres/conversation.go`
- Create: `internal/storage/postgres/conversation_test.go`

**Reference:** Read `internal/storage/memgraph/conversation.go` (416 lines). Note the
edge chain: spec -AUTHORED_VIA-> first log -CONTINUES-> next log, plus EXPLAINS edges
to matching changelogs.

- [ ] **Step 1: Write tests** for RecordConversation, ListConversations, ListAllConversations

- [ ] **Step 2: Implement conversation.go**

RecordConversation: INSERT log + edges (AUTHORED_VIA or CONTINUES, optionally EXPLAINS).
ListConversations: ORDER BY date (chain order). Go back and wire into GetSpec.

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres conversation log operations (spgr-khy)
```

---

### Task 13: Claims and Leases

**Files:**

- Create: `internal/storage/postgres/claim.go`
- Create: `internal/storage/postgres/claim_test.go`

**Reference:** Read `internal/storage/memgraph/claim.go` (189 lines).

- [ ] **Step 1: Write tests** for ClaimSpec, UnclaimSpec, Heartbeat

- [ ] **Step 2: Implement claim.go** (delete expired first, then INSERT, CLAIMED_BY edge)

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres claim/lease operations (spgr-khy)
```

---

### Task 14: Constitution

**Files:**

- Create: `internal/storage/postgres/constitution.go`
- Create: `internal/storage/postgres/constitution_test.go`

**Reference:** Read `internal/storage/memgraph/constitution.go` (303 lines).

- [ ] **Step 1: Write tests** for GetConstitution, UpdateConstitution

- [ ] **Step 2: Implement** (JSONB data column, INSERT ON CONFLICT UPDATE with version bump)

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres constitution operations (spgr-khy)
```

---

### Task 15: Execution Events and Bundles

**Files:**

- Create: `internal/storage/postgres/execution.go`
- Create: `internal/storage/postgres/execution_test.go`

**Reference:** Read `internal/storage/memgraph/execution.go` (359 lines). Note
RecordCompletion is atomic (verify claim + event + transition + delete claim + refresh
hashes). GenerateBundle joins specs, decisions, edges, claims.

**Dependencies:** Requires Tasks 8 (graph), 9 (decision), 13 (claims).

- [ ] **Step 1: Write tests** for all execution methods

- [ ] **Step 2: Implement execution.go**

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres execution events and bundle generation (spgr-khy)
```

---

### Task 16: Findings

**Files:**

- Create: `internal/storage/postgres/findings.go`
- Create: `internal/storage/postgres/findings_test.go`

**Reference:** Read `internal/storage/memgraph/findings.go` (250 lines).

- [ ] **Step 1: Write tests** for StoreFindings, ListFindings, ListAllFindings

- [ ] **Step 2: Implement** (atomic delete + insert for StoreFindings, HAS_FINDING edges)

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Commit**

```
feat(storage): add Postgres findings operations (spgr-khy)
```

---

### Task 17: Slices and Sync Mappings

**Files:**

- Create: `internal/storage/postgres/slice.go`
- Create: `internal/storage/postgres/slice_test.go`
- Create: `internal/storage/postgres/sync.go`
- Create: `internal/storage/postgres/sync_test.go`

**Reference:** Read `internal/storage/memgraph/slice.go` (299 lines) and
`internal/storage/memgraph/sync.go` (271 lines).

- [ ] **Step 1: Write tests** for slice CRUD (arrays as TEXT[], status transitions)

- [ ] **Step 2: Implement slice.go** (INSERT + BELONGS_TO + COMPOSES edges, pgx native []string)

- [ ] **Step 3: Run tests, verify pass**

- [ ] **Step 4: Write tests** for sync mapping CRUD

- [ ] **Step 5: Implement sync.go** (flat table, no ExternalRef nodes like Memgraph)

- [ ] **Step 6: Run tests, verify pass**

- [ ] **Step 7: Commit**

```
feat(storage): add Postgres slice and sync mapping operations (spgr-khy)
```

---

## Chunk 6: Wiring and E2E (Tasks 18-20)

Connect Postgres to the rest of the system, add schema validation tests, run the full
pipeline, and remove Memgraph code.

### Task 18: Wire Postgres Backend into Server and CLI

**Files:**

- Modify: `e2e/testutil/containers.go` (add StartPostgres)
- Modify: `e2e/testutil/server.go` (switch to postgres.Store)
- Modify: `internal/docker/compose.go` (update image)
- Modify: `cmd/specgraph/` (add --pg-url flag)

- [ ] **Step 1: Add StartPostgres to e2e containers**

Use `pgvector/pgvector:pg16` image with testcontainers Postgres module. Wait for
`"database system is ready to accept connections"` with occurrence 2.

- [ ] **Step 2: Update e2e server to use postgres.Store**

ServerInfo.Store type becomes `storage.ScopedBackend` (interface) instead of
`*memgraph.Store`. All service registrations use the interface.

- [ ] **Step 3: Update docker compose template**

Change `apache/age:latest` to `pgvector/pgvector:pg16`.

- [ ] **Step 4: Update CLI** to accept --pg-url and create postgres.Store

- [ ] **Step 5: Verify build** with `go build ./...`

- [ ] **Step 6: Commit**

```
feat(storage): wire Postgres backend into server, CLI, and e2e setup (spgr-khy)
```

---

### Task 19: E2E Schema Validation Tests

**Files:**

- Create: `e2e/api/schema_validation_test.go`

- [ ] **Step 1: Write structural schema validation tests**

Query information_schema and pg_indexes to verify all 12 tables, column types,
NOT NULL constraints, FK constraints, indexes, and pgvector extension.

- [ ] **Step 2: Write behavioral validation tests**

After API-driven scenarios, query DB directly to verify: internal edges created,
versions incremented, changelog entries correct, WipeData cleans up.

- [ ] **Step 3: Run e2e tests**

Run: `go test -tags e2e ./e2e/api/ -v`
Expected: All e2e tests pass including new schema validation.

- [ ] **Step 4: Commit**

```
test(e2e): add Postgres schema and behavioral validation tests (spgr-khy)
```

---

### Task 20: Full Pipeline Verification and Memgraph Cleanup

**Files:**

- Delete: `internal/storage/memgraph/` (entire directory)
- Modify: `go.mod` (remove neo4j-go-driver/v6)
- Modify: `CLAUDE.md` (update gotchas)
- Modify: `Taskfile.yml` (update test targets)

- [ ] **Step 1: Run full quality gate** (`task check`)

- [ ] **Step 2: Run integration tests** (`task test:integration`)

- [ ] **Step 3: Run e2e tests** (`task pr-prep` or `task test:e2e`)

- [ ] **Step 4: Delete** `internal/storage/memgraph/`

- [ ] **Step 5: Remove neo4j dependency** (`go mod tidy`)

- [ ] **Step 6: Update CLAUDE.md**

Remove: bolt readiness race, Cypher DELETE + count, Cypher references.
Add: pgx v5 conventions, recursive CTE patterns, Postgres gotchas.
Update Architecture table: memgraph -> postgres.

- [ ] **Step 7: Verify clean build** (`go build ./...`)

- [ ] **Step 8: Run full quality gate** (`task check`)

- [ ] **Step 9: Commit**

```
chore(storage): remove Memgraph backend and neo4j dependency (spgr-khy)
```

---

## Task Dependency Graph

```
Task 1 (schema + deps)
  |
  v
Task 2 (Store struct)
  |
  v
Task 3 (tx.go)
  |
  v
Task 4 (project CRUD)
  |
  +---> Task 5 (CreateSpec + changelog) --> Task 6 (List/Update) --> Task 7 (ListChanges)
  |
  +---> Task 8 (graph operations)
  |
  +---> Task 9 (decision CRUD) --> Task 10 (lifecycle) [needs 8 for RefreshDependencyHashes]
  |                                    |
  +---> Task 12 (conversation)         +--> Task 11 (authoring) [needs 8, 9, 17-slice]
  |
  +---> Task 13 (claims)
  |
  +---> Task 14 (constitution)
  |
  +---> Task 15 (execution) [needs 8, 9, 13]
  |
  +---> Task 16 (findings)
  |
  +---> Task 17 (slices + sync)
  |
  All above complete
  |
  v
Task 18 (wiring) --> Task 19 (e2e validation) --> Task 20 (cleanup)
```

Tasks 5-17 can be parallelized after Task 4 completes, with these constraints:
- Task 11 (authoring) depends on Tasks 8, 9, and 17 (slice portion)
- Task 15 (execution) depends on Tasks 8, 9, and 13
