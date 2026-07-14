# Phase 6: MCP Authoring Self-Teaching Path - Research

**Researched:** 2026-07-14
**Domain:** MCP tool interface design, embedded skill content, Go/ConnectRPC handler-layer format shims, Ginkgo/Gomega e2e
**Confidence:** HIGH (all claims grounded in the actual repo at this commit)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Reject raw protojson as the primary authoring interface. Neither the protojson blob nor the get→modify→update round-trip (as-is) is adopted.
- **D-02:** Research evaluates two directions and recommends one (or a blend): (a) YAML/token-friendly whole-doc format at the MCP write boundary; (b) section-by-section granular tool signature. Scored on token cost, self-teaching robustness, back-compat, implementation size. **Research recommends; planning locks.**
- **D-03:** The interface rethink covers BOTH surfaces — the `constitution` tool AND the author-funnel stage outputs (`spark`/`shape`/`specify`/`decompose` `output`). They must stay consistent.
- **D-04:** Existing forgiving mappers (`constitutionLayerFromString`, a proposed `referenceTypeFromString`) are prior art, not a locked answer.
- **D-05:** Full MCP-first rewrite of all 7 embedded skills.
- **D-06:** Skills teach the chosen write-input pattern as *the* authoring pattern.
- **D-07:** Demote CLI to a clearly-gated "Requires local CLI" appendix at the end of each skill. No co-equal CLI+MCP steps.
- **D-08:** Automated MCP-only e2e test is the gate. Drives `specgraph://prime` → skills fetch → tool calls with CLI unavailable; asserts constitution/spec reaches approved/completed. Fits `e2e/api/` Ginkgo/Gomega.
- **D-09:** Success criterion #4 covered by a content-level assertion; `TestContentProtoDrift`-style check is the precedent.
- **D-10:** `specgraph://prime` stays a state/orientation resource but becomes a reliable ENTRY POINT routing MCP-only agents to the authoring skills. Prime does NOT duplicate interface teaching inline. Minimal prime change.

### the agent's Discretion
- Exact write-input mechanism (YAML vs token-friendly vs section-by-section vs blend) — delegated to this research.
- e2e harness details for simulating "MCP-only / no CLI" — left to planning/implementation.

### Deferred Ideas (OUT OF SCOPE)
- amend/supersede lifecycle semantics (Phase 7), conversation-recording enforcement (Phase 8), JIT display-name reconciliation (Phase 9).
- **Concern flagged for research (NOT deferred):** `specgraph://prime` failed to load in the discuss session (`"unable to load: internal: internal error"`). Must confirm reliability. → See **Prime Load-Failure Finding** below.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| MCP-01 (#1002) | An agent in a fresh MCP-only project can author the project constitution to completion using only `specgraph://prime` + MCP-served skills, with no out-of-band CLI/YAML knowledge. | Recommendation (friendly-YAML write boundary, reusing the existing `internal/constitution/load` pipeline), the 7-skill rewrite map, prime-routing change, and the MCP-only e2e + content-drift verification architecture below all directly enable this. |
</phase_requirements>

## Summary

The root cause (#1002) is a **format mismatch at the MCP write boundary**: the `constitution` tool's `update` action and every author-funnel stage (`spark`/`shape`/`specify`/`decompose`) `protojson.Unmarshal` their input straight into the proto. That requires literal enum names (`CONSTITUTION_LAYER_PROJECT`, `REFERENCE_TYPE_ADR`) and camelCase proto field names — knowledge an MCP-only agent does not have. Meanwhile the served `specgraph-constitution` skill teaches the **CLI** path (`specgraph constitution import`) plus a **friendly YAML schema** (`layer: project`, `type: adr`). The agent authors that friendly YAML, has no CLI to import it, tries to feed it to the MCP tool, and silently fails.

The decisive finding: **the friendly YAML format the skill already teaches is already fully parsed and converted to proto by shipping code.** `internal/config` defines `ConstitutionConfig` (with `layer`, `type` friendly fields), `internal/constitution/load.FromYAML` parses it into `storage.Constitution`, and `load.ToProto` converts that to `*specv1.Constitution` ready for the `UpdateConstitution` RPC. This is exactly the pipeline the CLI's `constitution import` uses. Wiring it into the MCP `constitution.update` handler is a **~10-line handler-layer change with no proto regen**. The funnel stages have no equivalent friendly layer yet, but their proto shapes are simple (mostly repeated strings + a couple of enums), so an analogous friendly-YAML input layer is small and needs **no proto changes** either.

**Primary recommendation:** Adopt **Direction (a) — a friendly YAML / token-lean whole-doc format at the MCP write boundary — for BOTH surfaces.** For the constitution, reuse the existing `config`/`load` friendly pipeline verbatim. For the funnel, add a small parallel friendly-input layer (snake_case fields + lowercase-enum mappers). Reject Direction (b) (section-by-section actions): it multiplies tool round-trips and token cost, introduces partial-build state, and requires a much larger implementation for no self-teaching gain over "fill in one YAML block the skill shows you." This is a **handler-layer format shim + skills rewrite + prime routing + e2e/content tests — no `task proto` required.**

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Write-input format normalization (friendly YAML → proto) | API/Backend — MCP handler layer (`internal/mcp/tools_*.go`) | `internal/constitution/load`, `internal/config` (shared parse) | The MCP tool handler is the boundary where agent-authored input enters; format forgiveness belongs there, not in the RPC/storage layer (which stays proto-typed). |
| Constitution persistence | API/Backend — `ConstitutionService.UpdateConstitution` RPC (`internal/server`) | `internal/storage/postgres` | Unchanged. `update` replaces-and-bumps-version; whole-doc write fits it exactly. |
| Funnel stage persistence | API/Backend — `AuthoringService.{Spark,Shape,Specify,Decompose,Approve}` RPCs | `internal/storage/postgres` | Unchanged. Only the MCP input format changes; the RPC contract is identical. |
| Agent teaching surface (skills) | Content — embedded `SKILL.md` canonicals | MCP serving (`tools_skills.go`, `resources.go`) — no code change | Skills are `//go:embed` content served verbatim; the fix is content, not serving code. |
| Entry-point routing | API/Backend — prime render (`internal/render/prime.go`) via `GetPrime` RPC | `internal/prime` composer, `internal/mcp/resources.go` | Prime is composed server-side and rendered to markdown; routing text is a render-layer addition. |
| Verification | Test — `e2e/api/` (Ginkgo) + `internal/authoring` unit (content-drift) | `internal/mcp` in-process server harness | e2e drives the real MCP server; content-drift is a fast unit test. |

## Recommendation: Write-Input Interface (the D-02 decision)

### Scored comparison

| Criterion | (a) Friendly YAML whole-doc | (b) Section-by-section actions | Winner |
|-----------|-----------------------------|--------------------------------|--------|
| **Token cost** | 1 tool call per doc/stage. A full constitution (tech + 5 principles + 5 constraints + 3 antipatterns + process) = **1 call**. | ~15+ calls (set-tech, add-principle ×5, add-constraint ×5, add-antipattern ×3, set-process…), each with schema echo + result. Multiplies tokens & latency. | **(a)** |
| **Self-teaching robustness** ("impossible to get wrong") | Skill shows ONE YAML block; agent fills it. Format = what the skill *already* teaches today. Forgiving parser accepts `layer: project`, `type: adr` (lowercase). | Agent must learn N action schemas + the correct call sequence + which fields are repeatable. More surface, more ordering mistakes, partial-doc states. | **(a)** |
| **Backward compatibility** | `update` already replaces-whole-doc, so whole-doc input is the natural fit. Existing protojson callers = the current MCP tool + its tests only (no production/CLI consumers of the MCP path). Low risk; can auto-accept both. | Requires brand-new action schemas and dispatch; either new partial-update RPCs or an internal get-merge-update. Larger contract change. | **(a)** |
| **Implementation size** | Constitution: **~10 lines** (reuse `load.FromYAML`+`load.ToProto`). Funnel: small new friendly-input structs + 2 enum mappers. **No proto regen.** | 6+ new action schemas, param validation, dispatch, partial-state semantics across constitution + 4 funnel stages. Largest option; may touch proto. | **(a)** |
| **Consistency across both surfaces (D-03)** | One mental model — "author a YAML block, submit it" — for constitution AND every funnel stage. | Two very different shapes unless the funnel is *also* re-cut into granular actions, ballooning scope. | **(a)** |

### RECOMMENDATION — Direction (a), friendly YAML for both surfaces

Adopt a **friendly YAML / token-lean whole-doc write format** at the MCP boundary for the `constitution` tool and all four funnel stage tools. Concretely:

1. **Constitution (`constitution.update`)** — Replace the `protojson.Unmarshal` in `handleUpdate` (`internal/mcp/tools_core.go:105`) with the existing friendly pipeline:
   `raw → internal/constitution/load.FromYAML(raw) → load.ToProto(...) → UpdateConstitution`.
   This is the exact format the `specgraph-constitution` skill already documents (Step 3 YAML schema) and the exact pipeline `specgraph constitution import` uses (`load.FromYAML` is the "single source of YAML parsing for both the CLI's import and the server's RefreshConstitutionLayer"). YAML is a JSON superset, so agents may also paste JSON with friendly (`type`, `layer`) keys. **No proto change.**
   - Rename the tool param from `constitution` (currently "Full constitution JSON…") to reflect friendly input, and rewrite its `Description` to document the YAML shape inline (tool descriptions are an agent-facing teaching surface per criterion #4).
   - **Enhancement (recommended, not required):** have `constitution.get` optionally emit the same friendly YAML (a `ConstitutionConfigFromDomain` path already exists in `internal/config`) so get→edit→update round-trips in ONE consistent format. For fresh-init projects the agent typically authors from a blank template, so this is a nicety, not a blocker.

2. **Funnel stages (`author.{spark,shape,specify,decompose}`)** — Add a small parallel friendly-input layer (suggest a new `internal/authoring/load` package mirroring `internal/constitution/load`, or inline structs in `internal/mcp`). Parse a friendly YAML `output` into the stage proto:
   - Use **snake_case field names** (`scope_in`, `scope_out`, `success_must`, `chosen_approach`, `scope_sniff`) — these match the proto field names, read naturally, AND keep `TestContentProtoDrift` green (it scans backtick snake_case tokens against proto field names).
   - Map the two enum fields with friendly lowercase mappers: `scope_sniff: medium` → `SCOPE_SNIFF_MEDIUM` and `strategy: single_unit` → `DECOMPOSITION_STRATEGY_SINGLE_UNIT`. Model these on the existing `constitutionLayerFromString`/`passTypeFromString` helpers.
   - `approve` takes no `output` (already just a slug) — no change beyond description.
   - **No proto change** — the `SparkRequest/ShapeRequest/...` contracts are untouched; only the MCP `output` string is parsed differently before building the request.

3. **Back-compat guard:** Accept both formats defensively. Simplest robust rule: try the friendly parser first; friendly YAML with the documented keys is unambiguous. If planning wants belt-and-suspenders, detect a leading `{` with camelCase/`referenceType`-style keys and fall back to `protojson.Unmarshal`. Existing MCP tests that pass protojson will need updating to the friendly format (see landmines).

**Why not the issue's get→modify→update round-trip:** it keeps protojson field/enum names as the interface — precisely what the user rejected (D-01). Friendly YAML gives the same "server is the schema" self-teaching property (the skill's single YAML block is the schema) without exposing protojson.

## Code Map — files/symbols that must change

Grouped by the five focus areas. All paths verified at this commit.

### 1. Write-input interface (handler-layer shim — NO proto regen)

| File / symbol | Line(s) | Current state | Change |
|---------------|---------|---------------|--------|
| `internal/mcp/tools_core.go` — `constitutionTool.handleUpdate` | 105–121 | `protojson.Unmarshal([]byte(raw), &c)` straight into `specv1.Constitution` — **the defect**. | Parse via `load.FromYAML` → `load.ToProto`; friendly errors. |
| `internal/mcp/tools_core.go` — `constitutionTool.def()` Description + `constitution` param prop | 54–73 | Param doc: "Full constitution JSON for update (output from get…)". | Rewrite to document friendly YAML shape (teaching surface, criterion #4). |
| `internal/mcp/tools_core.go` — `constitutionLayerFromString` | 29–35 | Used by `get` only. | Reference pattern for new enum mappers; constitution layer handled by `load`/`config` already. |
| `internal/mcp/tools_authoring.go` — `authorTool.handleSpark/handleShape/handleSpecify/handleDecompose` | 169–294 | Each `protojson.Unmarshal([]byte(raw), &out)` into the stage proto — **same defect ×4**. | Parse friendly YAML `output` → stage proto via new friendly layer. |
| `internal/mcp/tools_authoring.go` — `authorTool.def()` Description + `output` param prop | 123–145 | `output` doc: "Stage output as a JSON string…". | Rewrite to document friendly YAML per stage. |
| `internal/constitution/load/load.go` — `FromYAML`, `ToProto`, `referenceTypeToProto`, `layerToProto` | 22–107 | Complete friendly→proto pipeline; used by CLI import + server refresh. | **Reuse as-is** for constitution. Template for the funnel `load` package. |
| `internal/config/config.go` — `ConstitutionConfig`, `ParseConstitutionConfig`, `ValidateLayer`, `ToDomain` | 113–347 | Friendly YAML struct (`layer`, `type`) + validation + domain mapping. | **Reuse as-is.** `type`/`layer` are already lowercase-friendly. |
| **NEW** `internal/authoring/load/` (suggested) | — | Does not exist. | Friendly YAML structs for Spark/Shape/Specify/Decompose outputs + `scopeSniffFromString`, `decompositionStrategyFromString` mappers → proto. |

`referenceTypeFromString` proposed in #1002/D-04 is **not needed** if the constitution path routes through `load.FromYAML` — `config`'s `referencesToDomain` + `load.referenceTypeToProto` already map lowercase `type: adr/spec/doc/url`. (Confirmed: `referenceTypeFromString` does not exist in the tree today.)

### 2. Skills corpus (content-only unless noted; all under `internal/mcp/skills/embedded/<name>/SKILL.md`)

Serving path is DONE — **no code change** to `skills.go`, `embedded.go`, `tools_skills.go`, or `resources.go` for the rewrite. Only `SKILL.md` bodies change. (`skills` repo-root and `plugin/<harness>/` are reverse-symlinks into `embedded/` — editing either edits the canonical.)

| Skill | CLI-refs / MCP-refs (grep) | Current posture | Rewrite work |
|-------|---------------------------|-----------------|--------------|
| `specgraph-constitution` (167 ln) | 4 / 3 | **Worst offender.** Step 1 `constitution show`, Step 4 `constitution import`, Step 5 `constitution emit`. Teaches friendly YAML (good) but imports via CLI (bad). | Lead with `constitution` MCP tool + the chosen YAML write format; keep YAML schema; demote all `specgraph constitution *` to gated appendix. This is the #1002 "quick win". |
| `specgraph-authoring` (89 ln) | 0 / 11 | Already MCP-first (routes to prompts + `author` tool). | Align to chosen write format — document the friendly-YAML `output` per stage explicitly; add gated CLI appendix. Add explicit round-trip per criterion #1. |
| `specgraph-troubleshooting` (97 ln) | 10 / 10 | Heavily CLI (`doctor`, `health`, etc.). | Reframe MCP-first (`health` tool, resources); demote CLI diagnostics to appendix. |
| `specgraph-drift` (63 ln) | 3 / 3 | Mixed CLI/MCP. | MCP-first; gated CLI appendix. |
| `specgraph-graph-query` (77 ln) | 0 / 7 | Mostly MCP already. | Light audit; add uniform gated CLI appendix; voice pass. |
| `specgraph-analytical-passes` (79 ln) | 0 / 5 | MCP-leaning. | Light audit; uniform appendix; voice pass. |
| `specgraph-conventions` (81 ln) | 0 / 3 | Reference-style. | Light audit; uniform appendix; voice pass. |

The **"Requires local CLI" gated appendix (D-07)** goes at the END of each skill, one uniform labeled section, so an MCP-only agent skips it.

### 3. Prime entry point (D-10 — minimal change)

| File / symbol | Line(s) | Change |
|---------------|---------|--------|
| `internal/render/prime.go` — `writeSkills` | 306–315 | Already emits a Skills section pointing at `specgraph_skills_list/get/search`. **Strengthen into an explicit "start here" routing line** for authoring (e.g., "To author the constitution or a spec, fetch `specgraph_skills_get name=specgraph-constitution` / `specgraph-authoring` first"). This is the minimal D-10 change. |
| `internal/render/prime.go` — `writeProjectConstitution` empty-state | 206–212 | Empty state currently says `Run \`specgraph constitution set\``. **Change the empty-state hint to route to the MCP skill/tool**, not the CLI (fresh-init projects hit exactly this branch). |
| `internal/mcp/resources.go` — `constitutionEmptyResource` | 36–42 | Same CLI-first empty hint (`Run \`specgraph constitution set\``). Update to MCP-first routing for consistency. |
| `internal/prime/composer.go` — `Project` | 43–111 | No logic change needed; already treats absent constitution as soft-empty (critical for fresh-init — see prime finding). |

Note: `RenderProjectMarkdown` has a byte-for-byte legacy-layout invariant test (`TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout`) — any wording change updates that golden expectation.

### 4. Verification (new tests)

| File | Change |
|------|--------|
| **NEW** `e2e/api/mcp_only_authoring_test.go` (suggested) | Ginkgo spec using the `skillsMCPClient` in-process MCP server harness (`e2e/api/skills_test.go:35`). Drive `specgraph://prime` → `specgraph_skills_get` → `constitution` tool (friendly YAML) → assert constitution persists; then full funnel `author` spark→…→approve via friendly YAML → assert approved. **Only MCP client calls — never construct a CLI/ConnectRPC service client** (that is the "no CLI" simulation). |
| **NEW/EXTEND** `internal/authoring/…_test.go` content-drift check | Extend the `TestContentProtoDrift` precedent (`internal/authoring/drift_test.go:15`) OR add a sibling in `internal/mcp/skills` asserting each rewritten `SKILL.md` references the MCP tool path (e.g., contains `constitution` tool + `author` tool usage, and the CLI appendix is present-but-gated). Covers criterion #4 / D-09. |
| `e2e/api/skills_test.go` | The "lists six skills" assertion count comment says "six" but lists 6 of 7 — verify/adjust expectations after content edits (bodies change; names don't). |

### 5. Proto — NONE required

No `.proto` edit, no `task proto`. Confirmed: the recommended direction is purely a handler-layer input-format change; `ConstitutionService`/`AuthoringService` request/response contracts are untouched. (If planning were to choose Direction (b), proto changes would likely be needed — another reason to prefer (a).)

## Prime Load-Failure Finding (the flagged `<deferred>` concern)

**Verdict: LOCAL NO-SERVER / NO-DB ARTIFACT, not a prime-handler code bug. Confidence: HIGH.**

Evidence:
1. **Error origin.** `"internal: internal error"` is the exact string `executionError` returns for any non-sentinel storage error: `connect.NewError(connect.CodeInternal, errors.New("internal error"))` (`internal/server/execution_handler.go:307–308`, also 121, 139). `GetPrime` → `getPrimeProject` → `composer.Project` → any backend call failing (DB down, unmigrated, or project not scoped) produces exactly this.
2. **Asymmetry is the tell.** In the discuss-session prime block, the **Skills** section loaded ("7 skills exposed via MCP") while Constitution / Graph Overview / Ready / Findings all failed with `internal error`. Skills come from the **embedded, DB-free** `skills.Source`; every failing section is **DB-backed** (`GetMergedConstitution`, `ListSpecs`, `GetReady`, `ListAllFindings`). Embedded-OK + all-DB-fail = backend/DB unavailable or the request wasn't project-scoped, not a logic defect.
3. **Fresh-init happy path does NOT error.** `composer.Project` treats `storage.ErrConstitutionNotFound` as a **soft-empty state** (composer.go:52–53) — no error. Empty `ListSpecs`/`GetReady`/`ListAllFindings` return empty slices, not errors. So a *healthy* fresh-init project returns prime successfully with empty-state markdown. This is precisely what D-10 relies on, and it holds.
4. **Live-server proof exists.** `e2e/api/prime_cross_surface_test.go` exercises `specgraph://prime` (and `GetPrime`) against a real server+Postgres and asserts success. That path is green in CI.

**Action for planning:** No prime *bug* to fix. The MCP-only e2e (D-08) should include a smoke assertion that `specgraph://prime` returns 200 with the empty-state constitution hint on a fresh project — which both proves reliability and guards the empty-state routing text you add. The session failure was environmental (MCP server/DB not wired, or missing `X-Specgraph-Project` scope during the local session).

## Common Pitfalls / Open Risks & Landmines

### Pitfall 1: MCP tests hard-code protojson `output`
`e2e/api/helpers_test.go:advanceStage` and existing MCP tool tests build stage inputs as proto/JSON. **Warning sign:** switching the MCP input format breaks any test that feeds protojson to the `constitution`/`author` *tools* (the ConnectRPC service-client tests in `helpers_test.go` are fine — they hit RPCs, not the MCP tool, and stay proto-typed). Audit every caller of the MCP `constitution`/`author` tools and migrate to friendly YAML.

### Pitfall 2: `TestContentProtoDrift` coupling
`internal/authoring/drift_test.go` fails if a `stage-*.md` (in `internal/authoring/content/`) references a backtick snake_case token that is not a proto field name. Using **snake_case** friendly YAML keys that match proto field names keeps this green; inventing new camelCase or renamed keys would break it. If the funnel friendly format renames a field, update the drift allowlist deliberately.

### Pitfall 3: prime golden-layout test
`TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout` asserts byte-for-byte legacy layout. Any wording change in `writeSkills`/empty-state must update the golden expectation in `internal/render/prime_test.go`.

### Pitfall 4: symlink editing of skills
`skills/` (repo root) and `plugin/{specgraph,cursor,opencode}/` are **reverse-symlinks into `internal/mcp/skills/embedded/`**. Edit via any path; it's the same canonical file. Do NOT create a second physical copy. `skills_symlink_test.go` guards this.

### Pitfall 5: repo conventions (hard gates from AGENTS.md)
- **License headers:** new `.go` files need `SPDX-License-Identifier: Apache-2.0` (run `task license:add`).
- **DCO:** every commit needs `Signed-off-by:` (`git commit -s`).
- **revive:** new Go packages (e.g., `internal/authoring/load`) need a `// Package … ` doc comment.
- **`task check`** before push (fmt→license→lint→build→unit); **`task pr-prep`** (Docker) for integration/e2e.
- **e2e requires Docker** (testcontainers `pgvector/pgvector:pg18`); the MCP-only e2e runs under `//go:build e2e` via `go test -tags e2e`.

### Pitfall 6: skill-count assertion drift
`e2e/api/skills_test.go` "lists six skills" enumerates 6 of the 7 names (omits `specgraph-constitution`). Bodies changing won't break it, but if planning touches names/summaries, re-check.

## Verification Architecture

**Nyquist validation:** `.planning/config.json` not inspected for an explicit `false`; treat as enabled. Test framework below.

### Test Framework
| Property | Value |
|----------|-------|
| Unit framework | Go `testing` (std) — `internal/authoring/drift_test.go` is the content-drift precedent |
| e2e framework | Ginkgo v2 / Gomega, `//go:build e2e`, MCP client `github.com/mark3labs/mcp-go/client` |
| e2e MCP harness | `e2e/api/skills_test.go:skillsMCPClient` — spins up real in-process `mcp.NewServer(mcpClient)` in an `httptest.Server`, returns a streamable-HTTP MCP client. **This is the harness for the MCP-only test.** |
| Quick run | `task test` (skips integration/e2e) |
| e2e run | `go test -tags e2e ./e2e/api/...` (Docker required) / `task pr-prep` |

### Phase Requirements → Test Map
| Criterion | Behavior | Test Type | Command | Exists? |
|-----------|----------|-----------|---------|---------|
| #1 skills describe MCP round-trip | rewritten SKILL.md reference MCP tools | unit (content assert) | `go test ./internal/mcp/skills/... ./internal/authoring/...` | ❌ Wave 0 |
| #2 full funnel MCP-only | prime→skills→`author` spark…approve via friendly YAML | e2e | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ❌ Wave 0 |
| #3 constitution approved MCP-only | `constitution` tool friendly-YAML write persists | e2e | same spec | ❌ Wave 0 |
| #4 skills_get/search reference MCP path | content-level assert vs embedded canonicals | unit | `go test ./internal/mcp/skills/...` | ❌ Wave 0 (extend `TestContentProtoDrift` precedent) |
| Prime reliability | `specgraph://prime` 200 + empty-state hint on fresh project | e2e | same spec / `prime_cross_surface_test.go` | ⚠️ partial (extend) |

### Sampling Rate
- **Per task commit:** `task test` (unit + content-drift, < 30s).
- **Per wave merge:** `go test -tags e2e ./e2e/api/...` (Docker).
- **Phase gate:** full suite + the MCP-only e2e green before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `e2e/api/mcp_only_authoring_test.go` — covers #2, #3, prime smoke (MCP-client-only).
- [ ] content-drift/reference assertion for rewritten skills — covers #1, #4.
- [ ] (if funnel friendly layer added) `internal/authoring/load/*_test.go` — friendly YAML → proto mapping incl. enum mappers.

## Validation Architecture

Nyquist feedback-sampling contract for this phase. Feeds `VALIDATION.md` 1:1 (Test Infrastructure → Sampling Rate → Per-Task/Behavior Verification Map → Wave 0 Requirements → Manual-Only Verifications). Every command/path below is grounded in the repo and consistent with the Verification Architecture and Code Map above.

### Test Infrastructure

| Property | Value |
|----------|-------|
| **Unit framework** | Go `testing` (std). Content-drift precedent: `internal/authoring/drift_test.go` (`TestContentProtoDrift`). |
| **e2e framework** | Ginkgo v2 / Gomega, `//go:build e2e`, MCP client `github.com/mark3labs/mcp-go/client`. |
| **e2e MCP harness** | `e2e/api/skills_test.go:skillsMCPClient` — spins a real in-process `mcp.NewServer(mcpClient)` inside an `httptest.Server`, returns a streamable-HTTP MCP client. **This is the "MCP-only / no CLI" harness (D-08):** the spec calls ONLY MCP client methods and never constructs a ConnectRPC/CLI service client. |
| **Config file** | None — Go test discovery; build tags (`//go:build e2e`) gate the e2e suite. Postgres via testcontainers (`pgvector/pgvector:pg18`, Docker required for e2e). |
| **Quick run command** | `task test` (unit + content-drift; skips `integration`/`e2e`; < 30s). |
| **Full suite command** | `go test -tags e2e ./e2e/api/...` (Docker) — or `task pr-prep` for the full check→integration→e2e pipeline. |
| **Estimated runtime** | Quick ~15–30s; e2e ~1–3 min (container spin-up dominated). |

### Sampling Rate

- **After every task commit:** `task test` — unit + content-drift reference assertions. Max feedback latency ~30s.
- **After every plan wave:** `go test -tags e2e ./e2e/api/...` — full e2e including the MCP-only authoring spec (Docker).
- **Before `/gsd-verify-work` (phase gate):** full suite green AND the MCP-only e2e (`-run MCPOnly`) green. No CLI-path fallback is permitted to satisfy the gate (D-08).
- **Max feedback latency:** 30s (quick loop) / ~3 min (wave/gate loop).

### Per-Requirement / Per-Behavior Verification Map

Phase requirement: **MCP-01 (#1002)**. Keyed by the 4 ROADMAP success criteria + prime-reliability.

| Ref | Behavior | Test Type | Automated Command | Exists Today? |
|-----|----------|-----------|-------------------|---------------|
| MCP-01 / #1 | Rewritten SKILL.md corpus describes the MCP write round-trip (constitution `update` + `author` stages via friendly YAML); CLI is present-but-gated appendix. | unit (content assert, extends `TestContentProtoDrift` precedent) | `go test ./internal/mcp/skills/... ./internal/authoring/...` | ❌ Wave 0 |
| MCP-01 / #2 | Full funnel MCP-only: `specgraph://prime` → `specgraph_skills_get` → `author` spark→shape→specify→decompose→approve via friendly YAML → spec reaches approved. | e2e (MCP-client-only) | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ❌ Wave 0 |
| MCP-01 / #3 | Constitution authored to completion MCP-only: `constitution` tool friendly-YAML write persists and reaches approved state. | e2e (MCP-client-only) | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ❌ Wave 0 |
| MCP-01 / #4 | `specgraph_skills_get`/`_search` return guidance referencing the MCP tool path, verified against embedded canonicals. | unit (content-level assert, D-09) | `go test ./internal/mcp/skills/...` | ❌ Wave 0 (extend `TestContentProtoDrift` precedent) |
| MCP-01 / prime-reliability | `specgraph://prime` returns 200 with the empty-state constitution hint (MCP-routing text) on a fresh project. | e2e (smoke, same MCP-only spec) | `go test -tags e2e ./e2e/api/ -run MCPOnly` | ⚠️ Partial — `prime_cross_surface_test.go` proves live prime; extend for empty-state routing assertion |

### Wave 0 Requirements

Test scaffolding that must exist before implementation tasks land:

- [ ] `e2e/api/mcp_only_authoring_test.go` — Ginkgo spec on the `skillsMCPClient` harness. MCP-client-only funnel (spark→approve) + constitution friendly-YAML write + `specgraph://prime` empty-state smoke. Covers criteria **#2, #3, prime-reliability**. Never constructs a CLI/ConnectRPC client.
- [ ] Content-drift / reference assertion for the rewritten skills — extend the `TestContentProtoDrift` precedent (`internal/authoring/drift_test.go`) or add a sibling under `internal/mcp/skills` asserting each rewritten `SKILL.md` references the MCP tool path and gates the CLI appendix. Covers criteria **#1, #4**.
- [ ] (Conditional — only if the funnel friendly-input layer is added, per Recommendation §2) `internal/authoring/load/*_test.go` — unit tests for friendly-YAML→proto mapping of Spark/Shape/Specify/Decompose outputs, including the enum mappers (`scopeSniffFromString`, `decompositionStrategyFromString`) returning UNSPECIFIED→error on invalid input.

*No new framework install needed — Go `testing` + the existing Ginkgo/Gomega e2e suite cover all phase behaviors.*

### Manual-Only Verifications

**None — all phase behaviors have automated verification.** D-08 mandates an automated MCP-only e2e gate, and criteria #1/#4 are covered by content-level unit assertions. No behavior in this phase requires a manual walkthrough.

## Security Domain

`security_enforcement` not set to `false` → included. This phase adds an input-parsing surface.

| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V5 Input Validation | **yes** | The friendly YAML parser must reject unknown layers/enum values (`config.ValidateLayer` already does for constitution; the funnel enum mappers must return UNSPECIFIED→error, mirroring `constitutionLayerFromString`). Malformed YAML → sanitized `errResult`, never a raw internal error. |
| V6 Cryptography | no | — |
| V2/V3/V4 Auth/Session/Access | no (unchanged) | MCP auth middleware is out of scope for this phase. |

| Threat Pattern | STRIDE | Mitigation |
|----------------|--------|------------|
| YAML bomb / oversized input at write boundary | DoS | Rely on existing request-size limits; parse into typed structs (not arbitrary maps). `yaml.Unmarshal` into a fixed struct bounds the shape. |
| Enum/layer smuggling (invalid value silently persists) | Tampering | Explicit `*FromString` mappers returning UNSPECIFIED must error, not default-write. This is the core #1002 correctness fix. |
| Handler error leakage | Info disclosure | Keep the `errResult`/`connectErrResult` house style; never surface raw `protojson`/`yaml` internals beyond a concise message. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `internal/authoring/load` does not yet exist and a new friendly layer for funnel outputs is needed. | Recommendation §2 | LOW — verified no such package; if a friendly funnel parser already existed it'd only shrink the work. |
| A2 | Migrating MCP tool tests to friendly YAML is the only test-breakage from the format switch (RPC service-client tests unaffected). | Pitfall 1 | MEDIUM — planning should grep all MCP `constitution`/`author` tool callers to confirm blast radius. |
| A3 | The discuss-session prime failure was environmental (no live server/DB or unscoped project), not a code bug. | Prime Finding | LOW — asymmetry (embedded skills OK, all DB sections fail) + green `prime_cross_surface_test.go` corroborate. Planning's e2e prime smoke will confirm definitively. |
| A4 | `.planning/config.json` treats `nyquist_validation`/`security_enforcement` as enabled (not explicitly false). | Verification/Security | LOW — default-enabled per GSD convention; not inspected this session. |

## Sources

### Primary (HIGH confidence — read directly this session)
- `internal/mcp/tools_core.go`, `internal/mcp/tools_authoring.go` — the two defect sites (protojson at write boundary).
- `internal/constitution/load/load.go`, `internal/config/config.go` — the friendly YAML→proto pipeline (the reuse asset).
- `proto/specgraph/v1/constitution.proto`, `proto/specgraph/v1/authoring.proto` — message shapes; confirmed no proto change needed.
- `internal/mcp/resources.go`, `internal/prime/composer.go`, `internal/render/prime.go` — prime + skills serving; empty-state = soft.
- `internal/server/execution_handler.go` — source of the `internal error` string.
- `internal/authoring/drift_test.go` — `TestContentProtoDrift` precedent for D-09.
- `e2e/api/skills_test.go`, `e2e/api/helpers_test.go`, `e2e/api/prime_cross_surface_test.go` — MCP-only harness + prime live proof.
- `internal/mcp/skills/embedded/*/SKILL.md` (all 7) + grep CLI/MCP ref counts.
- GitHub issue **#1002** (fetched via `gh`) — root cause, proposed direction, acceptance criteria, open sub-question.
- `.planning/ROADMAP.md`, `.planning/REQUIREMENTS.md` — locked goal + MCP-01.

### Secondary / Tertiary
- None required — all claims grounded in repo code.

## Metadata

**Confidence breakdown:**
- Recommendation (write-input direction): HIGH — the friendly pipeline already exists and is the format the skill already teaches.
- Code map: HIGH — every symbol read at this commit with line refs.
- Prime finding: HIGH — corroborated by error-string origin + asymmetry + live e2e.
- Verification architecture: HIGH — harness exists and is directly reusable.

**Research date:** 2026-07-14
**Valid until:** ~2026-08-14 (stable internal codebase; re-verify if `internal/mcp` or `internal/constitution/load` is refactored).
