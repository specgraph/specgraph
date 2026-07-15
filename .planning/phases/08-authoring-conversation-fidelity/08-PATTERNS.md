# Phase 8: Authoring Conversation Fidelity - Pattern Map

**Mapped:** 2026-07-15
**Files analyzed:** 12 (modified) + 2 (test-only) + 1 (deleted)
**Analogs found:** 14 / 14 (all in-repo; this is a close-the-holes refactor, every target has a verbatim sibling)

> **Framing for the planner:** Every file in this phase already has a working
> sibling in the *same file or package* that does exactly the target behavior for
> a different stage/action. There are **no "no analog" files** and **no
> RESEARCH.md-only patterns needed** — copy the reject branch into the accept
> branch, copy `handleShape` into `handleApprove`, copy `conversationRecordInput`
> into a `--conversation` loader, and delete the two dead paths. The proto field
> already exists (field 3); the change there is comment-only.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/server/authoring_handler.go` (Approve **accept** branch :453–485) | controller/handler | request-response (validate→tx→record) | **same file** — Approve **reject** branch :487–543 | exact (verbatim template, same func) |
| `proto/specgraph/v1/authoring.proto` (`ApproveRequest` :352–361) | proto/config | contract (comment-only) | **same message** — existing field 3 + reject-branch usage | exact (field exists; doc edit only) |
| `internal/mcp/tools_authoring.go` (`handleApprove` :332–344) | controller/tool-handler | request-response (thread exchanges) | **same file** — `handleShape` :230–262 (`parseOptionalExchanges` threading) | exact (sibling handler, same struct) |
| `internal/mcp/tools_authoring.go` (`authorTool.def` Description :115–177) | config/schema | static description | **same method** — existing `exchanges` prop :146–155 | exact (in-place doc flip) |
| `internal/mcp/tools_authoring.go` (`conversationTool` :420–500 — drop `record`) | controller/tool-handler | request-response (remove branch) | `findingsTool.handle` (`tools_core.go:170–178`), `sliceTool.handle` (`tools_execution.go:171–185`) — single-action switch shape | exact (list-only switch pattern) |
| `cmd/specgraph/authoring_cli_exchanges.go` (`cliSyntheticExchanges`) | utility | — (DELETE) | n/a — deletion | n/a |
| `cmd/specgraph/shape.go` / `specify.go` / `decompose.go` (`--conversation` loader) | controller/CLI-command | file-I/O → request-response | `conversationRecordInput`+`loadJSONFileRaw` (`conversation.go:60–91`, `util.go:37`) | role-match (loader reuse; shape mismatch — see Pitfall) |
| `cmd/specgraph/approve.go` (add `--conversation`, required) | controller/CLI-command | file-I/O → request-response | `shape.go` (post-loader form) + `conversation.go` loader | role-match |
| `cmd/specgraph/spark.go` (add `--conversation`, **optional**) | controller/CLI-command | file-I/O → request-response | `shape.go` + conditional-set pattern | role-match |
| `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` (:167–177) | docs/skill | static content | **same file** — "Required for shape/specify/decompose" block :167–169 | exact (in-file flip) |
| `internal/server/authoring_handler_test.go` | test | integration | reject-branch handler tests (same file) | role-match |
| `internal/storage/postgres/conversation_test.go` | test | integration | existing `RecordConversation` stage tests (same file) | role-match |
| `internal/mcp/tools_authoring_test.go` | test | unit (remove `TestConversationTool_Record*`) | existing action-handler unit tests (same file) | role-match |
| `e2e/api/mcp_only_conversation_test.go` (new) | test | e2e | `e2e/api/mcp_only_authoring_test.go` (`mcpProjectClient` harness) + `e2e/api/conversation_test.go` | role-match |

## Pattern Assignments

### `internal/server/authoring_handler.go` — Approve **accept** branch (D-02)

**Analog:** the Approve **reject** branch, `:487–543`, **in the same function** (`Approve`). It already validates + records exchanges atomically. Mirror it into the accept branch at `:453–485`.

**Validate-first pattern to add before the tx** (verbatim from reject `:488–491`):
```go
// Source: internal/server/authoring_handler.go:488-491 (reject branch)
if err := authoring.ValidateExchanges(req.Msg.GetConversationExchanges(), "approve"); err != nil {
    return nil, connect.NewError(connect.CodeInvalidArgument, err)
}
exchanges := exchangesFromProto(req.Msg.GetConversationExchanges())
```

**Entry construction + stage assignment** (reject `:498–517` — note the explicit stage set, Pitfall #2):
```go
// Source: internal/server/authoring_handler.go:498-517 (reject branch)
entry := storage.ConversationLogEntry{
    Exchanges:     exchanges,
    ExchangeCount: safeInt32(len(exchanges)),
    IsAmend:       false,
}
// ... inside the tx, after confirming stage:
// Record under the approved stage so the conversation is associated
// with the approval gate, not the current (decompose) stage.
entry.Stage = storage.SpecStageApproved
```
> Alternatively use the existing `buildConversationEntry(storage.SpecStageApproved, posture, exchanges)` helper (`:828`) — but note the reject branch builds the literal directly and sets `IsAmend:false`; `ApproveRequest` has no posture field, so pass `specv1.Posture_POSTURE_UNSPECIFIED` if using the helper.

**Record op to add into the EXISTING accept `runInTxOrSequential` block** (`:457–472`, which already holds `TransitionStage`, `acceptLinkedDecisions`, `GetSpec`). Add a fourth op, copied from reject `:520–525`:
```go
// Source: internal/server/authoring_handler.go:520-525 (reject branch) — add into accept tx
func(txCtx context.Context) error {
    if _, err := store.RecordConversation(txCtx, slug, entry); err != nil {
        return fmt.Errorf("record conversation: %w", err)
    }
    return nil
},
```

**Error handling:** unchanged — the accept branch already funnels tx errors through `h.stageError(err)` (`:473`), which preserves `connect.Code*` and maps sentinels (`ErrSpecNotFound`→NotFound, etc.). See `stageError` `:1002–1027`.

**Ordering note:** put `ValidateExchanges` + `exchangesFromProto` **before** `runInTxOrSequential` (outside the tx, matching reject) to keep lock time minimal (AGENTS.md: "validation that doesn't hit the DB stays outside").

---

### `proto/specgraph/v1/authoring.proto` — `ApproveRequest` (D-02, Pitfall #1)

**Analog:** the field already exists — **no new field**. Only the doc comment changes.

**Current** (`:358–360`):
```proto
// REQUIRED when action == APPROVE_ACTION_REJECT. Conversation exchanges
// capturing the rejection rationale.
repeated ConversationExchange conversation_exchanges = 3;
```
**Change:** reword the comment to state it is REQUIRED for both accept and reject (any non-default action). Keep `= 3`. Then run `task proto` (comment lives in the rawDesc; regen + commit `gen/`). `GetConversationExchanges()` already exists in `gen/` (the reject branch calls it).

---

### `internal/mcp/tools_authoring.go` — `handleApprove` (D-02)

**Analog:** `handleShape` `:230–262` (same file, same `authorTool` struct). It does `parseOptionalExchanges(params)` then threads the result onto the request as `ConversationExchanges`. `handleApprove` `:332–344` currently sends only `Slug`.

**Exchanges parse + thread** (adapt `handleShape:248–257`):
```go
// Source: internal/mcp/tools_authoring.go:248-257 (handleShape) — mirror into handleApprove
exchanges, exErr := parseOptionalExchanges(params)
if exErr != nil {
    return exErr, nil
}
if len(exchanges) == 0 { // approve REQUIRES exchanges (unlike optional-spark); friendlier than server rejection
    return errResult("exchanges is required for approve (JSON array of ConversationExchange)"), nil
}
resp, err := t.client.Authoring.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
    Slug:                  slug,
    ConversationExchanges: exchanges,
    // Action defaults to APPROVE_ACTION_UNSPECIFIED → server treats as ACCEPT.
}))
```
`parseOptionalExchanges` (`:71–82`) parses the `exchanges` string param in isolation (`{"exchanges":`+raw+`}`) to prevent JSON injection — reuse verbatim, do not hand-roll (V5 security control).

---

### `internal/mcp/tools_authoring.go` — `authorTool.def` Description (D-09, Pitfall #5)

**Analog:** the existing `exchanges` prop description, same method, `:146–155`. Flip the "not needed for approve" clause.

**Current** (`:148`):
```go
"required for shape/specify/decompose, optional for spark, not needed for approve. " +
```
**Change to:** `required for shape/specify/decompose/approve, optional for spark.` Also review the top-level Description sentence `:123–124` ("shape/specify/decompose also require `exchanges`") to include approve. Run `task skills:validate` + the content-drift test after.

---

### `internal/mcp/tools_authoring.go` — `conversationTool` remove `record` (D-06, Pattern 3)

**Analog:** single-action list-only tools — `findingsTool.handle` (`tools_core.go:170–178`) and the multi-action switches in `tools_execution.go` — show the canonical `switch action { case "list": ...; default: errResult("unknown action %q — valid: list") }` shape.

**Delete the `record` case from `handle`** (`:444–454`) → becomes:
```go
// Source: model on tools_core.go:170-178 (findingsTool.handle)
func (t *conversationTool) handle(ctx context.Context, params map[string]any) (*ToolResult, error) {
    action := stringParam(params, "action")
    switch action {
    case "list":
        return t.handleList(ctx, params)
    default:
        return errResult(fmt.Sprintf("unknown action %q — valid: list", action)), nil
    }
}
```
Also: delete `handleRecord` (`:456–485`); update `def()` `:427–439` — Description "Record and list…" → "List…", drop `"record"` from the `action` enum (`:432`), and drop the now-unused `stage`/`exchanges`/`is_amend` props (`:434–436`) except keep `stage` (it's a valid `list` filter — `handleList:494` uses it). **Keep `handleList` untouched** (criterion #4). Remove `TestConversationTool_Record*` unit tests (`tools_authoring_test.go:676+`) or the package won't compile.

---

### `cmd/specgraph/shape.go` / `specify.go` / `decompose.go` — `--conversation` loader (D-04/D-05, Pitfall #3)

**Analog:** `conversationRecordInput` struct + `loadJSONFileRaw` (`conversation.go:60–91`, `util.go:37`). This is the reuse target, but note the **shape mismatch**: `conversationRecordInput` expects an OBJECT `{"exchanges":[...]}`, while D-05 mandates the `--conversation` file be a bare JSON **array** `[...]` (same as the MCP `exchanges` param).

**Current CLI stamp to replace** (`shape.go:42`, identical at `specify.go:42`, `decompose.go:42`):
```go
ConversationExchanges: cliSyntheticExchanges("shape"),   // DELETE (D-04) — helper file removed
```

**Reuse the existing struct→proto mapping** (`conversation.go:82–91`, verbatim), but load into a bare slice for the array shape:
```go
// Model: conversation.go:82-91 (struct→proto), util.go:37 (loadJSONFileRaw)
// Chosen shape (D-05, A2): --conversation file is a JSON ARRAY of exchange objects.
type conversationExchangeInput struct {
    Role          string `json:"role"`
    Content       string `json:"content"`
    Stage         string `json:"stage"`
    Sequence      int32  `json:"sequence"`
    DecisionPoint bool   `json:"decision_point,omitempty"`
}
var input []conversationExchangeInput
if err := loadJSONFileRaw(conversationFile, &input); err != nil { // loadJSONFileRaw is CLI-only (util.go security note)
    return fmt.Errorf("shape: %w", err)
}
exchanges := make([]*specv1.ConversationExchange, len(input))
for i, e := range input {
    exchanges[i] = &specv1.ConversationExchange{
        Role: e.Role, Content: e.Content, Stage: e.Stage,
        Sequence: e.Sequence, DecisionPoint: e.DecisionPoint,
    }
}
```
**Flag registration** — model on `shape.go:24` / `conversation.go:44–49` (`StringVar` + `MarkFlagRequired`). For shape/specify/decompose/approve the flag is required; for spark it is optional (D-01). Support `--conversation -` for stdin (read `os.Stdin` when path is `-` before `loadJSONFileRaw`). Document the chosen array shape in flag help + error message (Pitfall #3). **Recommendation:** extract the load+map into one shared helper (e.g. `loadConversationFlag(path string) ([]*specv1.ConversationExchange, error)`) so all four commands share the single contract instead of copy-pasting.

---

### `cmd/specgraph/approve.go` — add required `--conversation` (D-05)

**Analog:** `approve.go:31–33` (current request build) + the shape.go loader form above. The CLI approve currently sends only `Slug`. Add the `--conversation` flag (required), load via the shared helper, and set `ConversationExchanges` on `ApproveRequest`. The server-side accept branch (above) enforces non-empty regardless.

---

### `cmd/specgraph/spark.go` — add optional `--conversation` (D-01/D-05)

**Analog:** `spark.go:33–36` + the conditional-set idiom already in `shape.go:34–38` (`if shapeJSONFile != "" { ... }`). Spark exchanges stay OPTIONAL: only set `ConversationExchanges` when the flag is provided; the server records spark conversations conditionally.

---

### `internal/mcp/skills/embedded/specgraph-authoring/SKILL.md` — approve teaching (D-09)

**Analog:** the "Required for shape / specify / decompose" block, same file, `:167–169`. Flip the approve block.

**Current** (`:176–177`):
```markdown
**Approve** needs only the `slug` (and explicit user sign-off). It does not
require an `output` or `exchanges` on a clean acceptance.
```
**Change:** approve now REQUIRES `exchanges` on clean acceptance (no `output`). Update `:167` to read "Required for shape / specify / decompose / **approve**". Remove any "then call `conversation record`" framing elsewhere in the file (grep for `conversation record`). Gate with `task skills:validate` + `TestContentProtoDrift` (backticked snake_case tokens must still map to proto fields).

---

## Shared Patterns

### Validation contract (V5 input validation)
**Source:** `authoring.ValidateExchanges` — `internal/authoring/validate.go:42`
**Apply to:** approve-accept handler (new), and it already backs shape/specify/decompose/reject. One contract for all surfaces.
```go
// internal/authoring/validate.go:42-48 — non-empty + role/content/length/stage/sequence gate
func ValidateExchanges(exchanges []*specv1.ConversationExchange, targetStage string) error {
    if len(exchanges) == 0 {
        return newValidationError("at least one exchange required")
    }
    if len(exchanges) > MaxConversationExchanges { /* 100 */ }
    // ... role∈{probe,response}, content ≤ 4096, stage match, strictly-increasing sequence
}
```
Handlers wrap the returned `*ValidationError` as `connect.NewError(connect.CodeInvalidArgument, err)`. Caps (`MaxConversationExchanges=100`, `MaxExchangeContentLen=4096`) are the DoS control — do not relax.

### Atomic record inside the stage transaction (ADR-004)
**Source:** `runInTxOrSequential` (`authoring_handler.go:988`) + `Store.RecordConversation` (`internal/storage/postgres/conversation.go:78`)
**Apply to:** approve-accept (add the record op to the existing block). Never record in a separate call — that is the exact #906 skip surface.
```go
// authoring_handler.go:988 — ops run in one RunInTransaction; any error rolls all back
func runInTxOrSequential(ctx context.Context, backend storage.TransactionalBackend, ops ...func(context.Context) error) error
```

### JSON exchange parsing in isolation (V5 / JSON-injection mitigation)
**Source:** `parseOptionalExchanges` (`tools_authoring.go:71`, MCP) and `loadJSONFileRaw` (`util.go:37`, CLI-only)
**Apply to:** `handleApprove` (reuse `parseOptionalExchanges` verbatim); CLI `--conversation` (reuse `loadJSONFileRaw`, CLI paths only — never in a server path per the `util.go` security note).

### Error handling / sanitization (V6 info-disclosure; test convention)
**Source:** `stageError` (`authoring_handler.go:1002`), `connectErrResult` (`internal/mcp/helpers.go:106`)
**Apply to:** all handler/tool paths. Tests assert `connect.CodeInvalidArgument` (not message strings, AGENTS.md Pitfall #4); mock backends return sentinel errors (`storage.ErrSpecNotFound`, …) so `errors.Is` in `stageError` fires.

### MCP tool action switch shape
**Source:** `findingsTool.handle` (`tools_core.go:170`), `claimTool.handle`/`sliceTool.handle` (`tools_execution.go`)
**Apply to:** `conversationTool.handle` after removing `record` — `switch action { case "list": …; default: errResult("unknown action %q — valid: list") }`.

## No Analog Found

None. Every target file has an in-repo (usually same-file / same-package) analog. This phase is wiring + deletion, not invention — RESEARCH.md confirms all primitives already ship.

## Metadata

**Analog search scope:** `internal/server/`, `internal/mcp/`, `internal/authoring/`, `internal/storage/postgres/`, `cmd/specgraph/`, `proto/specgraph/v1/`, `e2e/api/` (via codegraph_explore + targeted Read).
**Files scanned:** ~14 source files read in full or in targeted ranges; codegraph explored `authoring_handler.go`, `tools_authoring.go`, `validate.go`, `helpers.go`, `authoring.proto`, CLI stage commands.
**Pattern extraction date:** 2026-07-15
