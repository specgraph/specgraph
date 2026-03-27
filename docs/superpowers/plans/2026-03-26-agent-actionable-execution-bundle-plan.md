# Agent-Actionable Execution Bundle Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the thin YAML execution bundle with a rich Markdown document that gives agents everything they need to implement a spec and communicate status back.

**Architecture:** Extend `GenerateBundle` storage method to fetch claim state and dependencies with drift, then replace `renderBundleYAML` with `renderBundleMarkdown` using `text/template`. Reserve the old proto field and add a new one.

**Tech Stack:** Go, text/template, protobuf, ConnectRPC, Memgraph Cypher

**Spec:** `docs/superpowers/specs/2026-03-26-agent-actionable-execution-bundle-design.md`

---

## Chunk 1: Domain Types and Storage Layer

### Task 1: Add `DependencyInfo` type and extend `Bundle` struct

**Files:**

- Modify: `internal/storage/execution_domain.go:68-75`
- Modify: `internal/storage/graph.go:69-73`

- [ ] **Step 1: Add `DependencyInfo` to `execution_domain.go`**

Add the new type after the `Bundle` struct. Also add the `Claim` and `Dependencies` fields to `Bundle`:

```go
// Bundle is a self-contained package for an executing agent.
type Bundle struct {
	Version      int32
	Spec         *Spec
	Decisions    []*Decision
	Bootstrap    string
	Callbacks    *CallbackConfig
	Claim        *Claim
	Dependencies []DependencyInfo
}

// DependencyInfo captures an upstream spec's status and drift state for the bundle.
type DependencyInfo struct {
	Slug    string
	Stage   SpecStage
	Drifted bool
	Note    string
}
```

- [ ] **Step 2: Add `UpstreamContentHash` to `DependencyRef` in `graph.go`**

```go
// DependencyRef is a dependency with edge metadata for drift detection.
type DependencyRef struct {
	NodeRef
	ContentHashAtLink   string
	UpstreamContentHash string
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./internal/storage/...`
Expected: PASS (no consumers of the new fields yet)

- [ ] **Step 4: Commit**

```text
feat(storage): add DependencyInfo type and extend Bundle/DependencyRef structs (spgr-755)
```

### Task 2: Extend `GetDependenciesWithEdgeData` to return upstream content hash

**Files:**

- Modify: `internal/storage/memgraph/graph.go:163-195`
- Modify: `internal/storage/memgraph/lifecycle_test.go` (existing drift tests)

- [ ] **Step 1: Write the failing test**

In an existing drift test in `lifecycle_test.go`, add an assertion that the returned `DependencyRef` has `UpstreamContentHash` populated. Find one of the existing `GetDependenciesWithEdgeData` test calls (e.g., line ~373) and add:

```go
require.NotEmpty(t, deps[0].UpstreamContentHash, "upstream content hash should be populated")
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/memgraph/ -run TestLifecycle -count=1 -tags integration`
Expected: FAIL — `UpstreamContentHash` is empty string (zero value)

- [ ] **Step 3: Update the Cypher query and record parsing**

In `GetDependenciesWithEdgeData` (`graph.go:163`), extend the RETURN clause to include `upstream_content_hash` and parse it:

```go
func (s *Store) GetDependenciesWithEdgeData(ctx context.Context, slug string) ([]storage.DependencyRef, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(a {slug: $slug})-[dep:DEPENDS_ON]->(n)
		RETURN n.id AS id, n.slug AS slug, labels(n)[0] AS label,
		       COALESCE(n.stage, n.status, "") AS stage,
		       COALESCE(dep.content_hash_at_link, "") AS content_hash_at_link,
		       COALESCE(n.content_hash, "") AS upstream_content_hash
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: get dependencies with edge data: %w", err)
	}

	refs := make([]storage.DependencyRef, 0, len(records))
	for _, rec := range records {
		id, _ := rec.Get("id")
		sl, _ := rec.Get("slug")
		label, _ := rec.Get("label")
		stage, _ := rec.Get("stage")
		hash, _ := rec.Get("content_hash_at_link")
		upHash, _ := rec.Get("upstream_content_hash")
		refs = append(refs, storage.DependencyRef{
			NodeRef: storage.NodeRef{
				ID:    stringVal(id),
				Slug:  stringVal(sl),
				Label: storage.NodeLabel(stringVal(label)),
				Stage: stringVal(stage),
			},
			ContentHashAtLink:   stringVal(hash),
			UpstreamContentHash: stringVal(upHash),
		})
	}
	return refs, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/storage/memgraph/ -run TestLifecycle -count=1 -tags integration`
Expected: PASS

- [ ] **Step 5: Commit**

```text
feat(storage): return upstream content hash from GetDependenciesWithEdgeData (spgr-755)
```

### Task 3: Extend `GenerateBundle` to fetch claim and dependencies

**Files:**

- Modify: `internal/storage/memgraph/execution.go:17-37`
- Test: `internal/storage/memgraph/execution_test.go`

- [ ] **Step 1: Write failing test — bundle includes claim when claimed**

Add a new test in `execution_test.go`. **Important:** This file has `//go:build integration` at the top and is in `package memgraph_test`. The new tests must follow that convention.

```go
t.Run("GenerateBundle_IncludesClaim", func(t *testing.T) {
	clearDatabase(t)
	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)

	_, err = store.CreateSpec(ctx, "claimed-spec", "test intent", "p1", "medium")
	require.NoError(t, err)
	_, err = store.UpdateSpec(ctx, "claimed-spec", nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)

	_, err = store.ClaimSpec(ctx, "claimed-spec", "agent-1", 15*time.Minute)
	require.NoError(t, err)

	bundle, err := store.GenerateBundle(ctx, "claimed-spec")
	require.NoError(t, err)
	require.NotNil(t, bundle.Claim)
	require.Equal(t, "agent-1", bundle.Claim.Agent)
})
```

Note: `ptr("approved")` is a test helper that returns `*string`. The existing execution tests use `store.UpdateSpec(ctx, slug, nil, ptr("approved"), nil, nil, nil)` to skip directly to approved stage (see `execution_test.go:59`).

- [ ] **Step 2: Write failing test — bundle includes dependencies with drift**

```go
t.Run("GenerateBundle_IncludesDependencies", func(t *testing.T) {
	clearDatabase(t)
	ctx := context.Background()
	store, err := newStore(ctx, boltURI)
	require.NoError(t, err)

	// Create upstream and downstream specs.
	_, err = store.CreateSpec(ctx, "upstream-spec", "upstream", "p1", "medium")
	require.NoError(t, err)
	_, err = store.CreateSpec(ctx, "downstream-spec", "downstream", "p1", "medium")
	require.NoError(t, err)

	// Add DEPENDS_ON edge (downstream depends on upstream).
	err = store.AddEdge(ctx, "downstream-spec", "upstream-spec", storage.EdgeTypeDependsOn)
	require.NoError(t, err)

	// Advance downstream to approved.
	_, err = store.UpdateSpec(ctx, "downstream-spec", nil, ptr("approved"), nil, nil, nil)
	require.NoError(t, err)

	bundle, err := store.GenerateBundle(ctx, "downstream-spec")
	require.NoError(t, err)
	require.Len(t, bundle.Dependencies, 1)
	require.Equal(t, "upstream-spec", bundle.Dependencies[0].Slug)
})
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/storage/memgraph/ -run TestExecution -count=1 -tags integration`
Expected: FAIL — `Claim` is nil, `Dependencies` is empty

- [ ] **Step 4: Implement claim and dependency fetching in GenerateBundle**

Update `GenerateBundle` in `execution.go`:

```go
func (s *Store) GenerateBundle(ctx context.Context, slug string) (*storage.Bundle, error) {
	spec, err := s.GetSpec(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle: %w", err)
	}

	if spec.Stage != storage.SpecStageApproved && string(spec.Stage) != "in_progress" {
		return nil, fmt.Errorf("memgraph: generate bundle for %q: %w", slug, storage.ErrSpecNotApproved)
	}

	decisions, err := s.fetchLinkedDecisions(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle decisions: %w", err)
	}

	claim, err := s.fetchActiveClaim(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle claim: %w", err)
	}

	deps, err := s.fetchBundleDependencies(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("memgraph: generate bundle dependencies: %w", err)
	}

	return &storage.Bundle{
		Version:      2,
		Spec:         spec,
		Decisions:    decisions,
		Claim:        claim,
		Dependencies: deps,
	}, nil
}

// fetchActiveClaim returns the active claim for a spec, or nil if unclaimed.
func (s *Store) fetchActiveClaim(ctx context.Context, slug string) (*storage.Claim, error) {
	nowStr := s.now()
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		OPTIONAL MATCH (s)-[r:CLAIMED_BY]->(a)
		WHERE r.lease_expires >= $now
		RETURN r.agent AS agent, r.claimed_at AS claimed_at, r.lease_expires AS lease_expires
	`
	records, err := s.executeQuery(ctx, query, mergeParams(s.projectParam(), map[string]any{
		"slug": slug,
		"now":  nowStr,
	}))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	// OPTIONAL MATCH returns a row with nulls when there's no claim.
	agentVal, _ := records[0].Get("agent")
	if agentVal == nil {
		return nil, nil
	}

	return recordToClaim(slug, records[0])
}

// fetchBundleDependencies returns dependency info with drift state for the bundle.
func (s *Store) fetchBundleDependencies(ctx context.Context, slug string) ([]storage.DependencyInfo, error) {
	refs, err := s.GetDependenciesWithEdgeData(ctx, slug)
	if err != nil {
		return nil, err
	}
	if len(refs) == 0 {
		return nil, nil
	}

	deps := make([]storage.DependencyInfo, 0, len(refs))
	for _, ref := range refs {
		drifted := ref.ContentHashAtLink == "" || ref.ContentHashAtLink != ref.UpstreamContentHash
		var note string
		if drifted {
			if ref.ContentHashAtLink == "" {
				note = "dependency not yet baselined"
			} else {
				note = "content changed since baseline"
			}
		}
		deps = append(deps, storage.DependencyInfo{
			Slug:    ref.Slug,
			Stage:   storage.SpecStage(ref.Stage),
			Drifted: drifted,
			Note:    note,
		})
	}
	return deps, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/storage/memgraph/ -run TestExecution -count=1 -tags integration`
Expected: PASS

- [ ] **Step 6: Run full unit test suite to check nothing broke**

Run: `go test ./internal/storage/...`
Expected: PASS

- [ ] **Step 7: Commit**

```text
feat(storage): fetch claim state and dependencies in GenerateBundle (spgr-755)
```

## Chunk 2: Proto, Renderer, and Handler

### Task 4: Update proto — reserve `bundle_yaml`, add `bundle_content`

**Files:**

- Modify: `proto/specgraph/v1/execution.proto:33-40`

- [ ] **Step 1: Update the proto file**

Change the `Bundle` message in `execution.proto`:

```protobuf
message Bundle {
  int32 version = 1;
  Spec spec = 2;
  repeated Decision decisions = 3;
  string bootstrap = 4;
  CallbackConfig callbacks = 5;
  reserved 6;
  reserved "bundle_yaml";
  string bundle_content = 7;
}
```

- [ ] **Step 2: Regenerate proto code**

Run: `task proto`
Expected: Generates updated `.pb.go`, `.connect.go`, and TypeScript files. The `BundleYaml` field disappears; `BundleContent` appears.

- [ ] **Step 3: Fix build errors from removed `BundleYaml` field**

The following files reference `BundleYaml` and must be updated:

1. `internal/server/execution_handler.go:67` — change `pb.BundleYaml = renderBundleYAML(b)` to `pb.BundleContent = renderBundleYAML(b)` (temporarily; the renderer changes in Task 5)
2. `internal/server/execution_handler_test.go:185-186` — change `GetBundleYaml()` to `GetBundleContent()` in the two assertions that check the bundle YAML is non-empty and contains the spec slug
3. `cmd/specgraph/bundle.go:51` — change `GetBundleYaml()` to `GetBundleContent()`
4. `cmd/specgraph/bundle_test.go:27` — change `BundleYaml: "spec:\n  slug: test\n"` to `BundleContent: "spec:\n  slug: test\n"`

**Note:** Between Task 4 and Task 5, `renderBundleYAML` still writes YAML content to the `BundleContent` field. This is intentional — it keeps the build green at every commit. Task 5+6 replace the renderer.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Run tests**

Run: `go test ./cmd/specgraph/ -run TestRunBundle -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```text
feat(proto): reserve bundle_yaml, add bundle_content field (spgr-755)
```

### Task 5: Implement `renderBundleMarkdown`

**Files:**

- Create: `internal/server/execution_render.go`
- Test: `internal/server/execution_render_test.go`

The renderer is its own file since the template + helpers will be ~150+ lines.

- [ ] **Step 1: Write failing tests for the renderer**

Create `internal/server/execution_render_test.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"strings"
	"testing"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderBundleMarkdown_FullBundle(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug:        "my-feature",
			Intent:      "Add drift detection",
			Stage:       storage.SpecStageApproved,
			Priority:    storage.SpecPriorityP1,
			ContentHash: "abcd1234",
			ShapeOutput: &storage.ShapeOutput{
				ScopeIn:        []string{"detection loop", "CLI reporting"},
				ScopeOut:       []string{"UI dashboard"},
				ChosenApproach: "Event-driven with hash comparison",
				Risks:          []string{"Hash collisions"},
			},
			SpecifyOutput: &storage.SpecifyOutput{
				VerifyCriteria: []storage.VerifyCriterion{
					{Category: "functional", Description: "Drift detected within 5s"},
				},
				Invariants: []string{"Content hash is never empty"},
				Interfaces: []storage.InterfaceSection{
					{Name: "DriftDetector", Body: "func Detect(ctx, slug) ([]Change, error)"},
				},
				Touches: []storage.FileTouch{
					{Path: "internal/drift/detector.go", Purpose: "Core engine", ChangeType: "create"},
				},
			},
			DecomposeOutput: &storage.DecomposeOutput{
				Strategy: storage.StrategyVerticalSlice,
				Slices: []storage.DecomposeSlice{
					{ID: "slice-1", Intent: "Core detection loop", Verify: []string{"detector returns changes"}, Touches: []string{"internal/drift/detector.go"}},
				},
			},
		},
		Decisions: []*storage.Decision{
			{Slug: "adr-007", Title: "Use content hashing", Status: storage.DecisionStatusAccepted, Body: "Compare Murmur3-128 hashes", Rationale: "Timestamp comparison fails"},
		},
		Claim: &storage.Claim{
			Agent:        "agent-1",
			LeaseExpires: time.Date(2026, 3, 26, 15, 0, 0, 0, time.UTC),
		},
		Dependencies: []storage.DependencyInfo{
			{Slug: "auth-middleware", Stage: storage.SpecStageDone, Drifted: false},
			{Slug: "db-schema", Stage: storage.SpecStageInProgress, Drifted: true, Note: "content changed since baseline"},
		},
	}

	result := renderBundleMarkdown(b)

	// Frontmatter
	assert.True(t, strings.HasPrefix(result, "---\n"), "should start with frontmatter")
	assert.Contains(t, result, "version: 2")
	assert.Contains(t, result, "slug: my-feature")
	assert.Contains(t, result, "stage: approved")
	assert.Contains(t, result, "priority: p1")
	assert.Contains(t, result, "content_hash: abcd1234")

	// What to Build
	assert.Contains(t, result, "## What to Build")
	assert.Contains(t, result, "**Intent:** Add drift detection")
	assert.Contains(t, result, "- **In:** detection loop")
	assert.Contains(t, result, "- **Out:** UI dashboard")
	assert.Contains(t, result, "- [ ] functional: Drift detected within 5s")
	assert.Contains(t, result, "- Content hash is never empty")
	assert.Contains(t, result, "**DriftDetector**")
	assert.Contains(t, result, "| `internal/drift/detector.go` | Core engine | create |")

	// Work Slices
	assert.Contains(t, result, "## Work Slices")
	assert.Contains(t, result, "Strategy: `vertical_slice`")
	assert.Contains(t, result, "### Slice 1: Core detection loop")

	// How to Work
	assert.Contains(t, result, "## How to Work")
	assert.Contains(t, result, "specgraph claim my-feature --agent <your-id>")
	assert.Contains(t, result, "specgraph report-progress my-feature")
	assert.Contains(t, result, "**Current claim:** agent-1")

	// Dependencies
	assert.Contains(t, result, "| auth-middleware | done | no |")
	assert.Contains(t, result, "| db-schema | in_progress | **yes**")

	// Decisions
	assert.Contains(t, result, "## Decisions")
	assert.Contains(t, result, "### adr-007: Use content hashing")
	assert.Contains(t, result, "**Status:** accepted")
	assert.Contains(t, result, "**Decision:** Compare Murmur3-128 hashes")
	assert.Contains(t, result, "**Rationale:** Timestamp comparison fails")

	// Design Context
	assert.Contains(t, result, "## Design Context")
	assert.Contains(t, result, "**Chosen approach:** Event-driven with hash comparison")
	assert.Contains(t, result, "- Hash collisions")

	// Constitution pointer
	assert.Contains(t, result, "specgraph prime my-feature")
}

func TestRenderBundleMarkdown_Minimal(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug:        "minimal",
			Intent:      "Do a thing",
			Stage:       storage.SpecStageApproved,
			Priority:    storage.SpecPriorityP2,
			ContentHash: "aaaa",
		},
	}

	result := renderBundleMarkdown(b)

	assert.Contains(t, result, "## What to Build")
	assert.Contains(t, result, "**Intent:** Do a thing")
	assert.NotContains(t, result, "## Work Slices")
	assert.NotContains(t, result, "## Decisions")
	assert.NotContains(t, result, "## Design Context")
	assert.Contains(t, result, "## How to Work")
	assert.Contains(t, result, "Current claim:** unclaimed")
	assert.NotContains(t, result, "### Dependencies")
}

func TestRenderBundleMarkdown_SingleUnitWithSlice(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "single", Intent: "One thing", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "bbbb",
			DecomposeOutput: &storage.DecomposeOutput{
				Strategy: storage.StrategySingleUnit,
				Slices:   []storage.DecomposeSlice{{ID: "s1", Intent: "The whole thing"}},
			},
		},
	}

	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "## Work Slices")
	assert.Contains(t, result, "Strategy: `single_unit`")
}

func TestRenderBundleMarkdown_DecisionStatusDisplay(t *testing.T) {
	b := &storage.Bundle{
		Version: 2,
		Spec: &storage.Spec{
			Slug: "dec-test", Intent: "test", Stage: storage.SpecStageApproved,
			Priority: storage.SpecPriorityP2, ContentHash: "cccc",
		},
		Decisions: []*storage.Decision{
			{Slug: "d1", Title: "Proposed", Status: storage.DecisionStatusProposed, Body: "body", Rationale: "why"},
			{Slug: "d2", Title: "Superseded", Status: storage.DecisionStatusSuperseded, Body: "body2", Rationale: "why2"},
		},
	}

	result := renderBundleMarkdown(b)
	assert.Contains(t, result, "**Status:** proposed")
	assert.Contains(t, result, "**Status:** superseded")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/server/ -run TestRenderBundleMarkdown -count=1`
Expected: FAIL — `renderBundleMarkdown` does not exist

- [ ] **Step 3: Implement the renderer**

Create `internal/server/execution_render.go`:

```go
// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"strings"
	"text/template"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// decisionDisplayStatus converts a DecisionStatus domain value to a display string.
// e.g., "DECISION_STATUS_ACCEPTED" → "accepted"
func decisionDisplayStatus(s storage.DecisionStatus) string {
	raw := string(s)
	const prefix = "DECISION_STATUS_"
	if strings.HasPrefix(raw, prefix) {
		return strings.ToLower(raw[len(prefix):])
	}
	return strings.ToLower(raw)
}

var bundleFuncs = template.FuncMap{
	"decisionStatus": decisionDisplayStatus,
	"now":            func() string { return time.Now().UTC().Format(time.RFC3339) },
	"add1":           func(i int) int { return i + 1 },
	"join":           strings.Join,
}

var bundleTemplate = template.Must(template.New("bundle").Funcs(bundleFuncs).Parse(bundleTemplateText))

const bundleTemplateText = `---
version: {{ .Version }}
slug: {{ .Spec.Slug }}
stage: {{ .Spec.Stage }}
priority: {{ .Spec.Priority }}
content_hash: {{ .Spec.ContentHash }}
generated_at: {{ now }}
---

# Execution Bundle: {{ .Spec.Slug }}

## What to Build

**Intent:** {{ .Spec.Intent }}
{{ if and .Spec.ShapeOutput (or .Spec.ShapeOutput.ScopeIn .Spec.ShapeOutput.ScopeOut) }}
### Scope
{{ range .Spec.ShapeOutput.ScopeIn }}- **In:** {{ . }}
{{ end }}{{ range .Spec.ShapeOutput.ScopeOut }}- **Out:** {{ . }}
{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.VerifyCriteria }}
### Acceptance Criteria
{{ range .Spec.SpecifyOutput.VerifyCriteria }}- [ ] {{ .Category }}: {{ .Description }}
{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Invariants }}
### Invariants
{{ range .Spec.SpecifyOutput.Invariants }}- {{ . }}
{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Interfaces }}
### Interfaces
{{ range .Spec.SpecifyOutput.Interfaces }}**{{ .Name }}**
{{ .Body }}

{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Touches }}
### File Touches
| Path | Purpose | Change |
|------|---------|--------|
{{ range .Spec.SpecifyOutput.Touches }}| ` + "`{{ .Path }}`" + ` | {{ .Purpose }} | {{ .ChangeType }} |
{{ end }}{{ end }}{{ if .Spec.DecomposeOutput }}
## Work Slices

Strategy: ` + "`{{ .Spec.DecomposeOutput.Strategy }}`" + `
{{ range $i, $s := .Spec.DecomposeOutput.Slices }}
### Slice {{ add1 $i }}: {{ $s.Intent }}
{{ range $s.Verify }}- **Verify:** {{ . }}
{{ end }}{{ range $s.Touches }}- **Touches:** ` + "`{{ . }}`" + `
{{ end }}{{ if $s.DependsOn }}- **Depends on:** {{ join $s.DependsOn ", " }}{{ else }}- **Depends on:** none{{ end }}
{{ end }}{{ end }}
## How to Work

### Claim & Report

` + "```" + `
specgraph claim {{ .Spec.Slug }} --agent <your-id>
specgraph report-progress {{ .Spec.Slug }} --agent <your-id> --message "..."
specgraph report-blocker {{ .Spec.Slug }} --agent <your-id> --description "..."
specgraph report-completion {{ .Spec.Slug }} --agent <your-id>
` + "```" + `

All flags shown above are required for their respective commands.

{{ if .Claim }}**Current claim:** {{ .Claim.Agent }} (expires {{ .Claim.LeaseExpires.Format "2006-01-02T15:04:05Z07:00" }}){{ else }}**Current claim:** unclaimed{{ end }}
{{ if .Dependencies }}
### Dependencies
| Upstream | Status | Drifted |
|----------|--------|---------|
{{ range .Dependencies }}| {{ .Slug }} | {{ .Stage }} | {{ if .Drifted }}**yes** — {{ .Note }}{{ else }}no{{ end }} |
{{ end }}{{ end }}
### Constitution & Project Context
Run ` + "`specgraph prime {{ .Spec.Slug }}`" + ` for constitution constraints, coding conventions,
and project context.
{{ if .Decisions }}
## Decisions
{{ range .Decisions }}
### {{ .Slug }}: {{ .Title }}
**Status:** {{ decisionStatus .Status }}
**Decision:** {{ .Body }}
**Rationale:** {{ .Rationale }}
{{ end }}{{ end }}{{ if and .Spec.ShapeOutput .Spec.ShapeOutput.ChosenApproach }}
## Design Context

**Chosen approach:** {{ .Spec.ShapeOutput.ChosenApproach }}
{{ if .Spec.ShapeOutput.Risks }}**Risks:**
{{ range .Spec.ShapeOutput.Risks }}- {{ . }}
{{ end }}{{ end }}{{ end }}`

// renderBundleMarkdown produces a Markdown document from a Bundle.
func renderBundleMarkdown(b *storage.Bundle) string {
	var buf bytes.Buffer
	if err := bundleTemplate.Execute(&buf, b); err != nil {
		return "# Error rendering bundle\n\n" + err.Error() + "\n"
	}
	return buf.String()
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/server/ -run TestRenderBundleMarkdown -count=1`
Expected: PASS

Iterate on the template if assertions fail — adjust whitespace, conditionals. The tests check `Contains`, not exact matches, so minor whitespace differences are OK.

- [ ] **Step 5: Commit**

```text
feat(server): implement renderBundleMarkdown with text/template (spgr-755)
```

### Task 6: Wire renderer into handler and update CLI

**Files:**

- Modify: `internal/server/execution_handler.go:67`
- Modify: `cmd/specgraph/bundle.go:51`

- [ ] **Step 1: Update handler to use new renderer**

In `execution_handler.go`, change line 67:

```go
// Before:
pb.BundleContent = renderBundleYAML(b)

// After:
pb.BundleContent = renderBundleMarkdown(b)
```

- [ ] **Step 2: Delete `renderBundleYAML` function**

Remove the entire `renderBundleYAML` function (lines 233-279 of `execution_handler.go`) and its local types. It is no longer called.

Also remove the `"gopkg.in/yaml.v3"` import — it is only used by `renderBundleYAML` and will cause a build error if left behind.

- [ ] **Step 3: Update CLI to read `BundleContent`**

In `cmd/specgraph/bundle.go`, the `GetBundleContent()` call was already done in Task 4 Step 3. Verify it reads correctly.

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Run existing handler tests**

Run: `go test ./internal/server/ -run TestExecutionHandler -count=1`
Expected: PASS — existing tests should still pass (they check error codes, not bundle content)

- [ ] **Step 6: Run CLI tests**

Run: `go test ./cmd/specgraph/ -run TestRunBundle -count=1`
Expected: PASS — update `bundle_test.go` fake handler to return `BundleContent` with markdown content if not already done in Task 4

- [ ] **Step 7: Commit**

```text
feat(server): wire renderBundleMarkdown into GenerateBundle handler (spgr-755)
```

## Chunk 3: CLI Tests, E2E, and Cleanup

### Task 7: Update CLI test fake to return markdown

**Files:**

- Modify: `cmd/specgraph/bundle_test.go:21-29`

- [ ] **Step 1: Update fake handler and file output test**

Update `fakeBundleHandler` to return markdown content:

```go
func (fakeBundleHandler) GenerateBundle(_ context.Context, _ *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	return connect.NewResponse(&specv1.GenerateBundleResponse{
		Bundle: &specv1.Bundle{BundleContent: "---\nversion: 2\nslug: test\n---\n\n# Execution Bundle: test\n"},
	}), nil
}
```

Update the file output test assertion (line 67):

```go
assert.Contains(t, string(data), "# Execution Bundle: test")
```

Update the file path in the test to use `.md` extension:

```go
outPath := filepath.Join(dir, "bundle.md")
```

- [ ] **Step 2: Run CLI tests**

Run: `go test ./cmd/specgraph/ -run TestRunBundle -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```text
test(cli): update bundle test fakes for markdown output (spgr-755)
```

### Task 8: Update E2E tests

**Files:**

- Modify: `e2e/api/pipeline_test.go:199-208`
- Modify: `e2e/api/constitution_pipeline_test.go:162-166`

- [ ] **Step 1: Update pipeline E2E assertion**

In `pipeline_test.go`, update the bundle generation test to check version 2 and that `BundleContent` is present:

```go
It("generates an execution bundle", func() {
	resp, err := executionClient.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
		Slug:     pipelineSlug,
		Endpoint: serverInfo.BaseURL,
	}))
	Expect(err).NotTo(HaveOccurred())
	Expect(resp.Msg.GetBundle().GetSpec()).NotTo(BeNil())
	Expect(resp.Msg.GetBundle().GetSpec().GetSlug()).To(Equal(pipelineSlug))
	Expect(resp.Msg.GetBundle().GetVersion()).To(Equal(int32(2)))
	Expect(resp.Msg.GetBundle().GetBundleContent()).To(HavePrefix("---\n"))
})
```

- [ ] **Step 2: Update constitution pipeline E2E assertion**

In `constitution_pipeline_test.go`, add a similar `BundleContent` check to the existing bundle test.

- [ ] **Step 3: Run E2E tests**

Run: `task test:e2e` (or `go test -tags e2e ./e2e/api/ -count=1`)
Expected: PASS

- [ ] **Step 4: Commit**

```text
test(e2e): assert bundle v2 markdown output (spgr-755)
```

### Task 9: Run full quality gate

- [ ] **Step 1: Run `task check`**

Run: `task check`
Expected: PASS — fmt, lint, build, unit tests all green

- [ ] **Step 2: Run `task pr-prep`** (if Docker available)

Run: `task pr-prep`
Expected: PASS — includes integration and e2e tests

- [ ] **Step 3: Final commit if any fixes needed**

Fix any lint issues (license headers on new files, package comments, etc.) and commit:

```text
chore: fix lint issues from bundle v2 changes (spgr-755)
```

### Task 10: Close bead

- [ ] **Step 1: Close the bead**

Run: `bd close spgr-755 --reason="Execution bundle redesigned as markdown launchpad with full authoring outputs, claim instructions, dependency drift state, and decision rationale"`
