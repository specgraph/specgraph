# Example Spec: OAuth2 Refresh Token Rotation

This page shows a complete spec as it looks after passing through all five
stages of the [authoring funnel](authoring.md). It uses the OAuth2 refresh
token rotation scenario from the stage examples — assembled into a single,
cohesive document that represents what an agent receives when it claims work.

Use this as a reference for what "done" looks like before implementation begins.

---

## Header

!!! info "Why this section exists"
    Every spec starts with identity and metadata. The `slug` is how you reference
    it everywhere — in commands, edges, and conversation. The `intent` is the
    one-sentence answer to "what does this spec achieve?"

```yaml
slug: oauth-refresh-rotation
intent: Implement refresh token rotation with lineage tracking and grace periods
priority: p1
complexity: medium
stage: approved
version: 1
```

---

## Spark

!!! info "Why this section exists"
    Spark captures the raw idea before it gets refined away. The goal is speed —
    get the seed recorded so you can evaluate whether it's worth pursuing. The
    **kill test** is especially important: it forces you to articulate the
    conditions under which this work should be abandoned, which prevents sunk-cost
    thinking later.

| Field | Value |
|-------|-------|
| **Seed** | OAuth2 refresh token rotation — rotate on every use, invalidate old tokens after a grace period |
| **Signal** | Q1 security audit finding: refresh tokens are long-lived and never rotated, increasing exposure window if leaked |
| **Scope Sniff** | medium |
| **Kill Test** | If we migrate away from OAuth2 entirely (unlikely — no plans on roadmap) |

---

## Shape

!!! info "Why this section exists"
    Shape turns a vague idea into a bounded proposal. This is where most
    design work happens — scoping, exploring alternatives, making decisions,
    and defining what success looks like. A well-shaped spec prevents the
    most common failure mode: jumping from idea to implementation without
    exploring tradeoffs.

### Scope

!!! tip "In vs Out"
    Scope makes boundaries explicit. **In** lists what this spec delivers.
    **Out** lists what it explicitly does NOT deliver — and why. The "out"
    list is just as important as the "in" list: it prevents scope creep by
    making exclusions visible and deliberate, not accidental.

**In:**

- Rotate refresh token on every `grant_type=refresh_token` request
- Lineage chain tracking to detect token reuse attacks
- Configurable grace period for old tokens (handles mobile network retries)
- Revoke entire token family on detected reuse

**Out:**

- Token introspection endpoint changes (separate spec)
- PKCE enforcement changes (already in progress as `pkce-enforcement`)
- Refresh token encryption at rest (tracked as `token-encryption-at-rest`)

### Approaches

!!! tip "Recording rejected options"
    Approaches documents the design options you considered and their tradeoffs.
    This isn't busywork — it's the record of *why* you chose what you chose.
    When someone asks "why didn't we just use single-use tokens?" six months
    later, the answer is here, not buried in a Slack thread.

| Approach | Description | Tradeoffs |
|----------|-------------|-----------|
| single-use-tokens | Old token invalidated immediately on rotation | Simple. Breaks mobile clients that retry with the old token during network hiccups. |
| lineage-chain | Track parent→child token relationships, allow grace period | Handles concurrent requests. More storage, needs background cleanup. |
| sliding-window | Old token valid for N seconds regardless of rotation | Simplest grace period. Doesn't detect reuse attacks — both tokens stay valid. |

**Chosen:** lineage-chain — handles the concurrent request problem that affects
mobile clients without sacrificing reuse detection.

### Decisions

!!! tip "First-class graph nodes"
    Decisions are linked to the spec via `DECIDED_IN` edges. They capture
    architectural choices that affect multiple specs or have long-term
    consequences. Each decision has a slug so it can be referenced, queried,
    and impact-analyzed independently of the spec that originated it.

| Slug | Title | Decision | Rationale |
|------|-------|----------|-----------|
| token-rotation-lineage | Rotation strategy | Lineage chain tracking over single-use | Mobile clients replay old tokens during network retries; single-use would force re-auth on every retry. Lineage tracking detects true reuse (different client) vs benign retry (same client, race condition). |
| grace-period-30s | Grace period duration | 30 seconds | Covers 99th percentile mobile retry window (measured at 12s p99). 30s gives margin without leaving tokens valid long enough for practical exploitation. |
| revoke-on-reuse | Reuse detection response | Revoke entire token family | Per RFC 6819 §5.2.2.3. If a rotated-out token is used outside the grace window, assume compromise and invalidate the entire lineage chain, forcing re-authentication. |

### Success Criteria

!!! tip "MoSCoW prioritization"
    **Must** items are ship-blockers — if any fail, the spec isn't done.
    **Should** items are strong preferences you'd fix before calling it "good."
    **Won't** items are explicitly deferred — recording them prevents
    re-litigation of scope during implementation.

**Must:**

- Refresh token rotated on every use — new token pair issued, old token enters grace period
- Old token rejected after grace period expires
- Token reuse outside grace window revokes entire family
- Existing sessions not disrupted during rollout (grace period covers in-flight requests)

**Should:**

- Rotation adds < 50ms latency to the token endpoint (p99)
- Monitoring dashboard shows rotation rate, reuse detection events, family revocations

**Won't:**

- Migrate existing long-lived tokens to lineage format (they expire naturally)
- Support configurable per-client grace periods (single global setting for now)

---

## Specify

!!! info "Why this section exists"
    Specify makes the spec precise enough for implementation without making
    implementation decisions. It answers "what does this system do?" not "how
    is it built?" After Specify, the spec is testable — every verify criterion
    maps to an automated test, and every invariant is a property the system
    must maintain.

### Interface Contract

!!! tip "The external-facing behavior"
    The interface contract defines the API surface — inputs, outputs, status
    codes, and error conditions. It's what consumers depend on. Changes to
    this contract after approval require a new spec version.

```text
#### Token Endpoint
POST /oauth/token
  grant_type=refresh_token
  refresh_token=<old_token>

Response (200):
  access_token, refresh_token, expires_in, token_type

Error (401): old token past grace period
Error (401): token family revoked (reuse detected)
```

### Verify Criteria

!!! tip "Your acceptance tests"
    Each criterion should be directly translatable into an automated test.
    When the implementing agent reports completion, these are what get checked.
    If they all pass, the spec is done.

- `[rotation]` `POST /oauth/token` with valid refresh token returns new token pair and 200
- `[rotation]` Old refresh token accepted during grace period (30s), rejected after
- `[rotation]` Concurrent rotation requests (same old token, < 1s apart) both succeed -- both get valid new tokens in the same lineage
- `[security]` Using a rotated-out token after grace period triggers family revocation
- `[security]` After family revocation, all tokens in the lineage return 401
- `[expiry]` Token endpoint latency p99 < 50ms with rotation enabled (benchmark against baseline)

### Invariants

!!! tip "Stronger than verify criteria"
    Invariants are properties that must hold at all times — before, during, and
    after execution. A verify criterion says "this test passes," while an
    invariant says "this property is never violated." Invariants survive beyond
    the spec — they become system-level guarantees that future specs must also
    respect.

- At most one active refresh token per session outside the grace window
- Every refresh token has exactly one parent (except the root token)
- A revoked family cannot be un-revoked — new auth flow required
- `lineage_id` is immutable once set

### Touches

!!! tip "Blast radius and drift detection"
    Touches lists the files that will be modified. This helps the implementing
    agent understand the blast radius, and it enables SpecGraph's drift
    detection — if a touched file changes outside this spec's implementation,
    drift is flagged automatically.

- `internal/auth/token.go` — rotation logic, lineage tracking
- `internal/auth/store.go` — new queries for lineage chain operations
- `internal/auth/middleware.go` — reuse detection in token validation
- `migrations/007_token_lineage.sql` — add `lineage_id`, `parent_token_id`, `rotated_at` columns
- `internal/auth/cleanup.go` — background job for grace period enforcement

---

## Decompose

!!! info "Why this section exists"
    Decompose breaks the spec into independently deliverable slices. Each slice
    is small enough to implement, test, and review in isolation. The **strategy**
    determines how you cut: vertical slices deliver end-to-end value in each
    piece, horizontal layers split by architecture tier, or single unit
    delivers the spec as-is when it's small enough.

!!! tip "Graph edges from Depends On"
    The **Depends On** column creates `DEPENDS_ON` graph edges between slices.
    SpecGraph uses these to determine execution order and identify
    parallelizable work via the `specgraph ready` command.

**Strategy:** vertical_slice

| Slice | Intent | Verify | Depends On |
|-------|--------|--------|------------|
| rotation-storage | Add lineage tracking columns and queries | Migration runs; existing tokens still work; new tokens get `lineage_id` | — |
| rotation-endpoint | Implement token rotation in the refresh endpoint | New token pair issued on refresh; old token enters grace period; `parent_token_id` set correctly | rotation-storage |
| grace-enforcement | Background job invalidates tokens past grace period | Old token rejected after 30s; tokens within grace window still accepted | rotation-endpoint |
| reuse-detection | Detect and respond to token reuse attacks | Rotated-out token used after grace triggers family revocation; all family tokens return 401 | grace-enforcement |

---

## What Happens Next

This spec is now **approved** and ready for execution:

1. An agent (or human) **claims** the spec with a lease duration
2. **GenerateBundle** packages the spec + decisions + callback URLs
3. The agent calls **GetPrime** to get the project constitution and coding conventions
4. Implementation proceeds slice-by-slice, with **progress** and **blocker** reports along the way
5. **ReportCompletion** transitions the spec to `done` and releases the claim

If requirements change while the spec is in flight (`approved`, `in_progress`, or
`review`), it can be **amended** — returning to an earlier authoring stage. If a
completed spec (`done`) needs replacement, it can be **superseded** by a new spec
that links back to this one.
