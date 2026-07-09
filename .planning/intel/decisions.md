# Decisions Intel

Synthesized from all 14 ADR-classified documents in the full 177-document ingest corpus
(supersedes the prior 50-doc-only synthesis). Every entry is preserved separately — no
merging. See `INGEST-CONFLICTS.md` for contradictions found between entries.

This corpus contains three populations of ADR-typed documents:

- **5 domain-architecture ADRs (LOCKED)** — `docs/decisions/ADR-00{1,2,4,5,6}-*.md`. These govern
  the spec-graph data model and runtime semantics and are the load-bearing decisions for
  downstream planning.
- **5 process/tooling ADRs (LOCKED)** — under `docs/superpowers/specs/`, classified as ADR by the
  classifier's Status/Decision-section heuristic but governing release engineering and repo
  housekeeping, not the domain model. Kept in a clearly separate section below so a downstream
  reader does not conflate "10 locked ADRs" with "10 locked domain decisions." See
  `INGEST-CONFLICTS.md` INFO entry.
- **4 draft/proposed ADRs (NOT locked)** — early design-session drafts plus one CLI-design
  adjustment note. Retained for provenance/history.

---

## Domain-Architecture ADRs (LOCKED)

### ADR-001: Use 'statement' instead of 'principle' for Principle proto field

- **source:** `docs/decisions/ADR-001-principle-statement-field-naming.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-01
- **scope:** Principle proto message; constitution schema; Go structs; emitter; CLI show; YAML bootstrap; storage
- **decision:** Name the Principle proto field (field 2) `statement`, not `principle`. Avoids `Principle.principle` tautology; matches the YAML struct `ConstitutionPrinciple.Statement`.
- **supersedes:** `docs/plans/2026-02-28-slice-2-constitution-plan.md`'s Principle message definition (used the old `principle` field name) and, independently, `docs/plans/2026-02-28-vertical-slice-roadmap-design.md`, which also carries the stale `principle` field name in its Slice 2 proto description. Pre-v1, no persisted data, no migration needed. Auto-resolved, ADR wins — see `INGEST-CONFLICTS.md` INFO entry.

### ADR-002: Stable ULID IDs with Murmur3-128 Content Hash

- **source:** `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-18
- **scope:** Spec; Decision; ULID; content_hash; drift detection; sync adapters; graph edges
- **decision:**
  1. Keep ULIDs as stable node IDs for Spec (`spec-{ULID}`) and Decision (`dec-{ULID}`), assigned once at creation, never regenerated.
  2. Add a `content_hash` field (Murmur3-128, 32 hex chars) computed from substantive fields, recomputed on every create/update.
  3. Hash inputs (Spec): intent, stage, priority, complexity, all stage outputs. Hash inputs (Decision): title, status, decision, rationale.
  4. Hash excludes: id, slug, version, timestamps, history, supersession fields, notes, lifecycle, drift-ack fields.
- **explicitly rejected alternative:** "Content-addressable IDs (hash as the id)" — rejected because ID changes on every edit would break graph edges (DEPENDS_ON, BLOCKS, COMPOSES).
- **supersedes:** The implicit "content-addressable ID" convention in earlier proto comments/site docs, and — on the narrow point of Decision ID scheme only — the non-locked draft ADR-003's `d-{sha256(title)[:8]}` identity scheme (see draft ADR-003 below). Because ADR-002 is LOCKED and draft ADR-003 is `Proposed`/non-locked, this is auto-resolved per standard precedence (LOCKED wins over non-LOCKED) rather than a gating conflict — see `INGEST-CONFLICTS.md` INFO entry. The rest of draft ADR-003 (lifecycle, schema fields, cross-spec referencing) is unaffected and was implemented faithfully downstream (see `2026-03-31-decision-adr003-fields-design.md` in `constraints.md`).
- **consequences:** content_hash feeds ChangeLog nodes (`2026-03-18-changelog-graph-nodes-design.md`) and drift detection (`content_hash_at_link` on DEPENDS_ON edges, per `2026-03-19` content-hash-drift-detection design referenced in the ADR text).

### ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths

- **source:** `docs/decisions/ADR-004-optimistic-concurrency-transactions.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-19
- **scope:** RunInTransaction; ConnectRPC server; spec mutation; ChangeLog; version guards; ErrConcurrentModification
- **decision:** Wrap all multi-query write paths in `RunInTransaction` for atomic rollback. Keep version guards (`WHERE version = $expected`) for conflict detection. First writer wins; second receives `ErrConcurrentModification` → `CodeAborted` (retryable).
- **consequences:** All new multi-query write paths must use `RunInTransaction`. No distributed locking needed (single-server). Nested `RunInTransaction` calls reuse the outer transaction.
- **companion implementation spec:** `docs/superpowers/specs/2026-03-19-transaction-wrapped-write-paths-design.md` — content matches, no divergence.
- **note (awareness, not a contradiction):** The identity/auth subsystem's `AuthStore` (introduced much later, global rather than project-scoped, per `docs/plans/2026-05-22-identity-storage-design.md`) is a structurally separate store not wired into `*Store.RunInTransaction`. Later specs (`2026-06-15-cli-oidc-login-design.md`, `2026-06-16-spgr-g7st-self-service-api-keys-design.md`) implement bespoke `pool.Begin`-based atomic transactions directly on `AuthStore` rather than literally reusing `RunInTransaction`. This preserves ADR-004's atomicity *principle* but not its named *mechanism* for the identity domain — flagged for awareness only.

### ADR-005: No Native Windows Support — WSL Required

- **source:** `docs/decisions/ADR-005-no-native-windows-support.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-30
- **scope:** file locking; file permissions; Docker integration; shell scripts; Windows support; WSL
- **decision:** SpecGraph does not support native Windows. Windows users must use WSL. `lock_windows.go` retained only for cross-compilation (no-op stub with warning log).
- **consequences:** No Windows-specific CI/tests. No Windows-specific code paths beyond compilation stubs. Documentation must note WSL requirement.

### ADR-006: Spec Provenance Model

- **source:** `docs/decisions/ADR-006-spec-provenance-model.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-05-20
- **scope:** SpecLifecycle (removed); SpecProvenance (added); GetReady; drift detection; claim; report-completion; proto schema
- **decision:** Replace `SpecLifecycle` (task/living, vestigial post-`done`) with `SpecProvenance` — an enum for how a spec entered the graph: `AUTHORED`, `RETROACTIVE_FROM_PR`, `DECLARED`. Structured payload via `provenance_detail` oneof (proto fields 22–24). Stage drives funnel behavior; provenance drives forward-vs-imported axis.
- **consequences:** Wire-break at proto field 10 (pre-1.0, no production data — clean break, no migration). `specgraph ready` tightened to `stage=approved AND provenance=AUTHORED` with no active claim. `claim`/`report-completion` reject non-AUTHORED specs (`ErrClaimRequiresAuthored`/`ErrCompletionRequiresAuthored`). Drift detection unified on `stage=done` regardless of provenance. Provenance immutable through amend; supersede creates a fresh spec with fresh provenance.
- **full design:** `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md` (see `constraints.md`) — consistent expansion, not a competing decision.
- **supersedes:** The `lifecycle` (`task`/`living`) field used throughout earlier docs, including the Postgres schema DDL in `2026-04-01-postgres-storage-backend-design.md` (`lifecycle TEXT NOT NULL DEFAULT 'task'`) and the amend/supersede eligibility framing in `2026-04-08-lifecycle-nomenclature-inversion-design.md`. Sequential correction, not a simultaneous contradiction — auto-resolved, ADR wins, noted in `INGEST-CONFLICTS.md`.

---

## Process/Tooling ADRs (LOCKED — release engineering & repo housekeeping, not domain architecture)

### Release Infrastructure: release-please + goreleaser + cocogito

- **source:** `docs/superpowers/specs/2026-03-20-release-please-goreleaser-design.md`
- **status:** Approved — LOCKED (per doc); **superseded in practice** by the 2026-03-26 migration below (self-declared via that doc's `Supersedes:` field).
- **decision (as originally made):** Use release-please for versioning/changelog, goreleaser for builds/distribution, cocogito for commit validation.
- **current status:** No longer the active decision.

### Release Tooling Migration: git-cliff + goreleaser v2

- **source:** `docs/superpowers/specs/2026-03-26-release-tooling-migration-design.md`
- **status:** Approved — LOCKED; explicitly **Supersedes:** `2026-03-20-release-please-goreleaser-design.md`.
- **decision (as originally made):** Replace release-please with git-cliff for version computation/changelog; migrate goreleaser to v2 APIs; `workflow_dispatch` manual trigger instead of automatic release-on-push.
- **current status:** No longer the active decision. Its "Deprecated → New API Keys" equivalence table (`release.use_existing_draft: true` ⇔ `release.mode: append`) was later found to be factually false — the defect this introduced is the Problem statement of the next entry.

### Release Pipeline: Single-Job, GoReleaser-Owns-Release

- **source:** `docs/superpowers/specs/2026-06-05-release-single-job-goreleaser-design.md`
- **status:** Approved — LOCKED; explicitly **Supersedes:** `2026-03-26-release-tooling-migration-design.md`.
- **decision:** GoReleaser is sole owner of GitHub Releases; removes git-cliff and the two-job draft architecture that caused every published release since v0.3.7 (through v0.7.0) to have empty release notes — root cause: two independent release-creation paths for the same tag that don't coordinate (a `gh release create --draft` path and a separate `goreleaser release` path that can't see draft releases via the GitHub API).
- **This is the currently active release-pipeline decision** in the corpus.

**Note on this three-doc chain:** All three carry `Status: Approved` so the classifier marks all `locked: true`. Read together they form a self-declared linear supersession chain (each later doc's own `Supersedes:` field names its predecessor), not three simultaneously-contradicting locked ADRs. Treated as a chain (latest = active), not a BLOCKER — see `INGEST-CONFLICTS.md` INFO entry.

### Repository Organization Move

- **source:** `docs/superpowers/specs/2026-03-20-repo-org-move-design.md`
- **status:** Approved — LOCKED
- **decision:** Transfer repo to `github.com/specgraph/specgraph`; Go module path becomes `github.com/specgraph/specgraph`; site domain `specgraph.io`. Two-phase: (1) manual GitHub org transfer, (2) automated PR updating ~165 files' import paths.
- **scope:** go.mod, proto `go_package` options, site config, Docker/GHCR references. Independent scope — no contradiction with any other locked ADR.

### Idempotent Push: FindOrCreate for Sync Adapters

- **source:** `docs/superpowers/specs/2026-03-26-idempotent-push-design.md`
- **status:** Approved — LOCKED
- **decision:** Add `FindOrCreate(ctx, spec) (externalID string, created bool, err error)` to the sync `Adapter` interface. Sync handler calls `FindOrCreate` instead of `Push` to prevent orphaned duplicate external items when `CreateSyncMapping` fails after a successful `Push`. `Push` remains on the interface for backward compatibility.
- **scope:** `internal/sync/adapter.go`, `GitHubAdapter`, `BeadsAdapter`, `internal/server/sync_handler.go`. Independent scope — no contradiction with any other locked ADR.

---

## Draft / Proposed ADRs (NOT locked — historical/provenance context)

### SpecGraph ADR-001 (v3, draft): Architecture & Storage

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md`
- **status:** Draft — no explicit Accepted/Proposed status field found; filename marks it "draft" — **not locked**
- **decision (as originally proposed):** "Composable, not coupled" — one mandatory core + exactly ONE of two storage backends (Beads+Dolt OR Postgres+AGE), with Gastown/tracker-sync/tool-injection as independent optional integrations. No configuration is "degraded."
- **superseded status:** Explicitly and formally superseded. `docs/plans/2026-02-28-client-server-architecture-design.md` states in its header "**Supersedes:** Initial design session documents (v1.0-draft)" and in its closing section: "This design supersedes the architectural and storage decisions in: ... `specgraph-v1.0-draft-adr-001-storage.md` — entirely (new storage model)." See `INGEST-CONFLICTS.md` (INFO) for the full storage-backend lineage (Beads+Dolt/Postgres+AGE → Memgraph+Postgres+AGE → pure Postgres/pgx, no AGE, no Memgraph, no Beads-as-backend — the last of these is the backend reflected in current `CLAUDE.md`).

### SpecGraph ADR-002 (draft): Gastown Integration

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-002-gastown.md`
- **status:** No explicit Status field / no Decision-Consequences sections found — **not locked**
- **decision (as originally proposed):** SpecGraph specs become Beads that Gastown natively reads; Gastown integration requires the Beads(+Dolt) backend; the Postgres path has no native Gastown integration (needs an adapter or manual execution) — an explicit, stated tradeoff, not a detected conflict.
- **superseded status:** `docs/plans/2026-02-28-client-server-architecture-design.md` explicitly supersedes "the integration model (Beads sync adapter replaces native coupling)" — Beads is demoted from a candidate core storage backend to a push-only sync adapter alongside issue trackers. The Gastown-as-optional-integration concept survives (matches current `CLAUDE.md`: "SpecGraph is upstream of Gastown — SpecGraph does design; Gastown does execution via polecats"); the Beads-as-data-plane mechanism does not.

### SpecGraph ADR-003 (draft): Decisions as First-Class Graph Entities

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md`
- **status:** Proposed — **not locked**
- **decision (as originally proposed):** Decisions become first-class graph nodes with bidirectional edges to specs (`decided_in`, `references`). Decision identity: Beads path → the bead ID; Postgres path → a short hash of the normalized title (`d-` + first 8 hex chars of `sha256(normalize(title))`), with collisions on identical titles treated as intentional deduplication signals.
- **conflict on ID scheme only:** Contradicted by LOCKED ADR-002, which keeps Decision IDs as stable ULIDs (`dec-{ULID}`) and explicitly rejects content-addressable/hash-based IDs. Per standard precedence (LOCKED beats non-locked), this is auto-resolved with ADR-002 winning — logged as INFO in `INGEST-CONFLICTS.md`, not a WARNING, since there is no genuine ambiguity requiring a user pick.
- **everything else in this doc is not contradicted:** decision lifecycle (proposed→accepted→deprecated/superseded), schema fields, and cross-spec referencing UX are implemented faithfully downstream — `docs/plans/2026-03-06-storage-domain-types-design.md` promotes ShapeOutput decisions to first-class Decision graph nodes with `DECIDED_IN` edges, and `docs/superpowers/specs/2026-03-31-decision-adr003-fields-design.md` adds the ADR-003 schema fields (question, rejected_alternatives, confidence, tags, scope, origin_spec, origin_stage) to the Decision domain type/proto. Also directly referenced in current `CLAUDE.md`: "Decisions are first-class nodes (ADR-003) with bidirectional edges to specs" and the DECIDED_IN edge-direction gotcha (spec → decision, per ADR-003).

### Task 32 Design Adjustment: `specgraph read-mcp-resource`

- **source:** `docs/plans/2026-04-27-task-32-read-mcp-resource-design.md`
- **status:** Not locked (design-adjustment note, no Status/Accepted marker).
- **decision:** Corrects an obsolete plan assumption (stdio MCP transport, `specgraph mcp read-resource` parent command) after PR #923 removed stdio transport entirely. New design: top-level `specgraph read-mcp-resource <uri>` command, HTTP transport via `mark3labs/mcp-go`, plain-text stdout output, no new env vars/flags (env-var unification deferred to koanf adoption, tracked as `spgr-5kd5`).
- **supersedes:** The relevant Task 32 section of `docs/plans/2026-04-20-multi-platform-plugin-plan.md` (line 2953) — self-declared, non-locked correction, no blocker.
- **confirmed implemented:** matches current `CLAUDE.md`'s description of the `specgraph read-mcp-resource` CLI subcommand and its session-start hook usage exactly, and `docs/plans/2026-04-27-task-32-read-mcp-resource-plan.md` is the matching companion implementation plan (no divergence).

---

## Cross-cutting notes

- No LOCKED-vs-LOCKED contradiction exists in this corpus once the release-pipeline trio is read as a self-declared supersession sequence rather than three independent decisions (see `INGEST-CONFLICTS.md`).
- The 5 domain-architecture ADRs (001, 002, 004, 005, 006) do not overlap in scope with each other or with the 5 process/tooling ADRs — no contradictions detected among any locked pair.
- Two auto-resolved ADR-over-DOC/SPEC precedence cases are logged: ADR-001 over the Slice 2 constitution plan's (and the vertical-slice-roadmap doc's) `principle` field name, and ADR-002 over earlier "content-addressable ID" language in the v1.0 draft spec, draft ADR-001, and draft ADR-003's Decision-ID scheme.
