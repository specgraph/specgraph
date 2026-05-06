# Claude Code MCP verification

Empirical verification artifact for Phase B Slice 5 (`spgr-bncv`).
Captures what Claude Code's MCP integration does on the wire when connected to
a local `specgraph serve`.

Verification date: 2026-05-05. Claude Code version: 2.1.128.
Server: `./specgraph serve` on latest `main` after PR #934.

## Setup that worked

### 1. Export the API key

Claude Code reads project-level MCP config from `.mcp.json`, but the
`SPECGRAPH_API_KEY` placeholder is resolved from the environment of the shell
that launched Claude Code. If the variable is absent, the config file still
parses as JSON, but Claude Code cannot authenticate the HTTP MCP server.

```bash
export SPECGRAPH_API_KEY=$(yq '.api_keys[] | select(.id=="default-admin") | .key' ~/.config/specgraph/credentials.yaml)
```

Do not commit or paste the resolved key. The committed `.mcp.json` keeps the
secret as `${SPECGRAPH_API_KEY}`.

### 2. Project-level `.mcp.json`

Claude Code reads `<project>/.mcp.json`. The committed file uses Claude Code's
`${NAME}` environment-substitution syntax and requires an explicit HTTP server
type:

```json
{
  "mcpServers": {
    "specgraph": {
      "type": "http",
      "url": "http://127.0.0.1:7890/mcp/",
      "headers": {
        "Authorization": "Bearer ${SPECGRAPH_API_KEY}",
        "X-Specgraph-Project": "specgraph"
      }
    }
  }
}
```

This differs from Cursor (`${env:SPECGRAPH_API_KEY}`) and OpenCode
(`{env:SPECGRAPH_API_KEY}`).

## Captured behavior

### Missing env key surfaces as auth failure

Before `SPECGRAPH_API_KEY` was present in the launching environment, Claude
Code's `/mcp` status showed:

```text
Status: failed
Auth: not authenticated
SDK auth failed: Failed to parse JSON
```

The committed `.mcp.json` validates with `python3 -m json.tool .mcp.json`, so
the useful diagnosis is not "the file is malformed." The failure means Claude
Code attempted its auth flow without a usable bearer token.

### Authenticated connection succeeds

After exporting `SPECGRAPH_API_KEY` and reconnecting, Claude Code showed:

```text
Status: connected
Auth: authenticated
Capabilities: tools resources prompts
Tools: 21 tools
```

The server's MCP initialization log confirmed the client profile:

```text
INFO mcp: client initialized client_name=claude-code client_version=2.1.128 profile=authoring
```

This matches `internal/mcp/profiles.go:ProfileFromClientInfo` and the
`TestClientIDContract` row for `claude-code`.

## Profile registry confirmation

Claude Code requires no profile-mapping change. The first-party
`clientInfo.name` is `claude-code`, which maps to `ProfileAuthoring`.

## What this verification does not address

- **Codex verification** is deferred until there are users asking for Codex
  support. Cursor, OpenCode, and Claude Code now have empirical artifacts;
  Codex remains declared but not exercised by Phase B.
- **Environment propagation ergonomics** remain a user setup concern. Claude
  Code must be launched from an environment where `SPECGRAPH_API_KEY` is set,
  or the OS launch environment must provide it.
