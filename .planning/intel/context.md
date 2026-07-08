# Context Intel

Running notes keyed by topic, appended with source attribution. Sourced from all 98
DOC-classified documents in the full 177-document ingest corpus (supersedes the prior
50-doc-only synthesis, which covered exactly 1 DOC), plus cross-cutting provenance notes
gathered while synthesizing `decisions.md` and `constraints.md` that don't belong in either
(pure narrative/lineage context, not a decision or a constraint in their own right).

Most DOCs here are **implementation plans** — the task-by-task execution companion to a SPEC
or ADR already covered in `constraints.md`/`decisions.md`. Per ingest instructions, a plan/design
pair is only called out as a conflict if the plan's implementation narrative actually
**disagrees** with its paired design's decision; where the plan simply confirms/executes the
design, it is noted here as "confirms, no divergence" rather than duplicated in full.

---

## Foundational roadmap & tracking

### SpecGraph Implementation Roadmap (original v1.0-draft phase plan)
- **source:** `docs/initial-design-session/specgraph-v1.0-draft-roadmap.md`
- **status:** Superseded as a roadmap by `docs/plans/2026-02-28-client-server-architecture-design.md` §8 "Revised Roadmap" and further superseded in practice by `docs/plans/2026-02-28-vertical-slice-roadmap-design.md`'s slice-based plan. Retained as historical framing.
- **content:** Four phases — Foundation (spec schema, constitution, Beads+Postgres backends, claim protocol, bundle format, core CLI, linter, migration), Authoring (codebase scanner, authoring flow, CLI agent integration, Claude Code plugin), Coordination/Export/Integration (lease model, MCP server, drift detection, ADR/doc export, Gastown, tracker sync, Apache AGE), Scale (federation, multi-repo, metrics, governance). This four-phase structure is echoed (with different content per phase, reflecting the pivot away from Beads/AGE) in current `CLAUDE.md`'s "Roadmap" section — the phase *names* persisted across the full rewrite even though nearly every phase *item* changed.

### SpecGraph Implementation Tracker
- **source:** `docs/plans/2026-02-28-implementation-tracker.md`
- **content:** Progress checklist tracker across the 7 vertical slices, linking to each slice's plan doc. Pure tracking artifact, superseded in relevance once slices completed; no normative content of its own beyond linking.

### SpecGraph Design and Implementation Plans (index)
- **source:** `docs/plans/README.md`
- **content:** Index of `docs/plans/` with a status legend and naming conventions. Acts as a hub linking to nearly every doc in `docs/plans/` — this is the source of the cross-ref-graph "cycles" noted in `INGEST-CONFLICTS.md` (an index page naturally links to, and is linked back by, every doc it indexes; not a decision-dependency cycle).

## Vertical slice implementation plans (companions to the Slice 1–7 SPECs/roadmap)

- `docs/plans/2026-02-28-vertical-slice-plan.md` — CLI-server-Memgraph vertical slice proving the architecture end-to-end via ConnectRPC + Docker Compose + buf codegen. Confirms `2026-02-28-vertical-slice-roadmap-design.md`, no divergence.
- `docs/plans/2026-02-28-slice-2-constitution-plan.md` — ConstitutionService create/store/query/validate/emit + `specgraph init` generation. Uses the stale `principle` field name later corrected by ADR-001 (see `decisions.md`) — this is the plan ADR-001 explicitly supersedes.
- `docs/plans/2026-02-28-slice-3-authoring-funnel-plan.md` — AuthoringService (Spark-Shape-Specify-Decompose-Approve) with postures and analytical passes. Predates the Analytical Pass System redesign (`2026-03-20-analytical-pass-system-design.md`, which replaces the placeholder pass execution this plan describes) — sequential evolution, not a contradiction.
- `docs/plans/2026-02-28-slice-4-execution-bundles-plan.md` — ExecutionService (bundle generation, prime orientation, progress callbacks, lease sweeper). Predates the bundle-format rewrite to Markdown+frontmatter (`2026-03-26-agent-actionable-execution-bundle-design.md`).
- `docs/plans/2026-02-28-slice-5-spec-lifecycle-plan.md` — LifecycleService, drift detection, JSON Schema validation, spec linter. Superseded by `2026-03-07-slice-5-spec-lifecycle-revised-plan.md` below (same slice, revised for domain types).
- `docs/plans/2026-03-07-slice-5-spec-lifecycle-revised-plan.md` — revises Slice 5 to use domain types (post storage-domain-types work) instead of proto types directly. Confirms `2026-03-06-storage-domain-types-design.md`'s approach, no divergence.
- `docs/plans/2026-02-28-slice-6-sync-integration-plan.md` — SyncService/SyncAdapter (Beads, GitHub), tool injection, `SYNCED_TO` edges. Predates the idempotent-push ADR (`FindOrCreate`) and the later removal of `specgraph inject` — both are later corrections to mechanisms this plan first establishes, not contradictions of its core sync-service shape.
- `docs/plans/2026-02-28-slice-7-claude-code-plugin-plan.md` — original Claude Code plugin (SKILL.md files, plugin.json manifest, session-start hook). Superseded by the Skill Personas → Multi-Platform Plugin → Harness Parity Epic → embed-and-write chain (see `constraints.md`).
- `docs/plans/2026-03-03-slice-3.5-scanner-cleanup-plan.md` — removes the bespoke Go AST codebase scanner (`internal/scanner`, `--scan` flag), replacing it with agent-driven context gathering. Confirmed by current `CLAUDE.md`: no `internal/scanner/` package listed in the architecture table.

## Storage domain types & E2E testing implementation plans

- `docs/plans/2026-03-06-storage-domain-types-plan.md` — confirms `2026-03-06-storage-domain-types-design.md`, no divergence.
- `docs/plans/2026-03-07-domain-types-and-slice4-plan.md` — confirms `2026-03-07-domain-types-consistency-design.md`, no divergence.
- `docs/plans/2026-03-05-e2e-test-system-plan.md` — confirms `2026-03-05-e2e-test-system-design.md` (Ginkgo+Gomega, testcontainers, Memgraph-era), no divergence.
- `docs/plans/2026-03-17-full-pipeline-e2e-plan.md` — confirms `2026-03-17-full-pipeline-e2e-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-18-content-hash.md` — implementation plan for ADR-002's `content_hash` field (Murmur3-128) on Spec/Decision. Confirms ADR-002, no divergence.
- `docs/superpowers/plans/2026-03-19-transaction-wrapped-write-paths.md` — confirms `2026-03-19-transaction-wrapped-write-paths-design.md`/ADR-004, no divergence.
- `docs/superpowers/plans/2026-03-18-changelog-graph-nodes.md` — confirms `2026-03-18-changelog-graph-nodes-design.md`, no divergence.

## Authoring funnel, decisions & analytical passes implementation plans

- `docs/plans/2026-03-17-skill-personas-plan.md` — implementation plan for the persona rewrite; self-annotated superseded (matches the design doc's own superseded status in `constraints.md`).
- `docs/superpowers/plans/2026-03-20-analytical-pass-system-plan.md` — confirms `2026-03-20-analytical-pass-system-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-22-structured-specify-output.md` — confirms `2026-03-22-structured-specify-output-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-24-conversation-log-graph-nodes.md` — confirms `2026-03-24-conversation-log-graph-nodes-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-26-conversation-recording-wiring.md` — confirms `2026-03-26-conversation-recording-wiring-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-26-agent-actionable-execution-bundle-plan.md` — confirms `2026-03-26-agent-actionable-execution-bundle-design.md`, no divergence.
- `docs/superpowers/plans/2026-04-03-steel-thread-decomposition.md` — confirms `2026-04-03-steel-thread-decomposition-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-31-decision-adr003-fields.md` — confirms `2026-03-31-decision-adr003-fields-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-27-slice-cli-commands.md`, `2026-03-27-slice-service-handler.md`, `2026-03-27-slice-web-ui.md` — three implementation plans for the Slice-as-first-class-vertex feature (`2026-03-26-slice-first-class-vertex-design.md`): CLI commands, SliceService RPC handler, and web UI slice support respectively. All confirm the design, no divergence among the three or with the design.

## Spec lifecycle implementation plans

- `docs/superpowers/plans/2026-04-06-lifecycle-amendment-supersede.md` — confirms `2026-04-06-lifecycle-amendment-supersede-design.md` (pre-inversion eligibility framing — see `INGEST-CONFLICTS.md`), no divergence from its own paired design.
- `docs/superpowers/plans/2026-04-08-lifecycle-nomenclature-inversion.md` — confirms `2026-04-08-lifecycle-nomenclature-inversion-design.md`, no divergence.
- `docs/superpowers/plans/2026-05-20-spec-provenance-model.md` — confirms `2026-05-20-spec-provenance-model-design.md`/ADR-006 across proto, storage, server, CLI, MCP, render, linter, export — no divergence.

## Constitution implementation plans

- `docs/superpowers/plans/2026-04-07-layered-constitution.md` — confirms `2026-04-07-layered-constitution-design.md`, no divergence.
- `docs/superpowers/plans/2026-05-21-spgr-8ar-piece-a-implementation-plan.md` — Storage gap + export round-trip fix (`GetMergedConstitution`, `PrimeData`, `ConstitutionProvenance`, export schema v2) — Piece A of the multi-layer-constitution-completion design.
- `docs/superpowers/plans/2026-05-21-spgr-8ar-piece-b-implementation-plan.md` — `RefreshConstitutionLayer` RPC + `constitution import --from-url`/`sync` CLI via `go-getter` — Piece B.
- `docs/superpowers/plans/2026-05-21-spgr-8ar-piece-c-implementation-plan.md` — `--show-provenance` flag on `constitution show` — Piece C.
- `docs/superpowers/plans/2026-05-21-spgr-8ar-piece-d-implementation-plan.md` — deletes the deprecated single-layer `Store.GetConstitution` method + adds a CI grep guard against regrowth — Piece D.
- `docs/superpowers/plans/2026-05-22-spgr-8ar-piece-e-implementation-plan.md` — Prime Unification: collapses three drifted "prime" surfaces (RPC, MCP resource, CLI) onto one `internal/prime` composer — Piece E.
- All five Pieces A–E confirm `2026-05-21-multi-layer-constitution-completion-design.md` sequentially, no divergence among them.

## MCP server & harness integration implementation plans

- `docs/plans/2026-04-10-mcp-server-plan.md` — confirms `2026-04-10-mcp-server-design.md`'s original stdio+HTTP dual-transport plan (later the stdio transport was dropped entirely by PR #923, per the Task 32 design-adjustment ADR-typed doc in `decisions.md`) — the plan matches the design it was written against; the design itself was later partially superseded, not the plan diverging from it.
- `docs/plans/2026-04-20-multi-platform-plugin-plan.md` — confirms `2026-04-20-multi-platform-plugin-design.md` (`authoring.Composer`, MCP prompts, conversation recording, `runInTxOrSequential`).
- `docs/plans/2026-04-27-task-32-read-mcp-resource-plan.md` — implements the corrected Task 32 design (`2026-04-27-task-32-read-mcp-resource-design.md` in `decisions.md`), no divergence.
- `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-plan.md` — confirms `2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md`, no divergence (this whole package is itself later superseded by `managedfiles/`, per `constraints.md`).
- `docs/plans/2026-05-06-harness-parity-epic-plan.md` — confirms `2026-05-06-harness-parity-epic-design.md`, no divergence.
- `docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-plan.md` — confirms `2026-05-06-spgr-yyjf-deprecate-inject-design.md`, no divergence.
- `docs/plans/2026-05-07-pr940-review-fixes-plan.md` — phased TDD fix plan for PR #940 review findings (`internal/config/pointers` `atomicWrite` hardening, symlink TOCTOU guard, `init` caller hardening). Standalone bugfix plan, not paired with a separate design doc; self-references itself in cross_refs (harmless classifier artifact — see `INGEST-CONFLICTS.md`).
- `docs/plans/2026-05-08-spgr-rwrp-pr0-plan.md` + `docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md` — a spike-plan/spike-report pair verifying Claude Code plugin-loading claims (`extraKnownMarketplaces`, `autoUpdate`, `CLAUDE_PLUGIN_ROOT`) ahead of the harness-install-parity PR chain; the verification report's findings correct the harness-install-parity design's schema assumptions — self-consistent, not a conflict (this is exactly the intended purpose of a pre-PR spike).
- `docs/plans/2026-05-08-spgr-rwrp-pra-plan.md` — PR A: `internal/config/managedfiles` package foundation (types, primitives, safety guarantees, drift detection). First implementation PR of the harness-install-parity design.
- `docs/plans/2026-05-11-spgr-rwrp-pr-b-implementation-plan.md` — confirms `2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md`, no divergence.
- `docs/plans/2026-05-11-spgr-rwrp-pr-c-implementation-plan.md` — confirms `2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md`, no divergence.
- `docs/plans/2026-05-12-spgr-rwrp-pr-d-implementation-plan.md` — confirms `2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md`, no divergence.
- `docs/plans/2026-05-12-spgr-rwrp-pr-e-implementation-plan.md` — confirms `2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md`, no divergence.
- `docs/plans/2026-05-20-spgr-rwrp-pr-f-implementation-plan.md` — confirms `2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md`, no divergence.
- `docs/plans/2026-05-20-spgr-rwrp-pr-g-implementation-plan.md` — confirms `2026-05-20-spgr-rwrp-pr-g-doctor-design.md`, no divergence.
- `docs/verification/claude.md`, `docs/verification/cursor.md`, `docs/verification/opencode.md` — three empirical post-ship verification artifacts documenting each harness's actual MCP integration behavior against a running `specgraph serve` (`.mcp.json`/`.cursor/mcp.json`/`opencode.json`, `X-Specgraph-Project` header, client-profile registry, tool naming, fixes made). These confirm the harness-parity/install-parity designs were implemented correctly and record small in-the-field corrections (e.g., Cursor's `X-Specgraph-Project` header handling) — consistent with, not contradicting, the design corpus.

## CLI lifecycle & config implementation plans

- `docs/plans/2026-04-22-cli-lifecycle-split-plan.md` — confirms `2026-04-22-cli-lifecycle-split-design.md`, no divergence.
- `docs/plans/2026-06-02-koanf-config-loader-plan.md` — confirms `2026-06-02-koanf-config-loader-design.md`, no divergence.
- `docs/plans/2026-03-18-auth-interceptor-plan.md` — confirms `2026-03-18-auth-interceptor-design.md` (original v1 auth interceptor, later superseded incrementally — see `constraints.md`), no divergence from its own paired design.
- `docs/superpowers/plans/2026-06-08-non-fatal-postgres-startup.md` — confirms `2026-06-08-non-fatal-postgres-startup-design.md`, no divergence.
- `docs/superpowers/plans/2026-06-11-server-request-logging.md` — confirms `2026-06-11-server-request-logging-design.md`, no divergence.
- `docs/superpowers/plans/2026-06-08-opentelemetry-instrumentation.md` — confirms `2026-06-05-opentelemetry-instrumentation-design.md`, no divergence.

## Web UI implementation plans

- `docs/superpowers/plans/2026-03-22-graph-visualization-ui.md` — confirms `2026-03-22-graph-visualization-ui-design.md` (adds Dagre layout + Playwright e2e detail not in the design summary), no divergence.
- `docs/superpowers/plans/2026-03-22-markdown-cli-output.md` — confirms `2026-03-22-markdown-cli-output-design.md` (adds `findings list` CLI command detail), no divergence.
- `docs/superpowers/plans/2026-03-24-show-stage-detail.md` — confirms `2026-03-24-show-stage-detail-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-25-spec-detail-page.md` — confirms `2026-03-25-spec-detail-page-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-26-web-ui-demo-readiness.md` — confirms `2026-03-26-web-ui-demo-readiness-design.md`, no divergence.

## Data lifecycle & operations implementation plans

- `docs/superpowers/plans/2026-03-28-export-import-verify.md` — confirms `2026-03-28-export-import-verify-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-29-list-changes-rpc.md` — confirms `2026-03-29-list-changes-rpc-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-30-impact-notification.md` — confirms `2026-03-30-impact-notification-design.md`, no divergence.
- `docs/superpowers/plans/2026-04-01-postgres-storage-backend.md` — confirms `2026-04-01-postgres-storage-backend-design.md`, no divergence. This is the plan that actually executed the Memgraph→Postgres cutover (see storage-backend lineage note below).
- `docs/superpowers/plans/2026-05-05-project-wide-findings-api.md` — adds a `ListProjectFindings` RPC so MCP findings resources (`specgraph://findings`) can list findings without an invalid per-spec request. No separate SPEC-classified design doc in this batch; standalone implementation-driven addition, consistent with the existing `AnalyticalFinding`/`AnalyticalPassService` model.

## Identity, authn & authz implementation plans

- `docs/plans/2026-05-26-identity-authn-implementation-plan.md` — confirms `2026-05-22-identity-authn-design.md` (Resolver/Authorizer seam, JIT creation), no divergence.
- `docs/plans/2026-05-26-identity-bootstrap-ux-4a-rpc-surface-implementation-plan.md` — exposes `UsersBackend` as an `IdentityService` ConnectRPC API with Cedar admin gating and an `auth` CLI subtree — Plan 4a of the Bootstrap & UX design.
- `docs/plans/2026-05-26-identity-bootstrap-ux-4b-bootstrap-protections-implementation-plan.md` — DB-backed `bootstrap.Ensure`, multi-server credentials file, `SoftDeleteUser`/`PurgeUser`/`UnbindOIDC` operator-protection guards — Plan 4b of the Bootstrap & UX design. Plans 4a/4b together confirm `2026-05-22-identity-bootstrap-ux-design.md`, no divergence.
- `docs/plans/2026-05-26-identity-policy-engine-implementation-plan.md` — confirms `2026-05-26-identity-policy-engine-design.md` (Cedar adoption), no divergence.
- `docs/plans/2026-05-26-identity-storage-implementation-plan.md` — confirms `2026-05-22-identity-storage-design.md`, no divergence.
- `docs/plans/2026-06-15-cli-oidc-login-implementation-plan.md` — confirms `2026-06-15-cli-oidc-login-design.md`, no divergence.
- `docs/plans/2026-06-15-oidc-app-roles-login-sync-implementation-plan.md` — confirms `2026-06-15-oidc-app-roles-login-sync-design.md`, no divergence.
- `docs/superpowers/plans/2026-03-28-oidc-authentication.md` — confirms `2026-03-28-oidc-authentication-design.md` (original bearer-JWT OIDC), no divergence.
- `docs/superpowers/plans/2026-04-02-dashboard-auth.md` — confirms `2026-04-02-dashboard-auth-design.md`, no divergence.
- `docs/superpowers/plans/2026-06-12-oidc-interactive-ui-login.md` — confirms `2026-06-12-oidc-interactive-ui-login-design.md`, no divergence.
- `docs/superpowers/plans/2026-06-16-spgr-g7st-self-service-api-keys.md` — confirms `2026-06-16-spgr-g7st-self-service-api-keys-design.md`, no divergence.

## Release engineering & repo housekeeping implementation plans

- `docs/superpowers/plans/2026-03-20-release-please-goreleaser.md` — implementation plan for the (now-superseded) release-please+goreleaser ADR.
- `docs/superpowers/plans/2026-03-20-repo-org-move.md` — implementation plan for the repo-org-move ADR (bulk go.mod/proto/site/docker reference updates).
- `docs/superpowers/plans/2026-03-26-idempotent-push.md` — implementation plan for the idempotent-push ADR (`FindOrCreate`).
- `docs/superpowers/plans/2026-03-26-release-tooling-migration.md` — implementation plan for the (now-superseded) release-tooling-migration ADR (git-cliff + goreleaser v2).
- `docs/superpowers/plans/2026-06-05-release-single-job-goreleaser.md` — implementation plan for the currently-active release-single-job-goreleaser ADR.
- All four release-pipeline plans confirm their respective paired ADR, no divergence; the ADRs themselves form the supersession chain documented in `decisions.md`/`INGEST-CONFLICTS.md`.

## Site documentation implementation plans

- `docs/superpowers/plans/2026-03-20-quickstart-and-docs-overhaul.md` — confirms `REQ-quickstart-docs-overhaul` in `requirements.md`, no divergence.
- `docs/superpowers/plans/2026-04-03-site-docs-overhaul.md` — confirms `docs/superpowers/specs/2026-04-03-site-docs-overhaul-design.md` (DOC-classified design; updates docs for the Postgres migration, adds mermaid diagrams).
- `docs/superpowers/specs/2026-03-27-site-docs-feature-coverage-design.md` (DOC-classified despite `specs/` path) and its plan `docs/superpowers/plans/2026-03-27-site-docs-feature-coverage.md` — CLI reference generator + new concept/guide pages closing gaps between site docs and the implemented codebase (slices, drift, linting, sync).
- `docs/superpowers/specs/2026-04-10-site-narrative-restructure-design.md` (DOC-classified) and its plan `docs/superpowers/plans/2026-04-10-site-narrative-restructure.md` — marketing/IA restructure for an enterprise-tech-lead audience; landing page, concept-page reordering, "How It Works" page. Doc-maintenance work, not a product/architecture decision.

---

## Cross-cutting lineage notes (compiled while reading the full corpus)

### Storage backend: three-generation evolution
1. **Gen 1** (`docs/initial-design-session/specgraph-v1.0-draft-adr-001-storage.md`, historical): dual-path — Beads(+Dolt) OR Postgres(+AGE), operator picks one, "no degraded mode."
2. **Gen 2** (`docs/plans/2026-02-28-client-server-architecture-design.md`, 2026-02-28): Memgraph is the default; Postgres+AGE is the pluggable alternative (AGE required, no CTE fallback). Beads demoted to a push-only sync adapter. `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (same day) narrows further: "Backend: Memgraph only. Postgres+AGE deferred to a future effort." All the Gen-2-era test-system designs/plans (`2026-03-05-e2e-test-system-*`, `2026-03-17-full-pipeline-e2e-*`) and several early feature specs target Memgraph/Cypher accordingly — era-appropriate, not stale errors within their own generation.
3. **Gen 3** (`docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md` + its implementation plan, 2026-04-01, current): pure Postgres/pgx, no Memgraph, no AGE, no ltree — "graph queries are viable in SQL," recursive CTEs sufficient at project scale. This is the backend reflected in the current `CLAUDE.md` (`internal/storage/postgres/`, pgx v5, recursive CTEs, testcontainers with `pgvector/pgvector:pg18`). The site-docs-overhaul plan (`2026-04-03-site-docs-overhaul.md`) is the doc-side cleanup that followed this cutover (removes Memgraph terminology from public docs).

Each generation transition is self-documented as an explicit supersession in the superseding document's own text (not left implicit) — see `INGEST-CONFLICTS.md` for why this lineage is reported as INFO rather than as a WARNING/BLOCKER despite ADR > SPEC default precedence nominally favoring the oldest (draft-ADR) generation.

### Spec lifecycle field: `lifecycle` (task/living) → `provenance` (AUTHORED/RETROACTIVE_FROM_PR/DECLARED)
The `lifecycle` field (values `task`/`living`) appears in the Postgres schema DDL in `2026-04-01-postgres-storage-backend-design.md` (`lifecycle TEXT NOT NULL DEFAULT 'task'`) and in the original v1.0-draft spec's full schema example. ADR-006 (2026-05-20, locked) removes this column entirely in favor of `SpecProvenance`. Any requirement or plan referencing `lifecycle: living` / `lifecycle: task` post-2026-05-20 is stale.

### Amend/supersede eligibility: two eligibility tables exist across the corpus
`docs/superpowers/specs/2026-04-06-lifecycle-amendment-supersede-design.md` (2026-04-06) assumes amend works on a completed/done spec ("Returning a completed spec to an earlier authoring stage"). `docs/superpowers/specs/2026-04-08-lifecycle-nomenclature-inversion-design.md` (2026-04-08 — two days later) inverts this: amend is only for in-flight specs (`approved`/`in_progress`/`review`); supersede is only for done specs. The 2026-04-06 doc's substantive contribution (diff engine, CLI `--diff`, web changelog UI) is unaffected by the eligibility flip and remains current; only its worked-example prose about *when* amend applies is stale. Both docs' companion implementation plans confirm their respective paired design faithfully — the divergence is entirely between the two designs, not between a design and its own plan.

### Superseded/retired documents referenced but not present in this ingest batch
Several docs mention companion or predecessor designs that are **not** part of this 177-document classification set:
- `spgr-qe74` "Self-Service Authz design" — explicitly stated as "retired" and superseded by the Identity Policy Engine (Cedar) design.
- `spgr-tmqm` ("MCP OAuth resource server"), `spgr-1rq9` ("generic OAuth2 provider"), `spgr-c2lb` ("role-revocation latency") — repeatedly referenced as forward-looking follow-on work by the identity/auth docs; described but not designed in this batch.

### Skill-personas doc self-declares superseded status
`docs/plans/2026-03-17-skill-personas-design.md` carries an explicit header: "Status: Superseded by [2026-04-20-multi-platform-plugin-design.md] and [2026-05-06-harness-parity-epic-design.md]." Both successors are present in this ingest batch (unlike the prior 50-doc round, where only one was classified).

### Harness/plugin delivery mechanism: three-generation evolution
1. **Gen 1** (`2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md`): three separate per-harness MCP config files, hand-maintained, later synced by `specgraph init` via JSON Merge Patch.
2. **Gen 2** (`2026-05-06-harness-parity-epic-design.md`): adds in-tree `skills/` directory + per-harness dev-time symlinks (`plugin/<harness>/skills -> ../../skills`) — explicitly a dev-time-only artifact.
3. **Gen 3** (`2026-05-08-spgr-rwrp-harness-install-parity-design.md` + its PR-lettered children 0/A/B/C/D/E/F/G, all present in this ingest batch as SPEC+DOC pairs): embed-and-write via `//go:embed`, sentinel-hash drift detection, skills served exclusively via MCP resource fetch (`specgraph://skills/<name>`) with zero on-disk skill files for end users. Gen 1's `mcpconfigs/` package and Gen 2's dev-time symlinks are both explicitly folded in and deleted during Gen 3's rollout (PR B, PR F). The three `docs/verification/*.md` empirical-verification docs post-date and confirm Gen 3's actual field behavior.

### Identity epic sequencing (see also `INGEST-CONFLICTS.md` for the cross-ref-cycle note)
All five identity/authz design docs (`identity-storage-design`, `identity-authn-design`, `identity-bootstrap-ux-design`, `identity-policy-engine-design`, plus the later `role-downgrade-failclosed`, OIDC-interactive/CLI-login, app-roles/login-sync, and self-service-API-keys docs) are part of one named epic ("Identity, RBAC & Audit", `spgr-rjrt`) with an explicit linear sequencing stated in each doc's own "Sequencing" section: Storage is foundational → Authn depends on Storage → Bootstrap & UX depends on Storage+Authn → Policy Engine (Cedar) sits alongside and supersedes part of Authn (the "Permission computation" section) and the separately-retired Self-Service Authz design. Later docs build on top of this foundation in date order and are internally consistent with it. Every design doc in this epic has a matching, non-divergent implementation-plan DOC in this ingest batch.

### Citation-cycle observations from the cross_refs graph
Running cycle detection (DFS, three-color marking) over the full 177-node cross_refs graph found:
- Several apparent cycles that resolve to `docs/plans/README.md` (an index/hub page listing nearly every doc in `docs/plans/`) linking to, and being linked back by, the docs it indexes. Not a decision-dependency cycle — an index page is expected to be bidirectionally referenced.
- `2026-05-08-spgr-rwrp-pr0-plan.md` ↔ `2026-05-08-spgr-rwrp-pr0-claude-api-verification.md` — a spike-plan/spike-report companion pair that reference each other by design (see MCP/harness section above). Verified no contradicting decision between them.
- `2026-03-16-slice-7-global-daemon-and-plugin-design.md` ↔ `2026-04-22-cli-lifecycle-split-design.md` — the first is self-declared Superseded; the second is a narrower, unrelated-in-scope follow-on (compose lifecycle data-loss fix) that happens to cross-reference the daemon design for context. Verified no contradiction.
- `2026-05-07-pr940-review-fixes-plan.md` lists itself in its own `cross_refs` — a harmless classifier artifact (self-loop), not a real cross-document cycle.
- `Identity Storage Design` ↔ `Identity Bootstrap & UX design` — mutual reference by descriptive name (not resolvable by filename matching, so not caught by the automated basename-based cycle detector, but confirmed present in both docs' text). Both are companion docs in the same sequenced epic (Storage is explicitly foundational to Bootstrap & UX per that epic's own ordering) — benign, not a genuine contradiction.

None of the above cycles were treated as synthesis-halting BLOCKERs: each was individually inspected and found to contain no contradicting decision between its members. See `INGEST-CONFLICTS.md` for the formal INFO entry.
