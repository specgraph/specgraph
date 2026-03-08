// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Lifecycle", Ordered, func() {
	var (
		lifecycleClient specgraphv1connect.LifecycleServiceClient
		specClient      specgraphv1connect.SpecServiceClient
		graphClient     specgraphv1connect.GraphServiceClient
		ctx             context.Context
	)

	BeforeAll(func() {
		lifecycleClient = newLifecycleClient()
		specClient = newSpecClient()
		graphClient = newGraphClient()
		ctx = context.Background()
	})

	Describe("Amend flow", func() {
		const amendSlug = "lifecycle-amend-spec"

		It("creates a spec and advances to done", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   amendSlug,
				Intent: "Test amend lifecycle flow",
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
				Slug:  amendSlug,
				Stage: proto.String("done"),
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Stage).To(Equal("done"))
		})

		It("amends the done spec back into authoring", func() {
			resp, err := lifecycleClient.Amend(ctx, connect.NewRequest(&specv1.LifecycleAmendRequest{
				Slug:         amendSlug,
				Reason:       "Requirements changed after implementation",
				ReEntryStage: "shape",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Slug).To(Equal(amendSlug))
			Expect(resp.Msg.Stage).To(Equal("shape"))
		})
	})

	Describe("Supersede flow", func() {
		const (
			oldSlug = "lifecycle-supersede-old"
			newSlug = "lifecycle-supersede-new"
		)

		It("creates two specs", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   oldSlug,
				Intent: "Original spec to be superseded",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   newSlug,
				Intent: "Replacement spec",
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("supersedes old with new", func() {
			resp, err := lifecycleClient.Supersede(ctx, connect.NewRequest(&specv1.LifecycleSupersedeRequest{
				Slug:    oldSlug,
				NewSlug: newSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.OldSpec).NotTo(BeNil())
			Expect(resp.Msg.NewSpec).NotTo(BeNil())
			Expect(resp.Msg.OldSpec.Slug).To(Equal(oldSlug))
			Expect(resp.Msg.OldSpec.Stage).To(Equal("superseded"))
			Expect(resp.Msg.NewSpec.Slug).To(Equal(newSlug))
		})
	})

	Describe("Abandon flow", func() {
		const abandonSlug = "lifecycle-abandon-spec"

		It("creates a spec and abandons it", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   abandonSlug,
				Intent: "Test abandon lifecycle flow",
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := lifecycleClient.Abandon(ctx, connect.NewRequest(&specv1.LifecycleAbandonRequest{
				Slug:   abandonSlug,
				Reason: "No longer needed",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Slug).To(Equal(abandonSlug))
			Expect(resp.Msg.Stage).To(Equal("abandoned"))
		})
	})

	Describe("Drift detection", func() {
		const (
			upstreamSlug   = "lifecycle-drift-upstream"
			downstreamSlug = "lifecycle-drift-downstream"
		)

		It("creates two specs, advances to done, and adds a dependency", func() {
			for _, slug := range []string{upstreamSlug, downstreamSlug} {
				_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
					Slug:   slug,
					Intent: "Drift detection test spec " + slug,
				}))
				Expect(err).NotTo(HaveOccurred())

				_, err = specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
					Slug:  slug,
					Stage: proto.String("done"),
				}))
				Expect(err).NotTo(HaveOccurred())
			}

			// downstream DEPENDS_ON upstream
			_, err := graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
				FromSlug: downstreamSlug,
				ToSlug:   upstreamSlug,
				EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates upstream to trigger drift", func() {
			_, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
				Slug:   upstreamSlug,
				Intent: proto.String("Updated upstream intent to trigger drift"),
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("detects drift on downstream spec", func() {
			resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
				Slug: downstreamSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Reports).NotTo(BeEmpty())
			Expect(resp.Msg.Reports[0].SpecSlug).To(Equal(downstreamSlug))
			Expect(resp.Msg.Reports[0].Items).NotTo(BeEmpty())
		})

		It("acknowledges drift", func() {
			resp, err := lifecycleClient.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
				Slug: downstreamSlug,
				Note: "Reviewed upstream change, no action needed",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Acknowledged).To(BeTrue())
			Expect(resp.Msg.AcknowledgeNote).To(Equal("Reviewed upstream change, no action needed"))
		})
	})

	Describe("Lint", func() {
		It("returns Unimplemented", func() {
			_, err := lifecycleClient.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
				Slug: "lifecycle-amend-spec",
			}))
			Expect(err).To(HaveOccurred())

			var connErr *connect.Error
			Expect(errors.As(err, &connErr)).To(BeTrue())
			Expect(connErr.Code()).To(Equal(connect.CodeUnimplemented))
		})
	})
})
