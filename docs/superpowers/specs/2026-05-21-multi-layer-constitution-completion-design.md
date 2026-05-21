# Multi-Layer Constitution Completion + Prime Unification — Design Spec

**Date:** 2026-05-21
**Status:** Draft v5 (after three rounds of adversarial review + invariants/security audit)
**Bead:** spgr-8ar
**Predecessor:** [2026-04-07-layered-constitution-design.md](2026-04-07-layered-constitution-design.md)
**Goal:** Close the genuine multi-layer constitution gap (PrimeData and export do not use the merged constitution), add the affordances that make multi-layer support practically usable (remote-source import, drift sync, provenance display), retire the single-layer compatibility method, and unify the three "prime" surfaces (RPC, MCP resource, CLI) onto one composer so they stay consistent going forward.

## Context

The 2026-04-07 design landed the foundation: schema migration to `(project_slug, layer)`, layer-aware storage methods, the `internal/constitution/merge` package with provenance tracking, and `$delete` directives. Subsequent PRs wired the `ConstitutionService.GetConstitution` RPC handler to call `merge.Layers` by default, added `repeated ProvenanceEntry` to the proto response, and extended the CLI with `constitution import --layer` and `constitution show --layer`.

### Honest scope statement

The bead's six acceptance criteria are largely already met. Only item (6) — PrimeData uses merged constitution — is a genuine remaining gap. Adversarial review also revealed an unstated bug: `export.engine` silently flattens multi-layer constitutions on round-trip. A second review pass surfaced a structural problem: the three "prime" surfaces (the `ExecutionService.GetPrime` RPC, the `specgraph://prime` MCP resource, and the `specgraph prime` CLI) have drifted from each other in what content they expose and how — the CLI shows no constitution at all, the MCP resource shows a summary of the merged constitution, and the RPC returns flattened summary strings.

This spec intentionally expands the bead's scope to cover all of this in one coherent design because:

- **Coherence**: provenance plumbing through PrimeData (Piece A) is wasted unless the surfaces show it.
- **Avoiding drift**: fixing one surface in isolation creates the next round of "but the other surface still does X." The prime-unification (Piece E) closes that loop.
- **Single-source-of-truth pattern**: collapsing three independent compositions into one shared `internal/prime` composer matches the architectural commitment of "the merged constitution is the single source of truth."

If a reader feels the scope is too large, the bead can legitimately close after Piece A alone. Pieces B–E are intentional extensions, not requirements.

## Out of scope

- **Server-managed per-tenant credentials** for remote fetch. V1 uses a single optional env var.
- **Async/scheduled sync** of remote layers. V1 is on-demand only.
- **Constitution diff UI** in the web dashboard.
- **Layer-aware `constitution emit`** — continues to emit the merged composition.
- **SHA pinning resolution** — V1 stores user-supplied URL verbatim; content-hash-only drift.
- **"Overrides X" annotation** in provenance display — would require merge engine changes.
- **Token auth for git-protocol URLs** (`github.com/org/repo` shorthand, `git::https://...`) — V1 supports private GitHub only via raw HTTPS URLs.

---

## Pieces

This design ships as five PRs in dependency order. Each adds value on its own and is independently revertible.

| Piece | Title | Required by bead? | Depends on |
|---|---|---|---|
| **A** | Storage gap close + export round-trip fix | Yes (bead item 6 + correctness) | — |
| **B** | Remote-source import + sync (env-var auth) | No (net-new) | — |
| **C** | Surface provenance in `constitution show` | No (net-new) | A |
| **D** | Retire the single-layer compat method | No (net-new) | A |
| **E** | Prime unification (`internal/prime` composer) | No (net-new) | A |

Piece B is independent of A and could ship in either order. C, D, E all depend on A merging first. C, D, E are mutually independent and can ship in any order.

---

## Section 1: Storage interface evolution

Add one method to `ConstitutionBackend`:

```go
// GetMergedConstitution returns all layers composed into a single
// constitution plus per-field provenance. The single source of truth
// for "the effective constitution."
//
// Returns ErrConstitutionNotFound if no layers exist.
GetMergedConstitution(ctx context.Context) (*merge.Result, error)
```

Postgres implementation: call `GetAllLayers` then `merge.Layers`. If `GetAllLayers` returns empty, return `ErrConstitutionNotFound`.

Mark `Store.GetConstitution` (the single-layer compat method) `// Deprecated: use GetMergedConstitution`. Deletion happens in Piece D. The merge package itself does not change.

---

## Section 2: PrimeData carries provenance

**Piece A is domain-only.** Proto changes wait for Piece E to avoid duplicating the provenance field across multiple proto locations.

Update domain type:

```go
type PrimeData struct {
    Spec                   *Spec
    Decisions              []*Decision
    Constitution           *Constitution
    ConstitutionProvenance []ProvenanceEntry  // new
}

type ProvenanceEntry struct {
    Path  string
    Layer ConstitutionLayer
}
```

`GetPrimeData` calls `GetMergedConstitution` instead of `GetConstitution`. If no layers exist, `Constitution` is `nil` and `ConstitutionProvenance` is empty.

The proto `PrimeResponse` is **not** touched in Piece A. The existing summary fields (`ConstitutionSummary`, `CodingConventions`) keep populating from the merged constitution as today — they continue to reflect the highest-precedence layer's name in the `(<name> layer)` substring, which is acceptable for backward compat. Provenance is added to the proto only via Piece E's structured `ProjectView`/`SpecView` (Section 10), which is the single location.

This avoids the "three places to store the same data" problem that would result from adding `repeated ProvenanceEntry constitution_provenance` at the top of `PrimeResponse` while Piece E also adds it to the views.

---

## Section 3: Export round-trip fix

`internal/export/engine.go` today reads `*storage.Constitution` via `GetConstitution` and writes it back via `UpdateConstitution`. With multi-layer support, this collapses multi-layer projects to one layer on round-trip. Fix:

### Schema version bump: 1 → 2

```go
// internal/export/schema.go
const CurrentSchemaVersion = 2
```

### Document shape change

```go
type Data struct {
    // ... other fields ...

    // Constitution is the v1 single-layer field. Kept for legacy import only.
    // Always nil in v2-emitted documents (omitempty handles serialization).
    Constitution *storage.Constitution `json:"constitution,omitempty"`

    // Constitutions is the v2 list of layers in precedence order.
    Constitutions []*storage.Constitution `json:"constitutions,omitempty"`
}
```

Both fields with `omitempty` so v1 docs parse into `Constitution` and v2 docs parse into `Constitutions`; export only emits one (always `Constitutions` from v2 onwards).

### Export path

`Engine.collect` switches from `GetConstitution(ctx)` to `GetAllLayers(ctx)`. Layers are emitted in precedence order (user, org, project, domain) — already guaranteed by `GetAllLayers`'s ORDER BY (verified: `internal/storage/postgres/constitution.go:100-148`).

### Import path

```go
// 2. Constitutions — handle schema v1 (single) and v2 (list) with strict
// field/version validation to prevent silent data loss from mismatched docs.
var layers []*storage.Constitution
switch doc.SchemaVersion {
case 1:
    if doc.Data.Constitutions != nil {
        return nil, fmt.Errorf("v1 documents must use 'constitution' field, not 'constitutions'")
    }
    if doc.Data.Constitution != nil {
        layers = []*storage.Constitution{doc.Data.Constitution}
    }
case 2:
    if doc.Data.Constitution != nil {
        return nil, fmt.Errorf("v2 documents must use 'constitutions' field, not 'constitution'")
    }
    layers = doc.Data.Constitutions
}
for _, layer := range layers {
    if _, err := e.backend.UpdateConstitution(ctx, layer); err != nil {
        return nil, fmt.Errorf("update constitution layer %s: %w", layer.Layer, err)
    }
    res.Constitution++
}
```

The cross-field validation prevents a document that declares one schema version but populates the other version's field from silently importing wrong (or worse, losing data because the populated field is ignored by the branch).

### Forward-compatibility framing

A v1 document holds at most one layer (the schema only supported one). Upgrading v1 → v2 preserves whatever was there; it cannot recover layers that v1 could not express. The spec is "forward-compatible for the data v1 could hold," not "lossless across constitution histories." Test fixtures should make this explicit.

### Migration test

Required: integration test that constructs a v1 document by hand, imports it, re-exports as v2, asserts (a) exactly one element in `Constitutions`, (b) layer/content of that element matches the v1 input. Then a second test: store multiple layers via the API, export as v2, verify all layers present in precedence order.

---

## Section 4: Remote fetch via `hashicorp/go-getter`

### Library version

Pin `github.com/hashicorp/go-getter v1` (latest stable; v2 is alpha). Pin in `go.mod`.

**Verification gate**: before implementing Section 4, confirm the specific go-getter v1 API knobs used below (`Client.Decompressors`, `Client.Getters`, `Client.HttpClient`, `getter.ClientModeFile`). If any API differs from this spec, file a small spec amendment before writing the package.

### Security posture

In `internal/constitution/fetch`:

- `Client.Mode = ClientModeFile` only — no directory mode, no archive extraction.
- `Client.Getters` set explicitly to a restricted registry: `http`, `https`, `git`, `github` only. `s3`, `gcs`, `hg` are not registered (smaller attack surface; no auth surface to design for them).
- `Client.Decompressors = nil` — disables auto-extraction of zip/tar archives.
- `Client.HttpClient` is a custom `*http.Client` with a 10-second total timeout.
- `file://` getter registered only under `//go:build testfetch`. Production builds reject `file://` URLs at scheme-validation time.
- Body size cap enforced via `os.Stat` after fetch: reject if > 1 MiB with `CodeInvalidArgument`.

### Auth model — env-var default-public with host allow-list

V1 supports private GitHub **only via raw HTTPS URLs**, with explicit host scoping to prevent token leakage.

- **Default**: anonymous fetch. Public sources work without configuration.
- **Optional GitHub token**: `SPECGRAPH_FETCH_GITHUB_TOKEN` env var on the server.
- **Host allow-list**: token is injected **only** when the request host matches one of:
  - `raw.githubusercontent.com`
  - `api.github.com`
  - `*.githubusercontent.com` (covers any future raw subdomains)
- **Other hosts**: token is **never** injected. Includes `github.com` (shorthand → git protocol), `git::*` URLs, GitLab, Bitbucket, custom domains. Going through go-getter's git path means git's credential helpers handle auth (or fail with anonymous).
- **Cross-host redirect protection**: the custom `*http.Client.CheckRedirect` strips the `Authorization` header whenever a redirect crosses to a host outside the allow-list. Go's `net/http` does NOT do this automatically for `Authorization` headers (only for some Auth schemes), so this must be done manually.
- **Per-request host check**: the custom transport's `RoundTrip` re-checks the request host on every call (including post-redirect) and adds the header only when the host is in the allow-list. Defense-in-depth alongside the CheckRedirect.
- **Logging**: token value is never logged. Logs include URL host only. A test asserts log output excludes the token bytes verbatim.
- **Future**: server-managed per-tenant credentials and git-protocol auth deferred to follow-up beads.

Sample transport (illustrative — not a literal copy):

```go
type tokenTransport struct {
    base    http.RoundTripper
    token   string
    hosts   map[string]bool
}

func (t *tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
    if t.token != "" && t.hostAllowed(r.URL.Host) {
        r.Header.Set("Authorization", "Bearer "+t.token)
    }
    return t.base.RoundTrip(r)
}

func (t *tokenTransport) hostAllowed(host string) bool {
    if t.hosts[host] { return true }
    if strings.HasSuffix(host, ".githubusercontent.com") { return true }
    return false
}

// CheckRedirect on the client:
func stripAuthOnCrossHost(req *http.Request, via []*http.Request) error {
    if len(via) > 0 && req.URL.Host != via[len(via)-1].URL.Host {
        req.Header.Del("Authorization")
    }
    return nil
}
```

### URL syntax (Terraform-compatible subset)

| Source | Example | V1 support | Auth in V1 |
|---|---|---|---|
| HTTPS raw | `https://example.com/constitution.yaml` | ✓ | Anonymous + token (if host is github) |
| GitHub raw | `https://raw.githubusercontent.com/org/repo/ref/path` | ✓ | Anonymous + token via Authorization header |
| GitHub shorthand | `github.com/org/repo//constitution.yaml?ref=v1.2.3` | ✓ | Anonymous only (public repos) |
| Explicit git over HTTPS | `git::https://gitlab.example.com/org/repo.git//path?ref=main` | ✓ | Anonymous only (V1) |
| `git@host:` SSH | — | rejected | n/a |
| `file://` | — | tests only | n/a |
| `s3://`, `gcs://`, `hg::...` | — | not registered | n/a |

Unsupported schemes are rejected at parse time with `CodeInvalidArgument`.

### `internal/constitution/fetch` package

```go
package fetch

type Fetched struct {
    Body        []byte
    ResolvedURL string // user-supplied, verbatim
}

// Fetch retrieves a constitution file from the given URL.
// Auth (when applicable) is injected internally based on host + env var.
func Fetch(ctx context.Context, url string) (*Fetched, error) { ... }
```

The fetcher does **not** compute a hash. Hashing operates on the parsed domain struct (Section 5), so the fetch layer's job ends at returning raw bytes.

### URL credential sanitization

URLs with embedded credentials (userinfo in the authority component per RFC 3986 — e.g., `https://token@raw.githubusercontent.com/...`, `https://user:pass@example.com/...`) are **rejected at parse time** with `CodeInvalidArgument`. Common token-bearing query parameters (`token`, `access_token`, `api_key`, `password`) are also rejected. Rationale:

- Stored `source_url` is opaque text; credentials in it would persist in the database and surface in logs, error messages, and CLI output.
- The auth model is `SPECGRAPH_FETCH_GITHUB_TOKEN` env var (Section 4 above), not in-URL credentials. Allowing both creates two parallel auth surfaces and confuses the security boundary.
- The rejection is at parse time in `internal/constitution/fetch`, before any storage write or log line.

Error message names the env var as the supported alternative: `"URL contains embedded credentials; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access"`.

---

## Section 5: Canonical hashing — post-parse, on the domain struct

The first draft of this section proposed canonicalizing the *raw fetched bytes* through a separate YAML library. Adversarial review showed this would introduce a second YAML parser alongside the existing `gopkg.in/yaml.v3` loader, creating a parse/hash divergence risk and an unnecessary dependency. The revised design hashes the **parsed domain struct**, not the raw bytes.

### Flow

```text
upstream YAML/JSON bytes
  → parse via existing YAML loader (gopkg.in/yaml.v3, same path as `constitution import <file>`)
  → convert to *storage.Constitution domain struct
  → marshal to canonical JSON (encoding/json sorts map keys at every level recursively)
  → Murmur3-128 hash of the canonical JSON bytes
```

### Properties

- **Comment-resilient**: YAML comments are dropped by the parser. A comment-only upstream change produces equal hash. Correct for drift detection.
- **Whitespace-resilient**: same reasoning.
- **Key-order-resilient**: encoding/json sorts map keys; struct field order is determined by Go declaration order, which is stable across runs.
- **List-order-sensitive**: list reordering in the source produces a different hash. Intentional: constitution lists are semantically ordered (principle precedence, language allow-lists).
- **Type-coercion-tolerant**: YAML `1` vs `1.0` both decode to the same Go field type. Hashes match. Matches user intent.

### Algorithm

```go
// internal/constitution/hash/hash.go

// Hash computes the canonical hash of a Constitution domain struct.
// Returns hex-encoded Murmur3-128.
func Hash(c *storage.Constitution) (string, error) {
    canonical, err := canonicalJSON(c)
    if err != nil {
        return "", err
    }
    sum := murmur3.Sum128(canonical)
    return hex.EncodeToString(sum[:]), nil
}

// canonicalJSON marshals a Constitution with sorted map keys at every level.
// encoding/json sorts top-level map keys but for embedded interface{} maps we
// walk the structure manually before marshal.
func canonicalJSON(c *storage.Constitution) ([]byte, error) { ... }
```

### Shared dependency

Existing `Spec.ContentHash` uses `spaolacci/murmur3 v1.1.0` (verified in `go.mod`). The constitution hash module imports the same library. No new YAML dependency required.

### Source YAML compatibility note

Both the loader (`internal/config/config.go`) and the hasher operate on the same Go struct types, so any YAML extension supported by the loader (e.g., custom `UnmarshalYAML` for principles) flows through unchanged. There is no risk of "loader and hasher disagree on the meaning of a YAML construct" because they share the same parse step.

### Determinism assumption

`encoding/json.Marshal` sorts keys for `map[string]T` (and types whose keys implement `encoding.TextMarshaler`). All map fields in `*storage.Constitution` today are `map[string]string` (`Frameworks`, `Infrastructure`, `APIStandards`, `Data`, `ForbiddenReasons`), which satisfies this. Struct fields emit in declaration order, which is stable per Go's spec.

**Maintenance invariant**: if a future change adds a `map[K]V` field to `*storage.Constitution` where `K` is not `string` (or a text-marshalable type), the canonicalizer breaks silently. Either reject such fields at code review, or update `canonicalJSON` to walk the structure manually. The Go-version-stability test in CI catches regressions in existing fields but won't catch this proactively.

### Tested invariants

- Hash stable across runs (run-twice test in CI)
- Hash stable across whitespace and key-order in source YAML
- Hash stable across YAML comments
- Hash differs across list-order changes
- Hash differs when any semantic field differs
- Hash collision smoke (basic check, not a security claim)
- **Fixed-expected-hex test**: a sentinel Constitution struct hashes to a hard-coded expected value, asserting Go-version and encoding/json-version stability

---

## Section 6: `RefreshConstitutionLayer` RPC

```protobuf
service ConstitutionService {
    rpc RefreshConstitutionLayer(RefreshConstitutionLayerRequest)
        returns (RefreshConstitutionLayerResponse);
}

message RefreshConstitutionLayerRequest {
    ConstitutionLayer layer = 1; // required, non-UNSPECIFIED
    string source_url = 2;       // required
    bool dry_run = 3;
}

message RefreshConstitutionLayerResponse {
    Constitution before = 1;          // nil if no prior layer
    Constitution after = 2;           // newly fetched + parsed layer
    string previous_source_hash = 3;  // "" if no prior layer
    string new_source_hash = 4;
    bool changed = 5;                 // false iff hashes match (no write)
}
```

### Handler behavior

1. Validate `layer` is one of `user|org|project|domain`. Reject `UNSPECIFIED` and unknown values with `CodeInvalidArgument`.
2. Validate `source_url` scheme is in the allow-list. Reject with `CodeInvalidArgument` if not.
3. Call `fetch.Fetch(ctx, source_url)`.
4. Parse body via the YAML→domain helper (extracted into `internal/constitution/load` from the existing CLI path — see below).
5. Compute `new_source_hash` via `hash.Hash` on the parsed domain struct.
6. Load existing layer via `GetConstitutionLayer`. If hashes match: `changed=false`, return existing as `before`/`after`, no write.
7. If different (or no prior layer): set `changed=true`.
8. If `dry_run`: return without writing.
9. Otherwise: set `source_url` and `source_hash` on the parsed struct, call `UpdateConstitution`. (Verified: `UpdateConstitution` UPSERTs both fields at `internal/storage/postgres/constitution.go:198-207`.)

### YAML→domain helper relocation

Today the YAML parse path (`config.LoadConstitutionConfig` → `constitutionConfigToProto` → handler converts proto → domain) is split between `internal/config/` and `cmd/specgraph/`. For Piece B, the server side needs the same parse without round-tripping through proto.

Refactor as part of Piece B opener (1–2 commits before the RPC handler):

- Move `constitutionConfigToProto` and `constitutionFromProto` into a new `internal/constitution/load` package, exposing `LoadFromYAML(bytes []byte) (*storage.Constitution, error)` that returns the domain struct directly.
- CLI `constitution import` switches to call this same helper.
- Server-side RPC handler calls the same helper.

Single source of YAML parsing logic across the codebase.

---

## Section 7: CLI surface

```text
specgraph constitution import <path>                          # local file (existing)
specgraph constitution import --from-url <url> --layer <X>    # remote (new)
specgraph constitution sync [--layer X] [--dry-run] [--check]
specgraph constitution show [--layer X] [--show-provenance]
specgraph constitution emit ...                               # unchanged
```

(Prime CLI changes are in Section 10 — Piece E.)

### `constitution import --from-url`

Required: `--from-url <url>`, `--layer <user|org|project|domain>`. Calls `RefreshConstitutionLayer(layer, url, dry_run=false)`.

### `constitution sync`

Iterates the **layers actually present** (via `GetAllLayers`), not all four enum values. Calls `RefreshConstitutionLayer` for those with non-empty `source_url`. Layers without `source_url` are mentioned in the output as "no remote source" so users can see the full picture.

Example output (project has `org` and `project` layers, only `org` has a source):

```text
$ specgraph constitution sync
org      unchanged       (sha 3f2a8b...)
project  no remote source

1 of 1 remote layers checked, 0 updated.
```

Example where `org` drifted:

```text
$ specgraph constitution sync
org      changed         (sha 3f2a8b... → a8b29f...)
project  no remote source

1 of 1 remote layers checked, 1 updated.
```

Edge cases (each prints a clarifying line, exits 0):

- No layers exist: `"no constitution found; nothing to sync"`
- No layers have `source_url`: `"no remote layers configured; nothing to sync"`
- `--layer X` with no `source_url`: explicit error `CodeFailedPrecondition`

### Exit codes — `--check` flag

- **Default**: exit 0 for "ran successfully," regardless of drift state.
- **`--check`**: opt-in. Exit 1 if any layer changed or, with `--dry-run`, would change. For CI use.
- Exit 2 on hard failures (fetch error, parse error, RPC error). One bad layer does not abort the rest; other layers still attempted.

Flag name `--check` chosen over `--exit-code-on-drift` for brevity and familiarity (matches `gofmt -d -e`-style conventions). Documented in `--help` as "exit 1 if drift detected; useful in CI."

---

## Section 8: Drift semantics

V1 detects drift via **content-hash comparison only**:

1. `import --from-url` fetches body, parses, hashes via `hash.Hash` (Section 5), stores `source_hash` and `source_url` (verbatim).
2. `sync` re-fetches from `source_url`, re-parses, re-hashes, compares.
3. Hashes differ → drift detected.

### Data preservation on failure

**Fetch failures never modify or delete the existing layer.** If the upstream is unreachable, returns a 5xx, returns a parse-failing body, or fails any pre-flight check, `sync` reports the error for that layer and continues to the next. The previously-stored layer remains intact with its existing `source_hash` and `source_url`. Users explicitly delete a layer via a separate operation (currently: storage-level only; no CLI yet).

This means `sync` can be safely run in a CI cron without risk of "the upstream went down so my project lost its org layer."

### Known limitation: mutable refs

Storing the URL verbatim means `?ref=main` is re-resolved on every `sync`. If the upstream branch moves without content change, we won't detect the ref move (we'd see the new content correctly; we just won't surface "the ref pointer changed"). For most users this is fine — content drift is what matters.

**Git tags are also mutable by default**. `git push --force` can move them. The workaround "pin to a tag for immutability" is only valid if the upstream repo enforces tag protection (branch/tag protection rules, signed-tag enforcement, etc.). For real immutability, pin to a commit SHA: `?ref=abc1234...`. Documented in the CLI help.

SHA pinning resolution (rewriting `?ref=main` → `?ref=<sha>` at fetch time so the stored URL is sha-pinned) is a follow-up bead.

---

## Section 9: Surface provenance in `constitution show` — Piece C

### Renderer signature

The existing `internal/render/constitution.go:Constitution(c *specv1.Constitution) string` stays unchanged. Add a sibling function:

```go
// ConstitutionWithProvenance renders the same as Constitution but annotates
// each field with the layer that set it. provenance may be nil/empty, in
// which case the output is identical to Constitution.
func ConstitutionWithProvenance(c *specv1.Constitution, provenance []*specv1.ProvenanceEntry) string
```

No breaking change. Call sites that don't care continue to use `Constitution`. The CLI `--show-provenance` path calls the new function.

### Text mode

```text
Principles:
  p1: Prefer explicit over implicit                       (set by: project)
  p2: Tests are integration tests by default              (set by: org)

Tech > Languages:
  Primary:   go                                           (set by: project)
  Allowed:   [go, sql]                                    (set by: org)
  Forbidden: [javascript]                                 (set by: domain)
```

### JSON mode

`--json --show-provenance` emits a `provenance` sibling array:

```json
{
  "constitution": { ... },
  "provenance": [
    {"path": "principles[p1]", "layer": "project"},
    {"path": "principles[p2]", "layer": "org"}
  ]
}
```

Without `--show-provenance`, output is unchanged from today (no provenance section, no breaking change to scripts).

### Deferred

"Overrides X" annotations (`p3: ... (set by: project, overrides org)`) require expanding `merge.Result.Provenance` to track override chains across input layers. Currently only the winning layer is recorded. Out of scope.

---

## Section 10: Prime unification — Piece E

### Problem

Three "prime" surfaces today:

| Surface | What it does today |
|---|---|
| `ExecutionService.GetPrime` RPC (per-spec) | Returns flattened summary strings (`ConstitutionSummary`, `CodingConventions`). Used by polecats. |
| `specgraph://prime` MCP resource (project-level) | Calls merged `GetConstitution`. Renders markdown with constitution top constraints, graph overview, ready specs, findings, skills pointer. |
| `specgraph prime` CLI (project-level) | Calls `ListSpecs` and prints active specs as a table. **Does not show constitution.** |

The three are independently implemented and have drifted in what content they expose and how. Adding constitution provenance to one of them creates new drift unless we collapse them onto a shared composer.

### Design

**Two scopes:**

- **Project prime** — orient to a project (no specific spec).
- **Spec prime** — orient to executing a specific spec.

**One shared composer** in a new `internal/prime` package:

```go
package prime

type ProjectView struct {
    Constitution           *storage.Constitution
    ConstitutionProvenance []storage.ProvenanceEntry
    GraphOverview          GraphOverview  // counts by stage
    Ready                  []*storage.Spec  // top 10
    FindingsBySeverity     map[storage.FindingSeverity]int
    SkillsCount            int
}

type SpecView struct {
    Spec                   *storage.Spec
    Constitution           *storage.Constitution
    ConstitutionProvenance []storage.ProvenanceEntry
    Decisions              []*storage.Decision
    Slices                 []*storage.Slice
    Claims                 []*storage.Claim
    Blockers               []*storage.Blocker
}

func Project(ctx context.Context, b Backend) (*ProjectView, error)
func Spec(ctx context.Context, b Backend, slug string) (*SpecView, error)
```

### Backend interface — explicit aggregate

The composer's `Backend` is a wide aggregate, similar to `internal/export.Backend`:

```go
type Backend interface {
    storage.ConstitutionBackend  // GetMergedConstitution
    storage.Backend              // ListSpecs
    storage.GraphBackend         // GetReady
    storage.FindingsBackend      // ListProjectFindings
    storage.ExecutionBackend     // GetPrimeData
    storage.SliceBackend         // ListSlices

    // SkillsCount is unusual — skills live in MCP-server-side state, not
    // storage. The composer takes a `skills.Source` separately as a
    // constructor dependency, not part of the storage backend.
}

// Composer wraps a Backend + skills.Source for use by all three surfaces.
type Composer struct {
    backend storage.Backend // the wide aggregate above
    skills  skills.Source
}

func New(backend Backend, src skills.Source) *Composer { ... }
func (c *Composer) Project(ctx context.Context) (*ProjectView, error) { ... }
func (c *Composer) Spec(ctx context.Context, slug string) (*SpecView, error) { ... }
```

Chosen over the alternative "composer takes pre-built input structs" because that approach pushes orchestration burden back onto every surface, defeating the unification goal. The wide aggregate interface is the same pattern `internal/export` uses today; it's deliberate and ugly but consistent with project conventions.

Tests stub the wide interface. We will likely extract a `testbackend` helper in `internal/prime/internal/testbackend` to reduce boilerplate.

The composer queries each piece via the backend and assembles the view. It does not render — renderers are surface-specific.

### Surface mapping

| Concept | RPC | MCP Resource | CLI |
|---|---|---|---|
| Project prime | `GetPrime(slug="")` | `specgraph://prime` | `specgraph prime` |
| Spec prime | `GetPrime(slug=X)` | `specgraph://prime/spec/{slug}` | `specgraph prime <slug>` |

### Uniform options

| Option | CLI flag | MCP URI param | RPC field |
|---|---|---|---|
| Show provenance | `--show-provenance` | `?provenance=true` | `bool show_provenance` |
| Output format | `--json` | `?format=json` (default markdown) | always structured proto |

### Proto changes

The existing `PrimeResponse` keeps backward-compat summary fields. New typed fields added:

```protobuf
message PrimeResponse {
    // Existing summary fields — kept for backward compat with old polecats.
    string constitution_summary = 1;
    string project_context = 2;
    repeated Decision decisions = 3;
    string coding_conventions = 4;
    string callback_docs = 5;

    // New structured fields. Exactly one is populated depending on whether
    // the request specified a slug.
    oneof view {
        ProjectView project_view = 10;
        SpecView spec_view = 11;
    }
}
```

Both `ProjectView` and `SpecView` carry their own `repeated ProvenanceEntry constitution_provenance`. **This is the only place provenance lives in the proto** — the top-level `PrimeResponse` does not duplicate it.

`GetPrimeRequest.slug` becomes optional (today it rejects empty with `CodeInvalidArgument`; new behavior: empty slug returns project view, non-empty returns spec view). This is a backward-compat-friendly relaxation; existing callers either pass a slug (no change) or hit the error today (and would get success going forward — unlikely to be relied upon).

### Legacy summary fields — populated only for spec scope

The summary fields (`ConstitutionSummary`, `ProjectContext`, `CodingConventions`, `CallbackDocs`) only make semantic sense for spec scope (`ProjectContext = spec.Intent`, `CallbackDocs` describes report-progress RPCs for that spec). For project scope, these fields are **left zero**; the new `project_view` oneof carries the project-level data. Old polecats that issue a slug-less call (which they never did, since today's handler rejects empty slug) would get empty summary fields plus the new structured view — they ignore the new view, see empty summaries, and that's fine because no existing polecat code path produces a slug-less call.

### MCP URI parsing

The MCP resource handler dispatches based on path suffix:

- `specgraph://prime` → project view
- `specgraph://prime/spec/{slug}` → spec view (slug parsed from URI)

Query params (`?provenance=true`, `?format=json`) parsed via `net/url`. Defaults: provenance off, markdown format.

### CLI surface

```text
specgraph prime [--show-provenance] [--json]              # project view
specgraph prime <slug> [--show-provenance] [--json]       # spec view
```

The existing prime behavior (active specs table) becomes part of the project view's terminal renderer, joined by constitution summary and the other sections.

### Implementation order within Piece E

1. Create `internal/prime` package with `ProjectView`, `SpecView`, `Composer.Project`, `Composer.Spec`. Unit tests with stub backend.
2. Update `ExecutionService.GetPrime` handler to call `Composer.Project` or `Composer.Spec` based on slug. Populate legacy summary fields **only for spec scope**. Populate `view` oneof in both. Existing tests keep passing.
3. Update `specgraph://prime` MCP resource handler to call `Composer.Project`; add `specgraph://prime/spec/{slug}` templated resource. Render the views to markdown. Parse `?provenance=true` and `?format=json` query params.
4. Rewrite `cmd/specgraph/prime.go` to: (a) **preserve the existing `runUp` call so SessionStart-hook ergonomics don't regress**, (b) call the `GetPrime` RPC with the slug arg (empty for project scope), (c) render the response. Add `--show-provenance` and `--json` flags. Add the `{slug}` positional argument.
5. E2E tests verify each surface contains the same known facts seeded by the test (Section 13).

---

## Section 11: Retire the single-layer compat method — Piece D

Two commits, each independently revertible:

1. **Verify no remaining callers.** After Piece A merges, `PrimeData` and `export.engine` no longer call `Store.GetConstitution`. Audit via `grep -rn "GetConstitution\b" --include="*.go"` outside test mocks and the deprecation shim.
2. **Delete the method.** Remove from `ConstitutionBackend`, Postgres impl, all test mocks. Out-of-tree consumers get a compile error and migrate to `GetMergedConstitution`. No schema change.

The contract change (single-layer → no method) is intentional: silent layer-dropping is the bug Piece A fixed; no path back.

### CI guard against regrowth

Add a CI grep assertion (in `task check` or a dedicated `task lint:constitution-callers`) that fails if `Store.GetConstitution\b` (with word boundary) appears in any non-test, non-deprecation-shim production source file after Piece D ships. The check is one shell pipeline:

```bash
# Fails CI if the deprecated method reappears in production code.
if rg -n 'Store\.GetConstitution\b' --type go --type-not test -g '!*_test.go' \
   --glob '!internal/storage/constitution_deprecated.go' .; then
    echo "ERROR: Store.GetConstitution is deprecated; use GetMergedConstitution" >&2
    exit 1
fi
```

Prevents accidental reintroduction during future refactors or merge conflicts.

---

## Section 12: Error handling

| Condition | gRPC code |
|---|---|
| Unsupported URL scheme | `CodeInvalidArgument` |
| Layer unspecified or invalid | `CodeInvalidArgument` |
| Body parse failure (YAML/JSON) | `CodeInvalidArgument` |
| Body exceeds size cap | `CodeInvalidArgument` |
| Sync requested for layer with no `source_url` | `CodeFailedPrecondition` |
| **For HTTPS URLs only**: HEAD pre-flight returns 401/403 | `CodePermissionDenied` (error message names `SPECGRAPH_FETCH_GITHUB_TOKEN` and raw-URL alternative) |
| **For HTTPS URLs only**: HEAD pre-flight returns 404 | `CodeNotFound` (see GitHub-specific hint below) |
| Any other fetch failure (network, git-protocol auth, 5xx) | `CodeUnavailable` (with wrapped go-getter error in the message) |

### GitHub 404 — likely-auth hint

GitHub deliberately returns 404 (not 401/403) for missing files in private repos when the request is unauthenticated, to avoid leaking the existence of private resources. This means the most common "I forgot to set the token" path hits the 404 branch, not the 401 branch.

When the URL host is `raw.githubusercontent.com` or `*.githubusercontent.com` AND the HEAD returns 404 AND `SPECGRAPH_FETCH_GITHUB_TOKEN` is not set, the error message is:

```text
file not found OR private repo (404 from GitHub; if the file exists in a private repo, set SPECGRAPH_FETCH_GITHUB_TOKEN)
```

The gRPC code stays `CodeNotFound` (the upstream's intentional ambiguity), but the message gives the user the right next step. When the token IS set and 404 still returns, the message is just "file not found" — auth isn't the problem.
| Hash mismatch on `--dry-run` | *not an error* (`changed=true` in response) |

### HEAD pre-flight scope

HEAD pre-flight applies only to URLs we dispatch to the HTTPS getter (raw HTTPS, `https://raw.githubusercontent.com/...`). For git-protocol URLs (`github.com/org/repo`, `git::https://...`), auth happens during the fetch protocol negotiation; we cannot reliably distinguish 401/403/404/500 without parsing error strings. All such failures map to `CodeUnavailable` with the wrapped error included for diagnostic purposes.

This is a deliberate honest narrowing. Adversarial review noted that promising precise error mapping for go-getter outputs is fragile.

---

## Section 13: Testing

### Unit — `internal/constitution/fetch`

Drive go-getter with `file://` fixtures (under `//go:build testfetch`). Cover: happy path, unsupported scheme, oversized body, token injection for raw GitHub URL, no-token 401 path, log redaction of token bytes.

### Security — `internal/constitution/fetch` (Piece B)

Explicit security checklist; each item is its own test:

- **Token allow-list**: with `SPECGRAPH_FETCH_GITHUB_TOKEN` set, fetch a non-GitHub HTTPS URL (httptest server). Inspect the captured request — assert `Authorization` header is absent.
- **Cross-host redirect**: configure httptest server to redirect from `raw.githubusercontent.com` (mocked via Host header) to `attacker.example.com`. Assert the second request to the attacker host has no `Authorization` header.
- **Body size cap**: serve a 2 MiB body. Assert fetch returns `CodeInvalidArgument` before parse, and the body is never read into memory beyond the cap.
- **Path traversal / `file://` rejection in production**: build the binary without `testfetch` tag; assert `file:///etc/passwd` URL is rejected at scheme validation with `CodeInvalidArgument`.
- **Decompressor bypass**: serve a body with `Content-Type: application/zip`. Assert no extraction happens; body is treated as raw bytes (which then fails parse as YAML/JSON with `CodeInvalidArgument`).
- **YAML bomb resistance**: serve a YAML body with a deep alias graph (the classic "billion laughs" construction). Assert parse fails quickly (yaml.v3 has limits — verify they fire rather than hang).
- **Log redaction**: capture all log output during a successful fetch with token set. Assert the literal token bytes do not appear in any log line.
- **Decompressors actually disabled**: assert `client.Decompressors` is nil after construction (defensive — guards against future code that re-enables them).

### Unit — `internal/constitution/hash`

Hash stability across runs, whitespace, key-order, comments. Hash sensitivity to list-order, scalar changes, list-element additions/removals. Fixed-expected-hex test for Go-version stability.

### Unit — `internal/constitution/load` (new)

YAML/JSON → domain struct via the relocated helper. Cover existing CLI behaviors so the refactor is no-regression. Both CLI and RPC paths share the same helper.

### Unit — `internal/prime`

Stub backend returning canned data. Cover ProjectView, SpecView; non-existent spec error; empty constitution case; empty findings; provenance flow-through.

### Unit — handlers

`RefreshConstitutionLayer`: inject fake `Fetcher`; cover all error codes from Section 12. `GetPrime`: cover legacy summary fields populated only for spec scope; new `view` oneof populated correctly for both scopes; `view` oneof has exactly one arm populated.

### Renderer — `internal/render/constitution.go` (Piece C)

- **Byte-identical legacy output**: `Constitution(c)` (no provenance) produces output equal byte-for-byte to today's behavior. Test asserts on a golden file fixture.
- **Provenance annotations**: `ConstitutionWithProvenance(c, prov)` produces text output containing `(set by: <layer>)` annotations for every field present in the provenance map. Field types covered: scalars (process), keyed lists (principles, antipatterns), string lists (constraints), nested maps (tech).
- **Empty provenance**: `ConstitutionWithProvenance(c, nil)` and `ConstitutionWithProvenance(c, []*ProvenanceEntry{})` both produce output equal to `Constitution(c)`.
- **Partial provenance**: provenance map missing some keys — annotations appear only for keys present; absent keys render without annotation, no `(set by: unknown)` placeholder.
- **JSON mode**: `--json --show-provenance` emits a top-level object with `constitution` and `provenance` sibling fields; ordering of `provenance` entries is deterministic (alphabetical by path).
- **JSON mode without provenance**: `--json` alone produces output identical to today's JSON shape (no `provenance` key, not even empty).

### CLI behavior — `cmd/specgraph/prime.go` and `constitution sync` (Piece E + Piece B)

- **`runUp` preservation**: with no server running, invoke `specgraph prime` and assert (a) `runUp` is called (server starts), (b) the prime RPC succeeds. Regression test against accidental removal during the rewrite.
- **Slug validation**: `specgraph prime <invalid-slug>` — assert the CLI either pre-validates the slug shape or surfaces the RPC's `CodeNotFound` with a clear "spec not found" message (not a leaked storage error).
- **Project vs spec scope**: `specgraph prime` (no arg) produces project-view output; `specgraph prime <existing-slug>` produces spec-view output with constitution + decisions + claims sections.
- **`--check` exit codes**: `constitution sync --check` (unchanged remote) → exit 0; same with modified remote → exit 1; with fetch failure → exit 2.
- **`--dry-run --check` combination**: dry-run does not write; `--check` reports drift via exit code 1; subsequent `constitution show` confirms no write occurred.
- **`--json` mode for prime**: `specgraph prime --json` produces well-formed JSON that round-trips through `json.Unmarshal`.

### MCP — resources (Piece E)

- **Templated URI parses**: `specgraph://prime/spec/abc-123` parses to slug `abc-123`; the resource handler is invoked.
- **Empty slug rejected**: `specgraph://prime/spec/` (trailing slash, no slug) returns a clear error.
- **Query param parsing**: `?provenance=true`, `?provenance=false`, `?provenance=` (empty), `?provenance=foo` (invalid). First two are honored; empty defaults to false; invalid returns an error or warning.
- **`?format=json`**: produces JSON content with `mimeType: application/json`; default markdown unchanged.

### Integration (`//go:build integration`)

- Export v1 → import → re-export v2: shape and content correctness (Section 3).
- Multi-layer export round-trip: all layers in precedence order.
- `RefreshConstitutionLayer` against a `file://` fixture (testfetch build tag): hash compare, dry-run, write, second sync no-op.
- `GetPrime` for project + spec scopes: structured fields populated, provenance present when requested.

### E2E (`//go:build e2e`)

- CLI: `constitution import --from-url file://...` → `constitution show --show-provenance` → `constitution sync --check` → modify fixture → `constitution sync --check` (exit 1) → `constitution sync` (writes) → `constitution show` reflects update.
- CLI: `specgraph prime` (project) and `specgraph prime {slug}` (spec) produce coherent terminal output with constitution + active specs.
- MCP: `specgraph://prime` and `specgraph://prime/spec/{slug}` resources return matching markdown for same content.
- Cross-surface fact-presence: seed a known constitution (e.g., `name: "test-org-constitution"`, a unique principle ID like `p-cross-surface-1`), then assert each of the three surfaces' output contains these literal strings. Avoids building three parsers; catches the structural regression "surface X stopped including the constitution name."

---

## Section 14: Invariants

Consolidated list of invariants the system must maintain. Each is testable; references show where coverage lives.

### Merge consistency

- All read paths converge on the merged constitution. PrimeData (Piece A), MCP resources, RPC, CLI `constitution show` no-`--layer`, export (Piece A), prime composer (Piece E) — every read path resolves the constitution via `GetMergedConstitution` or `GetConstitutionLayer`, never via `Store.GetConstitution`. After Piece D this is structurally enforced.
- **Coverage**: integration tests per read path; Piece D CI grep guard.

### Provenance ↔ Constitution coupling

- `Constitution == nil` iff `ConstitutionProvenance` is empty in any of: `PrimeData`, `ProjectView`, `SpecView`.
- **Coverage**: unit tests for `GetPrimeData` and `internal/prime`.

### Source URL ↔ source_hash coupling

- If `source_url` is empty, `source_hash` is empty.
- If `source_url` is non-empty, `source_hash` matches the canonical hash of the constitution last fetched from that URL.
- **Coverage**: integration test for `RefreshConstitutionLayer` happy path and re-fetch idempotency.

### Hash determinism

- Two semantically equivalent YAML inputs (whitespace, comments, key order) produce equal hash.
- List reordering produces a different hash (intentional).
- **Coverage**: unit tests in `internal/constitution/hash` + fixed-expected-hex test.

### Fetch idempotency

- `fetch.Fetch(ctx, url)` called twice on an unchanged upstream returns equal canonical hashes.
- **Coverage**: integration test issues two consecutive `RefreshConstitutionLayer` calls; second has `changed=false`.

### Token containment

- Token bytes never appear in: log output, error messages, the stored `source_url` value, or HTTP requests to non-allow-list hosts.
- **Coverage**: Security tests in Section 13 (token allow-list, cross-host redirect, log redaction).

### URL credential containment

- URLs with embedded credentials (userinfo per RFC 3986, or common token query parameters like `token`/`access_token`/`api_key`/`password`) are rejected at parse time in `internal/constitution/fetch` with `CodeInvalidArgument`.
- No URL containing credentials is ever stored as `source_url` or written to a log line.
- **Coverage**: Section 13 fetch unit tests must include cases for `https://token@host/path`, `https://user:pass@host/path`, and `https://host/path?token=secret` — all rejected with explicit error referencing `SPECGRAPH_FETCH_GITHUB_TOKEN`.

### Data preservation on failure

- Fetch failure → existing layer unchanged.
- Parse failure → existing layer unchanged.
- Hash mismatch on `--dry-run` → existing layer unchanged.
- **Coverage**: integration test injects fetch error; pre-sync state == post-sync state.

### Concurrency

- Two concurrent `RefreshConstitutionLayer` calls for the same layer: existing version guard on `UpdateConstitution` ensures first-writer-wins; second returns `CodeAborted` (per ADR-004 / transactions PR pattern).
- Two concurrent calls for different layers: independent rows, no conflict.
- **Coverage**: integration test for both cases.

### v1 ↔ v2 export

- A v1 export document imports under v2 code; resulting state contains one layer matching the v1 content.
- A v2 export contains all layers in precedence order (user → org → project → domain).
- `schema_version: 1` + populated `constitutions` → explicit error.
- `schema_version: 2` + populated `constitution` → explicit error.
- **Coverage**: integration tests for forward migration + cross-field validation.

### `view` oneof

- For any `PrimeResponse`, exactly one of `project_view` and `spec_view` is populated.
- Legacy summary fields populated iff `spec_view` is populated.
- **Coverage**: unit test on `GetPrime` handler for both scopes.

### CLI byte-stability (backward compat)

- `constitution show` (no `--show-provenance`, no `--layer`) produces byte-identical output to today.
- `constitution sync` (no `--check`) exits 0 on successful run regardless of drift state.
- `constitution emit` output unchanged from today.
- `specgraph prime` output **changes** by addition (constitution section added) but the existing active-specs section is preserved.
- **Coverage**: golden-file tests for `constitution show`; e2e shell test for sync exit codes; golden-file tests for prime active-specs section.

### Renderer behavior

- `ConstitutionWithProvenance(c, nil)` ≡ `Constitution(c)` for all `c`.
- JSON mode without `--show-provenance` produces today's JSON shape (no `provenance` key).
- **Coverage**: Section 13 Renderer subsection.

### Source-URL allow-list

- Only `http`, `https`, `git`, `github` getters are registered in production builds.
- `file://` registered only under `//go:build testfetch`.
- `s3`, `gcs`, `hg` never registered.
- **Coverage**: build-tag tests for production vs testfetch builds.

---

## Open questions

- **Should `specgraph://prime/spec/{slug}` be a templated resource or a static one?** Templated keeps the catalog clean and matches `specgraph://skills/{name}` precedent.
- **Two layers with the same `source_url`** — V1 allows it; sync runs both. Likely rare; flag as a `bd doctor` future improvement.
- **What does the existing `GetPrime` `CallbackDocs` field do for project scope?** Spec says "left zero." Reconfirm during implementation that no existing polecat depends on this being non-empty for any path.

---

## References

- Predecessor: [2026-04-07-layered-constitution-design.md](2026-04-07-layered-constitution-design.md)
- Bead: `spgr-8ar`
- Related (incoming auth, not outgoing): `spgr-5wb` (Auth & Authorization epic)
- Library: [hashicorp/go-getter v1](https://github.com/hashicorp/go-getter)
- Hash library (in use): [spaolacci/murmur3 v1.1.0](https://github.com/spaolacci/murmur3)
- YAML loader (in use): [gopkg.in/yaml.v3 v3.0.1](https://gopkg.in/yaml.v3) (same parser used by hasher post-parse)
