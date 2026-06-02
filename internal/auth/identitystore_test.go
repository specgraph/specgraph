// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
)

func TestNewIdentityStore_RequiresUsers(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Users required")
}

func TestNewIdentityStore_RequiresTracker(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: &usersBackendStub{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "Tracker required")
}

func TestNewIdentityStore_BuildsSuccessfully(t *testing.T) {
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   &usersBackendStub{},
		Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	require.NotNil(t, store)
}

func TestNewIdentityStore_RejectsUnknownJITDefaultRole(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          &usersBackendStub{},
		Tracker:        &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: "reder", // typo for "reader"
		KnownRoles:     map[auth.Role]bool{auth.RoleReader: true, auth.RoleWriter: true, auth.RoleAdmin: true},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "JITDefaultRole")
}

func TestNewIdentityStore_RejectsUnknownClaimsMappingRole(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          &usersBackendStub{},
		Tracker:        &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		KnownRoles:     map[auth.Role]bool{auth.RoleReader: true},
		JITClaimsMapping: map[string][]config.ClaimMapping{
			"https://issuer": {{Claim: "groups", Value: "admins", Role: "superuser"}},
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown role")
}

// noopTracker implements auth.LastUsedTracker as a no-op stub used until
// Task 25 wires usagetracker.Manager.
type noopTracker struct{}

func (noopTracker) Touch(string) {}

func TestResolve_EmptyTokenUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolve_JWTShapeRoutesToOIDC(t *testing.T) {
	store := newTestIdentityStore(t)
	// 3-segment string but garbage payload — dispatches to OIDC, which
	// will fail because no verifier matches the issuer.
	_, err := store.Resolve(context.Background(), "abc.def.ghi")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolve_APIKeyShapeRoutesToKeyPath(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "spgr_sk_abc12345_thirtytwocharsecretthirtytwocha")
	require.ErrorIs(t, err, auth.ErrUnauthenticated) // no key in stub
}

// newTestIdentityStore builds an empty pgIdentityStore for dispatch tests.
func newTestIdentityStore(t *testing.T) auth.Resolver {
	t.Helper()
	r, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:   &usersBackendStub{},
		Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	return r
}
