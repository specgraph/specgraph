---
phase: 8
slug: authoring-conversation-fidelity
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-15
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (Go standard testing; Ginkgo/Gomega for e2e) |
| **Config file** | none — `Taskfile.yml` defines `task test`, `task test:integration`, `task test:e2e` |
| **Quick run command** | `task test` (unit; excludes `//go:build integration` and `//go:build e2e`) |
| **Full suite command** | `task pr-prep` (check → integration → e2e; requires Docker running) |
| **Estimated runtime** | ~30–60s unit; several minutes for integration/e2e (testcontainers `pgvector:pg18`) |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task check` (fmt → license → lint → build → unit)
- **Before `/gsd-verify-work`:** Full suite (`task pr-prep`) must be green — **requires Docker running**
- **Max feedback latency:** 60 seconds (unit tier)

---

## Per-Task Verification Map

> Populated by the planner against the final task IDs. Each task must map to an
> automated `task test` / `task test:integration` command or declare a Wave 0
> dependency. The integration + MCP-only e2e gate (Success Criteria 1–4) is the
> backstop that recording is protocol-enforced, not agent-discretionary.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 08-01-xx | 01 | 1 | CONV-01 | — | Approve-accept records a conversation under the stage tx | integration | `task test:integration` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Integration/e2e test asserting every funnel stage (shape/specify/decompose/approve) records a non-empty conversation — the "missing conversation cannot silently pass" backstop (Success Criteria 3).
- [ ] Docker running precondition documented in the plan's `<verify>` for integration/e2e tasks.

*Existing unit infrastructure (`go test`) covers the wiring/validator/loader tasks.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Skill/description prose accurately reflects the enforced-recording contract | CONV-01 | Prose review is not machine-assertable beyond token-drift checks | Read updated `internal/authoring/content/stage-*.md` and skill descriptions; confirm no "record if you choose" discretionary language remains |

*All functional phase behaviors have automated (unit + integration/e2e) verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
