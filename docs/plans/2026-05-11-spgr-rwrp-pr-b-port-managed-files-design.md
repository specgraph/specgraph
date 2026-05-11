# PR B — Port existing managed files into the `managedfiles` framework

- **Bead:** `spgr-rwrp.1` (child of `spgr-rwrp`)
- **Predecessor:** `spgr-vqg7` (PR A — framework foundation, merged in `5a10ce5`)
- **Parent design:** `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` §"PR B"
- **Date:** 2026-05-11

## Goal

Migrate the five files specgraph already manages onto the new
`internal/config/managedfiles/` framework, replace PR A's strategy
stubs with real implementations, delete `internal/config/mcpconfigs/`
and `internal/config/pointers/`. Behaviour for the dogfood repo and
any future end user must be byte-identical to today's `init` output
modulo the marker upgrade from `v=1` to `v=2`.

**Precise meaning of "byte-identical."** Three classes of bytes
exist in a managed file, each with its own equivalence claim:

1. **Marker lines.** v=1 emits `<!-- specgraph:init:start v=1 -->`;
   v=2 emits `<!-- specgraph:init:start v=2 sha256=<hash> -->`
   (≈71 additional bytes per block). Replaced wholesale on every
   v=1 → v=2 upgrade. Migration tests do **not** diff marker lines.
2. **Bytes strictly between markers.** Identical, byte-for-byte.
   Migration tests assert this.
3. **Bytes outside markers**, *minus the legacy-block surface*.
   Identical, byte-for-byte. Migration tests assert this **after
   subtracting** any `specgraph:<slug>:start/end` per-slug blocks
   that `purgeLegacyBlocks` removes (`markdownBlockStrategy` step 4
   below). The legacy-block surface is a third managed region whose
   sole canonical state is "removed"; bytes there are not preserved
   by design.

So the byte-equivalence guarantee is: between-markers identical,
outside-markers-minus-legacy-blocks identical, marker lines and
legacy blocks replaced wholesale. JSON files have no markers and
no legacy blocks; their "byte-identical" claim is the simpler
"`canonicalize(MergePatch(...))` reproduces today's
`mcpconfigs.canonicalize(MergePatch(...))`."

## Scope

In scope:

| Path | Strategy | Notes |
|---|---|---|
| `.mcp.json` | `JSONKeyMerge` | Claude Code |
| `.cursor/mcp.json` | `JSONKeyMerge` | Cursor |
| `opencode.json` | `JSONKeyMerge` | Managed keys: `mcp.specgraph.{enabled,headers,type,url}` + `$schema` (top-level). Refuses if `opencode.jsonc` exists. |
| `AGENTS.md` | `MarkdownBlock` | Claude |
| `.cursor/rules/specgraph-bootstrap.mdc` | `MarkdownBlock` | Cursor. `SupersedesPath: ".cursor/rules/specgraph-bootstrap.md"`. |

**Note on `$schema`.** Parent design table (`§"Per-harness manifest"`)
lists managed keys for `opencode.json` as
`mcp.specgraph.{enabled,headers,type,url}` and omits `$schema`.
Today's `mcpconfigs.openCodeConfig` (`configs.go:113-128`) includes
`"$schema": "https://opencode.ai/config.json"` in the patch.
Byte-identical migration requires PR B preserve today's behaviour
— `$schema` stays managed. The parent design's omission is a
documentation gap to reconcile separately.

**Posture inheritance: `$schema` is managed-value, not
managed-presence.** RFC 7396 merge applied with a patch containing
`"$schema": "<url>"` overwrites the user's value on every init.
That's intentional: if a future specgraph release changes the
schema URL (e.g., a major opencode schema bump), every user's
file updates silently on next init. The only sibling key with
"managed-presence" semantics in the entire framework is the future
`enabledPlugins["specgraph@specgraph-local"]` in PR E's
`.claude/settings.json` (parent design §"Drift detection" line 107).
All other keys are managed-value.

Out of scope (later PRs):

- `opencode.json`'s `plugin[]` union-merge (PR C)
- `.claude/settings.json` (PR E)
- `harnesses:` field in `.specgraph.yaml` (later PR — PR B hard-codes
  all three harnesses)
- Drift nudge / `PersistentPreRun` (PR G)
- `specgraph doctor` (PR G)
- **`--force` / `--keep-edits` CLI flags on `init`** — the framework's
  `SyncOptions` carries both fields, but `cmd/specgraph/init.go`
  passes `SyncOptions{}` (zero-value, both false). Surfacing the
  flags is `specgraph doctor --fix`'s job in PR G; PR B has no
  user-facing override for drifted-file overwrite. Documented here
  so a reader of the init.go snippet doesn't expect a `--force`
  flag this PR doesn't introduce.
- **`--dry-run` flag.** Parent design §"Dogfood discipline" cites
  `specgraph init --dry-run --check` invoked from `task plugin:check`.
  PR B does not add `--dry-run`; that work lands in PR G alongside
  `doctor`, which is the natural home for inspect-only output. The
  `task plugin:check` wiring is also deferred to PR G.

## Architecture

### Per-project params plumbing

The five PR-B files all interpolate `slug` + `serverURL`. The
framework needs to thread per-project values into the manifest
without coupling strategy code to specgraph's project-config types.

`ManagedFile` grows one optional field:

```go
type ManagedFile struct {
    Path           string
    Strategy       Strategy
    Source         string                              // for WholeFile (PR C+)
    Build          func(ProjectParams) ([]byte, error) // for JSONKeyMerge + MarkdownBlock
    Comment        CommentSyntax
    Harness        Harness
    SupersedesPath string
}
```

`Source` and `Build` are mutually exclusive — a manifest entry uses
one or the other. Validation lives in a `func init()` in
`manifest.go` that iterates `allManagedFiles()` and panics on shape
violations (both set, neither set, or wrong combination for the
strategy: `JSONKeyMerge|MarkdownBlock` require `Build`, `WholeFile`
requires `Source`). A `TestManifestShape` unit test exercises the
same invariants for belt-and-suspenders coverage that survives test
binaries built without `init()` side-effects.

**`Source` is unused-but-present in PR B.** Every PR B manifest
entry uses `Build`; no entry uses `Source`. `readSource`
(`source.go:21-26`) is reachable only from `wholeFileStrategy`,
which itself stays stubbed until PR C. So `Source`, `readSource`,
`source_release.go`'s embed FS, and the `dev` build tag's
`source_dev.go` are all scaffolding that PR C activates.

Removing them in PR B and re-adding in PR C is rejected: the
strategy interface and `func readSource` test scaffolding from PR A
were sized to support all three strategies, and the v=1 → v=2
upgrade plus the migration test suite already touch enough of the
package's surface that adding another delete-and-readd cycle is
churn for no gain. Spec explicitly notes the fields stay; any code
reviewer who flags them as dead is referred to this section.

Public API gains a `ProjectParams` parameter:

```go
type ProjectParams struct {
    Slug      string
    ServerURL string  // resolved, including scheme + host (no /mcp/ suffix)
}

func (p ProjectParams) Validate() error // lifts pointers.NewOptions logic

func InspectAll(cwd string, harnesses []Harness, params ProjectParams) ([]FileState, error)
func SyncAll(cwd string, harnesses []Harness, params ProjectParams, opts SyncOptions) ([]SyncResult, error)
```

Single-file variants (`Inspect`, `Sync`) take the same params and
forward to the strategy implementation.

This breaks the function signatures PR A shipped. All PR A call
sites are inside `internal/config/managedfiles/` itself (no external
callers in PR A; the new `cmd/specgraph/init.go` call site is added
in this same PR). PR A's `inspect_test.go` and `integration_test.go`
update to pass a zero or fixture `ProjectParams` — small mechanical
diff, no behaviour change.

**Signature cascade.** The strategy interface (`strategy.go:15-18`)
also takes `params`:

```go
type strategy interface {
    Inspect(cwd string, mf ManagedFile, params ProjectParams) (FileState, error)
    Sync(cwd string, mf ManagedFile, params ProjectParams, opts SyncOptions) (SyncResult, error)
}
```

This change cascades through:

- All three strategy stub implementations in `strategy.go`
  (JSONKeyMerge, MarkdownBlock, WholeFile — even though WholeFile
  remains stubbed, its interface conformance updates).
- The package-level `Inspect(cwd, mf, params)` (`inspect.go:25`) and
  `Sync(cwd, mf, opts)` → `Sync(cwd, mf, params, opts)` (`sync.go:15`).
- `InspectAll` and `SyncAll` both forward `params` to the per-file
  call.
- PR A's `strategy_test.go` exercises the interface conformance and
  updates to match.

`ProjectParams.Validate` rejects:

- empty or non-`safeSlugPattern` slug (`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
- server URL that isn't an absolute http/https URL with a non-empty host

Init calls `params.Validate()` once before `SyncAll`; strategy code
trusts the inputs.

### Manifest population

`manifest.go` grows entries (one per row in the in-scope table
above). Each `JSONKeyMerge` entry's `Build` closure mirrors the
respective `mcpconfigs.{cursor,claudeCode,openCode}Config` body, with
`Build` returning the canonicalized result of applying the patch to
either `{}` (Missing case, computed inside the strategy) or the
on-disk content. For symmetry, `Build` returns just the **patch
document** (the bytes today's `mcpconfigs.ManagedConfig.Patch` would
hold), not the merged result; the strategy applies the patch.

Markdown-block entries' `Build` returns the **block body content**
(the bytes that go between the start/end markers, excluding markers
themselves). The strategy frames it with markers + hash sentinel.

### Concurrency: goroutine safety of strategy code

`SyncAll` iterates serially in PR B. PR G's `doctor --fix` may
parallelise per-file Sync calls in the future. The contract PR B
commits to: **strategy methods are reentrant and share no mutable
state.** Package-level regexes (`legacyBlock`, `initStartLoose`,
`initStartAnyVersion`, `safeSlugPattern`) use `regexp.Regexp`,
which the standard library documents as safe for concurrent use.
The per-file `lock.go` primitive (PR A's port of
`pointers.acquireFileLock`) provides exclusion at the path level.
No package-level `sync.Mutex` or shared map is introduced. Stated
on the strategy interface's godoc so PR G can rely on it.

### Concurrency: Inspect vs Sync

`Sync` for `MarkdownBlock` and `JSONKeyMerge` takes a per-file
exclusive lock via PR A's `lock.go` primitives (ported from
`pointers/lock_unix.go` / `lock_windows.go`). `Inspect`
intentionally does **not** lock — it's expected to be cheap and
called from `doctor` and the drift-nudge on every CLI invocation;
serialising it behind a write lock would defeat the design.

The Inspect-during-Sync race is bounded by POSIX rename atomicity
(`pointers/sync.go:223-228` and the new framework's `atomicWrite`):
Sync writes a temp file and `os.Rename`s over the target, so
Inspect's `readFileNoFollow` either sees the pre-write content or
the post-write content, never a partial. On Windows, `os.Rename`
provides equivalent atomicity since Go 1.5 for same-volume renames.

What Inspect cannot guarantee: that its returned `State`
classification reflects the file's state at the *moment* the
caller acts on it. A `Synced` result followed by a concurrent
Sync from another process makes the next read see different
bytes. Callers (doctor, drift-nudge) treat Inspect as advisory.
The drift-nudge already prints "run `specgraph init` to refresh"
rather than acting; doctor's `--fix` mode acquires the lock via
Sync.

Documented as an invariant on `Inspect`'s godoc: "advisory; no
lock held; concurrent Sync may invalidate the result immediately."

### Build closure purity

`ManagedFile.Build` MUST be a pure function of `ProjectParams`:
same input → byte-identical output, no FS reads, no clock, no env
vars, no randomness. The framework invokes `Build` independently
on every `Inspect` and `Sync` call (no caching) — without purity,
Inspect could classify a file as `Synced` and a subsequent Sync
could compute a different canonical, producing `ActionRefreshed`
on a file the user never edited.

Stated on the `ManagedFile.Build` field's godoc. `TestManifestShape`
asserts purity for each registered entry by invoking `Build` twice
with the same fixed `ProjectParams{Slug: "test", ServerURL: "http://localhost:9090"}`
and comparing bytes. The test is purity-only — output *correctness*
is covered by the golden tests. The fixture-params value is a
package-level test constant so all purity assertions share the
same input.

A `Build` panic is treated as a programmer error (corrupt manifest
entry, typo on a map access), not a runtime condition. The
framework does not `recover()` inside `Sync`/`Inspect`; a panic
aborts the CLI with the standard Go panic trace. This matches the
"panic if manifest is malformed" choice in §"Per-project params
plumbing" — manifest issues are caught loudly, not silently.

### Atomic write: mode preservation

PR A's `atomic.go` mirrors `pointers/sync.go:185-234` byte-for-byte
on the rename/fsync path, but it relies on the **caller** to pass
the correct file mode. Today's `pointers/agents.go:116-119` and
`pointers/cursor.go:104-107` each `os.Stat` the existing file and
fall back to `0o600` if missing.

Both PR B strategies must apply the same rule. Stated as a
cross-strategy obligation: **before calling `atomicWrite`, the
strategy calls `os.Stat` on the target; if it exists, pass
`info.Mode().Perm()`. If missing, pass `0o600`.** Hardcoding
`0o600` regardless would change permissions on any user-`chmod
644`'d managed file — a silent regression caught only by a
post-migration `ls -l`.

The strategy unit tests assert the obligation: a fixture file
pre-chmod'd to `0o644`, after sync, must still be `0o644`.

### Strategy implementations

#### `jsonKeyMergeStrategy`

`Sync`:

1. `rejectSymlinkComponents` for the file path (already in framework).
2. For `opencode.json` only: refuse if `opencode.jsonc` exists at
   the same directory (`os.Lstat`, blocks dangling symlinks too).
   Return wrapped error; surfaced as `ActionError`.
3. Read existing file via `readFileNoFollow`. If missing, treat as
   `{}`. If present, validate it parses as JSON before merging.
4. `mf.Build(params)` returns the patch document.
5. Apply RFC 7396 merge patch using `github.com/evanphx/json-patch/v5`
   (already a project dep).
6. Canonicalize: `json.MarshalIndent(v, "", "  ")` + trailing newline.
   Map keys emitted alphabetically — deterministic.
7. If file existed and canonicalized output equals existing bytes:
   `ActionNoOp`. Else `atomicWrite` (mode preserved from existing if
   present, else `0o600`) → `ActionCreated` or `ActionRefreshed`.

`Inspect`:

- Missing file → `StateMissing`.
- Present, valid JSON, post-merge bytes equal existing → `StateSynced`.
- Present, post-merge bytes differ → `StateStale`.
- No `StateDrifted` for this strategy — managed keys are by definition
  always overwritten. `FileState.SentinelHash` left empty.

`Force` and `KeepEdits` are no-ops for `JSONKeyMerge` (no Drifted
state to override). Documented in the strategy doc-comment.

##### `Stale` semantics are near-permanent for JSONKeyMerge

Any whitespace difference between the user's file and the
canonical (different indent, key order in user-added siblings,
trailing newline) trips the `bytes.Equal` short-circuit and
classifies as `Stale`. This is **expected**: the strategy
canonicalizes to 2-space indent + alphabetical keys + trailing
newline on every write. A user who edits opencode.json by hand
with 4-space indent will see `Stale` until they accept the
canonicalization (next `init` rewrites to 2-space; their `theme`
sibling is preserved because RFC 7396 merge leaves untouched keys
alone).

Doctor and the drift-nudge surface this honestly: "stale" means
"the framework will rewrite on next init; user content under
non-managed keys will be preserved, but formatting will be
canonicalized." It's not a corruption signal. The strategy's
godoc and doctor's stale-line copy explicitly note this.

#### `markdownBlockStrategy`

##### Hash scope

Crucial invariant: for `MarkdownBlock`, the sentinel hash covers
**only the bytes between the start and end markers**, not the
whole file. Any other choice (e.g., the whole-file-minus-marker-lines
behaviour of PR A's `HashExcludingSentinel(CommentHTML, ...)`)
would classify any outside-block user edit (a paragraph added below
`<!-- specgraph:init:end -->` in `AGENTS.md`) as `Drifted` and
refuse to overwrite — a regression from today's `pointers/` package,
which preserves outside-block content bit-for-bit on every sync.

PR B adds a new private helper alongside `hash.go`:

```go
// extractManagedBlockBody returns the bytes strictly between the
// canonical start and end markers, or (nil, false) if no well-formed
// pair is present. The bytes do NOT include the surrounding marker
// lines or any leading/trailing newline adjacent to them.
//
// "Well-formed pair" means: exactly one start marker (v=1 OR v=2 —
// both are recognised so the same helper serves the v=1 defensive
// recompute path and the v=2 hash check), exactly one end marker,
// end strictly after start. Anything else returns (nil, false).
// Multiple starts, end-before-start, or unmatched markers should
// have been caught earlier by validateInitMarkers; if extract is
// called on such input it returns (nil, false) rather than guessing.
// Empty body between markers returns ([]byte{}, true) (non-nil
// empty slice) — semantically "the block exists and is empty," not
// "no block."
func extractManagedBlockBody(content []byte) ([]byte, bool)
```

The `markdownBlockStrategy` calls `extractManagedBlockBody` then
`hashBytes` (also private in `hash.go`) for both disk-side and
canonical-side hashing. `HashExcludingSentinel(CommentHTML, ...)`
is left alone — it remains the right primitive for whole-content
hashing when that's ever needed, but `markdownBlockStrategy` does
not use it.

PR A's `hash_test.go` `CommentHTML` cases keep passing (semantics
unchanged); new tests cover `extractManagedBlockBody` round-tripping
and edge cases (no markers, end-before-start, multiple starts).

##### Canonical block layout

`mf.Build(params)` returns the block body bytes (without markers).
The framework wraps it as:

```text
<!-- specgraph:init:start v=2 sha256=<hash> -->
<block body>
<!-- specgraph:init:end -->
```

`<hash>` is `hashBytes(block body)` — the bytes `mf.Build` returns,
verbatim. Identical to the bytes `extractManagedBlockBody` recovers
from a freshly-written canonical file.

`Sync` algorithm (after symlink rejection, file lock, no-follow read):

1. **Missing file.** Render canonical block. For files that need
   frontmatter (the `.mdc` file), prefix the default frontmatter.
   Write atomically. Return `ActionCreated`. Then run
   `supersedesGuardedDelete` if `mf.SupersedesPath != ""` (see below).
2. **File exists, no specgraph markers in body.** Append canonical
   block (separated by a blank line). Return `ActionCreated`.

   This is a **deliberate semantic divergence** from
   `pointers.syncAgents`, which returns `ActionUpdated` for this
   case (the file existed, it was updated to add a block). The
   framework reorients the noun: `Action` describes what happened
   to the *managed unit*, not the surrounding file. For
   `MarkdownBlock`, the managed unit is the block; if the block
   didn't exist before and now does, that's `ActionCreated`.
   Consistent with `WholeFile`'s `ActionCreated` semantics in PR C.

   The golden-test capture step records the **new** semantics —
   capturing rewrites every `pointers.ActionUpdated` to either:

   - `ActionCreated` — when the old code added a block to a file
     that didn't have one (no specgraph markers before).
   - `ActionRefreshed` — when the old code replaced an existing
     block in-place (markers existed; body content differed). This
     covers `pointers.syncCursor` (`cursor.go:111`) and the v=2-block-
     replace branch of `syncAgents`.

   The capture helper records the input fixture's pre-state so the
   rewrite is deterministic per case. Byte-output goldens stay
   valid; only the `SyncResult.Action` value differs.
3. **File exists with markers.** Parse markers; reject corrupted
   markers (more than one start, end before start, etc.) by porting
   and **adapting** `validateInitMarkers` from `pointers/agents.go`.
   Two adaptations are required:

   - **Version acceptance** (Rule 5 today, `agents.go:140-146`):
     existing rejects any marker not byte-equal to literal `v=1` —
     a verbatim port would refuse to read any `v=2` file the
     framework itself writes. Delegate to `ParseSentinel`
     (`sentinel.go:32` already encodes `{1: true, 2: true}` as
     supported).
   - **Canonical-start lookup** (Rule 4 today, `agents.go:148-162`):
     existing computes `canonical := bytes.Index(data, []byte(initStart))`
     where `initStart` is the **v=1 literal**. For a v=2-only file
     this returns -1, breaking the "overlap with canonical start"
     exception — a stray naked `<!-- specgraph:init:start -->`
     would falsely trip Rule 4. The ported validator computes
     canonical-start positions via `initStartAnyVersion` regex
     instead, so any well-formed start marker (v=1 or v=2) acts as
     a canonical anchor.

   The validator otherwise only checks structural invariants
   (count, ordering, no-naked-marker without version). Then classify:
   - **v=2 markers.** Compute hash of disk block body. If matches the
     sentinel's recorded hash:
     - If matches canonical hash → `StateSynced` / `ActionNoOp`.
     - Else → `StateStale` / `ActionRefreshed` (overwrite, fresh
       sentinel).

     If disk hash doesn't match recorded sentinel hash →
     `StateDrifted`. Skip without `--force`; `--force` rewrites;
     `--force --keep-edits` refreshes the sentinel to match disk.

   - **v=1 markers.** Defensive recompute: call private
     `renderV1MarkdownBlockBody(mf, params)` for the same file's prior
     canonical, hash it, compare to disk block body.
     - Match → treat as `StateStale`, rewrite to v=2 with fresh hash.
     - Mismatch → `StateDrifted`. Refuse without `--force`.

4. **Legacy `specgraph:<slug>:start/end` per-slug blocks.** Port
   `purgeLegacyBlocks` from `pointers/agents.go`. Applied only to
   `AGENTS.md` (the only file that ever had them). Cursor file skips
   the purge. Purge counts surfaced via `SyncResult.Detail`:
   `"purged 2 legacy blocks"` / `"skipped 1 malformed legacy block"`.

`Inspect` mirrors `Sync`'s classification without writing.

#### `wholeFileStrategy` (still stub in PR B)

Unchanged from PR A. Real implementation lands in PR C. Manifest
contains zero `WholeFile` entries after PR B, so the stub's
`errNotImplemented` is unreachable. The `errNotImplemented` sentinel
in `errors.go` therefore stays in PR B — removal is deferred to PR C
where the third stub disappears. This diverges from the user-brief's
"remove `errNotImplemented` as part of this PR" instruction;
following the design-doc's PR sequencing (WholeFile is PR C) is
authoritative.

### Helpers ported from `pointers/` and `mcpconfigs/`

The PR B migration carries forward more than the two body renderers
named in earlier drafts. The full port list, with target visibility
in the new `managedfiles/` package:

**Live-path helpers** (used by the production write path, not
vestigial):

- `splitFrontmatter(data []byte) (front, body []byte, err error)`
  — ported from `pointers/cursor.go:117-137`. The `.mdc` Sync path
  reuses it: existing `.mdc` files have YAML frontmatter that must
  be preserved verbatim while the framework manipulates only the
  body's managed block. Private to `managedfiles/`.
- `ErrFrontmatterMissing` sentinel — ported from
  `pointers/errors.go`. Returned by `splitFrontmatter`; surfaced
  via `SyncResult.Err` when the file pre-exists without `---`.
- `defaultCursorFrontmatter` constant — ported from
  `pointers/cursor.go:18-23`, including the trailing blank line.
  Used both for (a) writing a fresh `.mdc` (live path) and (b)
  computing the prior-canonical hash for the `.md` supersedes check
  (vestigial path).
- `safeSlugPattern` regex — ported from `pointers/sync.go:71`. Used
  by `ProjectParams.Validate`.
- `purgeLegacyBlocks` + `legacyBlock` regex — ported from
  `pointers/agents.go:184-202`. AGENTS.md only; runs on every Sync
  to remove pre-init per-slug blocks. Counts surface via
  `SyncResult.Detail`.
- `validateInitMarkers` + `initStartLoose` + `initStartAnyVersion`
  regexes — ported and adapted from `pointers/agents.go:134-182` per
  §"markdownBlockStrategy" step 3. Adaptations covered there.
- `canonicalize(data) []byte` — ported from `mcpconfigs/sync.go:180-190`.
  2-space indent + alphabetical keys + trailing newline. Used by
  `jsonKeyMergeStrategy.Sync` step 6.

**Vestigial v=1 renderers** (NOT on the production write path; only
invoked for v=1-marker upgrade hash-check and supersedes
prior-canonical computation):

- `renderV1AgentsBlockBody(params ProjectParams) []byte` — ported
  from `pointers/agents.go:41-52` (today's `renderAgentsBlock`).
- `renderV1CursorBlockBody(params ProjectParams) []byte` — ported
  from `pointers/cursor.go:25-27` (today's `renderCursorBody`).

**Sunset trigger correction.** The parent design's sunset trigger
(`task plugin:check` reports zero v=1 files for two consecutive
releases) was written assuming v=1 files existed in user
deployments. As established in §"Synthetic v=1 → v=2 upgrade
integration test", **no v=1 files exist anywhere at PR-B-merge**
— not in this repo, not in any end user's checkout (parent design
§Decisions: "no existing users"). A literal reading of the parent
trigger would fire on the first release post-PR-B and delete the
helpers in PR C, defeating their purpose.

PR B adds **a second clause** to the sunset trigger, to apply
only to these helpers: deletion requires (a) zero v=1 files for
two consecutive releases AND (b) at least 6 months elapsed since
v=2 rollout, allowing for the case where a future user has been
running an old binary against an unupdated managed file. Whichever
condition the second-clause-bearing release lands later, that's
when deletion can proceed.

Live-path helpers (the rest of the port list) have no sunset
trigger; they stay until the strategy they support is replaced.

Cross-strategy primitives PR A already landed (atomic write, lock,
symlink rejection, no-follow open) are not re-ported — strategies
call them directly via the framework's existing primitives.

Cursor file's predecessor (`.md`) also carried frontmatter; the
guarded-delete's hash compares disk content against
`frontmatter + renderV1CursorBlockBody(params) + body markers` — the
full file the prior canonical would have produced.

### `SupersedesPath` for the `.md` → `.mdc` rename

Manifest entry for `.cursor/rules/specgraph-bootstrap.mdc` carries
`SupersedesPath: ".cursor/rules/specgraph-bootstrap.md"`.

`supersedesGuardedDelete` runs **conditionally on the new file's
`Action`**:

| New file's Action | Run guarded delete? |
|---|---|
| `ActionCreated` | yes — fresh write, clean transition |
| `ActionRefreshed` | yes — v=1→v=2 upgrade case |
| `ActionForced` | yes — user already opted in to overwriting |
| `ActionNoOp` | yes — steady-state cleanup if the old file lingers (e.g., second `init` run on a partially-migrated repo) |
| `ActionSkipped` | **no** — the new file is Drifted and user content is preserved on the `.mdc`; mirror that by preserving the `.md` too |
| `ActionError` | no |

When it runs, it computes the prior canonical bytes for the old path
(full file: frontmatter + v=1 body + markers — see "Prior canonical
exact bytes" below), hashes them, compares to on-disk hash. Match →
delete. Mismatch → leave + surface as `Drifted` on the supersedes
path in the SyncResult's `Detail`.

#### Prior canonical exact bytes (cursor `.md`)

The dogfood `.md` was written by `pointers/cursor.go` as:

```text
---
description: SpecGraph MCP bootstrap — points the agent at the running SpecGraph server.
alwaysApply: true
---

<!-- specgraph:init:start v=1 -->
<v=1 body from renderAgentsBlock(params)>
<!-- specgraph:init:end -->
```

Critical: there is a trailing blank line after the closing `---`
(see `pointers/cursor.go:18-23`), and `renderCursorBody` appends a
`\n` after the end marker (`pointers/cursor.go:27`). The prior
canonical byte sequence is `defaultCursorFrontmatter +
renderAgentsBlock(params) + "\n"`. Off-by-one whitespace breaks the
hash match and the dogfood `.md` won't be deleted on migration —
the spec-level integration test must assert the exact dogfood case
passes.

### `SyncResult.Detail` string grammar

`Detail` is human-readable; for programmatic access PR G adds typed
fields. PR B pins the exact wording for the small set of cases that
emit non-empty `Detail` so unit tests have a fixed target and three
contributors don't write three different strings.

| Source | `Detail` exact string |
|---|---|
| `purgeLegacyBlocks` removed N>0 entries, no skipped | `purged %d legacy block(s)` (singular `block` when N=1) |
| `purgeLegacyBlocks` skipped M>0 malformed | `skipped %d malformed legacy block(s)` (singular when M=1) |
| Both purge counts > 0 | `purged %d legacy block(s); skipped %d malformed` (semicolon-separated; matches today's `pointers.SyncResult` rendering at `init.go:135-141`) |
| Supersedes-path Drifted (old file content doesn't match prior canonical) | `supersedes path %q left in place: prior-canonical mismatch` |
| Supersedes-path deleted successfully | (empty — no Detail; success is silent) |
| `Force=true, KeepEdits=true` on Drifted MarkdownBlock | `kept user edits; sentinel hash refreshed to match disk` |
| All other paths | empty |

A test fixture file `testdata/detail-grammar.txt` documents the
grammar; strategy unit tests assert against the fixture so the
strings drift as one unit.

### Init wiring

**Scope of replacement.** The snippet below replaces *only*
`init.go:96-153` — the `pointers.NewOptions` call, the `mcpconfigs`
call/loop, the `pointers.Sync` call/loop, and their per-file error
handling. Everything else in `init.go` is preserved verbatim:

- The project-config load + slug-derivation block (`init.go:36-89`),
  including `existing == nil` detection.
- `loadGlobalCfg()` + `globalCfg.ResolveServer(pc.Slug, pc.Server)`
  (`init.go:91-95`) — `serverURL` flows from this into the new
  `ProjectParams`.
- The `existing == nil` → `config.WriteProject(cwd, pc)` block
  (`init.go:102-109`) and its `projectCreated = true` flag —
  **must run before `SyncAll`** so `.specgraph.yaml` exists for any
  future strategy that consults it.
- The final `if projectCreated { fmt.Printf("Initialized project ...") }`
  message (`init.go:149-151`) — user-visible on first init.

`cmd/specgraph/init.go` becomes (only the central block is shown
here; preserved-verbatim regions bracket it):

```go
params := managedfiles.ProjectParams{Slug: pc.Slug, ServerURL: serverURL}
if err := params.Validate(); err != nil {
    return fmt.Errorf("validate project params: %w", err)
}

// Hard-coded for PR B; .specgraph.yaml-driven list lands later.
harnesses := []managedfiles.Harness{
    managedfiles.HarnessClaude,
    managedfiles.HarnessCursor,
    managedfiles.HarnessOpenCode,
}

results, err := managedfiles.SyncAll(cwd, harnesses, params, managedfiles.SyncOptions{})
var failedPaths []string
for _, r := range results {
    if r.Action == managedfiles.ActionError {
        fmt.Fprintf(os.Stderr, "%s: error: %v\n", r.Path, r.Err)
        failedPaths = append(failedPaths, r.Path)
    } else {
        line := fmt.Sprintf("%s: %s", r.Path, managedfiles.ActionName(r.Action))
        if r.Detail != "" {
            line += " (" + r.Detail + ")"
        }
        fmt.Println(line)
    }
}
if err != nil {
    return fmt.Errorf("sync managed files: %w", err)
}
// Match today's pointers-error message shape (init.go:145-147):
// "sync managed files: N failed: <comma-separated paths>".
if len(failedPaths) > 0 {
    return fmt.Errorf("sync managed files: %d failed: %s",
        len(failedPaths), strings.Join(failedPaths, ", "))
}
```

The `failedPaths` accumulator preserves today's user-facing error
shape (`init.go:125-147`) — a comma-separated list of which paths
failed, not just a count. Doctor (PR G) will provide richer
reporting, but PR B's interim init must not regress today's
diagnostic surface.

`ActionName(Action) string` and `CountErrors([]SyncResult) int`
(used by doctor, not init) are exported helpers on the
`managedfiles` package, not `cmd/specgraph`-private. PR G's
`doctor` consumes the same names for its action column; defining
them once on `managedfiles` avoids divergent action-name copy
between `init` and `doctor`. `ActionName` is a switch-on-iota
function (Created → "created", Refreshed → "refreshed", etc.).

`SyncResult` grows a `Detail` field (string) to carry purge counts
and supersedes-path notes — already present per the framework's
inspect path, but not yet on `SyncResult`. Adding it now is one
line. No typed counts; the doctor command (PR G) formats from
`Detail`.

### Tests

Three layers:

**1. Strategy unit tests.** PR A's `strategy_test.go` exercises the
stub interface (asserts each stub returns `errNotImplemented`). It
is **replaced** in PR B by two per-strategy test files:

- `jsonkeymerge_test.go` — new file.
- `markdownblock_test.go` — new file.

The WholeFile stub keeps a trimmed `wholefile_test.go` that asserts
the stub still returns `errNotImplemented` (placeholder for PR C).
`strategy_test.go` itself is deleted in PR B.

Each per-strategy file covers a six-case matrix across temp-dir
fixtures: Missing, Synced, Stale, sentinel-absent (drifted or
first-write depending on strategy), sentinel-corrupted, v=1-on-disk
(MarkdownBlock only). Each case asserts both `Inspect` state and
`Sync` action + byte output. Permission-preservation case (S1
above) appears in both per-strategy files.

**2. Behaviour-parity golden tests.** `testdata/golden/` directory
with subdirectories representing starting on-disk states +
`ProjectParams` JSON + post-sync expected file bytes.

The expected bytes are captured **once**, before `mcpconfigs/` and
`pointers/` are deleted, by running the old packages against the
starting states and snapshotting the output (with the marker
rewrite from v=1 to v=2 applied to the snapshot for the markdown
files — that's the sole expected diff). A throwaway capture
helper at `internal/config/managedfiles/internal/captureimpl/main.go`
generates the goldens; it imports the old packages, writes the
fixtures, and is deleted in the same commit that deletes
`mcpconfigs/`/`pointers/`. The captured byte files are committed
under `testdata/golden/` as **immutable fixtures** — they're not
regenerable from `main` after the cleanup commit.

If a future change requires regenerating goldens (e.g., a
canonicalization tweak), the regeneration path is: `git checkout`
the PR-B-pre-cleanup commit, re-run the capture helper, hand-merge
the new bytes back. Documented in `testdata/golden/README.md` along
with the original capture date and `mcpconfigs`/`pointers` commit
SHAs. (Alternative considered: preserve `mcpconfigs.canonicalize`
and the patch builders as private `managedfiles/` helpers under the
same vestigial-pattern as the v=1 markdown renderers. Rejected as
YAGNI — JSON canonicalization is a one-line `json.MarshalIndent`
that anyone can rewrite, unlike the markdown-block rendering which
has subtle whitespace invariants. The v=1 markdown helpers are
preserved; the JSON helpers are not.)

The test then runs `managedfiles.SyncAll` against a fresh copy of
the starting state and asserts byte-identical output. This is what
proves the migration produces no surprises for the dogfood repo.

**3. Synthetic v=1 → v=2 upgrade integration test.**

Premise correction: spot-checking the repo at PR-B-start, the
dogfood `AGENTS.md` (`/AGENTS.md:1-5`) carries no
`specgraph:init:start` markers (user-authored content only), and
`.cursor/rules/specgraph-bootstrap.md` does not exist (only a stale
`specgraph-bootstrap.md.lock` file remains). No managed file in
this checkout is in the v=1 state. The vestigial v=1 renderer is
defensive scaffolding for a state no observed file occupies.

The integration test therefore uses **synthetic fixtures**, not
"this repo's current state":

- A fixture `AGENTS.md` constructed by calling the old
  `pointers.renderAgentsBlock` (preserved as the private
  `renderV1AgentsBlockBody` helper) and writing it with v=1
  markers and a representative `ProjectParams` (slug `dogfood`,
  serverURL `http://localhost:9090`).
- A fixture `.cursor/rules/specgraph-bootstrap.md` constructed
  analogously with frontmatter + v=1 body.

The test runs `SyncAll`. Asserts:

- `AGENTS.md` body content unchanged at the byte level (only marker
  shape upgraded to v=2 with the new hash sentinel).
- `.cursor/rules/specgraph-bootstrap.md` deleted by the supersedes
  guard.
- `.cursor/rules/specgraph-bootstrap.mdc` created with frontmatter +
  v=2 markers + matching hash.
- JSON files (synthetic Missing → first-init case) round-trip
  through canonicalization to the expected bytes.

A second case asserts the **drifted** path: hand-edit the v=1
block body in the fixture before running `SyncAll`. Expect
`ActionSkipped` on the cursor `.mdc` write (since the new file
*doesn't exist*, this is actually Missing-with-drifted-supersedes;
strategy creates `.mdc` then supersedes-guard refuses to delete
the user-edited `.md`). Spec a third case for the AGENTS.md drift
path: hand-edit body, expect `AGENTS.md` Sync returns
`ActionSkipped` without `--force`.

**Re-establishing dogfood as a v=2 state.** After PR B lands, a
fresh `specgraph init` against this repo writes `AGENTS.md` (appends
a v=2 block to the user-authored file), creates `.mdc` (no `.md` to
supersede), and writes the three JSON files. The first re-init is
the first time these files exist in v=2 form in the dogfood
checkout. Any future PR-B-style migration test against v=1 state
will need synthetic fixtures, not git-checked-in dogfood files.

**Build tag.** The migration integration test is a **regular unit
test** (no `//go:build integration` tag) so `task check` (and the
pre-push hook) catches dogfood-discipline rot. It is **not** in the
class of tests that require Docker (`internal/storage/postgres/`'s
testcontainer suite uses the integration tag). Synthetic v=1
fixtures are constructed in-memory via `t.TempDir()` and the
preserved `renderV1*` helpers — no external state, no Docker, no
network — so the test runs in well under a second and fits the
unit-test budget.

**4. Primitives ports.** Symlink rejection, atomic write, file
locking already covered by PR A's `*_test.go`. No new tests needed
for those; the strategies exercise them through normal paths.

### Project-convention compliance

PR B introduces ~10 new `.go` files plus a `captureimpl/main.go` and
deletes ~10 `.go` files. To pass `task check` (pre-push hook
enforces; see CLAUDE.md §"Quality Gates"):

- **License headers.** Every new `.go` file gets a two-line
  `SPDX-License-Identifier: Apache-2.0` + `Copyright 2026 Sean Brandt`
  prologue, matching PR A. `task license:add` fixes omissions.
- **Package comments.** Any new sub-package (e.g.,
  `internal/config/managedfiles/internal/captureimpl/`) needs a
  `// Package captureimpl ...` doc comment on its first file or
  `revive` fails.
- **Conventional commits.** Both PR B commits (the goldens-capture
  commit and the cleanup commit) use the conventional-commits
  prefix `feat(managedfiles): ...` or `chore(managedfiles): ...` so
  `cog` passes. The commit body references `spgr-rwrp` and the
  parent design doc.
- **DCO sign-off.** Both commits carry `Signed-off-by: Sean Brandt
  <SeBrandt@geico.com>` trailer. Use `jj describe` / `jj commit`
  with the trailer set in the commit message (this repo is
  jj-colocated; the CLAUDE.md workflow uses jj for all commit
  operations).

### Cleanup

The capture helper at `internal/config/managedfiles/internal/captureimpl/main.go`
imports `mcpconfigs/` and `pointers/`; deleting those packages
without deleting the helper breaks the build. The cleanup is
therefore a **single atomic commit**, not a sequence of staged
deletions:

1. Capture-helper run produces `testdata/golden/` fixtures
   (separate commit that lands the goldens).
2. One cleanup commit deletes, together: `internal/config/mcpconfigs/`,
   `internal/config/pointers/`, and `internal/config/managedfiles/internal/captureimpl/`.
3. `go build ./... && task check` clean.

`errNotImplemented` stays — `wholeFileStrategy` still returns it
until PR C lands the third implementation. See §"`wholeFileStrategy`
(still stub in PR B)" above.

### Capture-helper invocation

The capture helper is a `main` binary, not a `_test.go` fixture,
because it needs to import `mcpconfigs`/`pointers` (deleted from
the test tree at the end of the PR). It runs once via `go run
./internal/config/managedfiles/internal/captureimpl` against the
checked-in starting fixtures under `testdata/golden/<case>/in/`
and writes expected output to `testdata/golden/<case>/out/`. A
`task capture-goldens` target wraps the invocation for one-shot
reproducibility before the cleanup commit.

**Lifecycle.** Three artefacts ship together in the goldens-capture
commit: the captured `testdata/golden/` bytes, the
`captureimpl/main.go` helper source, and the `task capture-goldens`
target. **All three are deleted in the cleanup commit** —
`captureimpl/` because it imports the deleted packages, and `task
capture-goldens` because it would otherwise dangle as a broken
alias. The captured bytes stay forever as immutable fixtures.

**Regeneration recipe** (in `testdata/golden/README.md`): "Check
out the PR-B-pre-cleanup commit SHA. Run `go run
./internal/config/managedfiles/internal/captureimpl`. Hand-merge
any new fixtures back to current `main`." The recipe does **not**
reference `task capture-goldens` — that target no longer exists.

Test ports: each existing `mcpconfigs/*_test.go` and `pointers/*_test.go`
case has a destination in `managedfiles/`. Ones that exercise the
framework primitives (locking, symlink, atomic-write) are already
covered by PR A's tests — drop the duplicates. Ones that exercise
business behaviour (marker validation, legacy-block purging,
canonicalization, opencode.jsonc refusal) port to the new tests with
assertions translated to `FileState` + `SyncResult` types.

## Discrepancies with parent design doc

Two items where PR B's reality diverges from the parent design's
prose. Recorded here so they're addressed in a doc-sync pass on the
parent rather than silently absorbed.

1. **`ManagedFile.Source` type.** Parent §"Architecture" shows
   `Source embed.FS`. PR A landed `Source string` with a
   package-level `var canonicalSources embed.FS` in
   `source_release.go`. PR B follows the code, not the design doc
   prose. Doc-sync pass on the parent should align.
2. **`opencode.json` managed keys.** Parent §"Per-harness manifest"
   omits `$schema`; today's code includes it. See in-scope table
   note above. PR B preserves today's behaviour; parent doc to be
   reconciled.

## Risks

- **Risk:** Golden snapshots silently diverge from today's output
  because of an undocumented behaviour we copy incorrectly.
  **Mitigation:** Capture the goldens by running today's code against
  the fixtures, not by hand-writing them. The capture step is part
  of the PR.
- **Risk:** The v=1 defensive hash check returns `Drifted` for the
  dogfood repo because of a whitespace/encoding subtlety in the v=1
  body renderer (CRLF, trailing newlines).
  **Mitigation:** The vestigial v=1 helper is a verbatim port of
  the existing renderer — same Go string-builder code, same
  newline behaviour. The migration integration test asserts the
  dogfood-shape fixture upgrades cleanly to v=2.
- **Risk:** `SyncResult.Detail` becomes a dumping ground for
  unstructured strings that PR G has to parse.
  **Mitigation:** PR G builds doctor output from `FileState`/`SyncResult`
  fields, not `Detail`. `Detail` is for human display only. If PR G
  needs structured purge counts, it adds typed fields then — YAGNI now.
- **Risk:** Build closures violate purity (e.g., a future contributor
  adds `time.Now()` to a marker), causing Inspect/Sync to disagree on
  state and producing spurious `ActionRefreshed`.
  **Mitigation:** `TestManifestShape` invokes each `Build` twice with
  the same `ProjectParams` and asserts byte-identical output.
  Documented invariant on the struct doc.
- **Risk:** The v=1 vestigial renderer is dead code with no observed
  v=1 file in any known checkout, leading future maintainers to
  delete it prematurely.
  **Mitigation:** Sunset trigger in the parent design doc
  (`task plugin:check` reports zero v=1 files for two consecutive
  releases) — already accounts for this. PR B's job is to make sure
  it works *if invoked*; integration tests cover synthetic v=1
  fixtures.

## What's NOT in this PR

Already enumerated in §"Scope / Out of scope" above. The five files
chosen for PR B are precisely those already managed today. Any
unmanaged file is a later PR per the parent design doc's sequencing.

## References

- Parent design: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
- PR A merge commit: `5a10ce5` (foundation framework + Claude API
  verification)
- Packages being deleted: `internal/config/mcpconfigs/`,
  `internal/config/pointers/`
- RFC 7396 JSON Merge Patch (already a project dep):
  `github.com/evanphx/json-patch/v5`

## Revision history

- **2026-05-11 v1**: initial design from brainstorming session.
- **2026-05-11 v2**: first adversarial review applied. 11 findings
  (C1: MarkdownBlock hash scope; C2: errNotImplemented removal
  timing; C3: validateInitMarkers v=2 acceptance; S1: supersedes
  trigger table; S2: $schema discrepancy; S3: API signature change;
  S4: golden regeneration; S5: Source-xor-Build invariant; M1: doc
  drift; M2: frontmatter exact bytes; M3: ActionCreated commitment).
- **2026-05-11 v3**: second adversarial review applied. 5 findings:
  - **C1' (validateInitMarkers canonical-start lookup)**: existing
    function uses `bytes.Index(initStart)` with v=1 literal; for a
    v=2-only file this returns -1 and Rule 4's overlap exception
    can't fire. Ported version uses `initStartAnyVersion` regex.
  - **C2' (Inspect lock policy)**: new §"Concurrency: Inspect vs
    Sync" makes the no-lock-on-Inspect invariant explicit, relies
    on POSIX rename atomicity.
  - **S1' (Build closure purity)**: new §"Build closure purity"
    declares Build must be pure; `TestManifestShape` asserts.
  - **S2' (ActionUpdated→Refreshed mapping)**: capture-rewrite rule
    extended to cover both the no-markers-append case
    (`ActionCreated`) and the in-place-block-replace case
    (`ActionRefreshed`).
  - **S3' (JSONKeyMerge Stale near-permanent)**: new subsection
    documents that `Stale` is expected to be the steady state for
    any user who edits opencode.json's whitespace; not corruption.
  - **M1' (dogfood migration fixture premise broken)**: this
    repo's `AGENTS.md` has no v=1 markers and
    `.cursor/rules/specgraph-bootstrap.md` does not exist.
    Integration test rewritten to use synthetic v=1 fixtures
    constructed via the preserved `renderV1*` helpers.
- **2026-05-11 v4**: third adversarial review applied. 4 findings:
  - **C (cleanup ordering + table malformation)**: the captureimpl
    helper imports `mcpconfigs`/`pointers`; staged deletion as
    written breaks the build mid-cleanup. Reframed as a single
    atomic cleanup commit. Also fixed: the `$schema` note paragraph
    was splitting the in-scope table; moved below the table.
  - **S (purity test fixture)**: `TestManifestShape` purity assertion
    needs a valid `ProjectParams` (Validate rejects empty values).
    Pinned a fixed test constant `{Slug: "test", ServerURL:
    "http://localhost:9090"}`. Build panic semantics declared:
    panic = programmer error, no `recover()`, abort the CLI.
  - **S (capture-helper invocation mechanism)**: new subsection
    pins it as a `main` binary invoked via `go run` (not a test),
    with a `task capture-goldens` wrapper, deleted in the cleanup
    commit.
  - **M (`ActionName`/`CountErrors` ownership)**: moved from
    cmd/specgraph-private to exported `managedfiles` helpers so
    PR G's `doctor` shares the same names. `hasErrors` was redundant
    with `CountErrors > 0`; dropped.
- **2026-05-11 v5**: fourth adversarial review applied. 6 findings:
  - **C (failedPaths regression in init)**: the v3 init snippet
    replaced today's `init.go:125-147` `failedPaths` list + joined
    error message with a bare count. Restored the list-of-paths
    error message to match today's diagnostic surface.
  - **S (atomic-write mode preservation as cross-strategy
    obligation)**: new §"Atomic write: mode preservation"; JSON
    strategy spec mentioned it, MarkdownBlock pseudocode didn't.
    Pinned as a contract both strategies must satisfy, with a
    permission-preservation test case in both per-strategy test
    files.
  - **S (strategy-interface signature cascade)**: the
    `strategy.Sync(cwd, mf, opts)` interface method also takes
    `params` because strategies invoke `Build(params)` to compute
    canonicals. Enumerated all four affected call sites
    (interface, stubs, package-level Inspect/Sync, InspectAll/SyncAll).
  - **S (helper-port list incomplete)**: §"Vestigial v=1 renderer"
    listed only two helpers; the production write path also needs
    `splitFrontmatter`, `ErrFrontmatterMissing`,
    `defaultCursorFrontmatter`, `safeSlugPattern`,
    `purgeLegacyBlocks` + `legacyBlock` regex,
    `validateInitMarkers` + marker regexes, `canonicalize` from
    `mcpconfigs`. Section renamed §"Helpers ported from
    `pointers/` and `mcpconfigs/`" with live-path vs. vestigial
    distinction and a per-helper sunset scope.
  - **M (test-layout reconciliation with PR A's strategy_test.go)**:
    PR A's existing `strategy_test.go` is deleted; replaced by
    `jsonkeymerge_test.go`, `markdownblock_test.go`, and a trimmed
    `wholefile_test.go` for the still-stubbed third strategy.
  - **M (`extractManagedBlockBody` version + edge-case
    semantics)**: helper now explicitly recognises both v=1 and v=2
    markers (so it serves both the defensive recompute path and
    the v=2 hash check), and empty-body case clarified (non-nil
    empty slice, not "no block").
- **2026-05-11 v6**: fifth adversarial review applied. 10 findings:
  - **C (init-snippet replacement scope underspecified)**: the
    snippet replaced only init.go:96-153; readers could delete the
    surrounding `WriteProject`, `loadGlobalCfg`/`ResolveServer`,
    and "Initialized project" message blocks. §"Init wiring" now
    leads with an explicit "Scope of replacement" listing the
    preserved-verbatim regions and the ordering invariant
    (`WriteProject` runs before `SyncAll`).
  - **C (byte-identical claim self-contradictory)**: v=2 markers
    add `sha256=<hash>` (≈71 bytes per block); a literal `diff`
    would always show changes. §Goal now precisifies: bytes
    *between markers* identical, bytes *outside markers*
    identical, marker lines themselves replaced wholesale.
  - **S (license headers on new files)**: new §"Project-convention
    compliance" enumerates SPDX header, package comment,
    conventional-commits prefix, and DCO sign-off requirements
    derived from CLAUDE.md.
  - **S (DCO + conventional-commits)**: same section pins commit
    format for both PR B commits (goldens-capture + cleanup).
  - **S (migration-test build-tag undecided)**: pinned as a
    **regular unit test** (no `//go:build integration`) so `task
    check` and the pre-push hook enforce dogfood-discipline rot
    detection. Synthetic fixtures fit the unit-test budget.
  - **S (`--force` / `--keep-edits` plumbing)**: explicitly added to
    "Out of scope" — `SyncOptions{}` zero-value flows through PR
    B's init; user-facing overrides land with PR G's `doctor --fix`.
  - **M (`task plugin:check` / `--dry-run`)**: explicitly out of
    scope; deferred to PR G alongside doctor.
  - **M (goroutine safety)**: new subsection states strategy
    methods are reentrant and share no mutable state, locking the
    contract for PR G's potential parallelisation.
  - **M (`$schema` posture: managed-value not managed-presence)**:
    explicit note that RFC 7396 merge overwrites `$schema` every
    init, distinguishing it from PR E's future
    `enabledPlugins[...]` managed-presence exception.
  - **M (synthetic-fixture in-memory construction)**: covered by
    the build-tag fix — `t.TempDir()` + `renderV1*` helpers, no
    Docker, no network.
- **2026-05-11 v7**: sixth adversarial review applied. No criticals
  this pass — diminishing returns reached on design correctness;
  remaining work is cross-section consistency.
  - **S (`purgeLegacyBlocks` contradicts byte-identical claim)**:
    v6 said "bytes outside markers unchanged," but
    `purgeLegacyBlocks` deletes pre-init `<slug>:start/end` blocks
    from AGENTS.md — bytes outside the init markers. §Goal now
    classifies three regions (marker lines, between-markers,
    outside-markers-minus-legacy-blocks) with separate equivalence
    claims.
  - **S (`task capture-goldens` outlives the helper)**: §"Capture-
    helper invocation" now explicitly deletes the task target
    alongside the helper in the cleanup commit; regeneration recipe
    in README.md uses `go run` directly, not the task target.
  - **M (`Source` field's PR-B fate)**: explicit note that `Source`,
    `readSource`, embed FS, and `dev` build tag stay as scaffolding
    PR C activates; delete-and-readd cycle rejected.
  - **M (`SyncResult.Detail` exact strings)**: new §"`SyncResult.Detail`
    string grammar" table pins the exact format for purge counts,
    supersedes-Drifted, and `--force --keep-edits` cases; fixture
    file documents the grammar.
  - **M (sunset trigger fires immediately)**: parent design's
    trigger assumed v=1 files existed in user deployments; with
    zero v=1 files anywhere at PR-B-merge, a literal reading
    deletes the helpers in PR C. Added a second clause: deletion
    requires AND-ed with "6 months elapsed since v=2 rollout."
