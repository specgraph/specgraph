# Constraints Intel

Synthesized from all 63 SPEC-classified documents in the full 177-document ingest corpus
(supersedes the prior 50-doc-only synthesis, which covered 41). One entry per document (each
may carry multiple constraint sub-types). Every entry lists `source`. Grouped by theme;
within a theme, ordered chronologically — later documents in the same theme often supersede
earlier ones. Supersession is called out explicitly where documented in-source, and
cross-checked against `INGEST-CONFLICTS.md`.

---

## Theme: Overall architecture & storage backend

### SpecGraph v2 Design: Client/Server Architecture
- **source:** `docs/plans/2026-02-28-client-server-architecture-design.md`
- **type:** protocol / schema (api-contract)
- **content:** Redesigns SpecGraph from embedded-library-with-dual-backends into a client/server system. ConnectRPC API (JSON/HTTP + gRPC wire compat) between all clients (CLI, skills, MCP proxy, future UI) and a single server process. **Memgraph is the default graph backend**; Postgres+AGE is the pluggable alternative (AGE required, no CTE fallback at this stage). Beads demoted from core backend to push-only sync adapter alongside issue trackers. Bundles are lean (spec + bootstrap + callback URLs); agents pull detailed context from a `/prime` endpoint. Declares itself the formal supersession of the `docs/initial-design-session/` v1.0-draft documents (see `decisions.md`, draft ADR-001/002 entries). Full proto service surface defined: SpecService, AuthoringService, DecisionService, ConstitutionService, ExecutionService, SyncService, ServerService. **Superseded in turn** by `2026-04-01-postgres-storage-backend-design.md` (Memgraph/AGE dropped for pure Postgres — see `INGEST-CONFLICTS.md`).

### SpecGraph Vertical Slice Roadmap
- **source:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md`
- **type:** protocol (roadmap/sequencing)
- **content:** Implementation roadmap: Constitution → Authoring → Execution Bundles → Lifecycle → Sync → Claude Code Plugin. States explicitly: "Backend: Memgraph only. Postgres+AGE deferred to a future effort." Slice 2 constitution proto description uses field name `principle` on the repeated Principle message — this is the stale pre-ADR-001 naming (auto-resolved, ADR-001 wins; see `decisions.md`/`INGEST-CONFLICTS.md`). Slices 1–7 fully enumerated with proto/storage/handler/CLI components per slice.

### E2E Test System Design
- **source:** `docs/plans/2026-03-05-e2e-test-system-design.md`
- **type:** nfr (test infrastructure) / protocol
- **content:** Two-tier Ginkgo/Gomega E2E test system covering CLI/API behavior and Docker-mode server lifecycle. Uses Memgraph testcontainers for the graph backend — reflects the Gen-2 (Memgraph) architecture era; superseded in practice once the Postgres backend design landed (companion later doc `2026-03-17-full-pipeline-e2e-design.md` still targets Memgraph too, both are era-appropriate, not contradictions).

### Storage Domain Types & Decision Promotion Design
- **source:** `docs/plans/2026-03-06-storage-domain-types-design.md`
- **type:** schema
- **content:** Decouples `Backend`/`DecisionBackend` storage interfaces from generated proto types (`storage.Spec`, `storage.Decision` domain types; proto↔domain converters live in the handler layer). Implements ADR-003's decision-promotion gap: `StoreShapeOutput` creates first-class `Decision` nodes (via `DecisionInput` messages) with `DECIDED_IN` edges, instead of storing decisions as raw JSON strings.

### Storage Domain Types Consistency & Slice 4 Execution
- **source:** `docs/plans/2026-03-07-domain-types-consistency-design.md`
- **type:** schema
- **content:** Extends the domain-types decoupling to `ConstitutionBackend` and `ClaimBackend`, and implements the Slice 4 `ExecutionBackend` using domain types throughout — eliminates remaining proto-type leaks across the storage boundary. Companion to `2026-03-06-storage-domain-types-design.md` above (same theme, immediately follow-on, no divergence).

### Slice 7: Global Daemon & Claude Code Plugin — Design
- **source:** `docs/plans/2026-03-16-slice-7-global-daemon-and-plugin-design.md`
- **type:** schema + protocol
- **content:** Self-declared **Status: Superseded** (per classification notes; retained for historical context). Proposes turning SpecGraph into a global daemon with XDG-layout config, per-project graph namespacing (multi-tenancy inside one Memgraph instance), and a Claude Code plugin wrapping the CLI. Superseded by the later harness-parity/install-parity design chain (below) and by the CLI lifecycle split design, which took the narrower path of fixing `up`/`down`/`install`/`uninstall` semantics rather than building a global daemon.

### Full Pipeline E2E Test Suite — Design
- **source:** `docs/plans/2026-03-17-full-pipeline-e2e-design.md`
- **type:** nfr (test infrastructure) / protocol
- **content:** Three-tier e2e test architecture (protocol/CLI/agent) validating the full SpecGraph pipeline against a real Memgraph instance, covering constitution integration, lifecycle transitions, and multi-project isolation. Era-appropriate Memgraph reference (see storage-backend evolution note in `INGEST-CONFLICTS.md`); the test-tier structure itself (protocol → CLI → agent) is architecture-independent and outlived the backend swap.

### Postgres Storage Backend Design
- **source:** `docs/superpowers/specs/2026-04-01-postgres-storage-backend-design.md`
- **type:** schema (api-contract + NFR)
- **content:** **Replaces the Memgraph/Cypher backend entirely with pure Postgres/SQL** (`internal/storage/postgres/`). Explicitly "Supersedes: ADR-001 assumption of Postgres+AGE." Decisions: project-scoped via WHERE clause (no RLS); separate typed node tables (specs, decisions, slices); in-process Subscribable pattern (no LISTEN/NOTIFY); optimistic versioning; big-bang migration (no dual-write); all edges (including internal HAS_CHANGE/HAS_FINDING) in one `edges` table; pgx v5 native driver, not database/sql; goose v3 migrations; no AGE, no ltree — pure SQL/recursive CTEs sufficient at this scale; pgvector `vector(3072)` columns reserved for future semantic search. Full schema DDL given (projects, specs, decisions, slices, edges, changelog_entries, findings, conversation_logs, claims, execution_events, constitutions, sync_mappings). Graph queries via recursive CTEs with PG14+ `CYCLE` clause (50-hop bound), manual path arrays for critical path. **This is the storage backend reflected in current `CLAUDE.md`.**

## Theme: Storage transaction / consistency layer

### Transaction-Wrapped Write Paths for Atomicity
- **source:** `docs/superpowers/specs/2026-03-19-transaction-wrapped-write-paths-design.md`
- **type:** nfr (consistency)
- **content:** Design source for ADR-004. Wraps all multi-query write paths (CreateSpec, UpdateSpec, TransitionStage, Store*Output, AmendSpec, Lifecycle{Amend,Supersede,Abandon}Spec, UpdateDecision) in `RunInTransaction`. Validation without DB access stays outside the transaction. Nested `RunInTransaction` calls reuse the outer transaction automatically.

### ChangeLog Graph Nodes for Version Tracking
- **source:** `docs/superpowers/specs/2026-03-18-changelog-graph-nodes-design.md`
- **type:** schema
- **content:** Introduces `ChangeLog` node type + `HAS_CHANGE` edge (internal-only, not in proto `EdgeType` enum, not exposed via AddEdge/RemoveEdge). Every material mutation (where `content_hash` changes) creates a ChangeLog node with field-level `FieldChange` deltas. Checkpoints at stage transitions. Removes the old opaque `history_json` field entirely (clean break, no migration).

### ConversationLog Graph Nodes for Authoring Audit Trail
- **source:** `docs/superpowers/specs/2026-03-24-conversation-log-graph-nodes-design.md`
- **type:** schema
- **content:** `ConversationLog` node type + `AUTHORED_VIA`/`CONTINUES`/`EXPLAINS` edges (all internal-only). Captures probe-and-response exchange pairs per authoring stage. `specgraph conversation record` CLI command. All writes under `RunInTransaction` per ADR-004.

### Wire Authoring Skills to Call RecordConversation
- **source:** `docs/superpowers/specs/2026-03-26-conversation-recording-wiring-design.md`
- **type:** protocol
- **content:** Wires authoring skills to actually call the `RecordConversation` RPC (previously the plumbing above existed but nothing called it) via a shared `conversation-recording.md` reference doc and CLI invocation. Also enables a real `conversation_count` Memgraph Cypher query on `ListSpecs` (was previously a placeholder).

## Theme: Decisions domain

### Design: Extend Decision Domain Type with ADR-003 Fields
- **source:** `docs/superpowers/specs/2026-03-31-decision-adr003-fields-design.md`
- **type:** schema
- **content:** Adds question, rejected_alternatives (structured), confidence (enum), tags, scope (enum), origin_spec, origin_stage, version to the Decision domain type/proto/storage. Generalizes `createChangeLog` to accept a node label parameter (`"Spec"` or `"Decision"`) so Decisions get ChangeLog support too. Backward-compatible: missing properties on legacy nodes default to zero values.

## Theme: Authoring funnel & personas

### Skill Personas — Design
- **source:** `docs/plans/2026-03-17-skill-personas-design.md`
- **type:** protocol (agent behavior)
- **content:** Self-declared **Status: Superseded** by `2026-04-20-multi-platform-plugin-design.md` and `2026-05-06-harness-parity-epic-design.md`; persona content now lives in `internal/authoring/content/persona.md` + embedded composer; per-stage SKILL.md now shared across harnesses. Retained here for historical context only. Original content: three-layer skill architecture (Persona/Domain/Execution), three postures (Drive/Partner/Support) with auto-detection heuristics, pushback protocol, per-stage elicitation sequences and quality heuristics.

### Structured SpecifyOutput
- **source:** `docs/superpowers/specs/2026-03-22-structured-specify-output-design.md`
- **type:** schema (api-contract)
- **content:** Replaces flat `SpecifyOutput` fields (single-string interface_contract, repeated-string verify_criteria/touches) with structured sub-messages: `InterfaceSection{name,body}`, `VerifyCriterion{category,description}`, `FileTouch{path,purpose,change_type}`. `invariants` stays `repeated string`. Wire-break at 0.2.0-dev, field numbers reused (not reserved) since semantic intent preserved.

### Analytical Pass System Design
- **source:** `docs/superpowers/specs/2026-03-20-analytical-pass-system-design.md`
- **type:** protocol (api-contract)
- **content:** New `AnalyticalPassService` (RunAnalyticalPass, StoreFindings, ListFindings) replacing hardcoded `CheckViolation` substring matching. All-LLM evaluation — "constitution is too fluid for mechanical checks." Unified `AnalyticalFinding` type replaces five separate finding types (RedTeamFinding, PeripheralVisionItem, ConsistencyIssue, SimplicityFinding, ConstitutionViolation) — clean break, no migration. `HAS_FINDING` edge is internal-only (same treatment as `HAS_CHANGE`). `StoreFindings` MUST use `RunInTransaction` per ADR-004. `SafetyFlag`/`StoreSafetyFlags` retained separately (deterministic, not LLM-driven).

### Steel Thread Decomposition Strategy
- **source:** `docs/superpowers/specs/2026-04-03-steel-thread-decomposition-design.md`
- **type:** protocol
- **content:** Adds `DECOMPOSITION_STRATEGY_STEEL_THREAD = 4` enum value. `slices[0]` is the thread (no deps); all other slices must transitively reach it. Server-side `validateSteelThread` enforces single-root + full reachability (not full acyclicity — existing cycle check covers that separately).

### Introduce Slice as First-Class Graph Vertex
- **source:** `docs/superpowers/specs/2026-03-26-slice-first-class-vertex-design.md`
- **type:** schema
- **content:** New `:Slice` graph node type + `SliceService` RPC (ListSlices/GetSlice/ClaimSlice/CompleteSlice), replacing Spec-nodes-distinguished-by-COMPOSES-edges for decomposition slices. `DecomposeOutput` slimmed to `strategy` + `slice_slugs` (reserves old `slices`/`child_spec_slugs` fields). Slices participate in dependency/impact/critical-path graph queries but not in `GetReady` (readiness is a Spec concept). Clean break, no migration.

### Agent-Actionable Execution Bundle
- **source:** `docs/superpowers/specs/2026-03-26-agent-actionable-execution-bundle-design.md`
- **type:** protocol (api-contract)
- **content:** Bundle format changes from YAML to **Markdown with YAML frontmatter** — primary consumer is an LLM agent; drops directly into bead files. Full authoring outputs and decisions inline (specs are frozen at approval); constitution via pointer only (`specgraph prime <slug>`) since it's project-wide and can change independently. Claim instructions always present. Dependencies include a pre-computed drift flag. `Bundle.bundle_yaml` field reserved (not removed); new field 7 `bundle_content` holds markdown.

### Bootstrap & Authoring Funnel Demo — Design Spec
- **source:** `docs/superpowers/specs/2026-03-21-bootstrap-funnel-demo-design.md`
- **type:** protocol (runbook, non-normative)
- **content:** Reproducible demo runbook (spark→shape→specify→decompose→approve→graph exploration) using Claude Code plugin skills. Notes teardown guidance should NOT reference `specgraph down --rm` (retired by the CLI-lifecycle-split design — see below); correct verb is `specgraph uninstall` or `task dev:reset`.

## Theme: Spec lifecycle

### Lifecycle Amendment & Supersede — Design Spec
- **source:** `docs/superpowers/specs/2026-04-06-lifecycle-amendment-supersede-design.md`
- **type:** protocol / nfr (UX)
- **content:** Adds `CompareVersions` RPC + `internal/diff` package (word-level inline diffs), `GetSpecAtVersion` (reconstructs state by replaying changelog deltas), enhanced `specgraph changes` CLI (`--diff`, `--from/--to`), web dashboard changelog timeline + version-comparison view, new `site/docs/concepts/lifecycle.md`. Written when amend/supersede eligibility was still in the pre-inversion ("broken") shape — its own worked example describes amend as "returning a **completed** spec to an earlier stage," which the very next design (below) flips. The diff/UI/docs infrastructure itself is orthogonal to eligibility and remains valid. See `INGEST-CONFLICTS.md` (auto-resolved/INFO).

### Lifecycle Nomenclature Inversion — Design Spec
- **source:** `docs/superpowers/specs/2026-04-08-lifecycle-nomenclature-inversion-design.md`
- **type:** protocol (state machine)
- **content:** **Supersedes the amend/supersede eligibility described in the 2026-04-06 doc above.** Fixes inverted semantics: **Amend** now eligible only from `{approved, in_progress, review}` (in-flight rework); **Supersede** now restricted to `{done}` only (replace finished work). Removes the `amended` semi-terminal stage entirely — `re_entry_stage` now required on every amend call, no more defaulting. New sentinels: `ErrSpecNotAmendable`, `ErrReEntryStageRequired`; `ErrSpecNotDone` repurposed for supersede. Goose migration force-moves any spec in the removed `amended` stage to `spark`.

### Spec Provenance Model — Design
- **source:** `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md`
- **type:** schema (source doc for ADR-006)
- **content:** Full design backing ADR-006 (see `decisions.md`). Full stage taxonomy: `spark→shape→specify→decompose→approved→in_progress→review→done`, plus terminal `superseded`/`abandoned`. Confirms amend eligibility is `{approved,in_progress,review}` only (matches the nomenclature-inversion doc, not the earlier amendment-supersede doc). `provenance_type` and `provenance_detail` are explicitly excluded from `contenthash.Spec` inputs. RETROACTIVE_FROM_PR/DECLARED specs are born directly at `stage=done`, skip the funnel, and have empty `conversation_logs`. `claim`/`report-completion` gated to `provenance=AUTHORED` only. Clean-break wire migration (proto field 10 repurposed, no data preserved, migration precondition-guards against non-empty `specs` table).

## Theme: Constitution

### Layered Constitution — Design Spec
- **source:** `docs/superpowers/specs/2026-04-07-layered-constitution-design.md`
- **type:** schema
- **content:** Multi-layer support (User→Org→Project→Domain precedence). Schema: `UNIQUE (project_slug, layer)` replacing single-row-per-project. Merge engine (`internal/constitution/merge`, pure/no storage deps): scalar=highest-layer-wins, string-lists=union+dedup, keyed-object-lists=merge-by-key. `$delete: true` directive removes an item after merge. Provenance map (field-path → source layer) returned alongside merged result.

### Multi-Layer Constitution Completion + Prime Unification — Design Spec
- **source:** `docs/superpowers/specs/2026-05-21-multi-layer-constitution-completion-design.md`
- **type:** schema + protocol (api-contract)
- **content:** Closes the gap where `PrimeData`/export still used the single-layer `GetConstitution` (silently flattening multi-layer projects). Adds `GetMergedConstitution` to `ConstitutionBackend`; deprecates (then, in the piece-D implementation plan, deletes) the single-layer method with a CI grep guard against regrowth. Export schema version bumps 1→2 (`constitution` singular field → `constitutions` list, cross-field validated). New `RefreshConstitutionLayer` RPC for remote-source import via `hashicorp/go-getter` (restricted to http/https/git/github getters, no archive decompression, 1 MiB body cap, GitHub-token-only via `SPECGRAPH_FETCH_GITHUB_TOKEN` with strict host allow-list + cross-host redirect stripping). Canonical hashing operates on the parsed domain struct (not raw bytes) via Murmur3-128, sharing the existing `spaolacci/murmur3` dependency. Also unifies three previously-drifted "prime" surfaces (ExecutionService.GetPrime RPC, `specgraph://prime` MCP resource, `specgraph prime` CLI) onto one shared `internal/prime` composer producing `ProjectView`/`SpecView`.

## Theme: MCP server & harness integration

### MCP Server Design
- **source:** `docs/plans/2026-04-10-mcp-server-design.md`
- **type:** protocol (api-contract)
- **content:** Adds a full MCP server via `mark3labs/mcp-go` (isolated to 3 files for swappability). Two transports originally planned: stdio (`specgraph mcp`, thin ConnectRPC-over-HTTP proxy) and embedded HTTP (`specgraph serve`, in-process `LocalClient`). Three tool tiers (core/authoring/execution) negotiated via `clientInfo.metadata["specgraph.role"]`. Auth reuses existing ConnectRPC auth entirely — no separate MCP auth layer. **Stdio transport later dropped entirely** (PR #923, "HTTP-only") — see the Task 32 design-adjustment ADR-typed doc in `decisions.md`.

### Multi-Platform Plugin Design
- **source:** `docs/plans/2026-04-20-multi-platform-plugin-design.md`
- **type:** protocol
- **content:** Replaces the Claude-only plugin with thin, structurally-equivalent plugins across Cursor, OpenCode, Codex, and Claude, all consuming shared MCP-delivered workflow content (authoring content composition, prompt composition, conversation recording, profiles). This is the harness-parity effort's first formal design; largely superseded/absorbed by the later `2026-05-06-harness-parity-epic-design.md` and `2026-05-08-spgr-rwrp-harness-install-parity-design.md`, which took over the harness-plurality work with a different delivery mechanism (embed-and-write vs this doc's original plugin-composition approach). Skill Personas design (above) cites this doc as one of its two superseding successors.

### Harness Parity Epic: Consolidate Claude Code, Cursor, and OpenCode Integration
- **source:** `docs/plans/2026-05-06-harness-parity-epic-design.md`
- **type:** protocol
- **content:** In-tree `skills/` directory (agentskills.io-spec-clean SKILL.md packages) + thin per-harness shims under `plugin/<harness>/`. Six v1 skills. OpenCode plugin corrected post-merge to use `experimental.chat.system.transform` + `tool.execute.after` (not the originally-attempted, nonexistent `session.start`/`tool.use` hooks — document itself flags this as a self-corrected drift, historical prose retained but code is source of truth; matches current `CLAUDE.md`'s OpenCode shim description exactly). **First stage of a chain superseded by** `2026-05-08-spgr-rwrp-harness-install-parity-design.md` below (symlink-based skill delivery → embed-and-write).

### Harness install parity via embed-and-write — design
- **source:** `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
- **type:** protocol + nfr
- **content:** **Supersedes the symlink-based skill delivery from the harness-parity epic above.** Uniform "embed-and-write" pattern: harness shim content embedded into the CLI binary (`//go:embed`), `specgraph init` writes canonical files every run. Drift detection via sentinel comments (version + sha256) with four states (Missing/Synced/Stale/Drifted). Single `internal/config/managedfiles/` package with a `Strategy` enum (`JSONKeyMerge`/`MarkdownBlock`/`WholeFile`) subsuming and deleting the earlier `mcpconfigs/`+`pointers/` packages. 14-file per-harness manifest. Skills delivered exclusively via MCP resource fetch (`specgraph://skills/<name>`) — zero on-disk skill files for end users; explicit accepted UX regression for Claude (loses native skill-card auto-load). `specgraph doctor` + drift-nudge (`isatty(stderr)`-gated, 24h-throttled) introduced. PRs sequenced 0→A→B→{C,D,E,F,G} (all present in this ingest batch as companion implementation-plan DOCs, no divergence found).

### PR B — Port existing managed files into the `managedfiles` framework
- **source:** `docs/plans/2026-05-11-spgr-rwrp-pr-b-port-managed-files-design.md`
- **type:** protocol
- **content:** Migrates the five pre-existing specgraph-managed config/markdown files onto the `managedfiles` framework with byte-identical output guarantees: `.mcp.json`, `.cursor/mcp.json`, `opencode.json` via `JSONKeyMerge`; `AGENTS.md`, `.cursor/rules/specgraph-bootstrap.mdc` via `MarkdownBlock`. Marker versioning bumps v=1 → v=2 to force one-time re-sync of pre-existing installs.

### PR C — OpenCode plugin under `.specgraph/agents/opencode/`
- **source:** `docs/plans/2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md`
- **type:** protocol
- **content:** `specgraph init` writes the OpenCode plugin TypeScript file via `WholeFileStrategy` (sentinel-versioned, hash-checked) and union-merges `opencode.json`'s `plugin` array entry for the new path.

### spgr-rwrp PR D — Cursor rule files via embed-and-write
- **source:** `docs/plans/2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md`
- **type:** protocol
- **content:** Writes Cursor `.mdc` rule files via embed-and-write with frontmatter-aware sentinel/hash handling (the sentinel must live inside/around YAML frontmatter without corrupting Cursor's own frontmatter parser).

### spgr-rwrp PR E — Claude plugin shim via embed-and-write
- **source:** `docs/plans/2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md`
- **type:** protocol
- **content:** Writes the Claude Code plugin shim (`.claude/settings.json`, marketplace.json, plugin.json, hooks) via embed-and-write, extending `JSONKeyMerge`/`WholeFile` strategies to cover Claude's specific file shapes.

### spgr-rwrp PR F — Skills via MCP resource handler
- **source:** `docs/plans/2026-05-20-spgr-rwrp-pr-f-skills-mcp-design.md`
- **type:** protocol (api-contract)
- **content:** Implements the skills-via-MCP piece of the harness-install-parity design. `internal/mcp/skills/` package with a `Source` interface (List/Get/Search), `embeddedSource` backed by `//go:embed`. Three new MCP tools (`specgraph_skills_list/_get/_search`) + templated resource `specgraph://skills/<name>`. Adds required `summary:` frontmatter (≤120 chars) as a SpecGraph-local extension to the agentskills.io spec. Deletes `plugin/specgraph/skills` symlink + `task plugin:sync` in the same PR (deliberately not a parallel-run).

### spgr-rwrp PR G — `specgraph doctor` + drift nudge + dogfood plumbing
- **source:** `docs/plans/2026-05-20-spgr-rwrp-pr-g-doctor-design.md`
- **type:** protocol + nfr
- **content:** Implements `specgraph doctor` (Binary/Server/Project-config/Managed-files check groups; exit codes 0/1/2; `--json`/`--fix`/`--harness`/`--exit-zero`). Drift-nudge as a `PersistentPreRun` cobra hook — primary gate is `isatty(stderr)` (closes a class of bug where a non-interactive hook like Claude's session-start would otherwise inject nudge text into the model-read prime payload). `specgraph health` becomes a deprecated alias for `specgraph doctor server`. `task plugin:check` wired into `task check`.

### spgr-7htb: idempotent `specgraph init` with managed per-harness MCP configs
- **source:** `docs/plans/2026-05-04-spgr-7htb-init-idempotent-mcp-configs-design.md`
- **type:** protocol (api-contract)
- **content:** `specgraph init` becomes idempotent; per-harness MCP configs (`.cursor/mcp.json`, `.mcp.json`, `opencode.json`) synced via JSON Merge Patch (RFC 7396, `evanphx/json-patch/v5`). Slug-conflict on re-init is a hard error (slug is the storage partition key). **Superseded by** the embed-and-write `managedfiles/` framework (2026-05-08 above), which folds this package in and deletes it.

### Deprecate `specgraph inject` in favor of MCP + extended `init`
- **source:** `docs/plans/2026-05-06-spgr-yyjf-deprecate-inject-design.md`
- **type:** protocol
- **content:** Removes `specgraph inject` (CLI, RPC, proto, domain enum) end-to-end — all three supported harnesses now speak MCP natively, so file-injection is no longer needed. Replaces with `internal/config/pointers/` (later folded into `managedfiles/`), writing minimal pointer files (`AGENTS.md`, `.cursor/rules/specgraph-bootstrap.md`) with managed-block marker fencing (`<!-- specgraph:init:start v=1 -->`). Legacy per-slug inject blocks in AGENTS.md are actively purged on next `init`; orphan `.claude/specs/*.md` files are left untouched.

## Theme: CLI lifecycle & config

### CLI Lifecycle Split — Design
- **source:** `docs/plans/2026-04-22-cli-lifecycle-split-design.md`
- **type:** nfr (data safety) + protocol
- **content:** Fixes a data-loss bug: `specgraph down` unconditionally ran `docker compose down -v`, destroying the Postgres volume. Splits `install`/`uninstall` (service registration) from `up`/`down` (runtime lifecycle, non-destructive by default) from `down --purge` (explicit, confirmation-guarded destructive teardown). `specgraph serve`'s exit defer changes from `ComposeDown` to non-destructive `ComposeStop`. **Retires `--rm` on `down` permanently** (hard error pointing to `uninstall`/`down --purge`) — no reintroduction with new meaning.

### Koanf Config Loader — Global Config Layering
- **source:** `docs/plans/2026-06-02-koanf-config-loader-design.md`
- **type:** nfr (config precedence)
- **content:** Replaces ad-hoc `os.Getenv` scattered config reads with a centralized `koanf`-based loader with explicit precedence (flag > env > file > default). Every global-config field becomes env-overridable except `SPECGRAPH_API_KEY` (explicitly excluded — public documented contract, resolved independently by harnesses) and four other special-purpose env vars (secret, behavioral toggle, dev/test seams). Only one clean-break rename: `SPECGRAPH_PG_URL` → `SPECGRAPH_SERVER_POSTGRES_URL` (with a deprecation warning). Custom `envKeyMapper` avoids underscore-collision ambiguity by building the mapping from known dotted keys rather than naive `_`→`.` replacement. This is the deferred-work item cross-referenced by the Task 32 read-mcp-resource design in `decisions.md` (`spgr-5kd5`).

### Non-fatal Postgres at startup (`docker: false`)
- **source:** `docs/superpowers/specs/2026-06-08-non-fatal-postgres-startup-design.md`
- **type:** nfr (availability)
- **content:** Makes `specgraph serve` startup non-fatal when an external (non-Docker-managed) Postgres is unreachable — server stays alive with a degraded-readiness probe listener and a connect retry/backoff loop, self-healing once Postgres becomes reachable, instead of crash-looping.

### Per-request access logging
- **source:** `docs/superpowers/specs/2026-06-11-server-request-logging-design.md`
- **type:** nfr (observability)
- **content:** Standard per-request access log line across ConnectRPC, REST, `/mcp/`, and static UI traffic, with outcome-based log levels. New `AccessLog`/`AccessLogInterceptor` middleware, a `reqctx` context carrier, and config knobs (`log.requests`, `server.probes.log_requests`).

### OpenTelemetry Instrumentation: Traces, Metrics & Context-Aware Logging
- **source:** `docs/superpowers/specs/2026-06-05-opentelemetry-instrumentation-design.md`
- **type:** nfr (observability)
- **content:** End-to-end OpenTelemetry traces, metrics, and context-aware `slog` logging across the CLI and server. `internal/telemetry` provides a single Init/Shutdown lifecycle (no-op when disabled), OTLP export, `otelconnect`/`otelpgx`/`otelhttp` wiring, and app-level metric instruments. Matches current `CLAUDE.md`'s description of `internal/telemetry/` exactly.

### Authentication & Authorization Design
- **source:** `docs/plans/2026-03-18-auth-interceptor-design.md`
- **type:** nfr (security) + api-contract
- **content:** Original v1 auth: single ConnectRPC interceptor, local-first (implicit OS-user admin identity when no keys configured), API keys as first auth primitive, JWT detection stub (`Unimplemented` until OIDC lands). Static `rpcPermissions` table maps every procedure to a `service:action` permission; three built-in roles (reader/writer/admin) with wildcard matching. **Superseded incrementally** by OIDC (below), then by the Cedar policy engine (Identity Policy Engine design) which deletes the static permission table entirely.

## Theme: Web UI

### Graph Visualization UI
- **source:** `docs/superpowers/specs/2026-03-22-graph-visualization-ui-design.md`
- **type:** protocol (api-contract) + nfr
- **content:** Embedded read-only SvelteKit graph explorer UI, served alongside the ConnectRPC API. New `GraphService.GetFullGraph` RPC; static file serving with CORS middleware for the embedded SPA.

### Markdown CLI Output
- **source:** `docs/superpowers/specs/2026-03-22-markdown-cli-output-design.md`
- **type:** protocol (api-contract)
- **content:** Replaces tab-writer CLI output with Markdown rendering via a new `internal/render` package, for AI-agent readability. Covers `GetSpec`, Edge, Decision, DriftReport, AnalyticalFinding, NodeRefList renderers. Adds `--json` flags and a `printJSON` helper (per `CLAUDE.md`'s `cmd/specgraph/output.go` gotcha).

### Show Stage Detail Design Spec
- **source:** `docs/superpowers/specs/2026-03-24-show-stage-detail-design.md`
- **type:** protocol
- **content:** Extends `specgraph show` to render authoring stage outputs (Spark, Shape, Specify, Decompose) inline via proto, storage, and render-pipeline changes.

### Spec Detail Page Design Spec
- **source:** `docs/superpowers/specs/2026-03-25-spec-detail-page-design.md`
- **type:** protocol (web UI)
- **content:** Expands the web UI spec detail page with collapsible `AccordionSection.svelte` sections for stage outputs, edges, and conversations.

### Web UI Demo-Readiness: Dashboard Overhaul + Detail Page Enhancements
- **source:** `docs/superpowers/specs/2026-03-26-web-ui-demo-readiness-design.md`
- **type:** protocol (web UI) + schema
- **content:** Dashboard tabs, detail-page metadata/findings sections, new `conversation_count` proto field, `SpecTable`/`TabBar`/`FindingsSection`/`MetadataBar` Svelte components, `ListSpecs` query changes, `analyticalPassClient` wiring — closes web UI data gaps identified before customer demos.

## Theme: Data lifecycle & operations (export, changes, notifications)

### Full Text Export/Import with Verification
- **source:** `docs/superpowers/specs/2026-03-28-export-import-verify-design.md`
- **type:** protocol (api-contract) + nfr (integrity)
- **content:** Signed, versioned JSON export/import/verify pipeline via ConnectRPC (`ExportService`, `ImportProject`, `VerifyExport`). HMAC signing + referential-integrity validation on import. CLI `export`/`import`/`verify` commands. Export schema later bumped 1→2 by the multi-layer constitution completion design (constitution singular field → list) — additive, not contradictory.

### ListChanges RPC Design
- **source:** `docs/superpowers/specs/2026-03-29-list-changes-rpc-design.md`
- **type:** protocol (api-contract)
- **content:** Exposes `ChangeLogBackend.ListChanges` via a new ConnectRPC endpoint (`SpecService.ListChanges`), `ChangeLogEntry` proto message, handler, `changes` CLI command, and changelog renderer.

### Impact Notification Service Design
- **source:** `docs/superpowers/specs/2026-03-30-impact-notification-design.md`
- **type:** protocol
- **content:** Event-driven impact notification: `ChangeEvent`, `ChangeSubscriber`, `Subscribable` interface, `RunInTransaction` post-commit hook (fires only after successful commit, per ADR-004's transaction boundary), `ImpactLogger` subscriber implementation.

## Theme: Identity, authn, and authz (the "Identity, RBAC & Audit" epic)

> Companion docs cross-reference each other by descriptive name rather than filename in
> several cases (see `INGEST-CONFLICTS.md` for the cross-ref-cycle assessment between
> Identity Storage and Identity Bootstrap & UX). Sequencing per their own "Sequencing"
> sections: **Storage is foundational → Authn depends on Storage → Bootstrap & UX depends
> on Storage+Authn → Policy Engine (Cedar) sits alongside, superseding parts of Authn.**

### Identity Storage Design
- **source:** `docs/plans/2026-05-22-identity-storage-design.md`
- **type:** schema
- **content:** Global (not project-scoped) Postgres store for Human/ServiceAccount/OIDCBinding/APIKey entities, reached through a separate constructor from the per-project `postgres.Store`. Plaintext API keys never persisted (memory-hard hash at rest, shown once). API keys have a separable indexed lookup prefix (accepted timing side-channel traded for O(log N) resolution). At most one bootstrap admin enforced at the schema layer. OIDC binding uniqueness is global on `(issuer, subject)`. No data migration from legacy YAML-backed stores (`config_store.go` etc. removed in the same release).

### Identity Authn Design
- **source:** `docs/plans/2026-05-22-identity-authn-design.md`
- **type:** protocol (nfr: security)
- **content:** Single resolver replaces the four-way ConfigStore/OIDCStore/CompositeStore/implicit-OS-user split. Resolution semantics: missing auth → unauthenticated (no more OS-user fallback); JIT for unknown OIDC subjects (rate-limited + optional email-domain allowlist); persisted role is authoritative on repeat login (claims-mapping NOT re-evaluated per request — later superseded by login-sync, see below); API-key role optionally clamped by a per-key downgrade at resolve time. Error categorization distinguishes credential failure (unauthenticated) from backend failure (service unavailable). "Permission computation" section explicitly self-annotated as superseded by the Identity Policy Engine (Cedar) design below.

### Identity Bootstrap & Operator UX Design
- **source:** `docs/plans/2026-05-22-identity-bootstrap-ux-design.md`
- **type:** protocol
- **content:** Two parallel bootstrap paths (local-mode via `specgraph init` with DB access; hosted-mode via server first-start, key surfaced in logs). Bootstrap user is a system identity (`display_name: "admin"`, no OIDC binding — this is what makes it immune to later login-sync demotion, see below). Credentials file becomes CLI-only (server no longer reads it for key provisioning) and supports multiple server entries. Two-flag protection (force + admin-role) against accidental bootstrap-identity deletion.

### Identity Policy Engine Adoption (Cedar)
- **source:** `docs/plans/2026-05-26-identity-policy-engine-design.md`
- **type:** protocol (nfr: security)
- **content:** Adopts Cedar (`cedar-policy/cedar-go`) as the authorization engine, replacing the static `rpcPermissions` table entirely (no deprecation cycle — internal mechanism, no external API). Engine wrapped behind SpecGraph-owned `auth.PolicyEngine` interface (only file importing cedar-go directly). Policy sources composable (embedded + operator filesystem dirs; DB/URL sources are additive follow-ups). Explicitly and formally supersedes "the retired Self-Service Authz design (`spgr-qe74`)" (not in this ingest batch) and the "Permission computation" section of the Authn design above. `Identity.Permissions map[string]bool` removed; `HasPermission`/wildcard logic deleted.

### Custom-role downgrade: fail-closed clamp (spgr-rjrt.9)
- **source:** `docs/superpowers/specs/2026-06-04-spgr-rjrt-9-role-downgrade-failclosed-design.md`
- **type:** nfr (security fix)
- **content:** Fixes a fail-**open** bug: `clampedRole` returned the user's fuller role whenever either side of the downgrade comparison was an unranked custom role. New behavior: unorderable-but-present downgrade floors to `reader` (never the user's fuller role). `CreateAPIKey` validates `role_downgrade` is one of the three built-ins at creation time; `RotateAPIKey` doesn't accept a new downgrade (inherits old, still floor-protected at resolve time).

### OIDC Authentication Design
- **source:** `docs/superpowers/specs/2026-03-28-oidc-authentication-design.md`
- **type:** protocol (api-contract) + nfr (security)
- **content:** Original bearer-JWT OIDC support: multi-provider + local fallback, static claims-to-role mapping, local JWKS validation (`go-oidc/v3`), `auth.mode` config (local/oidc/mixed), `CompositeStore` (ConfigStore-then-JWT-fallback), separate `credentials.yaml` (0600). Bootstrap auto-generates an admin key on first local-mode start. This is the bearer-token-only precursor — interactive login (browser flow) is added later (below); this design remains the basis for machine/CI bearer-JWT auth.

### Dashboard Authentication Design
- **source:** `docs/superpowers/specs/2026-04-02-dashboard-auth-design.md`
- **type:** protocol (api-contract) + nfr (security)
- **content:** Adds cookie-based session authentication to the web dashboard, removing the local-mode auth bypass. Login modal, ConnectRPC interceptor + REST middleware for session validation, CSRF protection. Explicitly frames itself as an interim step with an "OIDC migration path" — superseded functionally by the later Interactive OIDC login for the web UI design below, which replaces the session mechanism with a real OIDC Authorization Code + PKCE flow. Not a contradiction: this design's session-cookie infrastructure was the placeholder the later design explicitly builds on top of/replaces.

### Interactive OIDC login for the web UI
- **source:** `docs/superpowers/specs/2026-06-12-oidc-interactive-ui-login-design.md`
- **type:** protocol (api-contract) + nfr (security)
- **content:** Adds a real "Sign in with X" browser flow (Authorization Code + PKCE) issuing a SpecGraph-owned opaque server-side session (`spgr_ws_...`, decoupled from IdP token lifetime) — NOT the originally-considered "store the ID token in the cookie" approach (rejected: Entra `groups` claims blow the 4KB cookie limit). Entra ID and Okta native; GitHub only via an OIDC broker (GitHub itself issues opaque tokens, no `id_token`, no discovery doc — not natively OIDC). New `web_sessions` + `oidc_login_flows` Postgres tables (server-side flow state, multi-replica safe — an ephemeral signed-cookie approach was rejected for this reason). Interactive login bypasses the JIT rate limiter (still enforces email-allowlist + claims-mapping). Session cookie `SameSite=Lax` (not `Strict`) with an explicit "no GET mutates state" invariant it depends on.

### CLI OIDC Login Design
- **source:** `docs/plans/2026-06-15-cli-oidc-login-design.md`
- **type:** protocol (api-contract) + nfr (security)
- **content:** `specgraph login`/`specgraph logout` via server-brokered loopback redirect with PKCE (the server, not the CLI, talks to the IdP). Explicitly rejects a "paste a one-time code" fallback for remote/SSH sessions as phishable (detailed account-takeover walkthrough in-doc) — v1 has no remote/headless login path at all; SSH/headless sessions are hard-errored toward `specgraph auth api-key create`. Strict loopback validation (literal `127.0.0.1`/`::1` only, `localhost` explicitly rejected per RFC 8252 §8.3). New `cli_login_codes` table; atomic exchange via a dedicated `AuthStore`-level transaction (NOT `*Store.RunInTransaction`, which `AuthStore` cannot reach — see the ADR-004 cross-reference note in `decisions.md`). Config gate `auth.oidc.cli_login_enabled` (default true).

### OIDC App-Roles + Login-Sync — Design
- **source:** `docs/plans/2026-06-15-oidc-app-roles-login-sync-design.md`
- **type:** protocol (nfr: security)
- **content:** Fixes GitHub #996 (Entra "groups overage" silently breaks group-claim role mapping) by pivoting to app roles (`roles` claim, never subject to overage) instead of fetching overflowed groups via Graph API (explicitly rejected — adds egress, credentials, throttling). Introduces `sync_on_login` (default true): on each interactive login only (never per-request bearer/API-key), re-enforces the email-domain allowlist, refreshes DisplayName/Email (rename-guarded), and re-derives role from only the issuer just authenticated with (single-issuer, not most-privileged-across-bindings — that alternative was designed then rejected as unreachably non-atomic and permanently escalation-prone for stale bindings). Fail-open/fail-closed asymmetry: promotion-persist-failure is best-effort (proceed at old lower role); demotion-persist-failure is fail-closed (deny login). Custom roles are never treated as provable promotions (matches the fail-closed philosophy from the role-downgrade fix above). Explicit, documented Operational Warning: default-on `sync_on_login` can mass-demote every existing OIDC user to `default_role` on next login if `claims_mapping` hasn't been re-pointed from `groups` to `roles` before upgrade — the bootstrap admin key (no OIDC binding) is the only immune identity.

### Self-Service MCP API-Key Provisioning for OIDC Users
- **source:** `docs/superpowers/specs/2026-06-16-spgr-g7st-self-service-api-keys-design.md`
- **type:** protocol (nfr: security)
- **content:** New Cedar verb `apikey.self` (permit for any authenticated principal) + four new `IdentityService` RPCs (CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey) letting non-admin OIDC users self-provision role-capped, expiring MCP keys — replacing the documented (and "wildly over-privileged") practice of borrowing the bootstrap admin key. Critical fix baked into the design itself: the minted key's `role_downgrade` is floored at the caller's `EffectiveRole` (not the owner's bare `Role`) on both create and rotate, closing a privilege-escalation "laundering" hole that an earlier revision missed. Callers authenticating via `Source=="apikey"` are rejected from self-mint/rotate (anti key-chaining). Mandatory expiry (90d default / 365d max cap) and a per-user active-key quota (10 default). Explicitly forward-compatible only when claims-mapping/login-sync (above) are configured for the issuer — otherwise the design partially entrenches SpecGraph-as-identity-authority, which the sibling design `spgr-tmqm` (not in this ingest batch) aims to move toward the IdP instead.

---

## Theme: Initial design session (superseded scaffolding, non-ADR)

### SpecGraph: Live Spec-Driven Development Framework (v1.0-draft spec)
- **source:** `docs/initial-design-session/specgraph-v1.0-draft-spec.md`
- **type:** schema + protocol (foundational, partially superseded)
- **content:** The original full v1.0 spec. Defines: 9 design principles; the 4-layer constitution model (User→Org→Project→Domain) with a full YAML example; 3-tier codebase context gathering (Tier 0 orientation / Tier 1 navigation / Tier 2 deep); greenfield and existing-codebase bootstrap flows; the spec object (3-field minimum solo schema, full schema with identity/intent/scope/edges/contract/verification sections). Per `docs/plans/2026-02-28-client-server-architecture-design.md`'s explicit supersession note: this document's architecture/storage sections are superseded entirely; its spec schema, constitution, authoring funnel, and agent-collaboration sections survive (with minor updates) as foundational vocabulary for the current system. Downstream consumers should treat the schema/constitution/context sections as live reference material, and the architecture sections (storage backend choice, two-path Beads-or-Postgres model) as historical only.
