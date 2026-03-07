# Storage Domain Types Consistency & Slice 4 Execution

> **For Claude:** This design covers two phases: (A) refactoring ConstitutionBackend and ClaimBackend to use domain types instead of proto types, and (B) implementing Slice 4 ExecutionBackend with domain types from the start.

**Goal:** Eliminate all proto type usage from the storage layer. Every storage interface returns domain types defined in `internal/storage/`. Proto conversion happens exclusively in `internal/server/convert.go` at the handler boundary.

**Motivation:** PR #24 refactored Spec and Decision to domain types but grandfathered Constitution and Claim. New code (Slice 4) must follow the domain type pattern, and the grandfathered code must be brought into alignment — no exceptions.

---

## Current State

| Interface | Returns | Status |
|-----------|---------|--------|
| `Backend` (Spec CRUD) | `*storage.Spec` | Clean |
| `DecisionBackend` | `*storage.Decision` | Clean |
| `GraphBackend` | `*storage.Edge`, `[]NodeRef` | Clean |
| `AuthoringBackend` | Domain types (SparkOutput, etc.) | Clean |
| **`ConstitutionBackend`** | **`*specv1.Constitution`**, **`[]*specv1.Violation`** | **Proto leak** |
| **`ClaimBackend`** | **`*specv1.Claim`** | **Proto leak** |

---

## Phase A: Refactor Constitution & Claim

### Constitution Domain Types

File: `internal/storage/constitution_domain.go`

```go
type Constitution struct {
    Layer        string
    Name         string
    Version      int32
    Tech         *TechStack
    Principles   []Principle
    Process      *ConstitutionProcess
    Constraints  []string
    Antipatterns []Antipattern
    References   []Reference
    UpdatedAt    time.Time
}

type TechStack struct {
    Languages      *Languages
    Frameworks     map[string]string
    Infrastructure []string
    APIStandards   []string
    Data           []string
}

type Languages struct {
    Primary   string
    Allowed   []string
    Forbidden []string
}

type Principle struct {
    ID         string
    Principle  string
    Rationale  string
    Exceptions []string
}

type Antipattern struct {
    Pattern string
    Why     string
    Instead string
}

type Reference struct {
    Type string
    Path string
}

type ConstitutionProcess struct {
    SpecReview     string
    SecurityReview string
    Deployment     string
    Documentation  string
}

type Violation struct {
    Constraint string
    Message    string
    Severity   string // "error" | "warning"
}
```

### Updated ConstitutionBackend Interface

```go
type ConstitutionBackend interface {
    GetConstitution(ctx context.Context) (*Constitution, error)
    UpdateConstitution(ctx context.Context, constitution *Constitution) (*Constitution, error)
    CheckViolation(ctx context.Context, specSlug string) ([]Violation, error)
}
```

### Claim Domain Type

File: `internal/storage/claim_domain.go`

```go
type Claim struct {
    Slug         string
    Agent        string
    LeaseExpires time.Time
    ClaimedAt    time.Time
}
```

### Updated ClaimBackend Interface

```go
type ClaimBackend interface {
    ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*Claim, error)
    UnclaimSpec(ctx context.Context, slug, agent string) error
    Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*Claim, error)
}
```

### Conversion Functions (convert.go)

New functions needed:

- `constitutionToProto(*storage.Constitution) *specv1.Constitution`
- `constitutionFromProto(*specv1.Constitution) *storage.Constitution` (for UpdateConstitution request parsing)
- `violationsToProto([]storage.Violation) []*specv1.Violation`
- `claimToProto(*storage.Claim) *specv1.Claim`

### Impact

- `internal/storage/constitution.go` — remove `specv1` import, use domain types
- `internal/storage/claim.go` — remove `specv1` import, use domain types
- `internal/storage/memgraph/constitution.go` — return domain types, remove `timestamppb`
- `internal/storage/memgraph/claim.go` — return domain types, remove `timestamppb`
- `internal/server/constitution_handler.go` — add conversion calls
- `internal/server/claim_handler.go` — add conversion calls
- `internal/server/convert.go` — add new conversion functions
- `internal/server/authoring_handler.go` — if it accesses constitution/claim, update types
- Tests in `memgraph/` and `server/` — update expectations to domain types

---

## Phase B: Slice 4 Execution Domain Types

File: `internal/storage/execution_domain.go`

```go
type ExecutionEventType int

const (
    ExecutionEventTypeProgress   ExecutionEventType = iota + 1
    ExecutionEventTypeBlocker
    ExecutionEventTypeCompletion
)

type ExecutionEvent struct {
    ID        string
    SpecSlug  string
    Agent     string
    Type      ExecutionEventType
    Message   string
    CreatedAt time.Time
}

type CallbackConfig struct {
    Endpoint   string
    Prime      string
    Progress   string
    Blocker    string
    Completion string
}

type Bundle struct {
    Version    int32
    Spec       *Spec
    Decisions  []*Decision
    Bootstrap  string
    Callbacks  *CallbackConfig
}

type PrimeData struct {
    Spec         *Spec
    Decisions    []*Decision
    Constitution *Constitution
}
```

### ExecutionBackend Interface

File: `internal/storage/execution.go`

```go
var ErrSpecNotApproved = errors.New("spec is not in an approved or in_progress stage")
var ErrAgentNotClaimOwner = errors.New("agent does not hold the claim for this spec")

type ExecutionBackend interface {
    GenerateBundle(ctx context.Context, slug string) (*Bundle, error)
    RecordProgress(ctx context.Context, slug, agent, message string) error
    RecordBlocker(ctx context.Context, slug, agent, description string) error
    RecordCompletion(ctx context.Context, slug, agent string) error
    GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*ExecutionEvent, error)
    GetPrimeData(ctx context.Context, slug string) (*PrimeData, error)
    ReleaseExpiredClaims(ctx context.Context) (int, error)
}
```

### Conversion Functions (convert.go)

New functions needed:

- `bundleToProto(*storage.Bundle) *specv1.Bundle`
- `executionEventToProto(*storage.ExecutionEvent) *specv1.ExecutionEvent`
- `executionEventsToProto([]*storage.ExecutionEvent) []*specv1.ExecutionEvent`
- `primeDataToProto(*storage.PrimeData) *specv1.PrimeResponse` (composes the full response)

---

## Architecture Invariant

After both phases, the dependency graph is strictly:

```text
proto/specgraph/v1/*.proto
        |
        v (buf generate)
gen/specgraph/v1/*.pb.go
        |
        v (import)
internal/server/ ──────> internal/storage/ (domain types only)
   (convert.go)              (no proto imports)
        |
        v
internal/storage/memgraph/ (no proto imports)
```

No file in `internal/storage/` or `internal/storage/memgraph/` imports from `gen/`.

---

## File Changes Summary

### Phase A (refactor)

| File | Action |
|------|--------|
| `internal/storage/constitution_domain.go` | Create (domain types) |
| `internal/storage/claim_domain.go` | Create (domain type) |
| `internal/storage/constitution.go` | Update interface (remove specv1) |
| `internal/storage/claim.go` | Update interface (remove specv1) |
| `internal/storage/memgraph/constitution.go` | Return domain types |
| `internal/storage/memgraph/claim.go` | Return domain types |
| `internal/server/convert.go` | Add constitution/claim converters |
| `internal/server/constitution_handler.go` | Convert at boundary |
| `internal/server/claim_handler.go` | Convert at boundary |
| Tests | Update type expectations |

### Phase B (Slice 4)

| File | Action |
|------|--------|
| `proto/specgraph/v1/execution.proto` | Create |
| `internal/storage/execution_domain.go` | Create (domain types) |
| `internal/storage/execution.go` | Create (interface) |
| `internal/storage/memgraph/execution.go` | Create (implementation) |
| `internal/storage/memgraph/execution_test.go` | Create (integration tests) |
| `internal/server/execution_handler.go` | Create (handler) |
| `internal/server/execution_handler_test.go` | Create (handler tests) |
| `internal/server/sweeper.go` | Create |
| `internal/server/sweeper_test.go` | Create |
| `internal/server/convert.go` | Add execution converters |
| `cmd/specgraph/bundle.go` | Create (CLI) |
| `cmd/specgraph/progress.go` | Create (CLI) |
| `cmd/specgraph/serve.go` | Wire service + sweeper |
