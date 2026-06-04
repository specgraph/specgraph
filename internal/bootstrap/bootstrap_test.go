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
	keys          []*storage.APIKey // active keys returned by ListAPIKeys
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
func (s *stub) ListAPIKeys(_ context.Context, _ storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return s.keys, nil
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
	// Existing bootstrap user that ALREADY has an active key → true no-op.
	s := &stub{
		bootstrapUser: &storage.User{ID: "boot-user", DisplayName: "admin", Bootstrap: true, Role: "admin"},
		keys:          []*storage.APIKey{{ID: "existing-key", UserID: "boot-user"}},
	}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "existing bootstrap with a key is a no-op")
	require.Empty(t, res.Token, "no token minted when the bootstrap admin already has a key")
	require.Nil(t, s.createdKey, "no key minted when one already exists")
}

// TestEnsure_RecoversKeylessBootstrap covers the mint-failure-after-create
// window: a bootstrap user persisted with NO active key (an earlier Ensure
// crashed between CreateHuman and CreateAPIKey). A later Ensure must mint a key
// and return it, rather than no-op'ing forever on idempotency.
func TestEnsure_RecoversKeylessBootstrap(t *testing.T) {
	s := &stub{
		bootstrapUser: &storage.User{ID: "boot-user", DisplayName: "admin", Bootstrap: true, Role: "admin"},
		keys:          nil, // no active keys → unrecoverable without this fix
		createHuman: func(*storage.User) (*storage.User, error) {
			panic("CreateHuman must not be called when the bootstrap user already exists")
		},
	}
	res, err := bootstrap.Ensure(context.Background(), s, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created, "a keyless bootstrap must mint a recovery key")
	require.NotEmpty(t, res.Token, "the recovery token must be returned for the operator to save")
	require.True(t, strings.HasPrefix(res.Token, auth.APIKeyTokenPrefix()))
	require.Contains(t, res.Token, "boot1234", "token embeds the storage-assigned prefix")
	require.Equal(t, "boot-user", res.UserID)
	require.NotNil(t, s.createdKey, "a key must have been minted")
	require.Equal(t, "boot-user", s.createdKey.UserID, "the recovery key belongs to the existing bootstrap user")
}

// raceStub models losing the create race: GetBootstrap misses first (so
// Ensure proceeds to CreateHuman), CreateHuman returns ErrBootstrapExists
// (the winner created it concurrently), and the re-fetch GetBootstrap returns
// the winner.
type raceStub struct {
	winner     *storage.User
	getCalls   int
	keys       []*storage.APIKey // active keys the winner already has
	createdKey *storage.APIKey
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
func (r *raceStub) CreateAPIKey(_ context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	k.ID = "race-key"
	k.Prefix = "race5678"
	r.createdKey = k
	return k, nil
}
func (r *raceStub) ListAPIKeys(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return r.keys, nil
}

func TestEnsure_RaceLosesGracefully(t *testing.T) {
	// Winner already minted its key → loser observes a complete bootstrap.
	r := &raceStub{
		winner: &storage.User{ID: "winner", DisplayName: "admin", Bootstrap: true},
		keys:   []*storage.APIKey{{ID: "winner-key", UserID: "winner"}},
	}
	res, err := bootstrap.Ensure(context.Background(), r, bootstrap.Options{})
	require.NoError(t, err)
	require.False(t, res.Created, "race loser observes the winner's complete bootstrap")
	require.Equal(t, "winner", res.UserID)
	require.Empty(t, res.Token, "loser mints no key when the winner already has one")
	require.Nil(t, r.createdKey)
}

// TestEnsure_RaceRecoversKeylessWinner covers the worst-case race: the loser
// re-reads the winner's user, but the winner died between CreateHuman and
// CreateAPIKey, so the winner is keyless. The loser must recover it in-call
// rather than no-op'ing and deferring recovery to a hypothetical next Ensure.
func TestEnsure_RaceRecoversKeylessWinner(t *testing.T) {
	r := &raceStub{
		winner: &storage.User{ID: "winner", DisplayName: "admin", Bootstrap: true},
		keys:   nil, // winner crashed before minting → keyless
	}
	res, err := bootstrap.Ensure(context.Background(), r, bootstrap.Options{})
	require.NoError(t, err)
	require.True(t, res.Created, "the race loser must recover a keyless winner")
	require.NotEmpty(t, res.Token)
	require.Equal(t, "winner", res.UserID)
	require.NotNil(t, r.createdKey, "a recovery key was minted for the winner")
	require.Equal(t, "winner", r.createdKey.UserID)
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
func (e *errStub) ListAPIKeys(context.Context, storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	panic("ListAPIKeys must not be called when GetBootstrap errors")
}

func TestEnsure_GetBootstrapErrorSurfaced(t *testing.T) {
	sentinel := errors.New("db unreachable")
	_, err := bootstrap.Ensure(context.Background(), &errStub{getErr: sentinel}, bootstrap.Options{})
	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "non-ErrUserNotFound GetBootstrap error must be surfaced")
}
