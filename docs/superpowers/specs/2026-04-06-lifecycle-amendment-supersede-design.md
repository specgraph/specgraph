# Lifecycle Amendment & Supersede — Design Spec

**Date:** 2026-04-06
**Status:** Draft
**Goal:** Ensure amendment and supersede use cases are fully documented, visually clear on both CLI and web dashboard, and validated end-to-end from UI/UX through to database state.

## Context

The backend for amend/supersede/abandon is implemented: proto definitions, storage layer (Postgres with version guards, transactions, changelog entries), server handlers, and CLI commands all exist. Basic E2E tests cover RPC-level happy paths.

**Gaps this spec addresses:**

1. No site documentation dedicated to lifecycle transitions
2. CLI `changes` command doesn't render `reason` field or support inline diffs
3. Web dashboard has zero changelog/history UI and no lifecycle state indicators beyond basic stage badges
4. E2E tests don't verify changelog entries, field deltas, or content_hash updates after lifecycle operations
5. No arbitrary version comparison capability (API or UI)

## Approach: Shared Infrastructure Then Verticals

Build shared pieces (diff engine, changelog components, version comparison) first, then layer amendment and supersede flows on top.

**Phases:**

1. Shared infrastructure — diff computation, web changelog component, CLI `--diff` flag, `CompareVersions` RPC
2. Amendment vertical — CLI output fixes, web rendering of amended state, docs, E2E
3. Supersede vertical — web rendering of superseded state, SUPERSEDES edges, docs, E2E
4. Documentation — lifecycle concept page, cross-references

---

## Section 1: Backend — Diff Computation & API

### New RPC: `CompareVersions`

Added to `SpecService` in `proto/specgraph/v1/spec.proto`:

```protobuf
message CompareVersionsRequest {
  string slug = 1;
  int32 from_version = 2;  // 0 = auto (previous version relative to to_version)
  int32 to_version = 3;    // 0 = auto (latest)
}

message VersionDiff {
  string field = 1;
  string old_value = 2;
  string new_value = 3;
  repeated InlineDiff hunks = 4;
}

message InlineDiff {
  enum Op { EQUAL = 0; INSERT = 1; DELETE = 2; }
  Op op = 1;
  string text = 2;
}

message CompareVersionsResponse {
  int32 from_version = 1;
  int32 to_version = 2;
  string from_stage = 3;
  string to_stage = 4;
  repeated VersionDiff diffs = 5;
}
```

### Diff computation package: `internal/diff`

Thin wrapper around a Go diffmatchpatch library (e.g., `sergi/go-diff`). Tokenizes by word, produces `InlineDiff` hunks. Keeps diffing out of the storage layer.

### Storage addition: `GetSpecAtVersion`

```go
GetSpecAtVersion(ctx context.Context, slug string, version int32) (*Spec, error)
```

Reconstructs spec state at a given version by walking changelog entries forward from v1 and applying field deltas. The field set is small enough that reconstruction is cheap.

`CompareVersions` handler calls `GetSpecAtVersion` twice, then runs the diff engine on each changed field.

---

## Section 2: CLI — `specgraph changes` Enhancements

### Single command, progressive disclosure

```text
specgraph changes <slug>                            # field-level changelog (current + reason)
specgraph changes <slug> --diff                     # adds word-level inline diffs per entry
specgraph changes <slug> --diff --from=3 --to=7     # arbitrary version comparison
specgraph changes <slug> --diff --from=5            # compare v5 to latest
```

Existing flags (`--checkpoints`, `--since-version`, `--limit`, `--json`) compose with `--diff`.

### Rendering

**Without `--diff`** (enhanced current behavior):

```text
## v7 → amended (checkpoint)
  2026-04-05 | Hash: a1b2c3d4
  Reason: Requirements changed after stakeholder review

  | Field   | Old              | New                |
  |---------|------------------|--------------------|
  | stage   | done             | shape              |
  | intent  | Build auth flow  | Build OAuth2 flow  |
```

Key fix: `reason` field now rendered (currently captured but not displayed).

**With `--diff`** (inline word-level):

```text
## v7 → amended (checkpoint)
  2026-04-05 | Hash: a1b2c3d4
  Reason: Requirements changed after stakeholder review

  stage: done → shape
  intent: Build [-auth-]{+OAuth2+} flow
```

Uses `[-deleted-]` and `{+inserted+}` markers. Color-coded when terminal supports it (red for deletions, green for insertions).

**With `--diff --from=3 --to=7`** (version comparison):

```text
## Comparing v3 → v7

  stage: specify → amended
  intent: Build [-basic auth-]{+OAuth2 with PKCE+} flow
  shape_output:
    [-Single login form with username/password-]
    {+Authorization code flow with refresh token rotation+}
```

Calls `CompareVersions` RPC, renders returned hunks.

### JSON output

`--json` with `--diff` includes the `InlineDiff` hunks array so tooling can consume structured diffs.

---

## Section 3: Web Dashboard — Changelog & Diff Views

The web dashboard is read-only. All mutations happen via CLI. The dashboard renders results.

### 3a. Changelog Timeline Component

New `ChangelogTimeline.svelte` component on the spec detail page as an accordion section alongside Edges, Findings, and Conversations.

**Layout:**

- Vertical timeline, newest entry at top
- Each entry is a card: version badge, stage, timestamp, reason (if present), summary
- Checkpoint entries get a filled dot marker; non-checkpoint entries get a hollow dot
- Expandable: clicking an entry reveals field-level changes as a **side-by-side diff** (old on left, new on right) with word-level highlighting (green for insertions, red strikethrough for deletions)

**Data:** Calls `ListChanges` RPC lazily when the accordion opens. Caches the result.

### 3b. Version Comparison View

A "Compare versions" control above the timeline:

- Two dropdown selectors (from/to) populated from changelog versions
- "Compare" button calls `CompareVersions` RPC
- Renders a dedicated side-by-side panel showing all changed fields
- Large text fields (shape_output, specify_output, etc.) get scrollable diff panels with word-level inline highlighting

**Diff rendering:** Use a lightweight client-side library (e.g., `diff` npm package, ~7kb gzipped) to render the hunks from `CompareVersions` as highlighted HTML spans.

### 3c. Lifecycle State Indicators

- **Stage badge colors**: Extend the existing color map for `amended` (amber), `superseded` (gray with strikethrough styling), `abandoned` (red)
- **Supersession banner**: If `spec.supersededBy` is set, show a prominent banner: "This spec has been superseded by [slug]" with a link to the replacement. If `spec.supersedes` is set: "This spec supersedes [slug]" with a link to the old spec.
- **Graph view**: SUPERSEDES edges already render with orange dashes — no changes needed.

### 3d. Dashboard Summary

Extend the StatsBar on the main dashboard to include counts of `amended` and `superseded` specs as informational badges.

---

## Section 4: Site Documentation

### 4a. New concept page: `site/docs/concepts/lifecycle.md`

**Title:** "Lifecycle Transitions"

**Structure:**

1. **Overview** — Specs aren't static. After reaching `done`, three transitions exist: amend, supersede, abandon. Framed as "what happens when reality changes."

2. **Amendment** — Returning a completed spec to an earlier authoring stage.
   - When to use: scope refinement, requirements clarification, fixing mistakes in the spec itself.
   - Semantics: spec keeps its slug, version increments, stage resets to re-entry point, changelog gets a checkpoint entry with reason.
   - Re-entry stage selection guidance: intent shift → spark, structural change → shape, detail correction → specify.
   - Amended specs can be amended again after re-completion (not fully terminal).

3. **Supersession** — Replacing a spec with a fundamentally different one.
   - When to use: the approach is wrong, not just the details. New spec represents a different solution.
   - Semantics: old spec becomes `superseded` (terminal), new spec gets `supersedes` field, SUPERSEDES edge created (new → old), both get changelog entries.
   - The new spec starts fresh in the authoring funnel.

4. **Abandonment** — Dropping a spec entirely.
   - When to use: the problem is no longer relevant, or work absorbed elsewhere.
   - Semantics: fully terminal — cannot be amended or superseded after.

5. **Decision tree** — "Amend vs Supersede vs Abandon" flowchart:
   - Is the problem still relevant? No → Abandon.
   - Is the current approach still valid? Yes → Amend (pick re-entry stage). No → Supersede.

6. **Worked example** — Narrative walking through a spec that gets amended (requirements refined), then later the amended spec gets superseded (approach pivots). Shows CLI commands and graph state at each step.

### 4b. Cross-references from existing pages

- **`specs.md`**: Link `amended`, `superseded`, `abandoned` in the stage listing to the lifecycle page.
- **`authoring.md`**: Replace the brief "Terminal States" section with a callout: "See [Lifecycle Transitions](lifecycle.md) for details."
- **`cli-reference.md`**: Add "See also: [Lifecycle Transitions](../concepts/lifecycle.md)" link in the lifecycle section header.
- **Navigation config** (`zensical.toml`): Add `lifecycle.md` to the concepts section after `drift.md` and before `linting.md`.

---

## Section 5: E2E Test Coverage

### Test trust chain

Tests are layered so each level builds confidence on the one below:

1. **Storage method correctness** (integration tests against real Postgres) — proves read methods accurately reflect DB state
2. **Lifecycle integration tests** — use proven storage methods to verify state after transitions
3. **E2E tests** — use proven RPCs to verify full-stack behavior
4. **Playwright tests** — use API setup + browser assertions to verify UI rendering

### 5a. Storage Method Correctness (Layer 1)

Extend integration tests to prove storage read methods are trustworthy for lifecycle scenarios:

- `ListChanges` after amend: returns checkpoint entry with reason, field deltas include `stage: done → <re-entry>`, content_hash changed
- `ListChanges` after supersede: old spec has checkpoint with `superseded_by` delta, new spec has `supersedes` delta
- `ListChanges` after abandon: returns checkpoint entry with reason, stage delta `→ abandoned`
- `ListChanges` field delta completeness: verify `stage`, `superseded_by`, `supersedes`, `content_hash` fields are tracked (currently only `intent` tested)
- `GetDependenciesWithEdgeData` after amend + re-complete: verify `content_hash_at_link` reflects the new upstream hash
- `GetSpec` read-back after supersede: verify `superseded_by`/`supersedes` fields populated through the read path independently

### 5b. Lifecycle Integration Tests (Layer 2)

Extend `internal/storage/postgres/lifecycle_test.go` — use the now-proven storage methods as assertion tools:

- After amend: `ListChanges` returns correct changelog, `GetSpec` shows new stage/version/content_hash
- After amend + re-complete: `GetDependenciesWithEdgeData` shows refreshed `content_hash_at_link` on DEPENDS_ON edges
- After supersede: `ListChanges` on both specs correct, `ListEdges` shows SUPERSEDES edge, `GetSpec` on both shows correct `superseded_by`/`supersedes`
- After abandon: `ListChanges` correct, `GetSpec` shows `abandoned` stage

### 5c. API E2E Tests (Layer 3)

Extend `e2e/api/lifecycle_test.go`:

- After each lifecycle operation, call `ListChanges` RPC and verify changelog entry came through the full stack
- After supersede, call `CompareVersions` on old spec and verify `superseded_by` field appears in diffs

New `e2e/api/changes_test.go` for `CompareVersions` RPC:

- Create spec, advance through multiple stages, verify `CompareVersions` returns correct hunks
- Version auto-resolution (from=0, to=0)
- Error cases: invalid version, spec not found

### 5d. CLI Output Tests (Layer 3)

Tests that invoke the binary and assert on rendered output:

- `specgraph changes <slug>` after amendment shows reason field
- `specgraph changes <slug> --diff` shows `[-old-]{+new+}` markers
- `specgraph changes <slug> --diff --from=1 --to=3` renders version comparison header and inline diffs
- JSON output with `--json --diff` includes structured hunks array

### 5e. Playwright Browser Tests (Layer 4)

New `e2e/ui/tests/lifecycle.spec.ts`:

- **Changelog renders**: Navigate to spec detail, open Changelog accordion, verify timeline entries appear with correct versions/stages/timestamps
- **Checkpoint markers**: Amend a spec via API setup, verify the amendment entry has a visual checkpoint indicator
- **Diff expansion**: Click a changelog entry, verify side-by-side diff panel appears with highlighted changes
- **Version comparison**: Select two versions in picker, click Compare, verify diff panel renders with correct content
- **Supersession banner**: Navigate to superseded spec, verify "Superseded by [slug]" banner with working link
- **Supersedes link**: Navigate to replacement spec, verify "Supersedes [slug]" banner with working link
- **Stage badges**: Verify `amended`, `superseded`, `abandoned` render with correct colors
- **Graph view**: After supersede, verify SUPERSEDES edge visible between two specs
- **Dashboard counts**: Verify StatsBar includes amended/superseded counts

Test data setup: Playwright tests use ConnectRPC client calls in `beforeAll` to create and transition specs, then assert on rendered UI (same pattern as existing `detail.spec.ts`).

---

## Out of Scope

- Web UI action buttons (amend/supersede/abandon) — dashboard is read-only
- Drift detection UI — separate concern
- Lint results UI — separate concern
- Multi-spec batch operations
- Undo/revert operations
