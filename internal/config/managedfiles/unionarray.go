// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"errors"
	"fmt"
)

// unionPluginArray takes the existing on-disk JSON and the canonical
// merge output, and returns the canonical JSON with `plugin` rewritten
// to the union of {canonical[plugin], existing[plugin]} — canonical
// entries first, then existing-only entries in their original order.
// Used by jsonKeyMergeStrategy as a post-merge step for opencode.json
// so user-added plugin entries survive RFC 7396's array-replace
// semantics. The output is re-canonicalized internally; callers do not
// need to canonicalize again.
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
	existingPlugins, existErr := readPluginArray(existing)
	switch {
	case existErr == nil:
		// Happy path; fall through to union below.
	case errors.Is(existErr, errNoPluginField):
		// Missing plugin field on existing JSON, or empty input — nothing
		// to union with. Return canonical unchanged.
		return canonical, nil
	default:
		// Existing JSON has a plugin field but it's structurally wrong
		// (non-array, non-string entries, malformed JSON). Surface as an
		// error rather than silently dropping user-added entries — see
		// PR review feedback for the data-loss risk.
		return nil, fmt.Errorf("read existing plugin array: %w", existErr)
	}

	// Bound the plugin-array sizes to defuse the int-overflow path CodeQL
	// flags on the `len(a)+len(b)` capacity hint below (CWE-190). A
	// plugin array with >1M entries is never legitimate for the use
	// case; refuse rather than risk a wraparound on absurd input.
	const maxPlugins = 1 << 20
	if len(canonPlugins) > maxPlugins || len(existingPlugins) > maxPlugins {
		return nil, fmt.Errorf("plugin array too large: canon=%d existing=%d (max %d)",
			len(canonPlugins), len(existingPlugins), maxPlugins)
	}

	seen := make(map[string]bool, len(canonPlugins))
	union := make([]string, 0, len(canonPlugins)+len(existingPlugins))
	for _, p := range canonPlugins {
		if !seen[p] {
			seen[p] = true
			union = append(union, p)
		}
	}
	for _, p := range existingPlugins {
		if !seen[p] {
			seen[p] = true
			union = append(union, p)
		}
	}

	var doc map[string]any
	if unmarshalErr := json.Unmarshal(canonical, &doc); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal canonical: %w", unmarshalErr)
	}
	doc["plugin"] = union
	marshaled, marshalErr := json.Marshal(doc)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal merged doc: %w", marshalErr)
	}
	return canonicalize(marshaled)
}

// errNoPluginField is returned by readPluginArray when the JSON has
// no `plugin` field. Callers (unionPluginArray) treat absence as
// "nothing to union", not an error.
var errNoPluginField = errors.New("no plugin field")

// readPluginArray reads the `plugin` field as a []string. Returns
// errNoPluginField if the input is empty OR has no plugin field —
// the two cases are semantically equivalent for union-merge purposes
// ("no existing plugins to preserve"). Other parse failures (malformed
// JSON, non-array plugin, non-string entries) return distinct errors
// so callers can surface them instead of silently dropping content.
func readPluginArray(data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, errNoPluginField
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
