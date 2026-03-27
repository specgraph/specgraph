# Site Documentation Feature Coverage Gaps

**Bead:** spgr-drz
**Date:** 2026-03-27
**Status:** Approved

## Context

Feature review comparing the implemented codebase against `site/docs/` revealed
significant gaps. The slice epic (spgr-6sw, PRs #689-693) shipped Slice as a
first-class graph vertex with CLI commands, SliceService RPCs, and web UI — none
of which are reflected in the docs. Beyond slices, 30+ CLI commands have no
reference page, and drift detection, spec linting, and sync adapters are
implemented but undocumented or barely mentioned.

Conceptual docs (specs, constitution, authoring, decisions, passes) are strong.
Practical reference and guide content is missing.

## Goals

1. Auto-generate a CLI reference page from the Cobra command tree
2. Add concept pages for slices, drift detection, and spec linting
3. Add a guides section with CLI cookbook and sync guide
4. Update existing pages to reflect slices as first-class vertices
5. Fix stale content (scanner references, passes "placeholders", per-spec drift fields)

## Non-Goals

- Tool injection docs (feature needs its own design pass)
- Operations page (daemon, config, Docker modes) — LOW priority, deferred
- Auth system docs — shipped but not user-facing yet
- MkDocs theme or deployment changes
- README rewrite

---

## Workstream 1: Auto-generated CLI Reference

### `specgraph docs` Command

Add a `docs` subcommand that walks the Cobra command tree and generates a single
grouped markdown file. This is custom generation logic (not `cobra/doc`'s
one-file-per-command output) — iterate the command tree, group by domain, and
write sections with synopsis, flags, and examples into one file.

**File:** `cmd/specgraph/docs.go`

**Output:** `site/docs/cli-reference.md`

**Format:** Table of contents at top, then commands grouped by domain:

| Group | Commands |
|-------|----------|
| Spec Management | create, show, list, update |
| Authoring Funnel | spark, shape, specify, decompose, approve |
| Slices | slice list, slice get, slice claim, slice complete |
| Graph Queries | deps, impact, critical-path, ready |
| Drift & Linting | drift, drift acknowledge, lint |
| Execution Lifecycle | claim, unclaim, report-progress, report-blocker, report-completion, bundle |
| Lifecycle Transitions | amend, supersede, abandon |
| Decisions | decision create, decision list, decision get, decision update |
| Constitution | constitution subcommands |
| Sync | sync subcommands |
| Server & Config | up, down, serve, status, health, init |

**Header:** The file includes a generated-from notice: "Auto-generated from
`specgraph docs`. Do not edit manually."

### Taskfile Integration

`task docs:cli` runs the generation. Can be wired into a broader `task docs`
target later.

### Staleness Guard

A CI check regenerates the file and diffs against the committed version. Fails
if stale. This ensures the reference stays in sync with the code.

---

## Workstream 2: New Concept Pages

### concepts/slices.md — Slices & Execution Units

**Sections:**

1. **What is a slice?** — Decompose creates Slice nodes in the graph, not just
   JSON fields. Each slice is an independently claimable and completable unit of
   work with its own intent, verify criteria, and touches list.

2. **Graph model** — `(:Spec) -[:HAS_SLICE]-> (:Slice)`. Slices inherit the
   parent spec's edges for context but are tracked independently. The
   `COMPOSES` edge between specs is separate from the `HAS_SLICE` edge between
   a spec and its slices.

3. **Lifecycle** — `open` → `claimed` (agent or human takes it) → `completed`.
   The parent spec's done-state depends on all slices completing.

4. **CLI usage** — `slice list <parent-slug>`, `slice get <slug>`,
   `slice claim <slug>`, `slice complete <slug>`.

5. **Worked example** — The healthz spec from quickstart: decompose creates a
   single slice, claim it, complete it, parent transitions.

### concepts/drift.md — Drift Detection

**Sections:**

1. **What is drift?** — When an upstream spec changes after a dependency edge
   was baselined, the downstream spec has "drifted" — its assumptions may be
   stale.

2. **Mechanism** — `content_hash_at_link` on DEPENDS_ON edges records the
   upstream's content hash when the edge was created or acknowledged. Detection
   compares this edge hash against the upstream's current content hash.

3. **Per-edge acknowledgment** — Drift is acknowledged per-edge, not per-spec
   (changed in PR #43). Acknowledge one upstream at a time or all at once.

4. **CLI usage** — `drift [slug]` to check (single spec or all),
   `drift acknowledge <slug> --upstream <dep>` or `--all` with optional
   `--note`.

5. **Worked example** — Two specs with a dependency. Upstream gets amended,
   drift is detected, acknowledged with a note.

### concepts/linting.md — Spec Linting

**Sections:**

1. **What does the linter check?** — Schema validation (required fields per
   stage), edge consistency (no dangling references), constitution compliance,
   cycle detection.

2. **CLI usage** — `lint` (all specs) or `lint <slug>` (single spec).

3. **Relationship to passes** — Linting is synchronous and structural;
   analytical passes are deeper and async. Linting catches "this spec is
   malformed"; passes catch "this spec contradicts the constitution".

---

## Workstream 3: Guide Pages

### guides/index.md

Grid cards page (mirrors `concepts/index.md` style) linking to the two guides.

### guides/cli-cookbook.md — CLI Workflows

Practical recipes organized by task. Each recipe has a goal statement, commands,
and expected output. Self-contained — readers can jump to any recipe.

**Recipes:**

1. **Author a spec end-to-end** — spark through approve with real flags
   (extends quickstart with flag-level detail)
2. **Query the dependency graph** — `deps`, `impact`, `critical-path`, `ready`
   with example output
3. **Work with slices** — decompose a multi-slice spec, list slices, claim one,
   report progress, complete it
4. **Detect and resolve drift** — introduce a change, see drift fire,
   acknowledge
5. **Lint before merging** — run the linter, interpret violations, fix and
   re-lint
6. **Manage execution lifecycle** — claim → progress → blocker → completion
   flow
7. **Generate an execution bundle** — `bundle <slug>` and what the output
   contains

### guides/sync.md — Sync & Integration

**Sections:**

1. **Beads sync** — `sync beads <slug>`, dry-run mode (`--dry-run`), what gets
   pushed (slug, intent, stage), status polling
2. **GitHub sync** — `sync github <slug>`, issue format, label mapping, polling
3. No inject section — that feature needs its own design pass

---

## Workstream 4: Updates to Existing Pages

### architecture.md

- Add `SliceService` to API Surface table: "Create, list, get slices. Claim and
  complete slices for execution."
- Add `AnalyticalPassService` to API Surface table: "Run analytical passes and
  manage findings."
- Add `(:Spec) -[:HAS_SLICE]-> (:Slice)` to Graph Data Model diagram
- Add `(:Spec) -[:HAS_FINDING]-> (:Finding)` to Graph Data Model (noted as
  internal edges)

### quickstart.md

- Update decompose section to explain that slices are created as graph nodes
- Add "Claim & Complete" step after Approve showing `slice list`,
  `slice claim`, `slice complete`
- Update drift section to mention per-edge acknowledgment

### concepts/index.md

- Add 3 new grid cards: Slices & Execution Units, Drift Detection, Spec Linting

### concepts/authoring.md

- Update decompose stage to cross-reference `concepts/slices.md` for the full
  slice lifecycle

---

## Workstream 5: Accuracy Fixes

| Issue | Action |
|-------|--------|
| `passes.md` says "placeholders" | Update to reflect current implementation state |
| `index.md` version marker | Verify and update if stale |
| Scanner references (removed PR #22) | Grep and remove any stale mentions |
| Per-spec `DriftAcknowledged` (removed PR #43) | Verify no references remain; update if found |

---

## Navigation Update

`site/zensical.toml` nav array must be updated to include new pages and the
Guides section:

```toml
nav = [
  "index.md",
  "problem.md",
  "quickstart.md",
  "how-it-works.md",
  { "Concepts" = [
    "concepts/index.md",
    "concepts/specs.md",
    "concepts/constitution.md",
    "concepts/authoring.md",
    "concepts/decisions.md",
    "concepts/passes.md",
    "concepts/slices.md",
    "concepts/drift.md",
    "concepts/linting.md",
    "concepts/example-spec.md",
  ] },
  { "Guides" = [
    "guides/index.md",
    "guides/cli-cookbook.md",
    "guides/sync.md",
  ] },
  "cli-reference.md",
  "architecture.md",
  "ecosystem.md",
  "changelog.md",
]
```

---

## File Inventory

**New files (8):**

| File | Type |
|------|------|
| `cmd/specgraph/docs.go` | Go source — `specgraph docs` command |
| `site/docs/cli-reference.md` | Auto-generated CLI reference |
| `site/docs/concepts/slices.md` | Concept page |
| `site/docs/concepts/drift.md` | Concept page |
| `site/docs/concepts/linting.md` | Concept page |
| `site/docs/guides/index.md` | Guide section index |
| `site/docs/guides/cli-cookbook.md` | CLI workflow recipes |
| `site/docs/guides/sync.md` | Sync & integration guide |

**Modified files (6):**

| File | Changes |
|------|---------|
| `site/zensical.toml` | Add new pages and Guides section to nav |
| `site/docs/architecture.md` | Add SliceService, AnalyticalPassService, Slice/Finding nodes |
| `site/docs/quickstart.md` | Update decompose, add slice workflow, fix drift section |
| `site/docs/concepts/index.md` | Add 3 grid cards |
| `site/docs/concepts/authoring.md` | Cross-reference slices.md from decompose |
| `site/docs/concepts/passes.md` | Fix "placeholders" language |

**Possibly modified:**

| File | Condition |
|------|-----------|
| `site/docs/index.md` | If version marker is stale |
| `site/docs/concepts/passes.md` | Stale scanner reference on line 150 (scanner removed PR #22) |
