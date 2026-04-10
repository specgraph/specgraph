# SpecGraph Authoring Funnel — Demo Transcript

A real conversation authoring a spec through the complete funnel:
**Spark → Shape → Specify → Decompose → Approve → Bundle**

---

## Setup

```
> we need to setup specgraph
```

SpecGraph initializes the project: writes `.specgraph.yaml`, starts Postgres via Docker, starts the server, verifies health.

```
Status:  ok
Version: dev
```

No constitution exists yet. The constitution is your project's ground truth — analytical passes check every spec against it.

The agent walks through each constitution section conversationally — tech stack, principles, constraints, antipatterns, process, references — drafting each from codebase analysis and getting confirmation before moving on.

After review and iteration, the constitution is imported:

```
Constitution imported for project specgraph-demo
```

**Constitution summary:** 7 principles, 7 constraints, 4 antipatterns, 13 references — all derived from the codebase and confirmed by the author.

---

## Spark

```
> We need a way to handle inbound notifications/webhooks from systems like ADO,
> GH, Discord, Slack for updating state on a spec/slice, etc
```

**Probe — Signal:** Why now? Is this driven by a specific pain point, or building toward the Phase 3 coordination layer from the roadmap?

```
> the latter
```

**Probe — Scope sniff:** Gut feel: is this hours, days, or weeks?

```
> I think we can start small (support GH/ADO webhooks) but plan for other types
> if they differ. Couple weeks
```

**Probe — Kill test:** What would make this not worth doing? Candidates:
1. Polling turns out to be good enough
2. External systems don't emit the events we need
3. Auth/security surface is disproportionate

```
> 1, but also volume or other drivers towards putting kafka/messaging between
> inbound and SG
```

**Spark saved:**

| Field | Value |
|-------|-------|
| Slug | `inbound-webhooks` |
| Stage | spark |
| Seed | Inbound webhook/notification system for GH, ADO, Discord, Slack |
| Signal | Phase 3 coordination layer buildout |
| Scope | ~2 weeks; start with GH/ADO, design abstraction for others |
| Kill conditions | (1) Polling is good enough (2) Volume forces a message bus |

---

## Shape

### Scope In / Out

**Agent proposes scope boundaries:**

**In scope:**
- HTTP webhook receiver endpoint(s) on the SpecGraph server
- GitHub webhook handler (PR merged/closed, issue events, check run events)
- ADO webhook handler (PR completion, work item state changes)
- Webhook signature verification (GitHub HMAC-SHA256, ADO basic auth/shared secret)
- Event-to-action mapping: translate external events into SpecGraph state transitions
- Webhook registration/configuration in global config
- Provider abstraction interface so future sources plug in cleanly

**Out of scope:**
- Discord/Slack webhook handlers (future spec)
- Outbound notifications (SpecGraph to external systems)
- Message bus / async queue between receiver and SpecGraph (kill condition #2)
- Polling-based sync (exists separately; this spec is push-only)
- Webhook management UI in the web dashboard
- Retry/dead-letter queue for failed event processing

```
> Scope sounds good
```

### Approaches

**Agent generates three approaches with tradeoffs:**

**Approach A: Raw HTTP handlers on the existing mux (recommended)** — Register `/api/webhooks/github` and `/api/webhooks/ado` as standard HTTP handlers. Follows the proven pattern from `auth_handler.go`. No new dependencies. Provider-specific signature verification per-handler.

**Approach B: ConnectRPC WebhookService** — Define a WebhookService in proto. GitHub and ADO can't call ConnectRPC natively — requires a shim for zero benefit.

**Approach C: Webhook gateway as a separate process** — Lightweight sidecar. Over-engineered for two providers. Approaches kill condition #2.

```
> A for now, though we need to note the need to review to centralize some of
> those interceptors and/or their logic
```

### Decision Capture

Agent proposes two decisions. User pushes on the architecture:

```
> The various webhooks should funnel their input into a single handling chain.
```

**Decision 1 revised:** Provider handlers normalize into a common `WebhookEvent` type. A single processing chain maps normalized events to SpecGraph state transitions. Provider-specific concerns (signatures, payload schemas) isolated at the edge.

**Decision 2:** Accept manual middleware application for V1, track need to centralize HTTP middleware.

```
> yes
```

### Success Criteria

**Must (7):** Signature verification for both providers, normalization to common type, single processing chain, provider interface, config-driven, idempotent processing.

**Should (3):** Event log for audit, meaningful HTTP responses, CLI listing command.

**Won't (5):** Discord/Slack, outbound notifications, webhook UI, retry queue, message bus.

```
> LGTM
```

### Risks

- Provider payload schema drift
- Signature verification subtleties
- Event-to-action mapping ambiguity
- Publicly exposed endpoint security
- Idempotency complexity

```
> Also looks good
```

**Shape saved.** Spec advanced to Shape stage with 2 decisions linked in the graph.

---

## Specify

### Interface Contract

Agent drafts the full contract from Shape output:

**WebhookProvider interface:**
```go
type WebhookProvider interface {
    Name() string
    ValidateRequest(secret string, header http.Header, body []byte) error
    NormalizeEvent(header http.Header, body []byte) (*WebhookEvent, error)
}
```

**WebhookEvent** with provider, delivery ID, event type, external ID/URL, repo, and a `Metadata map[string]string` field.

**HTTP endpoints**, **Processor**, **Config schema**, **Sentinel errors** — all fully specified.

```
> Is 'metadata' the right thing to call the rest of the unstructured data in
> the event?
```

Agent proposes alternatives: `Extra`, `ProviderFields`, or `RawPayload json.RawMessage` (preserving the full original payload for audit/debugging). Recommends `RawPayload` for zero-loss preservation.

```
> yes
```

### Verify Criteria

24 test assertions across 7 categories: signature (6), normalization (4), processing (4), provider-interface (1), config (3), security (2).

```
> looks good to me
```

### Invariants

- No webhook event processed more than once (delivery ID dedup)
- Signature verification before payload parsing
- Payload size limit before any processing
- Provider logic never leaks into processing chain
- Secrets never logged or in error responses

```
> Hm, thought, should we be capturing a hash of the content (absent any time
> specific bits) to ensure that uniqueness, or do we trust event ids from the
> external systems?
```

Agent explains: GitHub's `X-GitHub-Delivery` is per-delivery-attempt — intentional redelivery gets a new ID. Proposes dual dedup: delivery ID (fast-path for retries) + content hash of normalized fields (semantic dedup for redeliveries). `WebhookEvent` gains a `ContentHash` field.

```
> that works for me
```

### Touches

14 new files, 3 modified. Agent lists each with purpose and change type.

```
> nope, though - do our e2e test plans include database content verification?
```

Agent adds 4 e2e verify criteria that query the database after webhook processing — event row verification, slice state change, delivery ID dedup, content hash dedup.

```
> fold them in
```

**Specify saved.** 24 verify criteria, 5 invariants, 17 file touches.

---

## Decompose

### Strategy

Agent recommends **horizontal layer** decomposition — storage first, then types, then providers, then processor.

```
> What about (and I'm asking) a steel thread approach, drive interfaces through
> e2e to allow for early proof and parallelism?
```

Agent agrees — steel thread is stronger. Proposes 4 slices:

**Slice 1: `steel-thread-github-pr`** (no dependencies)
One GitHub PR-merged event flows end-to-end: HTTP handler → HMAC verification → normalize → process → store. E2e test proves the full pipeline.

**Slice 2: `ado-provider`** (depends on slice 1)
ADO provider implementation — shared secret verification, payload normalization.

**Slice 3: `dedup-and-idempotency`** (depends on slice 1)
Dual dedup: delivery ID + content hash. Storage-level uniqueness constraints.

**Slice 4: `config-and-hardening`** (depends on slice 1)
Config validation at startup, rate limiting, payload size enforcement.

```
steel-thread-github-pr  (start immediately)
    ├── ado-provider         (parallel)
    ├── dedup-and-idempotency (parallel)
    └── config-and-hardening  (parallel)
```

```
> Compare that to your original approach. Contrast - which is going to deliver
> value and lowest chance of rework/wandering through hedges?
```

Agent compares honestly:

| | Horizontal | Steel thread |
|---|---|---|
| First value | Late | Early |
| Integration risk | High | Low |
| Interface correctness | Theoretical | Proven by e2e |
| Rework risk | Higher | Lower |
| Parallelism | Looks parallel, layers block | Genuinely parallel after slice 1 |

Steel thread wins on every dimension.

```
> yes
```

**Decompose saved.** 4 slices with steel thread strategy.

---

## Approve

Agent declares: "I helped author this spec through all stages, so I can't approve it. I'll facilitate the review — you make the call."

Full spec presented to the reviewer. Agent walks through 6 checkpoints one at a time:

| Checkpoint | Assessment | Result |
|---|---|---|
| Scope bounded? | 7 in, 6 out, explicit boundaries | Good |
| Interface defined? | Complete contract, mapping resolution deferred to steel thread | Good |
| Verify criteria testable? | 24 automatable criteria including e2e database verification | Good |
| Dependencies mapped? | No external deps, clean internal slice ordering | Good |
| Constitution compliance? | No violations, content hash extends p-5 pattern | Good |
| Risk acknowledgment? | All 5 risks have concrete mitigations in verify criteria | Good |

Each checkpoint explicitly confirmed by the reviewer before proceeding.

```
> confirmed
```

```
Approved: inbound-webhooks at 2026-04-03T17:12:23Z
```

---

## Execution Bundle

The approved spec generates a complete execution bundle — everything an implementing agent needs:

- Full scope and acceptance criteria (24 checkboxes)
- Interface contracts with Go type definitions
- Invariants that must hold
- File touch map (14 new, 3 modified)
- Work slices with dependency ordering
- Linked decisions with rationale
- Risk context

The spec is frozen and ready for an agent to claim and implement.

---

## Key Moments

Throughout the funnel, the human author made several interventions that meaningfully shaped the spec:

1. **Kill conditions** (Spark) — "Polling good enough" and "volume forces a message bus" established clear boundaries
2. **Single handling chain** (Shape) — Pushed the architecture from per-provider processing to normalize-then-process
3. **Middleware centralization** (Shape) — Flagged tech debt for future work
4. **RawPayload over Metadata** (Specify) — Questioned the naming, led to a better design (zero-loss preservation)
5. **Content hash dedup** (Specify) — Questioned trusting external delivery IDs, led to dual dedup strategy
6. **E2e database verification** (Specify) — Caught a gap in test coverage
7. **Steel thread over horizontal** (Decompose) — Changed the decomposition strategy entirely, reducing integration risk

Each of these decisions is captured in the graph with full conversation context — not just *what* was decided, but *why*.
