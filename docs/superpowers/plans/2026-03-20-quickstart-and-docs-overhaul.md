# Quick Start Guide & Documentation Overhaul Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Quick Start guide and documentation overhaul that gates the 0.1.0 release.

**Architecture:** Two workstreams — (1) write a new `site/docs/quickstart.md` that walks users from install to first spec using Claude Code skills, (2) review and update all existing site docs for truthfulness against 0.1.0 capabilities. Plus config changes for release-please version markers and nav updates.

**Tech Stack:** Markdown (Zensical static site), TOML (zensical.toml), JSON (release-please-config.json)

**Spec:** `docs/superpowers/specs/2026-03-20-quickstart-and-docs-overhaul-design.md`

---

## Chunk 1: Infrastructure & Quick Start

### Task 1: Release-Please and Changelog Config

**Files:**

- Modify: `release-please-config.json`
- Create: `site/docs/changelog.md` (symlink)

- [ ] **Step 1: Update release-please-config.json**

Add `extra-files` to the package config so release-please updates version markers
in docs automatically:

```json
{
  "packages": {
    ".": {
      "release-type": "go",
      "bump-minor-pre-major": true,
      "bump-patch-for-minor-pre-major": true,
      "extra-files": [
        { "type": "generic", "path": "site/docs/quickstart.md" },
        { "type": "generic", "path": "site/docs/index.md" }
      ],
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "docs", "section": "Documentation" },
        { "type": "perf", "section": "Performance" },
        { "type": "refactor", "section": "Code Refactoring" },
        { "type": "test", "section": "Tests" },
        { "type": "ci", "section": "CI" },
        { "type": "build", "section": "Build" },
        { "type": "chore", "section": "Miscellaneous" }
      ]
    }
  }
}
```

- [ ] **Step 2: Create changelog symlink**

```bash
ln -s ../../CHANGELOG.md site/docs/changelog.md
```

Note: `CHANGELOG.md` doesn't exist yet (release-please creates it on first release PR).
The symlink will be broken until then. If Zensical fails on a broken symlink during the
build verification step (Task 14), remove the changelog.md nav entry and replace with a
link to GitHub releases.

- [ ] **Step 3: Delete roadmap.md**

Remove `site/docs/roadmap.md`.

- [ ] **Step 4: Commit**

```
jj --no-pager describe -m "docs: wire release-please extra-files, add changelog symlink, remove roadmap"
jj --no-pager new -m "wip"
```

---

### Task 2: Write Quick Start Guide

**Files:**

- Create: `site/docs/quickstart.md`

**Context to read before writing:**

- `cmd/specgraph/init.go` — understand what `init` actually creates
- `cmd/specgraph/up.go` — understand service mode vs manual mode
- `plugin/specgraph/plugin.json` — understand plugin structure
- `plugin/specgraph/skills/specgraph/spark/SKILL.md` — skill trigger phrases
- `plugin/specgraph/skills/specgraph/shape/SKILL.md` — skill trigger phrases
- `plugin/specgraph/skills/specgraph/specify/SKILL.md` — skill trigger phrases
- `plugin/specgraph/skills/specgraph/decompose/SKILL.md` — skill trigger phrases
- `plugin/specgraph/skills/specgraph/approve/SKILL.md` — skill trigger phrases
- `cmd/specgraph/lifecycle.go` — drift CLI behavior (drift is a lifecycle subcommand)

- [ ] **Step 1: Write the Quick Start guide**

Create `site/docs/quickstart.md` with these sections. Use the health check endpoint
example throughout. Format: fast-track action block at top of each section, expandable
`<details>` explanations underneath.

**Section structure:**

````markdown
# Quick Start

Get from zero to your first authored spec in under 10 minutes.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (runs the Memgraph graph database)
- [Claude Code](https://claude.ai/claude-code) (recommended — SpecGraph is designed for agentic authoring)

## Install SpecGraph

<!-- x-release-please-start-version -->

**Homebrew** (macOS/Linux):

```bash
brew install specgraph/tap/specgraph
```

**Binary** (any platform):

Download from [GitHub releases v0.1.0](https://github.com/specgraph/specgraph/releases/tag/v0.1.0),
verify SHA256 checksum, and add to your PATH.

**Docker**:

```bash
docker pull ghcr.io/specgraph/specgraph:0.1.0
```

<!-- x-release-please-end -->

> **Note:** Homebrew and Docker install paths require a published release. If
> v0.1.0 hasn't been released yet, build from source: `go install github.com/specgraph/specgraph/cmd/specgraph@latest`

<details><summary>What does specgraph install?</summary>
A single binary. No runtime dependencies beyond Docker for the database.
</details>

## Start the Server

```bash
specgraph init my-project
specgraph up
```

<details><summary>What just happened?</summary>

`init` creates `.specgraph.yaml` at repo root (project slug only — global config
lives at `~/.config/specgraph/config.yaml`).

`up` starts a Memgraph container via Docker Compose, then installs and starts
`specgraph serve` as a system service (launchd on macOS, systemd on Linux).
The server listens on `localhost:9090` and registers ConnectRPC services for
spec management, authoring, graph queries, and more.

In manual mode (`mode: manual` in config), run `specgraph serve` in a separate
terminal instead.
</details>

## Install the Claude Code Plugin

SpecGraph ships a Claude Code plugin with skills for each authoring stage.

If you're working inside the specgraph repo, the plugin is auto-discovered.

For other projects:

```bash
mkdir -p .claude/plugins
ln -s /path/to/specgraph/plugin/specgraph .claude/plugins/specgraph
```

<details><summary>What does the plugin provide?</summary>
11 skills covering the authoring funnel (spark, shape, specify, decompose, approve),
graph queries (list, show, deps, ready), execution bundles, and a router/concierge
that detects your intent and activates the right skill.
</details>

## Author Your First Spec

Let's create a spec for adding a health check endpoint. SpecGraph is designed for
agentic authoring — you describe what you want in natural language, and the
skills guide you through each stage.

### Spark — Capture the idea

> **You:** "I have an idea — we need a health check endpoint for our API"

The `specgraph-spark` skill activates. It captures:

- **Seed:** Health check endpoint for API
- **Signal:** Operational need — no way to verify service is running
- **Scope sniff:** tiny
- **Kill test:** If we drop HTTP entirely

<details><summary>CLI equivalent</summary>

```bash
specgraph spark health-check
```

</details>

### Shape — Bound the scope

> **You:** "Let's scope this out"

The `specgraph-shape` skill activates. It bounds:

- **Scope in:** GET /healthz returning 200 with server status
- **Scope out:** Deep health checks, dependency probing, metrics
- **Approaches:** (1) Simple HTTP handler, (2) ConnectRPC health service — chose option 1
- **Risks:** None significant for this scope

<details><summary>CLI equivalent</summary>

```bash
specgraph shape health-check
```

</details>

### Specify — Define the contract

> **You:** "Define the contract"

The `specgraph-specify` skill activates. It defines:

- **Interface:** `GET /healthz` returns `200 {"status": "ok", "version": "0.1.0"}`
- **Acceptance criteria:** Returns 200 when server is running, includes version
- **Invariants:** Must respond within 100ms, no auth required

<details><summary>CLI equivalent</summary>

```bash
specgraph specify health-check
```

</details>

### Decompose — Break it down

> **You:** "Break it down"

The `specgraph-decompose` skill activates. For this tiny spec, decomposition is
trivial — a single slice:

- **Slice 1:** Implement /healthz handler with version injection

<details><summary>CLI equivalent</summary>

```bash
specgraph decompose health-check
```

</details>

### Approve — Freeze for execution

> **You:** "Looks good, approve it"

The `specgraph-approve` skill runs the approval checklist and freezes the spec.
It's now claimable by an agent or human for implementation.

<details><summary>CLI equivalent</summary>

```bash
specgraph approve health-check
```

</details>

## Check for Drift

After working for a while, upstream specs may change. Drift detection catches this:

```bash
specgraph drift
```

<details><summary>How does drift detection work?</summary>

Every spec has a content hash (Murmur3-128) computed from its substantive fields.
When you add a dependency edge, the upstream's hash is recorded on the edge.
`specgraph drift` compares the recorded hash against the current hash — if they
differ, the upstream changed after you baselined your work.

Use `specgraph drift acknowledge --upstream <slug>` to accept the change, or
`--all` to baseline everything.
</details>

## Next Steps

- [Concepts: Specs as Graph Nodes](concepts/specs.md) — understand the full spec schema
- [Concepts: The Authoring Funnel](concepts/authoring.md) — deep dive into each stage
- [Concepts: Constitution](concepts/constitution.md) — project ground truth
- [Full Example: OAuth2 Token Rotation](concepts/example-spec.md) — a complex spec through all stages
- [Architecture](architecture.md) — system design and code organization
- [GitHub Issues](https://github.com/specgraph/specgraph/issues) — contribute or report bugs
````

Adapt the above skeleton as needed. The key requirements:
- Version numbers must have `x-release-please-version` inline annotations or use
  `x-release-please-start-version` / `x-release-please-end` blocks
- Each authoring stage shows what the user says, what happens, and key outputs
- CLI equivalents are in collapsible `<details>` blocks
- Keep it under ~200 lines — this is a quickstart, not a tutorial

- [ ] **Step 2: Update zensical.toml nav**

Now that `quickstart.md` exists, update the nav in `site/zensical.toml`. Remove
`roadmap.md`, add `quickstart.md` and `changelog.md`:

```toml
nav = [
  "index.md",
  "problem.md",
  "quickstart.md",
  "how-it-works.md",
  { "Concepts" = ["concepts/index.md", "concepts/specs.md", "concepts/constitution.md", "concepts/authoring.md", "concepts/decisions.md", "concepts/passes.md", "concepts/example-spec.md"] },
  "architecture.md",
  "ecosystem.md",
  "changelog.md",
]
```

- [ ] **Step 3: Create plugin README**

Create a minimal `plugin/specgraph/README.md` with install instructions. Keep it
brief — the quickstart is the primary docs:

```markdown
# SpecGraph Claude Code Plugin

Skills for the SpecGraph authoring funnel and graph queries.

## Install

**Inside the specgraph repo:** Auto-discovered by Claude Code.

**For other projects:**

    mkdir -p .claude/plugins
    ln -s /path/to/specgraph/plugin/specgraph .claude/plugins/specgraph

## Skills

| Skill | Purpose |
|-------|---------|
| specgraph | Router — detects intent and activates the right skill |
| specgraph-spark | Capture a raw idea |
| specgraph-shape | Bound scope and explore approaches |
| specgraph-specify | Define interface contracts and acceptance criteria |
| specgraph-decompose | Break spec into deliverable slices |
| specgraph-approve | Freeze spec for execution |
| specgraph-list | List specs with filters |
| specgraph-show | Show spec details |
| specgraph-deps | Show dependency tree |
| specgraph-ready | Show specs ready to claim |
| specgraph-bundle | Generate execution bundle |
```

- [ ] **Step 4: Verify the guide builds**

```bash
cd site && uv run zensical build 2>&1 | tail -20
```

Expected: Build succeeds. If the changelog symlink breaks the build, remove
`"changelog.md"` from the nav and note it for post-release.

- [ ] **Step 5: Commit**

```
jj --no-pager describe -m "docs: add Quick Start guide and plugin README for 0.1.0 (spgr-m3xx)"
jj --no-pager new -m "wip"
```

---

## Chunk 2: Documentation Overhaul — High-Impact Pages

These pages need substantive changes based on the truthfulness review.

### Task 3: Update index.md

**Files:**

- Modify: `site/docs/index.md`

- [ ] **Step 1: Update project status**

Remove the Phase 2 status banner at the bottom:

```markdown
!!! info "Phase 2 — Authoring & CLI"

    Phase 1 (spec schema, constitution, storage, and query layer) is complete.
    Phase 2 (authoring flow, codebase scanner, CLI integration) is in progress.
    See the [roadmap](roadmap.md) for what's coming next.
```

Replace with a version-marked status:

```markdown
!!! success "v0.1.0" <!-- x-release-please-version -->

    SpecGraph v0.1.0 is the first public release. Install it, start the server,
    and [author your first spec](quickstart.md) in under 10 minutes.
```

- [ ] **Step 2: Review core concept cards**

Read each card in the `<div class="grid cards">` section. Verify:
- "Specs as Graph Nodes" — accurate, no changes needed
- "Constitution" — accurate, no changes needed
- "Authoring Funnel" — accurate, no changes needed
- "Agent-Native" — verify the description matches current execution capabilities

- [ ] **Step 3: Update "How It Works" summary**

Verify the summary paragraph links to `how-it-works.md` (not `roadmap.md`).
Remove any roadmap references.

- [ ] **Step 4: Commit**

```
jj --no-pager describe -m "docs: update index.md for 0.1.0 release"
jj --no-pager new -m "wip"
```

---

### Task 4: Update ecosystem.md

**Files:**

- Modify: `site/docs/ecosystem.md`

**Codebase references to verify against:**

- `internal/sync/beads.go` — what does the Beads adapter actually do?
- `internal/sync/github.go` — what does the GitHub adapter actually do?

- [ ] **Step 1: Clarify Gastown status**

The existing note box is already honest ("Gastown is designed but not yet built").
Add emphasis that SpecGraph functions fully independently:

After the note box (line 14), add or strengthen language like:
"SpecGraph does not depend on Gastown. The integration described below is the
target architecture for multi-agent execution."

- [ ] **Step 2: Fix Beads sync direction**

Lines 56-62 describe bidirectional sync ("Both SpecGraph and Gastown read and write
to the same database"). The actual Beads adapter is **push-only** (SpecGraph to Beads).
Update to be accurate.

Change the "Status" box (line 47-48) from "In Progress" to "Shipped (push-only)":

```markdown
!!! info "Status: Shipped"
    The Beads adapter pushes specs to Beads as issues. Pull (Beads to SpecGraph)
    is not yet implemented.
```

- [ ] **Step 3: Fix sync adapter descriptions**

Line 74 says "Bidirectional sync" for GitHub Issues. Check `internal/sync/github.go`
to verify — if it's push-only like Beads, update the description.

Line 79 says Linear is "Planned" — this is correct, keep it.

- [ ] **Step 4: Update Integration Points section**

Line 100: "Claude Code (Planned — Slice 7)" — the plugin is **shipped** with 11 skills.
Update to:

```markdown
- **Claude Code** (Shipped) — Skills and hooks integrate SpecGraph into the IDE workflow.
```

Line 105: "MCP Server (Planned — Phase 3)" — still planned, keep it.

- [ ] **Step 5: Commit**

```
jj --no-pager describe -m "docs: update ecosystem.md for truthfulness"
jj --no-pager new -m "wip"
```

---

### Task 5: Update passes.md

**Files:**

- Modify: `site/docs/concepts/passes.md`

**Codebase references:**

- `internal/authoring/passes.go` — pass registry and scheduling
- `internal/authoring/safety.go` — safety net implementation
- `internal/server/authoring_handler.go:750-794` — `runAnalyticalPasses()` placeholder

- [ ] **Step 1: Add implementation status note**

After the overview section (after line 13), add a callout:

```markdown
!!! warning "0.1.0 Implementation Status"
    In v0.1.0, the **pass scheduling infrastructure** is fully implemented —
    passes are registered per-stage with posture-aware auto/offered rules
    (`internal/authoring/passes.go`). However, **pass execution returns
    placeholder findings**. The safety net (pattern-based scanning) is fully
    functional.

    Real LLM-driven pass execution is tracked for 0.2.0.
```

- [ ] **Step 2: Update safety net description**

The safety net section (lines 126-158) describes five categories of catches:
security, data loss, consistency contradictions, constitution violations, showstoppers.

The actual implementation in `safety.go` only covers **security** and **data loss**
pattern matching. The consistency, constitution, and showstopper categories described
are aspirational.

Update the safety net section to be honest:

```markdown
The safety net catches:

- **Security issues** — hardcoded credentials, disabled authentication, missing
  encryption, command injection patterns
- **Data loss risks** — destructive operations without rollback plans, irreversible
  state changes

The safety net performs fast pattern matching with Unicode normalization (NFKC) and
zero-width character stripping to prevent evasion. Critical findings (e.g., "hardcoded
secret") are distinguished from warnings (e.g., "credential" appearing in scope text).

Future versions will add consistency, constitution, and structural validation to the
safety net.
```

- [ ] **Step 3: Update pattern examples**

Lines 149-154 show patterns including `TODO`, `hack`, `temp`, `fixme`, `workaround`,
`skip test`, `no rollback`. Check `safety.go` — these patterns don't actually exist
in the implementation. Only show patterns that are actually implemented:

```text
CRITICAL: "hardcoded secret", "hardcoded password", "disable auth",
          "skip validation", "no encryption", "rm -rf"
          "drop table", "drop all", "delete all",
          "without migration", "without backup", "no rollback",
          "force delete"
(Note: patterns are matched case-insensitively after NFKC normalization)
WARNING:  "credential", "injection", "eval(", "exec(",
          "plaintext", "truncate", "purge"
```

- [ ] **Step 4: Commit**

```
jj --no-pager describe -m "docs: update passes.md with 0.1.0 implementation status"
jj --no-pager new -m "wip"
```

---

### Task 6: Update architecture.md

**Files:**

- Modify: `site/docs/architecture.md`

**Codebase references:**

- `cmd/specgraph/serve.go` — which services are actually registered (verify API Surface table)
- `internal/storage/memgraph/` — only implemented backend
- `internal/auth/` — auth package (if it exists, add to code org tree)

- [ ] **Step 1: Update storage section**

Lines 66-74 present Memgraph and Postgres+AGE as two implemented backends. Only
Memgraph is implemented. Update:

```markdown
**Memgraph** (default, shipped) — Native Cypher queries running in Docker. The only
implemented backend in v0.1.0.

**Postgres + AGE** (planned) — Cypher via the Apache AGE extension on standard
Postgres. Designed but not yet implemented.
```

- [ ] **Step 2: Verify service list**

Read `cmd/specgraph/serve.go` and verify the API Surface table (lines 46-57) lists
exactly the services that are registered. Add or remove any that don't match.

- [ ] **Step 3: Verify code organization tree**

Read the tree structure (lines 123-144) against actual directory listing output.
Verify each directory exists and the description is accurate. Add any missing
directories (e.g., `internal/auth/` if it exists).

- [ ] **Step 4: Update system diagram**

Line 17: "MCP server proxy" should be marked "(planned)" since it's not built.
Line 18: "Tauri+Svelte UI (future)" — keep as-is or remove if no longer planned.
Line 26: "Memgraph | Postgres+AGE" — update to "Memgraph (Postgres planned)".
Line 34: "Linear" — add "(planned)" marker.

- [ ] **Step 5: Commit**

```
jj --no-pager describe -m "docs: update architecture.md for 0.1.0 accuracy"
jj --no-pager new -m "wip"
```

---

### Task 7: Update how-it-works.md

**Files:**

- Modify: `site/docs/how-it-works.md`

- [ ] **Step 1: Add execution lifecycle**

The page covers the authoring funnel well but doesn't show what happens after approval.
After the "Execution-Ready Output" section (line 107), the pipeline diagram (lines
114-148) should include execution stages.

Update the pipeline diagram's Execution box to show the actual stages:

```text
│              Execution                           │
│                                                  │
│   Claim → In Progress → Review → Done            │
│   Agents or humans consume approved specs        │
│                                                  │
│   Terminal states: Amended | Superseded | Abandoned│
```

- [ ] **Step 2: Verify pipeline diagram accuracy**

Check that "Postgres+AGE" in the storage box (line 138) is marked as planned:

```text
│   Query: Cypher over Memgraph (Postgres planned) │
```

- [ ] **Step 3: Commit**

```
jj --no-pager describe -m "docs: update how-it-works.md with execution lifecycle"
jj --no-pager new -m "wip"
```

---

## Chunk 3: Documentation Overhaul — Light-Touch Pages

These pages were found accurate in review. Verify specific claims against codebase
and fix any discrepancies found.

### Task 8: Verify and update concepts/authoring.md

**Files:**

- Modify: `site/docs/concepts/authoring.md`

**Codebase references:**

- `proto/specgraph/v1/authoring.proto` — stage enum, posture enum, output messages
- `internal/storage/spec_domain.go` — all stage constants

- [ ] **Step 1: Connect funnel to execution stages**

The page describes Spark through Approve but doesn't explain what happens after
Approve. Add a section or note after the Approve stage:

```markdown
### After Approval

Once approved, a spec enters the execution lifecycle:

- **In Progress** — an agent or human has claimed the spec and is working on it
- **Review** — implementation is complete and awaiting review
- **Done** — review passed, work is complete

Specs can also reach terminal states at any point:

- **Amended** — returned to an earlier authoring stage for changes
- **Superseded** — replaced by a newer spec
- **Abandoned** — dropped; no longer relevant
```

- [ ] **Step 2: Clarify posture auto-detection**

If the page describes posture auto-detection as a server-side feature, clarify that
it's a **skill-layer feature** in the Claude Code plugin. The server accepts a posture
parameter but doesn't detect it — the skills infer posture from conversation style.

- [ ] **Step 3: Verify proto field names**

Verify that all stage names, posture names, and output field names in the page match
the current proto definitions in `authoring.proto`.

- [ ] **Step 4: Commit**

```
jj --no-pager describe -m "docs: update authoring.md with execution stages and posture clarification"
jj --no-pager new -m "wip"
```

---

### Task 9: Verify concepts/specs.md

**Files:**

- Modify (if needed): `site/docs/concepts/specs.md`

**Codebase references:**

- `proto/specgraph/v1/spec.proto` — spec message fields
- `internal/storage/spec_domain.go` — domain type fields
- `proto/specgraph/v1/graph.proto` — edge types

- [ ] **Step 1: Verify proto field names**

Read `spec.proto` and verify every field name mentioned in specs.md still exists.
Check for renamed or removed fields.

- [ ] **Step 2: Verify edge type names**

Read `graph.proto` and verify all edge types listed (depends_on, blocks, composes,
relates_to, decided_in, informs, supersedes) match the `EdgeType` enum.

- [ ] **Step 3: Verify internal links**

Check that all markdown links in the file point to existing pages.

- [ ] **Step 4: Fix any discrepancies found and commit**

If changes needed:
```
jj --no-pager describe -m "docs: fix specs.md accuracy issues"
jj --no-pager new -m "wip"
```

If no changes needed, skip commit.

---

### Task 10: Verify concepts/constitution.md

**Files:**

- Modify (if needed): `site/docs/concepts/constitution.md`

**Codebase references:**

- `internal/storage/constitution_domain.go` — layer names, field types

- [ ] **Step 1: Verify layer names and fields**

Read `constitution_domain.go` and verify the four layers (User, Org, Project, Domain)
and all captured fields match the documentation.

- [ ] **Step 2: Add cross-link to passes**

Add a note linking to the constitution check pass in `passes.md`:

```markdown
The constitution is automatically checked at every authoring stage via the
[constitution check pass](passes.md#constitution-check).
```

- [ ] **Step 3: Verify internal links and commit if changed**

---

### Task 11: Verify concepts/decisions.md

**Files:**

- Modify (if needed): `site/docs/concepts/decisions.md`

**Codebase references:**

- `proto/specgraph/v1/decision.proto` — decision schema
- `docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md` — ADR-003

- [ ] **Step 1: Verify decision schema matches proto**

Read `decision.proto` and verify field names, lifecycle states (proposed, accepted,
deprecated, superseded), and edge directions match.

- [ ] **Step 2: Verify internal links and commit if changed**

---

### Task 12: Verify concepts/example-spec.md

**Files:**

- Modify (if needed): `site/docs/concepts/example-spec.md`

**Codebase references:**

- `proto/specgraph/v1/authoring.proto` — SparkOutput, ShapeOutput, SpecifyOutput, DecomposeOutput messages
- `proto/specgraph/v1/authoring.proto` — DecompositionStrategy enum

- [ ] **Step 1: Verify stage outputs map to proto**

Read `authoring.proto` and verify every field shown in the example spec exists in
the corresponding proto output message. Check for renamed or added fields.

- [ ] **Step 2: Verify decomposition strategy names**

Check that the strategy names used in the example match the `DecompositionStrategy`
enum values.

- [ ] **Step 3: Fix and commit if changed**

---

### Task 13: Verify concepts/index.md and problem.md

**Files:**

- Modify (if needed): `site/docs/concepts/index.md`
- Modify (if needed): `site/docs/problem.md`

- [ ] **Step 1: Verify concepts/index.md cards**

Read the card descriptions. Verify they accurately describe current capabilities.
Verify all internal links resolve to existing pages.

- [ ] **Step 2: Review problem.md**

Read for accuracy. The review found this page compelling and accurate. Verify no
links point to removed pages (roadmap.md).

- [ ] **Step 3: Fix and commit if changed**

---

## Chunk 4: Final Verification

### Task 14: Build Verification and Link Check

- [ ] **Step 1: Run Zensical build**

```bash
cd /Volumes/Code/github.com/seanb4t/specgraph/site && uv run zensical build 2>&1 | tail -30
```

Expected: Build succeeds with no errors. If the changelog symlink causes issues,
remove `"changelog.md"` from the nav in `zensical.toml`.

- [ ] **Step 2: Check for stale references**

Search site/docs/ for any remaining references to `roadmap.md` or `roadmap`.
Search for `planned|future|upcoming|coming soon` and verify all occurrences
are in clearly marked callout boxes or notes.

- [ ] **Step 3: Check version markers**

Search `site/docs/quickstart.md` and `site/docs/index.md` for `x-release-please`
annotations. Verify at least one version marker in each file using correct syntax.

- [ ] **Step 4: Squash and final commit**

Review all changes with `jj --no-pager log --no-graph -r 'ancestors(@,10) & ~ancestors(main)'`
to see all change IDs. Identify the change IDs for each logical group, then squash into
logical commits:

```bash
jj --no-pager squash --from <config-change> --into <target> -m "docs: infrastructure for 0.1.0 docs (nav, release-please, changelog)"
jj --no-pager squash --from <quickstart> --into <target> -m "docs: add Quick Start guide for 0.1.0"
jj --no-pager squash --from <overhaul-changes> --into <target> -m "docs: overhaul site docs for 0.1.0 truthfulness"
```

Exact squash strategy depends on how many changes accumulated.

- [ ] **Step 5: Close the bead**

```bash
bd close spgr-m3xx --reason="Quick Start guide written, all docs reviewed and updated for 0.1.0 truthfulness"
```
