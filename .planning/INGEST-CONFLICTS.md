## Conflict Detection Report

Full-corpus re-synthesis (177 classified documents: 14 ADR, 63 SPEC, 2 PRD, 98 DOC),
superseding the prior 50-document-only report. Mode: `new` (bootstrap — no existing
PROJECT.md/ROADMAP.md/REQUIREMENTS.md to check against).

### BLOCKERS (0)

None found.

- No two LOCKED ADRs contradict each other on the same scope. The three LOCKED
  process/tooling ADRs governing the release pipeline (`2026-03-20-release-please-goreleaser-design.md`,
  `2026-03-26-release-tooling-migration-design.md`, `2026-06-05-release-single-job-goreleaser-design.md`)
  form a self-declared linear supersession chain — each later doc's own `Supersedes:`
  field names its predecessor — rather than three simultaneously-active contradicting
  decisions. See the INFO entry below for why this is not treated as a BLOCKER.
- No `UNKNOWN`/low-confidence classifications exist in this batch (0 of 177).
- Cycle detection ran over the full 177-node cross_refs graph (DFS, three-color
  marking, well under the 50-hop cap). All detected cycles were individually
  inspected for a contradicting decision between their members; none were found.
  See the INFO entry below.
- Mode is `new`, so there is no existing locked CONTEXT.md decision to contradict.

### WARNINGS (0)

None found. This corpus is unusually self-documenting: nearly every apparent
disagreement between two documents is resolved by an explicit `Supersedes:` field,
a self-declared `Status: Superseded` header, or unambiguous ADR-vs-lower-precedence
ranking — leaving nothing that requires a user pick between competing variants. (The
prior 50-document round flagged two WARNINGs; both are re-examined below and
reclassified to INFO on the full corpus, with rationale for the reclassification.)

- Found: 2 PRD-classified documents (`2026-03-20-quickstart-and-docs-overhaul-design.md`,
  `2026-03-26-confluence-to-specgraph-design-bridge.md`).
- Impact: none — their scopes do not overlap (one is a docs/release gate, the other an
  external Confluence template product), so there is no divergent-acceptance-criteria
  case to raise into `competing-variants`.

### INFO (14)

[INFO] Process/tooling ADRs are a separate population from domain-architecture ADRs
  Note: 10 of the 14 ADR-classified documents in this corpus carry `locked: true`, but
  only 5 (`docs/decisions/ADR-001`, `ADR-002`, `ADR-004`, `ADR-005`, `ADR-006`) govern
  the spec-graph domain model. The other 5 locked ADRs live under
  `docs/superpowers/specs/` and were classified as ADR by the classifier's
  Status/Decision-section heuristic, but their subject matter is release engineering
  and repo housekeeping (`2026-03-20-release-please-goreleaser-design.md`,
  `2026-03-20-repo-org-move-design.md`, `2026-03-26-idempotent-push-design.md`,
  `2026-03-26-release-tooling-migration-design.md`,
  `2026-06-05-release-single-job-goreleaser-design.md`). Both are kept as genuinely
  LOCKED for precedence purposes (their own `Status: Approved` markers are honored,
  not silently overruled), but `decisions.md` keeps them in clearly separate sections
  so downstream readers do not conflate "10 locked ADRs" with "10 locked domain
  decisions." A downstream roadmap should treat the 5 domain-architecture ADRs as the
  load-bearing constraints and the 5 process ADRs as release/ops housekeeping context.

[INFO] Release-pipeline ADR chain is a self-declared supersession sequence, not a
  simultaneous LOCKED-vs-LOCKED contradiction
  Found: Three LOCKED ADRs address the same scope (GitHub release automation) with
  mutually incompatible mechanisms: (1) `2026-03-20-release-please-goreleaser-design.md`
  chose release-please+goreleaser+cocogito; (2) `2026-03-26-release-tooling-migration-design.md`
  replaced it with git-cliff+goreleaser v2, and explicitly states `Supersedes:
  2026-03-20-release-please-goreleaser-design.md`; (3) `2026-06-05-release-single-job-goreleaser-design.md`
  replaced that in turn with a GoReleaser-only single job, and explicitly states
  `Supersedes: 2026-03-26-release-tooling-migration-design.md`. Root cause documented
  in doc (3): the (2)-era pipeline had two independent, non-coordinating
  release-creation code paths that produced empty release notes on every publish
  from v0.3.7 through v0.7.0.
  Note: Because each successor doc's own text names its predecessor as superseded,
  this is treated as a linear chain (only doc 3 is the currently active decision)
  rather than a hard BLOCKER requiring the user to arbitrate between three
  simultaneously-valid locked decisions. `decisions.md` documents all three for
  provenance; only the 2026-06-05 doc should inform downstream release-pipeline
  requirements.

[INFO] Auto-resolved (reclassified from prior-round WARNING): LOCKED ADR-002 wins over
  non-locked draft ADR-003 on Decision-ID scheme
  Found: `docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md`
  (Status: Proposed, not locked) specifies Decision IDs on the Postgres path as
  `d-{first 8 hex of sha256(normalized title)}`, an intentionally content-addressable
  scheme where identical titles collide by design.
  Expected: LOCKED `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md` keeps
  Decision IDs as stable ULIDs (`dec-{ULID}`) and explicitly rejects
  content-addressable IDs as an alternative ("ID changes on every edit break graph
  edges").
  → No user action needed — per the standard precedence rule (LOCKED beats non-locked
  automatically), ADR-002 governs; draft ADR-003's ID-scheme text is stale. The prior
  50-document round surfaced this as a WARNING out of caution since draft ADR-003 was
  never formally marked Superseded in-source; on reflection this is a clean
  LOCKED-vs-non-LOCKED case with no genuine ambiguity requiring a user pick, so it is
  reclassified to INFO/auto-resolved here. The rest of draft ADR-003 (decision
  lifecycle, schema fields, cross-spec referencing) is unaffected and was implemented
  faithfully downstream (`2026-03-06-storage-domain-types-design.md`,
  `2026-03-31-decision-adr003-fields-design.md`).

[INFO] Auto-resolved: LOCKED ADR-001 wins over stale `principle` field name in two DOCs
  Found: `docs/plans/2026-02-28-slice-2-constitution-plan.md` and
  `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` both describe the Principle
  proto message's field 2 as `principle`.
  Expected: LOCKED `docs/decisions/ADR-001-principle-statement-field-naming.md` renames
  this field to `statement` and explicitly supersedes the Slice 2 plan's definition.
  → No user action needed — ADR wins over DOC/SPEC by default precedence. Pre-v1, no
  migration required.

[INFO] Auto-resolved: LOCKED ADR-002 wins over "content-addressable ID" language in
  early architecture docs
  Found: `docs/initial-design-session/specgraph-v1.0-draft-spec.md` and
  `docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md` describe spec/decision
  IDs using content-addressable framing.
  Expected: LOCKED ADR-002 explicitly supersedes "the implicit content-addressable ID
  convention in docs and proto comments" and mandates stable ULIDs + a separate
  `content_hash` field.
  → No user action needed — ADR wins by default precedence; this is stated as
  ADR-002's own explicit rationale for existing.

[INFO] Draft ADR-002 (Gastown, Beads-required) superseded by the sync-adapter model
  Note: `docs/initial-design-session/specgraph-v1.0-draft-adr-002-gastown.md` (non-locked)
  required the Beads(+Dolt) backend for native Gastown integration.
  `docs/plans/2026-02-28-client-server-architecture-design.md` explicitly lists this
  doc's "integration model (Beads sync adapter replaces native coupling)" as
  superseded — Beads demoted from a candidate core backend to a push-only sync
  adapter. Auto-resolved lineage, same self-documentation pattern as the storage
  backend evolution below; the Gastown-as-optional-integration concept itself
  survives and matches current `CLAUDE.md`.

[INFO] Storage backend: three-generation evolution, non-locked SPECs, no gate needed
  Note: (1) `docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md`
  (historical, not locked) proposed a dual-path Beads(+Dolt)-OR-Postgres(+AGE) backend.
  (2) `docs/plans/2026-02-28-client-server-architecture-design.md` (2026-02-28)
  explicitly superseded it with Memgraph-as-default + Postgres+AGE-as-pluggable-alternative,
  demoting Beads to a sync adapter; the same-day `2026-02-28-vertical-slice-roadmap-design.md`
  narrowed further to "Memgraph only." (3) `docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md`
  (2026-04-01) replaced Memgraph entirely with pure Postgres/pgx — no AGE, no
  Memgraph, no Beads-as-backend — and is explicitly labeled "Supersedes: ADR-001
  assumption of Postgres+AGE." This is the backend confirmed by the current
  `CLAUDE.md` (`internal/storage/postgres/`, pgx v5, recursive CTEs, testcontainers
  with `pgvector/pgvector:pg18`). A cluster of Gen-2-era test-system and feature
  SPECs/DOCs (`2026-03-05-e2e-test-system-*`, `2026-03-17-full-pipeline-e2e-*`,
  `2026-03-16-slice-7-global-daemon-and-plugin-design.md`, etc.) reference Memgraph
  accordingly — era-appropriate at time of writing, not stale errors within their own
  generation.
  → Not a BLOCKER or WARNING: none of the three architecture documents in this
  lineage is a LOCKED ADR, each transition is self-documented as an explicit
  supersession in the superseding document's own text, and the final generation is
  independently confirmed by the current, already-implemented codebase. Downstream
  planning should treat generation 3 (pure Postgres) as authoritative and generations
  1–2 as historical only.

[INFO] Spec lifecycle field replaced by ADR-006; stale references are pre-2026-05-20 only
  Note: The `lifecycle` field (`task`/`living`) appears in `docs/initial-design-session/specgraph-v1.0-draft-spec.md`
  and the Postgres schema DDL inside `2026-04-01-postgres-storage-backend-design.md`
  (`lifecycle TEXT NOT NULL DEFAULT 'task'`). LOCKED ADR-006 (2026-05-20) removes this
  field entirely in favor of `SpecProvenance`.
  → No user action needed — ADR wins by default precedence and by chronology; any
  requirement referencing `lifecycle: living`/`lifecycle: task` should be treated as
  stale by downstream planning.

[INFO] Amend/supersede eligibility flip is a two-day sequential correction, not a
  simultaneous contradiction
  Note: `docs/superpowers/specs/2026-04-06-lifecycle-amendment-supersede-design.md`
  (2026-04-06) assumed amend applies to a completed/done spec.
  `docs/superpowers/specs/2026-04-08-lifecycle-nomenclature-inversion-design.md`
  (2026-04-08, two days later, Approved) explicitly fixes this as inverted/backwards
  and flips it: amend is only for in-flight specs (`approved`/`in_progress`/`review`);
  supersede is only for done specs. Neither document is a LOCKED ADR. The 2026-04-06
  doc's diff-engine/CLI/web-UI infrastructure is unaffected by the eligibility flip
  and remains current; only its worked-example prose about *when* amend applies is
  stale.
  → No user action needed — later, more specific, Approved correction wins; both
  docs' paired implementation plans confirm their own design faithfully (the
  disagreement is between the two designs, not between either design and its own
  plan).

[INFO] Harness/plugin delivery mechanism: three-generation evolution, self-documented
  Note: (1) `2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md` — hand-maintained
  per-harness MCP config files synced via JSON Merge Patch. (2)
  `2026-05-06-harness-parity-epic-design.md` — adds an in-tree `skills/` directory plus
  dev-time symlinks per harness. (3) `2026-05-08-spgr-rwrp-harness-install-parity-design.md`
  and its lettered PR children (0/A/B/C/D/E/F/G, all present in this batch) —
  embed-and-write via `//go:embed`, sentinel-hash drift detection, skills served
  exclusively via MCP resource fetch. Generation 3 explicitly folds in and deletes
  generations 1 and 2's packages (PR B, PR F).
  → Not a BLOCKER or WARNING — none of these are LOCKED ADRs, each transition is
  self-documented, and three independent post-ship verification docs
  (`docs/verification/claude.md`, `cursor.md`, `opencode.md`) confirm generation 3 is
  what actually shipped.

[INFO] Auth/authz epic: sequential build-up, not contradictory alternatives
  Note: The "Identity, RBAC & Audit" epic (`spgr-rjrt`) spans 12+ SPEC documents from
  the original static-permission-table auth interceptor design
  (`2026-03-18-auth-interceptor-design.md`) through bearer-JWT OIDC
  (`2026-03-28-oidc-authentication-design.md`), session-cookie dashboard auth
  (`2026-04-02-dashboard-auth-design.md`), the Identity Storage/Authn/Bootstrap-UX
  trio (2026-05-22), Cedar policy engine adoption (2026-05-26, explicitly deletes the
  static permission table), a fail-closed role-downgrade fix (2026-06-04), interactive
  OIDC login for the web UI and CLI (2026-06-12/06-15), OIDC app-roles + login-sync
  (2026-06-15), and self-service API-key provisioning (2026-06-16). Each later
  document explicitly states what it supersedes or extends from its predecessors (own
  "Sequencing" sections; explicit supersession of the Authn design's "Permission
  computation" section — confirmed from both sides, Authn self-annotates it superseded
  and the Cedar design confirms superseding it — and the separately-retired
  `spgr-qe74` Self-Service Authz design).
  → Not a BLOCKER or WARNING — this is additive, sequenced epic construction with
  each step's scope and supersession explicitly self-declared; no two documents in
  this set assert genuinely incompatible requirements on the same still-live scope.

[INFO] ADR-004's `RunInTransaction` mechanism is not literally reused by the identity
  domain's `AuthStore`
  Note: `docs/plans/2026-05-22-identity-storage-design.md` introduces a global,
  non-project-scoped `AuthStore` structurally separate from the per-project
  `postgres.Store` that ADR-004's `RunInTransaction` wraps. Later specs
  (`2026-06-15-cli-oidc-login-design.md`, `2026-06-16-spgr-g7st-self-service-api-keys-design.md`)
  implement bespoke `pool.Begin`-based atomic transactions directly on `AuthStore`.
  → Flagged for awareness only, not a contradiction: ADR-004's atomicity *principle*
  is preserved in the identity domain, just not its named *mechanism*, and every
  later doc reconciles this explicitly in-source. No user action needed.

[INFO] Citation cycles in the cross_refs graph are benign companion/index references,
  not decision-dependency cycles (reclassified from prior-round WARNING)
  Found: Cycle detection (DFS, three-color marking) over the full 177-node cross_refs
  graph found several cycles, all resolving to one of: (a) `docs/plans/README.md`, an
  index/hub page listing nearly every doc in `docs/plans/`, which is naturally linked
  to and back by the docs it indexes; (b) mutual references between companion
  plan/design or spike-plan/spike-report pairs (`2026-05-08-spgr-rwrp-pr0-plan.md` ↔
  `...-claude-api-verification.md`; `2026-03-16-slice-7-global-daemon-and-plugin-design.md`
  ↔ `2026-04-22-cli-lifecycle-split-design.md`); (c) one classifier artifact where
  `2026-05-07-pr940-review-fixes-plan.md` lists itself in its own cross_refs; (d) a
  descriptive-name (not filename) mutual reference between `Identity Storage Design`
  and `Identity Bootstrap & UX design` — the same pair the prior 50-document round
  flagged as a WARNING.
  → Each cycle's members were individually inspected; none contains a contradicting
  decision. The Identity Storage ↔ Bootstrap-UX pair in particular has explicit,
  linear "Sequencing" sections in both documents ("structural foundation" /
  "depends on Identity Storage") that resolve ordering unambiguously outside the
  cross_refs field — the mutual reference is a "Companion designs:" front-matter
  courtesy link, not a circular decision dependency. Per synthesis judgment, none of
  these represent the kind of decision-dependency loop the cycle-detection step
  exists to catch (this synthesis reads each doc independently rather than
  recursively traversing cross_refs, so there is no risk of a synthesis loop).
  Reclassified from WARNING to INFO on the full corpus since no genuine ambiguity
  requiring a user pick was found in any cycle member. All docs in every cycle were
  synthesized normally into `decisions.md`/`constraints.md`/`context.md`.

[INFO] Zero competing PRD acceptance-criteria variants
  Note: This corpus contains exactly 2 PRD-classified documents
  (`2026-03-20-quickstart-and-docs-overhaul-design.md`,
  `2026-03-26-confluence-to-specgraph-design-bridge.md`). Their scopes do not overlap
  (internal docs/release gate vs. external Confluence design template), so there is
  no divergent-acceptance-criteria case on a shared requirement. The
  `competing-variants` bucket is consequently empty.

---

## Summary

- **0 blockers**
- **0 warnings**
- **14 auto-resolved / informational** entries (self-documented supersessions,
  ADR-over-lower-precedence auto-resolutions, benign citation cycles, and sequencing/
  awareness notes) — see above for full detail on each. Two entries the prior
  50-document round classified as WARNING (draft-ADR-003 vs ADR-002 on Decision-ID
  scheme; the Identity Storage ↔ Bootstrap-UX cross-ref cycle) are reclassified to
  INFO here on reflection against the standard precedence rules and full-corpus
  context — both resolve cleanly with no genuine ambiguity requiring a user pick.

**STATUS: READY** — no blockers and no warnings are present, so this intel may be
routed into `PROJECT.md`/`REQUIREMENTS.md`/`ROADMAP.md` generation without further
user gating, per the doc-conflict-engine safety gate. Downstream planning should
still read the INFO entries above, particularly the process-vs-domain ADR split and
the storage-backend/lifecycle-field/harness-delivery generation lineages, since they
materially affect which historical documents are authoritative.
