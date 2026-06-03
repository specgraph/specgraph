// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package bootstrap creates the first admin identity. It is the shared DB
// helper behind both bootstrap paths: `specgraph init` (local, direct DB) and
// the server's first start (hosted). The bootstrap user is a SYSTEM identity
// (display_name "admin", bootstrap=true, no OIDC binding) — never derived
// from the OS user, hostname, or any environmental signal.
package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/specgraph/specgraph/internal/auth"
	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the narrow slice of storage.UsersBackend that Ensure needs.
// storage.UsersBackend (and the postgres *AuthStore) satisfy it structurally.
type Backend interface {
	GetBootstrap(ctx context.Context) (*storage.User, error)
	CreateHuman(ctx context.Context, u *storage.User, binding *storage.OIDCBinding) (*storage.User, error)
	CreateAPIKey(ctx context.Context, k *storage.APIKey) (*storage.APIKey, error)
}

// Options parametrizes Ensure.
type Options struct {
	// Role for the bootstrap admin. Defaults to "admin".
	Role string
}

// Result reports what Ensure did.
type Result struct {
	Created bool   // true if this call created the bootstrap user + key
	Token   string // plaintext token (only set when Created; show once)
	UserID  string // bootstrap user id (set whether created or pre-existing)
}

// Ensure creates the bootstrap admin + an admin API key if no bootstrap user
// exists, and is a no-op otherwise. Idempotent and race-safe: concurrent
// callers converge to one bootstrap row (CreateHuman returns
// ErrBootstrapExists for the loser, which re-reads the winner).
func Ensure(ctx context.Context, b Backend, opts Options) (Result, error) {
	role := opts.Role
	if role == "" {
		role = "admin"
	}

	// Idempotency check.
	if existing, err := b.GetBootstrap(ctx); err == nil {
		return Result{Created: false, UserID: existing.ID}, nil
	} else if !errors.Is(err, storage.ErrUserNotFound) {
		return Result{}, fmt.Errorf("check bootstrap: %w", err)
	}

	// Create the system admin (no OIDC binding — backstop identity).
	user, err := b.CreateHuman(ctx, &storage.User{
		Kind:        storage.KindHuman,
		DisplayName: "admin", // literal; NOT env-derived
		Role:        role,
		Bootstrap:   true,
	}, nil)
	if err != nil {
		// Lost a race: another caller created the bootstrap first.
		if errors.Is(err, storage.ErrBootstrapExists) {
			existing, getErr := b.GetBootstrap(ctx)
			if getErr != nil {
				return Result{}, fmt.Errorf("re-read bootstrap after race: %w", getErr)
			}
			return Result{Created: false, UserID: existing.ID}, nil
		}
		return Result{}, fmt.Errorf("create bootstrap user: %w", err)
	}

	// Mint an admin key. Storage owns the prefix (see 4a Task 7); build the
	// token from the storage-assigned prefix.
	//
	// NOTE: if key minting fails after the bootstrap user was created, the user
	// persists with no key and a later Ensure short-circuits on idempotency
	// (GetBootstrap finds the user) — it will NOT retry the key. Recovering a
	// keyless bootstrap admin is tracked as a follow-up (spgr-rjrt orphaned-
	// bootstrap recovery); for now a mint failure here requires operator action.
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return Result{}, fmt.Errorf("generate bootstrap key: %w", err)
	}
	key, err := b.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  user.ID,
		PHCHash: phc,
		Label:   "bootstrap admin key",
	})
	if err != nil {
		return Result{}, fmt.Errorf("create bootstrap key: %w", err)
	}
	return Result{
		Created: true,
		Token:   auth.FormatAPIKeyToken(key.Prefix, secret),
		UserID:  user.ID,
	}, nil
}
