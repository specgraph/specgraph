# Recording 1: Webhooks Funnel — Transcript

**Spec:** inbound-webhooks | **Funnel:** Spark → Shape → Specify → Decompose → Approve

---

## Spark

```
> We need a way to handle inbound notifications/webhooks from systems like ADO,
> GH, Discord, Slack for updating state on a spec/slice, etc
```

**Signal probe:** Is this urgent because sync is push-only (CLI adapters shelling out)?

```
> Yeah, it's the Phase 3 coordination layer from the roadmap. Push over poll --
> we don't want to be shelling out on a timer to check if things changed.
```

**Scope sniff:** Weeks, not days? 4 providers with different wire formats.

```
> Couple weeks yeah. GH and ADO first since those are the ones we actually use.
> But the design should be extensible so adding Discord/Slack later is just a
> new provider implementation, not a rearchitect.
```

**Kill test:** Not worth doing if polling-based sync adapters are good enough.

```
> That's one. The other kill would be if event volume gets high enough that
> individual HTTP handlers can't keep up and we'd need a proper message
> bus/queue anyway. At that point the direct webhook approach becomes throwaway
> work.
```

Spark saved: `inbound-webhooks` at spark stage. Constitution check ran — no blockers.

---

## Shape

**Scope In/Out:** Proposed 9 in-scope items (both providers, common event type, dedup, Processor, Postgres storage, raw HTTP handlers, config) and 7 out-scope (Discord/Slack, outbound notifications, message bus, rate limiting, replay, CLI, UI).

```
> Scope looks right. One thing I want to be explicit about on the architecture
> side -- these have to be raw HTTP handlers, not ConnectRPC. GitHub and ADO
> send standard HTTP POST with their own formats. They can't speak Connect wire
> format. Provider-specific parsing happens at the edge, then we normalize into
> a common event type for the processor.
```

**Approaches:** Three presented — Provider-per-handler (recommended), Single dispatcher, Middleware chain.

```
> Approach A, agreed. The 30 lines of boilerplate per provider is fine -- it's
> honest code, not abstraction for abstraction's sake. One thing to think about
> on the dedup side: GitHub redeliveries get new delivery IDs, so delivery-ID
> dedup isn't sufficient. We need content hash dedup too.
```

**Decisions:** 4 proposed — raw-http-not-connectrpc, two-layer-dedup, provider-per-handler, webhook-project-scoping.

```
> Almost. On decision 4, I'd rather put the project in the URL path --
> /api/webhooks/{project}/{provider} -- not in config. That way you can
> configure one GitHub webhook per project in GitHub, and the URL itself carries
> the routing. Config just holds the shared secret per project+provider pair.
```

Decision 4 revised to `webhook-project-from-path`.

**Success Criteria:** 7 musts, 3 shoulds, 5 won'ts.

```
> Payload size limits should be a Must, not a Should. This is a publicly
> exposed endpoint -- without size limits, anyone can POST a 2GB body and OOM
> the server. Enforce before reading the body, not after parsing.
```

Payload size limits moved to Must.

**Risks:** 6 identified (signature divergence, event-to-action mapping, ADR-004 transactions, secret rotation, debugging, provider API stability).

Shape saved with 4 decisions linked. Constitution-check and peripheral-vision ran in background.

---

## Specify

**Interface Contract:**
- `WebhookProvider` Go interface (Name, ValidateRequest, NormalizeEvent) with 3 sentinel errors
- HTTP endpoint: `POST /api/webhooks/{project}/{provider}` with 7 response codes
- `WebhookEvent` domain type (8 fields including ContentHash)
- `WebhookBackend` storage interface (4 methods)
- `Processor` with 5-step processing sequence

```
> Contract looks solid. One note on the e2e tests we'll write later -- they
> should verify database state after processing, not just HTTP response codes.
```

**Verify Criteria:** 29 criteria across 10 categories (signature verification, normalization, dedup, persistence/DB state, payload limits, extensibility, project scoping, error handling, transaction integrity).

**Invariants:** 5 (no silent drops, signature before parsing, deterministic content hash, no spec/slice corruption via webhooks, provider isolation).

**Touches:** 14 new files + 5 modified files.

Specify saved. Red-team, consistency, and constitution-check passes ran.

---

## Decompose

**Strategy:** Steel Thread — prove the full round-trip with GitHub, then broaden.

**Slices:**
1. `steel-thread-github` (no deps) — Full pipeline with GitHub provider, 15 files
2. `broaden-ado` (depends on 1) — ADO provider implementation, 3 files
3. `event-to-action-mapping` (depends on 1) — Wire Processor to state transitions, 2 files
4. `e2e-tests` (depends on 1, 2, 3) — Cross-slice integration tests, 2 files

Slices 2 and 3 parallelize. Slice 4 is the fan-in.

```
> Good decomposition. The steel thread proves the full pipeline with GitHub,
> then ADO and event mapping can parallelize. Persist it.
```

Decompose saved. Simplicity and constitution-check passes ran.

---

## Approve

6-checkpoint review:

1. **Scope bounded?** ✅ — 9 in, 7 out, 2 kill conditions
2. **Interface defined?** ✅ — 5 interfaces, 7 status codes. Red-team flagged TOCTOU on delivery-ID check → fix with INSERT ON CONFLICT
3. **Verify criteria testable?** ✅ — 29 criteria, all concrete. Consistency flagged transaction boundary contradiction and short-circuit vs no-silent-drops → fix by persisting "duplicate" records
4. **Dependencies mapped?** ✅ — No deps (empty graph, standalone spec)
5. **Constitution compliance?** ✅ — No violations. Raw HTTP exception documented. Red-team flagged constant-time comparison requirement
6. **Risk acknowledgment?** ✅ — All 6 risks mitigated/accepted. Audit log flooding accepted for v1 with rate-limiting-in-front caveat

**Approved:** `inbound-webhooks` at 2026-04-10T14:24:32Z

Notable implementation caveats:
- INSERT ON CONFLICT for atomic dedup
- Constant-time comparison for all signature/secret checks
- Delivery-ID dedup must persist "duplicate" record
- Processor constructor needs SyncBackend + SliceBackend
- WebhookEventRecord needs ProjectSlug field
