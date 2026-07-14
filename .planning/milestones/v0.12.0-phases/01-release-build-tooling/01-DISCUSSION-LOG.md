# Phase 1: Release & Build Tooling - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-08
**Phase:** 1-Release & Build Tooling
**Areas discussed:** Scope correction (REL-01/CFG-01 closure), Pinning mechanism, Version source of truth, Drift detection, Other task tools scope

---

## Scope correction (REL-01 / CFG-01 closure)

Before any gray-area discussion, verification against the actual repo (git log, PR history, direct code reads) found that REL-01 and CFG-01 were already fully implemented on `main`, despite PROJECT.md/beads status showing them as in-progress/pending.

| Option | Description | Selected |
|--------|-------------|----------|
| Close them, discuss only CFG-02 | Mark REL-01/CFG-01 done now; focus discussion entirely on CFG-02 | ✓ |
| Still run a formal verification pass | Route through /gsd-secure-phase or /gsd-validate-phase style verification first | |
| Let me look at this myself first | Pause for independent user review before deciding | |

**User's choice:** Close them, discuss only CFG-02.
**Notes:** PROJECT.md, REQUIREMENTS.md, and ROADMAP.md were updated in this session to mark REL-01 and CFG-01 as Done, with the supporting evidence (PR #981, `v0.12.0` release verification, `internal/config/global.go` implementation) recorded inline.

---

## Pinning mechanism

| Option | Description | Selected |
|--------|-------------|----------|
| Switch to `go install ...@version` | Same install method CI already uses (ci.yml:78); guarantees identical version resolution | ✓ |
| Pin the brew formula/tap to an exact version | Homebrew doesn't version most formulas long-term — not durable | |
| Keep brew, add a version-check gate | Doesn't guarantee a matching install, just surfaces drift | |

**User's choice:** Switch `task tools` to `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@<version>`.
**Notes:** None.

---

## Version source of truth

| Option | Description | Selected |
|--------|-------------|----------|
| Taskfile var, CI reads it | One file (Taskfile.yml) owns the version; CI's install step reads that value | ✓ |
| Shared version file, both read it | New file (e.g. `.tool-versions`) both configs read via shell substitution | |
| Keep CI's env var as source, Taskfile reads it | Doesn't work directly — Taskfile.yml can't read GitHub Actions env outside CI | |

**User's choice:** Taskfile var, CI reads it.
**Notes:** Exact mechanism for CI to read the Taskfile-owned value left to research/planning (Claude's Discretion in CONTEXT.md).

---

## Drift detection

| Option | Description | Selected |
|--------|-------------|----------|
| No extra check needed | Fixing the install path already closes the drift vector structurally | ✓ |
| Add a version-check to `task doctor`/`task check` | Belt-and-suspenders for a pre-existing stale global install | |

**User's choice:** No extra check needed.
**Notes:** None.

---

## Other task tools scope

| Option | Description | Selected |
|--------|-------------|----------|
| No — golangci-lint only | CFG-02 names only golangci-lint; no other tool has a CI-side pin to drift against yet | ✓ |
| Yes — pin all task tools | Broader fix while the pattern is fresh, but beyond CFG-02's stated scope | |

**User's choice:** No — golangci-lint only.
**Notes:** Recorded as a Deferred Idea in CONTEXT.md for a possible future requirement.

## Claude's Discretion

- Exact Taskfile→CI version-reading mechanism (e.g. a `task tools:golangci-lint-version` print helper vs. other wiring) — left for research/planning to pick based on Taskfile.dev's actual capabilities.

## Deferred Ideas

- Pinning other `task tools`-installed dev tools (gofumpt, lefthook, actionlint, goreleaser, dprint, cocogitto, rumdl, yamlfmt, buf) the same way — no matching CI-side pin exists for these yet; would need its own requirement if picked up later.
