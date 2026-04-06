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
	Notes           string
	SparkOutput     string
	ShapeOutput     string
	SpecifyOutput   string
	DecomposeOutput string
}

// DecisionFields holds the substantive fields of a decision for delta computation.
type DecisionFields struct {
	Title                string
	Status               string
	Body                 string
	Rationale            string
	Question             string
	Confidence           string
	Scope                string
	Tags                 string // JSON string for comparison
	RejectedAlternatives string // JSON string for comparison
	OriginSpec           string
	OriginStage          string
}

// ComputeDecisionFieldDeltas compares two DecisionFields and returns a slice of
// FieldChange for every field that differs.
func ComputeDecisionFieldDeltas(old, updated *DecisionFields) []FieldChange {
	var deltas []FieldChange
	pairs := []struct {
		field  string
		oldVal string
		newVal string
	}{
		{"title", old.Title, updated.Title},
		{"status", old.Status, updated.Status},
		{"body", old.Body, updated.Body},
		{"rationale", old.Rationale, updated.Rationale},
		{"question", old.Question, updated.Question},
		{"confidence", old.Confidence, updated.Confidence},
		{"scope", old.Scope, updated.Scope},
		{"tags", old.Tags, updated.Tags},
		{"rejected_alternatives", old.RejectedAlternatives, updated.RejectedAlternatives},
		{"origin_spec", old.OriginSpec, updated.OriginSpec},
		{"origin_stage", old.OriginStage, updated.OriginStage},
	}
	for _, p := range pairs {
		if p.oldVal != p.newVal {
			deltas = append(deltas, FieldChange{Field: p.field, OldValue: p.oldVal, NewValue: p.newVal})
		}
	}
	return deltas
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
		{"notes", old.Notes, updated.Notes},
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
