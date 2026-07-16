// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/specgraph/specgraph/internal/config"
	"github.com/specgraph/specgraph/internal/storage"
)

// resolveLoginRole computes the role for an interactive login from the issuer's
// claims_mapping and the verified claims. Returns (newRole, changed).
//
// Rules, evaluated in this exact order (the ordering is the correctness hinge —
// conflating "no mappings" with "no match" would mass-demote mapping-less
// providers):
//
//  1. len(mappings) == 0           -> currentRole unchanged.
//  2. a rule matches               -> that rule's role.
//  3. mappings exist, none match   -> defaultRole (or "reader" if unset).
func resolveLoginRole(mappings []config.ClaimMapping, claims map[string]json.RawMessage, currentRole, defaultRole string) (string, bool) {
	if len(mappings) == 0 {
		return currentRole, false // rule 1
	}
	if matched := applyClaimsMapping(claims, mappings); matched != "" {
		return matched, matched != currentRole // rule 2
	}
	floor := defaultRole // rule 3
	if floor == "" {
		floor = string(RoleReader)
	}
	return floor, floor != currentRole
}

// isPromotion reports true ONLY when both roles are ranked built-ins and the
// new built-in rank is strictly higher than the current. Every other change — a
// rank decrease, equal roles, or ANY transition involving a custom/unranked
// role — returns false, so the login-sync error model treats it as a potential
// demotion and fails closed. This mirrors clampedRole's fail-closed philosophy
// for incomparable custom roles.
func isPromotion(current, next string) bool {
	return roleLessThan(Role(current), Role(next))
}

// applyLoginSync re-enforces the email allowlist, refreshes the email
// metadata, and re-derives the role from the issuer just authenticated for an
// existing OIDC user on interactive login. display_name is NOT computed or
// reconciled here (it is passed through unchanged) — that responsibility
// moved unconditionally upstream to reconcileDisplayName in
// materializeIdentity (AUTH-06, D-01/D-06), so this function stays gated on
// loginSyncEnabled && interactive for role/email/allowlist only. Returns the
// user to surface as the resolved Identity. On a successful change it mutates
// the passed-in user in place AND returns it; callers must pass a non-shared
// instance (resolveJWT passes the freshly-scanned struct from GetUserByID).
// Returns an error that DENIES the login on an allowlist miss or a failed
// demotion.
//
// Error model (classification is gated on `changed` FIRST):
//   - allowlist domain miss (present email)   -> deny (ErrUnauthenticated)
//   - persist returns ErrUserNotFound         -> deny (ErrUnauthenticated): the
//     user was concurrently soft-deleted between load and write; fail closed
//     regardless of changed/promotion
//   - changed && !isPromotion, persist fails  -> deny (ErrTransient), fail closed
//   - changed && isPromotion, persist fails   -> best-effort, old role
//   - !changed (metadata-only), persist fails -> best-effort
func (s *pgIdentityStore) applyLoginSync(ctx context.Context, claims *OIDCClaims, user *storage.User) (*storage.User, error) {
	// 1. Re-enforce the email-domain allowlist. Skip when the token carries no
	// email claim — an existing binding already passed at bind time, and some
	// providers intermittently omit email.
	if len(s.jitEmailAllowlist) > 0 && claims.Email != "" {
		domain := emailDomain(claims.Email)
		if domain == "" || !s.jitEmailAllowlist[domain] {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync refused — email domain not in allowlist",
				slog.String("issuer", claims.Issuer), slog.String("domain", domain))
			return nil, ErrUnauthenticated
		}
	}

	// 2. Compute the new role and metadata. display_name is NOT computed here
	// — it is reconciled unconditionally upstream in materializeIdentity (via
	// reconcileDisplayName) before this gate runs, so user.DisplayName is
	// already the reconciled value by the time applyLoginSync sees it. Pass
	// it through unchanged (D-06).
	//
	// Non-atomicity note: the upstream reconciliation write and this
	// function's write are two independent, sequential, non-transactional
	// UpdateUserOnLogin calls against the same row (see the accepted-tradeoff
	// comment at the reconciliation call site in materializeIdentity). If this
	// function denies the login below (allowlist miss or failed demotion),
	// the display-name write already committed upstream is NOT rolled back.
	newRole, changed := resolveLoginRole(s.jitClaimsMapping[claims.Issuer], claims.Raw, user.Role, string(s.jitDefaultRole))

	newEmail := user.Email
	if claims.Email != "" {
		newEmail = claims.Email
	}

	// 3. No-op skip: nothing to persist.
	if newEmail == user.Email && newRole == user.Role {
		return user, nil
	}

	// 4. Persist (single atomic UPDATE).
	if err := s.users.UpdateUserOnLogin(ctx, user.ID, user.DisplayName, newEmail, newRole); err != nil {
		// ErrUserNotFound means the active-row guard (deleted_at IS NULL) matched
		// nothing: the user was concurrently soft-deleted between the load in
		// resolveJWT and this write. Fail closed regardless of changed/promotion —
		// a deleted user must not finish authenticating.
		if errors.Is(err, storage.ErrUserNotFound) {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync target user not active — denying login",
				slog.String("user_id", user.ID), slog.String("issuer", claims.Issuer), slog.String("subject", claims.Subject))
			return nil, ErrUnauthenticated
		}
		// 5. Classify on `changed` first.
		if !changed {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync metadata update failed (proceeding)",
				slog.String("user_id", user.ID), slog.Any("error", err))
			return user, nil // best-effort
		}
		if isPromotion(user.Role, newRole) {
			slog.LogAttrs(ctx, slog.LevelWarn, "auth: login-sync promotion persist failed (proceeding at old role)",
				slog.String("user_id", user.ID), slog.String("old", user.Role), slog.String("new", newRole), slog.Any("error", err))
			return user, nil // best-effort, OLD (lower) role
		}
		slog.LogAttrs(ctx, slog.LevelError, "auth: login-sync demotion persist failed — denying login",
			slog.Bool("audit", true), slog.String("user_id", user.ID), slog.String("subject", claims.Subject),
			slog.String("issuer", claims.Issuer), slog.String("old", user.Role), slog.String("new", newRole), slog.Any("error", err))
		return nil, fmt.Errorf("%w: login-sync demotion persist failed", ErrTransient) // fail closed
	}

	// Success: mutate the returned user from the persisted values; audit role changes.
	if changed {
		slog.LogAttrs(ctx, slog.LevelInfo, "auth: login-sync role change",
			slog.Bool("audit", true), slog.String("user_id", user.ID), slog.String("subject", claims.Subject),
			slog.String("issuer", claims.Issuer), slog.String("old", user.Role), slog.String("new", newRole))
	}
	user.Email = newEmail
	user.Role = newRole
	return user, nil
}
