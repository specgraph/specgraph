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

// RejectedAlt holds a rejected alternative for content hashing.
// Defined here to avoid importing the storage package.
type RejectedAlt struct {
	Option string
	Reason string
}

// Decision computes a Murmur3-128 content hash for a decision's substantive fields.
func Decision(title, status, decision, rationale, question, confidence, scope string, tags []string, rejectedAlts []RejectedAlt) string {
	h := murmur3.New128()
	writeField(h, "title", title)
	writeField(h, "status", status)
	writeField(h, "decision", decision)
	writeField(h, "rationale", rationale)
	writeField(h, "question", question)
	writeField(h, "confidence", confidence)
	writeField(h, "scope", scope)

	// Sort tags for deterministic ordering.
	sorted := make([]string, len(tags))
	copy(sorted, tags)
	sort.Strings(sorted)
	for _, tag := range sorted {
		writeField(h, "tag", tag)
	}

	// Sort rejected alternatives by Option for deterministic ordering.
	alts := make([]RejectedAlt, len(rejectedAlts))
	copy(alts, rejectedAlts)
	sort.Slice(alts, func(i, j int) bool { return alts[i].Option < alts[j].Option })
	for _, alt := range alts {
		writeField(h, "rejected_alt_option", alt.Option)
		writeField(h, "rejected_alt_reason", alt.Reason)
	}

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
