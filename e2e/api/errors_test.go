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
		ctx         context.Context
	)

	BeforeEach(func() {
		specClient = specgraphv1connect.NewSpecServiceClient(http.DefaultClient, serverInfo.BaseURL)
		claimClient = specgraphv1connect.NewClaimServiceClient(http.DefaultClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("returns error for nonexistent spec slug", func() {
		_, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "totally-nonexistent-spec",
		}))
		Expect(err).To(HaveOccurred())
	})

	It("returns not-found when claiming nonexistent spec", func() {
		_, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      "err-no-such-spec-for-claim",
			Agent:         "test-agent",
			LeaseDuration: durationpb.New(15 * time.Minute),
		}))
		Expect(err).To(HaveOccurred())

		var connectErr *connect.Error
		Expect(errors.As(err, &connectErr)).To(BeTrue())
		Expect(connectErr.Code()).To(Equal(connect.CodeNotFound))
	})

	It("rejects double claim on same spec", func() {
		// Create and approve a spec for claiming.
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   "err-double-claim",
			Intent: "Test double claim rejection",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Advance to approved stage.
		approved := "approved"
		_, err = specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:  "err-double-claim",
			Stage: &approved,
		}))
		Expect(err).NotTo(HaveOccurred())

		// First claim succeeds.
		_, err = claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      "err-double-claim",
			Agent:         "agent-1",
			LeaseDuration: durationpb.New(15 * time.Minute),
		}))
		Expect(err).NotTo(HaveOccurred())

		// Second claim by different agent fails.
		_, err = claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug:      "err-double-claim",
			Agent:         "agent-2",
			LeaseDuration: durationpb.New(15 * time.Minute),
		}))
		Expect(err).To(HaveOccurred())

		var connectErr *connect.Error
		Expect(errors.As(err, &connectErr)).To(BeTrue())
		Expect(connectErr.Code()).To(Equal(connect.CodeFailedPrecondition))
	})

	It("returns error for invalid stage transition via authoring", func() {
		authoringClient := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, serverInfo.BaseURL)

		// Spark a spec.
		_, err := authoringClient.Spark(ctx, connect.NewRequest(&specv1.SparkRequest{
			Slug: "err-bad-transition",
			Output: &specv1.SparkOutput{
				Seed:   "bad transition test",
				Signal: "testing",
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Try to Specify without going through Shape first — should fail.
		_, err = authoringClient.Specify(ctx, connect.NewRequest(&specv1.SpecifyRequest{
			Slug: "err-bad-transition",
			Output: &specv1.SpecifyOutput{
				InterfaceContract: "POST /api",
				VerifyCriteria:    []string{"returns 200"},
			},
			Posture: specv1.Posture_POSTURE_DRIVE,
		}))
		Expect(err).To(HaveOccurred())
	})
})
