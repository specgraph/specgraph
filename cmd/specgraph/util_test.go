// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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

// --- loadJSONFileRaw tests ---

func TestLoadJSONFileRaw_NonexistentPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	var v map[string]any
	err := loadJSONFileRaw(path, &v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read")
}

func TestLoadJSONFileRaw_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{nope`), 0o600))

	var v map[string]any
	err := loadJSONFileRaw(path, &v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestLoadJSONFileRaw_ValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "good.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"key":"value","num":42}`), 0o600))

	var v map[string]any
	err := loadJSONFileRaw(path, &v)
	require.NoError(t, err)
	assert.Equal(t, "value", v["key"])
	assert.Equal(t, float64(42), v["num"])
}

func TestLoadJSONFileRaw_EmptyObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))

	var v map[string]any
	err := loadJSONFileRaw(path, &v)
	require.NoError(t, err)
	assert.Empty(t, v)
}

// --- loadJSONFile boundary tests ---

func TestLoadJSONFile_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))

	err := loadJSONFile(path, &specv1.Spec{})
	require.Error(t, err, "empty file should fail protojson unmarshal")
}

func TestLoadJSONFile_NullJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "null.json")
	require.NoError(t, os.WriteFile(path, []byte("null"), 0o600))

	// protojson.Unmarshal rejects null as top-level value.
	err := loadJSONFile(path, &specv1.Spec{})
	require.Error(t, err)
}

func TestLoadJSONFile_EmptyObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "obj.json")
	require.NoError(t, os.WriteFile(path, []byte("{}"), 0o600))

	msg := &specv1.Spec{}
	err := loadJSONFile(path, msg)
	require.NoError(t, err)
	assert.Equal(t, "", msg.GetSlug())
}

// --- printSafetyFlags tests ---

func TestPrintSafetyFlags_Nil(t *testing.T) {
	require.NotPanics(t, func() { printSafetyFlags(nil) })
}

func TestPrintSafetyFlags_Empty(t *testing.T) {
	require.NotPanics(t, func() { printSafetyFlags([]*specv1.SafetyFlag{}) })
}

func TestPrintSafetyFlags_NilElement(t *testing.T) {
	require.NotPanics(t, func() { printSafetyFlags([]*specv1.SafetyFlag{nil}) })
}

func TestPrintSafetyFlags_NonEmpty(t *testing.T) {
	flags := []*specv1.SafetyFlag{
		{Severity: specv1.FindingSeverity_FINDING_SEVERITY_WARNING, Category: specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY, Description: "test flag"},
		{Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE, Category: specv1.SafetyCategory_SAFETY_CATEGORY_DATA_LOSS, Description: "another flag"},
	}
	require.NotPanics(t, func() { printSafetyFlags(flags) })
}

// --- Fuzz tests ---

func FuzzLoadJSONFile(f *testing.F) {
	f.Add([]byte(`{"slug":"test"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"slug":"","intent":""}`))
	f.Add([]byte(`not json at all`))
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{"slug":"\u0000"}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fuzz.json")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		// Must not panic regardless of input.
		msg := &specv1.Spec{}
		_ = loadJSONFile(path, msg)
	})
}

func FuzzLoadJSONFileRaw(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`not json`))
	f.Add([]byte{})
	f.Add([]byte(`null`))
	f.Add([]byte(`{"nested":{"a":1}}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fuzz.json")
		require.NoError(t, os.WriteFile(path, data, 0o600))

		// Must not panic regardless of input.
		var v any
		_ = loadJSONFileRaw(path, &v)
	})
}

// --- printJSON tests (complement output_test.go) ---

func TestPrintJSON_SpecMessage(t *testing.T) {
	var buf bytes.Buffer
	msg := &specv1.Spec{Slug: "test-spec", Intent: "test intent"}
	err := printJSON(&buf, msg)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "test-spec")
	assert.Contains(t, buf.String(), "test intent")
}
