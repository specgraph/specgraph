# Context Intel

Running notes keyed by topic, appended with source attribution. Sourced from
the single `DOC`-classified document in this ingest batch, plus cross-cutting
provenance notes gathered while synthesizing `decisions.md` and
`constraints.md` that don't belong in either (pure narrative/lineage
context, not a decision or a constraint in their own right).

---

## SpecGraph Implementation Roadmap (original v1.0-draft phase plan)

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-roadmap.md`
- **date:** 2025-02-25 (companion to the v1.0-draft spec)
- **status:** Superseded as a roadmap by `docs/plans/2026-02-28-client-server-architecture-design.md` §8 "Revised Roadmap" (explicitly stated in that doc's closing section) and further superseded in practice by `docs/plans/2026-02-28-vertical-slice-roadmap-design.md`'s slice-based plan. Retained here as historical framing only.
- **content:** Four phases — Foundation (spec schema, constitution, Beads+Postgres backends, claim protocol, bundle format, core CLI, linter, migration), Authoring (codebase scanner, authoring flow, CLI agent integration, Claude Code plugin), Coordination/Export/Integration (lease model, MCP server, drift detection, ADR/doc export, Gastown, tracker sync, Apache AGE), Scale (federation, multi-repo, metrics, governance). "Highest-leverage" starting items per the original plan: spec schema, constitution, execution bundle format, core CLI, Claude Code skills. This four-phase structure is echoed (with different content per phase, reflecting the pivot away from Beads/AGE) in the current `CLAUDE.md` "Roadmap" section (Phase 1 Foundation → Phase 2 Authoring & CLI Integration → Phase 3 Coordination & Export → Phase 4 Scale) — the phase *names* persisted across the full rewrite even though nearly every phase *item* changed.

---

## Cross-cutting lineage notes (not sourced from a single DOC, compiled while reading the full corpus)

### Storage backend: three-generation evolution
1. **Gen 1** (`docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md`, historical): dual-path — Beads(+Dolt) OR Postgres(+AGE), operator picks one, "no degraded mode."
2. **Gen 2** (`docs/plans/2026-02-28-client-server-architecture-design.md`, 2026-02-28): Memgraph is the default; Postgres+AGE is the pluggable alternative (AGE required, no CTE fallback). Beads demoted to a push-only sync adapter. `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (same day) narrows further: "Backend: Memgraph only. Postgres+AGE deferred to a future effort."
3. **Gen 3** (`docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md`, 2026-04-01, current): pure Postgres/pgx, **no Memgraph, no AGE, no ltree** — "graph queries are viable in SQL," recursive CTEs sufficient at project scale. This is the backend reflected in the current `CLAUDE.md` (`internal/storage/postgres/`, pgx v5, recursive CTEs, testcontainers with `pgvector/pgvector:pg18`).

Each generation transition is **self-documented as an explicit supersession** in the superseding document's own text (not left implicit) — see `INGEST-CONFLICTS.md` for why this lineage is reported as INFO rather than as a WARNING/BLOCKER despite ADR > SPEC default precedence nominally favoring the oldest (draft-ADR) generation.

### Spec lifecycle field: `lifecycle` (task/living) → `provenance` (AUTHORED/RETROACTIVE_FROM_PR/DECLARED)
The `lifecycle` field (values `task`/`living`) appears in the Postgres schema DDL in `2026-04-01-postgres-storage-backend-design.md` (`lifecycle TEXT NOT NULL DEFAULT 'task'`) and in the original v1.0-draft spec's full schema example (`status: draft|approved|in-progress|review|done|amended|superseded|abandoned` alongside a separate lifecycle-like axis). ADR-006 (2026-05-20, locked) removes this column entirely in favor of `SpecProvenance`. Any requirement or plan referencing `lifecycle: living` / `lifecycle: task` post-2026-05-20 is stale.

### Amend/supersede eligibility: two eligibility tables exist across the corpus
`docs/superpowers/specs/2026-04-06-lifecycle-amendment-supersede-design.md` (Draft, 2026-04-06) assumes amend works on a **completed/done** spec ("Returning a completed spec to an earlier authoring stage"). `docs/superpowers/specs/2026-04-08-lifecycle-nomenclature-inversion-design.md` (Approved, 2026-04-08 — two days later) **inverts** this: amend is only for **in-flight** specs (`approved`/`in_progress`/`review`); supersede is only for **done** specs. The 2026-04-06 doc's substantive contribution (diff engine, CLI `--diff`, web changelog UI) is unaffected by the eligibility flip and remains current; only its worked-example prose about *when* amend applies is stale.

### Superseded/retired documents referenced but not present in this ingest batch
Several docs mention companion or predecessor designs that were **not** part of this 50-document classification set (so they cannot be cross-checked directly, only noted):
- `spgr-qe74` "Self-Service Authz design" — explicitly stated as "retired" and superseded by the Identity Policy Engine (Cedar) design.
- `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md` — referenced by the Spec Provenance Model design as the source of the original (buggy) `GetReady` intent.
- `spgr-tmqm` ("MCP OAuth resource server") and `spgr-1rq9` ("generic OAuth2 provider") and `spgr-c2lb` ("role-revocation latency") — repeatedly referenced as forward-looking follow-on work by the identity/auth docs; described but not designed in this batch.
- `docs/plans/2026-02-28-slice-2-constitution-plan.md` — the doc ADR-001 (field naming) explicitly supersedes; not in this ingest batch (only the vertical-slice-roadmap doc, which independently carries the same stale `principle` field-name reference, was classified).

### Skill-personas doc self-declares superseded status
`docs/plans/2026-03-17-skill-personas-design.md` carries an explicit header: "**Status:** Superseded by [2026-04-20-multi-platform-plugin-design.md] and [2026-05-06-harness-parity-epic-design.md]." Neither successor is a fresh document class here — the harness-parity epic *is* in this ingest batch; `2026-04-20-multi-platform-plugin-design.md` is referenced but not classified in this batch.

### Harness/plugin delivery mechanism: three-generation evolution
1. **Gen 1** (`2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md`): three separate per-harness MCP config files, hand-maintained, later synced by `specgraph init` via JSON Merge Patch.
2. **Gen 2** (`2026-05-06-harness-parity-epic-design.md`): adds in-tree `skills/` directory + per-harness dev-time symlinks (`plugin/<harness>/skills -> ../../skills`) — explicitly a dev-time-only artifact.
3. **Gen 3** (`2026-05-08-spgr-rwrp-harness-install-parity-design.md` + its PR-lettered children): embed-and-write via `//go:embed`, sentinel-hash drift detection, skills served exclusively via MCP resource fetch (`specgraph://skills/<name>`) with **zero on-disk skill files** for end users. Gen 1's `mcpconfigs/` package and Gen 2's dev-time symlinks are both explicitly folded in and deleted during Gen 3's rollout (PR B, PR F).

### Identity epic sequencing (see also INGEST-CONFLICTS.md for the cross-ref-cycle note)
All five identity/authz docs (`identity-storage-design`, `identity-authn-design`, `identity-bootstrap-ux-design`, `identity-policy-engine-design`, plus the later OIDC/self-service docs) are part of one named epic ("Identity, RBAC & Audit", `spgr-rjrt`) with an explicit linear sequencing stated in each doc's own "Sequencing" section: **Storage is foundational** → **Authn** depends on Storage → **Bootstrap & UX** depends on Storage+Authn → **Policy Engine (Cedar)** sits alongside and supersedes part of Authn (the "Permission computation" section) and the separately-retired Self-Service Authz design. Later docs (interactive OIDC login, CLI OIDC login, app-roles/login-sync, self-service API keys) build on top of this foundation in date order and are internally consistent with it.
