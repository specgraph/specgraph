---
phase: 1
slug: release-build-tooling
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-07-09
---

# Phase 1 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| repo write access → build/CI config | Anyone who can commit to `Taskfile.yml` or `ci.yml` controls what the local `task tools` and the CI `build-and-test` job execute. This is the same boundary that already governed `ci.yml` before this phase; this phase does not widen it. | Version-pin literals only (no secrets, no runtime data) |
| Go module proxy / GOSUMDB → installed golangci-lint binary | The pinned golangci-lint is fetched by `go install`, resolved and checksum-verified through the Go module proxy + checksum database — identical to the install command already running in CI before this phase. | Compiled Go binary, checksum-verified |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-1-01 | Tampering | `go install github.com/golangci/golangci-lint/v2/...` (Taskfile.yml + ci.yml install lines) | low | accept | Supply-chain trust model is unchanged by this phase — the binary is fetched and checksum-verified via the Go module proxy + GOSUMDB, exactly as CI's existing install already does. This phase only changes where the version string is declared, not how the binary is fetched or verified. golangci-lint is a well-established, already-vendored dependency (01-RESEARCH.md Package Legitimacy Audit: manual verdict OK, already running in production CI). | closed |
| T-1-02 | Tampering | `ci.yml` `$(task tools:golangci-lint-version)` command substitution | low | accept | The substituted value is a static, repo-committed version string echoed by a dependency-free leaf task. Injecting a malicious value would require write access to `Taskfile.yml` — the same trust boundary as editing `ci.yml` directly, which an attacker with that access already controls. No new privilege or external input is introduced. The double-quoted `@$(...)` form keeps the resolved value as a single argument (not re-evaluated as shell syntax). Hardened further post-review (commit `0f1beba7`, WR-01 fix): the resolved value is now captured into a variable and validated against `^v[0-9]+\.[0-9]+\.[0-9]+$` before use, failing fast on any malformed or empty output instead of silently passing it through. An independent automated security scan flagged this same line as a potential command-injection pattern during code review; on inspection the substitution was not exploitable as command injection at this call site (quoting prevents re-evaluation), but the WR-01 fix's format-validation guard directly addresses the scanner's suggested hardening. | closed |
| T-1-SC | Tampering | package-manager installs (npm/pip/cargo) | n/a | accept | Not applicable — this phase installs no new npm/pip/cargo package. The only install is a `go install` of an already-present, already-CI-installed Go module (no new dependency, no [ASSUMED]/[SUS]/[SLOP] package); Go modules are outside the automated package-legitimacy seam's scope and were manually verified OK in 01-RESEARCH.md. No legitimacy checkpoint required. | closed |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-1-01 | T-1-01 | Supply-chain trust model for the pinned golangci-lint install is unchanged from CI's pre-existing `go install` pattern — checksum-verified via GOSUMDB, no new install method or source introduced. | Claude (gsd-secure-phase, L1 grep-depth register carried from plan-time threat model) | 2026-07-09 |
| AR-1-02 | T-1-02 | Command-substitution-derived version string is repo-controlled (same trust boundary as editing ci.yml directly) and is now format-validated before use (WR-01 fix, commit `0f1beba7`); not exploitable as command injection since the captured value is a single double-quoted argument, not re-evaluated shell syntax. | Claude (gsd-secure-phase) | 2026-07-09 |
| AR-1-SC | T-1-SC | No new npm/pip/cargo package introduced by this phase; not applicable. | Claude (gsd-secure-phase) | 2026-07-09 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-07-09 | 3 | 3 | 0 | Claude (gsd-secure-phase, L1 grep-depth — register authored at plan time, asvs_level 1, short-circuit per threats_open: 0) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-07-09
