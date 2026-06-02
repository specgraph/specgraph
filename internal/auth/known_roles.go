// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// KnownRolesFrom returns the set of role names valid for assignment:
// the built-in roles plus any operator-defined custom role NAMES. Under
// Cedar, custom roles carry no permission list — their authorization is
// expressed as Cedar policies (e.g. in a DirectoryPolicySource). This set
// exists only so the resolver can reject JIT/claims-mapping references to
// roles that don't exist. A known role with no matching policy authorizes
// nothing (default-deny), which is the intended behavior.
func KnownRolesFrom(custom []string) map[Role]bool {
	known := map[Role]bool{
		RoleAdmin:  true,
		RoleWriter: true,
		RoleReader: true,
	}
	for _, name := range custom {
		if name != "" {
			known[Role(name)] = true
		}
	}
	return known
}
