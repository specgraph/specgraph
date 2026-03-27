// SPDX-License-Identifier: MIT
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

var _ = Describe("SliceService", Ordered, func() {
	var (
		sliceClient     specgraphv1connect.SliceServiceClient
		authoringClient specgraphv1connect.AuthoringServiceClient
		ctx             = context.Background()
		parentSlug      = "slice-svc-e2e"
	)

	// Setup: create clients and advance a spec through decompose to create slices.
	// Uses POSTURE_DRIVE which creates the spec on Spark if it doesn't exist.
	BeforeAll(func() {
		sliceClient = newSliceClient()
		authoringClient = newAuthoringClient()

		// Spark (POSTURE_DRIVE creates the spec automatically)
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: parentSlug,
			Output: &specv1.SparkOutput{
				Seed:   "test seed",
				Signal: "test signal",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Shape
		_, err = authoringClient.Shape(ctx, connect.NewRequest(&specv1.ShapeRequest{
			Slug: parentSlug,
			Output: &specv1.ShapeOutput{
				ScopeIn:        []string{"slice testing"},
				ChosenApproach: "test approach",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Specify
		_, err = authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: parentSlug,
			Output: &specv1.SpecifyOutput{
				Interfaces: []*specv1.InterfaceSection{{Name: "API", Body: "REST"}},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Decompose — creates slices
		_, err = authoringClient.Decompose(ctx, connect.NewRequest(&specv1.DecomposeRequest{
			Slug: parentSlug,
			Output: &specv1.DecomposeOutput{
				Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
				Slices: []*specv1.DecompositionSlice{
					{Id: "backend", Intent: "Backend API", Verify: []string{"tests pass"}},
					{Id: "frontend", Intent: "Frontend UI", Verify: []string{"renders"}, DependsOn: []string{"backend"}},
				},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("lists slices for a parent spec", func() {
		resp, err := sliceClient.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
			ParentSlug: parentSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slices).To(HaveLen(2))

		// Verify slice fields are populated
		slugs := make([]string, len(resp.Msg.Slices))
		for i, s := range resp.Msg.Slices {
			slugs[i] = s.SliceId
			Expect(s.ParentSlug).To(Equal(parentSlug))
			Expect(s.Intent).NotTo(BeEmpty())
			Expect(s.Status).To(Equal(specv1.SliceStatus_SLICE_STATUS_OPEN))
		}
		Expect(slugs).To(ContainElements("backend", "frontend"))
	})

	It("gets a single slice by slug", func() {
		resp, err := sliceClient.GetSlice(ctx, connect.NewRequest(&specv1.GetSliceRequest{
			Slug: parentSlug + "/backend",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slice.SliceId).To(Equal("backend"))
		Expect(resp.Msg.Slice.Intent).To(Equal("Backend API"))
		Expect(resp.Msg.Slice.Status).To(Equal(specv1.SliceStatus_SLICE_STATUS_OPEN))
	})

	It("returns not-found for nonexistent slice", func() {
		_, err := sliceClient.GetSlice(ctx, connect.NewRequest(&specv1.GetSliceRequest{
			Slug: parentSlug + "/nonexistent",
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})

	It("claims a slice", func() {
		resp, err := sliceClient.ClaimSlice(ctx, connect.NewRequest(&specv1.ClaimSliceRequest{
			Slug:     parentSlug + "/backend",
			Assignee: "e2e-agent",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slice.Status).To(Equal(specv1.SliceStatus_SLICE_STATUS_CLAIMED))
		Expect(resp.Msg.Slice.AssignedTo).To(Equal("e2e-agent"))
	})

	It("rejects claim on already-claimed slice", func() {
		_, err := sliceClient.ClaimSlice(ctx, connect.NewRequest(&specv1.ClaimSliceRequest{
			Slug:     parentSlug + "/backend",
			Assignee: "another-agent",
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
	})

	It("completes a claimed slice", func() {
		resp, err := sliceClient.CompleteSlice(ctx, connect.NewRequest(&specv1.CompleteSliceRequest{
			Slug: parentSlug + "/backend",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Slice.Status).To(Equal(specv1.SliceStatus_SLICE_STATUS_DONE))
	})

	It("rejects complete on unclaimed slice", func() {
		_, err := sliceClient.CompleteSlice(ctx, connect.NewRequest(&specv1.CompleteSliceRequest{
			Slug: parentSlug + "/frontend",
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
	})

	It("includes slices in GetFullGraph", func() {
		graphClient := newGraphClient()
		resp, err := graphClient.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		Expect(err).NotTo(HaveOccurred())

		sliceNodes := 0
		for _, n := range resp.Msg.Nodes {
			if n.Label == "Slice" {
				sliceNodes++
			}
		}
		Expect(sliceNodes).To(BeNumerically(">=", 2))
	})
})
