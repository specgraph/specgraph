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

func TestSyncStatusCmd_Flags(t *testing.T) {
	require.NotNil(t, syncStatusCmd)
	assert.Equal(t, "status", syncStatusCmd.Use)

	f := syncStatusCmd.Flags()
	assert.NotNil(t, f.Lookup("adapter"))
	assert.NotNil(t, f.Lookup("spec"))
}
