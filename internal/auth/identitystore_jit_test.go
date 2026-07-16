// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Task 19: per-issuer JIT rate limit ---

// TestResolveLogin_ThreadsIssuerOnBindingHit proves the claims-based
// interactive-login entrypoint (ResolveLogin → materializeIdentity) returns an
// Identity carrying Issuer = claims.Issuer for an existing binding (AUTH-05 /
// D-09), and that it never touches the JIT path when a binding exists.
func TestResolveLogin_ThreadsIssuerOnBindingHit(t *testing.T) {
	const issuer = "https://idp.example.com"
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, iss, sub string) (*storage.OIDCBinding, error) {
			require.Equal(t, issuer, iss)
			require.Equal(t, "sub-1", sub)
			return &storage.OIDCBinding{ID: "b1", UserID: "u1", Issuer: iss, Subject: sub}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			require.Equal(t, "u1", id)
			return activeUser("u1", "reader", storage.KindHuman), nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)

	id, err := store.ResolveLogin(context.Background(), &auth.OIDCClaims{Issuer: issuer, Subject: "sub-1"})
	require.NoError(t, err)
	require.Equal(t, issuer, id.Issuer, "ResolveLogin must thread the verified issuer onto the Identity")
	require.Equal(t, "oidc:sub-1", id.Subject)
	require.Equal(t, "u1", id.UserID)
}

// TestResolveLogin_ThreadsIssuerOnJIT proves the JIT branch of
// materializeIdentity (binding miss, interactive) also stamps Issuer, and that
// interactive JIT bypasses the rate limiter (burst would otherwise be spent).
func TestResolveLogin_ThreadsIssuerOnJIT(t *testing.T) {
	const issuer = "https://idp.example.com"
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "u-new"
			b.ID = "b-new"
			b.UserID = "u-new"
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
		JITEnabled: true, JITDefaultRole: auth.RoleReader,
	})
	require.NoError(t, err)

	id, err := store.ResolveLogin(context.Background(), &auth.OIDCClaims{Issuer: issuer, Subject: "sub-new", Email: "a@x.com"})
	require.NoError(t, err)
	require.Equal(t, issuer, id.Issuer, "JIT-created Identity must carry the issuer")
	require.Equal(t, "u-new", id.UserID)
}

func TestJIT_RateLimitExhaustion(t *testing.T) {
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
		JITEnabled:          true,
		JITDefaultRole:      auth.RoleReader,
		JITRateBurstPerHour: 5, // small burst so the test runs fast
	})
	require.NoError(t, err)

	// First 5 JITs succeed (bucket capacity = 5).
	for i := 0; i < 5; i++ {
		tok := p.mintToken(t, map[string]any{
			"iss": p.server.URL, "sub": "u" + strconv.Itoa(i),
			"aud": "aud-1", "exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		})
		_, err := store.Resolve(context.Background(), tok)
		require.NoError(t, err, "JIT %d should succeed", i)
	}
	// 6th JIT exhausts the bucket.
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u-exhaust",
		"aud": "aud-1", "exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	})
	var resolveErr error
	_, resolveErr = store.Resolve(context.Background(), tok)
	require.ErrorIs(t, resolveErr, auth.ErrUnauthenticated)
}

// TestJIT_RateLimitIsolation_TwoIssuers proves rate-limit buckets are
// per-issuer, not global. Two issuers (A, B) each have burst=1. Exhausting
// issuer A (one JIT succeeds, the second → ErrUnauthenticated) must NOT
// consume issuer B's budget — B's first JIT still succeeds.
func TestJIT_RateLimitIsolation_TwoIssuers(t *testing.T) {
	pA := newOIDCTestIssuer(t)
	pB := newOIDCTestIssuer(t)
	vA, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "issuer-a", Issuer: pA.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	vB, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "issuer-b", Issuer: pB.server.URL, ClientID: "aud-1",
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
		Users: stub, Verifiers: []*auth.OIDCVerifier{vA, vB}, Tracker: &noopTracker{},
		JITEnabled:          true,
		JITDefaultRole:      auth.RoleReader,
		JITRateBurstPerHour: 1, // burst of 1 per issuer
	})
	require.NoError(t, err)

	mint := func(p *oidcTestIssuer, sub string) string {
		return p.mintToken(t, map[string]any{
			"iss": p.server.URL, "sub": sub, "aud": "aud-1",
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		})
	}

	// Issuer A: first JIT consumes A's single token.
	_, err = store.Resolve(context.Background(), mint(pA, "a-1"))
	require.NoError(t, err, "issuer A first JIT should succeed")
	// Issuer A: second JIT exhausts A's bucket.
	_, err = store.Resolve(context.Background(), mint(pA, "a-2"))
	require.ErrorIs(t, err, auth.ErrUnauthenticated, "issuer A second JIT should be rate-limited")

	// Issuer B: first JIT must still succeed — proves buckets are per-issuer.
	_, err = store.Resolve(context.Background(), mint(pB, "b-1"))
	require.NoError(t, err, "issuer B first JIT must succeed despite A being exhausted")
}

// TestJIT_SeedsDisplayNameFromClaimsName proves jitResolve seeds the new
// user's DisplayName from claims.Name when present (D-07), falling back to
// claims.Subject when the provider supplies no name — eliminating the
// stale-fallback window that 09-01's reconciliation would otherwise need to
// self-heal on a later login.
func TestJIT_SeedsDisplayNameFromClaimsName(t *testing.T) {
	const issuer = "https://idp.example.com"

	t.Run("seeds from claims.Name when present", func(t *testing.T) {
		var captured *storage.User
		stub := &usersBackendStub{
			lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
				return nil, storage.ErrOIDCBindingNotFound
			},
			jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
				captured = u
				u.ID = "u-new"
				b.ID = "b-new"
				b.UserID = "u-new"
				return u, b, nil
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: stub, Tracker: &noopTracker{},
			JITEnabled: true, JITDefaultRole: auth.RoleReader,
		})
		require.NoError(t, err)

		id, err := store.ResolveLogin(context.Background(), &auth.OIDCClaims{
			Issuer: issuer, Subject: "sub-new", Name: "Ada Lovelace",
		})
		require.NoError(t, err)
		require.NotNil(t, captured, "JITCreateHuman must be called")
		require.Equal(t, "Ada Lovelace", captured.DisplayName, "seed must prefer claims.Name over claims.Subject")
		require.Equal(t, "Ada Lovelace", id.DisplayName)
	})

	t.Run("falls back to claims.Subject when Name is empty", func(t *testing.T) {
		var captured *storage.User
		stub := &usersBackendStub{
			lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
				return nil, storage.ErrOIDCBindingNotFound
			},
			jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
				captured = u
				u.ID = "u-new-2"
				b.ID = "b-new-2"
				b.UserID = "u-new-2"
				return u, b, nil
			},
		}
		store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
			Users: stub, Tracker: &noopTracker{},
			JITEnabled: true, JITDefaultRole: auth.RoleReader,
		})
		require.NoError(t, err)

		id, err := store.ResolveLogin(context.Background(), &auth.OIDCClaims{
			Issuer: issuer, Subject: "sub-no-name",
		})
		require.NoError(t, err)
		require.NotNil(t, captured, "JITCreateHuman must be called")
		require.Equal(t, "sub-no-name", captured.DisplayName, "seed must fall back to claims.Subject")
		require.Equal(t, "sub-no-name", id.DisplayName)
	})
}

// --- Task 20: email-domain allowlist ---

func TestJIT_EmailAllowlist_Match(t *testing.T) {
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
		JITEmailDomainAllowlist: []string{"example.com"},
	})
	require.NoError(t, err)
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "alice@example.com",
	})
	_, err = store.Resolve(context.Background(), tok)
	require.NoError(t, err)
}

func TestJIT_EmailAllowlist_Mismatch(t *testing.T) {
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
		JITEmailDomainAllowlist: []string{"example.com"},
	})
	require.NoError(t, err)
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "bob@other.com",
	})
	_, err = store.Resolve(context.Background(), tok)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestJIT_EmailAllowlist_MissingClaimRefuses(t *testing.T) {
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
		JITEmailDomainAllowlist: []string{"example.com"},
	})
	require.NoError(t, err)
	// Token has no "email" claim; allowlist non-empty → refuse.
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = store.Resolve(context.Background(), tok)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestJIT_EmptyAllowlistAllowsMissingClaim(t *testing.T) {
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
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		// JITEmailDomainAllowlist: nil (empty = no allowlist)
	})
	require.NoError(t, err)
	// Token has no "email" claim; empty allowlist → JIT succeeds; user.Email = "".
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	id, err := store.Resolve(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "", id.Email)
}

// --- Task 21: ClaimsMapping evaluated only at JIT creation ---

func TestJIT_ClaimsMapping_AppliesRole(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	var capturedRole string
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			capturedRole = u.Role
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		JITClaimsMapping: map[string][]config.ClaimMapping{
			p.server.URL: {
				{Claim: "groups", Value: "specgraph-admins", Role: "admin"},
			},
		},
	})
	require.NoError(t, err)
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"email":  "a@example.com",
		"groups": []string{"specgraph-admins"},
	})
	_, err = store.Resolve(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "admin", capturedRole, "claims-mapping should override default-role")
}

// TestJIT_ClaimsMapping_NoMatchFallsBackToDefault verifies that when no
// mapping rule fires, the JIT user gets the configured default role.
func TestJIT_ClaimsMapping_NoMatchFallsBackToDefault(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	var capturedRole string
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			capturedRole = u.Role
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleWriter,
		JITClaimsMapping: map[string][]config.ClaimMapping{
			p.server.URL: {
				{Claim: "groups", Value: "specgraph-admins", Role: "admin"},
			},
		},
	})
	require.NoError(t, err)
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"iat":    time.Now().Unix(),
		"email":  "a@example.com",
		"groups": []string{"some-other-group"}, // no match
	})
	_, err = store.Resolve(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "writer", capturedRole, "no claims-mapping match should fall back to default role")
}

// TestJIT_ClaimsMapping_ScalarStringClaim covers matchClaimValue's scalar-string
// branch: a claim carrying a single JSON string (not a []string array) must
// still match a mapping rule. The existing mapping tests only use array claims,
// leaving the string path — common for single-valued claims like "role" — uncovered.
func TestJIT_ClaimsMapping_ScalarStringClaim(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	var capturedRole string
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			capturedRole = u.Role
			u.ID = "u"
			b.ID = "b"
			b.UserID = "u"
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		JITClaimsMapping: map[string][]config.ClaimMapping{
			p.server.URL: {
				{Claim: "role", Value: "platform-admin", Role: "admin"},
			},
		},
	})
	require.NoError(t, err)
	tok := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": "a@example.com",
		"role":  "platform-admin", // scalar string, not an array
	})
	_, err = store.Resolve(context.Background(), tok)
	require.NoError(t, err)
	require.Equal(t, "admin", capturedRole, "a scalar-string claim must match a mapping rule")
}
