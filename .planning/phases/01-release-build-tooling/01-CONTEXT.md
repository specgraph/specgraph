# Phase 1: Release & Build Tooling - Context

**Gathered:** 2026-07-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Maintainers can cut a tagged release and trust the build/lint tooling without manual intervention or double-published/broken artifacts. Originally scoped to three requirements (REL-01, CFG-01, CFG-02) — **two are already shipped on `main`** (see below). The only remaining implementation work in this phase is CFG-02: pinning `task tools`' golangci-lint install to the same version CI uses.

</domain>

<decisions>
## Implementation Decisions

### Scope correction (found during discussion, not asked as a gray area)
- **D-00:** REL-01 (single-job goreleaser release model) and CFG-01 (koanf layered config loader) are **already fully implemented on `main`** — verified directly against the repo, not just beads/PROJECT.md status, which had lagged.
  - REL-01: PR #981 (merged 2026-06-05) matches `docs/superpowers/specs/2026-06-05-release-single-job-goreleaser-design.md` exactly. Verified `v0.12.0`'s actual GitHub Release: single publish, populated changelog, signed/SBOM'd assets — all three of REL-01's roadmap success criteria hold today.
  - CFG-01: `internal/config/global.go` implements the full flag>env>file>default koanf precedence from `docs/plans/2026-06-02-koanf-config-loader-design.md`, including the underscore-collision-safe env mapper and the `SPECGRAPH_PG_URL` deprecation warning (`cmd/specgraph/serve.go:284`).
  - PROJECT.md, REQUIREMENTS.md, and ROADMAP.md have been updated to mark REL-01/CFG-01 Done. **Planning for this phase should scope to CFG-02 only** — do not re-plan or re-verify REL-01/CFG-01 work.

### Pinning mechanism (CFG-02)
- **D-01:** Switch `task tools` (`Taskfile.yml:357`) from `brew install golangci-lint` to `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>` — the same install method CI already uses (`ci.yml:78`). This guarantees identical version resolution between local and CI; brew formula pinning was rejected as non-durable (old versions get pruned from core taps).
- **D-02:** No CI-side change to *how* golangci-lint is installed — only local `task tools` changes to match CI's existing method.

### Version source of truth
- **D-03:** The pinned version string becomes a single Taskfile.yml var; `ci.yml`'s install step reads that value instead of keeping its own independent `GOLANGCI_LINT_VERSION` env var. One file owns the version; the other consumes it. The exact mechanism for CI to read a Taskfile var (e.g. a small `task`-exposed print/helper target) is left to research/planning — not decided here.

### Drift detection
- **D-04:** No separate version-check step in `task check`/`task doctor`. Fixing the install path (D-01) closes the drift vector structurally — both CI and local now install the exact same pinned version by the exact same method, so there's nothing left to detect.

### Tool scope
- **D-05:** Pin golangci-lint only. Do NOT extend this to the other brew-installed dev tools (gofumpt, lefthook, actionlint, goreleaser, dprint, cocogitto, rumdl, yamlfmt, buf) — none of them currently have a CI-side pin to drift against, so there's no matching problem to fix, and CFG-02's requirement text names golangci-lint specifically. Extending scope would be new capability, not implementation of what's already scoped.

### Claude's Discretion
- Exact shape of the Taskfile→CI version-reading mechanism (D-03) — e.g. a `task tools:golangci-lint-version` print helper vs. some other wiring. Research/planning should pick whatever fits Taskfile.dev's actual capabilities cleanly.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### CFG-02 — golangci-lint pinning (active work)
- `.github/workflows/ci.yml` (line ~27, `GOLANGCI_LINT_VERSION: v2.12.1`; line ~78, install step) — current CI pin, the target of truth to match
- `Taskfile.yml` (line ~357, `task tools`; line ~211-214, `task lint`) — current unpinned brew install to be replaced

### REL-01 — already shipped (reference only, no new work)
- `docs/superpowers/specs/2026-06-05-release-single-job-goreleaser-design.md` — locked design, confirmed implemented
- `.github/workflows/release.yml`, `.goreleaser.yaml`, `cog.toml` — implementation matching the design

### CFG-01 — already shipped (reference only, no new work)
- `docs/plans/2026-06-02-koanf-config-loader-design.md` — locked design, confirmed implemented
- `internal/config/global.go`, `cmd/specgraph/serve.go` — implementation matching the design

No external specs for CFG-02 itself — it's a small, unambiguous tooling fix with no separate design doc; the decisions above fully capture scope.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `ci.yml`'s existing `go install ...@${{ env.GOLANGCI_LINT_VERSION }}` pattern is the exact install invocation to replicate locally — no new tooling pattern needed, just move it into `Taskfile.yml`.

### Established Patterns
- Other pinned-version tools in CI (`PROTOC_GEN_GO_VERSION`, `YAMLFMT_VERSION`, `ADDLICENSE_VERSION`) follow the same `env:`-block-with-version-string pattern as `GOLANGCI_LINT_VERSION` — useful precedent if the chosen version-source-of-truth mechanism needs to generalize, though D-05 keeps scope to golangci-lint only for this phase.

### Integration Points
- `task tools` (`Taskfile.yml:354-360`) is the single local install entrypoint to modify.
- `task lint` (`Taskfile.yml:211-214`) just invokes `golangci-lint run` — unaffected by the pinning fix itself, since it relies on whatever `task tools` installed.

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the decisions above — open to whatever concrete Taskfile mechanism the planner picks for D-03, as noted under Claude's Discretion.

</specifics>

<deferred>
## Deferred Ideas

- Pinning other `task tools`-installed dev tools (gofumpt, lefthook, actionlint, goreleaser, dprint, cocogitto, rumdl, yamlfmt, buf) the same way golangci-lint is being pinned here — explicitly deferred (D-05), no existing CI-side pin for these to drift against yet. Would need its own requirement if picked up later.

</deferred>

---

*Phase: 1-Release & Build Tooling*
*Context gathered: 2026-07-08*
