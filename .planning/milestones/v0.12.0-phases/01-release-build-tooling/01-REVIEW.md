---
phase: 01-release-build-tooling
reviewed: 2026-07-08T00:00:00Z
depth: standard
files_reviewed: 2
files_reviewed_list:
  - Taskfile.yml
  - .github/workflows/ci.yml
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 01: Code Review Report

**Reviewed:** 2026-07-08
**Depth:** standard
**Files Reviewed:** 2
**Status:** issues_found

## Summary

Reviewed the CFG-02 change that pins `golangci-lint`'s version to a single `GOLANGCI_LINT_VERSION` var in `Taskfile.yml`, exposes it via a new silent leaf task `tools:golangci-lint-version`, and switches `ci.yml`'s "Install Go tools" step to resolve the version through `$(task tools:golangci-lint-version)` instead of its own `env.GOLANGCI_LINT_VERSION`.

Verified empirically (ran `task tools:golangci-lint-version` locally and inspected raw bytes with `od -c`): output is exactly `v2.12.1\n` with no ANSI escapes, banner lines, or trailing whitespace — `silent: true` correctly suppresses Task's own "task: [name] cmd" echo line while leaving the command's real stdout intact. The command-substitution wiring in `ci.yml` is sound.

Job ordering is correct: in the `build-and-test` job, "Install Task" (`ci.yml:62-66`) runs before "Install Go tools" (`ci.yml:75-79`), so the `task` binary is on `PATH` when the substitution executes. The `e2e` and `e2e-agent` jobs don't reference `tools:golangci-lint-version` at all, so they're unaffected.

Supply-chain integrity is reasonable: `go install ...@v2.12.1` is checksum-verified against GOSUMDB by default, matching the trust model already used for the neighboring `protoc-gen-go`/`protoc-gen-connect-go` installs, and is a strict improvement over the previous unpinned `brew install golangci-lint` (which could silently drift between local dev and CI, and between CI runs, on Homebrew's own update cadence).

No critical issues found. One warning about failure-mode legibility in the CI command substitution, and two info-level notes about incomplete generalization of the single-source-of-truth pattern and a still-unpinned brew install list.

## Warnings

### WR-01: No explicit failure handling if `task tools:golangci-lint-version` fails inside the command substitution

**File:** `.github/workflows/ci.yml:75-79`
**Issue:** The step runs `go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(task tools:golangci-lint-version)"`. If the `task` invocation itself fails (e.g. a future Taskfile syntax error, a renamed/removed leaf task, or `task` not resolving `PATH` correctly in some future runner image change), bash's `-e` does not abort on a failed command *substitution* — only on the failure of the outer simple command. The substitution silently yields an empty string, producing `go install ".../golangci-lint@"`, which then fails with a generic "invalid version" style error from `go install` rather than a clear "failed to resolve golangci-lint version from Taskfile.yml" message. The build still fails (nothing is silently broken), but the CI log obscures the actual root cause, adding debugging friction for whoever next touches this pipeline.
**Fix:**
```yaml
      - name: Install Go tools
        run: |
          GOLANGCI_LINT_VERSION="$(task tools:golangci-lint-version)"
          if [ -z "$GOLANGCI_LINT_VERSION" ]; then
            echo "::error::failed to resolve golangci-lint version via 'task tools:golangci-lint-version'" >&2
            exit 1
          fi
          go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$GOLANGCI_LINT_VERSION"
          go install github.com/google/yamlfmt/cmd/yamlfmt@${{ env.YAMLFMT_VERSION }}
          go install github.com/google/addlicense@${{ env.ADDLICENSE_VERSION }}
```

## Info

### IN-01: Single-source-of-truth pattern not applied to the two other duplicated version pins in the same files

**File:** `Taskfile.yml:18-19`, `.github/workflows/ci.yml:29-30`
**Issue:** `PROTOC_GEN_GO_VERSION` and `PROTOC_GEN_CONNECT_GO_VERSION` are declared independently in both `Taskfile.yml`'s `vars:` block and `ci.yml`'s `env:` block — the exact "drift vector" (per the `tools:golangci-lint-version` task's own `desc:`) that this phase closed for `GOLANGCI_LINT_VERSION`. They happen to still match (`v1.36.11` / `v1.19.1` in both files) today, but nothing enforces that going forward; a future bump to one file without the other will silently drift, same as the bug this phase fixed.
**Fix:** Follow-up task: add `tools:protoc-gen-go-version` / `tools:protoc-gen-connect-go-version` leaf tasks (or a single `tools:print-versions` with multiple outputs) and switch the corresponding `ci.yml` `go install` lines to command substitution, mirroring the pattern just established for golangci-lint.

### IN-02: `task tools` still installs several tools via unpinned `brew install`

**File:** `Taskfile.yml:358`
**Issue:** `brew install gofumpt lefthook actionlint goreleaser dprint cocogitto rumdl yamlfmt buf beads` remains unpinned. This phase's stated goal (CFG-02) was pinning `golangci-lint` specifically, so this is not a regression, but it's worth noting these tools have the same latent local-dev-vs-CI drift potential that motivated this phase, and `rumdl`/`dprint`/`yamlfmt`/`addlicense` are already pinned independently in `ci.yml`'s `env:` block (`RUMDL_VERSION`, `DPRINT_VERSION`, `YAMLFMT_VERSION`, `ADDLICENSE_VERSION`) with no corresponding `Taskfile.yml` var — the reverse of the golangci-lint situation (CI pinned, local dev unpinned via brew's "whatever is current" version).
**Fix:** Out of scope for this review; flagging for a future CFG follow-up phase rather than blocking this change.

---

_Reviewed: 2026-07-08_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
