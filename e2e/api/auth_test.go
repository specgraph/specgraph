// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build e2e

package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/specgraph/specgraph/e2e/testutil"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

// withBearer returns a client option that injects a Bearer token into requests.
func withBearer(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(
		func(next connect.UnaryFunc) connect.UnaryFunc {
			return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
				req.Header().Set("Authorization", "Bearer "+token)
				return next(ctx, req)
			}
		},
	))
}

// authProjectClient returns an HTTP client with the project header set.
func authProjectClient() *http.Client {
	return projectClientFor(e2eProject)
}

// staticResolver is an in-process Resolver backed by a token→Identity map.
// Used in e2e tests in place of the database-backed pgIdentityStore so that
// auth tests can spin up isolated servers with known credentials without
// touching the users table.
type staticResolver struct {
	tokens map[string]*auth.Identity
}

// newStaticResolver builds a staticResolver from an auth config.
// It resolves tokens exactly as configured (no hashing — e2e tests use
// short, well-known tokens to keep test fixtures readable).
func newStaticResolver(cfg config.AuthConfig) (auth.Resolver, auth.Authorizer) {
	rolePerms := auth.LoadRolePerms(cfg.Roles)
	resolver := &staticResolver{tokens: make(map[string]*auth.Identity, len(cfg.APIKeys))}
	for _, ak := range cfg.APIKeys {
		resolver.tokens[ak.Key] = &auth.Identity{
			Subject:       "apikey:" + ak.ID,
			DisplayName:   ak.Name,
			Role:          auth.Role(ak.Role),
			EffectiveRole: auth.Role(ak.Role),
			Source:        "apikey",
		}
	}
	authorizer := auth.NewStaticTableAuthorizer(rolePerms)
	return resolver, authorizer
}

func (r *staticResolver) Resolve(_ context.Context, token string) (*auth.Identity, error) {
	if id, ok := r.tokens[token]; ok { //nolint:gosec // G101: token used as map key, not compared to literal credential
		return id, nil
	}
	return nil, auth.ErrUnauthenticated
}

func (r *staticResolver) HasAuth(_ context.Context) (bool, error) { return len(r.tokens) > 0, nil }

var _ = Describe("Auth", Label("auth"), func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("no keys configured", func() {
		var (
			info    *testutil.ServerInfo
			cleanup func()
		)

		BeforeEach(func() {
			// Start server with auth interceptor but no API keys configured.
			resolver, authorizer := newStaticResolver(config.AuthConfig{})
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			var err error
			info, cleanup, err = testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("rejects unauthenticated requests (no local bypass)", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL)
			_, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
			Expect(err).To(HaveOccurred())
			var connectErr *connect.Error
			Expect(errors.As(err, &connectErr)).To(BeTrue())
			Expect(connectErr.Code()).To(Equal(connect.CodeUnauthenticated))
		})

		It("allows health check without auth", func() {
			client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, info.BaseURL)
			resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Status).To(Equal("ok"))
		})
	})

	Describe("keys configured, no Bearer token", func() {
		var (
			info    *testutil.ServerInfo
			cleanup func()
		)

		BeforeEach(func() {
			authCfg := config.AuthConfig{
				APIKeys: []config.APIKeyConfig{
					{ID: "k1", Key: "secret-admin-key", Name: "Admin", Role: "admin"},
				},
			}
			resolver, authorizer := newStaticResolver(authCfg)
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			var err error
			info, cleanup, err = testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("rejects unauthenticated GetSpec with Unauthenticated", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL)
			_, err := client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: "nonexistent"}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeUnauthenticated))
		})

		It("rejects unauthenticated CreateSpec with Unauthenticated", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL)
			_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug: "auth-test", Intent: "test", Priority: "p2",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeUnauthenticated))
		})

		It("rejects unauthenticated ListDecisions with Unauthenticated", func() {
			client := specgraphv1connect.NewDecisionServiceClient(authProjectClient(), info.BaseURL)
			_, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodeUnauthenticated))
		})

		It("allows Health without Bearer token", func() {
			client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, info.BaseURL)
			resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Status).To(Equal("ok"))
		})
	})

	Describe("keys configured, valid reader key", func() {
		const readerKey = "reader-secret-key-123"

		var (
			info    *testutil.ServerInfo
			cleanup func()
		)

		BeforeEach(func() {
			authCfg := config.AuthConfig{
				APIKeys: []config.APIKeyConfig{
					{ID: "reader1", Key: readerKey, Name: "Reader", Role: "reader"},
				},
			}
			resolver, authorizer := newStaticResolver(authCfg)
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			var err error
			info, cleanup, err = testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("can ListSpecs with reader key", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(readerKey))
			_, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("cannot CreateSpec with reader key", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(readerKey))
			_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug: "reader-attempt", Intent: "should fail", Priority: "p3",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodePermissionDenied))
		})

		It("can ListDecisions with reader key", func() {
			client := specgraphv1connect.NewDecisionServiceClient(authProjectClient(), info.BaseURL, withBearer(readerKey))
			_, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("cannot CreateDecision with reader key", func() {
			client := specgraphv1connect.NewDecisionServiceClient(authProjectClient(), info.BaseURL, withBearer(readerKey))
			_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
				Slug: "reader-decision", Title: "should fail",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodePermissionDenied))
		})
	})

	Describe("keys configured, valid admin key", func() {
		const adminKey = "admin-secret-key-456"

		var (
			info     *testutil.ServerInfo
			cleanup  func()
			testSlug string
		)

		BeforeEach(func() {
			authCfg := config.AuthConfig{
				APIKeys: []config.APIKeyConfig{
					{ID: "admin1", Key: adminKey, Name: "Admin", Role: "admin"},
				},
			}
			resolver, authorizer := newStaticResolver(authCfg)
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			var err error
			info, cleanup, err = testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())

			// Use unique slug per It block to avoid duplicate-slug errors.
			testSlug = fmt.Sprintf("admin-test-%d", time.Now().UnixNano())

			// Seed a spec for read operations.
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(adminKey))
			_, err = client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug: testSlug, Intent: "admin seeded spec", Priority: "p1",
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("can ListSpecs", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(adminKey))
			resp, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Specs).NotTo(BeEmpty())
		})

		It("can CreateSpec", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(adminKey))
			_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug: "admin-create-test", Intent: "admin can create", Priority: "p2",
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("can GetSpec", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(adminKey))
			resp, err := client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: testSlug}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.GetSpec().GetSlug()).To(Equal(testSlug))
		})

		It("can CreateDecision", func() {
			client := specgraphv1connect.NewDecisionServiceClient(authProjectClient(), info.BaseURL, withBearer(adminKey))
			_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
				Slug: "admin-decision", Title: "admin decision",
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("keys configured, custom role", func() {
		const customKey = "custom-role-key-789"

		var (
			info    *testutil.ServerInfo
			cleanup func()
		)

		BeforeEach(func() {
			authCfg := config.AuthConfig{
				APIKeys: []config.APIKeyConfig{
					{ID: "custom1", Key: customKey, Name: "SpecOnly", Role: "spec-reader"},
				},
				Roles: map[string]config.RoleConfig{
					"spec-reader": {Permissions: []string{"spec:read"}},
				},
			}
			resolver, authorizer := newStaticResolver(authCfg)
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			var err error
			info, cleanup, err = testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if cleanup != nil {
				cleanup()
			}
		})

		It("can ListSpecs with spec:read permission", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(customKey))
			_, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("cannot CreateSpec with only spec:read permission", func() {
			client := specgraphv1connect.NewSpecServiceClient(authProjectClient(), info.BaseURL, withBearer(customKey))
			_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
				Slug: "custom-attempt", Intent: "should fail", Priority: "p3",
			}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodePermissionDenied))
		})

		It("cannot ListDecisions with only spec:read permission", func() {
			client := specgraphv1connect.NewDecisionServiceClient(authProjectClient(), info.BaseURL, withBearer(customKey))
			_, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodePermissionDenied))
		})

		It("cannot access graph endpoints with only spec:read permission", func() {
			client := specgraphv1connect.NewGraphServiceClient(authProjectClient(), info.BaseURL, withBearer(customKey))
			_, err := client.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{}))
			Expect(err).To(HaveOccurred())
			Expect(connect.CodeOf(err)).To(Equal(connect.CodePermissionDenied))
		})
	})

	Describe("Health always responds", Label("auth"), func() {
		It("responds when no auth interceptor is configured", func() {
			// The suite-level server has no auth interceptor.
			client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, serverInfo.BaseURL)
			resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Status).To(Equal("ok"))
		})

		It("responds when auth is configured and no token is provided", func() {
			authCfg := config.AuthConfig{
				APIKeys: []config.APIKeyConfig{
					{ID: "h1", Key: "health-test-key", Name: "Test", Role: "admin"},
				},
			}
			resolver, authorizer := newStaticResolver(authCfg)
			interceptor := auth.NewAuthInterceptor(resolver, authorizer)

			info, cleanup, err := testutil.StartServer(ctx, pgConnURL,
				connect.WithInterceptors(interceptor),
			)
			Expect(err).NotTo(HaveOccurred())
			defer cleanup()

			client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, info.BaseURL)
			resp, err := client.Health(ctx, connect.NewRequest(&specv1.HealthRequest{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Msg.Status).To(Equal("ok"))
		})
	})
})
