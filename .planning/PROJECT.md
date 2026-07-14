# SpecGraph

## What This Is

SpecGraph is a Live Spec-Driven Development Framework — specifications as a queryable graph, not static markdown. It provides the constitution (project ground truth), a spec schema, an authoring funnel (Spark → Shape → Specify → Decompose → Approve), and a storage + query layer that feeds execution engines like Gastown, which does the actual work via ephemeral polecat agents.

## Core Value

Specs stay live and queryable as a graph — with locked architectural decisions, drift detection, and a durable storage/query layer — so both humans and agent-based execution engines can trust the spec graph as ground truth instead of static, decaying markdown.

## Current Milestone: v0.14.0 Authoring Surface Correctness

**Goal:** Make the authoring funnel and MCP surface trustworthy end-to-end — an MCP-only agent can learn to author from the served skills, amend/supersede match natural lifecycle semantics with working re-entry, conversations record reliably, and JIT display names self-heal.

**Target features (correctness fixes on existing surfaces, from the open GitHub backlog):**

- MCP-served skills teach the MCP authoring round-trip, not just the CLI, so a fresh `init`-only project can author from scratch (#1002, critical)
- amend/supersede lifecycle semantics corrected — amend in-flight, supersede only from done (#900, high)
- amend re-entry allows re-authoring at the target stage (#899, high)
- authoring stages reliably record conversations (#906, medium)
- JIT reconciles `display_name` against a usable name claim on each login (#994, medium)

Version aligns to the cog-managed release line (v0.13.0 released → v0.14.0 next). Phase numbering continues from v0.12.0's Phase 5 (next milestone starts at Phase 6).

## Current State

**Shipped:** v0.12.0 Identity & Self-Service (5 phases, 29 plans, 69 tasks — closed 2026-07-13). Rounded out the identity/auth surface (self-service MCP API keys, live role-revocation enforcement, native GitHub OAuth2 + MCP OAuth 2.1 resource server, session-issuer audit), hardened release/build tooling, added a verifiable drift-detection interface, and shipped a project-selector web UI on a full shadcn-svelte + dark-mode foundation. Archived under `.planning/milestones/v0.12.0-*`.

**Open threads for the next milestone:**

- INTG-01 (Confluence comment-polling pagination bug, `spgr-jwbj`) — descoped from v0.12.0; parked in backlog **Phase 999.2 (Confluence Integration)** alongside EXPL-02 (one-way Confluence export). First step on promotion: locate the repo that owns the poller.
- Candidate v2 requirements are catalogued in `.planning/milestones/v0.12.0-REQUIREMENTS.md` (§ v2 Requirements): REL-02, CFG-03..05, DRFT-02, DEC-01..04, HRNS-01..03, DX-01..02, SCALE-01..03, EXPL-01..05, INTG-02, UI-01..02.

Fresh requirements for the next milestone are defined via `/gsd-new-milestone`.

## Requirements

### Validated

<!-- Shipped and confirmed valuable, per closed beads history + the docs/ corpus ingest. -->

- ✓ Spec schema, constitution (layered User → Org → Project → Domain), storage (Postgres/pgx, pgvector) — Phase 1 Foundation
- ✓ Authoring funnel (Spark → Shape → Specify → Decompose → Approve), analytical pass system, decisions as first-class graph nodes (ADR-003 fields) — Phase 2
- ✓ CLI + MCP server; skills served via `specgraph_skills_list/get/search` (no on-disk skill copies)
- ✓ Harness plugin parity: Claude Code, Cursor, OpenCode (`specgraph init` writes per-harness shims)
- ✓ OIDC login (CLI `specgraph login`/`logout`, web UI), OIDC app-roles + login-sync
- ✓ Storage backend migration: Memgraph+AGE → pure Postgres/pgx (Memgraph fully removed)
- ✓ v0.12.0 released
- ✓ Single-job goreleaser-owns-release model (`spgr-7r6g`) — merged PR #981; verified against `v0.12.0`'s actual GitHub Release (single publish, populated notes, signed/SBOM'd assets)
- ✓ Koanf layered config loader (`spgr-5kd5`) — `internal/config/global.go` implements the full flag>env>file>default precedence, including the `SPECGRAPH_PG_URL` deprecation warning
- ✓ Pin `task tools`' golangci-lint to match CI version (`spgr-vpmg`) — Phase 1: single `GOLANGCI_LINT_VERSION` var in `Taskfile.yml`, installed via `go install` (not unpinned `brew install`); `ci.yml` reads the same value via `$(task tools:golangci-lint-version)` command substitution instead of an independent env var
- ✓ Self-service / auto MCP API-key provisioning for OIDC users (`spgr-g7st`, AUTH-03) — Phase 2: owner-scoped self-mint/list/rotate/revoke handlers with RoleMin floor (create+rotate), quota-safe mint, expiry clamp (90d/180d), CSRF double-submit, anti-key-chaining, plus CLI self-variants and a `/keys` web dashboard
- ✓ Enforce app-role revocation on standing API/MCP keys (`spgr-c2lb`, AUTH-02) — Phase 2: operator `ResyncUserRole` RPC + `auth user resync` CLI writes the live DB role (clamping standing keys on next request) with optional `--revoke-keys` hard off-board; proven by live-floor integration tests
- ✓ Native generic OAuth2 + userinfo login provider (GitHub-direct) (`spgr-1rq9`, AUTH-01) — Phase 3: `oauth2LoginProvider` (Exchange→userinfo→`*OIDCClaims`, verified-email fallback, stable numeric subject) reusing the OIDC binding/JIT/claims-mapping machinery via a single canonical `ProviderIssuer` helper; live GitHub browser login verified
- ✓ MCP OAuth 2.1 resource server delegating auth to a real IdP (`spgr-tmqm`, AUTH-04) — Phase 3: RFC 9728 protected-resource metadata + `/mcp/`-scoped `WWW-Authenticate` challenge, RFC 8707 resource-URI audience binding (MCP-request-gated), RFC 7662 opaque-token introspection with multi-IdP trial and `spgr_sk_` guard-before-introspection; dev/prod https policy (RS disabled on http loopback)
- ✓ Populate `web_sessions.issuer` for audit / future RP-logout (`spgr-bbp2`, AUTH-05) — Phase 3: `Identity.Issuer` threaded through `materializeIdentity`; callback resolves via `ResolveLogin` and stamps the session issuer before `CreateSession`; verified by Docker integration test (no backfill of pre-existing empty-issuer rows, D-10)
- ✓ Interface and verify drift detection (`spgr-vch`, DRFT-01) — Phase 4: drift status reachable via a stable interface (`LifecycleService.CheckDrift`/`AcknowledgeDrift`, MCP `drift` tool), verified against real content-hash + DEPENDS_ON-edge scenarios by no-false-positive e2e, full-graph SkippedCount integration, and per-upstream ack round-trip tests (INTG-01 Confluence bug descoped — poller not in this repo)
- ✓ Web UI project selector + shadcn-svelte/dark-mode migration (D-01..D-14) — Phase 5: active-project store with default precedence (last-used → `default` → alpha-first), skeleton-on-switch across every project-scoped view (dashboard/graph/spec/decision/constitution) with correct empty/error states and provenance-derived constitution badges; full UI migrated to shadcn-svelte (Tailwind v4, Slate OKLCH tokens) with light/dark mode
- ✓ Amend/supersede lifecycle semantics corrected (LIFE-01, #900) — Phase 7: amend restricted to in-flight specs (`approved`/`in_progress`/`review`) returning to authoring, supersede permitted only from `done`; single-source-of-truth `IsValidReEntryStage` allowlist enforced at both handler and storage; active claim + CLAIMED_BY edge released inside the amend transaction (stale-lease fix); MCP `author` rerouted to `LifecycleService` and the divergent broken authoring amend/supersede path retired
- ✓ Amend re-entry re-authors the target stage (LIFE-02, #899) — Phase 7: `amend --re-entry <stage>` lands the spec one stage before the target (`PrecedingAuthStage`) so the subsequent stage command is a valid transition, eliminating the same-stage no-op; proven by an MCP-only e2e that amends to `shape`, lands at `spark`, and re-runs `shape` successfully

### Active

<!-- v0.14.0 scope — see REQUIREMENTS.md for full detail with REQ-IDs. Sourced from the open GitHub backlog (issue-first). -->

- [ ] **MCP-01**: MCP-served skills teach the MCP authoring round-trip so an MCP-only project can author from scratch (#1002)
- [ ] **CONV-01**: authoring stages reliably record conversations across the funnel (#906)
- [ ] **AUTH-06**: JIT reconciles `display_name` against a usable name claim on each login (#994)

**Deferred (not in this milestone):**

- Fix Confluence comment polling pagination bug (`spgr-jwbj` / #901) — backlog Phase 999.2; poller code is not in this repo (INTG-01 descoped from Phase 4), first step is to locate its owning repository

### Out of Scope

- Memgraph authentication/TLS hardening (`spgr-fn3`) — obsolete; the storage backend fully migrated to pure Postgres/pgx and Memgraph was removed entirely (see storage-backend lineage in `.planning/intel/context.md`). Never formally closed in beads after the migration landed; flagged for closure there, not carried into GSD tracking.

## Context

SpecGraph is a mature, already-shipped Go monorepo (tagged through v0.12.0). Issue tracking previously ran on `bd`/beads; this project is migrating tracking to GSD's `.planning/` artifacts. 793 historical beads issues were reviewed during migration — 757 closed (historical/completed work, not replayed here since `.planning/intel/decisions.md` and `constraints.md` already capture the substantive architectural history from the `docs/` corpus), 36 open/in-progress/deferred (folded into Active requirements above, minus one stale item moved to Out of Scope).

Full architectural history — locked ADRs, three-generation storage-backend lineage (Beads+Dolt/AGE draft → Memgraph+AGE → pure Postgres/pgx), the SpecLifecycle → SpecProvenance field replacement, and the harness/plugin delivery model's evolution — is distilled from 177 ingested `docs/` files into `.planning/intel/{decisions,constraints,context,SYNTHESIS}.md`.

## Constraints

- **Tech stack**: Go; ConnectRPC (not plain gRPC); pgx v5 native driver; PostgreSQL with pgvector — no Memgraph/graph-DB dependency
- **Platform**: No native Windows support, WSL required — ADR-005
- **Concurrency**: All multi-query write paths must use `RunInTransaction` — ADR-004
- **IDs**: Decision node IDs are stable ULIDs; content-addressable (hash-based) IDs were explicitly evaluated and rejected — ADR-002
- **Compliance**: All commits require a DCO `Signed-off-by:` trailer
- **Tracking**: `bd`/beads Dolt backend is being retired in favor of GSD `.planning/` — do not reintroduce beads-only workflows for new planning artifacts

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| ADR-001: Principle field renamed `principle` → `statement` | naming clarity | ✓ Good |
| ADR-002: Stable ULID decision IDs (not content-hash) | edge-reference stability under renames | ✓ Good |
| ADR-004: Optimistic concurrency via `RunInTransaction` | consistency for multi-query writes | ✓ Good |
| ADR-005: No native Windows support | WSL sufficient; avoids cross-platform burden | ✓ Good |
| ADR-006: `SpecProvenance` replaces `SpecLifecycle` (task/living) | task/living distinction proved insufficient | ✓ Good |
| Storage backend: pure Postgres/pgx (not Memgraph+AGE) | simplify ops, drop graph-DB dependency | ✓ Good |
| Migrate issue tracking from `bd`/beads to GSD `.planning/` | consolidate on one planning/tracking system | — Pending |
| CFG-02: Taskfile-as-source-of-truth for pinned tool versions (silent leaf task + CI command substitution, not a duplicated env var) | single declaration closes local/CI version drift structurally, not just for golangci-lint | ✓ Good — pattern flagged in code review (IN-01) as worth generalizing to `PROTOC_GEN_*` vars in a future phase |
| Phase 5: manual-fallback shadcn install + Slate via OKLCH token block | shadcn-svelte CLI blocks on an interactive preset prompt and its base-color enum has no `slate`, so `components.json`/`app.css`/`utils.ts` are authored by hand and Slate is delivered as a verified OKLCH block | ✓ Good |
| Phase 5: layout owns the single active-project breadcrumb; pages re-suspend to Skeleton via `+page.ts` load + `invalidateAll()` on project switch | prevents per-page breadcrumb duplication and gives end-to-end switch re-fetch without manual stale-guards | ✓ Good |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-07-14 — Phase 7 (Authoring Lifecycle Semantics) complete: LIFE-01 + LIFE-02 validated (amend in-flight, supersede done-only, re-entry re-authors the target stage). Next: Phase 8 Authoring Conversation Fidelity (CONV-01).*
