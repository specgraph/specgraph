// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

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

var _ = Describe("graph queries", Ordered, func() {
	var (
		specClient  specgraphv1connect.SpecServiceClient
		graphClient specgraphv1connect.GraphServiceClient
		ctx         context.Context
	)

	BeforeAll(func() {
		specClient = newSpecClient()
		graphClient = newGraphClient()
		ctx = context.Background()

		// Create specs for the dependency chain: gq-a -> gq-b -> gq-c (DEPENDS_ON).
		for _, slug := range []string{"gq-a", "gq-b", "gq-c", "gq-d"} {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:     slug,
				Intent:   "Graph query test spec " + slug,
				Priority: "p2",
			}))
			Expect(err).NotTo(HaveOccurred())
		}

		// gq-a depends on gq-b.
		_, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "gq-a",
			ToSlug:   "gq-b",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		// gq-b depends on gq-c.
		_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "gq-b",
			ToSlug:   "gq-c",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		// gq-d is standalone (no dependencies).
	})

	It("shows dependencies with GetDependencies", func() {
		resp, err := graphClient.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{
			Slug: "gq-a",
		}))
		Expect(err).NotTo(HaveOccurred())

		slugs := extractSlugs(resp.Msg.Dependencies)
		Expect(slugs).To(ContainElement("gq-b"))
	})

	It("shows transitive dependencies with GetTransitiveDeps", func() {
		resp, err := graphClient.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{
			Slug: "gq-a",
		}))
		Expect(err).NotTo(HaveOccurred())

		slugs := extractSlugs(resp.Msg.Dependencies)
		Expect(slugs).To(ContainElement("gq-b"))
		Expect(slugs).To(ContainElement("gq-c"))
	})

	It("shows specs ready to work on with GetReady", func() {
		resp, err := graphClient.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		Expect(err).NotTo(HaveOccurred())

		slugs := extractSlugs(resp.Msg.Ready)
		// gq-c has no dependencies, so it should be ready.
		Expect(slugs).To(ContainElement("gq-c"))
		// gq-d is standalone, so it should be ready.
		Expect(slugs).To(ContainElement("gq-d"))
	})

	It("shows critical path for a spec", func() {
		resp, err := graphClient.GetCriticalPath(ctx, connect.NewRequest(&specv1.GetCriticalPathRequest{
			Slug: "gq-a",
		}))
		Expect(err).NotTo(HaveOccurred())

		slugs := extractSlugs(resp.Msg.Path)
		Expect(slugs).To(ContainElement("gq-b"))
		Expect(slugs).To(ContainElement("gq-c"))
	})

	It("shows impact of changes to a spec", func() {
		resp, err := graphClient.GetImpact(ctx, connect.NewRequest(&specv1.GetImpactRequest{
			Slug: "gq-c",
		}))
		Expect(err).NotTo(HaveOccurred())

		slugs := extractSlugs(resp.Msg.Impacted)
		// gq-b depends on gq-c, and gq-a depends on gq-b, so both are impacted.
		Expect(slugs).To(ContainElement("gq-a"))
		Expect(slugs).To(ContainElement("gq-b"))
	})
})

// extractSlugs is a helper to pull slug strings from a slice of NodeRef.
func extractSlugs(refs []*specv1.NodeRef) []string {
	slugs := make([]string, len(refs))
	for i, r := range refs {
		slugs[i] = r.Slug
	}
	return slugs
}
