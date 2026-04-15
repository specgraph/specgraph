// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfileString(t *testing.T) {
	tests := []struct {
		profile Profile
		want    string
	}{
		{ProfileCore, "core"},
		{ProfileAuthoring, "authoring"},
		{ProfileExecution, "execution"},
		// Unknown values fall back to "core" per the current implementation.
		{Profile(99), "core"},
		{Profile(-1), "core"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Profile(%d)", tt.profile), func(t *testing.T) {
			require.Equal(t, tt.want, tt.profile.String())
		})
	}
}

func TestParseProfile(t *testing.T) {
	tests := []struct {
		input string
		want  Profile
	}{
		{"core", ProfileCore},
		{"authoring", ProfileAuthoring},
		{"execution", ProfileExecution},
		// Unknown strings default to ProfileCore.
		{"unknown", ProfileCore},
		{"", ProfileCore},
		{"CORE", ProfileCore},
		{"Authoring", ProfileCore},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("ParseProfile(%q)", tt.input), func(t *testing.T) {
			require.Equal(t, tt.want, ParseProfile(tt.input))
		})
	}
}

func TestProfileIncludes(t *testing.T) {
	tests := []struct {
		p     Profile
		other Profile
		want  bool
	}{
		// Core includes only core.
		{ProfileCore, ProfileCore, true},
		{ProfileCore, ProfileAuthoring, false},
		{ProfileCore, ProfileExecution, false},
		// Authoring includes core and authoring.
		{ProfileAuthoring, ProfileCore, true},
		{ProfileAuthoring, ProfileAuthoring, true},
		{ProfileAuthoring, ProfileExecution, false},
		// Execution includes all profiles.
		{ProfileExecution, ProfileCore, true},
		{ProfileExecution, ProfileAuthoring, true},
		{ProfileExecution, ProfileExecution, true},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Profile(%d).Includes(Profile(%d))", tt.p, tt.other), func(t *testing.T) {
			require.Equal(t, tt.want, tt.p.Includes(tt.other))
		})
	}
}
