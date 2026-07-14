# Phase 6: MCP Authoring Self-Teaching Path - Pattern Map

**Mapped:** 2026-07-14
**Files analyzed:** 15 (5 handler/code, 7 SKILL.md content, 3 test)
**Analogs found:** 15 / 15 (all in-repo — this is a modification-heavy phase; every net-new artifact has a strong sibling analog)

> Source of the file list: RESEARCH.md § "Code Map" (authoritative, line-referenced at HEAD) + CONTEXT.md § Canonical References. No proto changes (`task proto` NOT required — confirmed RESEARCH.md §5).

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/mcp/tools_core.go` (`constitution.handleUpdate` + `def`) | MCP tool handler | transform (friendly-YAML → proto → RPC) | `handleGet` (same file, already friendly) + `internal/constitution/load/load.go` | exact |
| `internal/mcp/tools_authoring.go` (`author.handleSpark/Shape/Specify/Decompose` + `def`) | MCP tool handler | transform (friendly-YAML → proto → RPC) | `constitution.handleUpdate` (post-fix) + `constitutionLayerFromString` | exact (role+flow) |
| **NEW** `internal/authoring/load/load.go` (+ `*_test.go`) | parse/transform package | transform (friendly YAML → stage proto) | `internal/constitution/load/load.go` | exact (direct template) |
| `internal/mcp/skills/embedded/specgraph-constitution/SKILL.md` | content (skill) | doc | `specgraph-authoring/SKILL.md` (already MCP-first) | exact |
| `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` | content (skill) | doc | itself (align to chosen write format) | exact |
| 5× other `SKILL.md` (graph-query, analytical-passes, drift, conventions, troubleshooting) | content (skill) | doc | `specgraph-authoring/SKILL.md` voice | exact |
| `internal/render/prime.go` (`writeSkills`, `writeProjectConstitution` empty-state) | render (markdown) | transform (proto → markdown) | existing `writeSkills` / `writeProjectConstitution` (same file) | exact |
| `internal/mcp/resources.go` (`constitutionEmptyResource`) | MCP resource | request-response | same func (empty-state text) | exact |
| **NEW** `e2e/api/mcp_only_authoring_test.go` | test (e2e Ginkgo) | request-response (MCP client only) | `e2e/api/skills_test.go` (`skillsMCPClient`) + `prime_cross_surface_test.go` | exact |
| **NEW/EXTEND** skills content-drift assert (`internal/mcp/skills/*_test.go` or extend `internal/authoring/drift_test.go`) | test (unit content) | transform (content ↔ proto/tool-name) | `internal/authoring/drift_test.go` (`TestContentProtoDrift`) | exact |

---

## Pattern Assignments

### `internal/mcp/tools_core.go` — `constitution.handleUpdate` rewrite (the #1002 defect site)

**Analogs:** `handleGet` (same file, lines 87–103 — already applies the friendly mapper) and `internal/constitution/load/load.go` (`FromYAML`→`ToProto`, the exact CLI-import pipeline).

**The defect to replace** (`tools_core.go:105–121`):

```go
func (t *constitutionTool) handleUpdate(ctx context.Context, params map[string]any) (*ToolResult, error) {
	raw := stringParam(params, "constitution")
	if raw == "" {
		return errResult("constitution is required for update (pass the full JSON from get, modified as needed)"), nil
	}
	var c specv1.Constitution
	if err := protojson.Unmarshal([]byte(raw), &c); err != nil {   // ← THE DEFECT: requires CONSTITUTION_LAYER_PROJECT etc.
		return errResult(fmt.Sprintf("invalid constitution JSON: %v", err)), nil
	}
	resp, err := t.client.Constitution.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &c,
	}))
	...
}
```

**Reuse pipeline** (already shipping, `internal/constitution/load/load.go:22` + `:38`):

```go
func FromYAML(data []byte) (*storage.Constitution, error) { /* config.ParseConstitutionConfig → cc.ToDomain() */ }
func ToProto(c *storage.Constitution) *specv1.Constitution { /* domain → *specv1.Constitution, with layerToProto / referenceTypeToProto */ }
```

**Replacement pattern** (planner: ~10 lines, no proto regen):

```go
con, err := load.FromYAML([]byte(raw))   // accepts friendly `layer: project`, `type: adr` (and JSON — YAML superset)
if err != nil {
	return errResult(fmt.Sprintf("invalid constitution: %v", err)), nil   // house-style errResult; V5 input-validation
}
resp, err := t.client.Constitution.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
	Constitution: load.ToProto(con),
}))
```

**Enum-mapper analog** (already in file, lines 29–35 — the template for ALL friendly `*FromString` mappers this phase adds; note UNSPECIFIED→error, the correctness fix):

```go
func constitutionLayerFromString(s string) specv1.ConstitutionLayer {
	key := "CONSTITUTION_LAYER_" + strings.ToUpper(s)
	if v, ok := specv1.ConstitutionLayer_value[key]; ok {
		return specv1.ConstitutionLayer(v)
	}
	return specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED
}
```

**`def()` teaching-surface rewrite** (lines 54–73 — criterion #4; tool `Description` + param doc are agent-facing). Replace the `"Full constitution JSON for update (output from get…)"` param doc with an inline friendly-YAML shape description. Keep the `objectSchema(props{…}, "action")` structure verbatim; only strings change.

---

### `internal/mcp/tools_authoring.go` — `author.{spark,shape,specify,decompose}` rewrite (same defect ×4)

**Analog:** `constitution.handleUpdate` (post-fix, above) for the flow; `constitutionLayerFromString` for the two new enum mappers.

**The defect to replace** (repeated at lines 178–181, 206–209, 239–242, 272–275 — one per stage), e.g. `handleSpark`:

```go
var out specv1.SparkOutput
if err := protojson.Unmarshal([]byte(raw), &out); err != nil {   // ← same defect, ×4
	return errResult(fmt.Sprintf("invalid spark output JSON: %v", err)), nil
}
```

**Pattern to copy** — parse friendly YAML `output` via the new `internal/authoring/load` package, keeping the surrounding posture/exchanges handling identical (`validateOptionalPosture` lines 85–95, `parseOptionalExchanges` lines 70–81 stay AS-IS):

```go
out, err := authload.SparkFromYAML([]byte(raw))
if err != nil {
	return errResult(fmt.Sprintf("invalid spark output: %v", err)), nil
}
posture, posErr := validateOptionalPosture(params)   // unchanged
...
resp, err := t.client.Authoring.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
	Slug: slug, Output: out, Posture: posture,
}))
```

**`approve` needs NO change** (lines 296–308 — already just a slug, no `output`).

**`def()` rewrite** (lines 123–145): the `output` param doc `"Stage output as a JSON string…"` becomes a per-stage friendly-YAML description. Dispatch/switch (lines 147–167) unchanged.

**Existing forgiving-mapper prior art in THIS file** (`postureFromString` 98–104, `authoringStageFromString` 107–113) — the enum-mapper house style to mirror for `scopeSniffFromString` / `decompositionStrategyFromString`.

---

### **NEW** `internal/authoring/load/load.go` (+ `load_test.go`) — friendly funnel input package

**Analog (direct template):** `internal/constitution/load/load.go` — copy its shape exactly.

**Structure to replicate:**

1. **Package doc comment** (revive gate — new packages fail lint without it), mirroring `load.go:4–8`:
   ```go
   // Package load parses authoring funnel stage YAML/JSON into the stage
   // proto outputs (SparkOutput/ShapeOutput/SpecifyOutput/DecomposeOutput).
   package load
   ```
2. **`FromYAML`-style parsers** per stage: `yaml.Unmarshal` into a typed friendly struct (NOT arbitrary maps — bounds the shape, V5/DoS mitigation), then map to proto. Mirror `FromYAML` (`load.go:22–33`) + `ToProto` (`load.go:38–77`).
3. **Snake_case YAML field tags** matching proto field names (`scope_in`, `scope_out`, `success_must`, `chosen_approach`, `scope_sniff`) — keeps `TestContentProtoDrift` green (Pitfall 2). Model tags on `ConstitutionConfig` (`config.go:114–123`, e.g. `` `yaml:"api_standards,omitempty"` ``).
4. **Two enum mappers**, copied from `referenceTypeToProto` (`load.go:94–107`) / `constitutionLayerFromString`, returning `_UNSPECIFIED` on unknown so the handler errors (not silent-write):
   ```go
   func scopeSniffFromString(s string) specv1.ScopeSniff { /* "SCOPE_SNIFF_"+UPPER, ok→val, else UNSPECIFIED */ }
   func decompositionStrategyFromString(s string) specv1.DecompositionStrategy { /* strategy: single_unit → DECOMPOSITION_STRATEGY_SINGLE_UNIT */ }
   ```
   (Proto enums confirmed: `authoring.proto:70 ScopeSniff`, `:49 DecompositionStrategy`, fields `scope_sniff = 4`, `strategy = 1`.)
5. **License header** `// SPDX-License-Identifier: Apache-2.0` (mirror `load.go:1–2`; `task license:add`).
6. **Unit test** mirrors the drift/mapping style — assert friendly YAML → proto round-trip AND `*FromString` invalid → UNSPECIFIED→error (Wave 0 conditional test, RESEARCH.md §"Wave 0 Requirements").

---

### `internal/mcp/skills/embedded/specgraph-constitution/SKILL.md` — MCP-first rewrite (worst offender)

**Analog:** `specgraph-authoring/SKILL.md` (already MCP-first — the voice/structure target).

**What to remove/demote** — the CLI-first steps that cause #1002:
- `Step 1` (line 24): `specgraph constitution show` → replace with `constitution` MCP tool `action: get`.
- `Step 4 "Write and Import"` (lines 129–144): `specgraph constitution import constitution.yaml` → replace with `constitution` MCP tool `action: update` passing the SAME YAML block inline. **This is the exact #1002 fix.**
- `Step 5` (146–154): `specgraph constitution emit` → move to gated appendix.

**What to KEEP verbatim:** the **YAML schema block (lines 49–101)** — it is already the friendly format the fixed handler parses (`layer: "project"`, `type: "adr"`). It becomes *the* write payload, not a file to import.

**Front-matter contract to preserve** (lines 1–8): `name:`, `summary:`, `description:`, `license:` — the e2e (`skills_test.go:102–103`) asserts `name: specgraph-<x>` and `summary:` remain present.

**Gated CLI appendix (D-07)** — append ONE uniform section at the end, labeled so an MCP-only agent skips it. Pattern to apply to ALL 7 skills:
```markdown
---
## Requires local CLI (source/CLI users only — MCP-only agents skip this)
...preserved `specgraph constitution import` / `emit` docs...
```

---

### Other 6 SKILL.md files — MCP-first alignment

**Analog:** `specgraph-authoring/SKILL.md` (voice + "How to invoke" MCP-tool framing, lines 37–49).

Per RESEARCH.md §2 rewrite map (ordered by effort):
- `specgraph-authoring` (89 ln, 0 CLI refs) — align to friendly-YAML `output` per stage; add explicit round-trip; gated appendix.
- `specgraph-troubleshooting` (97 ln, 10 CLI) — reframe on `health` tool + resources; demote `doctor`/`health` CLI.
- `specgraph-drift` (63 ln, 3/3) — MCP-first; gated appendix.
- `specgraph-graph-query` (77 ln, 0 CLI), `specgraph-analytical-passes` (79 ln), `specgraph-conventions` (81 ln) — light audit + uniform appendix + voice pass.

**Pitfall 4 (symlink):** `skills/` (root) and `plugin/{specgraph,cursor,opencode}/` are reverse-symlinks INTO `internal/mcp/skills/embedded/` — edit the canonical path; do NOT create a second copy (`skills_symlink_test.go` guards this).

---

### `internal/render/prime.go` + `internal/mcp/resources.go` — prime routing (D-10, minimal)

**Analog:** the existing `writeSkills` (already emits the routing section).

**`writeSkills`** (prime.go:306–315) — strengthen into an explicit "start here" line for authoring. Current:
```go
fmt.Fprintf(b, "## Skills\n\n%d skills exposed via MCP. ", count)
b.WriteString("Use `specgraph_skills_list` to see the catalog, ")
...
b.WriteString("to fetch a specific skill.\n\n")
```
Add a routing sentence pointing MCP-only agents to `specgraph_skills_get name=specgraph-constitution` / `specgraph-authoring` as the next step.

**`writeProjectConstitution` empty-state** (prime.go:206–212) — fresh-init projects hit this branch. Change the CLI hint to MCP routing:
```go
// current:
b.WriteString("## Constitution\n\n_No constitution configured. Run `specgraph constitution set` to define project ground truth._\n\n")
```

**`constitutionEmptyResource`** (resources.go:36–42) — identical CLI hint (`Run `specgraph constitution set``); update to MCP-first for consistency.

**Pitfall 3 (golden test):** `TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` asserts byte-for-byte layout — any wording change MUST update the golden expectation in `internal/render/prime_test.go`.

---

### **NEW** `e2e/api/mcp_only_authoring_test.go` — the MCP-only gate (D-08)

**Analogs:** `e2e/api/skills_test.go` (the `skillsMCPClient` harness) and `prime_cross_surface_test.go` (prime resource read + assertions).

**Harness pattern to copy verbatim** (`skills_test.go:35–58`) — spins the real in-process MCP server, returns an mcp-go client + cleanup:
```go
func skillsMCPClient(ctx context.Context) (*client.Client, func()) {
	mcpClient := mcppkg.NewClient(http.DefaultClient, serverInfo.BaseURL)
	srv := mcppkg.NewServer(mcpClient)
	httpSrv := httptest.NewServer(http.StripPrefix("/mcp", srv.HTTPHandler()))
	mcpURL := httpSrv.URL + "/mcp/"
	c, err := client.NewStreamableHttpClient(mcpURL, transport.WithHTTPBasicClient(httpSrv.Client()))
	...
	Expect(c.Initialize(ctx, mcp.InitializeRequest{ ... })).Error().NotTo(HaveOccurred())
	return c, func() { _ = c.Close(); httpSrv.Close() }
}
```

**The "no CLI" simulation (D-08):** call ONLY `mcpCli` methods — `ReadResource` + `CallTool`. **Never** construct a `specgraphv1connect.*ServiceClient` (that is what `prime_cross_surface_test.go:61–63` does and is exactly the surface to avoid here).

**Assertion helpers to reuse:** `toolText` (`skills_test.go:63–73`, asserts `!IsError`) and `mcpResourceText` (`prime_cross_surface_test.go:203–211`).

**Spec flow (RESEARCH.md §4):**
1. `ReadResource specgraph://prime` → assert 200 + empty-state constitution hint (prime smoke, extends `prime_cross_surface_test.go`).
2. `CallTool specgraph_skills_get name=specgraph-constitution`.
3. `CallTool constitution action:update` with friendly YAML → assert persists (`action:get` round-trip).
4. `CallTool author` spark→shape→specify→decompose→approve with friendly YAML → assert approved.

**Build-tag + package:** `//go:build e2e` + `package api_test` (skills_test.go:4–6). Runs via `go test -tags e2e ./e2e/api/ -run MCPOnly` (Docker/testcontainers).

**Pitfall 6:** `skills_test.go:106` "lists six skills" enumerates 6 of 7 (omits `specgraph-constitution`). Bodies changing won't break it; re-check only if names/summaries change.

---

### **NEW/EXTEND** skills content-drift / MCP-reference assertion (criterion #4, D-09)

**Analog:** `internal/authoring/drift_test.go` (`TestContentProtoDrift`, lines 15–67).

**Pattern to copy** — table-driven scan of embedded content with a fenced-block stripper + allowlist:
```go
fenceRE := regexp.MustCompile("(?s)```.*?```")           // strip code fences from prose scan
scanned := fenceRE.ReplaceAllString(string(content), "")
...
allowlist := map[string]bool{"author": true, "graph_query": true, ...}  // MCP tool/arg names
```

**New assertion (per criterion #4):** for each rewritten `SKILL.md`, assert it **references the MCP tool path** (e.g. contains the `constitution` / `author` tool usage) AND the CLI content is present-but-gated (under the "Requires local CLI" appendix header). Two placement options (RESEARCH.md §4): extend `TestContentProtoDrift` OR add a sibling under `internal/mcp/skills`. Prefer a sibling in `internal/mcp/skills` (keeps the drift test focused on `internal/authoring/content/`).

**Pitfall 2 coupling:** if the new funnel friendly format renames any field vs the proto, `TestContentProtoDrift` breaks — snake_case keys matching proto field names keep it green.

---

## Shared Patterns

### Friendly `*FromString` enum mapper (the core #1002 correctness pattern)
**Source:** `internal/mcp/tools_core.go:29–35` (`constitutionLayerFromString`) — also `passTypeFromString` (37–44), `postureFromString` (tools_authoring.go:98–104).
**Apply to:** every new funnel enum mapper (`scopeSniffFromString`, `decompositionStrategyFromString`) in `internal/authoring/load`.
```go
key := "PREFIX_" + strings.ToUpper(s)   // or ReplaceAll(s,"-","_") for hyphenated
if v, ok := specv1.Enum_value[key]; ok { return specv1.Enum(v) }
return specv1.Enum_ENUM_UNSPECIFIED    // UNSPECIFIED → caller errors, never silent-writes (Tampering mitigation)
```

### MCP handler house style
**Source:** `tools_core.go` / `tools_authoring.go` throughout.
**Apply to:** all handler edits.
- `stringParam(params, "x")` / `boolParam` for input; guard empty with `errResult("x is required for <action>")`.
- Success: `jsonResult(resp.Msg)`. RPC error: `connectErrResult(err)`. Validation error: `errResult(msg)` — never surface raw `protojson`/`yaml` internals (V6 info-disclosure mitigation).
- Tool `def()`: `objectSchema(props{...}, "requiredKey"...)`; `Description` + param docs are the agent teaching surface (criterion #4).

### Friendly YAML → proto pipeline (reuse, don't reinvent)
**Source:** `internal/constitution/load/load.go` (`FromYAML` + `ToProto`) — backed by `internal/config` (`ConstitutionConfig`, `ParseConstitutionConfig`, `ValidateLayer`, `ToDomain`).
**Apply to:** constitution handler (reuse verbatim); funnel handlers (replicate the package shape in `internal/authoring/load`).

### Gated CLI appendix (D-07)
**Source:** NEW convention (no exact analog — `specgraph-authoring` is CLI-free today).
**Apply to:** all 7 `SKILL.md` — ONE uniform `## Requires local CLI (…MCP-only agents skip this)` section at the END; preserves CLI docs without presenting them as the primary path.

### Repo conventions (hard gates — AGENTS.md / RESEARCH.md Pitfall 5)
**Apply to:** every code/test file touched.
- New `.go` files: `// SPDX-License-Identifier: Apache-2.0` header (`task license:add`).
- New packages (`internal/authoring/load`): `// Package … ` doc comment (revive).
- Every commit: `Signed-off-by:` (DCO, `git commit -s`).
- `task check` before push; `task pr-prep` (Docker) for e2e.

---

## No Analog Found

None. Every file — including all net-new artifacts — has a strong in-repo analog:
- new funnel `load` package → `internal/constitution/load` (direct template)
- MCP-only e2e → `skills_test.go` harness + `prime_cross_surface_test.go` structure
- skills content assert → `TestContentProtoDrift`
- gated-CLI appendix is the only genuinely-new *content* convention (no code analog needed — it is prose structure).

---

## Metadata

**Analog search scope:** `internal/mcp/`, `internal/constitution/load/`, `internal/config/`, `internal/render/`, `internal/authoring/`, `e2e/api/`, `internal/mcp/skills/embedded/`, `proto/specgraph/v1/authoring.proto`.
**Files read (grounded excerpts):** tools_core.go, tools_authoring.go, constitution/load/load.go, config/config.go, render/prime.go, mcp/resources.go, e2e/api/skills_test.go, e2e/api/prime_cross_surface_test.go, internal/authoring/drift_test.go, specgraph-constitution/SKILL.md, specgraph-authoring/SKILL.md, authoring.proto (enum/message grep).
**Pattern extraction date:** 2026-07-14
