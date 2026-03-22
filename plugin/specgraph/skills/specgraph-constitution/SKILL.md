---
name: specgraph-constitution
description: >
  Create or update a project constitution — the layered ground truth that
  analytical passes check every spec against. Use when "setup a constitution",
  "create constitution", "define principles", "configure ground truth",
  "project rules", "tech stack setup", or bootstrapping a new SpecGraph project.
---

# SpecGraph Constitution

Guide the user through creating or updating their project constitution. The
constitution is the project's ground truth — tech stack, principles, constraints,
antipatterns, and process rules. Analytical passes (constitution_check, red_team,
etc.) evaluate every spec against it.

---

## Persona

> **Read `references/persona.md` for the full shared persona** — core identity, posture system
> (Drive/Partner/Support with auto-detection), pushback protocol, tone calibration,
> judgment heuristics, and conversational style.

---

## Step 1: Check Current State

```bash
specgraph constitution show
```

- If a constitution exists, show a summary and ask what they want to change.
- If none exists, proceed to creation.

## Step 2: Scan the Codebase

Read these files (if they exist) to pre-populate the constitution:

- `go.mod` or `package.json` — primary language and dependencies
- `CLAUDE.md` or `AGENTS.md` — existing conventions and constraints
- `docs/adr/` or `docs/decisions/` — architectural decision records
- `.goreleaser.yaml`, `Dockerfile`, `docker-compose.yml` — infrastructure
- `proto/` — API standards
- `Taskfile.yml` or `Makefile` — build tooling

Don't read every file — scan for what matters to constitution fields.

## Step 3: Draft the Constitution

Walk through each section conversationally (one at a time in Drive posture,
discussion-first in Partner/Support). Present your draft for each section and
get approval before moving on.

### Constitution YAML Schema

```yaml
name: "<project name>"
layer: "project"  # user | org | project | domain

tech:
  languages:
    primary: "<main language>"
    allowed:
      - "<language>"
    forbidden:
      - "<language>"
    forbidden_reasons:
      "<language>": "<why>"
  frameworks:
    "<area>": "<framework>"      # e.g. http-server: connectrpc
  infrastructure:
    "<area>": "<technology>"     # e.g. database: memgraph
  api_standards:
    "<area>": "<standard>"       # e.g. rpc: connectrpc/protobuf
  data:
    "<area>": "<technology>"     # e.g. graph: memgraph

principles:
  - id: "p-<n>"
    statement: "<the principle>"
    rationale: "<why it matters>"
    exceptions: "<when it's OK to bend>"

constraints:
  - "<hard rule that must not be violated>"

antipatterns:
  - pattern: "<what to avoid>"
    why: "<why it's harmful>"
    instead: "<what to do instead>"

process:
  spec_review: "<when/how specs are reviewed>"
  security_review:
    when: "<trigger for security review>"
  deployment:
    strategy: "<deployment strategy>"
    rollback: "<rollback policy>"
  documentation:
    api_docs: "<API doc requirements>"
    runbook: "<runbook requirements>"

references:
  - type: "adr"     # adr | spec | doc | url
    path: "<path or URL>"
```

### Section-by-Section Guide

**Tech Stack** — What language, frameworks, infrastructure, and data stores
does this project use? Derive from `go.mod`/`package.json`, Dockerfiles, and
existing config. Ask the user to confirm or adjust.

**Principles** — What design/engineering principles guide this project?
Look for them in CLAUDE.md, ADRs, and READMEs. Each needs an ID, statement,
rationale, and exceptions. Propose 3-5 based on what you find; ask if they
want to add more.

**Constraints** — What hard rules must never be violated? These are stronger
than principles — no exceptions without explicit override. Look in CLAUDE.md
for MUST/MUST NOT rules.

**Antipatterns** — What patterns should be avoided? Each needs the pattern,
why it's harmful, and what to do instead. Look for documented gotchas or
past incidents.

**Process** — How are specs reviewed? When is security review triggered?
What's the deployment strategy? This section is often sparse for new projects
— that's OK. Ask if they have process requirements.

**References** — Link to ADRs, specs, or docs that inform the constitution.
Scan for `docs/adr/` or similar directories.

## Step 4: Write and Import

1. Write the YAML to `constitution.yaml` (or the user's preferred filename)
2. Import it:

```bash
specgraph constitution import constitution.yaml
```

3. Verify:

```bash
specgraph constitution show
```

4. Show the result and confirm it looks right.

## Step 5: Emit Tool Files (Optional)

Ask if they want to generate tool-specific files:

```bash
specgraph constitution emit --format claude-md --output CONSTITUTION.md
```

Available formats: `claude-md`, `cursorrules`, `agents-md`

---

## Quality Heuristics

- **Too generic?** "Write clean code" is not a useful principle. Push for
  project-specific principles with concrete rationale.
- **Too many?** 3-7 principles is ideal. More than 10 means some should be
  constraints or antipatterns instead.
- **Missing kill conditions?** Every constraint should be falsifiable — if you
  can't tell whether it's violated, it's not a constraint.
- **No antipatterns?** Every project has them. If the user can't think of any,
  ask about past incidents or code review feedback.
