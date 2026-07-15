# Deferred Items — Phase 03

## Pre-existing dprint formatting drift in `.planning/intel/classifications/*.json`

- **Found during:** Phase 03 Plan 01 execution (`task check` → `fmt:check`).
- **Issue:** `task check` reports 139 not-formatted files under
  `.planning/intel/classifications/*.json` (dprint wants multi-line array
  expansion). This drift pre-exists this plan and is unrelated to the Go source
  changes in AUTH-01/04/05.
- **Scope:** Out of scope for this plan (scope boundary — do not auto-fix
  unrelated pre-existing issues). All Go source touched by this plan is
  gofmt-clean and lints with 0 issues.
- **Suggested fix:** run `dprint fmt` on `.planning/` in a dedicated chore
  commit, or exclude generated `.planning/intel/` JSON from the dprint config.
