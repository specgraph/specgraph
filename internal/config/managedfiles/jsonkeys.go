// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"fmt"
	"strings"
)

// JSONKeyMode controls how a JSONKeyMerge strategy treats a managed key.
type JSONKeyMode int

const (
	// KeyManagedValue overwrites the key on every init. For object-valued
	// keys, JSON Merge Patch (RFC 7396) recursively merges; for scalar and
	// array values, the canonical value replaces whatever is on disk.
	KeyManagedValue JSONKeyMode = iota

	// KeyManagedPresence ensures the key exists. On first init the value
	// is written; on subsequent inits an existing value is preserved.
	// Useful for keys whose presence we want to guarantee but whose value
	// belongs to the user (e.g. enabledPlugins entries toggled via
	// /plugin disable).
	KeyManagedPresence

	// KeyManagedArrayUnion treats the key as an array. Canonical elements
	// are unioned with existing elements (set-union by reflect.DeepEqual;
	// duplicates collapse). Formalizes the unionPluginArray hook in
	// jsonkeymerge.go.
	KeyManagedArrayUnion
)

// JSONManagedKey is one managed key inside a JSONKeyMerge file.
type JSONManagedKey struct {
	// Path is a JSON Pointer (RFC 6901) addressing the key. Use
	// slash-separated segments; ~ and / inside a key are escaped as
	// ~0 and ~1 respectively.
	Path string
	// Mode is one of KeyManagedValue, KeyManagedPresence,
	// KeyManagedArrayUnion.
	Mode JSONKeyMode
	// Value computes the canonical value at init time. For static values,
	// the closure ignores ProjectParams.
	Value func(ProjectParams) (any, error)
}

// jsonPointerSet sets the value at `pointer` in `doc`, creating
// intermediate objects as needed. Implements RFC 6901 unescaping.
// `doc` must be a map[string]any (or nested maps); array indices are
// not supported because the framework only addresses object keys.
func jsonPointerSet(doc map[string]any, pointer string, value any) error {
	tokens, err := jsonPointerTokens(pointer)
	if err != nil {
		return err
	}
	if len(tokens) == 0 {
		return fmt.Errorf("jsonPointerSet: empty pointer")
	}
	cur := doc
	for i, tok := range tokens {
		if i == len(tokens)-1 {
			cur[tok] = value
			return nil
		}
		next, ok := cur[tok]
		if !ok {
			fresh := map[string]any{}
			cur[tok] = fresh
			cur = fresh
			continue
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("jsonPointerSet: %s segment %q is non-object %T", pointer, tok, next)
		}
		cur = nextMap
	}
	return nil
}

// jsonPointerGet returns the value at `pointer` in `doc` and a presence flag.
func jsonPointerGet(doc map[string]any, pointer string) (any, bool) {
	tokens, err := jsonPointerTokens(pointer)
	if err != nil || len(tokens) == 0 {
		return nil, false
	}
	var cur any = doc
	for _, tok := range tokens {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[tok]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

// jsonPointerTokens splits an RFC 6901 JSON Pointer into segments and
// unescapes ~1 → / and ~0 → ~ (order matters per the RFC: unescape ~1
// before ~0 to avoid double-unescaping).
func jsonPointerTokens(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("jsonPointerTokens: %q must start with /", pointer)
	}
	parts := strings.Split(pointer[1:], "/")
	for i, p := range parts {
		p = strings.ReplaceAll(p, "~1", "/")
		p = strings.ReplaceAll(p, "~0", "~")
		parts[i] = p
	}
	return parts, nil
}
