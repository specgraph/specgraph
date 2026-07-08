# Coding Conventions

**Analysis Date:** 2026-07-08

## Naming Patterns

**Files:**
- Lowercase, underscore-separated when needed: `change_event.go`, `claim_domain.go`, `auth_oidc_test.go`
- Domain-suffixed pairs: `<entity>.go` (interface/logic) + `<entity>_domain.go` (domain types), e.g. `internal/storage/claim.go` / `internal/storage/claim_domain.go`
- Test files always `<name>_test.go`, colocated with the code they test

**Packages:**
- Short, lowercase, no underscores: `storage`, `authoring`, `drift`, `render`, `sync`
- Every package has a `// Package <name> ...` doc comment on its first file (enforced by `revive`'s `package-comments` rule) — see `internal/storage/storage.go:4`

**Functions:**
- Exported: `PascalCase`, verb-first (`CreateSpec`, `TransitionStage`, `GetTransitiveDeps`)
- Unexported: `camelCase`
- Boolean-returning helpers use `Is`/`Has` prefixes where applicable

**Variables:**
- `camelCase` for locals, `PascalCase` for exported package-level vars
- Sentinel errors: `Err<Description>` (e.g. `ErrSpecNotFound`, `ErrNotClaimOwner`) — all declared centrally in `internal/storage/errors.go`

**Types:**
- Domain structs: `PascalCase` nouns (`Claim`, `Spec`, `Session`)
- Test doubles: `<scope>Fake<Interface>` naming, e.g. `logoutFakeWA` in `cmd/specgraph/logout_test.go:221` implementing `storage.WebAuthStore`

## Code Style

**Formatting:**
- Standard `gofmt`/`goimports` via `task fmt` (also formats YAML/Markdown via yamlfmt/dprint)
- No custom column-width or brace-style overrides — idiomatic Go formatting throughout

**Linting:**
- `golangci-lint` v2, config at `.golangci.yaml`
- Enabled linters: `errcheck`, `govet` (enable-all, `fieldalignment` disabled), `staticcheck`, `nilerr`, `revive`, `misspell`, `prealloc`, `unconvert`, `gosec`, `errorlint`, `wrapcheck`, `unparam`, `gocritic` (diagnostic+style+performance tags), `nolintlint`, `sloglint`
- Path exclusions: `gen/` (generated protobuf) is unlinted; `_test.go` files exempt from `gocritic`, `wrapcheck`, `errcheck`, `sloglint`; `cmd/` exempt from `wrapcheck` (main package doesn't need to wrap errors)
- `internal/scanner/` exempt from `revive var-naming` (domain name intentionally conflicts with stdlib `text/scanner`)
- `nolintlint` requires an explanation and specific linter name on every `//nolint` — bare `//nolint` fails CI
- `sloglint`: no mixed positional/key-value args, attributes only (no `slog.Any` misuse), static log messages, context-scoped

**Revive rules enabled:** `blank-imports`, `context-as-argument`, `context-keys-type`, `dot-imports` (forbidden), `error-return`, `error-strings`, `error-naming`, `exported`, `if-return`, `increment-decrement`, `var-naming`, `var-declaration`, `package-comments`, `range`, `receiver-naming`, `time-naming`, `unexported-return`, `indent-error-flow`, `errorf`, `empty-block`, `superfluous-else`, `unused-parameter`, `unreachable-code`, `redefines-builtin-id`

## Import Organization

**Order (standard Go grouping, enforced by goimports):**
1. Standard library
2. Third-party (`github.com/...`, `connectrpc.com/...`, `google.golang.org/...`)
3. Internal project packages (`github.com/specgraph/specgraph/internal/...`, `github.com/specgraph/specgraph/gen/...`)

**Path Aliases:**
- Generated proto packages aliased by version: `specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"`
- Connect service package imported directly: `"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"`
- No internal module path aliasing beyond proto version aliasing

**Dot imports:** forbidden by `revive` project-wide EXCEPT in Ginkgo/Gomega e2e test files, where `. "github.com/onsi/ginkgo/v2"` and `. "github.com/onsi/gomega"` are standard (test files are exempt from most style linters)

## Error Handling

**Sentinel errors:**
- Centralized in `internal/storage/errors.go` as package-level `var Err... = errors.New("...")`
- Callers check with `errors.Is()`, never string matching
- Handlers translate storage sentinel errors to ConnectRPC codes (`connect.CodeNotFound`, `connect.CodeAborted`, etc.) — see `stageError` and similar sanitizing methods in `internal/server/`
- Concurrent-modification conflicts return `storage.ErrConcurrentModification` mapped to `connect.CodeAborted` (retryable); tests must assert on error codes, not message strings

**Wrapping:**
- `fmt.Errorf("<context>: %w", err)` throughout non-`cmd/` packages — enforced by `wrapcheck` linter
- Double-wrap pattern for domain + underlying cause: `fmt.Errorf("postgres: %w: %w", storage.ErrInvalidStageTransition, err)` (`internal/storage/postgres/authoring.go:32`)
- Package/operation-prefixed messages: `"postgres: transition stage: %w"`, `"postgres: transition stage %q: %w"` — include identifiers (slugs) via `%q`
- `wrapcheck` allowed unwrapped sigs: `.Errorf(`, `errors.New(`, `errors.Unwrap(`, `errors.Join(`, `.Wrap(`, `.Wrapf(`, `.WithMessage(`, `.WithMessagef(`, `.WithStack(`
- `cmd/` package (CLI entry point) is exempt from `wrapcheck` — errors surface directly to the user without needing programmatic unwrapping

**Error checking:**
- `errcheck` enabled with `check-type-assertions: true` and `check-blank: true` — all errors must be handled or explicitly discarded with a documented reason
- `errorlint` enforced — use `errors.Is`/`errors.As`, never `==` comparison or type assertion on errors
- `nilerr` enforced — never return `nil` when an error occurred

## Logging

**Framework:** `slog` (structured logging), wired through `internal/telemetry/` as a context-enriching handler

**Patterns:**
- `sloglint` enforces: no mixing positional and key-value args, attributes-only calls, static (non-interpolated) messages, context passed for trace/log correlation (`context: scope`)
- Logging goes through the OpenTelemetry-integrated provider set up once at startup (single `Init`/`Shutdown`, no-op when telemetry disabled) — see `internal/telemetry/`

## Comments

**When to Comment:**
- Package-level doc comment mandatory (`// Package X ...`) — enforced by CI
- Exported identifiers require doc comments starting with the identifier name (`revive exported` rule)
- Inline comments explain *why*, not *what*, especially around non-obvious domain rules (e.g. edge direction, transaction boundaries) — see extensive gotchas in project `CLAUDE.md`

**Nolint comments:**
- Must include linter name and explanation: `//nolint:gosec // <reason>` — bare or unexplained `//nolint` fails `nolintlint`

## Function Design

**Size:** No hard-coded limit, but table-driven tests and multi-step storage operations favor extracting private helpers over deeply nested logic; `gocritic` flags overly complex branches

**Parameters:** Context always first (`context.Context`) per `revive context-as-argument`; unused parameters flagged (`unused-parameter`) — use `_` for intentionally unused params

**Return Values:** Errors always last return value (`error-return` rule); avoid unexported types leaking through exported function returns (`unexported-return`)

## Module Design

**Exports:** Interfaces defined in `internal/storage/` using domain types (not protobuf types); implementations live in subpackages (`internal/storage/postgres/`)

**Proto ↔ domain boundary:** Conversion functions bridge protobuf and domain types explicitly in `internal/server/` (handlers) — protobuf types never leak into storage interfaces

**License headers:** Every `.go`, `.sh`, `.py`, `.proto` file starts with:
```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt
```
Enforced by `addlicense` via `task license:check` / `task license:add`, and by lefthook pre-commit.

---

*Convention analysis: 2026-07-08*
