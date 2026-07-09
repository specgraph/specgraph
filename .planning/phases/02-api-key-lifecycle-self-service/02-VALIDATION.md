---
phase: 2
slug: api-key-lifecycle-self-service
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-09
---

# Phase 2 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (unit + `//go:build integration` testcontainers) |
| **Config file** | Taskfile.yml (`task test`, `task test:integration`) |
| **Quick run command** | `task test` |
| **Full suite command** | `task check` (fmt → license → lint → build → unit) then `task pr-prep` (+ integration/e2e) |
| **Estimated runtime** | ~60–120 seconds (unit); integration requires Docker |

---

## Sampling Rate

- **After every task commit:** Run `task test`
- **After every plan wave:** Run `task check`
- **Before `/gsd-verify-work`:** Full suite must be green (`task pr-prep` for DB-touching changes)
- **Max feedback latency:** 120 seconds (unit); integration on Docker as needed

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 2-01-01 | 01 | 1 | AUTH-03 | T-2-01 / — | `"self"` in knownVerbs + apikey.self-only drift test passes | unit | `go test ./internal/auth/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/auth/actions_test.go` — add `"self"` to BOTH hard-coded verb lists (`TestActionNames_AllParseToKnownVerb` + mirror)
- [ ] Owner-scoped storage unit tests — `RevokeAPIKeyForUser`/`RotateAPIKeyForUser`/`GetAPIKeyForUser` (NotFound on non-owner)
- [ ] Adversarial security tests — laundering floor (create+rotate), anti-key-chaining (`Source=="apikey"`), quota TOCTOU, CSRF double-submit

*Framework already present — go test + testcontainers. No install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Web "MCP Keys" panel one-time reveal modal | AUTH-03 | SvelteKit UI interaction | Log in via `specgraph_session` cookie, create key, confirm single reveal + CSRF-protected mutations |
| Operator forced re-sync immediacy on standing keys | AUTH-02 | Cross-session propagation | `auth user resync <id> --role <lower>`, then call MCP with standing key → reduced privilege without re-login |

*Remaining phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
