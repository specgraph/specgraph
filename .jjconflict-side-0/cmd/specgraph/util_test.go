// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadJSONFile_NonexistentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.json")
	err := loadJSONFile(path, &specv1.Spec{})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "read") || strings.Contains(err.Error(), "no such file"),
		"expected error about reading file, got: %s", err.Error())
}

func TestLoadJSONFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.json")
	require.NoError(t, os.WriteFile(path, []byte(`{not valid json`), 0o600))

	err := loadJSONFile(path, &specv1.Spec{})
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "parse"),
		"expected error about parsing, got: %s", err.Error())
}

func TestLoadJSONFile_ValidProtoJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")

	const jsonContent = `{"slug":"test-slug","intent":"test intent","priority":"p1"}`
	require.NoError(t, os.WriteFile(path, []byte(jsonContent), 0o600))

	msg := &specv1.Spec{}
	err := loadJSONFile(path, msg)
	require.NoError(t, err)
	assert.Equal(t, "test-slug", msg.GetSlug())
	assert.Equal(t, "test intent", msg.GetIntent())
	assert.Equal(t, "p1", msg.GetPriority())
}

func TestLoadJSONFile_UnknownFieldErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.json")

	// DiscardUnknown: false means unknown fields should cause an error.
	const jsonContent = `{"slug":"test-slug","unknown_field":"should-error"}`
	require.NoError(t, os.WriteFile(path, []byte(jsonContent), 0o600))

	err := loadJSONFile(path, &specv1.Spec{})
	require.Error(t, err, "expected error for unknown JSON field when DiscardUnknown is false")
}
