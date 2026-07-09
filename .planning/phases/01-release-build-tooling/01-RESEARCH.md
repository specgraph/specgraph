# Phase 1: Release & Build Tooling - Research

**Researched:** 2026-07-08
**Domain:** Build tooling version-pinning (Taskfile.dev + GitHub Actions)
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-00 (scope correction):** REL-01 (single-job goreleaser release model) and CFG-01 (koanf layered config loader) are already fully implemented on `main` — verified directly against the repo. Planning for this phase should scope to CFG-02 only — do not re-plan or re-verify REL-01/CFG-01 work.
- **D-01:** Switch `task tools` (`Taskfile.yml:357`) from `brew install golangci-lint` to `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>` — the same install method CI already uses (`ci.yml:78`). This guarantees identical version resolution between local and CI; brew formula pinning was rejected as non-durable (old versions get pruned from core taps).
- **D-02:** No CI-side change to *how* golangci-lint is installed — only local `task tools` changes to match CI's existing method.
- **D-03:** The pinned version string becomes a single Taskfile.yml var; `ci.yml`'s install step reads that value instead of keeping its own independent `GOLANGCI_LINT_VERSION` env var. The exact mechanism for CI to read a Taskfile var is left to research/planning.
- **D-04:** No separate version-check step in `task check`/`task doctor`. Fixing the install path (D-01) closes the drift vector structurally.
- **D-05:** Pin golangci-lint only. Do NOT extend this to other brew-installed dev tools (gofumpt, lefthook, actionlint, goreleaser, dprint, cocogitto, rumdl, yamlfmt, buf) — none currently have a CI-side pin to drift against.

### Claude's Discretion

- Exact shape of the Taskfile→CI version-reading mechanism (D-03) — e.g. a `task tools:golangci-lint-version` print helper vs. some other wiring. Research/planning should pick whatever fits Taskfile.dev's actual capabilities cleanly.

### Deferred Ideas (OUT OF SCOPE)

- Pinning other `task tools`-installed dev tools (gofumpt, lefthook, actionlint, goreleaser, dprint, cocogitto, rumdl, yamlfmt, buf) the same way golangci-lint is being pinned here — explicitly deferred (D-05), no existing CI-side pin for these to drift against yet. Would need its own requirement if picked up later.

</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| REL-01 | Adopt the single-job goreleaser-owns-release model | Already shipped on `main` (PR #981). Out of scope per CONTEXT.md D-00 — no research effort spent beyond confirming the CONTEXT.md claim is consistent with what this session observed. |
| CFG-01 | Adopt koanf for layered config + env provider | Already shipped on `main` (`internal/config/global.go`). Out of scope per CONTEXT.md D-00 — no research effort spent. |
| CFG-02 | Pin `task tools`' golangci-lint to match the CI version | Full research target of this document. See Architecture Patterns, Code Examples, and Common Pitfalls sections below. |

</phase_requirements>

## Summary

This phase's only open requirement is CFG-02: `task tools` (Taskfile.yml:357) installs golangci-lint via unpinned `brew install`, while CI (ci.yml:78) installs a pinned version via `go install ...@${{ env.GOLANGCI_LINT_VERSION }}`. Verified locally: the drift is real and already active — this environment's brew-installed `golangci-lint` reports **v2.12.2**, while CI is pinned to **v2.12.1**. D-01/D-02 already lock the fix (switch `task tools` to the same `go install` method CI uses); the only open research question was D-03's exact mechanism — how a GitHub Actions workflow reads a value that lives in a Taskfile.yml var, so the version string has exactly one declaration instead of two.

Taskfile.dev (Task v3) has no CLI flag that exposes a variable's resolved value for external consumption — `task --list`/`--list-all --json` was checked directly against the official schema and returns only task metadata (`name`, `desc`, `summary`, `up_to_date`, `location`); vars are absent from that schema entirely. The correct, docs-backed mechanism is a small dedicated task with `silent: true` that does nothing but `echo` the global var, called from CI via ordinary shell command substitution (`$(task tools:golangci-lint-version)`). `silent: true` is not cosmetic here — it suppresses Task's own command-echo banner, which is what makes the task's stdout *only* the bare version string, safe to capture in a shell variable.

Supporting findings: no Renovate `customManagers` exist for any of ci.yml's tool-version env vars today (checked `.github/renovate.json5` directly), so moving `GOLANGCI_LINT_VERSION`'s source of truth out of ci.yml's `env:` block breaks no existing automation. The fix's blast radius is exactly two files (Taskfile.yml, ci.yml) plus one job within ci.yml (`build-and-test` — the only job that installs golangci-lint at all); lefthook's pre-commit hook (`golangci-lint run --new-from-rev=HEAD~1`) needs no separate change since it already shells out to whatever `golangci-lint` binary `task tools` puts on `PATH`.

**Primary recommendation:** Add a global `GOLANGCI_LINT_VERSION` var to Taskfile.yml's existing `vars:` block; drop `golangci-lint` from the `brew install` line in `task tools` and add a `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{.GOLANGCI_LINT_VERSION}}` line (matching the two existing `protoc-gen-*` `go install` lines already in that task); add a `silent: true` leaf task `tools:golangci-lint-version` that echoes the var; in ci.yml, delete the `GOLANGCI_LINT_VERSION` env var and change the `build-and-test` job's "Install Go tools" step to resolve the version inline via `$(task tools:golangci-lint-version)`.

## Architectural Responsibility Map

This phase has no browser/API/DB tiers — it's a build-tooling fix. The tiers below are adapted to this phase's actual shape: where the version string is declared vs. where it's consumed.

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Version source of truth | Taskfile.yml global `vars:` block | — | The only place both consumers (local install, CI install) can reference via templating; GitHub Actions has no native way to read an arbitrary sibling YAML file's field, so the value must live somewhere both sides already invoke a tool for (Task itself). |
| Local developer tool install | `task tools` (Taskfile.yml:354) | — | Existing single entrypoint devs already run to provision tools; unaffected in shape, only in which line installs golangci-lint. |
| CI tool install | `.github/workflows/ci.yml` `build-and-test` job, "Install Go tools" step | — | The only CI job that installs golangci-lint at all — `e2e` and `e2e-agent` jobs never run lint and have no such step. |
| Cross-boundary value transfer (Taskfile → CI) | New `task tools:golangci-lint-version` leaf task, consumed via shell command substitution | — | Task is already installed in the CI runner (`arduino/setup-task`, earlier in the same job) before this step runs — no new tool dependency introduced. |
| Pre-commit lint consistency (transitive beneficiary) | `lefthook.yaml`'s `golangci-lint run --new-from-rev=HEAD~1` hook | `task tools` (upstream) | Not touched directly — it shells out to whatever `golangci-lint` is on `PATH`, so it automatically inherits the correct pinned version once `task tools` is fixed. |

## Standard Stack

No new external dependencies. This phase reconfigures the install source and version-sync mechanism for tools already present in the codebase.

### Core (existing, versions confirmed this session)

| Tool | Version | Purpose | Confirmed via |
|------|---------|---------|----------------|
| Task (go-task/task) | 3.50.0 (local); CI installs `3.x` via `arduino/setup-task@...` | Build/task runner; both the install mechanism and the new version-export mechanism | `task --version` run directly [VERIFIED: local environment] |
| golangci-lint v2 | CI pinned: `v2.12.1`; local (brew, unpinned): `v2.12.2` | Go linter aggregator | `golangci-lint version` run directly, and direct read of `ci.yml:27` [VERIFIED: local environment + direct repo read] |
| Go toolchain | 1.26.5 (local); CI pins `1.26.4` | Runs `go install` for golangci-lint | `go version` run directly [VERIFIED: local environment] |
| Homebrew | 6.0.9 | Installs the other 9 `task tools` packages (unaffected by this change, per D-05) | `brew --version` run directly [VERIFIED: local environment] |

**Installation:** No new install commands. The change is entirely to *which* command installs golangci-lint (see Code Examples).

**Version verification:** Not applicable in the npm/pip/cargo sense — golangci-lint is a Go module installed via `go install`, already running successfully in this repo's own production CI (`ci.yml:78`) today. The only "new" fact this phase introduces is a second call site for that same install command (inside `task tools`), not a new package.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `go install ...@{{.GOLANGCI_LINT_VERSION}}` in `task tools` | Keep `brew install golangci-lint` but pin a specific formula version (`brew install golangci-lint@2.12.1`) | Rejected in CONTEXT.md D-01 — Homebrew core taps prune old formula versions, so a specific version isn't durably installable via brew months later. |
| A `silent: true` print task + command substitution | A custom `yq`/`grep`/`awk` scraper in ci.yml reading Taskfile.yml's YAML directly | Fragile — reimplements Task's own templating/parsing (including any future `ref:`/`sh:` indirection) in a second, unmaintained place; breaks silently on reformatting. See Don't Hand-Roll. |
| Inline command substitution (`$(task tools:golangci-lint-version)`) in the existing "Install Go tools" step | A separate step exporting to `$GITHUB_ENV`, preserving `${{ env.GOLANGCI_LINT_VERSION }}` syntax in the install step | Both work. `$GITHUB_ENV` export is slightly more consistent visually with the two sibling `${{ env.YAMLFMT_VERSION }}` / `${{ env.ADDLICENSE_VERSION }}` lines, at the cost of one extra step and exporting a var that nothing else in the job needs. Inline substitution is the smaller, more tightly-scoped diff. Recommended: inline; documented as the primary approach in Code Examples, with the `$GITHUB_ENV` variant noted as a viable alternative. |

## Package Legitimacy Audit

Not applicable in the standard sense — this phase installs no *new* package. `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` is already installed by this exact command in this repo's own CI (`ci.yml:78`) today; this phase only adds a second call site (`task tools`) for the identical, already-running install.

The automated `package-legitimacy check` seam supports only npm/pypi/crates ecosystems — Go modules are outside its scope, confirmed by running it directly against this package (it rejected the `go` ecosystem argument). Manual verification instead:

| Package | Registry | Age | Source Repo | Verdict | Disposition |
|---------|----------|-----|-------------|---------|-------------|
| `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` | Go module (proxy.golang.org) | v1 line since 2018; v2 line current | github.com/golangci/golangci-lint (one of the most widely-used Go lint aggregators; already vendored into this repo's CI) | OK (manual — ecosystem not covered by automated seam) | Approved — already running in production CI, not a new dependency |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                         Taskfile.yml
                    vars: GOLANGCI_LINT_VERSION: v2.12.1
                    (the ONLY literal declaration)
                              │
              ┌───────────────┴────────────────┐
              │                                 │
              ▼                                 ▼
   task tools:golangci-lint-version        task tools
   (silent:true leaf task)                 (installer task)
              │                                 │
   cmds: echo '{{.GOLANGCI_LINT_VERSION}}'  cmds: go install golangci-lint@{{.GOLANGCI_LINT_VERSION}}
              │                                 │
              ▼                                 ▼
      stdout: "v2.12.1"                 ~/go/bin/golangci-lint (local dev machine)
      (nothing else — banner
       suppressed by silent:true)
              │
              │ consumed via $(...) command substitution
              ▼
   .github/workflows/ci.yml
   build-and-test job → "Install Go tools" step
              │
   go install golangci-lint@$(task tools:golangci-lint-version)
              │
              ▼
   ~/go/bin/golangci-lint (CI runner)
```

Both leaf paths (local `~/go/bin`, CI runner `~/go/bin`) resolve from the single Taskfile.yml var — there is no second literal version string anywhere once ci.yml's `GOLANGCI_LINT_VERSION` env var is removed.

### Recommended Project Structure

Only two existing files change; no new files or directories.

```
Taskfile.yml                    # source of truth: new global var + new print task + edited tools: task
.github/workflows/ci.yml        # consumer: build-and-test job's "Install Go tools" step reads via task
```

### Pattern: Taskfile-as-source-of-truth, consumed by CI via a silent print task

**What:** A `vars:` entry lives in exactly one file (Taskfile.yml). A dedicated task with `silent: true` does nothing but echo that var. External callers (here, a GitHub Actions step) capture the task's stdout via ordinary shell command substitution.

**When to use:** Any time a CI workflow needs to consume a value that's more naturally owned by the project's task runner than duplicated into workflow YAML — applies narrowly here per D-05 (golangci-lint only), but the same shape would generalize to the other pinned tools if that scope is picked up later.

**Example — Taskfile.yml** (add to the existing global `vars:` block, alongside `PROTOC_GEN_GO_VERSION`):
```yaml
vars:
  # ...existing vars unchanged...
  PROTOC_GEN_GO_VERSION: v1.36.11
  PROTOC_GEN_CONNECT_GO_VERSION: v1.19.1
  GOLANGCI_LINT_VERSION: v2.12.1
```

New leaf task (placed near `tools:`):
```yaml
  tools:golangci-lint-version:
    desc: Print the pinned golangci-lint version (single source of truth; ci.yml reads this)
    silent: true
    cmds:
      - echo '{{.GOLANGCI_LINT_VERSION}}'
```

`tools:` task, edited (golangci-lint moved off the brew line, onto its own `go install` line matching the two lines already there):
```yaml
  tools:
    desc: Install development tools
    cmds:
      - brew install gofumpt lefthook actionlint goreleaser dprint cocogitto rumdl yamlfmt buf beads
      - go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@{{.GOLANGCI_LINT_VERSION}}
      - go install google.golang.org/protobuf/cmd/protoc-gen-go@{{.PROTOC_GEN_GO_VERSION}}
      - go install connectrpc.com/connect/cmd/protoc-gen-connect-go@{{.PROTOC_GEN_CONNECT_GO_VERSION}}
      - echo "Tools installed. Run 'task hooks:install' to set up git hooks."
```
```
// Source: Taskfile.dev official docs via Context7 (/llmstxt/taskfile_dev_llms-full_txt) —
// vars: block + {{.VAR}} templating, and silent: true at the task level
```

**Example — ci.yml** (`build-and-test` job only; `e2e`/`e2e-agent` jobs are unaffected, they have no such step):

Remove from the top-level `env:` block:
```yaml
  GOLANGCI_LINT_VERSION: v2.12.1   # DELETE — Taskfile.yml is now the sole source of truth
```

Change the "Install Go tools" step (Task is already installed earlier in this same job, via the existing "Install Task" step):
```yaml
      - name: Install Go tools
        run: |
          go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(task tools:golangci-lint-version)"
          go install github.com/google/yamlfmt/cmd/yamlfmt@${{ env.YAMLFMT_VERSION }}
          go install github.com/google/addlicense@${{ env.ADDLICENSE_VERSION }}
```

### Anti-Patterns to Avoid

- **Re-declaring the version string in ci.yml "just in case":** defeats the entire point of D-03 — if ci.yml keeps its own `GOLANGCI_LINT_VERSION`, the drift this phase exists to fix simply reappears the next time either value is bumped alone.
- **Scraping Taskfile.yml's YAML with `grep`/`awk`/`yq` from CI instead of asking Task itself:** reimplements Task's own templating logic in a second, unmaintained place (breaks on reformatting, doesn't understand `ref:`/`sh:` indirection). See Don't Hand-Roll.
- **Giving the print task `deps:` on `generate`/`web:build` (or any other shared dependency):** the print task must stay a dependency-free leaf. If it inherited deps from a copy-pasted task, calling it from CI just to read a version string would trigger proto generation or a web build as an expensive side effect.
- **Relying on `task --list --json` for the value:** confirmed directly against the documented schema — it returns task metadata only (`name`, `desc`, `summary`, `up_to_date`, `location`), never variable values. Not a viable mechanism.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|--------------|-----|
| Extracting a Taskfile var's value for use outside Task | A custom `yq`/`grep`/`awk` YAML scraper in ci.yml | A `silent: true` print task, called via `$(task tools:golangci-lint-version)` | Task already parses and templates its own file correctly (including `sh:`/`ref:` indirection if ever used); a shell-side scraper duplicates that logic and breaks silently on reformatting or if the var is ever changed to a dynamic form. |
| Detecting version drift between local and CI | A separate `task doctor`/`task check` step that independently reads both values and diffs them | Eliminate the second declaration entirely (already locked as D-03/D-04) | A check step only detects drift *after* a second source of truth already exists to drift from. Removing the second declaration prevents the class of bug structurally — this is exactly D-04's stated rationale. |

**Key insight:** The whole phase is really "stop having two places that can disagree," not "build tooling to notice when they do." Every hand-rolled alternative considered here (YAML scraping, a drift-check task) re-introduces or papers over a second source of truth instead of removing it.

## Common Pitfalls

### Pitfall 1: Forgetting `silent: true` corrupts the captured version string
**What goes wrong:** Without `silent: true`, Task prints its own command-echo banner (e.g. something like `task: [tools:golangci-lint-version] echo v2.12.1`) to output before running the command. If ci.yml captures this via `$(task tools:golangci-lint-version)`, the captured string is no longer a clean version — it's contaminated with Task's own banner text, and `go install ...@<corrupted string>` fails.
**Why it happens:** Task's default behavior echoes each command before running it — this is a feature for interactive use, not for tasks meant to be consumed programmatically.
**How to avoid:** Set `silent: true` on the print task itself (task-level, not just command-level, though either documented form works per Taskfile.dev docs).
**Warning signs:** `go install` errors with a version string that contains the word "task" or the literal command text; run `task tools:golangci-lint-version` standalone once and visually confirm the *only* output line is the bare version string.

### Pitfall 2: Editing the wrong CI job(s)
**What goes wrong:** ci.yml has three jobs (`build-and-test`, `e2e`, `e2e-agent`). Only `build-and-test` has an "Install Go tools" step that touches golangci-lint — `e2e` and `e2e-agent` only run `task build` and E2E test suites, never lint. Removing the top-level `GOLANGCI_LINT_VERSION` env var (declared once, at workflow scope) is safe for all jobs since only one ever referenced it, but a planner adding a *new* install step should not add one to `e2e`/`e2e-agent`.
**Why it happens:** The env var is declared at workflow scope (visible to all jobs), which can make it look like all three jobs consume it.
**How to avoid:** Confirmed directly by reading ci.yml — grep for "Install Go tools" and note it appears exactly once, inside `build-and-test`.
**Warning signs:** A diff that touches the `e2e` or `e2e-agent` job definitions for this change is scope creep.

### Pitfall 3: Assuming this breaks a Renovate auto-update flow
**What goes wrong:** It's reasonable to worry that moving `GOLANGCI_LINT_VERSION` out of ci.yml's `env:` block breaks an existing Renovate bot flow that auto-bumps it via PR.
**Why it happens:** Renovate's `github-actions` manager is real and configured in this repo (`.github/renovate.json5`), so the concern is plausible on its face.
**How to avoid:** Checked directly — `.github/renovate.json5` has no `customManagers` block at all. Renovate's built-in `github-actions` manager only tracks `uses: action@sha # version` pins (and the repo already uses `helpers:pinGitHubActionDigests` for that), not arbitrary `env:` string literals like `GOLANGCI_LINT_VERSION`. None of the sibling tool-version vars (`YAMLFMT_VERSION`, `ADDLICENSE_VERSION`, `RUMDL_VERSION`, `DPRINT_VERSION`, `PROTOC_GEN_GO_VERSION`, `PROTOC_GEN_CONNECT_GO_VERSION`) are Renovate-managed today either — all are manually bumped. Moving the source of truth breaks nothing that exists.
**Warning signs:** N/A — confirmed non-issue, included here so the planner doesn't re-spend time checking it.

## Code Examples

See the full diffs under Architecture Patterns → Pattern: Taskfile-as-source-of-truth. Both files (Taskfile.yml, ci.yml) are shown there with exact, working syntax grounded in this repo's current content and official Taskfile.dev docs.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| `task tools` installed golangci-lint via unpinned `brew install golangci-lint` | `task tools` installs via `go install .../golangci-lint@{{.GOLANGCI_LINT_VERSION}}`, matching CI's method exactly | This phase (CFG-02) | Local and CI always resolve the identical golangci-lint version; brew's rolling-latest formula can no longer cause drift. |
| ci.yml declared its own independent `GOLANGCI_LINT_VERSION` env var | ci.yml resolves the version from Taskfile.yml via `$(task tools:golangci-lint-version)` | This phase (CFG-02) | Exactly one place declares the version; a future bump requires editing only Taskfile.yml. |

**Deprecated/outdated:**
- Using `brew install <tool>` as a version-pinning strategy for CI-parity purposes — Homebrew core taps prune old formula versions from the tap over time, so a version pin via brew isn't durable months later (the exact reason D-01 rejected it as an alternative).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | golangci-lint v2.12.2 is the current absolute-latest upstream release (as of research time) | Summary, Standard Stack | None — this phase does not recommend bumping the pinned version (D-01/D-02 are about the install *mechanism*, not the version value). This claim is contextual color confirming the drift direction, not load-bearing for any recommendation. Sourced from WebSearch only (LOW confidence per the classify-confidence seam), not independently fetched from GitHub's releases page directly. |

**If this table is empty:** N/A — one low-risk, non-load-bearing assumption logged above; everything else in this research is either a direct repo/tool read (VERIFIED) or from official Taskfile.dev docs via Context7 (CITED, MEDIUM confidence per the classify-confidence seam).

## Open Questions

None blocking planning. D-03's exact mechanism (the one item CONTEXT.md explicitly left open) is resolved above with a docs-backed, repo-grounded recommendation.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Task (go-task) CLI | `task tools`, new print task, ci.yml's "Install Task" step | Yes | 3.50.0 local; CI installs `3.x` via `arduino/setup-task` | — |
| Go toolchain | `go install` for golangci-lint | Yes | 1.26.5 local; CI pins `1.26.4` | — |
| Homebrew | `task tools`'s remaining brew-installed tools (unaffected by this change) | Yes | 6.0.9 | — |
| golangci-lint (pre-fix baseline) | Confirms the drift this phase fixes | Yes (drifted) | 2.12.2 locally vs. 2.12.1 in CI | Resolved by this phase's fix, not a blocker |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | None applicable — Taskfile.yml/ci.yml are tooling configuration, not application code. This repo's `go test` framework does not exercise YAML/task-runner config. |
| Config file | `Taskfile.yml`, `.github/workflows/ci.yml` (the artifacts under test) |
| Quick run command | `task tools:golangci-lint-version` (prints the pinned value — smoke-tests the templating) |
| Full suite command | Run `task tools`, then `golangci-lint version`, and diff against the printed value; push and inspect the `build-and-test` job's "Install Go tools" step log in the actual CI run |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| CFG-02 | `task tools` installs golangci-lint at the exact pinned version, via the same method CI uses | smoke | `task tools:golangci-lint-version` then `task tools` then `golangci-lint version` — confirm the printed and installed versions match | N/A — new leaf task, not a source file |
| CFG-02 | ci.yml's install step resolves the version from Taskfile.yml, not an independent literal | structural | `! rg -q 'GOLANGCI_LINT_VERSION:\s*v[0-9]' .github/workflows/ci.yml` (asserts the old independent declaration is gone) | N/A |
| CFG-02 | No other file in the repo still declares a separate golangci-lint version to drift against | structural | `rg -n 'golangci-lint|GOLANGCI_LINT' --type-not go -g '!gen/**'` — confirmed today: only `Taskfile.yml` (task body + brew line) and `ci.yml` reference it; `lefthook.yaml` and docs reference the *command*, not a version | N/A |

### Sampling Rate

- **Per task commit:** `task tools:golangci-lint-version` (sub-second, catches templating typos immediately)
- **Per wave merge:** Full local `task tools` run + `golangci-lint version` check; push and inspect the actual CI log line
- **Phase gate:** Green CI run on `build-and-test` showing the new install line, plus a clean local `task check` using the freshly-pinned binary

### Wave 0 Gaps

None — this is a structural/config verification, not a code-coverage gap. No test framework install is needed.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | N/A — no auth surface touched |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A |
| V5 Input Validation | No | N/A — no user input processed; the only "input" is a static, repo-committed version string |
| V6 Cryptography | No | N/A |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Supply-chain tampering of the pinned lint tool binary | Tampering | Unchanged by this phase. `go install` resolves through the Go module proxy and checksum database (GOSUMDB), the same trust model ci.yml already relies on today for this exact install command. This phase only changes *where* the version string is declared, not *how* the binary is fetched or verified. |

## Sources

### Primary (MEDIUM confidence — CITED via Context7)
- Taskfile.dev official docs (`/llmstxt/taskfile_dev_llms-full_txt`, reference/schema + docs/guide pages) — global `vars:` block syntax, `{{.VAR}}` template referencing across tasks
- Taskfile.dev official docs (same library, docs/guide page) — `silent: true` at command/task/global level; note that shell redirection remains the correct tool for suppressing a command's *actual* output, distinct from suppressing the command echo
- Taskfile.dev official docs (same library, reference/cli page) — `--list`/`--list-all --json` output schema (`name`, `task`, `desc`, `summary`, `up_to_date`, `location`); confirms vars are never included

### Verified (HIGH confidence — direct repo/tool read, this session)
- `Taskfile.yml` (full file) — current `tools:` task (line 354-362), existing global `vars:` block conventions (lines 3-19), all other task-naming precedent
- `.github/workflows/ci.yml` (full file) — current `env:` block (lines 24-37), "Install Go tools" step (lines 76-80), confirmed this step exists only in the `build-and-test` job, not `e2e`/`e2e-agent`
- `.github/renovate.json5` (full file) — confirmed no `customManagers` block; `github-actions` manager scope limited to action-pin updates
- `lefthook.yaml:41` — confirmed the pre-commit `golangci-lint run --new-from-rev=HEAD~1` hook shells out to whatever binary is on `PATH`, no independent version reference
- Local environment, commands run directly: `task --version` → 3.50.0; `go version` → go1.26.5; `golangci-lint version` → 2.12.2 (confirms the drift CONTEXT.md described); `brew --version` → 6.0.9
- Repo-wide `rg` search for `golangci-lint|GOLANGCI_LINT` (excluding `gen/`, Go source) — confirmed only `Taskfile.yml` and `ci.yml` declare/pin a version; all other hits (`CLAUDE.md`, `README.md`, `docs/plans/**`, `docs/superpowers/**`) reference the *command* in prose/historical planning docs, not a live version pin

### Tertiary (LOW confidence — WebSearch only, not independently fetched)
- golangci-lint v2.12.2 being the absolute-latest upstream release at research time (A1 in Assumptions Log) — not load-bearing for any recommendation in this document

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every version claim is either a direct local-environment read or a direct repo-file read
- Architecture (CI-reading mechanism): HIGH for the individual primitives (vars/silent/JSON-schema absence, all CITED via official docs); the specific composition (print task + command substitution) is original synthesis grounded in those citations, not itself a named, independently-documented pattern — treat the composition as a well-grounded recommendation, not a "the docs say to do exactly this" citation
- Pitfalls: HIGH — each is grounded in either official docs (silent mode) or direct repo inspection (job scoping, Renovate config, lefthook behavior)

**Research date:** 2026-07-08
**Valid until:** 30 days for the mechanism (stable Taskfile v3 API, unlikely to change); the specific version numbers cited as "current" (CI pin v2.12.1, latest upstream v2.12.2) may drift faster and should be re-checked at execution time if this research is consumed later than a few weeks out.
