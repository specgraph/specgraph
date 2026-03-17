# Full Pipeline E2E Test Suite — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a 3-tier E2E test suite that proves the full SpecGraph pipeline works with real Memgraph: authoring funnel → claim → execution → completion, plus project isolation, constitution integration, and lifecycle transitions.

**Architecture:** Focused Ginkgo/Gomega test suites organized by concern. Protocol tests use ConnectRPC against an in-process server with testcontainer Memgraph. CLI tests shell out to the `specgraph` binary. Agent tests invoke `claude --print` with the plugin.

**Tech Stack:** Go, Ginkgo v2, Gomega, testcontainers-go, ConnectRPC, Memgraph

**Design Doc:** `docs/plans/2026-03-17-full-pipeline-e2e-design.md`

---

## Chunk 1: Infrastructure & Protocol Pipeline Test

### Task 1: Add Missing E2E Client Helpers

**Files:**

- Modify: `e2e/api/helpers_test.go`

- [ ] **Step 1: Add newExecutionClient and projectClientFor helpers**

```go
// Add to e2e/api/helpers_test.go:

func newExecutionClient() specgraphv1connect.ExecutionServiceClient {
	return specgraphv1connect.NewExecutionServiceClient(projectClient(), serverInfo.BaseURL)
}

func newSyncClient() specgraphv1connect.SyncServiceClient {
	return specgraphv1connect.NewSyncServiceClient(projectClient(), serverInfo.BaseURL)
}

// projectClientFor returns an HTTP client with a custom project header.
// Used by isolation tests to target different projects.
func projectClientFor(slug string) *http.Client {
	return &http.Client{
		Transport: &projectRoundTripper{
			base:    http.DefaultTransport,
			project: slug,
		},
	}
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build -tags e2e ./e2e/...`
Expected: Success (e2e build tag needed for this package)

- [ ] **Step 3: Commit**

---

### Task 2: Full Happy-Path Pipeline Test (Protocol)

**Files:**

- Create: `e2e/api/pipeline_test.go`

- [ ] **Step 1: Write the pipeline test**

```go
//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Full pipeline", Ordered, func() {
	const (
		pipelineSlug  = "pipeline-full-test"
		pipelineAgent = "pipeline-agent-1"
	)

	var (
		specClient      specgraphv1connect.SpecServiceClient
		authoringClient specgraphv1connect.AuthoringServiceClient
		claimClient     specgraphv1connect.ClaimServiceClient
		execClient      specgraphv1connect.ExecutionServiceClient
		constClient     specgraphv1connect.ConstitutionServiceClient
		decisionClient  specgraphv1connect.DecisionServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		specClient = newSpecClient()
		authoringClient = newAuthoringClient()
		claimClient = newClaimClient()
		execClient = newExecutionClient()
		constClient = newConstitutionClient()
		decisionClient = newDecisionClient()
		ctx = context.Background()
	})

	It("sets a project constitution", func() {
		_, err := constClient.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Name:  "Pipeline Test Project",
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Principles: []*specv1.Principle{
					{Statement: "Keep it simple"},
					{Statement: "Test everything"},
				},
				Constraints: []*specv1.Constraint{
					{Rule: "No external dependencies without review"},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates a spec", func() {
		resp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   pipelineSlug,
			Intent: "Full pipeline end-to-end test spec",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal("spark"))
	})

	It("sparks the spec", func() {
		resp, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: pipelineSlug,
			Output: &specv1.SparkOutput{
				Seed:       "Build a widget service",
				Signal:     "Customer requests",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
				KillTest:   "No customer demand",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output.Seed).To(Equal("Build a widget service"))
	})

	It("shapes the spec with decisions", func() {
		resp, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: pipelineSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn:        []string{"widget CRUD", "widget listing"},
				ScopeOut:       []string{"widget analytics"},
				Approaches:     []*specv1.Approach{{Name: "rest-api", Description: "REST API"}},
				ChosenApproach: "rest-api",
				SuccessMust:    []string{"CRUD operations work"},
				Decisions: []*specv1.DecisionInput{
					{
						Slug:      "pipeline-decision-1",
						Title:     "Use REST over gRPC",
						Decision:  "REST API for simplicity",
						Rationale: "Easier for clients",
					},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output.ChosenApproach).To(Equal("rest-api"))
	})

	It("verifies decisions were promoted", func() {
		resp, err := decisionClient.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "pipeline-decision-1",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Title).To(Equal("Use REST over gRPC"))
	})

	It("specifies the spec", func() {
		resp, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: pipelineSlug,
			Output: &specv1.SpecifyOutput{
				InterfaceContract: "POST /api/v1/widgets",
				VerifyCriteria:    []string{"returns 201 on create", "returns 200 on list"},
				Invariants:        []string{"widget IDs are unique"},
				Touches:           []string{"internal/widget/handler.go"},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output.InterfaceContract).NotTo(BeEmpty())
	})

	It("decomposes the spec", func() {
		resp, err := authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: pipelineSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
				Slices: []*specv1.DecompositionSlice{
					{Id: "slice-create", Intent: "Create widget endpoint", Verify: []string{"POST returns 201"}},
					{Id: "slice-list", Intent: "List widgets endpoint", Verify: []string{"GET returns 200"}, DependsOn: []string{"slice-create"}},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output.Slices).To(HaveLen(2))
	})

	It("approves the spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
	})

	It("claims the spec", func() {
		resp, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      pipelineSlug,
			Agent:         pipelineAgent,
			LeaseDuration: durationpb.New(60_000_000_000), // 1 minute
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Agent).To(Equal(pipelineAgent))
	})

	It("generates an execution bundle", func() {
		resp, err := execClient.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Spec).NotTo(BeNil())
		Expect(resp.Msg.Spec.Slug).To(Equal(pipelineSlug))
		// Bundle should include constitution context
		Expect(resp.Msg.Constitution).NotTo(BeNil())
	})

	It("reports progress events", func() {
		_, err := execClient.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			Slug:    pipelineSlug,
			Agent:   pipelineAgent,
			Message: "implementing slice-create",
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = execClient.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			Slug:    pipelineSlug,
			Agent:   pipelineAgent,
			Message: "implementing slice-list",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("reports a blocker", func() {
		_, err := execClient.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
			Slug:    pipelineSlug,
			Agent:   pipelineAgent,
			Description: "blocked on dependency X",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("reports completion", func() {
		_, err := execClient.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
			Slug:  pipelineSlug,
			Agent: pipelineAgent,
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("verifies final state: spec is done", func() {
		resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal("done"))
	})

	It("verifies execution events are recorded", func() {
		resp, err := execClient.GetExecutionEvents(ctx, connect.NewRequest(&specv1.GetExecutionEventsRequest{
			Slug:  pipelineSlug,
			Limit: 10,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Events).To(HaveLen(4))
		// Events are ordered by time descending
		Expect(resp.Msg.Events[0].Type).To(Equal(specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION))
		Expect(resp.Msg.Events[1].Type).To(Equal(specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER))
		Expect(resp.Msg.Events[2].Type).To(Equal(specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS))
		Expect(resp.Msg.Events[3].Type).To(Equal(specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS))
	})
})
```

- [ ] **Step 2: Run to verify it compiles**

Run: `go build -tags e2e ./e2e/api/`

- [ ] **Step 3: Run e2e tests locally** (requires Docker)

Run: `go test -tags e2e ./e2e/api/ -run "Full pipeline" -v -count=1 -timeout=300s`

- [ ] **Step 4: Fix any failures and re-run**

- [ ] **Step 5: Commit**

---

### Task 3: Multi-Project Isolation Test

**Files:**

- Create: `e2e/api/isolation_test.go`

- [ ] **Step 1: Write the isolation test**

```go
//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Project isolation", Ordered, func() {
	var (
		alphaSpec specgraphv1connect.SpecServiceClient
		betaSpec  specgraphv1connect.SpecServiceClient
		alphaConst specgraphv1connect.ConstitutionServiceClient
		betaConst  specgraphv1connect.ConstitutionServiceClient
		ctx       context.Context
	)

	BeforeAll(func() {
		ctx = context.Background()
		alphaHTTP := projectClientFor("project-alpha")
		betaHTTP := projectClientFor("project-beta")
		alphaSpec = specgraphv1connect.NewSpecServiceClient(alphaHTTP, serverInfo.BaseURL)
		betaSpec = specgraphv1connect.NewSpecServiceClient(betaHTTP, serverInfo.BaseURL)
		alphaConst = specgraphv1connect.NewConstitutionServiceClient(alphaHTTP, serverInfo.BaseURL)
		betaConst = specgraphv1connect.NewConstitutionServiceClient(betaHTTP, serverInfo.BaseURL)
	})

	It("creates a spec in project-alpha", func() {
		_, err := alphaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-shared-name",
			Intent: "alpha intent",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("creates a spec with same slug in project-beta", func() {
		_, err := betaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-shared-name",
			Intent: "beta intent",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns alpha intent for project-alpha", func() {
		resp, err := alphaSpec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "iso-shared-name",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Intent).To(Equal("alpha intent"))
	})

	It("returns beta intent for project-beta", func() {
		resp, err := betaSpec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "iso-shared-name",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Intent).To(Equal("beta intent"))
	})

	It("lists specs only for the requesting project", func() {
		alphaList, err := alphaSpec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())

		betaList, err := betaSpec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())

		// Each project should see only its own specs
		alphaIntents := make([]string, 0)
		for _, s := range alphaList.Msg.Specs {
			if s.Slug == "iso-shared-name" {
				alphaIntents = append(alphaIntents, s.Intent)
			}
		}
		Expect(alphaIntents).To(ConsistOf("alpha intent"))

		betaIntents := make([]string, 0)
		for _, s := range betaList.Msg.Specs {
			if s.Slug == "iso-shared-name" {
				betaIntents = append(betaIntents, s.Intent)
			}
		}
		Expect(betaIntents).To(ConsistOf("beta intent"))
	})

	It("isolates constitution per project", func() {
		// Set constitution in alpha
		_, err := alphaConst.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Name:  "Alpha Constitution",
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Alpha should see it
		alphaResp, err := alphaConst.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(alphaResp.Msg.Name).To(Equal("Alpha Constitution"))

		// Beta should NOT see alpha's constitution
		_, err = betaConst.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).To(HaveOccurred()) // not found
	})
})
```

- [ ] **Step 2: Run e2e tests**

Run: `go test -tags e2e ./e2e/api/ -run "Project isolation" -v -count=1 -timeout=300s`

- [ ] **Step 3: Commit**

---

### Task 4: Constitution Integration Test

**Files:**

- Create: `e2e/api/constitution_pipeline_test.go`

- [ ] **Step 1: Write the constitution integration test**

Tests constitution storage, retrieval, and inclusion in bundles. Does NOT test semantic violation detection (placeholder passes).

- [ ] **Step 2: Run e2e tests**

- [ ] **Step 3: Commit**

---

### Task 5: Lifecycle Transitions Test

**Files:**

- Create: `e2e/api/lifecycle_pipeline_test.go`

- [ ] **Step 1: Write the lifecycle test**

Tests: approve → amend (re_entry_stage: "shape") → re-approve → supersede → abandon. Verifies history, version increments, stage transitions, superseded_by/supersedes links.

- [ ] **Step 2: Run e2e tests**

- [ ] **Step 3: Commit**

---

## Chunk 2: CLI Tier Tests

### Task 6: CLI Test Fixtures

**Files:**

- Create: `e2e/cli/testdata/spark-output.json`
- Create: `e2e/cli/testdata/shape-output.json`
- Create: `e2e/cli/testdata/specify-output.json`
- Create: `e2e/cli/testdata/decompose-output.json`

- [ ] **Step 1: Create JSON fixtures**

Each file contains the proto JSON for the corresponding authoring stage output. Use `protojson.Marshal` format. Reference the existing proto message definitions in `proto/specgraph/v1/authoring.proto`.

- [ ] **Step 2: Validate JSON**

Run: `python3 -c "import json; [json.load(open(f)) for f in ['e2e/cli/testdata/spark-output.json', 'e2e/cli/testdata/shape-output.json', 'e2e/cli/testdata/specify-output.json', 'e2e/cli/testdata/decompose-output.json']]"`

- [ ] **Step 3: Commit**

---

### Task 7: CLI Pipeline Test Suite

**Files:**

- Create: `e2e/cli/cli_suite_test.go`
- Create: `e2e/cli/pipeline_test.go`

- [ ] **Step 1: Write CLI suite setup**

```go
//go:build e2e_cli

package cli_test

// BeforeSuite: BuildBinary + StartMemgraph + StartServer
// AfterSuite: cleanup
```

Reuses `testutil.BuildBinary`, `testutil.StartMemgraph`, `testutil.StartServer`. The `CLIRunner` uses `serverInfo.ConfigPath` for `--config`.

- [ ] **Step 2: Write CLI pipeline test**

Walks through: create → spark → shape → specify → decompose → approve → claim → bundle → progress → show. Each step calls `cli.RunInDir(tmpDir, "specgraph", args...)` and verifies exit code + stdout.

- [ ] **Step 3: Run CLI e2e tests**

Run: `go test -tags e2e_cli ./e2e/cli/ -v -count=1 -timeout=300s`

- [ ] **Step 4: Commit**

---

### Task 8: Taskfile and CI Updates

**Files:**

- Modify: `Taskfile.yaml`
- Modify: `.github/workflows/ci.yaml`

- [ ] **Step 1: Add Taskfile entries**

```yaml
test:e2e:cli:
  cmd: go test -tags e2e_cli ./e2e/cli/ -v -count=1 -timeout=300s

test:e2e:agent:
  cmd: go test -tags e2e_agent ./e2e/agent/ -v -count=1 -timeout=600s
```

- [ ] **Step 2: Add CI workflow step**

Add `task test:e2e:cli` to the e2e job in CI.

- [ ] **Step 3: Commit**

---

## Chunk 3: Agent Tier Tests

### Task 9: Agent Test Suite

**Files:**

- Create: `e2e/agent/agent_suite_test.go`
- Create: `e2e/agent/pipeline_test.go`

- [ ] **Step 1: Write agent suite setup**

```go
//go:build e2e_agent

package agent_test

// BeforeSuite:
//   1. Check ANTHROPIC_API_KEY env var — skip if not set
//   2. Check claude CLI is available
//   3. BuildBinary + StartMemgraph + StartServer
//   4. Install specgraph plugin to temp dir
```

- [ ] **Step 2: Write agent pipeline test**

Each stage invokes `claude --print` with a prompt and verifies state via `specgraph show`. Non-deterministic — only assert stage transitions, not exact output content.

- [ ] **Step 3: Commit**

---

## Chunk 4: Verification

### Task 10: Run Full E2E Suite

- [ ] **Step 1: Run protocol tests**

Run: `task test:e2e:api`

- [ ] **Step 2: Run CLI tests**

Run: `task test:e2e:cli`

- [ ] **Step 3: Run `task check`**

Run: `task check`

- [ ] **Step 4: Fix any issues**

- [ ] **Step 5: Final commit**

---

## Summary

| Chunk | Tasks | Focus |
|-------|-------|-------|
| 1 | 1-5 | Protocol tier: helpers, pipeline, isolation, constitution, lifecycle |
| 2 | 6-8 | CLI tier: fixtures, pipeline test, Taskfile/CI |
| 3 | 9 | Agent tier: suite setup, agent-driven pipeline |
| 4 | 10 | Verification: run all, fix issues |

**Total:** 10 tasks across 4 chunks.
