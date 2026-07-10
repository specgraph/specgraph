// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// TestParseExpiresAt covers the three cases for the --expires-at flag value:
// empty (no expiry override), a valid RFC3339 timestamp, and a malformed one.
func TestParseExpiresAt(t *testing.T) {
	t.Run("empty yields nil with no error", func(t *testing.T) {
		ts, err := parseExpiresAt("")
		if err != nil {
			t.Fatalf("empty input must not error, got %v", err)
		}
		if ts != nil {
			t.Errorf("empty input must yield nil timestamp, got %v", ts)
		}
	})

	t.Run("valid RFC3339 parses", func(t *testing.T) {
		want := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
		ts, err := parseExpiresAt(want.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("valid RFC3339 must parse, got %v", err)
		}
		if ts == nil {
			t.Fatal("valid RFC3339 must yield a non-nil timestamp")
		}
		if got := ts.AsTime().UTC(); !got.Equal(want) {
			t.Errorf("parsed time = %v, want %v", got, want)
		}
	})

	t.Run("malformed yields an error", func(t *testing.T) {
		ts, err := parseExpiresAt("not-a-timestamp")
		if err == nil {
			t.Fatal("malformed input must return an error")
		}
		if ts != nil {
			t.Errorf("malformed input must yield nil timestamp, got %v", ts)
		}
	})
}

// apiKeyStubHandler records which IdentityService RPCs the CLI invokes so the
// self-vs-admin routing (branch on --user) can be asserted without a real
// backend. It returns canned responses with a fixed plaintext so the emit-once
// output can be checked.
type apiKeyStubHandler struct {
	specgraphv1connect.UnimplementedIdentityServiceHandler

	createMyReq *specv1.CreateMyAPIKeyRequest
	createReq   *specv1.CreateAPIKeyRequest
	listMyCalls int
	listReq     *specv1.ListAPIKeysRequest
	rotateMyReq *specv1.RotateMyAPIKeyRequest
	rotateReq   *specv1.RotateAPIKeyRequest
	revokeMyReq *specv1.RevokeMyAPIKeyRequest
	revokeReq   *specv1.RevokeAPIKeyRequest

	plaintext string
}

func (h *apiKeyStubHandler) CreateMyAPIKey(_ context.Context, req *connect.Request[specv1.CreateMyAPIKeyRequest]) (*connect.Response[specv1.CreateMyAPIKeyResponse], error) {
	h.createMyReq = req.Msg
	return connect.NewResponse(&specv1.CreateMyAPIKeyResponse{
		Key: &specv1.APIKey{Id: "key-self", Prefix: "spgr_sk_self"}, Plaintext: h.plaintext,
	}), nil
}

func (h *apiKeyStubHandler) CreateAPIKey(_ context.Context, req *connect.Request[specv1.CreateAPIKeyRequest]) (*connect.Response[specv1.CreateAPIKeyResponse], error) {
	h.createReq = req.Msg
	return connect.NewResponse(&specv1.CreateAPIKeyResponse{
		Key: &specv1.APIKey{Id: "key-admin", Prefix: "spgr_sk_admin"}, Plaintext: h.plaintext,
	}), nil
}

func (h *apiKeyStubHandler) ListMyAPIKeys(_ context.Context, _ *connect.Request[specv1.ListMyAPIKeysRequest]) (*connect.Response[specv1.ListMyAPIKeysResponse], error) {
	h.listMyCalls++
	return connect.NewResponse(&specv1.ListMyAPIKeysResponse{}), nil
}

func (h *apiKeyStubHandler) ListAPIKeys(_ context.Context, req *connect.Request[specv1.ListAPIKeysRequest]) (*connect.Response[specv1.ListAPIKeysResponse], error) {
	h.listReq = req.Msg
	return connect.NewResponse(&specv1.ListAPIKeysResponse{}), nil
}

func (h *apiKeyStubHandler) RotateMyAPIKey(_ context.Context, req *connect.Request[specv1.RotateMyAPIKeyRequest]) (*connect.Response[specv1.RotateMyAPIKeyResponse], error) {
	h.rotateMyReq = req.Msg
	return connect.NewResponse(&specv1.RotateMyAPIKeyResponse{
		Key: &specv1.APIKey{Id: "key-self2", Prefix: "spgr_sk_self2"}, Plaintext: h.plaintext,
	}), nil
}

func (h *apiKeyStubHandler) RotateAPIKey(_ context.Context, req *connect.Request[specv1.RotateAPIKeyRequest]) (*connect.Response[specv1.RotateAPIKeyResponse], error) {
	h.rotateReq = req.Msg
	return connect.NewResponse(&specv1.RotateAPIKeyResponse{
		Key: &specv1.APIKey{Id: "key-admin2", Prefix: "spgr_sk_admin2"}, Plaintext: h.plaintext,
	}), nil
}

func (h *apiKeyStubHandler) RevokeMyAPIKey(_ context.Context, req *connect.Request[specv1.RevokeMyAPIKeyRequest]) (*connect.Response[specv1.RevokeMyAPIKeyResponse], error) {
	h.revokeMyReq = req.Msg
	return connect.NewResponse(&specv1.RevokeMyAPIKeyResponse{}), nil
}

func (h *apiKeyStubHandler) RevokeAPIKey(_ context.Context, req *connect.Request[specv1.RevokeAPIKeyRequest]) (*connect.Response[specv1.RevokeAPIKeyResponse], error) {
	h.revokeReq = req.Msg
	return connect.NewResponse(&specv1.RevokeAPIKeyResponse{}), nil
}

// withAPIKeyStubClient stands up an httptest server backed by handler and points
// BOTH package-level client builders at it: identitySessionClient (self path)
// and identityClient (admin path), so a single stub captures either route.
func withAPIKeyStubClient(t *testing.T, handler *apiKeyStubHandler) {
	t.Helper()
	mux := http.NewServeMux()
	path, h := specgraphv1connect.NewIdentityServiceHandler(handler)
	mux.Handle(path, h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewIdentityServiceClient(http.DefaultClient, srv.URL)
	prevSession, prevAdmin := identitySessionClient, identityClient
	identitySessionClient = func() (specgraphv1connect.IdentityServiceClient, error) { return client, nil }
	identityClient = func() (specgraphv1connect.IdentityServiceClient, error) { return client, nil }
	t.Cleanup(func() { identitySessionClient = prevSession; identityClient = prevAdmin })
}

// runAPIKeyCmd resets output on the given command, runs it, and returns stdout.
func runAPIKeyCmd(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetContext(context.Background())
	t.Cleanup(func() { cmd.SetOut(nil); cmd.SetErr(nil) })
	require.NoError(t, cmd.RunE(cmd, args))
	return out.String()
}

// TestAuthAPIKeyCreate_RoutesOnUser asserts an empty --user routes create to the
// self RPC (CreateMyAPIKey, no user_id on the wire) while --user <other> routes
// to the admin RPC (CreateAPIKey with the target user_id).
func TestAuthAPIKeyCreate_RoutesOnUser(t *testing.T) {
	defer func(u, l, d, e string, j bool) {
		apiKeyCreateUser, apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON = u, l, d, e, j
	}(apiKeyCreateUser, apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON)
	apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON = "", "", "", false

	t.Run("no --user invokes CreateMyAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{plaintext: "spgr_sk_self_PLAINTEXT"}
		withAPIKeyStubClient(t, h)
		apiKeyCreateUser = ""
		apiKeyCreateLabel = "laptop"

		runAPIKeyCmd(t, authAPIKeyCreateCmd)
		require.NotNil(t, h.createMyReq, "self create must invoke CreateMyAPIKey")
		require.Nil(t, h.createReq, "self create must NOT invoke the admin CreateAPIKey")
		require.Equal(t, "laptop", h.createMyReq.GetLabel())
	})

	t.Run("--user other invokes admin CreateAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{plaintext: "spgr_sk_admin_PLAINTEXT"}
		withAPIKeyStubClient(t, h)
		apiKeyCreateUser = "user-other"
		apiKeyCreateLabel = ""

		runAPIKeyCmd(t, authAPIKeyCreateCmd)
		require.NotNil(t, h.createReq, "admin create must invoke CreateAPIKey")
		require.Nil(t, h.createMyReq, "admin create must NOT invoke the self CreateMyAPIKey")
		require.Equal(t, "user-other", h.createReq.GetUserId())
	})
}

// TestAuthAPIKeyCreate_SelfPrintsPlaintextOnce asserts the self create emits the
// plaintext exactly once with a secret-manager instruction and writes no file.
func TestAuthAPIKeyCreate_SelfPrintsPlaintextOnce(t *testing.T) {
	defer func(u, l, d, e string, j bool) {
		apiKeyCreateUser, apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON = u, l, d, e, j
	}(apiKeyCreateUser, apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON)
	apiKeyCreateUser, apiKeyCreateLabel, apiKeyCreateDown, apiKeyCreateExpires, authJSON = "", "", "", "", false

	const secret = "spgr_sk_self_ONLYONCE_9f3a"
	h := &apiKeyStubHandler{plaintext: secret}
	withAPIKeyStubClient(t, h)

	out := runAPIKeyCmd(t, authAPIKeyCreateCmd)
	require.Equal(t, 1, strings.Count(out, secret), "plaintext must be printed exactly once")
	require.Contains(t, out, "will not be shown again")
	require.Contains(t, strings.ToLower(out), "secret manager", "must instruct storing in a secret manager")
	require.NotContains(t, strings.ToLower(out), "export ", "must not run or suggest an export command that leaks to history")
}

// TestAuthAPIKeyList_RoutesOnUser asserts list routes to ListMyAPIKeys with no
// --user and to the admin ListAPIKeys with --user <id>.
func TestAuthAPIKeyList_RoutesOnUser(t *testing.T) {
	defer func(u string, r, j bool) { apiKeyListUser, apiKeyListRevoked, authJSON = u, r, j }(apiKeyListUser, apiKeyListRevoked, authJSON)
	apiKeyListRevoked, authJSON = false, false

	t.Run("no --user invokes ListMyAPIKeys", func(t *testing.T) {
		h := &apiKeyStubHandler{}
		withAPIKeyStubClient(t, h)
		apiKeyListUser = ""
		runAPIKeyCmd(t, authAPIKeyListCmd)
		require.Equal(t, 1, h.listMyCalls, "self list must invoke ListMyAPIKeys")
		require.Nil(t, h.listReq, "self list must NOT invoke the admin ListAPIKeys")
	})

	t.Run("--user other invokes admin ListAPIKeys", func(t *testing.T) {
		h := &apiKeyStubHandler{}
		withAPIKeyStubClient(t, h)
		apiKeyListUser = "user-other"
		runAPIKeyCmd(t, authAPIKeyListCmd)
		require.NotNil(t, h.listReq, "admin list must invoke ListAPIKeys")
		require.Equal(t, 0, h.listMyCalls, "admin list must NOT invoke ListMyAPIKeys")
		require.Equal(t, "user-other", h.listReq.GetUserId())
	})
}

// TestAuthAPIKeyRotate_RoutesOnUser asserts rotate routes to RotateMyAPIKey with
// no --user and to the admin RotateAPIKey with --user <id>.
func TestAuthAPIKeyRotate_RoutesOnUser(t *testing.T) {
	defer func(u, e string, j bool) { apiKeyRotateUser, apiKeyRotateExpires, authJSON = u, e, j }(apiKeyRotateUser, apiKeyRotateExpires, authJSON)
	apiKeyRotateExpires, authJSON = "", false

	t.Run("no --user invokes RotateMyAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{plaintext: "spgr_sk_rot_self"}
		withAPIKeyStubClient(t, h)
		apiKeyRotateUser = ""
		runAPIKeyCmd(t, authAPIKeyRotateCmd, "key-1")
		require.NotNil(t, h.rotateMyReq, "self rotate must invoke RotateMyAPIKey")
		require.Nil(t, h.rotateReq, "self rotate must NOT invoke the admin RotateAPIKey")
		require.Equal(t, "key-1", h.rotateMyReq.GetKeyId())
	})

	t.Run("--user other invokes admin RotateAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{plaintext: "spgr_sk_rot_admin"}
		withAPIKeyStubClient(t, h)
		apiKeyRotateUser = "user-other"
		runAPIKeyCmd(t, authAPIKeyRotateCmd, "key-1")
		require.NotNil(t, h.rotateReq, "admin rotate must invoke RotateAPIKey")
		require.Nil(t, h.rotateMyReq, "admin rotate must NOT invoke RotateMyAPIKey")
	})
}

// TestAuthAPIKeyRevoke_RoutesOnUser asserts revoke routes to RevokeMyAPIKey with
// no --user and to the admin RevokeAPIKey with --user <id>.
func TestAuthAPIKeyRevoke_RoutesOnUser(t *testing.T) {
	defer func(u string) { apiKeyRevokeUser = u }(apiKeyRevokeUser)

	t.Run("no --user invokes RevokeMyAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{}
		withAPIKeyStubClient(t, h)
		apiKeyRevokeUser = ""
		runAPIKeyCmd(t, authAPIKeyRevokeCmd, "key-1")
		require.NotNil(t, h.revokeMyReq, "self revoke must invoke RevokeMyAPIKey")
		require.Nil(t, h.revokeReq, "self revoke must NOT invoke the admin RevokeAPIKey")
		require.Equal(t, "key-1", h.revokeMyReq.GetKeyId())
	})

	t.Run("--user other invokes admin RevokeAPIKey", func(t *testing.T) {
		h := &apiKeyStubHandler{}
		withAPIKeyStubClient(t, h)
		apiKeyRevokeUser = "user-other"
		runAPIKeyCmd(t, authAPIKeyRevokeCmd, "key-1")
		require.NotNil(t, h.revokeReq, "admin revoke must invoke RevokeAPIKey")
		require.Nil(t, h.revokeMyReq, "admin revoke must NOT invoke RevokeMyAPIKey")
	})
}
