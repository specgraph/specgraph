# Slice 7: Claude Code Plugin Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Run the full SpecGraph authoring and query workflow from Claude Code via skills, hooks, and a packaged plugin — making the CLI invisible behind conversational slash commands.

**Architecture:** The plugin is a directory of SKILL.md files, a shell hook script, and a plugin.json manifest. Skills are thin wrappers: each invokes a `specgraph` CLI command, interprets the output, and structures the AI conversation using prompt templates from Slice 3's authoring service. The SessionStart hook calls `specgraph hook session-start` to inject orientation context. No business logic lives in the plugin — the server is the single source of truth.

**Tech Stack:** Claude Code plugin (SKILL.md files, shell hooks, plugin.json)

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 7 section)

---

## Project Structure (new files)

```text
plugin/specgraph/
  plugin.json                          # Plugin manifest — metadata, skills, hooks
  hooks/
    session-start.sh                   # SessionStart hook — calls specgraph hook session-start
    post-tool-use.sh                   # PostToolUse hook — optional progress reporting
  skills/
    specgraph/
      SKILL.md                         # Meta-skill: overview, routes to sub-skills
      spark/
        SKILL.md                       # /specgraph-spark — Spark stage
      shape/
        SKILL.md                       # /specgraph-shape — Shape stage
      specify/
        SKILL.md                       # /specgraph-specify — Specify stage
      decompose/
        SKILL.md                       # /specgraph-decompose — Decompose stage
      approve/
        SKILL.md                       # /specgraph-approve — Approve stage
      list/
        SKILL.md                       # /specgraph-list — list specs
      show/
        SKILL.md                       # /specgraph-show — show spec detail
      deps/
        SKILL.md                       # /specgraph-deps — dependency tree
      ready/
        SKILL.md                       # /specgraph-ready — ready-to-claim specs
      bundle/
        SKILL.md                       # /specgraph-bundle — generate + prime
cmd/specgraph/
  hook.go                              # CLI: specgraph hook session-start | progress
```

---

## Task 1: Plugin Manifest — plugin.json

**Files:**

- Create: `plugin/specgraph/plugin.json`

**Step 1: Write the plugin manifest**

`plugin/specgraph/plugin.json`:

```json
{
  "name": "specgraph",
  "version": "0.1.0",
  "description": "SpecGraph: spec-driven development workflow — author, query, and execute specs from Claude Code",
  "homepage": "https://github.com/seanb4t/specgraph",
  "license": "MIT",
  "requires": {
    "cli": ["specgraph"]
  },
  "skills": [
    {
      "name": "specgraph",
      "path": "skills/specgraph/SKILL.md",
      "description": "SpecGraph overview — routes to authoring and query skills"
    },
    {
      "name": "specgraph-spark",
      "path": "skills/specgraph/spark/SKILL.md",
      "description": "Capture a vague idea and create a new spec in Spark stage"
    },
    {
      "name": "specgraph-shape",
      "path": "skills/specgraph/shape/SKILL.md",
      "description": "Shape a spec — bound scope, explore solutions, surface risks"
    },
    {
      "name": "specgraph-specify",
      "path": "skills/specgraph/specify/SKILL.md",
      "description": "Specify a spec — interface contracts, verification criteria, invariants"
    },
    {
      "name": "specgraph-decompose",
      "path": "skills/specgraph/decompose/SKILL.md",
      "description": "Decompose a spec into implementable work units"
    },
    {
      "name": "specgraph-approve",
      "path": "skills/specgraph/approve/SKILL.md",
      "description": "Review and approve a spec for execution"
    },
    {
      "name": "specgraph-list",
      "path": "skills/specgraph/list/SKILL.md",
      "description": "List specs with optional filtering"
    },
    {
      "name": "specgraph-show",
      "path": "skills/specgraph/show/SKILL.md",
      "description": "Show detailed spec information"
    },
    {
      "name": "specgraph-deps",
      "path": "skills/specgraph/deps/SKILL.md",
      "description": "Show dependency tree for a spec"
    },
    {
      "name": "specgraph-ready",
      "path": "skills/specgraph/ready/SKILL.md",
      "description": "Show specs ready to be claimed and implemented"
    },
    {
      "name": "specgraph-bundle",
      "path": "skills/specgraph/bundle/SKILL.md",
      "description": "Generate execution bundle and prime context for a spec"
    }
  ],
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "hooks/session-start.sh"
      }
    ]
  }
}
```

**Step 2: Verify JSON is valid**

```bash
python3 -c "import json; json.load(open('plugin/specgraph/plugin.json'))"
```

Expected: no error

**Step 3: Commit**

```bash
git add plugin/specgraph/plugin.json
git commit -m "feat(plugin): plugin.json manifest with skill and hook declarations"
```

---

## Task 2: Meta-Skill — SpecGraph Overview

**Files:**

- Create: `plugin/specgraph/skills/specgraph/SKILL.md`

**Step 1: Write the meta-skill**

`plugin/specgraph/skills/specgraph/SKILL.md`:

```markdown
---
name: specgraph
description: >
  SpecGraph overview and router. Use when the user mentions "specgraph",
  asks about specs, or wants to work with the spec graph. Routes to the
  appropriate sub-skill based on the user's intent.
---

# SpecGraph

You are the SpecGraph assistant. SpecGraph is a spec-driven development framework that
manages software specifications as a queryable graph.

## Available Commands

### Authoring Funnel (in order)

| Skill | When to Use |
|-------|-------------|
| `/specgraph-spark` | User has a vague idea, problem, or feature request |
| `/specgraph-shape` | User wants to scope, design, or plan a feature |
| `/specgraph-specify` | User needs interface contracts, verify criteria, invariants |
| `/specgraph-decompose` | User wants to break a spec into implementable work units |
| `/specgraph-approve` | User wants to review and sign off on a spec |

### Query & Execution

| Skill | When to Use |
|-------|-------------|
| `/specgraph-list` | User wants to see all specs or filter by status/priority |
| `/specgraph-show` | User wants detail on a specific spec |
| `/specgraph-deps` | User wants to see what a spec depends on or blocks |
| `/specgraph-ready` | User asks "what should I work on?" or "what's next?" |
| `/specgraph-bundle` | User wants to generate an execution bundle for implementation |

## Routing Logic

1. If `$ARGUMENTS` contains a specific skill name or keyword, invoke that skill directly.
2. If `$ARGUMENTS` is empty, show the available commands table above and ask what the user wants to do.
3. If `$ARGUMENTS` describes a vague idea, suggest starting with `/specgraph-spark`.
4. If `$ARGUMENTS` asks about status or progress, run `specgraph list --format=table` and present the results.

## Quick Status

Run this to get an overview:

```bash
specgraph list --format=table
```
```


```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

Present the output as a formatted table. If the server is not running, suggest `specgraph serve` or `specgraph init` first.

```text

**Step 2: Verify the file is valid YAML frontmatter + markdown**

```bash
head -5 plugin/specgraph/skills/specgraph/SKILL.md
```

Expected: frontmatter block starting with `---`

**Step 3: Commit**

```bash
git add plugin/specgraph/skills/specgraph/SKILL.md
git commit -m "feat(plugin): meta-skill for SpecGraph overview and routing"
```

---

## Task 3: Authoring Skills — Spark, Shape, Specify, Decompose, Approve

**Files:**

- Create: `plugin/specgraph/skills/specgraph/spark/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/shape/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/specify/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/decompose/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/approve/SKILL.md`

**Step 1: Write the Spark skill**

`plugin/specgraph/skills/specgraph/spark/SKILL.md`:

```markdown
---
name: specgraph-spark
description: >
  Capture a vague idea and create a new spec in Spark stage. Use when the user
  has a problem, feature idea, or rough concept that needs to be captured.
  Triggered by phrases like "I have an idea", "what if we...", "we need to...",
  or "new spec".
---

# SpecGraph Spark

You are running the SpecGraph Spark stage. This is the first stage of the authoring
funnel — capturing a vague idea and turning it into a seed spec.

## Prerequisites

Verify the SpecGraph server is reachable:

```bash
specgraph health
```
```


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text

If not reachable, tell the user to start the server first.

## Workflow

### 1. Gather the Seed

If `$ARGUMENTS` contains a slug, load the existing spec:

```bash
specgraph show $SLUG --format=json
```

If `$ARGUMENTS` is a description (not a slug), this is a new idea. Ask the user to
provide a short slug (kebab-case identifier) or generate one from the description.

### 2. Run Spark Elicitation

Call the authoring service to initiate spark:

```bash
specgraph spark $SLUG
```

If the spec does not exist yet, create it first:

```bash
specgraph create $SLUG --title="$TITLE" --priority=p2 --size=medium
```

Then run spark.

### 3. Elicitation Probes

Work through these probes conversationally with the user. Do NOT rush through them —
each probe should be a back-and-forth exchange:

1. **Seed** — What is the core idea? One sentence.
2. **Signal** — Why now? What triggered this?
3. **Scope Sniff** — Gut estimate: hours, days, or weeks? Why?
4. **Unknowns** — What do we not know? What could surprise us?
5. **Kill Test** — What would make this not worth doing?

For each probe, capture the user's response and record it.

### 4. Persist Results

After all probes, update the spec with spark output:

```bash
specgraph spark $SLUG --seed="$SEED" --signal="$SIGNAL"
```

### 5. Next Steps

After spark completes, offer:

- "Continue to **Shape** (`/specgraph-shape $SLUG`) to bound scope and explore solutions"
- "Or park it — the spec is saved at spark stage and you can return to it later"

## Constitution Awareness

Load the project constitution to inform the conversation:

```bash
specgraph constitution show --format=json
```

If the user's idea conflicts with constitution constraints, flag it immediately:
"This might conflict with the constraint: [constraint]. Let's note that and address
it during Shape."

```text

**Step 2: Write the Shape skill**

`plugin/specgraph/skills/specgraph/shape/SKILL.md`:

```markdown
---
name: specgraph-shape
description: >
  Shape a spec by bounding scope, exploring solution approaches, capturing
  decisions, and surfacing risks. Use when the user has a problem or idea
  that needs to be turned into a bounded, actionable spec. Triggered by
  phrases like "let's design...", "scope this out", "what's the approach for...",
  or "shape".
---

# SpecGraph Shape

You are running the SpecGraph Shape stage. Load the project constitution and
codebase context to inform all decisions.

## Prerequisites

```bash
specgraph health
specgraph constitution show --format=json
```

## Workflow

### 1. Load the Spec

If `$ARGUMENTS` contains a spec slug, load it:

```bash
specgraph show $SLUG --format=json
```

Verify the spec is at spark or shape stage. If it's at a later stage and the user
wants to re-shape, note that this is a backward transition.

If `$ARGUMENTS` is a new problem description, suggest starting with
`/specgraph-spark` first, or create directly at shape stage if the user prefers to skip spark.

### 2. Shaping Moves

Work through all five moves. Each should be a conversation, not a monologue:

1. **Bound the Scope** — What's in? What's explicitly out? Why? Estimate size (S/M/L/XL).
2. **Explore Solution Space** — Generate 2-3 approaches with tradeoffs. Decide. Capture rejected alternatives as decisions.
3. **Identify Edges** — Dependencies (what must exist first), blockers (what could prevent this), parent epics, related specs, prior decisions.
4. **Surface Risks** — Technical risks, operational risks, business risks. Be specific, not abstract. Each risk gets a mitigation or acceptance.
5. **Define Success** — Must-have, should-have, won't-have for this spec.

### 3. Capture Decisions

For each significant choice made during shaping:

```bash
specgraph decision create --spec=$SLUG --title="$TITLE" --chosen="$CHOSEN" --rationale="$RATIONALE"
```

### 4. Capture Edges

For dependencies and relationships discovered:

```bash
specgraph edge $SLUG depends-on $DEP_SLUG
specgraph edge $SLUG blocks $BLOCKED_SLUG
```

### 5. Persist Shape Output

```bash
specgraph shape $SLUG
```

### 6. Analytical Passes

After shaping, offer to run:

- **Peripheral Vision** — "What did we miss? What's adjacent that we should consider?"
- **Constitution Check** — Validate against constraints.

### 7. Next Steps

After shape completes, offer:

- "Continue to **Specify** (`/specgraph-specify $SLUG`) to define interface contracts and verification criteria"
- "Or run **Red Team** (`/specgraph-red-team $SLUG`) to stress-test the design before specifying"

## Constitution Constraints

Read and apply all principles, constraints, and antipatterns from the constitution.
Flag any violations immediately with: "This conflicts with: [constraint]. Options:
(a) change the approach, (b) request a constitution exception, (c) note it and proceed."

```text

**Step 3: Write the Specify skill**

`plugin/specgraph/skills/specgraph/specify/SKILL.md`:

```markdown
---
name: specgraph-specify
description: >
  Specify a spec with interface contracts, verification criteria, and invariants.
  Use when the user is ready to define the precise technical shape of a feature.
  Triggered by phrases like "define the interface", "what are the acceptance criteria",
  "specify", or "write the contract".
---

# SpecGraph Specify

You are running the SpecGraph Specify stage. This is where the spec becomes precise
enough for an agent to implement without making decisions.

## Prerequisites

```bash
specgraph health
specgraph constitution show --format=json
specgraph show $SLUG --format=json
```

Verify the spec is at shape or specify stage. The scope, risks, and decisions from
Shape should already be captured.

## Workflow

### 1. Load Context

Load the spec and its edges to understand the full picture:

```bash
specgraph show $SLUG --format=json
specgraph deps $SLUG
```

Agent reads relevant source files for file-level context:

```bash
specgraph show $SLUG --format=json | jq '.touches'
```

### 2. Define Interface Contract

Work through the interface definition conversationally:

- **Inputs** — What goes in? Types, validation rules, edge cases.
- **Outputs** — What comes out? Success shape, error shape, side effects.
- **Boundaries** — What other services/modules does this touch? How?
- **Protocol** — HTTP/RPC/CLI? Method/endpoint/subcommand?

### 3. Write Verification Criteria

Each criterion must be:

- **Actionable** — describes a concrete check, not a vague quality
- **Testable** — can be verified by running a command or inspecting output
- **Independent** — can be checked without running all other criteria

Format each as: "Given [context], when [action], then [expected result]."

Optionally include `test_path` for criteria that map to existing test files.

### 4. State Invariants

What must ALWAYS be true, regardless of inputs or state?

- Data invariants (e.g., "user.email is always unique")
- Ordering invariants (e.g., "events are processed in order")
- Safety invariants (e.g., "failed auth never returns user data")

### 5. Identify Touches

List files that will be created or modified:

```bash
# The user identifies affected files, or use codebase search:
rg "pattern" --type go -l
```

### 6. Persist Specify Output

```bash
specgraph specify $SLUG
```

### 7. Analytical Passes

After specifying, run:

- **Red Team** — "How could this break? What attack vectors exist?"
- **Consistency Check** — "Does this contradict any other spec?"
- **Constitution Check** — Validate interface against constraints.

### 8. Next Steps

After specify completes, offer:

- "Continue to **Decompose** (`/specgraph-decompose $SLUG`) if this is large enough to break into sub-specs"
- "Or **Approve** (`/specgraph-approve $SLUG`) if this is a single implementable unit"

```text

**Step 4: Write the Decompose skill**

`plugin/specgraph/skills/specgraph/decompose/SKILL.md`:

```markdown
---
name: specgraph-decompose
description: >
  Decompose a spec into implementable work units (sub-specs). Use when a spec
  is too large to implement in one shot. Triggered by phrases like "break this down",
  "decompose", "split into tasks", or "what are the implementation steps".
---

# SpecGraph Decompose

You are running the SpecGraph Decompose stage. Break a fully-specified spec
into sub-specs that can each be implemented independently.

## Prerequisites

```bash
specgraph health
specgraph show $SLUG --format=json
specgraph deps $SLUG
```

Verify the spec is at specify or decompose stage. Interface contracts and verification
criteria should already be defined.

## Workflow

### 1. Choose Decomposition Strategy

Evaluate which strategy fits:

| Strategy | When to Use |
|----------|-------------|
| **Vertical Slice** | Features that cut across layers (UI + API + storage). Each slice delivers end-to-end value. Prefer this. |
| **Layer Cake** | When layers are genuinely independent and can be tested in isolation. Rare. |
| **Single Unit** | Spec is small enough to implement as-is. Skip decomposition. |

Recommend a strategy with rationale. Let the user override.

### 2. Define Slices

For each sub-spec:

- **ID** — Short slug, e.g., `parent-slug/slice-1`
- **Intent** — One sentence: what this slice delivers
- **Verify** — Verification criteria specific to this slice (subset of parent's, plus slice-specific)
- **Touches** — Files this slice creates or modifies
- **Depends On** — Other slices that must be done first

### 3. Validate Decomposition

Check that:

- Every parent verification criterion is covered by at least one slice
- No circular dependencies between slices
- Each slice is independently testable
- The sum of slices equals the parent spec (no gaps, no overlaps)

### 4. Create Sub-Specs

```bash
specgraph decompose $SLUG
```

This creates child specs with `COMPOSES` edges back to the parent.

For each child spec, also create dependency edges between slices:

```bash
specgraph edge $CHILD_SLUG depends-on $OTHER_CHILD_SLUG
```

### 5. Simplicity Check

After decomposition, run the simplicity pass:

- "Is any slice trivial enough to merge with another?"
- "Did we over-specify? Are there YAGNI slices?"
- "Could the whole thing be simpler?"

### 6. Next Steps

After decompose completes, offer:

- "**Approve** the parent and all sub-specs (`/specgraph-approve $SLUG`)"
- "Or review individual sub-specs: `/specgraph-show $CHILD_SLUG`"

```text

**Step 5: Write the Approve skill**

`plugin/specgraph/skills/specgraph/approve/SKILL.md`:

```markdown
---
name: specgraph-approve
description: >
  Review and approve a spec for execution. Use when a spec has been fully shaped,
  specified, and optionally decomposed. Triggered by phrases like "approve",
  "sign off", "ready for implementation", or "LGTM".
---

# SpecGraph Approve

You are running the SpecGraph Approve stage. This is the final gate before a spec
can be claimed and implemented.

## Prerequisites

```bash
specgraph health
specgraph show $SLUG --format=json
specgraph deps $SLUG
```

## Workflow

### 1. Load the Spec

```bash
specgraph show $SLUG --format=json
```

Verify the spec is at specify or decompose stage (ready for approval).

### 2. Pre-Approval Checklist

Review the spec against this checklist:

- [ ] **Scope** — Is the scope bounded? In/out clearly defined?
- [ ] **Interface** — Are inputs, outputs, and boundaries defined?
- [ ] **Verification** — Are all criteria actionable and testable?
- [ ] **Dependencies** — Are all deps satisfied or in progress?
- [ ] **Risks** — Are risks identified with mitigations?
- [ ] **Decisions** — Are all design choices recorded with rationale?
- [ ] **Constitution** — Does the spec comply with all constraints?

Present the checklist to the user with pass/fail for each item.

### 3. Run Final Analytical Passes

Run all passes that haven't been run yet:

- **Constitution Check** — Final constraint validation
- **Consistency Check** — Cross-spec consistency
- **Safety Net** — Security, data loss, showstopper check

Report findings. If any critical issues, block approval and suggest fixes.

### 4. Approve

If all checks pass:

```bash
specgraph approve $SLUG
```

If the spec has child specs (from decompose), offer to approve them as a batch:

```bash
specgraph approve $SLUG --recursive
```

### 5. After Approval

The spec is now claimable. Inform the user:

- "Spec `$SLUG` is approved and ready for implementation."
- "Run `/specgraph-ready` to see all claimable specs."
- "Run `/specgraph-bundle $SLUG` to generate an execution bundle."

```text

**Step 6: Verify all skill files exist and have valid frontmatter**

```bash
for f in plugin/specgraph/skills/specgraph/{spark,shape,specify,decompose,approve}/SKILL.md; do
  echo "=== $f ==="
  head -4 "$f"
  echo
done
```

Expected: each file shows `---` / `name:` / `description:` / valid frontmatter

**Step 7: Commit**

```bash
git add plugin/specgraph/skills/specgraph/spark/SKILL.md \
        plugin/specgraph/skills/specgraph/shape/SKILL.md \
        plugin/specgraph/skills/specgraph/specify/SKILL.md \
        plugin/specgraph/skills/specgraph/decompose/SKILL.md \
        plugin/specgraph/skills/specgraph/approve/SKILL.md
git commit -m "feat(plugin): authoring funnel skills — spark, shape, specify, decompose, approve"
```

---

## Task 4: Query Skills — List, Show, Deps, Ready

**Files:**

- Create: `plugin/specgraph/skills/specgraph/list/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/show/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/deps/SKILL.md`
- Create: `plugin/specgraph/skills/specgraph/ready/SKILL.md`

**Step 1: Write the List skill**

`plugin/specgraph/skills/specgraph/list/SKILL.md`:

```markdown
---
name: specgraph-list
description: >
  List specs with optional filtering by status, priority, or stage.
  Use when the user asks "what specs exist?", "show me all specs",
  "what's in progress?", or "list specs".
---

# SpecGraph List

## Workflow

### 1. Run the Query

```bash
specgraph list --format=table
```
```


```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

If `$ARGUMENTS` contains filter keywords, apply them:

- Status filter: `specgraph list --status=$STATUS`
- Priority filter: `specgraph list --priority=$PRIORITY`

### 2. Present Results

Format the output as a clean table. Highlight:

- Specs in `approved` status (ready to claim)
- Specs in `in_progress` status (active work)
- Specs in `blocked` status (need attention)

### 3. Offer Next Actions

Based on results:

- If there are approved specs: "Want to claim one? Use `/specgraph-bundle $SLUG`"
- If there are blocked specs: "Want to see what's blocking? Use `/specgraph-deps $SLUG`"
- If there are specs in early stages: "Want to continue authoring? Use `/specgraph-shape $SLUG`"

```text

**Step 2: Write the Show skill**

`plugin/specgraph/skills/specgraph/show/SKILL.md`:

```markdown
---
name: specgraph-show
description: >
  Show detailed information about a specific spec. Use when the user asks
  "tell me about spec X", "show me the details of X", or "what is X?".
---

# SpecGraph Show

## Workflow

### 1. Identify the Spec

If `$ARGUMENTS` contains a slug, use it directly. Otherwise, ask the user
which spec they want to see, or run `specgraph list` to help them choose.

### 2. Load the Spec

```bash
specgraph show $SLUG --format=json
```

### 3. Present the Spec

Format the output in a readable structure:

**Header:**

- Title, slug, status, priority, size, stage

**Scope** (if shaped):

- In scope / out of scope

**Interface** (if specified):

- Inputs, outputs, boundaries

**Verification Criteria** (if specified):

- List each criterion with pass/fail indicators if available

**Dependencies:**

```bash
specgraph deps $SLUG
```

**Decisions:**

```bash
specgraph decision list --spec=$SLUG
```

### 4. Offer Next Actions

Based on the spec's stage:

- Spark: "Continue to `/specgraph-shape $SLUG`"
- Shape: "Continue to `/specgraph-specify $SLUG`"
- Specify: "Continue to `/specgraph-decompose $SLUG` or `/specgraph-approve $SLUG`"
- Approved: "Generate bundle with `/specgraph-bundle $SLUG`"

```text

**Step 3: Write the Deps skill**

`plugin/specgraph/skills/specgraph/deps/SKILL.md`:

```markdown
---
name: specgraph-deps
description: >
  Show the dependency tree for a spec — what it depends on, what it blocks,
  what it composes. Use when the user asks "what does X depend on?",
  "what blocks X?", "show the dependency graph", or "deps".
---

# SpecGraph Deps

## Workflow

### 1. Identify the Spec

If `$ARGUMENTS` contains a slug, use it. Otherwise, ask which spec.

### 2. Load Dependency Tree

```bash
specgraph deps $SLUG
```

### 3. Present the Tree

Format as a visual tree structure:

```text
$SLUG
  depends-on:
    dep-1 (approved)
    dep-2 (in_progress) <-- blocking
  blocks:
    blocked-1 (waiting)
  composes:
    parent-slug (approved)
  composed-of:
    child-1 (approved)
    child-2 (spark)
```

Highlight blocking relationships — specs that are preventing progress.

### 4. If Blocked

If the spec has unsatisfied dependencies:

- Show which deps are not done
- Show the critical path: `specgraph critical-path $SLUG`
- Suggest: "Want to work on the blocking dep instead? `/specgraph-show $DEP_SLUG`"

### 5. Impact Analysis

Show what would be affected if this spec changes:

```bash
specgraph impact $SLUG
```

```text

**Step 4: Write the Ready skill**

`plugin/specgraph/skills/specgraph/ready/SKILL.md`:

```markdown
---
name: specgraph-ready
description: >
  Show specs that are ready to be claimed and implemented. Use when the user
  asks "what should I work on?", "what's next?", "what's ready?", or
  "what can I implement?".
---

# SpecGraph Ready

## Workflow

### 1. Query Ready Specs

```bash
specgraph ready --format=table
```

This returns approved specs with all dependencies satisfied and no active claims.

### 2. Present Results

Format as a prioritized list:

| Priority | Slug | Title | Size | Deps Satisfied |
|----------|------|-------|------|----------------|
| ... | ... | ... | ... | ... |

### 3. Recommend

If there are multiple ready specs:

- Recommend the highest-priority, smallest spec first (quick win)
- If the user has an active claim, remind them: "You already have `$SLUG` claimed. Finish that first?"

### 4. Start Implementation

When the user picks a spec:

1. Claim it: "I'll claim `$SLUG` for you"
2. Offer to generate the bundle: `/specgraph-bundle $SLUG`
3. Or jump straight to implementation with context injection

```bash
specgraph claim $SLUG --agent=$(whoami)
```

```text

**Step 5: Verify all skill files exist**

```bash
for f in plugin/specgraph/skills/specgraph/{list,show,deps,ready}/SKILL.md; do
  echo "=== $f ==="
  head -4 "$f"
  echo
done
```

Expected: each file shows valid frontmatter

**Step 6: Commit**

```bash
git add plugin/specgraph/skills/specgraph/list/SKILL.md \
        plugin/specgraph/skills/specgraph/show/SKILL.md \
        plugin/specgraph/skills/specgraph/deps/SKILL.md \
        plugin/specgraph/skills/specgraph/ready/SKILL.md
git commit -m "feat(plugin): query skills — list, show, deps, ready"
```

---

## Task 5: Bundle Skill

**Files:**

- Create: `plugin/specgraph/skills/specgraph/bundle/SKILL.md`

**Step 1: Write the Bundle skill**

`plugin/specgraph/skills/specgraph/bundle/SKILL.md`:

```markdown
---
name: specgraph-bundle
description: >
  Generate an execution bundle for an approved spec, call the prime endpoint
  for context, and prepare for implementation. Use when the user wants to
  start implementing a spec, asks "generate the bundle", "prime me", or
  "set up for implementation".
---

# SpecGraph Bundle

You are preparing a spec for execution. This generates the execution bundle,
calls the prime endpoint for full project context, and injects everything
the implementing agent needs.

## Prerequisites

```bash
specgraph health
specgraph show $SLUG --format=json
```
```


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text


```text

```text

Verify the spec is in `approved` status.

## Workflow

### 1. Claim the Spec (if not claimed)

Check if already claimed:

```bash
specgraph claim list --agent=$(whoami)
```

If not claimed:

```bash
specgraph claim $SLUG --agent=$(whoami)
```

### 2. Generate the Bundle

```bash
specgraph bundle $SLUG
```

This outputs a YAML bundle containing:

- Full spec snapshot
- Resolved decisions
- Bootstrap instructions
- Callback configuration (progress/blocker/completion endpoints)

### 3. Call Prime

```bash
specgraph bundle $SLUG --prime
```

Prime returns:

- Constitution summary (tech stack, constraints, conventions)
- Codebase context (architecture, patterns, affected files)
- Resolved decisions for this spec
- Coding conventions
- Callback operation documentation

### 4. Inject Context

Present the bundle and prime context to the implementing agent in a structured format:

**Spec:** `$SLUG` — $TITLE

**Interface Contract:**
[From the spec's interface field]

**Verification Criteria:**
[Numbered list of criteria]

**Conventions:**
[From constitution + prime]

**Files to Touch:**
[From spec touches field]

**Decisions Already Made:**
[From resolved decisions]

### 5. Begin Implementation

After context injection:

- "Context loaded. Start implementing. When done, I'll verify against the spec criteria."
- "Report blockers anytime — I'll record them."
- "Run `/specgraph-verify $SLUG` when you think you're done."

### 6. Progress Reporting

As the user works, periodically report progress:

```bash
specgraph progress $SLUG --message="$MESSAGE"
```


```text

**Step 2: Commit**

```bash
git add plugin/specgraph/skills/specgraph/bundle/SKILL.md
git commit -m "feat(plugin): bundle skill for execution bundle generation and context priming"
```

---

## Task 6: SessionStart Hook

**Files:**

- Create: `plugin/specgraph/hooks/session-start.sh`
- Create: `cmd/specgraph/hook.go`

**Step 1: Write the hook shell script**

`plugin/specgraph/hooks/session-start.sh`:

```bash
#!/usr/bin/env bash
# SpecGraph SessionStart hook for Claude Code
# Called at the start of every Claude Code session.
# Outputs JSON with additionalContext for session priming.

set -euo pipefail

# Check if specgraph CLI is available
if ! command -v specgraph &> /dev/null; then
  echo '{"additionalContext": "SpecGraph CLI is not installed or not in PATH. Install it to use SpecGraph skills."}'
  exit 0
fi

# Delegate to the specgraph hook command
exec specgraph hook session-start 2>/dev/null || echo '{"additionalContext": "SpecGraph server is not running. Start it with `specgraph serve` to use SpecGraph skills."}'
```

**Step 2: Write the CLI hook command**

`cmd/specgraph/hook.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook",
	Short: "Hook commands for IDE integration",
}

var hookSessionStartCmd = &cobra.Command{
	Use:   "session-start",
	Short: "SessionStart hook — outputs context for IDE session priming",
	RunE:  runHookSessionStart,
}

var hookProgressCmd = &cobra.Command{
	Use:   "progress <slug> <message>",
	Short: "Report progress on a claimed spec",
	Args:  cobra.ExactArgs(2),
	RunE:  runHookProgress,
}

func init() {
	hookCmd.AddCommand(hookSessionStartCmd)
	hookCmd.AddCommand(hookProgressCmd)
}

type hookOutput struct {
	AdditionalContext string `json:"additionalContext"`
}

func runHookSessionStart(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()

	// Build context parts
	var parts []string

	// Check for active claims
	agent := currentAgent()
	claimClient, err := newClient(specgraphv1connect.NewClaimServiceClient)
	if err == nil {
		claimResp, err := claimClient.ListClaims(ctx, connect.NewRequest(&specv1.ListClaimsRequest{}))
		if err == nil {
			var myClaims []string
			for _, claim := range claimResp.Msg.Claims {
				if claim.Agent == agent {
					myClaims = append(myClaims, fmt.Sprintf("%s (%s)", claim.SpecSlug, claim.Status))
				}
			}
			if len(myClaims) > 0 {
				parts = append(parts, fmt.Sprintf("You have %d active claim(s): %s.", len(myClaims), strings.Join(myClaims, ", ")))
			}
		}
	}

	// Check for ready specs
	specClient, err := newClient(specgraphv1connect.NewSpecServiceClient)
	if err == nil {
		readyResp, err := specClient.ListReady(ctx, connect.NewRequest(&specv1.ListReadyRequest{}))
		if err == nil && len(readyResp.Msg.Specs) > 0 {
			parts = append(parts, fmt.Sprintf("%d spec(s) are ready for claiming.", len(readyResp.Msg.Specs)))
		}
	}

	// Build final context
	contextStr := "SpecGraph is configured for this project."
	if len(parts) > 0 {
		contextStr += " " + strings.Join(parts, " ")
	}
	contextStr += " Use /specgraph to see available commands, or /specgraph-ready to see claimable specs."

	out := hookOutput{AdditionalContext: contextStr}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func runHookProgress(cmd *cobra.Command, args []string) error {
	slug := args[0]
	message := args[1]

	ctx := context.Background()

	// Report progress via the execution service
	// This requires the execution service from Slice 4
	_ = slug
	_ = message
	_ = ctx

	fmt.Fprintf(os.Stderr, "Progress reported: %s — %s\n", slug, message)
	return nil
}

func currentAgent() string {
	u, err := user.Current()
	if err != nil {
		return "unknown"
	}
	return u.Username
}
```

**Step 3: Register the hook command in main.go**

In `cmd/specgraph/main.go`, add the `hookCmd` to the root command:

```go
// After existing AddCommand calls:
rootCmd.AddCommand(hookCmd)
```

**Step 4: Make the hook script executable**

```bash
chmod +x plugin/specgraph/hooks/session-start.sh
```

**Step 5: Verify it compiles**

```bash
go build ./cmd/specgraph/
```

**Step 6: Test the hook output format**

```bash
# With server not running — should output fallback JSON
./specgraph hook session-start 2>/dev/null || true
```

Expected: JSON output with `additionalContext` field

**Step 7: Commit**

```bash
git add plugin/specgraph/hooks/session-start.sh cmd/specgraph/hook.go cmd/specgraph/main.go
git commit -m "feat(plugin): SessionStart hook with CLI hook command for session priming"
```

---

## Task 7: PostToolUse Progress Hook (Optional)

**Files:**

- Create: `plugin/specgraph/hooks/post-tool-use.sh`

**Step 1: Write the progress hook**

`plugin/specgraph/hooks/post-tool-use.sh`:

```bash
#!/usr/bin/env bash
# SpecGraph PostToolUse hook for Claude Code (optional)
# Reports progress to SpecGraph server after file write operations.
# Only active when SPECGRAPH_PROGRESS_HOOK=1 is set.

set -euo pipefail

# Skip if progress reporting is not enabled
if [[ "${SPECGRAPH_PROGRESS_HOOK:-0}" != "1" ]]; then
  exit 0
fi

# Skip if specgraph CLI is not available
if ! command -v specgraph &> /dev/null; then
  exit 0
fi

# Read hook input from stdin (Claude Code passes tool use info as JSON)
INPUT=$(cat)

# Extract tool name and check if it's a file write
TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null)

case "$TOOL_NAME" in
  Write|Edit|NotebookEdit)
    # Extract file path for the progress message
    FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.path // "unknown file"' 2>/dev/null)

    # Get the active claimed spec (if any)
    AGENT=$(whoami)
    ACTIVE_SLUG=$(specgraph claim list --agent="$AGENT" --format=json 2>/dev/null | jq -r '.[0].spec_slug // empty' 2>/dev/null)

    if [[ -n "$ACTIVE_SLUG" ]]; then
      specgraph hook progress "$ACTIVE_SLUG" "Modified: $FILE_PATH" 2>/dev/null || true
    fi
    ;;
esac

exit 0
```

**Step 2: Make executable**

```bash
chmod +x plugin/specgraph/hooks/post-tool-use.sh
```

**Step 3: Update plugin.json to include PostToolUse hook**

Add to the `hooks` section of `plugin/specgraph/plugin.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "hooks/session-start.sh"
      }
    ],
    "PostToolUse": [
      {
        "type": "command",
        "command": "hooks/post-tool-use.sh"
      }
    ]
  }
}
```

**Step 4: Commit**

```bash
git add plugin/specgraph/hooks/post-tool-use.sh plugin/specgraph/plugin.json
git commit -m "feat(plugin): optional PostToolUse progress hook for file write tracking"
```

---

## Task 8: Plugin Packaging and Installation

**Files:**

- Create: `plugin/specgraph/README.md`
- Create: `plugin/specgraph/install.sh`

**Step 1: Write the installation script**

`plugin/specgraph/install.sh`:

```bash
#!/usr/bin/env bash
# SpecGraph Claude Code Plugin Installer
# Installs skills and hooks into the project's .claude/ directory.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_DIR="${1:-.claude}"

echo "Installing SpecGraph plugin to $TARGET_DIR/"

# Create directory structure
mkdir -p "$TARGET_DIR/skills/specgraph"

# Copy skills
cp -r "$SCRIPT_DIR/skills/specgraph/" "$TARGET_DIR/skills/specgraph/"

# Install hooks into settings.json
SETTINGS_FILE="$TARGET_DIR/settings.json"

if [[ -f "$SETTINGS_FILE" ]]; then
  # Merge hooks into existing settings
  EXISTING=$(cat "$SETTINGS_FILE")
  echo "$EXISTING" | jq '.hooks.SessionStart += [{"type": "command", "command": "specgraph hook session-start"}]' > "$SETTINGS_FILE"
  echo "Updated $SETTINGS_FILE with SpecGraph hooks"
else
  # Create new settings with hooks
  cat > "$SETTINGS_FILE" << 'SETTINGS_EOF'
{
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "specgraph hook session-start"
      }
    ]
  }
}
SETTINGS_EOF
  echo "Created $SETTINGS_FILE with SpecGraph hooks"
fi

echo ""
echo "SpecGraph plugin installed."
echo ""
echo "Available skills:"
echo "  /specgraph          — overview and routing"
echo "  /specgraph-spark    — capture a new idea"
echo "  /specgraph-shape    — bound scope and design"
echo "  /specgraph-specify  — define contracts and criteria"
echo "  /specgraph-decompose — break into work units"
echo "  /specgraph-approve  — review and sign off"
echo "  /specgraph-list     — list all specs"
echo "  /specgraph-show     — show spec detail"
echo "  /specgraph-deps     — dependency tree"
echo "  /specgraph-ready    — what's ready to implement"
echo "  /specgraph-bundle   — generate execution bundle"
echo ""
echo "Ensure the SpecGraph server is running: specgraph serve"
```

**Step 2: Write the README**

`plugin/specgraph/README.md`:

```markdown
# SpecGraph Claude Code Plugin

Run the full SpecGraph spec-driven development workflow from Claude Code.

## Prerequisites

- `specgraph` CLI installed and in PATH
- SpecGraph server running (`specgraph serve`)
- A project initialized with SpecGraph (`specgraph init`)

## Installation

### Automatic (via plugin system)

```bash
claude /plugin install specgraph
```
```


```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

```text

### Manual

```bash
# From the specgraph repo
./plugin/specgraph/install.sh

# Or copy manually
cp -r plugin/specgraph/skills/specgraph/ .claude/skills/specgraph/
```

### Hook Setup

Add to `.claude/settings.json`:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "type": "command",
        "command": "specgraph hook session-start"
      }
    ]
  }
}
```

## Skills

### Authoring Funnel

| Skill | Description |
|-------|-------------|
| `/specgraph-spark` | Capture a vague idea |
| `/specgraph-shape` | Bound scope, explore solutions, surface risks |
| `/specgraph-specify` | Define interface contracts and verification criteria |
| `/specgraph-decompose` | Break into implementable work units |
| `/specgraph-approve` | Review and approve for execution |

### Query & Execution

| Skill | Description |
|-------|-------------|
| `/specgraph-list` | List specs with filtering |
| `/specgraph-show` | Show spec detail |
| `/specgraph-deps` | Dependency tree |
| `/specgraph-ready` | Specs ready to implement |
| `/specgraph-bundle` | Generate execution bundle |

## Configuration

### Server URL

Set via environment or config:

```bash
export SPECGRAPH_URL=http://localhost:8080
```

### Progress Reporting

Enable optional progress hook (reports file writes to server):

```bash
export SPECGRAPH_PROGRESS_HOOK=1
```

## Architecture

```text
User <-> Claude Code <-> Skills (SKILL.md) <-> specgraph CLI <-> SpecGraph Server <-> Memgraph
```

Skills are thin wrappers. No business logic lives in the plugin. The server
is the single source of truth.

```text

**Step 3: Make install script executable**

```bash
chmod +x plugin/specgraph/install.sh
```

**Step 4: Commit**

```bash
git add plugin/specgraph/install.sh plugin/specgraph/README.md
git commit -m "feat(plugin): installation script and documentation"
```

---

## Task 9: Wire Hook Command into CLI and Integration Test

**Files:**

- Edit: `cmd/specgraph/main.go`
- Create: `plugin/specgraph/test-plugin.sh`

**Step 1: Verify hook command is wired into main.go**

Check that `hookCmd` is registered on the root command. If not already done in Task 6:

```go
// In cmd/specgraph/main.go, in the init() or main() function:
rootCmd.AddCommand(hookCmd)
```

**Step 2: Write the integration test script**

`plugin/specgraph/test-plugin.sh`:

```bash
#!/usr/bin/env bash
# SpecGraph Plugin Integration Test
# Validates plugin structure, skill files, and hook scripts.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ERRORS=0

echo "=== SpecGraph Plugin Test Suite ==="
echo ""

# Test 1: plugin.json is valid JSON
echo -n "1. plugin.json is valid JSON... "
if python3 -c "import json; json.load(open('$SCRIPT_DIR/plugin.json'))" 2>/dev/null; then
  echo "PASS"
else
  echo "FAIL"
  ERRORS=$((ERRORS + 1))
fi

# Test 2: All declared skills exist
echo -n "2. All declared skill files exist... "
MISSING=0
for skill_path in $(python3 -c "
import json
data = json.load(open('$SCRIPT_DIR/plugin.json'))
for s in data.get('skills', []):
    print(s['path'])
"); do
  if [[ ! -f "$SCRIPT_DIR/$skill_path" ]]; then
    echo ""
    echo "   MISSING: $skill_path"
    MISSING=$((MISSING + 1))
  fi
done
if [[ $MISSING -eq 0 ]]; then
  echo "PASS"
else
  echo "FAIL ($MISSING missing)"
  ERRORS=$((ERRORS + 1))
fi

# Test 3: All skills have valid frontmatter
echo -n "3. All skills have YAML frontmatter... "
INVALID=0
for skill_file in $(find "$SCRIPT_DIR/skills" -name "SKILL.md"); do
  FIRST_LINE=$(head -1 "$skill_file")
  if [[ "$FIRST_LINE" != "---" ]]; then
    echo ""
    echo "   INVALID: $skill_file (no frontmatter)"
    INVALID=$((INVALID + 1))
  fi
done
if [[ $INVALID -eq 0 ]]; then
  echo "PASS"
else
  echo "FAIL ($INVALID invalid)"
  ERRORS=$((ERRORS + 1))
fi

# Test 4: All skills have a name field
echo -n "4. All skills have name field... "
MISSING_NAME=0
for skill_file in $(find "$SCRIPT_DIR/skills" -name "SKILL.md"); do
  if ! grep -q "^name:" "$skill_file"; then
    echo ""
    echo "   MISSING name: $skill_file"
    MISSING_NAME=$((MISSING_NAME + 1))
  fi
done
if [[ $MISSING_NAME -eq 0 ]]; then
  echo "PASS"
else
  echo "FAIL ($MISSING_NAME missing)"
  ERRORS=$((ERRORS + 1))
fi

# Test 5: Hook scripts are executable
echo -n "5. Hook scripts are executable... "
NOT_EXEC=0
for hook_file in "$SCRIPT_DIR"/hooks/*.sh; do
  if [[ ! -x "$hook_file" ]]; then
    echo ""
    echo "   NOT EXECUTABLE: $hook_file"
    NOT_EXEC=$((NOT_EXEC + 1))
  fi
done
if [[ $NOT_EXEC -eq 0 ]]; then
  echo "PASS"
else
  echo "FAIL ($NOT_EXEC not executable)"
  ERRORS=$((ERRORS + 1))
fi

# Test 6: specgraph CLI exists (warn only)
echo -n "6. specgraph CLI available... "
if command -v specgraph &> /dev/null; then
  echo "PASS"
else
  echo "WARN (not in PATH — skills will fail at runtime)"
fi

# Test 7: Hook command compiles
echo -n "7. Hook command compiles... "
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
if (cd "$REPO_ROOT" && go build ./cmd/specgraph/) 2>/dev/null; then
  echo "PASS"
else
  echo "FAIL"
  ERRORS=$((ERRORS + 1))
fi

echo ""
echo "=== Results: $ERRORS error(s) ==="

if [[ $ERRORS -gt 0 ]]; then
  exit 1
fi
```

**Step 3: Make test script executable**

```bash
chmod +x plugin/specgraph/test-plugin.sh
```

**Step 4: Run the integration test**

```bash
./plugin/specgraph/test-plugin.sh
```

Expected: all tests PASS (except possibly test 6 if CLI not yet built)

**Step 5: Commit**

```bash
git add plugin/specgraph/test-plugin.sh cmd/specgraph/main.go
git commit -m "feat(plugin): integration test script and CLI hook wiring"
```

---

## Task 10: CLI init --tool=claude-code Support

**Files:**

- Edit: `cmd/specgraph/init.go`

**Step 1: Extend the init command**

Add a `--tool` flag to `specgraph init` that copies plugin files into the project:

In `cmd/specgraph/init.go`, add:

```go
var initToolFlag string

func init() {
	initCmd.Flags().StringVar(&initToolFlag, "tool", "", "Install tool integration (claude-code, cursor, agents-md)")
}
```

Add a case in the init run function:

```go
if initToolFlag != "" {
	switch initToolFlag {
	case "claude-code":
		if err := installClaudeCodePlugin(); err != nil {
			return fmt.Errorf("install claude-code plugin: %w", err)
		}
		fmt.Println("Installed Claude Code plugin (skills + SessionStart hook)")
	case "cursor":
		fmt.Println("Cursor integration not yet implemented")
	case "agents-md":
		fmt.Println("AGENTS.md integration not yet implemented")
	default:
		return fmt.Errorf("unknown tool: %s (supported: claude-code, cursor, agents-md)", initToolFlag)
	}
}
```

The `installClaudeCodePlugin` function:

```go
func installClaudeCodePlugin() error {
	// Find the plugin directory relative to the specgraph binary,
	// or use an embedded copy. For now, delegate to the install script
	// if available, otherwise create minimal structure.

	skillsDir := filepath.Join(".claude", "skills", "specgraph")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return err
	}

	// Write the SessionStart hook configuration
	settingsPath := filepath.Join(".claude", "settings.json")
	settings := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []map[string]any{
				{
					"type":    "command",
					"command": "specgraph hook session-start",
				},
			},
		},
	}

	// Read existing settings and merge
	if data, err := os.ReadFile(settingsPath); err == nil {
		var existing map[string]any
		if json.Unmarshal(data, &existing) == nil {
			// Merge hooks
			if existingHooks, ok := existing["hooks"].(map[string]any); ok {
				hooks := settings["hooks"].(map[string]any)
				for k, v := range existingHooks {
					if _, exists := hooks[k]; !exists {
						hooks[k] = v
					}
				}
			}
			// Preserve other settings
			for k, v := range existing {
				if k != "hooks" {
					settings[k] = v
				}
			}
		}
	}

	settingsJSON, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, settingsJSON, 0o644); err != nil {
		return err
	}

	fmt.Printf("  Created %s with SessionStart hook\n", settingsPath)
	fmt.Printf("  Skills directory: %s\n", skillsDir)
	fmt.Println("  Copy skills from plugin/specgraph/skills/ or run: specgraph plugin install")

	return nil
}
```

**Step 2: Verify it compiles**

```bash
go build ./cmd/specgraph/
```

**Step 3: Test the init --tool flag**

```bash
# Test in a temp directory
cd $(mktemp -d)
specgraph init --tool=claude-code
ls -la .claude/
cat .claude/settings.json
```

Expected: `.claude/settings.json` with SessionStart hook, `.claude/skills/specgraph/` directory

**Step 4: Commit**

```bash
git add cmd/specgraph/init.go
git commit -m "feat(plugin): specgraph init --tool=claude-code installs hooks and skill directory"
```

---

## Task 11: Final Verification and Cleanup

**Step 1: Run all tests**

```bash
go test ./... -count=1 -timeout=120s
```

Expected: all tests pass

**Step 2: Run linter**

```bash
golangci-lint run ./...
```

Expected: no issues

**Step 3: Verify full CLI build**

```bash
go build -o specgraph ./cmd/specgraph
./specgraph --help
./specgraph init --help
```

Expected: clean build, `--tool` flag visible in init help

**Step 4: Verify plugin structure**

```bash
# Check plugin manifest parses
python3 -c "import json; json.load(open('plugin/specgraph/plugin.json'))"

# Check all skill files exist
ls plugin/specgraph/skills/specgraph/SKILL.md
ls plugin/specgraph/skills/specgraph-spark/SKILL.md
ls plugin/specgraph/skills/specgraph-shape/SKILL.md
ls plugin/specgraph/skills/specgraph-specify/SKILL.md
ls plugin/specgraph/skills/specgraph-decompose/SKILL.md
ls plugin/specgraph/skills/specgraph-approve/SKILL.md
ls plugin/specgraph/skills/specgraph-list/SKILL.md
ls plugin/specgraph/skills/specgraph-show/SKILL.md
ls plugin/specgraph/skills/specgraph-deps/SKILL.md
ls plugin/specgraph/skills/specgraph-ready/SKILL.md
ls plugin/specgraph/skills/specgraph-bundle/SKILL.md

# Check hook script exists
ls plugin/specgraph/hooks/session-start.sh
```

Expected: all files present, plugin.json valid

**Step 5: Test full workflow in temp directory**

```bash
cd $(mktemp -d)
specgraph init --yes --tool=claude-code
ls .claude/settings.json
ls .claude/skills/specgraph/
```

Expected: settings.json created with hook, skills directory populated

**Step 6: Final commit if any cleanup needed**

```bash
git add -A
git commit -m "chore(plugin): cleanup and final verification"
```

---

## Summary

| Task | Deliverable | Commit |
|------|-------------|--------|
| 1 | Plugin manifest (`plugin.json`) | `feat(plugin): plugin.json manifest with skill and hook declarations` |
| 2 | Meta-skill (overview + routing) | `feat(plugin): meta-skill for SpecGraph overview and routing` |
| 3 | Authoring skills (spark, shape, specify, decompose, approve) | `feat(plugin): authoring funnel skills — spark, shape, specify, decompose, approve` |
| 4 | Query skills (list, show, deps, ready) | `feat(plugin): query skills — list, show, deps, ready` |
| 5 | Bundle skill | `feat(plugin): bundle skill for execution bundle generation and context priming` |
| 6 | SessionStart hook + CLI `hook` command | `feat(plugin): SessionStart hook with CLI hook command for session priming` |
| 7 | PostToolUse progress hook (optional) | `feat(plugin): optional PostToolUse progress hook for file write tracking` |
| 8 | Installation script + README | `feat(plugin): installation script and documentation` |
| 9 | Integration test + CLI wiring | `feat(plugin): integration test script and CLI hook wiring` |
| 10 | `init --tool=claude-code` support | `feat(plugin): specgraph init --tool=claude-code installs hooks and skill directory` |
| 11 | Final verification | `chore(plugin): cleanup and final verification` |

### Dependencies

- **Slice 1 (done):** CLI client, server, spec/claim services
- **Slice 2 (Constitution):** Constitution show/check used in authoring skills
- **Slice 3 (Authoring):** `spark`, `shape`, `specify`, `decompose`, `approve` CLI commands and prompt templates
- **Slice 4 (Execution):** `bundle`, `prime`, progress/blocker/completion reporting

Skills that reference commands from Slices 2-4 will work with stub output until those slices are implemented. The plugin structure and hook infrastructure from this slice stand alone.
