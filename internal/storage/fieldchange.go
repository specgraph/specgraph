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
func ComputeFieldDeltas(old, new_ SpecFields) []FieldChange {
	var deltas []FieldChange
	pairs := []struct {
		field  string
		oldVal string
		newVal string
	}{
		{"intent", old.Intent, new_.Intent},
		{"stage", old.Stage, new_.Stage},
		{"priority", old.Priority, new_.Priority},
		{"complexity", old.Complexity, new_.Complexity},
		{"spark_output", old.SparkOutput, new_.SparkOutput},
		{"shape_output", old.ShapeOutput, new_.ShapeOutput},
		{"specify_output", old.SpecifyOutput, new_.SpecifyOutput},
		{"decompose_output", old.DecomposeOutput, new_.DecomposeOutput},
	}
	for _, p := range pairs {
		if p.oldVal != p.newVal {
			deltas = append(deltas, FieldChange{Field: p.field, OldValue: p.oldVal, NewValue: p.newVal})
		}
	}
	return deltas
}
