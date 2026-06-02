// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"testing"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/stretchr/testify/require"
)

func TestBuildActionEntities_GroupsByVerb(t *testing.T) {
	ents, err := buildActionEntities([]string{"spec.read", "spec.write", "graph.delete"})
	require.NoError(t, err)

	readGroup := cedar.NewEntityUID("SpecGraph::Action", "read")
	specRead := cedar.NewEntityUID("SpecGraph::Action", "spec.read")

	require.Contains(t, ents, readGroup, "verb group entity must exist")
	require.Contains(t, ents, specRead, "concrete action entity must exist")

	specReadEnt := ents[specRead]
	require.True(t, specReadEnt.Parents.Contains(readGroup),
		"spec.read must be a member of the read group")
}

func TestBuildActionEntities_RejectsUnknownVerb(t *testing.T) {
	_, err := buildActionEntities([]string{"spec.frobnicate"})
	require.Error(t, err)
}

func TestPrincipalEntity_CarriesRoleAndID(t *testing.T) {
	id := &Identity{UserID: "u1", EffectiveRole: RoleWriter, Email: "a@example.com", Subject: "apikey:k1"}
	uid, ent := principalEntity(id)

	require.Equal(t, cedar.NewEntityUID("SpecGraph::User", "u1"), uid)
	role, ok := ent.Attributes.Get("role")
	require.True(t, ok)
	require.Equal(t, cedar.String("writer"), role)
	email, ok := ent.Attributes.Get("email")
	require.True(t, ok)
	require.Equal(t, cedar.String("a@example.com"), email)
	gotID, ok := ent.Attributes.Get("id")
	require.True(t, ok)
	require.Equal(t, cedar.String("u1"), gotID)
}

func TestPrincipalEntity_FallsBackToSubject(t *testing.T) {
	id := &Identity{UserID: "", EffectiveRole: RoleReader, Subject: "apikey:k9"}
	uid, _ := principalEntity(id)
	require.Equal(t, cedar.NewEntityUID("SpecGraph::User", "apikey:k9"), uid)
}

func TestResourceEntity_Defaults(t *testing.T) {
	uid, ent := resourceEntity(ResourceRef{Type: "spec"})
	require.Equal(t, cedar.NewEntityUID("SpecGraph::Resource", "unspecified"), uid)
	_ = ent
}

func TestResourceEntity_CarriesAttributes(t *testing.T) {
	_, ent := resourceEntity(ResourceRef{Type: "apikey", ID: "key-1", Attributes: map[string]string{"owner_user_id": "u1"}})
	owner, ok := ent.Attributes.Get("owner_user_id")
	require.True(t, ok)
	require.Equal(t, cedar.String("u1"), owner)
}
