// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
)

// DefaultRolePermissions defines the built-in role permission bundles.
//
// RELOCATED here from config_store.go in this task. config_store.go is
// deleted in Task 30, but LoadRolePerms (below) and the legacy code both
// reference DefaultRolePermissions, so it must live in a file that
// survives. static_authorizer.go is the natural home (it owns role→perm
// policy) and itself survives until the Cedar plan. See Step 3b for the
// matching removal from config_store.go.
var DefaultRolePermissions = map[Role][]string{
	RoleReader: {"*:read"},
	RoleWriter: {"*:read", "*:write"},
	RoleAdmin:  {"*:*"},
}

// StaticTableAuthorizer implements Authorizer by consulting the static
// rpcPermissions table (permissions.go) and a snapshot of role→permissions
// loaded at construction time. This impl will be deleted when the Cedar
// plan introduces CedarAuthorizer.
type StaticTableAuthorizer struct {
	rolePerms map[Role]map[string]bool
}

// NewStaticTableAuthorizer builds a StaticTableAuthorizer from a
// role→[]permission mapping. The constructor expands the lists into a
// per-role lookup map for cheap membership checks at Authorize time.
//
// rolePerms is typically built by LoadRolePerms(cfg.Auth.Roles) — see
// `LoadRolePerms` (exported) in this file.
func NewStaticTableAuthorizer(rolePerms map[Role][]string) *StaticTableAuthorizer {
	expanded := make(map[Role]map[string]bool, len(rolePerms))
	for role, perms := range rolePerms {
		entry := make(map[string]bool, len(perms))
		for _, p := range perms {
			entry[p] = true
		}
		expanded[role] = entry
	}
	return &StaticTableAuthorizer{rolePerms: expanded}
}

// Authorize implements the Authorizer interface.
func (a *StaticTableAuthorizer) Authorize(_ context.Context, id *Identity, procedure string, _ any) (Decision, error) {
	if id == nil {
		return Decision{}, fmt.Errorf("static-table: nil identity")
	}
	required, ok := rpcPermissions[procedure]
	if !ok {
		return Decision{}, fmt.Errorf("static-table: unconfigured procedure %q", procedure)
	}
	perms, ok := a.rolePerms[id.EffectiveRole]
	if !ok {
		return Decision{
			Allowed: false,
			Reason:  fmt.Sprintf("static-table-deny:unknown-role:%s", id.EffectiveRole),
		}, nil
	}
	if hasPermissionInternal(perms, required) {
		return Decision{
			Allowed: true,
			Reason:  fmt.Sprintf("static-table-allow:%s", required),
		}, nil
	}
	return Decision{
		Allowed: false,
		Reason:  fmt.Sprintf("static-table-deny:%s not in %s", required, id.EffectiveRole),
	}, nil
}

// hasPermissionInternal is the wildcard-matching helper used by
// StaticTableAuthorizer. The same logic is temporarily duplicated in the
// exported auth.HasPermission (still consumed by legacy code in Phase A).
// The duplication is intentional and short-lived: Task 31 removes the
// exported HasPermission once the legacy stores are gone, leaving this
// package-private copy as the sole implementation. Package-private here
// because no caller outside this struct should do raw perm checks — they
// go through Authorize.
func hasPermissionInternal(perms map[string]bool, required string) bool {
	if len(perms) == 0 {
		return false
	}
	if perms["*:*"] {
		return true
	}
	if perms[required] {
		return true
	}
	parts := strings.SplitN(required, ":", 2)
	if len(parts) != 2 {
		return false
	}
	service, action := parts[0], parts[1]
	if perms[service+":*"] {
		return true
	}
	if perms["*:"+action] {
		return true
	}
	return false
}

// LoadRolePerms builds the rolePerms map from the YAML auth.roles section
// combined with the built-in DefaultRolePermissions. Exported so serve.go
// (a different package) can construct both the StaticTableAuthorizer and
// the KnownRoles set for JIT validation from a single source.
//
// custom is cfg.Auth.Roles from internal/config — a real named type, not
// an anonymous struct. The auth package imports internal/config (already
// imported elsewhere in the package, e.g., OIDCVerifier).
func LoadRolePerms(custom map[string]config.RoleConfig) map[Role][]string {
	out := make(map[Role][]string, len(DefaultRolePermissions)+len(custom))
	for role, perms := range DefaultRolePermissions {
		out[role] = append([]string(nil), perms...)
	}
	for name, rc := range custom {
		out[Role(name)] = append([]string(nil), rc.Permissions...)
	}
	return out
}
