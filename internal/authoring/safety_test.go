// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package authoring_test

import (
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

func TestSafetyNet_SecurityFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Store user passwords in plaintext for faster lookup",
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)

	var found bool
	for _, f := range flags {
		if f.Category == "security" && f.Severity == authoring.SeverityWarning {
			found = true
			break
		}
	}
	require.True(t, found, "expected a security flag with WARNING severity for ambiguous 'plaintext' pattern")
}

func TestSafetyNet_DataLossFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Drop all tables and recreate schema without migration",
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)

	var found bool
	for _, f := range flags {
		if f.Category == "data_loss" {
			found = true
			break
		}
	}
	require.True(t, found, "expected a data_loss flag")
}

func TestSafetyNet_ScopeFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Add a new endpoint",
		Scope:  []string{"skip validation on user input"},
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)
	require.Equal(t, authoring.SafetyCategorySecurity, flags[0].Category)
}

func TestSafetyNet_InvariantsFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent:     "Update schema",
		Invariants: []string{"Must allow truncate of stale data"},
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)
	require.Equal(t, authoring.SafetyCategoryDataLoss, flags[0].Category)
}

func TestSafetyResultsToProto(t *testing.T) {
	t.Run("converts domain flags to proto", func(t *testing.T) {
		flags := []authoring.SafetyFlagResult{
			{
				Category:    authoring.SafetyCategorySecurity,
				Severity:    authoring.SeverityCritical,
				Description: "test security flag",
			},
			{
				Category:    authoring.SafetyCategoryDataLoss,
				Severity:    authoring.SeverityCritical,
				Description: "test data loss flag",
			},
		}
		protos := authoring.SafetyResultsToProto(flags)
		require.Len(t, protos, 2)
		require.Equal(t, "security", protos[0].Category)
		require.Equal(t, specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, protos[0].Severity)
		require.Equal(t, "test security flag", protos[0].Description)
		require.Equal(t, "data_loss", protos[1].Category)
	})

	t.Run("empty input returns empty output", func(t *testing.T) {
		protos := authoring.SafetyResultsToProto(nil)
		require.Empty(t, protos)
	})
}

// TestSafetyNet_CriticalWinsOverWarning verifies that when both CRITICAL and WARNING
// patterns match for the same category, only the CRITICAL finding is returned.
// The test does not depend on the ordering of allPatternGroups.
func TestSafetyNet_CriticalWinsOverWarning(t *testing.T) {
	// "hardcoded secret" triggers security CRITICAL; "plaintext" triggers security WARNING.
	// Both match the same "security" category; only CRITICAL should be returned.
	input := &authoring.SafetyInput{
		Intent: "Store hardcoded secret in plaintext config file",
	}
	flags := authoring.RunSafetyNet(input)

	var securityFlags []authoring.SafetyFlagResult
	for _, f := range flags {
		if f.Category == authoring.SafetyCategorySecurity {
			securityFlags = append(securityFlags, f)
		}
	}

	require.Len(t, securityFlags, 1, "expected exactly one security flag (dedup by category)")
	require.Equal(t, authoring.SeverityCritical, securityFlags[0].Severity,
		"expected CRITICAL to win over WARNING when both patterns match")
}

func TestSafetyNet_Clean(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Add a new read-only API endpoint for listing users",
	}
	flags := authoring.RunSafetyNet(input)
	require.Empty(t, flags)
}
