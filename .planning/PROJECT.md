# SpecGraph

## What This Is

SpecGraph is a Live Spec-Driven Development Framework — specifications as a queryable graph, not static markdown. It provides the constitution (project ground truth), a spec schema, an authoring funnel (Spark → Shape → Specify → Decompose → Approve), and a storage + query layer that feeds execution engines like Gastown, which does the actual work via ephemeral polecat agents.

## Core Value

Specs stay live and queryable as a graph — with locked architectural decisions, drift detection, and a durable storage/query layer — so both humans and agent-based execution engines can trust the spec graph as ground truth instead of static, decaying markdown.

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

### Active

<!-- v1 scope — see REQUIREMENTS.md for full detail with REQ-IDs. Sourced from currently open/in-progress beads issues, P1+P2 priority. -->

- [ ] Fix Confluence comment polling pagination bug (`spgr-jwbj`) — deferred to backlog Phase 999.2; poller code is not in this repo (INTG-01 descoped from Phase 4), first step is to locate its owning repository

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

---
*Last updated: 2026-07-13 after Phase 5 complete — UI project selector + shadcn/dark-mode migration (D-01..D-14) shipped and UAT-passed; also reconciled Phase 4 (DRFT-01 drift detection moved Active→Validated) and marked the Confluence pagination bug (`spgr-jwbj`) deferred to backlog Phase 999.2. All five v1 phases complete.*
