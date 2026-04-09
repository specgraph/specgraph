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

var _ = Describe("Decision", Ordered, func() {
	var (
		client specgraphv1connect.DecisionServiceClient
		ctx    context.Context
	)

	BeforeAll(func() {
		client = newDecisionClient()
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

		d := resp.Msg.GetDecision()
		Expect(d.GetSlug()).To(Equal("use-memgraph"))
		Expect(d.GetTitle()).To(Equal("Use Memgraph for graph storage"))
		Expect(d.GetDecision()).To(Equal("We will use Memgraph as the primary graph database."))
		Expect(d.GetRationale()).To(Equal("Memgraph offers low-latency Cypher queries and good container support."))
		Expect(d.GetStatus()).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
		Expect(d.GetId()).NotTo(BeEmpty())
		Expect(d.GetCreatedAt()).NotTo(BeNil())
	})

	It("lists decisions", func() {
		resp, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.Decisions).NotTo(BeEmpty())

		slugs := make([]string, len(resp.Msg.Decisions))
		for i, d := range resp.Msg.Decisions {
			slugs[i] = d.GetSlug()
		}
		Expect(slugs).To(ContainElement("use-memgraph"))
	})

	It("shows a decision by slug", func() {
		resp, err := client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "use-memgraph",
		}))
		Expect(err).NotTo(HaveOccurred())

		d := resp.Msg.GetDecision()
		Expect(d.GetSlug()).To(Equal("use-memgraph"))
		Expect(d.GetTitle()).To(Equal("Use Memgraph for graph storage"))
		Expect(d.GetDecision()).To(Equal("We will use Memgraph as the primary graph database."))
		Expect(d.GetRationale()).To(Equal("Memgraph offers low-latency Cypher queries and good container support."))
		Expect(d.GetStatus()).To(Equal(specv1.DecisionStatus_DECISION_STATUS_PROPOSED))
		Expect(d.GetId()).NotTo(BeEmpty())
	})
})
