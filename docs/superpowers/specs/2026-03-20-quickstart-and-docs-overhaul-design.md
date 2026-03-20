# Quick Start Guide and Documentation Overhaul for 0.1.0

**Bead:** spgr-m3xx
**Date:** 2026-03-20
**Status:** Approved

## Context

SpecGraph 0.1.0 is the first public release. The site (`site/docs/`, Zensical) has
rich concept documentation but no getting-started content and several accuracy gaps.
This is the last gate before cutting the release.

## Goals

1. Write a Quick Start guide that takes a new user from install to first authored spec
2. Review and update all existing documentation for truthfulness against 0.1.0 capabilities
3. Wire release-please to keep version numbers in docs current
4. Remove internal project-management artifacts (roadmap) from user-facing docs

## Non-Goals

- README rewrite
- API reference documentation
- Site deployment (CloudPages)
- Analytical pass execution (tracked as 0.2.0 milestone beads: spgr-5pq, spgr-wjz, spgr-ney, spgr-ikx, spgr-bmd)

---

## Workstream 1: Quick Start Guide

### File

`site/docs/quickstart.md`

### Audience

Developers evaluating or adopting SpecGraph. Assumes CLI comfort but not prior
SpecGraph knowledge.

### Format

Each section has a fast-track command/action block at the top (copy-paste friendly),
with expandable explanations underneath for users who want to understand what happened.

### Flow

```text
Install → Start Server → Install Claude Code Plugin → Author First Spec → Check Drift → Next Steps
```

### Example Project

"Add a `/healthz` endpoint to my API" — small, universal, completable in 10 minutes.
Chosen over the OAuth2 example (which lives in `example-spec.md`) because it lets users
focus on learning the tool rather than a complex domain.

### Section Details

#### 1. Prerequisites

- Docker (for Memgraph database)
- One of: Homebrew, or a platform binary, or Docker for specgraph itself

#### 2. Install

Three paths shown in tabs or collapsible sections:

- **Homebrew:** `brew install specgraph/tap/specgraph`
- **Binary:** Download from GitHub releases, verify checksum, add to PATH
- **Docker:** `docker pull ghcr.io/specgraph/specgraph:latest`

Version numbers use inline `x-release-please-version` annotations so release-please
replaces the version on lines where it appears (e.g.,
`Download specgraph v0.1.0 <!-- x-release-please-version -->`). Use block
`x-release-please-start-version` / `x-release-please-end` markers only if multiple
lines within a block contain version values. The `extra-files` config with
`"type": "generic"` is required since `.md` has no default updater.

Note: The Docker image (`ghcr.io/specgraph/specgraph`) and Homebrew formula won't exist
until the first release-please PR is merged and goreleaser runs. The quickstart should
note this bootstrapping dependency or gate those install paths on the release existing.

#### 3. Start the Server

```bash
specgraph init my-project
specgraph up
```

`init` creates `.specgraph.yaml` at repo root (contains project slug only — global
config lives at `~/.config/specgraph/config.yaml`). `up` starts the Memgraph container
via Docker Compose, then installs and starts `specgraph serve` as a system service
(launchd on macOS, systemd on Linux). In manual mode (`mode: manual` in config),
`up` prints a message to run `specgraph serve` separately.

Expandable: what the global config contains, what the health check loop does,
what ConnectRPC services are registered on port 9090.

#### 4. Install the Claude Code Plugin

The SpecGraph plugin lives at `plugin/specgraph/` in the repo. Claude Code discovers
plugins from the project directory. Installation is:

```bash
# From the specgraph repo (plugin is auto-discovered)
# Or for external projects, copy/symlink the plugin directory:
mkdir -p .claude/plugins
ln -s /path/to/specgraph/plugin/specgraph .claude/plugins/specgraph
```

Note: A plugin README with install instructions needs to be created as part of this
work (none exists today). The quickstart should document the simplest path.

#### 5. Author Your First Spec (Primary: Claude Code Skills)

Walk through the authoring funnel using natural language with Claude Code:

1. **Spark:** "I have an idea — we need a health check endpoint for our API"
   - `specgraph-spark` skill activates
   - Captures seed, signal, scope_sniff, kill_test
   - Show what the conversation looks like

2. **Shape:** "Let's scope this out"
   - `specgraph-shape` skill activates
   - Bounds scope (in/out), explores approaches, surfaces risks
   - Show key outputs

3. **Specify:** "Define the contract"
   - `specgraph-specify` skill activates
   - Interface contract, acceptance criteria, invariants
   - Show key outputs

4. **Decompose:** "Break it down"
   - `specgraph-decompose` skill activates
   - Creates independently deliverable slices
   - Show key outputs

5. **Approve:** "Looks good, approve it"
   - `specgraph-approve` skill activates
   - Freezes spec for execution
   - Show final state

Each step shows what the user says, what the skill does, and what changed in the graph.

**Secondary: CLI Reference (collapsible)**

Equivalent CLI commands for each step, for users who want direct control or scripting.

```bash
specgraph spark health-check
specgraph shape health-check
specgraph specify health-check
specgraph decompose health-check
specgraph approve health-check
```

#### 6. Check for Drift

```bash
specgraph drift
```

Explain what drift detection does: compares content hashes on dependency edges to
detect when upstream specs change after downstream work was baselined.

#### 7. Next Steps

Links to:

- Concept docs (specs, constitution, authoring, decisions)
- Full example spec (`example-spec.md`)
- Architecture overview
- GitHub Issues for contributing

---

## Workstream 2: Documentation Overhaul

### Guiding Principle

Every page must describe SpecGraph as it actually exists in 0.1.0 — capabilities,
design, and problem space. Not a sales pitch. Not aspirational. Honest.

### Page-by-Page Plan

#### index.md

- Remove "Phase 2 — Authoring & CLI" status banner
- Add inline version marker (e.g., `v0.1.0 <!-- x-release-please-version -->`)
- Update core concept cards to reflect shipped capabilities
- Ensure nothing overpromises

#### problem.md

- Light review for accuracy
- Review found this compelling and accurate — minimal changes expected

#### how-it-works.md

- Verify pipeline diagram matches actual CLI/API flow
- Add execution stages (approved → in_progress → review → done) alongside authoring funnel
- Ensure the full spec lifecycle is represented, not just authoring

#### architecture.md

- Verify service list matches what `specgraph serve` actually registers
- Be honest about storage: Memgraph is the only implemented backend; Postgres is planned
- Verify code organization table matches current directory structure

#### ecosystem.md

- Gastown: clearly state it is designed but no code exists; SpecGraph functions independently
- Beads sync: clarify it is push-only (SpecGraph → Beads), not bidirectional
- MCP server: mark as planned, not in progress
- Linear adapter: mark as planned

#### concepts/authoring.md

- Connect funnel terminal (Approve) to execution stages (in_progress → review → done)
- Clarify posture auto-detection is a skill-layer feature (Claude Code plugin), not server-side
- Ensure the page doesn't imply the funnel is the entire lifecycle

#### concepts/passes.md

- Add clear note: pass scheduling infrastructure is implemented (posture-aware, stage-gated)
  but pass execution returns placeholders in 0.1.0
- Safety net (pattern-based scanning) is fully implemented and runs on every stage
- Real LLM-driven pass execution tracked for 0.2.0 (link to milestone beads)

#### concepts/specs.md

- Verify all proto field names still match `spec.proto` and `storage/spec_domain.go`
- Verify all edge type names match `graph.proto`
- Verify all CLI commands shown still exist
- Verify all internal links resolve

#### concepts/constitution.md

- Verify layer names and fields match `constitution_domain.go`
- Cross-link to constitution check pass in passes.md
- Verify all internal links resolve

#### concepts/decisions.md

- Verify decision schema matches `decision.proto`
- Verify edge directions match ADR-003
- Verify all internal links resolve

#### concepts/example-spec.md

- Verify all stage outputs map 1:1 to proto messages (`SparkOutput`, `ShapeOutput`,
  `SpecifyOutput`, `DecomposeOutput`)
- Verify decomposition strategy names match `DecompositionStrategy` enum
- This is the canonical example; proto changes require updates here per CLAUDE.md

#### concepts/index.md

- Verify card descriptions match current capabilities
- Verify all internal links resolve

### Pages to Remove

| Page | Reason |
|------|--------|
| `roadmap.md` | Internal project management; replaced by GitHub Issues/Projects |

### Pages to Add

| Page | Source |
|------|--------|
| `quickstart.md` | New (Workstream 1) |
| `changelog.md` | Symlink from repo-root `CHANGELOG.md` (auto-generated by release-please) |

---

## Configuration Changes

### zensical.toml Nav Update

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
    "concepts/example-spec.md",
  ] },
  "architecture.md",
  "ecosystem.md",
  "changelog.md",
]
```

### release-please-config.json

Add `extra-files` to the package config so version markers in docs are updated
automatically when release-please cuts a release PR:

```json
{
  "packages": {
    ".": {
      "release-type": "go",
      "extra-files": [
        { "type": "generic", "path": "site/docs/quickstart.md" },
        { "type": "generic", "path": "site/docs/index.md" }
      ]
    }
  }
}
```

### Changelog Symlink

```bash
ln -s ../../CHANGELOG.md site/docs/changelog.md
```

Note: `CHANGELOG.md` does not exist yet — release-please creates it on the first
release PR. The symlink can be created now (will be broken until first release).
If Zensical fails on a broken symlink, defer the nav entry until after the release.

---

## Risks

| Risk | Mitigation |
|------|------------|
| Zensical may not follow symlinks for changelog | Fall back to linking GitHub releases page |
| Health check example may feel too trivial | Keep it simple intentionally; link to OAuth2 example for depth |
| Claude Code plugin install process may change | Keep plugin install section brief, link to plugin README |
| Version markers may not render cleanly in all contexts | Test with Zensical build before committing |

## Success Criteria

1. Quick Start walkthrough completes end-to-end on a clean machine (verified by manual
   walkthrough: install, `init`, `up`, plugin install, author one spec, check drift)
2. Every page in site/docs/ accurately describes 0.1.0 capabilities (verified by: each
   page reviewed against codebase with specific checks listed in per-page plans above)
3. No page references unbuilt features without clearly marking them as planned (verified
   by: grep for `planned|future|upcoming|coming soon` confirms all occurrences are in
   clearly-marked callouts or notes)
4. Version markers use correct `x-release-please-version` (inline) or
   `x-release-please-start-version` / `x-release-please-end` (block) syntax and are
   present in `quickstart.md` and `index.md`
5. `release-please-config.json` includes `extra-files` entries for versioned docs
6. Changelog is accessible from site navigation (symlink or GitHub releases link)
7. `roadmap.md` is removed from site and nav
8. Zensical builds successfully with `cd site && uv run zensical build`
