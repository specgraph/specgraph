// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringParam(t *testing.T) {
	params := map[string]any{"action": "get", "slug": "auth"}
	require.Equal(t, "get", stringParam(params, "action"))
	require.Equal(t, "auth", stringParam(params, "slug"))
	require.Equal(t, "", stringParam(params, "missing"))
}

func TestIntParam(t *testing.T) {
	params := map[string]any{"limit": float64(10)} // JSON numbers are float64
	require.Equal(t, int32(10), int32Param(params, "limit"))
	require.Equal(t, int32(0), int32Param(params, "missing"))
}

func TestBoolParam(t *testing.T) {
	params := map[string]any{"recursive": true}
	require.True(t, boolParam(params, "recursive"))
	require.False(t, boolParam(params, "missing"))
}

func TestTextResult(t *testing.T) {
	r := textResult("hello")
	require.Len(t, r.Content, 1)
	require.Equal(t, "text", r.Content[0].Type)
	require.Equal(t, "hello", r.Content[0].Text)
	require.False(t, r.IsError)
}

func TestErrorResultFromString(t *testing.T) {
	r := errResult("something broke")
	require.True(t, r.IsError)
	require.Contains(t, r.Content[0].Text, "something broke")
}

func TestObjectSchema(t *testing.T) {
	s := objectSchema(
		props{
			"action": stringProp("op", "get", "list"),
			"slug":   stringProp("identifier"),
		},
		"action",
	)
	require.Equal(t, "object", s["type"])
	p := s["properties"].(props)
	require.Contains(t, p, "action")
	req := s["required"].([]string)
	require.Equal(t, []string{"action"}, req)
}
