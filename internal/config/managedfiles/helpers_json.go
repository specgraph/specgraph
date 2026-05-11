// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"encoding/json"
	"fmt"
)

// canonicalize returns the JSON document re-marshaled with 2-space
// indent, alphabetical key order (via encoding/json), and a trailing
// newline. Used by jsonKeyMergeStrategy for the no-op short-circuit
// and the bytes written to disk. Verbatim port of
// mcpconfigs/sync.go:180-190.
func canonicalize(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal indent: %w", err)
	}
	return append(out, '\n'), nil
}
