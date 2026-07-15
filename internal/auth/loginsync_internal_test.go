// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// loginSyncFakeBackend satisfies storage.UsersBackend via an embedded (nil)
// interface; only UpdateUserOnLogin is exercised by applyLoginSync. Any other
// method call would nil-panic, which correctly flags an unexpected dependency.
type loginSyncFakeBackend struct {
	storage.UsersBackend
	updateUserOnLogin func(ctx context.Context, userID, displayName, email, role string) error
}

func (f loginSyncFakeBackend) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	return f.updateUserOnLogin(ctx, userID, displayName, email, role)
}

type loginSyncTracker struct{}

func (loginSyncTracker) Touch(string) {}

func rawArr(vals ...string) json.RawMessage {
	b, _ := json.Marshal(vals)
	return b
}

func TestResolveLoginRole(t *testing.T) {
	adminRule := []config.ClaimMapping{{Claim: "roles", Value: "specgraph.admin", Role: "admin"}}
	claimsAdmin := map[string]json.RawMessage{"roles": rawArr("specgraph.admin")}
	claimsNone := map[string]json.RawMessage{"roles": rawArr("specgraph.other")}

	cases := []struct {
		name        string
		mappings    []config.ClaimMapping
		claims      map[string]json.RawMessage
		current     string
		defaultRole string
		wantRole    string
		wantChanged bool
	}{
		{"rule1 no mappings -> unchanged", nil, claimsNone, "admin", "reader", "admin", false},
		{"rule1 empty slice -> unchanged", []config.ClaimMapping{}, claimsNone, "writer", "reader", "writer", false},
		{"rule2 match", adminRule, claimsAdmin, "reader", "reader", "admin", true},
		{"rule2 match no change -> changed=false", adminRule, claimsAdmin, "admin", "reader", "admin", false},
		{"rule3 no match -> default_role", adminRule, claimsNone, "admin", "reader", "reader", true},
		{"rule3 default unset -> reader", adminRule, claimsNone, "admin", "", "reader", true},
		{"rule3 numeric claim never matches", adminRule, map[string]json.RawMessage{"roles": json.RawMessage(`[1]`)}, "admin", "reader", "reader", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			role, changed := resolveLoginRole(tc.mappings, tc.claims, tc.current, tc.defaultRole)
			require.Equal(t, tc.wantRole, role)
			require.Equal(t, tc.wantChanged, changed)
		})
	}
}

func TestIsPromotion(t *testing.T) {
	cases := []struct {
		current, next string
		want          bool
	}{
		{"reader", "writer", true},
		{"writer", "admin", true},
		{"reader", "admin", true},
		{"admin", "writer", false},     // demotion
		{"writer", "writer", false},    // equal
		{"reader", "auditor", false},   // builtin -> custom (incomparable)
		{"auditor", "admin", false},    // custom -> builtin
		{"auditor", "releaser", false}, // custom -> custom
	}
	for _, tc := range cases {
		require.Equalf(t, tc.want, isPromotion(tc.current, tc.next), "%s->%s", tc.current, tc.next)
	}
}

func newSyncStore(t *testing.T, fake loginSyncFakeBackend, mappings map[string][]config.ClaimMapping, allowlist []string) *pgIdentityStore {
	t.Helper()
	r, err := NewIdentityStore(IdentityStoreConfig{
		Users:                   fake,
		Tracker:                 loginSyncTracker{},
		LoginSyncEnabled:        true,
		KnownRoles:              map[Role]bool{RoleReader: true, RoleWriter: true, RoleAdmin: true},
		JITDefaultRole:          RoleReader,
		JITClaimsMapping:        mappings,
		JITEmailDomainAllowlist: allowlist,
	})
	require.NoError(t, err)
	return r.(*pgIdentityStore)
}

func TestApplyLoginSync_PromotesAndRefreshesMetadata(t *testing.T) {
	var gotRole, gotName, gotEmail string
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, dn, em, role string) error {
		gotName, gotEmail, gotRole = dn, em, role
		return nil
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{
		"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}},
	}, nil)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "old@x.io", Role: "reader"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "new@x.io", Name: "Ada",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.admin")},
	}

	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
	require.Equal(t, "admin", out.Role)
	require.Equal(t, "admin", gotRole)
	// display_name is passed through unchanged — reconciliation now lives in
	// reconcileDisplayName (materializeIdentity), not here. The stored value
	// ("sub-1") is untouched even though claims.Name ("Ada") is present.
	require.Equal(t, "sub-1", gotName)
	require.Equal(t, "new@x.io", gotEmail)
}

func TestApplyLoginSync_PreservesOperatorRename(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, dn, _, _ string) error {
		// applyLoginSync now passes DisplayName through unchanged unconditionally
		// (the staleness heuristic no longer lives in this function — it's
		// reconcileDisplayName's job upstream). This still exercises a real
		// UpdateUserOnLogin call because the role change below drives `changed`.
		require.Equal(t, "Operator Set Name", dn)
		return nil
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "x", Role: "admin"}}}, nil)
	user := &storage.User{ID: "u1", DisplayName: "Operator Set Name", Email: "e@x.io", Role: "reader"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "e@x.io", Name: "Token Name",
		Raw: map[string]json.RawMessage{"roles": rawArr("x")},
	}
	_, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
}

func TestApplyLoginSync_NoOpSkipsWrite(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		t.Fatal("UpdateUserOnLogin must not be called on a no-op login")
		return nil
	}}
	s := newSyncStore(t, fake, nil, nil) // no mappings -> rule 1 unchanged
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "admin"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@x.io", Name: "", Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
	require.Equal(t, "admin", out.Role)
}

func TestApplyLoginSync_DemotionPersistFailure_Denies(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}}}, nil)
	// current admin, token no longer grants admin -> rule 3 demote to reader -> persist fails -> deny.
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "admin"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "e@x.io",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.other")},
	}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.Error(t, err)
	require.Nil(t, out)
	require.ErrorIs(t, err, ErrTransient)
}

func TestApplyLoginSync_UserNotFound_Denies(t *testing.T) {
	// User concurrently soft-deleted between load and write: the active-row
	// guard makes UpdateUserOnLogin return ErrUserNotFound. Must fail closed
	// even on a best-effort (metadata-only) change, not let the user through.
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return storage.ErrUserNotFound
	}}
	s := newSyncStore(t, fake, nil, nil) // no mappings -> role unchanged (metadata-only path)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "old@x.io", Role: "admin"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "new@x.io", // metadata-only change
		Raw: map[string]json.RawMessage{},
	}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.Error(t, err)
	require.Nil(t, out)
	require.ErrorIs(t, err, ErrUnauthenticated)
}

func TestApplyLoginSync_DemotionSucceeds(t *testing.T) {
	var gotRole string
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, role string) error {
		gotRole = role
		return nil
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}}}, nil)
	// current admin, token no longer grants admin -> rule 3 demote to default_role (reader), persists.
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "admin"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "e@x.io",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.other")},
	}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)
	require.Equal(t, "reader", out.Role)
	require.Equal(t, "reader", gotRole)
}

func TestApplyLoginSync_PromotionPersistFailure_BestEffort(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, map[string][]config.ClaimMapping{"iss": {{Claim: "roles", Value: "app.admin", Role: "admin"}}}, nil)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@x.io", Role: "reader"}
	claims := &OIDCClaims{
		Issuer: "iss", Subject: "sub-1", Email: "e@x.io",
		Raw: map[string]json.RawMessage{"roles": rawArr("app.admin")},
	}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err)              // login proceeds
	require.Equal(t, "reader", out.Role) // at the OLD lower role
}

func TestApplyLoginSync_MetadataOnlyFailure_BestEffort(t *testing.T) {
	fake := loginSyncFakeBackend{updateUserOnLogin: func(_ context.Context, _, _, _, _ string) error {
		return errors.New("db down")
	}}
	s := newSyncStore(t, fake, nil, nil) // no mappings -> role unchanged (changed=false)
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "old@x.io", Role: "admin"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "new@x.io", // email changed, role not
		Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err) // metadata-only write failure must NOT deny
	require.Equal(t, "admin", out.Role)
}

func TestApplyLoginSync_AllowlistMiss_Denies(t *testing.T) {
	s := newSyncStore(t, loginSyncFakeBackend{}, nil, []string{"allowed.io"})
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@blocked.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "e@blocked.io", Raw: map[string]json.RawMessage{}}
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.ErrorIs(t, err, ErrUnauthenticated)
	require.Nil(t, out)
}

func TestApplyLoginSync_AllowlistSkippedOnAbsentEmail(t *testing.T) {
	// no mappings + no metadata change -> no write expected, just must not deny.
	s := newSyncStore(t, loginSyncFakeBackend{}, nil, []string{"allowed.io"})
	user := &storage.User{ID: "u1", DisplayName: "sub-1", Email: "e@allowed.io", Role: "reader"}
	claims := &OIDCClaims{Issuer: "iss", Subject: "sub-1", Email: "", Raw: map[string]json.RawMessage{}} // no email claim
	out, err := s.applyLoginSync(context.Background(), claims, user)
	require.NoError(t, err) // absent email skips the allowlist re-check
	require.Equal(t, "reader", out.Role)
}
