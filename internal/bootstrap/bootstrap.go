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
	// ListAPIKeys is used to detect a keyless bootstrap admin (a user that
	// persisted after CreateHuman but before CreateAPIKey) so Ensure can mint a
	// recovery key. A zero-Limit filter relies on the storage default page size.
	ListAPIKeys(ctx context.Context, f storage.ListAPIKeysFilter) ([]*storage.APIKey, error)
}

// Options parametrizes Ensure.
type Options struct {
	// Role for the bootstrap admin. Defaults to "admin".
	Role string
}

// Result reports what Ensure did.
type Result struct {
	// Created is true when this call produced a fresh plaintext token the
	// operator must save — either because it created the bootstrap user + key,
	// or because it recovered a pre-existing keyless bootstrap admin by minting
	// a new key. False on a true no-op (bootstrap user already has a key).
	Created bool
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

	// Idempotency check. A pre-existing bootstrap user is normally a no-op, but
	// recoverIfKeyless re-mints a key if the user persisted without one (an
	// earlier Ensure died between CreateHuman and CreateAPIKey).
	if existing, err := b.GetBootstrap(ctx); err == nil {
		return recoverIfKeyless(ctx, b, existing)
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
		// Lost a race: another caller created the bootstrap first. Re-read it and
		// run the same keyless-recovery check — if the race WINNER died between
		// CreateHuman and CreateAPIKey, the user is keyless and we must mint here
		// rather than no-op and defer recovery to a later Ensure.
		if errors.Is(err, storage.ErrBootstrapExists) {
			existing, getErr := b.GetBootstrap(ctx)
			if getErr != nil {
				return Result{}, fmt.Errorf("re-read bootstrap after race: %w", getErr)
			}
			return recoverIfKeyless(ctx, b, existing)
		}
		return Result{}, fmt.Errorf("create bootstrap user: %w", err)
	}

	// Mint the admin key. If this fails after CreateHuman committed, the user
	// persists keyless — but a later Ensure now recovers it via the idempotency
	// branch above (it re-mints when ListAPIKeys finds no active key).
	token, err := mintBootstrapKey(ctx, b, user.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{Created: true, Token: token, UserID: user.ID}, nil
}

// recoverIfKeyless returns a no-op Result for an existing bootstrap user that
// still has an active key, or mints a recovery key (Created + Token) if it has
// none. It closes the mint-failure-after-create window: a bootstrap user that
// persisted without a key would otherwise be unrecoverable, since every later
// Ensure short-circuits on the idempotency check or the race re-read. Shared by
// both of those paths so they recover identically.
func recoverIfKeyless(ctx context.Context, b Backend, existing *storage.User) (Result, error) {
	keys, err := b.ListAPIKeys(ctx, storage.ListAPIKeysFilter{UserID: existing.ID})
	if err != nil {
		return Result{}, fmt.Errorf("list bootstrap keys: %w", err)
	}
	if len(keys) > 0 {
		return Result{Created: false, UserID: existing.ID}, nil
	}
	token, err := mintBootstrapKey(ctx, b, existing.ID)
	if err != nil {
		return Result{}, err
	}
	return Result{Created: true, Token: token, UserID: existing.ID}, nil
}

// mintBootstrapKey generates a secret and persists an admin API key for userID,
// returning the one-time plaintext token. Storage owns the prefix (see 4a Task
// 7); the token is assembled from the storage-assigned prefix.
func mintBootstrapKey(ctx context.Context, b Backend, userID string) (string, error) {
	secret, phc, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return "", fmt.Errorf("generate bootstrap key: %w", err)
	}
	key, err := b.CreateAPIKey(ctx, &storage.APIKey{
		UserID:  userID,
		PHCHash: phc,
		Label:   "bootstrap admin key",
	})
	if err != nil {
		return "", fmt.Errorf("create bootstrap key: %w", err)
	}
	return auth.FormatAPIKeyToken(key.Prefix, secret), nil
}
