// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Authoring funnel", Ordered, func() {
	const authoringSlug = "auth-funnel-test"

	var (
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		authoringClient = newAuthoringClient()
		ctx = context.Background()
	})

	It("sparks a new spec from an idea", func() {
		resp, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: authoringSlug,
			Output: &specv1.SparkOutput{
				Seed:       "E2E test idea",
				Signal:     "Testing the full funnel",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
				KillTest:   "Test fails",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Seed).To(Equal("E2E test idea"))
	})

	It("shapes a sparked spec", func() {
		resp, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: authoringSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn:  []string{"feature A"},
				ScopeOut: []string{"feature B"},
				Approaches: []*specv1.Approach{
					{Name: "approach-1", Description: "Do it this way"},
				},
				ChosenApproach: "approach-1",
				SuccessMust:    []string{"works correctly"},
				Decisions: []*specv1.DecisionInput{
					{
						Slug:      "e2e-decision-1",
						Title:     "Use approach 1",
						Decision:  "We chose approach 1",
						Rationale: "Simplest option",
					},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.ChosenApproach).To(Equal("approach-1"))
	})

	It("promoted shape decisions to Decision nodes", func() {
		decisionClient := newDecisionClient()
		resp, err := decisionClient.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "e2e-decision-1",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetDecision().GetTitle()).To(Equal("Use approach 1"))
		Expect(resp.Msg.GetDecision().GetDecision()).To(Equal("We chose approach 1"))
		Expect(resp.Msg.GetDecision().GetStatus()).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
	})

	It("specifies a shaped spec", func() {
		resp, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: authoringSlug,
			Output: &specv1.SpecifyOutput{
				Interfaces: []*specv1.InterfaceSection{
					{Name: "API", Body: "POST /api/v1/things"},
				},
				VerifyCriteria: []*specv1.VerifyCriterion{
					{Category: "happy-path", Description: "returns 200"},
				},
				Invariants: []string{"data is valid"},
				Touches: []*specv1.FileTouch{
					{Path: "internal/server/handler.go", Purpose: "handler updates", ChangeType: "modify"},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Interfaces).NotTo(BeEmpty())
	})

	It("decomposes a specified spec", func() {
		resp, err := authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: authoringSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
				Slices: []*specv1.DecompositionSlice{
					{Id: "slice-1", Intent: "First slice", Verify: []string{"test passes"}},
					{Id: "slice-2", Intent: "Second slice", Verify: []string{"test passes"}, DependsOn: []string{"slice-1"}},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Slices).To(HaveLen(2))
		Expect(resp.Msg.SliceSlugs).To(HaveLen(2))
		Expect(resp.Msg.SliceSlugs[0]).To(HaveSuffix("/slice-1"))
		Expect(resp.Msg.SliceSlugs[1]).To(HaveSuffix("/slice-2"))
	})

	It("approves a decomposed spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: authoringSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
		Expect(resp.Msg.ApprovedAt).NotTo(BeNil())
	})

	It("returns AlreadyExists for duplicate slug", func() {
		const dupSlug = "dup-spark-e2e"

		// First Spark should succeed.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: dupSlug,
			Output: &specv1.SparkOutput{
				Seed:   "duplicate test idea",
				Signal: "testing duplicate rejection",
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		// Second Spark with same slug should fail with AlreadyExists.
		_, err = authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: dupSlug,
			Output: &specv1.SparkOutput{
				Seed:   "another idea",
				Signal: "should not matter",
			},
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeAlreadyExists))
	})
})

var _ = Describe("Authoring funnel — steel thread", Ordered, func() {
	const steelThreadSlug = "steel-thread-funnel-test"

	var (
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		authoringClient = newAuthoringClient()
		ctx = context.Background()
	})

	It("sparks a new spec", func() {
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: steelThreadSlug,
			Output: &specv1.SparkOutput{
				Seed:       "Steel thread E2E test idea",
				Signal:     "Testing steel thread decomposition",
				ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
				KillTest:   "Test fails",
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("shapes the spec", func() {
		_, err := authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: steelThreadSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn: []string{"interfaces"},
				Risks:   []string{"integration risk"},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("specifies the spec", func() {
		_, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: steelThreadSlug,
			Output: &specv1.SpecifyOutput{
				Interfaces:     []*specv1.InterfaceSection{{Name: "API", Body: "test"}},
				VerifyCriteria: []*specv1.VerifyCriterion{{Description: "passes"}},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("decomposes with steel thread strategy", func() {
		resp, err := authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: steelThreadSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
				Slices: []*specv1.DecompositionSlice{
					{Id: "thread", Intent: "Prove roundtrip", Verify: []string{"roundtrip works"}},
					{Id: "broaden-a", Intent: "Add feature A", Verify: []string{"feature A works"}, DependsOn: []string{"thread"}},
					{Id: "broaden-b", Intent: "Add feature B", Verify: []string{"feature B works"}, DependsOn: []string{"thread"}},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.Slices).To(HaveLen(3))
		Expect(resp.Msg.SliceSlugs).To(HaveLen(3))
		Expect(resp.Msg.SliceSlugs).To(ContainElement(HaveSuffix("/thread")))
	})

	It("approves the steel thread spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: steelThreadSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
	})

	It("rejects steel thread with disconnected slice", func() {
		const badSlug = "steel-thread-bad-test"
		// Advance to specify first
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug:   badSlug,
			Output: &specv1.SparkOutput{Seed: "bad steel thread", Signal: "test", ScopeSniff: specv1.ScopeSniff_SCOPE_SNIFF_SMALL, KillTest: "fails"},
		}))
		Expect(err).NotTo(HaveOccurred())
		_, err = authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug:   badSlug,
			Output: &specv1.ShapeOutput{ScopeIn: []string{"test"}, Risks: []string{"none"}},
		}))
		Expect(err).NotTo(HaveOccurred())
		_, err = authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug:   badSlug,
			Output: &specv1.SpecifyOutput{Interfaces: []*specv1.InterfaceSection{{Name: "X", Body: "y"}}, VerifyCriteria: []*specv1.VerifyCriterion{{Description: "z"}}},
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: badSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
				Slices: []*specv1.DecompositionSlice{
					{Id: "thread", Intent: "Prove roundtrip"},
					{Id: "island", Intent: "No path to thread"},
				},
			},
		}))
		Expect(err).To(HaveOccurred())
		var connErr *connect.Error
		Expect(errors.As(err, &connErr)).To(BeTrue())
		Expect(connErr.Code()).To(Equal(connect.CodeInvalidArgument))
	})
})
