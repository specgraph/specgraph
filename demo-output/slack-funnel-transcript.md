# Recording 2: Slack Funnel — Transcript

**Spec:** slack-authoring | **Funnel:** Spark → Shape → Specify → Decompose → Approve

---

## Spark

```
> We need Slack integration for SpecGraph. PMs and stakeholders need to see spec
> status, get notified on stage transitions, and approve specs without leaving
> Slack. They don't write specs -- that stays in the CLI/agent workflow -- but
> they need to track what's happening and give approvals.
```

**Signal probe:** Accessibility for non-developer stakeholders — PMs and leads who need visibility into the spec pipeline.

```
> Exactly. The authoring funnel is powerful but CLI-only. PMs aren't going to
> install the CLI or run Claude Code. They need Slack slash commands for read
> access and notifications when specs move through stages.
```

**Scope sniff:** Sidecar binary separate from the main server, calling ConnectRPC APIs.

```
> Yes, sidecar. Separate binary, separate deployment, zero imports from
> internal/. It consumes the same APIs as the CLI.
```

**Kill test:** Not worth doing if PMs are fine checking the web dashboard.

```
> No web dashboard exists yet. And even if it did, Slack is where they live.
> The other kill condition would be if Slack's API restrictions make the
> interaction model unworkable -- rate limits, Block Kit limitations, etc.
```

Spark saved: `slack-authoring` at spark stage. Constitution check ran — no blockers.

---

## Shape

**Scope In/Out:** Proposed scope — sidecar binary, slash commands (/spec show, list, ready, approve), Block Kit rendering, interactive approval with email identity, stage notifications via outbound webhooks, multi-project routing, multi-server support.

Out of scope: authoring from Slack, Workflow Builder, Socket Mode, RBAC, bidirectional sync, bot personality, embedded in main server.

```
> Scope sounds good.
```

**Approaches:** Three presented — (A) Query + Outbound Notifications (recommended), (B) Full bidirectional bridge, (C) Webhook-only passive mirror.

```
> A for now, though we need to note the need to review to centralize some of
> those interceptors and/or their logic.
```

**Decisions:** 5 proposed — sidecar binary, outbound webhooks for push notifications, slash commands for interaction, email identity for approval attribution, Block Kit rendering.

```
> yes
```

**Success Criteria:** 10 musts, 6 shoulds, 7 won'ts.

```
> LGTM
```

**Risks:** 7 identified — Slack API rate limits, outbound webhook reliability, OAuth/app manifest complexity, Block Kit character limits, email identity mapping brittleness, multi-server config complexity, main server deployment coordination.

Shape saved with 5 decisions linked. Constitution-check and peripheral-vision ran in background.

---

## Specify

**Interface Contract:**
- Interface 1: Slack Slash Command Handler (`POST /slack/commands`) — routes `/spec show|list|ready|approve|deps|help`
- Interface 2: Interactive Actions Handler (`POST /slack/actions`) — button callbacks for approve confirm/cancel
- Interface 3: Outbound Webhook Receiver (`POST /webhooks/specgraph`) — receives ChangeEvent POSTs from main server
- Interface 4: Outbound Webhook Dispatcher (Main Server) — `WebhookDispatcher` implementing `ChangeSubscriber`, fire-and-forget POSTs
- Interface 5: Sidecar Configuration Schema — `specgraph-slack.yaml` with Slack tokens, server mappings, project-to-channel routing
- Interface 6: ConnectRPC Client Calls — existing RPCs consumed by sidecar (GetSpec, ListSpecs, Approve, etc.)

Key design: outbound webhook payload is intentionally minimal (just the event envelope) — sidecar fetches full spec via GetSpec when rendering.

Also identified: ApproveRequest proto needs an `approved_by` field so the sidecar can record who approved from Slack.

```
> Keep it bundled. The dispatcher is a dozen lines of glue code in v1 -- it's
> not independently deployable or testable in a meaningful way yet. If someone
> else needs outbound webhooks for a non-Slack consumer, that's when we extract
> it into its own spec. YAGNI until then. Interfaces look good, move to verify
> criteria.
```

**Verify Criteria:** 33 criteria across 8 categories — slash commands happy path (8), slash commands error path (6), interactive actions (2), notifications (4), outbound webhook dispatcher (4), security (4), configuration (3), module boundary (1), plus the approved_by proto criterion.

```
> Good coverage. One add: we need a verify criterion for the ApproveRequest
> proto gap you identified -- something like 'Approve RPC accepts an optional
> approved_by field; sidecar populates it from Slack email.'
```

Criterion 33 added for approved_by.

**Invariants:** 5 — sidecar never mutates state except through ConnectRPC, dispatcher never blocks transactions, signing verification precedes all processing, single ChangeEvent produces at most one notification per channel, approval identity once recorded is immutable.

**Touches:** 10 new sidecar files + 2 new server files (notify package) + 9 modified files (config, serve.go, proto, gen, handler, domain, storage, go.mod). Collision check: shared files with inbound-webhooks but different sections — no structural conflict.

Specify saved. Red-team, consistency, and constitution-check passes ran.

---

## Decompose

**Strategy:** Steel Thread — prove the full slash-command round-trip with `/spec show` first, then broaden.

**Slices:**
1. `steel-thread-spec-show` (no deps) — Sidecar skeleton, config, signing, ConnectRPC client, `/spec show` with Block Kit rendering, plus approved_by proto change. 12 files.
2. `broaden-query-commands` (depends on 1) — `/spec list`, `/spec ready`, `/spec deps`, `/spec help`. 4 files modified.
3. `broaden-approve-flow` (depends on 1) — `/spec approve` with interactive confirmation, email identity, button flows. 4 files.
4. `outbound-notifications` (depends on 1) — Webhook dispatcher in main server, webhook receiver in sidecar, stage transition notifications. 7 files.

Slices 2, 3, and 4 parallelize after the steel thread.

```
> Good decomposition. Same pattern as inbound-webhooks -- steel thread proves
> the pipe, then fan out. Persist it.
```

Decompose saved. Simplicity and constitution-check passes ran.

---

## Approve

6-checkpoint review:

1. **Scope bounded?** ✅ — Constitution findings: 3 warnings (no DEPENDS_ON edges, go.mod pollution, fire-and-forget reliability), 3 notes (mechanical). Resolution: separate go.mod for sidecar (captured as decision), reliability accepted as v1 limitation.
2. **Interface defined?** ✅ — 6 interfaces spanning sidecar and server. Gap found: webhook event payload missing `previous_stage` field → added to event payload. ApproveRequest proto gap documented.
3. **Verify criteria testable?** ✅ — 33 criteria, all concrete assertions across 8 categories.
4. **Dependencies mapped?** ✅ — No hard dependencies. Soft overlap with inbound-webhooks at file level (different sections), acceptable.
5. **Constitution compliance?** ✅ — 4 passes across all stages, 0 critical. Sidecar aligns with thin-client principle, proto-first, graph-native.
6. **Risk acknowledgment?** ✅ — All 7 risks have mitigations or accepted v1 limitations.

**Approved:** `slack-authoring` at 2026-04-10T15:03:13Z

Notable decisions captured during review:
- Separate go.mod for sidecar to prevent dependency pollution
- previous_stage field on outbound webhook event payload
- Dispatcher bundled in this spec (YAGNI — extract when second consumer appears)
