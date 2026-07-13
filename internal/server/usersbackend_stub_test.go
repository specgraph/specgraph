// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"fmt"

	"github.com/specgraph/specgraph/internal/storage"
)

// unexpectedCallError is returned by stub guard methods when called unexpectedly.
type unexpectedCallError struct {
	method string
}

func (e *unexpectedCallError) Error() string {
	return fmt.Sprintf("unexpected call to UsersBackend.%s in test", e.method)
}

func errUnexpected(method string) error {
	return &unexpectedCallError{method: method}
}

// usersBackendStub is a test double for storage.UsersBackend with a two-tier
// default policy when a func field is unset:
//
//   - Read methods default benign so Tasks 4–10 tests need not wire every
//     field: ListUsers/ListAPIKeys/ListOIDCBindings return (nil, nil), and
//     GetUserByID returns storage.ErrUserNotFound.
//   - Mutating methods (CreateServiceAccount, UpdateUserRole, SoftDeleteUser,
//     PurgeUser, CreateAPIKey, RevokeAPIKey, RotateAPIKey, UnbindOIDC) fail
//     loud with errUnexpected so unintended writes surface immediately.
//
// Resolve/bootstrap/JIT methods (LookupAPIKeyByPrefix, LookupOIDCBinding,
// GetBootstrap, CreateHuman, JITCreateHuman, TouchLastUsed) are not exercised
// by the IdentityHandler; their guard stubs always return errUnexpected.
type usersBackendStub struct {
	listUsers            func(ctx context.Context, filter storage.ListUsersFilter) ([]*storage.User, error)
	getUserByID          func(ctx context.Context, id string) (*storage.User, error)
	createServiceAccount func(ctx context.Context, u *storage.User) (*storage.User, error)
	updateUserRole       func(ctx context.Context, userID, role string) error
	updateUserOnLogin    func(ctx context.Context, userID, displayName, email, role string) error
	softDeleteUser       func(ctx context.Context, userID string) error
	purgeUser            func(ctx context.Context, userID string) error
	createAPIKey         func(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error)
	revokeAPIKey         func(ctx context.Context, keyID string) error
	rotateAPIKey         func(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error)
	getAPIKeyForUser     func(ctx context.Context, userID, keyID string) (*storage.APIKey, error)
	revokeAPIKeyForUser  func(ctx context.Context, userID, keyID string) error
	rotateAPIKeyForUser  func(ctx context.Context, userID, keyID string, newKey *storage.APIKey) (*storage.APIKey, error)
	createAPIKeyForUser  func(ctx context.Context, k *storage.APIKey, quota int) (*storage.APIKey, error)
	countActiveAPIKeys   func(ctx context.Context, userID string) (int, error)
	listAPIKeys          func(ctx context.Context, filter storage.ListAPIKeysFilter) ([]*storage.APIKey, error)
	listOIDCBindings     func(ctx context.Context, userID string) ([]*storage.OIDCBinding, error)
	unbindOIDC           func(ctx context.Context, bindingID string) error
}

// compile-time assertion: usersBackendStub must implement the full interface.
var _ storage.UsersBackend = (*usersBackendStub)(nil)

// --- management methods (func-field dispatch) ---

func (s *usersBackendStub) ListUsers(ctx context.Context, filter storage.ListUsersFilter) ([]*storage.User, error) {
	if s.listUsers != nil {
		return s.listUsers(ctx, filter)
	}
	return nil, nil
}

func (s *usersBackendStub) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	if s.getUserByID != nil {
		return s.getUserByID(ctx, id)
	}
	return nil, storage.ErrUserNotFound
}

func (s *usersBackendStub) CreateServiceAccount(ctx context.Context, u *storage.User) (*storage.User, error) {
	if s.createServiceAccount != nil {
		return s.createServiceAccount(ctx, u)
	}
	return nil, errUnexpected("CreateServiceAccount")
}

func (s *usersBackendStub) UpdateUserRole(ctx context.Context, userID, role string) error {
	if s.updateUserRole != nil {
		return s.updateUserRole(ctx, userID, role)
	}
	return errUnexpected("UpdateUserRole")
}

func (s *usersBackendStub) UpdateUserOnLogin(ctx context.Context, userID, displayName, email, role string) error {
	if s.updateUserOnLogin != nil {
		return s.updateUserOnLogin(ctx, userID, displayName, email, role)
	}
	return errUnexpected("UpdateUserOnLogin")
}

func (s *usersBackendStub) SoftDeleteUser(ctx context.Context, userID string) error {
	if s.softDeleteUser != nil {
		return s.softDeleteUser(ctx, userID)
	}
	return errUnexpected("SoftDeleteUser")
}

func (s *usersBackendStub) PurgeUser(ctx context.Context, userID string) error {
	if s.purgeUser != nil {
		return s.purgeUser(ctx, userID)
	}
	return errUnexpected("PurgeUser")
}

func (s *usersBackendStub) CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error) {
	if s.createAPIKey != nil {
		return s.createAPIKey(ctx, k)
	}
	return nil, errUnexpected("CreateAPIKey")
}

func (s *usersBackendStub) RevokeAPIKey(ctx context.Context, keyID string) error {
	if s.revokeAPIKey != nil {
		return s.revokeAPIKey(ctx, keyID)
	}
	return errUnexpected("RevokeAPIKey")
}

func (s *usersBackendStub) RotateAPIKey(ctx context.Context, oldKeyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	if s.rotateAPIKey != nil {
		return s.rotateAPIKey(ctx, oldKeyID, newKey)
	}
	return nil, errUnexpected("RotateAPIKey")
}

func (s *usersBackendStub) GetAPIKeyForUser(ctx context.Context, userID, keyID string) (*storage.APIKey, error) {
	if s.getAPIKeyForUser != nil {
		return s.getAPIKeyForUser(ctx, userID, keyID)
	}
	return nil, errUnexpected("GetAPIKeyForUser")
}

func (s *usersBackendStub) RevokeAPIKeyForUser(ctx context.Context, userID, keyID string) error {
	if s.revokeAPIKeyForUser != nil {
		return s.revokeAPIKeyForUser(ctx, userID, keyID)
	}
	return errUnexpected("RevokeAPIKeyForUser")
}

func (s *usersBackendStub) RotateAPIKeyForUser(ctx context.Context, userID, keyID string, newKey *storage.APIKey) (*storage.APIKey, error) {
	if s.rotateAPIKeyForUser != nil {
		return s.rotateAPIKeyForUser(ctx, userID, keyID, newKey)
	}
	return nil, errUnexpected("RotateAPIKeyForUser")
}

func (s *usersBackendStub) CreateAPIKeyForUser(ctx context.Context, k *storage.APIKey, quota int) (*storage.APIKey, error) {
	if s.createAPIKeyForUser != nil {
		return s.createAPIKeyForUser(ctx, k, quota)
	}
	return nil, errUnexpected("CreateAPIKeyForUser")
}

func (s *usersBackendStub) CountActiveAPIKeys(ctx context.Context, userID string) (int, error) {
	if s.countActiveAPIKeys != nil {
		return s.countActiveAPIKeys(ctx, userID)
	}
	return 0, errUnexpected("CountActiveAPIKeys")
}

func (s *usersBackendStub) ListAPIKeys(ctx context.Context, filter storage.ListAPIKeysFilter) ([]*storage.APIKey, error) {
	if s.listAPIKeys != nil {
		return s.listAPIKeys(ctx, filter)
	}
	return nil, nil
}

func (s *usersBackendStub) ListOIDCBindings(ctx context.Context, userID string) ([]*storage.OIDCBinding, error) {
	if s.listOIDCBindings != nil {
		return s.listOIDCBindings(ctx, userID)
	}
	return nil, nil
}

func (s *usersBackendStub) UnbindOIDC(ctx context.Context, bindingID string) error {
	if s.unbindOIDC != nil {
		return s.unbindOIDC(ctx, bindingID)
	}
	return errUnexpected("UnbindOIDC")
}

// --- guard stubs: resolve/bootstrap/JIT paths not called by the handler ---

func (s *usersBackendStub) LookupAPIKeyByPrefix(_ context.Context, _ string) (*storage.APIKey, error) {
	return nil, errUnexpected("LookupAPIKeyByPrefix")
}

func (s *usersBackendStub) LookupOIDCBinding(_ context.Context, _, _ string) (*storage.OIDCBinding, error) {
	return nil, errUnexpected("LookupOIDCBinding")
}

func (s *usersBackendStub) GetBootstrap(_ context.Context) (*storage.User, error) {
	return nil, errUnexpected("GetBootstrap")
}

func (s *usersBackendStub) CreateHuman(_ context.Context, _ *storage.User, _ *storage.OIDCBinding) (*storage.User, error) {
	return nil, errUnexpected("CreateHuman")
}

func (s *usersBackendStub) JITCreateHuman(_ context.Context, _ *storage.User, _ *storage.OIDCBinding) (*storage.User, *storage.OIDCBinding, error) {
	return nil, nil, errUnexpected("JITCreateHuman")
}

func (s *usersBackendStub) TouchLastUsed(_ context.Context, _ string) error {
	return errUnexpected("TouchLastUsed")
}
