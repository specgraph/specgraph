# PR C — OpenCode plugin under `.specgraph/agents/opencode/`

- **Bead:** `spgr-zqpb` (child of `spgr-rwrp`)
- **Predecessor:** PR B (`ec85127`) — `managedfiles` framework with real `jsonKeyMergeStrategy` + `markdownBlockStrategy`; `wholeFileStrategy` still stubbed
- **Parent design:** `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md` §"PR C"
- **Date:** 2026-05-11

## Goal

`specgraph init` writes the OpenCode plugin TypeScript file directly to
`.specgraph/agents/opencode/specgraph.ts` from an embedded canonical
source, and ensures `opencode.json`'s `plugin` array references that
path. This closes the harness-parity gap from `spgr-cceg`: OpenCode's
prime + post-stage delivery now ships through `init`, not a README step.

## Scope

### In scope

| Item | Detail |
|---|---|
| Activate `wholeFileStrategy` | Replace PR A's `errNotImplemented` stub with real `Inspect` + `Sync`. Whole-file sentinel: `// specgraph:init v=2 sha256=... rev=...` on line 1. |
| Embed sources | `//go:embed plugin/opencode/.opencode/plugins/specgraph.ts` via `internal/config/managedfiles/source_release.go`'s existing `canonicalSources` FS. |
| Manifest entry | `.specgraph/agents/opencode/specgraph.ts`, `StrategyWholeFile`, `CommentSlash`, `HarnessOpenCode`, `Source` field pointing at the embed path. |
| `opencode.json` `plugin` array union-merge | `jsonKeyMergeStrategy.Sync` gains a path-keyed post-merge step that union-merges the `plugin` array for `opencode.json` only. Adds `./.specgraph/agents/opencode/specgraph.ts` if missing; preserves user-added entries; idempotent on second run. |
| `opencode.json` Build closure | Adds the `plugin` array key to the patch with our path. RFC 7396 still replaces the array on merge — the post-merge step is what produces union semantics. |
| Dogfood `.gitignore` | Add `.specgraph/agents/` so init-written content doesn't get committed. |
| Dogfood `opencode.json` cleanup | Remove the legacy `./plugin/opencode/.opencode/plugins/specgraph.ts` entry from the checked-in `plugin` array. The new entry lands on next `specgraph init` run via union-merge. |
| Remove `errNotImplemented` | After this PR all three strategies are implemented; the sentinel from `errors.go` and the stub references go away. `wholefile_test.go`'s stub-assertion is replaced by the real test matrix. |
| SMOKE_TEST re-run | `plugin/opencode/SMOKE_TEST.md`: walk through the OpenCode harness using the new managed path. Document any deltas in the smoke test file. |

### Out of scope

- Other PR C+ children (PR D Cursor, PR E Claude, PR F Skills, PR G doctor).
- `harnesses:` field in `.specgraph.yaml` (still hard-coded to all three in `cmd/specgraph/init.go`).
- `--force` / `--keep-edits` CLI flags on `init` (deferred to PR G).

## Architecture

### `wholeFileStrategy` implementation

`Inspect`:

1. `rejectSymlinkComponents` for `mf.Path`.
2. `readFileNoFollow(full)`. If `fs.ErrNotExist` → `StateMissing`.
3. `readSource(mf)` to obtain the embedded canonical bytes.
4. Parse line 1 as a sentinel via `ParseSentinel(CommentSlash, line)`.
   - No sentinel (line 0 isn't a managed file marker) → `StateDrifted` with `Detail: "no sentinel"`.
   - Sentinel `v=2` → compare `sentinel.SHA256` against
     `HashExcludingSentinel(CommentSlash, disk)`.
     - Mismatch → `StateDrifted` (user edited).
     - Match → compare against `HashExcludingSentinel(CommentSlash, canonical)`.
       - Match → `StateSynced`. Mismatch → `StateStale`.
   - Sentinel `v=1` (theoretically possible for forward compat) →
     defensive recompute analogous to `markdownBlockStrategy`'s v=1
     branch. Not exercised in PR C (no v=1 whole-file deployments
     exist); rely on `ParseSentinel`'s version-acceptance.
   - Sentinel corrupt (unknown version, parse error) → propagate
     `ErrCorruptedSentinel`.

`Sync`:

1. Acquire per-file lock via `acquireFileLock`.
2. Classify via the same logic as `Inspect`.
3. Dispatch on `State`:
   - `Missing` → write `renderWholeFile(canonical, hash)` →
     `ActionCreated`.
   - `Synced` → `ActionNoOp`.
   - `Stale` → rewrite canonical+fresh sentinel → `ActionRefreshed`.
   - `Drifted` → skip with `ActionSkipped` (matches markdown drifted
     behaviour with zero-value `SyncOptions`). `--force`: rewrite.
     `--force --keep-edits`: refresh sentinel to match disk hash;
     preserve user content. (Same `SyncOptions` plumbing as PR B; init
     passes zero-value, so PR C exercises only the no-force path.)
4. `atomicWrite` with mode preservation (`preserveMode(full)`,
   defaults to `0o600`).

`renderWholeFile(canonical []byte, sentinel Sentinel) []byte`:

```text
// specgraph:init v=2 sha256=<hash> rev=<short-sha>
<canonical content>
```

The `rev` field is optional (forensic only per parent design §"Drift
detection / Sentinel format"). PR C populates it from the build-time
`gen.RevisionShortSHA()` if available; empty otherwise.

`HashExcludingSentinel(CommentSlash, content)` already drops the
first-line sentinel (per PR A's `hash.go:40-44`). The canonical bytes
fed to `renderWholeFile` include no sentinel; the hash is computed
over those bytes verbatim.

### Manifest entry

```go
{
    Path:     ".specgraph/agents/opencode/specgraph.ts",
    Strategy: StrategyWholeFile,
    Source:   "plugin/opencode/.opencode/plugins/specgraph.ts",
    Comment:  CommentSlash,
    Harness:  HarnessOpenCode,
}
```

`Build` is nil for this entry (Source-xor-Build invariant from PR B
manifest validation: `WholeFile` requires `Source`).

`source_release.go` gets its first `//go:embed` directive:

```go
//go:embed plugin/opencode/.opencode/plugins/specgraph.ts
var canonicalSources embed.FS
```

The path is relative to the package directory (`internal/config/managedfiles/`),
so the actual disk path is
`internal/config/managedfiles/plugin/opencode/.opencode/plugins/specgraph.ts`.
**That path doesn't exist.** Two options:

| | |
|---|---|
| (a) Symlink | `internal/config/managedfiles/plugin -> ../../../plugin` |
| (b) Move embed scope | Make the embed directive use a path within `managedfiles/`, e.g. copy the source into `internal/config/managedfiles/embedded/opencode/specgraph.ts` |
| (c) Use a build-tag indirection | The `dev` build tag (PR A) reads from disk at `SPECGRAPH_DEV_SOURCE_ROOT`. Use the same primitive for release builds: make the canonical source path explicit. |

**My pick: (a) symlink** — minimal disruption, matches the pattern PR D
and PR E will use (they also need to embed `plugin/cursor/` and
`plugin/claude/` trees). The symlink lives at
`internal/config/managedfiles/plugin -> ../../../plugin` so all three
harnesses' canonical sources are reachable under one embed root via
`plugin/<harness>/...`.

Alternative if symlink turns out to be fragile cross-platform: (b) with
a `go:generate` task that mirrors source files into
`internal/config/managedfiles/embedded/`. Defer the decision to
implementation time; symlink first.

### `opencode.json` Build closure + plugin array union

`buildOpenCodeJSON(p ProjectParams) ([]byte, error)` in `manifest.go`
gains the `plugin` key:

```go
"plugin": []any{"./.specgraph/agents/opencode/specgraph.ts"},
```

RFC 7396 MergePatch then replaces any existing `plugin` array with
just our path. To preserve user-added entries, `jsonKeyMergeStrategy`
gains a post-merge hook keyed on `mf.Path`:

```go
// After canonicalize, if mf.Path matches a known union-array entry,
// re-union the array with the existing-on-disk one.
if mf.Path == "opencode.json" {
    canonical, err = unionPluginArray(existing, canonical)
    if err != nil { ... }
}
```

`unionPluginArray(existing, canonical []byte) []byte`:

1. Parse both JSON documents.
2. Extract `plugin` arrays from each.
3. Union: take the canonical's `plugin` array, append any existing
   entries not already present (string-equality match).
4. Stable ordering: canonical entries first, existing-only entries
   appended in their original order.
5. Re-marshal canonicalized.

Tests:

- Missing → write canonical with `plugin: ["./.specgraph/..."]`.
- Existing with our path only → no-op.
- Existing with our path + user entry → union, our path first.
- Existing with user entry only → union, our path prepended.
- Existing with `plugin` field absent → set to `[our-path]`.

Special case: the existing dogfood `opencode.json` has
`["./plugin/opencode/.opencode/plugins/specgraph.ts"]` — that's a
*different* path, not our managed path. Union-merge correctly
preserves it (user-added entries are paths the framework doesn't
own). PR C also edits the checked-in dogfood `opencode.json` to drop
that stale entry **as a separate commit before the init wiring lands**,
so the eventual init run produces the expected single-entry array.

### Dogfood files

- `.gitignore`: add `.specgraph/agents/` glob. The init-written
  `.specgraph/agents/opencode/specgraph.ts` is not committed.
- `opencode.json`: edit checked-in file to remove
  `./plugin/opencode/.opencode/plugins/specgraph.ts` from the `plugin`
  array. The array becomes empty; next `specgraph init` run adds
  `./.specgraph/agents/opencode/specgraph.ts` via union-merge. Edit
  lands in the same commit as the manifest change so dogfood stays
  consistent.

### `errNotImplemented` removal

After `wholeFileStrategy` is real, `errNotImplemented` from `errors.go`
is unused. Removed in PR C's cleanup commit. `wholefile_test.go`'s stub
assertion is deleted; replaced by the six-case matrix:

- Missing → ActionCreated, file exists with v=2 sentinel
- Synced → ActionNoOp
- Stale → ActionRefreshed
- Drifted (sentinel hash mismatches disk) → ActionSkipped, file
  unchanged
- Sentinel absent → ActionSkipped (Drifted) per `Detail: "no sentinel"`
- Sentinel corrupted (unknown version) → error propagates via
  `SyncResult.Err`

Mode preservation case: pre-chmod the fixture to `0o644`; after sync
the mode is still `0o644`.

### SMOKE_TEST

`plugin/opencode/SMOKE_TEST.md` (created in PR #941) walks the OpenCode
harness end-to-end. PR C re-runs it against the new path:

1. Fresh checkout, fresh `specgraph init`.
2. Verify `.specgraph/agents/opencode/specgraph.ts` exists with v=2
   sentinel.
3. Verify `opencode.json`'s `plugin` array contains
   `./.specgraph/agents/opencode/specgraph.ts`.
4. Open the repo in OpenCode; the plugin loads; prime injection works.
5. Run a tool call to confirm `tool.execute.after` fires the
   post-stage nudge.

Document any procedural changes vs PR #941's original smoke test in
the same file. PR C is the first time the smoke test walks against the
managed-path destination; deltas should be small (just the path
change).

## Tests

- **`wholefile_test.go`**: six-case matrix (Missing, Synced, Stale,
  Drifted, sentinel-absent, sentinel-corrupted) + mode-preservation,
  using `t.TempDir()` and synthetic canonical content (no dependency
  on the OpenCode plugin source).
- **`jsonkeymerge_test.go`**: extend with five `unionPluginArray`
  cases per §"opencode.json Build closure + plugin array union" above.
- **`manifest_test.go`**: assert 6 entries (was 5 in PR B); assert
  the new `.specgraph/agents/opencode/specgraph.ts` entry has the
  expected `Source`, `Strategy`, `Comment`, `Harness`.
- **`golden_test.go`**: regenerate the captured goldens? **No.** PR
  B's goldens captured the deleted `mcpconfigs`/`pointers` output as
  immutable byte references. PR C doesn't reproduce the dogfood from
  those goldens — it introduces a new managed file. PR B's golden
  test continues to cover the 5 PR-B entries; PR C's new entry is
  covered by `wholefile_test.go` + the integration test below.
- **Migration test** (synthetic): seed an existing dogfood-shape
  `opencode.json` with `plugin: ["./old/path"]`, run `SyncAll`, assert
  the array now contains both entries (union-merge) and the .ts file
  exists on disk.

## Risks

- **Symlink in `internal/config/managedfiles/`** — the cross-platform
  story for go-embed-through-symlink is supported on Unix and Windows
  (Go's `//go:embed` follows symlinks since 1.16) but may surprise a
  contributor. Documented in a comment alongside the symlink.
- **`gen/` package version** — `gen.RevisionShortSHA()` may not exist;
  in that case the sentinel's `rev` field is empty. Documented as
  forensic-only per parent design.
- **`unionPluginArray` ordering instability** — choose canonical-first
  ordering and document it. Tests pin the order so a future contributor
  can't silently reorder.

## Discrepancies with parent design

None at this time. PR C's scope matches parent design §"PR C" verbatim.

## References

- Parent design: `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`
- PR B merge commit: `ec85127`
- Bead: `spgr-zqpb`
- OpenCode plugin source: `plugin/opencode/.opencode/plugins/specgraph.ts`
- OpenCode smoke test: `plugin/opencode/SMOKE_TEST.md`

## Revision history

- **2026-05-11 v1**: initial design after brainstorming approval.
