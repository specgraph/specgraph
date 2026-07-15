---
name: specgraph-constitution
summary: Bootstrap or update a SpecGraph project constitution — the layered ground truth analytical passes check specs against.
description: Use when the user wants to "setup a constitution", "create a constitution", "define principles", "configure ground truth", "bootstrap a new SpecGraph project", or set up project tech stack, principles, constraints, and antipatterns that analytical passes will enforce.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Constitution

Guide the user through creating or updating their project constitution. The
constitution is the project's ground truth — tech stack, principles, constraints,
antipatterns, and process rules. Analytical passes (constitution_check, red_team,
etc.) evaluate every spec against it.

The constitution is layered: `user` → `org` → `project` → `domain`. More
specific layers override general ones. Most users start at the `project` layer.

Everything below runs over MCP — the `constitution` tool and the
`specgraph://constitution` resource. No local CLI is required. (Source/CLI
users have an equivalent path in the gated appendix at the end; MCP-only
agents skip it.)

---

## Step 1: Check Current State

Read the current constitution with the `constitution` tool:

- `constitution` tool, `action: get` — returns the full constitution (pass an
  optional `layer` to read a single layer).
- Or read the `specgraph://constitution` resource for the whole thing, or
  `specgraph://constitution/{layer}` for one layer.

Then:

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

Walk through each section conversationally. Present your draft for each section
and get approval before moving on.

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

## Step 4: Write It Over MCP

Persist the constitution with the `constitution` tool — pass the **same
friendly YAML block from Step 3 inline** as the write payload:

- `constitution` tool, `action: update` — send the friendly YAML above
  (the exact `layer: "project"` schema you drafted). No file, no import step,
  no format conversion. The block you showed the user is the block you write.

Then verify:

- `constitution` tool, `action: get` — read it back and confirm it looks right.
- Or re-read the `specgraph://constitution` resource.

Show the result and confirm it looks right with the user.

> The friendly YAML you draft in Step 3 IS the write payload. Do not translate
> it into another shape before calling `update` — the tool accepts this schema
> directly. (This is the whole point: what you teach the user is what the tool
> ingests.)

## Step 5: Emit Tool Files (Optional)

If the user wants tool-specific files (a `CONSTITUTION.md`, cursor rules, or an
`AGENTS.md` block) generated from the constitution, that generation currently
runs through the local CLI — see the gated appendix below. MCP-only agents can
skip this; the constitution itself is fully readable via
`specgraph://constitution`.

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

---

## Requires local CLI (source/CLI users only — MCP-only agents skip this)

These steps need the `specgraph` binary on a local machine. MCP-only agents do
not use them — the `constitution` tool and `specgraph://constitution` resource
above cover the same ground.

- Inspect the current constitution:

  ```bash
  specgraph constitution show
  ```

- Write a constitution from a YAML file (the CLI equivalent of
  `constitution` tool `action: update`):

  ```bash
  specgraph constitution import constitution.yaml
  ```

- Emit tool-specific files:

  ```bash
  specgraph constitution emit --format claude-md --output CONSTITUTION.md
  ```

  Available formats: `claude-md`, `cursorrules`, `agents-md`
