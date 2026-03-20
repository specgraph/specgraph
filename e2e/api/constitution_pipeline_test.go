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

var _ = Describe("Constitution pipeline", Ordered, func() {
	const specSlug = "const-pipeline-spec"

	var (
		constClient specgraphv1connect.ConstitutionServiceClient
		specClient  specgraphv1connect.SpecServiceClient
		claimClient specgraphv1connect.ClaimServiceClient
		execClient  specgraphv1connect.ExecutionServiceClient
		ctx         context.Context
	)

	BeforeAll(func() {
		// Use a dedicated project to avoid polluting the e2e-test project.
		httpClient := projectClientFor("const-pipeline-project")
		constClient = specgraphv1connect.NewConstitutionServiceClient(httpClient, serverInfo.BaseURL)
		specClient = specgraphv1connect.NewSpecServiceClient(httpClient, serverInfo.BaseURL)
		claimClient = specgraphv1connect.NewClaimServiceClient(httpClient, serverInfo.BaseURL)
		execClient = specgraphv1connect.NewExecutionServiceClient(httpClient, serverInfo.BaseURL)
		ctx = context.Background()
	})

	It("creates a project constitution with principles and constraints", func() {
		resp, err := constClient.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Name:  "pipeline-test-constitution",
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Principles: []*specv1.Principle{
					{
						Id:        "cp1",
						Statement: "All specs must have clear acceptance criteria",
						Rationale: "enables objective verification",
					},
					{
						Id:        "cp2",
						Statement: "Prefer composition over inheritance",
						Rationale: "reduces coupling",
					},
				},
				Constraints: []string{
					"no circular dependencies",
					"all handlers must validate input",
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Id).NotTo(BeEmpty())
		Expect(c.Name).To(Equal("pipeline-test-constitution"))
		Expect(c.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		Expect(c.Principles).To(HaveLen(2))
		Expect(c.Principles[0].Statement).To(Equal("All specs must have clear acceptance criteria"))
		Expect(c.Principles[1].Statement).To(Equal("Prefer composition over inheritance"))
		Expect(c.Constraints).To(HaveLen(2))
		Expect(c.Constraints).To(ContainElement("no circular dependencies"))
		Expect(c.Constraints).To(ContainElement("all handlers must validate input"))
	})

	It("retrieves the constitution and verifies all fields", func() {
		resp, err := constClient.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Name).To(Equal("pipeline-test-constitution"))
		Expect(c.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		Expect(c.Principles).To(HaveLen(2))
		Expect(c.Principles[0].Id).To(Equal("cp1"))
		Expect(c.Principles[1].Id).To(Equal("cp2"))
		Expect(c.Constraints).To(HaveLen(2))
	})

	It("updates the constitution with additional principles", func() {
		resp, err := constClient.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Name:  "pipeline-test-constitution",
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Principles: []*specv1.Principle{
					{
						Id:        "cp1",
						Statement: "All specs must have clear acceptance criteria",
						Rationale: "enables objective verification",
					},
					{
						Id:        "cp2",
						Statement: "Prefer composition over inheritance",
						Rationale: "reduces coupling",
					},
					{
						Id:        "cp3",
						Statement: "Every public API requires documentation",
						Rationale: "improves discoverability",
					},
				},
				Constraints: []string{
					"no circular dependencies",
					"all handlers must validate input",
					"no panics in library code",
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Principles).To(HaveLen(3))
		Expect(c.Constraints).To(HaveLen(3))
		Expect(c.Constraints).To(ContainElement("no panics in library code"))
	})

	It("verifies the update is reflected on retrieval", func() {
		resp, err := constClient.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Principles).To(HaveLen(3))
		Expect(c.Principles[2].Id).To(Equal("cp3"))
		Expect(c.Principles[2].Statement).To(Equal("Every public API requires documentation"))
		Expect(c.Constraints).To(HaveLen(3))
		Expect(c.Constraints).To(ContainElement("no panics in library code"))
	})

	It("creates a spec, advances to approved, and claims it", func() {
		_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug:   specSlug,
			Intent: "Test constitution pipeline with execution bundle",
		}))
		Expect(err).NotTo(HaveOccurred())

		updateResp, err := specClient.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug:  specSlug,
			Stage: proto.String("approved"),
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(updateResp.Msg.Stage).To(Equal("approved"))

		claimResp, err := claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug: specSlug,
			Agent:    "pipeline-agent",
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(claimResp.Msg.SpecSlug).To(Equal(specSlug))
	})

	It("generates an execution bundle with prime callback", func() {
		resp, err := execClient.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
			Slug:     specSlug,
			Endpoint: serverInfo.BaseURL,
		}))
		Expect(err).NotTo(HaveOccurred())

		bundle := resp.Msg
		Expect(bundle.Spec).NotTo(BeNil())
		Expect(bundle.Spec.Slug).To(Equal(specSlug))
		Expect(bundle.Version).To(BeNumerically(">=", 1))
		// Bundle carries callback URLs — agents call Prime to get constitution.
		// Constitution is NOT embedded in the Bundle proto by design: agents
		// fetch it via the Prime callback so they always get the latest version.
		Expect(bundle.Callbacks).NotTo(BeNil())
		Expect(bundle.Callbacks.Prime).To(ContainSubstring("/prime"))
	})

	It("verifies constitution is accessible via GetPrime", func() {
		resp, err := execClient.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{
			Slug: specSlug,
		}))
		Expect(err).NotTo(HaveOccurred())
		// Constitution set earlier in this suite should appear in prime data.
		Expect(resp.Msg.ConstitutionSummary).To(ContainSubstring("pipeline-test-constitution"))
		Expect(resp.Msg.CodingConventions).To(ContainSubstring("All specs must have clear acceptance criteria"))
		Expect(resp.Msg.CodingConventions).To(ContainSubstring("no panics in library code"))
	})
})
