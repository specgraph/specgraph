---
phase: 7
slug: authoring-lifecycle-semantics
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-14
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (unit) + Ginkgo/Gomega (e2e, `-tags e2e`) + testcontainers (postgres integration) |
| **Config file** | Taskfile.yml (task test / task test:integration / task test:e2e) |
| **Quick run command** | `task test` |
| **Full suite command** | `task pr-prep` (check → integration → e2e; requires Docker) |
| **Estimated runtime** | ~60–180 seconds (unit); integration/e2e longer (Docker) |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task test:integration` (postgres storage waves) / `task test:e2e` (MCP surface waves)
- **Before `/gsd-verify-work`:** Full suite must be green (`task pr-prep`)
- **Max feedback latency:** 180 seconds (unit); Docker suites out-of-band

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| _(planner fills per-task rows from PLAN.md; see RESEARCH.md § Validation Architecture)_ | | | | | | | | | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] _(planner determines — existing test infrastructure is present; likely "Existing infrastructure covers all phase requirements.")_

*If none: "Existing infrastructure covers all phase requirements."*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| _(planner fills; goal is full automated coverage via MCP-only e2e + storage integration)_ | | | |

*If none: "All phase behaviors have automated verification."*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
