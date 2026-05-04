# spgr-7htb: idempotent `specgraph init` with managed per-harness MCP configs

## Context

After Phase B Slice 5 Tasks 34 and 35, the SpecGraph repo carries three
hand-maintained MCP config files at root:

- `.cursor/mcp.json` (Cursor — `${env:NAME}` env-var syntax)
- `.mcp.json` (Claude Code — `${NAME}` env-var syntax, `"type": "http"`)
- `opencode.json` (OpenCode — `{env:NAME}` env-var syntax, `"type": "remote"`, `mcp.*` schema)

Codex (Task 36) will add a fourth when work-machine access is available.

These files all encode the same fundamental information — local server URL +
bearer auth via env-var + `X-Specgraph-Project: <slug>` — in three different
shapes and three different env-var substitution syntaxes. Hand-maintaining them
creates drift risk (change the slug or server URL → update N files) and
onboarding burden (each contributor needs to know which file matches which
client).

A WebFetch-grounded survey of harness MCP configuration support (2026-05-04):

| Harness | Repo-committable MCP config | Plugin embeds MCP | Skill declares MCP |
|---|---|---|---|
| Cursor | `.cursor/mcp.json` (only path) | NO — Marketplace plugins are one-click installs, not repo-distributed | NO — Rules in `.cursor/rules/` are instruction text only |
| Claude Code | `.mcp.json` at project root | technically YES (`.mcp.json` at plugin root) but rejected for consistency | NO — skills are instruction text |
| OpenCode | `opencode.json` at project root | NO — plugins are JS/TS modules; no MCP API per docs | NO — skill manifest only 5 fields |
| Codex | TBD (Task 36) | TBD | TBD |

Conclusion: NO harness supports skill-declared MCP. Cursor and OpenCode have
only one repo-committable shape (per-client root file). To avoid asymmetric
special-casing, every supported harness gets exactly one config file at a
documented project path; `specgraph init` generates and maintains all of them.

## Decision: idempotent `specgraph init` with managed-fields semantics

`specgraph init` is the single entry point for both fresh setup and ongoing
config reconciliation. Safe to run as often as the user wants; output converges
to a function of `.specgraph.yaml` + global config.

### Sequence

1. Resolve project state: read `.specgraph.yaml` if present; derive slug from
   arg / git remote / dirname if not.
2. **Slug-consistency check:** if both an arg and an existing `.specgraph.yaml`
   are present and the slugs differ, return error and refuse to write.
3. Write `.specgraph.yaml` (no-op if content unchanged).
4. Resolve server URL via the existing `(*GlobalConfig).ResolveServer(slug, repoOverride)`
   method on `internal/config/global.go`. The `repoOverride` argument is the
   optional `server:` field from `.specgraph.yaml`'s `ProjectConfig.Server`;
   if set, it takes precedence over global routing. The resolved URL is the
   value baked into each per-harness config's `url` field.
5. Sync the three per-harness MCP configs via JSON Merge Patch.
6. Print per-file action (`created` / `updated` / `no-op`).

**Behavior change vs current `runInit`:** the existing `runInit` calls `runUp`
to bring the Docker compose stack online before writing `.specgraph.yaml`. The
new idempotent flow drops that step — init is config-only. Users who need the
server brought up should run `specgraph up` separately (existing command).
Reasoning: `runUp` blocks for up to ~10 seconds on a cold start polling Docker
container health (the 10-retry × 1-second-sleep loop in `cmd/specgraph/up.go`) and produces visible Docker
compose output; that's surprising for a command meant to be run as often as
the user wants. Cleaner separation: `up` owns the server lifecycle; `init`
owns project + config files.

### Mutation primitive: JSON Merge Patch (RFC 7396)

Apply via `github.com/evanphx/json-patch/v5`'s `MergePatch(doc, patch []byte) ([]byte, error)`.
Each per-harness writer constructs a patch document containing only the fields
we manage; merging into an existing file replaces our managed keys and
preserves all siblings (other servers, top-level fields the user has added,
user customizations under our `specgraph` entry).

**New module dependency.** `github.com/evanphx/json-patch/v5` is not currently
in `go.mod`. The implementation plan must add it as a prerequisite step:
`go get github.com/evanphx/json-patch/v5 && go mod tidy`. The library is
widely used in the Kubernetes/Operator-SDK ecosystem and supports both RFC
6902 (JSON Patch) and RFC 7396 (JSON Merge Patch) — we use only the latter.

### Managed fields

The three harnesses use different top-level keys for the MCP server map:

- Cursor and Claude Code: `mcpServers.<server-name>.*` (plural)
- OpenCode: `mcp.<server-name>.*` (singular)

Each per-harness merge patch targets the right top-level key, with
`<server-name>` always `specgraph`. Within our `specgraph` entry, the managed
fields are:

- **All three:** `url`, `headers.Authorization`, `headers.X-Specgraph-Project`
- **Claude Code:** plus `type: "http"`
- **OpenCode:** plus `type: "remote"`, `enabled: true`, and top-level `$schema`
  (sibling of `mcp`, not under our entry)

Anything else under our `specgraph` entry is the user's: custom headers
(e.g., `X-Custom: foo`), comments, custom `timeout`, etc. — all preserved
across `init` runs.

### Per-developer overrides

Documented path is the harness's user-level config (`~/.cursor/mcp.json`,
`~/.config/opencode/opencode.json`, etc.) which takes precedence over
project-level in clients that merge user+project. Editing the project-level
file directly is unsupported — `init` will reset managed fields on the next
run.

## Decision: error on conflicting slug arg

If the user passes `specgraph init <slug>` and `.specgraph.yaml` already
exists with a different slug, refuse with:

```text
cannot change project slug from <existing> to <arg>; edit .specgraph.yaml directly or remove it
```

The slug is identity-defining. It is the storage partition key for the
project's specs, decisions, and findings (every Postgres query carries the
slug as a partition column — see `internal/storage/postgres/`); the
value of the `X-Specgraph-Project` header sent on every MCP request; and is
referenced by global-config server-routing rules (`Routes` in `GlobalConfig`).
Silently mutating it would orphan the project's existing data and break
header-based routing. Forcing the user to edit the YAML or remove it makes
the intent explicit.

## Decision: error on `opencode.jsonc` sibling

OpenCode is the only managed harness whose docs reference both `opencode.json`
and `opencode.jsonc` as supported config formats. Lookup order between them
isn't explicitly documented for project-root configs, so writing
`opencode.json` next to a user's pre-existing `opencode.jsonc` could leave
either file as the active one — at best confusing, at worst silently hiding
servers the user already configured in their `.jsonc`.

If `opencode.jsonc` exists in the project root, `init` refuses with:

```text
found opencode.jsonc alongside opencode.json; consolidate to one file (init does not yet manage opencode.jsonc)
```

The check is OpenCode-specific. Cursor and Claude Code MCP docs don't mention
JSONC; not checking `.cursor/mcp.jsonc` or `.mcp.jsonc` until there's
documented basis to do so. Comment-preserving merge over JSONC is deferred
to a future task; how that's implemented belongs in that task's design.

## Decision: out of scope

- **Codex MCP config** — added when Task 36 resumes (no work-machine access today).
- **`--force` flag** — manual override of slug-conflict / `opencode.jsonc` errors. YAGNI for now.
- **Drift detection / `specgraph configs status`** — surfaces "your file has fields we don't manage" diffs. Future enhancement.
- **Auto-running `init` in CI / post-merge hook** — out of scope; leave to user discretion.
- **Migrating the SpecGraph repo's currently-committed configs** — handled by
  Task 38 (dogfood cutover) when this repo gets `specgraph init`-ed.

## Rejected alternatives

### Separate `specgraph configs sync` command

Added surface area for no benefit. `init` already owns project setup; making
it idempotent makes a separate sync command redundant.

### Phase 1 add-only / Phase 2 merge

Overcautious for greenfield work. The "smart merge that does the right thing
every run" is the shipped behavior; no need to pre-stage with a less capable
variant.

### Plugin-bundled MCP for Claude Code (`plugin/specgraph/.mcp.json`)

Claude Code's plugin schema does support `.mcp.json` at plugin root, but using
that path for Claude Code while writing a project-root file for Cursor and
OpenCode creates asymmetry. Consistency across all three harnesses is more
valuable than the small surface-area win.

### Skill-bundled MCP

Empirical WebFetch survey on 2026-05-04 confirmed no harness supports
skill-declared MCP. Skills are instruction text only across Cursor (rules),
Claude Code (skills), and OpenCode (skills).

### Full-file overwrite of managed configs

Clobbers any other MCP servers the user has configured. Hostile to users with
multi-server setups.

## Acceptance criteria

- `specgraph init` for a fresh project writes `.specgraph.yaml` plus the three
  per-harness MCP configs, each with the correct shape and env-var syntax.
- `specgraph init` for an already-initialized project is a no-op when nothing
  has changed; reports `no-op` per file.
- Re-running `specgraph init` after editing `.specgraph.yaml`'s slug or
  server URL updates managed fields in all three configs, preserves
  user-added fields under our `specgraph` entry, and preserves any other MCP
  servers in those files.
- Slug arg conflict produces a clear error and refuses to write.
- `opencode.jsonc` sibling produces a clear error and refuses to write
  (OpenCode-only; no equivalent check on Cursor or Claude Code).
- Test coverage: per-harness golden-file tests for `ManagedConfigs`,
  table-driven `Sync` tests covering all merge scenarios, init integration
  tests covering the slug × existing matrix, and an idempotence test where
  `init` runs twice in succession and the second run produces byte-equal
  output to the first (the first run may reformat pre-existing pretty-printed
  or compact JSON to the canonical 2-space-indent + trailing-newline form;
  subsequent runs against canonical content are byte-equal no-ops). The
  no-op detection in `Sync` compares post-canonicalization bytes — `Sync`
  marshals merge-patch output to canonical form before comparing against the
  existing file's bytes, so format-only differences trigger an `updated`
  action on first run and `no-op` thereafter.

## Origin

Phase B Slice 5 design observation surfaced during Tasks 34-35: per-client
root config sprawl across Cursor + Claude Code + OpenCode (+ Codex eventually)
isn't sustainable; consolidation belongs in CLI tooling (`specgraph init`),
not contributor knowledge. Bead `spgr-7htb`.
