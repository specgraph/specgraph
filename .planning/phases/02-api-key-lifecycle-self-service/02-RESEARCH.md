# Phase 2: API Key Lifecycle & Self-Service - Research

**Researched:** 2026-07-09
**Domain:** Identity/Authn/Authz hardening on a brownfield Go / ConnectRPC / pgx-postgres / Cedar / SvelteKit codebase (SpecGraph). Credential minting, role clamping, and forced role re-sync.
**Confidence:** HIGH (grounded against actual source; AUTH-03 has a rev-5 canonical design; AUTH-02 design is the CONTEXT decisions D-01..D-05)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**AUTH-02 — Forced role re-sync / off-board (`spgr-c2lb`; net-new design)**

- **D-01 (trigger = Hybrid):** Forced re-sync is **operator-driven** this phase — SpecGraph does NOT call the IdP. But the server-side entrypoint MUST be built as a single reusable seam (RPC/service method) that a future automated or IdP-fetch driver can call without rework. Ship the operator path; leave the automation seam ready. Grounding: `resolveAPIKey` already computes `EffectiveRole = clampedRole(user.Role, key.RoleDowngrade)` from the **live DB role every request**, so the instant `users.role` is updated in the DB, every standing key reflects the new role on its next call. The only real gap is a non-interactive path to write that DB role.
- **D-02 (effect = re-derive role by default, revoke on demand):** A forced re-sync **re-derives / writes the user's DB role** by default (downgrade → all standing keys immediately clamp to the lower role; keys keep working at reduced privilege). A **separate explicit `--revoke-keys` flag/command** hard-revokes the user's standing keys for full off-boarding (forces re-mint). Two distinct operator intents, kept separate.
- **D-03 (durability = convergence, NOT provenance):** Do **NOT** add a `role_source` provenance column. A forced demotion sticks until the user's next interactive OIDC login re-derives from claims — at which point, if the app-role was truly revoked upstream, `claims_mapping` (rules 2/3) yields the same lower role, so it **converges**. No schema change. (Trade-off acknowledged: a login through a *mapping-less* provider leaves the manual lower role intact via rule 1 = freeze, which is fine; a login where the upstream role was NOT actually revoked would re-promote — acceptable, operator over-reached.)
- **D-04 (command surface):** New subcommand under the existing `specgraph auth user` group — e.g. `auth user resync <user-id> --role <r>` to force the demotion, plus `--revoke-keys` for the hard off-board. Backed by **one server-side RPC/entrypoint** the CLI calls now and future automation reuses (the D-01 seam), reusing existing `UpdateUserRole` + `RevokeAPIKey` plumbing. Exact subcommand/flag naming is planner's discretion.
- **D-05 (scope = bounded slice):** AUTH-02 delivers exactly the operator command + reusable RPC + `--revoke-keys`. **NO** scheduled/background job, **NO** IdP polling, **NO** provenance column. Roadmap SC#3 is met (revocation reaches standing keys on forced re-sync, not only next login). Automation/IdP-fetch is deferred; the hybrid seam (D-01) makes picking it up later cheap.

**AUTH-03 — Self-service MCP API keys (`spgr-g7st`; follows the rev-5 design)**

- **D-06 (design is canonical):** Implement per `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` — all locked decisions carry forward and MUST be honored (new Cedar verb `apikey.self` + `"self"` in `knownVerbs` + `self`-only-on-`apikey.*` drift test; four new `IdentityService` RPCs `CreateMyAPIKey`/`ListMyAPIKeys`/`RotateMyAPIKey`/`RevokeMyAPIKey`; owner strictly from `auth.IdentityFromContext`; owner-scoped storage mutations with `AND user_id = $caller`, `RowsAffected()==0 → CodeNotFound`; `role_downgrade` laundering fix — **floor at caller `EffectiveRole` on BOTH create and rotate** via new **exported** `auth.RoleMin`; `Source=="apikey"` anti-key-chaining rejection; CLI session-credential auth precedence — Finding D; `ListMyAPIKeys` hard-setting `UserID` from context; quota TOCTOU handling — explicit `AuthStore` tx + parent-`users`-row `FOR UPDATE`; per-identity rate-limit; emit-once delivery). **NOT re-decided here.**
- **D-07 (surfaces = BOTH CLI + web):** Ship both. CLI: `auth api-key create/list/rotate/revoke` self-variants (no `--user-id` → self path; `--user-id <other>` keeps the admin path). Web: new **"MCP Keys" dashboard panel** (one-time reveal modal, list/create/revoke/rotate, `specgraph_session`-cookie auth) — net-new SvelteKit work (dashboard is currently read-only). Both surfaces consume the same four RPCs.
- **D-08 (expiry caps = 90d default / 180d max):** Resolve the design's first open question — **lower the max cap from 365d to 180d** (default stays 90d), tightening the stale-privilege window. All values remain server-configurable. Per-user active-key quota stays at design default (10, server-configurable).
- **D-09 (web CSRF = explicit CSRF token):** Resolve the design's second open question — add an **explicit CSRF token** (synchronizer or double-submit) on the POST-only self-mint/revoke/rotate mutations, on top of the existing `SameSite=Lax` cookie + JSON-content-type preflight.

### the agent's / Planner's Discretion
- Exact CLI subcommand + flag names for the AUTH-02 resync/off-board command (D-04).
- Precise RPC/service-method shape of the reusable re-sync seam (D-01/D-04) — must be callable by both CLI-now and future automation.
- CSRF token mechanism specifics (synchronizer vs double-submit) and issuance/validation wiring (D-09).
- Any implementation-review-level details the g7st design flags but doesn't fully pin (rate-limit thresholds, audit-log line shape) — follow the design's guidance.

### Deferred Ideas (OUT OF SCOPE)
- **Automated / scheduled AUTH-02 re-sync** — background job that periodically re-applies roles. The D-01 hybrid RPC seam is built so this is a cheap follow-up.
- **IdP-proactive role fetch** (query the IdP's app-role assignments directly, no interactive login) — overlaps `spgr-tmqm` territory.
- **Role provenance column (`role_source`)** — considered and dropped (D-03 convergence).
- **Harness-config rewrite** (drop `${SPECGRAPH_API_KEY}` env indirection; file-native/OAuth delivery) — rides with `spgr-tmqm`.
- **MCP OAuth 2.1 resource-server behavior** (401 + `WWW-Authenticate`, RFC 9728/8707, IdP-issued short-lived audience-bound tokens) — `spgr-tmqm`, a later phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| **AUTH-02** (`spgr-c2lb`) | Enforce app-role revocation on standing API/MCP keys, forcing re-sync | New operator RPC seam re-uses `UpdateUserRole` (writes `users.role`) + `RevokeAPIKey`. `resolveAPIKey` (`identitystore.go:306`, clamp at `:346`) already reads live DB role every request → writing the role propagates to all standing keys on next call. No schema change (D-03). See §"AUTH-02 Implementation Approach". |
| **AUTH-03** (`spgr-g7st`) | Self-service / automatic MCP API-key provisioning for OIDC users | Canonical rev-5 design + verified §7 touch-surface (below). Four new `apikey.self` RPCs, owner-scoped storage mutations, `RoleMin` floor, `Source=="apikey"` gate, quota-safe mint via explicit `AuthStore` tx, CLI + web surfaces. See §"AUTH-03 Implementation Approach" and §"Verified Touch-Surface". |

**Phase success criteria mapping:**
- SC#1 (self-provision without borrowing admin key) → AUTH-03 four RPCs + CLI/web surfaces + CLI session-credential precedence fix (Finding D).
- SC#2 (effective role capped at caller's current role at mint/rotate, no laundering) → AUTH-03 `RoleMin(requestedOrInherit, caller.EffectiveRole)` floor on create AND rotate + `Source=="apikey"` rejection.
- SC#3 (revoked app-role stops carrying old privilege on forced re-sync, not only next login) → AUTH-02 operator resync RPC writing `users.role` (+ optional `--revoke-keys`).
</phase_requirements>

## Summary

This is a **brownfield hardening phase** on an already-shipped identity stack (Identity storage → Authn resolver → Cedar policy engine → OIDC app-roles/login-sync). Both requirements are additive: no new external dependencies, **no goose migration** (existing `api_keys` columns and `users.role` cover everything), no changes to existing admin RPCs.

**AUTH-03** is the large, near-locked piece. The rev-5 design doc is canonical and absorbed four adversarial review rounds; its §7 touch-surface is the authoritative work-breakdown. My grounding confirms the design's cited symbols/line-numbers are **accurate with only minor drift** (see §"Code Drift Ledger"). The core security invariant is *"snapshot ceiling, live floor"*: every self-minted key carries a non-empty `role_downgrade = RoleMin(requested-or-inherit, caller.EffectiveRole)`, floored on **both** create and rotate, so a self-minted key can never exceed the creator's effective role at mint time. Three additional gates matter: (a) reject callers whose `Source == "apikey"` (anti key-chaining); (b) owner-scoped storage WHERE clauses returning `CodeNotFound` uniformly (enumeration-hardening); (c) a quota-safe mint using an **explicit `AuthStore` transaction** that locks the parent `users` row `FOR UPDATE` — because `AuthStore` is structurally *not* reachable from `*Store.RunInTransaction` (ADR-004 caveat, confirmed in intel).

**AUTH-02** is small and reduces to plumbing: `resolveAPIKey` already resolves the owner's **live DB role every request**, so making revocation "reach standing keys" is simply *writing the DB role via a non-interactive path*. `UpdateUserRole` already does the write. The net-new work is (1) an operator-facing RPC seam built so future automation can reuse it, (2) a `auth user resync` CLI subcommand, and (3) an optional `--revoke-keys` hard off-board reusing `RevokeAPIKey`. D-03 chooses convergence over a provenance column, so no schema change.

**Primary recommendation:** Sequence strictly proto → gen (`task proto`) → auth (verbs/actions/RoleMin/Cedar) → storage (owner-scoped methods + quota mint) → server handlers → CLI → web, with the mandated drift/mirror tests and adversarial security tests (laundering, key-chaining, quota TOCTOU, CSRF, cross-user leak) as first-class deliverables. Treat the g7st §7 checklist as the AUTH-03 task list verbatim; treat D-01..D-05 as the AUTH-02 design.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Self-mint authorization (who may mint) | API / Backend (Cedar `apikey.self` + handler gates) | — | Authz is server-owned; Cedar gates on `EffectiveRole`, handler enforces source/floor. Never client-trusted. |
| Owner derivation | API / Backend (`auth.IdentityFromContext`) | — | Owner MUST come from the authenticated identity, never a request field (GitHub PAT rule). |
| Role floor / laundering fix | API / Backend (`auth.RoleMin` in server handler) | Auth pkg (helper) | `EffectiveRole` floor computed at mint time in the handler; ordering helper lives in `internal/auth`. |
| Ownership enforcement | Database / Storage (`AND user_id = $caller` WHERE) | — | Enforced in the SQL WHERE clause, not a pre-read, for enumeration-hardening. |
| Quota + TOCTOU serialization | Database / Storage (explicit `AuthStore` tx + `FOR UPDATE`) | — | `AuthStore` uses `s.pool` directly; not in `*Store.RunInTransaction`. Row lock serializes a user's mints. |
| Rate limiting (argon2 DoS) | API / Backend (per-identity limiter) | — | Reuse JIT limiter pattern (`identitystore.go:643`). |
| Emit-once delivery | API / Backend (plaintext in response) + CLI/Web (display once) | — | Secret returned once; never written to disk by SpecGraph. |
| CSRF protection | Frontend Server (HTTP layer: cookie→bearer + CSRF token) | Browser (token echo) | Cookie-auth'd mutations need CSRF defense at the `internal/server` HTTP boundary. |
| MCP Keys dashboard | Browser / Client (SvelteKit routes + lib) | Frontend Server (session cookie) | Net-new UI; consumes the four RPCs via `specgraph_session` cookie. |
| Forced role re-sync (AUTH-02) | API / Backend (new RPC → `UpdateUserRole`) | CLI (`auth user resync`) | Operator writes `users.role`; propagation is automatic via `resolveAPIKey`'s live-role read. |
| Hard off-board (`--revoke-keys`) | API / Backend (`RevokeAPIKey` over user's keys) | CLI flag | Distinct operator intent; reuses existing revoke plumbing. |

## Standard Stack

No new libraries. Everything below is **already in `go.mod` / the web toolchain** and verified present.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `connectrpc.com/connect` | v1.19.1 | RPC handlers (`IdentityService`) | Established RPC framework; NOT plain gRPC. `[VERIFIED: go.mod]` |
| `github.com/cedar-policy/cedar-go` | v1.7.0 | Authorization (`apikey.self` verb) | Existing policy engine; verb-group model. `[VERIFIED: go.mod]` |
| `github.com/jackc/pgx/v5` | v5.9.2 | Postgres driver + explicit tx for quota mint | Native driver, pool-based; `AuthStore` uses `s.pool` directly. `[VERIFIED: go.mod]` |
| `golang.org/x/time` | v0.15.0 (`/rate`) | Per-identity self-mint rate limiter | Same lib as JIT limiter (`rateLimiterFor`, `identitystore.go:643`). `[VERIFIED: go.mod]` |
| `golang.org/x/crypto/argon2` | (transitive) | argon2id PHC hashing of key secrets | Existing `argon2idVerify`/`GenerateAPIKeySecret`. `[VERIFIED: identitystore.go:250]` |
| SvelteKit + Vitest | vitest ^3.0.0 | MCP Keys panel + web unit tests | Existing web app; `web/src/routes`, `web/src/lib`. `[VERIFIED: web/package.json]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `google.golang.org/protobuf/types/known/timestamppb` | (in tree) | `expires_at` on requests | Already used in `auth_apikey.go` and handlers. `[VERIFIED: auth_apikey.go:12]` |
| `github.com/spf13/cobra` | (in tree) | CLI subcommands | `auth api-key` / `auth user` groups already cobra-based. `[VERIFIED: auth_apikey.go]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled double-submit CSRF token | A CSRF middleware library (`nosurf`/`gorilla/csrf`) | **Not present in go.mod** — adding a dep needs a legitimacy gate + `checkpoint:human-verify`. The design leans toward a token implementable with `crypto/rand` + the existing session; a full library is likely overkill for POST-only cookie mutations already protected by `SameSite=Lax` + JSON preflight. Planner's discretion (D-09). |
| New self-service config struct | Reusing legacy `APIKeyConfig` | Legacy `APIKeyConfig` (`global.go:254`) is the deprecated static-key model ("ignored after Authn plan, storage owns"). A **new** config struct is needed for expiry default/max, quota, and rate-limit knobs, defaulted in `globalDefaults()`. |

**Installation:** None. `go.mod` unchanged. Run `task proto` after editing `identity.proto`; `task build`; `task check`.

## Package Legitimacy Audit

**No external packages are installed by this phase.** Both requirements are additive against the existing dependency set (`connect`, `cedar-go`, `pgx/v5`, `golang.org/x/time`, `argon2`, SvelteKit/Vitest — all already in `go.mod`/`package.json`).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| *(none — no new deps)* | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

> ⚠️ **Only if the planner chooses a CSRF library for D-09** (rather than a hand-rolled token): that library MUST pass the Package Legitimacy Gate (`gsd-tools query package-legitimacy check --ecosystem crates`? no — `--ecosystem go` is not supported by the seam; use `npm view`-equivalent judgment + official Go module verification) and be gated behind a `checkpoint:human-verify` task. Recommendation is to hand-roll (see Alternatives), avoiding the audit entirely.

## AUTH-03 Implementation Approach (canonical g7st rev-5)

### The core invariant — "snapshot ceiling, live floor"
Every self-minted key stores a non-empty `role_downgrade`. Effective role over the key's life is `clampedRole(currentProjectedRole, mintTimeCeiling) = min(currentProjectedRole, mintTimeCeiling)`:
- **Never escalates** above the role held at mint time (owner promotion does NOT auto-upgrade the key — deliberate non-goal).
- **Still drops** on demotion (`min(reader, writer) = reader`), reflecting live DB role via `resolveAPIKey`.

**Mint-time floor (create AND rotate):**
```
role_downgrade = RoleMin(requestedDowngradeOrInherit, caller.EffectiveRole)
```
where an omitted/inherit request resolves to `caller.EffectiveRole` **before** calling `RoleMin` (empty is unranked → would otherwise fail-closed to reader). A `reader`-effective caller can only mint a `reader`-ceiling key. This closes the `role_downgrade`-laundering hole where `Identity{Role: admin, EffectiveRole: reader}` (a downgraded apikey caller) could otherwise resolve to the owner's admin role.

### The four gates (all handler-level, source-agnostic where possible)
1. **Cedar `apikey.self`** — permits any authenticated principal (reader/writer/admin). Its own verb; does not inherit read/write/delete/manage.
2. **`Source == "apikey"` rejection** (anti key-chaining, Finding F5) — reject `CreateMyAPIKey`/`RotateMyAPIKey` with `CodePermissionDenied`. Stops a leaked non-admin key from self-perpetuating; covers service accounts (apikey-only). A raw OIDC JWT (`Source == "oidc"`) may still mint, capped by the floor.
3. **Owner-scoped storage WHERE** (`AND user_id = $caller`) — `RowsAffected()==0 → CodeNotFound`, so "not yours"/"missing"/"already-revoked" are indistinguishable.
4. **Per-identity rate limit** — reuse `rateLimiterFor` pattern for the argon2id-DoS-adjacent mint/rotate endpoints.

### Lifecycle / idempotency (deliberate asymmetry, Finding F4)
- **Revoke:** `WHERE id=$k AND user_id=$caller` (no `revoked_at` guard) → re-revoking your own key is an idempotent **no-op success**; foreign/missing → `CodeNotFound`.
- **Rotate:** `WHERE id=$k AND user_id=$caller AND revoked_at IS NULL` → revoked/foreign/missing → uniformly `CodeNotFound` (cannot rotate a dead key). Rotate re-applies BOTH caps: `role_downgrade = RoleMin(oldDowngrade, caller.EffectiveRole)` and a defaulted, ≤-cap `expires_at` (a `--ttl`-less rotate defaults to 90d; it does **not** inherit the old window).

### Expiry & quota (D-08)
- **Expiry:** mandatory; default **90d**, server-configurable **max cap 180d** (lowered from the design's 365d per D-08). Request over cap → `CodeInvalidArgument`.
- **Quota:** default **10** active (non-revoked, non-expired) keys per user, any origin (schema has no self-vs-admin column; `label` is user-controlled, not a security boundary). Count query adds `AND (expires_at IS NULL OR expires_at > now())` (the `api_keys_active` index covers revocation but not expiry; un-indexed expiry filter is fine at this cardinality, over-counts fail *safe*).
- **TOCTOU (Finding F2):** `SELECT count(*) … FOR UPDATE` is **invalid** in Postgres and row-locks don't stop a phantom INSERT at READ COMMITTED. The mint MUST: open an explicit `AuthStore` transaction (the `pool.BeginTx` pattern `RotateAPIKey` already uses at `postgres/users.go:426`), lock the parent user row (`SELECT 1 FROM users WHERE id = $caller FOR UPDATE`) to serialize that user's mints, count, then insert. Over-cap → clear error pointing at revoke/rotate.

### Delivery (§4) & audit (§6)
Emit-once: plaintext returned exactly once in the response; CLI prints once with a `${SPECGRAPH_API_KEY}` shell-profile/secret-manager instruction (never runs `export`, never writes a credential file); web reveal modal offers copy + same instruction. Structured audit log on create/rotate/revoke with the self-identity as actor; `label` is user-controlled → treat as untrusted in logs (possible PII); never log token material.

## AUTH-02 Implementation Approach (D-01..D-05, net-new)

### Why it's mostly plumbing
`resolveAPIKey` (`identitystore.go:306`) loads the owner's user row and sets `EffectiveRole = clampedRole(Role(user.Role), Role(key.RoleDowngrade))` (`:346`) on **every request** — no cache. So the instant `users.role` is lowered in the DB, every standing key clamps to the lower role on its next call. `UpdateUserRole` (`identity_handler.go:155` → storage `postgres/users.go` UPDATE with `RowsAffected()==0 → ErrUserNotFound`) already performs that write. The gap AUTH-02 closes: today the only paths that write `users.role` non-interactively are admin `UpdateUserRole` and interactive login-sync — there is no *operator "force this user down now"* command framed as re-sync/off-board.

### The seam (D-01/D-04)
Add **one** server-side entrypoint (RPC on `IdentityService`, e.g. `ResyncUserRole`) that:
1. Writes the target role via the same storage path `UpdateUserRole` uses (re-derive/write role by default — D-02).
2. Optionally (a request flag mirroring `--revoke-keys`) revokes the user's standing keys by listing them (`ListAPIKeys{UserID}`) and calling `RevokeAPIKey` per active key — the hard off-board (D-02).
3. Is shaped so a **future automation/IdP-fetch driver** can call it directly without a CLI (D-01). Keep the role-derivation *input* explicit (operator supplies `--role`) this phase; a future driver supplies a derived role.

**Discretion (planner):** exact RPC name/shape and CLI subcommand/flag names. Suggested: `auth user resync <user-id> --role <r> [--revoke-keys]`. Reuse `render`/`printJSON` output conventions from `auth_user.go`.

### Convergence, not provenance (D-03)
No `role_source` column, no migration. A forced demotion persists until the user's next interactive OIDC login re-derives from claims. If the app-role was truly revoked upstream, `resolveLoginRole` rules 2/3 yield the same lower role → converges. Documented trade-offs: a login via a mapping-less provider (rule 1 = freeze) keeps the manual lower role (fine); a login where the upstream role was NOT actually revoked re-promotes (acceptable — operator over-reached). This mirrors the login-sync design's H2 "IdP is sole authority when sync is on" behavior.

## Verified Touch-Surface (g7st §7, grounded against current code)

> Line numbers below were **re-verified this session**. Where the design's number drifted, the current number is noted. `[VERIFIED: <file>]` = confirmed by reading the file.

**Proto / gen**
- `proto/specgraph/v1/identity.proto`: add 4 self RPCs (`CreateMyAPIKey`/`ListMyAPIKeys`/`RotateMyAPIKey`/`RevokeMyAPIKey`) + the AUTH-02 resync RPC → 8+ req/resp messages + service entries. `IdentityService` currently ends at `UnbindOIDC` (`identity.proto:32`). `[VERIFIED: identity.proto]`
- `task proto` regenerates `gen/` (committed). **Do not hand-edit `gen/`** — a Claude Code PreToolUse hook blocks it (AGENTS.md). Skill `buf-regen` documents the flow.

**Auth package**
- `internal/auth/engine.go:155`: `knownVerbs = map[string]bool{"read":true,"write":true,"delete":true,"manage":true}` — **add `"self":true`** or `actionVerb` errors → `NewCedarEngine` fails → **server won't boot**. `[VERIFIED: engine.go:155]`
- `internal/auth/actions.go:92-107`: `procedureActions` map — add the 4 self procedures → `"apikey.self"` and the resync procedure → `"user.manage"` (admin-only). `[VERIFIED: actions.go]`
- `internal/auth/actions_test.go`: **TWO tests need updating** (design under-counted): (a) `TestActionForProcedure_Identity` (`:50-71`) hard-mirrors the identity procedure→action map — add the 5 new procedures; (b) **`TestActionNames_AllParseToKnownVerb` (`:33-42`) hard-codes `[]string{"read","write","delete","manage"}`** — this MUST gain `"self"` or CI fails. **⚠️ This is a concrete drift the design predicted but did not name the second test.** Also add the new **`self`-verb-only-on-`apikey.*` drift test** here. `[VERIFIED: actions_test.go]`
- `internal/auth/policies/base.cedar` (48 lines): add the `apikey.self` permit (any authenticated role) **plus a comment** noting the handler further restricts it (rejects `Source=="apikey"`, floors role) — Cedar can't see source/role-floor because `principalEntity` (`engine.go:212`, design said `:218`/`:219`) exposes only `role`/`id`/`email`. `[VERIFIED: base.cedar, engine.go:212]`
- `internal/auth/`: new **exported** `RoleMin(a, b Role) Role` (fail-closed for unranked). Ordering helpers `roleRank` (`:256`), `roleLessThan` (`:270`), `clampedRole` (`:289`) are all **unexported** today. Handler must substitute `"" (inherit) → caller.EffectiveRole` **before** `RoleMin`. `[VERIFIED: identitystore.go:256,270,289]`

**Server / handlers**
- `internal/server/identity_handler.go`: 4 self handlers (owner from `auth.IdentityFromContext`, no target field) + `RoleMin` floor on create (`CreateAPIKey` model at `:223`) and rotate (`RotateAPIKey` at `:280`) + `Source=="apikey"` rejection + rotate expiry+role re-cap + per-identity rate limit. Plus the AUTH-02 resync handler reusing `UpdateUserRole` (`:155`) + `RevokeAPIKey` (`:266`). `ListMyAPIKeys` MUST hard-set the filter `UserID` from context — **`ListAPIKeys` with empty `UserID` returns ALL users' keys** (`identity_handler.go:319-324`, `msg.GetUserId()` passed straight through). `[VERIFIED: identity_handler.go]`

**Storage**
- `internal/storage/users.go` (`UsersBackend` interface, `:17`): add owner-scoped `GetAPIKeyForUser(userID,keyID)` (read, for rotate's old-downgrade re-floor), `RevokeAPIKeyForUser(userID,keyID)`, `RotateAPIKeyForUser(userID,keyID,roleDowngrade,expiresAt)` (explicit args — never the inherit-on-nil fallback), plus the count/quota method. `[VERIFIED: users.go:17-97]`
- `internal/storage/postgres/users.go`: implement on `*AuthStore`. Existing `RevokeAPIKey` (`:408`, uses `s.pool.Exec`, WHERE `revoked_at IS NULL`), `RotateAPIKey` (`:425`, `s.pool.BeginTx` + savepoints), `CreateAPIKey` (`:361`, `INSERT … SELECT … WHERE EXISTS`). Owner-scoped variants add `AND user_id = $userID`; the `RowsAffected()==0 → NotFound` pattern is at `:233,:249` (`UpdateUserRole`). The quota-safe mint opens `pool.BeginTx`, `SELECT 1 FROM users WHERE id=$caller FOR UPDATE`, counts, inserts. `[VERIFIED: postgres/users.go]`
- **Confirmed ADR-004 caveat:** `AuthStore` uses `s.pool` directly and is NOT wired into `*Store.RunInTransaction` (intel `decisions.md:57`). Do bespoke `pool.BeginTx` transactions, not `RunInTransaction`. `[VERIFIED: intel/decisions.md:57, postgres/users.go:426]`
- **No migration:** `api_keys` (`auth_migrations/001_initial.sql:36`) already has `user_id`, `prefix`, `phc_hash`, `role_downgrade` (`:41`), `label`, `expires_at` (`:43`), `last_used_at`, `revoked_at` (`:45`), `created_at`. Index `api_keys_active ON api_keys(user_id) WHERE revoked_at IS NULL` (`:49`) covers revocation but **not expiry**. `[VERIFIED: auth_migrations/001_initial.sql]`

**Compile-time interface stubs (MUST gain new methods or the build breaks)**
- `internal/auth/usersbackend_stub_test.go` and `internal/server/usersbackend_stub_test.go` (`var _ storage.UsersBackend = …`). The server stub at `:50-130` already lists `listAPIKeys`/`CreateAPIKey`/`RevokeAPIKey`/`RotateAPIKey`/`UpdateUserRole` — new owner-scoped + quota methods must be added to both stubs. `[VERIFIED: server/usersbackend_stub_test.go]`

**CLI**
- `cmd/specgraph/auth_apikey.go`: self-variants of `create/list/rotate/revoke` — no `--user` → self RPC path; `--user <other>` → existing admin path. Existing flags `--label`, `--role-downgrade`, `--expires-at` map to self-mint. `[VERIFIED: auth_apikey.go]`
- `cmd/specgraph/auth_user.go`: new `resync` subcommand (D-04) reusing `UpdateUserRole` client + a revoke-keys path. `[VERIFIED: auth_user.go:80 set-role pattern]`
- **CLI auth precedence fix (Finding D — mandatory, not advisory):** `client.go:102-111` `resolveAPIKey` prefers `SPECGRAPH_API_KEY` env **before** the stored credential. The stored `specgraph login` session is a `spgr_ws_` token in the credentials file (`login.go:257`, resolved via `resolveSession`). On a box that exported the bootstrap admin key into `SPECGRAPH_API_KEY`, the self-mint call would authenticate as that API key → with the F5 `Source=="apikey"` gate it now **hard-fails `PermissionDenied`**. The self-mint command MUST authenticate with the stored OIDC **session** (`spgr_ws_`) explicitly, ignoring/warning on a set `SPECGRAPH_API_KEY`. `[VERIFIED: client.go:102, login.go:257]`

**Web (net-new; dashboard is read-only today)**
- `web/src/routes/`: current routes are `constitution/`, `decision/`, `graph/`, `spec/` (all read-only). Add an "MCP Keys" panel/route. `[VERIFIED: web/src/routes]`
- `web/src/lib/`: client wiring in `web/src/lib/api/client.ts` (+ generated client under `web/src/lib/api/gen/specgraph`); auth state in `auth.svelte.ts`; existing `LoginModal.svelte` component as a modal reference. Add a one-time-reveal modal component. `[VERIFIED: web/src/lib]`
- **CSRF (D-09):** mutations flow cookie→bearer via `cookieToAuthHeader` (`auth_handler.go:136`) with `SameSite=Lax` session cookie (`auth_handler.go:179`). Add POST-only CSRF token issuance/validation at the `internal/server` HTTP boundary. `[VERIFIED: auth_handler.go]`

**Config**
- New self-service key-policy config struct (expiry default 90d / max 180d, quota 10, rate-limit thresholds), defaulted in `globalDefaults()` (`global.go:433` sets `OIDC` defaults today). **Do NOT reuse legacy `APIKeyConfig` (`:254`)** — it's the deprecated static-key model. `[VERIFIED: global.go:226-259,433]`

**Housekeeping:** new Go files need `// Package` doc comments (revive) + SPDX/Apache-2.0 headers (addlicense); all commits need DCO `Signed-off-by`.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌──────────────────────────────────────────────┐
   CLI (spgr_ws_ session) │           AUTH-03: self-mint flow            │
   Web (specgraph_session ├──────────────────────────────────────────────┤
        cookie + CSRF)    │                                              │
        │                 │  IdentityService.CreateMyAPIKey (apikey.self)│
        ▼                 │                    │                         │
  cookieToAuthHeader ─────┼──► authMW ──► Cedar gate (EffectiveRole)     │
  (cookie → Bearer)       │      │              │ permit any auth'd role │
        │                 │      ▼              ▼                        │
   CSRF token check ──────┼─► handler:  [1] reject Source=="apikey"      │
   (web mutations)        │             [2] owner = IdentityFromContext  │
                          │             [3] role_downgrade =             │
                          │                 RoleMin(reqOrInherit,        │
                          │                         caller.EffectiveRole)│
                          │             [4] rate-limit (per identity)    │
                          │                    │                         │
                          │                    ▼                         │
                          │  AuthStore.CreateMyAPIKeyForUser (explicit tx)│
                          │    BEGIN → SELECT 1 FROM users FOR UPDATE     │
                          │    → count active keys (≤ quota) → INSERT     │
                          │                    │                         │
                          │                    ▼ plaintext (emit once)   │
                          └──────────────────────────────────────────────┘

   ── standing key call ──►  resolveAPIKey ──► EffectiveRole =
                                               clampedRole(LIVE users.role,
                                                           key.role_downgrade)
                                                          ▲
                          ┌───────────────────────────────┴──────────────┐
   Operator CLI           │        AUTH-02: forced re-sync flow          │
   auth user resync ──────┼─► IdentityService.ResyncUserRole (user.manage)│
     --role <r>           │      │                                       │
     [--revoke-keys]      │      ├─► UpdateUserRole → writes users.role ──┘ (all standing keys clamp on next call)
                          │      └─► (if --revoke-keys) ListAPIKeys(uid)
                          │              → RevokeAPIKey per active key
                          └──────────────────────────────────────────────┘
```

### Pattern 1: Owner-scoped mutation returning uniform NotFound
**What:** Enforce ownership in the SQL WHERE clause, not a separate pre-read.
**When to use:** All self RPCs' storage mutations.
**Example:**
```go
// Source: established pattern at internal/storage/postgres/users.go:408 (RevokeAPIKey)
// + :233 (RowsAffected==0 → NotFound). Owner-scoped variant adds AND user_id.
tag, err := s.pool.Exec(ctx, `
    UPDATE api_keys SET revoked_at = $1
    WHERE id = $2::uuid AND user_id = $3::uuid`, s.now(), keyID, userID)
if err != nil { return fmt.Errorf("revoke key for user: %w", err) }
if tag.RowsAffected() == 0 { return storage.ErrAPIKeyNotFound } // not-yours == missing
```

### Pattern 2: Quota-safe mint (explicit AuthStore tx + row lock)
**What:** Serialize a single user's mints; avoid the invalid `count(*) FOR UPDATE`.
**Example:**
```go
// Source: g7st §2 Finding F2 + tx pattern at postgres/users.go:426
tx, _ := s.pool.BeginTx(ctx, pgx.TxOptions{})
defer tx.Rollback(ctx)
if _, err := tx.Exec(ctx, `SELECT 1 FROM users WHERE id = $1::uuid FOR UPDATE`, caller); err != nil { ... }
var n int
tx.QueryRow(ctx, `SELECT count(*) FROM api_keys
    WHERE user_id=$1::uuid AND revoked_at IS NULL
      AND (expires_at IS NULL OR expires_at > now())`, caller).Scan(&n)
if n >= quota { return errOverQuota } // → CodeResourceExhausted / clear FailedPrecondition
// ... INSERT with generated prefix ... tx.Commit(ctx)
```

### Pattern 3: Exported RoleMin floor (handler-side, before storage)
```go
// Source: g7st §7 Finding F3. Ordering helpers unexported in internal/auth.
func RoleMin(a, b Role) Role { // fail-closed for unranked
    ra, oka := roleRank[a]; rb, okb := roleRank[b]
    if !oka || !okb { return RoleReader }
    if ra <= rb { return a }
    return b
}
// handler: downgrade := requested; if downgrade == "" { downgrade = caller.EffectiveRole }
//          floored := auth.RoleMin(downgrade, caller.EffectiveRole)
```

### Anti-Patterns to Avoid
- **Accepting a caller-supplied `user_id`/target on self RPCs** — owner MUST come from `auth.IdentityFromContext`. There is no target field.
- **Passing `msg.GetUserId()` straight into `ListAPIKeys` for the self path** — an empty value lists ALL users' keys. Hard-set from context.
- **`SELECT count(*) … FOR UPDATE`** — invalid Postgres; row-locks don't stop phantom INSERTs at READ COMMITTED. Lock the parent `users` row instead.
- **Using `*Store.RunInTransaction` for API-key writes** — unreachable from `AuthStore`; use `pool.BeginTx`.
- **Inheriting the old key's `role_downgrade`/`expires_at` on rotate via the nil fallback** — re-pins a stale ceiling above the caller's current role (snapshot escalation). Pass explicit floored/capped values.
- **Letting owner promotion auto-upgrade a non-downgraded key** — dropped as a non-goal; mint a new key after promotion (GitHub-PAT posture).
- **Adding `"self"` to the Cedar policy without adding it to `knownVerbs`** — server won't boot.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Role ordering / clamping | A new comparator | Extend the existing `roleRank`/`roleLessThan`/`clampedRole` family; add exported `RoleMin` alongside | Consistency + fail-closed semantics already reviewed (spgr-rjrt.9). |
| Per-identity rate limiting | A custom token bucket | `golang.org/x/time/rate` via the `rateLimiterFor` sync.Map pattern (`identitystore.go:643`) | Already in tree; proven for JIT. |
| API-key secret + PHC hash | New crypto | `auth.GenerateAPIKeySecret` / `FormatAPIKeyToken` / `argon2idVerify` | Existing, constant-time, PHC-encoded. |
| Rotate atomicity | New tx logic | Model on `AuthStore.RotateAPIKey` (`postgres/users.go:425`) savepoint pattern | Handles prefix-collision retry + rollback. |
| Role write for AUTH-02 | A new update path | Existing `UpdateUserRole` (handler `:155`, storage `RowsAffected` guard `:233`) | The propagation is already automatic via `resolveAPIKey`. |
| Session cookie → bearer | New middleware | Existing `cookieToAuthHeader` (`auth_handler.go:136`) | Web mutations already route through it. |

**Key insight:** This domain has *already* been hardened across multiple adversarial review rounds. The failure mode here is **re-inventing** a comparator/limiter/tx that diverges from the reviewed fail-closed behavior — not a missing library. Extend, don't replace.

## Runtime State Inventory

> This is a hardening phase adding new capability, **not** a rename/refactor/migration. No stored keys, IDs, or OS-registered state are being renamed. Included for completeness per the protocol.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `api_keys` rows carry `role_downgrade`; `users.role` is the live role. AUTH-02 **writes** `users.role` (not a rename). Existing keys with empty `role_downgrade` are unaffected by AUTH-03 (only *new* self-minted keys get the floor). | None — no data migration; new writes only. |
| Live service config | None. No external service holds the string being changed. | None — verified: no IdP write path this phase (D-01 operator-only). |
| OS-registered state | None. No scheduler/daemon registrations. | None. |
| Secrets/env vars | `SPECGRAPH_API_KEY` (harness configs) — **read** by the CLI; Finding D changes CLI *precedence* (prefer session for self-mint), not the var name. | Code change only (CLI credential precedence). |
| Build artifacts | `gen/` regenerated by `task proto` after `identity.proto` edits (committed). | Run `task proto`; commit `gen/`. |

**Nothing found requiring data migration** — verified by reading the schema (`api_keys`/`users`), CONTEXT D-03 (no provenance column), and the g7st design ("No storage-schema change").

## Common Pitfalls

### Pitfall 1: `TestActionNames_AllParseToKnownVerb` breaks CI
**What goes wrong:** Adding `"self"` to `knownVerbs` + a `apikey.self` action, but the test at `actions_test.go:40` hard-codes `[]string{"read","write","delete","manage"}` → fails.
**Why it happens:** The design named the `AllParseToKnownVerb` *parse* failure and the mirror map, but the assertion list in the test is a second, separate hard-coded list.
**How to avoid:** Update the assertion slice to include `"self"` in the same task that adds the verb.
**Warning signs:** `task check` red on `internal/auth` before any handler exists.

### Pitfall 2: Server won't boot after Cedar policy edit
**What goes wrong:** Adding the `apikey.self` permit to `base.cedar` without `"self"` in `knownVerbs` → `actionVerb` errors → `NewCedarEngine` fails → startup aborts.
**How to avoid:** Add to `knownVerbs` (`engine.go:155`) **first / same commit** as the policy.
**Warning signs:** Startup error; integration tests fail to construct the engine.

### Pitfall 3: `ListMyAPIKeys` leaks all users' keys
**What goes wrong:** Reusing `ListAPIKeys` with `msg.GetUserId()` empty → returns every user's keys (`identity_handler.go:319`).
**How to avoid:** Hard-set the filter `UserID` from `IdentityFromContext`; add an explicit invariant test.

### Pitfall 4: Rotate re-pins a stale higher ceiling
**What goes wrong:** A since-demoted admin rotates an admin-ceiling key; the storage `RotateAPIKey` inherits the old `role_downgrade` and old `expires_at` (`postgres/users.go:437,460`) → fresh admin-ceiling key past cap.
**How to avoid:** Use `RotateMyAPIKey` → `RotateAPIKeyForUser` with **explicit** floored `role_downgrade = RoleMin(old, caller.EffectiveRole)` and a defaulted ≤-cap `expires_at`.

### Pitfall 5: Quota race admits > quota keys
**What goes wrong:** Two concurrent mints both pass a naive count check; `count(*) FOR UPDATE` is invalid; row locks don't block phantom INSERTs at READ COMMITTED.
**How to avoid:** Explicit `AuthStore` tx + `SELECT 1 FROM users WHERE id=$caller FOR UPDATE` to serialize the user's mints.

### Pitfall 6: Self-mint hard-fails on a normal dev box (Finding D)
**What goes wrong:** `SPECGRAPH_API_KEY` (bootstrap admin key) is set per onboarding; CLI prefers it (`client.go:103`); with the `Source=="apikey"` gate the self-mint call returns `PermissionDenied`.
**How to avoid:** Self-mint command authenticates with the stored `spgr_ws_` session explicitly; warn if `SPECGRAPH_API_KEY` is set.

### Pitfall 7: AUTH-02 demotion "un-sticks" on next login (expected, document it)
**What goes wrong:** Operator forces a demotion; user logs in via a **mapping-less** provider → rule 1 freeze keeps the manual role; or the upstream role wasn't actually revoked → re-promotion.
**How to avoid:** This is the D-03 convergence trade-off — document it, don't guard it. `--revoke-keys` is the hard off-board for true termination.

## Code Examples

### Owner-derived self-mint handler skeleton
```go
// Source: g7st §1; grounded on identity_handler.go:223 (CreateAPIKey) + auth.IdentityFromContext (auth_handler.go:124)
func (h *IdentityHandler) CreateMyAPIKey(ctx context.Context, req *connect.Request[specv1.CreateMyAPIKeyRequest]) (*connect.Response[...], error) {
    id, ok := auth.IdentityFromContext(ctx)
    if !ok { return nil, connect.NewError(connect.CodeUnauthenticated, errUnauth) }
    if id.Source == "apikey" { // Finding F5 anti key-chaining
        return nil, connect.NewError(connect.CodePermissionDenied, errNoKeyChaining)
    }
    if !h.selfMintLimiter(id.UserID).Allow() { // Finding K
        return nil, connect.NewError(connect.CodeResourceExhausted, errRateLimited)
    }
    downgrade := req.Msg.GetRoleDowngrade()
    if downgrade == "" { downgrade = string(id.EffectiveRole) }        // inherit → caller effective
    floored := auth.RoleMin(auth.Role(downgrade), id.EffectiveRole)    // §1 floor
    expiresAt := clampExpiry(req.Msg.GetExpiresAt(), cfg.DefaultTTL, cfg.MaxTTL) // D-08 90d/180d
    // ... AuthStore quota-safe mint with owner = id.UserID, role_downgrade = floored ...
}
```

### AUTH-02 resync handler skeleton
```go
// Source: D-01/D-02; reuses UpdateUserRole (identity_handler.go:155) + RevokeAPIKey (:266)
func (h *IdentityHandler) ResyncUserRole(ctx context.Context, req ...) (...) {
    if err := h.users.UpdateUserRole(ctx, req.Msg.GetId(), req.Msg.GetRole()); err != nil { // writes users.role → live floor
        return nil, h.identityError(ctx, err)
    }
    if req.Msg.GetRevokeKeys() { // D-02 hard off-board
        keys, _ := h.users.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: req.Msg.GetId()})
        for _, k := range keys { _ = h.users.RevokeAPIKey(ctx, k.ID) }
    }
    // ... return updated user ...
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| API keys admin-only (`apikey.manage`, admin) | Self-service `apikey.self` for any authenticated role | This phase (AUTH-03) | Non-admins mint their own role-capped keys; no bootstrap-key borrowing. |
| Role revocation reaches keys only on next interactive OIDC login | Operator can force re-sync onto standing keys immediately | This phase (AUTH-02) | SC#3 met; the `spgr-c2lb` gap named in prior designs is closed for the operator path. |
| rev-2 "interactive-source gate" for laundering | `RoleMin` floor at `caller.EffectiveRole` (source-agnostic) | g7st rev-3+ | The source gate was unimplementable (`resolveSession` returns `"oidc"` like a raw JWT); the floor works with existing `Identity` fields. |
| `clampedRole` fail-open on unranked roles | fail-closed to `reader` | spgr-rjrt.9 (shipped) | The self-mint floor builds on this guarantee. |

**Deprecated/outdated:**
- Legacy static `APIKeyConfig` (`global.go:254`) — "ignored after Authn plan (storage owns)". Do not extend it for self-service policy; add a new struct.
- The design's 365d max cap → superseded by D-08's **180d**.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | CSRF (D-09) is best implemented as a hand-rolled double-submit/synchronizer token (no new dependency) | Standard Stack / Alternatives | If a library is mandated, planner must add a legitimacy-gate + `checkpoint:human-verify` task. Low risk — POST-only + `SameSite=Lax` + JSON preflight already mitigate. |
| A2 | The AUTH-02 seam is a new `IdentityService` RPC (e.g. `ResyncUserRole`) mapped to `user.manage` | AUTH-02 Approach | If a different service placement is chosen, the actions-map/mirror-test wiring differs. Naming is explicitly planner discretion (D-04). |
| A3 | A new self-service config struct is required (not reusing `APIKeyConfig`) | Config touch-surface | If config is threaded differently, defaults location changes. Grounded in `global.go` reading — low risk. |
| A4 | Web mutations authenticate via `cookieToAuthHeader` (`specgraph_session`) exactly like `whoami`; the MCP Keys panel reuses that path | Web touch-surface | If a separate auth path is chosen for the panel, CSRF wiring differs. Grounded — low risk. |
| A5 | Rate-limit thresholds for self-mint follow the JIT limiter's config shape (per-hour refill + burst) | AUTH-03 gates | Exact numbers are implementation-review discretion per the design; wrong defaults are tunable, not structural. |

**Note:** AUTH-03's substantive decisions are **not** assumptions — they are locked in the canonical rev-5 design (D-06). The items above are the genuinely open, planner-discretion points.

## Open Questions

1. **CSRF token mechanism (D-09 discretion).**
   - What we know: POST-only mutations, `SameSite=Lax` cookie, JSON preflight already in place; token must be added on top.
   - What's unclear: synchronizer (server-stored) vs double-submit (cookie+header) — issuance/validation wiring at the `internal/server` HTTP boundary.
   - Recommendation: double-submit with a `crypto/rand` token set as a non-HttpOnly cookie + echoed header, validated in a small middleware wrapping the mutating routes. Avoids server-side token storage and a new dependency.

2. **Self-mint rate-limit thresholds.**
   - What we know: reuse `rateLimiterFor` (`golang.org/x/time/rate`) pattern; needs per-identity keying (not per-issuer).
   - What's unclear: refill/burst values.
   - Recommendation: conservative (e.g. burst 5, refill ~30/hr) as server-configurable defaults; tune at review.

3. **AUTH-02 `--revoke-keys` scope: active-only or include-expired?**
   - What we know: off-boarding intent is "kill their standing keys."
   - Recommendation: revoke all **active** (non-revoked) keys via `ListAPIKeys{UserID}` + `RevokeAPIKey`; already-revoked/expired are no-ops.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain + `task` | Build/proto/test | ✓ | (repo-pinned) | — |
| `buf` (via `task proto`) | Regenerate `gen/` after `identity.proto` | ✓ | (task tools) | — |
| Docker + `pgvector/pgvector:pg18` | Postgres integration tests (`task test:integration`) | ✓ (per AGENTS.md) | pg18 | Unit tests (`task test`) run without Docker; integration/e2e need it. |
| Node + pnpm + Vitest | Web panel unit tests | ✓ | vitest ^3.0.0 | — |

**Missing dependencies with no fallback:** none identified.
**Missing dependencies with fallback:** Postgres integration/e2e tests require Docker; if unavailable in a given environment, unit tests still gate most logic, but the quota-TOCTOU and owner-scoped SQL behaviors **must** be validated under `task test:integration` before phase gate.

## Validation Architecture

> `.planning/config.json` is absent → nyquist_validation treated as **enabled**.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify/require` (unit + Postgres integration via testcontainers); Ginkgo/Gomega (e2e); Vitest ^3 (web) |
| Config file | `Taskfile.yml` (test targets `:142` test, `:153` short, `:158` integration, `:163` e2e); `web/package.json` (`"test": "vitest run"`) |
| Quick run command | `task test` (skips `//go:build integration` + `//go:build e2e`) |
| Full suite command | `task pr-prep` (check → test:integration → test:e2e; requires Docker) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-03 | `self` verb parses + only on `apikey.*` (drift test) | unit | `go test ./internal/auth/ -run TestActionNames` | ✅ extend `actions_test.go` |
| AUTH-03 | `AllParseToKnownVerb` includes `self` | unit | `go test ./internal/auth/ -run TestActionNames_AllParseToKnownVerb` | ✅ update `actions_test.go:33` |
| AUTH-03 | identity procedure→action mirror includes 5 new RPCs | unit | `go test ./internal/auth/ -run TestActionForProcedure_Identity` | ✅ update `actions_test.go:50` |
| AUTH-03 | `RoleMin` table (fail-closed unranked; floor on laundering case) | unit | `go test ./internal/auth/ -run TestRoleMin` | ❌ Wave 0 |
| AUTH-03 | Handler floors `role_downgrade` on create AND rotate | unit | `go test ./internal/server/ -run TestCreateMyAPIKey\|TestRotateMyAPIKey` | ❌ Wave 0 |
| AUTH-03 | `Source=="apikey"` rejected (`PermissionDenied`) | unit | `go test ./internal/server/ -run TestSelfMint_RejectsApikeySource` | ❌ Wave 0 |
| AUTH-03 | `ListMyAPIKeys` hard-sets UserID (no cross-user leak) | unit | `go test ./internal/server/ -run TestListMyAPIKeys_ScopedToCaller` | ❌ Wave 0 |
| AUTH-03 | Owner-scoped revoke/rotate → NotFound for foreign/missing | integration | `go test -tags integration ./internal/storage/postgres/ -run TestAuthStore_.*ForUser` | ❌ Wave 0 |
| AUTH-03 | Quota enforced; concurrent mints don't exceed quota (TOCTOU) | integration | `go test -tags integration ./internal/storage/postgres/ -run TestAuthStore_SelfMintQuota` | ❌ Wave 0 |
| AUTH-03 | Expiry cap (90d default / 180d max) rejected over cap | unit | `go test ./internal/server/ -run TestSelfMint_ExpiryCap` | ❌ Wave 0 |
| AUTH-03 | CLI self-mint uses session, warns/ignores `SPECGRAPH_API_KEY` | unit | `go test ./cmd/specgraph/ -run TestSelfMint_SessionPrecedence` | ❌ Wave 0 |
| AUTH-03 | Web CSRF token required on mutations | web + go | `pnpm -C web test` + `go test ./internal/server/ -run TestCSRF` | ❌ Wave 0 |
| AUTH-02 | Resync writes `users.role` → standing key clamps (live floor) | integration | `go test -tags integration ./internal/storage/postgres/ -run TestResync_LiveRoleClamp` | ❌ Wave 0 |
| AUTH-02 | `--revoke-keys` revokes all active keys | integration | `go test -tags integration ./internal/server/ -run TestResync_RevokeKeys` | ❌ Wave 0 |
| AUTH-02 | CLI `auth user resync` happy/JSON paths | unit | `go test ./cmd/specgraph/ -run TestAuthUserResync` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `task test` (unit; < 30s target) for the touched package(s).
- **Per wave merge:** `task check` (fmt → license → lint → build → unit).
- **Phase gate:** `task pr-prep` (adds `test:integration` + `test:e2e`; Docker required) green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/auth/rolemin_test.go` — `RoleMin` table (covers AUTH-03 laundering floor)
- [ ] `internal/auth/actions_test.go` — add `self` drift test + update the two hard-coded lists (verb list + identity mirror)
- [ ] `internal/server/identity_selfkeys_test.go` — self handlers: source gate, floor, list-scoping, expiry cap, rate limit
- [ ] `internal/server/identity_resync_test.go` — AUTH-02 resync + revoke-keys
- [ ] `internal/storage/postgres/users_selfkeys_test.go` — owner-scoped mutations, quota, TOCTOU (integration)
- [ ] `internal/auth/usersbackend_stub_test.go` + `internal/server/usersbackend_stub_test.go` — add new interface methods (compile gate)
- [ ] `cmd/specgraph/*_test.go` — CLI self-mint precedence + resync command
- [ ] `web/src/lib/**/*.test.ts` — MCP Keys panel + CSRF token behavior (Vitest)

## Security Domain

> `security_enforcement` absent in config → treated as **enabled** (ASVS L1, block on high). This phase **mints and revokes credentials** — treat every gate as security-critical.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Owner from `auth.IdentityFromContext` only; CLI session (`spgr_ws_`) precedence over env key (Finding D); argon2id PHC hashing (existing). |
| V3 Session Management | yes | `specgraph_session` cookie `HttpOnly` + `SameSite=Lax` + dynamic `Secure`; emit-once secret; no credential files written by SpecGraph. |
| V4 Access Control | yes | Cedar `apikey.self` (any auth'd role) + handler gates: `Source=="apikey"` rejection, owner-scoped WHERE, `RoleMin` floor. Admin RPCs unchanged (`apikey.manage`). |
| V5 Input Validation | yes | `role_downgrade` must be built-in (`IsBuiltinRole`, existing); expiry ≤ cap → `CodeInvalidArgument`; `label` treated as untrusted in logs. |
| V6 Cryptography | yes | Reuse `GenerateAPIKeySecret`/argon2id — **never hand-roll**. CSRF token via `crypto/rand`. |
| V7 Errors & Logging | yes | Uniform `CodeNotFound` (enumeration-hardening); structured audit line on create/rotate/revoke/resync; never log token material or PII from `label`. |
| V11 Business Logic | yes | Per-identity rate limit on mint/rotate (argon2 DoS); per-user active-key quota (sprawl). |
| V13 API / Web Service | yes | POST-only mutations; ConnectRPC JSON content-type preflight; explicit CSRF token (D-09). |

### Known Threat Patterns for {Go / ConnectRPC / Cedar / pgx / SvelteKit credential-minting}

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Role-cap laundering (downgraded apikey mints owner-role key) | Elevation of Privilege | `RoleMin(requestedOrInherit, caller.EffectiveRole)` floor on **create AND rotate** |
| Key-chaining (leaked non-admin key mints successors to outlive revocation) | Elevation / Persistence | Reject `Source=="apikey"` on mint/rotate (`CodePermissionDenied`) |
| Rotate snapshot-escalation (demoted user rotates stale-ceiling key) | Elevation of Privilege | Re-floor `role_downgrade` + re-cap `expires_at` on rotate (explicit args, no inherit-on-nil) |
| Cross-user key enumeration / leak | Information Disclosure | Owner-scoped WHERE → uniform `NotFound`; `ListMyAPIKeys` hard-sets `UserID` from context |
| Quota TOCTOU (concurrent mints exceed quota) | Tampering / DoS | Explicit `AuthStore` tx + `SELECT 1 FROM users … FOR UPDATE` |
| CSRF on cookie-auth'd mint/revoke/rotate | Tampering | POST-only + `SameSite=Lax` + JSON preflight + explicit CSRF token (D-09) |
| argon2id compute DoS via mint/rotate spam | Denial of Service | Per-identity `golang.org/x/time/rate` limiter |
| Secret leakage into shell history / disk | Information Disclosure | Emit-once; instruction steers to secret manager; SpecGraph writes no credential file |
| Stale privilege on long-lived key after IdP demotion | Elevation (latency) | Mandatory expiry + tight 180d max cap (D-08) + AUTH-02 forced re-sync closes the operator gap |
| Bootstrap-admin over-privilege (borrowed key) | Elevation of Privilege | Self-service removes the need to borrow the bootstrap key at all |
| Missing `self` in `knownVerbs` → engine boot failure | Denial of Service (availability) | Add to `knownVerbs` same commit as policy; startup + drift tests |

## Sources

### Primary (HIGH confidence)
- `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` (rev 5) — canonical AUTH-03 design + §7 touch-surface. `[CITED]`
- `docs/superpowers/specs/2026-06-04-spgr-rjrt-9-role-downgrade-failclosed-design.md` — fail-closed `clampedRole` the floor builds on. `[CITED]`
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-design.md` — login-sync foundation, "Behavior across credential types", H2 manual-role-overwrite convergence behavior AUTH-02 reasons about. `[CITED]`
- `.planning/intel/decisions.md:48-57` (ADR-004 + AuthStore caveat), `.planning/intel/constraints.md:338` (CLI-login AuthStore tx). `[VERIFIED]`
- Source read this session: `internal/auth/{identitystore.go,auth.go,engine.go,actions.go,actions_test.go,policies/base.cedar}`, `internal/server/{identity_handler.go,auth_handler.go}`, `internal/storage/{users.go,postgres/users.go,postgres/auth_migrations/001_initial.sql}`, `cmd/specgraph/{auth_apikey.go,auth_user.go,client.go,login.go}`, `internal/config/global.go`, `proto/specgraph/v1/identity.proto`, `web/src/{routes,lib}`, `Taskfile.yml`, `web/package.json`. `[VERIFIED: codebase]`

### Secondary (MEDIUM confidence)
- AGENTS.md project conventions (Taskfile, ConnectRPC, pgx v5, committed `gen/`, license/DCO, jj-colocated). `[CITED]`
- `.claude/skills/buf-regen` — protobuf regeneration flow. `[CITED]`

### Tertiary (LOW confidence)
- None — no external web research needed (no new dependencies; domain is fully internal and pre-designed).

## Project Constraints (from AGENTS.md)

- **Taskfile is the automation surface.** Use `task proto` (regenerate `gen/`, incremental), `task build`, `task check` (before push; also pre-push hook), `task pr-prep` (before PR; needs Docker). Do NOT hand-edit `gen/` (Claude Code PreToolUse hook blocks it; edit `.proto` sources).
- **ConnectRPC, not plain gRPC** — handlers in `internal/server/`; proto generates `.pb.go` + `.connect.go`.
- **pgx v5 native driver** — pgxpool, not `database/sql`. `AuthStore` uses `s.pool` directly (NOT `*Store.RunInTransaction`, ADR-004 caveat).
- **Proto field changes:** use `reserved` for removed fields; run `task proto`; update all callers; `go build ./...`.
- **License headers required** on all new `.go`/`.sh`/`.proto` — `SPDX-License-Identifier: Apache-2.0` (run `task license:add`).
- **DCO required** — every commit needs `Signed-off-by:` (`git commit -s` or `jj describe` trailer).
- **`revive` package comments** — new Go packages need `// Package foo …`.
- **jj-colocated git** — use `jj --no-pager`; never `git push` (use `jj bookmark set` + `jj git push`); use `jj workspace add`, not `git worktree`.
- **Postgres integration tests need Docker** (testcontainers, `pgvector/pgvector:pg18`).
- **Handler error sanitization** — return error *codes* (`connect.CodeInternal`, `CodeNotFound`, `CodePermissionDenied`), not message strings; tests assert on codes.
- **Mock/fake backends must return sentinel errors** (`storage.ErrAPIKeyNotFound`, `storage.ErrUserNotFound`) where handler uses `errors.Is`.
- **No new goose migration this phase** (existing `api_keys`/`users` columns suffice; D-03 no provenance column).

## Code Drift Ledger

> Design/CONTEXT cited line numbers vs. actual (read this session). Overall drift is minor; all cited symbols exist.

| Symbol | Cited | Actual | Note |
|--------|-------|--------|------|
| `resolveAPIKey` | `:306` / clamp `:346` | `:306` / clamp `:346` | ✓ exact |
| `clampedRole` | `:289` | `:289` | ✓ exact |
| `roleRank` / `roleLessThan` | `:256` / `:270` | `:256` / `:270` | ✓ exact |
| rate limiter `rateLimiterFor` | `:643` | `:643` | ✓ exact |
| `knownVerbs` | `engine.go:155` | `:155` | ✓ exact |
| verb-group gating | `engine.go:183` | present (`actionVerb` `:167`, groups built in `buildActionEntities`) | ✓ pattern confirmed |
| `principalEntity` role exposure | `:218`/`:219` | `:212` | **drift ~6 lines**; exposes `role`/`id`/`email` as design states |
| `UpdateUserRole` handler | `:155` | `:155` | ✓ exact |
| `RevokeAPIKey`/`RotateAPIKey` storage | `:408`/`:425` | `:408`/`:425` | ✓ exact |
| `RowsAffected()==0 → NotFound` | `postgres/users.go:233` | `:233,:249` | ✓ (two sites) |
| `CreateAPIKey` storage | (impl) | `:361` | INSERT…SELECT…WHERE EXISTS |
| `client.go` env-before-session | `:102` | `:102-111` (`resolveAPIKey`) | ✓ exact |
| `auth_user.go` set-role | `:80` | `:80` | ✓ exact |
| `Identity` struct | `auth.go:19` (`Role`/`EffectiveRole`/`Source`) | `:19-27` | ✓ exact; `Source` only `"apikey"\|"oidc"` (`:26`) |
| **`actions_test.go` verb assertion** | design named parse test + mirror | `:40` hard-codes `[read,write,delete,manage]`; `:50` mirror | **⚠️ TWO lists to update — surfaced as Pitfall 1** |
| `api_keys_active` index | `001_initial.sql:49` | `:49` (`WHERE revoked_at IS NULL`) | ✓ exact; no expiry coverage |

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all deps verified in `go.mod`/`package.json`; no new packages.
- Architecture: HIGH — AUTH-03 canonical rev-5 design + verified touch-surface; AUTH-02 grounded on `resolveAPIKey` live-role read.
- Pitfalls: HIGH — derived from reading the actual tests/handlers/storage; the `actions_test.go` second-list pitfall is a fresh, verified finding.
- Security: HIGH — threat vectors enumerated from the design's adversarial review + confirmed code gates.

**Research date:** 2026-07-09
**Valid until:** ~2026-08-08 (30 days; stable brownfield subsystem). Re-verify line numbers with a quick grep if AUTH-03 in-progress work lands on `main` before planning (see STATE.md Blocker: `spgr-g7st` was already in progress — **check open PRs / current `main` before planning to avoid re-doing work**).
