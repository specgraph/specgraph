# Site Narrative Restructure — Design

**Date:** 2026-04-10
**Beads issue:** spgr-clz
**Scope:** Full narrative restructure of `site/docs/` for the enterprise tech lead audience

---

## Context

A documentation vs. code reality audit (spgr-clz) identified 19 items across correctness, comprehension, story alignment, and diagram quality. High-confidence factual fixes were already applied in that pass (amend re-entry behavior, passes callout, conversation recording, skill count, drift scope wording). This design addresses the remaining work: a full narrative restructure of the site around its primary audience and value proposition.

---

## Audience

**Primary reader:** Enterprise tech lead running a team where multiple engineers and AI agents work in parallel, needing shared architectural context, live dependency tracking, and governance at the spec layer.

**What they feel right now:** Isolated specs in static files can't coordinate parallel workers, can't enforce architectural constraints, and can't answer "what's the critical path?" File-based specs were adequate for human teams; they become a structural barrier as AI agents enter the picture.

**What they need to feel after 30 seconds on the landing page:** This is the infrastructure that solves that problem. It has a name — Spec-Driven Development — and SpecGraph is what it looks like at enterprise scale.

---

## Framing

**SDD is the practice. SpecGraph is the enterprise-scale implementation.**

Spec-Driven Development (SDD) is not jargon to be minimised — it is the practice this tool embodies. The site should define it once, clearly, and then use it confidently throughout. What changes is "reference implementation": SpecGraph *is* SDD at enterprise scale, not a reference to a category.

The hero headline: **"One ground truth. Every decision, every dependency, every engineer."**
Subhead: *"SpecGraph is Spec-Driven Development at enterprise scale — a live, queryable spec graph where architectural constraints are enforced, decisions are traceable, and every member of your team starts with the full picture. Human or AI, no one builds cold."*

---

## Information Architecture

### Nav order changes

| Position | Current | Proposed | Change |
|---|---|---|---|
| 3 | Quick Start | How It Works | Swapped — enterprise readers understand before they install |
| 4 | How It Works | Quick Start | Swapped |

### Concept section reordering

| Position | Current | Proposed | Rationale |
|---|---|---|---|
| 1 | Specs & the Graph | Ground Truth | Governance anchor — establishes what every engineer starts from |
| 2 | Constitution | The Spec Graph | Coordination layer — the queryable graph model |
| 3 | Authoring Funnel | Decisions | Promoted — first-class ADR governance resonates immediately with enterprise audience |
| 4 | Decisions | Authoring Funnel | |
| 5–9 | unchanged | unchanged | |

### Page renames

| Current file | New file | New H1 | Sidebar label |
|---|---|---|---|
| `concepts/constitution.md` | `concepts/ground-truth.md` | Ground Truth | Ground Truth |
| `concepts/specs.md` | `concepts/spec-graph.md` | The Spec Graph | The Spec Graph |

All internal links updated. Check whether Zensical supports redirect aliases; if so, add them for the old slugs to preserve any indexed or shared URLs.

---

## Landing Page (`index.md`)

### Hero
Replace current headline and subhead with:

> **One ground truth. Every decision, every dependency, every engineer.**
>
> SpecGraph is Spec-Driven Development at enterprise scale — a live, queryable spec graph where architectural constraints are enforced, decisions are traceable, and every member of your team starts with the full picture. Human or AI, no one builds cold.

### "Why" block
Expand from 2 sentences to a 4-sentence paragraph carrying the A+B story:

> AI coding teams produce code fast. The bottleneck has moved upstream — to specification, governance, and verification. Static specs in files can't coordinate parallel workers, can't enforce architectural constraints, and can't answer "what's the critical path?" Spec-Driven Development solves this. SpecGraph is SDD built for enterprise scale.

Then link to The Problem for the full evidence-backed case.

### Grid cards
Replace the current 4 cards with cards mapped to the SDD pillars:

| Card | Headline | Body |
|---|---|---|
| Ground Truth | "No engineer starts cold." | "Your tech stack, constraints, and architectural decisions — encoded once, inherited by every engineer and agent. Query before you build." |
| The Spec Graph | "Query your architecture." | "Specs are live graph nodes with typed edges. Find what's blocked, trace the critical path, detect drift — one command, not a grep script." |
| Authoring Funnel | "From rough idea to execution-ready spec." | "A five-stage AI-collaborative pipeline — Spark, Shape, Specify, Decompose, Approve. Human or agent, the funnel adds just enough structure at each step." |
| Architectural Governance | "Violations surface at the spec layer." | "Constitution checks, red-team passes, and drift detection catch problems before code review — or production." |

### "When to use" callout
Reframe with a positive target audience statement:

> SpecGraph is designed for enterprise teams where multiple engineers and AI agents need shared architectural context, live dependency tracking, and governance at the spec layer. For solo developers or small projects, simpler tools are a better fit.

### Project Status section
- Remove the duplicate changelog link (currently appears twice in the same paragraph)
- Add a confidence line: *"Core authoring, graph queries, ground truth, drift detection, and sync adapters are shipped. CLI and Claude Code plugin available now."*

### Bug fixes on this page
- Remove one of the two duplicate changelog links

---

## How It Works (`how-it-works.md`)

Restructured to carry the full SDD story. Section order and content:

### 1. Opening
Replace "four pillars" framing with explicit SDD framing:

> SDD has four layers. SpecGraph implements all of them.

Reorder the four sections to match the narrative: Ground Truth → The Spec Graph → Authoring → Execution.

### 2. Ground Truth (was: The Constitution)
- Rename heading from "The Constitution" to "Ground Truth"
- Keep existing layering explanation
- **Add:** A "What engineers and agents receive" subsection showing a concrete 10–15 line snippet of `specgraph constitution emit --format claude-md` stdout output. This is the "show me" moment — the concept doesn't fully land without seeing the actual artifact.

### 3. The Spec Graph (was: Specs as a Graph)
- Rename heading
- Keep existing graph model explanation
- **Add:** A "Live Queries" block — 4 concrete commands with realistic table output showing what you can ask that no static folder can answer:

```bash
# What's on the critical path to the checkout release?
specgraph critical-path checkout-flow

## Critical Path
| Slug            | Stage       |
|-----------------|-------------|
| auth-tokens     | in_progress |
| payment-service | approved    |
| checkout-flow   | approved    |

# What breaks if auth-tokens changes?
specgraph impact auth-tokens

## Impact
| Slug             | Stage    |
|------------------|----------|
| payment-service  | approved |
| session-mgmt     | approved |
| ...              | ...      |

# What's ready to claim right now?
specgraph ready

## Ready Specs
| Slug             | Stage    |
|------------------|----------|
| rate-limiter     | approved |
| audit-logging    | approved |
```

### 4. Authoring Funnel
- Keep largely as-is
- **Add** 2 sentences on posture: *"The funnel adapts to how your team works. Drive mode lets the AI lead and deliver a complete draft for review. Support mode keeps a senior engineer in control with AI filling gaps on request."*

### 5. Execution-Ready Output
- Reframe around governance-in-motion, not just "agents claim specs"
- **Add:** *"When upstream specs change, downstream dependencies surface as drift — reviewed and acknowledged before execution continues, not discovered in code review."*

### 6. "Putting It Together" diagram
- Clean subgraph labels (currently too dense — labels try to be both summary and explanation)
- Each subgraph gets a single clean label; detail stays in the surrounding prose
- **Add a feedback arrow** from Execution back toward Ground Truth, making the diagram a cycle. SDD is not a waterfall — decisions made during execution (drift acknowledgment, amendments, supersessions) feed back into the ground truth. The cycle makes this visible.

### 7. Closing "Where to go next"
Add a 3-link closing section (currently absent — page just ends after the diagram):
- The Problem — full evidence-backed case
- Quick Start — get running in under 10 minutes
- Ground Truth — the first concept to understand

---

## Concept Pages

### `concepts/ground-truth.md` (was `constitution.md`)
- Rename file, update H1 and sidebar label
- **Add** a "What engineers and agents receive" section with realistic `emit` output snippet (same content as How It Works, cross-referenced)
- All existing content preserved — layering model, merge semantics, import workflow, principles/constraints schema

### `concepts/spec-graph.md` (was `specs.md`)
- Rename file, update H1 and sidebar label
- **Add** the live queries block (same 4 commands from How It Works, cross-referenced to CLI cookbook for full recipe set)
- All existing content preserved — graph model, edge types, identity/ULID/content hash, drift detection intro

### `concepts/decisions.md`
- **Add** a governance-framed opener: *"In most teams, architectural decisions live in ADR files that drift from the specs they influenced. In SpecGraph, decisions are graph nodes with bidirectional edges — you can query every spec a decision shaped, and every decision a spec was built on."*
- All existing content preserved

### `concepts/index.md`
- Reorder grid cards to match new concept order: Ground Truth, The Spec Graph, Decisions, Authoring Funnel, Analytical Passes, Slices, Drift, Lifecycle, Linting, Example Spec
- **Fix** the Analytical Passes card description (currently describes linting, not passes): *"Red team, peripheral vision, consistency, and simplicity checks — posture-aware analysis that runs at each authoring stage."*
- Update card copy for Ground Truth, Spec Graph, Decisions to match new angles

---

## Problem Page (`problem.md`)

**Add a closing section** titled "What SDD does about this" — a table mapping each of the five gaps to its SpecGraph answer, followed by a CTA:

| Gap | SpecGraph answer |
|---|---|
| No Ground Truth | The constitution — layered architectural context every engineer and agent queries before building |
| No Governance | Constitution check and analytical passes enforce constraints at the spec layer, before code is written |
| No Addressability | Every spec has a stable ULID and slug — reorganise folders, rename files, references hold |
| No Execution Interface | Approved specs carry verify criteria, invariants, interface contracts, and typed dependencies — structured input for any executor |
| No Live Query | The spec graph answers `critical-path`, `impact`, `ready`, `drift` — direct traversal, not a grep script |

Closing line and CTA:
> This is Spec-Driven Development. SpecGraph is what it looks like at enterprise scale. → [How It Works]

---

## Quickstart (`quickstart.md`)

**Remove the stale "Note" callout** that warns Homebrew/binary/Docker require an unpublished release. v0.5.0 shipped April 4th; all three paths are live. The callout is the bug — not the install options.

No other changes to the quickstart. The health check example serves onboarding well as-is.

---

## Config Changes

- `zensical.toml` — update nav order (How It Works ↔ Quick Start swap) and concept section ordering to match the new IA. Update page references for renamed files (`constitution` → `ground-truth`, `specs` → `spec-graph`).

## Out of Scope

- Zensical framework changes (only config updates)
- Authoring Funnel concept page content (solid as-is)
- Slices, Drift, Lifecycle, Linting, Example Spec concept pages (no changes)
- Architecture page (no changes)
- Ecosystem page (no changes beyond skill count fix already applied)
- CLI Reference (auto-generated, no changes)

---

## Verification

After implementation:

1. `cd site && task build` — confirms site builds clean with no broken internal links
2. Read the landing page aloud — the hero + why block + cards should tell a coherent story in under 60 seconds
3. Navigate: Landing → How It Works → Ground Truth → The Spec Graph — the four-page journey should feel like one continuous argument
4. Confirm `specgraph constitution emit --format claude-md` output snippet is real (run the command, copy actual output)
5. Confirm the live queries table examples use realistic slug names consistent with the example-spec.md OAuth2 scenario used elsewhere in the docs
6. Check that all internal links resolve after the page renames (especially links from quickstart, how-it-works, and the concept index to `constitution.md` and `specs.md`)
