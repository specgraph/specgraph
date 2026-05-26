# Identity Bootstrap & Operator UX Design

**Date:** 2026-05-22
**Status:** Approved (2026-05-26)
**Part of:** Identity, RBAC & Audit epic. Companion designs: Identity Storage, Identity Authn, Policy Engine Adoption (Cedar).

## Problem

Identity exists in the database now. Someone has to be the first user, and there has to be a credential to authenticate them on the very first request — before any RPC could create that credential.

Existing code half-solves this. On first server start in "local" mode with no API keys configured, it generates a default admin key and writes it to a YAML credentials file. The server reads the same file on subsequent starts. With the new database-backed identity model, this duality (server writes; server reads; client also reads) breaks.

The bootstrap story also has to handle two deployment shapes — single-host local development where the operator has direct database access, and hosted/team deployments where the operator only has RPC access — and recover gracefully when the operator's environment doesn't match assumptions.

## Scope

The first-admin creation flow, the credentials file's role, and the operator-side protections against accidentally wiping the bootstrap identity.

Out of scope: how subsequent users are created (Authn spec covers JIT for Humans; ServiceAccount creation is a normal admin RPC), credential lifecycle policies (covered indirectly via Storage spec invariants), audit emission (#1).

## Architectural shape

Two parallel bootstrap paths exist because there are two deployment shapes:

- **Local-mode bootstrap.** Driven by `specgraph init`, which has direct database access. Creates the first Human, mints an admin key, persists everything to the database, and writes the credential to a known file path that the local CLI will subsequently read.
- **Hosted bootstrap.** Driven by the server's first start. Creates the first Human and mints the key the same way, but does not write to any filesystem the operator can access. Instead, the key is surfaced in the server's logs with a banner explaining what to do with it (copy into local credentials file; rotate immediately).

Both paths are idempotent (a bootstrap user already exists → skip) and race-safe (concurrent calls converge to one row by relying on the storage invariant that at most one bootstrap admin exists).

The two paths share a single helper that performs the database work. They differ only in whether they write the credentials file.

`specgraph init` detects which path applies from whether it has a working Postgres connection at startup. With one, it runs the local bootstrap; without one, it skips bootstrap entirely and prints a hint that the server's first start will handle it.

## The credentials file's role changes

Today the credentials file is dual-purpose: the server reads it as a source of API keys; the CLI reads it for outgoing auth headers. After this story, only the CLI reads it. Editing the file no longer creates a key on the server.

This is a user-visible behavior change that must be called out in release notes. Operators who relied on YAML-driven key provisioning move to `specgraph auth api-key create`.

**The credentials file is designed for multiple server configurations from day one.** A single CLI install commonly needs to authenticate against several SpecGraph deployments — a local dev instance, a team-hosted server, possibly dev/staging/prod tiers. The file holds entries for multiple server URLs; the CLI selects the active one via explicit flag, environment variable, or a configured default. The exact schema (URL-keyed entries vs named profiles with associated URLs, in the style of kubectl contexts or aws profiles) is an implementation choice; the property is that switching between servers does not require rewriting the file.

Bootstrap interactions with multi-server:

- **Local-mode bootstrap** adds (or refreshes) an entry for the local server URL; it does not overwrite entries pointing at other servers. The credentials file is operator-owned state that may already contain unrelated entries.
- **Hosted-mode bootstrap** prints the key together with the server URL it pertains to, so the operator can append cleanly to their own credentials file on a different host.

Out of scope: cross-machine sync of the credentials file. It remains a local artifact per operator.

## Bootstrap user is a system identity

The bootstrap user is a **system identity**, not a representation of any person. Concretely:

- `display_name` is the literal string `admin` (not derived from the OS user, hostname, or any environmental signal).
- `email` is empty or a placeholder; operator-editable later if desired.
- It exists so the operator can act on SpecGraph before OIDC is configured, and so recovery is possible when OIDC breaks. It is the system's backstop, conceptually equivalent to `root` on Unix or the `postgres` superuser.

The operator's actual personhood arrives later through normal OIDC sign-in. That sign-in creates a separate Human row via JIT (default role, typically `reader`), which the operator then promotes to `admin` using the bootstrap key. The bootstrap admin remains as the emergency-access identity; force-flags protect it from accidental destruction (see "Protecting the bootstrap identity" below).

This framing eliminates the bootstrap-vs-JIT collision class by construction: the bootstrap admin has no OIDC binding, so OIDC sign-ins are always fresh JITs and never accidentally resolve to the bootstrap user. Two separate concerns; two separate rows; no fallback-chain logic from environmental data.

After OIDC is operational and at least one real-person admin exists, operators MAY choose to soft-delete the bootstrap admin (force-flag-protected). The system supports this but does not require it; many deployments will keep the bootstrap admin indefinitely as a recovery identity. No auto-cleanup behavior.

## Protecting the bootstrap identity

The bootstrap user is just a user with a flag set, but losing it accidentally is operationally annoying (soft-delete recoverable, hard-delete not). Both destructive operations carry explicit overrides:

- Soft-deleting the bootstrap user requires an explicit force flag and is otherwise refused with a clear error.
- Hard-deleting (purging) the bootstrap user requires a separate force flag in addition to the admin-role requirement that purge already carries.

Rotating the bootstrap identity (delete the old, let a new one be created on next init or server start) is a supported deliberate operation. The two-flag pattern prevents it from happening by muscle-memory.

## Last-credential protection

A user can lock themselves out via `UnbindOIDC` if it's their only credential. The unbind handler refuses unless an explicit override flag is passed.

`RevokeAPIKey` does **not** carry this protection — revoking the last key is a normal step in a rotate-by-hand flow, and the user still has the OIDC path to recover. The asymmetry is deliberate.

## CLI shape

The CLI surfaces a subtree under `specgraph auth` for everything in this design:

- User management (list, show, change role, delete, purge).
- ServiceAccount management.
- API-key management (list, create, revoke, rotate).
- OIDC binding management (list, unbind).
- Identity introspection (`whoami`).

The exact command hierarchy is an implementation choice; the property is that all identity-related commands live under one consistent root and are discoverable via `specgraph auth --help`.

CLI credential resolution order: explicit flag → environment variable → credentials file. Implementer's call exactly how each is shaped.

## Non-goals

- Interactive bootstrap UX (e.g., a wizard); the current "init runs and prints what to do" model is preserved.
- Cross-host credential synchronization (operators manually copy/paste between machines if needed).

## Sequencing

Depends on Identity Storage for the bootstrap-admin invariant. Depends on Identity Authn for the resolution path that the minted key flows through. Touches the Policy Engine Adoption design via the policies that gate bootstrap-affecting operations (admin-role required, expressed as Cedar policies rather than handler-side flag checks).
