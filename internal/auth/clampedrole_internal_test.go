// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import "testing"

// TestClampedRole_FailClosed exercises clampedRole directly (it is unexported).
// The contract: a set downgrade must never leave the key with the owner's
// fuller role. Comparable built-in pairs clamp to the minimum; any pair that
// cannot be ordered (a custom role on either side) floors to reader.
func TestClampedRole_FailClosed(t *testing.T) {
	const (
		ops = Role("ops") // custom, unranked
		dev = Role("dev") // custom, unranked
	)
	cases := []struct {
		name     string
		userRole Role
		downgrade Role
		want     Role
	}{
		{"no cap keeps built-in owner role", RoleAdmin, "", RoleAdmin},
		{"no cap keeps reader", RoleReader, "", RoleReader},
		{"no cap keeps custom owner role", ops, "", ops},
		{"writer capped to reader", RoleWriter, RoleReader, RoleReader},
		{"admin capped to reader", RoleAdmin, RoleReader, RoleReader},
		{"admin capped to writer", RoleAdmin, RoleWriter, RoleWriter},
		{"downgrade above owner does not escalate", RoleReader, RoleWriter, RoleReader},
		{"admin downgrade on reader does not escalate", RoleReader, RoleAdmin, RoleReader},
		{"custom owner + builtin cap floors to reader", ops, RoleReader, RoleReader},
		{"builtin owner + custom cap floors to reader", RoleAdmin, ops, RoleReader},
		{"custom owner + custom cap floors to reader", ops, dev, RoleReader},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clampedRole(c.userRole, c.downgrade); got != c.want {
				t.Errorf("clampedRole(%q, %q) = %q, want %q", c.userRole, c.downgrade, got, c.want)
			}
		})
	}
}
