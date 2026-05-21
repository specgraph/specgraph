// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package storage

import "time"

// SpecStage represents a spec's stage, covering both authoring funnel stages
// (spark→shape→specify→decompose→approved→in_progress→review→done) and
// lifecycle terminal states (superseded, abandoned).
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
	SpecStageSuperseded SpecStage = "superseded"
	SpecStageAbandoned  SpecStage = "abandoned"
)

// ExcludesReEntry reports whether s is a stage that cannot be used as a re-entry
// target. Done, superseded, and abandoned specs cannot be re-entry targets.
func (s SpecStage) ExcludesReEntry() bool {
	switch s {
	case SpecStageDone, SpecStageSuperseded, SpecStageAbandoned:
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
	SpecStageSuperseded,
	SpecStageAbandoned,
}

// IsFullyTerminal reports whether s is a stage from which no further lifecycle
// transitions are allowed. Fully terminal stages cannot be superseded or abandoned.
func (s SpecStage) IsFullyTerminal() bool {
	switch s {
	case SpecStageSuperseded, SpecStageAbandoned:
		return true
	default:
		return false
	}
}

// FullyTerminalStages returns stages from which no lifecycle transitions are
// possible.
func FullyTerminalStages() []SpecStage {
	var out []SpecStage
	for _, s := range allSpecStages {
		if s.IsFullyTerminal() {
			out = append(out, s)
		}
	}
	return out
}

// IsAmendEligible reports whether s is a stage from which amend is allowed.
// Only execution-adjacent stages (approved, in_progress, review) qualify.
func (s SpecStage) IsAmendEligible() bool {
	switch s {
	case SpecStageApproved, SpecStageInProgress, SpecStageReview:
		return true
	default:
		return false
	}
}

// PrecedingAuthStage returns the authoring stage that immediately precedes s in
// the authoring funnel. For example, PrecedingAuthStage(shape) == spark because
// the shape authoring command transitions spark → shape.
// Returns s itself if s is the first authoring stage or not in the authoring sequence.
func (s SpecStage) PrecedingAuthStage() SpecStage {
	idx := stageIndex(s)
	if idx <= 0 {
		return s
	}
	return authoringStages[idx-1]
}

// IsValid reports whether s is a known spec stage.
func (s SpecStage) IsValid() bool {
	switch s {
	case SpecStageSpark, SpecStageShape, SpecStageSpecify,
		SpecStageDecompose, SpecStageApproved, SpecStageInProgress,
		SpecStageReview, SpecStageDone,
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

// SpecProvenanceType is the string-typed discriminator for how a spec
// entered the graph. Mirrors the SpecProvenance proto enum.
type SpecProvenanceType string

// Spec provenance discriminator values.
const (
	SpecProvenanceAuthored          SpecProvenanceType = "authored"
	SpecProvenanceRetroactiveFromPR SpecProvenanceType = "retroactive_from_pr"
	SpecProvenanceDeclared          SpecProvenanceType = "declared"
)

// IsValid reports whether p is a known spec provenance type.
func (p SpecProvenanceType) IsValid() bool {
	switch p {
	case SpecProvenanceAuthored, SpecProvenanceRetroactiveFromPR, SpecProvenanceDeclared:
		return true
	default:
		return false
	}
}

// SpecProvenanceDetail is the structured payload for non-AUTHORED specs.
// Exactly one of the embedded pointers is non-nil; both nil is valid (AUTHORED).
// The populated variant must match the Spec.Provenance discriminator —
// enforced at the server boundary with storage.ErrProvenanceMismatch.
type SpecProvenanceDetail struct {
	RetroactiveFromPR *RetroactivePRProvenance // populated when type == retroactive_from_pr
	Declared          *DeclaredProvenance      // populated when type == declared
}

// RetroactivePRProvenance carries PR metadata for retroactive-import specs.
type RetroactivePRProvenance struct {
	URL      string
	SHA      string
	MergedAt time.Time
	Title    string
}

// DeclaredProvenance carries declaration metadata for human-declared specs.
type DeclaredProvenance struct {
	DeclaredBy string
	Note       string
}

// SpecComplexity represents the complexity level of a spec.
type SpecComplexity string

// Spec complexity values.
const (
	SpecComplexityLow    SpecComplexity = "low"
	SpecComplexityMedium SpecComplexity = "medium"
	SpecComplexityHigh   SpecComplexity = "high"
)

// IsValid reports whether c is a known spec complexity.
func (c SpecComplexity) IsValid() bool {
	switch c {
	case SpecComplexityLow, SpecComplexityMedium, SpecComplexityHigh:
		return true
	default:
		return false
	}
}

// Spec is the storage-layer domain type for specifications.
// Handlers convert between this type and the proto Spec message.
type Spec struct {
	ID                string
	Slug              string
	Intent            string
	Stage             SpecStage
	Priority          SpecPriority
	Complexity        SpecComplexity
	Version           int32
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Provenance        SpecProvenanceType
	ProvenanceDetail  SpecProvenanceDetail
	SupersededBy      string                  // slug of replacement spec
	Supersedes        string                  // slug of spec this replaced
	Notes             string                  // free-text notes (conversation summaries, context)
	ContentHash       string                  // Murmur3-128 hash of substantive fields
	ConversationLogs  []*ConversationLogEntry // authoring conversation audit trail (populated by GetSpec)
	SparkOutput       *SparkOutput
	ShapeOutput       *ShapeOutput
	SpecifyOutput     *SpecifyOutput
	DecomposeOutput   *DecomposeOutput
	ConversationCount int // count of conversation log entries (populated by ListSpecs)
}
