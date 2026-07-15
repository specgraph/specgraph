---
name: specgraph-troubleshooting
summary: Diagnose stuck specs, drift loops, MCP errors, and other SpecGraph runtime issues.
description: Use when SpecGraph operations fail — MCP connection errors, init failures, missing configs, validation rejections, drift surprises, or auth failures. Triages the symptom and routes to the fix.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Troubleshooting

Common failure modes and their fixes. Start MCP-side: the `health` tool reports
server reachability and version, and the `specgraph://prime` /
`specgraph://graph` resources confirm the server is answering with real data.
Environment-level fixes (starting the stack, rewriting per-harness config) need
the local CLI — those live in the gated appendix at the end; MCP-only agents
skip them.

## MCP connection issues

**Symptom:** "MCP server unreachable" / "prime failed (server unreachable?)" at
session start.

**Triage (MCP-side first):**

1. Call the `health` tool — if it answers, the server is up and this is a
   client-config issue, not a down server.
2. Try reading `specgraph://prime` — a clean read confirms end-to-end MCP.
3. If `health` does not answer, the server or its DB container is down — that's
   an environment fix (see the CLI appendix: start the stack, then re-check
   `health`).

**Symptom:** "401 Unauthorized" on MCP calls.

**Cause:** the per-harness config expects `SPECGRAPH_API_KEY` in the
environment. The substitution syntax differs:

- Cursor: `${env:SPECGRAPH_API_KEY}`
- Claude Code: `${SPECGRAPH_API_KEY}`
- OpenCode: `{env:SPECGRAPH_API_KEY}`

**Fix:** set the env var, then re-run init (CLI appendix) to rewrite the config
with the right shape per file.

## Init and config failures

**Symptom:** "slug conflict" — init fails when `.specgraph.yaml` already has a
different slug.

**Fix:** decide which slug is right. If the existing one wins, drop the arg. If
the new one wins, edit `.specgraph.yaml` first, then re-run init.

**Symptom:** "ambiguous opencode config" — `opencode.jsonc` exists alongside
`opencode.json`.

**Fix:** OpenCode reads both, so we refuse to manage either. Pick one filename
and delete the other.

## Validation rejections

**Symptom:** "stage transition not allowed."

**Cause:** trying to go from spark straight to specify, or approve without a
decompose. The funnel is one-way; see `specgraph-conventions`.

**Fix:** run the missing stage first via the `author` tool.

**Symptom:** "invalid stage transition" immediately after an `amend`.

**Cause:** you tried to re-author the *same* stage the spec now sits at. After
`amend` with `re_entry_stage: <stage>`, the spec lands **one stage before**
`<stage>` — the land-one-before model. If you amended with
`re_entry_stage: shape`, the spec is at `spark`, so the valid next call is
`author action=shape`, not `author action=spark` (that would be a same-stage
no-op).

**Fix:** run `author action=<re_entry_stage>` — the stage named by the amend,
not the stage the spec landed at. See `specgraph-authoring`.

**Symptom:** "slug already in use."

**Fix:** slugs are unique per project. Either pick a different slug or update
the existing spec.

## Drift surprises

**Symptom:** drift detection reports every `DEPENDS_ON` edge, even ones that
look unchanged.

**Cause:** edges are unmigrated — they have an empty `content_hash_at_link`.
This always reports as drift until baselined.

**Fix:** acknowledge with the `drift` tool, `action: acknowledge` (baseline
every edge). See `specgraph-drift`.

## Auth flow

The server speaks ConnectRPC + MCP/HTTP and authenticates via bearer token.
MCP clients read `SPECGRAPH_API_KEY` from the environment through their
per-harness config. If a call returns 401, the key is missing or the config
substitution is wrong for that harness (see above).

## When all else fails

Capture the state before filing a bug:

- `health` tool output (server URL, version, reachability)
- `specgraph://prime` and `specgraph://graph` reads (is the graph answering?)
- `specgraph://findings` (any surfaced errors?)

For environment-level capture (Docker status, per-harness config paths,
last-write actions), use the CLI diagnostics in the appendix.

## Don't

- Don't assume a down server on the first error. Check the `health` tool — most
  "unreachable" reports are client-config, not a stopped server.
- Don't hand-edit per-harness MCP config files. Re-run init (CLI appendix); it
  reconciles them idempotently.

---

## Requires local CLI (source/CLI users only — MCP-only agents skip this)

These diagnostics and fixes need the `specgraph` binary on a local machine. An
MCP-only agent cannot run them — surface the symptom to the user and point them
here.

- Is the server running / is Docker reachable / start the stack:

  ```bash
  specgraph status
  specgraph up
  ```

- Reconcile per-harness MCP config (`.cursor/mcp.json`, `.mcp.json`,
  `opencode.json`) from the resolved server URL — run after the URL changes or
  a slug/config conflict:

  ```bash
  specgraph init
  ```

- Baseline unmigrated drift edges:

  ```bash
  specgraph drift acknowledge <slug> --all
  ```

- Full diagnostic capture for a bug report (resolved URL + project slug, Docker
  compose status, per-harness config paths and last-write actions, MCP
  connectivity check):

  ```bash
  specgraph status --verbose
  ```

The session-start hook uses `specgraph read-mcp-resource` and `specgraph prime`
under the hood; both read the same `SPECGRAPH_API_KEY` from the environment.
