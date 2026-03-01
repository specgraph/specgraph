# Slice 3: Authoring Funnel Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Author specs through the Spark-Shape-Specify-Decompose-Approve funnel with AI collaboration postures, analytical passes, and an always-on safety net.

**Architecture:** AuthoringService is a new ConnectRPC service with seven RPCs (Spark, Shape, Specify, Decompose, Approve, Amend, Supersede). Each RPC validates stage preconditions, transitions the spec, stores stage-specific output (scope, risks, decisions, analytical pass results), and returns structured data for the CLI. An AuthoringBackend interface abstracts Memgraph persistence. Analytical passes (Red Team, Peripheral Vision, Consistency, Simplicity, Constitution Check) produce typed findings stored on or linked to the spec node. Postures (Drive/Partner/Support) and prompt templates are server-side configuration served via a dedicated RPC. The codebase scanner adds Tier 1 (Shape) and Tier 2 (Specify) context gathering.

**Tech Stack:** Go, ConnectRPC (buf/connect-go), Memgraph (neo4j-go-driver/v5), Cobra, buf, testcontainers-go, protobuf

**Design Doc:** `docs/plans/2026-02-28-vertical-slice-roadmap-design.md` (Slice 3 section)

---

## Project Structure (new files)

```text
proto/specgraph/v1/
  authoring.proto                  # Authoring messages + AuthoringService
gen/specgraph/v1/                  # Generated (buf generate)
  authoring.pb.go
  specgraphv1connect/
    authoring.connect.go
internal/
  authoring/
    stages.go                      # Stage validation and transition rules
    stages_test.go                 # Stage transition tests
    posture.go                     # Posture types and detection heuristic
    posture_test.go                # Posture tests
    prompts.go                     # Prompt templates per stage
    prompts_test.go                # Prompt template tests
    safety.go                      # Safety net checks
    safety_test.go                 # Safety net tests
    passes.go                      # Analytical pass runner
    passes_test.go                 # Analytical pass tests
  storage/
    authoring.go                   # AuthoringBackend interface
  storage/memgraph/
    authoring.go                   # Memgraph implementation
    authoring_test.go              # Integration tests
  scanner/
    tier1.go                       # Tier 1 scanner (shape-level context)
    tier1_test.go                  # Tier 1 tests
    tier2.go                       # Tier 2 scanner (specify-level context)
    tier2_test.go                  # Tier 2 tests
  server/
    authoring_handler.go           # ConnectRPC handler
    authoring_handler_test.go      # Handler tests with mock
cmd/specgraph/
  spark.go                         # CLI: spark <slug>
  shape.go                         # CLI: shape <slug>
  specify.go                       # CLI: specify <slug>
  decompose.go                     # CLI: decompose <slug>
  approve.go                       # CLI: approve <slug>
```

---

## Task 1: Protobuf Schema -- Authoring Messages + AuthoringService

**Files:**

- Create: `proto/specgraph/v1/authoring.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/authoring.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";

// --- Enums ---

enum AuthoringStage {
  AUTHORING_STAGE_UNSPECIFIED = 0;
  AUTHORING_STAGE_SPARK = 1;
  AUTHORING_STAGE_SHAPE = 2;
  AUTHORING_STAGE_SPECIFY = 3;
  AUTHORING_STAGE_DECOMPOSE = 4;
  AUTHORING_STAGE_APPROVED = 5;
}

enum Posture {
  POSTURE_UNSPECIFIED = 0;
  POSTURE_DRIVE = 1;
  POSTURE_PARTNER = 2;
  POSTURE_SUPPORT = 3;
}

enum FindingSeverity {
  FINDING_SEVERITY_UNSPECIFIED = 0;
  FINDING_SEVERITY_CRITICAL = 1;
  FINDING_SEVERITY_WARNING = 2;
  FINDING_SEVERITY_NOTE = 3;
}

enum PeripheralDisposition {
  PERIPHERAL_DISPOSITION_UNSPECIFIED = 0;
  PERIPHERAL_DISPOSITION_ADDED_TO_SPEC = 1;
  PERIPHERAL_DISPOSITION_SEPARATE_SPEC = 2;
  PERIPHERAL_DISPOSITION_NOTE_FOR_IMPLEMENTER = 3;
}

enum DecompositionStrategy {
  DECOMPOSITION_STRATEGY_UNSPECIFIED = 0;
  DECOMPOSITION_STRATEGY_VERTICAL_SLICE = 1;
  DECOMPOSITION_STRATEGY_LAYER_CAKE = 2;
  DECOMPOSITION_STRATEGY_SINGLE_UNIT = 3;
}

// --- Stage Output Messages ---

message SparkOutput {
  string seed = 1;
  string signal = 2;
  repeated string questions = 3;
  string scope_sniff = 4;
  string kill_test = 5;
}

message ShapeOutput {
  repeated string scope_in = 1;
  repeated string scope_out = 2;
  repeated Approach approaches = 3;
  string chosen_approach = 4;
  repeated string risks = 5;
  repeated string success_must = 6;
  repeated string success_should = 7;
  repeated string success_wont = 8;
  repeated string decisions = 9;
}

message Approach {
  string name = 1;
  string description = 2;
  repeated string tradeoffs = 3;
}

message SpecifyOutput {
  string interface_contract = 1;
  repeated string verify_criteria = 2;
  repeated string invariants = 3;
  repeated string touches = 4;
}

message DecomposeOutput {
  DecompositionStrategy strategy = 1;
  repeated DecompositionSlice slices = 2;
}

message DecompositionSlice {
  string id = 1;
  string intent = 2;
  repeated string verify = 3;
  repeated string touches = 4;
  repeated string depends_on = 5;
}

// --- Analytical Pass Messages ---

message RedTeamFinding {
  FindingSeverity severity = 1;
  string finding = 2;
  string resolution = 3;
}

message PeripheralVisionItem {
  string item = 1;
  PeripheralDisposition disposition = 2;
}

message ConsistencyIssue {
  string type = 1;
  string description = 2;
  repeated string affected_specs = 3;
}

message SimplicityFinding {
  string area = 1;
  string suggestion = 2;
}

message ConstitutionViolation {
  string constraint = 1;
  string violation = 2;
  FindingSeverity severity = 3;
}

// --- Safety Net ---

message SafetyFlag {
  string category = 1;
  FindingSeverity severity = 2;
  string description = 3;
}

// --- Prompt Templates ---

message PromptTemplate {
  string stage = 1;
  string name = 2;
  string template = 3;
}

// --- Requests / Responses ---

message SparkRequest {
  string slug = 1;
  SparkOutput output = 2;
  Posture posture = 3;
}

message SparkResponse {
  SparkOutput output = 1;
  repeated SafetyFlag safety_flags = 2;
  repeated ConstitutionViolation constitution_violations = 3;
  repeated PromptTemplate next_prompts = 4;
}

message ShapeRequest {
  string slug = 1;
  ShapeOutput output = 2;
  Posture posture = 3;
}

message ShapeResponse {
  ShapeOutput output = 1;
  repeated PeripheralVisionItem peripheral_vision = 2;
  repeated SafetyFlag safety_flags = 3;
  repeated ConstitutionViolation constitution_violations = 4;
  repeated PromptTemplate next_prompts = 5;
}

message SpecifyRequest {
  string slug = 1;
  SpecifyOutput output = 2;
  Posture posture = 3;
}

message SpecifyResponse {
  SpecifyOutput output = 1;
  repeated RedTeamFinding red_team = 2;
  repeated ConsistencyIssue consistency_issues = 3;
  repeated SafetyFlag safety_flags = 4;
  repeated ConstitutionViolation constitution_violations = 5;
  repeated PromptTemplate next_prompts = 6;
}

message DecomposeRequest {
  string slug = 1;
  DecomposeOutput output = 2;
  Posture posture = 3;
}

message DecomposeResponse {
  DecomposeOutput output = 1;
  repeated SimplicityFinding simplicity = 2;
  repeated SafetyFlag safety_flags = 3;
  repeated ConstitutionViolation constitution_violations = 4;
}

message ApproveRequest {
  string slug = 1;
}

message ApproveResponse {
  string slug = 1;
  string stage = 2;
  google.protobuf.Timestamp approved_at = 3;
}

message AmendRequest {
  string slug = 1;
  string reason = 2;
  string target_stage = 3;
}

message AmendResponse {
  string slug = 1;
  string stage = 2;
  int32 version = 3;
}

message SupersedeRequest {
  string slug = 1;
  string superseded_by = 2;
  string reason = 3;
}

message SupersedeResponse {
  string slug = 1;
  string superseded_by = 2;
}

message GetPromptsRequest {
  string stage = 1;
}

message GetPromptsResponse {
  repeated PromptTemplate prompts = 1;
}

// --- Service ---

service AuthoringService {
  rpc Spark(SparkRequest) returns (SparkResponse);
  rpc Shape(ShapeRequest) returns (ShapeResponse);
  rpc Specify(SpecifyRequest) returns (SpecifyResponse);
  rpc Decompose(DecomposeRequest) returns (DecomposeResponse);
  rpc Approve(ApproveRequest) returns (ApproveResponse);
  rpc Amend(AmendRequest) returns (AmendResponse);
  rpc Supersede(SupersedeRequest) returns (SupersedeResponse);
  rpc GetPrompts(GetPromptsRequest) returns (GetPromptsResponse);
}
```

**Step 2: Run buf generate**

Run: `buf generate`
Expected: Clean generation, new files in `gen/specgraph/v1/`

**Step 3: Verify generated files exist**

Run: `ls gen/specgraph/v1/authoring.pb.go gen/specgraph/v1/specgraphv1connect/authoring.connect.go`
Expected: Both files listed

**Step 4: Run buf lint**

Run: `buf lint`
Expected: No errors

**Step 5: Commit**

```bash
git add proto/specgraph/v1/authoring.proto gen/specgraph/v1/authoring.pb.go gen/specgraph/v1/specgraphv1connect/authoring.connect.go
git commit -m "feat(authoring): add authoring.proto with funnel stages, passes, and safety net"
```

---

## Task 2: Stage Validation and Transition Rules

**Files:**

- Create: `internal/authoring/stages.go`
- Create: `internal/authoring/stages_test.go`

**Step 1: Write the failing test**

`internal/authoring/stages_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestValidTransitions(t *testing.T) {
	tests := []struct {
		from, to string
		valid    bool
	}{
		{"", "spark", true},
		{"spark", "shape", true},
		{"shape", "specify", true},
		{"specify", "decompose", true},
		{"decompose", "approved", true},
		{"shape", "spark", true},
		{"specify", "shape", true},
		{"decompose", "specify", true},
		{"approved", "decompose", true},
		{"approved", "spark", true},
		{"spark", "specify", false},
		{"spark", "approved", false},
		{"shape", "decompose", false},
		{"spark", "spark", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			err := authoring.ValidateTransition(tt.from, tt.to)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestAllStages(t *testing.T) {
	stages := authoring.AllStages()
	require.Equal(t, []string{"spark", "shape", "specify", "decompose", "approved"}, stages)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authoring/... -v -run TestValidTransitions`
Expected: FAIL (package does not exist)

**Step 3: Write minimal implementation**

`internal/authoring/stages.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package authoring implements the spec authoring funnel logic.
package authoring

import "fmt"

var stages = []string{"spark", "shape", "specify", "decompose", "approved"}

var forwardTransitions = map[string]string{
	"":           "spark",
	"spark":      "shape",
	"shape":      "specify",
	"specify":    "decompose",
	"decompose":  "approved",
}

var stageSet = func() map[string]bool {
	s := make(map[string]bool, len(stages))
	for _, st := range stages {
		s[st] = true
	}
	return s
}()

// AllStages returns the ordered list of authoring stages.
func AllStages() []string {
	return append([]string{}, stages...)
}

// ValidateTransition checks whether moving from one stage to another is allowed.
func ValidateTransition(from, to string) error {
	if from == to {
		return fmt.Errorf("cannot transition to same stage %q", from)
	}
	if to == "" || !stageSet[to] {
		return fmt.Errorf("invalid target stage %q", to)
	}
	if next, ok := forwardTransitions[from]; ok && next == to {
		return nil
	}
	if from != "" {
		fromIdx, toIdx := indexOf(from), indexOf(to)
		if fromIdx >= 0 && toIdx >= 0 && toIdx < fromIdx {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", from, to)
}

func indexOf(stage string) int {
	for i, s := range stages {
		if s == stage {
			return i
		}
	}
	return -1
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/authoring/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authoring/stages.go internal/authoring/stages_test.go
git commit -m "feat(authoring): add stage validation and transition rules"
```

---

## Task 3: Posture Types and Detection Heuristic

**Files:**

- Create: `internal/authoring/posture.go`
- Create: `internal/authoring/posture_test.go`

**Step 1: Write the failing test**

`internal/authoring/posture_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestDetectPosture(t *testing.T) {
	tests := []struct {
		name     string
		messages []string
		expected specv1.Posture
	}{
		{"short vague -> Drive", []string{"do it", "yes", "ok"}, specv1.Posture_POSTURE_DRIVE},
		{"long detailed -> Support", []string{"I have a very detailed plan for how this should work. The authentication system needs to use JWT tokens with refresh rotation. Here is the full specification with all the edge cases I've considered and the exact error handling I want."}, specv1.Posture_POSTURE_SUPPORT},
		{"medium -> Partner", []string{"Let's think about the auth flow", "What about using JWT?"}, specv1.Posture_POSTURE_PARTNER},
		{"empty -> Partner", []string{}, specv1.Posture_POSTURE_PARTNER},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, authoring.DetectPosture(tt.messages))
		})
	}
}

func TestResolvePosture(t *testing.T) {
	require.Equal(t, specv1.Posture_POSTURE_DRIVE,
		authoring.ResolvePosture(specv1.Posture_POSTURE_DRIVE, []string{"long detailed message here"}))
	require.Equal(t, specv1.Posture_POSTURE_DRIVE,
		authoring.ResolvePosture(specv1.Posture_POSTURE_UNSPECIFIED, []string{"ok"}))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authoring/... -v -run TestDetectPosture`
Expected: FAIL

**Step 3: Write minimal implementation**

`internal/authoring/posture.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

const (
	driveThreshold   = 20
	supportThreshold = 100
)

// DetectPosture infers a posture from message history.
func DetectPosture(messages []string) specv1.Posture {
	if len(messages) == 0 {
		return specv1.Posture_POSTURE_PARTNER
	}
	totalLen := 0
	for _, m := range messages {
		totalLen += len(m)
	}
	avg := totalLen / len(messages)
	switch {
	case avg < driveThreshold:
		return specv1.Posture_POSTURE_DRIVE
	case avg > supportThreshold:
		return specv1.Posture_POSTURE_SUPPORT
	default:
		return specv1.Posture_POSTURE_PARTNER
	}
}

// ResolvePosture returns the explicit posture if set, otherwise detects from messages.
func ResolvePosture(explicit specv1.Posture, messages []string) specv1.Posture {
	if explicit != specv1.Posture_POSTURE_UNSPECIFIED {
		return explicit
	}
	return DetectPosture(messages)
}
```

**Step 4: Run tests**

Run: `go test ./internal/authoring/... -v -run "TestDetectPosture|TestResolvePosture"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authoring/posture.go internal/authoring/posture_test.go
git commit -m "feat(authoring): add posture detection heuristic (Drive/Partner/Support)"
```

---

## Task 4: Prompt Templates

**Files:**

- Create: `internal/authoring/prompts.go`
- Create: `internal/authoring/prompts_test.go`

**Step 1: Write the failing test**

`internal/authoring/prompts_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestGetPrompts(t *testing.T) {
	prompts := authoring.GetPrompts("spark")
	require.NotEmpty(t, prompts)
	names := make([]string, len(prompts))
	for i, p := range prompts {
		names[i] = p.Name
		require.NotEmpty(t, p.Template)
	}
	require.Contains(t, names, "seed")
	require.Contains(t, names, "signal")
	require.Contains(t, names, "kill_test")

	require.NotEmpty(t, authoring.GetPrompts("shape"))
	require.NotEmpty(t, authoring.GetPrompts("specify"))
	require.NotEmpty(t, authoring.GetPrompts("decompose"))
	require.Empty(t, authoring.GetPrompts("nonexistent"))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authoring/... -v -run TestGetPrompts`
Expected: FAIL

**Step 3: Write minimal implementation**

`internal/authoring/prompts.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

// Prompt holds a named template for a stage.
type Prompt struct {
	Name     string
	Template string
}

var promptRegistry = map[string][]Prompt{
	"spark": {
		{Name: "seed", Template: "Describe the core idea in one sentence. What problem does this solve?"},
		{Name: "signal", Template: "What signal or event triggered this idea? Why now?"},
		{Name: "scope_sniff", Template: "At first glance, how big is this? (trivial / small / medium / large / epic)"},
		{Name: "unknowns", Template: "What don't you know yet? What needs investigation?"},
		{Name: "kill_test", Template: "What would make this idea not worth pursuing?"},
	},
	"shape": {
		{Name: "bound_scope", Template: "Define what is IN scope and what is OUT of scope."},
		{Name: "explore_solutions", Template: "Propose 2-3 approaches with tradeoffs. Which do you recommend?"},
		{Name: "identify_edges", Template: "What are the boundaries and integration points?"},
		{Name: "surface_risks", Template: "What could go wrong? Technical and product risks?"},
		{Name: "define_success", Template: "Define success: MUST / SHOULD / WON'T criteria."},
	},
	"specify": {
		{Name: "interface_contract", Template: "Define inputs, outputs, side effects, error cases."},
		{Name: "verify_criteria", Template: "Write testable verification criteria."},
		{Name: "invariants", Template: "State invariants: what must always be true?"},
	},
	"decompose": {
		{Name: "strategy", Template: "Choose: vertical_slice, layer_cake, or single_unit."},
		{Name: "slices", Template: "Break into slices with intent, verify, touches, and dependencies."},
	},
}

// GetPrompts returns the prompt templates for the given stage.
func GetPrompts(stage string) []Prompt {
	return promptRegistry[stage]
}

// PromptsToProto converts internal prompts to proto messages.
func PromptsToProto(stage string) []*specv1.PromptTemplate {
	prompts := GetPrompts(stage)
	result := make([]*specv1.PromptTemplate, len(prompts))
	for i, p := range prompts {
		result[i] = &specv1.PromptTemplate{
			Stage:    stage,
			Name:     p.Name,
			Template: p.Template,
		}
	}
	return result
}
```

**Step 4: Run tests**

Run: `go test ./internal/authoring/... -v -run TestGetPrompts`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authoring/prompts.go internal/authoring/prompts_test.go
git commit -m "feat(authoring): add prompt templates per funnel stage"
```

---

## Task 5: Safety Net Checks

**Files:**

- Create: `internal/authoring/safety.go`
- Create: `internal/authoring/safety_test.go`

**Step 1: Write the failing test**

`internal/authoring/safety_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestSafetyNet_SecurityFlags(t *testing.T) {
	flags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent: "Store user passwords in plaintext for faster lookup",
	})
	require.NotEmpty(t, flags)
	found := false
	for _, f := range flags {
		if f.Category == "security" {
			found = true
			require.Equal(t, specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, f.Severity)
		}
	}
	require.True(t, found)
}

func TestSafetyNet_DataLossFlags(t *testing.T) {
	flags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent: "Drop all tables and recreate schema without migration",
	})
	found := false
	for _, f := range flags {
		if f.Category == "data_loss" {
			found = true
		}
	}
	require.True(t, found)
}

func TestSafetyNet_Clean(t *testing.T) {
	flags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent: "Add a new read-only API endpoint for listing users",
	})
	require.Empty(t, flags)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authoring/... -v -run TestSafetyNet`
Expected: FAIL

**Step 3: Write minimal implementation**

`internal/authoring/safety.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import (
	"strings"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// SafetyInput is the data fed into the safety net.
type SafetyInput struct {
	Intent     string
	Scope      []string
	Invariants []string
}

// SafetyFlagResult is the internal representation of a safety flag.
type SafetyFlagResult struct {
	Category    string
	Severity    specv1.FindingSeverity
	Description string
}

var securityPatterns = []string{
	"plaintext", "hardcoded secret", "hardcoded password",
	"disable auth", "skip validation", "no encryption",
	"credential", "injection", "eval(", "exec(",
}

var dataLossPatterns = []string{
	"drop table", "drop all", "delete all", "truncate",
	"without migration", "without backup", "no rollback",
	"rm -rf", "force delete", "purge",
}

// RunSafetyNet checks the input for safety concerns. Always-on, cannot be disabled.
func RunSafetyNet(input *SafetyInput) []SafetyFlagResult {
	var flags []SafetyFlagResult
	text := strings.ToLower(input.Intent)
	for _, s := range input.Scope {
		text += " " + strings.ToLower(s)
	}
	for _, inv := range input.Invariants {
		text += " " + strings.ToLower(inv)
	}
	for _, pattern := range securityPatterns {
		if strings.Contains(text, pattern) {
			flags = append(flags, SafetyFlagResult{
				Category:    "security",
				Severity:    specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
				Description: "Potential security concern: " + pattern,
			})
		}
	}
	for _, pattern := range dataLossPatterns {
		if strings.Contains(text, pattern) {
			flags = append(flags, SafetyFlagResult{
				Category:    "data_loss",
				Severity:    specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
				Description: "Potential data loss: " + pattern,
			})
		}
	}
	return flags
}

// SafetyResultsToProto converts internal flags to proto messages.
func SafetyResultsToProto(flags []SafetyFlagResult) []*specv1.SafetyFlag {
	result := make([]*specv1.SafetyFlag, len(flags))
	for i, f := range flags {
		result[i] = &specv1.SafetyFlag{
			Category:    f.Category,
			Severity:    f.Severity,
			Description: f.Description,
		}
	}
	return result
}
```

**Step 4: Run tests**

Run: `go test ./internal/authoring/... -v -run TestSafetyNet`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authoring/safety.go internal/authoring/safety_test.go
git commit -m "feat(authoring): add always-on safety net (security, data loss)"
```

---

## Task 6: Analytical Pass Runner

**Files:**

- Create: `internal/authoring/passes.go`
- Create: `internal/authoring/passes_test.go`

**Step 1: Write the failing test**

`internal/authoring/passes_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestPassesForStage(t *testing.T) {
	tests := []struct {
		stage   string
		posture specv1.Posture
		expect  []string
	}{
		{"shape", specv1.Posture_POSTURE_DRIVE, []string{"peripheral_vision", "constitution_check"}},
		{"shape", specv1.Posture_POSTURE_PARTNER, []string{"constitution_check"}},
		{"specify", specv1.Posture_POSTURE_DRIVE, []string{"red_team", "consistency_check", "constitution_check"}},
		{"decompose", specv1.Posture_POSTURE_DRIVE, []string{"simplicity_check", "constitution_check"}},
		{"spark", specv1.Posture_POSTURE_DRIVE, []string{"constitution_check"}},
	}
	for _, tt := range tests {
		t.Run(tt.stage+"/"+tt.posture.String(), func(t *testing.T) {
			require.Equal(t, tt.expect, authoring.PassesForStage(tt.stage, tt.posture))
		})
	}
}

func TestOfferedPasses(t *testing.T) {
	offered := authoring.OfferedPasses("shape", specv1.Posture_POSTURE_PARTNER)
	require.Contains(t, offered, "peripheral_vision")

	offered = authoring.OfferedPasses("shape", specv1.Posture_POSTURE_DRIVE)
	require.Empty(t, offered)

	offered = authoring.OfferedPasses("specify", specv1.Posture_POSTURE_SUPPORT)
	require.Contains(t, offered, "red_team")
	require.Contains(t, offered, "consistency_check")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/authoring/... -v -run "TestPassesForStage|TestOfferedPasses"`
Expected: FAIL

**Step 3: Write minimal implementation**

`internal/authoring/passes.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring

import specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"

type passConfig struct {
	pass      string
	autoIn    []specv1.Posture
	offeredIn []specv1.Posture
}

var passRegistry = map[string][]passConfig{
	"spark": {
		{pass: "constitution_check", autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE, specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
	},
	"shape": {
		{pass: "peripheral_vision",
			autoIn:    []specv1.Posture{specv1.Posture_POSTURE_DRIVE},
			offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER},
		},
		{pass: "constitution_check", autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE, specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
	},
	"specify": {
		{pass: "red_team",
			autoIn:    []specv1.Posture{specv1.Posture_POSTURE_DRIVE},
			offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT},
		},
		{pass: "consistency_check",
			autoIn:    []specv1.Posture{specv1.Posture_POSTURE_DRIVE},
			offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT},
		},
		{pass: "constitution_check", autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE, specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
	},
	"decompose": {
		{pass: "simplicity_check",
			autoIn:    []specv1.Posture{specv1.Posture_POSTURE_DRIVE},
			offeredIn: []specv1.Posture{specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT},
		},
		{pass: "constitution_check", autoIn: []specv1.Posture{specv1.Posture_POSTURE_DRIVE, specv1.Posture_POSTURE_PARTNER, specv1.Posture_POSTURE_SUPPORT}},
	},
}

// PassesForStage returns passes that auto-run for the given stage and posture.
func PassesForStage(stage string, posture specv1.Posture) []string {
	var result []string
	for _, c := range passRegistry[stage] {
		for _, p := range c.autoIn {
			if p == posture {
				result = append(result, c.pass)
				break
			}
		}
	}
	return result
}

// OfferedPasses returns passes offered (not auto-run) for the given stage and posture.
func OfferedPasses(stage string, posture specv1.Posture) []string {
	var result []string
	for _, c := range passRegistry[stage] {
		for _, p := range c.offeredIn {
			if p == posture {
				result = append(result, c.pass)
				break
			}
		}
	}
	return result
}
```

**Step 4: Run tests**

Run: `go test ./internal/authoring/... -v -run "TestPassesForStage|TestOfferedPasses"`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/authoring/passes.go internal/authoring/passes_test.go
git commit -m "feat(authoring): add analytical pass runner with posture-aware scheduling"
```

---

## Task 7: AuthoringBackend Interface + Memgraph Implementation

**Files:**

- Create: `internal/storage/authoring.go`
- Create: `internal/storage/memgraph/authoring.go`
- Create: `internal/storage/memgraph/authoring_test.go`

**Step 1: Write the storage interface**

`internal/storage/authoring.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

var ErrInvalidStageTransition = errors.New("invalid stage transition")
var ErrSpecAlreadyApproved = errors.New("spec is already approved")

// AuthoringBackend defines storage operations for the authoring funnel.
type AuthoringBackend interface {
	TransitionStage(ctx context.Context, slug string, from, to string) error
	StoreSparkOutput(ctx context.Context, slug string, output *specv1.SparkOutput) error
	StoreShapeOutput(ctx context.Context, slug string, output *specv1.ShapeOutput) error
	StoreSpecifyOutput(ctx context.Context, slug string, output *specv1.SpecifyOutput) error
	StoreDecomposeOutput(ctx context.Context, slug string, output *specv1.DecomposeOutput) ([]*specv1.Spec, error)
	StoreRedTeamFindings(ctx context.Context, slug string, findings []*specv1.RedTeamFinding) error
	StorePeripheralVision(ctx context.Context, slug string, items []*specv1.PeripheralVisionItem) error
	StoreConsistencyIssues(ctx context.Context, slug string, issues []*specv1.ConsistencyIssue) error
	StoreSimplicityFindings(ctx context.Context, slug string, findings []*specv1.SimplicityFinding) error
	StoreSafetyFlags(ctx context.Context, slug string, flags []*specv1.SafetyFlag) error
	StoreConstitutionViolations(ctx context.Context, slug string, violations []*specv1.ConstitutionViolation) error
	SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error
	AmendSpec(ctx context.Context, slug, reason, targetStage string) (*specv1.Spec, error)
}
```

**Step 2: Write the failing integration test**

`internal/storage/memgraph/authoring_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph_test

import (
	"context"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/seanb4t/specgraph/internal/storage/memgraph"
	"github.com/stretchr/testify/require"
)

func TestTransitionStage(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "funnel-test", "Test the funnel", "p1", "low")
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "funnel-test", "", "spark")
	require.NoError(t, err)

	spec, err := store.GetSpec(ctx, "funnel-test")
	require.NoError(t, err)
	require.Equal(t, "spark", spec.Stage)

	err = store.TransitionStage(ctx, "funnel-test", "spark", "shape")
	require.NoError(t, err)

	err = store.TransitionStage(ctx, "funnel-test", "shape", "approved")
	require.ErrorIs(t, err, storage.ErrInvalidStageTransition)
}

func TestStoreSparkOutput(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "spark-out", "Spark output test", "p1", "low")
	require.NoError(t, err)

	err = store.StoreSparkOutput(ctx, "spark-out", &specv1.SparkOutput{
		Seed:       "Build a login system",
		Signal:     "User request",
		Questions:  []string{"OAuth or password?", "MFA required?"},
		ScopeSniff: "medium",
		KillTest:   "If no users need auth",
	})
	require.NoError(t, err)
}

func TestStoreDecomposeOutput(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "decomp-parent", "Parent spec", "p1", "medium")
	require.NoError(t, err)

	children, err := store.StoreDecomposeOutput(ctx, "decomp-parent", &specv1.DecomposeOutput{
		Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
		Slices: []*specv1.DecompositionSlice{
			{Id: "slice-1", Intent: "Auth endpoint", Verify: []string{"login works"}, Touches: []string{"auth.go"}},
			{Id: "slice-2", Intent: "Token refresh", Verify: []string{"refresh works"}, Touches: []string{"token.go"}, DependsOn: []string{"slice-1"}},
		},
	})
	require.NoError(t, err)
	require.Len(t, children, 2)
	require.Equal(t, "slice-1", children[0].Slug)
	require.Equal(t, "slice-2", children[1].Slug)
}

func TestAmendSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "amend-test", "Amend test", "p1", "low")
	require.NoError(t, err)
	require.NoError(t, store.TransitionStage(ctx, "amend-test", "", "spark"))
	require.NoError(t, store.TransitionStage(ctx, "amend-test", "spark", "shape"))
	require.NoError(t, store.TransitionStage(ctx, "amend-test", "shape", "specify"))

	spec, err := store.AmendSpec(ctx, "amend-test", "need to reconsider scope", "shape")
	require.NoError(t, err)
	require.Equal(t, "shape", spec.Stage)
}

func TestSupersedeSpec(t *testing.T) {
	boltURI, cleanup := setupMemgraph(t)
	defer cleanup()

	ctx := context.Background()
	store, err := memgraph.New(ctx, boltURI)
	require.NoError(t, err)
	defer store.Close(ctx)

	_, err = store.CreateSpec(ctx, "old-spec", "Original spec", "p1", "low")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "new-spec", "Replacement spec", "p1", "low")
	require.NoError(t, err)

	err = store.SupersedeSpec(ctx, "old-spec", "new-spec", "better approach found")
	require.NoError(t, err)
}
```

**Step 3: Run test to verify it fails**

Run: `go test ./internal/storage/memgraph/... -v -run "TestTransitionStage|TestStoreSparkOutput|TestStoreDecomposeOutput|TestAmendSpec|TestSupersedeSpec" -count=1 -timeout 120s`
Expected: FAIL (methods not defined)

**Step 4: Write Memgraph implementation**

`internal/storage/memgraph/authoring.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
)

func (s *Store) TransitionStage(ctx context.Context, slug string, from, to string) error {
	if err := authoring.ValidateTransition(from, to); err != nil {
		return fmt.Errorf("memgraph: %w: %w", storage.ErrInvalidStageTransition, err)
	}
	query := `
		MATCH (s:Spec {slug: $slug})
		WHERE s.stage = $from OR ($from = "" AND (s.stage IS NULL OR s.stage = ""))
		SET s.stage = $to, s.updated_at = datetime()
		RETURN s.slug
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "from": from, "to": to},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: transition stage: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: transition stage %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

func (s *Store) StoreSparkOutput(ctx context.Context, slug string, output *specv1.SparkOutput) error {
	return s.storeJSONProperty(ctx, slug, "spark_output", output)
}

func (s *Store) StoreShapeOutput(ctx context.Context, slug string, output *specv1.ShapeOutput) error {
	return s.storeJSONProperty(ctx, slug, "shape_output", output)
}

func (s *Store) StoreSpecifyOutput(ctx context.Context, slug string, output *specv1.SpecifyOutput) error {
	return s.storeJSONProperty(ctx, slug, "specify_output", output)
}

func (s *Store) StoreDecomposeOutput(ctx context.Context, slug string, output *specv1.DecomposeOutput) ([]*specv1.Spec, error) {
	if err := s.storeJSONProperty(ctx, slug, "decompose_output", output); err != nil {
		return nil, err
	}
	var children []*specv1.Spec
	for _, slice := range output.Slices {
		child, err := s.CreateSpec(ctx, slice.Id, slice.Intent, "p2", "medium")
		if err != nil {
			return nil, fmt.Errorf("memgraph: create child spec %q: %w", slice.Id, err)
		}
		query := `
			MATCH (child:Spec {slug: $child_slug}), (parent:Spec {slug: $parent_slug})
			CREATE (child)-[:COMPOSES]->(parent)
		`
		_, err = neo4j.ExecuteQuery(ctx, s.driver, query,
			map[string]any{"child_slug": slice.Id, "parent_slug": slug},
			neo4j.EagerResultTransformer)
		if err != nil {
			return nil, fmt.Errorf("memgraph: create COMPOSES edge: %w", err)
		}
		for _, dep := range slice.DependsOn {
			depQuery := `
				MATCH (from:Spec {slug: $from_slug}), (to:Spec {slug: $to_slug})
				CREATE (from)-[:DEPENDS_ON]->(to)
			`
			_, err = neo4j.ExecuteQuery(ctx, s.driver, depQuery,
				map[string]any{"from_slug": slice.Id, "to_slug": dep},
				neo4j.EagerResultTransformer)
			if err != nil {
				return nil, fmt.Errorf("memgraph: create DEPENDS_ON edge: %w", err)
			}
		}
		children = append(children, child)
	}
	return children, nil
}

func (s *Store) StoreRedTeamFindings(ctx context.Context, slug string, findings []*specv1.RedTeamFinding) error {
	return s.storeJSONProperty(ctx, slug, "red_team_findings", findings)
}

func (s *Store) StorePeripheralVision(ctx context.Context, slug string, items []*specv1.PeripheralVisionItem) error {
	return s.storeJSONProperty(ctx, slug, "peripheral_vision", items)
}

func (s *Store) StoreConsistencyIssues(ctx context.Context, slug string, issues []*specv1.ConsistencyIssue) error {
	return s.storeJSONProperty(ctx, slug, "consistency_issues", issues)
}

func (s *Store) StoreSimplicityFindings(ctx context.Context, slug string, findings []*specv1.SimplicityFinding) error {
	return s.storeJSONProperty(ctx, slug, "simplicity_findings", findings)
}

func (s *Store) StoreSafetyFlags(ctx context.Context, slug string, flags []*specv1.SafetyFlag) error {
	return s.storeJSONProperty(ctx, slug, "safety_flags", flags)
}

func (s *Store) StoreConstitutionViolations(ctx context.Context, slug string, violations []*specv1.ConstitutionViolation) error {
	return s.storeJSONProperty(ctx, slug, "constitution_violations", violations)
}

func (s *Store) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	query := `
		MATCH (old:Spec {slug: $old_slug}), (new:Spec {slug: $new_slug})
		SET old.stage = "superseded", old.updated_at = datetime()
		CREATE (new)-[:SUPERSEDES {reason: $reason}]->(old)
		RETURN old.slug
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"old_slug": slug, "new_slug": supersededBy, "reason": reason},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: supersede spec: %w", err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: supersede spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return nil
}

func (s *Store) AmendSpec(ctx context.Context, slug, reason, targetStage string) (*specv1.Spec, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, err
	}
	if err := authoring.ValidateTransition(spec.Stage, targetStage); err != nil {
		return nil, fmt.Errorf("memgraph: amend: %w: %w", storage.ErrInvalidStageTransition, err)
	}
	query := `
		MATCH (s:Spec {slug: $slug})
		SET s.stage = $stage, s.version = s.version + 1, s.updated_at = datetime()
		RETURN s.slug, s.id, s.intent, s.stage, s.priority, s.complexity, s.version,
		       s.created_at, s.updated_at
	`
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "stage": targetStage},
		neo4j.EagerResultTransformer)
	if err != nil {
		return nil, fmt.Errorf("memgraph: amend spec: %w", err)
	}
	if len(result.Records) == 0 {
		return nil, fmt.Errorf("memgraph: amend spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return recordToSpec(result.Records[0])
}

func (s *Store) storeJSONProperty(ctx context.Context, slug, property string, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("memgraph: marshal %s: %w", property, err)
	}
	query := fmt.Sprintf(`
		MATCH (s:Spec {slug: $slug})
		SET s.%s = $data, s.updated_at = datetime()
		RETURN s.slug
	`, property)
	result, err := neo4j.ExecuteQuery(ctx, s.driver, query,
		map[string]any{"slug": slug, "data": string(jsonBytes)},
		neo4j.EagerResultTransformer)
	if err != nil {
		return fmt.Errorf("memgraph: store %s: %w", property, err)
	}
	if len(result.Records) == 0 {
		return fmt.Errorf("memgraph: store %s for %q: %w", property, slug, storage.ErrSpecNotFound)
	}
	return nil
}
```

**Step 5: Run tests**

Run: `go test ./internal/storage/memgraph/... -v -run "TestTransitionStage|TestStoreSparkOutput|TestStoreDecomposeOutput|TestAmendSpec|TestSupersedeSpec" -count=1 -timeout 120s`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/storage/authoring.go internal/storage/memgraph/authoring.go internal/storage/memgraph/authoring_test.go
git commit -m "feat(authoring): add AuthoringBackend interface and Memgraph implementation"
```

---

## Task 8: ConnectRPC AuthoringService Handler

**Files:**

- Create: `internal/server/authoring_handler.go`
- Create: `internal/server/authoring_handler_test.go`

**Step 1: Write the handler**

`internal/server/authoring_handler.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/seanb4t/specgraph/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AuthoringHandler implements the ConnectRPC AuthoringService.
type AuthoringHandler struct {
	store   storage.AuthoringBackend
	backend storage.Backend
}

var _ specgraphv1connect.AuthoringServiceHandler = (*AuthoringHandler)(nil)

func (h *AuthoringHandler) Spark(ctx context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	msg := req.Msg
	if msg.Slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("slug is required"))
	}
	_, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Output.GetSeed(), "p2", "medium")
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.store.TransitionStage(ctx, msg.Slug, "", "spark"); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if err := h.store.StoreSparkOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{Intent: msg.Output.GetSeed()})
	return connect.NewResponse(&specv1.SparkResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto("shape"),
	}), nil
}

func (h *AuthoringHandler) Shape(ctx context.Context, req *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, "spark", "shape"); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if err := h.store.StoreShapeOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{Scope: msg.Output.GetScopeIn()})
	return connect.NewResponse(&specv1.ShapeResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto("specify"),
	}), nil
}

func (h *AuthoringHandler) Specify(ctx context.Context, req *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, "shape", "specify"); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if err := h.store.StoreSpecifyOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	safetyFlags := authoring.RunSafetyNet(&authoring.SafetyInput{
		Intent:     msg.Output.GetInterfaceContract(),
		Invariants: msg.Output.GetInvariants(),
	})
	return connect.NewResponse(&specv1.SpecifyResponse{
		Output:      msg.Output,
		SafetyFlags: authoring.SafetyResultsToProto(safetyFlags),
		NextPrompts: authoring.PromptsToProto("decompose"),
	}), nil
}

func (h *AuthoringHandler) Decompose(ctx context.Context, req *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	msg := req.Msg
	if err := h.store.TransitionStage(ctx, msg.Slug, "specify", "decompose"); err != nil {
		return nil, h.stageError(err)
	}
	if msg.Output != nil {
		if _, err := h.store.StoreDecomposeOutput(ctx, msg.Slug, msg.Output); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&specv1.DecomposeResponse{Output: msg.Output}), nil
}

func (h *AuthoringHandler) Approve(ctx context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if err := h.store.TransitionStage(ctx, req.Msg.Slug, "decompose", "approved"); err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.Slug,
		Stage:      "approved",
		ApprovedAt: timestamppb.Now(),
	}), nil
}

func (h *AuthoringHandler) Amend(ctx context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.AmendResponse], error) {
	spec, err := h.store.AmendSpec(ctx, req.Msg.Slug, req.Msg.Reason, req.Msg.TargetStage)
	if err != nil {
		return nil, h.stageError(err)
	}
	return connect.NewResponse(&specv1.AmendResponse{
		Slug:    spec.Slug,
		Stage:   spec.Stage,
		Version: spec.Version,
	}), nil
}

func (h *AuthoringHandler) Supersede(ctx context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if err := h.store.SupersedeSpec(ctx, req.Msg.Slug, req.Msg.SupersededBy, req.Msg.Reason); err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.SupersedeResponse{
		Slug:         req.Msg.Slug,
		SupersededBy: req.Msg.SupersededBy,
	}), nil
}

func (h *AuthoringHandler) GetPrompts(_ context.Context, req *connect.Request[specv1.GetPromptsRequest]) (*connect.Response[specv1.GetPromptsResponse], error) {
	return connect.NewResponse(&specv1.GetPromptsResponse{
		Prompts: authoring.PromptsToProto(req.Msg.Stage),
	}), nil
}

// RegisterAuthoringService registers the AuthoringService on the given mux.
func RegisterAuthoringService(mux *http.ServeMux, authoringStore storage.AuthoringBackend, backend storage.Backend) {
	handler := &AuthoringHandler{store: authoringStore, backend: backend}
	path, h := specgraphv1connect.NewAuthoringServiceHandler(handler)
	mux.Handle(path, h)
}

func (h *AuthoringHandler) stageError(err error) error {
	if errors.Is(err, storage.ErrInvalidStageTransition) {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if errors.Is(err, storage.ErrSpecNotFound) {
		return connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
```

**Step 2: Write the handler test**

`internal/server/authoring_handler_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/stretchr/testify/require"
)

func TestAuthoringHandler_GetPrompts(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, nil, nil)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: "spark",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)

	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
	}
	require.True(t, names["seed"])
	require.True(t, names["signal"])
	require.True(t, names["kill_test"])
}
```

**Step 3: Run tests**

Run: `go test ./internal/server/... -v -run TestAuthoringHandler -count=1`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/authoring_handler.go internal/server/authoring_handler_test.go
git commit -m "feat(authoring): add ConnectRPC AuthoringService handler"
```

---

## Task 9: CLI Authoring Commands

**Files:**

- Create: `cmd/specgraph/spark.go`
- Create: `cmd/specgraph/shape.go`
- Create: `cmd/specgraph/specify.go`
- Create: `cmd/specgraph/decompose.go`
- Create: `cmd/specgraph/approve.go`

**Step 1: Write spark command**

`cmd/specgraph/spark.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/spf13/cobra"
)

func authoringClient() (specgraphv1connect.AuthoringServiceClient, error) {
	return newClient(specgraphv1connect.NewAuthoringServiceClient)
}

var sparkCmd = &cobra.Command{
	Use:   "spark <slug>",
	Short: "Capture an idea and enter the authoring funnel",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpark,
}

var sparkSeed string

func init() {
	sparkCmd.Flags().StringVar(&sparkSeed, "seed", "", "seed idea (one sentence)")
	rootCmd.AddCommand(sparkCmd)
}

func runSpark(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   args[0],
		Output: &specv1.SparkOutput{Seed: sparkSeed},
	}))
	if err != nil {
		return fmt.Errorf("spark: %w", err)
	}
	fmt.Printf("Sparked: %s\n", args[0])
	if resp.Msg.Output != nil && resp.Msg.Output.Seed != "" {
		fmt.Printf("Seed: %s\n", resp.Msg.Output.Seed)
	}
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	if len(resp.Msg.NextPrompts) > 0 {
		fmt.Println("\nNext stage prompts (shape):")
		for _, p := range resp.Msg.NextPrompts {
			fmt.Printf("  - %s: %s\n", p.Name, p.Template)
		}
	}
	return nil
}
```

**Step 2: Write shape command**

`cmd/specgraph/shape.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var shapeCmd = &cobra.Command{
	Use:   "shape <slug>",
	Short: "Bound scope, explore solutions, and surface risks",
	Args:  cobra.ExactArgs(1),
	RunE:  runShape,
}

func init() {
	rootCmd.AddCommand(shapeCmd)
}

func runShape(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   args[0],
		Output: &specv1.ShapeOutput{},
	}))
	if err != nil {
		return fmt.Errorf("shape: %w", err)
	}
	fmt.Printf("Shaped: %s\n", args[0])
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	return nil
}
```

**Step 3: Write specify command**

`cmd/specgraph/specify.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var specifyCmd = &cobra.Command{
	Use:   "specify <slug>",
	Short: "Define interface contract, verification criteria, and invariants",
	Args:  cobra.ExactArgs(1),
	RunE:  runSpecify,
}

func init() {
	rootCmd.AddCommand(specifyCmd)
}

func runSpecify(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   args[0],
		Output: &specv1.SpecifyOutput{},
	}))
	if err != nil {
		return fmt.Errorf("specify: %w", err)
	}
	fmt.Printf("Specified: %s\n", args[0])
	for _, f := range resp.Msg.SafetyFlags {
		fmt.Printf("  [%s] %s: %s\n", f.Severity, f.Category, f.Description)
	}
	return nil
}
```

**Step 4: Write decompose command**

`cmd/specgraph/decompose.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var decomposeCmd = &cobra.Command{
	Use:   "decompose <slug>",
	Short: "Break a spec into work units",
	Args:  cobra.ExactArgs(1),
	RunE:  runDecompose,
}

func init() {
	rootCmd.AddCommand(decomposeCmd)
}

func runDecompose(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   args[0],
		Output: &specv1.DecomposeOutput{},
	}))
	if err != nil {
		return fmt.Errorf("decompose: %w", err)
	}
	fmt.Printf("Decomposed: %s\n", args[0])
	if resp.Msg.Output != nil {
		fmt.Printf("Strategy: %s\n", resp.Msg.Output.Strategy)
		for _, s := range resp.Msg.Output.Slices {
			fmt.Printf("  - %s: %s\n", s.Id, s.Intent)
		}
	}
	return nil
}
```

**Step 5: Write approve command**

`cmd/specgraph/approve.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/spf13/cobra"
)

var approveCmd = &cobra.Command{
	Use:   "approve <slug>",
	Short: "Mark a spec as approved and ready for execution",
	Args:  cobra.ExactArgs(1),
	RunE:  runApprove,
}

func init() {
	rootCmd.AddCommand(approveCmd)
}

func runApprove(_ *cobra.Command, args []string) error {
	client, err := authoringClient()
	if err != nil {
		return err
	}
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: args[0],
	}))
	if err != nil {
		return fmt.Errorf("approve: %w", err)
	}
	fmt.Printf("Approved: %s at %s\n", resp.Msg.Slug,
		resp.Msg.ApprovedAt.AsTime().Format(time.RFC3339))
	return nil
}
```

**Step 6: Verify build compiles**

Run: `go build ./cmd/specgraph/...`
Expected: Clean build

**Step 7: Commit**

```bash
git add cmd/specgraph/spark.go cmd/specgraph/shape.go cmd/specgraph/specify.go cmd/specgraph/decompose.go cmd/specgraph/approve.go
git commit -m "feat(authoring): add CLI commands (spark, shape, specify, decompose, approve)"
```

---

## Task 10: Codebase Scanner Tier 1 and Tier 2

**Files:**

- Create: `internal/scanner/tier1.go`
- Create: `internal/scanner/tier1_test.go`
- Create: `internal/scanner/tier2.go`
- Create: `internal/scanner/tier2_test.go`

**Step 1: Write the Tier 1 test**

`internal/scanner/tier1_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestTier1Scan(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "auth"), 0o755))
	goFile := filepath.Join(dir, "internal", "auth", "handler.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package auth\n\ntype Handler interface {\n\tLogin(user, pass string) error\n}\n\ntype Service struct {\n\tdb Database\n}\n"), 0o644))

	result, err := scanner.ScanTier1(dir)
	require.NoError(t, err)
	require.NotEmpty(t, result.Packages)
	require.NotEmpty(t, result.Interfaces)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/scanner/... -v -run TestTier1Scan`
Expected: FAIL (package does not exist)

**Step 3: Write Tier 1 implementation**

`internal/scanner/tier1.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package scanner provides codebase analysis at multiple tiers.
package scanner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Tier1Result contains shape-level codebase context.
type Tier1Result struct {
	Packages   []PackageInfo   `json:"packages"`
	Interfaces []InterfaceInfo `json:"interfaces"`
	Structs    []StructInfo    `json:"structs"`
}

type PackageInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type InterfaceInfo struct {
	Name    string   `json:"name"`
	Package string   `json:"package"`
	Methods []string `json:"methods"`
}

type StructInfo struct {
	Name    string   `json:"name"`
	Package string   `json:"package"`
	Fields  []string `json:"fields"`
}

// ScanTier1 performs a shape-level scan: packages, interfaces, key types.
func ScanTier1(root string) (*Tier1Result, error) {
	result := &Tier1Result{}
	fset := token.NewFileSet()
	seen := make(map[string]bool)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil
		}
		pkgName := f.Name.Name
		relPath, _ := filepath.Rel(root, filepath.Dir(path))
		if !seen[relPath] {
			seen[relPath] = true
			result.Packages = append(result.Packages, PackageInfo{Name: pkgName, Path: relPath})
		}
		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				switch t := typeSpec.Type.(type) {
				case *ast.InterfaceType:
					iface := InterfaceInfo{Name: typeSpec.Name.Name, Package: pkgName}
					for _, method := range t.Methods.List {
						for _, name := range method.Names {
							iface.Methods = append(iface.Methods, name.Name)
						}
					}
					result.Interfaces = append(result.Interfaces, iface)
				case *ast.StructType:
					s := StructInfo{Name: typeSpec.Name.Name, Package: pkgName}
					for _, field := range t.Fields.List {
						for _, name := range field.Names {
							s.Fields = append(s.Fields, name.Name)
						}
					}
					result.Structs = append(result.Structs, s)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
```

**Step 4: Run Tier 1 test**

Run: `go test ./internal/scanner/... -v -run TestTier1Scan`
Expected: PASS

**Step 5: Write Tier 2 test**

`internal/scanner/tier2_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanb4t/specgraph/internal/scanner"
	"github.com/stretchr/testify/require"
)

func TestTier2Scan(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal", "auth"), 0o755))
	goFile := filepath.Join(dir, "internal", "auth", "handler.go")
	require.NoError(t, os.WriteFile(goFile, []byte("package auth\n\nimport \"fmt\"\n\nfunc Login(user, pass string) error {\n\tfmt.Println(\"login\", user)\n\treturn nil\n}\n"), 0o644))
	testFile := filepath.Join(dir, "internal", "auth", "handler_test.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package auth_test\n\nimport \"testing\"\n\nfunc TestLogin(t *testing.T) {}\n"), 0o644))

	result, err := scanner.ScanTier2(dir, []string{"internal/auth"})
	require.NoError(t, err)
	require.NotEmpty(t, result.Functions)
	require.NotEmpty(t, result.TestFiles)
}
```

**Step 6: Write Tier 2 implementation**

`internal/scanner/tier2.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Tier2Result contains specify-level codebase context.
type Tier2Result struct {
	Functions []FunctionInfo `json:"functions"`
	Imports   []ImportInfo   `json:"imports"`
	TestFiles []string       `json:"test_files"`
}

type FunctionInfo struct {
	Name       string   `json:"name"`
	Package    string   `json:"package"`
	File       string   `json:"file"`
	Params     []string `json:"params"`
	IsExported bool     `json:"is_exported"`
}

type ImportInfo struct {
	Package string `json:"package"`
	Path    string `json:"path"`
}

// ScanTier2 performs a specify-level scan of specific directories.
func ScanTier2(root string, dirs []string) (*Tier2Result, error) {
	result := &Tier2Result{}
	fset := token.NewFileSet()
	for _, dir := range dirs {
		absDir := filepath.Join(root, dir)
		err := filepath.Walk(absDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() && path != absDir {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			relPath, _ := filepath.Rel(root, path)
			if strings.HasSuffix(path, "_test.go") {
				result.TestFiles = append(result.TestFiles, relPath)
				return nil
			}
			f, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if parseErr != nil {
				return nil
			}
			pkgName := f.Name.Name
			for _, imp := range f.Imports {
				importPath := strings.Trim(imp.Path.Value, `"`)
				result.Imports = append(result.Imports, ImportInfo{Package: pkgName, Path: importPath})
			}
			for _, decl := range f.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				fn := FunctionInfo{
					Name:       funcDecl.Name.Name,
					Package:    pkgName,
					File:       relPath,
					IsExported: ast.IsExported(funcDecl.Name.Name),
				}
				if funcDecl.Type.Params != nil {
					for _, param := range funcDecl.Type.Params.List {
						for _, name := range param.Names {
							fn.Params = append(fn.Params, name.Name)
						}
					}
				}
				result.Functions = append(result.Functions, fn)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
```

**Step 7: Run all scanner tests**

Run: `go test ./internal/scanner/... -v`
Expected: PASS

**Step 8: Commit**

```bash
git add internal/scanner/tier1.go internal/scanner/tier1_test.go internal/scanner/tier2.go internal/scanner/tier2_test.go
git commit -m "feat(scanner): add Tier 1 and Tier 2 codebase scanner"
```

---

## Task 11: Wire AuthoringService into Server

**Files:**

- Modify: `cmd/specgraph/serve.go:76` (add AuthoringService registration)

**Step 1: Add registration after existing services**

In `cmd/specgraph/serve.go`, after `server.RegisterClaimService(mux, store)` (line 76), add:

```go
server.RegisterAuthoringService(mux, store, store)
```

**Step 2: Verify build**

Run: `go build ./cmd/specgraph/...`
Expected: Clean build

**Step 3: Commit**

```bash
git add cmd/specgraph/serve.go
git commit -m "feat(authoring): wire AuthoringService into server mux"
```

---

## Task 12: End-to-End Verification

**Step 1: Run all unit tests**

Run: `go test ./internal/authoring/... -v -count=1`
Expected: All PASS

**Step 2: Run all integration tests**

Run: `go test ./internal/storage/memgraph/... -v -count=1 -timeout 120s`
Expected: All PASS

**Step 3: Run full build**

Run: `go build ./...`
Expected: Clean build

**Step 4: Run linter**

Run: `golangci-lint run ./...`
Expected: No errors

**Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(authoring): address lint and test issues from E2E verification"
```
