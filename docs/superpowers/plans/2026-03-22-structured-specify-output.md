# Structured SpecifyOutput Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace flat string fields in SpecifyOutput with structured sub-messages for multi-surface APIs and categorized criteria.

**Architecture:** Add InterfaceSection, VerifyCriterion, FileTouch proto messages. Update domain types, handler validation, proto-to-domain converters, safety scanner, tests, and skill format references. Clean break at 0.2.0-dev.

**Tech Stack:** Protobuf, Go, ConnectRPC, Ginkgo/Gomega (e2e), testify (unit)

**Spec:** `docs/superpowers/specs/2026-03-22-structured-specify-output-design.md`

---

## Chunk 1: Proto and Generated Code

### Task 1: Update proto with new messages and restructured SpecifyOutput

**Files:**

- Modify: `proto/specgraph/v1/authoring.proto:142-152`

- [ ] **Step 1: Replace SpecifyOutput and add new messages**

Insert the three new messages before SpecifyOutput, then replace SpecifyOutput.
Add them after the `Approach` message (line 140) and before `DecomposeOutput`:

```protobuf
// InterfaceSection defines one API surface in the specify stage contract.
message InterfaceSection {
  // Surface name, e.g. "WebhookService proto", "EventBus Go interface".
  string name = 1;
  // Free-form contract content (proto definitions, method signatures, etc.).
  string body = 2;
}

// VerifyCriterion defines one testable acceptance criterion with a category.
message VerifyCriterion {
  // Category grouping, e.g. "emission", "CRUD", "e2e".
  string category = 1;
  // The criterion description.
  string description = 2;
}

// FileTouch identifies a file expected to be created or modified by this spec.
message FileTouch {
  // File or package path relative to repo root.
  string path = 1;
  // What changes and why.
  string purpose = 2;
  // Type of change: "new", "modify", "delete".
  string change_type = 3;
}

// SpecifyOutput captures the precise contract and verification criteria for a spec.
// WIRE-BREAK: Fields 1, 2, 4 change type from string/repeated-string to
// repeated-message. Intentional at 0.2.0-dev (no production data). Field
// numbers reused because semantic intent is preserved.
message SpecifyOutput {
  // Interface contracts, one per API surface.
  repeated InterfaceSection interfaces = 1;
  // Categorized conditions that must hold for correctness.
  repeated VerifyCriterion verify_criteria = 2;
  // Invariants that must never be violated across any implementation state.
  repeated string invariants = 3;
  // Files, packages, or components expected to be modified.
  repeated FileTouch touches = 4;
}
```

- [ ] **Step 2: Regenerate Go code**

Run: `task proto`

- [ ] **Step 3: Verify generated code compiles**

Run: `go build ./gen/...`
Expected: Build succeeds (handler code will NOT compile yet — that's Task 2)

- [ ] **Step 4: Commit**

```text
feat(proto): restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch
```

---

## Chunk 2: Domain Types and Handler

### Task 2: Update domain types

**Files:**

- Modify: `internal/storage/authoring.go:63-69`

- [ ] **Step 1: Replace SpecifyOutput domain type and add sub-types**

Replace the existing `SpecifyOutput` struct with:

```go
// InterfaceSection defines one API surface in the specify stage contract.
type InterfaceSection struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// VerifyCriterion defines one testable acceptance criterion with a category.
type VerifyCriterion struct {
	Category    string `json:"category"`
	Description string `json:"description"`
}

// FileTouch identifies a file expected to be created or modified by this spec.
type FileTouch struct {
	Path       string `json:"path"`
	Purpose    string `json:"purpose"`
	ChangeType string `json:"change_type"`
}

// SpecifyOutput captures the precise contract and verification criteria.
type SpecifyOutput struct {
	Interfaces     []InterfaceSection `json:"interfaces,omitempty"`
	VerifyCriteria []VerifyCriterion  `json:"verify_criteria,omitempty"`
	Invariants     []string           `json:"invariants,omitempty"`
	Touches        []FileTouch        `json:"touches,omitempty"`
}
```

- [ ] **Step 2: Commit**

```text
refactor(storage): replace flat SpecifyOutput with structured sub-types
```

### Task 3: Update handler validation and converter

**Files:**

- Modify: `internal/server/authoring_handler.go:190-216` (validation + safety scanner)
- Modify: `internal/server/authoring_handler.go:615-622` (specifyOutputToDomain)

- [ ] **Step 1: Replace validation block (lines 190-211)**

Replace the old `interface_contract` length check and item iteration with:

```go
	if len(msg.Output.GetInterfaces()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces exceeds maximum of %d elements", maxElements))
	}
	for i, iface := range msg.Output.GetInterfaces() {
		if iface.GetName() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces[%d]: name is required", i))
		}
		if len(iface.GetBody()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("interfaces[%d]: body exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
	if len(msg.Output.GetVerifyCriteria()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria exceeds maximum of %d elements", maxElements))
	}
	for i, vc := range msg.Output.GetVerifyCriteria() {
		if vc.GetDescription() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria[%d]: description is required", i))
		}
		if len(vc.GetDescription()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("verify_criteria[%d]: description exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
	if len(msg.Output.GetInvariants()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invariants exceeds maximum of %d elements", maxElements))
	}
	for _, item := range msg.Output.GetInvariants() {
		if len(item) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invariants item exceeds maximum length of %d characters", maxFieldLen))
		}
	}
	if len(msg.Output.GetTouches()) > maxElements {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches exceeds maximum of %d elements", maxElements))
	}
	for i, ft := range msg.Output.GetTouches() {
		if ft.GetPath() == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: path is required", i))
		}
		if len(ft.GetPurpose()) > maxFieldLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("touches[%d]: purpose exceeds maximum length of %d characters", i, maxFieldLen))
		}
	}
```

- [ ] **Step 2: Update safety scanner text derivation (line ~213-214)**

Replace:

```go
safetyInput := &authoring.SafetyInput{
    Text:       msg.Output.GetInterfaceContract(),
    Invariants: msg.Output.GetInvariants(),
}
```

With:

```go
var contractText strings.Builder
for _, iface := range msg.Output.GetInterfaces() {
    if contractText.Len() > 0 {
        contractText.WriteString("\n\n")
    }
    contractText.WriteString(iface.GetName() + ":\n" + iface.GetBody())
}
safetyInput := &authoring.SafetyInput{
    Text:       contractText.String(),
    Invariants: msg.Output.GetInvariants(),
}
```

Ensure `"strings"` is imported (it likely already is).

- [ ] **Step 3: Replace specifyOutputToDomain (line ~615)**

Note: `make([]T, len(source))` produces a non-nil empty slice when source is
empty, which marshals to `[]` in JSON (not `null`). This is the correct
behavior for Memgraph JSON storage. Do NOT use `append` patterns that
produce `nil` slices.

```go
func specifyOutputToDomain(p *specv1.SpecifyOutput) *storage.SpecifyOutput {
	interfaces := make([]storage.InterfaceSection, len(p.GetInterfaces()))
	for i, iface := range p.GetInterfaces() {
		interfaces[i] = storage.InterfaceSection{
			Name: iface.GetName(),
			Body: iface.GetBody(),
		}
	}
	criteria := make([]storage.VerifyCriterion, len(p.GetVerifyCriteria()))
	for i, vc := range p.GetVerifyCriteria() {
		criteria[i] = storage.VerifyCriterion{
			Category:    vc.GetCategory(),
			Description: vc.GetDescription(),
		}
	}
	touches := make([]storage.FileTouch, len(p.GetTouches()))
	for i, ft := range p.GetTouches() {
		touches[i] = storage.FileTouch{
			Path:       ft.GetPath(),
			Purpose:    ft.GetPurpose(),
			ChangeType: ft.GetChangeType(),
		}
	}
	return &storage.SpecifyOutput{
		Interfaces:     interfaces,
		VerifyCriteria: criteria,
		Invariants:     p.GetInvariants(),
		Touches:        touches,
	}
}
```

- [ ] **Step 4: Verify contenthash is unaffected**

Run: `grep -n 'specify_output\|SpecifyOutput' internal/storage/contenthash/*.go`
Expected: No matches. `contenthash.Spec()` accepts `authoringOutputs map[string]string`
(serialized JSON), so the struct shape change is transparent. No hash code changes needed.

- [ ] **Step 5: Rename prompt registry entry**

In `internal/authoring/prompts.go:32`, change:

```go
{Name: "interface_contract", Template: "Define the public interface: inputs, outputs, error cases."},
```

To:

```go
{Name: "interfaces", Template: "Define the public interfaces: inputs, outputs, error cases for each API surface."},
```

- [ ] **Step 6: Verify build compiles**

Run: `go build ./...`

- [ ] **Step 7: Commit**

```text
feat(server): update specify handler for structured SpecifyOutput
```

---

## Chunk 3: Unit Tests

### Task 4: Update handler unit tests

**Files:**

- Modify: `internal/server/authoring_handler_test.go`

- [ ] **Step 1: Update all SpecifyOutput fixtures**

Replace every `&specv1.SpecifyOutput{InterfaceContract: "..."}` with the new
structured form. For example, line 306-307:

```go
Output: &specv1.SpecifyOutput{
    Interfaces: []*specv1.InterfaceSection{
        {Name: "API", Body: "POST /api/login"},
    },
},
```

And line 314:

```go
require.Len(t, resp.Msg.Output.Interfaces, 1)
require.Equal(t, "POST /api/login", resp.Msg.Output.Interfaces[0].Body)
```

Apply the same pattern to all test fixtures that use `SpecifyOutput`:

- Line 377: empty output `&specv1.SpecifyOutput{}` — can stay as-is
- Line 540: `InterfaceContract: "POST /api/login"` — update
- Line 901-902: `InterfaceContract: "interface contract"` — update

- [ ] **Step 2: Update prompt name assertion (line ~839)**

Change:

```go
require.True(t, names["interface_contract"])
```

To:

```go
require.True(t, names["interfaces"])
```

**Note:** This depends on Task 3 Step 5 (prompt rename). Both are in the same
commit, so they must be applied together.

- [ ] **Step 3: Add negative-path validation tests**

Add new test cases for the per-element validation rules introduced in Task 3.
Follow the existing pattern using `newAuthoringClient` (see
`TestAuthoringHandler_Specify_EmptySlug` at line 373 for the setup pattern).

The handler validates output fields (lines 190-211) BEFORE any store call,
so no spec seeding is needed — `&fakeAuthoringBackend{}` zero-value is
sufficient:

```go
func TestAuthoringHandler_Specify_InterfacesMissingName(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "", Body: "some body"},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_Specify_VerifyCriteriaMissingDescription(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "test", Description: ""},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_Specify_TouchesMissingPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			Touches: []*specv1.FileTouch{
				{Path: "", Purpose: "something"},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
```

- [ ] **Step 4: Add safety scanner multi-interface test**

Add a test verifying multi-interface output is accepted and returned
correctly. Mirrors the existing `TestAuthoringHandler_Specify_HappyPath`
pattern — bare `&fakeAuthoringBackend{}` with no seeding (the handler does
not verify spec existence via the authoring backend):

```go
func TestAuthoringHandler_Specify_MultipleInterfaces(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "multi-iface",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "ProtoService", Body: "service Webhook { rpc Send(...) }"},
				{Name: "GoInterface", Body: "type EventBus interface { Publish(event) }"},
			},
		},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Output.Interfaces, 2)
}
```

- [ ] **Step 5: Run unit tests**

Run: `go test ./internal/server/ -run TestAuthoring -v -count=1`
Expected: All tests pass

- [ ] **Step 6: Commit**

```text
test(server): update and add authoring handler tests for structured SpecifyOutput
```

---

## Chunk 4: E2E Tests and Fixtures

### Task 5: Update e2e API test fixtures

**Files:**

- Modify: `e2e/api/authoring_test.go:89-99`
- Modify: `e2e/api/pipeline_test.go:132-142`
- Modify: `e2e/api/lifecycle_pipeline_test.go:89-99`
- Modify: `e2e/api/errors_test.go:113-114`
- Verify: `e2e/cli/pipeline_test.go` (no edit needed — reads fixture file)

- [ ] **Step 1: Update each file's SpecifyOutput fixture**

Pattern — replace:

```go
Output: &specv1.SpecifyOutput{
    InterfaceContract: "POST /api/v1/things",
```

With:

```go
Output: &specv1.SpecifyOutput{
    Interfaces: []*specv1.InterfaceSection{
        {Name: "API", Body: "POST /api/v1/things"},
    },
```

For assertions, match the original semantics:

**authoring_test.go, pipeline_test.go** (emptiness check):

```go
// Before:
Expect(resp.Msg.Output.InterfaceContract).NotTo(BeEmpty())
// After:
Expect(resp.Msg.Output.Interfaces).NotTo(BeEmpty())
```

**lifecycle_pipeline_test.go** (value equality — MUST preserve `Equal`):

```go
// Before:
Expect(resp.Msg.Output.InterfaceContract).To(Equal("POST /api/v1/amended"))
// After:
Expect(resp.Msg.Output.Interfaces[0].Body).To(Equal("POST /api/v1/amended"))
```

**errors_test.go** (negative test — input fixture only, no output assertion):
This test verifies a failed stage transition, so `resp` is nil and there is
no output assertion. Only update the input fixture `InterfaceContract` →
`Interfaces`. Do NOT add output assertions.

Apply to all four files.

**Note:** `e2e/cli/pipeline_test.go` reads `e2e/cli/testdata/specify-output.json`
by path — no Go edit needed, the fixture update in Step 2 is sufficient.

- [ ] **Step 2: Update CLI test fixture**

Replace `e2e/cli/testdata/specify-output.json` with:

```json
{
  "interfaces": [
    {"name": "API", "body": "POST /api/v1/cli-pipeline"}
  ],
  "verifyCriteria": [
    {"category": "happy-path", "description": "returns 200 on valid input"},
    {"category": "validation", "description": "returns 400 on missing slug"}
  ],
  "invariants": [
    "spec state is consistent after each stage transition"
  ],
  "touches": [
    {"path": "internal/authoring/funnel.go", "purpose": "funnel logic", "changeType": "modify"},
    {"path": "internal/server/authoring_handler.go", "purpose": "handler updates", "changeType": "modify"}
  ]
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```text
test(e2e): update specify output fixtures for structured messages
```

---

## Chunk 5: Skill References and Documentation

### Task 6: Update skill format reference

**Files:**

- Modify: `plugin/specgraph/skills/specgraph-specify/references/specify-output-format.md`

- [ ] **Step 1: Replace the format reference**

Replace the entire file with the new schema showing `interfaces` (name+body),
`verifyCriteria` (category+description), `invariants` (strings), and `touches`
(path+purpose+changeType). See the design spec for the exact schema.

- [ ] **Step 2: Update SKILL.md persistence section**

In `plugin/specgraph/skills/specgraph-specify/SKILL.md`, update the JSON
example in the Persistence section (~line 188-219) to use the new schema shape.

- [ ] **Step 3: Update example spec**

In `site/docs/concepts/example-spec.md`, update the Specify section (~line
145-175) to show multiple interface sections and categorized criteria.

- [ ] **Step 4: Commit**

```text
docs: update specify output format references and example spec
```

---

## Chunk 6: Quality Gates

### Task 7: Run full quality gates

- [ ] **Step 1: Run task check**

Run: `task check`
Expected: All lint, build, and unit tests pass

- [ ] **Step 2: Run task pr-prep (if Docker available)**

Run: `task pr-prep`
Expected: Integration + e2e tests pass

- [ ] **Step 3: Create branch and PR**

```text
feat: restructure SpecifyOutput with InterfaceSection, VerifyCriterion, FileTouch

Closes: spgr-dp9, spgr-6vl
```
