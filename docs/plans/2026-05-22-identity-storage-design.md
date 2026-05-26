# Identity Storage Design

**Date:** 2026-05-22
**Status:** Approved (2026-05-26)
**Part of:** Identity, RBAC & Audit epic (proposed; not yet filed). Companion designs: Identity Authn, Self-Service Authz, Bootstrap & UX.

## Problem

Today, identity data (API keys, OIDC providers, custom roles) is reconstructed from YAML config at startup. There is no persistent User row; identities are synthesized at config-load and request-time. Adding any kind of CRUD requires a real persistence layer.

## Scope

The data model and storage architecture for identity.

Out of scope: how identities are resolved at request time (Authn spec), self-service permission patterns (Self-Service spec), bootstrap and operator workflow (Bootstrap spec), audit persistence (#1).

## Entities

- **Human.** A person. Onboarded via OIDC. Holds 0..N OIDC bindings (multiple providers per person) and 0..N API keys. Has a role.
- **ServiceAccount.** A machine identity. Holds 1..N API keys. Owned by a Human (ownership is transferable). Has a role. Cannot have OIDC bindings.
- **OIDCBinding.** Links a Human to an external OIDC subject. The (issuer, subject) pair is globally unique to prevent same-`sub` collisions across providers.
- **APIKey.** An issued credential. Owned by exactly one user (Human or ServiceAccount). May carry an optional role downgrade for least-privilege variants. Revocable; optionally expirable.

## Architectural constraints

1. **Identity is global, not project-scoped.** The existing per-project storage abstraction (`postgres.Store` with `WithProject`) does not apply. Identity tables live in the same Postgres database but are reached through a separate constructor.

2. **Schema evolves independently.** Migrations for identity must not collide with per-project migrations. Goose state is tracked separately.

3. **Same database, not necessarily same pool ownership.** Identity storage and per-project storage share a database connection string. Whether they share the underlying pgxpool or each owns one is an implementation choice; both must operate correctly under reasonable load. Shutdown ordering is the implementer's responsibility to get right.

4. **Plaintext API keys are never persisted.** Hash at rest using a modern memory-hard algorithm. Plaintext is shown to the user once, at creation.

5. **API keys have a separable, indexed lookup prefix.** Resolution must be O(log N) per request, not O(N). The associated timing side-channel (an observer can probe prefix existence by latency) is accepted in exchange for performance, given a prefix-entropy floor sufficient to make blind enumeration impractical. The secret is verified in constant time.

6. **At most one bootstrap admin exists at a time.** Enforced at the schema layer so multiple concurrent bootstrap paths (`init` and server safety net) can race safely without coordination.

7. **OIDC binding uniqueness is global, not per-provider.** The (issuer, subject) tuple uniquely identifies a binding. This protects against the latent risk that two providers happen to issue the same `sub` value to different people.

8. **Cross-domain access goes through the `users.Store` interface.** No direct SQL references to identity tables from outside the identity-storage package. Other domains that need identity questions answered (project-scoped RBAC, ownership rules, audit) depend on the interface, not on schema. This preserves identity's freedom to evolve its layout without rippling into consumers, and keeps cross-domain coupling explicit at the interface boundary.

## Non-goals

- Project-scoped identity (story #3).
- Groups, teams, or hierarchical identity.
- Role definitions in the database. Roles remain a YAML-defined policy concept; assignments live in the database.

## Lifecycle policies

- **Soft-delete is the default mutation.** Setting `deleted_at` cascades atomically to revoking the user's active API keys. OIDC binding rows remain (they're history) but resolve must ignore them — see Authn spec.
- **ServiceAccount soft-delete is symmetric to Human soft-delete.** Same cascade (`deleted_at` set, active keys revoked in the same transaction), same denial of re-authentication. The Kind discriminator does not change lifecycle semantics; only the constraints on what credentials each kind can hold.
- **Hard delete (purge) cascades through bindings and keys.** It is rare, administrative, and protected. (Authorization gate specified in Self-Service Authz spec; bootstrap-user protections in Bootstrap spec.)
- **Restoring a soft-deleted user un-deletes by clearing `deleted_at`.** Re-binding to a fresh OIDC subject is not supported; the historical binding is reused.
- **Soft-deleted users can never re-authenticate.** Verification of this property is the Authn spec's responsibility; the data-model contribution is that `deleted_at` is queryable per-resolve.

## Migration policy

No data migration from the existing YAML-backed `config_store`. Operators recreate identities via the new CLI after upgrade. Rationale: dual-source identity stores are a permanent maintenance burden, and there is no large existing population to migrate.

The existing `config_store.go`, `composite_store.go`, and `token_store.go` are removed in the same release that introduces this storage. Their absence is a breaking change called out in release notes.

## Sequencing relative to other stories

This is the structural foundation under three peers:

- **Authn:** depends on the entity model and the (issuer, subject) uniqueness for race-safe JIT.
- **Self-Service Authz:** depends on `Identity.UserID` being available post-resolve, which depends on the User row existing.
- **Bootstrap & UX:** depends on the bootstrap-admin invariant and the soft-/hard-delete protections.

The audit log story (#1) consumes events emitted by the other three. It does not depend on this storage story directly.
