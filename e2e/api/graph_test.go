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

	It("returns all nodes and edges via GetFullGraph", func() {
		// Create two specs with unique slugs for this test.
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug: "gfg-parent", Intent: "Parent spec", Priority: "p1",
		}))
		Expect(err).NotTo(HaveOccurred())
		_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug: "gfg-child", Intent: "Child spec", Priority: "p2",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Add dependency edge.
		_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "gfg-child", ToSlug: "gfg-parent",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		// GetFullGraph returns the whole graph.
		resp, err := graphClient.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(len(resp.Msg.Nodes)).To(BeNumerically(">=", 2))

		// Find our nodes.
		slugs := make([]string, 0)
		for _, n := range resp.Msg.Nodes {
			slugs = append(slugs, n.Slug)
		}
		Expect(slugs).To(ContainElements("gfg-parent", "gfg-child"))

		// Verify edge exists and all edge types are user-facing.
		Expect(len(resp.Msg.Edges)).To(BeNumerically(">=", 1))
		validEdgeTypes := map[specv1.EdgeType]bool{
			specv1.EdgeType_EDGE_TYPE_DEPENDS_ON: true,
			specv1.EdgeType_EDGE_TYPE_BLOCKS:     true,
			specv1.EdgeType_EDGE_TYPE_COMPOSES:   true,
			specv1.EdgeType_EDGE_TYPE_RELATES_TO: true,
			specv1.EdgeType_EDGE_TYPE_INFORMS:    true,
			specv1.EdgeType_EDGE_TYPE_DECIDED_IN: true,
			specv1.EdgeType_EDGE_TYPE_SUPERSEDES: true,
		}
		var foundEdge bool
		for _, e := range resp.Msg.Edges {
			if e.FromId == "gfg-child" && e.ToId == "gfg-parent" {
				foundEdge = true
				Expect(e.EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
			}
			// All returned edge types must be user-facing (not internal like HAS_CHANGE, HAS_FINDING).
			Expect(validEdgeTypes).To(HaveKey(e.EdgeType), "unexpected edge type: %v", e.EdgeType)
		}
		Expect(foundEdge).To(BeTrue())
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
