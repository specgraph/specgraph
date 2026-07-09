---
phase: 2
slug: api-key-lifecycle-self-service
status: final
nyquist_compliant: true
wave_0_complete: true
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

One row per executable task across all 8 plans (02-01 … 02-08). Commands are copied verbatim
from each task's `<automated>` verify. Task 2-08-03 is the sole manual checkpoint.

| Task ID | Plan | Wave | Requirement | Secure Behavior | Test Type | Automated Command | Test File | Status |
|---------|------|------|-------------|-----------------|-----------|-------------------|-----------|--------|
| 2-01-01 | 01 | 1 | AUTH-02, AUTH-03 | Five self/resync RPCs present in identity.proto | grep gate | `grep -c 'rpc CreateMyAPIKey\|rpc ListMyAPIKeys\|rpc RotateMyAPIKey\|rpc RevokeMyAPIKey\|rpc ResyncUserRole' proto/specgraph/v1/identity.proto` | n/a (grep) | ⬜ pending |
| 2-01-02 | 01 | 1 | AUTH-02, AUTH-03 | gen/ regenerates deterministically + builds | build gate | `task proto && git diff --quiet -- gen/ && task build` | n/a (build) | ⬜ pending |
| 2-02-01 | 02 | 1 | AUTH-03 | UsersBackend interface + ErrQuotaExceeded sentinel + stubs compile | build/vet | `go build ./... && go vet ./internal/storage/... ./internal/auth/... ./internal/server/...` | n/a (build) | ⬜ pending |
| 2-02-02 | 02 | 1 | AUTH-03 | Owner-scoped + quota-safe methods on *AuthStore compile | build gate | `go build ./internal/storage/...` | n/a (build) | ⬜ pending |
| 2-02-03 | 02 | 1 | AUTH-03 | Ownership NotFound (non-owner) + quota TOCTOU under real Postgres | integration | `go test -tags integration ./internal/storage/postgres/ -run 'TestAuthStore_.*ForUser\|TestAuthStore_SelfMintQuota\|TestAuthStore_RotateAPIKeyForUser_ExplicitArgs'` | creates | ⬜ pending |
| 2-03-01 | 03 | 1 | AUTH-03 | Self-service key-policy config struct + 90d/180d/quota defaults | unit | `go build ./internal/config/ && go test ./internal/config/ -run TestGlobalDefaults` | modifies | ⬜ pending |
| 2-03-02 | 03 | 1 | AUTH-03 | Double-submit CSRF middleware rejects missing/mismatched token | unit | `go test ./internal/server/ -run TestCSRF` | creates | ⬜ pending |
| 2-04-01 | 04 | 2 | AUTH-03, AUTH-02 | Exported auth.RoleMin fail-closed floor | unit | `go test ./internal/auth/ -run TestRoleMin` | creates | ⬜ pending |
| 2-04-02 | 04 | 2 | AUTH-03 | apikey.self verb registered (knownVerbs + base.cedar permit) | unit + grep | `go test ./internal/auth/ -run 'TestNewCedarEngine\|TestEngine' && grep -c 'apikey.self' internal/auth/policies/base.cedar` | modifies | ⬜ pending |
| 2-04-03 | 04 | 2 | AUTH-03 | Procedure→action map + mirror/drift tests | unit | `go test ./internal/auth/ -run 'TestActionNames_AllParseToKnownVerb\|TestActionForProcedure_Identity\|TestActionName'` | modifies | ⬜ pending |
| 2-05-01 | 05 | 3 | AUTH-03 | Mint pair: owner-from-ctx, source gate, RoleMin floor (create+rotate), expiry cap, rate limit; constructor threaded through serve.go | unit + build | `go test ./internal/server/ -run 'TestCreateMyAPIKey\|TestRotateMyAPIKey\|TestSelfMint_RejectsApikeySource\|TestSelfMint_ExpiryCap' && go build ./...` | creates | ⬜ pending |
| 2-05-02 | 05 | 3 | AUTH-03 | Owner-scoped list (no cross-user leak) + revoke NotFound on foreign key | unit | `go test ./internal/server/ -run 'TestListMyAPIKeys_ScopedToCaller\|TestRevokeMyAPIKey'` | creates | ⬜ pending |
| 2-06-01 | 06 | 4 | AUTH-02 | ResyncUserRole writes live role; revoke_keys revokes active keys; unknown → NotFound | unit | `go test ./internal/server/ -run 'TestResync'` | creates | ⬜ pending |
| 2-06-02 | 06 | 4 | AUTH-02 | `auth user resync` CLI calls RPC with id/role/revoke-keys; JSON path | unit + build | `go test ./cmd/specgraph/ -run 'TestAuthUserResync' && go build ./cmd/specgraph/` | creates | ⬜ pending |
| 2-07-01 | 07 | 4 | AUTH-03 | Session-preferring credential resolver (Finding D) | unit + build | `go build ./cmd/specgraph/ && go test ./cmd/specgraph/ -run 'TestSelfMint_SessionPrecedence'` | creates | ⬜ pending |
| 2-07-02 | 07 | 4 | AUTH-03 | Self-variants of auth api-key create/list/rotate/revoke | unit + build | `go test ./cmd/specgraph/ -run 'TestAuthAPIKey' && go build ./cmd/specgraph/` | creates | ⬜ pending |
| 2-08-01 | 08 | 4 | AUTH-03 | identityClient + CSRF interceptor sets X-CSRF-Token; keys state; plaintext surfaced once | unit (vitest) | `pnpm -C web test -- --run keys.test.ts` | creates | ⬜ pending |
| 2-08-02 | 08 | 4 | AUTH-03 | /keys route + RevealKeyModal (one-time reveal) compiles | build gate | `pnpm -C web run build` | n/a (build) | ⬜ pending |
| 2-08-03 | 08 | 4 | AUTH-03 | Human-verify live MCP Keys panel: one-time reveal + CSRF enforcement | manual | — (checkpoint:human-verify) | n/a (manual) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

**Sampling continuity:** No 3 consecutive tasks lack an automated verify — the only manual task
(2-08-03) is a terminal human-verify checkpoint, preceded by two automated web tasks (2-08-01 vitest,
2-08-02 build). Integration behavior for AUTH-02 live-role-clamp is deferred to the phase-gate
`task pr-prep` run (see Plan 06 deferred-coverage note).

---

## Wave 0 Requirements

All Wave 0 concerns are absorbed into in-plan TDD tasks (each task authors its own failing test in
RED before GREEN), so no separate pre-execution scaffold is outstanding. Coverage mapping:

- [x] `internal/auth/actions_test.go` — add `"self"` to BOTH hard-coded verb lists → **covered by 2-04-03** (mirror/drift tests) + **2-04-02** (`apikey.self` registration)
- [x] Owner-scoped storage unit/integration tests — `RevokeAPIKeyForUser`/`RotateAPIKeyForUser`/`GetAPIKeyForUser` (NotFound on non-owner) → **covered by 2-02-03**
- [x] Adversarial security tests — laundering floor (create+rotate) + anti-key-chaining (`Source=="apikey"`) → **2-05-01**; quota TOCTOU → **2-02-03**; CSRF double-submit → **2-03-02** / **2-08-01**

*Framework already present — go test + testcontainers + vitest. No install needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Web "MCP Keys" panel one-time reveal modal (task 2-08-03) | AUTH-03 | SvelteKit UI interaction | Log in via `specgraph_session` cookie, create key, confirm single reveal + CSRF-protected mutations |
| Operator forced re-sync immediacy on standing keys | AUTH-02 | Cross-session propagation (live-role clamp) | `auth user resync <id> --role <lower>`, then call MCP with standing key → reduced privilege without re-login. Also exercised by the deferred `task pr-prep` integration run (`TestResync_LiveRoleClamp` / `TestResync_RevokeKeys`). |

*Remaining phase behaviors have automated verification.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or are a terminal manual checkpoint (2-08-03)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (folded into in-plan TDD tasks; mapped above)
- [x] No watch-mode flags (all `vitest --run`, `go test` are one-shot)
- [x] Feedback latency < 120s (unit); integration on Docker at phase gate
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** finalized 2026-07-09 — validation contract complete against all 8 executable plans.
