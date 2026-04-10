# Recording 3: Slack Amendment — Transcript

**Spec:** slack-authoring (amend) | **Flow:** Amend → Re-Shape

---

## Amend

```
> Show me slack-authoring. I want to amend it -- we now want to add the full
> interactive authoring funnel so engineers can Spark, Shape, Specify, Decompose,
> and Approve through Slack. Not just read-only commands and approvals anymore.
> Amend it back to shape.
```

Agent fetches the current spec (approved, v6), runs `specgraph amend`:

```
Amended: slack-authoring (stage=spark, lifecycle=SPEC_LIFECYCLE_TASK, version=7)
```

**Insight:** Amend re-entry lands one stage before target — the spec goes to spark so the next authoring command (shape) can write to the shape stage. By design (commit 2d88e75).

Agent identifies 4 key design questions for the re-shape:

| Question | Trade-off |
|----------|-----------|
| How much authoring context does Slack provide? | CLI has filesystem; Slack doesn't |
| Modal forms vs. threaded conversation? | Modals cap at 50 blocks; threads are natural but harder to parse |
| Who drives the funnel — user or bot? | Bot-driven mirrors CLI skills vs. user fills forms |
| Where does LLM inference run? | Sidecar (violates thin-client) vs. server-side |

---

## Re-Shape

```
> Start the shape skill. Here's my direction: Threaded conversation, not modals
> -- the funnel is a conversation, not a form. One thread per spec, one channel
> per project. The bot drives the funnel like the CLI skills do -- it probes,
> proposes, and the user confirms or pushes back. LLM inference stays server-side
> -- the sidecar proxies the conversation to the server, which runs the same
> authoring logic the CLI skills use. The sidecar is still a thin client, it just
> has a new transport.
```

### Scope In/Out

**In scope (12 items):** Threaded conversational authoring through all 5 funnel stages, bot-driven probing, one thread per spec, one channel per project, server-side LLM inference, sidecar as transport adapter, plus all carried items from original shape (slash commands, Block Kit, approval flow, notifications, multi-project).

**Out of scope (9 items):** Modals/forms for input, cross-transport handoff, Workflow Builder, Socket Mode, RBAC, codebase scanning, direct editing from Slack, voice/huddle, manual analytical pass triggering.

**Key tension resolved:** Original shape explicitly excluded "authoring from Slack" because it needs codebase context. Resolved by: server-side inference (so sidecar stays thin) + hybrid context model (bot asks users to paste snippets when needed).

```
> [Selected: Hybrid -- user pastes context]
```

**Insight:** Transport-agnostic authoring — the authoring funnel's probing logic shouldn't depend on how context arrives (filesystem grep vs. pasted snippet vs. future MCP resource). The server-side authoring API should accept context as input, not assume it can fetch it. Slack is a forcing function for better API design.

### Approaches

**Approach 1: Turn-based unary RPCs (recommended)** — New `AuthoringConversationService` with `StartSession`, `SubmitTurn`, `CompleteStage` RPCs. Server manages session state, runs LLM, returns bot responses. Sidecar relays messages between Slack threads and server sessions.

**Approach 2: Server-side streaming RPC** — Bidirectional `AuthorInteractive` stream. Simpler API but harder to bridge with Slack's request/response model.

**Approach 3: Sidecar-hosted LLM** — No server changes, sidecar drives conversation. Fastest to ship but violates thin-client principle and creates behavior divergence.

```
> [Selected: Turn-based RPCs]
```

### Decisions

9 decisions captured — 4 carried from original shape, 5 new:

| # | Slug | Decision |
|---|------|----------|
| 1 | sidecar-binary | (carried) Standalone binary, zero internal/ imports |
| 2 | block-kit-rendering | (carried) Block Kit for all output |
| 3 | email-identity | (carried) Slack email maps to SpecGraph identity |
| 4 | outbound-webhooks | (carried) ChangeEvent dispatcher for push notifications |
| 5 | threaded-conversations | NEW — One thread per spec session, one channel per project |
| 6 | server-conversation-engine | NEW — Turn-based unary RPCs for interactive authoring |
| 7 | separate-go-module | NEW — Separate go.mod for sidecar to isolate dependencies |
| 8 | server-side-inference | NEW — LLM inference runs server-side, not in sidecar |
| 9 | user-pasted-context | NEW — No codebase scanning; bot asks for pasted snippets |

**Insight:** Decisions 6-8 form a single design choice: "the server is the authoring brain, clients are transport adapters." The conversation engine is the real deliverable; Slack is just the first consumer.

### Success Criteria

**Must (15):** Full authoring through all 5 stages via threaded conversation, bot-driven probing, pasted-context support, session survival across sidecar restarts, conversation recording, plus carried criteria from original shape.

**Should (12):** Review feedback via thread replies, findings summaries, typing indicators, session timeouts with warnings.

**Won't (9):** Modals/forms, cross-transport handoff, Workflow Builder, Socket Mode, RBAC, codebase scanning, direct editing, voice/huddle, manual analytical passes.

### Risks

12 risks — 5 new, 7 carried:

| # | Risk | Category |
|---|------|----------|
| 1 | Session state management complexity | New — server capability |
| 2 | LLM integration scope creep | New — server capability |
| 3 | Prompt template divergence between CLI and server | New — server capability |
| 4 | Slack thread UX limitations (no typing indicators, 4000 char limit) | New — UX gap |
| 5 | LLM response latency in Slack context (5-30s) | New — UX gap |
| 6-11 | Carried from original shape | Rate limits, reliability, OAuth, Block Kit, identity, config |
| 12 | Conversation quality without codebase context | New — quality gap |

**Insight:** The original spec was "add a client." The amended version is "add a platform capability that enables new clients." The conversation engine is the real deliverable.

Shape saved at v8. Constitution-check and background codebase scans completed — confirmed authoring is 100% client-side today, server is persistence + validation only. The conversation engine has some foundation: existing `GetPrompts` RPC returns prompt templates per stage, `RecordConversation` stores exchanges.
