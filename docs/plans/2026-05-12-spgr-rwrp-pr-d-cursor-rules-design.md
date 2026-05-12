# spgr-rwrp PR D — Cursor rule files via embed-and-write

**Date:** 2026-05-12
**Status:** Design (post-brainstorm; pre-implementation)
**Parent design:** [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md)
**Predecessors:** PRs 0/A/B/C — Claude API verification, `internal/config/managedfiles/` foundation, mcpconfigs/pointers port, OpenCode plugin embed-and-write.

## Problem

`specgraph init` already writes Cursor's `.cursor/mcp.json` and the bootstrap pointer (`.cursor/rules/specgraph-bootstrap.mdc`). The two main Cursor rule files — `specgraph.md` (one-screen routing pointer) and `post-stage.md` (analytical-pass guidance) — live only in `plugin/cursor/.cursor/rules/` and never reach end-user projects unless someone copies them by hand. That breaks the harness-parity promise (see the parent epic): a fresh `specgraph init` should produce a complete Cursor integration with no out-of-band file copies.

A secondary gap is the file extension. Cursor's rule format is `.mdc` (markdown with frontmatter and Cursor-specific metadata). The in-repo authoring sources use plain `.md`, relying on convention rather than the actual extension Cursor reads. Some users copying by hand are surely landing the files at `.cursor/rules/specgraph.md`, which Cursor silently ignores.

## Approach

Adopt the embed-and-write pattern PR C established for OpenCode:

1. Move the canonical content into `internal/config/managedfiles/embedded/cursor/`, renamed to `.mdc`.
2. Replace the authoring sources in `plugin/cursor/.cursor/rules/` with reverse-symlinks pointing back into `embedded/cursor/`, so devs editing under `plugin/cursor/` still feel like they're editing the authoring source while the binary embeds the canonical bytes.
3. Add two `WholeFile` entries to the manifest. Both carry `Comment: CommentHTML` and a new `HasFrontmatter: true` flag (see *Framework changes* below). Both carry `SupersedesPath` for hash-guarded cleanup of user-installed `.md` copies.
4. Extend the `WholeFile` strategy to position the sentinel after a leading YAML frontmatter block when `HasFrontmatter` is set. Cursor's mdc parser requires `---` on line 1, so the sentinel cannot sit above frontmatter.
5. Drop the misleading `:start` suffix from `CommentHTML` sentinels rendered in whole-file context. The `:start`/`:end` distinction is meaningful only for `MarkdownBlock`; a lone `<!-- specgraph:init:start ... -->` without a matching `:end` is confusing to read on disk. The parser already accepts both forms (`init(?::start)?` in `sentinel.go:50`), so the change is render-only and back-compatible.

## What changes on disk for end users

After `specgraph init`:

| Path | State |
|---|---|
| `.cursor/rules/specgraph.mdc` | **new** — written by init, drift-tracked by sentinel |
| `.cursor/rules/specgraph-post-stage.mdc` | **new** — written by init, drift-tracked by sentinel |
| `.cursor/rules/specgraph.md` | **deleted** if its content matches the hash of the pre-rename canonical (verbatim copy from `plugin/cursor/`); preserved if user-edited, with `SyncResult.Detail` for the new `.mdc` reading `supersedes path ".cursor/rules/specgraph.md" left in place: prior-canonical mismatch` so users see the retention reason at init time |
| `.cursor/rules/post-stage.md` | **deleted** under the same hash guard; preserved if edited, with the same Detail emission |

End-user file layout on `.mdc` files (frontmatter intact, sentinel between frontmatter and body). Note: the existing `splitFrontmatter` helper (`helpers_md.go:42-44`) consumes the blank line that follows the closing `---` into `front`, so on disk the blank line sits *between* `---` and the sentinel, and the sentinel is immediately followed by the body:

```text
---
description: SpecGraph routing — use when the user mentions specs, ...
alwaysApply: false
---

<!-- specgraph:init v=2 sha256=abc123... -->
# SpecGraph Routing

You have access to the SpecGraph MCP server. Detail lives in
`.cursor/skills/`; this rule is the one-screen pointer.
...
```

**Frontmatter ownership contract.** Frontmatter on the managed `.mdc` files (`description`, `alwaysApply`) is owned by `specgraph init`. This extends the existing init-owned-frontmatter contract already in force for `.cursor/rules/specgraph-bootstrap.mdc` (whose frontmatter comes from `defaultCursorFrontmatter` in `helpers_md.go:20-25`) to two more rule files. User edits to managed frontmatter are detected as drift and overwritten by `--force`. If you need a custom rule shape, copy the file under a different filename — Cursor will load any `.mdc` under `.cursor/rules/`.

## Framework changes

Three small, well-contained changes inside `internal/config/managedfiles/`.

### 1. `HasFrontmatter bool` on `ManagedFile`

Add one field to `types.go`:

```go
type ManagedFile struct {
    Path           string
    Strategy       StrategyKind
    Source         string
    Comment        CommentSyntax
    Harness        Harness
    SupersedesPath string
    HasFrontmatter bool // NEW: WholeFile only — sentinel after `---...---`
    Build          func(ProjectParams) ([]byte, error)
}
```

Default `false` preserves existing behavior. The `init()` manifest invariant check (manifest.go:84-103) grows two rules:

- `HasFrontmatter == true` requires `Strategy == StrategyWholeFile` (panic otherwise).
- `HasFrontmatter == true` requires `Comment != CommentNone` (panic otherwise). Belt-and-suspenders: today no `WholeFile` entry uses `CommentNone`, so this rule has no triggering scenario in the shipped manifest. It exists to prevent a future entry from silently rendering an empty sentinel line (which would write a stray blank line at the top of the body).

### 2. Frontmatter-aware whole-file classify and render

**Reuse the existing `splitFrontmatter` helper.** The function already exists in `helpers_md.go:32`, with signature `(front, body []byte, err error)`, ported from `pointers/cursor.go` in PR B. Its current contract:

- Requires the input to start with `---\n`; otherwise returns `ErrFrontmatterMissing` ("must start with '---'").
- Locates the first closing `\n---\n` and returns `front` as bytes-through-the-closing-`---` (plus the trailing blank line if present), `body` as the remainder.
- Returns `ErrFrontmatterMissing` ("frontmatter not closed before EOF") when an opener has no closer.

PR D reuses this helper unchanged. No new helper, no signature change. The whole-file integration is:

`wholeFileClassify` (when `mf.HasFrontmatter == true`):

1. Call `splitFrontmatter(existing)`.
   - On `ErrFrontmatterMissing`: classify as `StateDrifted` with `Detail: "frontmatter missing or unclosed"`. The user broke the frontmatter shape; init refuses to mutate without `--force`. Matches the existing pattern of refusing-to-mutate on corruption.
   - On success: parse the first line of `body` as a sentinel.
2. Parse the first line of `body` strictly. The sentinel MUST be on `body[0]`. If `body[0]` is blank, contains a non-sentinel comment, or contains anything else, classify as `StateDrifted` with `Detail: "no sentinel"`. (Rationale: a sentinel deeper in the file would let `--force --keep-edits` write a second sentinel above an orphan, accumulating cruft. Strict positioning makes orphan accumulation impossible — the user either sees Drifted and acknowledges, or the file is verbatim and refreshes cleanly.)
3. Hash semantics: hash covers `front` + `body-with-first-line-removed`. The sentinel itself is excluded; everything else (including frontmatter) participates. Any edit to `description`, `alwaysApply`, or the rule body triggers `Drifted`/`Stale` symmetrically.

`renderWholeFile` (when `mf.HasFrontmatter == true`):

1. Call `splitFrontmatter(canonical)`. The embedded source `.mdc` file lives on disk with real frontmatter and no sentinel — this call always succeeds on canonical input. A unit test (see *Tests*) pins this invariant.
2. Emit bytes: `front + sentinelLine + "\n" + body`. Because the existing `splitFrontmatter` (`helpers_md.go:42-44`) consumes the blank line after the closing `---` into `front`, the output is `---\n...\n---\n\n<sentinel>\n<body>` — blank line before the sentinel, body immediately after. The user-facing example earlier in this doc reflects that exact layout.

**`HashExcludingSentinel` signature — sibling function, not a mutated existing one.** Add:

```go
// HashExcludingSentinelAfterFrontmatter splits leading YAML frontmatter
// off content, strips the sentinel on the first line of the
// post-frontmatter body, and hashes (front + remaining-body) together.
// Returns ErrFrontmatterMissing if the content has no valid frontmatter.
//
// Existing HashExcludingSentinel is unchanged. All current call sites
// (wholefile.go, markdownblock.go, supersedes.go, tests) keep their
// current behavior. Only the new HasFrontmatter wholefile path calls
// the new function.
func HashExcludingSentinelAfterFrontmatter(syntax CommentSyntax, content []byte) (string, error)
```

Sibling-function form keeps existing hashing semantics byte-identical for every shipped path. Only the new opt-in path goes through the new function.

### 3. `CommentHTML` whole-file sentinel form drops `:start`

`RenderSentinel(CommentHTML, ...)` today returns `<!-- specgraph:init:start v=2 ... -->`. The `:start` suffix is meaningful only for the `MarkdownBlock` strategy, which writes a matching `<!-- specgraph:init:end -->`. For a whole-file sentinel — a single line with no pair — `:start` is misleading.

**Scope of the change is minimal.** A repo-wide grep confirms two facts:

- `markdownblock.go` does NOT call `RenderSentinel` — it writes the `:start` marker inline via string concatenation at `markdownblock.go:319` (v=2) and `markdownblock.go:380` (v=1). The block strategy is unaffected.
- The only non-test caller of `RenderSentinel` is `wholefile.go:132`. There are zero out-of-package callers (`grep -r RenderSentinel` is in-package only).

Given those facts, the cleanest change is to update `RenderSentinel`'s `CommentHTML` branch in place to emit the bare form:

```go
// Before (sentinel.go:79-83):
case CommentHTML:
    return "<!-- " + strings.Replace(body, "specgraph:init", "specgraph:init:start", 1) + " -->"

// After:
case CommentHTML:
    return "<!-- " + body + " -->"
```

No function split, no rename, no new export. Three test assertions in `sentinel_test.go` (lines 28, 105, 111-ish) update to expect the bare form. The parser (`sentinel.go:50`) already accepts both forms (`init(?::start)?`), so any pre-existing CommentHTML output written by older binaries — there is none in the wild, see *Back-compat anchor* below — would still round-trip.

## Manifest entries

```go
{
    Path:           ".cursor/rules/specgraph.mdc",
    Strategy:       StrategyWholeFile,
    Source:         "embedded/cursor/specgraph.mdc",
    Comment:        CommentHTML,
    Harness:        HarnessCursor,
    HasFrontmatter: true,
    SupersedesPath: ".cursor/rules/specgraph.md",
},
{
    Path:           ".cursor/rules/specgraph-post-stage.mdc",
    Strategy:       StrategyWholeFile,
    Source:         "embedded/cursor/specgraph-post-stage.mdc",
    Comment:        CommentHTML,
    Harness:        HarnessCursor,
    HasFrontmatter: true,
    SupersedesPath: ".cursor/rules/post-stage.md",
},
```

The manifest grows from 6 to 8 entries.

## SupersedesPath: hash guard against user-edited copies

The pre-rename `.md` content was never written by `specgraph init` — users either copied verbatim from the repo or edited their copy. Verbatim copies are safe to delete; edited copies must be preserved.

`supersedesGuardedDelete` requires the caller to provide `expectedPriorHash`. For PR D, that hash is `HashExcludingSentinel(CommentNone, <pre-PR-D canonical bytes>)`. The pre-rename `.md` files contained YAML frontmatter byte-for-byte (verify against the current `plugin/cursor/.cursor/rules/specgraph.md` and `post-stage.md` at PR D commit time); any line-ending normalization, BOM injection, or trailing-newline drift during copy-into-vestigial breaks the guard silently. The build-time invariant test below pins this.

To make those bytes available after the rename, preserve them as embedded static content:

- Add `internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md` and `.../vestigial/post-stage.md` (byte-for-byte copies of the pre-rename canonical).
- Embed them via `//go:embed`:

  ```go
  //go:embed embedded/cursor/vestigial/specgraph.md
  var vestigialCursorSpecgraphMD []byte

  //go:embed embedded/cursor/vestigial/post-stage.md
  var vestigialCursorPostStageMD []byte
  ```

- Add `vestigialCursorRulePriorHash(supersededPath string) string` that maps the SupersedesPath value (`".cursor/rules/specgraph.md"` or `".cursor/rules/post-stage.md"`) to the corresponding `HashExcludingSentinel(CommentNone, bytes)`. Panic on an unknown SupersedesPath (mirrors `computePriorCanonical` in `markdownblock.go:372-375`).
- The whole-file Sync path (see *Where supersedes is invoked* below) computes the prior hash via this helper before calling `supersedesGuardedDelete`.

**Why a parallel helper instead of extending `computePriorCanonical`.** The markdown-block path's `computePriorCanonical` (`markdownblock.go:368-385`) renders prior canonical from `ProjectParams` at runtime — the v=1 cursor bootstrap body interpolates the server URL and project slug. PR D's prior-canonical inputs are *parameter-free static bytes* (the pre-rename `.md` files contained no project-specific substitutions). A unified function would need an awkward dual-mode signature (params-or-bytes). Keeping the helpers separate matches the inputs cleanly: `computePriorCanonical(mf, params)` for renderer-based prior canonicals; `vestigialCursorRulePriorHash(supersededPath)` for static-bytes-based ones. Same sunset trigger applies to both.

**Hash-pinning test.** Add `TestVestigialCursorRulePriorHashPinned`: asserts the literal hex `HashExcludingSentinel(CommentNone, vestigialCursorSpecgraphMD)` and the same for `vestigialCursorPostStageMD` against two hex constants checked into the test file. Any edit to the vestigial files fails CI immediately with a clear message pointing at this test ("If you intentionally changed pre-rename canonical bytes, update both hex constants — but note this breaks SupersedesPath cleanup for any user with the old verbatim bytes on disk").

Sunset trigger: same as `renderV1CursorBlockBody` — zero superseded files in the dogfood repo for two consecutive releases. Tracked in a follow-up bead at PR D close.

`supersedesGuardedDelete` itself is unchanged.

## Where supersedes is invoked

The markdown-block strategy calls `supersedesGuardedDelete` inside its `Sync` at `markdownblock.go:163-176`. The whole-file strategy currently has no such call site — PR D adds one to `wholefile.go`'s `Sync`, mirroring the markdownblock pattern:

```go
// At the end of wholeFileStrategy.Sync, after the state-switch returns
// a SyncResult `res` (one of ActionNoOp, ActionCreated, ActionRefreshed,
// ActionForced):
if mf.SupersedesPath != "" && (res.Action == ActionCreated || res.Action == ActionRefreshed || res.Action == ActionForced || res.Action == ActionNoOp) {
    priorHash := vestigialCursorRulePriorHash(mf.SupersedesPath)
    if err := supersedesGuardedDelete(cwd, mf.SupersedesPath, priorHash); err != nil {
        if errors.Is(err, ErrPriorCanonicalMismatch) {
            if res.Detail != "" {
                res.Detail += "; "
            }
            res.Detail += fmt.Sprintf("supersedes path %q left in place: prior-canonical mismatch", mf.SupersedesPath)
        } else {
            return SyncResult{Path: mf.Path, Action: ActionError, Err: err}, nil
        }
    }
}
return res, nil
```

The exact Detail string matches the markdown-block strategy's, so doctor (PR G) can use one regex to find both flavors of orphan.

## Filesystem moves in this PR

1. Move authoring content (rename + relocate):
   - `plugin/cursor/.cursor/rules/specgraph.md` → `internal/config/managedfiles/embedded/cursor/specgraph.mdc`
   - `plugin/cursor/.cursor/rules/post-stage.md` → `internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc`
2. Preserve pre-rename bytes as vestigial:
   - Copy original `specgraph.md` bytes into `internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md`
   - Copy original `post-stage.md` bytes into `internal/config/managedfiles/embedded/cursor/vestigial/post-stage.md`
3. Reverse-symlinks under `plugin/cursor/`:
   - `plugin/cursor/.cursor/rules/specgraph.mdc` → `../../../../internal/config/managedfiles/embedded/cursor/specgraph.mdc`
   - `plugin/cursor/.cursor/rules/specgraph-post-stage.mdc` → `../../../../internal/config/managedfiles/embedded/cursor/specgraph-post-stage.mdc`
4. Add SMOKE_TEST procedure: `plugin/cursor/SMOKE_TEST.md` (manual end-to-end against a real Cursor session).

The canonical `.mdc` files contain real YAML frontmatter on line 1 — no sentinel in the embedded source. The sentinel is computed and inserted at render time.

## Documentation updates included in this PR

Files in this repo that name the old paths and would rot otherwise:

- `plugin/cursor/README.md` — table rows for `.cursor/rules/specgraph.md` and `post-stage.md` update to `.mdc` with new filenames. Add a one-line note about how init manages them now.
- `plugin/specgraph/README.md:58` — Cursor row references `plugin/cursor/.cursor/rules/post-stage.md`; update path.

Historical design docs under `docs/plans/` that mention the old names (`2026-04-20-multi-platform-plugin-design.md`, `2026-05-06-harness-parity-epic-design.md`, `2026-05-06-spgr-yyjf-deprecate-inject-design.md`) are out of scope — they document the state at their authoring time and shouldn't be revised retroactively.

## Tests

### Unit

- `sentinel_test.go`: update the three assertions that pin the current `:start`-suffixed CommentHTML output (line 28 `TestRenderSentinel_CommentHTML`, line 105 round-trip case, and any callers of `RenderSentinel(CommentHTML, ...)` in the file) to expect the bare `<!-- specgraph:init v=2 sha256=... -->` form. Add a defense-in-depth case asserting that the parser still accepts the legacy `:start` form on read (since `markdownblock.go`'s inline-emitted `:start` markers go through the same parser).
- `wholefile_test.go`: add a `CommentHTML` + `HasFrontmatter: false` round-trip (proves the existing slash-comment paths generalize cleanly to HTML).
- `wholefile_test.go`: add a `CommentHTML` + `HasFrontmatter: true` matrix across the four states — Missing/Synced/Stale/Drifted — verifying the sentinel lands on the first body line after the closing `---`, and that an edited frontmatter triggers Drifted just like an edited body.
- `manifest_test.go`: shape assertions — both new entries present when `HarnessCursor` is enabled, both carry `HasFrontmatter: true`, both carry the expected `SupersedesPath`. Bump the existing entry-count assertions from 6 → 8: `TestManifestShape` at `manifest_test.go:13` AND `TestManifest_AllHarnesses` in `integration_test.go:18` (both hard-code `if len(all) != 6` today and will both fail without the bump).
- Manifest invariant tests:
  - `HasFrontmatter: true` on a non-`WholeFile` entry panics in `init()`.
  - `HasFrontmatter: true` with `Comment == CommentNone` panics in `init()`.
- `TestNoLegacyWholeFileHTMLSentinels` (back-compat anchor): asserts that every shipped manifest entry with `Strategy == StrategyWholeFile && Comment == CommentHTML` also has `HasFrontmatter == true`. This pins the back-compat claim that PR D's in-place change to `RenderSentinel`'s `CommentHTML` branch cannot affect any pre-existing shipped file — no pre-PR-D entry combines WholeFile+HTML, so no shipped sentinel on disk carries the now-obsolete `:start` form. Prevents a later PR from silently introducing such a combination that would then produce the bare-`init` form by surprise.
- `TestVestigialCursorRulePriorHashPinned`: as described in *SupersedesPath* above.
- `TestEmbeddedMdcCanonicalSplitsCleanly`: for each PR-D-added embedded `.mdc` file, asserts `splitFrontmatter` succeeds (frontmatter is well-formed) and the post-frontmatter body is non-empty. Locks the assumption that `renderWholeFile` never sees `ErrFrontmatterMissing` on canonical input.

### Integration

`integration_test.go` declares `package managedfiles_test` and cannot reach the package-private `vestigialCursorSpecgraphMD` bytes directly. Seed scenarios from `internal/config/managedfiles/testdata/` fixtures — the directory already exists (`testdata/golden`) and PR D adds `testdata/cursor-vestigial/{specgraph.md,post-stage.md}` as byte-for-byte copies of the pre-rename canonical. The same fixture content is checked against `vestigialCursor*MD` in a build-tag-free test inside the internal package (next bullet) so a divergence between the embedded canonical and the test fixture also fails CI.

- Scenario 1: project starts with a verbatim copy of the old `.cursor/rules/specgraph.md` (seeded from `testdata/cursor-vestigial/specgraph.md`). Run `Sync`. Assert: `.cursor/rules/specgraph.mdc` exists with correct content + sentinel; `.cursor/rules/specgraph.md` is gone. Same for `post-stage.md`.
- Scenario 2: old `.md` is hand-edited (append a stray comment after seeding). Assert: `.mdc` is written; `.md` remains on disk; `SyncResult` for the `.mdc` entry carries `Detail` containing `supersedes path %q left in place: prior-canonical mismatch`.
- Scenario 3 (idempotency): a second `Sync` after the first reports `ActionNoOp` for both `.mdc` entries.
- Internal-package companion test (`vestigial_v1_test.go` or a new `cursor_rules_test.go`): assert `bytes.Equal(vestigialCursorSpecgraphMD, testdataReadFile("cursor-vestigial/specgraph.md"))` (and same for post-stage). Pins the embedded-vs-fixture invariant from the inside.

### E2E

`plugin/cursor/SMOKE_TEST.md` documents the manual procedure (analogous to PR C's). Prereqs: Cursor CLI, `specgraph` binary on PATH, MCP server running. Steps: `specgraph init`; open Cursor in the project; verify the two rules appear under Cursor's Rules panel with the expected descriptions; trigger each rule by prompting (mention of "spec" or "shape stage" should fire `specgraph.mdc`; a stage transition should fire `specgraph-post-stage.mdc`); inspect the on-disk file and confirm the sentinel line is between the frontmatter close and the H1.

CI does not run this. It's a checked-in manual procedure for release sign-off — same model as `plugin/opencode/SMOKE_TEST.md`.

## Out of scope

- Doctor / orphan surfacing for user-edited old `.md` files (PR G).
- Claude plugin shim (PR E).
- Skills via MCP resource handler (PR F).
- Updates to historical design docs in `docs/plans/` that mention the old names.

## Risks

1. **Cursor mdc parser tolerance of mid-body HTML comments.** The sentinel sits after the frontmatter close, on what is — for the user — the first body line. Cursor's rule processor should treat it as inert markdown content. If it doesn't (e.g., a future Cursor update treats `<!-- specgraph:init ... -->` as something semantic), the rule still loads but the comment may render visibly in the rule preview. Mitigation: the SMOKE_TEST step explicitly checks the rule preview pane.
2. **Hash drift on pre-rename bytes.** If someone re-edits `internal/config/managedfiles/embedded/cursor/vestigial/specgraph.md` after PR D lands, `supersedesGuardedDelete` will stop matching verbatim copies and the cleanup path quietly stops working. Mitigation: a unit test pins the expected hash; any edit to the vestigial file fails CI loudly.
3. **Symlink cycle inside the repo.** PR C's reverse-symlink under `plugin/opencode/` already established the pattern; doubling down for Cursor doesn't add structural risk. `//go:embed` patterns under `internal/config/managedfiles/` do not include `plugin/` — they read the real files inside `embedded/`. The reverse-symlinks under `plugin/cursor/.cursor/rules/` are author-convenience only; the build does not depend on them resolving. The check below is a fast pre-build validation, not a build dependency:
   - Test: `TestPluginCursorSymlinksResolve` (build-tag-free unit test) walks `plugin/cursor/.cursor/rules/`, resolves each `.mdc` symlink with `filepath.EvalSymlinks`, and asserts each resolves to a file under `internal/config/managedfiles/embedded/cursor/`. Catches broken symlinks before they reach a dev's editor.

## Open questions

(None remaining — both major decisions resolved in brainstorming: sentinel-after-frontmatter, explicit `HasFrontmatter` flag.)

## References

- Parent epic: `spgr-rwrp` — harness install parity via embed-and-write
- PR C precedent: [`2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md`](2026-05-11-spgr-rwrp-pr-c-opencode-plugin-design.md)
- Framework foundation: PR A landed `internal/config/managedfiles/`; PR B ported pointers and mcpconfigs in.
- Cursor rule format: <https://cursor.com/docs/context/rules>
