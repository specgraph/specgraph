# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v0.12.0 — Identity & Self-Service

**Shipped:** 2026-07-13
**Phases:** 5 | **Plans:** 29 | **Tasks:** 69
**Timeline:** ~5 days (2026-07-08 → 2026-07-13) | **Commits:** ~192 (53 `feat`)
**Verification:** 5/5 phases `passed` | **Requirements:** 9/10 satisfied (INTG-01 descoped)

### What Was Built
- **Self-service MCP API keys (AUTH-03):** owner-scoped create/list/rotate/revoke with a `RoleMin` role floor on mint *and* rotate, double-submit CSRF, quota/rate limits, one-time reveal — CLI self-variants + a `/keys` web panel.
- **Live role-revocation enforcement (AUTH-02):** `ResyncUserRole` seam clamps standing keys to the caller's live DB role on next request, with optional hard revoke.
- **External IdP integration (AUTH-01/04/05):** native GitHub OAuth2 login, MCP OAuth 2.1 resource server (RFC 9728 discovery, RFC 8707 audience, RFC 7662 introspection, fail-closed key-prefix guard), and `web_sessions.issuer` audit data.
- **Verifiable drift detection (DRFT-01):** real-DB proofs (no false positives, mixed-state skip counts, per-upstream ack round-trip) + API/MCP access docs.
- **Project-selector web UI:** deterministic default precedence, invalidate-on-switch re-fetch, universal `+page.ts` loads with skeleton/empty/error states, full shadcn-svelte + Tailwind v4 dark-mode migration.
- **Release/build hardening (REL-01/CFG-01/CFG-02):** single-job goreleaser release, koanf layered config, golangci-lint pinned to one Taskfile source of truth read back by CI.

### What Worked
- **Wave-based execution on the big UI phase (Phase 5, 14 plans / 3 waves)** kept a foundation-first order — primitives + tokens landed in Wave 1 so every later component migration had live styling to consume.
- **CONTEXT-decision scope contracts (D-01..D-14)** substituted cleanly for formal REQ-IDs on the UI phase, giving verification concrete anchors without inventing requirements.
- **Early codebase scouting caught a phantom requirement:** Phase 4 discuss-phase found the Confluence poller (INTG-01) simply doesn't live in this repo — descoping it *before* planning avoided wasted work.
- **Structural fixes over point fixes:** CFG-02 pinned the tool version at a single Taskfile source read by both local and CI, eliminating a whole class of drift rather than one instance.

### What Was Inefficient
- **Auto-generated MILESTONES.md accomplishments needed hand-curation** — several summary one-liners were deviation-log lines (`[Rule 3 - Blocking]…`) rather than user-facing accomplishments; the raw dump had to be trimmed to ~6.
- **Phase 5 required a gap-closure plan (05-14)** after verification found the Spec/Decision detail bodies still on the old light-only palette — the dark-mode migration wasn't fully caught until VERIFICATION.md flagged it.
- **STATE.md field drift:** the milestone.complete CLI couldn't match one STATE.md field and left `milestone_name: milestone` / `current_phase: 12.0` artifacts needing manual correction.

### Patterns Established
- **Live-role read as the revocation mechanism** — standing keys carry no cached privilege; every request re-derives the floor. Reuse for any future capability that must revoke instantly.
- **Additive, path-scoped protocol upgrades** — RFC 8707/7662 checks fire only under `WithMCPRequest`, leaving ConnectRPC/web-login semantics untouched. The template for evolving one auth surface without regressing the others.
- **shadcn-svelte manual-fallback install** — the CLI blocks on an interactive preset and lacks a `slate` base-color, so `components.json`/`app.css`/`utils.ts` are authored by hand and Slate ships as a verified OKLCH token block.
- **Layout owns the single active-project breadcrumb; pages re-suspend via `+page.ts` load + `invalidateAll()`** — end-to-end project-switch re-fetch with no per-page stale-guards.

### Key Lessons
1. **Scout the code before planning integration requirements.** INTG-01 assumed the poller lived here; a 20-minute scan turned a "phase requirement" into a clean descope routed to the backlog.
2. **Verify against the acceptance property, not the plan checklist.** Phase 5's dark-mode truth needed a dedicated gap-closure plan because component migration ≠ readable dark mode on every surface.
3. **Fix drift at its source.** One Taskfile var beats N synchronized copies — the CFG-02 pattern is worth generalizing (flagged IN-01: apply to `PROTOC_GEN_*`).

### Cost Observations
- Model mix: planner Opus / executor Sonnet (per `.planning/config.json` profile).
- Notable: the 14-plan UI phase dominated effort; wave batching kept context bounded despite the volume.

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v0.12.0 | 5 | 29 | First full milestone tracked in GSD `.planning/` (migrated off `bd`/beads); wave-based execution proven on a 14-plan UI phase |

### Cumulative Quality

| Milestone | Phases Verified | Requirements Satisfied | Descoped |
|-----------|-----------------|------------------------|----------|
| v0.12.0 | 5/5 passed | 9/10 | INTG-01 (code not in repo) |

### Top Lessons (Verified Across Milestones)

1. Scout the codebase before committing a requirement to a phase — assumptions about where code lives are cheap to check and expensive to plan around.
2. Verify against the user-facing acceptance property, not the task list.
