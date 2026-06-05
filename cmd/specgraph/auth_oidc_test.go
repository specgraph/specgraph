// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// --- Validation branches (execute before any RPC) ---

// TestAuthOIDCList_RequiresUser verifies the --user guard returns an error
// before any client is constructed.
func TestAuthOIDCList_RequiresUser(t *testing.T) {
	defer func(v string) { oidcListUser = v }(oidcListUser)
	oidcListUser = ""
	err := authOIDCListCmd.RunE(authOIDCListCmd, nil)
	require.EqualError(t, err, "--user is required")
}

// TestAuthOIDCUnbind_RequiresUser verifies the --user guard on unbind (the
// owner of the binding) returns an error before any client is constructed.
func TestAuthOIDCUnbind_RequiresUser(t *testing.T) {
	defer func(v string) { oidcUnbindUser = v }(oidcUnbindUser)
	oidcUnbindUser = ""
	err := authOIDCUnbindCmd.RunE(authOIDCUnbindCmd, []string{"binding-1"})
	require.EqualError(t, err, "--user is required (owner of the binding)")
}

// --- Happy-path round-trips (via the injectable identityClient seam) ---

// stubIdentityHandler serves canned ListOIDCBindings / UnbindOIDC responses so
// CLI commands can be exercised end-to-end (RPC round-trip + render) without a
// real backend. unbindReq captures the last UnbindOIDC request for assertions.
type stubIdentityHandler struct {
	specgraphv1connect.UnimplementedIdentityServiceHandler
	bindings  []*specv1.OIDCBinding
	unbindReq *specv1.UnbindOIDCRequest
}

func (h *stubIdentityHandler) ListOIDCBindings(_ context.Context, _ *connect.Request[specv1.ListOIDCBindingsRequest]) (*connect.Response[specv1.ListOIDCBindingsResponse], error) {
	return connect.NewResponse(&specv1.ListOIDCBindingsResponse{Bindings: h.bindings}), nil
}

func (h *stubIdentityHandler) UnbindOIDC(_ context.Context, req *connect.Request[specv1.UnbindOIDCRequest]) (*connect.Response[specv1.UnbindOIDCResponse], error) {
	h.unbindReq = req.Msg
	return connect.NewResponse(&specv1.UnbindOIDCResponse{}), nil
}

// withStubIdentityClient stands up an httptest server backed by handler and
// points the package-level identityClient at it for the duration of the test.
func withStubIdentityClient(t *testing.T, handler *stubIdentityHandler) {
	t.Helper()
	mux := http.NewServeMux()
	path, h := specgraphv1connect.NewIdentityServiceHandler(handler)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewIdentityServiceClient(http.DefaultClient, srv.URL)
	prev := identityClient
	identityClient = func() (specgraphv1connect.IdentityServiceClient, error) { return client, nil }
	t.Cleanup(func() { identityClient = prev })
}

// TestAuthOIDCList_RendersBindings drives `auth oidc list --user u1` against a
// stub server and asserts the rendered table contains the returned binding.
func TestAuthOIDCList_RendersBindings(t *testing.T) {
	defer func(v string, j bool) { oidcListUser = v; authJSON = j }(oidcListUser, authJSON)
	withStubIdentityClient(t, &stubIdentityHandler{
		bindings: []*specv1.OIDCBinding{
			{Id: "bind-1", Issuer: "https://idp.example.com", Subject: "sub-123"},
		},
	})
	oidcListUser = "u1"
	authJSON = false

	var buf bytes.Buffer
	authOIDCListCmd.SetOut(&buf)
	authOIDCListCmd.SetContext(context.Background())
	t.Cleanup(func() { authOIDCListCmd.SetOut(nil) })

	require.NoError(t, authOIDCListCmd.RunE(authOIDCListCmd, nil))
	out := buf.String()
	require.Contains(t, out, "bind-1")
	require.Contains(t, out, "sub-123")
	require.Contains(t, out, "ISSUER")
}

// TestAuthOIDCList_EmptyRendersPlaceholder asserts the no-bindings case renders
// the placeholder line rather than an empty/garbled table.
func TestAuthOIDCList_EmptyRendersPlaceholder(t *testing.T) {
	defer func(v string, j bool) { oidcListUser = v; authJSON = j }(oidcListUser, authJSON)
	withStubIdentityClient(t, &stubIdentityHandler{bindings: nil})
	oidcListUser = "u1"
	authJSON = false

	var buf bytes.Buffer
	authOIDCListCmd.SetOut(&buf)
	authOIDCListCmd.SetContext(context.Background())
	t.Cleanup(func() { authOIDCListCmd.SetOut(nil) })

	require.NoError(t, authOIDCListCmd.RunE(authOIDCListCmd, nil))
	require.Contains(t, buf.String(), "No OIDC bindings found.")
}

// TestAuthOIDCUnbind_PassesForceAndIDs verifies the unbind command forwards the
// binding ID (positional arg), owner --user, and --force flag into the request.
func TestAuthOIDCUnbind_PassesForceAndIDs(t *testing.T) {
	defer func(u string, f bool) { oidcUnbindUser = u; oidcUnbindForce = f }(oidcUnbindUser, oidcUnbindForce)
	h := &stubIdentityHandler{}
	withStubIdentityClient(t, h)
	oidcUnbindUser = "owner-9"
	oidcUnbindForce = true

	var buf bytes.Buffer
	authOIDCUnbindCmd.SetOut(&buf)
	authOIDCUnbindCmd.SetContext(context.Background())
	t.Cleanup(func() { authOIDCUnbindCmd.SetOut(nil) })

	require.NoError(t, authOIDCUnbindCmd.RunE(authOIDCUnbindCmd, []string{"bind-7"}))
	require.NotNil(t, h.unbindReq, "handler must have received the unbind request")
	require.Equal(t, "bind-7", h.unbindReq.GetBindingId())
	require.Equal(t, "owner-9", h.unbindReq.GetUserId())
	require.True(t, h.unbindReq.GetForce(), "--force must propagate into the request")
	require.Contains(t, buf.String(), "Unbound bind-7")
}

// TestAuthOIDCUnbind_SurfacesServerError confirms an RPC error is wrapped and
// returned rather than swallowed.
func TestAuthOIDCUnbind_SurfacesServerError(t *testing.T) {
	defer func(u string) { oidcUnbindUser = u }(oidcUnbindUser)
	prev := identityClient
	identityClient = func() (specgraphv1connect.IdentityServiceClient, error) {
		return nil, errors.New("dial failed")
	}
	t.Cleanup(func() { identityClient = prev })
	oidcUnbindUser = "owner-9"

	err := authOIDCUnbindCmd.RunE(authOIDCUnbindCmd, []string{"bind-7"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "dial failed"))
}
