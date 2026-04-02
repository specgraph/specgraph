# Postgres Storage Backend Design

**Bead:** spgr-khy (epic), spgr-khy.1 (planning task)
**Date:** 2026-04-01
**Status:** Approved
**Supersedes:** ADR-001 assumption of Postgres+AGE; closes spgr-8nf (backend evaluation)

## Summary

Replace the Memgraph/Cypher storage backend (`internal/storage/memgraph/`) with a pure
Postgres/SQL implementation (`internal/storage/postgres/`). The storage interface layer
(`internal/storage/*.go` -- 17 interfaces, domain types) stays unchanged. The Memgraph
package is ~11,300 LOC (including tests) across 16 implementation files. All consumers
(server handlers, CLI, drift engine, authoring engine) are unaffected.

## Motivation

- **Operational simplicity** -- one database to run, back up, and monitor instead of two
  (Memgraph + future Postgres for other features).
- **Stronger constraints** -- real FK constraints, CHECK constraints, partial unique
  indexes. Current Memgraph DDL is 4 lines of CREATE INDEX.
- **Mature ecosystem** -- pgdump, pg_restore, pgx driver, testcontainers, goose
  migrations. No "bolt readiness race" gotcha.
- **Better concurrency** -- MVCC with explicit version guards instead of Memgraph's
  opaque "Cannot resolve conflicting transactions" error mapping.
- **pgvector readiness** -- embedding columns for future semantic search over specs and
  decisions.
- **Graph queries are viable in SQL** -- analysis showed ~80% of operations are trivially
  relational, ~15% translate to recursive CTEs, and ~5% (critical path) need careful but
  well-documented SQL. At SpecGraph's scale (hundreds to low thousands of nodes per
  project), recursive CTEs execute in single-digit milliseconds.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Project scoping | WHERE clause with `project_slug` | Mirrors current pattern, avoids RLS complexity |
| Node tables | Separate typed tables (specs, decisions, slices) | 17/11/11 well-defined fields; real column constraints |
| Change events | In-process Subscribable pattern | All subscribers are in-process; LISTEN/NOTIFY adds complexity for no current benefit |
| Concurrency control | Optimistic versioning (version guard in WHERE) | Low-contention workload; retry semantics already wired into ConnectRPC layer |
| Migration strategy | Big bang -- build, test, swap | No current users/deployment; interface layer guarantees behavioral equivalence |
| Internal edges | All edge types in edges table | Preserves future graph traversal flexibility |
| Driver | pgx v5 native (not database/sql) | Performance, JSONB auto-marshal, CollectRows, batch queries, COPY |
| Migrations | goose v3 with embedded SQL | Native pgx integration, Go migration functions, embed.FS |
| Graph extensions | None (no AGE, no ltree) | AGE: immature Go driver, index bugs. ltree: trees only, not DAGs. Pure SQL is sufficient at this scale |
| Vector support | pgvector with vector(3072) columns | Future semantic search; nullable columns are zero-cost until populated |

## Schema Design

### Extensions

```sql
CREATE EXTENSION IF NOT EXISTS vector;
```

### Node Tables

```sql
CREATE TABLE projects (
    slug           TEXT PRIMARY KEY,
    sync_adapters  TEXT[] NOT NULL DEFAULT '{}',
    github_repo    TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE specs (
    slug             TEXT NOT NULL,
    project_slug     TEXT NOT NULL REFERENCES projects(slug),
    intent           TEXT NOT NULL DEFAULT '',
    stage            TEXT NOT NULL DEFAULT 'spark',
    priority         TEXT NOT NULL DEFAULT '',
    complexity       TEXT NOT NULL DEFAULT '',
    lifecycle        TEXT NOT NULL DEFAULT 'task',  -- 'task' or 'living'
    notes            TEXT NOT NULL DEFAULT '',
    content_hash     TEXT NOT NULL DEFAULT '',
    superseded_by    TEXT NOT NULL DEFAULT '',
    supersedes       TEXT NOT NULL DEFAULT '',
    version          INTEGER NOT NULL DEFAULT 1,
    spark_output     JSONB,
    shape_output     JSONB,
    specify_output   JSONB,
    decompose_output JSONB,
    safety_flags     JSONB,
    embedding        vector(3072),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug)
);

CREATE TABLE decisions (
    slug                  TEXT NOT NULL,
    project_slug          TEXT NOT NULL REFERENCES projects(slug),
    title                 TEXT NOT NULL DEFAULT '',
    status                TEXT NOT NULL DEFAULT 'proposed',
    body                  TEXT NOT NULL DEFAULT '',
    rationale             TEXT NOT NULL DEFAULT '',
    question              TEXT NOT NULL DEFAULT '',
    superseded_by         TEXT NOT NULL DEFAULT '',
    confidence            TEXT NOT NULL DEFAULT '',
    scope                 TEXT NOT NULL DEFAULT '',
    origin_spec           TEXT NOT NULL DEFAULT '',
    origin_stage          TEXT NOT NULL DEFAULT '',
    tags                  TEXT[] NOT NULL DEFAULT '{}',
    rejected_alternatives JSONB,
    content_hash          TEXT NOT NULL DEFAULT '',
    version               INTEGER NOT NULL DEFAULT 1,
    embedding             vector(3072),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug)
);

CREATE TABLE slices (
    slug           TEXT NOT NULL,
    project_slug   TEXT NOT NULL REFERENCES projects(slug),
    parent_slug    TEXT NOT NULL,       -- parent spec slug
    slice_id       TEXT NOT NULL,       -- local ID within decomposition
    intent         TEXT NOT NULL DEFAULT '',
    status         TEXT NOT NULL DEFAULT 'open',  -- open, claimed, done
    assigned_to    TEXT NOT NULL DEFAULT '',
    verify         TEXT[] NOT NULL DEFAULT '{}',
    touches        TEXT[] NOT NULL DEFAULT '{}',
    depends_on     TEXT[] NOT NULL DEFAULT '{}',   -- full sibling slice slugs
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, slug),
    FOREIGN KEY (project_slug, parent_slug) REFERENCES specs(project_slug, slug)
);
```

### Edges Table

All relationship types (user-facing and internal) live here:

```sql
CREATE TABLE edges (
    from_slug            TEXT NOT NULL,
    to_slug              TEXT NOT NULL,
    edge_type            TEXT NOT NULL,
    project_slug         TEXT NOT NULL REFERENCES projects(slug),
    content_hash_at_link TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, from_slug, to_slug, edge_type)
);

CREATE INDEX idx_edges_forward ON edges (project_slug, from_slug, edge_type) INCLUDE (to_slug);
CREATE INDEX idx_edges_reverse ON edges (project_slug, to_slug, edge_type) INCLUDE (from_slug);
```

`content_hash_at_link` is a dedicated column (not JSONB) because it is the only edge
property and is used in the drift detection hot path.

### Satellite Tables

Each satellite table has `spec_slug` + `project_slug` columns. Internal edges
(HAS_CHANGE, HAS_FINDING, etc.) also live in the `edges` table for graph completeness.

The Memgraph `Spec.ID` and `Decision.ID` fields (internal graph node IDs) are dropped --
the composite PK `(project_slug, slug)` serves as the identifier in Postgres.

The `edges` table intentionally has no FK constraints to node tables because edges can
reference any node type (Spec, Decision, Slice). Referential integrity for edges is
enforced by the application layer, same as in Memgraph.

```sql
CREATE TABLE changelog_entries (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    version      INTEGER NOT NULL,
    stage        TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL DEFAULT '',
    checkpoint   BOOLEAN NOT NULL DEFAULT false,
    summary      TEXT NOT NULL DEFAULT '',
    reason       TEXT NOT NULL DEFAULT '',
    changes      JSONB NOT NULL DEFAULT '[]',  -- []FieldChange
    date         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_changelog_spec ON changelog_entries (project_slug, spec_slug, version);

CREATE TABLE findings (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    pass_type    TEXT NOT NULL,
    severity     TEXT NOT NULL DEFAULT '',
    summary      TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    constraint_  TEXT NOT NULL DEFAULT '',  -- 'constraint' is reserved in SQL
    resolution   TEXT NOT NULL DEFAULT '',
    version      INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_findings_spec ON findings (project_slug, spec_slug, pass_type);

CREATE TABLE conversation_logs (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    stage        TEXT NOT NULL DEFAULT '',
    version      INTEGER NOT NULL DEFAULT 0,
    is_amend        BOOLEAN NOT NULL DEFAULT false,
    exchanges       JSONB NOT NULL DEFAULT '[]',  -- []ConversationExchange
    exchange_count  INTEGER NOT NULL DEFAULT 0,
    date            TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_conversations_spec ON conversation_logs (project_slug, spec_slug);

CREATE TABLE claims (
    spec_slug      TEXT NOT NULL,
    project_slug   TEXT NOT NULL REFERENCES projects(slug),
    agent          TEXT NOT NULL,
    lease_expires  TIMESTAMPTZ NOT NULL,
    claimed_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, spec_slug)
);
-- One active claim per spec enforced by PK (project_slug, spec_slug).
-- Expired claims are deleted by the application before inserting new ones.
-- A partial index on lease_expires is not possible because now() is volatile.

CREATE TABLE execution_events (
    id           TEXT NOT NULL,
    spec_slug    TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    agent        TEXT NOT NULL DEFAULT '',
    event_type   TEXT NOT NULL,  -- 'progress', 'blocker', 'completion'
    message      TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);
CREATE INDEX idx_exec_events_spec ON execution_events (project_slug, spec_slug, created_at DESC);

CREATE TABLE constitutions (
    id           TEXT NOT NULL,
    project_slug TEXT NOT NULL REFERENCES projects(slug),
    layer        TEXT NOT NULL DEFAULT '',
    name         TEXT NOT NULL DEFAULT '',
    version      INTEGER NOT NULL DEFAULT 1,
    data         JSONB NOT NULL DEFAULT '{}',  -- full Constitution struct (TechStack, Principles, etc.)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, id)
);

CREATE TABLE sync_mappings (
    spec_slug     TEXT NOT NULL,
    project_slug  TEXT NOT NULL REFERENCES projects(slug),
    adapter       TEXT NOT NULL,       -- 'beads', 'github'
    external_id   TEXT NOT NULL DEFAULT '',
    state         TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    last_sync     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_slug, spec_slug, adapter)
);
```

## Graph Query Patterns

### Transitive Dependencies (GetTransitiveDeps)

Postgres 14+ `CYCLE` clause for built-in cycle detection:

```sql
WITH RECURSIVE transitive AS (
    SELECT e.to_slug, 1 AS depth
    FROM edges e
    WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    UNION ALL
    SELECT e.to_slug, t.depth + 1
    FROM transitive t
    JOIN edges e ON e.from_slug = t.to_slug
                AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    WHERE t.depth < 50
) CYCLE to_slug SET is_cycle USING path
SELECT DISTINCT to_slug FROM transitive WHERE NOT is_cycle;
```

Node type resolution (Spec vs Decision vs Slice) via LEFT JOINs on the outer query.

### Reverse Impact (GetImpact)

Mirror of transitive deps, following edges backward:

```sql
WITH RECURSIVE impact AS (
    SELECT e.from_slug AS slug, 1 AS depth
    FROM edges e
    WHERE e.to_slug = $1 AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    UNION ALL
    SELECT e.from_slug, i.depth + 1
    FROM impact i
    JOIN edges e ON e.to_slug = i.slug
                AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    WHERE i.depth < 50
) CYCLE slug SET is_cycle USING path
SELECT DISTINCT slug FROM impact WHERE NOT is_cycle;
```

### Critical Path (GetCriticalPath)

Uses manual path array (not `CYCLE`) because the actual path is needed for `unnest`:

```sql
WITH RECURSIVE chains AS (
    SELECT $1::text AS current_slug, ARRAY[$1::text] AS path, 0 AS depth
    UNION ALL
    SELECT e.to_slug, c.path || e.to_slug, c.depth + 1
    FROM chains c
    JOIN edges e ON e.from_slug = c.current_slug
                AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    WHERE c.depth < 50 AND NOT e.to_slug = ANY(c.path)
),
leaf_paths AS (
    SELECT path FROM chains c
    WHERE NOT EXISTS (
        SELECT 1 FROM edges e
        WHERE e.from_slug = c.current_slug
          AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $2
    )
    ORDER BY array_length(path, 1) DESC LIMIT 1
)
SELECT node_slug, ordinality
FROM leaf_paths, unnest(path) WITH ORDINALITY AS t(node_slug, ordinality)
ORDER BY ordinality;
```

### Ready Specs (GetReady)

Cleaner in SQL than the Cypher equivalent -- uses `NOT EXISTS` anti-join patterns:

```sql
SELECT s.slug, 'Spec' AS label, s.stage
FROM specs s
WHERE s.project_slug = $1 AND s.stage <> 'done'
  AND NOT EXISTS (
      SELECT 1 FROM edges e JOIN specs dep ON dep.slug = e.to_slug AND dep.project_slug = $1
      WHERE e.from_slug = s.slug AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $1
        AND dep.stage <> 'done'
  )
  AND NOT EXISTS (
      SELECT 1 FROM edges e JOIN specs blocker ON blocker.slug = e.from_slug AND blocker.project_slug = $1
      WHERE e.to_slug = s.slug AND e.edge_type = 'BLOCKS' AND e.project_slug = $1
        AND blocker.stage <> 'done'
  );
```

### Edge Property Refresh (RefreshDependencyHashes)

```sql
UPDATE edges e
SET content_hash_at_link = COALESCE(upstream.content_hash, '')
FROM (
    SELECT slug, content_hash FROM specs WHERE project_slug = $2
    UNION ALL
    SELECT slug, content_hash FROM decisions WHERE project_slug = $2
) upstream
WHERE e.from_slug = $1 AND e.edge_type = 'DEPENDS_ON'
  AND e.project_slug = $2 AND upstream.slug = e.to_slug;
```

## Store Structure

### Go Struct

```go
type Store struct {
    pool       *pgxpool.Pool
    nowFunc    func() time.Time
    sliceOps   storage.SliceBackend
    project    string
    ownsPool   bool          // true for root Store, false for Scoped()
    shared     *sharedState  // subscribers, shared between root+scoped
}
```

`pgxpool.Pool` replaces `neo4j.Driver`. `Scoped()` returns a new `Store` with the
project slug set, sharing the same pool. Compile-time interface assertions mirror the
Memgraph package.

### Transaction Threading

Same context-key pattern as Memgraph. pgx's `tx.Begin()` on an existing transaction
auto-creates a savepoint, so nested `RunInTransaction` calls work without manual
savepoint management.

```go
func (s *Store) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
    if tx := txFromContext(ctx); tx != nil {
        return fn(ctx)  // reuse existing tx
    }
    // Begin, stash change events, run fn, commit, drain events, notify subscribers
}
```

### Query Execution Helper

```go
func (s *Store) query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
    if tx := txFromContext(ctx); tx != nil {
        return tx.Query(ctx, sql, args...)
    }
    return s.pool.Query(ctx, sql, args...)
}
```

### Concurrent Modification Detection

```go
result, err := tx.Exec(ctx,
    "UPDATE specs SET ..., version = version + 1 WHERE slug = $1 AND project_slug = $2 AND version = $3",
    slug, project, expectedVersion)
if result.RowsAffected() == 0 {
    return storage.ErrConcurrentModification
}
```

## Testing Strategy

- **testcontainers-go Postgres module** replaces Memgraph testcontainer. Wait strategy:
  `wait.ForLog("database system is ready").WithOccurrence(2)`.
- **Shared container per package** via `TestMain` + `sync.Once`. Schema applied via
  `WithInitScripts("testdata/schema.sql")`.
- **Transaction-per-test rollback** for fast isolation where applicable.
- **Unit tests** (`*_unit_test.go`) that don't hit the DB stay unchanged.
- **Integration tests** keep `//go:build integration` tag.
- Port Memgraph test logic file-by-file -- same assertions, different backend.

### E2E Schema Validation

E2E tests currently validate behavior via the ConnectRPC API surface. The Postgres
migration adds a **schema validation test** that runs as part of `e2e/api/` to verify the
database structure matches expectations after migrations run. This catches:

- Missing tables, columns, or indexes
- Wrong column types or constraints
- Orphan edges after node deletion (WipeData correctness)
- FK constraint enforcement (e.g., creating an edge for a nonexistent project fails)

Implementation approach: after the e2e test container is up and migrations have run,
query `information_schema.tables`, `information_schema.columns`, and `pg_indexes` to
assert:

- All expected tables exist with correct column names and types
- All expected indexes exist (including covering indexes and partial unique indexes)
- FK constraints are present on node tables (`specs.project_slug`, `slices.parent_slug`)
- The `vector` extension is installed
- `NOT NULL` constraints are enforced (attempt INSERT with NULL required field, expect failure)
- Primary key on `claims` (`project_slug`, `spec_slug`) prevents duplicate active claims

Additionally, after API-driven test scenarios (create spec, add edge, delete spec),
query the database directly to verify:

- Edge rows are cleaned up when `WipeData` is called
- Internal edges (HAS_CHANGE, HAS_FINDING) are created alongside the API-visible mutations
- Version columns are incremented correctly after updates
- Changelog entries exist with correct field deltas after mutations

## Migration Tooling

goose v3 with `embed.FS`:

```go
//go:embed migrations/*.sql
var migrations embed.FS
```

- Initial migration creates all tables, indexes, extensions.
- `Store.New()` runs `goose.Up()` on startup.
- goose uses `pgx/v5/stdlib` shim for the migration runner; all application queries use
  pgx native.

## Wiring Changes

- `cmd/specgraph/` config adds `--pg-url` flag / `SPECGRAPH_PG_URL` env var.
- Server startup creates `postgres.New(ctx, pgURL, opts...)`.
- `internal/docker/compose.go` gains a Postgres compose template.
- E2E tests switch to Postgres container.

## What Stays Unchanged

- All 17 storage interfaces in `internal/storage/`
- All domain types (Spec, Decision, Edge, etc.)
- All server handlers in `internal/server/`
- All CLI commands in `cmd/specgraph/`
- All proto definitions in `proto/specgraph/v1/`
- Drift engine, authoring engine, render package, export engine

## Implementation Approach

Port file-by-file from `internal/storage/memgraph/` to `internal/storage/postgres/`:

| Memgraph File | Postgres Equivalent | Notes |
|---------------|-------------------|-------|
| `memgraph.go` | `postgres.go` | Store struct, New(), Close(), Scoped(), options |
| `ddl.go` | `migrations/001_initial_schema.sql` | goose migration replaces runtime DDL |
| `tx.go` | `tx.go` | Context-threaded tx, query helper, event dispatch |
| `graph.go` | `graph.go` | Recursive CTEs replace Cypher traversals |
| `lifecycle.go` | `lifecycle.go` | Stage transitions, version guards |
| `authoring.go` | `authoring.go` | JSONB columns for stage outputs |
| `decision.go` | `decision.go` | CRUD + version guards |
| `changelog.go` | `changelog.go` | Insert-only audit table |
| `conversation.go` | `conversation.go` | Insert-only audit table |
| `claim.go` | `claim.go` | Partial unique index for active claims |
| `constitution.go` | `constitution.go` | JSONB for constitution fields |
| `execution.go` | `execution.go` | Bundle assembly + event recording |
| `findings.go` | `findings.go` | Analytical pass results |
| `slice.go` | `slice.go` | Decomposition unit CRUD |
| `sync.go` | `sync.go` | External system mapping |
| `project.go` | `project.go` | Project CRUD + WipeData |

Each file gets its integration test ported simultaneously.

## Post-Migration Cleanup

After Postgres backend passes all integration + e2e tests:

- Delete `internal/storage/memgraph/`
- Remove `neo4j-go-driver/v6` from `go.mod`
- Update ADR-001 to reflect Postgres as the storage backend
- Update CLAUDE.md (remove bolt readiness race, Cypher references, add Postgres gotchas)
- Update `internal/docker/compose.go` (remove Memgraph template)
