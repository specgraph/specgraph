// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package diff provides text diffing utilities for comparing spec versions.
// It wraps the sergi/go-diff diffmatchpatch library and exposes a simplified
// Hunk-based API with inline formatting support.
package diff

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// Op represents the type of a diff operation.
type Op int

const (
	// OpEqual indicates unchanged text.
	OpEqual Op = 0
	// OpInsert indicates text that was added.
	OpInsert Op = 1
	// OpDelete indicates text that was removed.
	OpDelete Op = 2
)

// Hunk is a single segment of a diff with an operation type and the associated text.
type Hunk struct {
	Op   Op
	Text string
}

// ComputeHunks computes the semantic diff between oldText and newText, returning
// a slice of Hunks. Returns nil when both inputs are empty.
func ComputeHunks(oldText, newText string) []Hunk {
	if oldText == "" && newText == "" {
		return nil
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldText, newText, false)
	dmp.DiffCleanupSemantic(diffs)

	hunks := make([]Hunk, 0, len(diffs))
	for _, d := range diffs {
		var op Op
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			op = OpInsert
		case diffmatchpatch.DiffDelete:
			op = OpDelete
		default:
			op = OpEqual
		}
		hunks = append(hunks, Hunk{Op: op, Text: d.Text})
	}
	return hunks
}

// FormatInline renders a slice of Hunks as an inline diff string using
// [-deleted-] and {+inserted+} markers for deleted and inserted text respectively.
// Equal text is rendered as-is.
func FormatInline(hunks []Hunk) string {
	var b strings.Builder
	for _, h := range hunks {
		switch h.Op {
		case OpDelete:
			b.WriteString("[-")
			b.WriteString(h.Text)
			b.WriteString("-]")
		case OpInsert:
			b.WriteString("{+")
			b.WriteString(h.Text)
			b.WriteString("+}")
		default:
			b.WriteString(h.Text)
		}
	}
	return b.String()
}
