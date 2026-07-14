---
phase: 6
slug: mcp-authoring-self-teaching-path
status: locked
nyquist_compliant: true
wave_0_complete: false
created: 2026-07-14
---

# Phase 6 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `06-RESEARCH.md` § Validation Architecture.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Unit framework** | Go `testing` (std). Content-drift precedent: `internal/authoring/drift_test.go` (`TestContentProtoDrift`). |
| **e2e framework** | Ginkgo v2 / Gomega, `//go:build e2e`, MCP client `github.com/mark3labs/mcp-go/client`. |
| **e2e MCP harness** | `e2e/api/skills_test.go:skillsMCPClient` — real in-process `mcp.NewServer(mcpClient)` in an `httptest.Server`; MCP-only (no CLI/ConnectRPC client) per D-08. |
| **Config file** | None — Go test discovery; `//go:build e2e` gates the e2e suite. Postgres via testcontainers (`pgvector/pgvector:pg18`, Docker for e2e). |
| **Quick run command** | `task test` |
| **Full suite command** | `go test -tags e2e ./e2e/api/...` (Docker) / `task pr-prep` |
| **Estimated runtime** | Quick ~15–30s; e2e ~1–3 min |

---

## Sampling Rate

- **After every task commit:** Run `task test` (unit + content-drift reference assertions)
- **After every plan wave:** Run `go test -tags e2e ./e2e/api/...`
- **Before `/gsd-verify-work`:** Full suite + MCP-only e2e (`-run MCPOnly`) must be green. No CLI-path fallback permitted (D-08).
- **Max feedback latency:** 30s (quick loop) / ~3 min (wave/gate loop)

---

## Per-Task Verification Map

| Ref | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|-----|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| #1 skills describe MCP round-trip | 1 | MCP-01 | — | N/A | unit (content assert) | `go test ./internal/mcp/skills/... ./internal/authoring/...` | ❌ W0 | ⬜ pending |
| #2 full funnel MCP-only | 3 | MCP-01 | T-6-01 | Friendly-YAML parser rejects unknown enum/layer | e2e | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ❌ W0 | ⬜ pending |
| #3 constitution approved MCP-only | 2/3 | MCP-01 | T-6-01 | `*FromString` mappers error on UNSPECIFIED, no default-write | e2e | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ❌ W0 | ⬜ pending |
| #4 skills_get/search reference MCP path | 1 | MCP-01 | — | N/A | unit (content-level, D-09) | `go test ./internal/mcp/skills/...` | ❌ W0 | ⬜ pending |
| prime-reliability | 1/3 | MCP-01 | T-6-03 | Handler errors sanitized (no raw internals) | e2e (smoke) | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ⚠️ partial | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `e2e/api/mcp_only_authoring_test.go` — MCP-client-only funnel (spark→approve) + constitution friendly-YAML write + `specgraph://prime` empty-state smoke. Covers #2, #3, prime-reliability.
- [ ] Content-drift / reference assertion for rewritten skills — extend the `TestContentProtoDrift` precedent (`internal/authoring/drift_test.go`) or sibling under `internal/mcp/skills`. Covers #1, #4.
- [ ] (Conditional — only if funnel friendly-input layer is added) `internal/authoring/load/*_test.go` — friendly-YAML→proto mapping for Spark/Shape/Specify/Decompose incl. enum mappers (UNSPECIFIED→error).

*No new framework install needed — Go `testing` + existing Ginkgo/Gomega e2e cover all phase behaviors.*

---

## Manual-Only Verifications

**None — all phase behaviors have automated verification.** D-08 mandates an automated MCP-only e2e gate; criteria #1/#4 are covered by content-level unit assertions.

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 180s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
