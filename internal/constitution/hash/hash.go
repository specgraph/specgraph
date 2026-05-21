// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package hash computes canonical content hashes of constitution domain
// structs. Used by drift detection to determine whether a remote
// constitution has changed since it was last fetched.
//
// Hashing operates post-parse on *storage.Constitution rather than on
// raw YAML bytes. This makes the hash resilient to comments, whitespace,
// and key ordering in the source file, and avoids introducing a second
// YAML parser alongside the existing yaml.v3 loader.
//
// Determinism assumption: all map fields in *storage.Constitution today
// are map[string]string, which encoding/json sorts deterministically.
// Adding a non-string-keyed map would silently break determinism — the
// fixed-expected-hex test in this package's tests guards against
// regressions on existing fields.
package hash

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/spaolacci/murmur3"

	"github.com/specgraph/specgraph/internal/storage"
)

// Hash returns the Murmur3-128 hex digest of the canonical JSON
// serialization of c. Two semantically equivalent inputs produce equal
// hashes; any field-level content change produces a different hash.
//
// Returns an error only if json.Marshal fails (which should never
// happen for well-formed *storage.Constitution values).
func Hash(c *storage.Constitution) (string, error) {
	canonical, err := canonicalJSON(c)
	if err != nil {
		return "", err
	}
	h1, h2 := murmur3.Sum128(canonical)
	// Murmur3-128 returns two uint64 halves. Concatenate big-endian for
	// stable hex output across architectures.
	var sum [16]byte
	for i := 0; i < 8; i++ {
		sum[i] = byte(h1 >> (56 - 8*i))   //nolint:gosec // G115: safe byte extraction from uint64 bit-shift
		sum[i+8] = byte(h2 >> (56 - 8*i)) //nolint:gosec // G115: safe byte extraction from uint64 bit-shift
	}
	return hex.EncodeToString(sum[:]), nil
}

// canonicalJSON marshals c with deterministic key ordering.
// encoding/json sorts top-level and nested map[string]X keys
// alphabetically; struct fields emit in declaration order, which is
// stable per the Go spec.
func canonicalJSON(c *storage.Constitution) ([]byte, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	return b, nil
}
