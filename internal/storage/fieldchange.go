// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

// FieldChange records a single field-level change in a spec mutation.
type FieldChange struct {
	Field    string
	OldValue string
	NewValue string
}

// SpecFields holds the substantive fields of a spec for delta computation.
// These are the fields included in the content hash.
type SpecFields struct {
	Intent          string
	Stage           string
	Priority        string
	Complexity      string
	SparkOutput     string
	ShapeOutput     string
	SpecifyOutput   string
	DecomposeOutput string
}

// ComputeFieldDeltas compares two SpecFields and returns a slice of FieldChange
// for every field that differs. Only fields where the value changed are included.
func ComputeFieldDeltas(old, updated *SpecFields) []FieldChange {
	var deltas []FieldChange
	pairs := []struct {
		field  string
		oldVal string
		newVal string
	}{
		{"intent", old.Intent, updated.Intent},
		{"stage", old.Stage, updated.Stage},
		{"priority", old.Priority, updated.Priority},
		{"complexity", old.Complexity, updated.Complexity},
		{"spark_output", old.SparkOutput, updated.SparkOutput},
		{"shape_output", old.ShapeOutput, updated.ShapeOutput},
		{"specify_output", old.SpecifyOutput, updated.SpecifyOutput},
		{"decompose_output", old.DecomposeOutput, updated.DecomposeOutput},
	}
	for _, p := range pairs {
		if p.oldVal != p.newVal {
			deltas = append(deltas, FieldChange{Field: p.field, OldValue: p.oldVal, NewValue: p.newVal})
		}
	}
	return deltas
}
