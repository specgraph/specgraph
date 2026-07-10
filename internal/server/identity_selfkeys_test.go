// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// testSelfServiceKeysConfig returns the D-08 default self-service key policy for
// tests (90d default / 180d max / quota 10 / 30 per hour, burst 5).
func testSelfServiceKeysConfig() config.SelfServiceKeysConfig {
	return config.SelfServiceKeysConfig{
		DefaultTTLDays:   90,
		MaxTTLDays:       180,
		Quota:            10,
		RateLimitPerHour: 30,
		RateLimitBurst:   5,
	}
}

// newSelfKeysHandler constructs an IdentityHandler directly (same-package
// access) so the self-key handlers can be exercised with an identity injected
// into the context, without standing up the HTTP transport. A nil logger
// discards output.
func newSelfKeysHandler(stub *usersBackendStub, cfg config.SelfServiceKeysConfig, logger *slog.Logger) *IdentityHandler {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &IdentityHandler{users: stub, logger: logger, selfKeys: cfg}
}

// ctxWithIdentity returns a background context carrying id.
func ctxWithIdentity(id *auth.Identity) context.Context {
	return auth.WithIdentity(context.Background(), id)
}

// --- Task 1: CreateMyAPIKey + RotateMyAPIKey ---

// TestCreateMyAPIKey_FloorsAtEffectiveRole proves the owner is taken from
// context and a caller whose EffectiveRole is reader can only mint a
// reader-ceiling key even when it explicitly requests admin (T-02-14).
func TestCreateMyAPIKey_FloorsAtEffectiveRole(t *testing.T) {
	var got *storage.APIKey
	var gotQuota int
	stub := &usersBackendStub{
		createAPIKeyForUser: func(_ context.Context, k *storage.APIKey, quota int) (*storage.APIKey, error) {
			got = k
			gotQuota = quota
			return &storage.APIKey{
				ID: "key-1", UserID: k.UserID, Prefix: "pfx12345",
				RoleDowngrade: k.RoleDowngrade, Label: k.Label, ExpiresAt: k.ExpiresAt,
				CreatedAt: time.Now(),
			}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{
		UserID: "u1", Role: auth.RoleAdmin, EffectiveRole: auth.RoleReader, Source: "oidc",
	})

	resp, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{
		RoleDowngrade: string(auth.RoleAdmin), Label: "laptop",
	}))
	require.NoError(t, err)
	require.Equal(t, "u1", got.UserID, "owner must come from context")
	require.Equal(t, string(auth.RoleReader), got.RoleDowngrade, "admin request floored at reader effective role")
	require.Equal(t, 10, gotQuota, "quota threaded from config")
	require.NotNil(t, got.ExpiresAt, "self-minted key must never live forever")
	require.NotEmpty(t, resp.Msg.GetPlaintext())
	require.True(t, strings.HasPrefix(resp.Msg.GetPlaintext(), auth.APIKeyTokenPrefix()))
}

// TestCreateMyAPIKey_EmptyDowngradeResolvesToEffectiveRole proves an omitted
// role_downgrade resolves to the caller's effective role before the floor.
func TestCreateMyAPIKey_EmptyDowngradeResolvesToEffectiveRole(t *testing.T) {
	var got *storage.APIKey
	stub := &usersBackendStub{
		createAPIKeyForUser: func(_ context.Context, k *storage.APIKey, _ int) (*storage.APIKey, error) {
			got = k
			return &storage.APIKey{ID: "key-2", UserID: k.UserID, Prefix: "pfx", RoleDowngrade: k.RoleDowngrade, CreatedAt: time.Now()}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleWriter, EffectiveRole: auth.RoleWriter, Source: "oidc"})

	_, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.NoError(t, err)
	require.Equal(t, string(auth.RoleWriter), got.RoleDowngrade)
}

// TestCreateMyAPIKey_NoIdentityUnauthenticated asserts a missing identity is
// rejected before any storage call.
func TestCreateMyAPIKey_NoIdentityUnauthenticated(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	_, err := h.CreateMyAPIKey(context.Background(), connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestSelfMint_RejectsApikeySource asserts an api-key caller cannot self-mint or
// self-rotate (anti key-chaining, T-02-15). Storage stubs fail loud, so the
// PermissionDenied must be returned before any storage call.
func TestSelfMint_RejectsApikeySource(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{
		UserID: "u1", Role: auth.RoleAdmin, EffectiveRole: auth.RoleAdmin, Source: "apikey",
	})

	_, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "create rejects apikey source")

	_, err = h.RotateMyAPIKey(ctx, connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "k1"}))
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "rotate rejects apikey source")
}

// TestSelfMint_ExpiryCap asserts an expiry beyond MaxTTLDays is rejected with
// CodeInvalidArgument before any storage call (T-02-18).
func TestSelfMint_ExpiryCap(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	over := timestamppb.New(time.Now().Add(200 * 24 * time.Hour)) // > 180d cap
	_, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{ExpiresAt: over}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestSelfMint_RateLimited asserts the per-identity limiter returns
// CodeResourceExhausted once the burst is spent (T-02-17).
func TestSelfMint_RateLimited(t *testing.T) {
	stub := &usersBackendStub{
		createAPIKeyForUser: func(_ context.Context, k *storage.APIKey, _ int) (*storage.APIKey, error) {
			return &storage.APIKey{ID: "key-1", UserID: k.UserID, Prefix: "pfx", RoleDowngrade: k.RoleDowngrade, CreatedAt: time.Now()}, nil
		},
	}
	cfg := testSelfServiceKeysConfig()
	cfg.RateLimitPerHour = 1 // refill 1/hour → will not refill within the test
	cfg.RateLimitBurst = 1
	h := newSelfKeysHandler(stub, cfg, nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	_, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.NoError(t, err, "first mint consumes the single burst token")

	_, err = h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err), "second mint is rate limited")
}

// TestSelfMint_AuditLogged asserts a successful create and rotate each emit a
// structured audit line carrying the actor/key_id/action, and that neither the
// plaintext secret nor the raw user-supplied label appears in the log output
// (log-injection / PII guard, T-02-33).
func TestSelfMint_AuditLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	const evilLabel = "evilinjectedlabel"
	stub := &usersBackendStub{
		createAPIKeyForUser: func(_ context.Context, k *storage.APIKey, _ int) (*storage.APIKey, error) {
			return &storage.APIKey{ID: "key-create", UserID: k.UserID, Prefix: "pfx", RoleDowngrade: k.RoleDowngrade, Label: k.Label, CreatedAt: time.Now()}, nil
		},
		getAPIKeyForUser: func(_ context.Context, _, _ string) (*storage.APIKey, error) {
			return &storage.APIKey{ID: "old", UserID: "u1", RoleDowngrade: string(auth.RoleReader)}, nil
		},
		rotateAPIKeyForUser: func(_ context.Context, _, _ string, k *storage.APIKey) (*storage.APIKey, error) {
			return &storage.APIKey{ID: "key-rotate", UserID: "u1", Prefix: "pfx", RoleDowngrade: k.RoleDowngrade, CreatedAt: time.Now()}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), logger)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	cResp, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{Label: evilLabel}))
	require.NoError(t, err)
	rResp, err := h.RotateMyAPIKey(ctx, connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "old"}))
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "apikey.self.create")
	require.Contains(t, out, "apikey.self.rotate")
	require.Contains(t, out, "actor=u1")
	require.Contains(t, out, "key_id=key-create")
	require.Contains(t, out, "key_id=key-rotate")
	require.Contains(t, out, "action=create")
	require.Contains(t, out, "action=rotate")

	// The plaintext secret must never be logged.
	require.NotContains(t, out, cResp.Msg.GetPlaintext(), "plaintext secret must not appear in audit log")
	require.NotContains(t, out, rResp.Msg.GetPlaintext(), "plaintext secret must not appear in audit log")
	// The raw user-supplied label must never be logged (log-injection / PII).
	require.NotContains(t, out, evilLabel, "raw label must not appear in audit log")
}

// TestRotateMyAPIKey_FloorsAtEffectiveRole proves rotate re-floors the role at
// the caller's live effective role and never re-pins the old key's stale
// higher ceiling (T-02-14). A reader-effective caller rotating a key that
// carries an admin downgrade yields a reader-ceiling key.
func TestRotateMyAPIKey_FloorsAtEffectiveRole(t *testing.T) {
	var newKey *storage.APIKey
	stub := &usersBackendStub{
		getAPIKeyForUser: func(_ context.Context, userID, keyID string) (*storage.APIKey, error) {
			require.Equal(t, "u1", userID)
			require.Equal(t, "old-key", keyID)
			return &storage.APIKey{ID: "old-key", UserID: "u1", RoleDowngrade: string(auth.RoleAdmin)}, nil
		},
		rotateAPIKeyForUser: func(_ context.Context, userID, _ string, k *storage.APIKey) (*storage.APIKey, error) {
			require.Equal(t, "u1", userID)
			newKey = k
			return &storage.APIKey{ID: "new-key", UserID: "u1", Prefix: "pfx", RoleDowngrade: k.RoleDowngrade, ExpiresAt: k.ExpiresAt, CreatedAt: time.Now()}, nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleAdmin, EffectiveRole: auth.RoleReader, Source: "oidc"})

	resp, err := h.RotateMyAPIKey(ctx, connect.NewRequest(&specv1.RotateMyAPIKeyRequest{KeyId: "old-key"}))
	require.NoError(t, err)
	require.Equal(t, string(auth.RoleReader), newKey.RoleDowngrade, "stale admin ceiling re-floored to reader")
	require.NotNil(t, newKey.ExpiresAt, "ttl-less rotate defaults to a bounded expiry")
	require.NotEmpty(t, resp.Msg.GetPlaintext())
}

// TestRotateMyAPIKey_RequiresKeyID asserts an empty key_id is rejected.
func TestRotateMyAPIKey_RequiresKeyID(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})
	_, err := h.RotateMyAPIKey(ctx, connect.NewRequest(&specv1.RotateMyAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- Task 2: ListMyAPIKeys + RevokeMyAPIKey ---

// TestListMyAPIKeys_ScopedToCaller seeds keys for two users in the fake and
// asserts the handler hard-sets the storage filter's UserID from context, so
// only the caller's keys are returned and an empty filter can never leak every
// user's keys (T-02-16).
func TestListMyAPIKeys_ScopedToCaller(t *testing.T) {
	seeded := map[string][]*storage.APIKey{
		"u1": {{ID: "k-u1a", UserID: "u1", Prefix: "p1"}, {ID: "k-u1b", UserID: "u1", Prefix: "p2"}},
		"u2": {{ID: "k-u2", UserID: "u2", Prefix: "p3"}},
	}
	var gotFilter storage.ListAPIKeysFilter
	stub := &usersBackendStub{
		listAPIKeys: func(_ context.Context, filter storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
			gotFilter = filter
			return seeded[filter.UserID], nil
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	resp, err := h.ListMyAPIKeys(ctx, connect.NewRequest(&specv1.ListMyAPIKeysRequest{}))
	require.NoError(t, err)
	require.Equal(t, "u1", gotFilter.UserID, "filter UserID must be hard-set from context")
	require.Len(t, resp.Msg.GetKeys(), 2, "only the caller's keys are returned")
	for _, k := range resp.Msg.GetKeys() {
		require.NotEqual(t, "k-u2", k.GetId(), "another user's key must never leak")
	}
}

// TestListMyAPIKeys_NoIdentityUnauthenticated asserts a missing identity is
// rejected before any storage call.
func TestListMyAPIKeys_NoIdentityUnauthenticated(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	_, err := h.ListMyAPIKeys(context.Background(), connect.NewRequest(&specv1.ListMyAPIKeysRequest{}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestRevokeMyAPIKey_ForeignKeyNotFound proves revoking a key the caller does
// not own surfaces as CodeNotFound (the owner-scoped storage call returns
// storage.ErrAPIKeyNotFound for a foreign key), never touching another user's
// key (T-02-16).
func TestRevokeMyAPIKey_ForeignKeyNotFound(t *testing.T) {
	var gotUserID, gotKeyID string
	stub := &usersBackendStub{
		revokeAPIKeyForUser: func(_ context.Context, userID, keyID string) error {
			gotUserID, gotKeyID = userID, keyID
			return storage.ErrAPIKeyNotFound
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	_, err := h.RevokeMyAPIKey(ctx, connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{KeyId: "foreign-key"}))
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	require.Equal(t, "u1", gotUserID, "owner scoped from context")
	require.Equal(t, "foreign-key", gotKeyID)
}

// TestRevokeMyAPIKey_RequiresKeyID asserts an empty key_id is rejected.
func TestRevokeMyAPIKey_RequiresKeyID(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})
	_, err := h.RevokeMyAPIKey(ctx, connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestRevokeMyAPIKey_NoIdentityUnauthenticated asserts a missing identity is
// rejected before any storage call.
func TestRevokeMyAPIKey_NoIdentityUnauthenticated(t *testing.T) {
	h := newSelfKeysHandler(&usersBackendStub{}, testSelfServiceKeysConfig(), nil)
	_, err := h.RevokeMyAPIKey(context.Background(), connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{KeyId: "k1"}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestRevokeMyAPIKey_AuditLogged asserts a successful revoke emits a structured
// audit line carrying the actor/key_id/action=revoke and no secret or raw label
// (T-02-33).
func TestRevokeMyAPIKey_AuditLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	stub := &usersBackendStub{
		revokeAPIKeyForUser: func(_ context.Context, _, _ string) error { return nil },
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), logger)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	_, err := h.RevokeMyAPIKey(ctx, connect.NewRequest(&specv1.RevokeMyAPIKeyRequest{KeyId: "key-revoke"}))
	require.NoError(t, err)

	out := buf.String()
	require.Contains(t, out, "apikey.self.revoke")
	require.Contains(t, out, "actor=u1")
	require.Contains(t, out, "key_id=key-revoke")
	require.Contains(t, out, "action=revoke")
}

// --- Task 3: CSRF mount + ErrQuotaExceeded mapping ---

// TestSelfMint_CSRFMountRejectsMissingToken proves csrfValidate is ACTUALLY
// mounted on the Connect IdentityService handler in RegisterIdentityService
// (T-02-31b): a cookie-authed POST to a self-key procedure without a matching
// X-CSRF-Token is rejected with 403, while a Bearer-authed request to the same
// path is exempt (CSRF passes it through — the CLI path stays green).
func TestSelfMint_CSRFMountRejectsMissingToken(t *testing.T) {
	mux := http.NewServeMux()
	RegisterIdentityService(mux, &usersBackendStub{}, testSelfServiceKeysConfig())
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	const proc = "/specgraph.v1.IdentityService/CreateMyAPIKey"

	// Cookie-authed POST without X-CSRF-Token → 403.
	cookieReq, err := http.NewRequest(http.MethodPost, srv.URL+proc, strings.NewReader("{}"))
	require.NoError(t, err)
	cookieReq.Header.Set("Content-Type", "application/json")
	cookieReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "abc123"}) //nolint:gosec // G124: test-only double-submit cookie; attributes irrelevant to this CSRF-rejection assertion
	cookieResp, err := http.DefaultClient.Do(cookieReq)
	require.NoError(t, err)
	defer cookieResp.Body.Close()
	require.Equal(t, http.StatusForbidden, cookieResp.StatusCode, "cookie-authed POST without X-CSRF-Token must be rejected")

	// Bearer-authed POST is exempt from CSRF — it passes through the validator
	// (and is then handled by Connect; without an injected identity it is
	// Unauthenticated, but crucially NOT a 403 from the CSRF gate).
	bearerReq, err := http.NewRequest(http.MethodPost, srv.URL+proc, strings.NewReader("{}"))
	require.NoError(t, err)
	bearerReq.Header.Set("Content-Type", "application/json")
	bearerReq.Header.Set("Authorization", "Bearer sometoken")
	bearerResp, err := http.DefaultClient.Do(bearerReq)
	require.NoError(t, err)
	defer bearerResp.Body.Close()
	require.NotEqual(t, http.StatusForbidden, bearerResp.StatusCode, "Bearer request must be exempt from CSRF")
}

// TestCreateMyAPIKey_QuotaExceededMapsResourceExhausted proves identityError
// maps the storage quota sentinel to CodeResourceExhausted rather than letting
// it fall through to CodeInternal (T-02-32).
func TestCreateMyAPIKey_QuotaExceededMapsResourceExhausted(t *testing.T) {
	stub := &usersBackendStub{
		createAPIKeyForUser: func(_ context.Context, _ *storage.APIKey, _ int) (*storage.APIKey, error) {
			return nil, storage.ErrQuotaExceeded
		},
	}
	h := newSelfKeysHandler(stub, testSelfServiceKeysConfig(), nil)
	ctx := ctxWithIdentity(&auth.Identity{UserID: "u1", Role: auth.RoleReader, EffectiveRole: auth.RoleReader, Source: "oidc"})

	_, err := h.CreateMyAPIKey(ctx, connect.NewRequest(&specv1.CreateMyAPIKeyRequest{}))
	require.Equal(t, connect.CodeResourceExhausted, connect.CodeOf(err))
}
