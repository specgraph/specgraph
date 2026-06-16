// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"encoding/json"

	"github.com/specgraph/specgraph/internal/config"
)

// resolveLoginRole computes the role for an interactive login from the issuer's
// claims_mapping and the verified claims. Returns (newRole, changed).
//
// Rules, evaluated in this exact order (the ordering is the correctness hinge —
// conflating "no mappings" with "no match" would mass-demote mapping-less
// providers):
//
//	1. len(mappings) == 0           -> currentRole unchanged.
//	2. a rule matches               -> that rule's role.
//	3. mappings exist, none match   -> defaultRole (or "reader" if unset).
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
