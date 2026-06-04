// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// authnTestStore builds a real AuthStore + a JIT-enabled resolver wired to a
// real OIDCVerifier pointed at issuer. The JWT-resolve and JIT-create paths are
// unit-tested with a UsersBackend stub elsewhere; this exercises them end-to-end
// against Postgres. Tables are truncated for a clean slate. Returns the store
// (for seeding + inspection) and the resolver under test.
func authnTestStore(t *testing.T, issuer *oidcTestIssuer, audience string) (*postgres.AuthStore, auth.Resolver) {
	t.Helper()
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	store, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	verifier, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: issuer.server.URL, ClientID: audience,
	})
	require.NoError(t, err)

	tracker := usagetracker.NewManager(store, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:          store,
		Tracker:        tracker,
		Verifiers:      []*auth.OIDCVerifier{verifier},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		KnownRoles:     auth.KnownRolesFrom(nil),
	})
	require.NoError(t, err)
	return store, resolver
}

// TestIdentityStore_JWT_ResolvesViaExistingBinding verifies that a JWT whose
// (issuer, subject) already has a persisted OIDC binding resolves to the bound
// user — end-to-end against a real AuthStore and OIDCVerifier.
func TestIdentityStore_JWT_ResolvesViaExistingBinding(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	store, resolver := authnTestStore(t, issuer, "aud-1")

	const subject = "oidc-subject-existing"
	user, err := store.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "Alice", Email: "alice@example.com", Role: "writer",
	}, &storage.OIDCBinding{
		Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "alice@example.com",
	})
	require.NoError(t, err)

	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "alice@example.com",
	})

	id, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, user.ID, id.UserID, "must resolve to the bound user")
	require.Equal(t, "oidc:"+subject, id.Subject)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, auth.Role("writer"), id.EffectiveRole, "OIDC has no per-key downgrade")
	require.Equal(t, "oidc", id.Source)
}

// TestIdentityStore_JWT_JITCreatesThenResolvesViaBinding verifies the
// just-in-time provisioning lifecycle end-to-end: a first login for an unknown
// subject creates a user + binding, and a second login resolves via that
// binding (same user, no duplicate binding).
func TestIdentityStore_JWT_JITCreatesThenResolvesViaBinding(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	store, resolver := authnTestStore(t, issuer, "aud-1")

	const subject = "oidc-subject-jit"
	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "bob@example.com",
	})

	// Precondition: no binding for this subject yet.
	_, err := store.LookupOIDCBinding(ctx, issuer.server.URL, subject)
	require.ErrorIs(t, err, storage.ErrOIDCBindingNotFound)

	// First login → JIT creates the user + binding.
	id1, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.NotEmpty(t, id1.UserID)
	require.Equal(t, "oidc", id1.Source)
	require.Equal(t, auth.RoleReader, id1.Role, "JIT uses the configured default role")
	require.Equal(t, "bob@example.com", id1.Email)

	// The binding + user are persisted.
	binding, err := store.LookupOIDCBinding(ctx, issuer.server.URL, subject)
	require.NoError(t, err)
	require.Equal(t, id1.UserID, binding.UserID)
	persisted, err := store.GetUserByID(ctx, id1.UserID)
	require.NoError(t, err)
	require.Equal(t, "bob@example.com", persisted.Email)
	require.Equal(t, "reader", persisted.Role)

	// Second login → resolves via the now-existing binding: same user, and no
	// duplicate binding created.
	id2, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, id1.UserID, id2.UserID, "second login must resolve to the same JIT-created user")

	bindings, err := store.ListOIDCBindings(ctx, id1.UserID)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "second login must resolve via the binding, not create a duplicate")
}
