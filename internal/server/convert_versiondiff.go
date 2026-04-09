// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/diff"
	"github.com/specgraph/specgraph/internal/storage"
)

func hunksToProto(hunks []diff.Hunk) []*specv1.InlineDiff {
	pbs := make([]*specv1.InlineDiff, len(hunks))
	for i, h := range hunks {
		var op specv1.InlineDiff_Op
		switch h.Op {
		case diff.OpEqual:
			op = specv1.InlineDiff_EQUAL
		case diff.OpInsert:
			op = specv1.InlineDiff_INSERT
		case diff.OpDelete:
			op = specv1.InlineDiff_DELETE
		}
		pbs[i] = &specv1.InlineDiff{
			Op:   op,
			Text: h.Text,
		}
	}
	return pbs
}

func computeVersionDiffs(from, to *storage.Spec) []*specv1.VersionDiff {
	pairs := []struct {
		field  string
		oldVal string
		newVal string
	}{
		{"intent", from.Intent, to.Intent},
		{"stage", string(from.Stage), string(to.Stage)},
		{"priority", string(from.Priority), string(to.Priority)},
		{"complexity", string(from.Complexity), string(to.Complexity)},
		{"superseded_by", from.SupersededBy, to.SupersededBy},
		{"supersedes", from.Supersedes, to.Supersedes},
		{"notes", from.Notes, to.Notes},
	}

	var diffs []*specv1.VersionDiff
	for _, p := range pairs {
		if p.oldVal == p.newVal {
			continue
		}
		hunks := diff.ComputeHunks(p.oldVal, p.newVal)
		diffs = append(diffs, &specv1.VersionDiff{
			Field:    p.field,
			OldValue: p.oldVal,
			NewValue: p.newVal,
			Hunks:    hunksToProto(hunks),
		})
	}
	return diffs
}
