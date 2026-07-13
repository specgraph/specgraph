# Milestones

## v0.12.0 Identity & Self-Service (Shipped: 2026-07-13)

**Phases completed:** 5 phases, 29 plans, 69 tasks
**Closeout:** override_closeout — all 5 in-repo phases verified (`VERIFICATION.md = passed`); 9/10 v1 requirements satisfied, INTG-01 descoped (see Known Gaps).
**Delivered:** Rounded out SpecGraph's identity/auth surface (self-service MCP API keys, live role-revocation enforcement, external IdP integration), hardened release/build tooling, added a verifiable drift-detection interface, and shipped a project-selector web UI on a full shadcn-svelte + dark-mode foundation.

**Key accomplishments:**

- **Release & build tooling (REL-01, CFG-01, CFG-02):** Single-job goreleaser-owned release pipeline (verified against the actual `v0.12.0` GitHub Release), layered koanf config loader (flag > env > file > default, with legacy `SPECGRAPH_PG_URL` deprecation warning), and golangci-lint pinned to one Taskfile.yml source of truth read back by CI — closing local/CI lint drift.
- **Self-service MCP API keys (AUTH-03):** OIDC users can create/list/rotate/revoke their own role-capped, expiring MCP keys via CLI and a net-new SvelteKit "MCP Keys" panel — `RoleMin` role floor on mint *and* rotate, double-submit CSRF gate, per-identity rate limit + quota, redacted audit logging, one-time plaintext reveal.
- **Role-revocation enforcement (AUTH-02):** `ResyncUserRole` seam clamps every standing key to the caller's live role on its next request (via the live-role read) and hard-revokes active keys on demand — proven at both the storage primitive and the RPC seam.
- **External identity providers (AUTH-01, AUTH-04, AUTH-05):** Native GitHub OAuth2 login, an MCP OAuth 2.1 resource server (RFC 9728 metadata discovery, RFC 8707 resource-audience assertion, bounded RFC 7662 introspection for opaque tokens, fail-closed `spgr_sk_` dispatch guard), and `web_sessions.issuer` populated for session audit / future RP-logout.
- **Verifiable drift detection (DRFT-01):** Three real-DB drift proofs (no false positive on unrelated edits, full-graph mixed-state skip counts, per-upstream acknowledge round-trip) plus API/MCP access documentation — no engine/proto/schema change.
- **Project-selector web UI + shadcn migration:** Deterministic project loader with three-tier default precedence and stale-fallback, invalidate-on-switch project Select with `X-Specgraph-Project` header propagation, universal `+page.ts` `load()` for every view (skeleton/empty/error states), and a full shadcn-svelte + Tailwind v4 (CSS-first) migration with Slate OKLCH light/dark tokens replacing all scoped CSS and hardcoded hex.

### Known Gaps

Milestone closed as `override_closeout` with the following documented, non-blocking gaps (per `milestones/v0.12.0-MILESTONE-AUDIT.md`, status `tech_debt`):

- **INTG-01** — Fix Confluence comment-polling pagination bug (`spgr-jwbj`). **Descoped from Phase 4:** the target Confluence poller/adapter code does not exist anywhere in this repository (integration checker confirmed no orphaned/half-wired code). Re-homed to backlog **Phase 999.2 (Confluence Integration)**. Not closeable via an in-repo closure phase — first step on promotion is to locate the owning repository.

**Tech debt carried forward (code hygiene, non-blocking):**

- `internal/auth/context.go:103-104` — `WithMCPRequest` doc comment is stale (says "unconsumed today"); the marker *is* consumed at `identitystore.go:505`. Doc drift only; code correctly wired.
- `cmd/specgraph/nudge.go:47` — `"confluence": true` sits in the drift-nudge skip allow-list but no `confluence` cobra command is registered. Harmless pre-emptive entry; remove or wire when the subcommand lands.
- Nyquist validation (informational, capability not enabled): `VALIDATION.md` `nyquist_compliant` is `false` for Phases 01/04/05. Discovery-only, not a blocker. Run `/gsd-validate-phase <N>` if formal coverage is desired later.

**Note:** git tag `v0.12.0` already existed (the release shipped as GitHub Release #993, "interactive OIDC login for the web UI") before this milestone close, so no new tag was created.

---
