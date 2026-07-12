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
- [x] **Phase 4: Verification & Integration Reliability** - Drift detection gets a verified interface (INTG-01 descoped — Confluence poller not in this repo) (completed 2026-07-10)
- [ ] **Phase 5: UI Project Selector & Refinements** - Web UI gains a project selector with a sensible default, plus project-specific UI refinements (constitution view, etc.) and a full shadcn-svelte + dark-mode migration — promoted from backlog 999.1 (13 plans, 3 waves)

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

**Plans**: 2/2 plans complete

Plans:

- [x] 04-01-PLAN.md — DRFT-01 SC#2 verification tests (no-false-positive e2e, full-graph SkippedCount integration, per-upstream ack round-trip e2e)
- [x] 04-02-PLAN.md — DRFT-01 SC#1 doc note: drift interface reachable via API (`LifecycleService.CheckDrift`/`AcknowledgeDrift`) and MCP `drift` tool

### Phase 5: UI Project Selector & Refinements

**Goal**: The web UI lets users select which project they're viewing (with a sensible default) and surfaces project-specific views/refinements (constitution, etc.) instead of assuming a single implicit project
**Depends on**: Nothing (promoted from backlog 999.1; builds on the existing SvelteKit web app + ConnectRPC surface)
**Requirements**: D-01..D-14 (defined in 05-CONTEXT.md; no formal REQ-IDs — CONTEXT decisions are the scope contract)
**Status**: Planned — 13 plans across 3 waves (2026-07-10)
**Success Criteria** (what must be TRUE):

  1. A user can pick the active project from the UI, with a sensible default (last-used → `default` → alpha-first), and switching re-fetches every project-scoped view (D-01..D-08)
  2. Project-scoped views (dashboard, graph, constitution, spec/decision detail) reflect the selected project with correct empty/error states and an active-project indicator; constitution badges re-derive across switches (D-09/D-10/D-11)
  3. The full web UI is migrated to shadcn-svelte (Tailwind v4, Slate theme) with light/dark mode (D-12/D-13/D-14)

**Plans**: 2/13 plans executed

Plans:
**Wave 1**

- [x] 05-01-PLAN.md — Wave 1: shadcn-svelte + Tailwind v4 foundation, Slate tokens, primitive set, badge-variants map
- [x] 05-02-PLAN.md — Wave 1: project store default precedence + case-insensitive sort + stale fallback (D-04/05/06) + tests

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 05-03-PLAN.md — Wave 2: app shell — +layout.ts load bootstrap, shadcn nav/selector/switch, active-project breadcrumb, dark mode (D-01/02/03/07/08/11/14)
- [ ] 05-04-PLAN.md — Wave 2: migrate AccordionSection, TabBar, FindingsSection
- [ ] 05-05-PLAN.md — Wave 2: migrate SpecTable, StatsBar, FunnelBar
- [ ] 05-06-PLAN.md — Wave 2: migrate SearchFilter, MetadataBar, ChangelogTimeline
- [ ] 05-07-PLAN.md — Wave 2: migrate LoginModal, RevealKeyModal
- [ ] 05-08-PLAN.md — Wave 2: migrate DiffView, VersionCompare
- [ ] 05-09-PLAN.md — Wave 2: reframe Graph, GraphMini on Card + theme tokens

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 05-10-PLAN.md — Wave 3: load-ify Dashboard + Graph pages with skeleton/empty/error states (D-01/02/09)
- [ ] 05-11-PLAN.md — Wave 3: load-ify Spec + Decision detail pages (D-01/02/09)
- [ ] 05-12-PLAN.md — Wave 3: Keys page shadcn restyle (user-scoped, no load refactor — D-09)
- [ ] 05-13-PLAN.md — Wave 3: Constitution polish — load-ify, provenance-derived badges, empty state (D-10)

## Backlog

### Phase 999.2: confluence integration (BACKLOG)

**Goal:** [Captured for future planning] — Confluence integration surface for SpecGraph. Home for INTG-01 (`spgr-jwbj`, the Confluence comment-polling pagination bug descoped from Phase 4 because the poller code does not live in this repo) plus the broader Confluence↔SpecGraph bridge ideas: one-way export of specs/decisions (EXPL-02, `spgr-9f6`) and the design-bridge template (`docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`). First step when promoting: locate/confirm which repo owns the Confluence comment poller.
**Requirements:** TBD (candidates: INTG-01, EXPL-02)
**Plans:** 0 plans

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Release & Build Tooling | 1/1 | Complete    | 2026-07-09 |
| 2. API Key Lifecycle & Self-Service | 8/8 | Complete    | 2026-07-10 |
| 3. External Identity Provider Integration | 4/4 | Complete    | 2026-07-10 |
| 4. Verification & Integration Reliability | 2/2 | Complete    | 2026-07-10 |
| 5. UI Project Selector & Refinements | 2/13 | In Progress|  |

---
*Roadmap created: 2026-07-08*
*Granularity: Standard (4-6 phases) — no `.planning/config.json` present, defaults applied*
*Phase ID convention: sequential — no `.planning/config.json` present, defaults applied*
