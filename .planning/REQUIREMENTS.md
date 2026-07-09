# Requirements: SpecGraph

**Defined:** 2026-07-08
**Core Value:** Specs stay live and queryable as a graph — with locked architectural decisions, drift detection, and a durable storage/query layer — so both humans and agent-based execution engines can trust the spec graph as ground truth instead of static, decaying markdown.

**Source:** Migrated from `bd`/beads (793 historical issues reviewed; 757 closed/historical, 36 open/in-progress/deferred). Each requirement below carries its originating beads ID for traceability back to the retired tracker.

## v1 Requirements

Sourced from beads issues at priority P1 (in-progress) and P2 (open). Each maps to a roadmap phase once `gsd-roadmapper` runs.

### Release

- [x] **REL-01**: Adopt the holomush single-job goreleaser-owns-release model (`spgr-7r6g`) — done: PR #981 merged; verified against `v0.12.0`'s actual GitHub Release

### Auth & Identity

- [ ] **AUTH-01**: Native generic OAuth2 + userinfo login provider (GitHub-direct) (`spgr-1rq9`)
- [x] **AUTH-02**: Enforce app-role revocation on standing API/MCP keys, forcing re-sync (`spgr-c2lb`)
- [x] **AUTH-03**: Self-service / automatic MCP API-key provisioning for OIDC users (`spgr-g7st`, in progress)
- [ ] **AUTH-04**: MCP OAuth 2.1 resource server delegating auth to a real IdP (`spgr-tmqm`)
- [ ] **AUTH-05**: Populate `web_sessions.issuer` for audit / future RP-logout (`spgr-bbp2`)

### Config & Build

- [x] **CFG-01**: Adopt koanf for layered config + env provider (`spgr-5kd5`) — done: `internal/config/global.go` implements full precedence + deprecation warning
- [x] **CFG-02**: Pin task tools' golangci-lint to match the CI version (`spgr-vpmg`) — done: `task tools` now installs via `go install` at the Taskfile.yml-pinned version; CI reads that same value via `$(task tools:golangci-lint-version)`

### Drift Detection

- [ ] **DRFT-01**: Interface and verify drift detection (`spgr-vch`)

### Integrations

- [ ] **INTG-01**: Fix Confluence comment polling pagination bug (`spgr-jwbj`)

## v2 Requirements

Deferred to a future release. Sourced from beads issues at priority P3/P4. Not in the current roadmap.

### Release

- **REL-02**: Verify `dist/digests.txt` is produced by `dockers_v2` for container provenance attestation (`spgr-7r6g.2`)

### Config & Build

- **CFG-03**: Add load-time validation for publish config fields (`spgr-4w34`)
- **CFG-04**: Harden `task clean` against pnpm symlink deletion flakes on macOS (`spgr-rt2m`)
- **CFG-05**: PR E follow-up — delete migration oracles + refactor cleanups (`spgr-vncb`)

### Drift Detection

- **DRFT-02**: Code-level drift detection — watch for repo/code changes outside a spec (`spgr-93k`)

### Decisions & Changelog

- **DEC-01**: Add `clear_tags`/`clear_rejected_alternatives` flags to `UpdateDecisionRequest` (`spgr-c0q`)
- **DEC-02**: `ListChanges`/`ListAllChanges` support for Decision changelog entries (`spgr-dwh`)
- **DEC-03**: ChangeLog compaction for long-lived specs (`spgr-jhn`)
- **DEC-04**: History truncation/archival for spec nodes (`spgr-qaw`)

### Harness Distribution

- **HRNS-01**: Publish the Claude Code plugin to the marketplace (`spgr-eo4n`)
- **HRNS-02**: Publish the OpenCode plugin to npm as `@specgraph/opencode-plugin` (`spgr-sa95`)
- **HRNS-03**: Add a Codex MCP config + shim (`spgr-uds0`)

### Workspace & DX

- **DX-01**: Per-workspace XDG isolation to prevent shared-volume footguns across jj workspaces (`spgr-xbsf`)
- **DX-02**: Emit fenced-code language in generated `cli-reference.md` (`spgr-7lyu`)

### Scale & Hardening

- **SCALE-01**: Scaling — pagination, rate limiting, batch APIs, and archival for large graphs (`spgr-g6p`, deferred)
- **SCALE-02**: Narrow handler signatures from `ScopedBackend` to specific interfaces (`spgr-dec.17`, deferred)
- **SCALE-03**: `StoreDecomposeOutput` DEPENDS_ON edge with non-sibling slug not tested (`spgr-6n2`, deferred)

### Design Explorations

- **EXPL-01**: Design vector store integration for semantic search (`spgr-1nk`)
- **EXPL-02**: Design one-way export to Confluence for specs, decisions, and project artifacts (`spgr-9f6`)
- **EXPL-03**: Design chat integration (Slack/Discord) for authoring + issue-tracker handoff (`spgr-n5b`)
- **EXPL-04**: Design import of specs from external spec systems (SpecKit, BMAD, OpenSpec, Superpowers) (`spgr-n95`)
- **EXPL-05**: Write an ADR or plan doc for early deployment profile work (`spgr-c6g`)

### Integrations

- **INTG-02**: Gastown spec version tracking integration (`spgr-edb`)

### UI

- **UI-01**: Web UI syntax highlighting and code block rendering for spec content (`spgr-2pk`)
- **UI-02**: Reconsider `SpecView.blockers` shape if blockers gain resolution state (`spgr-to31`)

## Out of Scope

| Feature | Reason |
|---------|--------|
| Memgraph authentication/TLS hardening (`spgr-fn3`) | Obsolete — storage backend fully migrated to Postgres/pgx; Memgraph removed entirely. Never formally closed in beads after the migration landed. |
| Replaying all 757 closed beads issues into GSD history | Redundant — `.planning/intel/decisions.md` and `constraints.md` already capture the substantive architectural history from the 177-doc `docs/` corpus ingest at a more useful level of abstraction. |

## Traceability

Populated by `gsd-roadmapper` during roadmap creation. See `.planning/ROADMAP.md` for full phase
detail (goals, success criteria, dependencies).

| Requirement | Phase | Status |
|-------------|-------|--------|
| REL-01 | Phase 1 | Done |
| AUTH-01 | Phase 3 | Pending |
| AUTH-02 | Phase 2 | Complete |
| AUTH-03 | Phase 2 | In progress |
| AUTH-04 | Phase 3 | Pending |
| AUTH-05 | Phase 3 | Pending |
| CFG-01 | Phase 1 | Done |
| CFG-02 | Phase 1 | Complete |
| DRFT-01 | Phase 4 | Pending |
| INTG-01 | Phase 4 | Pending |

**Coverage:**

- v1 requirements: 10 total
- Mapped to phases: 10/10 ✓
- Unmapped: 0 ✓

**Phase summary:**

- Phase 1 — Release & Build Tooling: REL-01, CFG-01, CFG-02
- Phase 2 — API Key Lifecycle & Self-Service: AUTH-02, AUTH-03
- Phase 3 — External Identity Provider Integration: AUTH-01, AUTH-04, AUTH-05
- Phase 4 — Verification & Integration Reliability: DRFT-01, INTG-01

---
*Requirements defined: 2026-07-08*
*Last updated: 2026-07-08 during Phase 1 discuss — REL-01 and CFG-01 found already shipped on `main` (verified against actual repo state, not just beads status); marked Done*
