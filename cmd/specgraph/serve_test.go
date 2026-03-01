// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBootstrapBackend is a test double for storage.ConstitutionBackend.
type mockBootstrapBackend struct {
	constitution *specv1.Constitution
	updateCalled bool
	updatedWith  *specv1.Constitution
}

func (m *mockBootstrapBackend) GetConstitution(_ context.Context) (*specv1.Constitution, error) {
	if m.constitution != nil {
		return m.constitution, nil
	}

	return nil, storage.ErrConstitutionNotFound
}

func (m *mockBootstrapBackend) UpdateConstitution(_ context.Context, c *specv1.Constitution) (*specv1.Constitution, error) {
	m.updateCalled = true
	m.updatedWith = c

	return c, nil
}

func (m *mockBootstrapBackend) CheckViolation(_ context.Context, _ string) ([]*specv1.Violation, error) {
	return nil, nil
}

func TestBootstrapConstitution_AlreadyExists(t *testing.T) {
	mock := &mockBootstrapBackend{
		constitution: &specv1.Constitution{
			Name:  "existing",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		},
	}

	err := bootstrapConstitution(context.Background(), mock, "/nonexistent/path.yaml")
	require.NoError(t, err)
	assert.False(t, mock.updateCalled, "UpdateConstitution must not be called when constitution already exists")
}

func TestBootstrapConstitution_FileNotFound(t *testing.T) {
	mock := &mockBootstrapBackend{}

	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	err := bootstrapConstitution(context.Background(), mock, nonexistent)
	require.NoError(t, err)
	assert.False(t, mock.updateCalled)
}

func TestBootstrapConstitution_FileTooLarge(t *testing.T) {
	mock := &mockBootstrapBackend{}

	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.yaml")

	data := make([]byte, maxConstitutionSize+1)
	for i := range data {
		data[i] = 'x'
	}
	require.NoError(t, os.WriteFile(largePath, data, 0o600))

	err := bootstrapConstitution(context.Background(), mock, largePath)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "exceeds 1 MiB"), "expected 'exceeds 1 MiB' in error, got: %s", err.Error())
	assert.False(t, mock.updateCalled)
}

func TestBootstrapConstitution_ValidYAML(t *testing.T) {
	mock := &mockBootstrapBackend{}

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "constitution.yaml")

	const yamlContent = `name: test-project
layer: project
principles:
  - statement: Keep it simple
`
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlContent), 0o600))

	err := bootstrapConstitution(context.Background(), mock, yamlPath)
	require.NoError(t, err)

	assert.True(t, mock.updateCalled, "UpdateConstitution must be called for a valid YAML file")
	require.NotNil(t, mock.updatedWith)
	assert.Equal(t, "test-project", mock.updatedWith.GetName())
	assert.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT, mock.updatedWith.GetLayer())
	require.Len(t, mock.updatedWith.GetPrinciples(), 1)
	assert.Equal(t, "Keep it simple", mock.updatedWith.GetPrinciples()[0].GetStatement())
}

func TestBootstrapConstitution_MalformedYAML(t *testing.T) {
	mock := &mockBootstrapBackend{}

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "bad.yaml")

	const badContent = "name: test\nprinciples:\n\t- statement: bad tab\n"
	require.NoError(t, os.WriteFile(yamlPath, []byte(badContent), 0o600))

	err := bootstrapConstitution(context.Background(), mock, yamlPath)
	require.Error(t, err)
	assert.False(t, mock.updateCalled)
}

func TestBootstrapConstitution_StatError(t *testing.T) {
	mock := &mockBootstrapBackend{}

	dir := t.TempDir()
	restrictedDir := filepath.Join(dir, "restricted")
	require.NoError(t, os.Mkdir(restrictedDir, 0o750))

	require.NoError(t, os.Chmod(restrictedDir, 0o000))
	t.Cleanup(func() { _ = os.Chmod(restrictedDir, 0o600) })

	targetPath := filepath.Join(restrictedDir, "constitution.yaml")

	err := bootstrapConstitution(context.Background(), mock, targetPath)
	require.Error(t, err)
	assert.False(t, mock.updateCalled)
}
