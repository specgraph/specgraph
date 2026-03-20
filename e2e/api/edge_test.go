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

var _ = Describe("graph edges", Ordered, func() {
	var (
		specClient  specgraphv1connect.SpecServiceClient
		graphClient specgraphv1connect.GraphServiceClient
		ctx         context.Context
	)

	BeforeAll(func() {
		specClient = newSpecClient()
		graphClient = newGraphClient()
		ctx = context.Background()

		// Create prerequisite specs for edge tests.
		for _, slug := range []string{"edge-a", "edge-b", "edge-c"} {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:     slug,
				Intent:   "Edge test spec " + slug,
				Priority: "p2",
			}))
			Expect(err).NotTo(HaveOccurred())
		}
	})

	It("adds a DEPENDS_ON edge between specs", func() {
		resp, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "edge-a",
			ToSlug:   "edge-b",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		edge := resp.Msg
		Expect(edge.FromId).NotTo(BeEmpty())
		Expect(edge.ToId).NotTo(BeEmpty())
		Expect(edge.EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
	})

	It("adds a COMPOSES edge", func() {
		resp, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "edge-a",
			ToSlug:   "edge-c",
			EdgeType: specv1.EdgeType_EDGE_TYPE_COMPOSES,
		}))
		Expect(err).NotTo(HaveOccurred())

		edge := resp.Msg
		Expect(edge.FromId).NotTo(BeEmpty())
		Expect(edge.ToId).NotTo(BeEmpty())
		Expect(edge.EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_COMPOSES))
	})

	It("lists edges for a spec", func() {
		resp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug: "edge-a",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Edges).To(HaveLen(2))
	})

	It("lists edges filtered by type", func() {
		resp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug:     "edge-a",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Edges).To(HaveLen(1))
		Expect(resp.Msg.Edges[0].EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
	})

	It("removes an edge", func() {
		_, err := graphClient.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
			FromSlug: "edge-a",
			ToSlug:   "edge-c",
			EdgeType: specv1.EdgeType_EDGE_TYPE_COMPOSES,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Verify only 1 edge remains.
		resp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug: "edge-a",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Edges).To(HaveLen(1))
		Expect(resp.Msg.Edges[0].EdgeType).To(Equal(specv1.EdgeType_EDGE_TYPE_DEPENDS_ON))
	})
})
