// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Authoring funnel", Ordered, func() {
	const authoringSlug = "auth-funnel-test"

	var (
		authoringClient specgraphv1connect.AuthoringServiceClient
		specClient      specgraphv1connect.SpecServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		authoringClient = specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, serverInfo.BaseURL)
		specClient = specgraphv1connect.NewSpecServiceClient(http.DefaultClient, serverInfo.BaseURL)
		ctx = context.Background()

		// Seed a spec so the authoring funnel has something to work with.
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   authoringSlug,
			Intent: "E2E authoring funnel test spec",
		}))
		Expect(err).NotTo(HaveOccurred())
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
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.ChosenApproach).To(Equal("approach-1"))
	})

	It("specifies a shaped spec", func() {
		resp, err := authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: authoringSlug,
			Output: &specv1.SpecifyOutput{
				InterfaceContract: "POST /api/v1/things",
				VerifyCriteria:    []string{"returns 200"},
				Invariants:        []string{"data is valid"},
				Touches:           []string{"internal/server/handler.go"},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Output).NotTo(BeNil())
		Expect(resp.Msg.Output.InterfaceContract).NotTo(BeEmpty())
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
	})

	It("approves a decomposed spec", func() {
		resp, err := authoringClient.Approve(ctx, connect.NewRequest(&specv1.ApproveRequest{
			Slug: authoringSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Stage).To(Equal(specv1.AuthoringStage_AUTHORING_STAGE_APPROVED))
		Expect(resp.Msg.ApprovedAt).NotTo(BeNil())
	})
})
