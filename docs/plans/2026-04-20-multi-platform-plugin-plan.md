# Multi-Platform Plugin — Phase A Implementation Plan (revised)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate SpecGraph's per-platform workflow intelligence out of the 13-skill Claude Code plugin and into server-embedded content composed dynamically into MCP prompt responses. Add server-side atomic coupling of stage persist + conversation recording by extending the existing `runInTxOrSequential` handler pattern with a recorded-conversation op. Ship a thin Claude Code plugin consuming the new path. Produce empirical verification findings that unblock Plan B (Cursor/OpenCode/Codex plugins).

**Architecture:** `authoring.Composer` loads stable content via `//go:embed` and composes with dynamic state (constitution, spec, related specs) into rich MCP prompt and tool responses. Proto adds `conversation_exchanges` to stage requests; handlers call `store.RecordConversation` inside the existing `runInTxOrSequential` pattern as a fourth op, joining the transaction via pgx context threading (RunInTransaction detects the existing tx and reuses it). No new storage interface method required beyond composing `ConversationBackend` into `AuthoringBackend`. Claude Code plugin reduces from 13 skills to MCP config + session-start hook + routing guide.

**Tech Stack:** Go, protobuf (buf), ConnectRPC, pgx v5, `github.com/mark3labs/mcp-go`, `//go:embed`, Taskfile, jj colocated with git.

**Design doc:** `docs/plans/2026-04-20-multi-platform-plugin-design.md`.

**Scope boundaries:**

- **In scope (Plan A)**: server-side composer, content migration, atomic coupling handler integration, `specgraph://prime` resource, `author.start_stage` tool, `ProfileFromClientInfo` extension, posture-absent warning, posture recording on conversation log, composer observability, thin Claude Code plugin, CLAUDE.md update, dogfood cutover, empirical verification of Cursor/OpenCode/Codex.
- **Out of scope (Plan B, future)**: Cursor, OpenCode, Codex platform plugins — blocked on empirical verification deliverables produced by this plan.
- **Out of scope (Phase 2, future)**: telemetry-driven server-side enforcement beyond required-argument coupling.

---

## Prerequisites

Refresh the jj workspace so main-branch MCP code is present.

```bash
jj workspace update-stale
jj --no-pager log -r '@-' --no-graph --limit 1
# expect current main commit (post-PR 898)
ls internal/mcp/
# expect: client.go convert.go helpers.go profiles.go prompts.go resources.go server.go tools_*.go types.go
```

If `internal/mcp/` is absent, workspace is more than update-stale behind; run `jj git fetch && jj rebase -d main@origin`.

---

## File Structure

### New files

```text
internal/authoring/
├── content.go                     # //go:embed declarations
├── content/
│   ├── persona.md
│   ├── orchestration.md
│   ├── conversation-recording.md
│   ├── quality-heuristics.md
│   ├── stage-spark.md
│   ├── stage-shape.md
│   ├── stage-specify.md
│   ├── stage-decompose.md
│   └── stage-approve.md
├── composer.go                    # authoring.Composer type
├── composer_test.go
├── composer_golden_test.go
├── drift_test.go                  # content/proto drift check
├── validate.go                    # ValidateExchanges
├── validate_test.go
└── testdata/golden/*.md

internal/mcp/
├── composer_adapter.go            # composerBackend: adapts Client → authoring.ComposerBackend

cmd/specgraph/
├── mcp_read_resource.go           # new subcommand

plugin/specgraph/
├── routing-guide.md               # stable meta-knowledge routing guide

docs/verification/
├── cursor-mcp-verification.md
├── opencode-mcp-verification.md
└── codex-mcp-verification.md
```

### Modified files

- `proto/specgraph/v1/authoring.proto` — add `conversation_exchanges` field to `ShapeRequest`, `SpecifyRequest`, `DecomposeRequest`, `SparkRequest`; add `action` + `conversation_exchanges` to `ApproveRequest`
- `gen/specgraph/v1/authoring.pb.go` — regenerated
- `internal/storage/authoring.go` — extend `AuthoringBackend` to compose `ConversationBackend`
- `internal/server/authoring_handler.go` — add exchange validation + 4th op for conversation recording in existing `runInTxOrSequential` calls; redefine `Approve` reject path
- `internal/server/authoring_handler_test.go` — add cases for all new paths
- `internal/mcp/prompts.go` — delegate to composer
- `internal/mcp/tools_authoring.go` — register `author.start_stage`
- `internal/mcp/resources.go` — add `specgraph://prime`
- `internal/mcp/profiles.go` — add `opencode`, `codex`
- `plugin/specgraph/.claude-plugin/plugin.json` — version bump
- `plugin/specgraph/hooks/session-start.sh` — read MCP resource
- `CLAUDE.md` — update skill-references section to reflect thin-plugin layout
- `e2e/api/` — rewrite authoring tests
- `plugin/specgraph/skills/` — **deleted**

### Deleted

- `plugin/specgraph/skills/` (entire tree)

---

## Task 1: Proto — add `conversation_exchanges` and Approve `action`

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto:239-335`
- Regenerate: `gen/specgraph/v1/authoring.pb.go`

- [ ] **Step 1: Edit proto**

Append to the existing messages. Do not renumber existing fields.

```protobuf
message SparkRequest {
  string slug = 1;
  SparkOutput output = 2;
  Posture posture = 3;
  // Conversation exchanges captured during spark elicitation. Optional for
  // spark (seed may be a single --seed argument); validated if present.
  repeated ConversationExchange conversation_exchanges = 4;
}

message ShapeRequest {
  string slug = 1;
  ShapeOutput output = 2;
  Posture posture = 3;
  // REQUIRED. Conversation exchanges for shape elicitation. Server persists
  // stage output and conversation log atomically via the existing
  // runInTxOrSequential pattern.
  repeated ConversationExchange conversation_exchanges = 4;
}

message SpecifyRequest {
  string slug = 1;
  SpecifyOutput output = 2;
  Posture posture = 3;
  // REQUIRED. See ShapeRequest.conversation_exchanges.
  repeated ConversationExchange conversation_exchanges = 4;
}

message DecomposeRequest {
  string slug = 1;
  DecomposeOutput output = 2;
  Posture posture = 3;
  // REQUIRED. See ShapeRequest.conversation_exchanges.
  repeated ConversationExchange conversation_exchanges = 4;
}

message ApproveRequest {
  string slug = 1;
  // Approval action. "accept" (default) advances the spec to approved;
  // "reject" records a rejection finding and conversation but does not
  // transition the stage.
  string action = 2;
  // REQUIRED when action == "reject". Conversation exchanges capturing the
  // rejection rationale.
  repeated ConversationExchange conversation_exchanges = 3;
}
```

- [ ] **Step 2: Regenerate proto**

Run: `task proto`
Expected: `gen/specgraph/v1/authoring.pb.go` regenerated. No errors.

- [ ] **Step 3: Run `buf lint`**

Run: `task lint` (covers buf lint per Taskfile) or directly: `buf lint ./proto/specgraph/v1/authoring.proto`
Expected: PASS.

- [ ] **Step 4: Run `go build`**

Run: `go build ./...`
Expected: PASS (generated struct fields are additive). If existing tests fail with field-not-found errors, leave them — Task 4+ addresses.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(proto): add conversation_exchanges to stage requests, action to Approve

Adds required conversation_exchanges on Shape/Specify/Decompose requests,
optional on Spark, and action+conversation_exchanges on Approve (action=reject
requires exchanges). Wiring into handlers lands in subsequent commits.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 2: Pure validation function for conversation exchanges

**Files:**

- Create: `internal/authoring/validate.go`
- Create: `internal/authoring/validate_test.go`

Design-aligned rules (per design doc §Validation):

- Non-empty array
- Each exchange has non-empty role (must be `"probe"` or `"response"`) and non-empty content
- Each exchange's `stage` (when set) must equal the target authoring stage
- Sequences must be **strictly increasing** (design wording)

- [ ] **Step 1: Write failing tests**

Create `internal/authoring/validate_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"errors"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestValidateExchanges(t *testing.T) {
	tests := []struct {
		name            string
		exchanges       []*specv1.ConversationExchange
		stage           string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "empty rejected",
			exchanges:       nil,
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "at least one exchange",
		},
		{
			name: "missing role rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "", Content: "x", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "role",
		},
		{
			name: "unknown role rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "narrator", Content: "x", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "role",
		},
		{
			name: "missing content rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "content",
		},
		{
			name: "mismatched stage rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "x", Stage: "spark", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "stage",
		},
		{
			name: "non-increasing sequence rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "q1", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "r1", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "sequence",
		},
		{
			name: "valid strictly-increasing accepted",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "what is scope?", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "X and Y in; Z out", Stage: "shape", Sequence: 2},
				{Role: "probe", Content: "risks?", Stage: "shape", Sequence: 3},
				{Role: "response", Content: "none", Stage: "shape", Sequence: 4},
			},
			stage: "shape",
		},
		{
			name: "missing stage field accepted when target provided",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "q", Stage: "", Sequence: 1},
			},
			stage: "shape",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExchanges(tt.exchanges, tt.stage)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExchanges err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantErrContains != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("err %q does not contain %q", err.Error(), tt.wantErrContains)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

Run: `go test ./internal/authoring/ -run TestValidateExchanges -v`
Expected: FAIL with "undefined: ValidateExchanges".

- [ ] **Step 3: Implement validator**

Create `internal/authoring/validate.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// ValidationError indicates conversation_exchanges failed a structural check.
type ValidationError struct {
	Reason string
}

func (e *ValidationError) Error() string { return "conversation_exchanges: " + e.Reason }

// ValidateExchanges enforces the structural invariants from the design doc:
// non-empty, role in {"probe","response"}, non-empty content, stage (when set)
// matches target, sequence strictly increasing.
//
// targetStage is the authoring stage of the call ("shape", "specify", etc.).
// An empty stage field on an exchange is treated as unspecified and accepted.
func ValidateExchanges(exchanges []*specv1.ConversationExchange, targetStage string) error {
	if len(exchanges) == 0 {
		return &ValidationError{Reason: "at least one exchange required"}
	}

	var lastSeq int32
	seenAny := false

	for i, ex := range exchanges {
		if ex == nil {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] is nil", i)}
		}
		role := ex.GetRole()
		if role == "" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] missing role", i)}
		}
		if role != "probe" && role != "response" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] role %q must be one of: probe, response", i, role)}
		}
		if ex.GetContent() == "" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] missing content", i)}
		}
		if s := ex.GetStage(); s != "" && s != targetStage {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] stage %q does not match target stage %q", i, s, targetStage)}
		}
		seq := ex.GetSequence()
		if seenAny && seq <= lastSeq {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] sequence %d is not strictly greater than previous %d", i, seq, lastSeq)}
		}
		lastSeq = seq
		seenAny = true
	}

	return nil
}
```

- [ ] **Step 4: Run test, verify it passes**

Run: `go test ./internal/authoring/ -run TestValidateExchanges -v`
Expected: PASS for all subtests.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(authoring): validate conversation_exchanges structural invariants

Adds ValidateExchanges enforcing non-empty, role in {probe,response},
non-empty content, stage tag match, strictly-increasing sequence. Consumed
by stage handlers in subsequent commits.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 3: Extend `AuthoringBackend` to compose `ConversationBackend`

**Files:**

- Modify: `internal/storage/authoring.go:163-169`
- Test: existing tests in `internal/server/authoring_handler_test.go` should keep compiling — no functional change

The handler's `store` variable is typed `storage.AuthoringBackend`. To call `store.RecordConversation` inside the existing `runInTxOrSequential` pattern, the interface must expose it. `ConversationBackend` already exists (`internal/storage/conversation.go:52-67`). Compose it into `AuthoringBackend`.

- [ ] **Step 1: Edit interface**

Edit `internal/storage/authoring.go` around line 166:

```go
// AuthoringBackend composes all authoring storage operations.
// Implementations satisfy all sub-interfaces.
// All methods accept domain types defined in this package, not protobuf types.
type AuthoringBackend interface {
	StageWriter
	AuthoringSpecLifecycle
	ConversationBackend
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: PASS. Postgres `Store` already satisfies `ConversationBackend` (it implements `RecordConversation` and friends per `internal/storage/postgres/conversation.go`).

- [ ] **Step 3: Run existing tests**

Run: `task test`
Expected: PASS — no functional change, only interface composition.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "refactor(storage): compose ConversationBackend into AuthoringBackend

AuthoringBackend now exposes RecordConversation/ListConversations alongside
stage and lifecycle operations. Enables handlers to call RecordConversation
inside the existing runInTxOrSequential transaction pattern as a fourth op.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 4: Shape handler — validate exchanges and atomic-record as 4th op

**Files:**

- Modify: `internal/server/authoring_handler.go:102-171`
- Modify: `internal/server/authoring_handler_test.go`

Add:

- Validation call before `runInTxOrSequential`
- Exchange conversion helper (proto → domain value-struct)
- Fourth op in `runInTxOrSequential` calling `store.RecordConversation(txCtx, slug, ConversationLogEntry{...})`

The fourth op joins the existing transaction because `RunInTransaction` detects the tx in context and reuses it (see `internal/storage/postgres/tx.go:33-44`).

- [ ] **Step 1: Add conversion helper**

In `internal/server/authoring_handler.go`, add a package-level helper near `shapeOutputToDomain`:

```go
// exchangesFromProto converts proto ConversationExchange messages to domain
// value-struct exchanges. Unknown role strings pass through; role validation
// happens in authoring.ValidateExchanges before persist.
func exchangesFromProto(ps []*specv1.ConversationExchange) []storage.ConversationExchange {
	out := make([]storage.ConversationExchange, len(ps))
	for i, p := range ps {
		out[i] = storage.ConversationExchange{
			Role:          storage.ConversationRole(p.GetRole()),
			Content:       p.GetContent(),
			Stage:         p.GetStage(),
			Sequence:      p.GetSequence(),
			DecisionPoint: p.GetDecisionPoint(),
		}
	}
	return out
}

// buildConversationEntry constructs a ConversationLogEntry for RecordConversation.
// posture is persisted alongside exchanges for future drift detection (design §Posture).
func buildConversationEntry(stage storage.SpecStage, posture specv1.Posture, exchanges []storage.ConversationExchange, isAmend bool) storage.ConversationLogEntry {
	return storage.ConversationLogEntry{
		Stage:         stage,
		Exchanges:     exchanges,
		ExchangeCount: int32(len(exchanges)),
		IsAmend:       isAmend,
		// Posture recording: when ConversationLogEntry gains a Posture field
		// (Task 10), wire it here. For now, captured as a metadata label if
		// backend supports it.
	}
}
```

- [ ] **Step 2: Write failing handler tests**

Add to `internal/server/authoring_handler_test.go`:

```go
func TestAuthoringHandler_Shape_RequiresConversationExchanges(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh", specv1.AuthoringStage_AUTHORING_STAGE_SHAPE)

	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "oauth-refresh",
		Output: validShapeOutput(),
		// conversation_exchanges omitted
	}))
	if err == nil {
		t.Fatal("expected error for missing exchanges")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Shape_RejectsEmptyExchanges(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh", specv1.AuthoringStage_AUTHORING_STAGE_SHAPE)

	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:                  "oauth-refresh",
		Output:                validShapeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Shape_PersistsAtomicallyWithExchanges(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh", specv1.AuthoringStage_AUTHORING_STAGE_SHAPE)

	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "oauth-refresh",
		Output: validShapeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "X in", Stage: "shape", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Shape: %v", err)
	}
	if resp.Msg.GetOutput() == nil {
		t.Error("expected output echoed back")
	}

	logs, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "oauth-refresh",
		Stage: "shape",
	}))
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 conversation log, got %d", len(logs.Msg.GetConversationLogs()))
	}
}

// validShapeOutput returns a minimally-valid ShapeOutput for tests.
func validShapeOutput() *specv1.ShapeOutput {
	return &specv1.ShapeOutput{
		ScopeIn:        []string{"X"},
		ScopeOut:       []string{"Y"},
		Approaches:     []*specv1.Approach{{Name: "a", Description: "d", Tradeoffs: "t"}},
		ChosenApproach: "a",
		Risks:          []string{"r"},
		SuccessMust:    []string{"m"},
	}
}
```

If `mustPrepareSpecAtStage` / `newTestClient` helpers don't exist, scan existing authoring-handler tests for equivalents; follow the pattern.

- [ ] **Step 3: Run tests, verify they fail**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Shape_ -v`
Expected: FAIL — handler doesn't validate or record yet.

- [ ] **Step 4: Update Shape handler**

Replace the body of `Shape` in `internal/server/authoring_handler.go:103-171`. The change adds validation before `runInTxOrSequential` and a fourth op that records the conversation in the same transaction:

```go
func (h *AuthoringHandler) Shape(ctx context.Context, req *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if msg.Output == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output is required"))
	}
	// Required conversation_exchanges per design §Conversation Recording Coupling.
	if err := authoring.ValidateExchanges(msg.GetConversationExchanges(), "shape"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// Existing field-size validations (preserved verbatim from current handler).
	for _, v := range []struct {
		name  string
		items []string
	}{
		{"scope_in", msg.Output.GetScopeIn()},
		{"scope_out", msg.Output.GetScopeOut()},
		{"risks", msg.Output.GetRisks()},
		{"success_must", msg.Output.GetSuccessMust()},
		{"success_should", msg.Output.GetSuccessShould()},
		{"success_wont", msg.Output.GetSuccessWont()},
	} {
		if err := validateStringSlice(v.name, v.items, maxElements, maxFieldLen); err != nil {
			return nil, err
		}
	}
	if len(msg.Output.GetApproaches()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("approaches exceeds maximum of %d elements", maxElements))
	}
	if len(msg.Output.GetDecisions()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("decisions exceeds maximum of %d elements", maxElements))
	}
	shapeDomain, err := shapeOutputToDomain(msg.Output)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	scope := make([]string, 0, len(msg.Output.GetScopeIn())+len(msg.Output.GetScopeOut()))
	scope = append(scope, msg.Output.GetScopeIn()...)
	scope = append(scope, msg.Output.GetScopeOut()...)
	safetyInput := &authoring.SafetyInput{
		Text:  strings.Join(msg.Output.GetRisks(), " "),
		Scope: scope,
	}
	var safetyFlags []authoring.SafetyFlagResult
	exchanges := exchangesFromProto(msg.GetConversationExchanges())
	entry := buildConversationEntry(storage.SpecStageShape, msg.GetPosture(), exchanges, false /* isAmend */)

	// Posture-absent warning (design §Posture).
	if msg.GetPosture() == specv1.Posture_POSTURE_UNSPECIFIED {
		h.logger.Warn("posture-absent", slog.String("stage", "shape"), slog.String("slug", msg.Slug))
	}

	// Four-op transaction: transition → store output → safety → record conversation.
	// All four land in one transaction via RunInTransaction's context threading;
	// any op failing rolls back all prior writes.
	if err := runInTxOrSequential(ctx, store,
		func(c context.Context) error {
			return store.TransitionStage(c, msg.Slug, storage.SpecStageSpark, storage.SpecStageShape)
		},
		func(c context.Context) error {
			return store.StoreShapeOutput(c, msg.Slug, shapeDomain)
		},
		func(c context.Context) error {
			var err error
			safetyFlags, err = persistSafetyFlags(c, store, msg.Slug, safetyInput)
			return err
		},
		func(c context.Context) error {
			_, err := store.RecordConversation(c, msg.Slug, entry)
			return err
		},
	); err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: safetyResultsToProto(safetyFlags),
		NextPrompts: promptsToProto(authoring.StageSpecify),
	}), nil
}
```

Key points:

- All existing field-size validations preserved verbatim
- `runInTxOrSequential` call gains a fourth op (RecordConversation)
- `h.stageError(err)` stays single-arg (matches existing signature at line 861)
- Posture-absent warning logged (addresses design §Posture)
- `RecordConversation` inside the tx joins via context per `RunInTransaction` semantics

- [ ] **Step 5: Run tests, verify they pass**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Shape_ -v`
Expected: PASS.

- [ ] **Step 6: Run full server tests for regressions**

Run: `go test ./internal/server/ -v`
Expected: PASS. Existing Shape tests may have relied on the old no-exchanges flow — update them to pass valid exchanges.

- [ ] **Step 7: Commit**

```bash
jj --no-pager commit -m "feat(authoring): Shape validates exchanges and atomic-records in same tx

Wires authoring.ValidateExchanges into the Shape handler and adds a fourth
op to the existing runInTxOrSequential call invoking RecordConversation.
The conversation log lands in the same transaction as TransitionStage,
StoreShapeOutput, and persistSafetyFlags; any failure rolls back all four.

Emits posture-absent warning when Posture is unspecified.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 5: Specify handler — same pattern

**Files:**

- Modify: `internal/server/authoring_handler.go:174-268`
- Modify: `internal/server/authoring_handler_test.go`

Mirror the Task 4 changes on the `Specify` handler: validate with `targetStage="specify"`, add fourth op with `storage.SpecStageSpecify`, emit posture-absent warning with `stage="specify"`.

- [ ] **Step 1: Write failing tests**

Same structure as Task 4, three tests:

- `TestAuthoringHandler_Specify_RequiresConversationExchanges`
- `TestAuthoringHandler_Specify_RejectsEmptyExchanges`
- `TestAuthoringHandler_Specify_PersistsAtomicallyWithExchanges`

Use `validSpecifyOutput()` helper returning a minimal valid `SpecifyOutput` (interfaces with at least one non-empty name, verify criteria, invariants, touches).

- [ ] **Step 2: Run, verify fail; update handler; run, verify pass**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Specify_ -v`
Expected FAIL, then after update, PASS.

Handler change mirrors Task 4 Step 4. Key differences:

- `ValidateExchanges(msg.GetConversationExchanges(), "specify")`
- `TransitionStage(c, msg.Slug, storage.SpecStageShape, storage.SpecStageSpecify)`
- `buildConversationEntry(storage.SpecStageSpecify, ...)`
- Preserve the existing interface/verify/invariants/touches validations

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "feat(authoring): Specify validates exchanges and atomic-records in same tx

Mirrors Shape handler changes on Specify. conversation_exchanges required
with stage='specify'; exchanges recorded atomically alongside stage output
and safety flags.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 6: Decompose handler — same pattern

**Files:**

- Modify: `internal/server/authoring_handler.go:271-345` (approximate; scan file for `Decompose` handler)
- Modify: `internal/server/authoring_handler_test.go`

Same shape as Tasks 4 and 5. Targets `"decompose"` for validation and `storage.SpecStageDecompose`.

Decompose's existing op returns `sliceSlugs`. Preserve that — the safety-op already captures it via closure. The fourth op (RecordConversation) doesn't interfere.

- [ ] **Step 1: Write failing tests**

Three tests mirroring Task 4. `validDecomposeOutput()` helper returning a minimal valid `DecomposeOutput`.

- [ ] **Step 2: Run, verify fail; update handler; run, verify pass**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Decompose_ -v`

Handler update follows Shape/Specify pattern. Preserve the `SliceSlugs` capture into the response.

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "feat(authoring): Decompose validates exchanges and atomic-records in same tx

Mirrors Shape and Specify on Decompose. conversation_exchanges required with
stage='decompose'; records atomically with stage output, safety flags, and
slice creation.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 7: Spark handler — optional exchanges

**Files:**

- Modify: `internal/server/authoring_handler.go:30-100` (approximate; the Spark handler at top of file)
- Modify: `internal/server/authoring_handler_test.go`

Spark accepts optional `conversation_exchanges`. When absent, no validation and no fourth op. When present, validate with `targetStage="spark"` and record via a fourth op.

- [ ] **Step 1: Write failing tests**

```go
func TestAuthoringHandler_Spark_ExchangesOptional(t *testing.T) {
	client, _ := newTestClient(t)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec",
		Output: &specv1.SparkOutput{Seed: "seed", Signal: "signal"},
	}))
	if err != nil {
		t.Fatalf("Spark without exchanges: %v", err)
	}
}

func TestAuthoringHandler_Spark_ExchangesValidatedWhenPresent(t *testing.T) {
	client, _ := newTestClient(t)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec-2",
		Output: &specv1.SparkOutput{Seed: "seed"},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "", Content: "x", Stage: "spark", Sequence: 1},
		},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Spark_RecordsExchangesWhenPresent(t *testing.T) {
	client, _ := newTestClient(t)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec-3",
		Output: &specv1.SparkOutput{Seed: "seed"},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "x", Stage: "spark", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Spark: %v", err)
	}
	logs, _ := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "new-spec-3",
		Stage: "spark",
	}))
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 spark conversation log")
	}
}
```

- [ ] **Step 2: Run, verify fail; update handler; run, verify pass**

In Spark handler, add an optional-exchanges block:

```go
// Conditional conversation_exchanges per design §Approve/spark coupling table.
if exs := msg.GetConversationExchanges(); len(exs) > 0 {
	if err := authoring.ValidateExchanges(exs, "spark"); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
}
```

Build entry if exchanges present; add a fourth op in the runInTxOrSequential call only when exchanges present. Two approaches:

1. Append the op conditionally to an `ops` slice, call `runInTxOrSequential(ctx, store, ops...)`
2. Duplicate the runInTxOrSequential branches: with-exchanges and without-exchanges

Approach 1 is cleaner:

```go
ops := []func(context.Context) error{
	func(c context.Context) error {
		return store.CreateSpec(c, msg.Slug, ...)  // preserve existing spark-specific ops
	},
	// ... other existing ops ...
}
if len(exchanges) > 0 {
	entry := buildConversationEntry(storage.SpecStageSpark, msg.GetPosture(), exchanges, false)
	ops = append(ops, func(c context.Context) error {
		_, err := store.RecordConversation(c, msg.Slug, entry)
		return err
	})
}
if err := runInTxOrSequential(ctx, store, ops...); err != nil {
	return nil, h.stageError(err)
}
```

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "feat(authoring): Spark accepts optional conversation_exchanges

Spark permits missing exchanges (seed may be a single --seed argument).
When present, validates via authoring.ValidateExchanges and records atomically
as an additional op in the existing runInTxOrSequential call.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 8: Approve handler — reject path with required exchanges

**Files:**

- Modify: `internal/server/authoring_handler.go:348` (Approve handler)
- Modify: `internal/server/authoring_handler_test.go`

Approve gains an `action` field. Semantics:

- `action == ""` or `action == "accept"`: existing approve flow (transition to approved stage)
- `action == "reject"`: conversation_exchanges required; record exchanges; create a finding tagged `"approve-rejected"` with `FindingSeverity_CRITICAL`; do not transition

Rejection state is represented by the finding. This matches design doc §Approve rules and uses the existing Finding storage rather than inventing new state.

- [ ] **Step 1: Write failing tests**

```go
func TestAuthoringHandler_Approve_AcceptUnchangedWithoutAction(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh", specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "oauth-refresh",
	}))
	if err != nil {
		t.Fatalf("Approve accept: %v", err)
	}
	// Verify spec now at approved stage.
	spec, _ := specClient.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{Slug: "oauth-refresh"}))
	if spec.Msg.GetSpec().GetStage() != specv1.AuthoringStage_AUTHORING_STAGE_APPROVED {
		t.Errorf("expected approved, got %v", spec.Msg.GetSpec().GetStage())
	}
}

func TestAuthoringHandler_Approve_RejectRequiresExchanges(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh-reject", specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "oauth-refresh-reject",
		Action: "reject",
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument for reject without exchanges, got %v", err)
	}
}

func TestAuthoringHandler_Approve_RejectRecordsFindingAndExchanges(t *testing.T) {
	client, _ := newTestClient(t)
	mustPrepareSpecAtStage(t, client, "oauth-refresh-reject-ok", specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "oauth-refresh-reject-ok",
		Action: "reject",
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "rationale?", Stage: "approve", Sequence: 1},
			{Role: "response", Content: "scope unclear", Stage: "approve", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Approve reject with exchanges: %v", err)
	}
	// Stage unchanged (still decompose).
	spec, _ := specClient.GetSpec(context.Background(), connect.NewRequest(&specv1.GetSpecRequest{Slug: "oauth-refresh-reject-ok"}))
	if spec.Msg.GetSpec().GetStage() != specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE {
		t.Errorf("stage should remain decompose, got %v", spec.Msg.GetSpec().GetStage())
	}
	// Conversation log present for approve stage.
	logs, _ := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{Slug: "oauth-refresh-reject-ok", Stage: "approve"}))
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 approve conversation log")
	}
	// Finding of type approve-rejected present.
	findings, _ := findingsClient.ListFindings(context.Background(), connect.NewRequest(&specv1.ListFindingsRequest{Slug: "oauth-refresh-reject-ok"}))
	found := false
	for _, f := range findings.Msg.GetFindings() {
		if f.GetPassType() == "approve-rejected" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected approve-rejected finding")
	}
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Approve_ -v`
Expected: FAIL.

- [ ] **Step 3: Update Approve handler**

Skeleton (adapt to existing handler's error-handling style):

```go
func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.Slug); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	action := msg.GetAction()
	if action == "" {
		action = "accept"
	}

	switch action {
	case "accept":
		// Existing accept flow — preserve current transition and response.
		// ...(preserve current Approve handler body here)...

	case "reject":
		if err := authoring.ValidateExchanges(msg.GetConversationExchanges(), "approve"); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		exchanges := exchangesFromProto(msg.GetConversationExchanges())
		entry := buildConversationEntry(storage.SpecStageApproved, msg.GetPosture(), exchanges, false)
		finding := storage.Finding{
			PassType: "approve-rejected",
			Severity: storage.FindingSeverityCritical,
			Message:  "approval rejected; see conversation log for rationale",
			// Other fields per Finding struct shape; scan storage.Finding definition
		}
		if err := runInTxOrSequential(ctx, store,
			func(c context.Context) error {
				_, err := store.RecordConversation(c, msg.Slug, entry)
				return err
			},
			func(c context.Context) error {
				return store.StoreFindings(c, msg.Slug, []storage.Finding{finding})
			},
		); err != nil {
			return nil, h.stageError(err)
		}
		return connect.NewResponse(&specv1.ApproveResponse{
			Slug:  msg.Slug,
			Stage: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE, // unchanged; pull from spec instead for accuracy
		}), nil

	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("action %q must be accept or reject", action))
	}
}
```

Note: `StoreFindings` / `storage.Finding` shape must be verified — scan `internal/storage/findings.go` for the exact struct and backend method signature. Adjust field names to match actual types.

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/server/ -run TestAuthoringHandler_Approve -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(authoring): Approve supports reject with required exchanges

Approve now accepts action=accept (default, existing behavior) or action=reject.
Reject requires conversation_exchanges; records them and creates an
approve-rejected critical finding in the same transaction. Stage is NOT
transitioned on reject; the rejection is represented by the finding.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 9: Atomic rollback integration test

**Files:**

- Create or modify: `internal/server/authoring_handler_integration_test.go` (or the relevant integration test file with `//go:build integration`)

Previously skipped; un-skipped here. Forces a failure in the safety-flag op and verifies that the conversation record AND the stage transition are both rolled back (spec stays at spark; no shape output; no conversation log for shape stage).

- [ ] **Step 1: Identify a forced-fail path**

Inspect `persistSafetyFlags` in `authoring_handler.go:894`. If it can be made to return an error via an input pattern (e.g., a safety input that triggers a deterministic failure), use that. Otherwise, inject a test-only backend wrapper that errors on the nth op.

Simplest approach: use a backend wrapper that implements `AuthoringBackend` but overrides `StoreSafetyFlags` to return an error after forwarding to the real backend. Place in `internal/server/testdata/` or similar test helpers.

- [ ] **Step 2: Write integration test**

```go
//go:build integration

func TestShape_AtomicRollback_ConversationNotPersistedOnSafetyFail(t *testing.T) {
	store := newTestPostgresStore(t)
	failingStore := &failingOnSafetyFlagsStore{AuthoringBackend: store}
	handler := newAuthoringHandlerWithStore(t, failingStore)

	mustPrepareSpecAtStage(t, handler, "oauth-refresh", specv1.AuthoringStage_AUTHORING_STAGE_SHAPE)

	_, err := handler.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "oauth-refresh",
		Output: validShapeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
	}))
	if err == nil {
		t.Fatal("expected error from safety-flags failure")
	}

	// Verify rollback: spec should still be at spark stage, no shape output,
	// no conversation log for shape stage.
	spec, _ := store.GetSpec(context.Background(), "oauth-refresh")
	if spec.Stage != storage.SpecStageSpark {
		t.Errorf("expected spec stage spark after rollback, got %v", spec.Stage)
	}
	logs, _ := store.ListConversations(context.Background(), "oauth-refresh", "shape")
	if len(logs) > 0 {
		t.Errorf("expected no conversation logs for shape after rollback, got %d", len(logs))
	}
}

// failingOnSafetyFlagsStore wraps an AuthoringBackend and fails StoreSafetyFlags.
type failingOnSafetyFlagsStore struct {
	storage.AuthoringBackend
}

func (f *failingOnSafetyFlagsStore) StoreSafetyFlags(ctx context.Context, slug string, flags []storage.SafetyFlag) error {
	return errors.New("injected safety-flags failure")
}
```

- [ ] **Step 3: Run integration test**

Run: `task test:integration -- -run TestShape_AtomicRollback -v`
Expected: PASS — the handler returns an error AND the spec remains at spark with no conversation log.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "test(authoring): verify atomic rollback on mid-transaction failure

Integration test uses a failing-StoreSafetyFlags backend wrapper to force
the third op to fail, and asserts that TransitionStage, StoreShapeOutput,
and RecordConversation are all rolled back: spec remains at spark stage
and no conversation log exists for the shape stage.

Exercises the design's core safety property (atomic coupling).

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 10: Posture recording on ConversationLogEntry

**Files:**

- Modify: `internal/storage/conversation.go` — add `Posture` field on `ConversationLogEntry`
- Modify: `internal/storage/postgres/conversation.go` — persist posture (may require DB migration; see below)
- Modify: `internal/storage/postgres/migrations/` — add migration
- Modify: `internal/server/authoring_handler.go` — `buildConversationEntry` propagates posture

Design §Posture requires server to record posture alongside exchanges for drift detection.

- [ ] **Step 1: Decide storage shape**

Option A: new column `posture TEXT` on `conversation_logs`. Requires migration.
Option B: store posture in `exchanges_json` or a metadata column if one exists.

Inspect `internal/storage/postgres/migrations/` to find the `conversation_logs` table definition. If it has a JSON metadata column, use that (no migration). Otherwise, add a migration.

- [ ] **Step 2: Write failing storage test**

```go
func TestRecordConversation_PersistsPosture(t *testing.T) {
	store := newTestPostgresStore(t)
	mustCreateSpec(t, store, "spec-1", "intent")
	entry := storage.ConversationLogEntry{
		Stage:         storage.SpecStageShape,
		Exchanges:     []storage.ConversationExchange{{Role: "probe", Content: "q", Stage: "shape", Sequence: 1}},
		ExchangeCount: 1,
		Posture:       "partner",
	}
	saved, err := store.RecordConversation(context.Background(), "spec-1", entry)
	if err != nil {
		t.Fatalf("RecordConversation: %v", err)
	}
	if saved.Posture != "partner" {
		t.Errorf("expected posture=partner, got %q", saved.Posture)
	}
}
```

- [ ] **Step 3: Run test, verify fail**

Run: `task test:integration -- -run TestRecordConversation_PersistsPosture -v`
Expected: FAIL — `Posture` field undefined or not persisted.

- [ ] **Step 4: Add `Posture` field + migration (if needed)**

`internal/storage/conversation.go`:

```go
type ConversationLogEntry struct {
	ID            string
	SpecSlug      string
	Stage         SpecStage
	Version       int32
	IsAmend       bool
	Exchanges     []ConversationExchange
	ExchangeCount int32
	Posture       string // "drive", "partner", "support", or "" when unspecified
	Date          time.Time
}
```

Migration (if needed): create a new file under `internal/storage/postgres/migrations/` following the goose naming convention (scan existing migrations for the next number). Example:

```sql
-- +goose Up
ALTER TABLE conversation_logs ADD COLUMN posture TEXT DEFAULT '';

-- +goose Down
ALTER TABLE conversation_logs DROP COLUMN posture;
```

Update the `RecordConversation` insert/select in `internal/storage/postgres/conversation.go` to include the new column.

- [ ] **Step 5: Update `buildConversationEntry`**

In `internal/server/authoring_handler.go`:

```go
func buildConversationEntry(stage storage.SpecStage, posture specv1.Posture, exchanges []storage.ConversationExchange, isAmend bool) storage.ConversationLogEntry {
	return storage.ConversationLogEntry{
		Stage:         stage,
		Exchanges:     exchanges,
		ExchangeCount: int32(len(exchanges)),
		IsAmend:       isAmend,
		Posture:       postureToString(posture),
	}
}

func postureToString(p specv1.Posture) string {
	switch p {
	case specv1.Posture_POSTURE_DRIVE:
		return "drive"
	case specv1.Posture_POSTURE_PARTNER:
		return "partner"
	case specv1.Posture_POSTURE_SUPPORT:
		return "support"
	default:
		return ""
	}
}
```

- [ ] **Step 6: Run tests, verify pass**

Run: `task test:integration -- -run TestRecordConversation -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
jj --no-pager commit -m "feat(storage): record posture on ConversationLogEntry

Adds Posture field to ConversationLogEntry, persisted via conversation_logs
migration. Handler's buildConversationEntry propagates the Posture enum from
the request. Provides data for future drift detection per design §Posture.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 11: Scaffold `internal/authoring/content/` with non-empty stubs + //go:embed

**Files:**

- Create: `internal/authoring/content.go`
- Create: `internal/authoring/content/*.md` (9 files with minimal placeholder text)

Scaffold must leave the repo green: `task check` runs on commit push. The initial stub content is minimal but non-empty so tests pass.

- [ ] **Step 1: Create content.go**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import "embed"

//go:embed content/*.md
var contentFS embed.FS

// Content returns the bytes of the named embedded content file.
// name is relative to the content/ directory (e.g. "persona.md").
func Content(name string) ([]byte, error) {
	return contentFS.ReadFile("content/" + name)
}
```

- [ ] **Step 2: Create nine files with stub content**

Each file starts with a one-line heading and a migration-pending note. Example for `content/persona.md`:

```markdown
# Persona

> Migration pending. See plugin/specgraph/skills/specgraph/persona.md for
> source content; populated in a subsequent commit.
```

Repeat for `orchestration.md`, `conversation-recording.md`, `quality-heuristics.md`, and five `stage-*.md` files.

- [ ] **Step 3: Write test asserting files are present and non-empty**

Create `internal/authoring/content_test.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import "testing"

func TestEmbeddedContent_Present(t *testing.T) {
	names := []string{
		"persona.md",
		"orchestration.md",
		"conversation-recording.md",
		"quality-heuristics.md",
		"stage-spark.md",
		"stage-shape.md",
		"stage-specify.md",
		"stage-decompose.md",
		"stage-approve.md",
	}
	for _, n := range names {
		data, err := Content(n)
		if err != nil {
			t.Errorf("content/%s: %v", n, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("content/%s: empty", n)
		}
	}
}
```

- [ ] **Step 4: Run tests and `task check`**

Run: `go test ./internal/authoring/ -v`
Expected: PASS.

Run: `task check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(authoring): scaffold //go:embed content/ with stub placeholders

Adds internal/authoring/content.go with //go:embed content/*.md and nine
stub markdown files with migration-pending placeholders. task check passes.
Stub content is replaced with migrated skill content in subsequent commits.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 12: Migrate persona content

**Files:**

- Modify: `internal/authoring/content/persona.md`

Source: `plugin/specgraph/skills/specgraph/persona.md`. Port core concepts; strip CLI-specific and skill-specific scaffolding.

- [ ] **Step 1: Read source**

```bash
cat plugin/specgraph/skills/specgraph/persona.md
```

- [ ] **Step 2: Write destination**

Retain: core identity, posture system (Drive/Partner/Support definitions and heuristics), pushback protocol, tone calibration, judgment heuristics, conversational style rules.

Remove: references to specific CLI commands, references to `references/*.md` paths, skill-specific bash blocks.

Target: ~800–1000 tokens.

- [ ] **Step 3: Verify test still passes**

Run: `go test ./internal/authoring/ -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate persona content from skills/specgraph/persona.md

Ports posture system, pushback protocol, tone calibration, and judgment
heuristics into embedded content. CLI-specific and skill-specific scaffolding
removed; behavioral framework retained.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 13: Migrate orchestration content

**Files:**

- Modify: `internal/authoring/content/orchestration.md`

Source: `plugin/specgraph/skills/specgraph/analytical-passes.md`.

- [ ] **Step 1: Read source**

```bash
cat plugin/specgraph/skills/specgraph/analytical-passes.md
```

- [ ] **Step 2: Write destination**

Retain: dispatch/collate/present protocol, pass registry names, severity gating rules, posture-aware dispatch policy.

Remove: subagent prompt templates, specific CLI invocations.

Target: ~600–800 tokens.

- [ ] **Step 3: Verify test still passes and commit**

Run: `go test ./internal/authoring/ -v`
Expected: PASS.

```bash
jj --no-pager commit -m "feat(authoring): migrate orchestration content (analytical pass protocol)

Ports dispatch/collate/present protocol, pass registry, severity gating, and
posture-aware dispatch policy into embedded content. Subagent templates and
CLI invocations removed.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 14: Migrate conversation-recording content

**Files:**

- Modify: `internal/authoring/content/conversation-recording.md`

Source: `plugin/specgraph/skills/specgraph/conversation-recording.md`.

- [ ] **Step 1: Read source and port**

Retain: what to capture (probe/response, stage, sequence, decision points), exchange format, amend semantics.

Rewrite persist section to reference the atomic-coupling contract:

> Conversation exchanges are persisted atomically as part of `author.shape` /
> `author.specify` / `author.decompose` tool calls via the
> `conversation_exchanges` argument. No separate `conversation.record` call is
> needed after a stage transition. The standalone `conversation.record` tool
> is reserved for amendments and approve-stage rejections.

Target: ~500–700 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate conversation-recording content

Ports capture rules, exchange format, and amend semantics. Persist section
rewritten to reference the atomic-coupling contract on author.* tools.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 15: Extract quality heuristics

**Files:**

- Modify: `internal/authoring/content/quality-heuristics.md`

Source: scan `plugin/specgraph/skills/specgraph-{shape,specify,decompose,spark,approve}/SKILL.md` for "Quality heuristics" / "Red flags" / similar sections.

- [ ] **Step 1: Consolidate**

Organize by stage with bullet lists. Example shape section:

```markdown
## Shape

- Unbounded scope — scope_in vague or scope_out empty
- Single approach — approaches has only one entry
- Untestable success criteria — success_must items aren't measurable
- Missing risks — risks empty on non-trivial scope
```

Cover: spark, shape, specify, decompose, approve.

Target: ~400–600 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): extract per-stage quality heuristics

Consolidates red-flag/pushback rules from per-stage SKILL.md files into a
single embedded quality-heuristics.md organized by stage.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 16: Migrate stage-spark content

**Files:**

- Modify: `internal/authoring/content/stage-spark.md`

Source: `plugin/specgraph/skills/specgraph-spark/SKILL.md`.

- [ ] **Step 1: Port**

Retain: what spark is, elicitation probes (seed/signal/scope_sniff/unknowns/kill_test), one-at-a-time rule, duplicate detection pointer (to `spec.list`), persistence contract (call `author.spark` with optional exchanges), next stage.

Remove: CLI-only commands, reference file paths.

Target: ~800–1000 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate stage-spark guidance

Ports spark-stage identity, elicitation probes, and persistence contract.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 17: Migrate stage-shape content

**Files:**

- Modify: `internal/authoring/content/stage-shape.md`

Source: `plugin/specgraph/skills/specgraph-shape/SKILL.md`.

- [ ] **Step 1: Port**

Retain: what shape is (bounded proposal with tradeoffs), five moves (scope in/out, approaches, decisions, success criteria, risks), step-gating, decisions as first-class nodes, persistence contract (required exchanges), next stage.

Target: ~1000–1200 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate stage-shape guidance"
```

Full conventional-commit message template with DCO sign-off applied as in prior tasks.

---

## Task 18: Migrate stage-specify content

**Files:**

- Modify: `internal/authoring/content/stage-specify.md`

Source: `plugin/specgraph/skills/specgraph-specify/SKILL.md`.

- [ ] **Step 1: Port**

Retain: four sections (interface contracts, verification criteria, invariants, file touches), step-at-a-time drafting, persistence contract.

Target: ~1000–1200 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate stage-specify guidance"
```

---

## Task 19: Migrate stage-decompose content

**Files:**

- Modify: `internal/authoring/content/stage-decompose.md`

Source: `plugin/specgraph/skills/specgraph-decompose/SKILL.md`.

- [ ] **Step 1: Port**

Retain: strategy selection (vertical/layer/single/steel thread), selection matrix, steel thread specifics, 1–4 hour slice target, persistence contract.

Target: ~900–1100 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate stage-decompose guidance"
```

---

## Task 20: Migrate stage-approve content

**Files:**

- Modify: `internal/authoring/content/stage-approve.md`

Source: `plugin/specgraph/skills/specgraph-approve/SKILL.md`.

- [ ] **Step 1: Port**

Retain: six-item checklist, self-approval prohibition, accept/reject paths (accept = `author.approve action=accept`; reject = `author.approve action=reject` with required exchanges).

Target: ~700–900 tokens.

- [ ] **Step 2: Commit**

```bash
jj --no-pager commit -m "feat(authoring): migrate stage-approve guidance"
```

---

## Task 21: Composer skeleton with stage assembly

**Files:**

- Create: `internal/authoring/composer.go`
- Create: `internal/authoring/composer_test.go`

- [ ] **Step 1: Write failing test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"strings"
	"testing"
)

type fakeComposerBackend struct {
	constitution *ConstitutionSummary
	specSummary  *SpecSummary
	related      []*RelatedSpec
}

func (f *fakeComposerBackend) GetConstitution(ctx context.Context) (*ConstitutionSummary, error) {
	if f.constitution != nil {
		return f.constitution, nil
	}
	return &ConstitutionSummary{PrimaryLanguage: "Go"}, nil
}
func (f *fakeComposerBackend) GetSpecSummary(ctx context.Context, slug string) (*SpecSummary, error) {
	if f.specSummary != nil {
		return f.specSummary, nil
	}
	return &SpecSummary{Slug: slug, Intent: "test"}, nil
}
func (f *fakeComposerBackend) GetRelatedSpecs(ctx context.Context, slug string) ([]*RelatedSpec, error) {
	return f.related, nil
}

func TestComposer_StageSectionsPresent(t *testing.T) {
	c := NewComposer(&fakeComposerBackend{})
	result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{
		Stage:   "shape",
		Slug:    "oauth-refresh",
		Posture: "partner",
	})
	if err != nil {
		t.Fatalf("ComposeStagePrompt: %v", err)
	}
	for _, marker := range []string{"# Persona", "# Orchestration", "# Conversation", "# Quality Heuristics", "# Shape", "oauth-refresh"} {
		if !strings.Contains(result.Body, marker) {
			t.Errorf("composed body missing marker %q", marker)
		}
	}
}
```

(Note: `# Persona`, `# Shape`, etc. as markers — migrated content must begin with those headings. Confirm during Tasks 12–20 content migration.)

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/authoring/ -run TestComposer_StageSectionsPresent -v`
Expected: FAIL.

- [ ] **Step 3: Implement composer**

Create `internal/authoring/composer.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"fmt"
	"strings"
)

// ConstitutionSummary is a bounded digest of the current constitution for
// inclusion in composed prompts. Full constitution available at specgraph://constitution.
type ConstitutionSummary struct {
	PrimaryLanguage string
	KeyConstraints  []string
	Antipatterns    []string
}

// SpecSummary is a bounded view of a spec for composition.
type SpecSummary struct {
	Slug              string
	Intent            string
	Stage             string
	PriorStageSummary string
}

// RelatedSpec is a single related-spec reference for the state section.
type RelatedSpec struct {
	Slug         string
	Intent       string
	Relationship string // "dependsOn", "blocks", "composes", etc.
}

// ComposerBackend is the read-only storage surface the composer needs.
type ComposerBackend interface {
	GetConstitution(ctx context.Context) (*ConstitutionSummary, error)
	GetSpecSummary(ctx context.Context, slug string) (*SpecSummary, error)
	GetRelatedSpecs(ctx context.Context, slug string) ([]*RelatedSpec, error)
}

// Composer assembles stage prompts from embedded content plus dynamic state.
type Composer struct {
	backend ComposerBackend
}

// NewComposer returns a Composer wired to the given storage backend.
func NewComposer(b ComposerBackend) *Composer { return &Composer{backend: b} }

// ComposeInput selects which stage prompt to compose.
type ComposeInput struct {
	Stage   string
	Slug    string
	Posture string
}

// ComposeResult carries the composed body plus observability counters.
type ComposeResult struct {
	Body           string
	StableTokens   int
	DynamicTokens  int
	TotalTokens    int
	TruncatedCount int
}

// ComposeStagePrompt assembles the full composed prompt for the given stage.
func (c *Composer) ComposeStagePrompt(ctx context.Context, in ComposeInput) (*ComposeResult, error) {
	var b strings.Builder

	for _, name := range []string{
		"persona.md",
		"orchestration.md",
		"conversation-recording.md",
		"quality-heuristics.md",
		"stage-" + in.Stage + ".md",
	} {
		data, err := Content(name)
		if err != nil {
			return nil, fmt.Errorf("load embedded content %s: %w", name, err)
		}
		b.Write(data)
		b.WriteString("\n\n")
	}
	stableLen := approxTokens(b.String())

	dynStart := b.Len()
	truncated, err := c.appendDynamicState(ctx, &b, in)
	if err != nil {
		return nil, fmt.Errorf("compose dynamic state: %w", err)
	}
	dynLen := approxTokens(b.String()[dynStart:])

	// Version footer (design §Embedded content versioning).
	fmt.Fprintf(&b, "\n---\nserver-version: %s\n", versionString())

	return &ComposeResult{
		Body:           b.String(),
		StableTokens:   stableLen,
		DynamicTokens:  dynLen,
		TotalTokens:    approxTokens(b.String()),
		TruncatedCount: truncated,
	}, nil
}

func (c *Composer) appendDynamicState(ctx context.Context, b *strings.Builder, in ComposeInput) (int, error) {
	var truncated int
	b.WriteString("# Current State\n\n")

	con, err := c.backend.GetConstitution(ctx)
	if err != nil {
		return 0, fmt.Errorf("get constitution: %w", err)
	}
	if con != nil {
		fmt.Fprintf(b, "**Constitution summary**: primary language %s", con.PrimaryLanguage)
		if len(con.KeyConstraints) > 0 {
			constraints := con.KeyConstraints
			if len(constraints) > 5 {
				constraints = constraints[:5]
				truncated++
			}
			fmt.Fprintf(b, "; key constraints: %s", strings.Join(constraints, ", "))
		}
		if len(con.Antipatterns) > 0 {
			antipatterns := con.Antipatterns
			if len(antipatterns) > 5 {
				antipatterns = antipatterns[:5]
				truncated++
			}
			fmt.Fprintf(b, "; antipatterns: %s", strings.Join(antipatterns, ", "))
		}
		b.WriteString(". For full constitution, read `specgraph://constitution`.\n\n")
	}

	if in.Slug != "" {
		spec, err := c.backend.GetSpecSummary(ctx, in.Slug)
		if err != nil {
			return truncated, fmt.Errorf("get spec summary: %w", err)
		}
		if spec != nil {
			fmt.Fprintf(b, "**Spec %s**: %s (stage: %s). For full spec, read `specgraph://spec/%s`.\n\n",
				spec.Slug, spec.Intent, spec.Stage, spec.Slug)
			if spec.PriorStageSummary != "" {
				fmt.Fprintf(b, "**Prior stage summary**: %s\n\n", spec.PriorStageSummary)
			}
		}

		related, err := c.backend.GetRelatedSpecs(ctx, in.Slug)
		if err != nil {
			return truncated, fmt.Errorf("get related specs: %w", err)
		}
		if len(related) > 0 {
			b.WriteString("**Related specs**: ")
			for i, r := range related {
				if i > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(b, "%s (%s)", r.Slug, r.Relationship)
			}
			b.WriteString(". Use `graph_query` for full traversal.\n\n")
		}
	}

	return truncated, nil
}

// approxTokens estimates token count using words * 0.75; used for observability, not hard enforcement.
func approxTokens(s string) int {
	words := strings.Fields(s)
	return (len(words) * 3) / 4
}

// versionString returns the runtime build version or "dev" as fallback. Real
// version injection happens in Task 22.
func versionString() string {
	return "dev"
}
```

- [ ] **Step 4: Run test, verify pass**

Run: `go test ./internal/authoring/ -run TestComposer_StageSectionsPresent -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "feat(authoring): composer skeleton with stage assembly and dynamic state

Introduces authoring.Composer: loads embedded stable content and appends
dynamic state (constitution + spec + related specs) using a read-only
ComposerBackend interface. Returns structured ComposeResult with approximate
token counts and truncation counter.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 22: Wire real version string via runtime/debug

**Files:**

- Modify: `internal/authoring/composer.go:versionString`

Replace the dev placeholder with actual build info.

- [ ] **Step 1: Write test**

```go
func TestVersionString_RealOrDev(t *testing.T) {
	v := versionString()
	if v == "" {
		t.Error("versionString returned empty")
	}
}
```

- [ ] **Step 2: Implement**

```go
import "runtime/debug"

func versionString() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
		for _, s := range info.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				if len(s.Value) > 12 {
					return s.Value[:12]
				}
				return s.Value
			}
		}
	}
	return "dev"
}
```

- [ ] **Step 3: Run, commit**

Run: `go test ./internal/authoring/ -run TestVersionString -v`
Expected: PASS.

```bash
jj --no-pager commit -m "feat(authoring): wire composer version footer to runtime/debug build info

Replaces 'dev' placeholder with vcs.revision (short) or module version from
runtime/debug.ReadBuildInfo. Falls back to 'dev' when build info unavailable.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 23: Composer observability metrics and logs

**Files:**

- Modify: `internal/authoring/composer.go` — emit metrics via `expvar` or existing logging pattern

Design §Composer observability requires phase-1 metrics. Check what metric library the codebase already uses (`internal/metrics/`, prom, `expvar`, etc.) and emit accordingly.

- [ ] **Step 1: Scan codebase for existing metrics idiom**

```bash
grep -rn "prometheus\|expvar\|metric" internal/ --include="*.go" | head -10
```

If nothing, use `slog` structured logs with a standard key set; that's enough for phase 1 observability.

- [ ] **Step 2: Add log emission in composer**

In `ComposeStagePrompt`, after building the result:

```go
slog.Info("composer.invocation",
	slog.String("stage", in.Stage),
	slog.String("slug", in.Slug),
	slog.String("posture", in.Posture),
	slog.Int("stable_tokens", stableLen),
	slog.Int("dynamic_tokens", dynLen),
	slog.Int("total_tokens", approxTokens(b.String())),
	slog.Int("truncated_count", truncated),
)
```

- [ ] **Step 3: Write test capturing log output**

Use `slogtest` or a test handler to capture the log record and assert the fields are present.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "feat(authoring): emit composer observability logs on every invocation

Logs stage, slug, posture, stable/dynamic/total token counts, and truncation
count per composer call. Satisfies design §Composer observability
phase-1 requirement using the existing slog pattern.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 24: Composer golden-file tests with token budgets

**Files:**

- Create: `internal/authoring/composer_golden_test.go`
- Create: `internal/authoring/testdata/golden/*.md`

- [ ] **Step 1: Write golden framework and run with -update**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func TestComposeGolden(t *testing.T) {
	cases := []struct {
		name      string
		stage     string
		slug      string
		maxStable int
		maxTotal  int
	}{
		{"spark", "spark", "", 4000, 6000},
		{"shape", "shape", "oauth-refresh", 4500, 7000},
		{"specify", "specify", "oauth-refresh", 4500, 7000},
		{"decompose", "decompose", "oauth-refresh", 4500, 7000},
		{"approve", "approve", "oauth-refresh", 4500, 7000},
	}
	c := NewComposer(&fakeComposerBackend{
		constitution: &ConstitutionSummary{
			PrimaryLanguage: "Go",
			KeyConstraints:  []string{"No panics", "Transactional writes"},
		},
		specSummary: &SpecSummary{Slug: "oauth-refresh", Intent: "Refresh tokens", Stage: "shape"},
	})
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.ComposeStagePrompt(context.Background(), ComposeInput{Stage: tt.stage, Slug: tt.slug, Posture: "partner"})
			if err != nil {
				t.Fatalf("compose: %v", err)
			}
			if result.StableTokens > tt.maxStable {
				t.Errorf("stable tokens %d > budget %d (restructure per design validation prerequisite)", result.StableTokens, tt.maxStable)
			}
			if result.TotalTokens > tt.maxTotal {
				t.Errorf("total tokens %d > budget %d", result.TotalTokens, tt.maxTotal)
			}
			goldenPath := filepath.Join("testdata", "golden", tt.name+".md")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(goldenPath, []byte(result.Body), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			if string(want) != result.Body {
				t.Errorf("composed output differs from golden %s", goldenPath)
			}
		})
	}
}
```

- [ ] **Step 2: Generate and commit goldens**

```bash
go test ./internal/authoring/ -run TestComposeGolden -update -v
go test ./internal/authoring/ -run TestComposeGolden -v  # must pass
```

If any budget is exceeded, trim the corresponding stage content until budgets pass, then regenerate goldens.

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "test(authoring): composer golden files with token budget assertions

Adds per-stage golden-file tests with stable and total token budget caps
(4500/7000 stable/total for shape/specify/decompose; 4000/6000 for spark).
Drift in embedded content or composition logic produces a visible failure
pointing to the golden path for review and refresh.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 25: Content/proto drift test

**Files:**

- Create: `internal/authoring/drift_test.go`

- [ ] **Step 1: Write drift test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"regexp"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestContentProtoDrift(t *testing.T) {
	cases := []struct {
		file    string
		message protoreflect.MessageDescriptor
	}{
		{"stage-shape.md", (&specv1.ShapeOutput{}).ProtoReflect().Descriptor()},
		{"stage-specify.md", (&specv1.SpecifyOutput{}).ProtoReflect().Descriptor()},
		{"stage-decompose.md", (&specv1.DecomposeOutput{}).ProtoReflect().Descriptor()},
		{"stage-spark.md", (&specv1.SparkOutput{}).ProtoReflect().Descriptor()},
	}

	fieldPattern := regexp.MustCompile("`([a-z][a-z0-9_]*)`")

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			content, err := Content(tc.file)
			if err != nil {
				t.Fatalf("load %s: %v", tc.file, err)
			}
			knownFields := map[string]bool{}
			fields := tc.message.Fields()
			for i := 0; i < fields.Len(); i++ {
				knownFields[string(fields.Get(i).Name())] = true
			}
			// Narrow allowlist: tokens that are demonstrably NOT proto fields
			// (English phrases, other-struct names, CLI args). Do NOT include
			// actual proto field names — the point is to CATCH drift on those.
			allowlist := map[string]bool{
				"specgraph":  true, // package name
				"author":     true, // MCP tool prefix
				"graph_query": true, // MCP tool name
				"spec_slug":  true, // MCP argument name
			}
			matches := fieldPattern.FindAllStringSubmatch(string(content), -1)
			for _, m := range matches {
				tok := m[1]
				if !strings.Contains(tok, "_") {
					continue // single-word tokens are common English; skip
				}
				if knownFields[tok] || allowlist[tok] {
					continue
				}
				t.Errorf("%s references %q which is not a field on %s and is not allowlisted (drift or typo?)",
					tc.file, tok, tc.message.Name())
			}
		})
	}
}
```

Key: allowlist explicitly excludes proto field names. If content uses `scope_in`, the drift test verifies it's still a valid field on `ShapeOutput`.

- [ ] **Step 2: Run test**

Run: `go test ./internal/authoring/ -run TestContentProtoDrift -v`
Expected: PASS if migrated content uses current field names. If any unexpected tokens fail, either they're typos (fix the content) or genuinely non-field (add to narrow allowlist with justification comment).

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "test(authoring): detect content/proto drift in CI

Scans stage-*.md for backticked snake_case tokens; each must be either a
field on the corresponding proto stage output or explicitly allowlisted as
a non-field token (MCP arg names, tool names). Proto field renames without
content updates fail CI.

Narrow allowlist: proto field names themselves are NEVER allowlisted — they
are exactly what the check must protect against drift.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 26: Composer adapter + MCP prompts delegation

> **Carry-over from Slice 3 (PR #922) review.** Before starting Slice 4,
> run `bd show spgr-bncv` — the notes carry a list of deferred items from
> the Slice 3 PR review that Task 26 should address naturally as the
> composer gets real callers: typed `Stage`/`Relationship` enums at the
> composer boundary, the `(nil, nil)` backend contract (document or switch
> to sentinel errors), and uncovered render branches
> (`PriorStageSummary`, multi-related-specs, nil constitution,
> empty-slug-on-non-spark). Separate deferrals (drift-test camelCase pass,
> visible truncation marker, tighter budgets) are independent follow-ups
> that can land in their own small commits within the slice.

**Files:**

- Create: `internal/mcp/composer_adapter.go`
- Modify: `internal/mcp/prompts.go`

- [ ] **Step 1: Create the adapter**

Create `internal/mcp/composer_adapter.go`:

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
)

type composerBackend struct {
	client *Client
}

func (b *composerBackend) GetConstitution(ctx context.Context) (*authoring.ConstitutionSummary, error) {
	resp, err := b.client.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	if err != nil {
		return nil, err
	}
	c := resp.Msg.GetConstitution()
	if c == nil {
		return &authoring.ConstitutionSummary{}, nil
	}
	var lang string
	if c.GetTech() != nil && c.GetTech().GetLanguages() != nil {
		lang = c.GetTech().GetLanguages().GetPrimary()
	}
	var antipatterns []string
	for _, ap := range c.GetAntipatterns() {
		antipatterns = append(antipatterns, ap.GetPattern())
	}
	return &authoring.ConstitutionSummary{
		PrimaryLanguage: lang,
		KeyConstraints:  c.GetConstraints(),
		Antipatterns:    antipatterns,
	}, nil
}

func (b *composerBackend) GetSpecSummary(ctx context.Context, slug string) (*authoring.SpecSummary, error) {
	resp, err := b.client.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: slug}))
	if err != nil {
		return nil, err
	}
	s := resp.Msg.GetSpec()
	if s == nil {
		return nil, nil
	}
	return &authoring.SpecSummary{
		Slug:   s.GetSlug(),
		Intent: s.GetIntent(),
		Stage:  s.GetStage().String(),
	}, nil
}

func (b *composerBackend) GetRelatedSpecs(ctx context.Context, slug string) ([]*authoring.RelatedSpec, error) {
	resp, err := b.client.Graph.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{Slug: slug}))
	if err != nil {
		return nil, err
	}
	var out []*authoring.RelatedSpec
	for _, dep := range resp.Msg.GetDependencies() {
		out = append(out, &authoring.RelatedSpec{
			Slug:         dep.GetSlug(),
			Intent:       dep.GetIntent(),
			Relationship: "dependsOn",
		})
	}
	return out, nil
}
```

Note: `GetDependenciesResponse` field shape may differ. Scan `gen/specgraph/v1/graph.pb.go` for the actual response type and adjust.

- [ ] **Step 2: Update prompts.go handlers**

Replace `stagePromptHandler` and `sparkPromptHandler` bodies:

```go
func stagePromptHandler(c *Client, stage string) PromptHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		specSlug := args["spec_slug"]
		if stage != "spark" && specSlug == "" {
			return nil, fmt.Errorf("spec_slug is required for %s prompt", stage)
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage:   stage,
			Slug:    specSlug,
			Posture: args["posture"],
		})
		if err != nil {
			return nil, fmt.Errorf("compose %s prompt: %w", stage, err)
		}
		return &PromptResult{
			Messages: []PromptMessage{{Role: "user", Content: result.Body}},
		}, nil
	}
}

func sparkPromptHandler(c *Client) PromptHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		topic := args["topic"]
		if topic == "" {
			return nil, fmt.Errorf("topic is required for spark prompt")
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage:   "spark",
			Slug:    args["spec_slug"],
			Posture: args["posture"],
		})
		if err != nil {
			return nil, fmt.Errorf("compose spark prompt: %w", err)
		}
		body := result.Body + "\n\n# Topic\n\n" + topic
		if ctxVal := args["context"]; ctxVal != "" {
			body += "\n\n# Additional Context\n\n" + ctxVal
		}
		return &PromptResult{
			Messages: []PromptMessage{{Role: "user", Content: body}},
		}, nil
	}
}
```

- [ ] **Step 3: Run existing mcp prompt tests**

Run: `go test ./internal/mcp/ -run TestPrompts -v`
Expected: tests that asserted thin templates now fail because responses are rich. Update assertions to check for composer markers (e.g., body contains "# Persona" or "# Shape") rather than the old one-liners.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "feat(mcp): MCP stage prompts delegate to authoring.Composer

MCP prompts now compose rich content (persona + orchestration + recording +
heuristics + stage + dynamic state) instead of returning thin templates from
GetPrompts. composerBackend bridges Client ConnectRPC calls to
authoring.ComposerBackend interface.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 27: `author.start_stage` tool (always registered)

**Files:**

- Modify: `internal/mcp/tools_authoring.go`
- Modify: `internal/mcp/tools_authoring_test.go`

Tool delivers the same composed content as prompts, for clients that don't expose prompts to the LLM. Always registered on the authoring profile.

- [ ] **Step 1: Write failing test**

```go
func TestAuthoringStartStageTool(t *testing.T) {
	c := newTestMCPClient(t)
	reg := NewRegistry()
	RegisterAuthoringTools(reg, c)

	tool, ok := reg.GetTool("author.start_stage")
	if !ok {
		t.Fatal("author.start_stage not registered")
	}
	result, err := tool.Handler(context.Background(), map[string]any{
		"stage": "shape",
		"slug":  "test-slug",
	})
	if err != nil || result.IsError {
		t.Fatalf("handler err=%v result=%+v", err, result)
	}
	if len(result.Content) == 0 || !strings.Contains(result.Content[0].Text, "# Shape") {
		t.Errorf("expected composed shape content, got %+v", result.Content)
	}
}
```

- [ ] **Step 2: Register tool**

In `RegisterAuthoringTools`:

```go
r.AddTool(ToolDef{
	Name:        "author.start_stage",
	Description: "Returns composed stage guidance (persona + orchestration + stage-specific content + current state). Use when the client does not expose MCP prompts to users, or for mid-conversation re-entry into a stage.",
	Profile:     ProfileAuthoring,  // NOT Tier — Profile is the field name per internal/mcp/types.go
	Schema: objectSchema(props{
		"stage":   stringProp("Stage: spark, shape, specify, decompose, approve"),
		"slug":    stringProp("Spec slug (required for shape/specify/decompose/approve; optional for spark)"),
		"posture": stringProp("Posture: drive, partner, support"),
	}, "stage"),
	Handler: authoringStartStageHandler(c),
})
```

Inspect `types.go` and the existing `ToolDef` usage in `tools_authoring.go` for the actual signature of `stringProp` (it may or may not take variadic enum values; match the existing pattern).

Implement the handler:

```go
func authoringStartStageHandler(c *Client) ToolHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		stage := stringParam(params, "stage")
		if stage == "" {
			return errResult("stage is required (spark|shape|specify|decompose|approve)"), nil
		}
		slug := stringParam(params, "slug")
		if stage != "spark" && slug == "" {
			return errResult(fmt.Sprintf("slug is required for stage %s", stage)), nil
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage:   stage,
			Slug:    slug,
			Posture: stringParam(params, "posture"),
		})
		if err != nil {
			return errResult(fmt.Sprintf("compose: %v", err)), nil
		}
		return &ToolResult{Content: []Content{{Type: "text", Text: result.Body}}}, nil
	}
}
```

- [ ] **Step 3: Run, verify pass**

Run: `go test ./internal/mcp/ -run TestAuthoringStartStageTool -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
jj --no-pager commit -m "feat(mcp): add author.start_stage tool (always registered)

Delivers composer content via tool response for clients that don't expose
prompts to users. Unconditionally registered on the authoring profile per
design §Delivery Channels: Tool Always Registered.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 28: `specgraph://prime` resource

**Files:**

- Modify: `internal/mcp/resources.go`
- Modify: `internal/mcp/resources_test.go`

Prime resource: constitution summary + graph counts by stage + top 10 ready + top 10 in-progress + finding counts by severity, bounded to ~2K tokens typical / 4K worst per design.

- [ ] **Step 1: Write failing test**

```go
func TestPrimeResource(t *testing.T) {
	c := newTestMCPClient(t)
	reg := NewRegistry()
	RegisterResources(reg, c)

	r, ok := reg.GetResourceByURI("specgraph://prime")
	if !ok {
		t.Fatal("specgraph://prime not registered")
	}
	content, err := r.Handler(context.Background(), "specgraph://prime")
	if err != nil {
		t.Fatalf("prime: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("empty prime content")
	}
	text := content[0].Text
	for _, marker := range []string{"Constitution", "Graph", "Ready"} {
		if !strings.Contains(text, marker) {
			t.Errorf("prime missing section %q", marker)
		}
	}
}
```

- [ ] **Step 2: Implement handler and register**

```go
func primeResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		var b strings.Builder
		b.WriteString("# SpecGraph Session Prime\n\n")

		if conResp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{})); err == nil && conResp.Msg.GetConstitution() != nil {
			con := conResp.Msg.GetConstitution()
			b.WriteString("## Constitution\n\n")
			if con.GetTech() != nil && con.GetTech().GetLanguages() != nil {
				fmt.Fprintf(&b, "Primary language: %s\n\n", con.GetTech().GetLanguages().GetPrimary())
			}
			if cs := con.GetConstraints(); len(cs) > 0 {
				top := cs
				if len(top) > 5 {
					top = top[:5]
				}
				b.WriteString("Top constraints:\n")
				for _, c := range top {
					fmt.Fprintf(&b, "- %s\n", c)
				}
				b.WriteString("\nFull at `specgraph://constitution`.\n\n")
			}
		}

		if listResp, err := c.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{})); err == nil {
			b.WriteString("## Graph Overview\n\n")
			counts := map[string]int{}
			for _, s := range listResp.Msg.GetSpecs() {
				counts[s.GetStage().String()]++
			}
			for stage, n := range counts {
				fmt.Fprintf(&b, "- %s: %d\n", stage, n)
			}
			b.WriteString("\n")
		}

		if readyResp, err := c.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{})); err == nil {
			b.WriteString("## Ready to Work\n\n")
			ready := readyResp.Msg.GetSpecs()
			if len(ready) > 10 {
				ready = ready[:10]
			}
			for _, s := range ready {
				fmt.Fprintf(&b, "- `%s`: %s\n", s.GetSlug(), s.GetIntent())
			}
			b.WriteString("\nFull list at `specgraph://graph/ready`.\n\n")
		}

		if findingsResp, err := c.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{})); err == nil {
			counts := map[string]int{}
			for _, f := range findingsResp.Msg.GetFindings() {
				counts[f.GetSeverity().String()]++
			}
			if len(counts) > 0 {
				b.WriteString("## Open Findings\n\n")
				for sev, n := range counts {
					fmt.Fprintf(&b, "- %s: %d\n", sev, n)
				}
				b.WriteString("\nFull at `specgraph://findings`.\n\n")
			}
		}

		return []ResourceContent{{URI: uri, MimeType: "text/markdown", Text: b.String()}}, nil
	}
}

// In RegisterResources:
r.AddResource(ResourceDef{
	URI:         "specgraph://prime",
	Name:        "prime",
	Description: "Session-priming digest: constitution summary, graph counts, ready specs, findings summary.",
	MimeType:    "text/markdown",
	IsTemplate:  false,
	Handler:     primeResourceHandler(c),
})
```

- [ ] **Step 3: Run, commit**

Run: `go test ./internal/mcp/ -run TestPrimeResource -v`
Expected: PASS.

```bash
jj --no-pager commit -m "feat(mcp): add specgraph://prime resource

Returns composed session-priming digest (constitution, graph counts, ready
specs, findings) read by platform plugins' session-start hooks. Bounded
per design §prime size discipline.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 29: `ProfileFromClientInfo` — add opencode, codex

**Files:**

- Modify: `internal/mcp/profiles.go:14-26`
- Modify: `internal/mcp/profiles_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestProfileFromClientInfo_AuthoringClientList(t *testing.T) {
	authoringNames := []string{"claude-code", "cursor", "windsurf", "opencode", "codex"}
	for _, n := range authoringNames {
		t.Run(n, func(t *testing.T) {
			info := &sdkmcp.Implementation{Name: n}
			if got := ProfileFromClientInfo(info); got != ProfileAuthoring {
				t.Errorf("ProfileFromClientInfo(%q) = %v, want %v", n, got, ProfileAuthoring)
			}
		})
	}
}
```

- [ ] **Step 2: Update profiles.go**

```go
func ProfileFromClientInfo(info *sdkmcp.Implementation) Profile {
	if info == nil {
		return ProfileCore
	}
	switch info.Name {
	case "polecat", "gastown":
		return ProfileExecution
	case "claude-code", "cursor", "windsurf", "opencode", "codex":
		return ProfileAuthoring
	default:
		return ProfileCore
	}
}
```

- [ ] **Step 3: Run, commit**

```bash
jj --no-pager commit -m "feat(mcp): add opencode and codex to authoring profile

Extends ProfileFromClientInfo. Windsurf retained pending verification in
docs/verification (Tasks 34–36).

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 30: Client identifier contract tests

**Files:**

- Create: `internal/mcp/client_id_contract_test.go`

Design §Testing requires automated regression tests for each platform's identifier. These tests fix what the server expects to see; if a platform renames itself, the test fails with a clear migration path.

- [ ] **Step 1: Write contract test**

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"testing"

	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// TestClientIDContract documents the expected clientInfo.name each platform
// reports during MCP initialize. Update when docs/verification/*.md captures
// empirical findings; regressing any of these without updating the test
// indicates a platform renamed itself and the profile mapping is stale.
func TestClientIDContract(t *testing.T) {
	cases := []struct {
		platform string
		name     string
		profile  Profile
	}{
		{"Claude Code", "claude-code", ProfileAuthoring},
		{"Cursor", "cursor", ProfileAuthoring},
		{"OpenCode", "opencode", ProfileAuthoring},
		{"Codex", "codex", ProfileAuthoring},
		{"Windsurf", "windsurf", ProfileAuthoring},
		{"Polecat", "polecat", ProfileExecution},
		{"Gastown", "gastown", ProfileExecution},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			got := ProfileFromClientInfo(&sdkmcp.Implementation{Name: tc.name})
			if got != tc.profile {
				t.Errorf("%s reports %q → want %v, got %v (did the platform rename? See docs/verification/)",
					tc.platform, tc.name, tc.profile, got)
			}
		})
	}
}
```

- [ ] **Step 2: Run, commit**

Run: `go test ./internal/mcp/ -run TestClientIDContract -v`
Expected: PASS.

```bash
jj --no-pager commit -m "test(mcp): client identifier contract tests

Documents expected clientInfo.name per platform and asserts the
ProfileFromClientInfo mapping for each. Regression guard for platform
rename — failing test points to docs/verification/ for remediation.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 31: Claude Code thin plugin replacement

**Files:**

- Modify: `plugin/specgraph/.claude-plugin/plugin.json`
- Modify: `plugin/specgraph/hooks/session-start.sh`
- Create: `plugin/specgraph/routing-guide.md`
- Delete: `plugin/specgraph/skills/` (entire tree)

- [ ] **Step 1: Update plugin.json**

```json
{
  "name": "specgraph",
  "description": "Thin Claude Code client for SpecGraph. Rich authoring workflow guidance is delivered from the SpecGraph MCP server.",
  "version": "0.3.0"
}
```

- [ ] **Step 2: Update session-start.sh**

```bash
#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Sean Brandt
set -euo pipefail

if ! command -v specgraph >/dev/null 2>&1; then
  echo "specgraph CLI not found; skipping session prime" >&2
  exit 0
fi

specgraph mcp read-resource specgraph://prime 2>&1 || {
  echo "specgraph prime failed (server unreachable?); session starts without prime" >&2
  exit 0
}
```

(`specgraph mcp read-resource` subcommand added in Task 32.)

- [ ] **Step 3: Write routing guide**

Create `plugin/specgraph/routing-guide.md`:

````markdown
# SpecGraph Routing Guide

You have access to the SpecGraph MCP server for spec-driven development on
this project. This guide tells you where to go; the MCP carries the how.

## When the user wants to author or update a spec

- Invoke the MCP prompt for the stage (`spark`, `shape`, `specify`,
  `decompose`, `approve`), or call the `author.start_stage` tool with the
  same stage and spec slug.
- Conduct the elicitation. Call the matching `author.<stage>` tool to
  persist stage output + conversation exchanges atomically in one call.

## When the user wants to query specs

- `spec.list` with filters
- `spec.get` for a single spec
- `graph_query` for dependency or impact traversal
- `specgraph://graph/ready` resource for "what can I work on"

## When the user wants to see the constitution

- `specgraph://constitution` resource for full content
- `constitution.update` tool to modify

## When the user wants analytical review

- `analytical_pass.run` for constitution-check, red-team, peripheral-vision,
  consistency, simplicity

## Never

- Don't call `author.<stage>` without `conversation_exchanges` for shape,
  specify, decompose — the server will reject the call.
- Don't approve a spec on behalf of the user; approval requires explicit
  user sign-off.

## Project setup (server not yet running)

```bash
docker info
specgraph init
specgraph serve
```
````

- [ ] **Step 4: Delete old skills**

```bash
rm -rf plugin/specgraph/skills/
ls plugin/specgraph/
# expect: .claude-plugin  hooks  routing-guide.md
```

- [ ] **Step 5: Verify plugin.json parses**

```bash
python3 -c "import json; j=json.load(open('plugin/specgraph/.claude-plugin/plugin.json')); assert j['name']=='specgraph' and j['version']=='0.3.0'"
```

- [ ] **Step 6: Commit**

```bash
jj --no-pager commit -m "feat(plugin): replace 13-skill Claude Code plugin with thin MCP client

Deletes plugin/specgraph/skills/ and adds routing-guide.md. Session-start
hook reads specgraph://prime via MCP instead of running the specgraph prime
CLI. All rich authoring workflow guidance now ships from the server composer.

Per design §Migration clean break; no backward compat.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 32: `specgraph mcp read-resource` CLI subcommand

> **⚠️ SUPERSEDED** — the prose below was invalidated by PR #923 (which dropped
> stdio transport and removed the `specgraph mcp` parent command). Task 32 was
> redesigned and re-planned in 2026-04-27 and shipped in PR #925 (merged
> 2026-05-04).
>
> **Current design:** [`docs/plans/2026-04-27-task-32-read-mcp-resource-design.md`](2026-04-27-task-32-read-mcp-resource-design.md)
>
> **Current implementation plan:** [`docs/plans/2026-04-27-task-32-read-mcp-resource-plan.md`](2026-04-27-task-32-read-mcp-resource-plan.md)
>
> Key deltas vs. the prose below: top-level `specgraph read-mcp-resource <uri>`
> command (no `mcp` parent), HTTP transport via `mark3labs/mcp-go` v0.45.0
> against `<baseURL>/mcp/`, no new flags or env vars, and `specgraph-cli` added
> to `internal/mcp/profiles.go:ProfileFromClientInfo`. The subcommand lives at
> `cmd/specgraph/read_mcp_resource.go`, not `cmd/specgraph/mcp_read_resource.go`.
> The prose is retained here for historical traceability only — do not follow it.

**Files:**

- Create: `cmd/specgraph/mcp_read_resource.go`
- Modify: `cmd/specgraph/mcp.go` — register subcommand
- Test: add to existing MCP test file

Required by Task 31's session-start hook.

- [ ] **Step 1: Write failing test**

```go
func TestMCPReadResource(t *testing.T) {
	server := startTestMCPServer(t)
	defer server.Close()

	out, err := runCLI(t, "mcp", "read-resource", "specgraph://prime",
		"--server", server.URL)
	if err != nil {
		t.Fatalf("read-resource: %v", err)
	}
	if !strings.Contains(out, "SpecGraph Session Prime") {
		t.Errorf("expected prime body in output, got: %s", out)
	}
}
```

- [ ] **Step 2: Implement subcommand**

Scan `cmd/specgraph/mcp.go` for the existing `specgraph mcp` command setup and the stdio transport wiring; reuse the same client constructor.

```go
// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mcpReadResourceCmd = &cobra.Command{
	Use:   "read-resource <uri>",
	Short: "Read an MCP resource via stdio transport and print its body.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		uri := args[0]
		client, err := newMCPStdioClient(cmd.Context())
		if err != nil {
			return fmt.Errorf("mcp client: %w", err)
		}
		defer client.Close()
		content, err := client.ReadResource(cmd.Context(), uri)
		if err != nil {
			return fmt.Errorf("read resource %s: %w", uri, err)
		}
		for _, c := range content {
			fmt.Print(c.Text)
		}
		fmt.Println()
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpReadResourceCmd)
}
```

If `newMCPStdioClient` doesn't exist, factor it out of the existing `specgraph mcp` command body as part of this task (note this in the commit message).

- [ ] **Step 3: Run, manual smoke, commit**

```bash
go test ./cmd/specgraph/ -run TestMCPReadResource -v
go build ./cmd/specgraph
./specgraph mcp read-resource specgraph://prime
```

```bash
jj --no-pager commit -m "feat(cli): specgraph mcp read-resource subcommand

Reads an MCP resource via stdio transport and prints its body to stdout.
Used by the thin Claude Code plugin's session-start hook.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 33: Update CLAUDE.md to reflect thin-plugin layout

**Files:**

- Modify: `CLAUDE.md`

Current CLAUDE.md Documentation section describes skill personas and symlinks. With skills deleted, those references are stale.

- [ ] **Step 1: Edit CLAUDE.md**

Locate the "Documentation" section (search for "Skill personas"). Replace with:

```markdown
## Documentation

- **Example spec** — `site/docs/concepts/example-spec.md` is the canonical example spec on the public site. When proto messages for authoring stages change (`SparkOutput`, `ShapeOutput`, `SpecifyOutput`, `DecomposeOutput`), check if the example spec needs updating.
- **Authoring content** — workflow guidance (persona, orchestration, stage-specific instructions) lives in `internal/authoring/content/*.md`, embedded via `//go:embed` and composed into MCP prompt responses by `internal/authoring/composer.go`. When proto stage-output messages change (`ShapeOutput`, `SpecifyOutput`, `DecomposeOutput` in `proto/specgraph/v1/authoring.proto`), update both the proto AND any field references in `internal/authoring/content/stage-*.md`. The `TestContentProtoDrift` CI test catches drift for backticked snake_case tokens.
- **Plugin** — `plugin/specgraph/` is the thin Claude Code plugin: `.claude-plugin/plugin.json`, `hooks/session-start.sh` (reads `specgraph://prime` via MCP), and `routing-guide.md` (stable meta-knowledge routing for the LLM). The previous 13-skill layout is retired; see `docs/plans/2026-04-20-multi-platform-plugin-design.md`.
```

Remove any lines that reference the removed `plugin/specgraph/skills/` tree, `persona.md` symlinks, `references/*-output-format.md`, or per-stage SKILL.md files.

- [ ] **Step 2: Verify grep finds no stale references in CLAUDE.md**

```bash
grep -n "plugin/specgraph/skills\|specgraph-shape\|specgraph-specify\|persona.md\|analytical-passes.md" CLAUDE.md
# expect: no matches
```

- [ ] **Step 3: Commit**

```bash
jj --no-pager commit -m "docs(claude): update CLAUDE.md for thin-plugin + composer layout

Removes references to the deleted plugin/specgraph/skills/ tree and documents
the new internal/authoring/content/ + composer model. Notes the content/proto
drift CI test.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 34: Verify Cursor MCP behavior

**Files:**

- Create: `docs/verification/cursor-mcp-verification.md`

- [ ] **Step 1: Install Cursor; configure MCP**

Connect Cursor to `specgraph mcp` stdio or `specgraph serve`'s `/mcp/`. Use Cursor's MCP config format (scan Cursor docs or `~/.cursor/mcp.json`).

- [ ] **Step 2: Capture clientInfo.name during initialize**

Add a debug log in `internal/mcp/server.go`'s initialize hook (temporary; remove before commit) or use server logs to capture.

- [ ] **Step 3: Test prompt exposure**

Invoke an MCP prompt from Cursor (via slash command / command palette / etc). Observe behavior: does Cursor surface the prompt to the user? Does the LLM receive it?

- [ ] **Step 4: Test session-start mechanism**

Determine Cursor's hook-like mechanism (Rules file, Custom Mode, etc.) and whether it can execute the prime-read convention.

- [ ] **Step 5: Document findings**

Create `docs/verification/cursor-mcp-verification.md`:

````markdown
# Cursor MCP Verification

> **Date**: <fill>
> **Cursor version**: <fill>
> **Verifier**: <fill>

## Client identifier

clientInfo.name reported: `<fill>` (expected: `cursor`; if different, update ProfileFromClientInfo).

## MCP configuration format

Config path: `<fill>`

```json
<paste current config shape>
```

## Prompt exposure

- Prompts listed in tools/list: YES/NO
- Prompts surfaced to user: YES/NO
- Invocation method: <slash command / menu / command palette / not surfaced>

## Session-start mechanism

Best fit: <Rules file / Custom Mode / other>
Path: <fill>
Can execute specgraph://prime read: YES/NO

## Notes

<surprises, quirks, bugs>
````

- [ ] **Step 6: If clientInfo.name differs, update `ProfileFromClientInfo`**

Update the mapping and the Client ID contract test in Task 30 with the actual string, preserving a reference to the verification doc.

- [ ] **Step 7: Commit verification**

```bash
jj --no-pager commit -m "docs(verification): Cursor MCP empirical findings

Documents client identifier, MCP config format, prompt exposure, and
session-start mechanism for Cursor. Findings feed Plan B (Cursor thin plugin).

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 35: Verify OpenCode MCP behavior

**Files:**

- Create: `docs/verification/opencode-mcp-verification.md`

Same structure as Task 34, adapted to OpenCode.

- [ ] Steps 1–7: Follow Task 34 pattern. Commit findings.

---

## Task 36: Verify Codex MCP behavior

**Files:**

- Create: `docs/verification/codex-mcp-verification.md`

Same structure as Task 34, adapted to Codex.

- [ ] Steps 1–7: Follow Task 34 pattern. Commit findings.

---

## Task 37: Rewrite e2e authoring tests for MCP path

**Files:**

- Modify: `e2e/api/authoring_test.go` (or the equivalent file in `e2e/`)
- Remove: any tests that depend on deleted skill files

- [ ] **Step 1: Audit**

```bash
grep -rn "skill\|SKILL\|plugin/specgraph/skills" e2e/
```

List files to update.

- [ ] **Step 2: Rewrite the end-to-end authoring flow**

Exercise the composer-backed prompt path AND the atomic persist path:

```go
//go:build e2e

var _ = Describe("MCP authoring funnel end-to-end", func() {
	It("drives shape via MCP prompt with atomic persist", func() {
		ctx := context.Background()
		_, err := authClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug:   "e2e-test-spec",
			Output: &specv1.SparkOutput{Seed: "test seed"},
		}))
		Expect(err).ToNot(HaveOccurred())

		promptClient := newMCPPromptClient(ctx)
		prompt, err := promptClient.InvokePrompt(ctx, "shape", map[string]string{"spec_slug": "e2e-test-spec"})
		Expect(err).ToNot(HaveOccurred())
		Expect(prompt.Body).To(ContainSubstring("# Shape"))
		Expect(prompt.Body).To(ContainSubstring("e2e-test-spec"))

		_, err = authClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug:   "e2e-test-spec",
			Output: validShapeOutput(),
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "X in", Stage: "shape", Sequence: 2},
			},
		}))
		Expect(err).ToNot(HaveOccurred())

		spec, _ := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: "e2e-test-spec"}))
		Expect(spec.Msg.GetSpec().GetStage()).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_SHAPE))
		logs, _ := authClient.ListConversations(ctx, connect.NewRequest(&specv1.ListConversationsRequest{Slug: "e2e-test-spec", Stage: "shape"}))
		Expect(logs.Msg.GetConversationLogs()).To(HaveLen(1))
	})

	It("rejects shape without exchanges", func() {
		ctx := context.Background()
		mustPrepareSpecAtStage(ctx, "e2e-reject-spec", specv1.AuthoringStage_AUTHORING_STAGE_SHAPE)
		_, err := authClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug:   "e2e-reject-spec",
			Output: validShapeOutput(),
		}))
		Expect(err).To(HaveOccurred())
		var ce *connect.Error
		Expect(errors.As(err, &ce)).To(BeTrue())
		Expect(ce.Code()).To(Equal(connect.CodeInvalidArgument))
	})
})
```

Add similar for specify, decompose, approve-reject.

- [ ] **Step 3: Delete obsolete skill-driven e2e tests**

Any test that `cat`'d a skill file or asserted skill content.

- [ ] **Step 4: Run suite**

Run: `task test:e2e`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
jj --no-pager commit -m "test(e2e): rewrite authoring funnel tests for MCP path

Exercises composer-backed MCP prompt and atomic persist of stage output +
conversation exchanges. Asserts rejection on missing exchanges. Removes
tests tied to the deleted plugin/specgraph/skills/ layout.

spgr-mv32

Signed-off-by: Sean Brandt <SeBrandt@geico.com>"
```

---

## Task 38: Dogfood cutover + Plan B handoff

**Files:**

- Audit any tooling / lefthook / CI referencing the old skill layout
- Update beads

- [ ] **Step 1: Grep for skill references outside planning docs**

```bash
grep -rn "plugin/specgraph/skills\|specgraph-shape\|specgraph-specify\|specgraph-decompose\|specgraph-approve\|specgraph-spark" \
  --exclude-dir=.jj --exclude-dir=.git --exclude-dir=docs/plans \
  --exclude-dir=docs/superpowers 2>&1
```

Expected: zero live references. If any remain (tooling, lefthook, CI, README), update them.

- [ ] **Step 2: Run full quality pipeline**

Run: `task pr-prep`
Expected: PASS — fmt, license, lint, build, unit, integration, e2e.

- [ ] **Step 3: Update beads**

```bash
bd update spgr-mv32 --status=in_progress --notes="Plan A complete pending PR review. Plan B (Cursor/OpenCode/Codex thin plugins) tracked as spgr-<new>."

bd create \
  --title="Multi-platform plugin Phase B — Cursor/OpenCode/Codex plugins" \
  --description="Depends on spgr-mv32 and docs/verification/*.md. Scope: thin plugins for the three non-Claude platforms per verified empirical findings. Design doc: docs/plans/2026-04-20-multi-platform-plugin-design.md." \
  --type=feature --priority=2
```

Record new beads ID.

- [ ] **Step 4: Open PR**

```bash
jj bookmark set plugin-phase-a -r @-
jj git push --bookmark plugin-phase-a
gh pr create --title "feat: multi-platform plugin phase A (composer, atomic coupling, Claude Code thin plugin)" --body "$(cat <<'EOF'
## Summary

Implements Plan A of the multi-platform plugin design:

- Server-side authoring.Composer with //go:embed content
- Proto: conversation_exchanges on stage requests; action on Approve
- Atomic persist: RecordConversation added as fourth op in runInTxOrSequential
- Server-side validation of exchanges (non-empty, role, content, stage, sequence)
- Posture recorded on ConversationLogEntry (migration added)
- Posture-absent warning log + metric
- specgraph://prime resource with bounded composition
- author.start_stage tool always registered; prompts additive
- opencode/codex added to ProfileFromClientInfo
- Client identifier contract tests
- Thin Claude Code plugin replaces 13-skill layout
- specgraph mcp read-resource CLI subcommand
- CLAUDE.md updated for thin-plugin layout
- Composer golden tests with token budget assertions + content/proto drift test
- Atomic rollback integration test
- Empirical verification findings for Cursor/OpenCode/Codex in docs/verification/

Closes spgr-mv32 (phase A).

## Test plan

- [x] task check passes
- [x] task test:integration passes
- [x] task test:e2e passes
- [x] Manual: specgraph mcp read-resource specgraph://prime primes session
- [x] Manual: Claude Code plugin loads; session-start hook fires
- [x] Manual: author.shape rejects missing exchanges

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Self-Review

### Spec coverage

| Design section | Task(s) |
|----------------|---------|
| Content Boundary | 11–20, 31 |
| Stateless Server (with bounded exceptions) | Design principle; enforced by stateless tool/prompt handlers |
| Posture (detect/transmit/default/record/warn) | 4–8 (warning); 10 (recording) |
| Prompt Composition structure A + (ii) | 21, 23 |
| Token Budget | 24 (golden + budget) |
| Implementation Layout | 11, 21 |
| Package boundary and dependency direction | 3, 21 |
| Composer observability | 23 |
| Embedded content versioning | 21, 22 |
| Conversation Recording Coupling | 1–9 |
| Validation rules | 2 |
| Approve/spark coupling | 7, 8 |
| specgraph://prime | 28 |
| Profile Mapping | 29 |
| Platform plugins — Claude Code | 31, 32 |
| Platform plugins — Cursor/OpenCode/Codex | 34–36 (verification); Plan B (implementation) |
| Delivery Channels | 26, 27 |
| Rollback | 9 (integration test) |
| Testing and Dogfood | 24, 25, 30, 37, 38 |
| Security / Auth | Inherits PR 898; no new surface |

### Placeholder scan

No "TBD" / "TODO" without concrete code. Explicit acknowledgments:

- Task 8's `storage.Finding` / `FindingSeverity` field shape is annotated as "verify against actual types" — the implementer must scan `internal/storage/findings.go` to pick exact field names. This is unavoidable without reading that file here, but the pattern and method names are pinned.
- Task 26's `GetDependenciesResponse` field shape similarly annotated.
- Task 10 conditional migration: implementer inspects existing `conversation_logs` schema to decide column vs metadata column.

### Type consistency

- `ConversationExchange` used consistently as value struct with typed `ConversationRole` and correct fields (Role/Content/Stage/Sequence/DecisionPoint).
- `h.stageError(err)` called with single argument everywhere.
- MCP field is `Profile: ProfileAuthoring` (not `Tier: TierAuthoring`).
- `runInTxOrSequential(ctx, store, op1, op2, op3, op4)` signature matches existing handler pattern.
- `buildConversationEntry` and `exchangesFromProto` signatures match across all tasks that call them.

---

## Execution Handoff

Plan saved to `docs/plans/2026-04-20-multi-platform-plugin-plan.md`. Two options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — batched with checkpoints for review in this session.

**Which approach?**
