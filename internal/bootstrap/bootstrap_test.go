// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package bootstrap_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/bootstrap"
	"github.com/specgraph/specgraph/internal/storage"
)

// stub implements the narrow bootstrap.Backend (GetBootstrap, CreateHuman,
// CreateAPIKey) that bootstrap.Ensure depends on.
type stub struct {
	bootstrapUser *storage.User
	createHuman   func(*storage.User) (*storage.User, error)
	createdKey    *storage.APIKey
}

func (s *stub) GetBootstrap(context.Context) (*storage.User, error) {
	if s.bootstrapUser == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.bootstrapUser, nil
}
func (s *stub) CreateHuman(_ context.Context, u *storage.User, _ *storage.OIDCBinding) (*storage.User, error) {
	return s.createHuman(u)
}
func (s *stub) CreateAPIKey(_ context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	k.ID = "key-1"
	k.Prefix = "boot1234" // simulate storage-assigned prefix
	s.createdKey = k
	return k, nil
}

// stub satisfies bootstrap.Backend with exactly these three methods — no
// other UsersBackend methods are needed because Ensure depends on the narrow
// Backend interface (Step 3).

func TestEnsure_CreatesBootstrapAdmin(t *testing.T) {
	var created *storage.User
	s := &stub{createHuman: func(u *storage.User) (*storage.User, error) {
		require.Equal(t, "admin", u.DisplayName, "system identity, not env-derived")
		require.True(t, u.Bootstrap)
		require.Equal(t, storage.KindHuman, u.Kind)
		u.ID = "boot-user"
		created = u
		return u, nil
	}}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created)
	require.NotEmpty(t, res.Token, "a new bootstrap returns the plaintext token")
	require.True(t, strings.HasPrefix(res.Token, auth.APIKeyTokenPrefix()))
	require.Contains(t, res.Token, "boot1234", "token embeds the storage-assigned prefix")
	require.Equal(t, "admin", created.Role)
}

func TestEnsure_IdempotentWhenExists(t *testing.T) {
	s := &stub{bootstrapUser: &storage.User{ID: "boot-user", DisplayName: "admin", Bootstrap: true, Role: "admin"}}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "existing bootstrap is a no-op")
	require.Empty(t, res.Token, "no token minted when bootstrap already exists")
}

// raceStub models losing the create race: GetBootstrap misses first (so
// Ensure proceeds to CreateHuman), CreateHuman returns ErrBootstrapExists
// (the winner created it concurrently), and the re-fetch GetBootstrap returns
// the winner.
type raceStub struct {
	winner   *storage.User
	getCalls int
}

func (r *raceStub) GetBootstrap(context.Context) (*storage.User, error) {
	r.getCalls++
	if r.getCalls == 1 {
		return nil, storage.ErrUserNotFound
	}
	return r.winner, nil
}
func (r *raceStub) CreateHuman(context.Context, *storage.User, *storage.OIDCBinding) (*storage.User, error) {
	return nil, storage.ErrBootstrapExists
}
func (r *raceStub) CreateAPIKey(context.Context, *storage.APIKey) (*storage.APIKey, error) {
	panic("CreateAPIKey must not be called when the create race is lost")
}

func TestEnsure_RaceLosesGracefully(t *testing.T) {
	r := &raceStub{winner: &storage.User{ID: "winner", DisplayName: "admin", Bootstrap: true}}
	res, err := bootstrap.Ensure(context.Background(), r, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "race loser observes the winner's bootstrap")
	require.Equal(t, "winner", res.UserID)
	require.Empty(t, res.Token, "loser mints no key")
}

// errStub returns a non-sentinel error from GetBootstrap; Ensure must surface
// it and mint nothing.
type errStub struct{ getErr error }

func (e *errStub) GetBootstrap(context.Context) (*storage.User, error) { return nil, e.getErr }
func (e *errStub) CreateHuman(context.Context, *storage.User, *storage.OIDCBinding) (*storage.User, error) {
	panic("CreateHuman must not be called when GetBootstrap errors")
}
func (e *errStub) CreateAPIKey(context.Context, *storage.APIKey) (*storage.APIKey, error) {
	panic("CreateAPIKey must not be called when GetBootstrap errors")
}

func TestEnsure_GetBootstrapErrorSurfaced(t *testing.T) {
	sentinel := errors.New("db unreachable")
	_, err := bootstrap.Ensure(context.Background(), &errStub{getErr: sentinel}, bootstrap.Options{})
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "non-ErrUserNotFound GetBootstrap error must be surfaced")
}
