# Harness Parity Epic Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver feature parity for Claude Code, Cursor, and OpenCode by building a shared in-tree skills layer, per-harness shims, an MCP `approve` prompt, and post-stage analytical-pass automation in all three harnesses.

**Architecture:** Single in-tree `skills/` directory at repo root holds agentskills.io-spec-clean SKILL.md packages. Per-harness shims under `plugin/<harness>/` consume them via symlink (or `task plugin:sync` copy) and add only harness-specific surface (manifest, hooks, rules, plugin modules). Post-stage automation lands as Claude `PostToolUse` hooks, a Cursor rules file, and an OpenCode TypeScript plugin.

**Tech Stack:** Go 1.22+ (existing), `agentskills/skills-ref` for skill validation, bash for Claude hooks, Cursor rules MDC, TypeScript + `@opencode-ai/plugin` for OpenCode.

**Companion design:** [docs/plans/2026-05-06-harness-parity-epic-design.md](2026-05-06-harness-parity-epic-design.md)

---

## Pre-flight (read before starting)

- **DCO email:** `Sean Brandt <4678+seanb4t@users.noreply.github.com>`.
- **License header on `.go` files:** match `LICENSE_HEADER` bytes-for-bytes — `// SPDX-License-Identifier: Apache-2.0` then `// Copyright 2026 Sean Brandt`. `task check` runs `addlicense -check -f LICENSE_HEADER`.
- **License header on `.sh` and `.ts` files:** same SPDX line in the appropriate comment style (`#` for shell, `//` for TS).
- **Pre-commit ritual** (apply before EVERY `jj commit`):
  1. `jj --no-pager status`
  2. If `.beads/issues.jsonl` is dirty → `jj --no-pager restore --from @- .beads/issues.jsonl`
  3. If `web/src/lib/api/gen/*.ts` is dirty → `jj --no-pager restore --from main --to @ web/src/lib/api/gen/`
  4. After commit, before any subsequent commit: confirm `@` is empty.
- **gh auth note:** before any `jj git push`, run `gh auth switch -u seanb4t -h github.com`.
- **Working directory:** main repo or a `jj workspace add` workspace. This plan assumes the main workspace.

---

## File structure

| File | Action | Responsibility |
|---|---|---|
| `skills/specgraph-authoring/SKILL.md` | Create | When and how to invoke MCP authoring prompts. |
| `skills/specgraph-graph-query/SKILL.md` | Create | Querying ready specs, dependencies, impact. |
| `skills/specgraph-analytical-passes/SKILL.md` | Create | Running constitution-check, peripheral-vision, etc. |
| `skills/specgraph-drift/SKILL.md` | Create | Detecting and acknowledging drift on done specs. |
| `skills/specgraph-conventions/SKILL.md` | Create | Slug conventions, stage transitions, approval rules. |
| `skills/specgraph-troubleshooting/SKILL.md` | Create | Common errors, MCP connection issues. |
| `Taskfile.yml` | Modify | Add `skills:validate` and `plugin:sync` tasks; wire `skills:validate` into `check`. |
| `internal/mcp/prompts.go` | Modify | Register `approve` prompt. |
| `internal/mcp/prompts_test.go` | Modify or create | Test that `approve` is registered and dispatches via `stagePromptHandler`. |
| `internal/mcp/profiles_test.go` | Modify or create | Cross-harness assertion that `claude-code`, `cursor`, `opencode` all map to authoring profile. |
| `plugin/specgraph/skills` | Create (symlink) | `→ ../../skills` so Claude sees in-tree skills under the plugin root. |
| `plugin/specgraph/hooks/post-stage.sh` | Create | Triggers `analytical_pass` after `author` tool calls. |
| `plugin/specgraph/hooks/hooks.json` | Modify | Add `PostToolUse` entry for `post-stage.sh`. |
| `plugin/specgraph/routing-guide.md` | Modify | Trim to one screen; remove bulk now in skills. |
| `plugin/specgraph/README.md` | Modify | Replace "no skills bundled" with current story. |
| `plugin/cursor/.cursor/rules/specgraph.md` | Create | Routing rules pointing to MCP and skills. |
| `plugin/cursor/.cursor/rules/post-stage.md` | Create | Post-stage analytical pass guidance. |
| `plugin/cursor/.cursor/skills` | Create (symlink) | `→ ../../../skills` |
| `plugin/cursor/README.md` | Create | Install instructions for the Cursor shim. |
| `plugin/opencode/.opencode/plugins/specgraph.ts` | Create | Session.start prime + tool.use post-stage hooks. |
| `plugin/opencode/.opencode/skills` | Create (symlink) | `→ ../../../skills` |
| `plugin/opencode/package.json` | Create | Publishable npm shape for the OpenCode plugin. |
| `plugin/opencode/README.md` | Create | Install instructions for the OpenCode shim. |
| `CLAUDE.md` | Modify | Update Documentation section to describe the new layout. |
| `internal/inject/inject.go` | Modify | Fix doc comments that say inject writes to `CLAUDE.md`. |
| `site/docs/**` | Modify | Audit and align init/inject/MCP pages. |
| `docs/plans/2026-02-28-slice-7-claude-code-plugin-plan.md` | Modify | Add `superseded-by:` frontmatter. |
| `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md` | Modify | Add `superseded-by:` frontmatter. |
| `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-plan.md` | Modify | Add `superseded-by:` frontmatter. |
| `docs/plans/2026-03-17-skill-personas-design.md` | Modify | Add `superseded-by:` frontmatter. |
| `docs/plans/2026-03-17-skill-personas-plan.md` | Modify | Add `superseded-by:` frontmatter. |
| `docs/plans/README.md` | Create | Index of active vs superseded plans. |

---

## Task ordering

Tasks correspond 1:1 to the bead children. Order respects the dependency graph in the design doc §G.2:

1. T8 — Plans archeology (parallel-safe, do first to clear desk)
2. T2 — Approve MCP prompt + cross-harness test (parallel-safe)
3. T1 — Skills foundation (blocks T3/T4/T5/T7)
4. T3 — Claude shim refactor (blocks T6/T7)
5. T4 — Cursor shim (blocks T6/T7)
6. T5 — OpenCode shim (blocks T6/T7)
7. T6 — Post-stage parity verification (blocks T7)
8. T7 — Doc drift fixes (blocks T9)
9. T9 — Humanizer pass
10. T10 — Codex follow-up bead

---

## Task 8: Plans archeology

**Files:**

- Modify: `docs/plans/2026-02-28-slice-7-claude-code-plugin-plan.md`
- Modify: `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md`
- Modify: `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-plan.md`
- Modify: `docs/plans/2026-03-17-skill-personas-design.md`
- Modify: `docs/plans/2026-03-17-skill-personas-plan.md`
- Create: `docs/plans/README.md`

- [ ] **Step 1: Add superseded-by header to each historical doc**

Insert this block immediately after the H1 title line of each file:

````markdown
> **Status:** Superseded by [2026-04-20-multi-platform-plugin-design.md](2026-04-20-multi-platform-plugin-design.md) and [2026-05-06-harness-parity-epic-design.md](2026-05-06-harness-parity-epic-design.md). Retained for historical context.
````

- [ ] **Step 2: Create `docs/plans/README.md` index**

Group plans by status. List each with date, title, and one-line summary. Active = directly referenced by current code or a current epic. Superseded = explicitly marked above. Historical = pre-current-architecture but not formally superseded.

- [ ] **Step 3: Verify with `task lint`**

Run: `task lint`
Expected: clean. Markdown linters may complain about line length; wrap to 80 cols if needed.

- [ ] **Step 4: Commit**

```bash
jj describe -m "docs(plans): mark superseded plans, add index"
```

---

## Task 2: Approve MCP prompt + cross-harness test

**Files:**

- Modify: `internal/mcp/prompts.go`
- Create or modify: `internal/mcp/prompts_test.go`
- Create or modify: `internal/mcp/profiles_test.go`

- [ ] **Step 1: Write the failing test (cross-harness profile)**

Add to `internal/mcp/profiles_test.go`:

````go
func TestProfileFromClientInfo_Harnesses(t *testing.T) {
    cases := []struct {
        name string
        want Profile
    }{
        {"claude-code", ProfileAuthoring},
        {"cursor", ProfileAuthoring},
        {"cursor-vscode", ProfileAuthoring},
        {"opencode", ProfileAuthoring},
        {"codex", ProfileAuthoring},
    }
    for _, c := range cases {
        t.Run(c.name, func(t *testing.T) {
            got := ProfileFromClientInfo(&sdkmcp.Implementation{Name: c.name})
            if got != c.want {
                t.Errorf("client %q: got %v, want %v", c.name, got, c.want)
            }
        })
    }
}
````

- [ ] **Step 2: Write the failing test for `approve` registration**

Add to `internal/mcp/prompts_test.go`:

````go
func TestRegisterPrompts_IncludesApprove(t *testing.T) {
    r := NewRegistry()
    RegisterPrompts(r, nil)
    if _, ok := r.GetPrompt("approve"); !ok {
        t.Fatal("approve prompt not registered")
    }
}
````

(Adapt to the actual registry API; check `internal/mcp/registry.go` for the exact accessor name. If `GetPrompt` does not exist, expose a list/iter accessor as part of this task.)

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/mcp/...`
Expected: profiles test passes (existing behavior is correct); approve test FAILS.

- [ ] **Step 4: Add `approve` prompt registration**

In `internal/mcp/prompts.go`, after the `decompose` block:

````go
r.AddPrompt(PromptDef{
    Name:        "approve",
    Description: "Walk a decomposed spec to approval, capturing decisions and edges.",
    Arguments: []PromptArgument{
        {Name: "spec_slug", Description: "Slug of the spec to approve.", Required: true},
    },
    Handler: stagePromptHandler(c, "approve"),
})
````

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/mcp/... -run 'TestRegisterPrompts_IncludesApprove|TestProfileFromClientInfo_Harnesses'`
Expected: PASS.

- [ ] **Step 6: Run `task check`**

Run: `task check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj describe -m "feat(mcp): register approve prompt and add cross-harness profile test"
```

---

## Task 1: Skills foundation

**Files:**

- Create: `skills/specgraph-authoring/SKILL.md`
- Create: `skills/specgraph-graph-query/SKILL.md`
- Create: `skills/specgraph-analytical-passes/SKILL.md`
- Create: `skills/specgraph-drift/SKILL.md`
- Create: `skills/specgraph-conventions/SKILL.md`
- Create: `skills/specgraph-troubleshooting/SKILL.md`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Write each SKILL.md with spec-clean frontmatter**

Each file follows this template (substitute name, description, body):

````markdown
---
name: specgraph-authoring
description: Use when authoring or updating SpecGraph specs (spark, shape, specify, decompose, approve). Routes the user through the right MCP prompt or tool for the current authoring stage.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph: Authoring Specs

[Body that progressively discloses context: when the skill applies, what the
authoring funnel is, the five stages, when to call MCP prompts vs the `author`
tool, and how to record stage outputs.]
````

For each skill, source content from the corresponding section of `plugin/specgraph/routing-guide.md` (about to be trimmed in T3) plus the relevant authoring content under `internal/authoring/content/`. Keep each SKILL.md body under ~200 lines; defer detail to references files in the skill folder if needed.

- [ ] **Step 2: Add `task skills:validate` to `Taskfile.yml`**

````yaml
skills:validate:
  desc: Validate skills/ packages against the agentskills.io spec
  cmds:
    - |
      if ! command -v skills-ref >/dev/null 2>&1; then
        echo "skills-ref not installed; install with: go install github.com/agentskills/skills-ref/cmd/skills-ref@latest"
        exit 1
      fi
      skills-ref validate skills/
````

If `skills-ref` is not Go-installable in 2026, fall back to a Docker invocation:

````yaml
cmds:
  - docker run --rm -v "$(pwd)/skills:/skills" agentskills/skills-ref:latest validate /skills
````

Pick the available method at implementation time. The contract is "validation runs as part of `task check`."

- [ ] **Step 3: Add `skills:validate` to `task check` deps**

In `Taskfile.yml`, under the `check` task, add `skills:validate` to the `deps` list (or `cmds` chain, matching existing style).

- [ ] **Step 4: Add `task plugin:sync` for symlink-or-copy**

````yaml
plugin:sync:
  desc: Refresh per-harness skills/ links to match the in-tree skills/
  cmds:
    - |
      for harness in specgraph cursor opencode; do
        case "$harness" in
          specgraph) link="plugin/specgraph/skills"; target="../../skills" ;;
          cursor) link="plugin/cursor/.cursor/skills"; target="../../../skills" ;;
          opencode) link="plugin/opencode/.opencode/skills"; target="../../../skills" ;;
        esac
        rm -rf "$link"
        ln -sfn "$target" "$link"
      done
````

(For Windows, ship a parallel `plugin:sync:copy` that uses `cp -R` — defer until a real Windows contributor reports breakage.)

- [ ] **Step 5: Run `task skills:validate`**

Run: `task skills:validate`
Expected: PASS for all six skills.

- [ ] **Step 6: Run `task check`**

Run: `task check`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj describe -m "feat(skills): add agentskills.io-compliant in-tree skills/ tree"
```

---

## Task 3: Claude shim refactor

**Files:**

- Create (symlink): `plugin/specgraph/skills`
- Create: `plugin/specgraph/hooks/post-stage.sh`
- Modify: `plugin/specgraph/hooks/hooks.json`
- Modify: `plugin/specgraph/routing-guide.md`
- Modify: `plugin/specgraph/README.md`

- [ ] **Step 1: Create the skills symlink**

Run: `task plugin:sync` (from T1 step 4) — creates `plugin/specgraph/skills → ../../skills`.

- [ ] **Step 2: Trim routing-guide.md**

Replace `plugin/specgraph/routing-guide.md` with a one-screen pointer body:

````markdown
# SpecGraph Routing Guide

You have access to the SpecGraph MCP server for spec-driven development on
this project. Detail lives in skills under `plugin/specgraph/skills/`; this
guide is a one-screen pointer.

## Authoring

Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`, `decompose`,
`approve`) or call the `author_start_stage` tool. Persist with the `author`
tool. See `skills/specgraph-authoring/`.

## Querying

Use the `spec`, `graph_query` tools and the `specgraph://graph/ready` resource.
See `skills/specgraph-graph-query/`.

## Analytical review

Use `analytical_pass` with `action: "run"`. See
`skills/specgraph-analytical-passes/`.

## Constitution

`specgraph://constitution` resource for content; `constitution` tool for
get/update.

## Setup

```bash
docker info
specgraph init
specgraph serve
```

## Never

- Don't invent dotted tool names; tools are flat with `action` parameters.
- Don't approve a spec on behalf of the user; approval requires explicit
  user sign-off.
````

- [ ] **Step 3: Update README**

Replace the relevant section of `plugin/specgraph/README.md`:

````markdown
## Layout

| Path | Purpose |
|------|---------|
| `.claude-plugin/plugin.json` | Plugin manifest |
| `hooks/session-start.sh` | Reads `specgraph://prime` from the MCP server at session start |
| `hooks/post-stage.sh` | Runs analytical passes after stage transitions |
| `skills/` | Symlink to repo-root `skills/` (shared with Cursor and OpenCode) |
| `routing-guide.md` | One-screen pointer to MCP prompts/tools/resources |
````

- [ ] **Step 4: Add post-stage hook script**

Create `plugin/specgraph/hooks/post-stage.sh`:

````bash
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt

set -euo pipefail

input="$(cat)"
tool_name="$(jq -r '.tool_name // empty' <<<"$input")"
action="$(jq -r '.tool_input.action // empty' <<<"$input")"

case "$tool_name:$action" in
  author:spark|author:shape|author:specify|author:decompose|author:approve)
    cat <<EOF
{
  "decision": "block",
  "reason": "Run analytical passes registered for the $action stage by calling the analytical_pass tool with action=run for each pass type returned by passes_for_stage."
}
EOF
    ;;
  *)
    exit 0
    ;;
esac
````

Make executable: `chmod +x plugin/specgraph/hooks/post-stage.sh`.

- [ ] **Step 5: Wire post-stage hook into hooks.json**

Modify `plugin/specgraph/hooks/hooks.json`:

````json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/session-start.sh" }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "mcp__specgraph__author",
        "hooks": [
          { "type": "command", "command": "${CLAUDE_PLUGIN_ROOT}/hooks/post-stage.sh" }
        ]
      }
    ]
  }
}
````

(Confirm matcher name format against current Claude Code hooks docs.)

- [ ] **Step 6: Smoke-test locally**

Run: `claude --plugin-dir ./plugin/specgraph` in a scratch project; check that skills resolve and the routing guide loads.

- [ ] **Step 7: Run `task check`**

Run: `task check`
Expected: PASS.

- [ ] **Step 8: Commit**

```bash
jj describe -m "refactor(plugin): consume shared skills/, trim routing guide, add post-stage hook"
```

---

## Task 4: Cursor shim

**Files:**

- Create: `plugin/cursor/.cursor/rules/specgraph.md`
- Create: `plugin/cursor/.cursor/rules/post-stage.md`
- Create (symlink): `plugin/cursor/.cursor/skills`
- Create: `plugin/cursor/README.md`

- [ ] **Step 1: Create the skills symlink**

Run: `task plugin:sync`

- [ ] **Step 2: Create the routing rule**

`plugin/cursor/.cursor/rules/specgraph.md`:

````markdown
---
description: SpecGraph routing — use when the user mentions specs, authoring stages (spark/shape/specify/decompose/approve), the constitution, drift, or analytical passes.
alwaysApply: false
---

# SpecGraph Routing

You have access to the SpecGraph MCP server. Detail lives in
`.cursor/skills/`; this rule is the routing pointer.

- Authoring: use MCP prompts `spark`, `shape`, `specify`, `decompose`,
  `approve` or the `author_start_stage` tool. Persist with `author`.
- Querying: `spec`, `graph_query`, `specgraph://graph/ready` resource.
- Analytical: `analytical_pass` with `action: "run"`.
- Constitution: `specgraph://constitution` resource; `constitution` tool.

Never invent dotted tool names; tools are flat with `action` parameters.
Never approve specs without explicit user sign-off.
````

- [ ] **Step 3: Create the post-stage rule**

`plugin/cursor/.cursor/rules/post-stage.md`:

````markdown
---
description: After a SpecGraph stage transition, run the analytical passes registered for that stage.
alwaysApply: false
---

# SpecGraph Post-Stage

After a successful `author` tool call (action: spark/shape/specify/decompose/
approve), call the `analytical_pass` tool with `action: "run"` for each pass
type returned by `passes_for_stage` for the just-completed stage.
````

- [ ] **Step 4: Write the install README**

`plugin/cursor/README.md` — install instructions, what-you-get summary, link to Cursor's remote-rules import flow.

- [ ] **Step 5: Smoke-test locally**

Open a Cursor window with a scratch project containing the shim. Verify the rule appears in Settings → Rules and that skills are discoverable.

- [ ] **Step 6: Run `task check`**

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj describe -m "feat(plugin): add Cursor shim with rules and shared skills/"
```

---

## Task 5: OpenCode shim

**Files:**

- Create: `plugin/opencode/.opencode/plugins/specgraph.ts`
- Create (symlink): `plugin/opencode/.opencode/skills`
- Create: `plugin/opencode/package.json`
- Create: `plugin/opencode/README.md`

- [ ] **Step 1: Create the skills symlink**

Run: `task plugin:sync`

- [ ] **Step 2: Write the plugin TypeScript**

`plugin/opencode/.opencode/plugins/specgraph.ts` should:

- Import `Plugin` type from `@opencode-ai/plugin`.
- Import `execFile` from `node:child_process` and wrap with `promisify` from `node:util` for safe argv-array invocation (no shell interpolation).
- Export a default `Plugin` object with two hooks:
  - `session.start`: invoke `specgraph` CLI with arguments `["read-mcp-resource", "specgraph://prime"]` via `execFile`, append stdout to the session output. Soft-fail on missing CLI or unreachable server (mirror the bash session-start hook).
  - `tool.use`: when phase is `after`, tool is `mcp__specgraph__author`, and `input.action` is one of spark/shape/specify/decompose/approve, call `ctx.suggest(...)` with the analytical-passes nudge.

Verify field names against the version of `@opencode-ai/plugin` pinned in `package.json` (step 3) and adjust as needed. The contract is "session start primes from the MCP, and a successful `author` stage action prompts an analytical pass." Use `execFile` (argv array) — never `exec` with a shell-interpolated string.

- [ ] **Step 3: Write package.json**

`plugin/opencode/package.json`:

````json
{
  "name": "@specgraph/opencode-plugin",
  "version": "0.1.0",
  "description": "OpenCode plugin for SpecGraph: session priming and post-stage analytical passes.",
  "license": "Apache-2.0",
  "main": ".opencode/plugins/specgraph.ts",
  "type": "module",
  "peerDependencies": {
    "@opencode-ai/plugin": "*"
  }
}
````

- [ ] **Step 4: Write the install README**

`plugin/opencode/README.md` — install instructions (local path in `opencode.json` plugin array), what-you-get summary, future npm install path.

- [ ] **Step 5: Smoke-test locally**

Run OpenCode against a scratch project that points to the local plugin path. Verify session prime appears and that a stage action triggers the suggestion.

- [ ] **Step 6: Run `task check`**

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj describe -m "feat(plugin): add OpenCode shim with TS plugin and shared skills/"
```

---

## Task 6: Post-stage parity (verification)

T3, T4, and T5 each landed their harness's piece of post-stage automation. T6 is the cross-harness verification + bead supersession step.

- [ ] **Step 1: Verify Claude post-stage hook fires**

Manual: in a Claude Code session, invoke `author` with `action: spark`. Confirm the post-stage hook prompts for analytical passes.

- [ ] **Step 2: Verify Cursor post-stage rule fires**

Manual: in Cursor, perform a stage transition. Confirm the agent suggests analytical passes.

- [ ] **Step 3: Verify OpenCode post-stage hook fires**

Manual: in OpenCode, perform a stage transition. Confirm the suggestion appears.

- [ ] **Step 4: Document harness differences**

Add a section to `plugin/specgraph/README.md` (or a new `docs/plans/2026-05-06-harness-parity-epic-postscript.md`) titled "Post-stage automation contract" describing the per-harness mechanism with the shared contract: "after a stage transition, analytical passes are surfaced." Avoid pretending semantics are identical.

- [ ] **Step 5: Commit**

```bash
jj describe -m "docs(plugin): document post-stage automation contract"
```

---

## Task 7: Documentation drift fixes

**Files:**

- Modify: `CLAUDE.md`
- Modify: `internal/inject/inject.go` (doc comments only)
- Modify: `site/docs/**` (audit pass)

- [ ] **Step 1: Update CLAUDE.md Documentation section**

Find the "Plugin" entry under Documentation. Replace with text describing the new `skills/` shared layer, the three per-harness shims, and the post-stage automation contract. Remove references to the retired 13-skill layout history.

- [ ] **Step 2: Fix inject.go doc comments**

Search for any comment in `internal/inject/inject.go` and surrounding files that says inject writes to `CLAUDE.md`. Replace with accurate description: writes to `.claude/specs/`, `.cursor/rules/`, `AGENTS.md` per the `--tool` flag.

```bash
rg -l "CLAUDE\.md" internal/inject/
```

For each match, verify the comment is accurate; fix any that misrepresent the writers.

- [ ] **Step 3: Audit site/docs**

```bash
rg -l "CLAUDE\.md|skills bundled|13.skill" site/docs/
```

For each hit, read the page and align it with current code behavior.

- [ ] **Step 4: Run `task lint`**

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj describe -m "docs: align plugin/skills/inject docs with current code"
```

---

## Task 9: Humanizer pass

**Files:** all new and touched docs from prior tasks.

- [ ] **Step 1: Read the humanizer skill**

Read `/Users/SeBrandt/.claude/skills/humanizer/SKILL.md` for the target patterns.

- [ ] **Step 2: Run pattern checks across new and touched docs**

```bash
rg -n 'leverage|robust|seamless|delve|crucial|vital|tapestry|in essence|it.s important to note|not just .* but also' \
  skills/ plugin/ docs/plans/2026-05-06-* CLAUDE.md README.md site/docs/
```

Review each hit. Replace with plain phrasing where the word is filler.

- [ ] **Step 3: Em-dash audit**

```bash
rg -n '—' skills/ plugin/ docs/plans/2026-05-06-*
```

Reduce to genuine parentheticals. Replace stylistic em-dashes with periods or commas.

- [ ] **Step 4: Rule-of-three audit**

Scan for triplet padding ("simple, clean, and elegant" / "fast, reliable, and scalable"). Reduce to one or two adjectives where the third is filler.

- [ ] **Step 5: Run `task lint`**

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
jj describe -m "docs: humanizer pass on new and touched docs"
```

---

## Task 10: Codex follow-up bead

**Output:** a new standalone bead "Add Codex MCP config + shim" with concrete acceptance criteria.

- [ ] **Step 1: Verify current Codex docs**

WebFetch the current canonical Codex docs URL in 2026 to confirm:

- The repo-committable config file path and shape.
- Whether Codex supports MCP at all.
- Whether Codex supports skills via any path.

- [ ] **Step 2: File the bead**

```bash
bd create --type feature --priority 3 --title "Add Codex MCP config + shim" --description "..."
```

Body should include:

- Why this is deferred (work-machine access, API stability uncertainty).
- The expected shape of `mcpconfigs.codexConfig` based on current Codex docs.
- A pointer to this epic as the architectural template (in-tree skills, per-harness shim).
- Acceptance criteria: `specgraph init` writes a Codex config file; `plugin/codex/` shim mirrors `plugin/cursor/` shape.

- [ ] **Step 3: Commit no code (this task is bead-only)**

---

## Beads bookkeeping

After all task commits land:

- [ ] **Step 1: Close `spgr-iap`**

```bash
bd update spgr-iap --status closed --note "Superseded by harness-parity epic; scope expanded to all three harnesses. See docs/plans/2026-05-06-harness-parity-epic-design.md and the T6 child bead."
```

- [ ] **Step 2: Mark all task beads closed**

For each of T1–T9 (T10 stays open until the Codex bead is filed in step 2 of T10):

```bash
bd update <task-bead-id> --status closed
```

- [ ] **Step 3: Close the epic**

```bash
bd update <epic-id> --status closed
```

- [ ] **Step 4: Push**

```bash
git pull --rebase
bd dolt push
git push
git status  # MUST show "up to date with origin"
```

---

## Self-review checklist

Run this after the full plan is complete, before opening a PR.

- [ ] All six skills validate via `task skills:validate`.
- [ ] `internal/mcp/prompts.go` registers `approve`; tests pass.
- [ ] `plugin/specgraph/skills` resolves to in-tree `skills/`.
- [ ] `plugin/cursor/.cursor/skills` resolves to in-tree `skills/`.
- [ ] `plugin/opencode/.opencode/skills` resolves to in-tree `skills/`.
- [ ] Post-stage automation fires in all three harnesses (manual smoke test).
- [ ] `routing-guide.md` is one screen and references skills.
- [ ] No README/CLAUDE.md statement contradicts code.
- [ ] `docs/plans/README.md` index lists active vs superseded.
- [ ] Humanizer pass complete; pattern-check has no filler hits.
- [ ] Codex follow-up bead filed.
- [ ] `spgr-iap` closed with supersession note.
- [ ] `task check` passes.
- [ ] `task pr-prep` passes.
