---
phase: 1
slug: release-build-tooling
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-08
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | None applicable — `Taskfile.yml`/`ci.yml` are tooling configuration, not application code. This repo's `go test` suite does not exercise YAML/task-runner config. |
| **Config file** | `Taskfile.yml`, `.github/workflows/ci.yml` (the artifacts under test) |
| **Quick run command** | `task tools:golangci-lint-version` (prints the pinned value — smoke-tests the templating) |
| **Full suite command** | `task tools` then `golangci-lint version` (confirm installed matches printed pinned value) |
| **Estimated runtime** | ~10 seconds (quick); ~30-60 seconds (full, dominated by `go install`) |

---

## Sampling Rate

- **After every task commit:** Run `task tools:golangci-lint-version`
- **After every plan wave:** Run `task tools` then `golangci-lint version` and diff against the printed pinned value
- **Before `/gsd-verify-work`:** Push and inspect the `build-and-test` job's "Install Go tools" step log in an actual CI run to confirm it resolves the same pinned version from `Taskfile.yml`
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | CFG-02 | — | `task tools` installs golangci-lint at the exact pinned version, via the same `go install` method CI uses | smoke | `task tools:golangci-lint-version && task tools && golangci-lint version` — confirm printed and installed versions match | ✅ | ⬜ pending |
| 01-01-02 | 01 | 1 | CFG-02 | — | `ci.yml`'s install step resolves the version from `Taskfile.yml`, not an independent literal | structural | `! rg -q 'GOLANGCI_LINT_VERSION:\s*v[0-9]' .github/workflows/ci.yml` (asserts the old independent declaration is gone) | ✅ | ⬜ pending |
| 01-01-03 | 01 | 1 | CFG-02 | — | No other file in the repo still declares a separate golangci-lint version to drift against | structural | `rg -n 'golangci-lint\|GOLANGCI_LINT' --type-not go -g '!gen/**'` — only `Taskfile.yml` and `ci.yml` should reference a version | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

*None: Existing infrastructure covers all phase requirements — this is a structural/config verification, not a code-coverage gap. No test framework install is needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| CI's "Install Go tools" step installs the version templated from `Taskfile.yml` | CFG-02 | Requires an actual GitHub Actions run to observe the resolved log line; not reproducible from a local shell alone | Push the branch, open the `build-and-test` job, confirm the "Install Go tools" step log shows the same version string as `task tools:golangci-lint-version` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
