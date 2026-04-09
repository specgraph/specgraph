// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage_test

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
)

func TestDriftType_IsValid(t *testing.T) {
	tests := []struct {
		driftType storage.DriftType
		expected  bool
	}{
		{storage.DriftTypeDependency, true},
		{storage.DriftTypeInterface, true},
		{storage.DriftTypeVerify, true},
		{storage.DriftType("bogus"), false},
		{storage.DriftType(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.driftType), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.driftType.IsValid())
		})
	}
}

func TestDriftSeverity_IsValid(t *testing.T) {
	tests := []struct {
		severity storage.DriftSeverity
		expected bool
	}{
		{storage.DriftSeverityHigh, true},
		{storage.DriftSeverityMedium, true},
		{storage.DriftSeverityLow, true},
		{storage.DriftSeverityInfo, true},
		{storage.DriftSeverity("critical"), false},
		{storage.DriftSeverity(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.severity), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.severity.IsValid())
		})
	}
}

func TestLintSeverity_IsValid(t *testing.T) {
	tests := []struct {
		severity storage.LintSeverity
		expected bool
	}{
		{storage.LintSeverityError, true},
		{storage.LintSeverityWarning, true},
		{storage.LintSeverityInfo, true},
		{storage.LintSeverity("debug"), false},
		{storage.LintSeverity(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.severity), func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.severity.IsValid())
		})
	}
}
