// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// UserList renders a slice of User protos as a tabular string.
// Returns "No users found.\n" when the slice is empty or nil.
func UserList(users []*specv1.User) string {
	if len(users) == 0 {
		return "No users found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s  %-16s  %-22s  %-12s  %-8s\n",
		"ID", "KIND", "DISPLAY NAME", "ROLE", "STATUS")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 90))
	for _, u := range users {
		status := "active"
		if u.GetDeletedAt() != nil {
			status = "deleted"
		}
		fmt.Fprintf(&b, "%-24s  %-16s  %-22s  %-12s  %-8s\n",
			u.GetId(),
			userKindLabel(u.GetKind()),
			truncate(u.GetDisplayName(), 20),
			u.GetRole(),
			status,
		)
	}
	return b.String()
}

// userKindLabel converts a UserKind enum to a human-readable label.
func userKindLabel(k specv1.UserKind) string {
	switch k {
	case specv1.UserKind_USER_KIND_HUMAN:
		return "human"
	case specv1.UserKind_USER_KIND_SERVICE_ACCOUNT:
		return "service_account"
	default:
		return "unspecified"
	}
}

// APIKeyList renders a slice of APIKey protos as a tabular string.
// The secret value is never included — only the prefix is shown.
// Returns "No API keys found.\n" when the slice is empty or nil.
func APIKeyList(keys []*specv1.APIKey) string {
	if len(keys) == 0 {
		return "No API keys found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s  %-16s  %-22s  %-8s\n",
		"ID", "PREFIX", "LABEL", "STATUS")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 76))
	for _, k := range keys {
		status := "active"
		if k.GetRevokedAt() != nil {
			status = "revoked"
		}
		fmt.Fprintf(&b, "%-24s  %-16s  %-22s  %-8s\n",
			k.GetId(),
			k.GetPrefix(),
			truncate(k.GetLabel(), 20),
			status,
		)
	}
	return b.String()
}

// OIDCBindingList renders a slice of OIDCBinding protos as a tabular string.
// Returns "No OIDC bindings found.\n" when the slice is empty or nil.
func OIDCBindingList(bindings []*specv1.OIDCBinding) string {
	if len(bindings) == 0 {
		return "No OIDC bindings found.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-24s  %-32s  %-24s\n",
		"ID", "ISSUER", "SUBJECT")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 84))
	for _, bi := range bindings {
		fmt.Fprintf(&b, "%-24s  %-32s  %-24s\n",
			bi.GetId(),
			truncate(bi.GetIssuer(), 30),
			bi.GetSubject(),
		)
	}
	return b.String()
}
