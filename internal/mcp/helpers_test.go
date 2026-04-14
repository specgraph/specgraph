// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"fmt"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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

func TestIntParam_Overflow(t *testing.T) {
	require.Equal(t, int32(0), int32Param(map[string]any{"v": float64(3e10)}, "v"))
}

func TestIntParam_Fractional(t *testing.T) {
	require.Equal(t, int32(0), int32Param(map[string]any{"v": 3.14}, "v"))
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

func TestStringPtrParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		key    string
		want   *string
	}{
		{
			name:   "present non-empty",
			params: map[string]any{"note": "hello"},
			key:    "note",
			want:   strPtr("hello"),
		},
		{
			name:   "missing key",
			params: map[string]any{},
			key:    "note",
			want:   nil,
		},
		{
			name:   "empty string",
			params: map[string]any{"note": ""},
			key:    "note",
			want:   nil,
		},
		{
			name:   "wrong type",
			params: map[string]any{"note": 42},
			key:    "note",
			want:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringPtrParam(tt.params, tt.key)
			if tt.want == nil {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
				require.Equal(t, *tt.want, *got)
			}
		})
	}
}

// strPtr is a test helper that returns a pointer to s.
func strPtr(s string) *string { return &s }

func TestStringSliceParam(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]any
		key    string
		want   []string
	}{
		{
			name:   "normal slice",
			params: map[string]any{"tags": []any{"a", "b", "c"}},
			key:    "tags",
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "missing key",
			params: map[string]any{},
			key:    "tags",
			want:   nil,
		},
		{
			name:   "wrong type (string, not slice)",
			params: map[string]any{"tags": "foo"},
			key:    "tags",
			want:   nil,
		},
		{
			name:   "empty slice",
			params: map[string]any{"tags": []any{}},
			key:    "tags",
			want:   []string{},
		},
		{
			name:   "mixed types — non-strings skipped",
			params: map[string]any{"tags": []any{"x", 42, "y"}},
			key:    "tags",
			want:   []string{"x", "y"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringSliceParam(tt.params, tt.key)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestJsonResult(t *testing.T) {
	msg := &specv1.HealthResponse{Status: "ok"}
	r := jsonResult(msg)
	require.NotNil(t, r)
	require.False(t, r.IsError)
	require.Len(t, r.Content, 1)
	require.Equal(t, "text", r.Content[0].Type)
	require.Contains(t, r.Content[0].Text, "ok")
}

func TestConnectErrResult(t *testing.T) {
	t.Run("nil error returns nil nil", func(t *testing.T) {
		result, err := connectErrResult(nil)
		require.NoError(t, err)
		require.Nil(t, result)
	})

	t.Run("unauthenticated returns Go error", func(t *testing.T) {
		connectErr := connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("token expired"))
		result, err := connectErrResult(connectErr)
		require.Error(t, err)
		require.Nil(t, result)
		require.Contains(t, err.Error(), "authentication required")
	})

	t.Run("not found returns tool error result", func(t *testing.T) {
		connectErr := connect.NewError(connect.CodeNotFound, fmt.Errorf("missing"))
		result, err := connectErrResult(connectErr)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsError)
		require.Contains(t, result.Content[0].Text, "missing")
	})

	t.Run("aborted returns concurrent modification message", func(t *testing.T) {
		connectErr := connect.NewError(connect.CodeAborted, fmt.Errorf("version conflict"))
		result, err := connectErrResult(connectErr)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsError)
		require.Contains(t, result.Content[0].Text, "concurrent modification")
	})

	t.Run("not-modified error returns 'not modified'", func(t *testing.T) {
		notModified := connect.NewNotModifiedError(http.Header{})
		result, err := connectErrResult(notModified)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsError)
		require.Contains(t, result.Content[0].Text, "not modified")
	})

	t.Run("internal error returns message text", func(t *testing.T) {
		connectErr := connect.NewError(connect.CodeInternal, fmt.Errorf("database unavailable"))
		result, err := connectErrResult(connectErr)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.True(t, result.IsError)
		require.Contains(t, result.Content[0].Text, "database unavailable")
	})
}

func TestStringProp(t *testing.T) {
	t.Run("no enum", func(t *testing.T) {
		p := stringProp("a slug identifier")
		require.Equal(t, "string", p["type"])
		require.Equal(t, "a slug identifier", p["description"])
		require.Nil(t, p["enum"])
	})

	t.Run("with enum values", func(t *testing.T) {
		p := stringProp("an action", "get", "list", "delete")
		require.Equal(t, "string", p["type"])
		enum, ok := p["enum"].([]string)
		require.True(t, ok)
		require.Equal(t, []string{"get", "list", "delete"}, enum)
	})
}

func TestIntProp(t *testing.T) {
	p := intProp("page size")
	require.Equal(t, "integer", p["type"])
	require.Equal(t, "page size", p["description"])
}

func TestBoolProp(t *testing.T) {
	p := boolProp("include archived")
	require.Equal(t, "boolean", p["type"])
	require.Equal(t, "include archived", p["description"])
}

func TestArrayProp(t *testing.T) {
	items := stringProp("a tag")
	p := arrayProp("list of tags", items)
	require.Equal(t, "array", p["type"])
	require.Equal(t, "list of tags", p["description"])
	require.Equal(t, items, p["items"])
}
