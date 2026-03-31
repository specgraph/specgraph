// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestDecisionConfidence_IsValid(t *testing.T) {
	tests := []struct {
		val  storage.DecisionConfidence
		want bool
	}{
		{storage.DecisionConfidenceHigh, true},
		{storage.DecisionConfidenceMedium, true},
		{storage.DecisionConfidenceLow, true},
		{"", false},
		{"extreme", false},
		{"HIGH", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.val), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.val.IsValid())
		})
	}
}

func TestDecisionScope_IsValid(t *testing.T) {
	tests := []struct {
		val  storage.DecisionScope
		want bool
	}{
		{storage.DecisionScopeProject, true},
		{storage.DecisionScopeTeam, true},
		{storage.DecisionScopeOrg, true},
		{"", false},
		{"global", false},
		{"PROJECT", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.val), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.val.IsValid())
		})
	}
}
