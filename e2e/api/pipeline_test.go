// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Full pipeline", Ordered, func() {
	const (
		pipelineSlug  = "pipeline-full-test"
		pipelineAgent = "pipeline-agent-1"
	)

	var (
		specClient         specgraphv1connect.SpecServiceClient
		authoringClient    specgraphv1connect.AuthoringServiceClient
		claimClient        specgraphv1connect.ClaimServiceClient
		executionClient    specgraphv1connect.ExecutionServiceClient
		constitutionClient specgraphv1connect.ConstitutionServiceClient
		decisionClient     specgraphv1connect.DecisionServiceClient
		ctx                context.Context
		decomposeResp      *connect.Response[specv1.DecomposeResponse]
	)

	BeforeAll(func() {
		// Use a dedicated project to avoid polluting the e2e-test project
		// (which the existing constitution_test.go expects to be clean).
		httpClient := projectClientFor("pipeline-project")
		specClient = specgraphv1connect.NewSpecServiceClient(httpClient, serverInfo.BaseURL)
		authoringClient = specgraphv1connect.NewAuthoringServiceClient(httpClient, serverInfo.BaseURL)
		claimClient = specgraphv1connect.NewClaimServiceClient(httpClient, serverInfo.BaseURL)
		executionClient = specgraphv1connect.NewExecutionServiceClient(httpClient, serverInfo.BaseURL)
		constitutionClient = specgraphv1connect.NewConstitutionServiceClient(httpClient, serverInfo.BaseURL)
		decisionClient = specgraphv1connect.NewDecisionServiceClient(httpClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("sets project constitution", func() {
		resp, err := constitutionClient.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:  "pipeline-e2e",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary: "Go",
						Allowed: []string{"Go"},
					},
				},
				Principles: []*specv1.Principle{
					{
						Id:        "p-pipeline",
						Statement: "Pipeline test principle",
						Rationale: "validates full flow",
					},
				},
				Constraints: []string{"must pass pipeline test"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Constitution).NotTo(BeNil())
		Expect(resp.Msg.Constitution.Name).To(Equal("pipeline-e2e"))
	})

	It("creates a spec", func() {
		resp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   pipelineSlug,
			Intent: "Full pipeline happy-path test spec",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slug).To(Equal(pipelineSlug))
		Expect(resp.Msg.Stage).To(Equal("spark"))
	})

	It("sparks the spec", func() {
		resp, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: pipelineSlug,
			Output: &specv1.SparkOutput{
				Seed:       "Pipeline test idea",
				Signal:     "Full pipeline validation",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
				KillTest:   "Pipeline test fails",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Seed).To(Equal("Pipeline test idea"))
	})

	It("shapes the spec with decisions", func() {
		resp, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: pipelineSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn:  []string{"pipeline feature"},
				ScopeOut: []string{"unrelated feature"},
				Approaches: []*specv1.Approach{
					{Name: "direct-approach", Description: "Implement directly"},
				},
				ChosenApproach: "direct-approach",
				SuccessMust:    []string{"pipeline completes end-to-end"},
				Decisions: []*specv1.DecisionInput{
					{
						Slug:      "pipeline-decision-1",
						Title:     "Use direct approach",
						Decision:  "We chose the direct approach",
						Rationale: "Simplest path to validation",
					},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.ChosenApproach).To(Equal("direct-approach"))
	})

	It("verifies decisions were promoted", func() {
		resp, err := decisionClient.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "pipeline-decision-1",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Title).To(Equal("Use direct approach"))
		Expect(resp.Msg.Decision).To(Equal("We chose the direct approach"))
		Expect(resp.Msg.Status).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
	})

	It("specifies the spec", func() {
		resp, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: pipelineSlug,
			Output: &specv1.SpecifyOutput{
				InterfaceContract: "POST /api/v1/pipeline",
				VerifyCriteria:    []string{"returns 200"},
				Invariants:        []string{"data is consistent"},
				Touches:           []string{"internal/pipeline/handler.go"},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.InterfaceContract).NotTo(BeEmpty())
	})

	It("decomposes the spec", func() {
		var err error
		decomposeResp, err = authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: pipelineSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
				Slices: []*specv1.DecompositionSlice{
					{Id: "pipeline-slice-1", Intent: "First pipeline slice", Verify: []string{"test passes"}},
					{Id: "pipeline-slice-2", Intent: "Second pipeline slice", Verify: []string{"integration test passes"}, DependsOn: []string{"pipeline-slice-1"}},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(decomposeResp.Msg.Output).NotTo(BeNil())
		Expect(decomposeResp.Msg.Output.Slices).To(HaveLen(2))
	})

	It("verifies decomposition slices have dependency metadata", func() {
		Expect(decomposeResp).NotTo(BeNil())
		slices := decomposeResp.Msg.Output.Slices
		Expect(slices).To(HaveLen(2))
		Expect(slices[1].Id).To(Equal("pipeline-slice-2"))
		Expect(slices[1].DependsOn).To(ContainElement("pipeline-slice-1"))
	})

	It("approves the spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
		Expect(resp.Msg.ApprovedAt).NotTo(BeNil())
	})

	It("claims the spec", func() {
		resp, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      pipelineSlug,
			Agent:         pipelineAgent,
			LeaseDuration: durationpb.New(60_000_000_000), // 1 minute
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.SpecSlug).To(Equal(pipelineSlug))
		Expect(resp.Msg.Agent).To(Equal(pipelineAgent))
		Expect(resp.Msg.ClaimedAt).NotTo(BeNil())
		Expect(resp.Msg.LeaseExpires).NotTo(BeNil())
	})

	It("generates an execution bundle", func() {
		resp, err := executionClient.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
			Slug:     pipelineSlug,
			Endpoint: serverInfo.BaseURL,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Spec).NotTo(BeNil())
		Expect(resp.Msg.Spec.Slug).To(Equal(pipelineSlug))
		Expect(resp.Msg.Version).To(BeNumerically(">=", int32(1)))
	})

	It("returns prime data for the spec", func() {
		resp, err := executionClient.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.ProjectContext).NotTo(BeEmpty())
		Expect(resp.Msg.CallbackDocs).NotTo(BeEmpty())
		Expect(resp.Msg.ConstitutionSummary).NotTo(BeEmpty())
	})

	It("reports first progress event", func() {
		resp, err := executionClient.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			Slug:    pipelineSlug,
			Agent:   pipelineAgent,
			Message: "Started implementation",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Acknowledged).To(BeTrue())
	})

	It("reports second progress event", func() {
		resp, err := executionClient.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			Slug:    pipelineSlug,
			Agent:   pipelineAgent,
			Message: "Implementation 50% complete",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Acknowledged).To(BeTrue())
	})

	It("reports a blocker", func() {
		resp, err := executionClient.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
			Slug:        pipelineSlug,
			Agent:       pipelineAgent,
			Description: "Blocked on dependency resolution",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Acknowledged).To(BeTrue())
	})

	It("reports completion", func() {
		resp, err := executionClient.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
			Slug:  pipelineSlug,
			Agent: pipelineAgent,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Acknowledged).To(BeTrue())
		Expect(resp.Msg.NewStage).To(Equal("done"))
	})

	It("verifies final state is done", func() {
		resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal("done"))
	})

	It("verifies execution events are recorded", func() {
		resp, err := executionClient.GetExecutionEvents(ctx, connect.NewRequest(&specv1.GetExecutionEventsRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Events).To(HaveLen(4))

		// ULID ordering within the same millisecond is non-deterministic,
		// so check by type counts rather than positional order.
		typeCounts := map[specv1.ExecutionEventType]int{}
		for _, event := range resp.Msg.Events {
			Expect(event.SpecSlug).To(Equal(pipelineSlug))
			Expect(event.Agent).To(Equal(pipelineAgent))
			typeCounts[event.Type]++
		}
		Expect(typeCounts[specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS]).To(Equal(2))
		Expect(typeCounts[specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_BLOCKER]).To(Equal(1))
		Expect(typeCounts[specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION]).To(Equal(1))
	})
})
