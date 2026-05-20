# spgr-rwrp PR E — Claude plugin shim via embed-and-write

**Date:** 2026-05-12
**Bead:** spgr-kir0
**Status:** Design (post-brainstorm; pre-implementation)
**Parent design:** [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md)
**Predecessors:** PRs 0/A/B/C/D — Claude API verification, `internal/config/managedfiles/` foundation, mcpconfigs/pointers port, OpenCode plugin embed-and-write, Cursor rule files with frontmatter-aware WholeFile.

## Problem

`specgraph init` already writes Claude Code's `.mcp.json` (JSONKeyMerge) and `AGENTS.md` (MarkdownBlock). The Claude plugin shim itself — `plugin/specgraph/` — is still an in-repo authoring directory that never reaches end-user projects unless someone copies it by hand or symlinks a marketplace at `plugin/specgraph/`. That breaks the parent epic's parity promise: a fresh `specgraph init` should produce a complete Claude Code integration with no out-of-band file copies.

PR D shipped the Cursor analog of this. PR E does the same for Claude Code, with two design wrinkles that didn't appear in Cursor:

1. **JSON files can't carry comments**, so the existing in-file sentinel mechanism (a leading comment line carrying `specgraph:init v=2 sha256=...`) has no expression in `plugin.json` and `marketplace.json`. The framework needs a hash-only state-classification path.
2. **`enabledPlugins["specgraph@specgraph-local"]` requires managed-presence semantics**, not the unconditional overwrite that `JSONKeyMerge` does today. A user who runs `/plugin disable specgraph` in Claude Code expects that choice to survive the next `specgraph init`. The framework needs a per-key "write only if absent" mode.

A third item is bookkeeping rather than design: while we're extending `JSONKeyMerge`, we align the existing `Build`-closure entries onto the same declarative shape (`JSONKeys []JSONManagedKey`) so the framework has one model for managed JSON, not two.

## Approach

Adopt the embed-and-write pattern PRs C and D established:

1. Move plugin canonicals into `internal/config/managedfiles/embedded/claude/`. Replace the authoring sources at `plugin/specgraph/.claude-plugin/`, `plugin/specgraph/hooks/`, and `plugin/specgraph/routing-guide.md` with reverse-symlinks back into `embedded/claude/`.
2. Add five `WholeFile` manifest entries under `.specgraph/agents/claude/` for the plugin shim files.
3. Add one `JSONKeyMerge` entry for `.claude/settings.json` carrying the marketplace registration and the enabled-plugin presence key.
4. Extend `wholeFileStrategy` to accept `Comment: CommentNone` for JSON files. State classification falls back to byte-hash matching against canonical + a unified priors registry.
5. Extend `JSONKeyMerge` with a declarative `JSONKeys []JSONManagedKey` field carrying three modes: `KeyManagedValue` (overwrite, today's default), `KeyManagedPresence` (write only if absent), and `KeyManagedArrayUnion` (set-union with existing array, formalizing today's `unionPluginArray` special case).
6. Migrate the three existing `JSONKeyMerge` entries (`.mcp.json`, `.cursor/mcp.json`, `opencode.json`) onto `JSONKeys`. Remove the path-keyed `unionPluginArray` post-merge hook. Remove `Build` from the `JSONKeyMerge` path of the validator.
7. Fold the previous `hooks/hooks.json` content into `plugin.json`'s `hooks` field; drop the standalone file. Rename the hook scripts to `specgraph-session-start.sh` / `specgraph-post-stage.sh` to avoid collisions with user-added hooks.
8. Fold in three PR D review-polish items: unify `vestigialCursorRulePriorHash` + `computePriorCanonical` into one priors registry; extract `keepEditsBodyForFrontmatter` / `keepEditsBodyPlain` helpers from `wholeFileStrategy.Sync`; export drift-detail magic strings as `const` for PR G's doctor regex.

## What changes on disk for end users

After `specgraph init`:

| Path | State |
|---|---|
| `.claude/settings.json` | `extraKnownMarketplaces.specgraph-local` reset to canonical on every init; `enabledPlugins["specgraph@specgraph-local"]` written `true` on first init, preserved thereafter (presence-only). Other keys untouched. |
| `.specgraph/agents/claude/.claude-plugin/plugin.json` | **new** — written by init; drift detected via byte-hash (no in-file sentinel; JSON can't carry one). |
| `.specgraph/agents/claude/.claude-plugin/marketplace.json` | **new** — written by init; drift via byte-hash. |
| `.specgraph/agents/claude/hooks/specgraph-session-start.sh` | **new** — written by init; drift via sentinel (`# specgraph:init v=2 ...`). |
| `.specgraph/agents/claude/hooks/specgraph-post-stage.sh` | **new** — written by init; drift via sentinel. |
| `.specgraph/agents/claude/routing-guide.md` | **new** — written by init; drift via sentinel (`<!-- specgraph:init v=2 ... -->`, `HasFrontmatter: false`). |

After Claude Code starts:

- The local marketplace is registered; `/plugin list` shows `specgraph@specgraph-local`.
- `SessionStart` hook fires and primes the session via `specgraph read-mcp-resource specgraph://prime`.
- `PostToolUse` hook fires after `mcp__specgraph__author` calls and emits a structured `decision: block` payload asking the model to run analytical passes for the just-completed stage.

## Framework changes

Five extensions to `internal/config/managedfiles/`. Three implement PR E directly; two are the PR D review fold-ins.

### 1. `wholeFileStrategy` accepts `Comment: CommentNone`

`CommentNone` already short-circuits `RenderSentinel` to `""` and `ParseSentinel` to a zero `Sentinel` (`sentinel.go:63, 103`). The remaining work in PR E is:

- Allow the combination in `validateManifestEntry`. The PR D back-compat anchor (currently a test invariant in `manifest_test.go:194-202`) moves into the validator as an explicit allow-list of `WholeFile` combinations:

    ```go
    if mf.Strategy == StrategyWholeFile {
        if mf.HasFrontmatter && mf.Comment == CommentNone {
            return fmt.Errorf("manifest entry %q: HasFrontmatter requires a non-None comment style", mf.Path)
        }
        // Supported combinations:
        //   CommentNone  + !HasFrontmatter → JSON files (no in-file sentinel)   [new in PR E]
        //   CommentHash  + !HasFrontmatter → shell / Python / YAML scripts
        //   CommentSlash + !HasFrontmatter → TypeScript / JS plugin source
        //   CommentHTML  + !HasFrontmatter → plain Markdown                     [new in PR E]
        //   CommentHTML  +  HasFrontmatter → Markdown with leading frontmatter
    }
    ```

- The shape test in `manifest_test.go:194-202` flips from a negative back-compat anchor ("no entry should hit `WholeFile+CommentHTML+!HasFrontmatter`") into a positive shape test ("every shipping `WholeFile` entry hits one of the five supported combos"). The PR D anchor itself goes away: `WholeFile+CommentHTML+!HasFrontmatter` is now a shipping combo, exercised by `routing-guide.md` and a new test family.
- In `wholeFileStrategy.Sync` and `.Inspect`: when `Comment == CommentNone`, skip the sentinel render/parse path entirely. The on-disk file is byte-equal to the canonical loaded from `Source`. Hash is `sha256(file_bytes)` with no exclusion logic.

State classification for `CommentNone` `WholeFile` entries:

| Disk state | Classification | Action |
|---|---|---|
| File missing | Missing | Write canonical. |
| Hash(file) == Hash(canonical) | Synced | No-op. |
| Hash(file) ∈ priors-for-this-path | Stale-managed | Overwrite. |
| Hash(file) ∉ priors and ≠ canonical | Drifted-userowned | Refuse with `Detail: "no sentinel"`. Doctor (PR G) flags. |

### 2. `JSONKeys []JSONManagedKey` replaces `Build` on `JSONKeyMerge` entries

New types:

```go
// JSONKeyMode controls how a JSONKeyMerge strategy treats a managed key.
type JSONKeyMode int

const (
    // KeyManagedValue overwrites the key on every init. For object-valued
    // keys, JSON Merge Patch (RFC 7396) recursively merges; for scalar and
    // array values, the canonical value replaces whatever is on disk.
    // This is today's default behavior for every key in a Build patch.
    KeyManagedValue JSONKeyMode = iota

    // KeyManagedPresence ensures the key exists. On first init, the value
    // is written. On subsequent inits, an existing value (any value,
    // including the canonical one or a user-edited variant) is preserved.
    // Useful for keys whose presence we want to guarantee but whose value
    // belongs to the user — e.g. enabledPlugins entries that users toggle
    // via /plugin disable.
    KeyManagedPresence

    // KeyManagedArrayUnion treats the key as an array. The canonical value
    // is unioned with any existing array elements (set-union by value
    // equality; duplicates collapse). Formalizes today's unionPluginArray
    // post-merge hook in jsonkeymerge.go:130-135.
    KeyManagedArrayUnion
)

// JSONManagedKey is one managed key inside a JSONKeyMerge file.
type JSONManagedKey struct {
    // Path is a JSON Pointer (RFC 6901) addressing the key. Use slash-
    // separated segments; characters with special meaning in a JSON
    // Pointer (~, /) must be escaped per RFC 6901 §3.
    Path string

    // Mode is one of KeyManagedValue, KeyManagedPresence,
    // KeyManagedArrayUnion.
    Mode JSONKeyMode

    // Value computes the canonical value for this key at init time.
    // Called once per Inspect/Sync. Static values just ignore params.
    Value func(ProjectParams) (any, error)
}
```

`ManagedFile` gains a `JSONKeys []JSONManagedKey` field. The validator rule:

- `Strategy == StrategyJSONKeyMerge` requires `JSONKeys` non-empty and `Build == nil`.
- `Strategy == StrategyMarkdownBlock` still requires `Build`; `JSONKeys` must be nil.
- `Strategy == StrategyWholeFile` requires `Source`; `Build` and `JSONKeys` must be nil.

`jsonkeymerge.go`'s `jsonKeyMergeCanonical` is rewritten:

1. Start from `existing` (or `{}` if missing).
2. Build a merge patch from all `KeyManagedValue` keys: walk each `Path` as a JSON Pointer and set its value in the patch object. Apply `jsonpatch.MergePatch(existing, patch)`.
3. For each `KeyManagedPresence` key: if the path exists in `existing`, copy that value into the merged result (overriding any patched value); else set Value.
4. For each `KeyManagedArrayUnion` key: read the existing array (empty if absent), append canonical elements not already present (set-union by `reflect.DeepEqual`), set the result.
5. Canonicalize.

The path-keyed `unionPluginArray` post-merge hook in `jsonkeymerge.go:130-135` is removed; `opencode.json`'s `/plugin` key migrates to `KeyManagedArrayUnion`.

### 3. Unified priors registry (fold-in #1)

PR C introduced `computePriorCanonical` for supersedes-path lookups (cursor.mdc files that relocated). PR D introduced `vestigialCursorRulePriorHash` for prior-content-hash lookups (cursor rules that were renamed `.md` → `.mdc`). PR E adds a third use case (JSON files whose content changed shape between versions). Three separate mechanisms today.

Unify into one registry:

```go
// priorsRegistry maps path → list of canonical SHA256 hashes that should
// classify the on-disk file as Stale-managed rather than Drifted-userowned.
// Each entry corresponds to a canonical version that has shipped at some
// point in SpecGraph's history. Adding a hash here says "if a user has
// THIS specific old version on disk, init may safely overwrite."
type priorsRegistry map[string][]string

func priorsFor(path string) []string { ... }
```

PR D's `vestigialCursorRulePriorHash` map merges into `priorsRegistry` directly (already a path → hash mapping). PR C's `computePriorCanonical` (a helper that re-renders a known prior `Build`-closure result with a stable `ProjectParams`) is reframed as a registry-population helper: at package init, each `Build`-style prior is rendered once and its hash registered against its destination path. The supersedes-path mechanism (a separate concern — relocations like `.cursor/rules/specgraph.mdc` → `.specgraph/agents/cursor/...`) stays as-is; this unification covers prior-hash lookups only.

### 4. `keepEditsBodyForFrontmatter` / `keepEditsBodyPlain` helpers (fold-in #2)

PR D inlined two body-extraction branches in `wholeFileStrategy.Sync` to handle the frontmatter / no-frontmatter cases. PR E touches `Sync` to add the `CommentNone` path, so we extract while warm. Pure refactor.

### 5. Exported drift-detail consts (fold-in #3)

PR D's `Detail` string format uses two magic-prefix strings. PR G's doctor will regex-match them to render user-facing classifications. Make them exported `const` now:

```go
const (
    DriftDetailNoSentinel              = "no sentinel"
    DriftDetailFrontmatterBrokenPrefix = "frontmatter broken: "
    DriftDetailSupersedesPath          = "supersedes path "
)
```

Producers in `wholeFile.go`, `markdownblock.go`, and the new JSON path use the consts. PR G's regex tests against the same identifiers.

## Manifest entries

Total grows from 8 → 14, matching the parent design's table.

### Migrated (3, identical observable behavior)

| Path | JSONKeys |
|---|---|
| `.mcp.json` | `/mcpServers/specgraph` (KeyManagedValue, dynamic via ProjectParams) |
| `.cursor/mcp.json` | `/mcpServers/specgraph` (KeyManagedValue, dynamic) |
| `opencode.json` | `/$schema` (KeyManagedValue, static); `/mcp/specgraph` (KeyManagedValue, dynamic); `/plugin` (KeyManagedArrayUnion, static `["./.specgraph/agents/opencode/specgraph.ts"]`) |

### New (6)

| # | Path | Strategy | Comment | HasFm | Source / JSONKeys |
|---|---|---|---|---|---|
| 9 | `.claude/settings.json` | JSONKeyMerge | None | — | see below |
| 10 | `.specgraph/agents/claude/.claude-plugin/plugin.json` | WholeFile | None | false | `embedded/claude/.claude-plugin/plugin.json` |
| 11 | `.specgraph/agents/claude/.claude-plugin/marketplace.json` | WholeFile | None | false | `embedded/claude/.claude-plugin/marketplace.json` |
| 12 | `.specgraph/agents/claude/hooks/specgraph-session-start.sh` | WholeFile | Hash | false | `embedded/claude/hooks/specgraph-session-start.sh` |
| 13 | `.specgraph/agents/claude/hooks/specgraph-post-stage.sh` | WholeFile | Hash | false | `embedded/claude/hooks/specgraph-post-stage.sh` |
| 14 | `.specgraph/agents/claude/routing-guide.md` | WholeFile | HTML | false | `embedded/claude/routing-guide.md` |

`.claude/settings.json` JSONKeys:

````go
JSONKeys: []JSONManagedKey{
    {
        Path: "/extraKnownMarketplaces/specgraph-local",
        Mode: KeyManagedValue,
        Value: func(_ ProjectParams) (any, error) {
            return map[string]any{
                "source": map[string]any{
                    "type": "directory",
                    "path": "./.specgraph/agents/claude",
                },
                "autoUpdate": false,
            }, nil
        },
    },
    {
        Path: "/enabledPlugins/specgraph@specgraph-local",
        Mode: KeyManagedPresence,
        Value: func(_ ProjectParams) (any, error) { return true, nil },
    },
},
````

Verified by PR 0 (see `docs/plans/2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`):

- `source.path` is the marketplace root — the directory containing `.claude-plugin/`, NOT `.claude-plugin/` itself.
- `autoUpdate: false` is honored at the marketplace-entry level. For `directory`-source marketplaces Claude reads files live (no cache copy), so file-content drift is not blocked by the flag — that is exactly what we want operationally, since every `specgraph init` should be visible to the next session.
- `${CLAUDE_PLUGIN_ROOT}` resolves to the plugin's own root directory regardless of marketplace shape.

## Repo-internal renames and canonical relocation

Hook script renames (collision-proofing — `session-start.sh` / `post-stage.sh` are generic names a user-added hook could trip over):

````text
plugin/specgraph/hooks/session-start.sh  →  …/hooks/specgraph-session-start.sh
plugin/specgraph/hooks/post-stage.sh     →  …/hooks/specgraph-post-stage.sh
````

Canonical relocation (mirrors PR C):

````text
internal/config/managedfiles/embedded/claude/
├── .claude-plugin/
│   ├── plugin.json                       ← new (hooks declaration inlined)
│   └── marketplace.json                  ← new
├── hooks/
│   ├── specgraph-session-start.sh        ← moved + renamed
│   └── specgraph-post-stage.sh           ← moved + renamed
└── routing-guide.md                      ← moved from plugin/specgraph/

plugin/specgraph/                         ← reverse-symlinks for author convenience
├── .claude-plugin/  →  ../../internal/config/managedfiles/embedded/claude/.claude-plugin/
├── hooks/           →  ../../internal/config/managedfiles/embedded/claude/hooks/
├── routing-guide.md →  ../../internal/config/managedfiles/embedded/claude/routing-guide.md
├── README.md                             ← stays (not managed; author docs)
└── skills/                               ← stays (owned by PR F)
````

`plugin/specgraph/hooks/hooks.json` is deleted — the hooks array moves into `plugin.json`:

````json
{
  "name": "specgraph",
  "description": "Thin Claude Code client for SpecGraph. Rich authoring workflow guidance is delivered from the SpecGraph MCP server.",
  "version": "0.4.0",
  "hooks": [
    {
      "event": "SessionStart",
      "type": "command",
      "command": "${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-session-start.sh"
    },
    {
      "event": "PostToolUse",
      "matcher": "mcp__specgraph__author",
      "type": "command",
      "command": "${CLAUDE_PLUGIN_ROOT}/hooks/specgraph-post-stage.sh"
    }
  ]
}
````

`marketplace.json` content:

````json
{
  "name": "specgraph-local",
  "owner": { "name": "SpecGraph" },
  "plugins": [
    {
      "name": "specgraph",
      "source": "./",
      "description": "Thin Claude Code client for SpecGraph. Rich authoring workflow guidance is delivered from the SpecGraph MCP server.",
      "version": "0.4.0"
    }
  ]
}
````

`plugins[].source: "./"` — single-plugin co-located marketplace; the plugin shares the marketplace root. PR 0 confirmed `..` traversal is blocked, so any future multi-plugin layout must keep all plugins inside the marketplace root.

## Tests

### Unit (`internal/config/managedfiles/`)

- `TestWholeFileJSON_*` — `WholeFile + CommentNone + !HasFrontmatter`. Cases: Missing, Synced, Stale-via-prior, Drifted-userowned.
- `TestJSONKeyMerge_PresenceMode_*` — `KeyManagedPresence`. Cases: key absent → write; key present with canonical value → preserve; key present with non-canonical value (user disabled) → preserve.
- `TestJSONKeyMerge_ArrayUnion_*` — `KeyManagedArrayUnion`. Cases: array absent → set to canonical; array present with disjoint elements → union; array present with overlapping elements → dedupe; array present and equal to canonical → no-op. Migrates the existing `opencode.json` plugin-array tests into this family.
- `TestPriorsRegistry_*` — unified registry. Both cursor-rule priors (PR D) and JSON priors (PR E) accessible via `priorsFor(path)`.
- `TestManifestValidator_WholeFileCombinations` — positive shape test asserting every shipping `WholeFile` entry falls into one of the five supported combos. Replaces the negative anchor in `manifest_test.go:194-202`.
- `TestWholeFileHTMLPlainMarkdown_*` — covers the newly-supported `WholeFile + CommentHTML + !HasFrontmatter` combo. Cases: Missing, Synced, Stale-via-prior, Drifted-userowned. Sentinel placed at line 1.
- `TestManifestValidator_JSONKeyMergeXOR` — `JSONKeyMerge` requires `JSONKeys` non-empty; `Build` must be nil.
- `TestManifestValidator_JSONKeysOnNonJSONKeyMerge` — `JSONKeys` is rejected on `WholeFile` and `MarkdownBlock` entries.
- `TestJSONKeyMerge_MigratedEntries_*` — golden-file tests that the three migrated entries produce byte-identical canonical output to the pre-migration `Build` closures.

### Integration

- `TestClaudeSettingsJSON_Integration` — writes `.claude/settings.json` via real strategy invocation. Asserts marketplace entry is overwritten and presence key is preserved across two inits with a user-edit between.

### E2E (`e2e/api/`)

- `TestClaudePluginShim_E2E` — runs `specgraph init` in a tempdir, asserts all 14 managed files appear at their expected paths with expected content. Runs init a second time and asserts a hand-flipped `enabledPlugins["specgraph@specgraph-local"] = false` is not flipped back to `true`. Asserts the marketplace path value remains `./.specgraph/agents/claude`.

## Documentation updates included in this PR

- `CLAUDE.md`: update the "Plugin shims" paragraph to describe Claude alongside Cursor/OpenCode; mention `.claude/settings.json` is now init-managed.
- `plugin/specgraph/README.md`: rewrite to reflect the reverse-symlink layout and the canonical-at-`embedded/claude/` location.
- `docs/plans/2026-05-08-spgr-rwrp-harness-install-parity-design.md`: mark PR E as merged in the status table (during the merge commit, not in this PR's content).

## Out of scope

- Removing the `plugin/specgraph/skills` symlink and adopting MCP-resource skills delivery — that's PR F.
- Migrating `MarkdownBlock` entries onto a declarative key shape — `Build` remains the right model for arbitrary text-body content. Only `JSONKeyMerge` adopts `JSONKeys`.
- `specgraph doctor` and the drift nudge — PR G.
- Publishing the Claude plugin to a public marketplace — `spgr-eo4n` covers that.
- Codex MCP config + shim — `spgr-uds0` covers that.

## Risks

- **Existing `JSONKeyMerge` regression on migrated entries.** The migration must produce byte-identical canonical output to today's `Build`-closure path. Mitigation: golden-file tests on each migrated entry's output across a matrix of `ProjectParams` (`TestJSONKeyMerge_MigratedEntries_*`).
- **Claude Code's marketplace.json schema rejecting our minimal entry.** PR 0 verified the marketplace fields we use; PR E's `marketplace.json` is constructed from those exact verified fields. If Claude Code's schema tightens in a future version, our E2E test should catch it on the next Claude update.
- **JSON Pointer escaping bugs.** Keys like `specgraph@specgraph-local` contain `@` — fine in JSON Pointer. Keys with `/` or `~` would need RFC 6901 escaping. PR E's only such case is `enabledPlugins/specgraph@specgraph-local`; tests cover the path-parse round-trip explicitly.
- **Existing user with hand-installed plugin at `~/.claude/plugins/specgraph/`.** Out of scope to detect; users with a global install bypass our marketplace registration. Documented as a known edge case in `plugin/specgraph/README.md`.

## Open questions

None. All three brainstorming questions resolved:

- Q1 (JSON sentinel form): hash-only via priors registry, no in-file sentinel for JSON files.
- Q2 (managed-presence semantics): `KeyManagedPresence` mode on `JSONManagedKey`.
- Q3 (`extraKnownMarketplaces.specgraph-local` schema): path value is the marketplace root (`.specgraph/agents/claude`), NOT `.claude-plugin/`, per PR 0.

## References

- Parent epic: [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md)
- PR 0 Claude API verification: [`2026-05-08-spgr-rwrp-pr0-claude-api-verification.md`](2026-05-08-spgr-rwrp-pr0-claude-api-verification.md)
- PR D Cursor rule design (template for this doc): [`2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md`](2026-05-12-spgr-rwrp-pr-d-cursor-rules-design.md)
- RFC 6901 (JSON Pointer): <https://tools.ietf.org/html/rfc6901>
- RFC 7396 (JSON Merge Patch): <https://tools.ietf.org/html/rfc7396>
