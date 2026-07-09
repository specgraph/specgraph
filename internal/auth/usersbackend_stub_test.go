// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/argon2"

	"github.com/specgraph/specgraph/internal/storage"
)

// compile-time assertion: usersBackendStub must implement storage.UsersBackend.
var _ storage.UsersBackend = (*usersBackendStub)(nil)

// TestStubSecretLength guards the 32-char invariant on stubPHCSecret, which
// the API-key parser (apiKeySecretLen in identitystore.go) depends on. Every
// test token is built from this constant via stubAPIKeyToken.
func TestStubSecretLength(t *testing.T) {
	if len(stubPHCSecret) != 32 {
		t.Fatalf("stubPHCSecret must be exactly 32 chars (got %d); update all callers", len(stubPHCSecret))
	}
}

// --- Shared API-key test fixtures ---

// stubPHCSecret is the canonical 32-char test secret. It MUST be exactly
// 32 characters to match the API-key parser's expected secret length
// (apiKeySecretLen in identitystore.go). TestStubSecretLength guards this
// at runtime. Do not change without updating all dependents — every token is
// built from this constant via stubAPIKeyToken.
const stubPHCSecret = "test-secret-32-chars-padding-aaa" //nolint:gosec // test fixture, not a real credential

// stubPHCHash is the argon2id PHC encoding of stubPHCSecret, computed once
// in init() with the same parameters production uses. A token built by
// stubAPIKeyToken verifies against this hash.
var stubPHCHash string

func init() {
	salt := []byte("teststaltestsalt") // exactly 16 bytes
	hash := argon2.IDKey([]byte(stubPHCSecret), salt, 2, 19456, 1, 32)
	stubPHCHash = fmt.Sprintf("$argon2id$v=19$m=19456,t=2,p=1$%s$%s",
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash))
}

// stubAPIKeyToken builds a well-formed API-key token whose secret is
// stubPHCSecret, so it verifies against stubPHCHash. Callers pass an
// 8-char prefix.
func stubAPIKeyToken(prefix string) string { //nolint:unparam // prefix varies by test; kept as parameter for clarity
	return "spgr_sk_" + prefix + "_" + stubPHCSecret
}

// usersBackendStub is a hand-rolled stub for storage.UsersBackend used
// by Resolver unit tests. Each field is a function the test sets to
// control behavior. Unset functions return a default that flags an
// unexpected-call test bug.
type usersBackendStub struct {
	lookupAPIKey      func(ctx context.Context, prefix string) (*storage.APIKey, error)
	lookupOIDCBinding func(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error)
	getUserByID       func(ctx context.Context, id string) (*storage.User, error)
	getBootstrap      func(ctx context.Context) (*storage.User, error)
	jitCreateHuman    func(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error)
	touchLastUsed     func(ctx context.Context, keyID string) error
	listUsers         func(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error)
	updateUserOnLogin func(ctx context.Context, userID, displayName, email, role string) error
}

func (s *usersBackendStub) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*storage.APIKey, error) {
	if s.lookupAPIKey == nil {
		return nil, storage.ErrAPIKeyNotFound
	}
	return s.lookupAPIKey(ctx, prefix)
}

func (s *usersBackendStub) LookupOIDCBinding(ctx context.Context, issuer, subject string) (*storage.OIDCBinding, error) {
	if s.lookupOIDCBinding == nil {
		return nil, storage.ErrOIDCBindingNotFound
	}
	return s.lookupOIDCBinding(ctx, issuer, subject)
}

func (s *usersBackendStub) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	if s.getUserByID == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.getUserByID(ctx, id)
}

func (s *usersBackendStub) GetBootstrap(ctx context.Context) (*storage.User, error) {
	if s.getBootstrap == nil {
		return nil, storage.ErrUserNotFound
	}
	return s.getBootstrap(ctx)
}

func (s *usersBackendStub) JITCreateHuman(ctx context.Context, u *storage.User, b *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	if s.jitCreateHuman == nil {
		return nil, nil, errUnexpectedCall("JITCreateHuman")
	}
	return s.jitCreateHuman(ctx, u, b)
}

func (s *usersBackendStub) TouchLastUsed(ctx context.Context, keyID string) error {
	if s.touchLastUsed == nil {
		return nil // fire-and-forget; default no-op
	}
	return s.touchLastUsed(ctx, keyID)
}

func (s *usersBackendStub) ListUsers(ctx context.Context, f storage.ListUsersFilter) ([]*storage.User, error) {
	if s.listUsers == nil {
		return nil, errUnexpectedCall("ListUsers")
	}
	return s.listUsers(ctx, f)
}

// Methods unused by Resolver — these are guards that fail loud if the
// resolver calls them unexpectedly.
func (s *usersBackendStub) CreateHuman(_ context.Context, _ *storage.User, _ *storage.OIDCBinding) (*storage.User, error) {
	return nil, errUnexpectedCall("CreateHuman")
}
func (s *usersBackendStub) CreateServiceAccount(_ context.Context, _ *storage.User) (*storage.User, error) {
	return nil, errUnexpectedCall("CreateServiceAccount")
}
func (s *usersBackendStub) UpdateUserRole(_ context.Context, _, _ string) error {
	return errUnexpectedCall("UpdateUserRole")
}
func (s *usersBackendStub) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	if s.updateUserOnLogin != nil {
		return s.updateUserOnLogin(ctx, userID, displayName, email, role)
	}
	return errUnexpectedCall("UpdateUserOnLogin")
}
func (s *usersBackendStub) SoftDeleteUser(_ context.Context, _ string) error {
	return errUnexpectedCall("SoftDeleteUser")
}
func (s *usersBackendStub) PurgeUser(_ context.Context, _ string) error {
	return errUnexpectedCall("PurgeUser")
}
func (s *usersBackendStub) CreateAPIKey(_ context.Context, _ *storage.APIKey) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("CreateAPIKey")
}
func (s *usersBackendStub) RevokeAPIKey(_ context.Context, _ string) error {
	return errUnexpectedCall("RevokeAPIKey")
}
func (s *usersBackendStub) RotateAPIKey(_ context.Context, _ string, _ *storage.APIKey) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("RotateAPIKey")
}
func (s *usersBackendStub) GetAPIKeyForUser(_ context.Context, _, _ string) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("GetAPIKeyForUser")
}
func (s *usersBackendStub) RevokeAPIKeyForUser(_ context.Context, _, _ string) error {
	return errUnexpectedCall("RevokeAPIKeyForUser")
}
func (s *usersBackendStub) RotateAPIKeyForUser(_ context.Context, _, _ string, _ *storage.APIKey) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("RotateAPIKeyForUser")
}
func (s *usersBackendStub) CreateAPIKeyForUser(_ context.Context, _ *storage.APIKey, _ int) (*storage.APIKey, error) {
	return nil, errUnexpectedCall("CreateAPIKeyForUser")
}
func (s *usersBackendStub) CountActiveAPIKeys(_ context.Context, _ string) (int, error) {
	return 0, errUnexpectedCall("CountActiveAPIKeys")
}
func (s *usersBackendStub) ListAPIKeys(_ context.Context, _ storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	return nil, errUnexpectedCall("ListAPIKeys")
}
func (s *usersBackendStub) ListOIDCBindings(_ context.Context, _ string) ([]*storage.OIDCBinding, error) {
	return nil, errUnexpectedCall("ListOIDCBindings")
}
func (s *usersBackendStub) UnbindOIDC(_ context.Context, _ string) error {
	return errUnexpectedCall("UnbindOIDC")
}

func errUnexpectedCall(method string) error {
	return errors.New("usersBackendStub: unexpected call to " + method + " (test bug)")
}

// activeUser builds an active user for tests.
func activeUser(id, role string, kind storage.Kind) *storage.User { //nolint:unparam // kind parameter kept for future tests using service accounts
	return &storage.User{
		ID: id, Kind: kind, Role: role, DisplayName: "test-" + id,
		CreatedAt: time.Now(),
	}
}

// activeKey builds an active API key for tests. PHCHash is the verifiable
// stubPHCHash (computed in init() from stubPHCSecret), so a token built via
// stubAPIKeyToken(prefix) will successfully argon2id-verify against it.
func activeKey(id, userID, prefix string) *storage.APIKey { //nolint:unused // available for future tasks that need a pre-built key fixture
	return &storage.APIKey{
		ID: id, UserID: userID, Prefix: prefix,
		PHCHash:   stubPHCHash,
		CreatedAt: time.Now(),
	}
}
