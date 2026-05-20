---
name: specgraph-troubleshooting
description: Use when SpecGraph operations fail — MCP connection errors, init failures, missing configs, validation rejections, drift surprises, or auth failures. Triages the symptom and routes to the fix.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Troubleshooting

Common failure modes and their fixes. When in doubt, run `specgraph init`
first — it's idempotent and reconciles per-harness MCP config files against
the global config.

## MCP connection issues

**Symptom:** "MCP server unreachable" / "specgraph prime failed (server
unreachable?)" at session start.

**Triage:**

1. `specgraph status` — is the server running?
2. `docker info` — is Docker reachable? The DB runs in a container.
3. `specgraph up` — start the stack if it's down.
4. `specgraph init` — rewrites `.cursor/mcp.json`, `.mcp.json`, and
   `opencode.json` from the resolved server URL. Run this if the URL
   recently changed.

**Symptom:** "401 Unauthorized" on MCP calls.

**Fix:** the per-harness config expects `SPECGRAPH_API_KEY` in the
environment. The substitution syntax differs:

- Cursor: `${env:SPECGRAPH_API_KEY}`
- Claude Code: `${SPECGRAPH_API_KEY}`
- OpenCode: `{env:SPECGRAPH_API_KEY}`

`specgraph init` writes the right shape per file.

## Init failures

**Symptom:** "slug conflict" — `specgraph init <slug>` fails when
`.specgraph.yaml` already has a different slug.

**Fix:** decide which slug is right. If the existing one wins, drop the arg.
If the new one wins, edit `.specgraph.yaml` first, then re-run init.

**Symptom:** "ambiguous opencode config" — `opencode.jsonc` exists alongside
`opencode.json`.

**Fix:** OpenCode reads both, so we refuse to manage either. Pick one
filename and delete the other.

## Validation rejections

**Symptom:** "stage transition not allowed."

**Cause:** trying to go from spark straight to specify, or approve without a
decompose. The funnel is one-way; see `specgraph-conventions`.

**Fix:** run the missing stage first.

**Symptom:** "slug already in use."

**Fix:** slugs are unique per project. Either pick a different slug or
update the existing spec.

## Drift surprises

**Symptom:** drift detection reports every DEPENDS_ON edge, even ones that
look unchanged.

**Cause:** edges are unmigrated — they have an empty
`content_hash_at_link`. This always reports as drift until baselined.

**Fix:** `specgraph drift acknowledge <slug> --all` baselines all the
spec's edges. See `specgraph-drift`.

## Auth flow

The server speaks ConnectRPC + MCP/HTTP and authenticates via bearer token.
The CLI reads `SPECGRAPH_API_KEY` from the environment for both
`specgraph` commands and `specgraph read-mcp-resource` (used by the
session-start hook). If the env var isn't set, the CLI tells you which
harness config expects it.

## When all else fails

`specgraph status --verbose` dumps:

- Resolved server URL and project slug
- Docker compose status
- Per-harness config file paths and last-write actions
- MCP connectivity check

That's the right pre-bug-report capture.
