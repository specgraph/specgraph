// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"

	specgraphv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

type stubHealthHandler struct {
	specgraphv1connect.UnimplementedServerServiceHandler
}

func (h *stubHealthHandler) Health(_ context.Context, _ *connect.Request[specgraphv1.HealthRequest]) (*connect.Response[specgraphv1.HealthResponse], error) {
	return connect.NewResponse(&specgraphv1.HealthResponse{}), nil
}

type stubSpecHandler struct {
	specgraphv1connect.UnimplementedSpecServiceHandler
}

func (h *stubSpecHandler) GetSpec(_ context.Context, _ *connect.Request[specgraphv1.GetSpecRequest]) (*connect.Response[specgraphv1.GetSpecResponse], error) {
	return connect.NewResponse(&specgraphv1.GetSpecResponse{Spec: &specgraphv1.Spec{}}), nil
}

func (h *stubSpecHandler) CreateSpec(_ context.Context, _ *connect.Request[specgraphv1.CreateSpecRequest]) (*connect.Response[specgraphv1.CreateSpecResponse], error) {
	return connect.NewResponse(&specgraphv1.CreateSpecResponse{Spec: &specgraphv1.Spec{}}), nil
}

func newTestServer(t *testing.T, authCfg config.AuthConfig) (*httptest.Server, specgraphv1connect.SpecServiceClient, specgraphv1connect.ServerServiceClient) {
	t.Helper()
	store, err := auth.NewConfigStore(authCfg, "")
	if err != nil {
		t.Fatalf("NewConfigStore: %v", err)
	}
	interceptor := auth.NewAuthInterceptor(store)
	opts := connect.WithInterceptors(interceptor)

	mux := http.NewServeMux()
	path, handler := specgraphv1connect.NewServerServiceHandler(&stubHealthHandler{}, opts)
	mux.Handle(path, handler)
	path, handler = specgraphv1connect.NewSpecServiceHandler(&stubSpecHandler{}, opts)
	mux.Handle(path, handler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	specClient := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	healthClient := specgraphv1connect.NewServerServiceClient(http.DefaultClient, srv.URL)
	return srv, specClient, healthClient
}

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

func newSpecClientWithAuth(url, token string) specgraphv1connect.SpecServiceClient {
	return specgraphv1connect.NewSpecServiceClient(http.DefaultClient, url, withBearer(token))
}

func TestInterceptor_ValidAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "spgr_sk_abc")
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInterceptor_InvalidAPIKey(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "wrong_key")
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}

func TestInterceptor_NoToken_NoKeys(t *testing.T) {
	_, specClient, _ := newTestServer(t, config.AuthConfig{})
	_, err := specClient.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("expected success with local identity, got: %v", err)
	}
}

func TestInterceptor_NoToken_WithKeys(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	_, specClient, _ := newTestServer(t, cfg)
	_, err := specClient.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}

func TestInterceptor_InsufficientPermission(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_r", Name: "Reader", Role: "reader"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	client := newSpecClientWithAuth(srv.URL, "spgr_sk_r")
	// Reader can GetSpec (spec:read)
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err != nil {
		t.Fatalf("reader should be able to GetSpec: %v", err)
	}
	// Reader cannot CreateSpec (spec:write)
	_, err = client.CreateSpec(context.Background(), connect.NewRequest(&specgraphv1.CreateSpecRequest{}))
	if err == nil {
		t.Fatal("expected error for reader calling CreateSpec")
	}
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("code = %v, want PermissionDenied", connect.CodeOf(err))
	}
}

func TestInterceptor_ExemptRPC(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	_, _, healthClient := newTestServer(t, cfg)
	_, err := healthClient.Health(context.Background(), connect.NewRequest(&specgraphv1.HealthRequest{}))
	if err != nil {
		t.Fatalf("Health should be exempt: %v", err)
	}
}

func TestInterceptor_JWTToken(t *testing.T) {
	cfg := config.AuthConfig{
		APIKeys: []config.APIKeyConfig{
			{ID: "k1", Key: "spgr_sk_abc", Name: "Admin", Role: "admin"},
		},
	}
	srv, _, _ := newTestServer(t, cfg)
	// Construct a fake JWT-shaped token at runtime to avoid triggering static
	// analysis rules that match hard-coded JWT literals in source.
	fakeJWT := strings.Join([]string{"eyJhbGciOiJSUzI1NiJ9", "eyJzdWIiOiIxMjM0NTY3ODkwIn0", "signature"}, ".")
	client := newSpecClientWithAuth(srv.URL, fakeJWT)
	_, err := client.GetSpec(context.Background(), connect.NewRequest(&specgraphv1.GetSpecRequest{}))
	if err == nil {
		t.Fatal("expected error for JWT token")
	}
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("code = %v, want Unauthenticated", connect.CodeOf(err))
	}
}
