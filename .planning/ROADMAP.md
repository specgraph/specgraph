# Roadmap: SpecGraph

## Milestones

- ✅ **v0.12.0 Identity & Self-Service** — Phases 1-5 (shipped 2026-07-13)

Full phase detail for shipped milestones is archived under `.planning/milestones/`.

## Phases

<details>
<summary>✅ v0.12.0 Identity & Self-Service (Phases 1-5) — SHIPPED 2026-07-13</summary>

Maintenance/hardening slice migrated from `bd`/beads: closed an in-flight release-pipeline fix, rounded out the identity/auth surface (API-key self-service, revocation enforcement, external IdP integration), added a verifiable drift-detection interface, and shipped a project-selector web UI on a full shadcn-svelte + dark-mode foundation. INTG-01 (Confluence poller) descoped — code not in this repo; re-homed to backlog 999.2.

- [x] **Phase 1: Release & Build Tooling** (1/1 plans) — completed 2026-07-09 — REL-01, CFG-01, CFG-02
- [x] **Phase 2: API Key Lifecycle & Self-Service** (8/8 plans) — completed 2026-07-10 — AUTH-02, AUTH-03
- [x] **Phase 3: External Identity Provider Integration** (4/4 plans) — completed 2026-07-10 — AUTH-01, AUTH-04, AUTH-05
- [x] **Phase 4: Verification & Integration Reliability** (2/2 plans) — completed 2026-07-10 — DRFT-01 (INTG-01 descoped)
- [x] **Phase 5: UI Project Selector & Refinements** (14/14 plans) — completed 2026-07-12 — project selector + full shadcn-svelte + dark-mode migration (promoted from backlog 999.1)

Details: `.planning/milestones/v0.12.0-ROADMAP.md` · Requirements: `.planning/milestones/v0.12.0-REQUIREMENTS.md` · Audit: `.planning/milestones/v0.12.0-MILESTONE-AUDIT.md`

</details>

## Backlog

### Phase 999.2: confluence integration (BACKLOG)

**Goal:** [Captured for future planning] — Confluence integration surface for SpecGraph. Home for INTG-01 (`spgr-jwbj`, the Confluence comment-polling pagination bug descoped from Phase 4 because the poller code does not live in this repo) plus the broader Confluence↔SpecGraph bridge ideas: one-way export of specs/decisions (EXPL-02, `spgr-9f6`) and the design-bridge template (`docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`). First step when promoting: locate/confirm which repo owns the Confluence comment poller.
**Requirements:** TBD (candidates: INTG-01, EXPL-02)
**Plans:** Not started

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Release & Build Tooling | v0.12.0 | 1/1 | Complete | 2026-07-09 |
| 2. API Key Lifecycle & Self-Service | v0.12.0 | 8/8 | Complete | 2026-07-10 |
| 3. External Identity Provider Integration | v0.12.0 | 4/4 | Complete | 2026-07-10 |
| 4. Verification & Integration Reliability | v0.12.0 | 2/2 | Complete | 2026-07-10 |
| 5. UI Project Selector & Refinements | v0.12.0 | 14/14 | Complete | 2026-07-12 |

---
*Roadmap created: 2026-07-08*
*v0.12.0 milestone archived: 2026-07-13*
