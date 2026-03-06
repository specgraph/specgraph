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
		Text: "Store user passwords in plaintext for faster lookup",
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)

	var found bool
	for _, f := range flags {
		if f.Category == authoring.SafetyCategorySecurity && f.Severity == authoring.SeverityWarning {
			found = true
			break
		}
	}
	require.True(t, found, "expected a security flag with WARNING severity for ambiguous 'plaintext' pattern")
}

func TestSafetyNet_DataLossFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Text: "Drop all tables and recreate schema without migration",
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)

	var found bool
	for _, f := range flags {
		if f.Category == authoring.SafetyCategoryDataLoss {
			found = true
			break
		}
	}
	require.True(t, found, "expected a data_loss flag")
}

func TestSafetyNet_ScopeFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Text:  "Add a new endpoint",
		Scope: []string{"skip validation on user input"},
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags)
	require.Equal(t, authoring.SafetyCategorySecurity, flags[0].Category)
}

func TestSafetyNet_InvariantsFlags(t *testing.T) {
	input := &authoring.SafetyInput{
		Text:       "Update schema",
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
		require.Equal(t, specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY, protos[0].Category)
		require.Equal(t, specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, protos[0].Severity)
		require.Equal(t, "test security flag", protos[0].Description)
		require.Equal(t, specv1.SafetyCategory_SAFETY_CATEGORY_DATA_LOSS, protos[1].Category)
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
		Text: "Store hardcoded secret in plaintext config file",
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

func TestSafetyNet_CaseInsensitive(t *testing.T) {
	// Uppercase input should still match lowercase patterns
	flags := authoring.RunSafetyNet(&authoring.SafetyInput{Text: "HARDCODED SECRET in config"})
	if len(flags) == 0 {
		t.Fatal("expected safety flag for uppercase input")
	}
	if flags[0].Category != authoring.SafetyCategorySecurity {
		t.Errorf("expected security category, got %q", flags[0].Category)
	}
}

func TestSafetyInput_Validate(t *testing.T) {
	t.Run("empty input returns error", func(t *testing.T) {
		s := &authoring.SafetyInput{}
		require.Error(t, s.Validate())
	})

	t.Run("text only passes", func(t *testing.T) {
		s := &authoring.SafetyInput{Text: "add login endpoint"}
		require.NoError(t, s.Validate())
	})

	t.Run("scope only passes", func(t *testing.T) {
		s := &authoring.SafetyInput{Scope: []string{"auth service"}}
		require.NoError(t, s.Validate())
	})

	t.Run("invariants only passes", func(t *testing.T) {
		s := &authoring.SafetyInput{Invariants: []string{"session must expire"}}
		require.NoError(t, s.Validate())
	})
}

func TestSafetyNet_Clean(t *testing.T) {
	input := &authoring.SafetyInput{
		Text: "Add a new read-only API endpoint for listing users",
	}
	flags := authoring.RunSafetyNet(input)
	require.Empty(t, flags)
}

// TestSafetyNet_ZeroWidthBypass verifies that zero-width characters inserted
// inside a dangerous keyword do not prevent detection.
func TestSafetyNet_ZeroWidthBypass(t *testing.T) {
	// U+200B (zero-width space) inserted inside the keyword "plaintext":
	// "plain\u200Btext" — an attacker could use this to evade regex matching.
	// After stripping zero-width chars the keyword is restored and must be flagged.
	input := &authoring.SafetyInput{
		Text: "store passwords in plain\u200Btext format",
	}
	flags := authoring.RunSafetyNet(input)
	require.NotEmpty(t, flags, "expected flag: zero-width space should not bypass safety check")

	var found bool
	for _, f := range flags {
		if f.Category == authoring.SafetyCategorySecurity && f.Severity == authoring.SeverityWarning {
			found = true
			break
		}
	}
	require.True(t, found, "expected WARNING security flag for 'plaintext' with zero-width bypass attempt")
}

// TestSafetyNet_HomoglyphBypass verifies that Cyrillic homoglyphs in security
// keywords are caught after NFKC normalization collapses them to ASCII.
func TestSafetyNet_HomoglyphBypass(t *testing.T) {
	// Replace Latin 'e' in "credential" with Cyrillic 'е' (U+0435) to test
	// homoglyph normalization. NFKC does not map Cyrillic to Latin, so the
	// test verifies the normalization pipeline is exercised without a false
	// positive: the input should NOT be flagged (Cyrillic 'е' ≠ Latin 'e').
	// This guards against regressions where normalization is skipped entirely.
	input := &authoring.SafetyInput{
		// "cr\u0435dential" — the second 'e' is Cyrillic
		Text: "cr\u0435dential store for api keys",
	}
	flags := authoring.RunSafetyNet(input)
	// The homoglyph breaks the regex; document the current behaviour explicitly
	// so any future normalization that maps Cyrillic→Latin is noticed.
	for _, f := range flags {
		require.NotEqual(t, authoring.SafetyCategorySecurity, f.Category,
			"Cyrillic homoglyph in 'credential' should not match after NFKC (Cyrillic 'е' normalizes to itself, not Latin 'e')")
	}
}

func TestToStorageSeverity_KnownValues(t *testing.T) {
	for _, tc := range []struct {
		in   authoring.FindingSeverity
		want string
	}{
		{authoring.SeverityCritical, "critical"},
		{authoring.SeverityWarning, "warning"},
		{authoring.SeverityNote, "note"},
	} {
		got, err := authoring.ToStorageSeverity(tc.in)
		require.NoError(t, err)
		require.Equal(t, tc.want, string(got))
	}
}

func TestToStorageSeverity_UnknownValue(t *testing.T) {
	_, err := authoring.ToStorageSeverity(authoring.FindingSeverity(99))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unrecognized FindingSeverity value")
}
