# Identity Authn Design

**Date:** 2026-05-22
**Status:** Approved (2026-05-26)
**Part of:** Identity, RBAC & Audit epic. Companion designs: Identity Storage, Self-Service Authz, Bootstrap & UX.

## Problem

The current authentication path has three structural issues that block dynamic user management:

- **Routing is split across stores** (ConfigStore for API keys, OIDCStore for JWTs, CompositeStore as the router). Adding a persistent identity store would make this a four-way dispatch.
- **Identity is synthesized at request time from claims** for OIDC and from config at startup for API keys. There is no persistent User row, so the resolve path has nothing to look up.
- **Implicit OS-user identity** (the no-auth-configured branch) is a special case the rest of the design must work around.

After this story, all three collapse: one resolver, one database, no implicit identities.

## Scope

The request-time authentication and identity-resolution path.

Out of scope: data model (Storage spec), self-service permission patterns (Self-Service spec), bootstrap (Bootstrap spec).

## Architectural shape

A single resolver handles all credentials. It dispatches internally on whether the token is JWT-shaped (delegated to an OIDC verifier) or an API-key (prefix-lookup against the database). The interceptor and the dashboard cookie middleware both call this resolver; they no longer know about routing.

The OIDC integration is split: a verifier that performs only JWT signature/audience/expiry checks, and the resolver layer that materializes a user from claims. The verifier has no database dependency.

## Resolution semantics

- A missing or unparseable Authorization is unauthenticated. The previous "no Authorization → synthesize OS user" branch is gone.
- A valid JWT for an unknown OIDC subject becomes a new Human (JIT) iff JIT is enabled. Otherwise it is unauthenticated. The role assigned at JIT creation comes from claims-mapping (existing semantics) or a configured default if no claims match.
- A valid JWT for a known subject loads the persisted user. The persisted role is authoritative; claims-mapping is not re-evaluated. This is a deliberate trade-off (group changes in the IdP do not propagate without admin action) and must be documented for operators.
- A valid API key prefix that hashes to the stored secret loads the owner. The effective role is the owner's role, optionally clamped downward by an optional per-key role downgrade. The clamp is applied at resolve time so demoting a user instantly demotes all their keys without a sweep.
- Soft-deleted users cannot authenticate by any path. Resolution fails as if the credential were invalid.

## JIT controls

JIT user creation is a write path triggerable by anyone who can present a valid JWT against a configured issuer. It is bounded:

- **Per-issuer rate limit.** Defense-in-depth against amplification (misconfigured trust, hostile provider, aggressive testing). Process-local — multi-replica deployments accept that the effective rate is multiplied by replica count.
- **Optional email-domain allowlist.** A secondary filter when present. If the allowlist is non-empty and the token lacks an `email` claim, JIT is refused — operators get a clear error pointing at OIDC scope configuration.

Defaults: JIT enabled, rate limit on, allowlist empty. Operators can tighten or disable per deployment profile.

## Error categorization

The resolver must distinguish three failure classes that map to three different connect/HTTP codes:

- **Credential failure** (bad/missing/expired/revoked, deleted user, rate-limited, allowlist mismatch) → unauthenticated.
- **Backend failure** (database unavailable, pool exhausted, network timeout) → service unavailable. This is the critical bug class to avoid: a transient DB issue must not look like an auth failure to the caller, because the remediation is different.
- **Context cancellation** → propagate the underlying error.

The current code conflates these. The new resolver must not.

## Permission computation

> **Note (2026-05-26):** this section is superseded by the [Identity Policy Engine Adoption (Cedar)](2026-05-26-identity-policy-engine-design.md) design. Permission decisions move from a server-start snapshot of role-to-permissions to Cedar policy evaluation against the resolved Identity. The text below is retained for historical context only; the actual mechanism is Cedar.

Permissions are not stored on the User row. They are derived per request from the resolved role against a snapshot of role-to-permissions loaded at server start (built-in roles plus YAML-defined custom roles). Wildcard matching semantics are unchanged from the existing implementation.

Implication: role definitions cannot change without a server restart. Operators editing the YAML custom-role definitions must restart. This is a deliberate constraint to keep the resolve path lock-free.

## Dashboard cookie path

The dashboard's session-cookie authentication continues to work. The cookie carries the raw API key (existing behavior); the new resolver verifies it the same way it verifies any Bearer token. The REST `/api/auth/whoami` endpoint becomes a thin shim that returns whatever the canonical `WhoAmI` RPC would return — single source of truth for the identity shape across protocols.

Cookie validity tracks key validity transparently: revocation, soft-delete, and expiry all manifest as the next cookie-bearing request failing authentication. No separate cookie-revocation table.

## Non-goals

- Re-evaluating claims-mapping per request (would require either request-time IdP lookup or accepting stale claims; out of scope for this story).
- Token refresh, offline tokens, or any OAuth flow beyond verifying presented JWTs.
- Custom verifiers per provider (the existing per-provider verifier model is preserved).

## Sequencing

Depends on Storage spec for the entity model and lifecycle invariants. Is depended on by Self-Service Authz (which extends the post-resolve identity) and Bootstrap & UX (which exercises the dashboard cookie + API-key paths).
