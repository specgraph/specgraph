// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

func TestUserToProto_HumanActive(t *testing.T) {
	created := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	u := &storage.User{
		ID: "u1", Kind: storage.KindHuman, DisplayName: "Alice",
		Email: "a@x.com", Role: "admin", CreatedAt: created,
	}
	pb, err := userToProto(u)
	require.NoError(t, err)
	require.Equal(t, "u1", pb.GetId())
	require.Equal(t, specv1.UserKind_USER_KIND_HUMAN, pb.GetKind())
	require.Equal(t, "Alice", pb.GetDisplayName())
	require.Equal(t, "admin", pb.GetRole())
	require.Equal(t, created.Unix(), pb.GetCreatedAt().GetSeconds())
	require.Nil(t, pb.GetDeletedAt(), "active user has no deleted_at")
}

func TestUserToProto_SoftDeleted(t *testing.T) {
	del := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	u := &storage.User{ID: "u2", Kind: storage.KindServiceAccount, Role: "reader", DeletedAt: &del}
	pb, err := userToProto(u)
	require.NoError(t, err)
	require.Equal(t, specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, pb.GetKind())
	require.Equal(t, del.Unix(), pb.GetDeletedAt().GetSeconds())
}

func TestUserToProto_UnknownKindErrors(t *testing.T) {
	_, err := userToProto(&storage.User{ID: "u3", Kind: storage.Kind("alien")})
	require.Error(t, err)
}

func TestAPIKeyToProto_NoSecretMaterial(t *testing.T) {
	k := &storage.APIKey{
		ID: "k1", UserID: "u1", Prefix: "abc12345",
		PHCHash: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaA",
		Label:   "ci", CreatedAt: time.Now(),
	}
	pb := apiKeyToProto(k)
	require.Equal(t, "k1", pb.GetId())
	require.Equal(t, "abc12345", pb.GetPrefix())
	require.Equal(t, "ci", pb.GetLabel())
	require.NotContains(t, pb.String(), "argon2", "PHC hash must never be serialized")
}

func TestOIDCBindingToProto(t *testing.T) {
	b := &storage.OIDCBinding{ID: "b1", UserID: "u1", Issuer: "https://idp", Subject: "sub", EmailAtBind: "a@x.com", CreatedAt: time.Now()}
	pb := oidcBindingToProto(b)
	require.Equal(t, "b1", pb.GetId())
	require.Equal(t, "https://idp", pb.GetIssuer())
	require.Equal(t, "sub", pb.GetSubject())
}
