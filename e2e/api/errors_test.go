// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("error handling", func() {
	var (
		specClient  specgraphv1connect.SpecServiceClient
		claimClient specgraphv1connect.ClaimServiceClient
		graphClient specgraphv1connect.GraphServiceClient
		ctx         context.Context
	)

	BeforeEach(func() {
		specClient = specgraphv1connect.NewSpecServiceClient(http.DefaultClient, serverInfo.BaseURL)
		claimClient = specgraphv1connect.NewClaimServiceClient(http.DefaultClient, serverInfo.BaseURL)
		graphClient = specgraphv1connect.NewGraphServiceClient(http.DefaultClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("returns error for nonexistent spec slug", func() {
		_, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "totally-nonexistent-spec",
		}))
		Expect(err).To(HaveOccurred())
	})

	It("rejects creating a spec with empty slug", func() {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "",
			Intent: "Missing slug",
		}))
		Expect(err).To(HaveOccurred())
	})

	It("rejects claim on unapproved spec", func() {
		// Create a spec (starts in spark stage, not approved).
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "err-unapproved-claim",
			Intent: "Test unapproved claim rejection",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Try to claim it — should fail because it's in spark stage, not approved.
		_, err = claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      "err-unapproved-claim",
			Agent:         "test-agent",
			LeaseDuration: durationpb.New(15 * time.Minute),
		}))
		Expect(err).To(HaveOccurred())

		var connectErr *connect.Error
		Expect(errors.As(err, &connectErr)).To(BeTrue())
		Expect(connectErr.Code()).To(Equal(connect.CodeFailedPrecondition))
	})

	It("detects edge cycles", func() {
		// Create two specs.
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "err-cycle-a",
			Intent: "Cycle test A",
		}))
		Expect(err).NotTo(HaveOccurred())

		_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "err-cycle-b",
			Intent: "Cycle test B",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Add edge A -> B.
		_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "err-cycle-a",
			ToSlug:   "err-cycle-b",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Add edge B -> A (creates cycle) — should error.
		_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "err-cycle-b",
			ToSlug:   "err-cycle-a",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		Expect(err).To(HaveOccurred())
	})
})
