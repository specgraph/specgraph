// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestHasPermission_ExactMatch(t *testing.T) {
	perms := map[string]bool{"spec:read": true, "spec:write": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected spec:read to match")
	}
	if auth.HasPermission(perms, "spec:delete") {
		t.Fatal("expected spec:delete to not match")
	}
}

func TestHasPermission_FullWildcard(t *testing.T) {
	perms := map[string]bool{"*:*": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected *:* to match spec:read")
	}
	if !auth.HasPermission(perms, "lifecycle:delete") {
		t.Fatal("expected *:* to match lifecycle:delete")
	}
}

func TestHasPermission_ActionWildcard(t *testing.T) {
	perms := map[string]bool{"*:read": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected *:read to match spec:read")
	}
	if auth.HasPermission(perms, "spec:write") {
		t.Fatal("expected *:read to not match spec:write")
	}
}

func TestHasPermission_ServiceWildcard(t *testing.T) {
	perms := map[string]bool{"spec:*": true}
	if !auth.HasPermission(perms, "spec:read") {
		t.Fatal("expected spec:* to match spec:read")
	}
	if !auth.HasPermission(perms, "spec:delete") {
		t.Fatal("expected spec:* to match spec:delete")
	}
	if auth.HasPermission(perms, "decision:read") {
		t.Fatal("expected spec:* to not match decision:read")
	}
}

func TestHasPermission_EmptyPerms(t *testing.T) {
	if auth.HasPermission(nil, "spec:read") {
		t.Fatal("nil perms should deny everything")
	}
	if auth.HasPermission(map[string]bool{}, "spec:read") {
		t.Fatal("empty perms should deny everything")
	}
}
