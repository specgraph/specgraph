// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// stringPtrParam extracts an optional string parameter, returning nil if absent or empty.
// Used to populate proto optional (oneof) *string fields that should only be set when provided.
func stringPtrParam(params map[string]any, key string) *string {
	v, ok := params[key].(string)
	if !ok || v == "" {
		return nil
	}
	return proto.String(v)
}

// props is shorthand for JSON Schema property maps.
type props = map[string]any

// stringParam extracts a string parameter, returning "" if absent or wrong type.
func stringParam(params map[string]any, key string) string {
	v, ok := params[key].(string)
	if !ok {
		return ""
	}
	return v
}

// int32Param extracts an int32 parameter. JSON numbers arrive as float64.
func int32Param(params map[string]any, key string) int32 {
	switch v := params[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v) //nolint:gosec // internal callers always pass bounded API page sizes
	default:
		return 0
	}
}

// boolParam extracts a bool parameter, returning false if absent.
func boolParam(params map[string]any, key string) bool {
	v, ok := params[key].(bool)
	if !ok {
		return false
	}
	return v
}

// stringSliceParam extracts a []string parameter from a JSON array.
func stringSliceParam(params map[string]any, key string) []string {
	raw, ok := params[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// jsonResult marshals a proto message to JSON and returns it as a text result.
func jsonResult(msg proto.Message) *ToolResult {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return errResult(fmt.Sprintf("marshal response: %v", err))
	}
	return textResult(string(data))
}

// textResult wraps a string in a ToolResult.
func textResult(text string) *ToolResult {
	return &ToolResult{
		Content: []Content{{Type: "text", Text: text}},
	}
}

// errResult creates an error ToolResult from a message string.
func errResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []Content{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// connectErrResult maps a ConnectRPC error to an MCP tool error result.
// Auth errors return a Go error (protocol-level); everything else is a tool result.
func connectErrResult(err error) (*ToolResult, error) {
	if err == nil {
		return nil, nil
	}
	code := connect.CodeOf(err)
	if code == connect.CodeUnauthenticated {
		return nil, fmt.Errorf("authentication required: %w", err)
	}
	msg := connect.CodeOf(err).String()
	if connect.IsNotModifiedError(err) {
		msg = "not modified"
	} else {
		var ce *connect.Error
		if errors.As(err, &ce) {
			msg = ce.Message()
		}
	}
	if code == connect.CodeAborted {
		msg = "concurrent modification — retry the operation"
	}
	return errResult(msg), nil
}

// --- Schema builders ---

// objectSchema builds a JSON Schema object with the given properties and required fields.
func objectSchema(properties props, required ...string) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// stringProp builds a JSON Schema string property. Optional enum values constrain it.
func stringProp(desc string, enum ...string) map[string]any {
	p := map[string]any{"type": "string", "description": desc}
	if len(enum) > 0 {
		p["enum"] = enum
	}
	return p
}

// intProp builds a JSON Schema integer property.
func intProp(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

// boolProp builds a JSON Schema boolean property.
//
//nolint:unused // used by tool handler files added in subsequent tasks
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

// arrayProp builds a JSON Schema array property with the given item schema.
func arrayProp(desc string, items map[string]any) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": items}
}
