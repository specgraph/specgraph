// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"time"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// timestampSkew is the minimum sleep to guarantee Memgraph datetime ordering.
// Memgraph stores datetime at second precision; sleep >1s ensures updated_at differs.
const timestampSkew = 1100 * time.Millisecond

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

		It("amends the done spec back into authoring with re-entry stage", func() {
			resp, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:         amendSlug,
				Reason:       "Requirements changed after implementation",
				ReEntryStage: "shape",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(amendSlug))
			Expect(resp.Msg.GetSpec().GetStage()).To(Equal("shape"))

			// Finding .16: verify history and version through proto conversion.
			spec := resp.Msg.GetSpec()
			Expect(spec.GetVersion()).To(BeNumerically(">=", int32(2)), "version should increment after amend")
			Expect(spec.GetHistory()).NotTo(BeEmpty(), "history should contain at least one entry")
			lastEntry := spec.GetHistory()[len(spec.GetHistory())-1]
			Expect(lastEntry.GetReason()).To(Equal("Requirements changed after implementation"))
		})
	})

	Describe("Amend flow (default stage)", func() {
		const amendDefaultSlug = "lifecycle-amend-default"

		It("creates a spec and advances to done", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   amendDefaultSlug,
				Intent: "Test amend with default stage",
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
				Slug:  amendDefaultSlug,
				Stage: proto.String("done"),
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Stage).To(Equal("done"))
		})

		It("amends the done spec to amended stage when no re-entry stage specified", func() {
			resp, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:   amendDefaultSlug,
				Reason: "Needs revision",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(amendDefaultSlug))
			Expect(resp.Msg.GetSpec().GetStage()).To(Equal("amended"))
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
			resp, err := lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
				Slug:    oldSlug,
				NewSlug: newSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.OldSpec).NotTo(BeNil())
			Expect(resp.Msg.NewSpec).NotTo(BeNil())
			Expect(resp.Msg.OldSpec.Slug).To(Equal(oldSlug))
			Expect(resp.Msg.OldSpec.Stage).To(Equal("superseded"))
			Expect(resp.Msg.OldSpec.SupersededBy).To(Equal(newSlug))
			Expect(resp.Msg.NewSpec.Slug).To(Equal(newSlug))
			Expect(resp.Msg.NewSpec.Supersedes).To(Equal(oldSlug))
		})

		It("creates a SUPERSEDES edge from new to old", func() {
			oldSpecResp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
				Slug: oldSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			oldSpecID := oldSpecResp.Msg.Id

			edgeResp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
				Slug:     newSlug,
				EdgeType: specv1.EdgeType_EDGE_TYPE_SUPERSEDES,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(edgeResp.Msg.Edges).NotTo(BeEmpty(), "expected SUPERSEDES edge from new to old")
			found := false
			for _, e := range edgeResp.Msg.Edges {
				if e.EdgeType == specv1.EdgeType_EDGE_TYPE_SUPERSEDES {
					Expect(e.ToId).To(Equal(oldSpecID), "SUPERSEDES edge should point to old spec")
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "SUPERSEDES edge not found")
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

			resp, err := lifecycleClient.TransitionAbandon(ctx, connect.NewRequest(&specv1.TransitionAbandonRequest{
				Slug:   abandonSlug,
				Reason: "No longer needed",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(abandonSlug))
			Expect(resp.Msg.GetSpec().GetStage()).To(Equal("abandoned"))

			// Finding .16: verify history and version through proto conversion.
			spec := resp.Msg.GetSpec()
			Expect(spec.GetVersion()).To(BeNumerically(">=", int32(2)), "version should increment after abandon")
			Expect(spec.GetHistory()).NotTo(BeEmpty(), "history should contain at least one entry")
			lastEntry := spec.GetHistory()[len(spec.GetHistory())-1]
			Expect(lastEntry.GetReason()).To(Equal("No longer needed"))
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
			// Drift detection compares updated_at timestamps. Sleep >1s to
			// guarantee upstream's timestamp is strictly newer than downstream's,
			// matching the integration test pattern in lifecycle_test.go.
			time.Sleep(timestampSkew)

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
			Expect(resp.Msg.Report.Acknowledged).To(BeTrue())
			Expect(resp.Msg.Report.AcknowledgeNote).To(Equal("Reviewed upstream change, no action needed"))
		})

		It("CheckDrift returns acknowledged state after AcknowledgeDrift", func() {
			resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
				Slug: downstreamSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Reports).NotTo(BeEmpty())
			report := resp.Msg.Reports[0]
			Expect(report.SpecSlug).To(Equal(downstreamSlug))
			Expect(report.Acknowledged).To(BeTrue(), "CheckDrift should reflect acknowledged state set by AcknowledgeDrift")
			Expect(report.AcknowledgeNote).To(Equal("Reviewed upstream change, no action needed"))
		})
	})

	Describe("Lint", Ordered, func() {
		const lintSlug = "lifecycle-lint-spec"

		BeforeAll(func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   lintSlug,
				Intent: "Test lint",
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns lint results for a valid spec", func() {
			resp, err := lifecycleClient.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
				Slug: lintSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Results).NotTo(BeEmpty())
			Expect(resp.Msg.Results[0].SpecSlug).To(Equal(lintSlug))
			Expect(resp.Msg.Results[0].Passed).To(BeTrue(), "valid spec should pass lint")
		})

		It("returns violations for a spec with a dangling dependency", func() {
			danglingSlug := "lint-dangling-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   danglingSlug,
				Intent: "Test lint violations",
			}))
			Expect(err).NotTo(HaveOccurred())

			// Add a DEPENDS_ON edge to a nonexistent spec.
			_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
				FromSlug: danglingSlug,
				ToSlug:   "nonexistent-spec",
				EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := lifecycleClient.Lint(ctx, connect.NewRequest(&specv1.LintRequest{
				Slug: danglingSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Results).NotTo(BeEmpty())
			Expect(resp.Msg.Results[0].Passed).To(BeFalse(), "spec with dangling dep should fail lint")

			hasDangling := false
			for _, v := range resp.Msg.Results[0].Violations {
				if v.Rule == "edge.dangling_ref" {
					hasDangling = true
				}
			}
			Expect(hasDangling).To(BeTrue(), "expected edge.dangling_ref violation")
		})
	})

	Describe("Lint all specs", func() {
		It("returns results for all specs when slug is empty", func() {
			resp, err := lifecycleClient.Lint(ctx, connect.NewRequest(&specv1.LintRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Results).NotTo(BeEmpty(), "lint-all should return results for existing specs")
		})
	})

	// Depends on "Drift detection" Describe above (order guaranteed by outer Ordered container).
	// That block creates two done specs with a dependency and bumps upstream's updated_at.
	Describe("Drift detection (all specs)", func() {
		It("detects drift across all eligible specs when slug is empty", func() {
			resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Reports).NotTo(BeEmpty(), "drift-all should find at least one drifted spec")
		})
	})

	Describe("Error paths", func() {
		It("rejects amend on a spark-stage spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-amend-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test amend error path",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:   errSlug,
				Reason: "should fail",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects abandon on an already-abandoned spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-abandon-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test abandon error path",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.TransitionAbandon(ctx, connect.NewRequest(&specv1.TransitionAbandonRequest{
				Slug:   errSlug,
				Reason: "first abandon",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.TransitionAbandon(ctx, connect.NewRequest(&specv1.TransitionAbandonRequest{
				Slug:   errSlug,
				Reason: "second abandon should fail",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects amend on a superseded (terminal) spec with FailedPrecondition", func() {
			baseSlug := "lifecycle-err-amend-terminal-" + time.Now().Format("150405")
			newSlug := baseSlug + "-v2"

			// Create two specs: the original and the replacement.
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   baseSlug,
				Intent: "Original spec",
			}))
			Expect(err).NotTo(HaveOccurred())
			_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   newSlug,
				Intent: "Replacement spec",
			}))
			Expect(err).NotTo(HaveOccurred())

			// Supersede the original spec (makes it terminal).
			_, err = lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
				Slug:    baseSlug,
				NewSlug: newSlug,
			}))
			Expect(err).NotTo(HaveOccurred())

			// Attempt to amend the superseded spec — should fail.
			_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:   baseSlug,
				Reason: "should fail on terminal spec",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects supersede with nonexistent new spec with NotFound", func() {
			errSlug := "lifecycle-err-supersede-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test supersede error path",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
				Slug:    errSlug,
				NewSlug: "nonexistent-spec-xyz",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
		})
	})
})
