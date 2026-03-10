// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
)

var _ = Describe("Constitution", Ordered, func() {
	var (
		client     specgraphv1connect.ConstitutionServiceClient
		specClient specgraphv1connect.SpecServiceClient
		ctx        context.Context
	)

	BeforeAll(func() {
		client = newConstitutionClient()
		specClient = newSpecClient()
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

	Context("CheckViolation", func() {
		It("returns failed-precondition when no spec exists", func() {
			_, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
				SpecSlug: "nonexistent-spec-for-check",
			}))
			Expect(err).To(HaveOccurred())

			var connErr *connect.Error
			Expect(err).To(BeAssignableToTypeOf(connErr))
			connErr = err.(*connect.Error)
			Expect(connErr.Code()).To(Equal(connect.CodeNotFound))
		})

		It("returns no violations for a compliant spec", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   "go-auth-handler",
				Intent: "Implement Go authentication handler with ConnectRPC",
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
				SpecSlug: "go-auth-handler",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Violations).To(BeEmpty())
		})

		It("detects forbidden language in spec intent", func() {
			_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug:   "java-migration-tool",
				Intent: "Build a Java-based data migration tool",
			}))
			Expect(err).NotTo(HaveOccurred())

			resp, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
				SpecSlug: "java-migration-tool",
			}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Violations).NotTo(BeEmpty())

			v := resp.Msg.Violations[0]
			Expect(v.Rule).To(Equal("forbidden-language"))
			Expect(v.Severity).To(Equal(specv1.ViolationSeverity_VIOLATION_SEVERITY_ERROR))
			Expect(v.SpecSlug).To(Equal("java-migration-tool"))
			Expect(v.Message).To(ContainSubstring("Java"))
		})

		It("rejects request with empty spec_slug", func() {
			_, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
				SpecSlug: "",
			}))
			Expect(err).To(HaveOccurred())

			var connErr *connect.Error
			Expect(err).To(BeAssignableToTypeOf(connErr))
			connErr = err.(*connect.Error)
			Expect(connErr.Code()).To(Equal(connect.CodeInvalidArgument))
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
