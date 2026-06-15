// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/storage"
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
