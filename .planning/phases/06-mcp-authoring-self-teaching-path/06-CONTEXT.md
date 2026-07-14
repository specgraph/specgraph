# Phase 6: MCP Authoring Self-Teaching Path - Context

**Gathered:** 2026-07-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the MCP authoring surface **self-teaching for an MCP-only project**. An agent in a fresh `specgraph init` project (`.mcp.json` + managed files, **no source, no local CLI**) must be able to author the project constitution — and walk a spec through the full authoring funnel (Spark → Shape → Specify → Decompose → Approve) — to a completed/approved state using ONLY `specgraph://prime` and the MCP-served skills, with no out-of-band CLI/YAML knowledge.

Root cause (per #1002): the served skills teach the **CLI** path (`specgraph constitution import`), while the `constitution` MCP tool's `update` action requires raw protojson (literal enum names like `CONSTITUTION_LAYER_PROJECT`, proto field names like `referenceType`). The author-funnel stage tools have the identical defect (protojson `output` param). An MCP-only agent authors from its CLI/YAML mental model and silently fails.

**In scope:** the MCP write-input interface for constitution + author-funnel stages; MCP-first rewrite of all 7 embedded skills; prime-as-entry-point routing; an automated MCP-only e2e verification.

**Out of scope (own phases):** amend/supersede lifecycle semantics (Phase 7), conversation-recording enforcement (Phase 8), JIT display-name reconciliation (Phase 9). Requirements are fixed by ROADMAP.md — this discussion clarifies HOW, not WHAT.

</domain>

<decisions>
## Implementation Decisions

### Write-Input Interface (the #1002 open sub-question — DECIDED to keep open)
- **D-01:** **Reject raw protojson as the primary authoring interface.** The current `constitution.update` / `author.<stage>` protojson-blob input is NOT the right MCP authoring interface. The get→modify→update round-trip proposed in #1002 is also NOT adopted as-is — the user is not convinced protojson round-trip is the right format.
- **D-02:** **Research must evaluate two alternative directions and recommend one (or a blend):** (a) a **YAML / token-friendly whole-doc format** at the MCP write boundary, and (b) a **section-by-section tool signature** (e.g. granular actions like set-tech / add-principle / add-constraint / add-antipattern / set-process / add-reference) so the agent builds the constitution incrementally against small, self-describing schemas instead of hand-assembling a blob. The specific mechanism is deliberately left open for the research phase to score (token cost, self-teaching robustness, back-compat, implementation size).
- **D-03:** **The interface rethink covers BOTH surfaces:** the `constitution` tool AND the author-funnel stage outputs (`spark`/`shape`/`specify`/`decompose` protojson `output`). They share the same defect and must stay consistent; success criteria #2 requires completing the full funnel MCP-only, so the funnel cannot be left protojson-only.
- **D-04:** The existing forgiving mappers are relevant prior art for research, not a locked answer: `constitutionLayerFromString` (already used by `get`, bypassed by `update`), and a proposed `referenceTypeFromString`. Whether to wire them in depends on which interface direction wins.

### Skill Rewrite
- **D-05:** **Full MCP-first rewrite of all 7 embedded skills** (`specgraph-authoring`, `specgraph-constitution`, `specgraph-graph-query`, `specgraph-analytical-passes`, `specgraph-drift`, `specgraph-conventions`, `specgraph-troubleshooting`). Every skill leads with MCP tools/resources; the whole corpus gets a uniform MCP-first voice, not just the critical-path two.
- **D-06:** Skills must teach the chosen write-input pattern (from D-01/D-02) as *the* authoring pattern, so an MCP-only agent literally cannot get the input format wrong.

### CLI Treatment in Skills
- **D-07:** **Demote CLI to a clearly-gated "Requires local CLI" appendix** at the end of each skill — explicitly labeled so an MCP-only agent skips it. CLI docs are preserved for source/CLI users but never presented as the primary/first path. No co-equal CLI+MCP steps (that reintroduces the exact #1002 confusion).

### Verification
- **D-08:** **Automated MCP-only e2e test is the gate.** It drives the full path through the MCP surface only (`specgraph://prime` → skills fetch → tool calls) with the CLI unavailable, and asserts the constitution/spec reaches an approved/completed state. Fits the existing Ginkgo/Gomega e2e suite (`e2e/api/`). Regression-proof; no CLI fallback possible. (A manual walkthrough was considered and not required.)
- **D-09:** Success criterion #4 (skills_get/search return guidance that references the MCP tool path, verified against the embedded canonicals) should be covered by a content-level assertion — the existing `TestContentProtoDrift`-style check is the closest precedent.

### Prime Entry Point
- **D-10:** **`specgraph://prime` stays a state/orientation resource but is made a reliable ENTRY POINT** that clearly routes an MCP-only agent to the authoring skills (`specgraph_skills_list`/`get`/`search`) as the next step. Prime does NOT duplicate the round-trip/interface teaching inline — the skills carry the depth; prime routes to them. Minimal prime change.

### the agent's Discretion
- The exact write-input mechanism (YAML vs token-friendly vs section-by-section vs blend) is delegated to research per D-02 — research recommends, planning locks.
- Test harness details for simulating "MCP-only / no CLI" in the e2e (per D-08) are left to planning/implementation.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Driving requirement
- GitHub issue **#1002** (`https://github.com/specgraph/specgraph/issues/1002`) — full root-cause analysis, proposed direction, acceptance criteria, and the flagged forgiving-input/alt-format sub-question. The single most important ref for this phase.
- `.planning/ROADMAP.md` § "Phase 6: MCP Authoring Self-Teaching Path" — locked goal + 4 success criteria.
- `.planning/REQUIREMENTS.md` § MCP-01.

### MCP write-input surface (the interface to rethink — D-01..D-04)
- `internal/mcp/tools_core.go` — `constitutionTool` (`def`/`handle`/`handleGet`/`handleUpdate`), `constitutionLayerFromString`, `passTypeFromString`. `handleUpdate` does `protojson.Unmarshal` straight into the proto (the defect); `handleGet` already applies the friendly layer mapper.
- `internal/mcp/tools_authoring.go` — `authorTool` (`def` + `handleSpark`/`handleShape`/`handleSpecify`/`handleDecompose`/`handleApprove`/`handleAmend`/`handleSupersede`). Same protojson-`output` defect across the funnel stages.
- `internal/constitution/load/load.go` — friendly YAML→proto mapping already used by CLI import; prior art for a forgiving/friendly MCP write format.

### Skills corpus (rewrite all 7 — D-05..D-07)
- `internal/mcp/skills/embedded/specgraph-constitution/SKILL.md` — currently CLI-first (`specgraph constitution show`/`import`); the concrete #1002 symptom.
- `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` — already partly MCP-first (routes to MCP prompts + `author` tool); align to chosen interface.
- `internal/mcp/skills/embedded/specgraph-graph-query/SKILL.md`, `.../specgraph-analytical-passes/SKILL.md`, `.../specgraph-drift/SKILL.md`, `.../specgraph-conventions/SKILL.md`, `.../specgraph-troubleshooting/SKILL.md` — audit + rewrite MCP-first.
- `internal/mcp/skills/skills.go` + `internal/mcp/skills/embedded.go` — skill Source/serving; `internal/mcp/tools_skills.go` — `specgraph_skills_list`/`get`/`search`.

### Prime entry point (D-10)
- `internal/prime/composer.go`, `internal/prime/prime.go` — prime ProjectView composition.
- `internal/mcp/resources.go` — `specgraph://prime` resource handler + skills resource handler.

### Verification (D-08/D-09)
- `e2e/api/` — existing Ginkgo/Gomega e2e suite (`go test -tags e2e`) — home for the new MCP-only authoring e2e.
- Content-drift precedent: `TestContentProtoDrift` (see AGENTS.md "Authoring content" note) — pattern for the skill-content assertion.

### Repo conventions (from AGENTS.md)
- Skills are embedded canonicals under `internal/mcp/skills/embedded/`; repo-root `skills/` and `plugin/<harness>/` are reverse-symlinks — editing either edits the canonical. Serve via MCP, no on-disk end-user copies.
- Proto changes (if a new tool signature needs proto): edit `proto/`, run `task proto`, `gen/` is committed. Package doc comments required (revive). Apache-2.0 SPDX headers required. DCO sign-off required.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`constitutionLayerFromString` (tools_core.go:29)** — friendly-string→enum mapper already used by `get`; the template for any forgiving/friendly write mapping (and the proposed `referenceTypeFromString`).
- **`internal/constitution/load/load.go`** — existing YAML→proto load path (CLI import). If YAML/friendly-format wins (D-02), this is likely reusable at the MCP boundary rather than reimplemented.
- **`author.def()` schema (tools_authoring.go)** — the funnel tool's action/param dispatch pattern; a section-by-section redesign would extend this shape.
- **`specgraph_skills_*` tools + `specgraph://skills/<name>` resource** — serving path is done; only the served CONTENT changes.

### Established Patterns
- MCP tools are `ToolDef{Name, Description, Schema, Handler}` with `handle` dispatching on an `action` param; `stringParam`/`jsonResult`/`errResult`/`connectErrResult` helpers are the house style.
- Tool descriptions are agent-facing teaching surface (criteria #4) — the `constitution`/`author` tool `Description` + param docs must be rewritten alongside the skills.
- `TestContentProtoDrift`-style CI tests assert skill/content ↔ proto alignment — extend for the "skills reference the MCP path" assertion.

### Integration Points
- The chosen write-input mechanism must round-trip cleanly through the ConnectRPC handlers (`internal/server/`) and storage; if a new friendly format is added at the MCP boundary it maps to the same `UpdateConstitution`/`Spark`/`Shape`/... RPCs — no storage-schema change expected.
- Prime routing (D-10) connects `internal/prime` + `internal/mcp/resources.go` to the skills Source.

</code_context>

<specifics>
## Specific Ideas

- The user explicitly pushed back on #1002's protojson round-trip proposal: *"I'm not convinced that round trip or protojson is the right format here. We should explore yaml and/or some other token friendly format, or perhaps a change in tool signature to allow for setting section by section — door is wide open."* Research should treat this as a genuine open design comparison, not a rubber-stamp of the issue's proposal.
- Preference throughout: make the format **impossible for an MCP-only agent to get wrong**, and **token-lean**.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

### Concern flagged for research
- `specgraph://prime` **failed to load in the current session** (`"unable to load: internal: internal error"` in the session prime block). Since prime is the designated MCP-only entry point (D-10), research/planning MUST confirm prime resolves reliably. This may be a local no-server artifact — verify before assuming a code bug.

</deferred>

---

*Phase: 6-MCP Authoring Self-Teaching Path*
*Context gathered: 2026-07-14*
