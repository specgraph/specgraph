// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeDecisionStatus(t *testing.T) {
	tests := []struct {
		raw      string
		expected storage.DecisionStatus
	}{
		{"DECISION_STATUS_PROPOSED", storage.DecisionStatusProposed},
		{"DECISION_STATUS_ACCEPTED", storage.DecisionStatusAccepted},
		{"DECISION_STATUS_SUPERSEDED", storage.DecisionStatusSuperseded},
		{"DECISION_STATUS_DEPRECATED", storage.DecisionStatusDeprecated},
		{"proposed", storage.DecisionStatusProposed},
		{"accepted", storage.DecisionStatusAccepted},
		{"superseded", storage.DecisionStatusSuperseded},
		{"deprecated", storage.DecisionStatusDeprecated},
		{"unknown", storage.DecisionStatus("unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeDecisionStatus(tt.raw))
		})
	}
}

func TestLegacyDecisionStatus(t *testing.T) {
	tests := []struct {
		status storage.DecisionStatus
		legacy string
	}{
		{storage.DecisionStatusProposed, "DECISION_STATUS_PROPOSED"},
		{storage.DecisionStatusAccepted, "DECISION_STATUS_ACCEPTED"},
		{storage.DecisionStatusSuperseded, "DECISION_STATUS_SUPERSEDED"},
		{storage.DecisionStatusDeprecated, "DECISION_STATUS_DEPRECATED"},
		{storage.DecisionStatus("unknown"), ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.legacy, legacyDecisionStatus(tt.status))
		})
	}
}
