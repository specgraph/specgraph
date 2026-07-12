# Deferred Items — Phase 05

Out-of-scope discoveries logged during execution. NOT fixed (SCOPE BOUNDARY:
only auto-fix issues directly caused by the current task's changes).

## From 05-01 (foundation)

- **`task check` `fmt:check` fails on 139 pre-existing `.planning/` files.**
  `dprint check` flags 138 files under `.planning/intel/classifications/*.json`
  and 1 under `.planning/research/` for JSON array formatting. None are touched
  by plan 05-01 (all 05-01 changes are under `web/`). Fix with `dprint fmt` in a
  dedicated cleanup, or add `.planning/intel` / `.planning/research` to the
  dprint ignore config. Verified: zero overlap between dprint-flagged files and
  this plan's changed files.

- **`task check` `lint:markdown` (rumdl) reports ~950 issues in 85 `.planning/`
  markdown files.** All pre-existing planning docs (CONTEXT/PLAN/etc.), none
  authored by 05-01. Fix with `rumdl fmt` (auto-fixes 914 of 950) in a
  docs-cleanup pass.

Substantive gates for 05-01 all pass independently: `license:check`, `lint:go`
(0 issues), `build` (Go binary embeds the static bundle), `task test` (Go unit),
and `pnpm -C web test` (10/10). The two failing `task check` sub-stages are
repo-wide pre-existing doc-formatting debt, unrelated to this UI foundation.
