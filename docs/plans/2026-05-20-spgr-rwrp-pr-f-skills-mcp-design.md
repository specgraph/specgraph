# spgr-rwrp PR F — Skills via MCP resource handler

**Date:** 2026-05-20
**Bead:** spgr-i3he
**Status:** Design (post-brainstorm; pre-implementation)
**Parent design:** [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md)
**Predecessors:** PRs 0/A/B/C/D/E — Claude API verification, `internal/config/managedfiles/` foundation, mcpconfigs/pointers port, OpenCode plugin embed-and-write, Cursor rule files with frontmatter-aware WholeFile, Claude plugin shim with JSONKeys + WholeFile + CommentNone.

## Problem

The six SpecGraph skills (`specgraph-authoring`, `-graph-query`, `-analytical-passes`, `-drift`, `-conventions`, `-troubleshooting`) live in `<repo>/skills/` and are exposed to harnesses through a single dev-time symlink at `plugin/specgraph/skills → ../../skills`. End-user projects never receive these files — they only exist for in-repo authoring convenience. As a result:

1. Models running against `specgraph init`-ed projects have no way to discover or fetch the skills. The Claude plugin shim, the Cursor `.mdc` rules, and the OpenCode plugin all *reference* skills (`specgraph-authoring` is named in routing guidance, for example), but the bytes aren't reachable.
2. The lone `plugin/specgraph/skills` symlink is a footgun: it reads correctly when authors edit the repo, but creates the false impression that "skills are part of the plugin shim" — leading reviewers to ask why Cursor and OpenCode don't have parallel symlinks. They don't because skills were never *meant* to be on-disk in end-user projects; the parent epic locks MCP-fetch as the chosen mechanism.
3. There is a `task plugin:sync` target whose only job is refreshing this symlink. It's vestigial.

PR F closes the loop: embed the skills tree into the binary, serve it via the existing MCP resource/tools machinery, point harnesses at the MCP path via prime, and delete the symlink + task + stale doc paragraphs in the same change so nothing references a path that no longer exists.

## Approach

Land in ten focused commits, each green:

1. Relocate the six skill directories from `<repo>/skills/` to `internal/mcp/skills/embedded/` and replace `<repo>/skills` with a directory symlink pointing back. No content change, just the move.
2. Add a required `summary:` frontmatter field (≤120 chars) to all six `SKILL.md` files in their new location.
3. Update `internal/skillvalidate` to require `summary`.
4. Introduce the new `internal/mcp/skills/` package with a small read-only `Source` interface (`List`, `Get`, `Search`) and one implementation, `embeddedSource`, backed by `//go:embed embedded/*/SKILL.md`. Eagerly parse + validate at constructor time so `specgraph serve` refuses to start on a malformed binary.
5. Implement `Search` on the embedded source with two modes: case-insensitive substring (default) and RE2 regex.
6. Add three MCP tools — `specgraph_skills_list`, `specgraph_skills_get`, `specgraph_skills_search` — all dependent on a `skills.Source` (not the concrete type), so future composite/remote sources plug in without touching handlers.
7. Add the templated resource `specgraph://skills/<name>` to `RegisterResources`, returning the same bytes as `specgraph_skills_get`.
8. Update the prime template with a short pointer paragraph that names the three tools and the resource URI — no per-skill enumeration in prime.
9. Delete `plugin/specgraph/skills` symlink and `task plugin:sync`.
10. Rewrite the `CLAUDE.md` "Plugin shims" and "Shared skills" paragraphs to describe MCP-fetch delivery; update `plugin/specgraph/README.md`; add the three new tool names to `plugin/specgraph/routing-guide.md`; verify the top-level `README.md`.

The interface boundary in step 2 is the load-bearing design choice. Three reasonable future evolutions all become additive:

- **Project-local extension skills** — ship a `dirSource` that reads `.specgraph/skills/*/SKILL.md`, wire it behind a `compositeSource` in front of `embeddedSource`.
- **Remote registry skills** — ship a `remoteSource` with an HTTP client and a local cache; same composite pattern.
- **Per-project skill pinning** — a wrapper source that filters by manifest.

None of those touch the MCP handlers, the prime template, or the tool surface. Only the registry constructor changes.

## What changes for end users

After `specgraph init`, no new files appear on disk. Skills are served by the server, fetched by harnesses:

| Surface | What it returns |
|---|---|
| Tool `specgraph_skills_list()` | `[ { name, summary, uri } ]` — six entries, ordered by `name`. `uri` is `specgraph://skills/<name>`. |
| Tool `specgraph_skills_get(name)` | `text/markdown` content: the verbatim `SKILL.md` bytes (frontmatter included). `connect.CodeNotFound` if name unknown. |
| Tool `specgraph_skills_search(query, regex?)` | Same row shape as `_list`. `regex` defaults to `false` (case-insensitive substring across name, summary, body). `connect.CodeInvalidArgument` for unparseable regex. |
| Resource `specgraph://skills/<name>` | Same bytes as `_get`. Registered as a templated handler alongside `specgraph://spec/{slug}`. |
| Resource `specgraph://prime` | Grows a `## Skills` section: a short paragraph naming the three tools and the resource URI. No per-skill content. |

The Claude plugin shim's `routing-guide.md`, Cursor's `.mdc` rules, and OpenCode's `specgraph.ts` already reference skills by name. After PR F those references resolve via MCP fetch instead of by file-system path. No harness-side edits are required because the existing routing text doesn't say *how* to load a skill — that decision was deferred to PR F.

## Package layout

```text
internal/mcp/
├── resources.go              # existing — gains skillsResourceHandler; RegisterResources signature grows a src skills.Source param
├── tools_skills.go           # NEW — RegisterSkillTools(r, src) and the three handlers
├── tools_skills_test.go      # NEW
├── server.go                 # existing — constructs skills.NewEmbedded() once at startup and threads it through
└── skills/                   # NEW subpackage
    ├── doc.go                # Package comment: Source contract, summary extension
    ├── skills.go             # Source interface, Meta, Skill, Errors
    ├── embedded.go           # embeddedSource implementation; //go:embed directive
    ├── embedded_test.go      # FS contents, parsing, search, fakes
    └── embedded/             # canonical content lives here (real files)
        ├── specgraph-authoring/SKILL.md
        ├── specgraph-graph-query/SKILL.md
        ├── specgraph-analytical-passes/SKILL.md
        ├── specgraph-drift/SKILL.md
        ├── specgraph-conventions/SKILL.md
        └── specgraph-troubleshooting/SKILL.md

skills -> internal/mcp/skills/embedded   # repo-root reverse-symlink (single directory)
```

Tools follow the existing per-domain pattern (`tools_authoring.go`, `tools_core.go`, `tools_graph.go`, etc.) — `tools_skills.go` joins that family. The subpackage owns the `Source` contract and the embedded implementation; the parent `internal/mcp` package owns the MCP-binding glue (tool definitions, resource handler, registration).

The `//go:embed embedded/*/SKILL.md` directive in `internal/mcp/skills/embedded.go` reads real files inside the package. The repo-root `<repo>/skills/` becomes a single reverse-symlink to `internal/mcp/skills/embedded/`, matching the pattern PR C/D/E established for `internal/config/managedfiles/embedded/<harness>/` (canonicals inside the package, `plugin/specgraph/` paths reverse-symlinked back to them).

This direction is deliberate: `//go:embed` does not portably follow symlinks to subdirectories, so canonicals must live inside the embedding package. The reverse-symlink at the repo root preserves the existing `<repo>/skills/<name>/SKILL.md` paths that `task skills:validate skills/`, the GitHub UI, and human authors rely on. The validator (`internal/skillvalidate`) keeps working because `os.Stat(root)` at `validate.go:58` resolves the top-level `<repo>/skills` symlink before `filepath.WalkDir` starts walking — once the walk root is the real directory inside the package, the walk proceeds normally without descending more symlinks. (`filepath.WalkDir` itself does *not* follow directory symlinks; this works because of the up-front `os.Stat`.) A new `TestValidateRoots_FollowsRepoSymlink` pins this behaviour explicitly so a future change to the walk-root handling doesn't silently regress.

The `//go:embed embedded/*/SKILL.md` glob only picks up direct-child `SKILL.md` files. This is tied to the **single-file SKILL.md invariant** today's skills satisfy. If a future skill ships a `references/` subdirectory or images, three things must change together: the embed directive (broaden to `embedded/*` or list explicit subdirs), the `Source.Get` return shape (likely growing an `Assets map[string][]byte` field or similar), and the validator (today validates only SKILL.md). The "Out of scope" list captures this — the design's current shape is correct for single-file skills and intentionally doesn't try to anticipate the multi-file case.

The canonical-relocation commit moves all six skill directories from `<repo>/skills/` to `internal/mcp/skills/embedded/` and replaces `<repo>/skills/` with the directory symlink in one atomic `git mv` + symlink-create operation. A `TestSkillsSymlink` test (in the package, mirroring `internal/config/managedfiles/symlink_pluginshim_test.go`) asserts the symlink resolves to the embedded directory so a broken symlink doesn't silently slip through review.

### `Source` interface

```go
// Source is the read-only catalog interface for SKILL.md packages.
type Source interface {
    List(ctx context.Context) ([]Meta, error)
    Get(ctx context.Context, name string) (Skill, error)
    Search(ctx context.Context, query string, opts SearchOptions) ([]Meta, error)
}

// Meta is one catalog row: what specgraph_skills_list returns per skill.
type Meta struct {
    Name    string
    Summary string
    URI     string // canonical fetch URI, e.g. "specgraph://skills/specgraph-authoring"
}

// Skill is the full payload returned by Get and by the resource handler.
type Skill struct {
    Meta
    Body []byte // verbatim SKILL.md bytes (frontmatter + content)
}

// SearchOptions tune Source.Search. Zero value = case-insensitive
// substring search across name, summary, and body, no result cap.
type SearchOptions struct {
    Mode   SearchMode    // Text (default) or Regex
    Fields []SearchField // empty = all of {Name, Summary, Body}
    Limit  int           // 0 = no cap
}

type SearchMode int
const (
    SearchText  SearchMode = iota // case-insensitive substring
    SearchRegex                   // RE2 regex
)

type SearchField int
const (
    FieldName SearchField = iota
    FieldSummary
    FieldBody
)

// Errors returned by Source. Map at the handler boundary to connect codes.
var (
    ErrNotFound     = errors.New("skill not found")     // → CodeNotFound
    ErrInvalidQuery = errors.New("invalid query")       // → CodeInvalidArgument
)
```

### `embeddedSource`

```go
//go:embed embedded/*/SKILL.md
var embeddedFS embed.FS

// NewEmbedded loads and validates every embedded SKILL.md once. Returns a
// Source whose List/Get/Search read from the prebuilt in-memory catalog.
// Any malformed skill (missing required frontmatter, summary > 120 chars,
// invalid YAML) returns a precise error from the constructor and causes
// `specgraph serve` startup to fail loudly.
func NewEmbedded() (Source, error)
```

Internal shape:

```go
type embeddedSource struct {
    byName map[string]Skill // populated once in NewEmbedded
    order  []string         // sorted names, used by List/Search for stable order
}
```

Wiring at server startup (`internal/mcp/server.go`, currently calling `RegisterResources(reg, client)` then `RegisterPrompts(reg, client)` at L41-42):

```go
src, err := skills.NewEmbedded()
if err != nil {
    return nil, fmt.Errorf("load skills: %w", err) // fatal at server startup
}
RegisterResources(reg, client, src) // signature grows from (r, c)
RegisterPrompts(reg, client)
RegisterSkillTools(reg, src)        // new — appended after existing registrations
```

Test fakes — handler unit tests in `internal/mcp/tools_skills_test.go` and `resources_test.go` define an in-test `fakeSource` struct that satisfies `skills.Source` (three methods, exported types only). The interface and types stay exported so any package — including future external skill-source implementations — can implement them.

`Search` walks `order`, applying the predicate built from `query`/`opts`:

- `SearchText` (default): lowercases query and each scanned field, uses `strings.Contains`.
- `SearchRegex`: `regexp.Compile(query)` once at call time; returns `ErrInvalidQuery` on compile error. RE2 — no catastrophic backtracking.

The `SearchOptions.Fields` and `SearchOptions.Limit` knobs are exposed in Go but **not** plumbed through the MCP tool surface in this PR — the tool exposes only the `regex` boolean. The richer options stay available for in-package callers and future tool growth.

## SKILL.md schema change

All six current `SKILL.md` files get a `summary:` field inserted between `name:` and `description:`. Example:

````markdown
---
name: specgraph-authoring
summary: Author a SpecGraph spec through the spark → shape → specify → decompose → approve funnel.
description: |
  Use when the user wants to author or update a SpecGraph spec — sparking
  an idea, shaping a problem statement, specifying detail, decomposing into
  work, or walking a spec to approval. Routes to the right MCP prompt for
  the current authoring stage.
license: Apache-2.0
metadata:
  source: https://github.com/specgraph/specgraph
---

# SpecGraph Authoring
...
````

Constraints (enforced by `internal/skillvalidate`):

- `summary` is required.
- ≤ 120 characters after YAML decoding (the body, not the source bytes — multi-line scalars normalize).
- No leading/trailing whitespace.

The validator changes are minimal — one entry in the required-keys list plus one new check. The existing `TestValidateRoots_RejectsMissingFrontmatter`-family tests grow two cases (`RejectsMissingSummary`, `RejectsOverlongSummary`).

The agentskills.io spec doesn't define `summary`. We add it as a SpecGraph-local extension, documented in the new `internal/mcp/skills/doc.go` package comment.

## Public surface details

### Tool — `specgraph_skills_list`

- Args: none.
- Returns: `[ { name string, summary string, uri string } ]`. Six entries today, ordered by `name` ascending.
- Errors: never (the catalog is built at startup; if it had failed, the server wouldn't be running).

### Tool — `specgraph_skills_get`

- Schema (built with the existing helpers in `internal/mcp/helpers.go`, matching the `tools_authoring.go` convention): `objectSchema(props{"name": stringProp("Skill name (kebab-case, e.g. specgraph-authoring)")}, "name")`. The handler reads `name := stringParam(params, "name")` from the `map[string]any` argument.
- Returns: `text/markdown` content (single content item) with the verbatim `SKILL.md` bytes — that is, the handler projects `Skill.Body` from the `Source.Get` return; `Skill.Meta` is available to the handler internally but not surfaced in the MCP response (the frontmatter is already present in `Body`, so re-emitting `Meta` would be redundant).
- Errors: `CodeNotFound` if `name` not in catalog (maps from `skills.ErrNotFound`); `CodeInvalidArgument` if `name` is empty.

### Tool — `specgraph_skills_search`

- Schema: `objectSchema(props{"query": stringProp("Substring (default) or RE2 regex"), "regex": boolProp("Treat query as RE2 regex; defaults to false. Must be a JSON boolean, not the string \"true\"")}, "query")`. `query` is required; `regex` is optional and defaults to `false` when omitted. Handler reads `query := stringParam(params, "query")` and `regex := boolParam(params, "regex")` from the `map[string]any` argument. Note: per the existing `boolParam` convention at `helpers.go:57-62`, a non-bool value (including the stringified `"true"` some clients emit) silently maps to `false` rather than erroring — that's a project-wide quirk, not PR-F-introduced. The `boolProp` description above surfaces it to clients; the alternative (a strict bool-or-error reader) is out of scope and would touch every existing tool.
- Returns: `[ { name, summary, uri } ]` — same shape as `_list`, filtered by the predicate. Note: the response does **not** indicate which field matched (a body-match and a name-match look identical at the tool boundary). See "Out of scope" for the rationale.
- Errors: `CodeInvalidArgument` if `query` empty or (in regex mode) un-compilable (maps from `skills.ErrInvalidQuery`).

### Resource — `specgraph://skills/<name>`

- Handler lives in `internal/mcp/resources.go` next to `primeResourceHandler` and the `specgraph://spec/{slug}` family. The function takes a `skills.Source` parameter.
- Registered inside the existing `RegisterResources` function as a templated entry — `IsTemplate: true`, `URI: "specgraph://skills/{name}"`, mirroring the convention `specgraph://spec/{slug}` uses at `resources.go:369-373`. Signature grows from `(r *Registry, c *Client)` to `(r *Registry, c *Client, src skills.Source)`. The single caller (`internal/mcp/server.go`) passes the source built at startup. No new registration entry point.
- URI parsing: a new `extractSkillName(uri string) (string, error)` helper, a sibling to the existing `extractSlugFromURI` at `resources.go:52-58` (which returns the last segment unconditionally and is too permissive for skills). Canonical algorithm: strip the `specgraph://` prefix; split on `/` (vanilla `strings.Split`, no `SplitN`); assert `len(parts) == 2 && parts[0] == "skills" && skillvalidate.NameRegex.MatchString(parts[1])`. The regex `^[a-z0-9]+(?:-[a-z0-9]+)*$` is **defined once** at `internal/skillvalidate/name.go` as `var NameRegex = regexp.MustCompile(...)` (added in commit 3 alongside the `summary` field) and imported by both this resource handler and the `skills` subpackage's `embeddedSource` constructor (commit 4). One regex, three callers, no drift. Edge cases — each returns `ErrNotFound` mapped to `connect.CodeNotFound`:
  - `specgraph://skills` → `parts == ["skills"]`, len 1, reject
  - `specgraph://skills/` → `parts == ["skills", ""]`, name fails regex, reject
  - `specgraph://skills//` → `parts == ["skills", "", ""]`, len 3, reject
  - `specgraph://skills/foo/` → `parts == ["skills", "foo", ""]`, len 3, reject
  - `specgraph://skills/foo/bar` → len 3, reject
  - `specgraph://SKILLS/foo` → first segment uppercase, reject (URI scheme + first segment are case-sensitive — we don't tolerate mixed-case)
  - `specgraph://skills/Foo` → name fails regex (uppercase F), reject
  - `specgraph://skills/foo%20bar` → name fails regex (`%`), reject — we don't URL-decode; skill names never need encoding by convention
  - `specgraph://skills/specgraph-authoring` → accept, return `"specgraph-authoring"`

  `TestSkillsResourceHandler_RejectsMalformedURI` (see Tests) enumerates all of the rejection cases above plus a couple of accept cases.
- Payload: identical to `specgraph_skills_get(name)`.

### Resource — `specgraph://prime` (modified)

Inserts a new section between "Graph Overview" and "Ready to Work" (or at a stable position determined during implementation; see `internal/mcp/resources.go:223+` for the existing order):

````markdown
## Skills

This server exposes 6 skills covering authoring, graph queries, analytical
passes, drift, conventions, and troubleshooting.

- `specgraph_skills_list` — see the catalog with one-line summaries
- `specgraph_skills_search` — search by keyword (or regex with `regex: true`)
- `specgraph_skills_get` / `specgraph://skills/<name>` — fetch a specific skill
````

The exact wording and skill-count are templated from the live catalog so a future skill addition updates prime automatically.

## Cleanup landed in this PR

| Removal | Why |
|---|---|
| `plugin/specgraph/skills` symlink | Dev-time artifact only. `plugin/cursor/skills` and `plugin/opencode/skills` never existed — the parent design's "delete in all three shims" was overstated. |
| `task plugin:sync` target (`Taskfile.yml`) | Existed only to refresh the now-deleted symlink. |
| References to `task plugin:sync` in CONTRIBUTING / docs | Search for `plugin:sync` repo-wide; remove or update each hit. |
| `CLAUDE.md` "Plugin shims" + "Shared skills" paragraphs | Rewritten to describe MCP-fetch delivery; remove the "All three symlink `skills/` to the in-tree `skills/`" line. |
| `plugin/specgraph/README.md` | Update any "skills are symlinked" language to describe MCP-resource fetch. |
| Top-level `README.md` | Search for `plugin:sync` / `skills/` references; edit if any. |

## Tests

### Unit — `internal/mcp/skills/embedded_test.go`

- `TestEmbeddedFS_ListsAllSixSkills` — `fs.WalkDir(embeddedFS, ...)` finds exactly the six expected SKILL.md paths.
- `TestNewEmbedded_ParsesEverySkill` — constructor succeeds; `List` returns six entries in name order.
- `TestNewEmbedded_FailsOnMissingSummary` — uses a private constructor variant taking an `fs.FS` to inject a malformed in-memory FS; asserts a precise error.
- `TestNewEmbedded_FailsOnOverlongSummary` — same pattern, 121-char summary.
- `TestGet_KnownSkillReturnsBody` — `Get("specgraph-authoring")` returns bytes containing the `name:` line.
- `TestGet_UnknownReturnsErrNotFound`.
- `TestSearch_TextMatchesAcrossFields` — query "drift" matches the drift skill (name) and any body mention.
- `TestSearch_TextCaseInsensitive` — "DRIFT" matches the same.
- `TestSearch_RegexAnchors` — `\\bdrift\\b` matches the drift skill but not "drifted" inside another body.
- `TestSearch_RegexInvalidReturnsErrInvalidQuery`.
- `TestSearch_FieldsRestriction` — `Fields: []SearchField{FieldName}` scopes the scan; queries that match only in body return no rows.
- `TestSearch_LimitClampsResults`.
- `TestSearch_StableOrder` — same query twice returns the same row order.

### Validator — `internal/skillvalidate/validate_test.go`

- `TestValidateRoots_RejectsMissingSummary` — new.
- `TestValidateRoots_RejectsOverlongSummary_BlockScalar` — new. Fixture uses a YAML block-scalar (`summary: >` or `summary: |`) whose **source bytes** are under 120 chars (e.g., 4 lines of 25 chars = 100 source bytes) but whose **decoded value** is over 120 chars after newlines fold to spaces. This pins the "after YAML decoding" semantics — a naïve `len(rawBytes)` check would pass; the decoded check fails. The companion `TestValidateRoots_RejectsOverlongSummary_FlowScalar` covers the single-line case.
- `TestValidateRoots_AcceptsValidSkill` — extended fixture includes `summary:`.
- `TestValidateRoots_RejectsNonKebabName` — new. Fixture with `name: Foo_Bar` (matching dirname `Foo_Bar`) — passes the pre-PR-F `parsed.Name == dirName` check (`validate.go:106-111`) but fails the new `NameRegex` invariant. Pins the gap that pass-4 surfaced: previously a skill named `Foo_Bar` would pass the validator and then be unreachable via `specgraph://skills/Foo_Bar` because the resource regex rejected it. After commit 3 the validator rejects too, so resource handler and validator agree.
- `TestValidateRoots_FollowsRepoSymlink` — new (mentioned in "Package layout"). Asserts the validator walks correctly when the root path is a directory symlink, pinning the `os.Stat`-resolves-top-level behaviour.

### MCP handlers — `internal/mcp/tools_skills_test.go` and `resources_test.go`

All handler tests live in `package mcp` (internal tests) so they can construct a `Registry` directly and assert against the unexported fields used by the existing `RegisterResources` / `RegisterAuthoringTools` tests. The fake `Source` is a per-test struct literal satisfying the three `skills.Source` methods — no dependency on the `embeddedSource` constructor, no need for `_test.go` files in the `skills` subpackage.

- `TestSpecgraphSkillsList_ReturnsCatalog` — uses a fake `skills.Source` with two entries; asserts the tool returns the expected JSON shape.
- `TestSpecgraphSkillsGet_KnownAndUnknown`.
- `TestSpecgraphSkillsSearch_TextAndRegex`.
- `TestSkillsResourceHandler_KnownAndUnknown` — confirms the resource handler returns the expected payload and errors on unknown name.
- `TestSkillsResourceHandler_RejectsMalformedURI` — `specgraph://skills`, `specgraph://skills/foo/bar`, and `specgraph://skills/` all return `CodeNotFound`. Pins `extractSkillName`'s strictness.

### E2E — `e2e/api/skills_test.go`

A Ginkgo test boots the real server with the embedded source, then via the same `testutil.NewCLIRunner` pattern PR E used:

- `specgraph read-mcp-resource specgraph://skills/specgraph-authoring` returns bytes containing `name: specgraph-authoring`.
- A tool-call to `specgraph_skills_list` returns six rows.
- A search for `drift` returns at least the drift skill.

## Sequencing

Ten commits, each green under `task check`:

| # | Commit | Description |
|---|--------|-------------|
| 1 | `refactor(skills): relocate canonicals into internal/mcp/skills/embedded/` | One working-copy snapshot containing both the directory move and the new repo-root symlink. Author flow: `mv skills internal/mcp/skills/embedded && ln -s internal/mcp/skills/embedded skills`, then let jj snapshot the result — jj detects the rename by content similarity and produces a single commit with the move plus the symlink. (Using `git mv` works too in a colocated repo but is redundant since jj re-snapshots from the working tree.) Adds `TestSkillsSymlink` (parallels `internal/config/managedfiles/symlink_pluginshim_test.go`) asserting the symlink target exists. The `Taskfile.yml` `skills:validate` invocation (`go run ./internal/skillvalidate/cmd ./skills` at L295) and `internal/skillvalidate/validate_test.go`'s `TestValidateRoots_RealSkills` fixture (`"../../skills"` at L179/182) both flow through the new symlink — `os.Stat(root)` at `validate.go:58` resolves the top-level link and `filepath.WalkDir` then walks the real directory inside the package. Both paths therefore need **no edits**; the implementer should resist any urge to "fix" the test path to point at the new canonical location, because doing so would couple the test to internal package layout and defeat the validator-via-symlink invariant. No content change to any SKILL.md. |
| 2 | `feat(skills): add summary frontmatter field to all 6 SKILL.md` | Six file edits in the new location; no code change. |
| 3 | `feat(skillvalidate): require summary (≤120 chars) + enforce kebab-case name regex` | Validator + tests. Adds two new invariants: (a) `summary` is present and ≤120 chars after YAML decode; (b) `name` matches the canonical kebab-case regex (`^[a-z0-9]+(?:-[a-z0-9]+)*$`) and equals the dirname. The regex is exported as `skillvalidate.NameRegex` so downstream consumers (commit 4's `embedded.go` and commit 7's `extractSkillName`) import the same `var` — the validator owns the invariant; the resource handler and the in-binary catalog re-use it. `task check` enforces both. |
| 4 | `feat(mcp/skills): add Source interface + embedded source with eager parse + validate at constructor time` | New package; constructor parses and validates every embedded SKILL.md. Unit tests for List/Get. |
| 5 | `feat(mcp/skills): implement Search (text + regex) on embedded source` | Search method + tests. |
| 6 | `feat(mcp): add specgraph_skills_list, _get, _search tools` | New `internal/mcp/tools_skills.go` with `RegisterSkillTools(r, src)`. `internal/mcp/server.go` introduces `src, err := skills.NewEmbedded()` once at startup, treats a non-nil err as fatal, and passes `src` to the new `RegisterSkillTools(reg, src)` call. The existing `RegisterResources(reg, client)` and `RegisterPrompts(reg, client)` calls stay unchanged in this commit — `src` is only consumed by the new tool registration. Handlers depend on the `skills.Source` interface, not the concrete embedded type. |
| 7 | `feat(mcp): add specgraph://skills/<name> resource handler` | Templated handler added to `internal/mcp/resources.go` (`IsTemplate: true`); `extractSkillName` helper added. `RegisterResources` signature grows a `src skills.Source` parameter — the call site in `internal/mcp/server.go` reuses the **same** `src` introduced in commit 6 (no new `skills.NewEmbedded` call), and the `internal/mcp/resources_test.go` callers (`TestRegisterResources_Count`, `TestRegisterResources_Templates`, and any registry-construction test helper) are updated in lockstep so the commit stays green. |
| 8 | `feat(mcp): update prime template with skills pointer section` | `primeResourceHandler` signature grows from `(c *Client) ResourceHandler` to `(c *Client, src skills.Source) ResourceHandler` so prime can read the live catalog count from `src.List(ctx)`. The handler emits a `## Skills` heading + the pointer paragraph. The `RegisterResources` change in commit 7 already threaded `src` through; this commit just plumbs it into the prime closure. |
| 9 | `chore: delete plugin/specgraph/skills symlink and task plugin:sync` | Symlink + Taskfile target + any references. |
| 10 | `docs: rewrite CLAUDE.md plugin shims + shared skills sections` | CLAUDE.md, plugin/specgraph/README.md, top-level README check, and `plugin/specgraph/routing-guide.md` (add the three new tool names so the model knows about `specgraph_skills_list/_search/_get` when reading the routing guide). |

E2E test (`e2e/api/skills_test.go`) lands in commit 6 or 7 — whichever first exposes the user-facing surface it asserts.

## Documentation updates included in this PR

- `CLAUDE.md` — rewrite the "Plugin shims" paragraph to describe MCP-fetch delivery (the existing paragraph mentions `task plugin:sync` and "All three symlink `skills/`"). Rewrite the "Shared skills" paragraph to describe how the three tools and the resource URI work; the `task skills:validate` reference stays but its surrounding context updates.
- `plugin/specgraph/README.md` — replace "symlinks" language with MCP-fetch description.
- Top-level `README.md` — verify; edit if it mentions the symlink/sync pattern.
- New `internal/mcp/skills/doc.go` package comment — documents the `Source` contract, the SpecGraph-local `summary:` extension, and the future-source playbook (composite/remote).

## Deliberate departures from the parent epic

Two PR F items in `2026-05-08-spgr-rwrp-harness-install-parity-design.md` get reshaped here:

- **Prime catalog shape (parent epic §"Skills delivery" line 139).** The parent epic says prime grows "a `Skills available` catalog block listing names and descriptions". This design reduces that to a pointer-only paragraph (the three tools + the resource URI; no per-skill enumeration). Rationale, decided during brainstorming: keeping prime small forces the model to use the tool for catalog access, which is the same path every other discoverable surface in SpecGraph uses. The trade-off is one extra tool round-trip per first-skill-use; the gain is a prime digest that doesn't grow with the skill catalog over time.
- **Doctor's "Skills (N served via MCP)" line (parent epic §"PR F" line 365).** The parent epic includes this under PR F. This design defers it to PR G alongside `specgraph doctor` itself, since (a) doctor is being built fresh in PR G and (b) the line is one row in a section that doesn't exist yet. Easier to land both in the same PR than to ship a partial doctor.

## Out of scope

The following are explicitly deferred — not because they're uninteresting, but to keep PR F under review and let real usage inform the design:

- **Project-local skills source.** No `dirSource` reading `.specgraph/skills/*/SKILL.md`. The interface boundary makes this trivial to add when needed (~50 lines + tests).
- **Remote registry source.** No HTTP fetch path. Same rationale.
- **Skill versioning / pinning.** No version field in `Meta`; no per-project pin manifest.
- **Skill assets beyond `SKILL.md`.** Current skills are single-file. If a future skill ships a `references/` directory or images, the embed directive (`embedded/*/SKILL.md`) and the `Source.Get` payload shape both need to grow. See "Package layout" for the three coordinated changes that would be required.
- **Tool surface for `Fields` / `Limit`.** The `_search` tool exposes only `regex?`. The richer `SearchOptions` knobs stay in the `Source` interface — used by package-internal tests and available to future in-process callers (composite source, ranking wrapper) — but no MCP tool exposes them until evidence justifies the surface. This is a deliberate asymmetry between the Go-internal interface (extensibility-leaning) and the MCP tool surface (YAGNI-leaning); the interface knobs cost ~3 declarations and one switch arm in `embeddedSource.Search`, which we accept as cheap optionality. If a year from now the knobs have no second caller, dropping them is a one-commit cleanup.
- **Match-source attribution in `_search` results.** Search returns `[]Meta` rows but doesn't indicate *which* field matched (name vs summary vs body). A body-only match looks identical to a name match at the tool boundary, which can produce loose matches the model can't easily disambiguate without a follow-up `_get`. Adding a `MatchedFields []SearchField` field to `Meta` (or a parallel `[]MatchInfo` return) is the natural growth point if this becomes a usability problem; deferred to keep the row shape compatible with `_list` for now.
- **Claude on-disk fallback.** Per the parent design, only ship this if real user feedback shows the Claude UX regression is unacceptable. That's a chosen-change follow-up bead, not a PR-F item.

## Risks

- **Skills aren't auto-loaded in Claude anymore.** This is the deliberate trade-off the parent design accepts: Claude Code's native skill UI no longer surfaces SpecGraph skills; the model has to choose to fetch via the catalog. We accept it for cross-harness uniformity. The mitigation is the explicit pointer in prime + the search tool — both make skills *findable* even without UI cards.
- **`summary:` field churn.** Adding a required field to a non-standard YAML extension means SpecGraph forks the agentskills.io spec slightly. Risk is low (we own the only consumer); document the extension in the package comment and in `CLAUDE.md`.
- **Body-search noise.** Substring search across body text can produce loose matches (e.g., a query for "spec" matches every skill). The `SearchOptions.Fields` knob exists to mitigate, but it's Go-internal in PR F. If model-side noise becomes a real problem we expose the knob to the tool.
- **Eager-validate startup failure.** A malformed binary that passed `task check` but fails `NewEmbedded` would block server start. By construction this shouldn't happen — both the disk validator (`skillvalidate`) and the in-binary test (`TestEmbeddedFS_ListsAllSixSkills` + `TestNewEmbedded_ParsesEverySkill`) gate on the same rules. The `task check` test using the real embed FS is the load-bearing guard.
- **Rename + symlink in a jj-colocated repo.** Commit 1 packages a directory rename plus a new repo-root symlink into one working-copy snapshot. jj detects the rename from content similarity in its snapshot diff, so the author flow is plain `mv` + `ln -s` followed by `jj describe -m`; the alternative `git mv` is functionally equivalent because jj re-snapshots from the working tree anyway. Mitigation is just verification: after the snapshot, run `jj diff --summary` and confirm the output shows the six SKILL.md paths moving (not deleting + adding) and the new symlink appearing as a single entry.

## Open questions

None outstanding. The brainstorming round closed:

- Prime catalog shape — pointer only, no per-skill enumeration.
- List tool return shape — `{ name, summary, uri }`.
- `specgraph_skills_get` tool — kept, even though it overlaps with the resource URI, for tool-only MCP clients.
- Summary source — new required frontmatter field per skill.
- Validation gate — eager validate at server start.
- Storage abstraction — `Source` interface, embedded as first impl, future sources additive.
- Search — interface method with text/regex modes; tool exposes the common case.

## References

- Parent epic design: [`2026-05-08-spgr-rwrp-harness-install-parity-design.md`](2026-05-08-spgr-rwrp-harness-install-parity-design.md), §"PR F" (lines 355-371) and §"Skills delivery" (lines 132-153).
- Existing prime resource handler: [`internal/mcp/resources.go`](../../internal/mcp/resources.go), `primeResourceHandler` at L223 and `RegisterResources` at L367.
- Existing skill validator: [`internal/skillvalidate/validate.go`](../../internal/skillvalidate/validate.go).
- Predecessor design (PR E): [`2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md`](2026-05-12-spgr-rwrp-pr-e-claude-plugin-design.md).
- agentskills.io SKILL.md package format — the upstream spec our `summary:` field extends.
