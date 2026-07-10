# Roadmap: SpecGraph

## Overview

This roadmap covers the current v1 slice of work migrated from `bd`/beads into GSD: closing
out an in-flight release-pipeline fix, rounding out the identity/auth surface (API-key
self-service, revocation enforcement, and external IdP integration), and two smaller
reliability items (a verifiable drift-detection interface and a Confluence pagination bug).
This is maintenance/hardening work on an already-shipped, mature product (through v0.12.0) —
not a greenfield build. Two items are already underway per beads status (REL-01, AUTH-03) and
their phases start "In progress" rather than "Not started."

The four phases group by natural subsystem boundary: build/release tooling, MCP API-key
lifecycle, external identity-provider integration, and detection/integration reliability.
Phases 1, 2, and 4 have no dependencies on each other and could in principle be worked in any
order; Phase 3 is sequenced after Phase 2 to avoid churning the identity subsystem twice.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Release & Build Tooling** - Ship the single-job goreleaser release pipeline and centralize config/lint tooling (completed 2026-07-09)
- [x] **Phase 2: API Key Lifecycle & Self-Service** - OIDC users self-provision MCP API keys; revoked roles can't survive on standing keys (completed 2026-07-10)
- [x] **Phase 3: External Identity Provider Integration** - Add native GitHub OAuth2, MCP OAuth 2.1 resource-server delegation, and session-issuer audit data
- [ ] **Phase 4: Verification & Integration Reliability** - Drift detection gets a verified interface (INTG-01 descoped — Confluence poller not in this repo)

## Phase Details

### Phase 1: Release & Build Tooling

**Goal**: Maintainers can cut a tagged release and trust the build/lint tooling without manual intervention or double-published/broken artifacts
**Depends on**: Nothing (first phase)
**Requirements**: REL-01, CFG-01, CFG-02
**Status**: Complete — REL-01 and CFG-01 confirmed already shipped on `main` (PR #981 and the koanf loader in `internal/config/global.go`; beads status had lagged actual repo state); CFG-02 closed by 01-01-PLAN.md
**Success Criteria** (what must be TRUE):

  1. ✓ A pushed release tag produces exactly one coherent GitHub Release (correct changelog notes and artifacts) via a single goreleaser-owned job — no dual-path race, no empty release notes. **Met** — verified against `v0.12.0`'s actual release.
  2. ✓ All SpecGraph server/CLI config is sourced through one layered koanf loader (flag > env > file > default), with the legacy `SPECGRAPH_PG_URL` env var emitting a deprecation warning instead of silently breaking config. **Met** — verified in `internal/config/global.go` and `cmd/specgraph/serve.go`.
  3. ✓ `task check`'s golangci-lint run uses the same pinned version as CI, so a clean local `task check` guarantees a clean CI lint run. **Met** — `Taskfile.yml` pins `GOLANGCI_LINT_VERSION: v2.12.1` and installs it via `go install`; `ci.yml`'s "Install Go tools" step reads that same value via `$(task tools:golangci-lint-version)`.

**Plans**: 1/1 plans complete

- [x] 01-01-PLAN.md — Pin `task tools`' golangci-lint to CI's version via a single Taskfile.yml var read back by ci.yml (CFG-02; REL-01/CFG-01 traceability-only, already shipped)

### Phase 2: API Key Lifecycle & Self-Service

**Goal**: OIDC users can safely self-provision scoped MCP API keys, and a revoked app-role can no longer be exploited via an already-issued key
**Depends on**: Nothing (independent of Phase 1; builds on already-shipped Identity Storage/Authn/Cedar/login-sync foundations — see `.planning/intel/decisions.md` and `constraints.md`)
**Requirements**: AUTH-02, AUTH-03
**Status**: In progress — AUTH-03 (`spgr-g7st`) is already underway per beads; AUTH-02 has not started
**Success Criteria** (what must be TRUE):

  1. An authenticated OIDC user can create, list, rotate, and revoke their own role-capped, expiring MCP API key without borrowing an admin's bootstrap key
  2. A self-minted key's effective role is capped at the caller's own current role at mint/rotate time — no privilege-escalation "laundering" through a stale or elevated role
  3. When a user's app role is revoked or downgraded upstream, their standing API/MCP keys stop carrying the old privilege on forced re-sync, not only on next interactive login

**Plans**: 8/8 plans complete
**Wave 1**

- [x] 02-01-PLAN.md — Proto & codegen: five IdentityService RPCs (4 self + ResyncUserRole) + task proto (AUTH-02, AUTH-03)
- [x] 02-02-PLAN.md — Storage: owner-scoped mutations + quota-safe mint + ErrQuotaExceeded + integration tests (AUTH-03)
- [x] 02-03-PLAN.md — Server foundation: self-service key-policy config (90d/180d/quota 10) + double-submit CSRF middleware (validate + issue-on-whoami-GET) (AUTH-03)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 02-04-PLAN.md — Auth: exported RoleMin floor + apikey.self Cedar verb + action map + mirror/drift tests (AUTH-03, AUTH-02)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 02-05-PLAN.md — Server: four self-mint handlers with source-gate + RoleMin floor + rate limit + expiry cap + CSRF-validator mount + ErrQuotaExceeded→ResourceExhausted (AUTH-03)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 02-06-PLAN.md — AUTH-02 forced re-sync: ResyncUserRole seam + `auth user resync --revoke-keys` CLI + standing-key live-floor integration test (AUTH-02)
- [x] 02-07-PLAN.md — CLI: self-variants of auth api-key + session-preferring resolver (Finding D) (AUTH-03)
- [x] 02-08-PLAN.md — Web: MCP Keys dashboard panel + one-time reveal modal + CSRF echo (AUTH-03)

### Phase 3: External Identity Provider Integration

**Goal**: SpecGraph authenticates users and MCP clients against real external identity providers, with enough audit metadata to support session audit and future RP-initiated logout
**Depends on**: Phase 2
**Requirements**: AUTH-01, AUTH-04, AUTH-05
**Success Criteria** (what must be TRUE):

  1. A user can log in via a native GitHub OAuth2 + userinfo flow (no Entra/Okta broker required), using the same session model as existing OIDC providers
  2. An MCP client can authenticate to SpecGraph's MCP server via a standard OAuth 2.1 resource-server flow, with token validation delegated to the configured external IdP rather than a SpecGraph-issued API key
  3. Every web session record stores which issuer authenticated it, so an operator can audit login-provider usage per session and a future RP-initiated logout can target the correct issuer

**Plans**: 4/4 plans executed

**Wave 1**

- [x] 03-01-PLAN.md — Identity-materialization seam (Exchange→*OIDCClaims, shared materializeIdentity, ResolveLogin) + AUTH-05 session issuer population (AUTH-01, AUTH-05)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 03-02-PLAN.md — AUTH-01 native oauth2 + userinfo login provider (GitHub-direct): kind gate relax + oauth2LoginProvider + verified-email fallback (AUTH-01)
- [x] 03-03-PLAN.md — AUTH-04 protocol surface: RFC 9728 protected-resource metadata + WWW-Authenticate challenge scoped to /mcp/ (AUTH-04)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 03-04-PLAN.md — AUTH-04 token validation: RFC 8707 resource-URI audience binding + RFC 7662 opaque-token introspection (AUTH-04)

### Phase 4: Verification & Integration Reliability

**Goal**: Maintainers can trust that reported drift signals are correct and verifiable
**Depends on**: Nothing
**Requirements**: DRFT-01
**Success Criteria** (what must be TRUE):

  1. Drift status for any spec (or the full graph) is queryable through a stable, documented interface (CLI/API/MCP), not only inferable by reading code
  2. The drift interface is verified against real content-hash and DEPENDS_ON-edge scenarios — a test suite (or equivalent verification) confirms it flags true drift and doesn't false-positive on unrelated edits

**INTG-01 descoped (2026-07-10):** Phase 4 discuss-phase scouting found the Confluence comment-polling code (`spgr-jwbj`) does not exist anywhere in this repository — no adapter, poller, or pagination code; `internal/sync/` has only beads + github adapters. The bug targets a separate system/repo. INTG-01 is removed from this phase pending identification of its owning repository; see `04-CONTEXT.md` D-05. It should either be re-homed to that repo's planning or formally deferred.

**Plans**: 2 plans

Plans:

- [x] 04-01-PLAN.md — DRFT-01 SC#2 verification tests (no-false-positive e2e, full-graph SkippedCount integration, per-upstream ack round-trip e2e)
- [ ] 04-02-PLAN.md — DRFT-01 SC#1 doc note: drift interface reachable via API (`LifecycleService.CheckDrift`/`AcknowledgeDrift`) and MCP `drift` tool

## Backlog

### Phase 999.1: UI needs a project selector, and default, as well as UI refinements around project specifics ( constitution, etc ) (BACKLOG)

**Goal:** [Captured for future planning]
**Requirements:** TBD
**Plans:** 4/4 plans complete

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Release & Build Tooling | 1/1 | Complete    | 2026-07-09 |
| 2. API Key Lifecycle & Self-Service | 8/8 | Complete    | 2026-07-10 |
| 3. External Identity Provider Integration | 4/4 | Complete    | 2026-07-10 |
| 4. Verification & Integration Reliability | 0/TBD | Not started | - |

---
*Roadmap created: 2026-07-08*
*Granularity: Standard (4-6 phases) — no `.planning/config.json` present, defaults applied*
*Phase ID convention: sequential — no `.planning/config.json` present, defaults applied*
