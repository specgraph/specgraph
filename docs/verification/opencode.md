# OpenCode MCP verification

Empirical verification artifact for Phase B Slice 5 Task 35 (`spgr-bncv`).
Captures what OpenCode's MCP integration does on the wire when connected to a
local `specgraph serve`.

Verification date: 2026-05-04. OpenCode version: 1.14.30.
Server: `task dev` (post-PR-#927 main, includes Task 34's loopback fix).

## Setup that worked

### 1. Export the API key

OpenCode inherits env from the launching shell. Same key as Cursor (Task 34):

```bash
export SPECGRAPH_API_KEY=$(yq '.api_keys[] | select(.id=="default-admin") | .key' ~/.config/specgraph/credentials.yaml)
```

### 2. Project-level `opencode.json` (committed)

OpenCode auto-loads `opencode.json` at the project root by walking up to the
nearest git directory. The committed file uses OpenCode's `{env:NAME}`
substitution syntax (no leading `$`):

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "specgraph": {
      "type": "remote",
      "url": "http://127.0.0.1:7890/mcp/",
      "enabled": true,
      "headers": {
        "Authorization": "Bearer {env:SPECGRAPH_API_KEY}",
        "X-Specgraph-Project": "specgraph"
      }
    }
  }
}
```

Differences from the equivalent `.cursor/mcp.json`:

- Top-level key is `mcp` (singular), not `mcpServers`.
- Server entry needs `"type": "remote"` (parallels Claude Code's `"type": "http"`).
- Env-var substitution is `{env:NAME}` — no leading `$`. Cursor uses `${env:NAME}`,
  Claude Code uses `${NAME}`. Three clients, three syntaxes.
- File lives at the project root (no `.opencode/` subdirectory).

OpenCode merges configs rather than replacing, so a global
`~/.config/opencode/opencode.json` with other servers (Context7, etc.) coexists
with this project entry. Project entries override on conflicts.

## Captured behavior

### `clientInfo.Name = "opencode"` — matches existing mapping

Captured via the `mcp: client initialized` log line emitted by the
`OnAfterInitialize` hook in `internal/mcp/server.go` (added in PR #927):

```text
2026/05/04 13:41:03 INFO mcp: client initialized client_name=opencode client_version=1.14.30 profile=authoring
```

This **matches** the bare `"opencode"` already listed in
`internal/mcp/profiles.go:ProfileFromClientInfo` (added in Slice 4). No
profile-mapping fix needed; OpenCode sessions correctly resolve to
`ProfileAuthoring` without any code change. The
`TestClientIDContract`'s existing `OpenCode` row already covers this.

### Connection succeeds out of the box

OpenCode UI shows `specgraph connected ✓ Enabled` immediately after launch
with the project-level `opencode.json` in place. Bottom status bar reads
`1 MCP /status`. No restart-toggle or per-server logout/login dance like
Cursor required.

### Auto-invokes stage prompts on connect

Different from Cursor: OpenCode invokes the stage prompts (`spark`, `shape`,
`specify`, `decompose`) automatically during the connection phase, before
any user action. Captured as a cascade of `composer.invocation_failed`
warnings:

```text
2026/05/04 13:41:03 ERROR composer.invocation_failed stage=shape slug=$1 posture="" err="compose dynamic state: get constitution: not_found: constitution not found"
2026/05/04 13:41:03 ERROR composer.invocation_failed stage=decompose slug=$1 posture="" err="..."
2026/05/04 13:41:03 ERROR composer.invocation_failed stage=spark slug="" posture="" err="..."
2026/05/04 13:41:03 ERROR composer.invocation_failed stage=specify slug=$1 posture="" err="..."
```

The underlying error is `not_found: constitution not found` — same data-state
issue Cursor hit (the `specgraph` project doesn't have a constitution defined
yet). Run `specgraph constitution set` to populate.

The ERROR-level logging on these auto-invoke failures is louder than ideal
given the failure is data state, not a transport or auth defect. Filed as a
separate concern (see "What this PR does not address" below).

### `slug=$1` placeholder rendering

The auto-invoked prompt errors show `slug=$1` as a literal string in three
of the four invocations (`stage=spark` is the exception, with `slug=""`).
Hypothesis: OpenCode uses positional-arg placeholders when the prompt has
declared a required argument that the user hasn't bound yet. The MCP server
logs the literal argument value it received, so this is OpenCode's
auto-discovery behavior surfacing — not a bug in our composer.

Worth a follow-up: either OpenCode's behavior is documented and our prompts
should declare argument constraints differently, or OpenCode is sending
unfilled placeholders that should be treated as a no-op rather than an
explicit `$1` invocation. For now, the prompts are best invoked from
OpenCode with explicit slug values.

### Project-header propagation works

The Task 34 fix (PR #927's `auth.WithProject` / `ProjectFromContext`
threading) carries through cleanly: every loopback RPC authenticates and
dispatches with the correct project context. No `X-Specgraph-Project header
required` errors in the auto-invocation cascade — only data-state errors.

## Profile registry confirmation

OpenCode requires no changes to `internal/mcp/profiles.go` or
`TestClientIDContract`. The existing entries are correct as-is.

## What this PR does not address

- **Constitution data is empty** for the `specgraph` project — same out-of-scope
  state as Task 34. Populate via `specgraph constitution set`.
- **OpenCode auto-invocation noise** — the four `composer.invocation_failed`
  ERROR lines on every OpenCode connect are noisy and the failure is data
  state, not code. The composer could downgrade `not_found` errors to
  WARN-or-DEBUG when no slug is provided, since auto-discovery is expected
  behavior. Worth a separate bead if it gets annoying in practice.
- **`slug=$1` placeholder behavior** — investigate OpenCode prompt-arg
  semantics. May be a docs-only fix on our side (declare prompt args more
  explicitly), or a no-op. Not blocking.
- **Tasks 36 (Codex)** — same flow with a different client; expected to be
  fast since the infrastructure is in place.
