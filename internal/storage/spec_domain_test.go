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
		Stage:      storage.SpecStageSpark,
		Priority:   storage.SpecPriorityP1,
		Complexity: "medium",
		Version:    1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	assert.Equal(t, "spec-abc1234", spec.ID)
	assert.Equal(t, "login-api", spec.Slug)
	assert.Equal(t, "REST endpoint for OAuth2", spec.Intent)
	assert.Equal(t, storage.SpecStageSpark, spec.Stage)
	assert.Equal(t, storage.SpecPriorityP1, spec.Priority)
	assert.Equal(t, "medium", spec.Complexity)
	assert.Equal(t, int32(1), spec.Version)
	assert.Equal(t, now, spec.CreatedAt)
	assert.Equal(t, now, spec.UpdatedAt)
}
