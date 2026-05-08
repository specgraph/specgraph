# Deprecate `specgraph inject` in favor of MCP + extended `init`

## Context

The Harness Parity Epic (`spgr-cceg`, PR #939, merged 2026-05-06) shipped MCP
parity across all three supported harnesses: Claude Code, Cursor, and OpenCode.
Every harness we have deliberately chosen to support now speaks MCP natively,
gets per-harness config wiring from `specgraph init`, and has a session-prime
path into `specgraph://prime`.

That landing materially changes the case for `specgraph inject`.

`inject` writes per-spec markdown digests into the workspace under
[`internal/inject/`](../../internal/inject/) — `.claude/specs/<slug>.md`,
`.cursor/rules/specgraph-<slug>.md`, and per-slug managed blocks in `AGENTS.md`
delimited by `<!-- specgraph:<slug>:start -->` / `<!-- specgraph:<slug>:end -->`
markers. The content is the same digest the MCP `specgraph://spec/{slug}`
resource returns. It was useful when the harness story was "MCP isn't
guaranteed; some agents only read files in the repo." That is no longer true
for any harness we support.

This design replaces `inject` with two changes to `specgraph init`:

1. Always write `AGENTS.md` and `.cursor/rules/specgraph-bootstrap.md` as
   **minimal pointer files** that direct the agent to the running SpecGraph
   MCP server.
2. Manage those files with init-flavored marker fencing, complementing the
   JSON-merge sync that `internal/config/mcpconfigs/` already does for
   `.mcp.json`, `.cursor/mcp.json`, and `opencode.json`.

`inject` (the CLI command, the `Inject` RPC, the `internal/inject/` package,
and every Go caller that still references `storage.InjectToolType` or
`specv1.InjectTool*`) is deleted in the same PR. Legacy per-slug `inject`
blocks in `AGENTS.md` are actively purged on first run of the new `init`.
Orphaned `.claude/specs/*.md` and `.cursor/rules/specgraph-<slug>.md` files
are left on disk untouched, with deletion guidance surfaced in CHANGELOG.

The new pointer file deliberately uses the filename
`.cursor/rules/specgraph-bootstrap.md`, **not** `specgraph.md`. The latter is
already shipped by the harness-parity epic at
[`plugin/cursor/.cursor/rules/specgraph.md`](../../plugin/cursor/.cursor/rules/specgraph.md)
with `alwaysApply: false` and a real routing payload. Init owning a different
filename means: the plugin shim continues to own routing content, init owns
the always-apply MCP-pointer content, and there is no contention over a
single managed file.

## Goal and scope

### Goals

1. Remove `specgraph inject` end-to-end: CLI subcommand, ConnectRPC method,
   proto messages, domain-type enum, every test that exercises any of those,
   and every doc that still references the command.
2. Extend `specgraph init` to write and reconcile two new pointer files:
   `AGENTS.md` and `.cursor/rules/specgraph-bootstrap.md`.
3. Adopt the same managed-block fencing pattern that inject pioneered, but
   with a single init-flavored marker (`<!-- specgraph:init:start v=1 -->`)
   instead of per-slug markers. Inject's marker contract was per-slug and
   unversioned; init's is project-level and versioned.
4. Actively purge legacy `inject`-flavored per-slug managed blocks during the
   `init` run, with a per-file count surfaced to the user.
5. Keep the change behind one PR. No multi-step deprecation period — the
   harnesses we support all speak MCP, the users running on `inject` are us
   and a small handful of early adopters.

### Non-goals

1. Removing or modifying `constitution emit --format claude-md`. That command
   writes `CLAUDE.md` from the resolved constitution layers and serves a
   different audience (project bootstrap, not per-session priming). Init does
   **not** touch `CLAUDE.md` in this work. A separate bead will track whether
   `init` should also reconcile `CLAUDE.md` once the constitution and pointer
   stories are clearly distinct.
2. Adding a REST resource API. If we ever add non-MCP consumers (Aider, Cody,
   PR bots, CI summarizers), the principled fix is a REST endpoint on the
   server, not file injection. That's a separate bead and out of scope here.
3. Cleaning up orphan `inject`-era files (`.claude/specs/<slug>.md`,
   `.cursor/rules/specgraph-<slug>.md`). We notify in the changelog and stop.
   The user can `rm -rf` whenever they feel like it; SpecGraph is not in the
   business of touching files it didn't write.
4. Cross-harness behavior parity beyond what was already shipped in
   `spgr-cceg`. This is a cleanup bead, not a feature bead.

### What stays the same

- `specgraph init` continues to write and reconcile the three MCP config files
  (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`) exactly as it does today.
- The MCP server's resource and prompt surface
  ([`internal/mcp/resources.go`](../../internal/mcp/resources.go),
  [`internal/mcp/prompts.go`](../../internal/mcp/prompts.go)) is unchanged.
  The pointer files direct the agent to existing resources; no new MCP
  surface is added.
- `constitution emit --format claude-md` keeps working as-is.
- Skills under `skills/` and per-harness plugin shims under `plugin/` are
  untouched.

## Architecture

### Module layout

```text
internal/config/
├── mcpconfigs/         existing — per-harness JSON config sync (.mcp.json,
│                       .cursor/mcp.json, opencode.json)
└── pointers/           NEW — markdown pointer-file sync (AGENTS.md,
                        .cursor/rules/specgraph-bootstrap.md)
```

A new `internal/config/pointers/` package, sibling to
`internal/config/mcpconfigs/`. The reasoning for a new package, rather than
extending `mcpconfigs/`:

- `mcpconfigs/` operates on JSON Merge Patch (RFC 7396) over structured config
  files. It is correct for `.mcp.json`-shape things and only those.
- Pointer files are markdown. The reconciliation primitive is **managed-block
  marker fencing** — read-modify-write of a delimited region inside a file
  whose surrounding bytes are user-owned. Different model entirely.
- Cursor rule files require YAML frontmatter, which `mcpconfigs/` neither has
  nor needs.

The two packages share a common output convention (Action constants of type
`Action string`, project-relative `Path` field, the same canonical
`{"created", "updated", "no-op"}` action set) so `cmd/specgraph/init.go` can
fold both result slices into one printed list. They do **not** share the same
struct: pointers needs a `LegacyBlocksPurged` count and (per the failure
model below) an `Err` field. See "Public surface" for the precise types.

### Public surface of `internal/config/pointers/`

```go
package pointers

// Action reuses the string-typed enum convention from mcpconfigs.Action.
// Values are intentionally identical so init can render a unified output.
type Action string

const (
    ActionCreated Action = "created"
    ActionUpdated Action = "updated"
    ActionNoOp    Action = "no-op"
    ActionError   Action = "error"
)

// SyncResult reports what Sync did to a single managed pointer file.
//
// Compared to mcpconfigs.SyncResult, this struct adds Err (for continue-on-
// error semantics) and LegacyBlocksPurged (for the AGENTS.md migration
// surface). Path is project-relative, matching the mcpconfigs convention.
type SyncResult struct {
    Path               string // project-relative path
    Action             Action
    Err                error // populated when Action == ActionError
    LegacyBlocksPurged int   // count of per-slug inject blocks removed; 0 for non-AGENTS.md targets
}

// Sync reconciles all pointer files for the project. Returns one SyncResult
// per file (always two: AGENTS.md and .cursor/rules/specgraph-bootstrap.md).
//
// Files are processed serially in a deterministic order. A failure on one
// file is reported via SyncResult.Err with Action == ActionError and does
// **not** abort the rest of the run; the caller decides the exit code based
// on the slice. This differs from mcpconfigs.Sync, which aborts on first
// error — see the "Failure isolation" subsection for the rationale and how
// init reconciles the two semantics.
func Sync(projectDir string, opts Options) []SyncResult

// Options carries the canonical values that init derives once and threads
// into the pointer templates.
type Options struct {
    // ServerURL is the resolved http(s) URL of the SpecGraph server, after
    // init's url.Parse validation. Written into the managed block.
    ServerURL string

    // ProjectSlug is the project identity used as the X-Specgraph-Project
    // header value (matches config.ProjectConfig.Slug). Written into the
    // managed block so a future MCP client can pin the right project.
    ProjectSlug string
}
```

`Sync` is the only exported function. Per-file logic (template rendering,
marker scanning, frontmatter handling) is internal to the package.

**Contract specifics:**

- **`projectDir` must be an existing directory.** `Sync` calls
  `os.Stat(projectDir)` first; a missing directory or a non-directory path
  produces a single-element error result for an aggregate "projectDir"
  pseudo-path and an empty pointer slice. The caller treats this exactly
  like any other `ActionError`. (Alternative considered: panic. Rejected
  because `init` is a CLI command and stack traces are user-hostile.)
- **The returned slice always has exactly two entries** when `projectDir` is
  valid, in this fixed order: `AGENTS.md` first, then
  `.cursor/rules/specgraph-bootstrap.md`. This ordering is contract, not
  incidental — tests assert on slice indices and the CLI output ordering
  matches.
- **`Sync` never returns `nil`.** Worst case it returns a two-element slice
  where both entries have `Action == ActionError`.

The `Options` field is named `ProjectSlug`, not `ProjectID`, because that's
the term the rest of the codebase uses (`ProjectConfig.Slug`,
`X-Specgraph-Project` header value). The previous draft of this design used
`ProjectID`; the rename keeps terminology consistent.

### Marker contract

A single managed block per file, fenced with init-flavored markers:

```text
<!-- specgraph:init:start v=1 -->
... canonical content rendered from template ...
<!-- specgraph:init:end -->
```

Properties:

- **Single block per file.** No per-slug variants. The pointer is project-wide
  and stable; there's nothing to vary by slug.
- **Versioned start marker.** `v=1` lets us evolve the canonical content
  without confusing scanners. Current and only version is `v=1`.
- **Idempotent.** If the existing block byte-equals the canonical render, the
  file is left untouched and the result is `ActionNoOp`.
- **User content preserved outside the block.** Anything outside the markers
  is left alone. If the file exists without markers, the block is appended
  with a leading blank line.
- **User content inside the block is overwritten.** This is intentional. The
  block is init-managed; if the user wants their text persisted, they put it
  outside the markers. Tests cover this contract explicitly so the behavior
  is documented, not silent.
- **Corrupted markers are fatal.** End-before-start, start-without-end, or two
  start markers with no end between them all return `SyncResult.Err` and skip
  the write. The error message names the file and the failure mode; the user
  fixes it manually. We do not heuristically guess.

### Legacy block purge

Per-slug `inject` blocks fenced as `<!-- specgraph:<slug>:start -->` /
`<!-- specgraph:<slug>:end -->` are **actively purged** during the `init` run,
not migrated. There is no semantic mapping from "N per-slug blocks with spec
digests" to "one project-level pointer block"; preservation would mean
shipping stale per-spec content forever.

The purge regex matches the slug character class that inject actually wrote.
Inject's `safeSlugPattern` in
[`internal/inject/inject.go`](../../internal/inject/inject.go) is
`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`. The purge regex therefore matches the same
class:

```text
<!-- specgraph:([a-zA-Z0-9][a-zA-Z0-9._-]*):start -->
…body…
<!-- specgraph:\1:end -->
```

Properties of the purge:

- The literal token `init` is excluded from the slug capture group: the
  purge regex never matches the init-flavored block, only per-slug
  variants. Implementation detail — express this as either a negative
  lookahead substitute (Go's `regexp` doesn't support lookaround, so use a
  post-filter on the captured slug) or a separate exclusion check after
  candidate match.
- Slugs that don't match this class — e.g. with whitespace, slashes, or
  characters outside the inject character class — are not matched and not
  purged. They aren't blocks inject could have written, so leaving them
  alone is correct.
- Mismatched start/end pairs (orphan start, orphan end, end-before-start
  with the *same* slug) trigger the corrupted-markers error path; the file
  is not modified. This catches the case where a user manually deleted half
  a block.
- Purged blocks are counted per-file and surfaced in `SyncResult` and in the
  CLI output. Users see exactly how many legacy blocks were removed from
  exactly which files.

#### Phase order

For each pointer file, the package processes phases in this fixed order:

1. **Validate the init marker contract** on the existing file content.
   Reject `<!-- specgraph:init:start -->` without `v=1` (corruption rule
   #4); reject orphaned/mismatched init markers (rules #1–#3). If any of
   these fire, return `ActionError` immediately — no purge attempt, no
   write.
2. **Purge legacy per-slug blocks**, applying the post-filter that excludes
   the literal slug `init` from the capture group. This phase only runs
   for `AGENTS.md`.
3. **Reconcile the init managed block** against the canonical render:
   no-op, update, or insert.
4. **Atomic write** if and only if phases 2 or 3 produced a different byte
   string than what's currently on disk.

This ordering means a pathologically-shaped legacy block with literal slug
`init` (e.g. `<!-- specgraph:init:start -->...<!-- specgraph:init:end -->`,
no `v=1`) is detected as corruption in phase 1, before phase 2 ever sees
it. The slug-`init` exclusion in phase 2 is therefore belt-and-braces, not
load-bearing.

### Cursor rule frontmatter

`.cursor/rules/specgraph-bootstrap.md` requires Cursor-specific YAML
frontmatter:

```yaml
---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---
```

`alwaysApply: true` is the deliberate choice for this file: the pointer
should be in scope on every Cursor turn so the model never forgets where MCP
lives. The harness-parity-shipped
[`plugin/cursor/.cursor/rules/specgraph.md`](../../plugin/cursor/.cursor/rules/specgraph.md)
already exists with `alwaysApply: false` and a routing payload; the two files
have different roles and live at different paths.

Frontmatter handling rules:

- On **create**, the package writes the canonical frontmatter above plus the
  managed block.
- On **update**, the package preserves any existing frontmatter — including
  user edits to `description`, addition of new fields, etc. — and rewrites
  only the managed block in the body. `alwaysApply` is **not** enforced on
  update; the user can change it.
- A file existing **without** frontmatter (no leading `---`) is a fatal error.
  Cursor itself will reject the file in that state, and we don't want to
  silently convert a hand-written rule into a managed one. The error message
  instructs the user to either remove the file or add the frontmatter
  manually.

### `init` flow

The current `runInit` in `cmd/specgraph/init.go` does the following: locate
the project root via `config.FindProjectRoot`; resolve a `ProjectConfig`
using this precedence — **existing `.specgraph.yaml` wins; otherwise the
positional arg is used; otherwise the slug is derived from git remote /
dirname via `config.LoadProject`** (the positional arg only validates
consistency when both an arg and an existing config are present; **no
prompting** in any case); resolve and validate the server URL; build
`mcpconfigs.ManagedConfigs(slug, serverURL)`; optionally write
`.specgraph.yaml`; then call `mcpconfigs.Sync` and print `path: action` per
result. There is no `--json` flag today.

The change extends that flow at the end. The integration is **conditional**:
`pointers.Sync` runs only if `mcpconfigs.Sync` returned no error, because
the design (see "Failure isolation") deliberately keeps the existing
abort-on-error behavior for the higher-stakes JSON configs.

```text
specgraph init
  ├─ existing — find/load/derive project (yaml > arg > derived), validate server URL
  ├─ existing — write .specgraph.yaml if absent
  ├─ existing — mcpconfigs.Sync(cwd, configs)  → []SyncResult, error
  │      ├─ print every entry of the partial slice as "<path>: <action>"
  │      └─ if error != nil: exit 1 (skip pointers entirely)
  ├─ NEW — pointers.Sync(cwd, pointers.Options{ServerURL, ProjectSlug}) → []SyncResult
  │      → always two entries; per-file errors carried in SyncResult.Err
  ├─ NEW — print "<path>: <action>[ (purged N legacy blocks)]" for each pointers result
  │      (entries with Action == ActionError print "<path>: error: <message>")
  └─ exit 1 if any pointers result has Action == ActionError; else exit 0
```

Each file write is its own atomic rename; failures on one file do not abort
the others **inside `pointers.Sync`** (see "Failure isolation" below for why
this differs from `mcpconfigs.Sync`).

A `--json` flag is **out of scope** for this design. The original brainstorm
called for one; on review there is no existing `init --json` implementation
to extend, and adding one is orthogonal to the inject-deprecation question.
A separate bead can pick that up if/when it matters.

## Behavior, edge cases, and error handling

### Idempotency contract

Re-running `specgraph init` on a project that's already in the canonical
state must produce zero file writes. Six states the package handles:

| State | Action |
|---|---|
| File doesn't exist | Create with canonical content. `Action = ActionCreated`. |
| File exists, no markers, has user content | Append managed block with leading blank line. Preserve user content above. `Action = ActionUpdated`. |
| File exists with current managed block, byte-equal to canonical | No write. `Action = ActionNoOp`. |
| File exists with stale managed block (e.g. old serverURL) | Replace block content. `Action = ActionUpdated`. |
| File exists with legacy per-slug inject blocks (AGENTS.md only) | Strip legacy blocks, then ensure init block. `Action = ActionUpdated`. `LegacyBlocksPurged > 0`. |
| File exists with corrupted markers | Don't write. `Action = ActionError`. Message identifies the corruption mode and the file. |

### Output format

The current `init` output is one line per result: `<path>: <action>` (no
icons, no table). The new pointer rows extend that with an optional purge
suffix when relevant:

```text
.mcp.json: updated
.cursor/mcp.json: no-op
opencode.json: no-op
AGENTS.md: updated (purged 3 legacy blocks)
.cursor/rules/specgraph-bootstrap.md: created
```

Format rules:

- Action strings match the existing mcpconfigs constants exactly:
  `created`, `updated`, `no-op`. New constant: `error`.
- The legacy-purge suffix appears only when `LegacyBlocksPurged > 0`.
- A `pointers` row with `Action == ActionError` prints
  `<path>: error: <message>` and contributes to a non-zero exit code.

### Errors and corruption

- **Symlinks in the path.** `internal/config/mcpconfigs/sync.go` already
  rejects symlinks anywhere in the resolved path
  (`rejectSymlinkComponents`, lines 156–175). The new `pointers/` package
  copies the same logic — the function is a small standalone helper and
  reproducing it is cheaper than introducing a shared util. Symlink errors
  abort the write for that file only and emit `SyncResult.Err`.
- **Atomic write.** Write to `<file>.tmp.<random>` in the same directory,
  then `os.Rename` over the target. The full new file content (including
  any legacy-block purge) is computed in memory before any write, so a
  crash mid-rename never leaves a partially-purged file on disk. The temp
  file is removed on failure.
- **Trailing newline.** All written files end with `\n`. This matches the
  invariant `mcpconfigs.canonicalize` already enforces for JSON files
  (`sync.go` line 189) and is consistent with how `inject.writeAgentsMD`
  appends a trailing newline today.
- **Corruption detection rules** (all return `ActionError` without writing):
  1. End marker before any start marker for the *same* slug or for `init`.
  2. Start marker with no matching end marker before EOF.
  3. Two start markers with no end between them.
  4. A start marker `<!-- specgraph:init:start -->` without the recognized
     `v=1` suffix. Init only ever writes the versioned form; an unversioned
     init marker can only have come from a hand-edit or a future version we
     don't recognize, and silently adopting it would lose information.
- **Failure isolation.** `pointers.Sync` processes files serially in a
  deterministic order; a failure on one pointer file does not abort the
  rest of `pointers.Sync`. This intentionally diverges from
  `mcpconfigs.Sync`, which returns on first error: pointer-file failures
  are likely to be local and recoverable (e.g. corrupted markers in one
  file) and the user benefits from seeing the other file's status. Init
  reconciles the two semantics by:
  1. Calling `mcpconfigs.Sync` first; if it returns an error, print the
     partial slice and exit non-zero. (No change from today.)
  2. Calling `pointers.Sync` only if `mcpconfigs.Sync` succeeded. Print
     every pointer result. Exit non-zero if any pointer result has
     `Action == ActionError`.

  The mcpconfigs/pointers ordering keeps the existing abort-on-error story
  for the JSON files (which are higher-stakes — a malformed `.mcp.json`
  takes the harness offline) and adds continue-on-error only for the
  markdown pointers.

### File locking

Lifted from inject's existing per-OS lock (`internal/inject/lock_unix.go` /
`lock_windows.go`). On Unix, the implementation acquires an exclusive
advisory lock on a sibling file `<path>.lock`, **not** on the target file
directly: that's what `acquireFileLock` does today and the new package
mirrors it byte-for-byte. The lock file is intentionally never removed
(deleting it between unlock and a concurrent open would create a new inode
and break mutual exclusion).

On Windows, `lock_windows.go` is currently a **no-op** with a warning log;
that's the inherited behavior and this design does not change it. The
practical implication: concurrent `specgraph init` runs on Windows are not
serialized at the lock layer, but the full read-modify-write (including
legacy purge) is still computed in memory before a single atomic
`os.Rename`, so the worst case is "last writer wins" rather than a
partially-purged file on disk. Improving the Windows lock is out of scope
for this bead.

On Unix, concurrent `specgraph init` runs serialize per pointer file on its
respective `.lock` sibling. The full read-modify-write happens under one
lock, so a racing `init` either observes the pre-state or the post-state —
never a partially-purged intermediate.

### What `inject` removal touches

Concrete deletion list, derived from `rg -l '[Ii]nject'` against the working
tree on 2026-05-06. Each entry is either a full-file delete, a targeted
section/method/test removal, or a doc edit.

**Code — full-file deletes:**

- `cmd/specgraph/inject.go` — CLI subcommand entry point (`injectCmd`,
  `runInject`, `injectTool`, `injectOutput`).
- `internal/inject/` — entire package: `inject.go`, `inject_test.go`,
  `lock_unix.go`, `lock_windows.go`. Lock helpers are re-implemented inside
  `internal/config/pointers/` rather than imported, since `internal/inject/`
  is going away.

**Code — targeted edits:**

- `cmd/specgraph/sync_test.go` — delete the inject-specific tests
  (`TestInjectCmd_Flags`, `TestInjectCmd_RequiresSlug`,
  `TestInjectCmd_AcceptsSlug`, `TestInjectCmd_ToolAliases` — at the top of
  the file in the working tree, line numbers will drift by the time this
  ships). Leave the rest of the file (sync command tests) intact.
- `cmd/specgraph/docs.go` — drop `"inject"` from the `Server & Config`
  command-list entry (line 57 today).
- `cmd/specgraph/client.go` — no inject-specific code (the grep hits are
  unrelated header-injection comments). **No edit needed**, but listed here
  to head off a wrong cleanup.
- `internal/server/sync_handler.go` — delete the `Inject` method (begins
  around line 243 in the current working tree; the surrounding type and
  method definitions stay). Remove the `internal/inject` import. Verify no
  remaining references after the method is gone.
- `internal/server/sync_handler_test.go` — delete every `TestSyncHandler_Inject_*`
  test (11 tests in the working tree on 2026-05-06, including
  `_SpecNotFound`, `_MissingSlug`, `_MissingTool`, `_Success`,
  `_SuccessCursor`, `_SuccessAgentsMD`, `_ConstitutionWarning`,
  `_ConstitutionNotFound_NoWarning`, `_OutputDirOutsideRoot`, and
  `_EmptyOutputDir`). Keep helpers like `TestInjectProject` (unrelated —
  it's about test scoping) and the `errorSyncBackend` "inject errors"
  comment (not the same kind of inject).
- `internal/server/convert_constitution.go` — delete `injectToolFromProto`
  (line 267 today). Verify no remaining callers in the file.
- `internal/server/convert_constitution_test.go` — delete the corresponding
  unit tests for `injectToolFromProto`.
- `internal/server/convert_test.go` — delete `TestInjectToolFromProto` if
  present (the grep hit suggests there's a test file at this path; verify
  during implementation).
- `internal/storage/sync.go` — delete `InjectToolType`, `InjectToolClaudeCode`,
  `InjectToolCursor`, `InjectToolAgentsMD` (lines 31–38 today). These are
  orphaned the moment the handler stops calling them.
- `internal/auth/permissions.go` — delete the `SyncServiceInjectProcedure`
  entry. (Subagent confirmed this is the right file, not e2e/api/auth_test.go.)
- `internal/auth/permissions_test.go` — delete the
  `specgraphv1connect.SyncServiceInjectProcedure` reference at line 72.

**Proto:**

- `proto/specgraph/v1/sync.proto` — mark the `Inject` RPC method, the
  `InjectRequest` and `InjectResponse` messages, and the `InjectTool` enum
  as `reserved` per the SpecGraph proto-removal rule. Run `task proto` to
  regenerate.

**Generated code (regenerate, do not hand-edit):**

- `gen/specgraph/v1/sync.pb.go`
- `gen/specgraph/v1/specgraphv1connect/sync.connect.go`
- `web/src/lib/api/gen/specgraph/v1/sync_pb.ts`

After regen, `go build ./...` is the compile-time gate that catches any
caller the design missed.

**Docs:**

- `README.md` — remove the `specgraph inject` row in the Sync & Injection
  command table (lines 113–115 today). Rename the table section to "Sync"
  if it's the only inject row; otherwise leave the heading.
- `CLAUDE.md` — drop `internal/inject/` from the architecture table
  (line 62 today); add `internal/config/pointers/` next to
  `internal/config/mcpconfigs/`. Remove any "Gotchas" lines referencing
  inject targets.
- `AGENTS.md` (project root) — current grep shows zero inject hits in this
  file. **No edit needed** unless one appears during implementation.
- `site/docs/architecture.md` — remove the "Tool Injection" node from the
  Mermaid SyncService subgraph (line 30); update the SyncService row in the
  capability table (line 56) to drop "and inject context into tool files";
  remove the `inject/` entry from the directory tree (line 211). Replace
  with a brief mention of init-managed pointer files in the appropriate
  spot.
- `site/docs/cli-reference.md` — delete the `### specgraph inject` section
  (lines 954–onward).
- `site/docs/ecosystem.md` — remove the "Tool Injection" bullet (line 82)
  and update line 125 to drop the "Add tool injection for coding-agent
  context" framing.
- `CHANGELOG.md` — add an entry: `inject` removed; legacy per-slug AGENTS.md
  blocks are auto-purged on next `specgraph init`; orphan files under
  `.claude/specs/` and per-slug files under `.cursor/rules/` are not
  touched and may be deleted manually.

**Bead/CLI references in older plan docs (`docs/plans/2026-02-28-*`,
`docs/plans/2026-03-*`, `docs/plans/2026-04-*`, `docs/superpowers/`):**

- Historical plan documents reference `specgraph inject` as an active
  command. These are point-in-time records and are **not** edited by this
  PR. The CHANGELOG entry plus the design doc's predecessor reference
  serves as the bridge.

### Migration story

We deliberately do **not** clean up legacy inject-era files outside `AGENTS.md`:

- Per-slug `.claude/specs/<slug>.md` files: left on disk. CHANGELOG mentions
  they are now orphaned and can be deleted manually.
- Per-slug `.cursor/rules/specgraph-<slug>.md` files: left on disk. Same
  treatment.
- Per-slug blocks **inside** `AGENTS.md`: actively purged (see legacy purge
  above), because they are inside a file `init` is now writing to.

The asymmetry is deliberate. Inside files we manage, we own the contents
between markers and the surrounding context. Files we used to write but no
longer touch are not ours to delete.

## Testing strategy

### Unit tests for `internal/config/pointers/`

Following the pattern in `internal/config/mcpconfigs/sync_test.go`. Each test
gets a `t.TempDir()` projectDir and asserts on file content plus the
returned `SyncResult`.

| Test | What it verifies |
|---|---|
| `TestSync_CreatesAgentsMD` | File doesn't exist → `ActionCreated`, content matches template, ends in `\n`. |
| `TestSync_CreatesCursorRule` | File doesn't exist → frontmatter present (with `alwaysApply: true`), markers in body, `.cursor/rules/specgraph-bootstrap.md` is the path. |
| `TestSync_NoOpWhenIdentical` | File exists with current managed block → `ActionNoOp`, byte content unchanged. |
| `TestSync_UpdatesWhenContentDiffers` | File exists with stale serverURL → `ActionUpdated`, block reflects new value. |
| `TestSync_PreservesUserContentAroundBlock` | User content above and below the managed block is untouched after update. |
| `TestSync_OverwritesUserContentInsideBlock` | User edits *inside* the markers are overwritten by the canonical render. Locks down the documented contract. |
| `TestSync_AppendsBlockToFileWithoutMarkers` | File exists with user content but no markers → block appended with leading blank line; user content untouched. |
| `TestSync_PurgesLegacyInjectBlocks_SimpleSlugs` | Two per-slug legacy blocks (e.g. `foo`, `bar-baz`) in AGENTS.md → both removed, init block present, `LegacyBlocksPurged == 2`. |
| `TestSync_PurgesLegacyInjectBlocks_RealisticSlugs` | Per-slug blocks with **uppercase, dots, and underscores** in the slug (e.g. `MySpec.v2`, `my_spec`) — these are valid inject slugs per `safeSlugPattern` and MUST be purged. Regression guard against the original draft's too-narrow regex. |
| `TestSync_PurgesLegacyAndUpdatesInitBlock` | Legacy blocks AND a stale init block → all legacy purged, init block updated, count reported. |
| `TestSync_LegacyMarkerWithInvalidSlugNotPurged` | Marker with whitespace or path separators in the slug → not matched (not a real inject slug), not purged. Guards against false positives. |
| `TestSync_DoesNotPurgeInitMarker` | A canonical `<!-- specgraph:init:start v=1 -->` block is **not** matched by the legacy purge regex (the literal `v=1` suffix prevents the legacy regex from matching at all). |
| `TestSync_LegacyShapedInitMarkerIsCorruption` | An `<!-- specgraph:init:start --> ... <!-- specgraph:init:end -->` block (slug=`init`, no `v=1`) hits phase-1 corruption check (rule #4) and returns `ActionError` *before* the legacy purge phase runs. Locks down the phase-order contract. |
| `TestSync_RejectsCorruptedMarkers_EndBeforeStart` | End before start → `ActionError`, no write. |
| `TestSync_RejectsCorruptedMarkers_StartWithoutEnd` | Start with no end → `ActionError`, no write. |
| `TestSync_RejectsCorruptedMarkers_DoubleStart` | Two start markers with no end between → `ActionError`, no write. |
| `TestSync_RejectsInitMarkerWithoutVersion` | Start marker `<!-- specgraph:init:start -->` (no `v=1`) → `ActionError` with remediation text; no write. |
| `TestSync_RejectsSymlinkInPath` | AGENTS.md parent dir is a symlink → `ActionError`, no write. |
| `TestSync_RefusesCursorRuleWithoutFrontmatter` | Existing `.cursor/rules/specgraph-bootstrap.md` with no frontmatter → `ActionError` referencing the file and remediation; no write. |
| `TestSync_PreservesCursorRuleFrontmatter` | Hand-edited frontmatter (extra fields, custom description, `alwaysApply: false`) → frontmatter unchanged, only managed block updated. |
| `TestSync_FailureOnOneFileDoesNotAbortOther` | AGENTS.md has corrupted markers, cursor rule is fine → results slice has both entries; AGENTS.md is `ActionError`, cursor rule is `ActionCreated`/`ActionUpdated`. |
| `TestSync_AtomicWriteOnFailure` | Set the parent directory to read-only after the temp file is created (or use a per-test FS abstraction with a fault-injection wrapper) so `os.Rename` fails → original file's bytes unchanged. |
| `TestSync_ConcurrentInvocations` | Two goroutines call `Sync` concurrently → both succeed, file content is one or the other's complete output (no interleaving), no partial-purge state ever observed. |

### Integration tests in `cmd/specgraph/init_test.go`

| Test | What it verifies |
|---|---|
| `TestInit_FreshProject_WritesPointers` | New project → `init` writes both pointer files with correct content; output includes both new rows. |
| `TestInit_RerunIsNoOp` | After a successful first run, re-running produces all `no-op` across all five files. |
| `TestInit_PurgesLegacyInjectArtifacts` | AGENTS.md seeded with legacy per-slug blocks (including ones with dots, underscores, and uppercase) → run reports purge count, init block present, all legacy blocks gone. |
| `TestInit_PointerErrorDoesNotAffectMcpConfigs` | Force a pointer write to fail; mcpconfigs results are still printed first and reflect their actual state. Init exits non-zero. |
| `TestInit_McpconfigsAbortStillPrintsPartial` | Force `mcpconfigs.Sync` to fail on the second config; first config's result is printed; `pointers.Sync` is **not** called; exit non-zero. (Regression guard for the existing abort-on-error semantics.) |

### E2E and proto verification

- `e2e/api/auth_test.go` and `e2e/api/helpers_test.go` — current grep shows
  zero inject-RPC references in these files. (The earlier draft of this
  design listed them in error; the only matches were unrelated header-
  injection comments.) **No e2e edits expected**, but verify during
  implementation; if any inject e2e cases exist, delete them.
- After `task proto`, verify `gen/specgraph/v1/sync.pb.go` no longer contains
  `InjectRequest` / `InjectResponse` / `InjectTool`, and the connect service
  interface has no `Inject`. Verify the same for
  `gen/specgraph/v1/specgraphv1connect/sync.connect.go` and
  `web/src/lib/api/gen/specgraph/v1/sync_pb.ts`.
- `go build ./...` is the compile-time gate: every removed type or method
  must have all callers also removed for the build to pass. Run before
  every commit.

### Lint, format, DCO

`task check` must pass. New `pointers/` package needs a doc comment on its
first `.go` file (revive). License headers on every new file. DCO sign-off on
the commit.

### Manual smoke test (in implementation plan, not design)

Because the legacy purge is destructive, manual verification on a real repo
that has inject artifacts is non-negotiable before merge:

1. Confirm AGENTS.md gets the new init block.
2. Confirm legacy per-slug blocks in AGENTS.md are gone (including any with
   uppercase, dots, or underscores in the slug).
3. Confirm `.cursor/rules/specgraph-bootstrap.md` is created (and that the
   pre-existing `.cursor/rules/specgraph.md` from the harness-parity epic
   is **untouched**).
4. Confirm orphan `.claude/specs/<slug>.md` files are still on disk
   (we explicitly chose not to touch them).
5. Re-run `init`; confirm all `no-op`.
6. Manually edit a managed block; re-run `init`; confirm restoration without
   touching content outside the block.
7. Manually edit content **outside** the managed block (e.g. add a section
   above it); re-run `init`; confirm the user's section is preserved.

### Out of scope for testing

- Windows-specific behavior. `lock_windows.go` is lifted as-is; the project's
  existing best-effort Windows posture stands.
- Performance. Pointer files are ~15 lines, exactly two of them per project.

## References

- Bead: `spgr-yyjf` — Deprecate `specgraph inject` in favor of MCP + extended
  `init`.
- Predecessor epic:
  [`docs/plans/2026-05-06-harness-parity-epic-design.md`](2026-05-06-harness-parity-epic-design.md).
  Note especially the existing
  [`plugin/cursor/.cursor/rules/specgraph.md`](../../plugin/cursor/.cursor/rules/specgraph.md)
  shipped by that epic — this design deliberately uses
  `.cursor/rules/specgraph-bootstrap.md` as a sibling, not an overlay.
- Existing init writers:
  [`internal/config/mcpconfigs/configs.go`](../../internal/config/mcpconfigs/configs.go),
  [`internal/config/mcpconfigs/sync.go`](../../internal/config/mcpconfigs/sync.go).
- Inject internals (to be deleted):
  [`internal/inject/inject.go`](../../internal/inject/inject.go),
  [`internal/inject/lock_unix.go`](../../internal/inject/lock_unix.go),
  [`cmd/specgraph/inject.go`](../../cmd/specgraph/inject.go).
- Inject callers across the rest of the tree (all to be edited or deleted):
  [`internal/server/sync_handler.go`](../../internal/server/sync_handler.go),
  [`internal/server/convert_constitution.go`](../../internal/server/convert_constitution.go),
  [`internal/storage/sync.go`](../../internal/storage/sync.go),
  [`internal/auth/permissions.go`](../../internal/auth/permissions.go).
- MCP resources that cover the same content surface inject wrote:
  [`internal/mcp/resources.go`](../../internal/mcp/resources.go) — handlers
  for `specgraph://spec/{slug}` and `specgraph://constitution`.
- Current `init` flow:
  [`cmd/specgraph/init.go`](../../cmd/specgraph/init.go).
