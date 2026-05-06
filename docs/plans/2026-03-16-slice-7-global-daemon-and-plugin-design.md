# Slice 7: Global Daemon & Claude Code Plugin — Design

> **Status:** Superseded by [2026-04-20-multi-platform-plugin-design.md](2026-04-20-multi-platform-plugin-design.md) and [2026-05-06-harness-parity-epic-design.md](2026-05-06-harness-parity-epic-design.md). Retained for historical context.
>
> **Supersedes:** `2026-02-28-slice-7-claude-code-plugin-plan.md` (which assumed per-project server model)
>
> **Partially superseded:** The CLI command surface described in §`specgraph up` / §`specgraph down` (lines 151–193) was revised in `2026-04-22-cli-lifecycle-split-design.md`. `install`/`uninstall` are now dedicated verbs, `--rm` is retired, and `down --purge` is the confirmation-guarded destructive flag. Sections below that describe the original `up`/`down` surface are kept for historical record but should not be treated as current spec.

**Goal:** Transform SpecGraph from a per-project tool into a global development daemon, then ship a Claude Code plugin that wraps the CLI in conversational skills.

**Two major deliverables:**

1. Global daemon architecture (config, lifecycle, graph namespacing)
2. Claude Code plugin (skills, hooks, plugin manifest)

---

## Architecture Overview

SpecGraph becomes a **global development daemon** — one server process, one Memgraph instance, serving all projects. The Claude Code plugin is a thin layer of SKILL.md files that shell out to the `specgraph` CLI. No business logic lives in the plugin.

### Data Residency

| Data | Location | Format | Why |
|------|----------|--------|-----|
| Project identity | `<repo>/.specgraph.yaml` | YAML | Maps directory → project slug on server |
| Constitution | Server (Memgraph) | Graph node, keyed by project slug | Durable, queryable, single source of truth |
| Specs, edges, decisions | Server (Memgraph) | Graph nodes/edges, keyed by project slug | Core graph data |
| Sync config (adapters, targets) | Server (Memgraph) | Project metadata node | Project-scoped settings |
| Server locations, Docker, routing | `~/.config/specgraph/config.yaml` | YAML | Infrastructure config, user-specific |
| Docker compose file | `~/.local/share/specgraph/` | YAML | Persistent runtime data |
| Server logs | `~/.local/state/specgraph/` | Files | Ephemeral runtime state |
| User service definition | Platform-native location | plist/unit | OS-managed server lifecycle |

### Config Files

**Per-repo** — `<repo>/.specgraph.yaml` (committed):

```yaml
project: specgraph                # optional — auto-derived from git remote if omitted
# server: "https://..."           # optional — override server for this repo
```

Minimal by design. Everything else (constitution, sync config, adapters) lives server-side and is managed via CLI/RPC. The `server` field is an optional override for teams with a shared specgraph instance.

The `project` field is **optional**. If omitted or if `.specgraph.yaml` doesn't exist, the slug is auto-derived:

- Parse `git remote get-url origin` → extract `owner/repo` → normalize to `owner-repo`
- No git remote → use directory name
- Auto-derived slugs are logged so the user knows what was inferred

**Global** — `~/.config/specgraph/config.yaml`:

```yaml
# Server configuration — used by `specgraph up` / `specgraph serve`
# Omit entirely on client-only machines (remote server only)
server:
  listen: "0.0.0.0:7890"
  mode: service                    # service (default) | manual
  backend: memgraph
  memgraph:
    bolt_uri: "bolt://localhost:7687"
  docker: true                     # manage Memgraph container lifecycle

# Client configuration — used by all CLI commands
client:
  default_server: "http://localhost:7890"
  routes:                          # optional: route projects to different servers
    - project: "org-b-*"          # glob match on project slug (filepath.Match semantics)
      server: "https://specgraph.corp:7890"
```

The two top-level keys are independently meaningful:

- **Client-only machine** (remote server): only `client:` present. No `server:` section.
- **Server-only machine** (shared infra): only `server:` present. No `client:` section.
- **Local dev** (default): both present.

**Client config resolution order** (first match wins):

1. `.specgraph.yaml` `server` field (per-repo override)
2. Global `client.routes` — glob match on project slug
3. Global `client.default_server`

**First-run behavior:** If `~/.config/specgraph/config.yaml` doesn't exist, `specgraph up` or `specgraph prime` writes sensible defaults (both `server:` and `client:` sections, localhost, Docker-managed Memgraph).

### XDG Directory Layout

| XDG Variable | Path | Contents |
|-------------|------|----------|
| `XDG_CONFIG_HOME` | `~/.config/specgraph/config.yaml` | Global config (server + client) |
| `XDG_DATA_HOME` | `~/.local/share/specgraph/docker-compose.yaml` | Generated compose file |
| `XDG_STATE_HOME` | `~/.local/state/specgraph/server.log` | Server logs |
| (platform) | `~/Library/LaunchAgents/com.specgraph.server.plist` | macOS user service |
| (platform) | `~/.config/systemd/user/specgraph.service` | Linux user service |

On macOS, respects `XDG_*` env vars if set, otherwise uses `~/.config`, `~/.local/share`, `~/.local/state` (de facto standard on macOS for CLI tools). Service definitions use platform-native locations (LaunchAgents on macOS, systemd user units on Linux).

---

## Graph Isolation (Multi-Project Namespacing)

Project isolation uses **edges, not properties** — leveraging the graph for what it is. Every domain node (Spec, Decision, ExternalRef, etc.) connects to its `(:Project)` node via a `[:BELONGS_TO]` edge:

```cypher
-- Creating a spec within a project
MATCH (p:Project {slug: $project})
CREATE (p)<-[:BELONGS_TO]-(s:Spec {slug: $slug, title: $title, ...})
```

Every query scopes to a project via the edge:

```cypher
-- GetSpec: pattern match through Project node
MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
RETURN s
```

**Why edge-only (no `project` property on nodes):**

- **Normalized** — project identity lives in one place (the Project node). No duplication across thousands of nodes.
- **Graph-native** — "all nodes in this project" is a single traversal from the Project node.
- **Phase 4 ready** — cross-project queries (federation, multi-repo) become trivial: traverse from multiple Project nodes.
- **Rename-safe** — changing a project slug updates one node, not every node in the graph.
- **Performant** — Memgraph keeps adjacency lists in memory. The `BELONGS_TO` hop is O(1) in practice.

**Enforcement:** The `memgraph.Store` constructor takes a project slug. All query methods include the `BELONGS_TO` edge pattern. Callers never construct raw Cypher — the storage layer is the enforcement boundary.

```go
// Current
store, err := memgraph.New(ctx, boltURI)

// New
store, err := memgraph.New(ctx, boltURI, memgraph.WithProject(projectSlug))
```

When `projectSlug` is empty, the store returns an error — no implicit "all projects" queries.

**Indexes:**

- `CREATE INDEX ON :Project(slug)` — unique project lookup
- `CREATE INDEX ON :Spec(slug)` — slug lookup (not globally unique, scoped by BELONGS_TO edge)
- Similar indexes on Decision(slug), ExternalRef(external_id), etc.

**Note:** If profiling reveals the two-node pattern match is a bottleneck on hot paths (e.g., GetSpec called thousands of times), a denormalized `project` property can be added as an additive optimization. Start normalized, denormalize only with evidence.

---

## Server Lifecycle

### New CLI Commands

| Command | Purpose | Behavior |
|---------|---------|----------|
| `specgraph up` | Start the daemon | Idempotent. In `service` mode: generate + load user service. In `manual` mode: start foreground. Both ensure Docker container first if `docker: true`. |
| `specgraph down` | Stop the daemon | Stop the user service (or foreground process). Optionally stop Docker container with `--rm`. |
| `specgraph prime` | Session initialization | 1. `up` (idempotent). 2. Resolve project slug from CWD. 3. Register project in graph (if new). 4. Output orientation context (constitution summary, active specs, ready queue). |

### Server Modes

| Mode | When | Behavior |
|------|------|----------|
| `service` (default) | Local dev | Installs a user-level service (launchd on macOS, systemd on Linux). Auto-starts on login, auto-restarts on crash, OS manages lifecycle. |
| `manual` | CI, containers, debugging | `specgraph up` runs `specgraph serve` in foreground. Caller manages the process. |

### `specgraph up` Details

```text
specgraph up
  1. Load ~/.config/specgraph/config.yaml (write defaults if missing)
  2. Check health endpoint on configured address (server.listen)
     - Healthy → "Already running", exit 0
  3. If server.docker:
     - Ensure docker-compose.yaml at ~/.local/share/specgraph/
     - docker compose up -d --wait
  4. If server.mode == "service":
     a. Generate service definition (if not exists or config changed):
        - macOS: ~/Library/LaunchAgents/com.specgraph.server.plist
        - Linux: ~/.config/systemd/user/specgraph.service
     b. Load/enable the service:
        - macOS: launchctl bootstrap gui/$(id -u) <plist>
        - Linux: systemctl --user enable --now specgraph
  5. If server.mode == "manual":
     - exec specgraph serve (foreground, caller manages process)
  6. Health-check loop (max 10s, 500ms interval)
  7. Print "SpecGraph server running at http://..."
```

### `specgraph down` Details

```text
specgraph down [--rm]
  1. If server.mode == "service":
     - macOS: launchctl bootout gui/$(id -u)/com.specgraph.server
     - Linux: systemctl --user stop specgraph
  2. If --rm (remove service permanently):
     - macOS: delete plist file
     - Linux: systemctl --user disable specgraph, delete unit file
  3. If server.docker:
     - docker compose -f ~/.local/share/specgraph/docker-compose.yaml down --timeout 10
  4. Print "SpecGraph stopped"
```

### Service Definitions

**macOS** — `~/Library/LaunchAgents/com.specgraph.server.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.specgraph.server</string>
  <key>ProgramArguments</key>
  <array>
    <string>/path/to/specgraph</string>
    <string>serve</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>~/.local/state/specgraph/server.log</string>
  <key>StandardErrorPath</key>
  <string>~/.local/state/specgraph/server.log</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>SPECGRAPH_CONFIG</key>
    <string>~/.config/specgraph/config.yaml</string>
  </dict>
</dict>
</plist>
```

**Linux** — `~/.config/systemd/user/specgraph.service`:

```ini
[Unit]
Description=SpecGraph Development Server
After=docker.service

[Service]
Type=exec
ExecStart=/path/to/specgraph serve
Restart=on-failure
RestartSec=5
Environment=SPECGRAPH_CONFIG=%h/.config/specgraph/config.yaml

[Install]
WantedBy=default.target
```

Both reference the actual `specgraph` binary path (resolved at generation time via `os.Executable()`) and point to the global config. The service definitions are regenerated if the config changes (detected by comparing a hash of relevant config fields).

### `specgraph prime` Details

```text
specgraph prime
  1. Run `up` logic (idempotent)
  2. Find .specgraph.yaml by walking CWD upward to filesystem root (like git root discovery)
     - Found → read project slug (if present)
     - Not found or no project field → derive from git remote or directory name
  3. Resolve server for this project (config resolution order)
  4. Ensure project exists in graph:
     - GET project metadata → exists? done
     - Doesn't exist → create Project node with slug
  5. Output orientation context:
     - Project identity (slug, server)
     - Constitution summary (if set)
     - Table of non-terminal specs: slug, stage, priority (one row per spec)
```

**Note:** `specgraph prime` is the only entry point for the plugin's SessionStart hook. There is no separate `specgraph hook session-start` command — `prime` serves both interactive CLI users and hook callers. Its output is human-readable text; hooks capture it via stdout.

### `specgraph init` Details

```text
specgraph init [project-slug]
  1. Ensure server is running (calls `up` logic)
  2. Determine project slug:
     - Argument provided → use it
     - No argument → auto-derive from git remote or directory name
  3. Write .specgraph.yaml to repo root with project slug
  4. Create Project node in graph (if not exists) with:
     - slug, created_at, sync config (defaults: adapters=[])
  5. Constitution setup (interactive):
     - Prompt: "Create a constitution for this project? [Y/n]"
     - If yes → prompt for project name, principles, constraints, tech stack
     - Store constitution server-side via UpdateConstitution RPC
     - If no → skip (constitution can be added later via `specgraph constitution update`)
  6. Print summary: "Project <slug> initialized. Run `specgraph constitution show` to review."
```

**Non-interactive mode:** `specgraph init --yes` skips all prompts (creates project with defaults, no constitution). For CI and scripting. `specgraph init <slug> --constitution=path/to/file.yaml` imports a constitution from a file without prompting.

### `prime` vs `init` Relationship

Both `prime` and `init` can create a Project node — they are complementary, not sequential:

| Path | Who uses it | What happens |
|------|-------------|--------------|
| `specgraph init myproject` | Developer setting up a new repo | Writes `.specgraph.yaml`, creates Project node, optional interactive constitution setup |
| `specgraph prime` (no prior init) | Developer who skipped init, or Claude Code session | Auto-derives slug, creates Project node silently, no `.specgraph.yaml` written, no constitution |
| `specgraph prime` (after init) | Normal steady-state | Reads `.specgraph.yaml`, verifies Project node exists, dumps orientation context |

`init` is the **intentional setup** path — it writes the repo-local config file and offers interactive constitution creation. `prime` is the **idempotent session start** — it works with or without prior `init`, but the experience is better with it (explicit slug, constitution in place).

### `specgraph constitution import`

```text
specgraph constitution import [--project=<slug>] [file]
  - Reads YAML from file argument or stdin
  - Parses into Constitution domain type
  - Calls UpdateConstitution RPC for the specified project (or CWD-resolved project)
  - Prints: "Constitution imported for project <slug> (version N)"
```

### Project Node Schema

The `(:Project)` node in Memgraph:

```cypher
CREATE (p:Project {
  slug: $slug,                    -- unique identifier (kebab-case)
  created_at: datetime(),         -- when project was registered
  sync_adapters: $adapters,       -- list of active adapter names, e.g. ["beads", "github"]
  github_repo: $ghRepo,           -- owner/repo for GitHub adapter (optional)
  updated_at: datetime()          -- last modification time
})
```

Constitution is stored as a separate `(:Constitution)` node linked to the project:

```cypher
MATCH (p:Project {slug: $project})
CREATE (p)-[:HAS_CONSTITUTION]->(c:Constitution { ... })
```

All other domain nodes (Spec, Decision, ExternalRef, etc.) connect to the Project node via `[:BELONGS_TO]` edges — see "Graph Isolation" section for details.

### Server Process Model

In `service` mode (default), the server runs as a **user-level service** managed by the OS (launchd on macOS, systemd on Linux). This provides:

- **Auto-start on login** — no need to remember `specgraph up`
- **Auto-restart on crash** — KeepAlive (launchd) / Restart=on-failure (systemd)
- **Clean shutdown on logout** — OS sends SIGTERM
- **Log management** — standard OS log infrastructure
- **No PID files or lock files** — OS handles process lifecycle

In `manual` mode, `specgraph serve` runs in the foreground. This is for CI, containers, or developers who want explicit control.

The server process handles all projects — it receives the project slug via RPC request context (header or field), resolves the appropriate store scope, and delegates.

### Route Glob Matching

Route patterns in `~/.config/specgraph/config.yaml` use Go's `filepath.Match` semantics:

- `*` matches any sequence of non-separator characters
- `?` matches any single non-separator character
- `[...]` matches character ranges
- No `**` support (project slugs are flat, not hierarchical)
- Case-sensitive matching

---

---

## Claude Code Plugin

### Plugin Structure

```text
plugin/specgraph/
  plugin.json                          # Manifest
  hooks/
    session-start.sh                   # SessionStart → specgraph prime
  skills/
    specgraph/
      SKILL.md                         # Meta-skill: overview + router
      spark/SKILL.md                   # /specgraph-spark
      shape/SKILL.md                   # /specgraph-shape
      specify/SKILL.md                 # /specgraph-specify
      decompose/SKILL.md              # /specgraph-decompose
      approve/SKILL.md                 # /specgraph-approve
      list/SKILL.md                    # /specgraph-list
      show/SKILL.md                    # /specgraph-show
      deps/SKILL.md                    # /specgraph-deps
      ready/SKILL.md                   # /specgraph-ready
      bundle/SKILL.md                  # /specgraph-bundle
```

### Plugin Principles

1. **No business logic in the plugin.** Skills shell out to CLI commands. The server is the single source of truth.
2. **Skills are conversational wrappers.** They structure the AI interaction (elicitation probes, decision capture, routing) around CLI output.
3. **SessionStart hook calls `specgraph prime`.** This ensures the server is running and provides orientation context at session start.
4. **Graceful degradation.** If the server isn't reachable and can't be started, skills report the error clearly rather than failing silently.

### Skill Design Pattern

Each skill follows the same structure:

```markdown
---
name: specgraph-<command>
description: >
  <When to trigger — phrases, keywords, intent signals>
---

# SpecGraph <Stage>

## Prerequisites

- specgraph health (verify server)

## Workflow

1. Load context (show spec, constitution)
2. Interactive phase (elicitation probes, user conversation)
3. Persist results (CLI command with captured data)
4. Next steps (suggest next stage or action)
```

### Meta-Skill Routing Logic

The meta-skill (`/specgraph`) is the entry point when the user doesn't invoke a specific sub-skill. Its routing:

1. `$ARGUMENTS` contains a slug → `specgraph show $SLUG`, present results, suggest next action
2. `$ARGUMENTS` contains a keyword (spark, shape, list, etc.) → invoke that sub-skill via `Skill` tool
3. `$ARGUMENTS` describes a vague idea → suggest `/specgraph-spark`
4. `$ARGUMENTS` asks about status/progress → run `specgraph list --format=table`, present summary
5. `$ARGUMENTS` is empty → show available commands table, ask what the user wants to do

### SessionStart Hook

```bash
#!/usr/bin/env bash
# hooks/session-start.sh
exec specgraph prime 2>&1
```

Output is injected into the session context, giving the AI:

- Project identity and constitution summary
- Table of non-terminal specs (slug, stage, priority) — compact, queryable for details via CLI

### plugin.json

```json
{
  "name": "specgraph",
  "version": "0.1.0",
  "description": "SpecGraph: spec-driven development — author, query, and execute specs",
  "homepage": "https://github.com/specgraph/specgraph",
  "license": "MIT",
  "skills": [
    { "name": "specgraph", "path": "skills/specgraph/SKILL.md" },
    { "name": "specgraph-spark", "path": "skills/specgraph/spark/SKILL.md" },
    { "name": "specgraph-shape", "path": "skills/specgraph/shape/SKILL.md" },
    { "name": "specgraph-specify", "path": "skills/specgraph/specify/SKILL.md" },
    { "name": "specgraph-decompose", "path": "skills/specgraph/decompose/SKILL.md" },
    { "name": "specgraph-approve", "path": "skills/specgraph/approve/SKILL.md" },
    { "name": "specgraph-list", "path": "skills/specgraph/list/SKILL.md" },
    { "name": "specgraph-show", "path": "skills/specgraph/show/SKILL.md" },
    { "name": "specgraph-deps", "path": "skills/specgraph/deps/SKILL.md" },
    { "name": "specgraph-ready", "path": "skills/specgraph/ready/SKILL.md" },
    { "name": "specgraph-bundle", "path": "skills/specgraph/bundle/SKILL.md" }
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

---

## Implementation Scope

This design breaks into two major phases within Slice 7:

### Phase A: Global Daemon Infrastructure

- XDG config/data/state directory setup
- New global config schema (`~/.config/specgraph/config.yaml`)
- Per-repo `.specgraph.yaml` reader with project slug resolution (auto-derive from git remote)
- Config resolution chain (repo → routes → default)
- `specgraph up` / `specgraph down` commands (user-service generation + load, Docker lifecycle)
- `specgraph prime` command (up + project registration + orientation context)
- `specgraph init` rework (register project server-side, interactive constitution creation)
- `specgraph constitution import` command (import YAML from stdin/file)
- Graph namespacing — `[:BELONGS_TO]` edges from all domain nodes to `(:Project)` node
- Indexes on `Project(slug)`, `Spec(slug)`, `Decision(slug)`, etc.
- Update all Cypher queries in memgraph package to include `BELONGS_TO` edge pattern
- Remove old per-project config/constitution code (bootstrapConstitution, LoadConstitutionYAML, etc.)
- Existing `specgraph health` command already exists (Slice 1) — no changes needed

### Phase B: Claude Code Plugin

- plugin.json manifest
- SessionStart hook (`session-start.sh` → `specgraph prime`)
- Meta-skill (overview/router — dispatches to sub-skills based on user intent)
- Authoring skills (spark, shape, specify, decompose, approve)
- Query skills (list, show, deps, ready)
- Bundle skill

### Out of Scope (Phase 4)

- Multi-server federation
- PostToolUse progress hook (optional enhancement)
- `specgraph constitution export` for backup
- Remote server TLS/auth

---

## Open Questions (Resolved)

| Question | Decision |
|----------|----------|
| Per-project vs global server? | Global daemon |
| Constitution storage? | Server-side only (Memgraph) |
| Sync config storage? | Server-side only (Memgraph) |
| Graph isolation? | Edge-based (`[:BELONGS_TO]` edges to `(:Project)` node) |
| Server lifecycle? | User-level service (launchd/systemd) via `up`/`down`, `manual` mode for CI |
| Plugin SessionStart? | `specgraph prime` (combines up + context dump) |
| Project identity format? | Kebab-case slug, not forge-specific |
| Config resolution? | .specgraph.yaml server → global routes → global default |
| XDG compliance? | Yes — config/data/state in standard locations |
