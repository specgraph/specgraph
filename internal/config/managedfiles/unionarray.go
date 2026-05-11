// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"errors"
	"fmt"
)

// unionPluginArray takes the existing on-disk JSON and the
// already-canonicalized merge output, and returns the canonical JSON
// with `plugin` rewritten to the union of {canonical[plugin],
// existing[plugin]} — canonical entries first, then existing-only
// entries in their original order. Used by jsonKeyMergeStrategy as a
// post-merge step for opencode.json so user-added plugin entries
// survive RFC 7396's array-replace semantics.
//
// If canonical has no plugin field, canonical is returned unchanged
// (no-op). This handles the case where the build function does not
// yet emit a plugin key.
func unionPluginArray(existing, canonical []byte) ([]byte, error) {
	canonPlugins, err := readPluginArray(canonical)
	if errors.Is(err, errNoPluginField) {
		// Canonical emits no plugin array; nothing to union.
		return canonical, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read canonical plugin array: %w", err)
	}
	existingPlugins, err := readPluginArray(existing)
	if err != nil {
		// Missing plugin field on existing is expected (first init,
		// or user removed it); just return canonical unchanged.
		return canonical, nil
	}

	seen := make(map[string]bool, len(canonPlugins))
	out := make([]string, 0, len(canonPlugins)+len(existingPlugins))
	for _, p := range canonPlugins {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for _, p := range existingPlugins {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	var doc map[string]any
	if err := json.Unmarshal(canonical, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal canonical: %w", err)
	}
	doc["plugin"] = out
	marshaled, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal merged doc: %w", err)
	}
	return canonicalize(marshaled)
}

// errNoPluginField is returned by readPluginArray when the JSON has
// no `plugin` field. Callers (unionPluginArray) treat absence as
// "nothing to union", not an error.
var errNoPluginField = errors.New("no plugin field")

// readPluginArray reads the `plugin` field as a []string.
func readPluginArray(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty input")
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	raw, ok := doc["plugin"]
	if !ok {
		return nil, errNoPluginField
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("plugin is not an array")
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("plugin entry is not a string: %v", v)
		}
		out = append(out, s)
	}
	return out, nil
}
