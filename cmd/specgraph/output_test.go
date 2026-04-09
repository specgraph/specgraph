// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"bytes"
	"fmt"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errWriter is an io.Writer that always returns an error.
type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("write failed") }

func TestPrintJSON_ValidMessage(t *testing.T) {
	var buf bytes.Buffer
	err := printJSON(&buf, &specv1.Spec{Slug: "test"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "test")
}

func TestPrintJSON_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	err := printJSON(&buf, &specv1.Spec{})
	require.NoError(t, err)

	out := buf.String()
	// Should be valid JSON — at minimum an opening brace.
	assert.Contains(t, out, "{")
}

func TestPrintJSON_ErrorWriter(t *testing.T) {
	err := printJSON(errWriter{}, &specv1.Spec{Slug: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}
