// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpecMsg(t *testing.T) {
	t.Run("non-empty slug", func(t *testing.T) {
		got := specMsg("my-spec", "not found")
		assert.Equal(t, `spec "my-spec" not found`, got)
	})

	t.Run("empty slug", func(t *testing.T) {
		got := specMsg("", "not found")
		assert.Equal(t, "spec not found", got)
	})
}
