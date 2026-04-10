# Recording 4: Supersede Demo — Transcript

**Spec:** event-ingestion-pipeline superseding inbound-webhooks | **Flow:** Supersede → Shape

---

## Pre-work (done before recording)

- `specgraph claim inbound-webhooks --agent demo-agent` — claimed
- `specgraph report-completion inbound-webhooks --agent demo-agent` — transitioned to done
- `specgraph create event-ingestion-pipeline --intent "Unified event ingestion pipeline..."` — sparked at v1

---

## Supersede

```
> We implemented inbound-webhooks but we've learned that raw HTTP handlers won't
> scale. We need schema validation on inbound payloads, dead-letter handling for
> events that fail processing, CloudEvents format for standardization, and
> pluggable routing instead of hardcoded provider handlers. I've already sparked
> event-ingestion-pipeline as the replacement. Supersede inbound-webhooks with
> event-ingestion-pipeline and shape the replacement.
```

Agent fetches both specs, verifies state:

| Spec | Stage | Version |
|------|-------|---------|
| inbound-webhooks | done | v6 |
| event-ingestion-pipeline | spark | v1 |

Executes the supersede:

```
Superseded: inbound-webhooks (lifecycle=SPEC_LIFECYCLE_TASK)
Created:    event-ingestion-pipeline (lifecycle=SPEC_LIFECYCLE_TASK, stage=spark)
```

**Insight:** Supersede creates a lineage edge, not a replacement. The graph now has `inbound-webhooks → SUPERSEDED_BY → event-ingestion-pipeline`. The superseded spec's full history remains queryable — nothing is deleted.

---

## Shape (of the replacement)

Constitution principles identified as load-bearing:
- "Proto is the API contract source of truth" — CloudEvents needs proto representation
- "Domain types in storage, proto types at the boundary"
- "All multi-query writes use transactions" / ADR-004
- Anti-pattern: "Business logic in the CLI layer"

### Scope In/Out

**In scope (7 items):** CloudEvents v1.0 envelope, JSON Schema validation, dead-letter storage (queryable), pluggable router by event attributes, GitHub + ADO adapters (reusing signature verification), migration path from existing schema, routing config.

**Out of scope (8 items):** Outbound event emission, external schema registry, message broker/queue, replay/redelivery, Discord/Slack adapters, event batching, SSE/WebSocket, rate limiting enforcement.

```
> Scope is right. One refinement: the migration path from existing webhook_events
> table should be a separate slice, not just an in-scope bullet. It's a
> non-trivial data migration that shouldn't block the new pipeline from being
> testable. Move to approaches.
```

### Approaches

**Approach A: Linear pipeline with router (recommended)** — HTTP → Adapter → Schema Validator → Dedup → Router → Handler. Router maps (type, source) to handlers. Dead-letter taps at validation and handler failure points.

**Approach B: Composable middleware chain** — Each stage is `func(ctx, *CloudEvent) (*CloudEvent, error)`. Dead-letter as wrapping middleware. Over-engineering for 4 fixed stages.

**Approach C: Event bus with subscribers** — Internal pub/sub. Dangerously close to the "message bus" kill condition from spark.

```
> A. Linear pipeline. The middleware chain is too clever for what we need. And
> the event bus would violate our own kill condition. Keep it simple.
```

### Decisions

5 decisions captured:

| # | Slug | Decision |
|---|------|----------|
| 1 | cloudevents-canonical-envelope | All events normalized to CloudEvents v1.0 at adapter boundary |
| 2 | linear-pipeline-over-middleware-or-bus | Linear pipeline (Adapter → Validator → Dedup → Router → Handler) |
| 3 | pluggable-routing-by-event-attributes | Router dispatches by (type, source) attributes, replacing URL-path routing |
| 4 | dead-letter-at-explicit-failure-points | Dead-letter at two explicit taps: validation failure and handler failure |
| 5 | json-schema-local-files | Schemas embedded or local files, not a registry service |

### Success Criteria

**Must (11):** CloudEvents normalization at adapter, schema validation before routing, dead-letter for validation and handler failures, pluggable routing by attributes, GitHub/ADO adapters with signature verification, two-layer dedup, payload size limits, transactional processing, queryable dead-letter.

**Should (4):** PR-merged event triggers slice completion, dead-letter includes replay context, adapter isolation (panic containment), structured pipeline logs.

**Won't (6):** Dead-letter replay, rate limiting enforcement, Discord/Slack adapters, outbound events, external schema registry, webhook CLI commands.

### Risks

6 risks:
1. CloudEvents envelope overhead for simple payloads
2. JSON Schema validation as hot-path bottleneck
3. Router pattern-matching expressiveness (exact match vs. DSL)
4. Dead-letter storage growth without automatic cleanup
5. Migration from webhook_events table is non-trivial
6. Two-layer dedup hash input changes with CloudEvents field structure

Shape saved at v3. Constitution-check and peripheral-vision analytical passes ran in background.

**Insight:** The migration-as-separate-slice pattern avoids coupling "new system works" with "old data is migrated" — two different failure modes that deserve separate verification.
