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
		if f.Category == "security" && f.Severity == specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL {
			found = true
			break
		}
	}
	require.True(t, found, "expected a security flag with CRITICAL severity")
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

func TestSafetyNet_Clean(t *testing.T) {
	input := &authoring.SafetyInput{
		Intent: "Add a new read-only API endpoint for listing users",
	}
	flags := authoring.RunSafetyNet(input)
	require.Empty(t, flags)
}
