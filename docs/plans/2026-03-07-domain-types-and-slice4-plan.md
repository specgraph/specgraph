# Domain Types Consistency & Slice 4 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Refactor ConstitutionBackend and ClaimBackend to use domain types (Phase A), then implement Slice 4 ExecutionBackend with domain types from the start (Phase B). After completion, no file in `internal/storage/` or `internal/storage/memgraph/` imports from `gen/`.

**Architecture:** Storage interfaces return domain types defined in `internal/storage/`. Proto conversion happens in `internal/server/convert.go`. The emitter package also moves to domain types. Config's `ToProto()` becomes `ToDomain()`.

**Tech Stack:** Go, ConnectRPC, Memgraph, Cobra, buf, testcontainers-go

**Design Doc:** `docs/plans/2026-03-07-domain-types-consistency-design.md`

---

## Project Structure (new/modified files)

```text
internal/storage/
  constitution_domain.go          # NEW: Constitution domain types
  claim_domain.go                 # NEW: Claim domain type
  execution_domain.go             # NEW: Execution domain types
  constitution.go                 # MODIFY: remove specv1 import
  claim.go                        # MODIFY: remove specv1 import
  execution.go                    # NEW: ExecutionBackend interface
internal/storage/memgraph/
  constitution.go                 # MODIFY: return domain types
  claim.go                        # MODIFY: return domain types
  execution.go                    # NEW: Memgraph execution backend
  execution_test.go               # NEW: Integration tests
internal/server/
  convert.go                      # MODIFY: add constitution/claim/execution converters
  constitution_handler.go         # MODIFY: convert at boundary
  claim_handler.go                # MODIFY: convert at boundary
  constitution_handler_test.go    # MODIFY: mock returns domain types
  claim_handler_test.go           # MODIFY: mock returns domain types
  execution_handler.go            # NEW: ExecutionService handler
  execution_handler_test.go       # NEW: Handler tests
  sweeper.go                      # NEW: Background lease sweeper
  sweeper_test.go                 # NEW: Sweeper tests
internal/emitter/
  emitter.go                      # MODIFY: accept domain types
  emitter_test.go                 # MODIFY: use domain types
internal/config/
  config.go                       # MODIFY: ToDomain() replaces ToProto()
cmd/specgraph/
  serve.go                        # MODIFY: use ToDomain(), wire execution
  serve_test.go                   # MODIFY: mock returns domain types
  bundle.go                       # NEW: CLI bundle command
  progress.go                     # NEW: CLI progress command
proto/specgraph/v1/
  execution.proto                 # NEW: ExecutionService proto
```

---

## Phase A: Refactor Constitution & Claim to Domain Types

---

### Task 1: Constitution Domain Types

**Files:**

- Create: `internal/storage/constitution_domain.go`

**Step 1: Create the domain types file**

`internal/storage/constitution_domain.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// ConstitutionLayer defines the scope at which a constitution applies.
type ConstitutionLayer string

const (
	ConstitutionLayerUnspecified ConstitutionLayer = ""
	ConstitutionLayerUser       ConstitutionLayer = "user"
	ConstitutionLayerOrg        ConstitutionLayer = "org"
	ConstitutionLayerProject    ConstitutionLayer = "project"
	ConstitutionLayerDomain     ConstitutionLayer = "domain"
)

// ViolationSeverity classifies how critical a constitution violation is.
type ViolationSeverity string

const (
	ViolationSeverityError   ViolationSeverity = "error"
	ViolationSeverityWarning ViolationSeverity = "warning"
	ViolationSeverityInfo    ViolationSeverity = "info"
)

// Constitution is the project ground truth.
type Constitution struct {
	ID           string
	Layer        ConstitutionLayer
	Name         string
	Version      int32
	Tech         *TechStack
	Principles   []Principle
	Process      *ProcessConfig
	Constraints  []string
	Antipatterns []Antipattern
	References   []Reference
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TechStack describes the technology stack for a project.
type TechStack struct {
	Languages      *Languages
	Frameworks     map[string]string
	Infrastructure map[string]string
	APIStandards   map[string]string
	Data           map[string]string
}

// Languages specifies which programming languages are permitted.
type Languages struct {
	Primary          string
	Allowed          []string
	Forbidden        []string
	ForbiddenReasons map[string]string
}

// Principle captures a guiding design or engineering principle.
type Principle struct {
	ID         string
	Statement  string
	Rationale  string
	Exceptions string
}

// ProcessConfig describes the team's review and deployment processes.
type ProcessConfig struct {
	SpecReview     string
	SecurityReview *SecurityReviewConfig
	Deployment     *DeploymentConfig
	Documentation  *DocumentationConfig
}

// SecurityReviewConfig describes when security reviews are required.
type SecurityReviewConfig struct {
	When string
}

// DeploymentConfig describes the deployment strategy.
type DeploymentConfig struct {
	Strategy string
	Rollback string
}

// DocumentationConfig describes documentation requirements.
type DocumentationConfig struct {
	APIDocs string
	Runbook string
}

// Antipattern describes a known bad practice to avoid.
type Antipattern struct {
	Pattern string
	Why     string
	Instead string
}

// Reference points to an external document related to the constitution.
type Reference struct {
	Type string
	Path string
}

// Violation represents a single constitution rule violation found in a spec.
type Violation struct {
	Rule     string
	Severity ViolationSeverity
	Message  string
	SpecSlug string
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/constitution_domain.go
git commit -m "feat(storage): add constitution domain types"
```

---

### Task 2: Claim Domain Type

**Files:**

- Create: `internal/storage/claim_domain.go`

**Step 1: Create the domain type file**

`internal/storage/claim_domain.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// Claim represents an active claim/lease on a spec by an agent.
type Claim struct {
	Slug         string
	Agent        string
	LeaseExpires time.Time
	ClaimedAt    time.Time
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 3: Commit**

```bash
git add internal/storage/claim_domain.go
git commit -m "feat(storage): add claim domain type"
```

---

### Task 3: Update Storage Interfaces to Domain Types

**Files:**

- Modify: `internal/storage/constitution.go`
- Modify: `internal/storage/claim.go`

**Step 1: Update constitution.go**

Replace the entire file with:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// ErrConstitutionNotFound is returned when no constitution exists.
var ErrConstitutionNotFound = errors.New("constitution not found")

// ConstitutionBackend defines storage operations for the project constitution.
type ConstitutionBackend interface {
	// GetConstitution returns the active constitution.
	GetConstitution(ctx context.Context) (*Constitution, error)

	// UpdateConstitution stores or replaces the constitution, bumping its version.
	UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)

	// CheckViolation checks a spec against constitution constraints.
	// Returns a list of violations (empty if compliant).
	CheckViolation(ctx context.Context, specSlug string) ([]Violation, error)
}
```

**Step 2: Update claim.go**

Replace the entire file with:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"time"
)

// ErrSpecNotFound is returned when a spec does not exist.
var ErrSpecNotFound = errors.New("spec not found")

// ErrSpecAlreadyClaimed is returned when a spec has an active claim by another agent.
var ErrSpecAlreadyClaimed = errors.New("spec already claimed")

// ErrNotClaimOwner is returned when the agent does not own the claim.
var ErrNotClaimOwner = errors.New("agent does not own the claim")

// ErrSpecNotClaimed is returned when the spec is not claimed.
var ErrSpecNotClaimed = errors.New("spec is not claimed")

// ClaimBackend defines storage operations for spec claims/leases.
type ClaimBackend interface {
	// ClaimSpec creates a CLAIMED_BY relationship between a spec and an agent.
	ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*Claim, error)

	// UnclaimSpec removes the CLAIMED_BY relationship.
	UnclaimSpec(ctx context.Context, slug, agent string) error

	// Heartbeat extends the lease for a claimed spec.
	Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*Claim, error)
}
```

**Step 3: Verify storage package compiles (will fail — dependents need updating)**

```bash
go build ./internal/storage/
```

Expected: PASS (the storage package itself compiles — it only defines interfaces)

**Step 4: Commit**

```bash
git add internal/storage/constitution.go internal/storage/claim.go
git commit -m "refactor(storage): update constitution and claim interfaces to domain types"
```

---

### Task 4: Add Conversion Functions in convert.go

**Files:**

- Modify: `internal/server/convert.go`

**Step 1: Add constitution and claim converters**

Append after the existing edge converters (after line 163):

```go
// --- Constitution ---

var constitutionLayerToProtoMap = map[storage.ConstitutionLayer]specv1.ConstitutionLayer{
	storage.ConstitutionLayerUser:    specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
	storage.ConstitutionLayerOrg:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
	storage.ConstitutionLayerProject: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
	storage.ConstitutionLayerDomain:  specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
}

var constitutionLayerFromProtoMap = map[specv1.ConstitutionLayer]storage.ConstitutionLayer{
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:    storage.ConstitutionLayerUser,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:     storage.ConstitutionLayerOrg,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT: storage.ConstitutionLayerProject,
	specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:  storage.ConstitutionLayerDomain,
}

var violationSeverityToProtoMap = map[storage.ViolationSeverity]specv1.ViolationSeverity{
	storage.ViolationSeverityError:   specv1.ViolationSeverity_VIOLATION_SEVERITY_ERROR,
	storage.ViolationSeverityWarning: specv1.ViolationSeverity_VIOLATION_SEVERITY_WARNING,
	storage.ViolationSeverityInfo:    specv1.ViolationSeverity_VIOLATION_SEVERITY_INFO,
}

func constitutionToProto(c *storage.Constitution) *specv1.Constitution {
	if c == nil {
		return nil
	}
	pb := &specv1.Constitution{
		Id:           c.ID,
		Layer:        constitutionLayerToProtoMap[c.Layer],
		Name:         c.Name,
		Version:      c.Version,
		Constraints:  c.Constraints,
		CreatedAt:    timeToProto(c.CreatedAt),
		UpdatedAt:    timeToProto(c.UpdatedAt),
	}

	if c.Tech != nil {
		pb.Tech = techStackToProto(c.Tech)
	}
	for _, p := range c.Principles {
		pb.Principles = append(pb.Principles, &specv1.Principle{
			Id:         p.ID,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if c.Process != nil {
		pb.Process = processConfigToProto(c.Process)
	}
	for _, ap := range c.Antipatterns {
		pb.Antipatterns = append(pb.Antipatterns, &specv1.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range c.References {
		pb.References = append(pb.References, &specv1.Reference{
			Path: ref.Path,
		})
	}

	return pb
}

func techStackToProto(t *storage.TechStack) *specv1.TechConfig {
	pb := &specv1.TechConfig{
		Frameworks:     t.Frameworks,
		Infrastructure: t.Infrastructure,
		ApiStandards:   t.APIStandards,
		Data:           t.Data,
	}
	if t.Languages != nil {
		pb.Languages = &specv1.LanguageConfig{
			Primary:          t.Languages.Primary,
			Allowed:          t.Languages.Allowed,
			Forbidden:        t.Languages.Forbidden,
			ForbiddenReasons: t.Languages.ForbiddenReasons,
		}
	}
	return pb
}

func processConfigToProto(p *storage.ProcessConfig) *specv1.ProcessConfig {
	pb := &specv1.ProcessConfig{
		SpecReview: p.SpecReview,
	}
	if p.SecurityReview != nil {
		pb.SecurityReview = &specv1.SecurityReviewConfig{When: p.SecurityReview.When}
	}
	if p.Deployment != nil {
		pb.Deployment = &specv1.DeploymentConfig{
			Strategy: p.Deployment.Strategy,
			Rollback: p.Deployment.Rollback,
		}
	}
	if p.Documentation != nil {
		pb.Documentation = &specv1.DocumentationConfig{
			ApiDocs: p.Documentation.APIDocs,
			Runbook: p.Documentation.Runbook,
		}
	}
	return pb
}

func constitutionFromProto(pb *specv1.Constitution) *storage.Constitution {
	if pb == nil {
		return nil
	}
	c := &storage.Constitution{
		ID:          pb.Id,
		Layer:       constitutionLayerFromProtoMap[pb.Layer],
		Name:        pb.Name,
		Version:     pb.Version,
		Constraints: pb.Constraints,
	}
	if pb.CreatedAt != nil {
		c.CreatedAt = pb.CreatedAt.AsTime()
	}
	if pb.UpdatedAt != nil {
		c.UpdatedAt = pb.UpdatedAt.AsTime()
	}
	if pb.Tech != nil {
		c.Tech = techStackFromProto(pb.Tech)
	}
	for _, p := range pb.Principles {
		c.Principles = append(c.Principles, storage.Principle{
			ID:         p.Id,
			Statement:  p.Statement,
			Rationale:  p.Rationale,
			Exceptions: p.Exceptions,
		})
	}
	if pb.Process != nil {
		c.Process = processConfigFromProto(pb.Process)
	}
	for _, ap := range pb.Antipatterns {
		c.Antipatterns = append(c.Antipatterns, storage.Antipattern{
			Pattern: ap.Pattern,
			Why:     ap.Why,
			Instead: ap.Instead,
		})
	}
	for _, ref := range pb.References {
		c.References = append(c.References, storage.Reference{
			Path: ref.Path,
		})
	}
	return c
}

func techStackFromProto(pb *specv1.TechConfig) *storage.TechStack {
	t := &storage.TechStack{
		Frameworks:     pb.Frameworks,
		Infrastructure: pb.Infrastructure,
		APIStandards:   pb.ApiStandards,
		Data:           pb.Data,
	}
	if pb.Languages != nil {
		t.Languages = &storage.Languages{
			Primary:          pb.Languages.Primary,
			Allowed:          pb.Languages.Allowed,
			Forbidden:        pb.Languages.Forbidden,
			ForbiddenReasons: pb.Languages.ForbiddenReasons,
		}
	}
	return t
}

func processConfigFromProto(pb *specv1.ProcessConfig) *storage.ProcessConfig {
	p := &storage.ProcessConfig{
		SpecReview: pb.SpecReview,
	}
	if pb.SecurityReview != nil {
		p.SecurityReview = &storage.SecurityReviewConfig{When: pb.SecurityReview.When}
	}
	if pb.Deployment != nil {
		p.Deployment = &storage.DeploymentConfig{
			Strategy: pb.Deployment.Strategy,
			Rollback: pb.Deployment.Rollback,
		}
	}
	if pb.Documentation != nil {
		p.Documentation = &storage.DocumentationConfig{
			APIDocs: pb.Documentation.ApiDocs,
			Runbook: pb.Documentation.Runbook,
		}
	}
	return p
}

func violationsToProto(violations []storage.Violation) []*specv1.Violation {
	result := make([]*specv1.Violation, len(violations))
	for i, v := range violations {
		result[i] = &specv1.Violation{
			Rule:     v.Rule,
			Severity: violationSeverityToProtoMap[v.Severity],
			Message:  v.Message,
			SpecSlug: v.SpecSlug,
		}
	}
	return result
}

// --- Claim ---

func claimToProto(c *storage.Claim) *specv1.Claim {
	return &specv1.Claim{
		SpecSlug:     c.Slug,
		Agent:        c.Agent,
		ClaimedAt:    timeToProto(c.ClaimedAt),
		LeaseExpires: timeToProto(c.LeaseExpires),
	}
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/server/
```

Expected: FAIL — handlers still pass proto types, will fix in next task.

**Step 3: Commit**

```bash
git add internal/server/convert.go
git commit -m "feat(server): add constitution, claim, and violation converters"
```

---

### Task 5: Update Memgraph Constitution Implementation

**Files:**

- Modify: `internal/storage/memgraph/constitution.go`

**Step 1: Rewrite to return domain types**

The key changes:

1. Remove `specv1` and `timestamppb` imports
2. `GetConstitution` returns `*storage.Constitution`
3. `UpdateConstitution` accepts/returns `*storage.Constitution`
4. `CheckViolation` returns `[]storage.Violation`
5. `recordToConstitution` returns `*storage.Constitution`

The implementation logic stays the same — only the return types and field mappings change. The Memgraph storage now creates `storage.Constitution` structs instead of `specv1.Constitution` protos.

Key mapping changes in `recordToConstitution`:

- `specv1.ConstitutionLayer(layerVal)` → `storage.ConstitutionLayer` string constant lookup
- `timestamppb.New(t)` → `time.Time` directly
- `specv1.Constitution{...}` → `storage.Constitution{...}`
- Nested proto types → domain types (TechStack, Languages, Principle, etc.)

In `UpdateConstitution`, where it serializes to JSON for Memgraph:

- `constitution.Tech.Languages.Primary` etc. same field access, just different source type
- JSON marshalling of struct fields works identically

In `CheckViolation`:

- `specv1.Violation{...}` → `storage.Violation{...}`
- `specv1.ViolationSeverity_VIOLATION_SEVERITY_ERROR` → `storage.ViolationSeverityError`

**Step 2: Verify integration tests still compile**

```bash
go build ./internal/storage/memgraph/
```

Expected: FAIL — tests still use proto types (fixed in Task 6)

**Step 3: Commit**

```bash
git add internal/storage/memgraph/constitution.go
git commit -m "refactor(memgraph): constitution backend returns domain types"
```

---

### Task 6: Update Memgraph Claim Implementation

**Files:**

- Modify: `internal/storage/memgraph/claim.go`

**Step 1: Rewrite to return domain types**

Key changes:

1. Remove `specv1` and `timestamppb` imports
2. `ClaimSpec` returns `*storage.Claim`
3. `Heartbeat` returns `*storage.Claim`
4. `recordToClaim` returns `*storage.Claim` (domain)

In `recordToClaim`:

- `specv1.Claim{SpecSlug: slug, Agent: agent, ...}` → `storage.Claim{Slug: slug, Agent: agent, ...}`
- `timestamppb.New(t)` → `t` (time.Time directly)

**Step 2: Commit**

```bash
git add internal/storage/memgraph/claim.go
git commit -m "refactor(memgraph): claim backend returns domain types"
```

---

### Task 7: Update Memgraph Tests

**Files:**

- Modify: `internal/storage/memgraph/constitution_test.go`
- Modify: `internal/storage/memgraph/claim_test.go`

**Step 1: Update constitution_test.go**

Key changes:

- Remove `specv1` import
- `&specv1.Constitution{...}` → `&storage.Constitution{...}`
- `specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT` → `storage.ConstitutionLayerProject`
- Proto sub-message constructors → domain type constructors
- `got.Layer` comparison uses domain enum

**Step 2: Update claim_test.go**

Key changes:

- Remove `specv1` import
- Assertions on returned `*storage.Claim` instead of `*specv1.Claim`
- `claim.SpecSlug` → `claim.Slug`
- `claim.LeaseExpires.AsTime()` → `claim.LeaseExpires` (already time.Time)

**Step 3: Run integration tests**

```bash
go test ./internal/storage/memgraph/ -run "TestConstitution|TestClaimAndUnclaim|TestHeartbeat" -v -count=1 -timeout=120s
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/storage/memgraph/constitution_test.go internal/storage/memgraph/claim_test.go
git commit -m "test(memgraph): update constitution and claim tests for domain types"
```

---

### Task 8: Update Emitter to Domain Types

**Files:**

- Modify: `internal/emitter/emitter.go`
- Modify: `internal/emitter/emitter_test.go`

**Step 1: Update emitter.go**

Key changes:

1. Replace `specv1` import with `storage` import
2. `Emit(c *specv1.Constitution, format specv1.OutputFormat)` → `Emit(c *storage.Constitution, format string)`
3. All internal functions accept `*storage.Constitution`
4. Field access stays the same but types change:
   - `c.Tech.Languages.Primary` (same)
   - `c.Tech.Languages.ForbiddenReasons` (same)
   - `c.Tech.Frameworks` (same — map[string]string)
   - `c.Principles[i].Statement` (same)
   - `c.Antipatterns[i].Pattern` (same)

The `format` parameter changes from proto enum to string. The handler will pass the string value.

Format constants:

- `"claude-md"` → CLAUDE.md
- `"cursorrules"` → .cursorrules
- `"agents-md"` → AGENTS.md

**Step 2: Update emitter_test.go**

Key changes:

- Replace `specv1` import with `storage`
- `&specv1.Constitution{...}` → `&storage.Constitution{...}`
- `specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD` → `"claude-md"`

**Step 3: Verify tests pass**

```bash
go test ./internal/emitter/ -v -count=1
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/emitter/emitter.go internal/emitter/emitter_test.go
git commit -m "refactor(emitter): accept domain types instead of proto types"
```

---

### Task 9: Update Handlers to Convert at Boundary

**Files:**

- Modify: `internal/server/constitution_handler.go`
- Modify: `internal/server/claim_handler.go`

**Step 1: Update constitution_handler.go**

Key changes:

1. `GetConstitution`: `h.store.GetConstitution(ctx)` returns `*storage.Constitution` → convert with `constitutionToProto(c)` before returning
2. `UpdateConstitution`: `msg.Constitution` is `*specv1.Constitution` → convert with `constitutionFromProto(msg.Constitution)` before passing to store, then convert result back
3. `CheckViolation`: `h.store.CheckViolation(ctx, slug)` returns `[]storage.Violation` → convert with `violationsToProto(violations)` before returning
4. `EmitToolFiles`: `h.store.GetConstitution(ctx)` returns domain type → pass domain type to `emitter.Emit(c, formatStr)` where `formatStr` is derived from the proto enum

For `EmitToolFiles`, add a format mapping:

```go
var outputFormatMap = map[specv1.OutputFormat]string{
	specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD:   "claude-md",
	specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES: "cursorrules",
	specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD:   "agents-md",
}
```

**Step 2: Update claim_handler.go**

Key changes:

1. `ClaimSpec`: `h.store.ClaimSpec(...)` returns `*storage.Claim` → convert with `claimToProto(claim)` before returning
2. `Heartbeat`: same conversion pattern

**Step 3: Verify handlers compile**

```bash
go build ./internal/server/
```

**Step 4: Commit**

```bash
git add internal/server/constitution_handler.go internal/server/claim_handler.go
git commit -m "refactor(server): constitution and claim handlers convert at boundary"
```

---

### Task 10: Update Handler Tests

**Files:**

- Modify: `internal/server/constitution_handler_test.go`
- Modify: `internal/server/claim_handler_test.go`

**Step 1: Update constitution_handler_test.go**

Key changes:

- Mock backend returns `*storage.Constitution` and `[]storage.Violation` instead of proto types
- Test assertions on RPC responses still use proto types (that's correct — tests are RPC clients)
- The mock `GetConstitution()` returns `&storage.Constitution{Layer: storage.ConstitutionLayerProject, ...}`
- The mock `UpdateConstitution()` stores `*storage.Constitution`
- The mock `CheckViolation()` returns `[]storage.Violation{...}`

**Step 2: Update claim_handler_test.go**

Key changes:

- Mock backend returns `*storage.Claim` instead of `*specv1.Claim`
- `&specv1.Claim{SpecSlug: slug, ...}` → `&storage.Claim{Slug: slug, ...}`

**Step 3: Run handler tests**

```bash
go test ./internal/server/ -run "TestConstitution|TestClaim" -v -count=1
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/server/constitution_handler_test.go internal/server/claim_handler_test.go
git commit -m "test(server): update constitution and claim handler tests for domain types"
```

---

### Task 11: Update Config and Serve Bootstrap

**Files:**

- Modify: `internal/config/config.go` — rename `ToProto()` to `ToDomain()`, `ConstitutionConfigFromProto()` to `ConstitutionConfigFromDomain()`
- Modify: `cmd/specgraph/serve.go` — call `ToDomain()` instead of `ToProto()`
- Modify: `cmd/specgraph/serve_test.go` — mock returns domain types

**Step 1: Update config.go**

- `ToProto() *specv1.Constitution` → `ToDomain() *storage.Constitution`
- `ConstitutionConfigFromProto(pb *specv1.Constitution)` → `ConstitutionConfigFromDomain(c *storage.Constitution)`
- Replace proto imports with storage imports
- Map proto enum values to domain enum values

**Step 2: Update serve.go**

- Line 141: `constitution := cy.ToProto()` → `constitution := cy.ToDomain()`
- `bootstrapConstitution` passes `*storage.Constitution` to `store.UpdateConstitution`
- No proto import needed in serve.go

**Step 3: Update serve_test.go**

- Mock `GetConstitution()` returns `*storage.Constitution`
- Mock `UpdateConstitution()` accepts `*storage.Constitution`
- Mock `CheckViolation()` returns `[]storage.Violation`

**Step 4: Run tests**

```bash
go test ./cmd/specgraph/ -run TestBootstrap -v -count=1
go test ./internal/config/ -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/config/config.go cmd/specgraph/serve.go cmd/specgraph/serve_test.go
git commit -m "refactor: config and serve bootstrap use domain types"
```

---

### Task 12: Phase A Verification

**Step 1: Verify no proto imports in storage layer**

```bash
grep -r 'specv1\|specgraphv1' internal/storage/ --include="*.go"
```

Expected: NO matches

**Step 2: Run all tests**

```bash
task check
```

Expected: PASS

**Step 3: Commit any remaining fixes**

---

## Phase B: Slice 4 — Execution Bundles & Prime

---

### Task 13: Protobuf Schema — execution.proto

**Files:**

- Create: `proto/specgraph/v1/execution.proto`

**Step 1: Write the proto file**

`proto/specgraph/v1/execution.proto`:

```protobuf
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

syntax = "proto3";

package specgraph.v1;

option go_package = "github.com/seanb4t/specgraph/gen/specgraph/v1;specgraphv1";

import "google/protobuf/timestamp.proto";
import "specgraph/v1/spec.proto";
import "specgraph/v1/decision.proto";

// --- Enums ---

enum ExecutionEventType {
  EXECUTION_EVENT_TYPE_UNSPECIFIED = 0;
  EXECUTION_EVENT_TYPE_PROGRESS = 1;
  EXECUTION_EVENT_TYPE_BLOCKER = 2;
  EXECUTION_EVENT_TYPE_COMPLETION = 3;
}

// --- Messages ---

message CallbackConfig {
  string endpoint = 1;
  string prime = 2;
  string progress = 3;
  string blocker = 4;
  string completion = 5;
}

message Bundle {
  int32 version = 1;
  Spec spec = 2;
  repeated Decision decisions = 3;
  string bootstrap = 4;
  CallbackConfig callbacks = 5;
  string bundle_yaml = 6;
}

message PrimeResponse {
  string constitution_summary = 1;
  string project_context = 2;
  repeated Decision decisions = 3;
  string coding_conventions = 4;
  string callback_docs = 5;
}

message ExecutionEvent {
  string id = 1;
  string spec_slug = 2;
  string agent = 3;
  ExecutionEventType type = 4;
  string message = 5;
  google.protobuf.Timestamp created_at = 6;
}

// --- Requests/Responses ---

message GenerateBundleRequest {
  string slug = 1;
  string endpoint = 2;
}

message GetPrimeRequest {
  string slug = 1;
}

message ReportProgressRequest {
  string slug = 1;
  string agent = 2;
  string message = 3;
}

message ReportProgressResponse {
  bool acknowledged = 1;
}

message ReportBlockerRequest {
  string slug = 1;
  string agent = 2;
  string description = 3;
}

message ReportBlockerResponse {
  bool acknowledged = 1;
}

message ReportCompletionRequest {
  string slug = 1;
  string agent = 2;
}

message ReportCompletionResponse {
  bool acknowledged = 1;
  string new_stage = 2;
}

message GetExecutionEventsRequest {
  string slug = 1;
  int32 limit = 2;
}

message GetExecutionEventsResponse {
  repeated ExecutionEvent events = 1;
}

// --- Service ---

service ExecutionService {
  rpc GenerateBundle(GenerateBundleRequest) returns (Bundle);
  rpc GetPrime(GetPrimeRequest) returns (PrimeResponse);
  rpc ReportProgress(ReportProgressRequest) returns (ReportProgressResponse);
  rpc ReportBlocker(ReportBlockerRequest) returns (ReportBlockerResponse);
  rpc ReportCompletion(ReportCompletionRequest) returns (ReportCompletionResponse);
  rpc GetExecutionEvents(GetExecutionEventsRequest) returns (GetExecutionEventsResponse);
}
```

**Step 2: Generate Go code**

```bash
task proto
```

**Step 3: Verify generated code compiles**

```bash
go build ./gen/...
```

**Step 4: Commit**

```bash
git add proto/specgraph/v1/execution.proto
git commit -m "feat(execution): protobuf schema for ExecutionService"
```

---

### Task 14: Execution Domain Types and Storage Interface

**Files:**

- Create: `internal/storage/execution_domain.go`
- Create: `internal/storage/execution.go`

**Step 1: Create domain types**

`internal/storage/execution_domain.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// ExecutionEventType classifies execution callback events.
type ExecutionEventType int

const (
	ExecutionEventTypeProgress   ExecutionEventType = iota + 1
	ExecutionEventTypeBlocker
	ExecutionEventTypeCompletion
)

// String returns the string representation of the event type.
func (t ExecutionEventType) String() string {
	switch t {
	case ExecutionEventTypeProgress:
		return "progress"
	case ExecutionEventTypeBlocker:
		return "blocker"
	case ExecutionEventTypeCompletion:
		return "completion"
	default:
		return "unknown"
	}
}

// ParseExecutionEventType converts a string to an ExecutionEventType.
func ParseExecutionEventType(s string) ExecutionEventType {
	switch s {
	case "progress":
		return ExecutionEventTypeProgress
	case "blocker":
		return ExecutionEventTypeBlocker
	case "completion":
		return ExecutionEventTypeCompletion
	default:
		return 0
	}
}

// ExecutionEvent records a progress, blocker, or completion event from an agent.
type ExecutionEvent struct {
	ID        string
	SpecSlug  string
	Agent     string
	Type      ExecutionEventType
	Message   string
	CreatedAt time.Time
}

// CallbackConfig holds the endpoint paths for agent callbacks.
type CallbackConfig struct {
	Endpoint   string
	Prime      string
	Progress   string
	Blocker    string
	Completion string
}

// Bundle is a self-contained package for an executing agent.
type Bundle struct {
	Version   int32
	Spec      *Spec
	Decisions []*Decision
	Bootstrap string
	Callbacks *CallbackConfig
}

// PrimeData holds the raw data needed to compose a prime response.
type PrimeData struct {
	Spec         *Spec
	Decisions    []*Decision
	Constitution *Constitution
}
```

**Step 2: Create the interface**

`internal/storage/execution.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
)

// ErrSpecNotApproved is returned when a bundle is requested for a spec not in an executable stage.
var ErrSpecNotApproved = errors.New("spec is not in an approved or in_progress stage")

// ErrAgentNotClaimOwner is returned when an agent reports an event but does not hold the claim.
var ErrAgentNotClaimOwner = errors.New("agent does not hold the claim for this spec")

// ExecutionBackend defines storage operations for execution bundles and agent callbacks.
type ExecutionBackend interface {
	// GenerateBundle assembles a bundle from the spec, its decisions, and the constitution.
	GenerateBundle(ctx context.Context, slug string) (*Bundle, error)

	// RecordProgress stores a progress event from an executing agent.
	RecordProgress(ctx context.Context, slug, agent, message string) error

	// RecordBlocker stores a blocker event from an executing agent.
	RecordBlocker(ctx context.Context, slug, agent, description string) error

	// RecordCompletion stores a completion event and transitions spec to done.
	RecordCompletion(ctx context.Context, slug, agent string) error

	// GetExecutionEvents returns execution events for a spec, ordered by time descending.
	GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*ExecutionEvent, error)

	// GetPrimeData returns the data needed to compose a prime response.
	GetPrimeData(ctx context.Context, slug string) (*PrimeData, error)

	// ReleaseExpiredClaims finds and releases all CLAIMED_BY relationships past their lease.
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}
```

**Step 3: Verify it compiles**

```bash
go build ./internal/storage/
```

**Step 4: Commit**

```bash
git add internal/storage/execution_domain.go internal/storage/execution.go
git commit -m "feat(execution): domain types and storage interface"
```

---

### Task 15: Memgraph Execution Backend

**Files:**

- Create: `internal/storage/memgraph/execution.go`
- Create: `internal/storage/memgraph/execution_test.go`

**Step 1: Write the integration tests**

Tests verify: GenerateBundle (not found, not approved, approved with decisions), callbacks (no claim, with claim, progress→blocker→completion→done transition), GetPrimeData, ReleaseExpiredClaims.

All test assertions use domain types (`*storage.Bundle`, `*storage.ExecutionEvent`, `*storage.Claim`).

**Step 2: Implement the Memgraph execution backend**

Key methods:

- `GenerateBundle`: GetSpec (domain), check executable stages, getSpecDecisions (domain), compose Bundle
- `getSpecDecisions`: MATCH spec→DECIDED_IN→decision, use recordToDecision (returns domain)
- `verifyClaimOwner`: MATCH spec CLAIMED_BY agent with valid lease
- `recordExecutionEvent`: CREATE ExecutionEvent node with HAS_EVENT edge
- `RecordProgress/RecordBlocker`: verify claim, record event
- `RecordCompletion`: verify claim, record event, transition spec to done, release claim
- `GetExecutionEvents`: MATCH spec→HAS_EVENT→events, use recordToExecutionEvent (returns domain)
- `GetPrimeData`: GetSpec + getSpecDecisions + GetConstitution (all domain)
- `ReleaseExpiredClaims`: MATCH expired CLAIMED_BY, DELETE, return count

`recordToExecutionEvent` creates `*storage.ExecutionEvent` with `ParseExecutionEventType(typeStr)`.

**Step 3: Run integration tests**

```bash
go test ./internal/storage/memgraph/ -run TestExecution -v -count=1 -timeout=120s
```

Expected: PASS

**Step 4: Commit**

```bash
git add internal/storage/memgraph/execution.go internal/storage/memgraph/execution_test.go
git commit -m "feat(execution): memgraph storage backend with integration tests"
```

---

### Task 16: Execution Converters in convert.go

**Files:**

- Modify: `internal/server/convert.go`

**Step 1: Add execution converters**

```go
// --- Execution ---

var executionEventTypeToProtoMap = map[storage.ExecutionEventType]specv1.ExecutionEventType{
	storage.ExecutionEventTypeProgress:   specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS,
	storage.ExecutionEventTypeBlocker:    specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER,
	storage.ExecutionEventTypeCompletion: specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION,
}

func executionEventToProto(e *storage.ExecutionEvent) *specv1.ExecutionEvent {
	return &specv1.ExecutionEvent{
		Id:        e.ID,
		SpecSlug:  e.SpecSlug,
		Agent:     e.Agent,
		Type:      executionEventTypeToProtoMap[e.Type],
		Message:   e.Message,
		CreatedAt: timeToProto(e.CreatedAt),
	}
}

func executionEventsToProto(events []*storage.ExecutionEvent) []*specv1.ExecutionEvent {
	result := make([]*specv1.ExecutionEvent, len(events))
	for i, e := range events {
		result[i] = executionEventToProto(e)
	}
	return result
}

func bundleToProto(b *storage.Bundle) (*specv1.Bundle, error) {
	spec := specToProto(b.Spec)
	decisions, err := decisionsToProto(b.Decisions)
	if err != nil {
		return nil, err
	}

	var callbacks *specv1.CallbackConfig
	if b.Callbacks != nil {
		callbacks = &specv1.CallbackConfig{
			Endpoint:   b.Callbacks.Endpoint,
			Prime:      b.Callbacks.Prime,
			Progress:   b.Callbacks.Progress,
			Blocker:    b.Callbacks.Blocker,
			Completion: b.Callbacks.Completion,
		}
	}

	return &specv1.Bundle{
		Version:   b.Version,
		Spec:      spec,
		Decisions: decisions,
		Bootstrap: b.Bootstrap,
		Callbacks: callbacks,
	}, nil
}
```

**Step 2: Verify it compiles**

```bash
go build ./internal/server/
```

**Step 3: Commit**

```bash
git add internal/server/convert.go
git commit -m "feat(server): add execution domain-to-proto converters"
```

---

### Task 17: ConnectRPC ExecutionService Handler

**Files:**

- Create: `internal/server/execution_handler.go`
- Create: `internal/server/execution_handler_test.go`

**Step 1: Write handler tests with mock backend**

Mock backend implements `storage.ExecutionBackend` returning domain types.
Tests verify: GenerateBundle (success, not found), GetPrime, ReportProgress (success, no claim), ReportCompletion, bundle YAML rendering.

**Step 2: Implement the handler**

Key patterns:

- `GenerateBundle`: calls store, converts `*storage.Bundle` via `bundleToProto()`, renders YAML, sets endpoint
- `GetPrime`: calls store.GetPrimeData, composes `PrimeResponse` using constitution/decisions from domain types
- `ReportProgress/Blocker`: validates fields, calls store, maps ErrAgentNotClaimOwner → CodePermissionDenied
- `ReportCompletion`: calls store, returns `NewStage: "done"`
- `GetExecutionEvents`: calls store, converts via `executionEventsToProto()`
- `renderBundleYAML`: string builder producing lean YAML
- `composeConstitutionSummary`/`composeCodingConventions`/`composeCallbackDocs`: accept domain types

**Step 3: Run tests**

```bash
go test ./internal/server/ -run TestExecution -v -count=1
```

Expected: PASS

**Step 4: Wire into serve.go**

Add after existing service registrations in `cmd/specgraph/serve.go`:

```go
server.RegisterExecutionService(mux, store)
```

**Step 5: Commit**

```bash
git add internal/server/execution_handler.go internal/server/execution_handler_test.go cmd/specgraph/serve.go
git commit -m "feat(execution): ConnectRPC handler with domain type conversion"
```

---

### Task 18: Background Lease Sweeper

**Files:**

- Create: `internal/server/sweeper.go`
- Create: `internal/server/sweeper_test.go`

**Step 1: Write sweeper test**

Mock implements `server.ClaimSweeper` (single-method interface: `ReleaseExpiredClaims`).
Tests verify: runs on interval, stops on context cancel.

**Step 2: Implement the sweeper**

```go
type ClaimSweeper interface {
	ReleaseExpiredClaims(ctx context.Context) (int, error)
}

func StartSweeper(ctx context.Context, store ClaimSweeper, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				released, err := store.ReleaseExpiredClaims(ctx)
				// log if err or released > 0
			}
		}
	}()
}
```

**Step 3: Wire into serve.go**

Add before `ListenAndServe`:

```go
server.StartSweeper(ctx, store, 60*time.Second)
```

**Step 4: Run tests**

```bash
go test ./internal/server/ -run TestSweeper -v -count=1
```

Expected: PASS

**Step 5: Commit**

```bash
git add internal/server/sweeper.go internal/server/sweeper_test.go cmd/specgraph/serve.go
git commit -m "feat(execution): background lease sweeper for expired claims"
```

---

### Task 19: CLI — bundle and progress commands

**Files:**

- Create: `cmd/specgraph/bundle.go`
- Create: `cmd/specgraph/progress.go`

**Step 1: Implement bundle command**

`bundle <slug>` with `--output` and `--endpoint` flags. Calls `ExecutionServiceClient.GenerateBundle`, outputs YAML to stdout or file.

**Step 2: Implement progress command**

`progress <slug>` with `--limit` flag. Calls `ExecutionServiceClient.GetExecutionEvents`, formats output.

**Step 3: Verify build**

```bash
go build -o specgraph ./cmd/specgraph
./specgraph bundle --help
./specgraph progress --help
```

**Step 4: Commit**

```bash
git add cmd/specgraph/bundle.go cmd/specgraph/progress.go
git commit -m "feat(execution): CLI bundle and progress commands"
```

---

### Task 20: Final Verification and Cleanup

**Step 1: Verify no proto imports in storage layer**

```bash
grep -r 'specv1\|specgraphv1\|timestamppb' internal/storage/ --include="*.go"
```

Expected: NO matches

**Step 2: Run full check suite**

```bash
task check
```

Expected: PASS (fmt, lint, build, unit tests)

**Step 3: Run buf lint**

```bash
buf lint
```

Expected: No issues

**Step 4: Verify CLI**

```bash
go build -o specgraph ./cmd/specgraph
./specgraph --help
./specgraph bundle --help
./specgraph progress --help
```

Expected: bundle and progress appear in command list

**Step 5: Update implementation tracker**

Update `docs/plans/2026-02-28-implementation-tracker.md` Slice 4 section to mark tasks complete.

**Step 6: Commit**

```bash
git add -A
git commit -m "chore(execution): final verification and cleanup"
```

---

## Summary

| Task | Phase | What | Files | Test Type |
|------|-------|------|-------|-----------|
| 1 | A | Constitution domain types | `storage/constitution_domain.go` | Compile |
| 2 | A | Claim domain type | `storage/claim_domain.go` | Compile |
| 3 | A | Update interfaces | `storage/constitution.go`, `claim.go` | Compile |
| 4 | A | Conversion functions | `server/convert.go` | Compile |
| 5 | A | Memgraph constitution | `memgraph/constitution.go` | Integration |
| 6 | A | Memgraph claim | `memgraph/claim.go` | Integration |
| 7 | A | Memgraph tests | `memgraph/*_test.go` | Integration |
| 8 | A | Emitter domain types | `emitter/emitter.go` | Unit |
| 9 | A | Handler conversion | `server/*_handler.go` | Compile |
| 10 | A | Handler tests | `server/*_handler_test.go` | Unit |
| 11 | A | Config + serve | `config/config.go`, `serve.go` | Unit |
| 12 | A | Phase A verification | -- | All |
| 13 | B | Proto schema | `execution.proto` | Compile |
| 14 | B | Domain types + interface | `storage/execution*.go` | Compile |
| 15 | B | Memgraph execution | `memgraph/execution.go` | Integration |
| 16 | B | Execution converters | `server/convert.go` | Compile |
| 17 | B | Handler | `server/execution_handler.go` | Unit (mock) |
| 18 | B | Lease sweeper | `server/sweeper.go` | Unit |
| 19 | B | CLI commands | `cmd/specgraph/bundle.go`, `progress.go` | Build |
| 20 | B | Final verification | -- | All |
