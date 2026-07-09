# Phase 2: API Key Lifecycle & Self-Service - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-09
**Phase:** 2-API Key Lifecycle & Self-Service
**Areas discussed:** AUTH-02 re-sync mechanism, AUTH-02 scope depth, AUTH-03 surface scope, AUTH-03 deferred open questions

---

## AUTH-02 re-sync mechanism (spgr-c2lb)

### Trigger

| Option | Description | Selected |
|--------|-------------|----------|
| Operator-driven | Explicit admin action writes the DB role; SpecGraph does not call the IdP. resolveAPIKey propagates to standing keys automatically. Simplest, no new egress. | |
| IdP-proactive fetch | SpecGraph queries the IdP for current app-role assignments on-demand/scheduled. Fully automated; adds egress, credentials, throttling, per-provider integration. | |
| Hybrid | Ship operator-driven now, design the entrypoint so an automated/IdP driver can reuse the same internal path later. | ✓ |

**User's choice:** Hybrid
**Notes:** Operator path ships this phase; the reusable RPC seam makes future automation cheap.

### Effect on standing keys

| Option | Description | Selected |
|--------|-------------|----------|
| Re-derive DB role only | Force-write the lower role; resolveAPIKey clamps all standing keys to it. Keys keep working at reduced privilege. No deletion. | |
| Revoke standing keys | Hard-revoke the user's keys so they must re-mint. Stronger kill-switch; more disruptive. | |
| Both, role by default | Re-derive role by default (covers downgrade); separate explicit flag/command to hard-revoke for full off-boarding. | ✓ |

**User's choice:** Both, role by default
**Notes:** Two distinct operator intents kept separate — downgrade vs full off-board.

### Durability vs login-sync

| Option | Description | Selected |
|--------|-------------|----------|
| Make revocation durable | Add role_source provenance column so an operator demotion isn't clobbered by login-sync rule 2/3 on next login. | |
| No provenance, rely on convergence | No schema change; a demotion sticks until next interactive login re-derives from claims (which yields the lower role anyway if truly revoked upstream). | ✓ |
| Other / describe | — | |

**User's choice:** No provenance, rely on convergence
**Notes:** Keeps AUTH-02 schema-stable; convergence is acceptable because a genuinely-revoked upstream role re-derives low on next login.

### Command surface

| Option | Description | Selected |
|--------|-------------|----------|
| auth user subcommand + reusable RPC | New `auth user resync <user-id> --role <r>` (+ `--revoke-keys`), backed by one reusable RPC reusing UpdateUserRole/RevokeAPIKey. | ✓ |
| Under api-key group | Frame as a key-lifecycle op (`auth api-key revoke-user <user-id>`). | |
| You decide | Placement/naming left to the agent. | |

**User's choice:** auth user subcommand + reusable RPC
**Notes:** Exact subcommand/flag naming is planner's discretion.

---

## AUTH-02 scope depth

| Option | Description | Selected |
|--------|-------------|----------|
| Bounded slice | Operator command + reusable RPC + `--revoke-keys` only; no scheduler, no IdP polling, no provenance. Meets SC#3. | ✓ |
| Full subsystem | Also build automated/scheduled re-sync or IdP polling this phase. Larger surface; toward spgr-tmqm territory. | |

**User's choice:** Bounded slice
**Notes:** Automation/IdP-fetch deferred; the hybrid seam is ready for it.

---

## AUTH-03 surface scope

| Option | Description | Selected |
|--------|-------------|----------|
| CLI-first, web deferred | Ship CLI self-service + all four RPCs + safeguards; defer the web MCP Keys panel. | |
| Both CLI + web | Ship the CLI surface AND the web MCP Keys dashboard panel (net-new SvelteKit + CSRF). | ✓ |
| Other / describe | — | |

**User's choice:** Both CLI + web
**Notes:** Makes the web CSRF hardening decision live this phase.

---

## AUTH-03 deferred open questions

### Expiry caps

| Option | Description | Selected |
|--------|-------------|----------|
| 90d default / 180d max | Keep 90d default, lower max cap to 180d to shrink the stale-role window. Server-configurable. | ✓ |
| 90d default / 365d max | Keep the design's original caps. | |
| You decide | — | |

**User's choice:** 90d default / 180d max
**Notes:** Tighter max cap directly bounds the stale-privilege-until-relogin window.

### Web CSRF hardening

| Option | Description | Selected |
|--------|-------------|----------|
| Explicit CSRF token | POST-only + explicit CSRF token on top of SameSite=Lax + JSON preflight. Strongest; more work. | ✓ |
| SameSite=Strict | Tighten cookie to Strict; simpler but dashboard-wide UX side effects. | |
| Rely on Lax + preflight | Existing implicit mitigation only. Least work. | |

**User's choice:** Explicit CSRF token
**Notes:** Chosen because these mutations mint credentials; token mechanism (synchronizer vs double-submit) is planner's discretion.

---

## the agent's Discretion

- Exact CLI subcommand + flag names for the AUTH-02 resync/off-board command.
- Precise RPC/service-method shape of the reusable re-sync seam (callable by CLI-now and future automation).
- CSRF token mechanism specifics and wiring.
- Implementation-review-level details the g7st design flags but doesn't fully pin (rate-limit thresholds, audit-log line shape) — follow the design.

## Deferred Ideas

- Automated / scheduled AUTH-02 re-sync (background job) — seam ready.
- IdP-proactive role fetch — overlaps spgr-tmqm.
- Role provenance column (`role_source`) for durable manual grants.
- Harness-config rewrite (drop `${SPECGRAPH_API_KEY}` indirection) — spgr-tmqm.
- MCP OAuth 2.1 resource-server behavior — spgr-tmqm, later phase.
