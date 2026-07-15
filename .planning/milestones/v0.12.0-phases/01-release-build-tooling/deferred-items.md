# Deferred Items — Phase 01 (release-build-tooling)

Out-of-scope discoveries found during execution of 01-01-PLAN.md. Not fixed per the
executor's scope boundary (only auto-fix issues directly caused by the current task's changes).

## `task lint` pre-existing failures (unrelated to CFG-02)

Found while running `task lint` as part of Task 2 verification. Both files predate this
plan (from the docs/ corpus ingest, commit `040f5181`) and are untouched by 01-01-PLAN.md's
edits (Taskfile.yml, .github/workflows/ci.yml only).

- **`lint:markdown` (rumdl):** 303 issues in 18/236 files, dominated by
  `.planning/intel/constraints.md` (MD032 — list should be preceded by blank line).
  Fix: `rumdl fmt` would auto-fix 299/303.
- **`lint:yaml` (yamlfmt):** formatting drift in `.planning/INGEST-MANIFEST.yaml`
  (indentation/spacing).

Both edited files in this plan (`Taskfile.yml`, `.github/workflows/ci.yml`) were verified
individually clean against `yamlfmt -lint` and `actionlint` before commit.
