---
phase: 02-api-key-lifecycle-self-service
plan: 07
subsystem: cli
tags: [api-keys, self-service, cli, cobra, connectrpc, go, credentials]

requires:
  - phase: 02-01
    provides: generated CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey RPCs + request/response messages
  - phase: 02-05
    provides: server self-service handlers enforcing owner-from-context + the Source=="apikey" anti key-chaining reject gate
provides:
  - "Self-variants of `auth api-key create/list/rotate/revoke` that route to the self RPCs when no --user is given"
  - "Admin path preserved: --user <other> keeps the existing CreateAPIKey/ListAPIKeys/RotateAPIKey/RevokeAPIKey calls"
  - "Session-preferring credential resolver (resolveSessionCredential) that ignores SPECGRAPH_API_KEY for self commands (Finding D)"
  - "warnIfEnvKeyIgnored — stderr warning when SPECGRAPH_API_KEY is set during a self command"
  - "newSessionClient / identitySessionClient — a session-authenticated IdentityService client builder"
affects: [web keys dashboard, phase verification, AUTH-03 CLI half]

tech-stack:
  added: []
  patterns:
    - "Branch-on-flag CLI routing: an empty --user selects the self (owner-from-context) RPC; a set --user selects the admin RPC"
    - "Session-preferring credential path for self-service commands — decoupled from the env-first resolveAPIKey used by admin/other commands"
    - "Emit-once secret with a secret-manager instruction and zero on-disk persistence (never runs export / writes a credential file)"

key-files:
  created: []
  modified:
    - cmd/specgraph/client.go
    - cmd/specgraph/identity_client.go
    - cmd/specgraph/auth_apikey.go
    - cmd/specgraph/auth_apikey_test.go
    - cmd/specgraph/client_test.go

key-decisions:
  - "Self path uses a distinct session-preferring client (identitySessionClient) rather than reusing identityClient, so the env-first resolveAPIKey stays untouched for admin/other commands"
  - "rotate and revoke gained a --user selector flag (previously absent); its presence selects the admin RPC, its emptiness selects the owner-scoped self RPC — the admin Rotate/Revoke requests carry no user_id so --user acts purely as a path selector"
  - "Self create/rotate emit the plaintext once with a secret-manager + ${SPECGRAPH_API_KEY} shell-profile instruction and explicitly never write to disk"

patterns-established:
  - "Session-vs-env credential precedence is command-scoped: self-service commands prefer the stored login session; admin/other commands keep env-first precedence"

requirements-completed: [AUTH-03]

coverage:
  - id: D1
    description: "auth api-key create/list/rotate/revoke route to the self RPCs (CreateMyAPIKey/ListMyAPIKeys/RotateMyAPIKey/RevokeMyAPIKey) when --user is empty and to the admin RPCs when --user is set"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "cmd/specgraph/auth_apikey_test.go#TestAuthAPIKeyCreate_RoutesOnUser"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_apikey_test.go#TestAuthAPIKeyList_RoutesOnUser"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_apikey_test.go#TestAuthAPIKeyRotate_RoutesOnUser"
        status: pass
      - kind: unit
        ref: "cmd/specgraph/auth_apikey_test.go#TestAuthAPIKeyRevoke_RoutesOnUser"
        status: pass
    human_judgment: false
  - id: D2
    description: "Self create/rotate print the plaintext exactly once with a secret-manager instruction and write no credential file"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "cmd/specgraph/auth_apikey_test.go#TestAuthAPIKeyCreate_SelfPrintsPlaintextOnce"
        status: pass
    human_judgment: false
  - id: D3
    description: "Finding D: the session-preferring resolver returns the stored spgr_ws_ login session and ignores SPECGRAPH_API_KEY; setting the env key during a self command warns on stderr; the default env-first resolveAPIKey is unchanged for admin/other commands"
    requirement: "AUTH-03"
    verification:
      - kind: unit
        ref: "cmd/specgraph/client_test.go#TestSelfMint_SessionPrecedence"
        status: pass
    human_judgment: false

duration: 9min
completed: 2026-07-10
status: complete
---

# Phase 02 Plan 07: AUTH-03 Self-Service API Key CLI Summary

**Self-variants of `auth api-key create/list/rotate/revoke` that route to the owner-from-context self RPCs when no `--user` is given, authenticating with the stored OIDC login session (never SPECGRAPH_API_KEY) so the server's `Source=="apikey"` anti key-chaining gate never hard-fails on a dev box (Finding D) — while `--user <other>` preserves the existing admin path.**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-07-10T01:07:03Z
- **Completed:** 2026-07-10T01:16:27Z
- **Tasks:** 2
- **Files modified:** 5 (3 production, 2 test)

## Accomplishments

- **Finding-D credential fix** — `resolveSessionCredential(serverURL)` loads the stored `spgr_ws_` session token directly and deliberately ignores `SPECGRAPH_API_KEY`; `newSessionAuthenticatedHTTPClient` / `newSessionClient` build a client on it; `identitySessionClient` is the test-substitutable var the self commands use. `warnIfEnvKeyIgnored` emits a stderr warning when the env key is set during a self command. The env-first `resolveAPIKey` (admin/other commands) is untouched.
- **Four self CLI variants** — `create`/`list`/`rotate`/`revoke` branch on whether `--user` is empty. Empty → the self RPC (`CreateMyAPIKey` with no `user_id`, `ListMyAPIKeys`, `RotateMyAPIKey`, `RevokeMyAPIKey`) via the session-preferring client. `--user <other>` → the unchanged admin RPC path.
- **Emit-once secret hardening** — self create/rotate print the plaintext exactly once via `printSelfMintedKey`, with an instruction to store it in a secret manager or the `${SPECGRAPH_API_KEY}` shell profile, and never run `export` or write a credential file (T-02-25).
- **Help-text behavior note (cursor #7)** — self-path `create`/`rotate` help documents the mandatory expiry: omit `--expires-at` → 90d server default, hard max 180d (D-08), unlike admin mint where expiry is optional.
- **Flag wiring** — added a `--user` selector to `rotate` and `revoke` (previously key-id only); refreshed `list`/`create` `--user` and `--expires-at` help to describe the self-vs-admin split.

## Task Commits

1. **Task 1: Session-preferring resolver (Finding D)** — `afb23e6d` (feat)
2. **Task 2: Self-variants of create/list/rotate/revoke + tests** — `e3f179f2` (feat)
3. **Lint fixups (govet shadow + gosec nolint)** — `572af0a1` (style)

**Plan metadata:** committed with this SUMMARY (docs).

## Files Created/Modified

- `cmd/specgraph/client.go` — `resolveSessionCredential`, `warnIfEnvKeyIgnored`, `newSessionAuthenticatedHTTPClient`, `newSessionClient`
- `cmd/specgraph/identity_client.go` — `identitySessionClient` var (session-authenticated IdentityService client)
- `cmd/specgraph/auth_apikey.go` — self-vs-admin branching on all four verbs; `printSelfMintedKey` helper; `--user` selector on rotate/revoke; updated help text
- `cmd/specgraph/auth_apikey_test.go` — stub-backed routing tests for all four verbs + emit-once plaintext assertion
- `cmd/specgraph/client_test.go` — `TestSelfMint_SessionPrecedence` (session-vs-env precedence + warning)

## Decisions Made

- Self commands use a **distinct** session-preferring client (`identitySessionClient` / `newSessionClient`) rather than overloading `identityClient`, keeping the env-first `resolveAPIKey` precedence intact for admin/other commands.
- `rotate` and `revoke` gained a `--user` selector flag: because the admin `RotateAPIKeyRequest`/`RevokeAPIKeyRequest` carry no `user_id`, the flag acts purely as a path selector (empty → owner-scoped self RPC; set → admin RPC on any key by id).
- Self create/rotate never persist the secret — the output instructs the user to store it themselves; the command runs no `export` and writes no credential file (T-02-25).

## Deviations from Plan

None — plan executed as written. Two structural notes (not scope changes):
- The Task 1 test `TestSelfMint_SessionPrecedence` was placed in `cmd/specgraph/client_test.go` (it exercises `client.go` functions) rather than `auth_apikey_test.go`, keeping Task 1 self-contained and its verify command satisfied at Task 1's commit.
- A third `style` commit (`572af0a1`) fixed `govet` shadow warnings and added `//nolint:gosec` for test-fixture tokens. These cross-scope linters (`govet -shadow`, `gosec`) are not gated by the per-file lefthook pre-commit hook (per AGENTS.md), so they were caught by a manual `golangci-lint run ./cmd/specgraph/...` and fixed before finalizing. `golangci-lint` → 0 issues.

## Issues Encountered

- Initial self-branch code shadowed the outer `err` (from `parseExpiresAt`) via `client, err :=` / `resp, err :=` inside the create/rotate `if` blocks — flagged by `govet -shadow`. Resolved by renaming the inner error vars (`clientErr`, `callErr`). No behavior change.

## Next Phase Readiness

- AUTH-03's CLI half (SC#1) is delivered: a user self-provisions/lists/rotates/revokes their own key from the CLI using their login session, never the bootstrap admin key. The admin path is preserved under `--user <other>`.
- `go build ./cmd/specgraph/` and `go test ./cmd/specgraph/ -run 'TestAuthAPIKey|TestSelfMint_SessionPrecedence'` are green; `golangci-lint run ./cmd/specgraph/...` reports 0 issues.
- Ready for the remaining phase plan (02-08) and phase verification.

---
*Phase: 02-api-key-lifecycle-self-service*
*Completed: 2026-07-10*

## Self-Check: PASSED

- `cmd/specgraph/client.go`, `identity_client.go`, `auth_apikey.go`, `auth_apikey_test.go`, `client_test.go` — all FOUND
- Commits `afb23e6d`, `e3f179f2`, `572af0a1` — all FOUND in git log
- `go build ./cmd/specgraph/` — OK
- `go test ./cmd/specgraph/ -run 'TestAuthAPIKey|TestSelfMint_SessionPrecedence'` — green
- `golangci-lint run ./cmd/specgraph/...` — 0 issues
