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

// TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive exercises the
// resolveJWT login-sync gate end-to-end through the existing-binding branch
// against a real AuthStore + OIDCVerifier. A login-sync-enabled resolver with a
// claims mapping that grants admin must:
//   - leave the persisted role untouched on a NON-interactive bearer resolve
//     (sync did not run); and
//   - re-derive + persist the elevated role on an INTERACTIVE login resolve
//     (sync ran). The applyLoginSync algorithm itself is fully covered by the
//     white-box tests in loginsync_internal_test.go; this asserts only the
//     two-line gate.
func TestIdentityStore_JWT_LoginSync_GateRunsOnlyInteractive(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)

	pool := postgrestest.SharedPool(t, ctx)
	store, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close(ctx) })

	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	verifier, err := auth.NewOIDCVerifier(ctx, config.OIDCProviderConfig{
		ID: "test", Issuer: issuer.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	tracker := usagetracker.NewManager(store, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })

	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:            store,
		Tracker:          tracker,
		Verifiers:        []*auth.OIDCVerifier{verifier},
		LoginSyncEnabled: true,
		JITDefaultRole:   auth.RoleReader,
		KnownRoles:       auth.KnownRolesFrom(nil),
		JITClaimsMapping: map[string][]config.ClaimMapping{
			issuer.server.URL: {{Claim: "roles", Value: "app.admin", Role: "admin"}},
		},
	})
	require.NoError(t, err)

	const subject = "oidc-subject-loginsync"
	user, err := store.CreateHuman(ctx, &storage.User{
		Kind: storage.KindHuman, DisplayName: "Carol", Email: "carol@example.com", Role: "reader",
	}, &storage.OIDCBinding{
		Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "carol@example.com",
	})
	require.NoError(t, err)

	token := issuer.mintToken(t, map[string]any{
		"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "carol@example.com", "roles": []string{"app.admin"},
	})

	// Non-interactive bearer resolve: the gate is closed, so login-sync does
	// not run and the persisted reader role is surfaced unchanged.
	id, err := resolver.Resolve(ctx, token)
	require.NoError(t, err)
	require.Equal(t, user.ID, id.UserID)
	require.Equal(t, auth.Role("reader"), id.Role, "non-interactive resolve must not run login-sync")
	persisted, err := store.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "reader", persisted.Role, "persisted role must be untouched without interactive login")

	// Interactive login resolve: the gate opens, login-sync re-derives the
	// admin role from the roles claim and persists it.
	id, err = resolver.Resolve(auth.WithInteractiveLogin(ctx), token)
	require.NoError(t, err)
	require.Equal(t, user.ID, id.UserID)
	require.Equal(t, auth.RoleAdmin, id.Role, "interactive login must run login-sync and elevate the role")
	persisted, err = store.GetUserByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, "admin", persisted.Role, "interactive login must persist the re-derived role")
}

// TestIdentityStore_DisplayNameReconciliation proves the D-01 decoupling
// end-to-end against real Postgres: a stale (display_name == subject)
// JIT-fallback display name self-heals to a usable "name" claim on BOTH the
// interactive resolve path (ResolveLogin) and the non-interactive bearer
// resolve path (Resolve) — reconciliation is unconditional, not gated by
// LoginSyncEnabled or the interactive/non-interactive distinction (AUTH-06
// SC2/SC3). It also covers the SC4 no-regression direction: a resolve with no
// usable name claim leaves an operator-set display_name unchanged.
func TestIdentityStore_DisplayNameReconciliation(t *testing.T) {
	ctx := context.Background()
	issuer := newOIDCTestIssuer(t)
	store, resolver := authnTestStore(t, issuer, "aud-1")

	t.Run("interactive resolve (ResolveLogin) reconciles a stale display_name", func(t *testing.T) {
		const subject = "oidc-subject-reconcile-interactive"
		user, err := store.CreateHuman(ctx, &storage.User{
			Kind: storage.KindHuman, DisplayName: subject, Email: "dana@example.com", Role: "reader",
		}, &storage.OIDCBinding{
			Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "dana@example.com",
		})
		require.NoError(t, err)

		id, err := resolver.ResolveLogin(ctx, &auth.OIDCClaims{
			Issuer: issuer.server.URL, Subject: subject, Email: "dana@example.com", Name: "Dana Scully",
		})
		require.NoError(t, err)
		require.Equal(t, user.ID, id.UserID)
		require.Equal(t, "Dana Scully", id.DisplayName)

		persisted, err := store.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Dana Scully", persisted.DisplayName,
			"interactive resolve must persist the reconciled display_name")
		require.Equal(t, "reader", persisted.Role, "reconciliation must not touch role")
	})

	t.Run("non-interactive bearer resolve (Resolve) reconciles a stale display_name", func(t *testing.T) {
		const subject = "oidc-subject-reconcile-noninteractive"
		user, err := store.CreateHuman(ctx, &storage.User{
			Kind: storage.KindHuman, DisplayName: subject, Email: "fox@example.com", Role: "reader",
		}, &storage.OIDCBinding{
			Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "fox@example.com",
		})
		require.NoError(t, err)

		token := issuer.mintToken(t, map[string]any{
			"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": "fox@example.com", "name": "Fox Mulder",
		})

		id, err := resolver.Resolve(ctx, token)
		require.NoError(t, err)
		require.Equal(t, user.ID, id.UserID)

		persisted, err := store.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Fox Mulder", persisted.DisplayName,
			"non-interactive bearer resolve must ALSO persist the reconciled display_name — "+
				"reconciliation is unconditional, not gated by interactive/LoginSyncEnabled")
		require.Equal(t, "reader", persisted.Role, "reconciliation must not touch role")
	})

	t.Run("no usable name claim leaves an operator-set display_name unchanged (SC4)", func(t *testing.T) {
		const subject = "oidc-subject-reconcile-operator-set"
		user, err := store.CreateHuman(ctx, &storage.User{
			Kind: storage.KindHuman, DisplayName: "Operator Renamed", Email: "walter@example.com", Role: "reader",
		}, &storage.OIDCBinding{
			Issuer: issuer.server.URL, Subject: subject, EmailAtBind: "walter@example.com",
		})
		require.NoError(t, err)

		token := issuer.mintToken(t, map[string]any{
			"iss": issuer.server.URL, "sub": subject, "aud": "aud-1",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": "walter@example.com", // no "name" / "preferred_username" claim
		})

		_, err = resolver.Resolve(ctx, token)
		require.NoError(t, err)

		persisted, err := store.GetUserByID(ctx, user.ID)
		require.NoError(t, err)
		require.Equal(t, "Operator Renamed", persisted.DisplayName,
			"an operator-set display_name must survive a resolve with no usable name claim")
	})
}
