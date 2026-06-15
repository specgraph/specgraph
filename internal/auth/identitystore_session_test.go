// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// compile-time assertion: fakeWebAuth must implement storage.WebAuthStore.
var _ storage.WebAuthStore = (*fakeWebAuth)(nil)

// fakeWebAuth is a hand-rolled stub for storage.WebAuthStore used by the
// session-resolution tests. Only LookupSessionByHash carries behavior; the
// other methods are unused by the Resolver and return zero values.
type fakeWebAuth struct {
	lookupSessionByHash func(ctx context.Context, tokenHash []byte) (*storage.Session, error)
}

func (f *fakeWebAuth) LookupSessionByHash(ctx context.Context, tokenHash []byte) (*storage.Session, error) {
	if f.lookupSessionByHash == nil {
		return nil, storage.ErrSessionNotFound
	}
	return f.lookupSessionByHash(ctx, tokenHash)
}

func (f *fakeWebAuth) CreateSession(_ context.Context, _ *storage.Session) (*storage.Session, error) {
	return nil, nil
}
func (f *fakeWebAuth) RevokeSession(_ context.Context, _ []byte) error { return nil }
func (f *fakeWebAuth) DeleteExpiredSessions(_ context.Context) (int64, error) {
	return 0, nil
}
func (f *fakeWebAuth) CreateLoginFlow(_ context.Context, _ *storage.LoginFlow) (string, error) {
	return "", nil
}
func (f *fakeWebAuth) ConsumeLoginFlow(_ context.Context, _ string) (*storage.LoginFlow, error) {
	return nil, nil
}
func (f *fakeWebAuth) DeleteExpiredLoginFlows(_ context.Context) (int64, error) {
	return 0, nil
}

// --- Task 9: opaque web-session resolution (spgr_ws_...) ---

// TestResolveSession resolves a seeded active session whose UserID maps to an
// active user, asserting the returned Identity mirrors the OIDC shape.
func TestResolveSession(t *testing.T) {
	const token = "spgr_ws_TOKEN" //nolint:gosec // G101: test token literal, not a real credential
	wantHash := sha256.Sum256([]byte(token))

	web := &fakeWebAuth{
		lookupSessionByHash: func(_ context.Context, tokenHash []byte) (*storage.Session, error) {
			require.True(t, bytes.Equal(wantHash[:], tokenHash), "lookup must use sha256(token)")
			return &storage.Session{
				ID:          "s1",
				TokenHash:   tokenHash,
				UserID:      "u1",
				Issuer:      "https://issuer.example/",
				OIDCSubject: "subject-123",
				ExpiresAt:   time.Now().Add(time.Hour),
			}, nil
		},
	}
	stub := &usersBackendStub{
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			require.Equal(t, "u1", id)
			u := activeUser(id, "writer", storage.KindHuman)
			u.Email = "alice@example.com"
			return u, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, WebAuth: web, Tracker: &noopTracker{},
	})
	require.NoError(t, err)

	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "oidc:subject-123", id.Subject)
	require.Equal(t, "oidc", id.Source)
	require.Equal(t, auth.Role("writer"), id.Role)
	require.Equal(t, auth.Role("writer"), id.EffectiveRole)
	require.Equal(t, "alice@example.com", id.Email)
}

// TestResolveSession_Errors covers the error discipline of resolveSession.
func TestResolveSession_Errors(t *testing.T) {
	const token = "spgr_ws_TOKEN" //nolint:gosec // G101: test token literal, not a real credential

	t.Run("missing session → Unauthenticated", func(t *testing.T) {
		web := &fakeWebAuth{
			lookupSessionByHash: func(_ context.Context, _ []byte) (*storage.Session, error) {
				return nil, storage.ErrSessionNotFound
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: &usersBackendStub{}, WebAuth: web, Tracker: &noopTracker{},
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrUnauthenticated)
	})

	t.Run("backend error → Transient", func(t *testing.T) {
		web := &fakeWebAuth{
			lookupSessionByHash: func(_ context.Context, _ []byte) (*storage.Session, error) {
				return nil, errors.New("connection refused")
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: &usersBackendStub{}, WebAuth: web, Tracker: &noopTracker{},
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrTransient)
	})

	t.Run("WebAuth nil → Unauthenticated (no panic)", func(t *testing.T) {
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: &usersBackendStub{}, Tracker: &noopTracker{},
			// WebAuth left nil.
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrUnauthenticated)
	})

	t.Run("inactive (expired) session → Unauthenticated", func(t *testing.T) {
		web := &fakeWebAuth{
			lookupSessionByHash: func(_ context.Context, _ []byte) (*storage.Session, error) {
				return &storage.Session{
					ID: "s1", UserID: "u1", OIDCSubject: "sub",
					ExpiresAt: time.Now().Add(-time.Hour), // expired
				}, nil
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: &usersBackendStub{}, WebAuth: web, Tracker: &noopTracker{},
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrUnauthenticated)
	})

	t.Run("revoked session → Unauthenticated", func(t *testing.T) {
		revoked := time.Now().Add(-time.Minute)
		web := &fakeWebAuth{
			lookupSessionByHash: func(_ context.Context, _ []byte) (*storage.Session, error) {
				return &storage.Session{
					ID: "s1", UserID: "u1", OIDCSubject: "sub",
					ExpiresAt: time.Now().Add(time.Hour), // not expired
					RevokedAt: &revoked,                  // but revoked
				}, nil
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: &usersBackendStub{}, WebAuth: web, Tracker: &noopTracker{},
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrUnauthenticated)
	})

	t.Run("soft-deleted user → Unauthenticated", func(t *testing.T) {
		web := &fakeWebAuth{
			lookupSessionByHash: func(_ context.Context, _ []byte) (*storage.Session, error) {
				return &storage.Session{
					ID: "s1", UserID: "u1", OIDCSubject: "sub",
					ExpiresAt: time.Now().Add(time.Hour),
				}, nil
			},
		}
		deleted := time.Now().Add(-time.Hour)
		stub := &usersBackendStub{
			getUserByID: func(_ context.Context, id string) (*storage.User, error) {
				u := activeUser(id, "reader", storage.KindHuman)
				u.DeletedAt = &deleted
				return u, nil
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: stub, WebAuth: web, Tracker: &noopTracker{},
		})
		require.NoError(t, err)
		_, err = store.Resolve(context.Background(), token)
		require.ErrorIs(t, err, auth.ErrUnauthenticated)
	})
}

// --- Task 9 (d): interactive-login bypasses the per-issuer JIT rate limiter ---

// TestJITResolve_InteractiveBypassesLimiter proves that an interactive-login
// context skips the per-issuer JIT rate limiter while a bearer (non-interactive)
// context still consumes it. With JITRateBurstPerHour=1:
//   - a non-interactive JIT after the bucket is drained → ErrUnauthenticated
//   - an interactive JIT after the bucket is drained → still succeeds
//
// The email-domain allowlist still applies to interactive logins, so an
// out-of-domain interactive token is rejected (only the rate counter is
// bypassed, not the allowlist).
func TestJITResolve_InteractiveBypassesLimiter(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:              true,
		JITDefaultRole:          auth.RoleReader,
		JITRateBurstPerHour:     1, // single token per issuer
		JITEmailDomainAllowlist: []string{"example.com"},
	})
	require.NoError(t, err)

	mint := func(sub, email string) string {
		return p.mintToken(t, map[string]any{
			"iss": p.server.URL, "sub": sub, "aud": "aud-1",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": email,
		})
	}

	// Drain the issuer's single rate-limit token via a non-interactive JIT.
	_, err = store.Resolve(context.Background(), mint("drain", "drain@example.com"))
	require.NoError(t, err, "first non-interactive JIT consumes the only token")

	// (i) Non-interactive JIT with the bucket exhausted → rejected.
	_, err = store.Resolve(context.Background(), mint("bearer", "bearer@example.com"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated,
		"non-interactive JIT must be rate-limited once the bucket is drained")

	// (ii) Interactive JIT with the bucket exhausted → still succeeds (bypass).
	ictx := auth.WithInteractiveLogin(context.Background())
	_, err = store.Resolve(ictx, mint("inter-1", "inter1@example.com"))
	require.NoError(t, err, "interactive JIT must bypass the exhausted limiter")
	// A second interactive JIT also succeeds — proving no per-token decrement.
	_, err = store.Resolve(ictx, mint("inter-2", "inter2@example.com"))
	require.NoError(t, err, "interactive JIT must not decrement the limiter")

	// The allowlist still gates interactive logins — out-of-domain rejected.
	_, err = store.Resolve(ictx, mint("inter-bad", "mallory@evil.com"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated,
		"interactive login must still honor the email-domain allowlist")
}
