# Phase 2: API Key Lifecycle & Self-Service - Context

**Gathered:** 2026-07-09
**Status:** Ready for planning

<domain>
## Phase Boundary

Two identity-hardening requirements on the already-shipped Identity/Authn/Cedar/login-sync foundation:

- **AUTH-03 (`spgr-g7st`):** an authenticated OIDC user can self-provision — create, list, rotate, revoke — their own role-capped, expiring MCP API key, without borrowing the bootstrap admin key. The minted key's effective role is capped at the caller's own current effective role at mint/rotate time (no privilege-escalation "laundering").
- **AUTH-02 (`spgr-c2lb`):** when a user's app-role is revoked or downgraded upstream, an operator can force a re-sync that lands the new (lower) role on the user's **standing** API/MCP keys immediately — not only on the user's next interactive OIDC login — with an option to hard-revoke their keys for full off-boarding.

This is HOW-to-implement clarification on fixed roadmap scope. New identity capabilities (native GitHub OAuth2, MCP OAuth 2.1 resource server, session-issuer audit) belong to Phase 3; automated/scheduled re-sync and IdP-role-fetch are explicitly deferred (see Deferred Ideas).

**Maturity split found during scouting (important for downstream):**
- **AUTH-03 is heavily pre-designed.** `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` is a rev-5 design that absorbed four adversarial review rounds. Treat it as the **canonical/near-locked spec** — its decisions flow through without re-litigation. This discussion resolved only the two decisions the design itself deferred "to implementation review," plus the surface scope.
- **AUTH-02 has NO design doc.** Both sibling designs name `spgr-c2lb` as "the full fix" and defer it. The decisions below are the substantive design for it.

</domain>

<decisions>
## Implementation Decisions

### AUTH-02 — Forced role re-sync / off-board (spgr-c2lb; net-new design)

- **D-01 (trigger = Hybrid):** The forced re-sync is **operator-driven** this phase — SpecGraph does NOT call the IdP. But the server-side entrypoint MUST be built as a single reusable seam (RPC/service method) that a future automated or IdP-fetch driver can call without rework. Ship the operator path; leave the automation seam ready.
  - Grounding: `resolveAPIKey` (`internal/auth/identitystore.go:346`) already computes `EffectiveRole = clampedRole(user.Role, key.RoleDowngrade)` from the **live DB role every request**. So the instant `users.role` is updated in the DB, every standing key for that user reflects the new role on its next call. The only real gap is a non-interactive path to write that DB role.
- **D-02 (effect = re-derive role by default, revoke on demand):** A forced re-sync **re-derives / writes the user's DB role** by default (covers downgrade → all standing keys immediately clamp to the lower role, keys keep working at reduced privilege). A **separate explicit `--revoke-keys` flag/command** hard-revokes the user's standing keys for full off-boarding (forces re-mint). Two distinct operator intents, kept separate.
- **D-03 (durability = convergence, NOT provenance):** Do **NOT** add a `role_source` provenance column. Accept that a forced demotion sticks until the user's next interactive OIDC login re-derives from claims — at which point, if the app-role was truly revoked upstream, `claims_mapping` (rules 2/3) yields the same lower role, so it **converges**. No schema change. (Trade-off acknowledged: a login through a *mapping-less* provider leaves the manual lower role intact via rule 1 = freeze, which is fine; a login where the upstream role was NOT actually revoked would re-promote — acceptable, since that means the operator over-reached.)
- **D-04 (command surface):** New subcommand under the existing `specgraph auth user` group — e.g. `auth user resync <user-id> --role <r>` to force the demotion, plus `--revoke-keys` for the hard off-board. Backed by **one server-side RPC/entrypoint** that the CLI calls now and future automation can reuse (the D-01 seam), reusing existing `UpdateUserRole` + `RevokeAPIKey` plumbing (`internal/server/identity_handler.go:155`, `cmd/specgraph/auth_user.go:80`). Exact subcommand/flag naming is planner's discretion.
- **D-05 (scope = bounded slice):** AUTH-02 delivers exactly the operator command + reusable RPC + `--revoke-keys`. **NO** scheduled/background job, **NO** IdP polling, **NO** provenance column this phase. Roadmap SC#3 is met (revocation reaches standing keys on forced re-sync, not only next login). Automation/IdP-fetch is a deferred idea; the hybrid seam (D-01) is what makes picking it up later cheap.

### AUTH-03 — Self-service MCP API keys (spgr-g7st; follows the rev-5 design)

- **D-06 (design is canonical):** Implement per `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` — all of its locked decisions carry forward and MUST be honored, including: new Cedar verb `apikey.self` (+ `"self"` added to `knownVerbs` in `engine.go:155` or the server won't boot; + the `self`-verb-only-on-`apikey.*` drift test); four new `IdentityService` RPCs (`CreateMyAPIKey`/`ListMyAPIKeys`/`RotateMyAPIKey`/`RevokeMyAPIKey`); owner derived strictly from `auth.IdentityFromContext` (no caller-supplied target); owner-scoped storage mutations with `AND user_id = $caller` WHERE clauses (`RowsAffected()==0 → CodeNotFound`); the `role_downgrade` laundering fix (**floor at caller `EffectiveRole` on BOTH create and rotate** via a new **exported** `auth.RoleMin`); the `Source=="apikey"` anti-key-chaining rejection; CLI session-credential auth precedence (Finding D); `ListMyAPIKeys` hard-setting `UserID` from context; the quota TOCTOU handling (explicit `AuthStore` tx + parent-`users`-row `FOR UPDATE`, since `AuthStore` is NOT wired into `*Store.RunInTransaction`); per-identity rate-limit on self-mint/rotate; emit-once delivery. **These are NOT re-decided here** — see the design doc.
- **D-07 (surfaces = BOTH CLI + web):** Ship both surfaces this phase. CLI: `auth api-key create/list/rotate/revoke` self-variants (no `--user-id` → self path; `--user-id <other>` keeps the admin path). Web: the new **"MCP Keys" dashboard panel** (one-time reveal modal, list/create/revoke/rotate, `specgraph_session`-cookie auth) — net-new SvelteKit work, since the dashboard is currently read-only. Both surfaces consume the same four RPCs.
- **D-08 (expiry caps = 90d default / 180d max):** Resolve the design's first open question by **lowering the max cap from 365d to 180d** (default stays 90d), because a demoted user's key keeps the stale higher role until interactive re-login, bounded only by expiry + the mint-time ceiling — a tighter max shrinks that stale-privilege window. All values remain server-configurable. Per-user active-key quota stays at the design default (10, server-configurable).
- **D-09 (web CSRF = explicit CSRF token):** Resolve the design's second open question by adding an **explicit CSRF token** (synchronizer or double-submit) on the POST-only self-mint/revoke/rotate mutations, on top of the existing `SameSite=Lax` cookie + JSON-content-type preflight. Chosen over `SameSite=Strict` (dashboard-wide UX side effects) and over relying on the implicit mitigation alone, because these mutations mint credentials.

### Discretion (planner/researcher)
- Exact CLI subcommand + flag names for the AUTH-02 resync/off-board command (D-04).
- Precise RPC/service-method shape of the reusable re-sync seam (D-01/D-04) — must be callable by both CLI-now and future automation.
- CSRF token mechanism specifics (synchronizer vs double-submit) and issuance/validation wiring (D-09).
- Any implementation-review-level details the g7st design flags but doesn't fully pin (e.g. rate-limit thresholds, audit-log line shape) — follow the design's guidance.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### AUTH-03 — self-service API keys (the governing spec)
- `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md` — **canonical/near-locked design (rev 5).** Full authorization model, four RPCs, the escalation/laundering fix, anti-key-chaining gate, quota/expiry/TOCTOU handling, delivery, web mutation safety, and the §7 implementation touch-surface. MUST read before planning/implementing AUTH-03.
- `docs/superpowers/specs/2026-06-04-spgr-rjrt-9-role-downgrade-failclosed-design.md` — the fail-closed `clampedRole`/downgrade-validation behavior the self-mint floor builds on.

### AUTH-02 — role revocation on standing keys (context; no dedicated design doc)
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-design.md` — app-roles + `sync_on_login` behavior, the "Behavior across credential types (MCP / API keys)" section (standing keys resolve live DB role per request; revocation not enforced until re-login = the exact gap AUTH-02 closes), and the H2 login-sync-overwrites-manual-role behavior that D-03 (convergence) reasons about. MUST read before implementing AUTH-02.
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-implementation-plan.md` — companion implementation plan for the login-sync foundation this phase extends.

### Architectural constraints (project intel)
- `.planning/intel/decisions.md`, `.planning/intel/constraints.md` — locked ADRs and the Identity/RBAC/Audit epic lineage (Identity Storage → Authn → Bootstrap/UX → Cedar policy engine). Note ADR-004 caveat: `AuthStore` is NOT reachable by `*Store.RunInTransaction` — API-key writes use an explicit `AuthStore` transaction.

### Key existing code (grounding)
- `internal/auth/identitystore.go` — `resolveAPIKey` (`:306`, reads live DB role; `:346` clamp), `clampedRole` (`:289`), JIT/`resolveSession`, rate limiter (`:643`).
- `internal/auth/loginsync.go` — `applyLoginSync` (`:67`), `resolveLoginRole`, `isPromotion` — the interactive-login role derivation AUTH-02 complements.
- `internal/auth/engine.go` — Cedar engine, `knownVerbs` (`:155`), verb-group gating (`:183`), `principalEntity` role exposure (`:218`).
- `internal/auth/actions.go` + `actions_test.go` — procedure→action map (hard-mirrored test).
- `internal/auth/auth.go` — `Identity` struct (`:19`; `Role` vs `EffectiveRole` vs `Source`).
- `internal/server/identity_handler.go` — `UpdateUserRole` (`:155`), `ListAPIKeys`/`RevokeAPIKey`/`RotateAPIKey` handlers; where the four self RPCs + AUTH-02 resync RPC land.
- `internal/storage/users.go` (interface) + `internal/storage/postgres/users.go` (impl) — `RevokeAPIKey`/`RotateAPIKey` (`:408`,`:425`); add owner-scoped `GetAPIKeyForUser`/`RevokeAPIKeyForUser`/`RotateAPIKeyForUser`.
- `internal/storage/postgres/auth.go` — `AuthStore` (separate tx path from `*Store.RunInTransaction`).
- `cmd/specgraph/auth_user.go` (`:80` set-role), `cmd/specgraph/auth_apikey.go`, `cmd/specgraph/login.go`, `cmd/specgraph/client.go` (`:102` `SPECGRAPH_API_KEY`-before-session precedence — Finding D).
- `proto/specgraph/v1/identity.proto` — four new self RPCs + the AUTH-02 resync RPC (`task proto` regenerate).
- `web/src/routes/` — dashboard routes (currently read-only: constitution/decision/graph/spec); new "MCP Keys" panel + `web/src/lib/` client wiring.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`resolveAPIKey` already reads the live DB role every request** — no cache to invalidate; AUTH-02's "make revocation reach standing keys" reduces to "write the DB role," which `UpdateUserRole` already does. The net-new AUTH-02 work is the operator command + reusable RPC + optional key-revoke, not a new propagation mechanism.
- `UpdateUserRole` RPC + `auth user set-role` CLI (`auth_user.go:80`) and `RevokeAPIKey` — the plumbing the AUTH-02 resync/off-board command reuses.
- `clampedRole` / `roleRank` / `roleLessThan` (unexported in `internal/auth`) — the ordering logic the new **exported** `auth.RoleMin` (g7st §7, Finding F3) generalizes for the self-mint floor.
- The JIT rate-limiter pattern (`identitystore.go:643`) — reused for the self-mint/rotate per-identity rate limit (g7st §5 Finding K).
- Existing admin API-key RPCs (`CreateAPIKey`/`ListAPIKeys`/`RotateAPIKey`/`RevokeAPIKey` → `apikey.manage`) — **unchanged**; the self RPCs are additive siblings.
- `specgraph_session` cookie + `whoami` auth path — the web MCP Keys panel reuses it (`auth_handler.go`).

### Established Patterns
- Cedar authz gates by verb **suffix** via action groups (`engine.go:183`) — the `apikey.self` permit applies to any future `*.self`; the design mandates a drift test restricting `self` to `apikey.*`.
- Owner-scoped mutations return `CodeNotFound` uniformly (`postgres/users.go:233` pattern) so "not yours" and "doesn't exist" are indistinguishable (enumeration-hardening).
- `AuthStore` writes use their own explicit transaction (NOT `*Store.RunInTransaction`, ADR-004) — the quota-safe mint locks the parent `users` row `FOR UPDATE`.
- No storage-schema change for AUTH-03 (existing `api_keys` columns cover everything); no schema change for AUTH-02 either (D-03 convergence, no provenance column). **This phase adds no goose migration.**

### Integration Points
- New `IdentityService` RPCs in `proto/specgraph/v1/identity.proto` → `task proto` → `internal/auth/actions.go` map + `actions_test.go` mirror + Cedar `base.cedar` policy + `knownVerbs`.
- CLI: `cmd/specgraph/` self-mint wiring + session-credential auth precedence fix (Finding D); AUTH-02 resync under `auth user`.
- Web: new panel in `web/src/routes/` + client in `web/src/lib/` + CSRF-token issuance/validation on the mutations.
- Compile-time interface stubs (`usersbackend_stub_test.go` in both `internal/auth` and `internal/server`) must gain the new owner-scoped methods.

</code_context>

<specifics>
## Specific Ideas

- Follow the g7st rev-5 design's §7 "implementation touch-surface" as the AUTH-03 work-breakdown checklist — it is deliberately exhaustive (it warns "don't under-state it").
- AUTH-02's command should read as an operator "force this user's role down now (and optionally kill their keys)" action — the value proposition is *immediacy on standing keys without waiting for the user to log in*.

</specifics>

<deferred>
## Deferred Ideas

- **Automated / scheduled AUTH-02 re-sync** — a background job that periodically re-applies roles. Deferred; the D-01 hybrid RPC seam is built so this is a cheap follow-up.
- **IdP-proactive role fetch** (query the IdP's app-role assignments directly, no interactive login) — deferred; overlaps `spgr-tmqm` (MCP OAuth 2.1 resource server / IdP-as-authority) territory and adds egress/credentials/throttling.
- **Role provenance column (`role_source`)** to make operator/manual grants durable against login-sync overwrite — considered and dropped this phase (D-03 convergence); revisit if non-deterministic manual-grant durability across providers becomes a real operator pain point.
- **Harness-config rewrite** (drop `${SPECGRAPH_API_KEY}` env indirection; file-native/OAuth credential delivery) — explicitly out of scope per the g7st design; rides with `spgr-tmqm`.
- **MCP OAuth 2.1 resource-server behavior** (401 + `WWW-Authenticate`, RFC 9728/8707, IdP-issued short-lived audience-bound tokens) — `spgr-tmqm`, a later phase.

None of the above are needed to satisfy Phase 2's three roadmap success criteria.

</deferred>

---

*Phase: 2-API Key Lifecycle & Self-Service*
*Context gathered: 2026-07-09*
