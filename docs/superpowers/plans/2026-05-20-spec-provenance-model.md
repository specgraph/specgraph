<!--
SPDX-License-Identifier: Apache-2.0
-->

# Spec Provenance Model Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the `SpecLifecycle` field (task/living) with a `SpecProvenance` axis (AUTHORED/RETROACTIVE_FROM_PR/DECLARED) end-to-end across proto, storage, server, CLI, MCP, render, linter, and export — landing as a single PR.

**Architecture:** Wire-break at proto field 10 (`SpecLifecycle lifecycle` → `SpecProvenance provenance_type`) plus a `oneof provenance_detail` at fields 22/23/24 carrying type-specific structured payload. Storage domain types mirror the shape with a string-typed discriminator and optional variant pointers. Postgres migration 007 drops `lifecycle` column and adds `provenance_type TEXT NOT NULL` + `provenance_detail JSONB NOT NULL`, with a precondition guard refusing to run on a non-empty `specs` table. Stage progressions diverge by provenance: AUTHORED walks the funnel; RETROACTIVE_FROM_PR and DECLARED are born at `done`. `GetReady` gains provenance + claim filters. Claim and report-completion are gated to `provenance_type = AUTHORED`.

**Tech Stack:** Go 1.26.3, protobuf via `buf` (`task proto`), ConnectRPC, pgx v5 + pgxpool, goose SQL migrations, Cobra CLI, mark3labs/mcp-go for MCP tools, testify (unit) + ginkgo/gomega (e2e), murmur3 content hashing, jj-colocated git, Taskfile.dev for orchestration.

**Source design:** `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md` (merged in PR #953)

**Single PR constraint:** All tasks below land as one branch and one PR (`fix/spec-provenance-model` or similar). Commits are separated by task for review clarity, but the PR is monolithic.

**Mid-PR build state:** The proto change in Phase 1 intentionally breaks the build. `task check` will not pass again until Phase 7. This is the cost of the clean break and is documented in the design's risks section.

---

## Phase 0 — Setup

### Task 0.1: Confirm working state and create branch

**Files:** none — working-copy state check only.

- [ ] **Step 1: Confirm clean working copy**

  ```bash
  jj --no-pager status
  ```

  Expected output: `Working copy changes: ...` (any drift from `specgraph init` is fine — those files will be `jj restore`d before `task check`); `Parent commit (@-): ... main | <recent commit>`. If unrelated files are modified beyond the init-managed ones (`.claude/settings.json`, `.cursor/mcp.json`, `.mcp.json`, `opencode.json`), stop and investigate.

- [ ] **Step 2: Fetch + ensure main is up to date**

  ```bash
  jj --no-pager git fetch
  jj --no-pager log -r 'ancestors(@-, 3)' --no-graph
  ```

  Expected: most recent commit on `main` is at or after the design-doc merge (PR #953).

- [ ] **Step 3: Create a working change off main**

  ```bash
  jj --no-pager new main -m '(wip) spec-provenance model implementation'
  ```

  Expected: `Working copy ... (empty) ...`. We'll squash this WIP message later when we structure individual task commits.

---

## Phase 1 — Foundation (proto, sentinels, ADR stub)

The proto change is first because every other layer compiles against the new types. The build will break immediately and stay broken until Phase 7. This is intentional — the type checker becomes the find-every-caller tool.

### Task 1.1: Add new sentinel errors

**Files:**
- Modify: `internal/storage/errors.go`

Adding sentinels first so handler code in later phases can reference them without forward declarations.

- [ ] **Step 1: Append sentinels at the end of the file**

  Open `internal/storage/errors.go` and append after the existing sentinel block:

  ```go
  // --- Provenance ---

  // ErrProvenanceMismatch is returned when provenance_type and provenance_detail are inconsistent.
  var ErrProvenanceMismatch = errors.New("provenance_type does not match populated provenance_detail variant")

  // ErrAuthoredRequiresSparkOnly is returned when an AUTHORED spec create includes stage outputs beyond spark.
  var ErrAuthoredRequiresSparkOnly = errors.New("AUTHORED provenance: only spark_output may be set at create")

  // ErrRetroactiveRequiresAllOutputs is returned when a RETROACTIVE_FROM_PR create is missing one or more funnel outputs.
  var ErrRetroactiveRequiresAllOutputs = errors.New("RETROACTIVE_FROM_PR provenance: spark/shape/specify/decompose outputs all required")

  // ErrRetroactiveRequiresPRRef is returned when a RETROACTIVE_FROM_PR create is missing url or sha.
  var ErrRetroactiveRequiresPRRef = errors.New("RETROACTIVE_FROM_PR provenance: url and sha are required")

  // ErrDeclaredRequiresAllOutputs is returned when a DECLARED create is missing one or more funnel outputs.
  var ErrDeclaredRequiresAllOutputs = errors.New("DECLARED provenance: spark/shape/specify/decompose outputs all required")

  // ErrDeclaredRequiresDeclaredBy is returned when a DECLARED create is missing declared_by.
  var ErrDeclaredRequiresDeclaredBy = errors.New("DECLARED provenance: declared_by is required")

  // ErrClaimRequiresAuthored is returned when claim is invoked on a non-AUTHORED spec.
  var ErrClaimRequiresAuthored = errors.New("claim requires provenance_type = AUTHORED")

  // ErrCompletionRequiresAuthored is returned when report-completion is invoked on a non-AUTHORED spec.
  var ErrCompletionRequiresAuthored = errors.New("report-completion requires provenance_type = AUTHORED")
  ```

- [ ] **Step 2: Run a focused build to verify the errors package compiles**

  ```bash
  go build ./internal/storage/
  ```

  Expected: no output (success). If it fails, fix the syntax before proceeding.

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "feat(storage): add provenance sentinel errors

  Eight new sentinels for the upcoming provenance model: mismatch
  validation, per-provenance create-time invariants, and claim/completion
  gating. Defined ahead of the proto change so subsequent layers can
  reference them.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/errors.go
  ```

### Task 1.2: Add ADR-006 stub

**Files:**
- Create: `docs/decisions/ADR-006-spec-provenance-model.md`

Stub now so the touch surface is complete; finalize content in Task 8.3.

- [ ] **Step 1: Create the ADR stub**

  ```bash
  cat > docs/decisions/ADR-006-spec-provenance-model.md <<'EOF'
  <!-- SPDX-License-Identifier: Apache-2.0 -->

  # ADR-006: Spec Provenance Model

  - **Status:** Proposed
  - **Date:** 2026-05-20
  - **Supersedes:** SpecLifecycle enum (task/living)
  - **Implementation:** see docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md

  ## Context

  *(to be finalized in Task 8.3 after implementation)*

  ## Decision

  Replace the `SpecLifecycle` enum + field with `SpecProvenance` (AUTHORED / RETROACTIVE_FROM_PR / DECLARED) plus a structured `provenance_detail` oneof. See design doc for full rationale.

  ## Consequences

  *(to be finalized in Task 8.3)*
  EOF
  ```

- [ ] **Step 2: Commit the stub**

  ```bash
  jj --no-pager commit -m "docs(adr): ADR-006 stub for spec provenance model

  Stub placeholder; finalized after implementation lands.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" docs/decisions/ADR-006-spec-provenance-model.md
  ```

### Task 1.3: Update the proto schema

**Files:**
- Modify: `proto/specgraph/v1/spec.proto`

- [ ] **Step 1: Replace the SpecLifecycle enum block with SpecProvenance**

  In `proto/specgraph/v1/spec.proto`, find lines 13-19 (the existing `SpecLifecycle` enum) and replace with:

  ```proto
  // --- Enums ---

  // SpecProvenance records how a spec entered the graph. Replaces the
  // earlier SpecLifecycle field (task/living) at pre-1.0 with a clean
  // wire-break — see docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md.
  enum SpecProvenance {
    SPEC_PROVENANCE_UNSPECIFIED         = 0;
    SPEC_PROVENANCE_AUTHORED            = 1; // forward-authored via the funnel
    SPEC_PROVENANCE_RETROACTIVE_FROM_PR = 2; // imported from a merged PR/commit
    SPEC_PROVENANCE_DECLARED            = 3; // human-declared as describing existing reality
  }

  // AuthoredProvenance is the empty payload variant for AUTHORED specs.
  // The audit trail for AUTHORED specs lives in stage outputs and conversation logs.
  message AuthoredProvenance {}

  message RetroactiveFromPrProvenance {
    string url       = 1; // PR URL
    string sha       = 2; // merge commit SHA
    google.protobuf.Timestamp merged_at = 3;
    string title     = 4; // PR title at import time
  }

  message DeclaredProvenance {
    string declared_by = 1; // human or system identifier
    string note        = 2; // free-text rationale
  }
  ```

- [ ] **Step 2: Update the Spec message — replace field 10 and add the oneof**

  In the same file, find the `Spec` message. Replace line 33 (`SpecLifecycle lifecycle = 10; // task (default) | living`) with:

  ```proto
    // WIRE-BREAK: field 10 was `SpecLifecycle lifecycle`. Repurposed at
    // pre-1.0 (no production data); semantic intent preserved — it's still
    // "how should the funnel treat this spec," with cleaner axes.
    SpecProvenance provenance_type = 10;
  ```

  After the `conversation_count` field (line 45) and before the closing brace of the `Spec` message, add:

  ```proto
    // provenance_detail carries the type-specific payload for non-AUTHORED specs.
    // The populated variant MUST match provenance_type — enforced server-side
    // with storage.ErrProvenanceMismatch.
    oneof provenance_detail {
      AuthoredProvenance          authored             = 22;
      RetroactiveFromPrProvenance retroactive_from_pr  = 23;
      DeclaredProvenance          declared             = 24;
    }
  ```

- [ ] **Step 3: Update the `Spec.stage` doc comment**

  Find line 27 (currently `string stage = 4;        // spark | shape | specify | decompose | approved | in_progress | done`) and update the inline comment to enumerate all stages:

  ```proto
    string stage = 4;        // spark | shape | specify | decompose | approved | in_progress | review | done | superseded | abandoned
  ```

- [ ] **Step 4: Regenerate proto code**

  ```bash
  task proto
  ```

  Expected: regeneration runs; `gen/specgraph/v1/spec.pb.go` is updated. If `task proto` reports "no changes" the fingerprint is stale — delete `.task/checksum/proto*` and retry.

- [ ] **Step 5: Verify build breakage as expected**

  ```bash
  go build ./... 2>&1 | head -30
  ```

  Expected: many compile errors referencing `SpecLifecycle`, `Lifecycle` field on `Spec`, `lifecycleToProtoMap`, etc. This is the intended state — every caller of the removed type now needs an update.

- [ ] **Step 6: Commit (build intentionally broken)**

  ```bash
  jj --no-pager commit -m "feat(proto)!: replace SpecLifecycle with SpecProvenance

  Wire-break at field 10 (lifecycle → provenance_type) plus oneof
  provenance_detail at fields 22-24. Stage doc-string updated to
  enumerate all 8 stages (was missing review/superseded/abandoned).

  Build intentionally broken; subsequent tasks chase callers via the
  type checker. See docs/superpowers/plans/2026-05-20-spec-provenance-model.md
  for the full task ordering.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" proto/specgraph/v1/spec.proto gen/
  ```

  Note the `!` after `feat(proto)` indicates a breaking change per Conventional Commits.

---

## Phase 2 — Domain + conversion layer

### Task 2.1: Update storage domain types

**Files:**
- Modify: `internal/storage/spec_domain.go`

- [ ] **Step 1: Remove the SpecLifecycle type and constants**

  In `internal/storage/spec_domain.go`, find the `SpecLifecycle` block (lines 134-151) and delete it entirely:

  ```go
  // DELETE this block:
  // SpecLifecycle represents the lifecycle model of a spec.
  type SpecLifecycle string
  // ... and the IsValid method
  ```

- [ ] **Step 2: Add the new provenance domain types in the same location**

  Replace the removed block with:

  ```go
  // SpecProvenanceType is the string-typed discriminator for how a spec
  // entered the graph. Mirrors the SpecProvenance proto enum.
  type SpecProvenanceType string

  // Spec provenance discriminator values.
  const (
      SpecProvenanceAuthored          SpecProvenanceType = "authored"
      SpecProvenanceRetroactiveFromPR SpecProvenanceType = "retroactive_from_pr"
      SpecProvenanceDeclared          SpecProvenanceType = "declared"
  )

  // IsValid reports whether p is a known spec provenance type.
  func (p SpecProvenanceType) IsValid() bool {
      switch p {
      case SpecProvenanceAuthored, SpecProvenanceRetroactiveFromPR, SpecProvenanceDeclared:
          return true
      default:
          return false
      }
  }

  // SpecProvenanceDetail is the structured payload for non-AUTHORED specs.
  // Exactly one of the embedded pointers is non-nil; both nil is valid (AUTHORED).
  // The populated variant must match the Spec.Provenance discriminator —
  // enforced at the server boundary with storage.ErrProvenanceMismatch.
  type SpecProvenanceDetail struct {
      RetroactiveFromPR *RetroactivePRProvenance // populated when type == retroactive_from_pr
      Declared          *DeclaredProvenance      // populated when type == declared
  }

  // RetroactivePRProvenance carries PR metadata for retroactive-import specs.
  type RetroactivePRProvenance struct {
      URL      string
      SHA      string
      MergedAt time.Time
      Title    string
  }

  // DeclaredProvenance carries declaration metadata for human-declared specs.
  type DeclaredProvenance struct {
      DeclaredBy string
      Note       string
  }
  ```

- [ ] **Step 3: Update the Spec struct**

  Find the `Spec` struct (line 174-196). Remove the `Lifecycle SpecLifecycle` line and add provenance fields:

  ```go
  type Spec struct {
      ID                string
      Slug              string
      Intent            string
      Stage             SpecStage
      Priority          SpecPriority
      Complexity        SpecComplexity
      Version           int32
      CreatedAt         time.Time
      UpdatedAt         time.Time
      Provenance        SpecProvenanceType
      ProvenanceDetail  SpecProvenanceDetail
      SupersededBy      string
      Supersedes        string
      Notes             string
      ContentHash       string
      ConversationLogs  []*ConversationLogEntry
      SparkOutput       *SparkOutput
      ShapeOutput       *ShapeOutput
      SpecifyOutput     *SpecifyOutput
      DecomposeOutput   *DecomposeOutput
      ConversationCount int
  }
  ```

- [ ] **Step 4: Verify package compiles**

  ```bash
  go build ./internal/storage/
  ```

  Expected: success. If failures reference SpecLifecycle, search for stragglers:

  ```bash
  grep -rn 'SpecLifecycle' internal/storage/ | grep -v _test
  ```

- [ ] **Step 5: Commit**

  ```bash
  jj --no-pager commit -m "feat(storage): provenance domain types

  Replace SpecLifecycle string + Spec.Lifecycle field with
  SpecProvenanceType + Spec.Provenance + Spec.ProvenanceDetail
  (string discriminator + struct with optional variant pointers).

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/spec_domain.go
  ```

### Task 2.2: Update server convert layer

**Files:**
- Modify: `internal/server/convert_spec.go`

- [ ] **Step 1: Remove the lifecycle conversion**

  In `internal/server/convert_spec.go`, delete lines 70-86 (the `// --- Lifecycle ---` section, the `lifecycleToProtoMap`, and the `lifecycleToProto` function).

- [ ] **Step 2: Remove the lifecycle call site in specToProto**

  Find the call `lc, err := lifecycleToProto(s.Lifecycle)` near line 16-18 of `specToProto` and the `Lifecycle: lc,` line in the struct literal (~line 30). Delete both.

- [ ] **Step 3: Add provenance conversion**

  At the bottom of `convert_spec.go`, add:

  ```go
  // --- Provenance ---

  // provenanceToProtoMap maps storage provenance values to proto enums.
  var provenanceToProtoMap = map[storage.SpecProvenanceType]specv1.SpecProvenance{
      storage.SpecProvenanceAuthored:          specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED,
      storage.SpecProvenanceRetroactiveFromPR: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
      storage.SpecProvenanceDeclared:          specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
  }

  func provenanceToProto(p storage.SpecProvenanceType) (specv1.SpecProvenance, error) {
      if v, ok := provenanceToProtoMap[p]; ok {
          return v, nil
      }
      return specv1.SpecProvenance_SPEC_PROVENANCE_UNSPECIFIED, fmt.Errorf("unknown provenance: %q", p)
  }

  // provenanceDetailToProto packages the storage-side detail struct into the
  // proto oneof. The returned value should be assigned to Spec.ProvenanceDetail
  // (a oneof in proto, which generates a sum-type interface in Go).
  func provenanceDetailToProto(d storage.SpecProvenanceDetail) specv1.IsSpec_ProvenanceDetail {
      switch {
      case d.RetroactiveFromPR != nil:
          return &specv1.Spec_RetroactiveFromPr{
              RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
                  Url:      d.RetroactiveFromPR.URL,
                  Sha:      d.RetroactiveFromPR.SHA,
                  MergedAt: timeToProto(d.RetroactiveFromPR.MergedAt),
                  Title:    d.RetroactiveFromPR.Title,
              },
          }
      case d.Declared != nil:
          return &specv1.Spec_Declared{
              Declared: &specv1.DeclaredProvenance{
                  DeclaredBy: d.Declared.DeclaredBy,
                  Note:       d.Declared.Note,
              },
          }
      default:
          // AUTHORED — empty payload.
          return &specv1.Spec_Authored{Authored: &specv1.AuthoredProvenance{}}
      }
  }

  // proto-side back to domain (used by handler validation).
  func provenanceFromProto(p specv1.SpecProvenance) (storage.SpecProvenanceType, error) {
      for domainVal, protoVal := range provenanceToProtoMap {
          if protoVal == p {
              return domainVal, nil
          }
      }
      return "", fmt.Errorf("unknown provenance enum: %q", p.String())
  }

  func provenanceDetailFromProto(pb specv1.IsSpec_ProvenanceDetail) storage.SpecProvenanceDetail {
      switch v := pb.(type) {
      case *specv1.Spec_RetroactiveFromPr:
          return storage.SpecProvenanceDetail{
              RetroactiveFromPR: &storage.RetroactivePRProvenance{
                  URL:      v.RetroactiveFromPr.GetUrl(),
                  SHA:      v.RetroactiveFromPr.GetSha(),
                  MergedAt: v.RetroactiveFromPr.GetMergedAt().AsTime(),
                  Title:    v.RetroactiveFromPr.GetTitle(),
              },
          }
      case *specv1.Spec_Declared:
          return storage.SpecProvenanceDetail{
              Declared: &storage.DeclaredProvenance{
                  DeclaredBy: v.Declared.GetDeclaredBy(),
                  Note:       v.Declared.GetNote(),
              },
          }
      case *specv1.Spec_Authored:
          return storage.SpecProvenanceDetail{}
      default:
          return storage.SpecProvenanceDetail{}
      }
  }
  ```

  Note: the exact generated type names (`Spec_RetroactiveFromPr`, `IsSpec_ProvenanceDetail`) depend on how protoc-gen-go names oneof variants. Run `task proto` and `grep '^type Spec_' gen/specgraph/v1/spec.pb.go` to confirm names if compilation fails.

- [ ] **Step 4: Wire provenance into specToProto**

  Near the top of `specToProto`, replace the deleted lifecycle assignment with:

  ```go
      pv, err := provenanceToProto(s.Provenance)
      if err != nil {
          return nil, fmt.Errorf("spec %q: %w", s.Slug, err)
      }
  ```

  In the struct literal where `Lifecycle: lc,` was, add:

  ```go
          ProvenanceType:   pv,
          ProvenanceDetail: provenanceDetailToProto(s.ProvenanceDetail),
  ```

- [ ] **Step 5: Verify**

  ```bash
  go build ./internal/server/
  ```

  Expected: success. If failures reference generated oneof type names, run:

  ```bash
  grep '^type Spec_' gen/specgraph/v1/spec.pb.go
  ```

  and adjust the type assertion names in `provenanceDetailToProto` accordingly.

- [ ] **Step 6: Commit**

  ```bash
  jj --no-pager commit -m "feat(server): provenance proto↔domain conversion

  Replace lifecycleToProto with provenanceToProto/FromProto and
  provenanceDetailToProto/FromProto. Generated oneof-variant type
  names referenced explicitly.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/server/convert_spec.go
  ```

---

## Phase 3 — Storage (migration + queries)

### Task 3.1: Postgres migration 007

**Files:**
- Create: `internal/storage/postgres/migrations/007_spec_provenance.sql`

- [ ] **Step 1: Write the migration**

  ```bash
  cat > internal/storage/postgres/migrations/007_spec_provenance.sql <<'EOF'
  -- SPDX-License-Identifier: Apache-2.0
  -- Copyright 2026 Sean Brandt

  -- +goose Up

  -- Precondition guard: this migration is a clean-break replacement.
  -- It refuses to run if any rows exist in specs to prevent accidental
  -- data loss in environments where the clean-break assumption is wrong.
  DO $$
  BEGIN
    IF (SELECT count(*) FROM specs) > 0 THEN
      RAISE EXCEPTION 'migration 007 refuses to run on a non-empty specs table; clean-break design assumes no data — see docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md';
    END IF;
  END
  $$;

  ALTER TABLE specs DROP COLUMN lifecycle;
  ALTER TABLE specs ADD COLUMN provenance_type TEXT NOT NULL;
  ALTER TABLE specs ADD COLUMN provenance_detail JSONB NOT NULL;

  -- +goose Down
  ALTER TABLE specs DROP COLUMN provenance_detail;
  ALTER TABLE specs DROP COLUMN provenance_type;
  ALTER TABLE specs ADD COLUMN lifecycle TEXT NOT NULL DEFAULT 'task';
  EOF
  ```

- [ ] **Step 2: Commit**

  ```bash
  jj --no-pager commit -m "feat(postgres): migration 007 drops lifecycle, adds provenance columns

  Clean-break replacement with a precondition guard refusing to run on a
  non-empty specs table. provenance_type TEXT NOT NULL + provenance_detail
  JSONB NOT NULL. Down migration restores the lifecycle column for
  reversibility.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/postgres/migrations/007_spec_provenance.sql
  ```

### Task 3.2: Update postgres spec.go insert/select

**Files:**
- Modify: `internal/storage/postgres/spec.go`

- [ ] **Step 1: Update the insert in CreateSpec**

  Find the `INSERT INTO specs` query (around lines 49-59) and replace the column list + values:

  ```go
  row := s.queryRow(txCtx,
      `INSERT INTO specs
          (id, slug, project_slug, intent, stage, priority, complexity,
           provenance_type, provenance_detail, notes,
           content_hash, version, created_at, updated_at)
       VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, '', $10, 1, $11, $11)
       RETURNING id, slug, project_slug, intent, stage, priority, complexity,
                 provenance_type, provenance_detail,
                 superseded_by, supersedes, notes, content_hash, version,
                 spark_output, shape_output, specify_output, decompose_output,
                 created_at, updated_at`,
      specID, slug, s.project, intent, defaultInitialStage, priority, complexity,
      string(storage.SpecProvenanceAuthored), `{"type":"authored","data":null}`,
      ch, now,
  )
  ```

  Also: remove the `defaultLifecycle` reference; if `defaultInitialStage` and `defaultLifecycle` are constants in this file, leave `defaultInitialStage` but delete `defaultLifecycle`.

- [ ] **Step 2: Update the GetSpec SELECT**

  Find the SELECT in `GetSpec` (line ~110-117) and replace `s.lifecycle` with `s.provenance_type, s.provenance_detail`. Update the corresponding `scanSpec` helper or the inline Scan call to read both columns. The provenance_detail JSONB needs to be scanned into a `[]byte` or `*json.RawMessage`, then unmarshalled into `storage.SpecProvenanceDetail`.

  Approach: write a helper `func decodeProvenanceDetail(raw []byte) (storage.SpecProvenanceDetail, error)` that switches on the envelope `{"type": "...", "data": {...}}` shape and populates the right pointer.

  ```go
  // decodeProvenanceDetail parses the JSONB envelope into a domain detail struct.
  func decodeProvenanceDetail(raw []byte) (storage.SpecProvenanceDetail, error) {
      if len(raw) == 0 {
          return storage.SpecProvenanceDetail{}, nil
      }
      var env struct {
          Type string          `json:"type"`
          Data json.RawMessage `json:"data"`
      }
      if err := json.Unmarshal(raw, &env); err != nil {
          return storage.SpecProvenanceDetail{}, fmt.Errorf("decode provenance_detail envelope: %w", err)
      }
      switch storage.SpecProvenanceType(env.Type) {
      case storage.SpecProvenanceAuthored:
          return storage.SpecProvenanceDetail{}, nil
      case storage.SpecProvenanceRetroactiveFromPR:
          var d struct {
              URL      string    `json:"url"`
              SHA      string    `json:"sha"`
              MergedAt time.Time `json:"merged_at"`
              Title    string    `json:"title"`
          }
          if err := json.Unmarshal(env.Data, &d); err != nil {
              return storage.SpecProvenanceDetail{}, fmt.Errorf("decode retroactive_from_pr: %w", err)
          }
          return storage.SpecProvenanceDetail{
              RetroactiveFromPR: &storage.RetroactivePRProvenance{
                  URL: d.URL, SHA: d.SHA, MergedAt: d.MergedAt, Title: d.Title,
              },
          }, nil
      case storage.SpecProvenanceDeclared:
          var d struct {
              DeclaredBy string `json:"declared_by"`
              Note       string `json:"note"`
          }
          if err := json.Unmarshal(env.Data, &d); err != nil {
              return storage.SpecProvenanceDetail{}, fmt.Errorf("decode declared: %w", err)
          }
          return storage.SpecProvenanceDetail{
              Declared: &storage.DeclaredProvenance{DeclaredBy: d.DeclaredBy, Note: d.Note},
          }, nil
      default:
          return storage.SpecProvenanceDetail{}, fmt.Errorf("unknown provenance type %q in detail envelope", env.Type)
      }
  }

  // encodeProvenanceDetail produces the JSONB envelope for storage.
  func encodeProvenanceDetail(p storage.SpecProvenanceType, d storage.SpecProvenanceDetail) ([]byte, error) {
      var data any
      switch p {
      case storage.SpecProvenanceAuthored:
          data = nil
      case storage.SpecProvenanceRetroactiveFromPR:
          if d.RetroactiveFromPR == nil {
              return nil, storage.ErrProvenanceMismatch
          }
          data = d.RetroactiveFromPR
      case storage.SpecProvenanceDeclared:
          if d.Declared == nil {
              return nil, storage.ErrProvenanceMismatch
          }
          data = d.Declared
      default:
          return nil, fmt.Errorf("unknown provenance type %q", p)
      }
      env := struct {
          Type string `json:"type"`
          Data any    `json:"data"`
      }{Type: string(p), Data: data}
      return json.Marshal(env)
  }
  ```

  Add these helpers to `spec.go` (or a new `provenance_jsonb.go` in the same package if file size warrants).

- [ ] **Step 3: Update scanSpec to decode provenance_detail**

  Find `scanSpec` (in the same file or wherever it's defined; search via `grep -n 'func scanSpec' internal/storage/postgres/spec.go`). Update the column list and Scan targets. Replace `lifecycle` parsing with `provenanceType` and `provenanceDetail`:

  ```go
  var (
      // ... existing scan targets ...
      provenanceType   string
      provenanceDetail []byte
      // ...
  )
  // Scan in the new column order:
  err := row.Scan(
      // ... existing ...,
      &provenanceType, &provenanceDetail,
      // ... existing ...,
  )

  detail, err := decodeProvenanceDetail(provenanceDetail)
  if err != nil {
      return nil, err
  }
  spec.Provenance = storage.SpecProvenanceType(provenanceType)
  spec.ProvenanceDetail = detail
  ```

  Whatever the exact `scanSpec` shape is, the pattern is: pull two columns instead of one, populate two Spec fields instead of one.

- [ ] **Step 4: Verify build**

  ```bash
  go build ./internal/storage/postgres/
  ```

  Expected: success. If `lifecycle` is still referenced elsewhere in this package, grep:

  ```bash
  grep -n 'lifecycle' internal/storage/postgres/spec.go
  ```

  and remove all occurrences (column names, scan targets, struct field references).

- [ ] **Step 5: Commit**

  ```bash
  jj --no-pager commit -m "feat(postgres): provenance read/write in spec.go

  - INSERT writes provenance_type + provenance_detail columns
  - SELECT reads them and decodes the JSONB envelope via
    decodeProvenanceDetail into storage.SpecProvenanceDetail
  - New helpers encodeProvenanceDetail / decodeProvenanceDetail
    enforce envelope type ↔ discriminator invariant
  - defaultLifecycle constant removed

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/postgres/spec.go
  ```

### Task 3.3: Rewrite GetReady query

**Files:**
- Modify: `internal/storage/postgres/graph.go` (function `GetReady` at line 358)

- [ ] **Step 1: Replace the GetReady query body**

  Find `func (s *Store) GetReady` (line 358) and replace the query string:

  ```go
  func (s *Store) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
      rows, err := s.query(ctx,
          `SELECT s.slug, 'Spec' AS label, s.stage
           FROM specs s
           WHERE s.project_slug = $1
             AND s.stage = 'approved'
             AND s.provenance_type = 'authored'
             AND NOT EXISTS (
                 -- Active claim
                 SELECT 1 FROM claims c
                 WHERE c.project_slug = $1 AND c.spec_slug = s.slug
                   AND (c.expires_at IS NULL OR c.expires_at > NOW())
             )
             AND NOT EXISTS (
                 SELECT 1 FROM edges e
                 JOIN specs dep ON dep.slug = e.to_slug AND dep.project_slug = $1
                 WHERE e.from_slug = s.slug AND e.edge_type = 'DEPENDS_ON' AND e.project_slug = $1
                   AND dep.stage <> 'done'
             )
             AND NOT EXISTS (
                 SELECT 1 FROM edges e
                 JOIN specs blocker ON blocker.slug = e.from_slug AND blocker.project_slug = $1
                 WHERE e.to_slug = s.slug AND e.edge_type = 'BLOCKS' AND e.project_slug = $1
                   AND blocker.stage <> 'done'
             )`,
          s.project,
      )
      // ... existing scan loop unchanged ...
  }
  ```

  Note: the active-claim check assumes the `claims` table has `project_slug`, `spec_slug`, `expires_at`. Verify with:

  ```bash
  grep -n 'CREATE TABLE.*claims' internal/storage/postgres/migrations/*.sql
  ```

  Adjust the claim subquery to match the actual schema if columns differ.

- [ ] **Step 2: Verify build**

  ```bash
  go build ./internal/storage/postgres/
  ```

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "feat(postgres): GetReady filters by provenance + active claim

  Query rewritten per design doc Section 3:
  - stage = 'approved' (was stage <> 'done')
  - provenance_type = 'authored' (explicit predicate; design Section 3)
  - no active claim
  - dependency + blocker checks unchanged

  Effect: design-stage and execution-stage specs no longer appear in
  ready; LIVING (now DECLARED/RETROACTIVE) specs naturally excluded.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/postgres/graph.go
  ```

---

## Phase 4 — Server handlers

### Task 4.1: Spec_handler create validation

**Files:**
- Modify: `internal/server/spec_handler.go`

The existing `CreateSpec` RPC takes only slug + intent + priority + complexity. The new design requires extending it to optionally accept provenance + provenance_detail + all four stage outputs (for non-AUTHORED creates).

- [ ] **Step 1: Read the existing CreateSpec handler**

  ```bash
  grep -n 'func.*CreateSpec' internal/server/spec_handler.go
  ```

  Note the handler signature and the path through validation + storage.

- [ ] **Step 2: Add a provenance validation helper**

  Add to `spec_handler.go` (or a new file `internal/server/provenance_validate.go`):

  ```go
  // validateProvenance enforces the create-time invariants per design.
  // - AUTHORED: only spark_output may be set; no provenance_detail
  // - RETROACTIVE_FROM_PR: all 4 outputs + retroactive_from_pr detail with url+sha
  // - DECLARED: all 4 outputs + declared detail with declared_by
  func validateProvenance(
      pt storage.SpecProvenanceType,
      pd storage.SpecProvenanceDetail,
      spark *storage.SparkOutput,
      shape *storage.ShapeOutput,
      specify *storage.SpecifyOutput,
      decompose *storage.DecomposeOutput,
  ) error {
      switch pt {
      case storage.SpecProvenanceAuthored:
          if shape != nil || specify != nil || decompose != nil {
              return storage.ErrAuthoredRequiresSparkOnly
          }
          if pd.RetroactiveFromPR != nil || pd.Declared != nil {
              return storage.ErrProvenanceMismatch
          }
      case storage.SpecProvenanceRetroactiveFromPR:
          if spark == nil || shape == nil || specify == nil || decompose == nil {
              return storage.ErrRetroactiveRequiresAllOutputs
          }
          if pd.RetroactiveFromPR == nil {
              return storage.ErrProvenanceMismatch
          }
          if pd.RetroactiveFromPR.URL == "" || pd.RetroactiveFromPR.SHA == "" {
              return storage.ErrRetroactiveRequiresPRRef
          }
          if pd.Declared != nil {
              return storage.ErrProvenanceMismatch
          }
      case storage.SpecProvenanceDeclared:
          if spark == nil || shape == nil || specify == nil || decompose == nil {
              return storage.ErrDeclaredRequiresAllOutputs
          }
          if pd.Declared == nil {
              return storage.ErrProvenanceMismatch
          }
          if pd.Declared.DeclaredBy == "" {
              return storage.ErrDeclaredRequiresDeclaredBy
          }
          if pd.RetroactiveFromPR != nil {
              return storage.ErrProvenanceMismatch
          }
      default:
          return fmt.Errorf("unknown provenance type %q", pt)
      }
      return nil
  }
  ```

- [ ] **Step 3: Wire validation into CreateSpec**

  In the `CreateSpec` handler, after slug/intent/priority validation but before the storage call, decode the request's provenance from proto, decode the four stage outputs (if present in the request), call `validateProvenance`, and pass the validated values through to the storage layer.

  Note: this requires extending `CreateSpecRequest` proto to carry the new fields. Per the design "extend existing CreateSpec to accept all four funnel outputs in a single call." Update the proto:

  ```proto
  message CreateSpecRequest {
    string slug = 1;
    string intent = 2;
    string priority = 3;
    string complexity = 4;
    SpecProvenance provenance_type = 5;
    oneof provenance_detail {
      AuthoredProvenance          authored             = 6;
      RetroactiveFromPrProvenance retroactive_from_pr  = 7;
      DeclaredProvenance          declared             = 8;
    }
    SparkOutput     spark_output     = 9;
    ShapeOutput     shape_output     = 10;
    SpecifyOutput   specify_output   = 11;
    DecomposeOutput decompose_output = 12;
  }
  ```

  Add these fields to `proto/specgraph/v1/spec.proto` in the `CreateSpecRequest` message (the existing fields are at 1-4). Run `task proto` again to regenerate.

- [ ] **Step 4: Extend the storage CreateSpec to accept the new arguments**

  Open `internal/storage/postgres/spec.go` `CreateSpec` and add parameters: `provenance storage.SpecProvenanceType`, `detail storage.SpecProvenanceDetail`, `spark *storage.SparkOutput`, `shape *storage.ShapeOutput`, `specify *storage.SpecifyOutput`, `decompose *storage.DecomposeOutput`.

  Logic:
  - If `provenance == AUTHORED`, initial stage = `spark`, only spark_output is inserted.
  - If `provenance == RETROACTIVE_FROM_PR` or `DECLARED`, initial stage = `done`, all four stage outputs are inserted, content_hash is pre-computed.

  Pre-compute content_hash by calling `contenthash.Spec(intent, "done", priority, complexity, outputsMap)` where `outputsMap` contains marshaled stage outputs.

  Persist the JSONB envelope via `encodeProvenanceDetail(provenance, detail)`.

- [ ] **Step 5: Verify build**

  ```bash
  go build ./internal/server/ ./internal/storage/postgres/
  ```

- [ ] **Step 6: Commit**

  ```bash
  jj --no-pager commit -m "feat(server): provenance validation in CreateSpec

  - CreateSpecRequest proto extended with provenance_type, provenance_detail,
    and all four stage outputs
  - validateProvenance enforces the three create-time invariants
  - Storage CreateSpec accepts the new arguments and pre-computes
    content_hash for born-at-done flows

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" proto/specgraph/v1/spec.proto gen/ internal/server/spec_handler.go internal/storage/postgres/spec.go
  ```

### Task 4.2: Claim and completion gating

**Files:**
- Modify: `internal/server/claim_handler.go`, `internal/server/execution_handler.go`

- [ ] **Step 1: Gate Claim on provenance**

  In `claim_handler.go`, find the Claim handler. After fetching the spec but before issuing the claim, add:

  ```go
  if spec.Provenance != storage.SpecProvenanceAuthored {
      return nil, connect.NewError(connect.CodeInvalidArgument, storage.ErrClaimRequiresAuthored)
  }
  ```

- [ ] **Step 2: Gate RecordCompletion on provenance**

  Same pattern in `execution_handler.go` for the `RecordCompletion` RPC. Fetch spec, check `spec.Provenance == AUTHORED`, return `ErrCompletionRequiresAuthored` otherwise.

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/server/
  ```

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "feat(server): gate claim+completion on provenance=AUTHORED

  Non-AUTHORED specs (RETROACTIVE_FROM_PR, DECLARED) are born at done
  and never visit approved/in_progress/review — claim and
  report-completion reject with explicit sentinels.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/server/claim_handler.go internal/server/execution_handler.go
  ```

---

## Phase 5 — Interfaces (CLI, MCP, render)

### Task 5.1: CLI lifecycle.go output

**Files:**
- Modify: `cmd/specgraph/lifecycle.go`

The CLI subcommands for amend/supersede/abandon print `lifecycle=...` in their output. Replace with `provenance=...`.

- [ ] **Step 1: Update each print statement**

  Find each occurrence of `.GetLifecycle().String()` (lines 59, 88, 89, 117 per the earlier grep) and replace with `.GetProvenanceType().String()`. Update the field label too:

  ```go
  // Before:
  fmt.Printf("Amended: %s (stage=%s, lifecycle=%s, version=%d)\n", ..., s.GetLifecycle().String(), ...)
  // After:
  fmt.Printf("Amended: %s (stage=%s, provenance=%s, version=%d)\n", ..., s.GetProvenanceType().String(), ...)
  ```

  Do the same for the other three sites in this file.

- [ ] **Step 2: Verify build**

  ```bash
  go build ./cmd/specgraph/
  ```

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "feat(cli): replace lifecycle with provenance in lifecycle CLI output

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" cmd/specgraph/lifecycle.go
  ```

### Task 5.2: CLI spec create flags

**Files:**
- Modify: `cmd/specgraph/spec.go` (or `cmd/specgraph/create.go` — search first)

- [ ] **Step 1: Locate the create command and any existing --lifecycle flag**

  ```bash
  grep -n 'create\|lifecycle\|cobra.Command' cmd/specgraph/spec.go | head -20
  ```

- [ ] **Step 2: Add --provenance and detail flags**

  Add to the create-command flag set:

  ```go
  cmd.Flags().String("provenance", "authored", "provenance type: authored | retroactive_from_pr | declared")
  cmd.Flags().String("pr-url", "", "PR URL (required when provenance=retroactive_from_pr)")
  cmd.Flags().String("pr-sha", "", "merge commit SHA (required when provenance=retroactive_from_pr)")
  cmd.Flags().String("pr-title", "", "PR title at import time (optional)")
  cmd.Flags().String("declared-by", "", "human or system identifier (required when provenance=declared)")
  cmd.Flags().String("declared-note", "", "free-text rationale (optional)")
  ```

  In the command's `RunE`, parse the flags and build the proto `CreateSpecRequest` with the populated provenance fields. For `authored` (default), no detail is needed. For non-AUTHORED, the caller must also provide stage outputs via JSON files (`--spark-json`, `--shape-json`, etc.) or another mechanism — defer the file-loading helpers to subagent judgment if not already in the codebase.

- [ ] **Step 3: Verify build**

  ```bash
  go build ./cmd/specgraph/
  ```

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "feat(cli): --provenance flag and detail flags on spec create

  Replaces the implicit task-lifecycle default. For non-AUTHORED creates,
  the caller passes --pr-url/--pr-sha or --declared-by along with the
  stage-output JSON files.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" cmd/specgraph/spec.go
  ```

### Task 5.3: MCP tools_spec.go

**Files:**
- Modify: `internal/mcp/tools_spec.go`

- [ ] **Step 1: Update the MCP spec tool's create-action schema**

  Find the `create` action handler in `tools_spec.go`. Update the param schema to accept `provenance` (string), `provenance_detail` (JSON object), and the four stage outputs (each a JSON-encoded string, similar to the existing `output` param pattern in `tools_authoring.go`).

- [ ] **Step 2: Parse and forward to the storage layer**

  Convert the friendly provenance string to `storage.SpecProvenanceType`, parse the JSON envelope into `storage.SpecProvenanceDetail`, and call the extended `CreateSpec` storage method.

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/mcp/
  ```

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "feat(mcp): provenance + stage-outputs on spec create tool

  The MCP spec tool's create action now accepts provenance and (for
  non-AUTHORED) all four stage outputs in a single call.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/mcp/tools_spec.go
  ```

### Task 5.4: Render layer

**Files:**
- Modify: `internal/render/spec.go`

The existing `lifecycleString` function (line 57-66) is dead code after this change. Replace with a provenance-aware render.

- [ ] **Step 1: Replace lifecycleString**

  Delete the existing function and add:

  ```go
  func provenanceString(p specv1.SpecProvenance) string {
      switch p {
      case specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED:
          return "AUTHORED"
      case specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR:
          return "RETROACTIVE_FROM_PR"
      case specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED:
          return "DECLARED"
      default:
          return "UNSPECIFIED"
      }
  }

  // renderProvenanceBlock formats the provenance line(s) for spec render output.
  // Always renders at least one line (no silent-default).
  func renderProvenanceBlock(s *specv1.Spec) string {
      pt := s.GetProvenanceType()
      switch d := s.GetProvenanceDetail().(type) {
      case *specv1.Spec_RetroactiveFromPr:
          r := d.RetroactiveFromPr
          return fmt.Sprintf(
              "provenance:   %s\n              %s\n              merged %s (commit %s)",
              provenanceString(pt), r.GetUrl(),
              r.GetMergedAt().AsTime().Format("2006-01-02"),
              r.GetSha(),
          )
      case *specv1.Spec_Declared:
          return fmt.Sprintf(
              "provenance:   %s\n              declared by %s: %q",
              provenanceString(pt), d.Declared.GetDeclaredBy(), d.Declared.GetNote(),
          )
      default:
          return fmt.Sprintf("provenance:   %s", provenanceString(pt))
      }
  }
  ```

- [ ] **Step 2: Wire renderProvenanceBlock into the spec-render output**

  Find the main spec-render function (search via `grep -n 'func.*Spec.*string' internal/render/spec.go` — usually `RenderSpec` or similar) and insert a call to `renderProvenanceBlock(s)` after the title/slug and before the stage details.

- [ ] **Step 3: Verify build + render tests**

  ```bash
  go build ./internal/render/
  go test ./internal/render/ -run TestSpec
  ```

  Some existing tests may reference lifecycle and fail. Update them to reference provenance.

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "feat(render): provenance-aware spec output

  Replaces lifecycleString switch with provenanceString and a
  renderProvenanceBlock that handles per-variant detail formatting.
  Always renders at least one provenance line.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/render/spec.go
  ```

---

## Phase 6 — Cross-cutting (linter, export)

### Task 6.1: Linter schema.go

**Files:**
- Modify: `internal/linter/schema.go`

- [ ] **Step 1: Replace the lifecycle validation block**

  Find lines 71-76 (the current `spec.Lifecycle.IsValid()` check) and replace:

  ```go
  // Validate provenance type
  if spec.Provenance != "" && !spec.Provenance.IsValid() {
      findings = append(findings, ...{
          Severity: linter.SeverityError,
          Message:  fmt.Sprintf("invalid provenance %q", spec.Provenance),
          Location: "provenance",
      })
  }
  ```

  (Match the exact severity/finding-struct shape of the surrounding code.)

- [ ] **Step 2: Add a provenance-vs-stage consistency check**

  After the type validation, add:

  ```go
  // Provenance and stage consistency: non-AUTHORED specs must be at done.
  if spec.Provenance != storage.SpecProvenanceAuthored &&
     spec.Provenance != "" &&
     spec.Stage != storage.SpecStageDone {
      findings = append(findings, ...{
          Severity: linter.SeverityError,
          Message:  fmt.Sprintf("provenance %q requires stage=done (got %q)", spec.Provenance, spec.Stage),
          Location: "provenance",
      })
  }
  ```

- [ ] **Step 3: Verify build**

  ```bash
  go build ./internal/linter/
  ```

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "feat(linter): validate provenance + provenance-stage consistency

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/linter/schema.go
  ```

### Task 6.2: Export engine

**Files:**
- Modify: `internal/export/engine.go`

- [ ] **Step 1: Replace lifecycle handling in the export path**

  Find lines 439-451 (the `spec.Lifecycle` references). Update to read/write `Provenance` and `ProvenanceDetail` instead:

  ```go
  // Restore provenance + detail + notes via UpdateSpec / re-export.
  if spec.Provenance != "" || spec.Notes != "" || spec.SupersededBy != "" || spec.Supersedes != "" {
      // ... pass spec.Provenance and spec.ProvenanceDetail through the update path ...
  }
  ```

  The exact shape depends on how `UpdateSpec` is being used in this file — match the existing pattern.

- [ ] **Step 2: Verify build**

  ```bash
  go build ./internal/export/
  ```

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "feat(export): serialize provenance instead of lifecycle

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/export/engine.go
  ```

---

## Phase 7 — Tests

### Task 7.1: Provenance creation-path integration tests

**Files:**
- Create: `internal/server/spec_handler_provenance_test.go`

- [ ] **Step 1: Write the AUTHORED-create test**

  ```go
  // SPDX-License-Identifier: Apache-2.0
  // Copyright 2026 Sean Brandt

  package server_test

  import (
      "context"
      "testing"

      "connectrpc.com/connect"
      "github.com/stretchr/testify/require"
      specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
      "github.com/specgraph/specgraph/internal/storage"
  )

  func TestCreateSpec_AuthoredHappyPath(t *testing.T) {
      ctx := context.Background()
      svc, cleanup := newTestSpecService(t)
      defer cleanup()

      req := &specv1.CreateSpecRequest{
          Slug:           "test-authored",
          Intent:         "test the authored happy path",
          ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED,
          // No detail set; no stage outputs except spark (optional at create per design).
      }
      resp, err := svc.CreateSpec(ctx, connect.NewRequest(req))
      require.NoError(t, err)
      require.Equal(t, specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED, resp.Msg.Spec.GetProvenanceType())
      require.Equal(t, "spark", resp.Msg.Spec.GetStage())
  }
  ```

  `newTestSpecService` is a helper to construct the service with a backing test store; reuse the pattern from other `_test.go` files in `internal/server/`. If no such helper exists, write a minimal one using the test backend in `authoring_handler_test.go:175` as a template.

- [ ] **Step 2: Run the test**

  ```bash
  go test ./internal/server/ -run TestCreateSpec_AuthoredHappyPath -v
  ```

  Expected: PASS.

- [ ] **Step 3: Add the RETROACTIVE happy-path test**

  Append to the same file:

  ```go
  func TestCreateSpec_RetroactiveBornAtDone(t *testing.T) {
      ctx := context.Background()
      svc, cleanup := newTestSpecService(t)
      defer cleanup()

      req := &specv1.CreateSpecRequest{
          Slug:           "test-retroactive",
          Intent:         "test the retroactive create path",
          ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
          ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
              RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
                  Url: "https://github.com/specgraph/specgraph/pull/952",
                  Sha: "b0684373",
              },
          },
          SparkOutput:     mustParseSparkOutput(t, `{"seed":"s","signal":"s","scopeSniff":"SCOPE_SNIFF_TINY","killTest":"k"}`),
          ShapeOutput:     mustParseShapeOutput(t, `{"scopeIn":["x"],"scopeOut":["y"],"approaches":[{"name":"a","description":"d","tradeoffs":["t"]}],"chosenApproach":"a","risks":["r"],"successMust":["m"],"successShould":["s"],"successWont":["w"]}`),
          SpecifyOutput:   mustParseSpecifyOutput(t, `{"interfaces":[{"name":"i","body":"b"}],"verifyCriteria":[{"category":"c","description":"d"}],"invariants":["i"],"touches":[{"path":"p","purpose":"u","changeType":"new"}]}`),
          DecomposeOutput: mustParseDecomposeOutput(t, `{"strategy":"DECOMPOSITION_STRATEGY_SINGLE_UNIT","slices":[{"id":"s","intent":"i","verify":["v"],"touches":["t"]}]}`),
      }
      resp, err := svc.CreateSpec(ctx, connect.NewRequest(req))
      require.NoError(t, err)
      require.Equal(t, "done", resp.Msg.Spec.GetStage())
      require.Equal(t, specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR, resp.Msg.Spec.GetProvenanceType())
  }
  ```

  Define `mustParseSparkOutput` etc. helpers at the top of the file using `protojson.Unmarshal`.

- [ ] **Step 4: Add rejection tests for invariant violations**

  Add tests for each sentinel from Task 1.1:

  ```go
  func TestCreateSpec_RetroactiveMissingPRRef(t *testing.T) {
      // ... omit Url ...
      _, err := svc.CreateSpec(ctx, connect.NewRequest(req))
      require.Error(t, err)
      require.ErrorIs(t, err, storage.ErrRetroactiveRequiresPRRef)
  }
  ```

  Repeat for: `ErrAuthoredRequiresSparkOnly` (AUTHORED with shape_output set), `ErrRetroactiveRequiresAllOutputs` (omit one stage output), `ErrDeclaredRequiresAllOutputs`, `ErrDeclaredRequiresDeclaredBy`, `ErrProvenanceMismatch` (type/detail mismatch).

- [ ] **Step 5: Add claim/completion rejection tests**

  ```go
  func TestClaim_RejectsRetroactive(t *testing.T) {
      // Create a retroactive spec (born at done), then try to claim it.
      _, err := claimSvc.Claim(ctx, connect.NewRequest(claimReq))
      require.ErrorIs(t, err, storage.ErrClaimRequiresAuthored)
  }

  func TestCompletion_RejectsDeclared(t *testing.T) {
      // ... similar ...
      require.ErrorIs(t, err, storage.ErrCompletionRequiresAuthored)
  }
  ```

- [ ] **Step 6: Commit**

  ```bash
  jj --no-pager commit -m "test(server): provenance creation paths + sentinel rejection

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/server/spec_handler_provenance_test.go
  ```

### Task 7.2: GetReady mixed-seed integration test

**Files:**
- Create: `internal/storage/postgres/graph_ready_provenance_test.go`

- [ ] **Step 1: Write the seed-and-assert test**

  ```go
  //go:build integration

  package postgres_test

  import (
      "context"
      "testing"

      "github.com/stretchr/testify/require"
      "github.com/specgraph/specgraph/internal/storage"
  )

  func TestGetReady_ProvenanceAndStageFilters(t *testing.T) {
      ctx := context.Background()
      store, cleanup := newTestStore(t)
      defer cleanup()

      // Seed 6 specs spanning the stage + provenance space.
      mustSeed(t, store, "a-authored-approved",    storage.SpecStageApproved,   storage.SpecProvenanceAuthored)
      mustSeed(t, store, "b-authored-spark",       storage.SpecStageSpark,      storage.SpecProvenanceAuthored)
      mustSeed(t, store, "c-authored-done",        storage.SpecStageDone,       storage.SpecProvenanceAuthored)
      mustSeed(t, store, "d-declared-done",        storage.SpecStageDone,       storage.SpecProvenanceDeclared)
      mustSeed(t, store, "e-authored-superseded",  storage.SpecStageSuperseded, storage.SpecProvenanceAuthored)
      mustSeed(t, store, "f-authored-abandoned",   storage.SpecStageAbandoned,  storage.SpecProvenanceAuthored)

      ready, err := store.GetReady(ctx)
      require.NoError(t, err)
      require.Len(t, ready, 1)
      require.Equal(t, "a-authored-approved", ready[0].Slug)
  }
  ```

  `newTestStore` and `mustSeed` follow the existing testcontainers pattern in `internal/storage/postgres/`.

- [ ] **Step 2: Run the test**

  ```bash
  go test -tags integration ./internal/storage/postgres/ -run TestGetReady_ProvenanceAndStageFilters -v
  ```

  Expected: PASS.

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "test(postgres): GetReady filters by stage and provenance

  Mixed-seed integration test asserting that only AUTHORED specs at
  stage=approved appear in ready. Covers superseded/abandoned exclusion,
  DECLARED exclusion, and pre-approved exclusion.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/postgres/graph_ready_provenance_test.go
  ```

### Task 7.3: Migration precondition test

**Files:**
- Create: `internal/storage/postgres/migration_007_test.go`

- [ ] **Step 1: Write the precondition test**

  ```go
  //go:build integration

  package postgres_test

  import (
      "context"
      "testing"

      "github.com/stretchr/testify/require"
  )

  func TestMigration007_RefusesNonEmptyTable(t *testing.T) {
      ctx := context.Background()
      // Roll back to migration 006, manually insert a row into specs, then re-run up to 007.
      db, cleanup := newTestDB(t)
      defer cleanup()

      // Migrate to 006 first.
      require.NoError(t, gooseUp(db, "006"))

      // Insert a row (specs table exists with lifecycle column at this point).
      _, err := db.Exec(ctx, "INSERT INTO specs (id, slug, project_slug, intent, stage, priority, complexity, lifecycle, notes, content_hash, version, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())",
          "spec-01TEST", "test-slug", "test-project", "test", "spark", "p2", "medium", "task", "", "deadbeef", 1)
      require.NoError(t, err)

      // Now try to migrate to 007 — should fail.
      err = gooseUp(db, "007")
      require.Error(t, err)
      require.Contains(t, err.Error(), "migration 007 refuses to run on a non-empty specs table")
  }
  ```

  `gooseUp(db, version)` helper applies migrations up to the given version using the `pressly/goose/v3` API. Reuse existing test infrastructure if available.

- [ ] **Step 2: Run the test**

  ```bash
  go test -tags integration ./internal/storage/postgres/ -run TestMigration007 -v
  ```

  Expected: PASS.

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "test(postgres): migration 007 refuses non-empty specs table

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" internal/storage/postgres/migration_007_test.go
  ```

### Task 7.4: Update existing tests that referenced lifecycle

**Files:**
- Modify: any `*_test.go` file currently referencing `Lifecycle`, `SpecLifecycle`, `LIFECYCLE_TASK`, `LIFECYCLE_LIVING`, etc.

- [ ] **Step 1: Find all stragglers**

  ```bash
  grep -rln 'Lifecycle\|LIFECYCLE_' --include='*_test.go' . | head -20
  ```

- [ ] **Step 2: Update each test file**

  For each file: replace lifecycle references with provenance equivalents. The mechanical mapping:

  - `SpecLifecycle_SPEC_LIFECYCLE_TASK` → `SpecProvenance_SPEC_PROVENANCE_AUTHORED`
  - `SpecLifecycle_SPEC_LIFECYCLE_LIVING` → `SpecProvenance_SPEC_PROVENANCE_DECLARED`
  - `storage.SpecLifecycleTask` → `storage.SpecProvenanceAuthored`
  - `storage.SpecLifecycleLiving` → `storage.SpecProvenanceDeclared`
  - `spec.Lifecycle` → `spec.Provenance`
  - `Lifecycle: storage.SpecLifecycleTask` → `Provenance: storage.SpecProvenanceAuthored`

  For tests that previously used `LIVING`, decide case-by-case whether the test intent maps to DECLARED (most likely, since LIVING described existing reality) or RETROACTIVE_FROM_PR (less common).

- [ ] **Step 3: Run the full test suite**

  ```bash
  task test
  ```

  Expected: all tests pass. If failures remain, address them one at a time.

- [ ] **Step 4: Commit**

  ```bash
  jj --no-pager commit -m "test: migrate lifecycle references to provenance

  Mechanical sweep across *_test.go files; mapping per plan Task 7.4.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" $(grep -rln 'Provenance' --include='*_test.go' . | head -30)
  ```

---

## Phase 8 — Documentation

### Task 8.1: CHANGELOG entry

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add a breaking-change entry**

  At the top of `CHANGELOG.md` (under the unreleased section if one exists, or a new `## [Unreleased]` header):

  ```markdown
  ## [Unreleased]

  ### Changed (BREAKING)

  - **Replaced `SpecLifecycle` (task/living) with `SpecProvenance`** (AUTHORED / RETROACTIVE_FROM_PR / DECLARED). Wire-break at proto field 10 plus a new `provenance_detail` oneof. Postgres column `lifecycle` removed; `provenance_type` and `provenance_detail` columns added. See `docs/decisions/ADR-006-spec-provenance-model.md`.
  - **`specgraph ready` now requires `stage=approved` AND `provenance=authored`**. Previously surfaced any spec at `stage <> done`, including mid-design and superseded/abandoned specs.
  - **`claim` and `report-completion` reject non-AUTHORED specs** with `ErrClaimRequiresAuthored` / `ErrCompletionRequiresAuthored`.
  ```

- [ ] **Step 2: Commit**

  ```bash
  jj --no-pager commit -m "docs(changelog): breaking-change entries for provenance model

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" CHANGELOG.md
  ```

### Task 8.2: Concept doc

**Files:**
- Modify: `site/docs/concepts/spec-graph.md`

- [ ] **Step 1: Find and update the lifecycle row**

  ```bash
  grep -n 'lifecycle\|task.*living' site/docs/concepts/spec-graph.md
  ```

  Replace the row that says `| **Lifecycle** | lifecycle (task / living), ... |` with a provenance row:

  ```markdown
  | **Provenance** | `provenance` (AUTHORED / RETROACTIVE_FROM_PR / DECLARED), `superseded_by`, `supersedes` |
  ```

- [ ] **Step 2: If a longer prose section discusses lifecycle, rewrite it**

  Substantively update any explanation of `living` specs to describe DECLARED specs, and add a short paragraph on RETROACTIVE_FROM_PR for retroactive imports. Reference the design doc for full detail.

- [ ] **Step 3: Commit**

  ```bash
  jj --no-pager commit -m "docs(concept): update spec-graph concept doc for provenance model

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" site/docs/concepts/spec-graph.md
  ```

### Task 8.3: Finalize ADR-006

**Files:**
- Modify: `docs/decisions/ADR-006-spec-provenance-model.md`

- [ ] **Step 1: Replace the stub with full content**

  ```markdown
  <!-- SPDX-License-Identifier: Apache-2.0 -->

  # ADR-006: Spec Provenance Model

  - **Status:** Accepted
  - **Date:** 2026-05-20
  - **Supersedes:** SpecLifecycle enum (task/living)

  ## Context

  SpecGraph specs previously carried a `lifecycle` field with values
  `task` and `living`. The intent was to distinguish one-time work
  (task) from ongoing contracts (living). The actual implementation
  encoded only the type tag — `GetReady` never consulted it, drift
  detection ignored it, and the conceptual boundary between
  task/living/done was vestigial after `done`.

  Adding a `retroactive` lifecycle value (an early proposal) would
  have created a third value behaviorally identical to LIVING,
  distinguished only by provenance.

  ## Decision

  Replace `SpecLifecycle` with `SpecProvenance` — an enum capturing
  **how a spec entered the graph**, not how the funnel should treat
  it after `done`. Values: AUTHORED, RETROACTIVE_FROM_PR, DECLARED.
  Per-variant structured payload via a `provenance_detail` oneof.

  Stage drives funnel behavior; provenance drives the
  forward-vs-imported axis. Done specs of any provenance share
  drift, dependency, and supersession semantics.

  See `docs/superpowers/specs/2026-05-20-spec-provenance-model-design.md`
  for full design rationale, alternatives considered, and adversarial
  review trail.

  ## Consequences

  - Wire-break at proto field 10 (pre-1.0, no production data).
  - `specgraph ready` semantics tightened: only AUTHORED specs at
    `stage=approved` with no active claim.
  - `claim` and `report-completion` reject non-AUTHORED specs.
  - Drift detection unified on `stage=done`.
  - Provenance is immutable through amend; supersede creates a fresh
    spec with fresh provenance.
  - Adding new provenance values (e.g. `IMPORTED_FROM_BACKUP`) in
    the future is wire-compatible per proto enum evolution rules.
  ```

- [ ] **Step 2: Commit**

  ```bash
  jj --no-pager commit -m "docs(adr): finalize ADR-006 spec provenance model

  Status moves from Proposed to Accepted; full Context/Decision/Consequences.

  Signed-off-by: Sean Brandt <4678+seanb4t@users.noreply.github.com>" docs/decisions/ADR-006-spec-provenance-model.md
  ```

---

## Phase 9 — Final verification + PR

### Task 9.1: Run task check

- [ ] **Step 1: Verify all unrelated drift restored**

  ```bash
  jj --no-pager restore .claude/settings.json .cursor/mcp.json .mcp.json opencode.json
  jj --no-pager status
  ```

  Expected: working copy clean (or only unrelated files restored to canonical form).

- [ ] **Step 2: Run task check**

  ```bash
  task check 2>&1 | tail -20
  ```

  Expected: `Success: No issues found in N files` for markdown lint; all Go tests pass; no FAIL lines. If anything fails, fix it and recommit. Squash the fix into the relevant phase commit using `jj squash --into <change-id>` to keep history clean.

### Task 9.2: Run task pr-prep (optional but recommended)

- [ ] **Step 1: Run pr-prep**

  ```bash
  task pr-prep 2>&1 | tail -20
  ```

  Expected: all of `task check` + integration + e2e tests pass. Requires Docker (per CLAUDE.md). If Docker is unavailable, skip and note in PR description.

### Task 9.3: Set bookmark and push

- [ ] **Step 1: Confirm commit stack**

  ```bash
  jj --no-pager log -r 'ancestors(@-, 30)' --no-graph | head -60
  ```

  Expected: ordered list of commits from Task 1.1 through Task 8.3, all with proper conventional-commits + DCO sign-off, atop the main branch.

- [ ] **Step 2: Create the bookmark**

  ```bash
  jj --no-pager bookmark create feat/spec-provenance-model -r @-
  ```

- [ ] **Step 3: Push**

  ```bash
  jj --no-pager git push --bookmark feat/spec-provenance-model
  ```

  Expected: bookmark pushed; GitHub returns a PR-create URL.

### Task 9.4: Open the PR

- [ ] **Step 1: Open the PR from the main colocated checkout**

  ```bash
  (cd /Users/SeBrandt/Code/github.com/specgraph && gh pr create \
    --head feat/spec-provenance-model \
    --base main \
    --title "feat!: replace SpecLifecycle with SpecProvenance model" \
    --body "$(cat <<'EOF'
  ## Summary

  Implements the spec-provenance model design from PR #953.

  - Wire-break at proto field 10 (lifecycle → provenance_type) plus a new provenance_detail oneof at fields 22-24
  - SpecLifecycle (task/living) replaced with SpecProvenance (AUTHORED / RETROACTIVE_FROM_PR / DECLARED)
  - Postgres migration 007 drops lifecycle column, adds provenance_type + provenance_detail (with precondition guard)
  - `specgraph ready` rewritten: stage=approved AND provenance=authored AND no active claim AND deps satisfied
  - `claim` and `report-completion` gated on provenance=AUTHORED (eight new sentinel errors)
  - Extended CreateSpec accepts provenance + all four stage outputs for born-at-done flows
  - Render, CLI, MCP, linter, export all updated
  - ADR-006 records the model decision

  ## Test plan

  - [x] `task check` passes
  - [ ] `task pr-prep` (integration + e2e) — see CI
  - [x] Provenance creation-path tests cover AUTHORED happy-path, RETROACTIVE born-at-done, DECLARED born-at-done, and all eight sentinel rejection paths
  - [x] GetReady mixed-seed test asserts only AUTHORED-at-approved appears
  - [x] Migration 007 precondition test asserts refusal on non-empty table

  ## Breaking changes

  See CHANGELOG.md. Wire-break is intentional (pre-1.0, no production data per project status).
  EOF
  )")
  ```

  Expected: gh returns a PR URL.

- [ ] **Step 2: Verify CI on the PR**

  ```bash
  (cd /Users/SeBrandt/Code/github.com/specgraph && gh pr checks)
  ```

  Expected: CI workflows queued; address any failures by recommitting on the branch and re-pushing.

---

## Self-Review

Run through this list with fresh eyes after writing the plan.

1. **Spec coverage:** Every section of the design doc is covered by at least one task:
   - Proto changes (Section "The Model" → Task 1.3, 4.1)
   - Storage domain types (Section "Storage-side domain type" → Task 2.1)
   - JSONB envelope (Section "JSONB envelope" → Task 3.2)
   - Stage progressions per provenance (Section → Tasks 4.1, 4.2)
   - GetReady semantics (Section → Task 3.3)
   - Drift behavior (Section → unchanged; verified by existing drift tests + Task 7.2)
   - Render output (Section → Task 5.4)
   - Migration footprint (Section → Tasks 3.1, 6.1, 6.2)
   - Sentinel errors (Section → Task 1.1)
   - ADR-006, CHANGELOG, concept doc → Tasks 8.1, 8.2, 8.3
   - Spec.stage proto comment fix → Task 1.3 Step 3
   - Migration precondition → Task 3.1 + Task 7.3

2. **Placeholder scan:** every code step shows actual code; no "TBD" or "handle appropriately" placeholders. Three steps (Task 4.1 Step 4, Task 5.2 Step 2, Task 6.1 Step 1) reference patterns in existing code and instruct the implementer to match them — this is acceptable given the surface area, but the implementer must read those existing files before coding.

3. **Type consistency:** `SpecProvenance`, `SpecProvenanceType`, `SpecProvenanceDetail`, `RetroactivePRProvenance`, `DeclaredProvenance` are used consistently across all phases. Proto enum values (`SPEC_PROVENANCE_*`) and domain constants (`SpecProvenanceAuthored` etc.) match. Sentinel error names match the design's enumeration.

4. **Commit-by-task tracing:** every task ends with a `jj commit` step naming exactly the files touched, conventional-commits typed, and DCO-signed. Phase 1.3 carries the breaking-change `!` per the convention.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-20-spec-provenance-model.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration. Best for plans of this size (24 tasks) because each task is self-contained and the subagent doesn't accumulate context from earlier ones.

2. **Inline Execution** — execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints. Easier if you want to handle minor issues in real time without subagent round-trips.

**Which approach?**
