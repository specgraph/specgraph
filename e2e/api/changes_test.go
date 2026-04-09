// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("CompareVersions", Ordered, func() {
	var (
		client specgraphv1connect.SpecServiceClient
		ctx    context.Context
	)

	const compareSlug = "compare-versions-e2e"

	BeforeAll(func() {
		client = newSpecClient()
		ctx = context.Background()

		// Create spec at v1.
		_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   compareSlug,
			Intent: "Initial intent for compare versions test",
		}))
		Expect(err).NotTo(HaveOccurred())

		// Update intent to produce v2.
		_, err = client.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:   compareSlug,
			Intent: proto.String("Updated intent for compare versions test"),
		}))
		Expect(err).NotTo(HaveOccurred())
	})

	It("compares explicit versions from=1 to=2 and returns an intent diff with hunks", func() {
		resp, err := client.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        compareSlug,
			FromVersion: 1,
			ToVersion:   2,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetFromVersion()).To(Equal(int32(1)))
		Expect(resp.Msg.GetToVersion()).To(Equal(int32(2)))

		// Find the intent diff.
		var intentDiff *specv1.VersionDiff
		for _, d := range resp.Msg.GetDiffs() {
			if d.GetField() == "intent" {
				intentDiff = d
				break
			}
		}
		Expect(intentDiff).NotTo(BeNil(), "expected an intent diff between v1 and v2")
		Expect(intentDiff.GetOldValue()).To(Equal("Initial intent for compare versions test"))
		Expect(intentDiff.GetNewValue()).To(Equal("Updated intent for compare versions test"))
		Expect(intentDiff.GetHunks()).NotTo(BeEmpty(), "expected inline diff hunks")
	})

	It("auto-resolves from=0 to previous version", func() {
		// from=0 means: resolve to toVersion-1.
		resp, err := client.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        compareSlug,
			FromVersion: 0,
			ToVersion:   2,
		}))
		Expect(err).NotTo(HaveOccurred())
		// from=0 with to=2 resolves from to version 1.
		Expect(resp.Msg.GetFromVersion()).To(Equal(int32(1)))
		Expect(resp.Msg.GetToVersion()).To(Equal(int32(2)))
		Expect(resp.Msg.GetDiffs()).NotTo(BeEmpty(), "expected diffs when auto-resolving from version")
	})

	It("auto-resolves to=0 to latest version", func() {
		// to=0 means: resolve to latest version.
		resp, err := client.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        compareSlug,
			FromVersion: 1,
			ToVersion:   0,
		}))
		Expect(err).NotTo(HaveOccurred())
		// to=0 resolves to the latest version (2).
		Expect(resp.Msg.GetToVersion()).To(Equal(int32(2)))
		Expect(resp.Msg.GetFromVersion()).To(Equal(int32(1)))
		Expect(resp.Msg.GetDiffs()).NotTo(BeEmpty(), "expected diffs when auto-resolving to version")
	})

	It("returns CodeNotFound for a version that is too high", func() {
		_, err := client.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        compareSlug,
			FromVersion: 1,
			ToVersion:   9999,
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})

	It("returns CodeNotFound for a nonexistent spec", func() {
		_, err := client.CompareVersions(ctx, connect.NewRequest(&specv1.CompareVersionsRequest{
			Slug:        "nonexistent-spec-compare-xyz",
			FromVersion: 1,
			ToVersion:   2,
		}))
		Expect(err).To(HaveOccurred())
		Expect(connect.CodeOf(err)).To(Equal(connect.CodeNotFound))
	})
})
