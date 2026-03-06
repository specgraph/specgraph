// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"
	"time"

	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestNewSpec(t *testing.T) {
	now := time.Now().UTC()
	spec := &storage.Spec{
		ID:         "spec-abc1234",
		Slug:       "login-api",
		Intent:     "REST endpoint for OAuth2",
		Stage:      "spark",
		Priority:   "p1",
		Complexity: "medium",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	assert.Equal(t, "login-api", spec.Slug)
	assert.Equal(t, int32(1), spec.Version)
	assert.False(t, spec.CreatedAt.IsZero())
}
