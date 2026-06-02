// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
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

// TestResolve_APIKeyShapeRoutesToKeyPath verifies that a well-formed
// spgr_sk_-prefixed token actually reaches the UsersBackend (LookupAPIKeyByPrefix),
// proving the API-key dispatch path is taken rather than the JWT path.
// Previously this was tautological (the stub returned ErrUnauthenticated for
// everything); now we observe routing by requiring lookupAPIKey to be called.
func TestResolve_APIKeyShapeRoutesToKeyPath(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			called = true
			require.Equal(t, "abc12345", prefix)
			return nil, storage.ErrAPIKeyNotFound
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	_, resolveErr := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, resolveErr, auth.ErrUnauthenticated)
	require.True(t, called, "LookupAPIKeyByPrefix must be called for a well-formed API-key token")
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

// --- Task 11: parse spgr_sk_<prefix>_<secret> ---

func TestResolveAPIKey_MalformedTokens(t *testing.T) {
	store := newTestIdentityStore(t)
	bad := []string{
		"not-a-key",
		"spgr_sk_",               // missing parts
		"spgr_sk_short_secret",   // prefix too short
		"spgr_sk_abc12345_short", // secret too short
		"spgr_pk_abc12345_thirtytwocharsecretthirtytwocha", // wrong vendor prefix
	}
	for _, tok := range bad {
		_, err := store.Resolve(context.Background(), tok)
		require.ErrorIs(t, err, auth.ErrUnauthenticated, "token %q should be Unauthenticated", tok)
	}
}

func TestResolveAPIKey_WellFormedReachesLookup(t *testing.T) {
	called := false
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			called = true
			require.Equal(t, "abc12345", prefix)
			return nil, storage.ErrAPIKeyNotFound
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
	require.True(t, called, "lookupAPIKey should have been invoked once parse logic is wired")
}

// --- Task 12: argon2id verify against PHCHash ---

func TestResolveAPIKey_WrongSecretUnauthenticated(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix,
				PHCHash: stubPHCHash,
			}, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	// Well-formed token, correct prefix, but a DIFFERENT secret of the
	// SAME length — derive it from stubPHCSecret by flipping the first
	// char so the parse succeeds (length matches) but argon2id verify fails.
	wrongSecret := "X" + stubPHCSecret[1:]
	_, err = store.Resolve(context.Background(), "spgr_sk_abc12345_"+wrongSecret)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveAPIKey_MatchingSecretSucceeds(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				CreatedAt: time.Now(),
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	id, err := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "apikey:k1", id.Subject)
}

// --- Task 13: owner load + soft-delete check ---

func TestResolveAPIKey_SoftDeletedOwnerUnauthenticated(t *testing.T) {
	deletedAt := time.Now().Add(-time.Hour)
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u-del", Prefix: prefix, PHCHash: stubPHCHash,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "reader", storage.KindHuman)
			u.DeletedAt = &deletedAt
			return u, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// --- Task 14: EffectiveRole = min(user.Role, key.RoleDowngrade) ---

func TestResolveAPIKey_RoleDowngradeClamps(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				RoleDowngrade: "reader",
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "writer", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	id, err := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, auth.Role("reader"), id.EffectiveRole)
}

func TestResolveAPIKey_NoDowngradeEqualsRole(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				// RoleDowngrade: "" (zero value)
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "writer", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	id, err := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, id.Role, id.EffectiveRole)
}

// --- Task 15: fire-and-forget TouchLastUsed ---

// countingTracker records every Touch call.
type countingTracker struct {
	touched []string
}

func (c *countingTracker) Touch(keyID string) {
	c.touched = append(c.touched, keyID)
}

func TestResolveAPIKey_SuccessTouchesLastUsed(t *testing.T) {
	tracker := &countingTracker{}
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: tracker,
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, []string{"k1"}, tracker.touched)
}

func TestResolveAPIKey_FailureDoesNotTouch(t *testing.T) {
	tracker := &countingTracker{}
	stub := &usersBackendStub{
		// No lookupAPIKey set → returns ErrAPIKeyNotFound.
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: tracker,
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
	require.Empty(t, tracker.touched)
}

// --- Security invariants: revoked / expired keys, no escalation ---

func TestResolveAPIKey_RevokedKeyUnauthenticated(t *testing.T) {
	revokedAt := time.Now().Add(-time.Hour)
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			// Matching PHCHash so the secret verifies; the revoke gate must
			// still reject before the owner load.
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				RevokedAt: &revokedAt,
			}, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveAPIKey_ExpiredKeyUnauthenticated(t *testing.T) {
	// Fixed clock so the expiry boundary is deterministic.
	fixedNow := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	expiredAt := fixedNow.Add(-time.Hour)
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				ExpiresAt: &expiredAt,
			}, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
		Now: func() time.Time { return fixedNow },
	})
	require.NoError(t, err)
	_, err = store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveAPIKey_DowngradeAboveRoleNoEscalation(t *testing.T) {
	stub := &usersBackendStub{
		lookupAPIKey: func(_ context.Context, prefix string) (*storage.APIKey, error) {
			return &storage.APIKey{
				ID: "k1", UserID: "u1", Prefix: prefix, PHCHash: stubPHCHash,
				RoleDowngrade: "admin", // HIGHER than the owner's reader role
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			return activeUser(id, "reader", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	id, err := store.Resolve(context.Background(), stubAPIKeyToken("abc12345"))
	require.NoError(t, err)
	require.Equal(t, auth.Role("reader"), id.Role)
	require.Equal(t, auth.Role("reader"), id.EffectiveRole, "downgrade above role must not escalate")
}
