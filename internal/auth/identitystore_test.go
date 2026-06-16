// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"encoding/base64"
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

// noopTracker implements auth.LastUsedTracker as a no-op stub. Tests use it
// for isolation so they never spin up the real async usagetracker.Manager.
type noopTracker struct{}

func (noopTracker) Touch(string) {}

func TestResolve_EmptyTokenUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t)
	_, err := store.Resolve(context.Background(), "")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// TestResolve_JWTShapeRoutesToOIDC verifies that a JWT-shaped token (three
// dot-separated segments) is routed to the OIDC path rather than the API-key
// path. Routing is observed by confirming that LookupOIDCBinding is called
// (not LookupAPIKeyByPrefix) when the token carries a known issuer and passes
// signature verification.
func TestResolve_JWTShapeRoutesToOIDC(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	oidcCalled := false
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, _ string) (*storage.OIDCBinding, error) {
			oidcCalled = true
			require.Equal(t, p.server.URL, issuer, "peek issuer must match verifier issuer")
			return nil, storage.ErrOIDCBindingNotFound
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
	})
	require.NoError(t, err)

	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "route-probe", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, resolveErr := store.Resolve(context.Background(), token)
	// No binding → ErrUnauthenticated (JIT path not yet enabled)
	require.ErrorIs(t, resolveErr, auth.ErrUnauthenticated)
	require.True(t, oidcCalled, "LookupOIDCBinding must be called: proves JWT was routed to OIDC path, not API-key path")
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

// --- Task 16: JWT issuer peek + verifier routing ---

func TestResolveJWT_UnknownIssuerUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t) // no verifiers configured
	// JWT-shaped token (exactly 2 dots) whose middle segment is valid
	// base64url-encoded JSON carrying an iss claim. peekIssuer succeeds and
	// extracts the issuer, but no verifier is configured for it, so the
	// verifier-map lookup misses → ErrUnauthenticated.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"iss":"https://unknown.example/","sub":"u"}`))
	token := header + "." + payload + ".sig"
	_, err := store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveJWT_UndecodablePayloadUnauthenticated(t *testing.T) {
	store := newTestIdentityStore(t)
	// JWT-shaped token (exactly 2 dots) whose middle segment is NOT valid
	// base64url, so peekIssuer's base64-decode branch fails →
	// ErrUnauthenticated. Exercises the decode-error path via the real JWT route.
	_, err := store.Resolve(context.Background(), "eyJhbGciOiJSUzI1NiJ9.!!!.sig")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolve_FourSegmentTokenRoutesToAPIKeyPath(t *testing.T) {
	store := newTestIdentityStore(t)
	// Four segments (three dots) is NOT JWT-shaped (isJWTShaped requires
	// exactly two dots), so it falls through to the API-key resolver, which
	// rejects it as malformed → ErrUnauthenticated. This never reaches
	// peekIssuer or the OIDC path.
	_, err := store.Resolve(context.Background(), "not.a.valid.jwt")
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// TestResolveJWT_KnownIssuerRoutes verifies that a valid JWT from a configured
// issuer reaches the OIDCVerifier (issuer peek + verify both executed) and then
// proceeds to LookupOIDCBinding. No existing binding → returns ErrUnauthenticated
// because JIT is not yet enabled (Task 18).
func TestResolveJWT_KnownIssuerRoutes(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	bindingLookupCalled := false
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
			bindingLookupCalled = true
			require.Equal(t, p.server.URL, issuer)
			require.Equal(t, "user-123", subject)
			return nil, storage.ErrOIDCBindingNotFound // forces stub JIT path (Task 18)
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:     stub,
		Verifiers: []*auth.OIDCVerifier{v},
		Tracker:   &noopTracker{},
		// JITEnabled: false (Task 18 enables and tests JIT)
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss":   p.server.URL,
		"sub":   "user-123",
		"aud":   "aud-1",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": "alice@example.com",
	})
	_, resolveErr := store.Resolve(context.Background(), token)
	require.ErrorIs(t, resolveErr, auth.ErrUnauthenticated) // JIT disabled → reject on binding miss
	require.True(t, bindingLookupCalled, "LookupOIDCBinding must be reached: confirms issuer peek, verifier routing, and token verification all succeeded")
}

// --- Task 17: JWT existing binding resolves to owner ---

func TestResolveJWT_ExistingBindingResolves(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{
				ID: "b1", UserID: "u1", Issuer: issuer, Subject: subject,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "writer", storage.KindHuman)
			u.Email = "alice@example.com"
			return u, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "user-123", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "alice@example.com",
		// Claims that would map to "admin" if evaluated — but claims-mapping
		// is JIT-only, so the DB role (writer) must win.
		"groups": []string{"specgraph-admins"},
	})
	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, "oidc:user-123", id.Subject)
	require.Equal(t, auth.Role("writer"), id.Role) // NOT admin from claims
	require.Equal(t, auth.Role("writer"), id.EffectiveRole)
	require.Equal(t, "oidc", id.Source)
}

// TestResolveJWT_ExistingBinding_IgnoresClaimsMapping is the real
// security-invariant test for "ClaimsMapping fires ONLY at JIT creation."
// Unlike TestResolveJWT_ExistingBindingResolves (which configures no mapping,
// making "DB role wins" trivially true), this test configures a live
// JITClaimsMapping for the token's issuer mapping groups:["specgraph-admins"]
// → admin, AND an existing binding whose owner has DB role "writer". If the
// re-login (existing-binding) path ever applied claims-mapping, id.Role would
// become "admin" — a privilege-escalation regression. Asserting "writer"
// proves the mapping was NOT applied on the binding path.
func TestResolveJWT_ExistingBinding_IgnoresClaimsMapping(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)

	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{
				ID: "b1", UserID: "u1", Issuer: issuer, Subject: subject,
			}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "writer", storage.KindHuman) // DB role: writer
			u.Email = "alice@example.com"
			return u, nil
		},
		// jitCreateHuman intentionally unset: the binding path must never call
		// it. usersBackendStub flags an unexpected JITCreateHuman call as a bug.
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
		// Live mapping that WOULD elevate to admin if evaluated on this path.
		JITClaimsMapping: map[string][]config.ClaimMapping{
			p.server.URL: {
				{Claim: "groups", Value: "specgraph-admins", Role: "admin"},
			},
		},
		KnownRoles: map[auth.Role]bool{auth.RoleReader: true, auth.RoleWriter: true, auth.RoleAdmin: true},
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "user-123", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email":  "alice@example.com",
		"groups": []string{"specgraph-admins"}, // maps to admin IF evaluated
	})
	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "u1", id.UserID)
	require.Equal(t, auth.Role("writer"), id.Role, "claims-mapping must NOT apply on the existing-binding path")
	require.Equal(t, auth.Role("writer"), id.EffectiveRole, "claims-mapping must NOT apply on the existing-binding path")
}

// --- Task 18: JIT happy path ---

func TestResolveJWT_JITCreatesNewUser(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	var capturedUser *storage.User
	var capturedBinding *storage.OIDCBinding
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
		jitCreateHuman: func(_ context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
			u.ID = "new-user"
			b.ID = "new-binding"
			b.UserID = u.ID
			capturedUser, capturedBinding = u, b
			return u, b, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled:     true,
		JITDefaultRole: auth.RoleReader,
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "new-sub", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": "new@example.com",
	})
	id, err := store.Resolve(context.Background(), token)
	require.NoError(t, err)
	require.Equal(t, "new-user", id.UserID)
	require.Equal(t, auth.Role("reader"), id.Role)
	require.NotNil(t, capturedUser)
	require.Equal(t, "new@example.com", capturedUser.Email)
	require.NotNil(t, capturedBinding)
	require.Equal(t, "new-sub", capturedBinding.Subject)
}

func TestResolveJWT_JITDisabledRejects(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return nil, storage.ErrOIDCBindingNotFound
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
		JITEnabled: false,
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "x", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

func TestResolveJWT_SoftDeletedUserUnauthenticated(t *testing.T) {
	p := newOIDCTestIssuer(t)
	v, err := auth.NewOIDCVerifier(context.Background(), config.OIDCProviderConfig{
		ID: "test", Issuer: p.server.URL, ClientID: "aud-1",
	})
	require.NoError(t, err)
	deletedAt := time.Now()
	stub := &usersBackendStub{
		lookupOIDCBinding: func(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
			return &storage.OIDCBinding{ID: "b1", UserID: "u-del"}, nil
		},
		getUserByID: func(_ context.Context, id string) (*storage.User, error) {
			u := activeUser(id, "writer", storage.KindHuman)
			u.DeletedAt = &deletedAt
			return u, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Verifiers: []*auth.OIDCVerifier{v}, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	token := p.mintToken(t, map[string]any{
		"iss": p.server.URL, "sub": "u", "aud": "aud-1",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
	})
	_, err = store.Resolve(context.Background(), token)
	require.ErrorIs(t, err, auth.ErrUnauthenticated)
}

// --- Task 22: HasAuth ---

func TestHasAuth_OnlyBootstrapReturnsFalse(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
			require.Equal(t, storage.KindHuman, f.Kind)
			require.False(t, f.IncludeDeleted)
			// Return ONLY the bootstrap user.
			u := activeUser("u-boot", "admin", storage.KindHuman)
			u.Bootstrap = true
			return []*storage.User{u}, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	has, err := store.HasAuth(context.Background())
	require.NoError(t, err)
	require.False(t, has)
}

func TestHasAuth_NonBootstrapUserReturnsTrue(t *testing.T) {
	stub := &usersBackendStub{
		listUsers: func(_ context.Context, _ storage.ListUsersFilter) ([]*storage.User, error) {
			return []*storage.User{
				activeUser("u1", "reader", storage.KindHuman),
			}, nil
		},
	}
	store, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users: stub, Tracker: &noopTracker{},
	})
	require.NoError(t, err)
	has, err := store.HasAuth(context.Background())
	require.NoError(t, err)
	require.True(t, has)
}

// --- Identity context helpers ---

func TestIdentityFromContext(t *testing.T) {
	id := &auth.Identity{
		Subject:     "test:user",
		DisplayName: "Test User",
		Role:        auth.RoleAdmin,
		Source:      "test",
	}

	ctx := auth.WithIdentity(context.Background(), id)
	got, ok := auth.IdentityFromContext(ctx)
	if !ok {
		t.Fatal("IdentityFromContext returned false")
	}
	if got.Subject != "test:user" {
		t.Errorf("subject = %q, want test:user", got.Subject)
	}
}

func TestIdentityFromContext_Missing(t *testing.T) {
	_, ok := auth.IdentityFromContext(context.Background())
	if ok {
		t.Error("IdentityFromContext returned true for empty context")
	}
}

func TestWithIdentity_Nil(t *testing.T) {
	ctx := auth.WithIdentity(context.Background(), nil)
	_, ok := auth.IdentityFromContext(ctx)
	if ok {
		t.Error("IdentityFromContext returned true after WithIdentity(nil)")
	}
}

func TestNewIdentityStore_ValidatesMappingRoles_WhenLoginSyncOnAndJITOff(t *testing.T) {
	_, err := auth.NewIdentityStore(auth.IdentityStoreConfig{
		Users:            &usersBackendStub{},
		Tracker:          &noopTracker{},
		JITEnabled:       false, // JIT off…
		LoginSyncEnabled: true,  // …but login-sync on
		KnownRoles:       map[auth.Role]bool{auth.RoleReader: true, auth.RoleWriter: true, auth.RoleAdmin: true},
		JITClaimsMapping: map[string][]config.ClaimMapping{
			"https://issuer": {{Claim: "roles", Value: "x", Role: "admln"}}, // typo
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown role")
}
