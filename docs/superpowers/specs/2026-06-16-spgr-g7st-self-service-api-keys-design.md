# Self-Service MCP API-Key Provisioning for OIDC Users

- **Issue:** spgr-g7st (external: gh-996)
- **Status:** pending spec review (rev 5 — incorporates four adversarial review rounds)
- **Date:** 2026-06-16
- **Related:** spgr-tmqm (MCP OAuth resource server — strategic follow-on), spgr-1rq9 (generic OAuth2 provider), spgr-c2lb (role-revocation latency), gh #996 (OIDC app-roles + login-sync)

## Problem

Today API keys are admin-only. `CreateAPIKey` is gated by the Cedar `apikey.manage`
verb, which `base.cedar` permits only for `principal.role == "admin"`. A regular
reader/writer user — including someone who only ever logs into the web dashboard via
OIDC — cannot mint an MCP credential for themselves. The browser dashboard is read-only
with no key/profile UI.

The harness MCP configs that `specgraph init` writes reference
`Bearer ${SPECGRAPH_API_KEY}` (Claude `.mcp.json`, Cursor `.cursor/mcp.json`, OpenCode
`opencode.json`), but nothing populates that variable. The documented onboarding flow
has the user extract the **bootstrap admin key** from the credentials file and export it:

```bash
export SPECGRAPH_API_KEY=$(yq '.api_keys[] | select(.id=="default-admin") | .key' \
  ~/.config/specgraph/credentials.yaml)
```

This pushes every MCP client to run as the **bootstrap admin** — wildly over-privileged.
The OIDC app-roles work (gh #996) makes role *assignment* correct, but the
key-provisioning gap undercuts it in practice: a correctly-scoped reader still has to
borrow the admin key to get a working MCP credential.

## Goal / Acceptance Criterion

A non-admin OIDC user can obtain a working, role-appropriate MCP credential without an
admin minting it for them and without using the bootstrap admin key.

## Strategic Context: Where This Sits

During brainstorming we separated two efforts:

1. **This spec (spgr-g7st):** self-service, role-capped, expiring, owner-managed **API
   keys** — the credential SpecGraph itself issues and owns. This is the permanent
   **fallback** credential (CI, service accounts, OAuth-incapable harnesses, and the
   generic OAuth2 / GitHub-direct providers of spgr-1rq9). It satisfies the *mechanism*
   of the acceptance criterion immediately (a non-admin mints their own key). The
   *value* of that key being "role-appropriate" depends on the user's role being correct,
   which in turn depends on gh #996 app-roles / claims-mapping (see the honesty note
   below). Absent that, a JIT user is `reader` by default and the minted key is
   reader-capped regardless of IdP groups — still a strict improvement over borrowing the
   bootstrap admin key.

2. **spgr-tmqm (strategic north-star, separate spec):** make SpecGraph an
   MCP-spec (2025-06-18) **OAuth 2.1 resource server** that delegates authentication to a
   real IdP. The IdP becomes the authorization server and the system of record for
   identity + roles; harnesses receive short-lived, audience-bound, role-claimed tokens.
   SpecGraph explicitly does **not** become an authorization server (no `/authorize`,
   `/token`, or user-token signing) — that would entrench the very thing we want to shed.

These are complementary, not either/or. Mature systems keep self-service API keys
alongside OAuth (GitHub has PATs next to GitHub Apps).

> **Forward-compat honesty note.** The claim "self-service keys cap against an
> *IdP-projected* role" is only true *when claims-mapping + login-sync are configured*
> for the issuer. Today the role is **SpecGraph-stored**: JIT sets it from
> `claims_mapping` or falls back to `reader` (`identitystore.go:532`), login-sync
> refreshes it only if `loginSyncEnabled` **and** the issuer has a mapping
> (`loginsync.go:28`), and `UpdateUserRole` (admin) sets it locally with no IdP linkage.
> In a deployment **without** configured claims-mapping, the role is
> SpecGraph-authoritative and these keys partially *entrench* SpecGraph-as-authority —
> the opposite of the spgr-tmqm direction. This spec is forward-compatible **conditional
> on** claims-mapping being configured; it does not, by itself, move identity ownership
> to the IdP. That migration is spgr-tmqm + gh #996.

### Best-practice grounding

- **OWASP Secrets Management** and **GitHub PAT model**: minimize long-lived bearer
  secrets, prefer short-lived tokens (the spgr-tmqm direction); when static credentials
  are necessary, they must be least-privilege, expiring, rotatable, revocable, and
  delivered once.
- **GitHub's defining rule**: a token cannot grant capabilities beyond its owner and
  becomes inactive if the owner loses access. SpecGraph already enforces this via
  `clampedRole` (fail-closed). Self-service must derive the owner strictly from the
  authenticated identity, never from a client-supplied field.
- **Expiration**: GitHub recommends it and auto-revokes unused tokens; OWASP §2.7.4 says
  secrets should expire. Self-service keys make expiry mandatory.

## Design

### 1. Authorization model

Self-scoping is expressed at the **procedure level**, matching the existing Cedar model
(each RPC procedure maps 1:1 to a `domain.verb` action).

- **New Cedar verb `apikey.self`** in `internal/auth/policies/base.cedar`, permitted for
  **any authenticated principal** (reader, writer, or admin):

  ```cedar
  permit (principal, action in SpecGraph::Action::"self", resource) when {
    principal has role &&
    (principal.role == "reader" || principal.role == "writer" || principal.role == "admin")
  };
  ```

  `self` is its own verb — it does **not** inherit `read`/`write`/`delete`/`manage`,
  mirroring how `manage` is deliberately isolated.

  > **Required, security-sensitive plumbing (do not omit).** Adding the `self` verb is
  > NOT a clean drop-in. The string `"self"` MUST be added to `knownVerbs` in
  > `internal/auth/engine.go:155`; without it `actionVerb` errors, `buildActionEntities`
  > fails, `NewCedarEngine` returns an error, and **the server refuses to start**, and
  > `TestActionNames_AllParseToKnownVerb` (`actions_test.go`) fails CI. The
  > procedure→action mirror in `actions_test.go` must also gain the four new RPCs.
  >
  > **Verb-group footgun.** Cedar gates by verb *suffix* via action groups
  > (`engine.go:183`), so the permit applies to *every* `*.self` action. Today only
  > `apikey.self` exists. To prevent a future `user.self`/`spec.self` from silently
  > inheriting the all-principals permit, add a drift test asserting that the `self` verb
  > is used **only** by `apikey.*` procedures. (Scoping the policy to an "apikey resource
  > type" is **not** an option today: all resources are a single `SpecGraph::Resource`
  > entity with a placeholder id — per-resource-type modeling is the deferred seam in
  > `engine.go:228`, owned by spgr-tmqm. The drift test is the only real mitigation.)

- **New `IdentityService` RPCs** (`proto/specgraph/v1/identity.proto`), each mapped to
  `apikey.self` in `internal/auth/actions.go`:
  - `CreateMyAPIKey`
  - `ListMyAPIKeys`
  - `RotateMyAPIKey`
  - `RevokeMyAPIKey`

- **Handlers** (`internal/server/identity_handler.go`) derive `user_id` strictly from
  `auth.IdentityFromContext(ctx)` (the `Whoami` pattern). There is **no** caller-supplied
  target field on these requests. `ListMyAPIKeys` MUST hard-set the storage filter's
  `UserID` from context — note `ListAPIKeys` with an **empty** `UserID` returns *all*
  users' keys (`identity_handler.go:319`), so this is a load-bearing invariant that needs
  an explicit test.

- **Ownership enforcement is done in the storage WHERE clause, not a generic pre-read.**
  The existing `RevokeAPIKey`/`RotateAPIKey` mutate by `keyID` without an owner check
  (`users.go:408`, `:425`). Add **owner-scoped** storage mutations
  (`RevokeAPIKeyForUser(userID, keyID)` / `RotateAPIKeyForUser(userID, keyID, roleDowngrade,
  expiresAt)`) whose WHERE clause includes `AND user_id = $userID`; `RowsAffected() == 0 →
  CodeNotFound` (the established pattern at `postgres/users.go:233`). So "not yours" and
  "doesn't exist" are indistinguishable (deliberate enumeration-hardening). Idempotency
  differs by op (see lifecycle note): **revoke** omits the `revoked_at IS NULL` guard →
  re-revoking your own key is a no-op success; **rotate** keeps `revoked_at IS NULL` →
  rotating a revoked/foreign/missing key is uniformly `NotFound`.
  - **Rotate also needs the old `role_downgrade`** to re-floor it (Finding 1, round 4).
    Add a small **owner-scoped** read `GetAPIKeyForUser(userID, keyID)` (same `AND
    user_id` guard) used only by the rotate handler to fetch the current downgrade; it is
    race-benign (`role_downgrade` is immutable; a concurrent revoke just yields the same
    `NotFound` on the subsequent mutate). This is a narrow, owner-scoped lookup — **not**
    the general `GetAPIKeyByID` we declined — so it does not widen the enumeration surface.

- **CRITICAL (escalation) — close the `role_downgrade`-laundering hole.**
  `clampedRole` clamps a new key against the **owner's** `Role`, not the **caller's**
  `EffectiveRole`. The two diverge **only** for an `apikey`-source caller presenting a
  *downgraded* key: `Identity{Role: admin, EffectiveRole: reader}` passes the `apikey.self`
  gate as `reader` (Cedar gates on `EffectiveRole`, `engine.go:219`), then an inherit-mint
  would resolve to the owner's **admin** role — laundering the cap. The fix is a single,
  source-agnostic invariant, implementable with the **existing** `Identity` fields:

  > **The minted key's `role_downgrade` is floored at the caller's `EffectiveRole`:**
  > `role_downgrade = roleMin(requestedDowngradeOrInherit, caller.EffectiveRole)`, where
  > an omitted (inherit) request resolves to `caller.EffectiveRole`. Therefore **every
  > self-minted key (on BOTH create AND rotate) carries a non-empty `role_downgrade` ≤ the
  > caller's effective role at that moment.** A `reader`-effective caller can only mint a
  > `reader`-ceiling key.
  >
  > **`roleMin` does not exist yet (Finding F3).** `roleRank`/`roleLessThan`/`clampedRole`
  > are all *unexported* in `internal/auth` (`identitystore.go:256,270,289`), and the floor
  > runs in the server package. Add a new **exported** helper (e.g. `auth.RoleMin(a, b Role)
  > Role`, fail-closed for unranked) — enumerated in §7.

  This supersedes the rev-2 "interactive-source gate," which was **unimplementable** —
  `Identity.Source` is only `"apikey"|"oidc"` (`auth.go:26`) and `resolveSession` returns
  `"oidc"` identically to a raw bearer JWT, so no allowlist can distinguish an interactive
  session from a CI/workload token.

- **Caller-credential gate (anti key-chaining, Finding F5).** Independent of the escalation
  fix above, **reject `CreateMyAPIKey` / `RotateMyAPIKey` when the caller's `Source ==
  "apikey"`** (`CodePermissionDenied`). This is implementable (the `"apikey"` value *does*
  exist; only the session-vs-JWT distinction does not) and stops a **leaked non-admin API
  key from self-perpetuating** — minting fresh keys to outlive revocation of the stolen
  original (a new lateral-persistence vector this feature would otherwise open: today a
  non-admin key can mint nothing). Note the honest limit: a raw **OIDC JWT** (e.g. CI /
  workload identity, `Source == "oidc"`) can still self-mint; that is acceptable because
  the `EffectiveRole` floor caps it and such tokens are themselves short-lived.
  **Service accounts are fully covered by this gate** (they authenticate only via API
  keys → `Source == "apikey"`), so a separate `kind == service_account` check is dropped
  as redundant — it would otherwise force an extra `GetUserByID` (the `Identity` struct
  carries no `Kind`, `auth.go:19`).

- **CLI auth precedence caveat (Finding D — now load-bearing).** `client.go` resolves
  `SPECGRAPH_API_KEY` **before** the stored session (`client.go:102`), so on a box that
  followed the documented onboarding (which exports the *bootstrap admin key* into that
  var), `specgraph auth api-key create` would authenticate as that **API key**. With the
  F5 `Source=="apikey"` gate, that call now **hard-fails** `PermissionDenied` (not just
  mis-scopes) — so the self-mint command MUST authenticate with the stored OIDC **session**
  credential explicitly (ignoring / warning on a set `SPECGRAPH_API_KEY`). F5 makes this
  fix mandatory, not advisory.

- The existing admin RPCs (`CreateAPIKey`, `ListAPIKeys`, `RotateAPIKey`,
  `RevokeAPIKey` → `apikey.manage`) are **unchanged**.

Resource-ownership Cedar rules (the documented future seam in
`internal/auth/cedar_authorizer.go`) are intentionally **not** introduced here; they are
designed properly, across all RPCs, in spgr-tmqm. Dedicated self-scoped procedures give
us the guarantee we need now without that larger change.

### 2. Key lifecycle & policy

- **Mandatory expiry** on self-service keys. Default **90 days**; a server-configurable
  **maximum cap** (default **365 days**) bounds requests. A request exceeding the cap is
  rejected with `CodeInvalidArgument`. **`RotateMyAPIKey` MUST re-apply BOTH caps**
  (Findings G + F1): `RotateAPIKey` today inherits the old key's `role_downgrade` and
  accepts an `expires_at` override (`identity_handler.go:293`, `postgres/users.go:437,483`).
  Without re-capping, a rotate (a) extends expiry past the cap and (b) **re-pins the old
  ceiling above the caller's current role** — e.g. a since-demoted admin rotates an
  admin-ceiling key into a fresh admin-ceiling key, defeating the §1 "every self-minted key
  ≤ caller EffectiveRole" invariant and re-introducing snapshot escalation. Therefore
  `RotateAPIKeyForUser` takes **explicit** `role_downgrade` and `expires_at` arguments
  (never the inherit-on-nil fallback that `RotateAPIKey` uses at `users.go:460`): the
  handler sets `role_downgrade = roleMin(oldDowngrade, caller.EffectiveRole)` (old value
  from the owner-scoped read in §1) and **always** computes a defaulted, ≤-cap `expires_at`
  (a `rotate` with no `--ttl` defaults to 90d, it does **not** inherit the old window).
  Rotate is never more permissive than a fresh `CreateMyAPIKey`. (Admin-minted keys via the
  admin path retain today's behavior.)

- **Role binding = "snapshot ceiling, live floor."** Because every self-minted key's
  `role_downgrade` is floored at the caller's `EffectiveRole` at mint time (see §1), the
  key's effective role over its life is `clampedRole(currentProjectedRole, mintTimeCeiling)`
  = `min(currentProjectedRole, mintTimeCeiling)`. Consequences, all intended:
  - **Never escalates** above the role the creator held at mint time, even if the owner is
    later **promoted** in the projection. (We explicitly **drop** the rev-2 idea that a
    "non-downgraded key auto-upgrades on promotion" — that is now a **non-goal**; to use a
    higher role, mint a new key after promotion. This is the safer, GitHub-PAT-like posture
    and removes the laundering class entirely.)
  - **Still drops** if the owner is **demoted** in the projection
    (`min(reader, writer) = reader`).
  - **Staleness caveat (spgr-c2lb).** "Projection" is refreshed from the IdP **only on
    interactive login-sync** — an API key never triggers sync (`identitystore.go:469`,
    `loginsync.go:67`). A user demoted in the IdP who never interactively logs in again
    keeps the stale higher role (bounded by `mintTimeCeiling` and by expiry). This is why
    the expiry cap exists and should stay relatively tight; the full fix is spgr-c2lb.

- **Per-user active-key quota.** A server-configurable cap (default **10**) on **active
  (non-revoked, non-expired) keys per user — any origin.** The schema has **no column
  distinguishing self-service from admin-minted keys** (and `label` is user-controlled, so
  it can't be a security boundary), so the quota counts *all* of a user's active keys.
  The existing `api_keys_active ON api_keys(user_id) WHERE revoked_at IS NULL` index
  (`001_initial.sql:49`) covers revocation but **not** expiry, so the count query adds
  `AND (expires_at IS NULL OR expires_at > now())` (un-indexed, fine at this cardinality;
  worst case over-counts expired keys → fails *safe*). Because the quota is enforced only
  on the self path, "≤10 active keys/user" is a self-mint budget, not a system invariant.
  **Concurrency (Finding F2 — the rev-3 guard was wrong).** `SELECT count(*) … FOR UPDATE`
  is invalid in Postgres, and row-locks don't stop a phantom INSERT at READ COMMITTED
  (`tx.go:61`), so two concurrent mints both pass. Also, API-key writes live on `AuthStore`
  (`postgres/auth.go`), which is **not** wired into `*Store.RunInTransaction` — so the ADR-004
  citation doesn't apply directly. The mint MUST instead: open an explicit `AuthStore`
  transaction (the pattern `RotateAPIKey` already uses, `users.go:426`), **lock the parent
  user row** (`SELECT 1 FROM users WHERE id = $caller FOR UPDATE`) to serialize that user's
  mints, count, then insert. Over-cap → clear error pointing at revoke/rotate.

- **Rotate/revoke idempotency semantics (deliberate, resolved per Finding F4).**
  **Revoke** uses `WHERE id=$k AND user_id=$caller` (no `revoked_at` guard): re-revoking
  your own key is an idempotent **no-op success**; a foreign/missing id → 0 rows →
  `CodeNotFound`. **Rotate** uses `WHERE id=$k AND user_id=$caller AND revoked_at IS NULL`:
  a revoked, foreign, or missing key → 0 rows → `CodeNotFound` (you cannot rotate a dead
  key). The two ops intentionally differ; "already-revoked" is success for revoke and
  NotFound for rotate — both avoid leaking *other users'* key existence.

- **Token format unchanged.** `spgr_sk_<prefix>_<secret>`, argon2id PHC hash, plaintext
  returned exactly once. **No storage-schema change** — the existing `api_keys` columns
  (`user_id`, `prefix`, `phc_hash`, `role_downgrade`, `label`, `expires_at`,
  `last_used_at`, `revoked_at`, `created_at`) already cover everything.

### 3. Surfaces

Both surfaces consume the same four RPCs.

**CLI** (`cmd/specgraph/`):

- `specgraph auth api-key create` with **no** `--user-id` self-mints via `CreateMyAPIKey`,
  authenticating with the session token that `specgraph login` already stores (resolvable
  as a bearer via `resolveSession`). Flags: `--label`, `--role` (self-downgrade),
  `--ttl` / `--expires` (≤ cap).
- Self variants for `list`, `revoke`, `rotate`.
- Passing `--user-id <other>` preserves the existing admin path (`CreateAPIKey` etc.).

**Web** (`web/src/routes/`):

- A new "MCP Keys" dashboard panel, authenticated via the existing `specgraph_session`
  cookie (same mechanism as `whoami`).
- Lists the caller's own keys (label, created, last-used, expiry, status).
- **Create** → one-time reveal modal with a copy button and the env-var instruction
  (below). **Revoke** and **Rotate** actions.

### 4. Delivery

The minted secret is shown **exactly once** and never written to disk by SpecGraph:

- **CLI** prints the token to stdout once, followed by an explicit instruction to set
  `${SPECGRAPH_API_KEY}` in the harness environment. The instruction steers the user
  toward a shell profile or secret manager rather than an inline `export …` (which would
  leak the secret into shell history, the anti-pattern this issue calls out). The CLI
  does **not** run the export itself and does **not** write any credential file.
- **Web** reveal modal offers copy-to-clipboard plus the same `${SPECGRAPH_API_KEY}`
  instruction.

Delivery is **harness-agnostic**: the existing `Bearer ${SPECGRAPH_API_KEY}` contract in
the init-managed configs is preserved exactly. **No `internal/config/managedfiles/manifest.go`
change.** (File-native credential delivery — e.g. OpenCode's `{file:…}` interpolation —
and the broader harness-config rewrite ride along with spgr-tmqm.)

### 5. Web mutation safety & abuse controls

- **CSRF (Finding J).** The web panel issues **cookie-authenticated state-changing**
  RPCs (create/revoke/rotate). Today this is *implicitly* mitigated by the
  `specgraph_session` cookie being `SameSite=Lax` (`auth_handler.go:179`, not sent on
  cross-site POST) plus ConnectRPC requiring an `application/json` content-type (forcing a
  CORS preflight). This spec makes that reliance **explicit** and requires: the new
  mutations are POST-only (no state change on GET), and we evaluate tightening the session
  cookie to `SameSite=Strict` or adding a CSRF token for these RPCs during implementation
  review.
- **Rate-limiting (Finding K).** `CreateMyAPIKey` / `RotateMyAPIKey` perform argon2id
  hashing and are unauthenticated-adjacent abuse/DoS vectors that the quota (a count cap,
  not a rate cap) does not bound. Add a per-identity rate limit on the self-mint/rotate
  endpoints, reusing the JIT limiter pattern (`identitystore.go:643`).

### 6. Audit & observability

Reuse existing metadata: `label` (purpose), `last_used_at` (async `tracker.Touch`),
`created_at`. Emit a structured audit log entry on create / rotate / revoke with the actor
being the self-identity. No new audit storage. **Note:** `label` is user-controlled; treat
it as untrusted in logs (it may carry PII) and do not log token material.

### 7. Implementation touch-surface (don't under-state it)

Although there is **no DB migration**, the change is broader than "add a handler":

- `proto/specgraph/v1/identity.proto`: four new RPCs (= 8 request/response messages +
  service-method entries) → `task proto` regenerate.
- `internal/auth/engine.go`: add `"self"` to `knownVerbs` (server won't boot otherwise).
- `internal/auth/actions.go` + `actions_test.go`: map four procedures to `apikey.self`,
  update the hard-mirrored test map, add the `self`-verb-only-on-apikey drift test.
- `internal/auth/policies/base.cedar`: new permit policy **plus a comment** noting the
  handler further restricts `apikey.self` (rejects `Source=="apikey"`, floors role) — the
  policy reads more permissive than the effective gate because `principalEntity` exposes
  only `role`/`id`/`email` (`engine.go:218`), so source/role-floor can't live in Cedar
  today (Finding 2, round 4).
- `internal/auth/`: new **exported** `RoleMin(a, b Role) Role` helper (fail-closed for
  unranked) — the floor runs in the server package and the ordering helpers are unexported
  today (Finding F3). The handler must substitute `"" (inherit) → caller.EffectiveRole`
  **before** calling `RoleMin` (empty is unranked → would otherwise fail closed to reader).
- `internal/server/identity_handler.go`: four handlers + `RoleMin`-floor on the minted
  `role_downgrade` (create **and** rotate) + `Source=="apikey"` rejection + rotate
  expiry+role re-cap. (No `kind` check / `GetUserByID` — the source gate covers service
  accounts.)
- `internal/storage/users.go` (the `UsersBackend` **interface**) + `internal/storage/postgres/users.go`
  (impl): new owner-scoped `GetAPIKeyForUser` (read) / `RevokeAPIKeyForUser` /
  `RotateAPIKeyForUser(userID, keyID, roleDowngrade, expiresAt)` (WHERE `user_id`); the
  create path opens an explicit `AuthStore` tx and locks the parent `users` row before
  count+insert (Finding F2 — `AuthStore` is not wired into `*Store.RunInTransaction`).
- Compile-time interface stubs must gain the new methods:
  `internal/auth/usersbackend_stub_test.go`, `internal/server/usersbackend_stub_test.go`
  (`var _ storage.UsersBackend = …`).
- `cmd/specgraph/`: self-mint CLI wiring + session-credential auth precedence.
- `web/src/routes/` + `web/src/lib/`: MCP Keys panel.
- New Go files need `// Package` doc comments (revive) and SPDX headers (addlicense).
- `cmd/specgraph/`: self-mint CLI wiring + session-credential auth precedence.
- `web/src/routes/` + `web/src/lib/`: MCP Keys panel.
- New Go files need `// Package` doc comments (revive) and SPDX headers (addlicense).

## Out of Scope (→ spgr-tmqm)

- MCP-spec OAuth 2.1 resource-server behavior (401 + `WWW-Authenticate`, RFC 9728
  protected-resource metadata, RFC 8707 audience binding).
- Short-lived / audience-bound / IdP-issued tokens; the IdP as authorization server.
- Resource-ownership Cedar policies across all RPCs.
- Harness-config rewrite (dropping `${SPECGRAPH_API_KEY}` env indirection;
  file-native/OAuth credential delivery).
- Role-revocation latency on stale projections (spgr-c2lb).

## Risks & Mitigations

- **Privilege escalation via cap-laundering (was CRITICAL in review)** → the minted key's
  `role_downgrade` is floored at the caller's `EffectiveRole` (not the owner's bare
  `Role`), on **both create and rotate**, so a self-minted/rotated key can never exceed the
  creator's effective role at that moment, regardless of identity source.
- **Rotate snapshot-escalation (Finding F1)** → `RotateMyAPIKey` re-floors `role_downgrade`
  at the caller's current `EffectiveRole`; rotate is never more permissive than re-minting.
- **Leaked non-admin key self-perpetuation / key-chaining (Finding F5)** → `Source=="apikey"`
  callers are rejected from mint/rotate, so a stolen non-admin key cannot mint successors
  to outlive its own revocation, and service accounts (apikey-only) are excluded. (A raw
  OIDC JWT can still mint, capped by the floor.)
- **Quota TOCTOU (Finding F2)** → explicit `AuthStore` tx with a parent-`users`-row
  `FOR UPDATE` lock around count+insert (not the invalid `count(*) FOR UPDATE`).
- **Stale role on long-lived keys (IdP demotion not propagated)** → bounded by mandatory
  expiry + relatively tight cap; full fix is spgr-c2lb.
- **`ListMyAPIKeys` cross-user leak** → hard-set `UserID` from context (empty filter
  returns all keys); explicit invariant test.
- **Self-mint endpoint abuse / argon2id DoS** → per-identity rate limit.
- **CSRF on cookie-auth'd web mutations** → `SameSite` cookie + JSON content-type
  preflight, POST-only, evaluate `SameSite=Strict`/CSRF token.
- **Self-service key sprawl** → per-user active-key quota + mandatory expiry.
- **Key existence enumeration** on rotate/revoke → owner-scoped storage WHERE clause
  returns `NotFound` uniformly for not-yours / missing / already-revoked.
- **Secret leakage into shell history** → emit-once + instruction steering away from
  inline `export`; no SpecGraph-written credential files.
- **Entrenching SpecGraph-as-authority** → acknowledged: only forward-compatible when
  claims-mapping is configured (see honesty note); does not itself migrate identity
  ownership to the IdP (that is spgr-tmqm + gh #996).

## Open Questions

- **Expiry cap tightening.** Given the stale-role caveat (a demoted user's key keeps the
  old role until interactive login, up to the cap), should the default/max be tighter than
  90d/365d? Leaning toward keeping the default but lowering the **max cap** (e.g., 180d).
  Decide at implementation review.
- **Web CSRF hardening level.** `SameSite=Lax` + JSON preflight (rely on existing) vs.
  `SameSite=Strict` vs. explicit CSRF token. Decide at implementation review.

Defaults (90d expiry, 365d max cap, 10 active-key quota) are server-configurable.
