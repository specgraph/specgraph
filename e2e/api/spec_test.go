// SPDX-License-Identifier: MIT
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

var _ = Describe("Spec lifecycle", Ordered, func() {
	var (
		client specgraphv1connect.SpecServiceClient
		ctx    context.Context
	)

	BeforeAll(func() {
		client = newSpecClient()
		ctx = context.Background()
	})

	It("creates a spec with slug, intent, and priority", func() {
		resp, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:     "oauth-refresh-rotation",
			Intent:   "Rotate OAuth refresh tokens on each use to limit replay window",
			Priority: "p1",
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := resp.Msg
		Expect(spec.Slug).To(Equal("oauth-refresh-rotation"))
		Expect(spec.Intent).To(Equal("Rotate OAuth refresh tokens on each use to limit replay window"))
		Expect(spec.Priority).To(Equal("p1"))
		Expect(spec.Stage).To(Equal("spark"))
		Expect(spec.Id).NotTo(BeEmpty())
		Expect(spec.Version).To(BeNumerically(">=", 1))
		Expect(spec.CreatedAt).NotTo(BeNil())
	})

	It("lists specs and the created spec appears", func() {
		resp, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		Expect(err).NotTo(HaveOccurred())

		slugs := make([]string, len(resp.Msg.Specs))
		for i, s := range resp.Msg.Specs {
			slugs[i] = s.Slug
		}
		Expect(slugs).To(ContainElement("oauth-refresh-rotation"))
	})

	It("shows spec details by slug", func() {
		resp, err := client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "oauth-refresh-rotation",
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := resp.Msg
		Expect(spec.Slug).To(Equal("oauth-refresh-rotation"))
		Expect(spec.Intent).To(Equal("Rotate OAuth refresh tokens on each use to limit replay window"))
		Expect(spec.Priority).To(Equal("p1"))
		Expect(spec.Stage).To(Equal("spark"))
	})

	It("updates spec fields", func() {
		resp, err := client.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:     "oauth-refresh-rotation",
			Intent:   proto.String("Rotate OAuth refresh tokens on each use to prevent replay attacks"),
			Priority: proto.String("p0"),
		}))
		Expect(err).NotTo(HaveOccurred())

		spec := resp.Msg
		Expect(spec.Slug).To(Equal("oauth-refresh-rotation"))
		Expect(spec.Intent).To(Equal("Rotate OAuth refresh tokens on each use to prevent replay attacks"))
		Expect(spec.Priority).To(Equal("p0"))
		// Stage should remain unchanged
		Expect(spec.Stage).To(Equal("spark"))
	})
})
