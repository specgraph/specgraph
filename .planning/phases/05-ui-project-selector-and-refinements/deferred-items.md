# Deferred Items — Phase 05

Items discovered during execution that are out of scope for the current plan.

## 05-02

- **Pre-existing `dprint fmt` drift in `.planning/intel/classifications/*.json` (139 files).**
  `task check` fails at `fmt:check` because these committed JSON classification
  files are not dprint-formatted. Unrelated to plan 05-02 changes (web store +
  Go handler test). Fix separately with `dprint fmt` on `.planning/intel/`.

## 05-03

- **Same pre-existing `dprint fmt` drift in `.planning/intel/classifications/*.json`
  (139 files) still blocks `task check` at `fmt:check`.** Confirmed unrelated to
  05-03 (web shell rebuild); all 05-03 web files pass `task fmt:check`,
  `pnpm build`, and `pnpm test`. Same remediation: `dprint fmt` on `.planning/intel/`.

## 05-05

- **Same pre-existing `dprint fmt` drift in `.planning/intel/classifications/*.json`
  (138 files) + `.planning/research/.cache` (1 file) still blocks `task check` at
  `fmt:check`.** Confirmed unrelated to 05-05 (data-display component migration).
  The three migrated Svelte files touch nothing in `.planning/`; `dprint` has no
  Svelte plugin so it does not format them. `task web:build`, `task build`, and
  `pnpm build` all pass. Same remediation: `dprint fmt` on `.planning/intel/`.
