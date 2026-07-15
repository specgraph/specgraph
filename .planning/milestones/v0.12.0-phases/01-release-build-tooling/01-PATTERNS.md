# Phase 1: Release & Build Tooling - Pattern Map

**Mapped:** 2026-07-08
**Files analyzed:** 2 (both existing config files, direct edits — no new source files)
**Analogs found:** 2 / 2 (in-file sibling patterns; no external analog needed)

## Scope Note

This phase (CFG-02 only) touches exactly two existing files with small, surgical edits. There is no new source file requiring a cross-codebase analog search — the closest and most correct "analog" for each edit is the sibling pattern already present in the same file (the `protoc-gen-*` `go install` lines in `Taskfile.yml`, the `YAMLFMT_VERSION`/`ADDLICENSE_VERSION` env-var pattern in `ci.yml`). RESEARCH.md's Architecture Patterns section already contains verified, repo-grounded diffs; this document re-grounds those diffs against the actual current file contents (line numbers confirmed by direct read, 2026-07-08) so the planner can cite exact line ranges.

## File Classification

| Modified File | Role | Data Flow | Closest Analog (in-repo) | Match Quality |
|---------------|------|-----------|---------------------------|----------------|
| `Taskfile.yml` | config (build/task-runner definition) | batch (one-shot local tool install) | Same file: `protoc-gen-go`/`protoc-gen-connect-go` `go install` lines (`Taskfile.yml:360-361`) inside the same `tools:` task | exact — same task, same install-line shape, same `{{.VERSION_VAR}}` templating convention |
| `.github/workflows/ci.yml` | config (CI workflow definition) | batch (one-shot CI tool install step) | Same file: `YAMLFMT_VERSION`/`ADDLICENSE_VERSION` `env:`-block + `go install ...@${{ env.VAR }}` pattern (`ci.yml:32-33`, `ci.yml:79-80`) | exact — same job, same step, same env-var-to-install-line shape (this is the pattern CFG-02 is *removing* for golangci-lint specifically, per D-03) |

## Pattern Assignments

### `Taskfile.yml` — add global var + new leaf task + edit `tools:` task

**Analog A (vars block convention):** `Taskfile.yml:3-19` (global `vars:` block)

Current content (lines 1-19):
```yaml
# https://taskfile.dev
version: "3"
vars:
  BINARY_NAME: specgraph
  MAIN_PKG: ./cmd/specgraph
  # Version stamping for non-goreleaser builds (local dev, hand-built images).
  # The vars live in `package main`, so the linker symbol is `main.version` /
  # `main.commit` — the full import path is silently ignored by the Go linker.
  # Falls back to dev/none when git is unavailable (preserving prior behavior).
  VERSION:
    sh: v=$(git describe --tags --always --dirty 2>/dev/null) && echo "${v#v}" || echo dev
  COMMIT:
    sh: git rev-parse --short HEAD 2>/dev/null || echo none
  LDFLAGS: -X main.version={{.VERSION}} -X main.commit={{.COMMIT}}
  # Local buf codegen plugins (pinned to match gen/ output + go.mod). Kept local
  # rather than remote (buf.build/...) so `buf generate` never hits the Buf
  # Schema Registry, which rate-limits unauthenticated requests in CI (spgr-3zs0).
  PROTOC_GEN_GO_VERSION: v1.36.11
  PROTOC_GEN_CONNECT_GO_VERSION: v1.19.1
```

**Copy this convention:** append `GOLANGCI_LINT_VERSION: v2.12.1` immediately after `PROTOC_GEN_CONNECT_GO_VERSION` (line 19), following the identical flat `KEY: value` shape — no `sh:` indirection needed since this is a static pin, matching `PROTOC_GEN_GO_VERSION`/`PROTOC_GEN_CONNECT_GO_VERSION` rather than the dynamic `VERSION`/`COMMIT` vars above them.

**Analog B (install-line + templating convention):** `Taskfile.yml:354-362` (`tools:` task)

Current content:
```yaml
  # Tools
  tools:
    desc: Install development tools
    cmds:
      - brew install golangci-lint gofumpt lefthook actionlint goreleaser dprint cocogitto rumdl yamlfmt buf beads
      # Local buf codegen plugins (see buf.gen.yaml). The TypeScript es plugin
      # (protoc-gen-es) is installed into web/node_modules via `pnpm install`.
      - go install google.golang.org/protobuf/cmd/protoc-gen-go@{{.PROTOC_GEN_GO_VERSION}}
      - go install connectrpc.com/connect/cmd/protoc-gen-connect-go@{{.PROTOC_GEN_CONNECT_GO_VERSION}}
      - echo "Tools installed. Run 'task hooks:install' to set up git hooks."
```

**Copy this convention, two edits:**
1. Remove `golangci-lint` from the `brew install` line (line 357) — leaves `brew install gofumpt lefthook actionlint goreleaser dprint cocogitto rumdl yamlfmt buf beads`.
2. Add a new `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{.GOLANGCI_LINT_VERSION}}` line, positioned alongside the two existing `protoc-gen-*` `go install` lines (matches their exact templating shape: `go install <module path>@{{.<VAR_NAME>}}`).

**New leaf task — no existing analog in this Taskfile** (confirmed via `rg -n "silent:" Taskfile.yml` → zero hits), so this is genuinely new shape, not a copy-paste of an existing task. Place it adjacent to `tools:` (e.g. immediately after, following the `tools:` → `hooks:install`/`hooks:uninstall` naming convention already used elsewhere in the file for `tools:`-prefixed and colon-namespaced task families):
```yaml
  tools:golangci-lint-version:
    desc: Print the pinned golangci-lint version (single source of truth; ci.yml reads this)
    silent: true
    cmds:
      - echo '{{.GOLANGCI_LINT_VERSION}}'
```
`silent: true` at the task level is required — without it, Task's command-echo banner contaminates the captured stdout when ci.yml calls this via command substitution (see RESEARCH.md Pitfall 1).

---

### `.github/workflows/ci.yml` — remove independent env var, resolve version via Task

**Analog (env-var-driven install-line convention, the pattern being removed for golangci-lint specifically):** `ci.yml:24-37` (workflow-level `env:` block) and `ci.yml:76-80` (`build-and-test` job, "Install Go tools" step)

Current content (env block, lines 24-37):
```yaml
env:
  GO_VERSION: "1.26.4"
  NODE_VERSION: "22"
  GOLANGCI_LINT_VERSION: v2.12.1
  # Local buf codegen plugins — pinned to match gen/ + go.mod. Kept local (not
  # remote buf.build/... plugins) so codegen never hits the BSR rate limit (spgr-3zs0).
  PROTOC_GEN_GO_VERSION: v1.36.11
  PROTOC_GEN_CONNECT_GO_VERSION: v1.19.1
  YAMLFMT_VERSION: v0.21.0
  ADDLICENSE_VERSION: v1.2.0
  RUMDL_VERSION: 0.1.42
  RUMDL_SHA256: "749d461e5add3c88f60dfb3b275f8de04c63bed98189721dcf075968dc3d1a94"
  DPRINT_VERSION: "0.52.0"
  DPRINT_SHA256: "3172f1564e4984ab0b511d5872b128ac91429a9e32a2db95977f3611a524d224"
```

Current content ("Install Go tools" step, lines 76-80, `build-and-test` job):
```yaml
      - name: Install Go tools
        run: |
          go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${{ env.GOLANGCI_LINT_VERSION }}
          go install github.com/google/yamlfmt/cmd/yamlfmt@${{ env.YAMLFMT_VERSION }}
          go install github.com/google/addlicense@${{ env.ADDLICENSE_VERSION }}
```

**Copy this convention, two edits:**
1. Delete line 27 (`GOLANGCI_LINT_VERSION: v2.12.1`) from the `env:` block — `YAMLFMT_VERSION`/`ADDLICENSE_VERSION`/etc. stay untouched (D-05 scopes this to golangci-lint only).
2. In the "Install Go tools" step, replace the golangci-lint install line with:
   ```yaml
   go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(task tools:golangci-lint-version)"
   ```
   Keep the `yamlfmt`/`addlicense` lines exactly as-is (they still read `${{ env.YAMLFMT_VERSION }}` / `${{ env.ADDLICENSE_VERSION }}` — unaffected by this change).

**Precondition already satisfied — Task is available at this point in the job:** `ci.yml:63-67`, the "Install Task" step (`arduino/setup-task@...`), runs before "Install buf codegen plugins" (line 72-75) and before "Install Go tools" (line 76-80). No new step is needed to make `task` available for the command substitution.

**Job-scope guard (Pitfall 2 from RESEARCH.md):** confirmed only `build-and-test` (starting line 39) has an "Install Go tools" step. `e2e` and `e2e-agent` jobs (later in the file) never install golangci-lint — do not touch those job definitions.

---

## Shared Patterns

### Version-pin templating (`{{.VAR}}` in Taskfile.yml, `${{ env.VAR }}` in ci.yml)
**Source:** `Taskfile.yml:18-19` (`PROTOC_GEN_GO_VERSION`, `PROTOC_GEN_CONNECT_GO_VERSION`) and `ci.yml:30-33` (`PROTOC_GEN_GO_VERSION`, `PROTOC_GEN_CONNECT_GO_VERSION`, `YAMLFMT_VERSION`, `ADDLICENSE_VERSION`)
**Apply to:** Both edits above — the new `GOLANGCI_LINT_VERSION` var in Taskfile.yml follows the flat-string convention of its siblings; the ci.yml side switches from that same env-var convention to command substitution specifically for golangci-lint per D-03, while every other tool in ci.yml keeps the env-var convention unchanged.

### Silent print-task for cross-boundary value transfer
**Source:** No existing analog in this repo (novel to this phase) — grounded instead in Taskfile.dev's documented `silent:` field (cited in RESEARCH.md, Sources → Primary). Do not search further for an in-repo precedent; RESEARCH.md already confirmed none exists.
**Apply to:** `Taskfile.yml`'s new `tools:golangci-lint-version` task only. Do not give it `deps:` on `generate`/`web:build` — it must stay a dependency-free leaf (RESEARCH.md Anti-Patterns).

## No Analog Found

None — both files are direct edits to existing, in-repo config with adjacent sibling patterns to copy from. No new source file is created in this phase.

## Metadata

**Analog search scope:** `Taskfile.yml` (full file structure via targeted reads: lines 1-25, 200-220, 350-365), `.github/workflows/ci.yml` (lines 1-85, covering `env:` block and `build-and-test` job through "Install Go tools")
**Files scanned:** 2 (both edited files, read directly; no broader codebase search needed since RESEARCH.md already exhaustively confirmed no other file references a golangci-lint version to drift against)
**Pattern extraction date:** 2026-07-08
