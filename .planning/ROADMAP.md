# Roadmap: SpecGraph

## Milestones

- 🚧 **v0.14.0 Authoring Surface Correctness** — Phases 6-9 (in progress)
- ✅ **v0.12.0 Identity & Self-Service** — Phases 1-5 (shipped 2026-07-13)

Full phase detail for shipped milestones is archived under `.planning/milestones/`.

## Phases

### v0.14.0 Authoring Surface Correctness (Phases 6-9)

Correctness fixes on existing authoring + MCP surfaces, sourced from the open GitHub backlog. Makes the authoring funnel and MCP surface trustworthy end-to-end: an MCP-only agent can learn to author from the served skills, amend/supersede match natural lifecycle semantics with working re-entry, conversations record reliably, and JIT display names self-heal.

- [x] **Phase 6: MCP Authoring Self-Teaching Path** — An MCP-only project (fresh `init`, no source/CLI) can author a constitution to completion from the served skills alone — MCP-01 (#1002, critical) (completed 2026-07-14)
- [x] **Phase 7: Authoring Lifecycle Semantics** — amend works in-flight, supersede only from done, and amend re-entry lets the target stage re-author — LIFE-01 (#900), LIFE-02 (#899) (completed 2026-07-14)
- [x] **Phase 8: Authoring Conversation Fidelity** — every authoring stage records its conversation, enforced by the protocol — CONV-01 (#906) (completed 2026-07-15)
- [ ] **Phase 9: JIT Display Name Reconciliation** — each login reconciles a JIT user's `display_name` against a usable name claim — AUTH-06 (#994)

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

## Phase Details

### Phase 6: MCP Authoring Self-Teaching Path

**Goal**: An agent in a fresh MCP-only project (created by `specgraph init` — `.mcp.json` + managed files, no source, no local CLI) can author the project constitution to completion using only `specgraph://prime` and the MCP-served skills, with no out-of-band CLI/YAML knowledge.
**Depends on**: Nothing (first phase of milestone)
**Requirements**: MCP-01 (#1002)
**Success Criteria** (what must be TRUE):

  1. The MCP-served authoring skills describe the full MCP `author`-tool round-trip (per-stage tool calls and their inputs/outputs), not just the CLI equivalents.
  2. Starting from only `specgraph://prime` in a fresh `init`-only project, an agent can discover and complete every authoring stage (Spark → Shape → Specify → Decompose → Approve) without any CLI/YAML knowledge.
  3. The constitution reaches an approved/completed state via MCP tool calls alone (no shell/CLI fallback required).
  4. `specgraph_skills_get`/`specgraph_skills_search` return authoring guidance that references the MCP tool path, verified against the embedded skill canonicals.

**Plans**: 5/5 plans complete
**Wave 1**

  - [x] 06-01-PLAN.md — `internal/authoring/load` friendly funnel YAML→proto package (TDD)
  - [x] 06-02-PLAN.md — MCP-first rewrite of all 7 embedded skills + content-reference gate
  - [x] 06-03-PLAN.md — `specgraph://prime` entry-point routing + empty-state MCP hints

**Wave 2** *(blocked on Wave 1 completion)*

  - [x] 06-04-PLAN.md — MCP write-input handler shim (constitution + funnel friendly YAML)

> **Atomic-release constraint (pass-3 review, HIGH):** 06-02 (skills teach friendly-YAML `output`/`exchanges`) and 06-04 (handlers accept that friendly YAML) MUST land in the same merge window (single PR or stacked merge, no intermediate deploy). Shipping 06-02 without 06-04 leaves a window where a live MCP-only agent hits the still-protojson handlers and reproduces #1002. This is a merge/release constraint only — it does not change the wave graph.

**Wave 3** *(blocked on Wave 2 completion)*

  - [x] 06-05-PLAN.md — MCP-only authoring e2e verification gate

### Phase 7: Authoring Lifecycle Semantics

**Goal**: amend and supersede match natural spec lifecycle semantics, and amend re-entry lets the target stage be re-authored immediately.
**Depends on**: Nothing (independent of Phase 6; sequenced after it)
**Requirements**: LIFE-01 (#900), LIFE-02 (#899)
**Success Criteria** (what must be TRUE):

  1. A user can `amend` a spec while it is in flight (`>= approved` and `< done`) and the spec returns to authoring.
  2. `supersede` is permitted only on a `done` spec and is rejected for in-flight specs.
  3. After `amend --re-entry <stage>`, the user can immediately run that stage (e.g. `shape`) without hitting an `invalid stage transition` no-op.
  4. Re-entry lands the spec at the target stage so the subsequent stage command is a valid transition.

**Plans**: 4/5 plans executed

Plans: *(linearized 1→2→3→4→5 during `--reviews` incorporation to close the claim-release-before-reroute ordering window; one plan per wave)*

**Wave 1**

- [x] 07-01-PLAN.md — Supersede reason: proto field + storage/handler threading + CLI --reason (D-06/D-07)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 07-02-PLAN.md — Release active claim on amend + storage integration tests (D-08/D-10)

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 07-03-PLAN.md — Reroute MCP author tool to LifecycleService; re_entry_stage allowlist (IsValidReEntryStage) + new_slug params + next-step hint (D-01/D-03/D-04/D-05/D-07)

**Wave 4** *(blocked on Wave 3 completion)*

- [x] 07-04-PLAN.md — Retire the divergent broken amend/supersede path; delete RPCs/handlers/storage + prune fakes (D-02)

**Wave 5** *(blocked on Wave 4 completion)*

- [x] 07-05-PLAN.md — Skills teaching + MCP-only done→amend→re-author→supersede e2e (D-09/D-10)

### Phase 8: Authoring Conversation Fidelity

**Goal**: Every authoring stage reliably records its conversation, with recording enforced by the protocol rather than left to agent discretion.
**Depends on**: Nothing (independent; sequenced after Phase 7)
**Requirements**: CONV-01 (#906)
**Success Criteria** (what must be TRUE):

  1. Running a spec through the authoring funnel via skills (shape/specify/decompose/approve) records a conversation entry for every stage.
  2. Conversation recording is enforced by the stage protocol/handler, not dependent on the agent choosing to record.
  3. A stage that reaches completion has an associated, non-empty conversation record — a missing conversation cannot silently pass.
  4. Recorded conversations are retrievable/queryable after the funnel completes.

**Plans**: 4/4 plans complete

**Wave 1** *(parallel — no file overlap)*

- [x] 08-01-PLAN.md — Proto field-3 comment + server approve-accept enforcement (validate+record under `approved`) + handler/storage integration tests (D-02/D-03)
- [x] 08-02-PLAN.md — MCP: thread required exchanges into `handleApprove`, remove `conversation` record action (keep `list`), flip author Description + SKILL.md (D-02/D-06/D-09)
- [x] 08-03-PLAN.md — CLI: shared `loadConversationFlag` (`--conversation` bare-array/stdin), rewire 5 stage commands, delete `cliSyntheticExchanges` (D-01/D-04/D-05)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 08-04-PLAN.md — MCP-only funnel conversation-fidelity e2e (positive + negative) + update existing funnel e2e (D-10)

> **Atomic-release note:** Wave-1 plans land in one merge window — the server (08-01) starts hard-rejecting approve-accept without exchanges, so the MCP (08-02) and CLI (08-03) exchange-supplying paths must ship together to avoid a broken approve path.

### Phase 9: JIT Display Name Reconciliation

**Goal**: On each successful login, a JIT-provisioned user's `display_name` is reconciled against a usable name claim, replacing a stale subject-hash fallback.
**Depends on**: Nothing (isolated identity/auth fix; sequenced last)
**Requirements**: AUTH-06 (#994)
**Success Criteria** (what must be TRUE):

  1. On login, a JIT user's `display_name` is updated when a usable `name`/`preferred_username` claim becomes available.
  2. A stale subject-hash fallback `display_name` is replaced once a usable claim appears.
  3. Reconciliation runs on every successful login, not only at first provisioning.
  4. When no usable claim is present, the existing `display_name` is preserved (no regression back to a subject-hash value).

**Plans**: 2 plans

**Wave 1**

- [ ] 09-01-PLAN.md — Extract display-name reconciliation into an unconditional `reconcileDisplayName` helper wired into `materializeIdentity`; remove the redundant block from `applyLoginSync` + update its two white-box tests (D-01/D-03/D-06)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 09-02-PLAN.md — Introspection-path `claims.Name` parity fix + `jitResolve` seed-from-`claims.Name` + per-path unit tests (introspection/JIT/oauth2) + real-Postgres reconciliation integration test (D-02/D-04/D-05/D-07)

## Backlog

### Phase 999.2: confluence integration (BACKLOG)

**Goal:** [Captured for future planning] — Confluence integration surface for SpecGraph. Home for INTG-01 (`spgr-jwbj`, the Confluence comment-polling pagination bug descoped from Phase 4 because the poller code does not live in this repo) plus the broader Confluence↔SpecGraph bridge ideas: one-way export of specs/decisions (EXPL-02, `spgr-9f6`) and the design-bridge template (`docs/designs/2026-03-26-confluence-to-specgraph-design-bridge.md`). First step when promoting: locate/confirm which repo owns the Confluence comment poller.
**Requirements:** TBD (candidates: INTG-01, EXPL-02)
**Plans:** 4/4 plans complete

Plans:

- [ ] TBD (promote with /gsd-review-backlog when ready)

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 6. MCP Authoring Self-Teaching Path | v0.14.0 | 5/5 | Complete    | 2026-07-14 |
| 7. Authoring Lifecycle Semantics | v0.14.0 | 5/5 | Complete    | 2026-07-14 |
| 8. Authoring Conversation Fidelity | v0.14.0 | 4/4 | Complete    | 2026-07-15 |
| 9. JIT Display Name Reconciliation | v0.14.0 | 0/2 | Not started | - |
| 1. Release & Build Tooling | v0.12.0 | 1/1 | Complete | 2026-07-09 |
| 2. API Key Lifecycle & Self-Service | v0.12.0 | 8/8 | Complete | 2026-07-10 |
| 3. External Identity Provider Integration | v0.12.0 | 4/4 | Complete | 2026-07-10 |
| 4. Verification & Integration Reliability | v0.12.0 | 2/2 | Complete | 2026-07-10 |
| 5. UI Project Selector & Refinements | v0.12.0 | 14/14 | Complete | 2026-07-12 |

---
*Roadmap created: 2026-07-08*
*v0.12.0 milestone archived: 2026-07-13*
*v0.14.0 roadmap added: 2026-07-13 — Phases 6-9 (MCP-01, LIFE-01, LIFE-02, CONV-01, AUTH-06)*
