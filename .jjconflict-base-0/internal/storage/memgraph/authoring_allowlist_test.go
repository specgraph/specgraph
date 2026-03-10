// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStoreJSONProperty_DisallowedProperty verifies that storeJSONProperty
// rejects property names not in the allowedJSONProperties allowlist.
// This is the primary Cypher-injection guard for dynamic property writes.
func TestStoreJSONProperty_DisallowedProperty(t *testing.T) {
	s := &Store{}
	err := s.storeJSONProperty(context.Background(), "any-slug", "disallowed_property", map[string]string{"key": "value"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "disallowed property name")
}
