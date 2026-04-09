// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Claim protocol", Ordered, func() {
	const (
		claimSlug  = "claim-test-spec"
		claimAgent = "e2e-agent-1"
	)

	var (
		specClient  specgraphv1connect.SpecServiceClient
		claimClient specgraphv1connect.ClaimServiceClient
		ctx         context.Context
	)

	BeforeAll(func() {
		specClient = newSpecClient()
		claimClient = newClaimClient()
		ctx = context.Background()
	})

	It("creates a spec and advances it to approved", func() {
		// Create the spec.
		createResp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   claimSlug,
			Intent: "Test the claim protocol end-to-end",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(createResp.Msg.GetSpec().GetSlug()).To(Equal(claimSlug))
		Expect(createResp.Msg.GetSpec().GetStage()).To(Equal("spark"))

		// Advance through authoring funnel to approved.
		Expect(advanceStage(ctx, claimSlug, "approved")).To(Succeed())
	})

	It("claims the approved spec", func() {
		resp, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      claimSlug,
			Agent:         claimAgent,
			LeaseDuration: durationpb.New(60_000_000_000), // 1 minute
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetClaim().GetSpecSlug()).To(Equal(claimSlug))
		Expect(resp.Msg.GetClaim().GetAgent()).To(Equal(claimAgent))
		Expect(resp.Msg.GetClaim().GetClaimedAt()).NotTo(BeNil())
		Expect(resp.Msg.GetClaim().GetLeaseExpires()).NotTo(BeNil())
	})

	It("rejects a double-claim by a different agent", func() {
		_, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug: claimSlug,
			Agent:    "e2e-agent-2",
		}))
		Expect(err).To(HaveOccurred())

		var connectErr *connect.Error
		Expect(err).To(BeAssignableToTypeOf(connectErr))
		connectErr = err.(*connect.Error)
		Expect(connectErr.Code()).To(Equal(connect.CodeFailedPrecondition))
	})

	It("unclaims the spec", func() {
		_, err := claimClient.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
			SpecSlug: claimSlug,
			Agent:    claimAgent,
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("allows re-claim after unclaim", func() {
		resp, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug: claimSlug,
			Agent:    "e2e-agent-2",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetClaim().GetAgent()).To(Equal("e2e-agent-2"))

		// Clean up: unclaim so other tests are unaffected.
		_, err = claimClient.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
			SpecSlug: claimSlug,
			Agent:    "e2e-agent-2",
		}))
		Expect(err).NotTo(HaveOccurred())
	})
})
