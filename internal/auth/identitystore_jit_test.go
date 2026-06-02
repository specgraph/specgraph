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
