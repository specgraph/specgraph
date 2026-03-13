// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncBeadsCmd_Flags(t *testing.T) {
	require.NotNil(t, syncBeadsCmd)
	assert.Equal(t, "beads", syncBeadsCmd.Use)

	f := syncBeadsCmd.Flags()
	assert.NotNil(t, f.Lookup("stage"))
	assert.NotNil(t, f.Lookup("priority"))
	assert.NotNil(t, f.Lookup("dry-run"))
}

func TestSyncGitHubCmd_Flags(t *testing.T) {
	require.NotNil(t, syncGitHubCmd)
	assert.Equal(t, "github", syncGitHubCmd.Use)

	f := syncGitHubCmd.Flags()
	assert.NotNil(t, f.Lookup("stage"))
	assert.NotNil(t, f.Lookup("priority"))
	assert.NotNil(t, f.Lookup("dry-run"))
}

func TestInjectCmd_Flags(t *testing.T) {
	require.NotNil(t, injectCmd)
	assert.Equal(t, "inject <slug>", injectCmd.Use)

	f := injectCmd.Flags()
	assert.NotNil(t, f.Lookup("tool"))
	assert.NotNil(t, f.Lookup("output"))
}

func TestInjectCmd_RequiresSlug(t *testing.T) {
	err := injectCmd.Args(injectCmd, []string{})
	assert.Error(t, err)
}

func TestInjectCmd_AcceptsSlug(t *testing.T) {
	err := injectCmd.Args(injectCmd, []string{"my-spec"})
	assert.NoError(t, err)
}

func TestInjectCmd_ToolAliases(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"claude-code", false},
		{"claude", false},
		{"cursor", false},
		{"agents-md", false},
		{"agents", false},
		{"bogus", true},
		{"CLAUDE-CODE", false}, // case-insensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			// Save and restore the global flag.
			old := injectTool
			defer func() { injectTool = old }()
			injectTool = tt.input

			// runInject will fail at client creation (no server),
			// but we can detect tool validation errors by checking
			// if the error message mentions "unsupported tool".
			err := runInject(injectCmd, []string{"test-slug"})
			require.Error(t, err, "runInject should error (no server running)")
			if tt.wantErr {
				assert.Contains(t, err.Error(), "unsupported tool")
			} else {
				assert.NotContains(t, err.Error(), "unsupported tool")
			}
		})
	}
}

func TestSyncStatusCmd_Flags(t *testing.T) {
	require.NotNil(t, syncStatusCmd)
	assert.Equal(t, "status", syncStatusCmd.Use)

	f := syncStatusCmd.Flags()
	assert.NotNil(t, f.Lookup("adapter"))
	assert.NotNil(t, f.Lookup("spec"))
}
