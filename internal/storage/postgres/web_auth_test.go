// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/postgres"
)

func TestWebAuth_SessionLifecycle(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&userID))

	hash := []byte("0123456789abcdef0123456789abcdef") // 32 bytes stand-in
	sess, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: hash, UserID: userID, Issuer: "iss", OIDCSubject: "sub",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	require.NotEmpty(t, sess.ID)

	got, err := auth.LookupSessionByHash(ctx, hash)
	require.NoError(t, err)
	require.Equal(t, userID, got.UserID)
	require.True(t, got.IsActive(time.Now()))

	require.NoError(t, auth.RevokeSession(ctx, hash))
	got, err = auth.LookupSessionByHash(ctx, hash)
	require.NoError(t, err)
	require.False(t, got.IsActive(time.Now()))

	_, err = auth.LookupSessionByHash(ctx, []byte("nope"))
	require.ErrorIs(t, err, storage.ErrSessionNotFound)
}

func TestWebAuth_CreateSessionUnknownUser(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	_, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: []byte("x"), UserID: "00000000-0000-0000-0000-000000000000",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	require.ErrorIs(t, err, storage.ErrUserNotFound)
}

func TestWebAuth_DeleteExpiredSessions(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&userID))

	_, err := auth.CreateSession(ctx, &storage.Session{
		TokenHash: []byte("expired-hash"), UserID: userID,
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	n, err := auth.DeleteExpiredSessions(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(1))
}

func TestWebAuth_LoginFlowConsumeSingleUse(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "st", Nonce: "no", CodeVerifier: "cv", ProviderID: "entra",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	require.NoError(t, err)
	require.NotEmpty(t, id)

	f, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "st", f.State)
	require.Equal(t, "entra", f.ProviderID)

	_, err = auth.ConsumeLoginFlow(ctx, id)
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)

	_, err = auth.ConsumeLoginFlow(ctx, "not-a-uuid")
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)
}

func TestWebAuth_LoginFlowExpired(t *testing.T) {
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)

	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "st", Nonce: "no", CodeVerifier: "cv", ProviderID: "entra",
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	require.NoError(t, err)
	_, err = auth.ConsumeLoginFlow(ctx, id)
	require.ErrorIs(t, err, storage.ErrLoginFlowNotFound)
}

// newAuthStoreWithUser seeds a clean auth store with a single user and returns
// the store plus that user's id. Mirrors the inline setup used by the older
// tests (authTestSetup + sharedTestPool + truncateAuthTables + user insert);
// because it truncates the shared pool it is NOT parallel-safe, so the CLI
// tests below run sequentially like the rest of this file.
func newAuthStoreWithUser(t *testing.T) (*postgres.AuthStore, string) {
	t.Helper()
	ctx := context.Background()
	auth := authTestSetup(t)
	pool := sharedTestPool(t, ctx)
	truncateAuthTables(t, pool)
	var userID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO users (kind, display_name, email, role) VALUES ('human','U','u@example.com','reader') RETURNING id`).Scan(&userID))
	return auth, userID
}

func TestExchangeCLICode_RoundTrip(t *testing.T) {
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()

	codeHash := sha256.Sum256([]byte("rawcode"))
	challenge := "CHALLENGE"
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "oidc:subj", challenge, time.Now().Add(time.Minute)))

	tokenHash := sha256.Sum256([]byte("spgr_ws_token"))
	sess, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour),
	}, challenge)
	require.NoError(t, err)
	require.Equal(t, userID, sess.UserID)
	require.Equal(t, "oidc:subj", sess.OIDCSubject)

	// Single-use: second exchange fails.
	_, err = auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{
		TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour),
	}, challenge)
	require.ErrorIs(t, err, storage.ErrCLICodeNotFound)
}

func TestExchangeCLICode_ChallengeMismatchLeavesCode(t *testing.T) {
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()

	codeHash := sha256.Sum256([]byte("rawcode2"))
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "", "GOOD", time.Now().Add(time.Minute)))

	tokenHash := sha256.Sum256([]byte("tok"))
	_, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "BAD")
	require.ErrorIs(t, err, storage.ErrCLIChallengeMismatch)

	// Code is NOT consumed; a correct verifier still works.
	sess, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "GOOD")
	require.NoError(t, err)
	require.Equal(t, userID, sess.UserID)
}

func TestExchangeCLICode_Expired(t *testing.T) {
	auth, userID := newAuthStoreWithUser(t)
	ctx := context.Background()
	codeHash := sha256.Sum256([]byte("rawcode3"))
	require.NoError(t, auth.CreateCLICode(ctx, codeHash[:], userID, "", "C", time.Now().Add(-time.Second)))
	tokenHash := sha256.Sum256([]byte("tok"))
	_, err := auth.ExchangeCLICode(ctx, codeHash[:], &storage.Session{TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)}, "C")
	require.ErrorIs(t, err, storage.ErrCLICodeNotFound)
}

func TestLoginFlow_CLIFieldsRoundTrip(t *testing.T) {
	auth, _ := newAuthStoreWithUser(t)
	ctx := context.Background()
	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "s", Nonce: "n", CodeVerifier: "v", ProviderID: "p",
		CLICallback: "http://127.0.0.1:5000/callback", CLIState: "cs", CLIChallenge: "cc",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	got, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "http://127.0.0.1:5000/callback", got.CLICallback)
	require.Equal(t, "cs", got.CLIState)
	require.Equal(t, "cc", got.CLIChallenge)
}

// TestLoginFlow_WebStillWorks guards against the nullable-scan regression:
// the new NOT NULL DEFAULT '' columns must let a web-only flow round-trip.
func TestLoginFlow_WebStillWorks(t *testing.T) {
	auth, _ := newAuthStoreWithUser(t)
	ctx := context.Background()
	id, err := auth.CreateLoginFlow(ctx, &storage.LoginFlow{
		State: "s", Nonce: "n", CodeVerifier: "v", ProviderID: "p",
		ExpiresAt: time.Now().Add(time.Minute),
	})
	require.NoError(t, err)
	got, err := auth.ConsumeLoginFlow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "", got.CLICallback)
}
