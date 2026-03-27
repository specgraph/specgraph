// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNodeLabel_IsValid(t *testing.T) {
	assert.True(t, NodeLabelSpec.IsValid())
	assert.True(t, NodeLabelDecision.IsValid())
	assert.True(t, NodeLabelSlice.IsValid())
	assert.False(t, NodeLabel("Unknown").IsValid())
}
