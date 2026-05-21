// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("RefreshConstitutionLayer", func() {
	// Each It block uses a different layer so they can run independently in
	// the shared-DB e2e suite without state pollution.
	var (
		client specgraphv1connect.ConstitutionServiceClient
		ctx    context.Context
	)

	BeforeEach(func() {
		// Use a dedicated project to avoid polluting the main e2e-test project.
		client = specgraphv1connect.NewConstitutionServiceClient(
			projectClientFor("const-refresh-project"),
			serverInfo.BaseURL,
		)
		ctx = context.Background()
	})

	It("first refresh creates the layer with changed=true and no Before", func() {
		var body atomic.Value
		body.Store([]byte(`name: e2e-initial-org
layer: org
principles:
  - id: e2e-p1
    statement: E2E principle
`))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			b := body.Load().([]byte)
			_, _ = w.Write(b)
		}))
		defer srv.Close()

		resp, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: srv.URL + "/constitution.yaml",
		}))

		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Msg.GetChanged()).To(BeTrue(), "first refresh must report changed")
		Expect(resp.Msg.GetBefore()).To(BeNil(), "no prior layer; Before should be nil")
		Expect(resp.Msg.GetAfter().GetName()).To(Equal("e2e-initial-org"))
		Expect(resp.Msg.GetNewSourceHash()).NotTo(BeEmpty())
	})

	It("second refresh on unchanged content reports changed=false", func() {
		// Uses project layer to avoid collision with the org layer written above.
		var body atomic.Value
		body.Store([]byte(`name: e2e-initial-project
layer: project
principles:
  - id: e2e-p1
    statement: E2E principle project
`))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			b := body.Load().([]byte)
			_, _ = w.Write(b)
		}))
		defer srv.Close()

		url := srv.URL + "/constitution.yaml"

		first, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			SourceUrl: url,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(first.Msg.GetChanged()).To(BeTrue())

		second, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			SourceUrl: url,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Msg.GetChanged()).To(BeFalse(), "unchanged content must report not-changed")
		Expect(second.Msg.GetNewSourceHash()).To(Equal(first.Msg.GetNewSourceHash()))
	})

	It("refresh after content change reports changed=true with Before populated", func() {
		// Uses domain layer to avoid collision with org/project layers written above.
		var body atomic.Value
		body.Store([]byte(`name: e2e-initial-domain
layer: domain
principles:
  - id: e2e-p1
    statement: E2E domain principle
`))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			b := body.Load().([]byte)
			_, _ = w.Write(b)
		}))
		defer srv.Close()

		url := srv.URL + "/constitution.yaml"

		first, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
			SourceUrl: url,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(first.Msg.GetChanged()).To(BeTrue())

		// Mutate the served content.
		body.Store([]byte(`name: e2e-modified-domain
layer: domain
principles:
  - id: e2e-p1
    statement: E2E domain principle (modified)
`))

		second, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN,
			SourceUrl: url,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(second.Msg.GetChanged()).To(BeTrue(), "modified content must report changed")
		Expect(second.Msg.GetBefore()).NotTo(BeNil())
		Expect(second.Msg.GetBefore().GetName()).To(Equal("e2e-initial-domain"))
		Expect(second.Msg.GetAfter().GetName()).To(Equal("e2e-modified-domain"))
		Expect(second.Msg.GetNewSourceHash()).NotTo(Equal(first.Msg.GetNewSourceHash()))
	})

	It("dry_run on modified content reports changed but does not write", func() {
		// Uses user layer to avoid collision with org/project/domain layers written above.
		var body atomic.Value
		body.Store([]byte(`name: e2e-initial-user
layer: user
principles:
  - id: e2e-p1
    statement: E2E user principle
`))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			b := body.Load().([]byte)
			_, _ = w.Write(b)
		}))
		defer srv.Close()

		url := srv.URL + "/constitution.yaml"

		// Establish a baseline.
		_, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
			SourceUrl: url,
		}))
		Expect(err).NotTo(HaveOccurred())

		// Mutate the served content.
		body.Store([]byte(`name: e2e-would-change-user
layer: user
`))

		// Dry-run: should report changed but not persist.
		dry, err := client.RefreshConstitutionLayer(ctx, connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
			SourceUrl: url,
			DryRun:    true,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(dry.Msg.GetChanged()).To(BeTrue())

		// Verify persistence by reading back the layer — should still be e2e-initial-user.
		get, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER,
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(get.Msg.GetConstitution().GetName()).To(Equal("e2e-initial-user"),
			"dry-run must not have written the modified content")
	})
})
