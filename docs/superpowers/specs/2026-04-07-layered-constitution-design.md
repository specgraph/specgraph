# Layered Constitution — Design Spec

**Date:** 2026-04-07
**Status:** Draft
**Goal:** Implement multi-layer constitution support with strategic merge, `$delete` directives, provenance tracking, and full test coverage across all layers.

## Context

The constitution is SpecGraph's ground truth for project standards. The proto schema defines four layers (User → Org → Project → Domain, more specific overrides general), but the current implementation stores a single constitution per project with no merging. This spec adds real multi-layer support.

## Layer Model

Four layers in precedence order (lowest to highest):

1. **User** — personal preferences and defaults
2. **Org** — organization-wide standards and constraints
3. **Project** — project-specific overrides and additions
4. **Domain** — domain-specific rules (highest precedence)

Higher layers override lower layers. All layers are optional — a project with only an org layer gets that org constitution as-is.

---

## Section 1: Storage

### Schema Migration

Change the `constitutions` table uniqueness constraint from `PRIMARY KEY (project_slug, id)` to `UNIQUE (project_slug, layer)`, keeping `id` as primary key. This allows up to 4 rows per project.

Add columns:
- `source_url` (TEXT, default '') — where this layer was imported from. Empty for local imports, URL for future remote sources.
- `source_hash` (TEXT, default '') — content hash of the source at import time. For future drift detection between cached and remote versions.

Each row stores the raw layer data as JSONB, including any `$delete` directives. The merge engine processes these on read.

### Read Path

`GetConstitution` with no layer filter:
1. Fetch all rows for the project ordered by layer precedence
2. Run the merge engine
3. Return merged result with provenance entries

`GetConstitution` with a layer filter:
1. Fetch the single row for `(project_slug, layer)`
2. Return raw data (including `$delete` markers)
3. No provenance (single layer, provenance is self-evident)

### Write Path

`UpdateConstitution` upserts by `(project_slug, layer)`. Each layer is independently versioned. Updating one layer does not touch others.

---

## Section 2: Merge Engine

New package: `internal/constitution/merge`

A pure package with no storage or proto dependencies. Takes a slice of constitution domain structs ordered by precedence (lowest to highest) and returns the merged result plus provenance.

### Merge Rules

- **Scalar fields** (e.g., `deployment_strategy`, `security_review`): highest layer wins, replacing lower.
- **String lists** (e.g., `languages`, `frameworks`, `datastores`, `api_standards`): union across layers, deduped.
- **Keyed object lists**: merge by key. If both org and project define the same key, project's version wins. New keys are added.

| List | Merge Key |
|------|-----------|
| principles | `id` |
| antipatterns | `pattern` |
| references | `path` |

### `$delete` Directive

Any list item with `$delete: true` removes that item from the merged result. Processed after the layer's additions are merged. Example:

```yaml
# project-layer.yaml — removes Java from org's language list
tech_config:
  languages:
    - go
    - python
principles:
  - id: "p3"
    $delete: true
```

Semantics:
- `$delete` on a key that doesn't exist in lower layers is a no-op (not an error)
- `$delete` consumes the item — the merged result never contains `$delete` entries
- If an item is added at org, overridden at project, then `$delete`'d at domain — deleted wins

### Provenance

The merge engine produces a provenance map alongside the merged result:

```go
type MergeResult struct {
    Constitution *Constitution
    Provenance   map[string]ConstitutionLayer // field path → source layer
}
```

Field paths use dot notation for scalars and bracket notation for list items:
- `"tech_config.languages[go]"` → project
- `"principles[p1]"` → org
- `"process.deployment_strategy"` → project

### Input/Output Types

The merge engine works on domain types (`storage.Constitution`), not proto types. The `$delete` directive is represented as a boolean field on the relevant structs (e.g., `Principle.Delete bool`).

---

## Section 3: Proto & API Changes

### GetConstitutionRequest

Add optional layer filter:

```protobuf
message GetConstitutionRequest {
  ConstitutionLayer layer = 1; // 0 (UNSPECIFIED) = return merged result
}
```

### GetConstitutionResponse

Add provenance entries:

```protobuf
message ProvenanceEntry {
  string path = 1;
  ConstitutionLayer layer = 2;
}

message GetConstitutionResponse {
  Constitution constitution = 1;
  repeated ProvenanceEntry provenance = 2; // only populated when returning merged
}
```

### UpdateConstitutionRequest

No structural change. The handler now upserts by `(project_slug, layer)` using the layer field in the `Constitution` message.

### Constitution message

Add source tracking fields:

```protobuf
message Constitution {
  // ... existing fields ...
  string source_url = N;
  string source_hash = N+1;
}
```

### `$delete` in proto

Not represented in proto messages. `$delete` is a YAML/JSON authoring convention stored in JSONB and processed by the merge engine. Individual layer queries return raw data including `$delete` markers. Merged results never contain them.

---

## Section 4: CLI Changes

### `constitution import`

Add `--layer` flag with default `project`:

```bash
# Default layer is project
specgraph constitution import my-constitution.yaml

# Explicit layer
specgraph constitution import org-standards.yaml --layer org
```

Layer resolution order: explicit `--layer` flag > `layer:` field in YAML > default `project`.

### `constitution show`

Add optional `--layer` flag:

```bash
# Show merged constitution (default)
specgraph constitution show

# Show raw org layer
specgraph constitution show --layer org
```

### `constitution emit`

Always operates on the merged result. No `--layer` flag. Tool files (CLAUDE.md, .cursorrules) reflect the effective constitution.

---

## Section 5: Web Dashboard

**Merged view (default):** The constitution page shows the merged constitution. Same layout as today, now reflecting all layers combined.

**Provenance badges:** Fields and list items display a small badge indicating which layer they came from, using the provenance entries from the `GetConstitution` response.

**No layer editing.** Constitution mutations stay in CLI (read-only dashboard).

---

## Section 6: Testing

### Unit Tests — Merge Engine (`internal/constitution/merge/`)

**Positive:**
- Two layers: scalar override (project replaces org)
- Two layers: string list union (project adds to org's languages)
- Two layers: keyed object merge (project overrides principle by id)
- Four layers in full precedence order — domain wins over all
- `$delete` removes item from lower layer
- Provenance map tracks correct source layer for each field path
- Single layer returns itself with provenance pointing to that layer

**Negative:**
- `$delete` on key that doesn't exist in lower layers — no-op, no error
- `$delete` with no merge key (missing `id` on principle) — error
- Empty layer slice — returns empty constitution, empty provenance
- Duplicate keys within same layer — last wins

**Boundary:**
- All four layers present but only domain has content — result equals domain
- Layer overrides every field — nothing inherited
- `$delete` removes the only item in a list — list becomes empty, not nil
- Principle overridden at project then `$delete`'d at domain — deleted wins

### Integration Tests — Storage (`internal/storage/postgres/`)

**Positive:**
- Store org layer, store project layer — both rows exist independently
- `GetConstitution` no filter: returns merged result with provenance
- `GetConstitution` with layer filter: returns raw single layer including `$delete`
- Update project layer — org layer unchanged, merged result reflects update
- Version increments per layer independently

**Negative:**
- `GetConstitution` for project with no layers — returns ErrConstitutionNotFound
- `GetConstitution` with layer filter for non-existent layer — returns ErrConstitutionNotFound
- Update with invalid layer — returns error

**Boundary:**
- Store all four layers, delete one, verify merged result excludes deleted layer
- Upsert same layer — replaces, doesn't duplicate
- Concurrent updates to different layers — both succeed
- Concurrent updates to same layer — version guard catches conflict

### E2E API Tests (`e2e/api/`)

- Import org + project with overrides and `$delete`, `GetConstitution` returns correct merged result with provenance
- `GetConstitution` with layer filter returns raw layer
- Update one layer, re-fetch merged — change reflected
- Analytical pass (constitution-check) validates against merged constitution

### E2E CLI Tests

- `constitution import --layer org` + `constitution import --layer project` + `constitution show` — merged
- `constitution show --layer org` — raw org content
- `constitution emit` — tool file from merged result

### Playwright Tests (`e2e/ui/`)

- Constitution page renders merged view
- Layer provenance badges visible
- Refreshing after layer import shows updated content

### DB Verification

Storage tests establish read method correctness first, then use proven methods as assertion tools for write operations — same trust chain as lifecycle tests.

---

## Section 7: Documentation

- **`concepts/constitution.md`**: Remove "Planned" admonition. Document multi-layer behavior, merge semantics, `$delete`, override precedence, with worked example.
- **`cli-reference.md`**: Regenerate via `specgraph docs cli`.
- **`guides/cli-cookbook.md`**: Add recipe for multi-layer constitution setup.

---

## Out of Scope

- Remote source fetching (URLs, git repos) — future work, `source_url` field is stored for forward compatibility
- Periodic sync / auto-refresh of remote layers — future work
- Layer editing in the web dashboard — CLI only
- Full JSON Patch (RFC 6902) operations — strategic merge is sufficient
- Cross-project layer sharing (one org constitution applied to many projects) — future work, currently each project imports its own copy

## Future Considerations

The `source_url` and `source_hash` fields enable future drift detection between cached constitution layers and their remote sources. When remote fetching is implemented, a `constitution sync` command could pull updates from source URLs and re-merge. The merge engine interface is designed to be extended with additional patch operations if strategic merge proves insufficient.
