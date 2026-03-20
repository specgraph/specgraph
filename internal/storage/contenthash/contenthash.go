// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package contenthash computes Murmur3-128 content hashes for specs and decisions.
package contenthash

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/spaolacci/murmur3"
)

// Spec computes a Murmur3-128 content hash for a spec's substantive fields.
// authoringOutputs is a map of property name to JSON string (e.g., "spark_output" to "...").
// Nil or empty authoringOutputs are treated the same (no outputs).
func Spec(intent, stage, priority, complexity string, authoringOutputs map[string]string) string {
	h := murmur3.New128()
	writeField(h, "intent", intent)
	writeField(h, "stage", stage)
	writeField(h, "priority", priority)
	writeField(h, "complexity", complexity)

	// Sort keys for deterministic ordering.
	keys := make([]string, 0, len(authoringOutputs))
	for k := range authoringOutputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		writeField(h, k, authoringOutputs[k])
	}

	hi, lo := h.Sum128()
	return fmt.Sprintf("%016x%016x", hi, lo)
}

// Decision computes a Murmur3-128 content hash for a decision's substantive fields.
func Decision(title, status, decision, rationale string) string {
	h := murmur3.New128()
	writeField(h, "title", title)
	writeField(h, "status", status)
	writeField(h, "decision", decision)
	writeField(h, "rationale", rationale)

	hi, lo := h.Sum128()
	return fmt.Sprintf("%016x%016x", hi, lo)
}

// writeField writes a length-prefixed key and value to the hasher.
// Length prefixing prevents "ab"+"c" from hashing the same as "a"+"bc".
// murmur3.Hash128 is an interface — do NOT use a pointer to it.
func writeField(h murmur3.Hash128, key, value string) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(len(key))) //nolint:gosec // key length will never overflow uint32
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(key))
	binary.BigEndian.PutUint32(buf[:], uint32(len(value))) //nolint:gosec // value length will never overflow uint32
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(value))
}
