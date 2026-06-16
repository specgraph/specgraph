# OIDC App-Roles + Login-Sync — Design

- **Status:** Draft (design)
- **Date:** 2026-06-15
- **Tracking:** GitHub #996 (OIDC `claims_mapping`: groups overage pattern not handled)
- **Follow-up (out of scope):** `spgr-g7st` — self-service / auto MCP API-key
  provisioning; `spgr-c2lb` — enforce app-role revocation on standing API/MCP
  keys (force re-sync)
- **Reviewed:** adversarial review 2026-06-15 — findings folded in below
  (security premise sound; default-on demotion risk accepted with eyes open,
  see Operational Warning)

## Problem

`claims_mapping` rules that target the `groups` claim silently never match for
users whose group membership exceeds the IdP's inline cap (Entra "groups
overage"). In that case the token omits `groups` entirely and substitutes the
`_claim_names` / `_claim_sources` indirection, so a rule like
`{claim: groups, value: <id>, role: admin}` never fires and the user falls
through to `default_role` (GitHub #996).

A second, related limitation surfaced during design: `claims_mapping` is
evaluated **only at JIT (just-in-time) user creation** (`identitystore.go:513-523`).
On every subsequent sign-in the stored binding resolves the DB-persisted role
and `claims_mapping` is ignored. So even when group mapping *does* work, changing
a user's IdP assignment never updates their SpecGraph role.

## Decision: pivot to app roles, not group resolution

Rather than fetch the overflowed group list from the directory API (rejected —
see Alternatives), we lean on **app roles**, the Microsoft-recommended workaround
the issue itself names. App roles ride **inline in the token as a `roles` array
claim** and are **never subject to overage** — assigning an app role to a large
group still emits the flattened role values in `roles`. Operators target the
`roles` claim with the existing `claims_mapping` mechanism, which already matches
string-array claims (`matchClaimValue`, `identitystore.go:579-593`).

No Graph API calls, no client-credentials grant, no overage detection, no
`GroupResolver`.

### Example operator config (already valid today)

````yaml
auth:
  oidc:
    sync_on_login: true   # NEW (default true)
    providers:
      - id: entra
        issuer: "https://login.microsoftonline.com/<tenant>/v2.0"
        client_id: "<client-id>"
        audience: "<client-id>"
        client_secret: "<secret>"
        interactive: true
        display_name: "Entra"
        claims_mapping:
          - claim: "roles"
            value: "specgraph.admin"
            role: "admin"
          - claim: "roles"
            value: "specgraph.user"
            role: "writer"
````

`value` matches the app role's **Value** field in the Entra app registration
(e.g. `specgraph.admin`), not its display name. `admin`/`writer` are built-in
roles (`auth.go:12-14`) validated at startup against `KnownRoles`
(`identitystore.go:115-126`).

## Scope

On each **interactive login** — the OIDC browser callback **and** the CLI login
broker, both of which flow through `resolver.Resolve(WithInteractiveLogin(ctx), …)`
(`auth_oidc_handler.go:215`) — for an OIDC user whose binding already exists,
SpecGraph will:

1. **Re-enforce the email-domain allowlist** (M-1) — `applyLoginSync` re-checks
   `jitEmailAllowlist` (the same gate `jitResolve` applies at creation,
   `identitystore.go:481-493`). On a domain miss, the login is **denied**
   (`ErrUnauthenticated`); a domain removed from the allowlist revokes access on
   next interactive login rather than only blocking new users.
   **Absent-email exception (R4 MEDIUM-5):** unlike `jitResolve` (which denies an
   empty email under a non-empty allowlist, `identitystore.go:483-487`), for an
   **existing** binding `applyLoginSync` **skips** the allowlist re-check when the
   token carries no email claim — the user already passed at bind time
   (`email_at_bind`, `auth_migrations/001_initial.sql`). Only a **present** email
   whose domain is now disallowed denies. This preserves revoke-on-domain-removal
   without locking out established users when a provider intermittently omits
   `email`/`preferred_username`.
2. **Refresh metadata** — update `DisplayName` and `Email` from the verified
   token claims, with guards (see Metadata refresh below).
3. **Re-evaluate role (single-issuer)** — re-derive the role from **only the
   issuer the user just authenticated with** and update the stored role. The
   role reflects **what the live token grants**. A user with multiple bindings
   gets the role derived from their most-recent interactive login's issuer
   (per-last-login). See Role re-eval.

Per-request bearer JWTs and API-key requests (the MCP path, machine/CLI clients)
are **not** interactive logins and therefore trigger **no** sync. First-time
users still go through the unchanged `jitResolve` path.

### Metadata refresh (H3 — `OIDCClaims` has no name field today)

`OIDCClaims` currently exposes only `Issuer/Subject/Email/Nonce/Raw`
(`oidc_verifier.go:23-29`) — there is **no parsed display name**, and
`jitResolve` sets `DisplayName = claims.Subject` (the opaque `sub`) with the
note "operator can rename later" (`identitystore.go:528`). Naively refreshing
`DisplayName` from claims would either have no source or overwrite an operator
rename with the opaque subject on every login.

Required changes:

- Add a parsed `Name string` to `OIDCClaims`, sourced in `oidc_verifier.go` from
  the `name` claim, falling back to `preferred_username` (the same fallback
  order already used for email).
- `applyLoginSync` updates `DisplayName` **only when** the stored value still
  equals the OIDC subject (i.e. never renamed by an operator) **and** the new
  `Name` is non-empty. Operator renames are preserved.
- `Email` is refreshed from the verified `email`/`preferred_username` resolution
  (`oidc_verifier.go:99-109`); skip the write when unchanged.

**Caveats (L-2/L-3):**

- The "stored == subject ⇒ never renamed" test is a **value-equality heuristic**,
  not true provenance (same limitation as the deferred `role_source` column). It
  false-positives if an operator renames a user to a value equal to the opaque
  `sub`, and behaves oddly for IdPs where `sub == email`.
- `Email` has **no rename guard** (only skip-if-unchanged). This is benign today
  because no admin email-edit operation exists (`UsersBackend` exposes only
  `UpdateUserRole`), but the asymmetry with `DisplayName` is intentional and
  must be revisited if an email-edit feature lands.
- When both `name` and `email` claims are absent, `Name` and `Email` both fall
  back to `preferred_username`, so `DisplayName` becomes an email-shaped string.

### Out of scope

- Directory/Graph API group fetching and the `_claim_names`/`_claim_sources`
  overage indirection (rejected approach).
- Self-service or auto MCP API-key provisioning (`spgr-g7st`).
- Enforcing app-role *revocation* on standing API/MCP keys without re-login
  (`spgr-c2lb`) — see Operational Warning.
- Re-evaluating role on non-interactive (per-request) credentials.

## Architecture

### Hook point

A new method on `pgIdentityStore`:

````go
// applyLoginSync re-enforces the email allowlist, refreshes profile metadata,
// and re-derives the role from the issuer just authenticated (single-issuer)
// for an existing OIDC user on interactive login. Returns the user to surface
// as the resolved Identity, or an error that DENIES the login.
//
// Error model is asymmetric by privilege direction:
//   - Allowlist miss            -> deny (ErrUnauthenticated).
//   - PROMOTION persist failure -> best-effort: log a warning, proceed at the
//     OLD (lower) role. Fails closed; no privilege is granted unbacked by the DB.
//   - DEMOTION persist failure  -> DENY the login (fail closed). Never return an
//     Identity whose role exceeds what is currently persisted; emit an
//     error-level security event. "Demotion" = any change that is NOT a provable
//     built-in promotion (see Demotion classifier).
//   - Metadata-only failure     -> best-effort: log a warning, proceed.
func (s *pgIdentityStore) applyLoginSync(ctx context.Context, claims *OIDCClaims, user *storage.User) (*storage.User, error)
````

**Returned-Identity invariant (M-4):** mutate the returned `user` struct only
from values that `UpdateUserOnLogin` persisted successfully. On any persist
error, return the originally-loaded `user` unchanged (promotion path) or an
error (demotion/allowlist path). The Identity's role MUST never exceed the
persisted role.

The store gains a `loginSyncEnabled bool` field (wired from config — see Wiring
below). `applyLoginSync` is called from `resolveJWT` in the
**existing-binding** branch — after the soft-delete check (`identitystore.go:451-458`)
and before the Identity is built (`identitystore.go:459`) — gated on:

````go
if s.loginSyncEnabled && InteractiveLoginFromContext(ctx) {
    var err error
    user, err = s.applyLoginSync(ctx, claims, user)
    if err != nil {
        return nil, err // deny: allowlist miss or failed demotion
    }
}
````

**Wiring (L-4):** add `LoginSyncEnabled bool` to the identity-store config
struct and thread `cfg.Auth.OIDC.SyncOnLogin` into it at the store construction
site in `serve.go` (alongside `JITClaimsMapping`/`JITDefaultRole`,
`serve.go:162-173`).

`InteractiveLoginFromContext` is the **sole gate** distinguishing a login event
from a per-request bearer JWT. Its only production setter is the browser callback
(`auth_oidc_handler.go:215`). Add a defensive comment at the `WithInteractiveLogin`
definition forbidding new callers on any per-request/bearer path — one misplaced
setter would let an ordinary MCP/RPC request mutate roles.

### Role re-eval decision (single-issuer)

A login carries exactly **one** issuer's token. The re-derived role is computed
from **that issuer's** `claims_mapping` against the verified claims — the role
reflects what the **live token** grants. We do **not** read or combine other
bindings (the rejected most-privileged-across-bindings approach added a
`derived_role` column, a cross-binding transaction the auth storage layer cannot
provide, and a permanent stale-escalation hole; see Alternatives #5).

Multi-binding note: a `User` may hold multiple bindings (`oidc_bindings` is
unique only on `(issuer, subject)`; `ListOIDCBindings` returns many,
`users.go:106-107`). With single-issuer, the role is re-derived from the issuer
just logged in through. **Scope of the guarantee (R4 HIGH-2):** when that issuer
**has `claims_mapping`**, the role equals what its mapping asserts for the live
token (rules 2/3). When that issuer has **no** mappings (rule 1), the prior role
is left **unchanged** — so a user who logs in through a mapping-less provider
**retains** whatever role was last persisted (possibly derived months ago from a
*different* provider). This is a deliberate, narrower form of role retention: it
avoids the C1 mass-demotion for mapping-less providers, but it is **not** the
stronger "your role always equals the credential you just presented" property —
that holds only for issuers that have mappings. The "flap" between mapping-having
issuers is still the safe direction (role = live token from that issuer).

#### Pure, table-tested helper

````go
// resolveLoginRole computes the new role for an interactive login from the
// issuer's mappings and the verified claims. Returns (newRole, changed).
func resolveLoginRole(
    mappings []config.ClaimMapping,   // this issuer's claims_mapping (nil/empty if none)
    claims map[string]json.RawMessage,
    currentRole string,
    defaultRole string,
) (role string, changed bool)
````

Rules — **evaluated in this exact order** (the ordering is the single
correctness hinge; conflating "no mappings" with "no match" re-introduces the C1
mass-demotion for every mapping-less provider):

| # | Condition | Result |
|---|---|---|
| 1 | `len(mappings) == 0` (no `claims_mapping` for the issuer) | role **unchanged** — guard; never wipe roles for a provider without mappings |
| 2 | mappings configured, a rule matches | that rule's role |
| 3 | mappings configured, no rule matches | `default_role` (or `reader` if unset, matching `jitResolve`) |

Rule-1-before-rule-3 ordering MUST be table-tested explicitly. Note
`buildClaimsMappingByIssuer` only inserts a key when `len(ClaimsMapping) > 0`
(`serve.go:775`), so "no provider entry" and "empty slice" both collapse to a
nil/empty lookup → rule 1.

This makes the IdP authoritative on interactive login over the **`roles` claim**,
**for issuers that have `claims_mapping`**: losing your app-role assignment on a
mapping-having issuer you log in through demotes you to `default_role` on next
login via that issuer. (Logging in via a mapping-less issuer is rule 1 → no
change; see the Scope-of-the-guarantee note above.)

**Matching caveats (inherited from `matchClaimValue`, `identitystore.go:579-593`):**

- Values are matched by **exact, case-sensitive string `==`**. App-role `Value`s
  in the IdP MUST be strings; a numeric claim (e.g. `"roles":[1]`) never matches
  → rule 3. Document this as a hard requirement.
- `applyClaimsMapping` returns the **first** matching rule in config order
  (`identitystore.go:564-575`), not the most-privileged. A user holding both
  `specgraph.user` and `specgraph.admin` gets whichever rule is listed first —
  order rules **most-privileged-first**. Document this.

### Demotion classifier (custom roles fail closed — H4)

The error model (above) treats demotions as fail-closed and promotions as
best-effort. "Promotion" is defined **narrowly and provably**:

````go
// isPromotion reports true ONLY when both roles are built-ins and the new
// built-in rank is strictly higher than the current. Everything else — a rank
// decrease, equal, or ANY transition involving a custom/unranked role — is
// treated as a potential demotion (fail closed).
func isPromotion(current, next string) bool
````

Rationale: `roleRank` (`identitystore.go:246`) covers only built-ins
(`reader=1, writer=2, admin=3`); the codebase deliberately treats custom roles as
**incomparable** (`roleLessThan` returns false both ways, `clampedRole` fails
closed to `reader`, `identitystore.go:260-292`). We do **not** invent a synthetic
rank for custom roles. Instead, any change that is not a provable built-in
promotion is conservatively classified as a demotion, so a custom-role user can
never be left **more** privileged than the IdP currently grants on a persist
failure. This matches `clampedRole`'s fail-closed philosophy.

**`isPromotion` MUST NOT be consulted when the role did not change** — see the
algorithm below, where classification is gated on `changed` first. `isPromotion`
returns false for equal roles by construction, so consulting it on a
role-unchanged login would mis-route a metadata-only write failure into the deny
path.

### Algorithm (explicit ordering — R4 HIGH-1)

`applyLoginSync` MUST compose the helpers in this exact order. The three concerns
(skip-if-unchanged, role classification, error bucket) are only correct together:

````text
1. Allowlist: if token has an email AND its domain ∉ allowlist  -> return ErrUnauthenticated (deny).
   (absent email on an existing binding -> skip this check; see Scope step 1)
2. Compute newRole, changed := resolveLoginRole(issuerMappings, claims, user.Role, defaultRole)
   Compute newDisplayName (rename guard), newEmail (skip-if-empty).
3. If (newDisplayName, newEmail, newRole) == (user.DisplayName, user.Email, user.Role):
       -> NO write, NO classification, return user unchanged. (L3 no-op skip)
4. Else call UpdateUserOnLogin(newDisplayName, newEmail, newRole). On success,
   mutate the returned user from the persisted values; emit an audit line if
   `changed`. Return user.
5. On UpdateUserOnLogin error, classify by `changed` FIRST:
       changed == false                 -> metadata-only: log warning, return ORIGINAL user (best-effort, login proceeds).
       changed && isPromotion(old,new)  -> promotion:     log warning, return ORIGINAL (lower) user (best-effort).
       changed && !isPromotion(old,new) -> demotion:      emit error-level audit line, return (nil, err) (DENY).
````

Key invariant: the returned Identity's role **never exceeds** the persisted role
(M-4). The audit line (Operational Warning) fires on step 4 (successful change)
and on the step-5 demotion-deny branch — **never** on a metadata-only failure, to
avoid spurious "failed demotion" security events.

### Storage

`UpdateUserRole` exists (`users.go:49-51`) but there is no profile update. Add
one method to `UsersBackend` and its Postgres impl:

````go
// UpdateUserOnLogin sets display_name, email, and role on an active user in a
// single UPDATE (deleted_at IS NULL guard, like UpdateUserRole). Returns
// ErrUserNotFound if no active row matched.
UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error
````

Single-issuer means the entire mutation is **one `UPDATE` on the `users` row**
(display_name, email, role are all columns there) — atomic as a single statement,
**no transaction needed**, no new column, no `RunInTransaction` (which is in any
case unreachable from `pgIdentityStore`: it lives on `*Store`/`tx.go:42`, while
`AuthStore` uses `s.pool` directly and ignores `txFromContext`). This follows the
existing `UpdateUserRole` shape (`postgres/users.go:225-237`).

The surrounding logic is still **read-modify-write** (role computed from the user
loaded earlier in `resolveJWT`, written here), so a concurrent admin
`UpdateUserRole` racing the login is **last-writer-wins** — acceptable and
documented (H2: when sync is on, the IdP is the sole authority over the
`roles`-derived role, so login-sync winning is intended). The fail-closed
demotion guarantee holds because the single `UPDATE` either commits the new
(lower) role or returns an error that denies the login.

**Startup role validation (H-2 — current guard is insufficient).** The doc
previously assumed `claims_mapping` roles "are validated against `KnownRoles` at
startup." That guard is **gated on `cfg.JITEnabled`** (`identitystore.go:115`),
but `JITClaimsMapping` is wired unconditionally (`serve.go:170`) and login-sync
consumes it independently of JIT. So with `jit_create.enabled: false` +
`sync_on_login: true` + a typo'd role (`role: admln`), startup passes and the
next login assigns `admln`, which authorizes nothing (default-deny,
`known_roles.go`) → **silent lockout** — and exactly the manual-management
audience told to set `jit_create.enabled: false`. **Required change:** validate
`JITClaimsMapping` and `JITDefaultRole` against `KnownRoles` whenever
`loginSyncEnabled || JITEnabled` (not `JITEnabled` alone). Update the
`NewIdentityStore` guard at `identitystore.go:115`.

**Write amplification (L3):** skip the `UPDATE` entirely when the computed
`(displayName, email, role)` tuple equals the stored values, to avoid redundant
row writes/WAL on every no-op login. (Note: the `users` table has **no
`updated_at` column**, `auth_migrations/001_initial.sql`; the optimization is
about avoiding redundant writes, not trigger churn.)

## Config

Add one field to `OIDCConfig` (`global.go:206-219`):

````go
// SyncOnLogin enables refreshing DisplayName/Email and re-evaluating the
// role from token claims on each interactive login. Default true.
SyncOnLogin bool `yaml:"sync_on_login" koanf:"sync_on_login"`
````

Default **true**, set **only in `globalDefaults()`** (the struct-defaults path,
exactly as `CLILoginEnabled` is handled, `global.go:428`). It MUST NOT be
defaulted in `applyPostLoad`: a bool whose desired default is `true` cannot be
distinguished there from an operator's explicit `sync_on_login: false`, so an
`applyPostLoad` guard would force-enable sync and make opt-out impossible.

Operators who manage roles manually set it to `false` to retain the legacy
JIT-only behavior.

## Behavior across credential types (MCP / API keys)

- **MCP harnesses** authenticate with static API keys (`spgr_sk_…`) via
  `${SPECGRAPH_API_KEY}` (`managedfiles/manifest.go:42,63,93`). API-key requests
  are not interactive logins → **no sync, no role change** on the MCP path. The
  per-request invariant holds: nothing on `/mcp/` can mutate a role.
- API-key requests resolve the **owner user's current DB role** every request
  (`resolveAPIKey`). So a role change from a user's interactive login (web or
  CLI) **propagates to their MCP key automatically** on its next call — no cache
  to go stale.
- **Boundary:** role changes reach MCP/API-key usage only **after** the human
  re-authenticates interactively via an **OIDC** login (web callback or
  `specgraph login`). A user who never interactively re-logs-in won't see
  app-role changes take effect — including app-role *removal*, which is **not
  enforced** on standing keys until re-login (follow-up `spgr-c2lb`). Note the
  web dashboard also offers an **API-key paste** login (`auth_handler.go:81`)
  that stores the raw key in a session cookie and resolves per-request as an API
  key; that path is **non-interactive** and never drives sync. Only OIDC
  interactive sessions do.

## Operational Warning — default-on demotion (C1, accepted risk)

`sync_on_login` defaults **true** and re-derives every existing OIDC user's role
on their next interactive login. This is intentional, but carries a real
lockout/mass-demotion hazard that operators MUST understand:

- The premise of #996 is that existing `claims_mapping` rules target the broken
  `groups` claim. **Until** rules are re-pointed at `roles` **and** each `value`
  exactly matches the IdP app-role `Value` (case-sensitive), every OIDC user
  hits rule 3 (no match) and is **demoted to `default_role` on next login**.
- If admins were promoted via app roles that aren't yet assigned, or via manual
  `auth user set-role` (see H2 below), they are demoted too. The **bootstrap
  admin key is the only immune identity** (a SYSTEM admin with no OIDC binding,
  `bootstrap.go`) — if it was discarded, a misconfig can leave the deployment
  with **no admin**.

Required mitigations in docs/runbook (behavior unchanged per maintainer
decision):

- **Migrate before first login:** re-target `claims_mapping` to `roles`, verify
  every app-role `Value` string, and assign app roles to admins **before**
  anyone logs in post-upgrade. Or set `sync_on_login: false` during migration
  and flip it on after verifying.
- **Keep the bootstrap admin key** recoverable until app-role mapping is
  confirmed working. (The bootstrap admin is immune because it has no OIDC
  binding and authenticates via `resolveAPIKey`, not `resolveJWT` — see
  `bootstrap.go:44-48` for its `spgr_sk_`/admin shape.)
- **Revoking via app-roles — do NOT delete the whole block (R4 MEDIUM-3).** To
  *revoke* a provider's role grants, keep **≥1** mapping rule so rules 2/3 apply
  (no-match → `default_role`). Deleting the **entire** `claims_mapping` block
  flips the provider to **rule 1 = freeze**: nobody logging in via it is
  re-evaluated and everyone keeps their last role. This is the inverse hazard of
  the mass-demotion above — silent privilege *retention* after an operator
  believes they revoked grants.
- **No-match demotion floor (R4 LOW).** Rule 3's floor is
  `jit_create.default_role` (or `reader` if unset), reused even when
  `jit_create.enabled: false`. Operators running manual JIT-off setups who never
  set `jit_create.default_role` will see no-match users demoted to `reader`.
  Document that `jit_create.default_role` governs login-sync's floor regardless
  of `jit_create.enabled`.
- **Audit logging (M-3):** there is no audit subsystem today, only `slog`.
  `applyLoginSync` MUST emit a dedicated structured line with an `audit=true`
  attribute (subject, issuer, old → new role) on each role-change **decision** —
  i.e. on a successful change (algorithm step 4) and on the **error-level**
  demotion-deny branch (step 5, `changed && !isPromotion`). It MUST NOT fire on
  metadata-only no-op failures (`changed == false`). The Operational-Warning
  "demotions are visible" guarantee depends on the failed-demotion line existing.

### H2 — IdP is the sole role authority when sync is on

There is a single `role` column on `users` with no provenance (schema:
`auth_migrations/001_initial.sql`). When `sync_on_login` is enabled, the IdP (via
`claims_mapping`) is the **sole** authority for the `roles`-derived role of
synced OIDC users: a manual `auth user set-role` promotion is **overwritten** on
that user's next interactive login **through a provider that has `claims_mapping`**
(rules 2/3). **Qualifier (R4 MEDIUM-4):** if the user logs in through a
mapping-less provider, rule 1 leaves the manual grant **intact** — so manual
durability is non-deterministic across providers. This is documented, not guarded
— operators must not rely on manual grants for users covered by a provider's
`claims_mapping`. (A `role_source` provenance column to make manual grants
durable was considered and deferred.)

**Scope of "authority" (M-1 honesty):** login-sync is authoritative over the
**role derived from the `roles` claim only**. It is *not* a general access gate
beyond what is enforced: the email-domain allowlist **is** re-enforced on each
interactive login (deny on miss — see Scope step 1), but standing API/MCP keys
are not revoked until re-login (`spgr-c2lb`).

## Error handling (asymmetric by privilege direction — H-1)

`applyLoginSync`'s failure behavior is determined by the **Algorithm** above; the
buckets below restate its step-5 classification (always gated on `changed` first):

- **Allowlist domain miss** → **deny** the login (`ErrUnauthenticated`). (Absent
  email on an existing binding skips the check — see Scope step 1.)
- **`changed && !isPromotion` (demotion) persist fails** → **deny** (fail closed).
  Never return an Identity whose role exceeds the persisted role; emit an
  error-level `audit=true` line.
- **`changed && isPromotion` (promotion) persist fails** → **best-effort**: log a
  warning, proceed at the OLD (lower) role. No privilege granted unbacked by DB.
- **`changed == false` (role unchanged, metadata-only) persist fails** →
  best-effort: log a warning, proceed; role is unaffected. **`isPromotion` is not
  consulted here** — this bucket is reached only because classification is gated
  on `changed`, which is what keeps it reachable (R4 HIGH-1).
- Soft-deleted / inactive users are already rejected upstream in `resolveJWT`
  before the sync hook; `UpdateUserOnLogin`'s `deleted_at IS NULL` guard is
  defense-in-depth.

The asymmetry is the point: a **sync hiccup must never leave a user more
privileged than the IdP currently grants**, but it may transiently leave them
*less* privileged (safe), and a role-unchanged login is never denied for a
metadata write hiccup.

## Testing

- **`resolveLoginRole`** table tests, asserting **rule ordering**: `len==0`
  (no mappings) → **unchanged even though no rule matches** (the C1 hinge);
  configured + match → mapped role; configured + no-match → `default_role`;
  `default_role` unset → `reader`; most-privileged-first rule ordering;
  numeric/non-string claim → no match.
- **`isPromotion` classifier (H4)**: built-in rank increase → true; built-in
  decrease/equal → false; any transition involving a custom role (custom→builtin,
  builtin→custom, custom→custom) → false (treated as demotion / fail-closed).
- **Fail-closed demotion (H-1)**: demotion decided + `UpdateUserOnLogin` errors
  → login **denied**, Identity NOT returned at the old elevated role, audit line
  emitted. Promotion + persist error → login proceeds at old lower role.
- **Metadata-only write failure does NOT deny (R4 HIGH-1)**: role unchanged
  (rule 1, or rule 2 matching the same role) + email/name changed +
  `UpdateUserOnLogin` errors → login **proceeds** (best-effort), NO deny, NO
  "failed demotion" audit line. This is the regression-guard for the classifier
  ordering bug.
- **No-op skip**: unchanged `(displayName, email, role)` tuple → no `UPDATE`
  issued at all (so no persist-failure path).
- **Allowlist re-enforcement (M-1)**: existing-binding user whose email domain
  was removed from the allowlist → login **denied**; same user with a token that
  **omits** the email claim → login **allowed** (absent-email skip, R4 MEDIUM-5).
- **Startup validation (H-2)**: `sync_on_login: true` + `jit_create.enabled:
  false` + typo'd mapping role → `NewIdentityStore` returns an error at startup
  (not a runtime lockout).
- **Identity store**: interactive login updates role + metadata; a plain
  bearer-JWT (non-interactive) request makes **no** change (preserves the
  per-request invariant); API-key request makes no change.
- **Multi-binding**: a user with bindings on two issuers gets the role of the
  issuer they logged in through (per-last-login); logging in via a mapping-less
  issuer (rule 1) **retains** the prior role (R4 HIGH-2 retention behavior).
- **Metadata guards**: `DisplayName` updated only when stored value still equals
  the subject (not after an operator rename); no-op tuple skips the write.
- **Flag**: `sync_on_login: false` reproduces today's JIT-only behavior; default
  unset → enabled (defaulted in `globalDefaults`, explicit `false` honored).
- **Storage**: `UpdateUserOnLogin` updates the three columns in a single
  statement, respects the active-row guard, returns `ErrUserNotFound` for
  missing/deleted rows (Postgres integration test).

## Documentation

- `site/docs/guides/oidc-login.md`: recommend targeting the `roles` (app-role)
  claim over `groups`; explain the groups-overage limitation and why app roles
  avoid it; document `sync_on_login`, its demotion semantics, the migration
  warning above, the IdP-sole-authority rule (H2), the string-`Value` and
  most-privileged-first requirements, and that the issuer **must be
  tenant-pinned** (not the multi-tenant `common`/`organizations` endpoint, which
  could admit attacker-controlled `roles` from another tenant — L4).
- Note the MCP boundary (role changes apply after the next interactive OIDC
  login; key-paste sessions never sync) and the per-last-login behavior for
  multi-binding users.

## Alternatives considered

1. **Directory/Graph group fetch (rejected).** Detect the overage pattern and
   call `getMemberObjects` via app-only client credentials, behind a
   `GroupResolver` seam. Rejected by the maintainer in favor of app roles:
   adds network egress during auth, directory-API credentials, throttling and
   error handling, and targets the deprecated `graph.windows.net` overage
   endpoint. App roles solve the same need inline.
2. **First-class `app_role_mapping` config (rejected).** A dedicated mapping
   block separate from `claims_mapping`. Unnecessary — `claims_mapping` already
   matches the `roles` array claim. YAGNI.
3. **Generic distributed-claims fetcher (rejected).** Follow
   `_claim_sources.endpoint` for any claim. Over-engineered; only `groups`
   overage matters and it special-cases Graph anyway.
4. **Sync on every JWT-bearing request (rejected).** Most current, but adds a DB
   write to every machine/CLI/MCP request. Login-event scoping bounds the cost.
5. **Most-privileged role across all bindings (designed, then rejected in
   review round 3).** Persist each binding's last-derived role in a new
   `oidc_bindings.derived_role` column and take the max on login. Rejected:
   (a) the cross-binding transaction is **unreachable** from `pgIdentityStore`
   (`RunInTransaction` is on `*Store`/`tx.go:42`; `AuthStore` uses `s.pool`
   directly), so the writes would be non-atomic; (b) a stale `derived_role` from
   a provider the user stopped using becomes **permanent privilege escalation**
   with no live token backing it; (c) empty/`NULL` `derived_role` overloads
   "no mapping" vs "not yet evaluated", causing spurious post-migration
   demotion; (d) custom roles are **incomparable** (`roleRank`/`clampedRole`,
   `identitystore.go:246-292`), so "most-privileged" has no coherent definition
   for them. Single-issuer ("role = what the live token grants") is simpler and
   strictly safer — the per-last-login "flap" is the secure failure direction.
