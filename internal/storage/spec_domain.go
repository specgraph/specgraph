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
	Lifecycle    SpecLifecycle  // "task" (default) or "living"
	SupersededBy string         // slug of replacement spec
	Supersedes   string         // slug of spec this replaced
	History      []HistoryEntry // lifecycle event log
}

// HistoryEntry records a lifecycle event on a spec.
type HistoryEntry struct {
	Version int32
	Stage   SpecStage
	Summary string
	Reason  string
	Date    time.Time
}
