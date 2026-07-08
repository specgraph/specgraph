# Decisions Intel

Synthesized from 8 ADR-classified documents (5 real Accepted ADRs in `docs/decisions/`,
3 draft ADRs from `docs/initial-design-session/`). Every entry is preserved separately —
no merging. See `INGEST-CONFLICTS.md` for contradictions found between entries.

---

## ADR-001: Use 'statement' instead of 'principle' for Principle proto field

- **source:** `docs/decisions/ADR-001-principle-statement-field-naming.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-01
- **scope:** Principle proto message; constitution schema; Go structs; emitter; CLI show; YAML bootstrap; storage
- **decision:** Name the Principle proto field (field 2) `statement`, not `principle`. Avoids `Principle.principle` tautology; matches the YAML struct `ConstitutionPrinciple.Statement`.
- **supersedes:** `docs/plans/2026-02-28-slice-2-constitution-plan.md`'s Principle message definition (used the old `principle` field name — pre-v1, no persisted data, no migration needed).

## ADR-002: Stable ULID IDs with Murmur3-128 Content Hash

- **source:** `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-18
- **scope:** Spec; Decision; ULID; content_hash; drift detection; sync adapters; graph edges
- **decision:**
  1. Keep ULIDs as stable node IDs for Spec (`spec-{ULID}`) and Decision (`dec-{ULID}`), assigned once at creation, never regenerated.
  2. Add a `content_hash` field (Murmur3-128, 32 hex chars) computed from substantive fields, recomputed on every create/update.
  3. Hash inputs (Spec): intent, stage, priority, complexity, all stage outputs. Hash inputs (Decision): title, status, decision, rationale.
  4. Hash excludes: id, slug, version, timestamps, history, supersession fields, notes, lifecycle, drift-ack fields.
- **explicitly rejected alternative:** "Content-addressable IDs (hash as the id)" — rejected because ID changes on every edit would break graph edges (DEPENDS_ON, BLOCKS, COMPOSES). See `INGEST-CONFLICTS.md` — this directly contradicts draft-adr-003's decision-ID scheme below.
- **consequences:** content_hash feeds ChangeLog nodes (2026-03-18 changelog-graph-nodes design) and drift detection (`content_hash_at_link` on DEPENDS_ON edges).

## ADR-004: Optimistic Concurrency with Transaction-Wrapped Write Paths

- **source:** `docs/decisions/ADR-004-optimistic-concurrency-transactions.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-19
- **scope:** RunInTransaction; ConnectRPC server; spec mutation; ChangeLog; version guards; ErrConcurrentModification
- **decision:** Wrap all multi-query write paths in `RunInTransaction` for atomic rollback. Keep version guards (`WHERE version = $expected`) for conflict detection. First writer wins; second receives `ErrConcurrentModification` → `CodeAborted` (retryable).
- **consequences:** All *new* multi-query write paths must use `RunInTransaction`. No distributed locking needed (single-server). Nested `RunInTransaction` calls reuse the outer transaction.
- **note (see INGEST-CONFLICTS.md, INFO):** The identity/auth subsystem's `AuthStore` (introduced later, global not project-scoped per the Identity Storage design) is a structurally separate store not wired into `*Store.RunInTransaction`. Later specs (CLI OIDC Login, Self-Service API Keys) had to implement bespoke `pool.Begin`-based atomic transactions on `AuthStore` directly rather than literally reusing `RunInTransaction`. This preserves ADR-004's atomicity *principle* but not its named *mechanism* for the identity domain — flagged for awareness, not a contradiction requiring resolution.

## ADR-005: No Native Windows Support — WSL Required

- **source:** `docs/decisions/ADR-005-no-native-windows-support.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-03-30
- **scope:** file locking; file permissions; Docker integration; shell scripts; Windows support; WSL
- **decision:** SpecGraph does not support native Windows. Windows users must use WSL. `lock_windows.go` retained only for cross-compilation (no-op stub with warning log).
- **consequences:** No Windows-specific CI/tests. No Windows-specific code paths beyond compilation stubs. Documentation must note WSL requirement.

## ADR-006: Spec Provenance Model

- **source:** `docs/decisions/ADR-006-spec-provenance-model.md`
- **status:** Accepted — **LOCKED**
- **date:** 2026-05-20
- **scope:** SpecLifecycle (removed); SpecProvenance (added); GetReady; drift detection; claim; report-completion; proto schema
- **decision:** Replace `SpecLifecycle` (task/living, vestigial post-`done`) with `SpecProvenance` — an enum for **how a spec entered the graph**: `AUTHORED`, `RETROACTIVE_FROM_PR`, `DECLARED`. Structured payload via `provenance_detail` oneof (proto fields 22–24). Stage drives funnel behavior; provenance drives forward-vs-imported axis.
- **consequences:** Wire-break at proto field 10 (pre-1.0, no production data — clean break, no migration). `specgraph ready` tightened to `stage=approved AND provenance=AUTHORED` with no active claim. `claim`/`report-completion` reject non-AUTHORED specs (`ErrClaimRequiresAuthored`/`ErrCompletionRequiresAuthored`). Drift detection unified on `stage=done` regardless of provenance. Provenance immutable through amend; supersede creates a fresh spec with fresh provenance.
- **full design:** `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md` (see constraints.md).

---

## Draft ADR-001 (v3): Architecture & Storage — HISTORICAL, NOT LOCKED

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md`
- **status:** Draft — no explicit Accepted/Proposed status field found; filename marks it "draft" — **not locked**
- **confidence:** medium
- **scope:** storage backend; Beads+Dolt; Postgres+AGE; Gastown integration; issue tracker sync; tool injection; spec schema; authoring funnel
- **decision (as originally proposed):** "Composable, not coupled" — one mandatory core + exactly ONE of two storage backends (Beads+Dolt OR Postgres+AGE), with Gastown/tracker-sync/tool-injection as independent optional integrations. No configuration is "degraded."
- **superseded status:** Explicitly and formally superseded. `docs/plans/2026-02-28-client-server-architecture-design.md` states in its header "**Supersedes:** Initial design session documents (v1.0-draft)" and in its closing section: "This design supersedes the architectural and storage decisions in: ... `specgraph-v1.0-draft-adr-001-storage.md` — entirely (new storage model)." See `INGEST-CONFLICTS.md` (INFO) for the full storage-backend lineage (Beads+Dolt/Postgres+AGE → Memgraph+Postgres+AGE → pure Postgres/pgx, no AGE, no Memgraph, no Beads-as-backend).

## Draft ADR-002: Gastown Integration — HISTORICAL, NOT LOCKED

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-002-gastown.md`
- **status:** No explicit Status field / no Decision-Consequences sections found — **not locked**
- **confidence:** medium
- **scope:** Gastown; Beads; Dolt; SpecGraph; authoring funnel; polecats; Mayor; Refinery
- **decision (as originally proposed):** SpecGraph specs become Beads that Gastown natively reads; Gastown integration requires the Beads(+Dolt) backend; the Postgres path has no native Gastown integration (needs an adapter or manual execution).
- **superseded status:** `docs/plans/2026-02-28-client-server-architecture-design.md` explicitly supersedes "the integration model (Beads sync adapter replaces native coupling)" — Beads is demoted from a candidate core storage backend to a push-only sync adapter alongside issue trackers. Gastown-as-an-optional-integration concept survives; the Beads-as-data-plane mechanism does not.

## Draft ADR-003: Decisions as First-Class Graph Entities — HISTORICAL, NOT LOCKED

- **source:** `docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md`
- **status:** Proposed — **not locked**
- **confidence:** high
- **scope:** decisions; spec graph; ADR export; decision schema; Beads; Postgres; spec-decision edges
- **decision (as originally proposed):** Decisions become first-class graph nodes with bidirectional edges to specs (`decided_in`, `references`). **Decision identity: Beads path → the bead ID; Postgres path → a short hash of the normalized title (`d-` + first 8 hex chars of `sha256(normalize(title))`), with collisions on identical titles treated as *intentional* deduplication signals.**
- **conflict:** Directly contradicted by locked ADR-002, which keeps Decision IDs as stable ULIDs (`dec-{ULID}`) and explicitly rejects content-addressable/hash-based IDs. See `INGEST-CONFLICTS.md` [WARNING] — flagged per explicit ingest instructions rather than silently resolved by default LOCKED-wins precedence, because this draft was never formally marked Superseded and its content-addressable dedup mechanism could otherwise leak into downstream requirements uncorrected.
- **status of everything else in this doc:** the rest of draft-adr-003 (decision lifecycle: proposed→accepted→deprecated/superseded; decision schema fields; cross-spec referencing UX) is **not** contradicted by later material — `docs/plans/2026-02-28-client-server-architecture-design.md` explicitly lists this doc as surviving "unchanged" except for storage specifics, and `docs/superpowers/specs/2026-03-31-decision-adr003-fields-design.md` implements most of its schema (question, rejected_alternatives, confidence, tags, scope, origin_spec, origin_stage) faithfully.
