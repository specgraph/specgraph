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
		if f.Category == "security" && f.Severity == specv1.FindingSeverity_FINDING_SEVERITY_WARNING {
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
				Severity:    specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
				Description: "test security flag",
			},
			{
				Category:    authoring.SafetyCategoryDataLoss,
				Severity:    specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL,
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

func TestSafetyNet_Clean(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Add a new read-only API endpoint for listing users",
	}
	flags := authoring.RunSafetyNet(input)
	require.Empty(t, flags)
}
