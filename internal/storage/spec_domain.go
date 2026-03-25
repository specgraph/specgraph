// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import "time"

// SpecStage represents a spec's stage, covering both authoring funnel stages
// (spark→shape→specify→decompose→approved→in_progress→review→done) and
// lifecycle terminal states (amended, superseded, abandoned).
type SpecStage string

// Spec stage lifecycle values.
const (
	SpecStageSpark      SpecStage = "spark"
	SpecStageShape      SpecStage = "shape"
	SpecStageSpecify    SpecStage = "specify"
	SpecStageDecompose  SpecStage = "decompose"
	SpecStageApproved   SpecStage = "approved"
	SpecStageInProgress SpecStage = "in_progress"
	SpecStageReview     SpecStage = "review"
	SpecStageDone       SpecStage = "done"
	SpecStageAmended    SpecStage = "amended"
	SpecStageSuperseded SpecStage = "superseded"
	SpecStageAbandoned  SpecStage = "abandoned"
)

// ExcludesReEntry reports whether s is a stage that cannot be used as a re-entry
// target. Amended specs cannot cycle back (semantically invalid), and
// superseded/abandoned specs are fully terminal states.
func (s SpecStage) ExcludesReEntry() bool {
	switch s {
	case SpecStageDone, SpecStageAmended, SpecStageSuperseded, SpecStageAbandoned:
		return true
	default:
		return false
	}
}

// allSpecStages lists every known SpecStage value. It is the single source of
// truth that FullyTerminalStages iterates over, so IsFullyTerminal and the
// storage layer never diverge.
var allSpecStages = []SpecStage{
	SpecStageSpark,
	SpecStageShape,
	SpecStageSpecify,
	SpecStageDecompose,
	SpecStageApproved,
	SpecStageInProgress,
	SpecStageReview,
	SpecStageDone,
	SpecStageAmended,
	SpecStageSuperseded,
	SpecStageAbandoned,
}

// IsFullyTerminal reports whether s is a stage from which no further lifecycle
// transitions are allowed. Unlike ExcludesReEntry (which also includes Amended),
// fully terminal stages cannot be superseded or abandoned.
func (s SpecStage) IsFullyTerminal() bool {
	switch s {
	case SpecStageSuperseded, SpecStageAbandoned:
		return true
	default:
		return false
	}
}

// FullyTerminalStages returns stages from which no lifecycle transitions are
// possible. This excludes Amended, which can still be superseded or abandoned.
func FullyTerminalStages() []SpecStage {
	var out []SpecStage
	for _, s := range allSpecStages {
		if s.IsFullyTerminal() {
			out = append(out, s)
		}
	}
	return out
}

// IsValid reports whether s is a known spec stage.
func (s SpecStage) IsValid() bool {
	switch s {
	case SpecStageSpark, SpecStageShape, SpecStageSpecify,
		SpecStageDecompose, SpecStageApproved, SpecStageInProgress,
		SpecStageReview, SpecStageDone, SpecStageAmended,
		SpecStageSuperseded, SpecStageAbandoned:
		return true
	default:
		return false
	}
}

// SpecPriority represents the priority level of a spec.
type SpecPriority string

// Spec priority values.
const (
	SpecPriorityP0 SpecPriority = "p0"
	SpecPriorityP1 SpecPriority = "p1"
	SpecPriorityP2 SpecPriority = "p2"
	SpecPriorityP3 SpecPriority = "p3"
)

// IsValid reports whether p is a known spec priority.
func (p SpecPriority) IsValid() bool {
	switch p {
	case SpecPriorityP0, SpecPriorityP1, SpecPriorityP2, SpecPriorityP3:
		return true
	default:
		return false
	}
}

// SpecLifecycle represents the lifecycle model of a spec.
type SpecLifecycle string

// Spec lifecycle model values.
const (
	SpecLifecycleTask   SpecLifecycle = "task"
	SpecLifecycleLiving SpecLifecycle = "living"
)

// IsValid reports whether l is a known spec lifecycle.
func (l SpecLifecycle) IsValid() bool {
	switch l {
	case SpecLifecycleTask, SpecLifecycleLiving:
		return true
	default:
		return false
	}
}

// Spec is the storage-layer domain type for specifications.
// Handlers convert between this type and the proto Spec message.
type Spec struct {
	ID           string
	Slug         string
	Intent       string
	Stage        SpecStage
	Priority     SpecPriority
	Complexity   string
	Version      int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Lifecycle    SpecLifecycle // "task" (default) or "living"
	SupersededBy string        // slug of replacement spec
	Supersedes   string        // slug of spec this replaced
	Notes        string        // free-text notes (conversation summaries, context)
	ContentHash      string                // Murmur3-128 hash of substantive fields
	ConversationLogs []*ConversationLogEntry // authoring conversation audit trail (populated by GetSpec)
	SparkOutput      *SparkOutput
	ShapeOutput      *ShapeOutput
	SpecifyOutput    *SpecifyOutput
	DecomposeOutput  *DecomposeOutput
}
