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
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// resyncStubHandler serves a canned ResyncUserRole response and captures the
// last request so the CLI command can be exercised end-to-end (RPC round-trip +
// output split) without a real backend.
type resyncStubHandler struct {
	specgraphv1connect.UnimplementedIdentityServiceHandler
	req  *specv1.ResyncUserRoleRequest
	role string // role echoed back on the returned user
}

func (h *resyncStubHandler) ResyncUserRole(_ context.Context, req *connect.Request[specv1.ResyncUserRoleRequest]) (*connect.Response[specv1.ResyncUserRoleResponse], error) {
	h.req = req.Msg
	role := h.role
	if role == "" {
		role = req.Msg.GetRole()
	}
	return connect.NewResponse(&specv1.ResyncUserRoleResponse{
		User: &specv1.User{Id: req.Msg.GetId(), Role: role},
	}), nil
}

// withResyncStubClient stands up an httptest server backed by handler and points
// the package-level identityClient at it for the duration of the test.
func withResyncStubClient(t *testing.T, handler *resyncStubHandler) {
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

// TestAuthUserResync_ForwardsIDRoleAndRevoke drives `auth user resync <id>
// --role reader --revoke-keys` against a stub server and asserts the parsed id,
// role, and revoke-keys flag reach the ResyncUserRole request, and that the
// human summary reflects the applied role and off-board.
func TestAuthUserResync_ForwardsIDRoleAndRevoke(t *testing.T) {
	defer func(r string, rk bool, j bool) {
		userResyncRole = r
		userResyncRevokeKeys = rk
		authJSON = j
	}(userResyncRole, userResyncRevokeKeys, authJSON)

	h := &resyncStubHandler{}
	withResyncStubClient(t, h)
	userResyncRole = "reader"
	userResyncRevokeKeys = true
	authJSON = false

	var buf bytes.Buffer
	authUserResyncCmd.SetOut(&buf)
	authUserResyncCmd.SetContext(context.Background())
	t.Cleanup(func() { authUserResyncCmd.SetOut(nil) })

	require.NoError(t, authUserResyncCmd.RunE(authUserResyncCmd, []string{"user-9"}))
	require.NotNil(t, h.req, "handler must have received the resync request")
	require.Equal(t, "user-9", h.req.GetId())
	require.Equal(t, "reader", h.req.GetRole())
	require.True(t, h.req.GetRevokeKeys(), "--revoke-keys must propagate into the request")

	out := buf.String()
	require.Contains(t, out, "user-9")
	require.Contains(t, out, "reader")
	require.Contains(t, out, "revoked", "human summary reflects the key off-board")
}

// TestAuthUserResync_NoRevokeKeepsKeys asserts the default (no --revoke-keys)
// leaves RevokeKeys false and the human summary states keys stay active.
func TestAuthUserResync_NoRevokeKeepsKeys(t *testing.T) {
	defer func(r string, rk bool, j bool) {
		userResyncRole = r
		userResyncRevokeKeys = rk
		authJSON = j
	}(userResyncRole, userResyncRevokeKeys, authJSON)

	h := &resyncStubHandler{}
	withResyncStubClient(t, h)
	userResyncRole = "reader"
	userResyncRevokeKeys = false
	authJSON = false

	var buf bytes.Buffer
	authUserResyncCmd.SetOut(&buf)
	authUserResyncCmd.SetContext(context.Background())
	t.Cleanup(func() { authUserResyncCmd.SetOut(nil) })

	require.NoError(t, authUserResyncCmd.RunE(authUserResyncCmd, []string{"user-9"}))
	require.False(t, h.req.GetRevokeKeys(), "default must not revoke keys")
	require.Contains(t, buf.String(), "active", "human summary states keys remain active")
}

// TestAuthUserResync_JSONEmitsResponse asserts the --json path emits the RPC
// response via printJSON (machine-readable user shape).
func TestAuthUserResync_JSONEmitsResponse(t *testing.T) {
	defer func(r string, rk bool, j bool) {
		userResyncRole = r
		userResyncRevokeKeys = rk
		authJSON = j
	}(userResyncRole, userResyncRevokeKeys, authJSON)

	withResyncStubClient(t, &resyncStubHandler{role: "reader"})
	userResyncRole = "reader"
	userResyncRevokeKeys = false
	authJSON = true

	var buf bytes.Buffer
	authUserResyncCmd.SetOut(&buf)
	authUserResyncCmd.SetContext(context.Background())
	t.Cleanup(func() { authUserResyncCmd.SetOut(nil) })

	require.NoError(t, authUserResyncCmd.RunE(authUserResyncCmd, []string{"user-9"}))
	out := buf.String()
	require.Contains(t, out, "\"role\"")
	require.Contains(t, out, "reader")
	require.Contains(t, out, "user-9")
}

// TestAuthUserResync_SurfacesClientError confirms a client-construction error is
// returned rather than swallowed.
func TestAuthUserResync_SurfacesClientError(t *testing.T) {
	defer func(r string) { userResyncRole = r }(userResyncRole)
	prev := identityClient
	identityClient = func() (specgraphv1connect.IdentityServiceClient, error) {
		return nil, errors.New("dial failed")
	}
	t.Cleanup(func() { identityClient = prev })
	userResyncRole = "reader"

	err := authUserResyncCmd.RunE(authUserResyncCmd, []string{"user-9"})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "dial failed"))
}

// TestAuthUserResync_Registered asserts the resync command is wired under the
// `auth user` group with the --role and --revoke-keys flags.
func TestAuthUserResync_Registered(t *testing.T) {
	var found *cobra.Command
	for _, c := range authUserCmd.Commands() {
		if strings.HasPrefix(c.Use, "resync") {
			found = c
			break
		}
	}
	require.NotNil(t, found, "resync must be registered under `auth user`")
	require.NotNil(t, found.Flags().Lookup("role"), "--role flag must exist")
	require.NotNil(t, found.Flags().Lookup("revoke-keys"), "--revoke-keys flag must exist")
}
