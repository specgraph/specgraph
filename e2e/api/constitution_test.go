// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Constitution", Ordered, func() {
	var (
		client specgraphv1connect.ConstitutionServiceClient
		ctx    context.Context
	)

	BeforeAll(func() {
		client = newConstitutionClient()
		ctx = context.Background()
	})

	It("returns not-found when no constitution exists", func() {
		_, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).To(HaveOccurred())

		var connErr *connect.Error
		Expect(err).To(BeAssignableToTypeOf(connErr))
		connErr = err.(*connect.Error)
		Expect(connErr.Code()).To(Equal(connect.CodeNotFound))
	})

	It("creates/bootstraps a constitution via UpdateConstitution", func() {
		resp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:  "specgraph-e2e",
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary:   "Go",
						Allowed:   []string{"Go", "Python"},
						Forbidden: []string{"Java"},
						ForbiddenReasons: map[string]string{
							"Java": "project scope limited to Go and Python",
						},
					},
					Frameworks: map[string]string{
						"api": "ConnectRPC",
					},
					Infrastructure: map[string]string{
						"graph": "Memgraph",
					},
				},
				Principles: []*specv1.Principle{
					{
						Id:        "p1",
						Statement: "Specs are graph nodes with first-class edges",
						Rationale: "enables dependency tracking",
					},
				},
				Constraints: []string{
					"no global mutable state",
					"all public APIs must have integration tests",
				},
				Antipatterns: []*specv1.Antipattern{
					{
						Pattern: "god object",
						Why:     "violates single-responsibility",
						Instead: "decompose into focused types",
					},
				},
				References: []*specv1.Reference{
					{
						ReferenceType: specv1.ReferenceType_REFERENCE_TYPE_ADR,
						Path:          "docs/adr/003-decisions.md",
					},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Id).NotTo(BeEmpty())
		Expect(c.Name).To(Equal("specgraph-e2e"))
		Expect(c.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		Expect(c.Version).To(BeNumerically(">=", 1))
		Expect(c.Tech).NotTo(BeNil())
		Expect(c.Tech.Languages.Primary).To(Equal("Go"))
		Expect(c.Tech.Languages.Forbidden).To(ContainElement("Java"))
		Expect(c.Principles).To(HaveLen(1))
		Expect(c.Constraints).To(HaveLen(2))
		Expect(c.Antipatterns).To(HaveLen(1))
		Expect(c.References).To(HaveLen(1))
	})

	It("shows the constitution via GetConstitution", func() {
		resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())

		c := resp.Msg.Constitution
		Expect(c.Name).To(Equal("specgraph-e2e"))
		Expect(c.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		Expect(c.Tech.Languages.Primary).To(Equal("Go"))
		Expect(c.Tech.Frameworks["api"]).To(Equal("ConnectRPC"))
		Expect(c.Tech.Infrastructure["graph"]).To(Equal("Memgraph"))
		Expect(c.Principles[0].Id).To(Equal("p1"))
		Expect(c.Constraints).To(ContainElement("no global mutable state"))
		Expect(c.Antipatterns[0].Pattern).To(Equal("god object"))
		Expect(c.References[0].Path).To(Equal("docs/adr/003-decisions.md"))
	})

	It("bumps version on subsequent update", func() {
		getResp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		Expect(err).NotTo(HaveOccurred())
		prevVersion := getResp.Msg.Constitution.Version

		updResp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{
				Layer:       specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
				Name:        "specgraph-e2e",
				Constraints: []string{"no global mutable state"},
				Tech: &specv1.TechConfig{
					Languages: &specv1.LanguageConfig{
						Primary:   "Go",
						Allowed:   []string{"Go", "Python"},
						Forbidden: []string{"Java"},
						ForbiddenReasons: map[string]string{
							"Java": "project scope limited to Go and Python",
						},
					},
				},
			},
		}))
		Expect(err).NotTo(HaveOccurred())
		Expect(updResp.Msg.Constitution.Version).To(BeNumerically(">", prevVersion))
	})

	Context("Multi-layer", Ordered, func() {
		const (
			orgLayerName = "specgraph-e2e-org"
			projLayerName = "specgraph-e2e-proj"
		)

		It("stores org layer independently", func() {
			resp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
				Constitution: &specv1.Constitution{
					Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
					Name:  orgLayerName,
					Tech: &specv1.TechConfig{
						Languages: &specv1.LanguageConfig{
							Primary: "Go",
							Allowed: []string{"Go", "Python"},
						},
					},
					Principles: []*specv1.Principle{
						{Id: "p1", Statement: "Specs are graph nodes", Rationale: "enables dependency tracking"},
						{Id: "p2", Statement: "org-level principle two", Rationale: "org rationale"},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Constitution.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG))
		})

		It("stores project layer independently", func() {
			resp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
				Constitution: &specv1.Constitution{
					Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
					Name:  projLayerName,
					Tech: &specv1.TechConfig{
						Languages: &specv1.LanguageConfig{
							Primary: "Go",
							Allowed: []string{"Go", "TypeScript"},
						},
					},
					Principles: []*specv1.Principle{
						{Id: "p2", Statement: "project override of p2", Rationale: "project rationale"},
						{Id: "p3-new", Statement: "project-only principle", Rationale: "project specific"},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Constitution.Layer).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		})

		It("returns merged constitution with provenance when no layer filter", func() {
			resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
			Expect(err).NotTo(HaveOccurred())

			c := resp.Msg.Constitution
			Expect(c.Tech).NotTo(BeNil())
			Expect(c.Tech.Languages).NotTo(BeNil())
			// Go, Python (org) and TypeScript (project) should all be present
			Expect(c.Tech.Languages.Allowed).To(ContainElements("Go", "Python", "TypeScript"))
			// p1 from org, p2 overridden by project, p3-new from project
			Expect(c.Principles).To(HaveLen(3))
			ids := make([]string, 0, len(c.Principles))
			for _, p := range c.Principles {
				ids = append(ids, p.Id)
			}
			Expect(ids).To(ContainElements("p1", "p2", "p3-new"))
			// Provenance must be populated for merged results.
			Expect(resp.Msg.Provenance).NotTo(BeEmpty())
			// Verify specific provenance: p2 overridden by project layer.
			var p2Prov specv1.ConstitutionLayer
			for _, pe := range resp.Msg.Provenance {
				if pe.Path == "principles[p2]" {
					p2Prov = pe.Layer
				}
			}
			Expect(p2Prov).To(Equal(specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT))
		})

		It("returns raw org layer when layer filter is set", func() {
			resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
				Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			}))
			Expect(err).NotTo(HaveOccurred())

			c := resp.Msg.Constitution
			Expect(c.Name).To(Equal(orgLayerName))
			// Raw layer response — no provenance
			Expect(resp.Msg.Provenance).To(BeEmpty())
		})
	})

	Context("EmitToolFiles", func() {
		It("emits CLAUDE.md format", func() {
			resp, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
				Format: specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Filename).To(Equal("CLAUDE.md"))
			Expect(resp.Msg.Content).To(ContainSubstring("Project Constitution"))
			Expect(resp.Msg.Content).To(ContainSubstring("Go"))
		})

		It("emits .cursorrules format", func() {
			resp, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
				Format: specv1.OutputFormat_OUTPUT_FORMAT_CURSORRULES,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Filename).To(Equal(".cursorrules"))
			Expect(resp.Msg.Content).To(ContainSubstring("Project Rules"))
		})

		It("emits AGENTS.md format", func() {
			resp, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
				Format: specv1.OutputFormat_OUTPUT_FORMAT_AGENTS_MD,
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Filename).To(Equal("AGENTS.md"))
			Expect(resp.Msg.Content).To(ContainSubstring("Agent Instructions"))
		})

		It("rejects unspecified format", func() {
			_, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
				Format: specv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED,
			}))
			Expect(err).To(HaveOccurred())

			var connErr *connect.Error
			Expect(err).To(BeAssignableToTypeOf(connErr))
			connErr = err.(*connect.Error)
			Expect(connErr.Code()).To(Equal(connect.CodeInvalidArgument))
		})
	})
})
