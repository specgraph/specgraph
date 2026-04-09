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

var _ = Describe("Project isolation", Ordered, func() {
	var (
		alphaSpec  specgraphv1connect.SpecServiceClient
		betaSpec   specgraphv1connect.SpecServiceClient
		alphaConst specgraphv1connect.ConstitutionServiceClient
		betaConst  specgraphv1connect.ConstitutionServiceClient
		ctx        context.Context
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
		resp, err := alphaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-shared-name",
			Intent: "Alpha intent for isolation test",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetSlug()).To(Equal("iso-shared-name"))
		Expect(resp.Msg.GetSpec().GetIntent()).To(Equal("Alpha intent for isolation test"))
	})

	It("creates a spec with same slug in project-beta", func() {
		resp, err := betaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-shared-name",
			Intent: "Beta intent for isolation test",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetSlug()).To(Equal("iso-shared-name"))
		Expect(resp.Msg.GetSpec().GetIntent()).To(Equal("Beta intent for isolation test"))
	})

	It("creates a unique spec in each project for exclusion testing", func() {
		_, err := alphaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-alpha-only",
			Intent: "Alpha-exclusive spec",
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = betaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "iso-beta-only",
			Intent: "Beta-exclusive spec",
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("returns alpha intent for project-alpha", func() {
		resp, err := alphaSpec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "iso-shared-name",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetIntent()).To(Equal("Alpha intent for isolation test"))
	})

	It("returns beta intent for project-beta", func() {
		resp, err := betaSpec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "iso-shared-name",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetSpec().GetIntent()).To(Equal("Beta intent for isolation test"))
	})

	It("lists specs only for the requesting project", func() {
		alphaResp, err := alphaSpec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())

		alphaSlugs := make([]string, len(alphaResp.Msg.Specs))
		for i, s := range alphaResp.Msg.Specs {
			alphaSlugs[i] = s.Slug
		}
		Expect(alphaSlugs).To(ContainElement("iso-shared-name"))
		Expect(alphaSlugs).To(ContainElement("iso-alpha-only"))
		Expect(alphaSlugs).NotTo(ContainElement("iso-beta-only"), "alpha project should not see beta specs")

		betaResp, err := betaSpec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())

		betaSlugs := make([]string, len(betaResp.Msg.Specs))
		for i, s := range betaResp.Msg.Specs {
			betaSlugs[i] = s.Slug
		}
		Expect(betaSlugs).To(ContainElement("iso-shared-name"))
		Expect(betaSlugs).To(ContainElement("iso-beta-only"))
		Expect(betaSlugs).NotTo(ContainElement("iso-alpha-only"), "beta project should not see alpha specs")

		// Verify the intents are different — each project has its own spec
		var alphaIntent, betaIntent string
		for _, s := range alphaResp.Msg.Specs {
			if s.Slug == "iso-shared-name" {
				alphaIntent = s.Intent
			}
		}
		for _, s := range betaResp.Msg.Specs {
			if s.Slug == "iso-shared-name" {
				betaIntent = s.Intent
			}
		}
		Expect(alphaIntent).To(Equal("Alpha intent for isolation test"))
		Expect(betaIntent).To(Equal("Beta intent for isolation test"))
	})

	It("isolates decisions per project", func() {
		alphaDecision := specgraphv1connect.NewDecisionServiceClient(
			projectClientFor("project-alpha"), serverInfo.BaseURL)
		betaDecision := specgraphv1connect.NewDecisionServiceClient(
			projectClientFor("project-beta"), serverInfo.BaseURL)

		_, err := alphaDecision.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
			Slug:      "iso-decision-1",
			Title:     "Alpha-only decision",
			Decision:  "Use approach A",
			Rationale: "Alpha project reasons",
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = betaDecision.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "iso-decision-1",
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})

	It("isolates edges per project", func() {
		alphaGraph := specgraphv1connect.NewGraphServiceClient(
			projectClientFor("project-alpha"), serverInfo.BaseURL)
		betaGraph := specgraphv1connect.NewGraphServiceClient(
			projectClientFor("project-beta"), serverInfo.BaseURL)

		// Create two specs in alpha for edge testing.
		for _, slug := range []string{"iso-edge-a", "iso-edge-b"} {
			_, err := alphaSpec.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   slug,
				Intent: "edge test " + slug,
			}))
			Expect(err).NotTo(HaveOccurred())
		}

		// Add edge in alpha.
		_, err := alphaGraph.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "iso-edge-b",
			ToSlug:   "iso-edge-a",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Beta should not see the edge (spec doesn't exist in beta's project).
		// ListEdges returns empty (not an error) for unknown slugs.
		betaResp, err := betaGraph.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug: "iso-edge-b",
		}))
		if err != nil {
			// Some implementations return not-found for unknown slugs — also acceptable.
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
		} else {
			Expect(betaResp.Msg.Edges).To(BeEmpty(), "beta should not see alpha's edges")
		}
	})

	It("isolates constitution per project", func() {
		_, err := alphaConst.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:  "alpha-constitution",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary: "Go",
					},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = betaConst.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:  "beta-constitution",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary: "Rust",
					},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		alphaResp, err := alphaConst.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(alphaResp.Msg.Constitution.Name).To(Equal("alpha-constitution"))
		Expect(alphaResp.Msg.Constitution.Tech.Languages.Primary).To(Equal("Go"))

		betaResp, err := betaConst.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(betaResp.Msg.Constitution.Name).To(Equal("beta-constitution"))
		Expect(betaResp.Msg.Constitution.Tech.Languages.Primary).To(Equal("Rust"))
	})
})
