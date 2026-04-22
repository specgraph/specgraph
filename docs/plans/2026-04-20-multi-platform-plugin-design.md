# Multi-Platform Plugin Design

> **Date**: 2026-04-20
> **Status**: Draft
> **Phase**: 3 (Coordination & Export)
> **Tracking**: spgr-mv32
> **Depends on**: 2026-04-10 MCP Server Design (merged via PR 897, 898)

## Overview

Replace the single-platform (Claude Code) plugin with thin, structurally-equivalent plugins for Cursor, OpenCode, Codex, and Claude Code. All four consume the same MCP-delivered workflow guidance and share no platform-specific behavioral intelligence. Rich authoring workflow content — persona, orchestration protocol, quality heuristics, stage guidance, output schemas — migrates from platform-specific skill files into server-embedded content that composes dynamically into MCP prompt responses with current constitution and spec state.

### Goals

- Structural parity across Claude Code, Cursor, OpenCode, and Codex: each platform has equivalent access to the full SpecGraph authoring, query, and execution surface
- Eliminate the staleness risk of per-repository guidance files by making MCP the single live source of workflow intelligence
- Keep plugins thin: MCP config, session-start mechanism, routing guide, optional slash commands, init bootstrap
- Preserve the existing stateless MCP server design; no session state introduced
- Leave the door open to selective server-side workflow enforcement later, driven by telemetry

### Non-Goals

- Identical output quality across platforms (LLM capability differences are real and unresolved by this design; parity is of delivered guidance and capability, not of generated content)
- Server-side session state or conversation tracking beyond what is already persisted (specs, conversation records, findings)
- Deprecating the CLI path; `specgraph` CLI commands remain available and increasingly share implementation with MCP prompt composition
- Backward compatibility with the current 13-skill Claude Code plugin — clean break, no dual path

## Guiding Principle

**Plugins carry only stable meta-knowledge about *using* the MCP externally. Anything project-specific, data-driven, time-varying, or referenced during prompt composition lives server-side.**

This principle resolves earlier drafts that proposed injecting persona, posture, and orchestration guidance into `.cursor/rules/` and `AGENTS.md` per-project. That approach recreates exactly the staleness MCP was designed to eliminate — user edits diverge from server evolution, per-platform copies drift, the whole point of a live context protocol is lost.

Refined rule: stable meta-knowledge that is *referenced during MCP prompt composition* lives server-side alongside the composition. Stable meta-knowledge that is *only* about invoking MCP externally (routing guide, MCP config, slash command definitions) lives in plugins.

## Content Boundary

| Category | Location | Mechanism | Examples |
|----------|----------|-----------|----------|
| Plugin content | Per-platform plugin files | Native platform format | MCP server config, session-start hook, routing guide, slash commands, init bootstrap instructions |
| Server-embedded stable content | `internal/authoring/content/` (new) | `//go:embed` markdown | Persona concept, posture rules, orchestration protocol, conversation recording protocol, quality heuristics, stage definitions, output format schemas (derived from proto) |
| Server-composed dynamic content | Storage + composer | Live query, composed per invocation | Current constitution summary, current spec state, related spec summaries, current findings |

## Architecture

### Delivery Flow

```text
Platform plugin                                  specgraph server
───────────────                                  ─────────────────
[1] session start   ──reads──▶  specgraph://prime resource
                                     ↓ composes from
                                     constitution + graph + ready + findings

[2] user intent    ──invokes──▶ MCP prompt (e.g., shape)
    on stage work                    ↓ composes from
                                     embedded stable content
                                   + current constitution (summary)
                                   + current spec state (summary)
                                   + related spec summaries
                                   + output format schema
                                     ↓
                                   returns rich composed message

[3] LLM conducts conversation with user, acts via MCP tools:
    - author.shape / author.specify / author.decompose — persist stage output
      AND conversation exchanges atomically in a single call (see Conversation
      Recording Coupling section)
    - analytical_pass.run — auto-dispatched per posture + stage policy
    - graph_query, spec.get — pull detail as needed
    - conversation.record — used for amendments and approve-stage rejections only
```

### Stateless Server (with bounded exceptions)

Every MCP tool and prompt invocation is a pure function of its arguments plus persisted state (specs, conversation records, findings, constitution). The server maintains no per-session in-memory conversation state across tool calls.

Rationale: every capability attributed to a stateful server (posture detection, continuation, step gating enforcement, workflow rejection) reduces for SpecGraph's model to either (a) reading persisted state, (b) accepting explicit arguments, or (c) client-side LLM reasoning that the server has no better handle on. Stateful conversation tracking adds session lifecycle complexity, scaling concerns, and reconnection semantics without a unique capability that justifies them.

**Distinction — stateful server vs. stateful data**: stateless *server* does not mean stateless *data*. The server persists rich domain data (specs, conversation records, findings, constitution, posture recorded alongside exchanges) and every handler reads from that persistent store. What it does not maintain is per-session in-memory scratch state across tool calls. Posture recording for future drift detection is stateful-data, not stateful-server.

**Bounded exception — per-connection capability routing**: during MCP `initialize`, the server inspects the client's declared `prompts` / `tools` capabilities and remembers that inspection for the life of the connection to decide which delivery channel to advertise. This is a small amount of per-connection state that survives until disconnect. It is not conversation state; it is a one-time handshake artifact. Documented here so the "stateless" claim is honest about its boundaries.

**Atomicity precedent**: SpecGraph already applies server-managed atomicity where the client shouldn't carry the burden — the `HAS_CHANGE` edge (Spec → ChangeLog) is created automatically by the storage layer on mutations rather than via a separate client call (see `CLAUDE.md`). The Conversation Recording Coupling section extends the same principle to conversation exchanges on stage transitions.

**Forward compatibility**: MCP protocol supports session IDs at the transport layer. Adding optional session conversation state later is additive — new tools or hooks opt in without breaking existing stateless calls. Starting stateful-conversation and removing later is not symmetrically safe, so the door is intentionally left open in the stateless direction.

### Posture

The Drive / Partner / Support posture system survives with these properties:

- **Concept**: Stable behavioral framework — Drive drafts aggressively and seeks approval; Partner proposes, discusses, iterates; Support asks user first and offers to draft when stuck
- **Detection**: LLM-side, based on the user's direct message (LLM has strictly more signal than the server could infer from tool-call residue)
- **Delivery**: Rules and heuristics embedded server-side, composed into MCP prompt responses. The LLM reads the rules, applies them to the live user message, and decides the posture
- **Transmission**: Explicit `posture` argument on `author.*` tool calls and any other tool whose behavior varies by posture
- **Server default**: On absence of posture, server defaults to Partner — matches today's skill default — AND emits a warning log + metric tagged "posture-absent" on each such call. Silently defaulting would hide an LLM bug (the LLM should have detected and passed posture). The warning surfaces the gap for telemetry without breaking the call. Phase 2 may tighten this to reject in authoring profiles once telemetry shows whether absent-posture is rare or common
- **Recording**: Server records posture with each call (alongside conversation exchanges) so a future drift detector has data. Detector itself is not in this design's scope

## MCP Prompt Composition

### Structure: A + (ii)

One composed message per prompt invocation (Option A) containing stable content in full and dynamic state as concise summaries with explicit resource pointers for full detail (Option ii).

A rich composed `shape` prompt response, sectioned:

1. **Stage framing**: identity ("You are running the shape stage"), purpose, relationship to preceding/succeeding stages
2. **Posture rules**: how to adapt behavior by posture; default-Partner handling
3. **Current state summary**: spec slug, intent, prior-stage content (spark output summary), constitution key constraints/antipatterns (bounded), related specs inline (slug + relationship type)
4. **How to proceed**: stage-specific elicitation moves, step ordering, step-gating guidance
5. **Output format**: JSON schema for `ShapeOutput`, derived from proto, with example
6. **Orchestration**: how to call `author.shape` with the stage output and required `conversation_exchanges` argument (server-side coupling guarantees they land together); which analytical passes auto-dispatch per posture + severity policy
7. **Quality gates**: heuristics to apply, pushback protocol, pointer to fuller persona context
8. **Pointers**: `specgraph://constitution` for full constitution, `specgraph://spec/{slug}` for full related specs, `graph_query` tool for graph traversal

### Token Budget

Measured against current skill content (2026-04-20): `persona.md` ≈870 tokens, `analytical-passes.md` ≈960, `conversation-recording.md` ≈635, per-stage SKILL.md ranges 990–2,117, output format schemas ~250 each. A composed `shape` prompt that includes persona + orchestration subset + recording protocol + quality heuristics + stage-specific guidance + output schema lands around **3,500–4,000 stable tokens** before any dynamic state.

- **Target**: ~7K tokens typical, 12K tokens worst case
- **Rationale**: fits in every target client's context window (Cursor/Claude/OpenCode/Codex all >32K) with substantial room for system prompt and conversation history; deduplication of persona/orchestration/recording (embedded once server-side vs. symlinked into every stage skill today) yields no net increase versus current skill-driven delivery
- **Stable core**: ~3.5–4K tokens per stage prompt
- **Dynamic state**: ~1–3K tokens typical, up to ~5K on mature projects with large constitutions
- **Over-threshold handling**: when a constitution, spec, or related-spec set exceeds the dynamic-state sub-budget, the composer includes a bounded digest (top N constraints, top M antipatterns, intent + stage + key fields, top K related specs by edge weight) and an explicit pointer to the full resource URI. Truncation markers read `(truncated; read specgraph://... for full content)` so the LLM knows to pull detail if the conversation demands it.

**Validation prerequisite**: before implementation is declared complete, produce a measured composed response for each stage and record token counts. If the stable core exceeds ~5K tokens, the composer must be restructured — likely by extracting posture, orchestration, and quality-heuristics content into separate MCP resources (`specgraph://guidance/posture`, `specgraph://guidance/orchestration`, `specgraph://guidance/heuristics`) that the LLM pulls on demand rather than inlining in every stage prompt. This restructure preserves the principle (still server-side, still composable, still not project-specific) but shifts delivery from push to pull. Keep this as the documented fallback if measurement shows inline composition too heavy.

### Implementation Layout

```text
internal/
├── authoring/
│   ├── content/              # NEW — //go:embed source
│   │   ├── persona.md
│   │   ├── orchestration.md
│   │   ├── conversation-recording.md
│   │   ├── quality-heuristics.md
│   │   ├── stage-spark.md
│   │   ├── stage-shape.md
│   │   ├── stage-specify.md
│   │   ├── stage-decompose.md
│   │   └── stage-approve.md
│   ├── composer.go           # NEW — composes prompt responses from embedded + dynamic
│   ├── prompts.go            # EXISTING — becomes thin wrapper around composer
│   ├── passes.go
│   └── ...
├── mcp/
│   ├── prompts.go            # EXISTING — delegates to authoring.Composer
│   ├── resources.go          # EXISTING + new specgraph://prime resource
│   ├── profiles.go           # EXISTING — adds opencode, codex mappings
│   └── ...
```

**`authoring.Composer`** is the single entry point for rich prompt composition. It:

- Loads stable content via `//go:embed`
- Queries persisted state (constitution, spec, related specs, findings) via existing storage interfaces
- Summarizes when over token budget
- Returns a structured result the MCP prompt handler converts to `PromptResult` messages, or the `author.start_stage` tool handler returns as tool content

### Package boundary and dependency direction

Today `internal/authoring/` is template-only — `prompts.go` returns static strings with no storage dependency. Introducing `composer.go` pulls `internal/authoring/` into the storage dependency graph (it needs to read constitution, spec, related specs, findings). This is a real boundary change worth noting:

- The composer depends on storage interfaces in `internal/storage/`, not on any concrete backend
- This matches the existing pattern in `internal/server/` handlers (depend on interfaces, not implementations)
- `internal/authoring/passes.go` and friends may already have implicit storage coupling; composer formalizes the pattern
- No cyclic dependency: `internal/server/` consumes `internal/authoring/`; `internal/authoring/` does not reach back

### Composer observability

The composer emits structured logs and metrics on each invocation:

- **Metrics**: composition duration, total output tokens, stable-core tokens, dynamic-state tokens, truncation count (how many dynamic sections were digested rather than inlined), per-stage invocation rate
- **Logs**: prompt/tool invocation (stage, slug, posture), source storage query timings, any truncation that occurred (which section, original size, digested size)

These are operational metrics for composer health, distinct from the phase-2 LLM-behavior telemetry deferred elsewhere in the design. Composer observability ships with phase 1.

### Embedded content versioning

Embedded markdown content is versioned by the server binary version. Each composer response includes a compact footer with the server version so downstream analysis (or user-visible debug) can attribute guidance to a specific release. In a long-lived MCP session that spans a server upgrade, clients will see differently-versioned guidance on successive invocations; this is expected and stateless-consistent — no session-bound cache of guidance content exists to go stale.

## Conversation Recording Coupling

### Regression to address

PR 894 made persist + record "structurally inseparable" in the existing skill files by wrapping both commands in the same bash block with retry/trap/abort logic. That is shell-level enforcement — the bash script cannot succeed at persist without then succeeding at record, or aborting both.

Moving this guarantee into MCP prompt prose ("persist and record are the same step, not optional") is a real regression. LLMs follow prose instructions imperfectly, especially under context pressure. The original fix for PR 894 was necessitated precisely because earlier prose-level instructions were being skipped.

### Alternatives considered

Two mechanisms can close this regression server-side:

**A. Required-argument atomic coupling** (chosen). `conversation_exchanges` becomes a required argument on `author.shape` / `author.specify` / `author.decompose`. The server persists stage output and conversation record in a single transaction.

**B. Persisted-state gate on stage transitions** (rejected). Keep `conversation_exchanges` optional; the `author.*` handler reads from storage and rejects the transition if no conversation record exists for the spec at this stage within the current amend cycle.

A chosen over B because B introduces an ordering dependency — the LLM must call `conversation.record` before `author.shape`. If the LLM calls in the wrong order, the persist fails; if it then records and retries, that's a multi-step retry dance that restores the exact reliability problem PR 894 fixed. A lands both writes in a single call with atomic commit, eliminating the ordering problem entirely.

Pattern precedent: SpecGraph's `HAS_CHANGE` edge pattern (noted in `CLAUDE.md`) is already a case of server-managed atomicity — the change log edge is created automatically by the storage layer, not by a separate client call. Option A extends the same principle to conversation recording.

### Validation (what the required argument actually enforces)

The reviewer observation "required argument catches forgot-to-call but not fake content" is correct. To close that gap as far as server-side validation reasonably can, the server validates `conversation_exchanges` on each `author.*` call:

- **Non-empty**: at least one exchange. Empty array is rejected with `CodeInvalidArgument`.
- **Per-exchange structure**: each exchange has a non-empty `role` and non-empty `content`. Missing either rejects the whole call.
- **Stage tag match**: each exchange's `stage` field (if present on `ConversationExchange`) must equal the stage of the `author.*` call. Prevents passing unrelated exchanges from a prior stage.
- **Sequence monotonicity**: the `sequence` field on each exchange (if present) must be strictly increasing. Prevents shuffled/duplicate-by-copy submissions.

These checks catch the trivially-fake cases (empty arrays, duplicated structural noise, mismatched stages). They do not catch sophisticated fabrication (the LLM inventing a plausible-looking conversation). That residual gap is an LLM-behavior problem outside the server's reach at this layer; it is surfaced to the phase-2 telemetry work as a detection target ("do recorded exchanges match what the prompt template would have elicited?").

### Interaction with the standalone `conversation.record` tool

The standalone tool remains registered and keeps the existing `Authoring.RecordConversation` RPC. Its role narrows to two cases:

- **Amendments**: re-entering an already-approved stage records the amendment conversation via the standalone tool.
- **Out-of-band additions**: explicit user requests to augment a prior stage's record.

Approve-stage rejections are handled atomically via `Approve(action=APPROVE_ACTION_REJECT)`: the server records both the conversation log (under the `approved` stage) and the `approve-rejected` critical finding in a single transaction. The standalone `conversation.record` RPC is NOT used for approve-stage rejections — the atomic `Approve` RPC is the correct path.

When `author.*` atomic recording has already written exchanges for a stage, calling `conversation.record` for the same stage + spec appends rather than replaces. The server does not dedupe — exchanges are additive.

### Approve and spark coupling — resolved

| Stage | Required `conversation_exchanges`? | Rationale |
|-------|-----------------------------------|-----------|
| `spark` | **Optional** | Spark is often terse (single `--seed`); forcing exchanges on trivial sparks is friction. When exchanges present, server validates per rules above |
| `shape` | **Required** | Core authoring stage; skill prose today says "required" |
| `specify` | **Required** | Core authoring stage; skill prose today says "required" |
| `decompose` | **Required** | Core authoring stage; skill prose today says "required" |
| `approve` | **Optional — required on rejection** | Approve records only when the user rejects. Approve tool accepts optional `conversation_exchanges`; server requires them if the approve action is "reject" |

This resolves the design-level contract; no open question remains for implementation.

### Trade-offs of the chosen approach

- **Proto change**: new field on `ShapeRequest`, `SpecifyRequest`, `DecomposeRequest`, conditionally on `SparkRequest` / `ApproveRequest`; must be handled per proto evolution conventions
- **Atomic transaction complexity**: the server must persist two records in a single transaction. `internal/storage/postgres/tx.go` already has `RunInTransaction`, so this is a straightforward application of an existing pattern
- **LLM burden**: the LLM must accumulate conversation exchanges in its own context window and pass them on the `author.*` call. This is already what the skills ask it to do; the protocol simply makes it required

## Resources

### New: `specgraph://prime`

Returns a composed session-priming document: current constitution summary, graph state overview (spec counts by stage), ready specs list, in-progress specs, open findings summary. Read by each platform's session-start mechanism.

**Why resource, not prompt**: resources are listable and subscribable; semantically "read this to know the state." Prompts are for "start this workflow." Priming is the former.

**Token budget**: ~2K typical, 4K worst case. Prime is read on every session start across every platform — it is the hottest path in the server's MCP footprint. Composition rules:

- Constitution: top-level summary only (primary language, key constraints top 5, antipatterns top 5) — full constitution at `specgraph://constitution`
- Graph overview: counts by stage only, not per-spec listings
- Ready specs: top 10 by priority (slug + intent); link to `specgraph://graph/ready` for full list
- In-progress specs: top 10 by recent activity (slug + stage); link to `specgraph://specs?stage=in-progress`
- Findings: counts by severity only; link to `specgraph://findings` for detail

For projects with fewer specs than these thresholds, the prime inlines full lists. Thresholds kick in only on projects large enough to need them.

### Existing (unchanged from PR 897/898)

`specgraph://spec/{slug}`, `specgraph://specs`, `specgraph://decision/{slug}`, `specgraph://constitution`, `specgraph://constitution/{layer}`, `specgraph://graph`, `specgraph://graph/ready`, `specgraph://findings`, `specgraph://spec/{slug}/changes`

## Profile Mapping

Extend `internal/mcp/profiles.go` `ProfileFromClientInfo` to include `opencode` and `codex` in the `ProfileAuthoring` bucket alongside existing `claude-code`, `cursor`, `windsurf`. Exact client identifier strings require empirical verification during implementation.

## Platform Plugins

Each plugin carries, at most:

1. **MCP server configuration** — platform-native format for pointing the MCP client at `specgraph mcp` (stdio) or `specgraph serve`'s `/mcp/` endpoint (HTTP)
2. **Session-start mechanism** — platform-native hook or rule that reads `specgraph://prime` on session init
3. **Routing guide** — ~200–500 tokens of stable meta-knowledge: "you have access to the SpecGraph MCP; for authoring work, invoke the appropriate stage prompt or `author.start_stage` tool; for queries, use tools or resources." The routing guide describes *where to go* for various user intents (shape a spec, list specs, check ready work) without duplicating the *how* (posture rules, quality heuristics, output schemas) that lives server-side. The guide is small enough that it is effectively stable across server releases; if server-side workflow changes introduce new stages or flows, the routing guide updates at the same cadence as the plugin release. Project-variable routing (e.g., team-specific slash commands) is explicitly out of scope — see Explicit Scope Exclusions
4. **Init bootstrap instructions** — how to set up a new SpecGraph project (Docker, `specgraph init`, `specgraph serve`) — useful before the MCP server is even running
5. **Optional slash commands** — convenience wrappers that invoke specific MCP prompts (e.g., `/spec shape <slug>` invokes the `shape` MCP prompt with that argument)

### Per-Platform Shapes

| Platform | MCP config | Session start | Routing guide | Slash commands |
|----------|-----------|---------------|---------------|----------------|
| Claude Code | `.claude-plugin/plugin.json` + MCP registration | `hooks/session-start.sh` reads `specgraph://prime` | Small skill file | `commands/*.md` |
| Cursor | Cursor's MCP settings file | Cursor's rule or auto-load mechanism | `.cursor/rules/specgraph.md` (stable, rarely changes) | Cursor command format (if supported) |
| OpenCode | OpenCode's MCP config | Platform-specific | Platform-native instructions file | Platform command format (if supported) |
| Codex | Codex's MCP config | Platform-specific | Platform-native instructions file | Platform command format (if supported) |

Exact mechanisms for Cursor, OpenCode, and Codex require verification during implementation. The design presumes platforms converge toward Claude Code's hook model over time and uses each platform's most-hook-like mechanism available today. If a platform genuinely cannot run anything at session boundaries, the plugin for that platform omits priming entirely — we do not compensate by turning the routing guide into an instruction-following substitute, because that reintroduces the instruction-dependence the design principle rejects. Cold-start sessions on such a platform simply begin without prime; they still function correctly as soon as the LLM interacts with any spec-aware tool.

### Delivery Channels: Tool Always Registered, Prompts Additive

The composer produces structured content suitable for either delivery channel. Two principles:

1. **The tool path is always registered.** `author.start_stage(slug, stage, posture?)` is advertised on every MCP connection regardless of what capabilities the client declares. This is the guaranteed-available channel.
2. **The prompt path is additive where supported.** `shape` / `specify` / `decompose` / etc. prompts are registered and available to clients that surface prompts to users. Clients that don't consume prompts simply see them unused; no error, no gate.

Rationale for always-register-tool: at design time we have not verified prompt exposure on Cursor, OpenCode, or Codex. Betting the design on capability negotiation being honestly implemented across three unverified platforms is exactly the kind of assumption that fails under scrutiny. Tools are the lowest-common-denominator MCP primitive — every client implements them. Registering the tool path unconditionally means the feature works regardless of what any client's capability declaration says.

**Implementation consequence**: phase 1 ships both the prompt handlers and the `author.start_stage` tool. Every authoring-profile client sees the tool; clients that support prompts additionally see the prompts. The composer stays the single source of truth.

**Platform plugin routing guide**: for platforms that expose prompts to users (verified during implementation), the routing guide steers the LLM to prefer prompts for stage starts (cleaner UX via slash commands) and `author.start_stage` for mid-conversation re-entry. For platforms that don't, the routing guide steers the LLM to use `author.start_stage` in both cases.

### Exit criterion for dual delivery

Dual delivery is maintained until one condition holds:

- **Consolidate on tool delivery** if post-phase-1 telemetry shows prompt invocation rates below a threshold (e.g., <10% of stage starts) across all four platforms, suggesting prompts add UX surface without material usage
- **Consolidate on prompt delivery** if every platform's prompt support is verified and prompt invocation rates dominate

Consolidation is a future simplification, not a phase-1 requirement. Phase 1 ships both paths intentionally; the exit criterion is a stake in the ground so we don't maintain dual delivery indefinitely out of inertia.

## Migration

### Context: No Active Users

SpecGraph has no active users at the time of this design. The existing 13-skill Claude Code plugin is replaced with the new thin plugin in the same release that ships the enriched MCP prompts and composer. No backward compatibility, no parallel path, no deprecation window.

### Single-Release Scope

One release ships all of the following together:

- Embed existing skill content into `internal/authoring/content/` via `//go:embed`, reworked into the composable sections
- Build `authoring.Composer` and integrate with `internal/mcp/prompts.go` and the new `author.start_stage` tool handler
- Add required `conversation_exchanges` argument to `ShapeRequest` / `SpecifyRequest` / `DecomposeRequest` (proto change) and corresponding server-side atomic persist
- Add `specgraph://prime` resource with the size discipline described above
- Add `opencode` and `codex` to `ProfileFromClientInfo`; confirm or remove `windsurf`
- Ship thin plugins for all four platforms simultaneously
- Remove old 13-skill Claude Code plugin content

### Testing and Dogfood

- **End-to-end path coverage**: existing `e2e/` tests that exercise the Claude Code skill path are rewritten to exercise the MCP-composed delivery path end-to-end (MCP prompt invocation or `author.start_stage` tool call → composer → atomic persist of stage output + conversation record). Both prompt delivery and tool delivery are exercised.
- **Composer golden-file tests**: for each stage (spark, shape, specify, decompose, approve), a fixture project + composer invocation produces a composed response that is asserted against a committed golden file. Token count assertions on the golden files run in CI — regressions over budget fail the build.
- **Content/proto drift tests**: a CI check verifies that field names referenced in embedded content (e.g., `stage-shape.md` mentioning `scope_in`, `approaches[].tradeoffs`) match current proto field names. Drift — embedded content referencing a renamed or removed field — fails CI. This closes the drift hazard that would otherwise move from the skills layer to the embedded-content layer.
- **Client identifier contract tests**: for each of the four target platforms, a smoke test issues an MCP `initialize` and verifies the client name matches the identifier string in `ProfileFromClientInfo`. Regression — a platform changes its reported name — fails CI for that platform's profile mapping.
- **Posture round-trip**: tests call `author.*` with each posture value, verify the server records it against the conversation exchange, and verify `ListConversations` returns the recorded posture.
- **Conversation coupling validation**: tests for each rejection mode of the validation rules (empty exchanges, missing role, mismatched stage tag, non-monotonic sequence).
- **Platform smoke tests**: per-platform connection tests confirm the MCP server is reachable from Claude Code, Cursor, OpenCode, and Codex with the verified identifier strings.
- **Dogfood**: SpecGraph's own development authoring (used to plan new SpecGraph features) switches to the new path in the same release; mis-landings that harm our own velocity are the primary early warning signal before the phase-2 telemetry exists.

### Rollback

Rollback has two layers because the proto change is schema-evolving:

- **Pre-persistence rollback** (no new stage outputs written with atomic coupling yet): straightforward `git revert` of the release. The skills in `plugin/specgraph/skills/` remain in version control indefinitely as the source material; reverting them is a revert away.
- **Post-persistence rollback** (new stage outputs have been written with associated conversation records via atomic coupling): the atomic writes land in separate persistent storage (`specs`, `conversation_records`) using the existing schema — the proto field is on the request, not on the persisted row shape. Reverting the server code is still a straightforward git revert; the old server reads the persisted data with no migration needed. What the old server *cannot* do is honor the required-argument contract on new calls, which means clients running the new plugins against an old server would see the coupling check absent. This is acceptable because our dogfood usage (the only active usage) would revert plugins at the same time as the server.

Proto evolution rule: the `conversation_exchanges` field uses a new field number in the request messages, per standard proto evolution. Removing the field later (if we ever reverse course) reserves the number; adding back a different field would not reuse it.

### Deferred to a Separate Design — Telemetry-Driven Server Enforcement

A subsequent design document will cover:

- Instrument prompt invocations and tool calls
- Observe which behaviors LLMs skip under pressure on each platform
- Propose selective server-side enforcement (e.g., `author.shape` preconditions beyond the required-argument coupling already in this design) based on data

Out of scope for this design document.

## Error Handling

### Composer errors

- **Missing constitution**: composer proceeds with empty constitution section + warning in the composed output; does not block
- **Spec not found for stage prompt**: composer returns a structured error message as the prompt response; LLM surfaces to user
- **Token budget exceeded despite summarization**: composer truncates with explicit `(truncated; see resource X)` markers; does not fail
- **Storage unreachable**: composer returns a structured error referencing the stateless retry model; the LLM can retry the prompt invocation once storage is back

### Session-start errors

- **MCP server unreachable at session start**: platform hook logs warning; session continues without prime; LLM can still use the MCP path once the server becomes reachable
- **Prime resource fails**: same — log, continue, recover on next interaction

### Missing or invalid conversation exchanges

- **`author.shape` / `author.specify` / `author.decompose` called without `conversation_exchanges`**: rejected with `CodeInvalidArgument` and a message directing the caller to include the exchanges. Server does not auto-generate placeholder exchanges. The LLM must re-call with the exchanges it has accumulated.
- **Validation failures** (empty array, missing role/content, mismatched stage, non-monotonic sequence): rejected with `CodeInvalidArgument` and the specific validation rule that failed in the message.
- **Approve-stage rejection without exchanges**: rejected with `CodeInvalidArgument` — exchanges are required when the approve action is "reject" (see Conversation Recording Coupling section).

## Security and Authentication

This design relies on the authentication model introduced in PR 898:

- **HTTP-mode MCP** (`/mcp/` endpoint on `specgraph serve`): gated by the `RequireAuth` middleware. Unauthenticated callers receive HTTP 401, which the MCP spec treats as an OAuth trigger.
- **Stdio-mode MCP** (`specgraph mcp` command): passes the user's configured API key with every loopback ConnectRPC call; OIDC credentials are resolved via the file-backed `TokenStore` fallback per PR 898.
- **Loopback transport**: uses `WithHTTPContextFunc` to propagate the caller's bearer token into MCP context for end-to-end RBAC, ensuring handlers see the caller's identity rather than the server process identity.

This design introduces no new authentication surface. The composer runs inside the same ConnectRPC boundary that PR 898 secured; the new `author.start_stage` tool and the new required `conversation_exchanges` field inherit the existing auth model without modification.

## Telemetry (phase 2 preview)

Not implemented in phase 1, but the design leaves hooks for:

- Per-call recording of posture, stage, spec slug, platform client name
- Drift detection: posture inconsistency within a single spec's authoring cycle
- Step-completeness detection: stage output persisted without matching conversation record; analytical passes skipped for a posture that should dispatch them

These observations drive phase 2 decisions about which behaviors need server-side enforcement via tool preconditions.

## Open Empirical Verifications

The following require implementation-time verification. None block this design because:

- The tool delivery path is always registered (Delivery Channels section), so the core feature works regardless of each platform's prompt exposure behavior
- The routing guide gracefully handles platforms without session-start hooks (those platforms cold-start without prime but still function)
- The client-identifier profile mapping is a one-line server change per platform

Verification tasks, all required before the implementation plan is final:

- OpenCode and Codex client identifier strings reported during MCP `initialize`
- `windsurf` identifier (inherited from PR 897, status of its own verification unknown — confirm it still maps correctly or remove as part of this work)
- OpenCode, Codex, and Cursor MCP prompt surfacing to users (informs routing-guide text and UX design for slash commands, not the core delivery model)
- OpenCode, Codex, and Cursor session-start mechanisms and their native formats
- OpenCode, Codex, and Cursor MCP server configuration file formats

## Explicit Scope Exclusions

**Project-specific plugin customization.** Teams may reasonably want per-project slash commands (e.g., `/spec-weekly-review`). That is stable meta-knowledge but also project-variable, which the guiding principle does not cleanly classify. Resolution for this design: per-project plugin customization is out of scope; all plugin files are shipped by the SpecGraph repo with one canonical definition per platform. Future work may introduce a mechanism (e.g., user-space plugin overlays) but this design does not address it.

**LLM output quality parity.** As stated in Non-Goals, model differences across Claude / GPT-4o / Gemini / etc. mean identical guidance does not produce identical output. The design targets structural and capability parity of the delivered guidance and available operations, not parity of the LLM's eventual spec content.

**Detection of LLM guidance-following failure.** The phase-2 telemetry work will address "did the LLM follow the guidance?" observationally. This phase assumes the LLM follows the composed guidance and the server-side coupling mechanism (required `conversation_exchanges`) catches the highest-risk failure mode.

## Cross-References

- `plugin/specgraph/skills/` — existing 13-skill plugin content; source for phase 1 content migration
- `internal/authoring/prompts.go` — current thin template registry to be superseded by `composer.go`
- `internal/mcp/prompts.go` — current thin prompt handlers that delegate to `authoring.GetPrompts`
- `internal/mcp/profiles.go` — `ProfileFromClientInfo` extension point
- `docs/plans/2026-04-10-mcp-server-design.md` — MCP server foundation this design builds on
- Beads: spgr-mv32
