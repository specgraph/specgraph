# Codebase Concerns

**Analysis Date:** 2026-07-08

## Tech Debt

**Authoring output read-back not implemented:**
- Issue: A follow-up enrichment step for authoring stage output is explicitly deferred with a `TODO(Slice 4)` marker rather than implemented.
- Files: `internal/server/authoring_handler.go:115`
- Impact: Clients relying on the authoring RPC to read back enriched output after a stage transition currently get the pre-enrichment value; downstream MCP/CLI consumers must re-fetch separately.
- Fix approach: Implement the deferred read-back once "Slice 4" work is scheduled, or remove the TODO if superseded.

**`internal/config/managedfiles` carries PR-staging debt markers:**
- Issue: Multiple comments (`doc.go:20`, `sync.go:12`, `manifest.go:285`, `markdownblock.go:375`, `wholefile.go:117`) reference "PR A"/"PR B"/"PR D"/"PR E" staging language from when the managed-files framework was built incrementally. A stub-dispatch comment in `sync.go:12` still describes "per-strategy stubs that return errNotImplemented," which reads as unfinished even though the surrounding code has since been completed.
- Files: `internal/config/managedfiles/sync.go`, `manifest.go`, `markdownblock.go`, `wholefile.go`, `doc.go`
- Impact: Low functional risk (the stubs described appear to be fully implemented today), but the stale comments make it hard for a new contributor to tell what's actually still incomplete vs. historical narration.
- Fix approach: Sweep these comments during the next managed-files touch; replace PR-sequence narration with a description of current behavior.

**Global mutable priors registry in `managedfiles`:**
- Issue: `internal/config/managedfiles/priors.go` uses a package-level `var globalPriors = &priorsRegistry{...}` populated via `init()`-time `registerPrior` calls scattered across the package (e.g. `vestigial_cursor_rules.go:55`'s `var _ = func() bool {...}()` trick).
- Files: `internal/config/managedfiles/priors.go`, `internal/config/managedfiles/vestigial_cursor_rules.go`
- Impact: Correctly mutex-guarded, so not a data race, but registration order depends on Go's file-level `init()` ordering (alphabetical within a package) and is invisible from any single file. New "prior hash" entries added carelessly could silently no-op if registered after first read in a test that imports the package partially.
- Fix approach: Consider an explicit `RegisterAllPriors()` called once from a single well-documented entry point instead of scattered `init()`/top-level `var _ = func(){...}()` side effects.

**Unbounded per-IP rate-limiter bucket map:**
- Issue: `ipRateLimiter.buckets` (`internal/server/ratelimit.go`) grows one entry per distinct client IP for the lifetime of the process and is never evicted.
- Files: `internal/server/ratelimit.go`
- Impact: The type's own doc comment concedes this ("kept in memory for the process lifetime … acceptable for the public OIDC start/callback endpoints"), but a long-running server exposed to the internet on `/api/auth/oidc/start` could accumulate a large map under a distributed scan/spray attack, which is a memory-growth vector rather than a request-rate one.
- Fix approach: Add periodic sweep of idle buckets (e.g. LRU with TTL) if this endpoint is internet-facing in production; otherwise leave as documented acceptable tradeoff for now.

**`table.go` legacy CLI output helper still load-bearing:**
- Issue: Per `CLAUDE.md`, `cmd/specgraph/table.go`'s `tableWriter` is depended on by `sync.go` and `prime.go` even though most other commands have migrated to the `internal/render` package.
- Files: `cmd/specgraph/table.go`, `cmd/specgraph/sync.go`, `cmd/specgraph/prime.go`
- Impact: Two output-formatting code paths exist side by side; a change to CLI output conventions must be applied twice.
- Fix approach: Migrate `sync.go` and `prime.go` to `internal/render`, then delete `table.go`.

## Known Bugs

**No confirmed open bugs identified from static inspection.** No `FIXME`/`HACK`/`XXX` markers exist in non-test Go source, and only one `TODO` was found (see Tech Debt above). This does not rule out latent bugs — see Test Coverage Gaps for where regressions are most likely to slip through unnoticed.

## Security Considerations

**Error sanitization is manual and per-handler:**
- Risk: Every RPC handler under `internal/server/` implements its own `<name>Error(...)` function (`analytical_pass_handler.go:352`, `claim_handler.go:93`, `constitution_handler.go:262`, `decision_handler.go:163`, `execution_handler.go:293`, `graph_handler.go:201`, `export_handler.go:148`, `spec_handler.go:291`) to map storage errors to sanitized Connect error codes before returning to clients. A new handler added without following this pattern could leak internal error strings (e.g. raw SQL errors, file paths) to API clients.
- Files: `internal/server/*_handler.go` (grep for `sanitiz`)
- Current mitigation: `error_sanitize_test.go` (659 lines) exercises this extensively, and `CLAUDE.md` documents the requirement that test assertions use error codes, not message strings.
- Recommendations: Consider a shared generic error-sanitization helper (a single `sanitizeStorageError(err error) *connect.Error` used by all handlers) to remove the need to hand-roll the mapping per handler and prevent drift as new storage sentinel errors are added.

**Public unauthenticated OIDC endpoints rate-limited only in-process:**
- Risk: `internal/server/ratelimit.go` rate-limits `/api/auth/oidc/start`/callback per-IP in a single process's memory. In any horizontally-scaled deployment (multiple server replicas), the effective rate limit is `perSec * replica_count`, not the configured value.
- Files: `internal/server/ratelimit.go`
- Current mitigation: `trustedProxy`-aware IP extraction (`clientIP`) suggests some thought was given to correct source-IP resolution behind a load balancer, but no shared/distributed limiter store exists.
- Recommendations: If SpecGraph server is deployed with multiple replicas, move to a shared limiter (e.g., Redis token bucket) or document the per-replica caveat operationally.

**Symlink handling in managed-file writes:**
- Risk: `internal/config/managedfiles` explicitly defines `ErrSymlinkRejected` (`errors.go:17`), indicating prior awareness that managed files (AGENTS.md, `.cursor/rules/*`, `.mcp.json`, etc., written by `specgraph init` into user projects) could be attacked via symlink swap (TOCTOU) if not guarded.
- Files: `internal/config/managedfiles/errors.go`, `internal/config/managedfiles/atomic.go`, `internal/config/managedfiles/open_unix.go`
- Current mitigation: Dedicated `open_unix.go` and `atomic.go` for atomic, symlink-safe file writes; error sentinel actively used.
- Recommendations: None — this looks like a resolved concern, called out here for visibility since it touches filesystem writes into arbitrary user project directories.

**Cedar policy engine drives authz (`internal/auth`):**
- Risk: `internal/auth/engine.go`, `identitystore.go` (651 lines), `oidc_verifier.go`, and `loginprovider.go` implement a nontrivial custom authorization/identity layer (OIDC + Cedar policy + role ranking via `roleRank` in `identitystore.go:256`). Any gap in `knownVerbs`/`procedureActions`/`exemptProcedures` maps (`engine.go:155`, `actions.go:17`, `exempt.go:15`) could under- or over-authorize a procedure.
- Files: `internal/auth/engine.go`, `internal/auth/actions.go`, `internal/auth/exempt.go`, `internal/auth/identitystore.go`
- Current mitigation: `identitystore_test.go` (765 lines) provides substantial coverage; embedded default Cedar policy (`embedded_source.go`).
- Recommendations: When adding a new ConnectRPC procedure, verify it is deliberately added to (or intentionally omitted from) `procedureActions`/`exemptProcedures` — a missed procedure could default to either fully open or fully denied depending on the map's default-case behavior; confirm the default is fail-closed.

## Performance Bottlenecks

**Recursive CTE graph traversals bounded but potentially expensive at scale:**
- Problem: `GetTransitiveDeps`, `GetImpact` (`internal/storage/postgres/graph.go:279` and nearby) use recursive CTEs with Postgres `CYCLE` detection, bounded to 50 hops. `GetCriticalPath` builds a manual path array with `unnest WITH ORDINALITY`.
- Files: `internal/storage/postgres/graph.go` (678 lines)
- Cause: Graph traversal queries over large spec graphs (many specs, many DEPENDS_ON/composition edges) will scan proportionally more rows per hop; the 50-hop bound caps worst-case pathological graphs but does not cap per-hop fan-out.
- Improvement path: If a project's spec graph grows to thousands of nodes with high edge fan-out, consider adding query-level result-size limits or a materialized closure table for the deepest traversals used by `drift` and `impact` commands.

**In-memory rate-limiter bucket growth:** see Security Considerations above — this is also a memory/perf concern under sustained distinct-IP traffic.

## Fragile Areas

**Managed-files sentinel/versioning system:**
- Files: `internal/config/managedfiles/sentinel.go`, `helpers_md.go`, `markdownblock.go`, `wholefile.go`, `manifest.go`
- Why fragile: This subsystem parses and rewrites HTML-comment sentinels (`<!-- specgraph:init:start v=N sha256=... -->`) embedded in user-owned files (AGENTS.md, `.cursor/rules/*.mdc`) across multiple supported sentinel versions (`supportedVersions = map[int]bool{1: true, 2: true}` in `sentinel.go:31`), multiple regex variants for legacy/loose matching (`initStartLoose`, `initStartAnyVersion`, `legacyBlock` in `helpers_md.go`), and a "priors" registry to distinguish "stale but ours" from "user modified" content. Because it directly rewrites files inside arbitrary end-user git repositories, a regression here (e.g., a regex that fails to match a real-world sentinel variant) risks corrupting or duplicating content in a user's project files.
- Safe modification: Any change to sentinel format or regexes must add a new prior hash via `registerPrior` (not just update the live template) so previously-shipped canonical content is still recognized as "Stale" rather than misclassified as "Drifted-userowned." Run the full `internal/config/managedfiles` test suite (`wholefile_test.go` at 869 lines is the largest test file in the package) before merging.
- Test coverage: Substantial (`wholefile_test.go` 869 lines) but the combinatorial space (2 sentinel versions × N file types × drift/stale/synced states) is large; new file types added to the managed-files framework should get an equivalently thorough test file.

**`internal/server/authoring_handler.go` (1149 lines, largest non-test server file):**
- Files: `internal/server/authoring_handler.go`
- Why fragile: Implements the full Spark→Shape→Specify→Decompose authoring funnel RPC surface in one file, coupling stage-transition validation, proto↔domain conversion, and the deferred read-back TODO noted above. Its test file is the single largest test file in the repo (2208 lines), which itself signals the handler's surface area/complexity.
- Safe modification: Prefer adding new stage logic in `internal/authoring/` (stage logic package) and keeping `authoring_handler.go` as thin RPC glue; avoid growing this file further without splitting by stage.
- Test coverage: High line-count coverage exists, but size makes it hard to audit for missing edge cases by inspection alone.

**Proto/content drift between stage-output messages and embedded markdown:**
- Files: `proto/specgraph/v1/authoring.proto`, `internal/authoring/content/stage-*.md`
- Why fragile: Per `CLAUDE.md`, changes to `ShapeOutput`/`SpecifyOutput`/`DecomposeOutput` proto messages must be manually mirrored into backticked snake_case field references inside the embedded markdown prompt content — a purely convention-based (not compiler-enforced) coupling.
- Safe modification: `TestContentProtoDrift` (CI test) catches drift for backticked tokens, but only for the specific token pattern it checks; a field rename that isn't referenced in backticks, or added prose that doesn't use the field name verbatim, could still drift silently.
- Test coverage: One targeted drift test; no broader schema-doc consistency check exists.

## Scaling Limits

**Graph traversal hop bound:**
- Current capacity: 50-hop cap on recursive CTE traversals (`GetTransitiveDeps`, `GetImpact`, `GetCriticalPath`).
- Limit: Any dependency chain or blocking chain longer than 50 specs will be silently truncated rather than erroring, per the `CYCLE`-bounded recursive CTE design noted in `CLAUDE.md`.
- Scaling path: If real projects approach this depth (unlikely at current adoption but plausible for large decomposed epics), raise the bound and re-benchmark query latency, or surface a "path truncated" indicator to callers instead of silent truncation.

## Dependencies at Risk

**Large transitive Google Cloud / OTel dependency surface:**
- Risk: `go list -m -u all` shows many transitive `cloud.google.com/go/*` modules pulled in (likely via OpenTelemetry's `autoexport` or an indirect exporter dependency) with available minor-version updates. These are not directly imported by SpecGraph code but inflate the dependency graph and `go.sum` surface.
- Impact: Larger attack surface for supply-chain concerns (more transitive modules to audit) and slower `go mod tidy`/build-cache warm-up; not a functional risk today.
- Migration plan: Periodically run `go mod tidy` and audit whether the OTel `autoexport` dependency (`go.opentelemetry.io/contrib/exporters/autoexport`) truly requires pulling in the full Google Cloud SDK tree, or whether a narrower exporter set would shrink this.

## Missing Critical Features

**No distributed rate limiting for multi-replica deployments** — see Security Considerations. Not a missing feature for a single-instance deployment, but a gap for horizontally-scaled production use of the OIDC login endpoints.

## Test Coverage Gaps

**Postgres integration paths excluded from default `task check`:**
- What's not tested by the fast gate: `internal/storage/postgres/` integration tests (testcontainers-based) and `//go:build e2e` suites are excluded from `task check` / `go test ./...` by build tags, per `CLAUDE.md`. They only run under `task pr-prep` (requires Docker) or CI.
- Files: `internal/storage/postgres/*_test.go` (e.g. `users_test.go` 1159 lines, `graph_test.go` 722 lines), `e2e/`
- Risk: A contributor who runs only `task check` before pushing can introduce a regression in the Postgres-backed storage layer or in ConnectRPC end-to-end flows that isn't caught until CI/`task pr-prep` runs. `CLAUDE.md`'s own "Build-tagged dirs escape task check" gotcha corroborates this as a known, recurring risk (also noted in the user's memory: symbol-deletion audits must include e2e/ and integration files).
- Priority: Medium-high — this is a process gap more than a code gap, but it has caused missed regressions before (per project memory).

**Agent-CLI-dependent e2e tests self-skip when the `claude` CLI is unavailable or unauthenticated:**
- What's not tested: `e2e/agent/agent_suite_test.go` skips its suite entirely if the `claude` CLI binary isn't on `PATH`, isn't authenticated, or produces no output (e.g., when run nested inside another Claude Code session) — see lines 42, 53, 57, 60.
- Files: `e2e/agent/agent_suite_test.go`
- Risk: These are exactly the conditions likely to occur in a generic CI runner or in a nested-agent development environment (like this one), meaning the agent-integration e2e suite may silently no-op in exactly the environments where it would be run.
- Priority: Medium — verify these tests actually execute (not skip) in the primary CI pipeline; if they always skip there too, the agent-CLI integration path has no live coverage.

**Referential-integrity edge case explicitly left to unit tests, not exercised e2e:**
- What's not tested: `e2e/api/lifecycle_test.go:374` skips a "dangling edges cannot be created" e2e scenario because the storage backend enforces referential integrity, deferring to unit test coverage instead.
- Files: `e2e/api/lifecycle_test.go`
- Risk: Low — this looks like a deliberate, reasonable test-layer decision (integrity is a DB-constraint concern, better tested at the storage-unit level) rather than a genuine gap.
- Priority: Low.

---

*Concerns audit: 2026-07-08*
