# Decision ADR-003 Fields Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the Decision domain type with ADR-003 fields (question, rejected alternatives, confidence, tags, scope, origin_spec, origin_stage) and generalize ChangeLog to support Decision nodes.

**Architecture:** Bottom-up: domain types → content hash → proto → storage → handler/converter → CLI → render. Each task produces a compilable, testable increment. ChangeLog generalization is a separate task that can be done after storage fields land.

**Tech Stack:** Go, protobuf/ConnectRPC, Memgraph (Cypher), Murmur3 content hashing

**Spec:** `docs/superpowers/specs/2026-03-31-decision-adr003-fields-design.md`

---

## Chunk 1: Domain Types, Content Hash, Proto

### Task 1: Add domain enums, RejectedAlternative, and extend Decision struct

**Files:**

- Modify: `internal/storage/decision.go`
- Test: `internal/storage/decision_test.go` (create if absent)

- [ ] **Step 1: Write failing tests for new enum validation**

In `internal/storage/decision_test.go`:

```go
func TestDecisionConfidence_IsValid(t *testing.T) {
    assert.True(t, storage.DecisionConfidenceHigh.IsValid())
    assert.True(t, storage.DecisionConfidenceMedium.IsValid())
    assert.True(t, storage.DecisionConfidenceLow.IsValid())
    assert.False(t, storage.DecisionConfidence("bogus").IsValid())
    assert.False(t, storage.DecisionConfidence("").IsValid())
}

func TestDecisionScope_IsValid(t *testing.T) {
    assert.True(t, storage.DecisionScopeProject.IsValid())
    assert.True(t, storage.DecisionScopeTeam.IsValid())
    assert.True(t, storage.DecisionScopeOrg.IsValid())
    assert.False(t, storage.DecisionScope("bogus").IsValid())
    assert.False(t, storage.DecisionScope("").IsValid())
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/ -run TestDecisionConfidence -v`
Expected: FAIL — types not defined yet

- [ ] **Step 3: Implement new types and extend Decision struct**

In `internal/storage/decision.go`, add after `DecisionStatus` types:

```go
// DecisionConfidence represents the confidence level in a decision.
type DecisionConfidence string

// Decision confidence values.
const (
    DecisionConfidenceHigh   DecisionConfidence = "high"
    DecisionConfidenceMedium DecisionConfidence = "medium"
    DecisionConfidenceLow    DecisionConfidence = "low"
)

// IsValid reports whether c is a known decision confidence level.
func (c DecisionConfidence) IsValid() bool {
    switch c {
    case DecisionConfidenceHigh, DecisionConfidenceMedium, DecisionConfidenceLow:
        return true
    default:
        return false
    }
}

// DecisionScope represents how broadly a decision applies.
type DecisionScope string

// Decision scope values.
const (
    DecisionScopeProject DecisionScope = "project"
    DecisionScopeTeam    DecisionScope = "team"
    DecisionScopeOrg     DecisionScope = "org"
)

// IsValid reports whether s is a known decision scope.
func (s DecisionScope) IsValid() bool {
    switch s {
    case DecisionScopeProject, DecisionScopeTeam, DecisionScopeOrg:
        return true
    default:
        return false
    }
}

// RejectedAlternative records an option that was considered but not chosen.
type RejectedAlternative struct {
    Option string
    Reason string
}
```

Add new fields to the `Decision` struct (after `SupersededBy`, before `CreatedAt`):

```go
Question              string
RejectedAlternatives  []RejectedAlternative
Confidence            DecisionConfidence
Tags                  []string
Scope                 DecisionScope
OriginSpec            string
OriginStage           string
Version               int
```

Update `DecisionBackend` interface signatures:

```go
CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
    rejectedAlts []RejectedAlternative, confidence DecisionConfidence,
    tags []string, scope DecisionScope, originSpec, originStage string) (*Decision, error)

UpdateDecision(ctx context.Context, slug string, title *string, status *DecisionStatus,
    body, rationale, supersededBy, question *string,
    rejectedAlts *[]RejectedAlternative, confidence *DecisionConfidence,
    tags *[]string, scope *DecisionScope, originSpec, originStage *string) (*Decision, error)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/ -run "TestDecisionConfidence|TestDecisionScope" -v`
Expected: PASS

- [ ] **Step 5: Commit**

Message: `feat(storage): add Decision domain types for ADR-003 fields (spgr-bk8)`

---

### Task 2: Extend content hash for Decision

**Files:**

- Modify: `internal/storage/contenthash/contenthash.go`
- Modify: `internal/storage/contenthash/contenthash_test.go`

- [ ] **Step 1: Write failing test for expanded Decision hash**

In `internal/storage/contenthash/contenthash_test.go`, add:

```go
func TestDecision_NewFieldsChangeHash(t *testing.T) {
    base := contenthash.Decision("title", "proposed", "body", "rationale",
        "question", "high", "project", nil, nil)
    require.Len(t, base, 32)

    withQuestion := contenthash.Decision("title", "proposed", "body", "rationale",
        "different question", "high", "project", nil, nil)
    require.NotEqual(t, base, withQuestion, "question should affect hash")

    withTags := contenthash.Decision("title", "proposed", "body", "rationale",
        "question", "high", "project", []string{"auth", "storage"}, nil)
    require.NotEqual(t, base, withTags, "tags should affect hash")
}

func TestDecision_TagsSortedForDeterminism(t *testing.T) {
    h1 := contenthash.Decision("t", "s", "b", "r", "q", "h", "p",
        []string{"z", "a", "m"}, nil)
    h2 := contenthash.Decision("t", "s", "b", "r", "q", "h", "p",
        []string{"a", "m", "z"}, nil)
    require.Equal(t, h1, h2, "tag order should not affect hash")
}

func TestDecision_RejectedAltsSortedForDeterminism(t *testing.T) {
    alts1 := []storage.RejectedAlternative{
        {Option: "Redis", Reason: "ops"},
        {Option: "DynamoDB", Reason: "cost"},
    }
    alts2 := []storage.RejectedAlternative{
        {Option: "DynamoDB", Reason: "cost"},
        {Option: "Redis", Reason: "ops"},
    }
    h1 := contenthash.Decision("t", "s", "b", "r", "q", "h", "p", nil, alts1)
    h2 := contenthash.Decision("t", "s", "b", "r", "q", "h", "p", nil, alts2)
    require.Equal(t, h1, h2, "rejected alt order should not affect hash")
}
```

Note: `contenthash.Decision` now needs `storage.RejectedAlternative` as a parameter type. Import `internal/storage` in the contenthash package. To avoid a circular dependency, pass `[]RejectedAlternative` where `RejectedAlternative` is defined in the contenthash package itself (mirroring the storage type), or pass pre-formatted strings. **Preferred approach:** define a local `RejectedAlt` struct in contenthash to avoid importing storage:

```go
// RejectedAlt is a contenthash-local type to avoid importing storage.
type RejectedAlt struct {
    Option string
    Reason string
}
```

Update tests to use `contenthash.RejectedAlt` instead of `storage.RejectedAlternative`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/storage/contenthash/ -run "TestDecision_NewFields|TestDecision_Tags|TestDecision_Rejected" -v`
Expected: FAIL — signature mismatch

- [ ] **Step 3: Implement expanded Decision hash**

In `internal/storage/contenthash/contenthash.go`:

```go
// RejectedAlt holds an option and reason for content hashing.
type RejectedAlt struct {
    Option string
    Reason string
}

// Decision computes a Murmur3-128 content hash for a decision's substantive fields.
func Decision(title, status, decision, rationale, question, confidence, scope string,
    tags []string, rejectedAlts []RejectedAlt) string {
    h := murmur3.New128()
    writeField(h, "title", title)
    writeField(h, "status", status)
    writeField(h, "decision", decision)
    writeField(h, "rationale", rationale)
    writeField(h, "question", question)
    writeField(h, "confidence", confidence)
    writeField(h, "scope", scope)

    // Sort tags for determinism.
    sorted := make([]string, len(tags))
    copy(sorted, tags)
    sort.Strings(sorted)
    for _, tag := range sorted {
        writeField(h, "tag", tag)
    }

    // Sort rejected alternatives by Option for determinism.
    sortedAlts := make([]RejectedAlt, len(rejectedAlts))
    copy(sortedAlts, rejectedAlts)
    sort.Slice(sortedAlts, func(i, j int) bool {
        return sortedAlts[i].Option < sortedAlts[j].Option
    })
    for _, alt := range sortedAlts {
        writeField(h, "rejected_option", alt.Option)
        writeField(h, "rejected_reason", alt.Reason)
    }

    hi, lo := h.Sum128()
    return fmt.Sprintf("%016x%016x", hi, lo)
}
```

- [ ] **Step 4: Fix all callers of `contenthash.Decision`**

Two callers in `internal/storage/memgraph/decision.go`:
- `CreateDecision` (line 23): pass new fields
- `UpdateDecision` (line 194): pass new fields

These will compile-error until Task 4 (storage layer) updates the callers. For now, pass zero values to make it compile:

```go
ch := contenthash.Decision(title, initialStatus, body, rationale, "", "", "",
    nil, nil)
```

Update both call sites to pass zero values temporarily.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/storage/contenthash/ -v`
Expected: PASS (new tests pass, existing `TestDecision` test needs updated call signature too)

- [ ] **Step 6: Commit**

Message: `feat(contenthash): expand Decision hash with ADR-003 fields (spgr-bk8)`

---

### Task 3: Update proto schema

**Files:**

- Modify: `proto/specgraph/v1/decision.proto`

- [ ] **Step 1: Add new enums, messages, and fields to decision.proto**

Add after `DecisionStatus` enum:

```protobuf
enum DecisionConfidence {
  DECISION_CONFIDENCE_UNSPECIFIED = 0;
  DECISION_CONFIDENCE_HIGH = 1;
  DECISION_CONFIDENCE_MEDIUM = 2;
  DECISION_CONFIDENCE_LOW = 3;
}

enum DecisionScope {
  DECISION_SCOPE_UNSPECIFIED = 0;
  DECISION_SCOPE_PROJECT = 1;
  DECISION_SCOPE_TEAM = 2;
  DECISION_SCOPE_ORG = 3;
}

message RejectedAlternative {
  string option = 1;
  string reason = 2;
}
```

Add to `Decision` message after field 10 (`content_hash`):

```protobuf
string question = 11;
repeated RejectedAlternative rejected_alternatives = 12;
DecisionConfidence confidence = 13;
repeated string tags = 14;
DecisionScope scope = 15;
string origin_spec = 16;
string origin_stage = 17;
int32 version = 18;
```

Add to `CreateDecisionRequest` (after field 4):

```protobuf
string question = 5;
repeated RejectedAlternative rejected_alternatives = 6;
DecisionConfidence confidence = 7;
repeated string tags = 8;
DecisionScope scope = 9;
string origin_spec = 10;
string origin_stage = 11;
```

Add to `UpdateDecisionRequest` (after field 6):

```protobuf
optional string question = 7;
optional DecisionConfidence confidence = 8;
repeated string tags = 9;
optional DecisionScope scope = 10;
optional string origin_spec = 11;
optional string origin_stage = 12;
repeated RejectedAlternative rejected_alternatives = 13;
```

Note: `repeated` fields in proto3 cannot be `optional` — presence is determined by whether the list is empty. For `UpdateDecision`, an empty `tags` list vs absent list is indistinguishable in proto3. The handler will treat empty repeated fields as "no change" (same as existing behavior for non-repeated optional fields).

- [ ] **Step 2: Regenerate Go code**

Run: `task proto`

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Compiles (handler code hasn't changed yet)

- [ ] **Step 4: Commit**

Message: `feat(proto): add Decision ADR-003 fields to proto schema (spgr-bk8)`

---

## Chunk 2: Storage Layer (Memgraph)

### Task 4: Update Memgraph CreateDecision with new fields

**Files:**

- Modify: `internal/storage/memgraph/decision.go`

- [ ] **Step 1: Update CreateDecision to accept and store new fields**

Update `CreateDecision` signature to match the new `DecisionBackend` interface. Add new properties to the CREATE Cypher query and params map.

New Cypher properties in the CREATE clause:
```
question: $question,
rejected_alternatives_json: $rejected_alternatives_json,
confidence: $confidence,
tags_json: $tags_json,
scope: $scope,
origin_spec: $origin_spec,
origin_stage: $origin_stage,
version: $version
```

New params:
```go
"question":                     question,
"rejected_alternatives_json":   marshalRejectedAlts(rejectedAlts),
"confidence":                   string(confidence),
"tags_json":                    marshalTags(tags),
"scope":                        string(scope),
"origin_spec":                  originSpec,
"origin_stage":                 originStage,
"version":                      int64(1),
```

Add helper functions (in decision.go or a shared helper):
```go
func marshalRejectedAlts(alts []storage.RejectedAlternative) string
func unmarshalRejectedAlts(raw string) ([]storage.RejectedAlternative, error)
func marshalTags(tags []string) string
func unmarshalTags(raw string) ([]string, error)
```

JSON format: `[{"option":"X","reason":"Y"}]` for rejected alts, `["a","b"]` for tags.

Update the RETURN clause and `recordToDecision` to include all new fields at positions 10-17.

Update content hash call to pass actual new field values.

- [ ] **Step 2: Update recordToDecision to parse new fields**

After existing field parsing (position 9 = content_hash), add parsing for positions 10-17 using `recordStringOptional` for backward compatibility with existing nodes:

```go
question, err := recordStringOptional(rec, 10, "question")
rejectedAltsJSON, err := recordStringOptional(rec, 11, "rejected_alternatives_json")
confidence, err := recordStringOptional(rec, 12, "confidence")
tagsJSON, err := recordStringOptional(rec, 13, "tags_json")
scope, err := recordStringOptional(rec, 14, "scope")
originSpec, err := recordStringOptional(rec, 15, "origin_spec")
originStage, err := recordStringOptional(rec, 16, "origin_stage")
version, err := recordInt64Optional(rec, 17, "version") // need to add this helper
```

Populate the new fields on the returned `Decision` struct. Unmarshal JSON fields.

- [ ] **Step 3: Update GetDecision and ListDecisions RETURN clauses**

All three query methods must return the new properties. Add `d.question, d.rejected_alternatives_json, d.confidence, d.tags_json, d.scope, d.origin_spec, d.origin_stage, d.version` to each RETURN clause.

- [ ] **Step 4: Verify build compiles**

Run: `go build ./internal/storage/memgraph/`

- [ ] **Step 5: Commit**

Message: `feat(memgraph): store ADR-003 fields on Decision nodes (spgr-bk8)`

---

### Task 5: Update Memgraph UpdateDecision with new fields and version bump

**Files:**

- Modify: `internal/storage/memgraph/decision.go`

- [ ] **Step 1: Update UpdateDecision signature and SET clauses**

Update signature to match interface. Add conditional SET clauses for each new field:

```go
if question != nil {
    setClauses = append(setClauses, "d.question = $question")
    params["question"] = *question
}
if rejectedAlts != nil {
    setClauses = append(setClauses, "d.rejected_alternatives_json = $rejected_alternatives_json")
    params["rejected_alternatives_json"] = marshalRejectedAlts(*rejectedAlts)
}
if confidence != nil {
    setClauses = append(setClauses, "d.confidence = $confidence")
    params["confidence"] = string(*confidence)
}
if tags != nil {
    setClauses = append(setClauses, "d.tags_json = $tags_json")
    params["tags_json"] = marshalTags(*tags)
}
if scope != nil {
    setClauses = append(setClauses, "d.scope = $scope")
    params["scope"] = string(*scope)
}
if originSpec != nil {
    setClauses = append(setClauses, "d.origin_spec = $origin_spec")
    params["origin_spec"] = *originSpec
}
if originStage != nil {
    setClauses = append(setClauses, "d.origin_stage = $origin_stage")
    params["origin_stage"] = *originStage
}
```

Add version bump: `setClauses = append(setClauses, "d.version = coalesce(d.version, 0) + 1")`

Update the content hash recomputation to pass new field values from the parsed decision.

Update the RETURN clause to include new fields.

- [ ] **Step 2: Fix all callers of UpdateDecision**

Search for all callers of `store.UpdateDecision` in handler tests, handler code, and other tests. Update call signatures to pass `nil` for new parameters where they don't set new fields.

Key files:
- `internal/server/decision_handler.go:118` — add nil params for new fields
- `internal/server/decision_handler_test.go` — update all mock/fake `UpdateDecision` implementations
- `internal/server/test_scoper_test.go` — update stub backend
- `internal/storage/memgraph/decision_test.go` — update integration test calls

- [ ] **Step 3: Verify build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

Message: `feat(memgraph): UpdateDecision handles ADR-003 fields with version bump (spgr-bk8)`

---

### Task 6: Fix all callers of CreateDecision

**Files:**

- Modify: `internal/server/decision_handler.go`
- Modify: `internal/server/decision_handler_test.go`
- Modify: `internal/server/test_scoper_test.go`

- [ ] **Step 1: Update CreateDecision handler to pass new fields**

In `decision_handler.go`, `CreateDecision` method: extract new fields from `msg` and pass to `store.CreateDecision`. For fields not yet in the proto request (until Task 3 proto changes are wired), pass zero values.

- [ ] **Step 2: Update all fake/mock CreateDecision implementations**

Search for `CreateDecision(` in test files. Update signatures to match the new interface. Pass zero values for new params where tests don't exercise them.

- [ ] **Step 3: Run full build and unit tests**

Run: `go build ./... && go test -short ./...`

- [ ] **Step 4: Commit**

Message: `feat(server): wire ADR-003 fields through CreateDecision handler (spgr-bk8)`

---

## Chunk 3: Handler, Converter, ChangeLog

### Task 7: Add enum maps and converter functions for new fields

**Files:**

- Modify: `internal/server/convert_decision.go`

- [ ] **Step 1: Add confidence and scope enum maps**

```go
var decisionConfidenceToProtoMap = map[storage.DecisionConfidence]specv1.DecisionConfidence{
    storage.DecisionConfidenceHigh:   specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
    storage.DecisionConfidenceMedium: specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM,
    storage.DecisionConfidenceLow:    specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW,
}

var decisionConfidenceFromProtoMap = map[specv1.DecisionConfidence]storage.DecisionConfidence{
    specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH:   storage.DecisionConfidenceHigh,
    specv1.DecisionConfidence_DECISION_CONFIDENCE_MEDIUM: storage.DecisionConfidenceMedium,
    specv1.DecisionConfidence_DECISION_CONFIDENCE_LOW:    storage.DecisionConfidenceLow,
}
```

Same pattern for `DecisionScope`.

Add `rejectedAltsToProto` / `rejectedAltsFromProto` helpers.

- [ ] **Step 2: Update decisionToProto to include new fields**

Map all new domain fields to proto fields in `decisionToProto`.

- [ ] **Step 3: Update handler to extract new fields from Update requests**

In `decision_handler.go` `UpdateDecision`: extract optional confidence, scope, question, etc. from the proto request and pass to `store.UpdateDecision`.

In `CreateDecision`: extract new fields from `CreateDecisionRequest` and pass to `store.CreateDecision`.

- [ ] **Step 4: Run tests**

Run: `go test -short ./internal/server/ -run TestDecision -v`

- [ ] **Step 5: Commit**

Message: `feat(server): converter and handler support for ADR-003 Decision fields (spgr-bk8)`

---

### Task 8: Add DecisionFields and ChangeLog generalization

**Files:**

- Modify: `internal/storage/fieldchange.go`
- Modify: `internal/storage/memgraph/changelog.go`

- [ ] **Step 1: Add DecisionFields struct and ComputeDecisionFieldDeltas**

In `internal/storage/fieldchange.go`:

```go
// DecisionFields holds the substantive fields of a decision for delta computation.
type DecisionFields struct {
    Title                string
    Status               string
    Body                 string
    Rationale            string
    Question             string
    Confidence           string
    Scope                string
    Tags                 string // JSON-serialized for comparison
    RejectedAlternatives string // JSON-serialized for comparison
    OriginSpec           string
    OriginStage          string
}

// ComputeDecisionFieldDeltas compares two DecisionFields and returns a slice of FieldChange
// for every field that differs.
func ComputeDecisionFieldDeltas(old, updated *DecisionFields) []FieldChange {
    var deltas []FieldChange
    pairs := []struct {
        field  string
        oldVal string
        newVal string
    }{
        {"title", old.Title, updated.Title},
        {"status", old.Status, updated.Status},
        {"decision", old.Body, updated.Body},
        {"rationale", old.Rationale, updated.Rationale},
        {"question", old.Question, updated.Question},
        {"confidence", old.Confidence, updated.Confidence},
        {"scope", old.Scope, updated.Scope},
        {"tags", old.Tags, updated.Tags},
        {"rejected_alternatives", old.RejectedAlternatives, updated.RejectedAlternatives},
        {"origin_spec", old.OriginSpec, updated.OriginSpec},
        {"origin_stage", old.OriginStage, updated.OriginStage},
    }
    for _, p := range pairs {
        if p.oldVal != p.newVal {
            deltas = append(deltas, FieldChange{Field: p.field, OldValue: p.oldVal, NewValue: p.newVal})
        }
    }
    return deltas
}
```

- [ ] **Step 2: Generalize createChangeLog to accept a node label**

In `internal/storage/memgraph/changelog.go`, change `createChangeLog` signature:

```go
func (s *Store) createChangeLog(ctx context.Context, label, slug string, entry *storage.ChangeLogEntry, changes []storage.FieldChange) error
```

Replace the hardcoded Cypher:
```go
query := fmt.Sprintf(`
    MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(n:%s {slug: $slug})
    WHERE n.version = $expected_version
    CREATE (n)-[:HAS_CHANGE]->(cl:ChangeLog {
        ...
    })
    RETURN cl.id
`, label)
```

Update all existing callers of `createChangeLog` in the memgraph package (search for `s.createChangeLog(`) to pass `"Spec"` as the first argument.

- [ ] **Step 3: Generalize ListChanges to accept a node label**

In `ListChanges`, change the check query and main query from `(s:Spec {slug: $slug})` to use a label parameter. Either:
- Add a `label` parameter to `ListChanges` (preferred, keeps it generic), OR
- Add a separate `ListDecisionChanges` method

**Recommended:** Add label parameter. Update `ChangeLogBackend` interface if needed, or keep it spec-only and add a separate internal method for decisions.

For now, keep `ChangeLogBackend.ListChanges` as-is (spec-only) and add an internal `listChangesByLabel` method that both spec and decision changelog queries use.

- [ ] **Step 4: Wire ChangeLog into UpdateDecision**

In `UpdateDecision`, inside the `RunInTransaction` block, after the mutation:
1. Build `DecisionFields` for old and new state
2. Call `ComputeDecisionFieldDeltas`
3. Call `s.createChangeLog("Decision", slug, entry, deltas)`

The `ChangeLogEntry.Stage` field: for decisions, use the decision's `Status` string (e.g., "proposed", "accepted") since decisions don't have authoring stages.

- [ ] **Step 5: Run tests**

Run: `go build ./... && go test -short ./...`

- [ ] **Step 6: Commit**

Message: `feat(memgraph): generalize ChangeLog for Decision nodes (spgr-bk8)`

---

## Chunk 4: CLI, Render, Integration Tests

### Task 9: Update CLI decision create with new flags

**Files:**

- Modify: `cmd/specgraph/decision.go`

- [ ] **Step 1: Add flag variables and register flags**

```go
var (
    decisionQuestion    string
    decisionConfidence  string
    decisionTags        string
    decisionScope       string
    decisionOriginSpec  string
    decisionOriginStage string
    decisionRejected    []string // repeatable --rejected flag
)
```

Register in init():
```go
decisionCreateCmd.Flags().StringVar(&decisionQuestion, "question", "", "the question being decided")
decisionCreateCmd.Flags().StringVar(&decisionConfidence, "confidence", "", "confidence level (high|medium|low)")
decisionCreateCmd.Flags().StringVar(&decisionTags, "tags", "", "comma-separated tags")
decisionCreateCmd.Flags().StringVar(&decisionScope, "scope", "", "decision scope (project|team|org)")
decisionCreateCmd.Flags().StringVar(&decisionOriginSpec, "origin-spec", "", "slug of originating spec")
decisionCreateCmd.Flags().StringVar(&decisionOriginStage, "origin-stage", "", "authoring stage")
decisionCreateCmd.Flags().StringArrayVar(&decisionRejected, "rejected", nil, `rejected alternative "Option:Reason" (repeatable)`)
```

- [ ] **Step 2: Update runDecisionCreate to pass new fields**

Parse `--rejected` flags into `[]RejectedAlternative` (split on first colon). Parse `--tags` into `[]string` (split on comma, trim whitespace). Map confidence/scope strings to proto enums. Pass to `CreateDecisionRequest`.

- [ ] **Step 3: Run CLI build**

Run: `go build ./cmd/specgraph/`

- [ ] **Step 4: Commit**

Message: `feat(cli): add ADR-003 flags to decision create command (spgr-bk8)`

---

### Task 10: Update render for new Decision fields

**Files:**

- Modify: `internal/render/decision.go`
- Modify: `internal/render/decision_test.go`

- [ ] **Step 1: Write failing test for new field rendering**

```go
func TestDecisionRender_NewFields(t *testing.T) {
    d := &specv1.Decision{
        Slug:       "use-postgres",
        Title:      "Token storage mechanism",
        Status:     specv1.DecisionStatus_DECISION_STATUS_ACCEPTED,
        Question:   "Where to store refresh tokens?",
        Confidence: specv1.DecisionConfidence_DECISION_CONFIDENCE_HIGH,
        Scope:      specv1.DecisionScope_DECISION_SCOPE_PROJECT,
        Tags:       []string{"auth", "storage"},
        OriginSpec: "login-api",
        RejectedAlternatives: []*specv1.RejectedAlternative{
            {Option: "Redis", Reason: "Adds ops complexity"},
        },
    }
    got := render.Decision(d)
    assert.Contains(t, got, "Where to store refresh tokens?")
    assert.Contains(t, got, "HIGH")
    assert.Contains(t, got, "PROJECT")
    assert.Contains(t, got, "auth, storage")
    assert.Contains(t, got, "login-api")
    assert.Contains(t, got, "Redis")
    assert.Contains(t, got, "Adds ops complexity")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run TestDecisionRender_NewFields -v`

- [ ] **Step 3: Update Decision renderer**

In `render/decision.go`, add new fields to the metadata pairs and add sections:

After the existing metadata table, add:
- Confidence, Scope, Origin Spec, Origin Stage to metadata pairs (if non-empty/non-default)
- Tags as inline comma-separated after metadata table
- Question as `**Question:** ...` section
- Rejected Alternatives as a table with Option | Reason columns

- [ ] **Step 4: Run tests**

Run: `go test ./internal/render/ -v`

- [ ] **Step 5: Commit**

Message: `feat(render): display ADR-003 fields in decision output (spgr-bk8)`

---

### Task 11: Integration tests

**Files:**

- Modify: `internal/storage/memgraph/decision_test.go`

- [ ] **Step 1: Update CreateDecision integration test**

Update the existing `TestCreateDecision` to pass new fields and assert they are stored and returned correctly.

- [ ] **Step 2: Add test for UpdateDecision with new fields**

Test updating question, tags, confidence, scope, rejected alternatives. Assert version is incremented. Assert content hash changes.

- [ ] **Step 3: Add test for backward compatibility**

Create a decision with old API (zero values for new fields), then read it back. Assert new fields default to zero values gracefully.

- [ ] **Step 4: Run integration tests**

Run: `go test -tags integration -run TestDecision -v ./internal/storage/memgraph/`

- [ ] **Step 5: Run full quality gate**

Run: `task check`

- [ ] **Step 6: Commit**

Message: `test(integration): Decision ADR-003 field storage and backward compat (spgr-bk8)`

---

### Task 12: Final quality gate and PR

- [ ] **Step 1: Run full pr-prep**

Run: `task pr-prep`

- [ ] **Step 2: Create branch and PR**

Branch: `spgr-bk8-decision-adr003`
PR title: `feat: extend Decision type with ADR-003 fields (spgr-bk8)`

- [ ] **Step 3: Close bead**

Run: `bd close spgr-bk8 --reason="PR #NNN adds ADR-003 fields to Decision type"`
