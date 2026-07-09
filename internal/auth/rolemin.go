// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// RoleMin returns the less-privileged of a and b, fail-closed to RoleReader
// when either is unranked. It reuses the roleRank ordering (identitystore.go)
// rather than introducing a second comparator, and mirrors clampedRole's
// spgr-rjrt.9 semantics: an unranked/empty role on either side collapses to
// the most-restrictive built-in (reader) instead of falling through to the
// fuller role. Consumed by the self-mint create/rotate floor (Plan 05).
//
// See docs/superpowers/specs/2026-06-04-spgr-rjrt-9-role-downgrade-failclosed-design.md.
func RoleMin(a, b Role) Role {
	ra, oka := roleRank[a]
	rb, okb := roleRank[b]
	if !oka || !okb {
		return RoleReader
	}
	if ra <= rb {
		return a
	}
	return b
}
