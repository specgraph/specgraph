# Phase 7: Authoring Lifecycle Semantics - Pattern Map

**Mapped:** 2026-07-14
**Files analyzed:** 15 (2 created, 13 modified) + test-double sweep
**Analogs found:** 15 / 15 (in-repo analogs for every file; this is a consolidation refactor, so most analogs are sibling methods in the file being edited)

> **Build-on note:** `07-RESEARCH.md` already carries the exhaustive file:line blast radius and signature ripple. This document does **not** re-list those — it maps each new/modified file to the **single closest analog whose shape the executor should copy**, with the verbatim excerpt. Where RESEARCH gives the "what to change," PATTERNS gives the "copy this existing shape."

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/mcp/tools_authoring.go` (`handleAmend`/`handleSupersede`, `def()`) | MCP tool handler | request-response | `handleApprove`/`handleSpark` in same file + `cmd/specgraph/lifecycle.go` `runAmend` | exact (same file siblings) |
| `internal/storage/postgres/lifecycle.go` (claim release in `LifecycleAmendSpec`) | storage | transform (tx mutation) | `RecordCompletion` claim+edge delete (`execution.go:161-177`) + `GetActiveClaim` | exact |
| `internal/storage/postgres/lifecycle.go` (`reason` in `LifecycleSupersedeSpec`) | storage | transform (tx mutation) | `LifecycleAbandonSpec` `reason`→changelog in same file (`:246`) | exact (same file sibling) |
| `internal/storage/lifecycle.go` (`LifecycleSupersedeSpec` iface +reason) | storage interface | — | `LifecycleAmendSpec`/`LifecycleAbandonSpec` iface entries (same file `:111,117`) | exact |
| `internal/server/lifecycle_handler.go` (thread `msg.Reason`) | API handler | request-response | `TransitionAbandon` passing `msg.Reason` (same file `:127`) | exact (same file sibling) |
| `proto/specgraph/v1/lifecycle.proto` (`reason` on `TransitionSupersedeRequest`) | config/proto | — | `TransitionAbandonRequest.reason=2` (same file `:126`) | exact |
| `proto/specgraph/v1/authoring.proto` (delete Amend/Supersede RPCs+msgs) | config/proto | — | whole-type removal (no `reserved`) — see Shared Pattern "Proto removal" | role-match |
| `cmd/specgraph/lifecycle.go` (`--reason` flag on supersede) | CLI | request-response | `amendReason`/`abandonReason` flag wiring in same file (`:308,318`) | exact (same file sibling) |
| `internal/storage/postgres/authoring.go` (delete `AmendSpec`/`SupersedeSpec`) | storage | — | deletion; keep `TransitionStage` sibling | n/a (removal) |
| `internal/server/authoring_handler.go` (delete `Amend`/`Supersede`) | API handler | — | deletion | n/a (removal) |
| `internal/storage/authoring.go` (drop iface methods + `AmendResult`) | storage interface | — | deletion | n/a (removal) |
| `e2e/api/mcp_only_lifecycle_test.go` (NEW) | test | request-response | `e2e/api/mcp_only_authoring_test.go` (Phase 6 template) | exact |
| `internal/storage/postgres/lifecycle_test.go` (NEW claim-release + reason cases) | test | — | existing `lifecycle_test.go` amend/supersede cases | exact |
| `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` | doc/skill | — | own funnel-table + payload structure in same file | exact |
| `internal/mcp/skills/embedded/specgraph-troubleshooting/SKILL.md` | doc/skill | — | same-package troubleshooting entries | role-match |

---

## Pattern Assignments

### `internal/mcp/tools_authoring.go` — reroute `handleAmend`/`handleSupersede` + param renames (D-01/D-03/D-04/D-05/D-07)

**Analog A — the reroute call shape:** copy the exact `handleApprove` shape (same file, `:330-342`), which is the simplest Lifecycle-style handler (slug-guard → `t.client.<Service>.<Method>` → `connectErrResult`/`jsonResult`). The client field `t.client.Lifecycle` already exists (`internal/mcp/client.go:25`, wired `:43`) — swap `t.client.Authoring.Amend` → `t.client.Lifecycle.TransitionAmend`, no new plumbing.

Current (broken) `handleAmend` to replace — `tools_authoring.go:344-366`:
```go
func (t *authorTool) handleAmend(ctx context.Context, params map[string]any) (*ToolResult, error) {
	slug := stringParam(params, "slug")
	if slug == "" {
		return errResult("slug is required for amend"), nil
	}
	targetStageStr := stringParam(params, "target_stage")           // → rename re_entry_stage
	targetStage := specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED  // → drop enum conversion
	if targetStageStr != "" {
		targetStage = authoringStageFromString(targetStageStr)       // → delete helper (RESEARCH:162)
		...
	}
	resp, err := t.client.Authoring.Amend(ctx, connect.NewRequest(&specv1.AmendRequest{  // → Lifecycle.TransitionAmend
		Slug:        slug,
		Reason:      stringParam(params, "reason"),
		TargetStage: targetStage,
	}))
	if err != nil {
		return connectErrResult(err)
	}
	return jsonResult(resp.Msg), nil
}
```

Target shape (pass `re_entry_stage` string straight through — the handler validates it, RESEARCH Pitfall 5). Copy the required-field guard style from `handleSpark:201-207`:
```go
slug := stringParam(params, "slug")
if slug == "" { return errResult("slug is required for amend"), nil }
reEntry := stringParam(params, "re_entry_stage")
if reEntry == "" { return errResult("re_entry_stage is required (spark|shape|specify|decompose)"), nil }
reason := stringParam(params, "reason")
if reason == "" { return errResult("reason is required for amend"), nil }   // D-04
resp, err := t.client.Lifecycle.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
	Slug: slug, Reason: reason, ReEntryStage: reEntry,
}))
if err != nil { return connectErrResult(err) }
// D-05 next-step hint composed from reEntry — see Shared Pattern "Next-step hint".
return jsonResult(resp.Msg), nil
```

Request field names come verbatim from `TransitionAmendRequest{Slug, Reason, ReEntryStage}` — confirmed by `runAmend` (`cmd/specgraph/lifecycle.go:50-54`) and proto `lifecycle.proto:100-108`.

**Analog B — supersede shape:** current `handleSupersede:368-385` renames `superseded_by`→`new_slug`, reroutes to `Lifecycle.TransitionSupersede`, keeps `reason` optional. Copy `runSupersede` (`cmd/specgraph/lifecycle.go:79-82`) for the request shape:
```go
resp, err := t.client.Lifecycle.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
	Slug: slug, NewSlug: newSlug, Reason: stringParam(params, "reason"),  // Reason added by D-06 proto change
}))
```

**Analog C — tool `def()` param docs** (`:141-172`): the `props{}` block with `stringProp(...)` is the teaching surface. Rename the two params and rewrite their docs to teach land-one-before. Existing lines to edit:
```go
"target_stage":  stringProp("Target stage for amend: spark, shape, specify, decompose, approved"),  // → "re_entry_stage"
"superseded_by": stringProp("Slug of the replacement spec (required for supersede)"),                 // → "new_slug"
```
Mirror the verbose, example-laden `stringProp` style already used for `output`/`exchanges` (`:146-165`) when writing the land-one-before teaching.

---

### `internal/storage/postgres/lifecycle.go` — claim release in `LifecycleAmendSpec` (D-08)

**Analog:** the canonical two-statement claim+edge delete from `RecordCompletion` (`execution.go:161-177`), plus `GetActiveClaim` (`execution.go:326-350`) for the conditional agent lookup. Copy verbatim:
```go
// execution.go:162 — delete claim row (agent-scoped)
`DELETE FROM claims WHERE project_slug = $1 AND spec_slug = $2 AND agent = $3`
// execution.go:171 — delete CLAIMED_BY edge (from_slug = spec, to_slug = agent)
`DELETE FROM edges WHERE project_slug = $1 AND from_slug = $2 AND to_slug = $3 AND edge_type = 'CLAIMED_BY'`
```

**Placement:** inside the existing `RunInTransaction` in `LifecycleAmendSpec` (`lifecycle.go:55-107`), after the stage `UPDATE` guard passes (`:71`), before `recomputeContentHash` (`:83`). Use `s.exec(txCtx, ...)` (same `txCtx`) exactly as the sibling changelog/hash calls do. Conditional fetch pattern (unclaimed `approved` specs hold no claim → skip):
```go
claim, cErr := s.GetActiveClaim(txCtx, slug)   // returns nil for unclaimed (execution.go:337-338)
if cErr != nil { return fmt.Errorf("postgres: amend spec: get active claim: %w", cErr) }
if claim != nil {
	// two DELETEs above, args (s.project, slug, claim.Agent)
}
```
`slices` table untouched (deferred idea). `claims` PK is `(project_slug, spec_slug)` (migrations/001:145) → at most one row, no fan-out.

---

### `internal/storage/postgres/lifecycle.go` — `reason` in `LifecycleSupersedeSpec` (D-06)

**Analog:** `LifecycleAbandonSpec` (same file, `:246`) already threads a `reason` param straight into `ChangeLogEntry.Reason` (`:294`). Copy that precedence-free wiring, but apply the precedence decision (RESEARCH A2): prefer user `reason` when non-empty, else keep the existing default. The old-spec changelog default lives at `:217`:
```go
Reason:      fmt.Sprintf("Superseded by %s", newSlug),   // → reason if reason != "" else this default
```
Signature change: `LifecycleSupersedeSpec(ctx, oldSlug, newSlug string)` → `(ctx, oldSlug, newSlug, reason string)` (impl `:113`). `ChangeLogEntry.Reason` already exists (`changelog.go`) — **no migration** (RESEARCH:188).

---

### `internal/storage/lifecycle.go` — interface signature (D-06)

**Analog:** the sibling interface entries in the same block (`:111`, `:117`). Add `reason` to line 114 to match the impl:
```go
// current
LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*Spec, *Spec, error)
// →
LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug, reason string) (*Spec, *Spec, error)
```
Compile-time assertion `var _ storage.LifecycleBackend = (*Store)(nil)` (`postgres/lifecycle.go:19`) will flag every unfixed fake — see Shared Pattern "Fake backend sweep."

---

### `internal/server/lifecycle_handler.go` — thread `msg.Reason` (D-06)

**Analog:** `TransitionAbandon` (same file, `:127`) passes `msg.Reason` straight to the store. `TransitionSupersede` (`:95`) currently calls with two args; add the third:
```go
// current :95
oldSpec, newSpec, err := store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug)
// →
oldSpec, newSpec, err := store.LifecycleSupersedeSpec(ctx, msg.Slug, msg.NewSlug, msg.Reason)
```
No new validation needed — `reason` is optional for supersede (unlike amend, which already validates required `reason` at `:51`). Error mapping via `h.lifecycleError` (`:97`) is inherited.

---

### `proto/specgraph/v1/lifecycle.proto` — add `reason` (D-06)

**Analog:** `TransitionAbandonRequest` (same file, `:124-127`) shows the `reason` field shape. Field number `3` is free on `TransitionSupersedeRequest` (`slug=1`, `new_slug=2`):
```proto
message TransitionSupersedeRequest {
  string slug = 1;
  string new_slug = 2;
  // Optional. Reason recorded in the supersede changelog entry.
  string reason = 3;
}
```
Run `task proto` after; `gen/` is committed.

---

### `proto/specgraph/v1/authoring.proto` — delete Amend/Supersede RPCs + messages (D-02)

**Analog / correction:** whole-type + RPC-method removal, **NOT `reserved`** (RESEARCH "Proto removal semantics"). Delete `rpc Amend`/`rpc Supersede` (`:440-443`) and the four messages `AmendRequest`/`AmendResponse`/`SupersedeRequest`/`SupersedeResponse` (`:375-412`). The AGENTS.md `reserved` gotcha applies only to fields inside a retained message — not here. Run `task proto`, then `go build ./...`.

---

### `cmd/specgraph/lifecycle.go` — `--reason` flag on supersede (D-07)

**Analog:** `amendReason` flag wiring in the same file. Copy the var + `init()` registration shape:
```go
// var block near :72
var supersedeWith string
var supersedeReason string   // NEW

// runSupersede :79 — add Reason
resp, err := client.TransitionSupersede(cmd.Context(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
	Slug: args[0], NewSlug: supersedeWith, Reason: supersedeReason,
}))

// init() :314-315 — register (NOT required, unlike amend --reason at :309)
supersedeCmd.Flags().StringVar(&supersedeReason, "reason", "", "reason for supersession (optional)")
```
Note: amend's `--reason` is `MarkFlagRequired` (`:309`); supersede's is optional — do **not** mark it required.

---

### `e2e/api/mcp_only_lifecycle_test.go` (NEW) — MCP-only done→amend→re-author→supersede (D-10)

**Analog:** `e2e/api/mcp_only_authoring_test.go` (full file, 296 lines) is the exact template. Copy wholesale:
- `mcpProjectClient(ctx)` harness (`:49-72`) — verbatim; use a fresh `const` project slug (e.g. `mcp-only-lifecycle-project`) instead of `mcpOnlyProject` to keep DB isolation (see the per-Describe project rationale `:24-31`).
- The friendly-YAML fixtures block (`:91-139`) — reuse the spark/shape/specify/decompose YAML + exchanges verbatim to reach `approved`.
- The `author := func(args map[string]any)` helper (`:228-236`) and the `spark→shape→specify→decompose→approve` walk (`:238-242`).
- Assertion style: `res.IsError` for rejections (`:275`), `toolText(res)` / `ContainSubstring` for stage landings (`:257-258`).

**New sequence (beyond the template):** to reach `done` via MCP-only, chain the `claim` tool then `report` tool after `approve` (RESEARCH Code Examples; tool shape at `tools_execution.go:46-103`). `claim` args: `{action:"claim", spec_slug, agent}`. Then `report` completion. Then `author action=amend` (assert re-entry landing) and `author action=supersede` (assert done-only). Negative cases mirror the template's rejection `It(...)` blocks: amend-on-done rejected, supersede-on-non-done rejected — assert `res.IsError` `BeTrue()`.

---

### `internal/storage/postgres/lifecycle_test.go` (NEW cases) — claim-release + supersede reason (D-08/D-06)

**Analog:** existing amend/supersede integration cases in the same file (`lifecycle_test.go:19-320` per RESEARCH). New `TestLifecycleAmend_ReleasesClaim`: seed a spec to `in_progress`/`review`, claim it (via the store's claim path), amend, then assert the `claims` row **and** `CLAIMED_BY` edge are gone; also assert amending an unclaimed `approved` spec is a harmless no-op. Extend supersede cases to assert the user `reason` lands in the changelog `Reason`.

---

### Skills — `specgraph-authoring/SKILL.md` and/or `specgraph-troubleshooting/SKILL.md` (D-09)

**Analog:** the existing structured tables + "When this skill applies" sections in `specgraph-authoring/SKILL.md` (frontmatter `:1-8`, funnel table `:32-38`, invoke section `:40-57`). Add an amend/supersede/re-entry subsection using the same markdown-table + fenced-example style. Teach the land-one-before model verbatim as the user framed it (CONTEXT `<specifics>`): "`re_entry_stage: shape` means redo shape → spec lands at `spark` → `author action=shape` succeeds." Keep backticked tokens (`re_entry_stage`, `new_slug`, stage names) consistent — no existing skills drift-gate exists (RESEARCH A3), so correctness is on the author.

---

## Shared Patterns

### Proto removal (whole-type / RPC-method)
**Source:** RESEARCH "Proto removal semantics" (corrects AGENTS.md).
**Apply to:** `authoring.proto` Amend/Supersede deletion.
Delete the `rpc` lines and message blocks outright. **No `reserved`** for whole messages or service methods — proto3 has no such construct. `reserved` is only for fields removed from a *retained* message (not the case here). Sequence: reroute MCP first → `task proto` → delete impls/fakes last (RESEARCH Pitfall 1) to avoid build breaks.

### Next-step hint (D-05)
**Source:** intent of `NextPrompts`/`promptsToProto` in `authoring_handler.go`; compose a plain string in the MCP handler (no proto change — `TransitionAmendResponse` already returns `Spec`).
**Apply to:** `handleAmend` result. Compose from `re_entry_stage`, e.g. `"Amended; re-entering shape — call \`author action=shape\` next."` Append to the `jsonResult` payload text.

### Fake backend sweep (compile-blocking)
**Source:** compile-time assertions `var _ storage.LifecycleBackend = (*Store)(nil)` (`postgres/lifecycle.go:19`) and `var _ storage.AuthoringBackend = (*Store)(nil)` (`postgres/authoring.go:20`).
**Apply to:** every test double when changing `LifecycleSupersedeSpec` signature or removing `AmendSpec`/`SupersedeSpec`. RESEARCH "Blast Radius" tables enumerate all of them (`fakeAuthoringBackend`, `authoringTestBackend`, `stubBackend`, `errorBackend`, `fakeLifecycleBackend`, MCP `mockAuthoringService`). Run `go build ./... && go vet ./...` after each interface edit.

### Storage tx mutation idiom
**Source:** every `Lifecycle*Spec` method in `postgres/lifecycle.go`.
**Apply to:** claim-release insert. Pattern = `RunInTransaction(ctx, func(txCtx)…)` → `GetSpec(txCtx)` pre-read → version-guarded `UPDATE … WHERE version = $n` → `preconditionError` on 0 rows → `recomputeContentHash(txCtx)` → `createChangeLog(txCtx)`. All sub-calls take `txCtx`, never `ctx` (ADR-004). New DELETEs join the same `txCtx`.

### Handler error assertions use codes
**Source:** AGENTS.md; `lifecycle_handler.go` `lifecycleError` (`:275+`).
**Apply to:** all rewritten amend/supersede tests. Assert `connect.CodeOf(err)` / `res.IsError`, never error message strings. Mock backends return sentinel errors (`storage.ErrSpecNotDone`, `storage.ErrSpecNotAmendable`), never `fmt.Errorf` (RESEARCH Pitfalls 3, 4).

### Required project scope inherited free
**Source:** `scopeStore(ctx, h.scoper)` at the top of every lifecycle handler (`lifecycle_handler.go:43`).
**Apply to:** rerouted MCP tool automatically inherits `X-Specgraph-Project` enforcement — no MCP-side scope code needed.

---

## No Analog Found

None. Every file has an in-repo analog (this is a consolidation refactor against an already-shipped correct lifecycle path). The only genuinely "new" mechanics — claim-release inside amend and the supersede `reason` — are copy-shape from `RecordCompletion`/`GetActiveClaim` and `LifecycleAbandonSpec` respectively, both cited above.

## Deletions (no pattern to copy — remove cleanly)

| File | Symbols to delete | Sibling to preserve |
|------|-------------------|---------------------|
| `internal/storage/postgres/authoring.go` | `AmendSpec` (`:363`), `SupersedeSpec` (`:297`) | `TransitionStage` (`:27`) — powers funnel + approve; do NOT touch |
| `internal/server/authoring_handler.go` | `Amend` (`:551`), `Supersede` (`:593`) | funnel handlers + `NextPrompts`/`promptsToProto` |
| `internal/storage/authoring.go` | `AuthoringSpecLifecycle` methods / interface, `AmendResult` (`:136`) | rest of `AuthoringBackend` |
| `internal/storage/stage_validation.go` | `ValidateAmendTransition` (`:78`) — orphaned after `AmendSpec` deletion (optional cleanup) | `ValidateTransition` |
| `internal/mcp/tools_authoring.go` | `authoringStageFromString` (`:111`) — unused for amend after D-03 | `postureFromString` |

## Metadata

**Analog search scope:** `internal/mcp/`, `internal/storage/postgres/`, `internal/storage/`, `internal/server/`, `cmd/specgraph/`, `proto/specgraph/v1/`, `e2e/api/`, `internal/mcp/skills/embedded/`.
**Files read this session:** `tools_authoring.go`, `client.go`, `postgres/lifecycle.go`, `postgres/execution.go`, `cmd/specgraph/lifecycle.go`, `server/lifecycle_handler.go`, `lifecycle.proto`, `authoring.proto`, `storage/lifecycle.go`, `e2e/api/mcp_only_authoring_test.go`, `specgraph-authoring/SKILL.md`, `tools_execution.go`.
**Pattern extraction date:** 2026-07-14
