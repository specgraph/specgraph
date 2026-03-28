// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestConversationRole_IsValid(t *testing.T) {
	valid := []storage.ConversationRole{
		storage.ConversationRoleProbe,
		storage.ConversationRoleResponse,
	}
	for _, r := range valid {
		t.Run(string(r), func(t *testing.T) {
			assert.True(t, r.IsValid(), "role %q should be valid", r)
		})
	}

	invalid := []storage.ConversationRole{
		"",
		"bogus",
		"admin",
	}
	for _, r := range invalid {
		name := string(r)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			assert.False(t, r.IsValid(), "role %q should be invalid", r)
		})
	}
}
