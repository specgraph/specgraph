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

var _ = Describe("Decision", Ordered, func() {
	var (
		client specgraphv1connect.DecisionServiceClient
		ctx    context.Context
	)

	BeforeAll(func() {
		client = specgraphv1connect.NewDecisionServiceClient(http.DefaultClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("creates a decision with slug and fields", func() {
		resp, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
			Slug:      "use-memgraph",
			Title:     "Use Memgraph for graph storage",
			Decision:  "We will use Memgraph as the primary graph database.",
			Rationale: "Memgraph offers low-latency Cypher queries and good container support.",
		}))
		Expect(err).NotTo(HaveOccurred())

		d := resp.Msg
		Expect(d.Slug).To(Equal("use-memgraph"))
		Expect(d.Title).To(Equal("Use Memgraph for graph storage"))
		Expect(d.Decision).To(Equal("We will use Memgraph as the primary graph database."))
		Expect(d.Rationale).To(Equal("Memgraph offers low-latency Cypher queries and good container support."))
		Expect(d.Status).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
		Expect(d.Id).NotTo(BeEmpty())
		Expect(d.CreatedAt).NotTo(BeNil())
	})

	It("lists decisions", func() {
		resp, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Decisions).NotTo(BeEmpty())

		slugs := make([]string, len(resp.Msg.Decisions))
		for i, d := range resp.Msg.Decisions {
			slugs[i] = d.Slug
		}
		Expect(slugs).To(ContainElement("use-memgraph"))
	})

	It("shows a decision by slug", func() {
		resp, err := client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "use-memgraph",
		}))
		Expect(err).NotTo(HaveOccurred())

		d := resp.Msg
		Expect(d.Slug).To(Equal("use-memgraph"))
		Expect(d.Title).To(Equal("Use Memgraph for graph storage"))
		Expect(d.Decision).To(Equal("We will use Memgraph as the primary graph database."))
		Expect(d.Rationale).To(Equal("Memgraph offers low-latency Cypher queries and good container support."))
		Expect(d.Status).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
		Expect(d.Id).NotTo(BeEmpty())
	})
})
