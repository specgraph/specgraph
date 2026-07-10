// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/auth/usagetracker"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
	"github.com/specgraph/specgraph/internal/storage/postgres/postgrestest"
)

// buildIdentityTestServer wires the real interceptor (resolver + Cedar
// authorizer) and the IdentityService over an httptest server. Returns the
// base URL and a helper to mint a token for a user of a given role.
func buildIdentityTestServer(t *testing.T, ctx context.Context) (string, *postgres.AuthStore) {
	t.Helper()
	pool := postgrestest.SharedPool(t, ctx) // postgres harness
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)

	engine, err := auth.NewCedarEngine(ctx, []auth.PolicySource{auth.NewEmbeddedPolicySource()}, auth.ActionNames())
	require.NoError(t, err)
	authorizer := auth.NewCedarAuthorizer(engine)
	interceptor := auth.NewAuthInterceptor(resolver, authorizer)

	mux := http.NewServeMux()
	server.RegisterIdentityService(mux, authStore, config.SelfServiceKeysConfig{
		DefaultTTLDays: 90, MaxTTLDays: 180, Quota: 10, RateLimitPerHour: 30, RateLimitBurst: 5,
	}, connect.WithInterceptors(interceptor))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL, authStore
}

// mintFor creates a user of the given role + an API key, returning the token.
func mintFor(t *testing.T, ctx context.Context, store *postgres.AuthStore, role string, bootstrap bool) string {
	t.Helper()
	u, err := store.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: role + "-user", Role: role, Bootstrap: bootstrap}, nil)
	require.NoError(t, err)
	secret, phc, err := auth.GenerateAPIKeySecret()
	require.NoError(t, err)
	created, err := store.CreateAPIKey(ctx, &storage.APIKey{UserID: u.ID, PHCHash: phc}) // storage assigns the prefix
	require.NoError(t, err)
	return auth.FormatAPIKeyToken(created.Prefix, secret)
}

func tokenClient(baseURL, token string) specgraphv1connect.IdentityServiceClient {
	httpc := &http.Client{Transport: &bearerTransport{token: token, base: http.DefaultTransport}}
	return specgraphv1connect.NewIdentityServiceClient(httpc, baseURL)
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (b *bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("Authorization", "Bearer "+b.token)
	return b.base.RoundTrip(r2)
}

func TestIntegration_IdentityAdminCanManage(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true)

	client := tokenClient(baseURL, adminToken)
	resp, err := client.ListUsers(ctx, connect.NewRequest(&specv1.ListUsersRequest{}))
	require.NoError(t, err, "admin may ListUsers (user.manage)")
	require.GreaterOrEqual(t, len(resp.Msg.GetUsers()), 1)
}

func TestIntegration_IdentityReaderDeniedManage(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	_ = mintFor(t, ctx, store, "admin", true) // sole bootstrap user (users_one_bootstrap allows only one active)
	readerToken := mintFor(t, ctx, store, "reader", false)

	client := tokenClient(baseURL, readerToken)
	_, err := client.ListUsers(ctx, connect.NewRequest(&specv1.ListUsersRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "reader denied user.manage")
}

func TestIntegration_IdentityWhoamiOpenToReader(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	_ = mintFor(t, ctx, store, "admin", true) // sole bootstrap user (users_one_bootstrap allows only one active)
	readerToken := mintFor(t, ctx, store, "reader", false)

	client := tokenClient(baseURL, readerToken)
	resp, err := client.Whoami(ctx, connect.NewRequest(&specv1.WhoamiRequest{}))
	require.NoError(t, err, "any authenticated principal may whoami (identity.read)")
	require.Equal(t, "reader", resp.Msg.GetEffectiveRole())
}

func TestIntegration_BootstrapDeleteProtected(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true) // bootstrap admin
	client := tokenClient(baseURL, adminToken)

	boot, err := store.GetBootstrap(ctx)
	require.NoError(t, err)

	// Without force: refused.
	_, err = client.SoftDeleteUser(ctx, connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: boot.ID}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// With force: allowed.
	_, err = client.SoftDeleteUser(ctx, connect.NewRequest(&specv1.SoftDeleteUserRequest{Id: boot.ID, Force: true}))
	require.NoError(t, err)
}

func TestIntegration_LastCredentialUnbindProtected(t *testing.T) {
	ctx := context.Background()
	baseURL, store := buildIdentityTestServer(t, ctx)
	adminToken := mintFor(t, ctx, store, "admin", true)
	client := tokenClient(baseURL, adminToken)

	// A Human with exactly one OIDC binding and no API keys.
	u, err := store.CreateHuman(ctx, &storage.User{Kind: storage.KindHuman, DisplayName: "person", Role: "reader"},
		&storage.OIDCBinding{Issuer: "https://idp", Subject: "sub-1"})
	require.NoError(t, err)
	bindings, err := store.ListOIDCBindings(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, bindings, 1)

	// Without force: refused (only credential).
	_, err = client.UnbindOIDC(ctx, connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: bindings[0].ID, UserId: u.ID}))
	require.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))

	// With force: allowed.
	_, err = client.UnbindOIDC(ctx, connect.NewRequest(&specv1.UnbindOIDCRequest{BindingId: bindings[0].ID, UserId: u.ID, Force: true}))
	require.NoError(t, err)
}

// TestIntegration_SessionIssuer proves AUTH-05 / D-09 end-to-end: an interactive
// resolve (ResolveLogin) surfaces the verified issuer on the Identity, and a web
// session minted from it persists issuer = the authenticating provider's issuer.
// It also asserts the no-backfill invariant (D-10): a pre-existing empty-issuer
// session is left untouched.
func TestIntegration_SessionIssuer(t *testing.T) {
	ctx := context.Background()
	pool := postgrestest.SharedPool(t, ctx)
	authStore, err := postgres.NewAuth(ctx, pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = authStore.Close(ctx) })
	_, err = pool.Exec(ctx, `TRUNCATE users CASCADE`)
	require.NoError(t, err)

	const issuer = "https://idp.example.com"
	// Existing user + OIDC binding so ResolveLogin hits the binding path.
	u, err := authStore.CreateHuman(ctx,
		&storage.User{Kind: storage.KindHuman, DisplayName: "person", Role: "reader"},
		&storage.OIDCBinding{Issuer: issuer, Subject: "sub-1"})
	require.NoError(t, err)

	// A pre-existing session with an EMPTY issuer (the pre-AUTH-05 shape).
	staleHash := []byte("stale-session-hash-000000000000000")
	stale, err := authStore.CreateSession(ctx, &storage.Session{
		TokenHash: staleHash, UserID: u.ID, OIDCSubject: "sub-1",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Empty(t, stale.Issuer, "precondition: stale session starts with empty issuer")

	tracker := usagetracker.NewManager(authStore, usagetracker.Config{})
	t.Cleanup(func() { _ = tracker.Close(ctx) })
	resolver, err := auth.NewIdentityStore(auth.IdentityStoreConfig{Users: authStore, Tracker: tracker})
	require.NoError(t, err)

	// Interactive resolve from verified claims → Identity carries the issuer.
	id, err := resolver.ResolveLogin(auth.WithInteractiveLogin(ctx),
		&auth.OIDCClaims{Issuer: issuer, Subject: "sub-1"})
	require.NoError(t, err)
	require.Equal(t, issuer, id.Issuer, "ResolveLogin must surface the verified issuer")

	// Mint a session threading the issuer, exactly as handleCallback does.
	freshHash := []byte("fresh-session-hash-0000000000000000")
	_, err = authStore.CreateSession(ctx, &storage.Session{
		TokenHash: freshHash, UserID: id.UserID, OIDCSubject: "sub-1",
		Issuer:    id.Issuer,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)

	// The freshly-minted session persists the issuer.
	got, err := authStore.LookupSessionByHash(ctx, freshHash)
	require.NoError(t, err)
	require.Equal(t, issuer, got.Issuer, "minted web session must persist the authenticating issuer")

	// No backfill (D-10): the pre-existing empty-issuer session is untouched.
	stillStale, err := authStore.LookupSessionByHash(ctx, staleHash)
	require.NoError(t, err)
	require.Empty(t, stillStale.Issuer, "no-backfill: pre-existing empty-issuer session must remain empty")
}
