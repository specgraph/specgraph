// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

func TestUserList_RendersRows(t *testing.T) {
	out := render.UserList([]*specv1.User{
		{Id: "u1", Kind: specv1.UserKind_USER_KIND_HUMAN, DisplayName: "Alice", Role: "admin", CreatedAt: timestamppb.Now()},
	})
	require.Contains(t, out, "u1")
	require.Contains(t, out, "Alice")
	require.Contains(t, out, "admin")
}

func TestUserList_Empty(t *testing.T) {
	require.Contains(t, strings.ToLower(render.UserList(nil)), "no users")
}

func TestAPIKeyList_RedactsSecret(t *testing.T) {
	out := render.APIKeyList([]*specv1.APIKey{{Id: "k1", Prefix: "abc12345", Label: "ci"}})
	require.Contains(t, out, "abc12345")
	require.Contains(t, out, "ci")
}

func TestOIDCBindingList_RendersRows(t *testing.T) {
	out := render.OIDCBindingList([]*specv1.OIDCBinding{{Id: "b1", Issuer: "https://idp", Subject: "sub"}})
	require.Contains(t, out, "https://idp")
}
