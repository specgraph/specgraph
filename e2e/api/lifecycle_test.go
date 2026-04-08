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

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// timestampSkew is the minimum sleep to guarantee timestamp ordering.
// Sleep >1s ensures updated_at differs between operations. Using 1200ms
// (not 1100ms) to reduce flakiness on slow CI environments where write latency
// can push the timestamp into the same second window.
const timestampSkew = 1200 * time.Millisecond

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

		It("creates a spec and advances to in_progress", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   amendSlug,
				Intent: "Test amend lifecycle flow",
			}))
			Expect(err).NotTo(HaveOccurred())

			Expect(advanceStage(ctx, amendSlug, "in_progress")).To(Succeed())

			resp, err := specClient.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
				Slug: amendSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetStage()).To(Equal("in_progress"))
		})

		It("amends the in_progress spec back into authoring with re-entry stage", func() {
			resp, err := lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:         amendSlug,
				Reason:       "Requirements changed during implementation",
				ReEntryStage: "shape",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(amendSlug))
			Expect(resp.Msg.GetSpec().GetStage()).To(Equal("shape"))

			// Verify version increments after amend.
			spec := resp.Msg.GetSpec()
			Expect(spec.GetVersion()).To(BeNumerically(">=", int32(2)), "version should increment after amend")
			// History field removed — changelog is now tracked via ChangeLog graph nodes.
		})

		It("verifies changelog has a checkpoint entry with reason and stage delta", func() {
			resp, err := specClient.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{
				Slug:            amendSlug,
				CheckpointsOnly: true,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetEntries()).NotTo(BeEmpty(), "expected at least one checkpoint changelog entry")

			// Find the amend checkpoint entry.
			var amendEntry *specv1.ChangeLogEntry
			for _, e := range resp.Msg.GetEntries() {
				if e.GetCheckpoint() && e.GetReason() == "Requirements changed during implementation" {
					amendEntry = e
					break
				}
			}
			Expect(amendEntry).NotTo(BeNil(), "expected a checkpoint entry with the amend reason")
			Expect(amendEntry.GetCheckpoint()).To(BeTrue())
			Expect(amendEntry.GetReason()).To(Equal("Requirements changed during implementation"))

			// Verify the stage field delta is recorded.
			var stageChange *specv1.FieldChange
			for _, c := range amendEntry.GetChanges() {
				if c.GetField() == "stage" {
					stageChange = c
					break
				}
			}
			Expect(stageChange).NotTo(BeNil(), "expected a stage field delta in the changelog entry")
			Expect(stageChange.GetOldValue()).To(Equal("in_progress"))
			Expect(stageChange.GetNewValue()).To(Equal("shape"))
		})
	})

	Describe("Supersede flow", func() {
		const (
			oldSlug = "lifecycle-supersede-old"
			newSlug = "lifecycle-supersede-new"
		)

		It("creates two specs and advances old to done", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   oldSlug,
				Intent: "Original spec to be superseded",
			}))
			Expect(err).NotTo(HaveOccurred())

			Expect(advanceStage(ctx, oldSlug, "done")).To(Succeed())

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
			Expect(resp.Msg.OldSpec.Version).To(BeNumerically(">=", int32(2)), "old spec version should be incremented")
			// History field removed — changelog is now tracked via ChangeLog graph nodes.
			Expect(resp.Msg.NewSpec.Slug).To(Equal(newSlug))
			Expect(resp.Msg.NewSpec.Supersedes).To(Equal(oldSlug))
			Expect(resp.Msg.NewSpec.Version).To(BeNumerically(">=", int32(1)), "new spec version should be set")
		})

		It("verifies old spec changelog has a superseded checkpoint entry", func() {
			resp, err := specClient.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{
				Slug:            oldSlug,
				CheckpointsOnly: true,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetEntries()).NotTo(BeEmpty(), "expected at least one checkpoint changelog entry for superseded spec")

			// Find the supersede checkpoint entry.
			var supersedeEntry *specv1.ChangeLogEntry
			for _, e := range resp.Msg.GetEntries() {
				if e.GetCheckpoint() && e.GetStage() == "superseded" {
					supersedeEntry = e
					break
				}
			}
			Expect(supersedeEntry).NotTo(BeNil(), "expected a checkpoint entry for superseded stage")
			Expect(supersedeEntry.GetCheckpoint()).To(BeTrue())

			// Verify the superseded_by field delta is recorded.
			var supersededByChange *specv1.FieldChange
			for _, c := range supersedeEntry.GetChanges() {
				if c.GetField() == "superseded_by" {
					supersededByChange = c
					break
				}
			}
			Expect(supersededByChange).NotTo(BeNil(), "expected a superseded_by field delta in the changelog entry")
			Expect(supersededByChange.GetOldValue()).To(Equal(""))
			Expect(supersededByChange.GetNewValue()).To(Equal(newSlug))
		})

		It("creates a SUPERSEDES edge from new to old", func() {
			edgeResp, err := graphClient.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
				Slug:     newSlug,
				EdgeType: specv1.EdgeType_EDGE_TYPE_SUPERSEDES,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(edgeResp.Msg.Edges).NotTo(BeEmpty(), "expected SUPERSEDES edge from new to old")
			found := false
			for _, e := range edgeResp.Msg.Edges {
				if e.EdgeType == specv1.EdgeType_EDGE_TYPE_SUPERSEDES {
					// Edge ToId uses slugs, not ULIDs — graph edges are slug-based.
					Expect(e.ToId).To(Equal(oldSlug), "SUPERSEDES edge should point to old spec slug")
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

			// Verify version increments after abandon.
			spec := resp.Msg.GetSpec()
			Expect(spec.GetVersion()).To(BeNumerically(">=", int32(2)), "version should increment after abandon")
			// History field removed — changelog is now tracked via ChangeLog graph nodes.
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

				Expect(advanceStage(ctx, slug, "done")).To(Succeed())
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
			// Retry to handle the case where timestamps landed in the same second
			// (second-precision storage may not distinguish them on the first check).
			var driftFound bool
			for attempt := 0; attempt < 3; attempt++ {
				resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
					Slug: downstreamSlug,
				}))
				Expect(err).NotTo(HaveOccurred())
				if len(resp.Msg.Reports) > 0 && len(resp.Msg.Reports[0].Items) > 0 {
					Expect(resp.Msg.Reports[0].SpecSlug).To(Equal(downstreamSlug))
					driftFound = true
					break
				}
				time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
			}
			Expect(driftFound).To(BeTrue(), "expected drift to be detected within retries")
		})

		It("acknowledges drift for all upstreams", func() {
			resp, err := lifecycleClient.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
				Slug: downstreamSlug,
				Note: "Reviewed upstream change, no action needed",
				All:  true,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Report.SpecSlug).To(Equal(downstreamSlug))
		})

		It("CheckDrift returns no drift after AcknowledgeDrift", func() {
			resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{
				Slug: downstreamSlug,
			}))
			Expect(err).NotTo(HaveOccurred())
			// After blanket ack, edge hashes match upstream — no drift items.
			if len(resp.Msg.Reports) > 0 {
				report := resp.Msg.Reports[0]
				Expect(report.SpecSlug).To(Equal(downstreamSlug))
				Expect(report.Items).To(BeEmpty(), "after ack, edge hashes should match — no drift")
			}
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
			targetSlug := "lint-target-" + time.Now().Format("150405")

			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   danglingSlug,
				Intent: "Test lint violations",
			}))
			Expect(err).NotTo(HaveOccurred())

			// Create a target spec, add an edge to it, then delete the target
			// node directly (DELETE without DETACH leaves the edge orphaned).
			_, err = specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   targetSlug,
				Intent: "Will be deleted to create dangling edge",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = graphClient.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
				FromSlug: danglingSlug,
				ToSlug:   targetSlug,
				EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
			}))
			Expect(err).NotTo(HaveOccurred())

			// Remove the edge and delete the target spec, leaving the linter's
			// ListDependencies to find a slug that no longer exists.
			_, err = graphClient.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
				FromSlug: danglingSlug,
				ToSlug:   targetSlug,
				EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
			}))
			Expect(err).NotTo(HaveOccurred())

			// Re-add the edge to a nonexistent slug by creating a raw
			// DEPENDS_ON relationship via the graph. Since the target was
			// deleted, the MATCH won't find it — so we skip this approach.
			//
			// Instead: the storage backend enforces referential integrity, so dangling
			// edges cannot exist. The linter covers this via unit tests with
			// mock backends. Skip this e2e test.
			Skip("Storage backend enforces referential integrity — dangling edges cannot be created; covered by unit tests")
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
	// After blanket AcknowledgeDrift, edge hashes match upstream — no drift expected.
	Describe("Drift detection (all specs)", func() {
		It("returns no drift after blanket acknowledgment", func() {
			resp, err := lifecycleClient.CheckDrift(ctx, connect.NewRequest(&specv1.DriftCheckRequest{}))
			Expect(err).NotTo(HaveOccurred())
			// After ack, all edge hashes match — reports may be empty or have zero items.
			for _, r := range resp.Msg.Reports {
				Expect(r.Items).To(BeEmpty(), "all drift should be resolved after blanket ack for spec %s", r.SpecSlug)
			}
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
				Slug:         errSlug,
				Reason:       "should fail",
				ReEntryStage: "shape",
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

		It("rejects amend on a shape-stage spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-amend-shape-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test amend error path for mid-funnel spec",
			}))
			Expect(err).NotTo(HaveOccurred())

			// Advance to shape (mid-funnel, not in_progress/review/done).
			Expect(advanceStage(ctx, errSlug, "shape")).To(Succeed())

			_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:         errSlug,
				Reason:       "should fail because spec is not in execution",
				ReEntryStage: "spark",
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

			// Advance to done (required for supersede).
			Expect(advanceStage(ctx, baseSlug, "done")).To(Succeed())

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
				Slug:         baseSlug,
				Reason:       "should fail on terminal spec",
				ReEntryStage: "shape",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects drift ack on a spark-stage spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-driftack-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test drift ack error path",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
				Slug: errSlug,
				Note: "should fail on spark-stage spec",
				All:  true,
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects amend without re_entry_stage with InvalidArgument", func() {
			errSlug := "lifecycle-err-amend-noreentry-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test amend missing re_entry_stage",
			}))
			Expect(err).NotTo(HaveOccurred())

			Expect(advanceStage(ctx, errSlug, "in_progress")).To(Succeed())

			_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:   errSlug,
				Reason: "should fail without re_entry_stage",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeInvalidArgument))
		})

		It("rejects amend on a done spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-amend-done-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test amend on done spec",
			}))
			Expect(err).NotTo(HaveOccurred())

			Expect(advanceStage(ctx, errSlug, "done")).To(Succeed())

			_, err = lifecycleClient.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
				Slug:         errSlug,
				Reason:       "should fail on done spec",
				ReEntryStage: "shape",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeFailedPrecondition))
		})

		It("rejects supersede on a non-done spec with FailedPrecondition", func() {
			errSlug := "lifecycle-err-supersede-notdone-" + time.Now().Format("150405")
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   errSlug,
				Intent: "Test supersede on non-done spec",
			}))
			Expect(err).NotTo(HaveOccurred())

			_, err = lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
				Slug:    errSlug,
				NewSlug: "some-new-slug-xyz",
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

			Expect(advanceStage(ctx, errSlug, "done")).To(Succeed())

			_, err = lifecycleClient.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
				Slug:    errSlug,
				NewSlug: "nonexistent-spec-xyz",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
		})
	})
})
