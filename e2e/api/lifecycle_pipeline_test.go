// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Lifecycle Pipeline", Ordered, func() {
	const (
		pipelineSlug    = "lifecycle-pipeline-spec"
		replacementSlug = "lifecycle-pipeline-v2"
	)

	var (
		lifecycleClient specgraphv1connect.LifecycleServiceClient
		specClient      specgraphv1connect.SpecServiceClient
		graphClient     specgraphv1connect.GraphServiceClient
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		lifecycleClient = newLifecycleClient()
		specClient = newSpecClient()
		graphClient = newGraphClient()
		authoringClient = newAuthoringClient()
		ctx = context.Background()
	})

	It("creates a spec and advances to approved", func() {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   pipelineSlug,
			Intent: "Full lifecycle pipeline test",
		}))
		Expect(err).NotTo(HaveOccurred())

		Expect(advanceStage(ctx, pipelineSlug, "approved")).To(Succeed())

		resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetStage()).To(Equal("approved"))
	})

	It("amends with re-entry to shape stage", func() {
		resp, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug:         pipelineSlug,
			Reason:       "Requirements evolved after initial delivery",
			ReEntryStage: "shape",
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := resp.Msg.GetSpec()
		Expect(spec.GetSlug()).To(Equal(pipelineSlug))
		// re-entry "shape" lands at "spark" so the Shape RPC (spark→shape) can run.
		Expect(spec.GetStage()).To(Equal("spark"))
		Expect(spec.GetVersion()).To(BeNumerically(">=", int32(2)))
		// History field removed — changelog is now tracked via ChangeLog graph nodes.
	})

	It("re-traverses funnel: shape", func() {
		resp, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: pipelineSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn:        []string{"amended scope"},
				ScopeOut:       []string{"out of scope"},
				SuccessMust:    []string{"amended behaviour works"},
				ChosenApproach: "revised-approach",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "what changed in scope?", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "amended scope only", Stage: "shape", Sequence: 2},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.ScopeIn).To(ContainElement("amended scope"))
	})

	It("re-traverses funnel: specify", func() {
		resp, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: pipelineSlug,
			Output: &specv1.SpecifyOutput{
				Interfaces: []*specv1.InterfaceSection{
					{Name: "API", Body: "POST /api/v1/amended"},
				},
				VerifyCriteria: []*specv1.VerifyCriterion{
					{Category: "happy-path", Description: "returns 200"},
				},
				Invariants: []string{"data consistent"},
				Touches: []*specv1.FileTouch{
					{Path: "internal/amended/handler.go", Purpose: "handler updates", ChangeType: "modify"},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "what are the amended interfaces?", Stage: "specify", Sequence: 1},
				{Role: "response", Content: "POST /api/v1/amended", Stage: "specify", Sequence: 2},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Interfaces[0].Body).To(Equal("POST /api/v1/amended"))
	})

	It("re-traverses funnel: decompose", func() {
		resp, err := authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: pipelineSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
				Slices: []*specv1.DecompositionSlice{
					{Id: "amended-slice-1", Intent: "Amended slice", Verify: []string{"test passes"}},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "how to decompose amended spec?", Stage: "decompose", Sequence: 1},
				{Role: "response", Content: "single amended slice", Stage: "decompose", Sequence: 2},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Slices).To(HaveLen(1))
	})

	It("re-traverses funnel: approve", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: pipelineSlug,
			ConversationExchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "ready to approve?", Stage: "approve", Sequence: 1},
				{Role: "response", Content: "approved", Stage: "approve", Sequence: 2},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
	})

	It("re-advances to done again", func() {
		Expect(claimAndComplete(ctx, pipelineSlug)).To(Succeed())

		resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetStage()).To(Equal("done"))
	})

	It("creates a replacement spec", func() {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   replacementSlug,
			Intent: "Replacement for pipeline spec v1",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("supersedes old spec with new", func() {
		resp, err := lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
			Slug:    pipelineSlug,
			NewSlug: replacementSlug,
		}))
		Expect(err).NotTo(HaveOccurred())

		Expect(resp.Msg.OldSpec).NotTo(BeNil())
		Expect(resp.Msg.OldSpec.Slug).To(Equal(pipelineSlug))
		Expect(resp.Msg.OldSpec.Stage).To(Equal("superseded"))
		Expect(resp.Msg.OldSpec.SupersededBy).To(Equal(replacementSlug))

		Expect(resp.Msg.NewSpec).NotTo(BeNil())
		Expect(resp.Msg.NewSpec.Slug).To(Equal(replacementSlug))
		Expect(resp.Msg.NewSpec.Supersedes).To(Equal(pipelineSlug))
	})

	It("has a SUPERSEDES edge from new to old", func() {
		edgeResp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug:     replacementSlug,
			EdgeType: specv1.EdgeType_EDGE_TYPE_SUPERSEDES,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(edgeResp.Msg.Edges).NotTo(BeEmpty())

		found := false
		for _, e := range edgeResp.Msg.Edges {
			if e.EdgeType == specv1.EdgeType_EDGE_TYPE_SUPERSEDES && e.ToId == pipelineSlug {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "SUPERSEDES edge from replacement to original not found")
	})

	It("rejects drift check on superseded spec with FailedPrecondition", func() {
		// Superseded is not done/amended, so CheckDrift should reject it.
		_, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
			Slug: pipelineSlug,
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
	})

	It("rejects amend on superseded spec with FailedPrecondition", func() {
		_, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug:         pipelineSlug,
			Reason:       "should fail on superseded spec",
			ReEntryStage: "spark",
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
	})

	It("abandons the replacement spec", func() {
		// Capture version before abandon for comparison.
		getResp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: replacementSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		versionBefore := getResp.Msg.GetSpec().GetVersion()

		resp, err := lifecycleClient.TransitionAbandon(ctx, connect.NewRequest(&specv1.TransitionAbandonRequest{
			Slug:   replacementSlug,
			Reason: "Project direction changed",
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := resp.Msg.GetSpec()
		Expect(spec.GetSlug()).To(Equal(replacementSlug))
		Expect(spec.GetStage()).To(Equal("abandoned"))
		Expect(spec.GetVersion()).To(BeNumerically(">", versionBefore))
		// History field removed — changelog is now tracked via ChangeLog graph nodes.
	})

	It("lints the abandoned spec", func() {
		resp, err := lifecycleClient.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
			Slug: replacementSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Results).NotTo(BeEmpty())
		// Terminal specs should still be lintable.
		Expect(resp.Msg.Results[0].SpecSlug).To(Equal(replacementSlug))
	})
})
