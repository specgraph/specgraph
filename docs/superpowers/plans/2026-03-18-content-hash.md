# Content Hash Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Murmur3-128 content hash field to Spec and Decision nodes for change detection, while keeping stable ULID-based IDs for graph identity.

**Architecture:** The `content_hash` field is a 32-char hex string (Murmur3-128) computed from a spec's substantive fields (intent, stage, priority, complexity, authoring outputs). It is computed in the storage layer on every create/update, stored as a Memgraph node property, and exposed through the proto API. The hash excludes metadata (version, timestamps, history) so it captures "what the spec says" not "when it was last touched."

**Tech Stack:** Go, Murmur3 (`github.com/spaolacci/murmur3`), Protocol Buffers, Memgraph (Cypher)

**Decision:** spgr-hya — Stable ULID IDs with Murmur3-128 content hash

**Beads:**

- spgr-h08 — Write ADR
- spgr-18l — Proto changes
- spgr-lmt — Implementation
- spgr-csm — Site/plan doc updates

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md` | Create | Design decision record |
| `proto/specgraph/v1/spec.proto` | Modify | Add `content_hash` field, fix `id` comment |
| `proto/specgraph/v1/decision.proto` | Modify | Add `content_hash` field, fix `id` comment |
| `internal/storage/spec_domain.go` | Modify | Add `ContentHash` field to `Spec` struct |
| `internal/storage/decision.go` | Modify | Add `ContentHash` field to `Decision` struct |
| `internal/storage/contenthash/contenthash.go` | Create | Murmur3-128 hash computation |
| `internal/storage/contenthash/contenthash_test.go` | Create | Unit tests for hash computation |
| `internal/storage/memgraph/memgraph.go` | Modify | Store/read `content_hash` in CreateSpec, UpdateSpec, GetSpec, ListSpecs |
| `internal/storage/memgraph/decision.go` | Modify | Store/read `content_hash` in Decision CRUD |
| `internal/storage/memgraph/memgraph_test.go` | Modify | Assert content_hash on create/update |
| `internal/server/convert.go` | Modify | Map `ContentHash` between domain and proto |
| `internal/server/spec_handler_test.go` | Modify | Update mock to set ContentHash |
| `site/docs/concepts/specs.md` | Modify | Rewrite Identity section |
| `site/docs/how-it-works.md` | Modify | Fix content-addressable language |
| `docs/plans/2026-02-28-client-server-architecture-design.md` | Modify | Fix Identity Scheme section |
| `docs/plans/2026-02-28-vertical-slice-plan.md` | Modify | Fix proto snippet comment |

---

## Chunk 1: ADR and Proto Changes

### Task 1: Write ADR-002 (spgr-h08)

**Files:**

- Create: `docs/decisions/ADR-002-stable-ulid-ids-content-hash.md`

- [ ] **Step 1: Write the ADR document**

Follow the format from `docs/decisions/ADR-001-principle-statement-field-naming.md`:

```markdown
# ADR-002: Stable ULID IDs with Murmur3-128 Content Hash

- **Status:** Accepted
- **Date:** 2026-03-18
- **Bead:** spgr-hya
- **Supersedes:** Implicit "content-addressable ID" convention in docs and proto comments

## Context

The proto comments and site documentation describe spec and decision IDs as
"content-addressable" — implying the ID is derived from the spec's content and
changes when the content changes. However, the actual implementation generates
ULIDs (timestamp + randomness) via `newID()` in the Memgraph storage layer.
These are assigned once at creation and never change.

Content-addressable IDs that change on every mutation would break graph edges
(DEPENDS_ON, BLOCKS, COMPOSES, etc.) and require cascading updates across the
graph on every spec edit — significant complexity for the storage layer.

The system still needs a way to detect whether a spec's content has changed,
primarily for drift detection and sync adapter reconciliation.

## Decision

1. **Keep ULIDs as stable node IDs.** The `id` field on Spec and Decision
   remains a ULID (`spec-{ULID}`, `dec-{ULID}`), assigned once at creation.
   Graph edges reference these stable IDs and never need updating.

2. **Add a `content_hash` field.** A Murmur3-128 hash (32 hex characters)
   computed from the spec's substantive fields. Recomputed on every create
   and update.

3. **Hash inputs (Spec):** `intent`, `stage`, `priority`, `complexity`, and
   all authoring stage outputs (`spark_output`, `shape_output`,
   `specify_output`, `decompose_output`).

4. **Hash inputs (Decision):** `title`, `status` (string representation of
   the `DecisionStatus` enum), `decision`, `rationale`.

5. **Hash excludes:** `id`, `slug`, `version`, `created_at`, `updated_at`,
   `history`, `superseded_by`, `supersedes`, `notes`, `lifecycle`,
   `drift_acknowledged`, `drift_acknowledge_note`.

## Rationale

- **Graph stability:** Edges reference IDs. Changing IDs on content mutation
  would require O(edges) updates per edit — unacceptable complexity.
- **Change detection without diffing:** Comparing two 32-char strings is O(1)
  vs. comparing every field. Drift detection and sync adapters benefit directly.
- **Murmur3-128 over SHA-256:** This is fingerprinting, not security. Murmur3
  is ~10x faster and 32 hex chars is more readable than 64. 128 bits provides
  sufficient collision resistance for change detection.
- **Separate concerns:** The slug is the human name. The ULID is the machine
  pointer. The content hash is the fingerprint. Three jobs, three fields.

## Alternatives Considered

- **Content-addressable IDs (hash as the id):** Rejected — ID changes on every
  edit break graph edges. The complexity cost is not justified.
- **Hash only slug + spark as identity:** Rejected — unclear benefit over using
  slug as key. Two people sparking the same slug with different wording produce
  different IDs, defeating dedup.
- **SHA-256 full (64 hex chars):** Rejected — cryptographic strength unnecessary
  for fingerprinting. Longer string, slower computation, no benefit.
- **SHA-256 truncated:** Rejected — if we're truncating anyway, use a hash
  designed for the purpose (Murmur3) rather than truncating a crypto hash.

## Consequences

- Proto messages gain a `content_hash` field (Spec field 15, Decision field 10).
- Storage layer computes hash on create/update; no caller changes needed.
- Drift detection can compare hashes instead of field-by-field diffing.
- Sync adapters can use hash comparison for efficient reconciliation.
- Site docs and proto comments updated to remove "content-addressable" language
  from ID descriptions.
```

- [ ] **Step 2: Commit**

```bash
jj commit -m "docs(adr): ADR-002 stable ULID IDs with Murmur3-128 content hash"
```

---

### Task 2: Update proto files (spgr-18l)

**Files:**

- Modify: `proto/specgraph/v1/spec.proto:23,36` (fix id comment on line 23, add content_hash field 15 after line 36)
- Modify: `proto/specgraph/v1/decision.proto:25,33` (fix id comment on line 25, add content_hash field 10 after line 33)

- [ ] **Step 1: Update spec.proto**

Change the `id` field comment from `content-addressable` to `stable ULID`. Add `content_hash` as field 15 after `notes` (field 14).

In `proto/specgraph/v1/spec.proto`, change:

```protobuf
  string id = 1;           // content-addressable, e.g. "spec-k7m3p"
```

to:

```protobuf
  string id = 1;           // stable ULID, e.g. "spec-01JQXYZ..."
```

Add after the last field:

```protobuf
  string content_hash = 15; // Murmur3-128 hex digest of substantive fields; changes on every mutation
```

- [ ] **Step 2: Update decision.proto**

Change:

```protobuf
  string id = 1;                             // content-addressable, e.g. "dec-a7f3b2c"
```

to:

```protobuf
  string id = 1;                             // stable ULID, e.g. "dec-01JQXYZ..."
```

Add `content_hash` field after `updated_at` (field 9):

```protobuf
  string content_hash = 10;                  // Murmur3-128 hex digest of substantive fields; changes on every mutation
```

- [ ] **Step 3: Regenerate proto code**

Run: `task proto`
Expected: `gen/specgraph/v1/spec.pb.go` and `gen/specgraph/v1/decision.pb.go` regenerated with new `ContentHash` field.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: PASS (new field is zero-valued, no callers break)

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(proto): add content_hash field to Spec and Decision messages"
```

---

## Chunk 2: Hash Computation and Storage Wiring

### Task 3: Implement content hash computation (includes murmur3 dependency)

**Files:**

- Create: `internal/storage/contenthash/contenthash.go`
- Create: `internal/storage/contenthash/contenthash_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/storage/contenthash/contenthash_test.go
package contenthash_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage/contenthash"
	"github.com/stretchr/testify/require"
)

func TestSpecHash_Deterministic(t *testing.T) {
	h1 := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	h2 := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32) // 128 bits = 32 hex chars
}

func TestSpecHash_ChangesOnFieldChange(t *testing.T) {
	base := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	changed := contenthash.Spec("implement login", "shape", "p1", "medium", nil)
	require.NotEqual(t, base, changed)
}

func TestSpecHash_IncludesAuthoringOutputs(t *testing.T) {
	noOutputs := contenthash.Spec("implement login", "spark", "p1", "medium", nil)
	withOutputs := contenthash.Spec("implement login", "spark", "p1", "medium",
		map[string]string{"spark_output": `{"seed":"test"}`})
	require.NotEqual(t, noOutputs, withOutputs)
}

func TestDecisionHash_Deterministic(t *testing.T) {
	h1 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	h2 := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	require.Equal(t, h1, h2)
	require.Len(t, h1, 32)
}

func TestDecisionHash_ChangesOnFieldChange(t *testing.T) {
	base := contenthash.Decision("Use Memgraph", "accepted", "We chose Memgraph", "Fast graph queries")
	changed := contenthash.Decision("Use Memgraph", "superseded", "We chose Memgraph", "Fast graph queries")
	require.NotEqual(t, base, changed)
}
```

- [ ] **Step 2: Add murmur3 dependency**

Run: `go get github.com/spaolacci/murmur3`

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/storage/contenthash/ -v`
Expected: FAIL — package does not exist yet

- [ ] **Step 4: Write the implementation**

```go
// internal/storage/contenthash/contenthash.go
package contenthash

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/spaolacci/murmur3"
)

// Spec computes a Murmur3-128 content hash for a spec's substantive fields.
// authoringOutputs is a map of property name → JSON string (e.g., "spark_output" → "...").
// Nil or empty authoringOutputs are treated the same (no outputs).
func Spec(intent, stage, priority, complexity string, authoringOutputs map[string]string) string {
	h := murmur3.New128()
	writeField(h, "intent", intent)
	writeField(h, "stage", stage)
	writeField(h, "priority", priority)
	writeField(h, "complexity", complexity)

	// Sort keys for deterministic ordering.
	keys := make([]string, 0, len(authoringOutputs))
	for k := range authoringOutputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeField(h, k, authoringOutputs[k])
	}

	hi, lo := h.Sum128()
	return fmt.Sprintf("%016x%016x", hi, lo)
}

// Decision computes a Murmur3-128 content hash for a decision's substantive fields.
func Decision(title, status, decision, rationale string) string {
	h := murmur3.New128()
	writeField(h, "title", title)
	writeField(h, "status", status)
	writeField(h, "decision", decision)
	writeField(h, "rationale", rationale)

	hi, lo := h.Sum128()
	return fmt.Sprintf("%016x%016x", hi, lo)
}

// writeField writes a length-prefixed key and value to the hasher.
// Length prefixing prevents "ab"+"c" from hashing the same as "a"+"bc".
// murmur3.Hash128 is an interface — do NOT use a pointer to it.
func writeField(h murmur3.Hash128, key, value string) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(key)))
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(key))
	binary.BigEndian.PutUint32(buf[:], uint32(len(value)))
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(value))
}
```

Note: `writeField` uses length-prefixed framing so field boundaries are unambiguous — this prevents hash collisions from concatenation ambiguity (e.g., intent="ab" + stage="c" vs intent="a" + stage="bc").

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/storage/contenthash/ -v`
Expected: PASS — all 5 tests pass

- [ ] **Step 6: Tidy and commit**

```bash
go mod tidy
jj commit -m "feat(storage): add Murmur3-128 content hash computation"
```

---

### Task 4: Add ContentHash to domain types

**Files:**

- Modify: `internal/storage/spec_domain.go` (add `ContentHash string` field to `Spec`)
- Modify: `internal/storage/decision.go` (add `ContentHash string` field to `Decision`)

- [ ] **Step 1: Add ContentHash field to Spec struct**

In `internal/storage/spec_domain.go`, add `ContentHash string` after `Notes`:

```go
ContentHash string
```

- [ ] **Step 2: Add ContentHash field to Decision struct**

In `internal/storage/decision.go`, add `ContentHash string` after `UpdatedAt`:

```go
ContentHash string
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
jj commit -m "feat(storage): add ContentHash field to Spec and Decision domain types"
```

---

### Task 5: Wire content hash into Memgraph Spec operations

**Files:**

- Modify: `internal/storage/memgraph/memgraph.go` (CreateSpec, UpdateSpec, GetSpec, ListSpecs, recordToSpec/recordToSpecOffset)
- Modify: `internal/storage/memgraph/lifecycle.go` (RETURN clauses that feed recordToSpecOffset)
- Modify: `internal/storage/memgraph/lifecycle_test.go` (update recordToSpecOffset offsets from 16 → 17)

- [ ] **Step 1: Write failing integration test**

In `internal/storage/memgraph/memgraph_test.go`, add a test (or modify existing CreateAndGetSpec test) that asserts `spec.ContentHash` is non-empty and 32 chars after creation:

```go
func TestCreateSpec_SetsContentHash(t *testing.T) {
	// ... standard memgraph setup ...
	spec, err := store.CreateSpec(ctx, "hash-test", "Test content hashing", "p1", "medium")
	require.NoError(t, err)
	require.Len(t, spec.ContentHash, 32, "content_hash should be 32-char hex")

	// Update should change the hash.
	newIntent := "Updated intent"
	updated, err := store.UpdateSpec(ctx, "hash-test", &newIntent, nil, nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, updated.ContentHash, 32)
	require.NotEqual(t, spec.ContentHash, updated.ContentHash, "hash should change when intent changes")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/storage/memgraph/ -run TestCreateSpec_SetsContentHash -v`
Expected: FAIL — ContentHash is empty string

- [ ] **Step 3: Wire CreateSpec**

In `internal/storage/memgraph/memgraph.go`, in `CreateSpec`:

1. Import `contenthash` package
2. After building parameters, compute hash: `ch := contenthash.Spec(intent, defaultInitialStage, priority, complexity, nil)`
3. Add `content_hash` to the Cypher CREATE and RETURN clauses
4. Pass `ch` as parameter `$content_hash`

- [ ] **Step 4: Wire UpdateSpec**

In `UpdateSpec`, compute `content_hash` within the same Cypher query to avoid race conditions.
Use a WITH clause to read back all hash-input fields after the SET, then compute the hash in Go
and include it in the same query's SET. Alternatively, use the `recomputeContentHash` method
(from Task 9) wrapped in `RunInTransaction` with the main update.

Preferred single-query approach:

1. In the existing UPDATE Cypher, add all hash-input fields to the RETURN clause
   (`s.intent, s.stage, s.priority, s.complexity, s.spark_output, s.shape_output, s.specify_output, s.decompose_output`)
2. After executing and reading the returned values, compute the hash in Go
3. Execute a second SET query for just `content_hash` within the same transaction

If UpdateSpec does not currently use transactions, the two-query approach is acceptable since
SpecGraph is single-writer. Note this in a comment for future multi-writer scenarios.

- [ ] **Step 4a: Update lifecycle.go RETURN clauses**

`lifecycle.go` has RETURN clauses (around lines 152, 264-271, and 380) that feed into
`recordToSpecOffset`. Add `s.content_hash` to ALL of these RETURN clauses. Note:
`SupersedeSpec` (lines 264-271) returns two full spec records (old at offset 0, new at
offset 16→17) — both RETURN groups need the new field.

- [ ] **Step 4b: Update lifecycle_test.go offsets**

The existing `lifecycle_test.go` uses `recordToSpecOffset(rec, 16)` for the second spec in
two-spec RETURN rows. Adding `content_hash` increases the field count from 16 to 17. Update
all offset values and mock record construction accordingly.

- [ ] **Step 5: Wire recordToSpec**

In `recordToSpec` / `recordToSpecOffset`, read the `content_hash` field from the neo4j record into `spec.ContentHash`. Add it to the RETURN clause of all queries that use recordToSpec (CreateSpec, GetSpec, ListSpecs, UpdateSpec).

- [ ] **Step 6: Run test to verify it passes**

Run: `go test -tags integration ./internal/storage/memgraph/ -run TestCreateSpec_SetsContentHash -v`
Expected: PASS

- [ ] **Step 7: Run full integration test suite**

Run: `go test -tags integration ./internal/storage/memgraph/ -v`
Expected: PASS — existing tests still pass (some may need updated field counts/offsets in recordToSpec)

- [ ] **Step 8: Commit**

```bash
jj commit -m "feat(memgraph): compute and store content_hash on spec create/update"
```

---

### Task 6: Wire content hash into Memgraph Decision operations

**Files:**

- Modify: `internal/storage/memgraph/decision.go`

- [ ] **Step 1: Write failing integration test**

Add test asserting `decision.ContentHash` is non-empty and 32 chars after creation.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration ./internal/storage/memgraph/ -run TestCreateDecision_SetsContentHash -v`
Expected: FAIL

- [ ] **Step 3: Wire CreateDecision, UpdateDecision, and recordToDecision**

All Cypher queries that create, update, or read decisions need `content_hash`:

1. **CreateDecision:** Compute `contenthash.Decision(title, string(status), body, rationale)` and
   include `content_hash` in the CREATE clause and RETURN. Note: `status` is a `DecisionStatus`
   enum — convert to its string name before hashing.
2. **UpdateDecision:** Recompute hash after applying updates (same approach as UpdateSpec —
   read back all hash-input fields, compute hash, SET in same transaction or second query).
3. **GetDecision:** Add `d.content_hash` to RETURN clause.
4. **ListDecisions:** Add `d.content_hash` to RETURN clause.
5. **recordToDecision:** Read `content_hash` from the neo4j record into `decision.ContentHash`.

- [ ] **Step 4: Run test to verify it passes**

Expected: PASS

- [ ] **Step 5: Commit**

```bash
jj commit -m "feat(memgraph): compute and store content_hash on decision create/update"
```

---

### Task 7: Wire content hash into proto conversion

**Files:**

- Modify: `internal/server/convert.go` (specToProto, decisionToProto)
- Modify: `internal/server/spec_handler_test.go` (update mock)

- [ ] **Step 1: Update specToProto**

Add `ContentHash: s.ContentHash` to the proto mapping in `specToProto`.

- [ ] **Step 2: Update decisionToProto**

Add `ContentHash: d.ContentHash` to the proto mapping in `decisionToProto`.

- [ ] **Step 3: Update mock backends**

Update ALL mock methods across ALL test files that return `*storage.Spec` or `*storage.Decision`
to set a dummy `ContentHash` (e.g., `strings.Repeat("a", 32)`). Mock backends exist in:

- `internal/server/spec_handler_test.go` (mockBackend)
- `internal/server/authoring_handler_test.go` (fakeBackend, authoringTestBackend, fullAuthoringTestBackend, fakeFullBackend)
- `internal/server/decision_handler_test.go` (mockDecisionBackend)
- `internal/server/sync_handler_test.go` (mockSpecReader, syncTestBackend)
- `internal/server/lifecycle_handler_test.go` (fakeLifecycleBackend)
- `internal/server/test_scoper_test.go` (stubBackend)

Update methods: `CreateSpec`, `GetSpec`, `ListSpecs`, `UpdateSpec`, `CreateDecision`,
`GetDecision`, `ListDecisions`, `UpdateDecision` — wherever they return domain types.

- [ ] **Step 4: Run unit tests**

Run: `go test ./internal/server/ -v`
Expected: PASS

- [ ] **Step 5: Run full build and lint**

Run: `task check`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
jj commit -m "feat(server): wire ContentHash through proto conversion layer"
```

---

## Chunk 3: Authoring Hash Updates

### Task 8: Recompute content hash when authoring outputs or stage change

**Files:**

- Modify: `internal/storage/memgraph/authoring.go`

This is important: `storeJSONProperty` writes authoring outputs (spark_output, shape_output, etc.) which are hash inputs. After storing, the content_hash must be recomputed.

- [ ] **Step 1: Write failing integration test**

```go
func TestStoreSparkOutput_UpdatesContentHash(t *testing.T) {
	// Create spec, record initial hash.
	spec, err := store.CreateSpec(ctx, "authoring-hash", "Test", "p1", "medium")
	require.NoError(t, err)
	initialHash := spec.ContentHash

	// Store spark output (method is StoreSparkOutput, not SaveSparkOutput).
	err = store.StoreSparkOutput(ctx, "authoring-hash", &storage.SparkOutput{Seed: "test seed"})
	require.NoError(t, err)

	// Re-read spec, hash should have changed.
	updated, err := store.GetSpec(ctx, "authoring-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHash, updated.ContentHash)
}
```

- [ ] **Step 2: Run test to verify it fails**

Expected: FAIL — hash unchanged after authoring output stored

- [ ] **Step 3: Add `recomputeContentHash` private method**

Add a private method `recomputeContentHash(ctx, slug)` on `*Store` that:

1. Reads all hash-input fields and authoring output JSONs from the spec node
2. Calls `contenthash.Spec(...)` with the current values
3. SETs `content_hash` on the node

Only call `recomputeContentHash` for the four authoring output properties that are hash inputs:
`spark_output`, `shape_output`, `specify_output`, `decompose_output`.

Do NOT call it for analytical pass properties (`red_team_findings`, `peripheral_vision`,
`consistency_issues`, `simplicity_findings`, `safety_flags`, `constitution_violations`) —
these are not hash inputs.

Implementation: add a conditional in `storeJSONProperty` or check in callers:

```go
var hashInputProperties = map[string]bool{
	"spark_output":    true,
	"shape_output":    true,
	"specify_output":  true,
	"decompose_output": true,
}
```

After `storeJSONProperty` succeeds, only call `recomputeContentHash` if `hashInputProperties[property]`.

Note: child spec creation in `StoreDecomposeOutput` does NOT affect the parent's content hash.
The parent hash changes because `decompose_output` JSON was written, not because children were created.

- [ ] **Step 4: Wire `recomputeContentHash` into `TransitionStage` and `AmendSpec`**

Both `TransitionStage` (`authoring.go:41`) and `AmendSpec` (`authoring.go:278`) change the
`stage` field via direct Cypher SET — they do NOT go through `storeJSONProperty`. Since `stage`
is a hash input, the content hash will go stale unless we call `recomputeContentHash` in these
methods too.

In `TransitionStage`: call `recomputeContentHash(ctx, slug)` after the stage update succeeds.
In `AmendSpec`: call `recomputeContentHash(ctx, slug)` after the amend update succeeds.

Also wire `LifecycleAmendSpec` (`lifecycle.go:110`) — it modifies stage at line 145
(`SET s.stage = $stage`), so it needs the same `recomputeContentHash` call.

- [ ] **Step 5: Write test for TransitionStage hash update**

```go
func TestTransitionStage_UpdatesContentHash(t *testing.T) {
	spec, err := store.CreateSpec(ctx, "stage-hash", "Test", "p1", "medium")
	require.NoError(t, err)
	initialHash := spec.ContentHash

	// Spark → Shape transition changes stage (a hash input).
	err = store.TransitionStage(ctx, "stage-hash", "spark", "shape")
	require.NoError(t, err)

	updated, err := store.GetSpec(ctx, "stage-hash")
	require.NoError(t, err)
	require.NotEqual(t, initialHash, updated.ContentHash)
}
```

- [ ] **Step 6: Run test to verify it passes**

Expected: PASS

- [ ] **Step 7: Run full integration suite**

Run: `go test -tags integration ./internal/storage/memgraph/ -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
jj commit -m "feat(memgraph): recompute content_hash when authoring outputs or stage change"
```

---

## Chunk 4: Documentation Updates

### Task 9: Update site docs (spgr-csm)

**Files:**

- Modify: `site/docs/concepts/specs.md:5-7` (intro paragraph)
- Modify: `site/docs/concepts/specs.md:99-114` (Identity section)
- Modify: `site/docs/concepts/specs.md:124` (Core Schema table — add content_hash to Identity row)
- Modify: `site/docs/how-it-works.md:55-57`

- [ ] **Step 1: Rewrite specs.md intro (lines 5-7)**

Replace lines 5-7:

```markdown
A spec is a **work unit** in the SpecGraph graph. Every spec has a stable,
content-addressable identity (e.g. `spec-k7m3p`), a human-readable slug
(e.g. `oauth-refresh-rotation`), and structured content that progresses through
```

with:

```markdown
A spec is a **work unit** in the SpecGraph graph. Every spec has a stable
identity (a ULID like `spec-01JQXYZ...`), a human-readable slug
(e.g. `oauth-refresh-rotation`), and structured content that progresses through
```

- [ ] **Step 2: Rewrite specs.md Identity section (lines 99-115)**

Replace lines 99-115:

```markdown
## Identity

Every spec has a **content-addressable identity** with the format
`spec-{short-hash}`. The hash is derived from the spec's content, which gives
you three properties:

1. **Merge-conflict-free** — two developers can create specs independently and
   merge without ID collisions. There are no sequential counters to fight over.
2. **Change detection** — if the content changes, the hash changes. You always
   know whether a spec has been modified since you last saw it.
3. **Distributed-safe** — no central authority assigns IDs. Teams across repos,
   time zones, or organizations produce globally unique identifiers by default.

The human-readable slug (`oauth-refresh-rotation`) exists for convenience in
conversation, CLI output, and documentation. The stable `spec-{hash}` is what
the graph stores and what edges reference.
```

with:

```markdown
## Identity

Every spec has three identity fields:

- **`id`** — a stable ULID (e.g. `spec-01JQXYZ...`), assigned once at creation
  and never changed. Graph edges (`DEPENDS_ON`, `BLOCKS`, `COMPOSES`) reference
  this ID. ULIDs are timestamp-based and globally unique without coordination.
- **`slug`** — a human-readable name (e.g. `oauth-refresh-rotation`) used in
  CLI output, documentation, and conversation.
- **`content_hash`** — a Murmur3-128 fingerprint (32 hex characters) of the
  spec's substantive fields: intent, stage, priority, complexity, and all
  authoring stage outputs. Recomputed on every create or update.

This gives you three properties:

1. **Merge-conflict-free** — ULIDs have no sequential counters. Two developers
   can create specs independently and merge without ID collisions.
2. **Change detection** — the `content_hash` changes whenever the spec's
   content changes. Drift detection and sync adapters can compare hashes
   instead of diffing every field.
3. **Distributed-safe** — no central authority assigns IDs. Teams across repos,
   time zones, or organizations produce globally unique identifiers by default.
```

- [ ] **Step 3: Update Core Schema table**

Change the Identity row from:

```markdown
| **Identity** | `id`, `slug`, `version`, `created_at`, `updated_at` |
```

to:

```markdown
| **Identity** | `id`, `slug`, `version`, `content_hash`, `created_at`, `updated_at` |
```

- [ ] **Step 4: Fix how-it-works.md**

Change:

```markdown
Every spec has a **content-addressable identity**: its ID is derived from its
content, so you can detect when a spec has changed and track its history without
relying on mutable names or paths.
```

to:

```markdown
Every spec has a **stable identity** (ULID-based) and a **content hash** — a
Murmur3-128 fingerprint of the spec's substantive fields. The hash changes when
content changes, enabling drift detection without field-by-field comparison.
```

- [ ] **Step 5: Commit**

```bash
jj commit -m "docs(site): update identity docs to reflect ULID + content_hash design"
```

---

### Task 10: Update plan docs

**Files:**

- Modify: `docs/plans/2026-02-28-client-server-architecture-design.md:83-91`
- Modify: `docs/plans/2026-02-28-vertical-slice-plan.md:171`

- [ ] **Step 1: Fix architecture design Identity Scheme**

Change:

```markdown
### Identity Scheme

Content-addressable hashing (merge-conflict-free IDs), type-prefixed:
```

to:

```markdown
### Identity Scheme

Stable ULID-based IDs (merge-conflict-free), type-prefixed. A separate
`content_hash` field (Murmur3-128) provides change detection:
```

Update the example table to show ULID-style IDs:

```markdown
| Spec | `spec-` | `spec-01JQXYZ...` |
| Decision | `dec-` | `dec-01JQXYZ...` |
```

- [ ] **Step 2: Fix vertical slice plan proto comment**

Change:

```protobuf
  string id = 1;           // content-addressable, e.g. "spec-k7m3p"
```

to:

```protobuf
  string id = 1;           // stable ULID, e.g. "spec-01JQXYZ..."
```

- [ ] **Step 3: Commit**

```bash
jj commit -m "docs(plans): update plan docs to reflect ULID + content_hash design"
```

---

## Execution Notes

- **Task dependency order:** 1 → 2 → 3 → 4 → 5/6 (parallel) → 7 → 8 → 9/10 (parallel)
- **Integration tests (Tasks 5, 6, 8) require Docker** for Memgraph testcontainers
- **After Task 2**, run `task proto` to regenerate — the `gen/` files are committed
- **murmur3.Hash128 type:** `murmur3.New128()` returns `murmur3.Hash128` which is an **interface**, not a struct. Do NOT use a pointer to it (`*murmur3.Hash128` will not compile). Access `Sum128()` directly on the interface value — no type assertion needed.
- **Field numbers:** Spec `content_hash` = field 15 (after `notes` at 14). Decision `content_hash` = field 10 (after `updated_at` at 9).
- **Intentionally excluded:** `docs/initial-design-session/specgraph-v1.0-draft-adr-003-decisions.md` mentions "content-addressed" twice (lines 38, 200) but in the context of beads/Dolt identity and an unimplemented Postgres design path — not SpecGraph's current ID scheme. Leave these references as-is.
- **DecisionStatus enum:** The `contenthash.Decision` function takes the string representation of the status. The proto enum values are `DECISION_STATUS_ACCEPTED`, `DECISION_STATUS_SUPERSEDED`, etc. — but the storage layer uses lowercase strings (`"accepted"`, `"superseded"`). Use whichever form the storage layer already uses consistently — the key is that `CreateDecision` and `UpdateDecision` must hash the same string form to avoid mismatches.
