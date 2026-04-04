// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
)

// --- Domain types for authoring stage outputs ---

// SparkOutput captures the initial idea and early scoping from the spark stage.
type SparkOutput struct {
	Seed       string   `json:"seed,omitempty"`
	Signal     string   `json:"signal,omitempty"`
	Questions  []string `json:"questions,omitempty"`
	ScopeSniff string   `json:"scope_sniff,omitempty"`
	KillTest   string   `json:"kill_test,omitempty"`
}

// Approach describes one candidate implementation strategy for a spec.
type Approach struct {
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Tradeoffs   []string `json:"tradeoffs,omitempty"`
}

// DecisionInput captures a design decision made during spec shaping.
type DecisionInput struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Body      string `json:"decision"`
	Rationale string `json:"rationale"`
}

// ShapeOutput captures scope boundaries, approaches, and success criteria.
type ShapeOutput struct {
	ScopeIn        []string        `json:"scope_in,omitempty"`
	ScopeOut       []string        `json:"scope_out,omitempty"`
	Approaches     []Approach      `json:"approaches,omitempty"`
	ChosenApproach string          `json:"chosen_approach,omitempty"`
	Risks          []string        `json:"risks,omitempty"`
	SuccessMust    []string        `json:"success_must,omitempty"`
	SuccessShould  []string        `json:"success_should,omitempty"`
	SuccessWont    []string        `json:"success_wont,omitempty"`
	Decisions      []DecisionInput `json:"decisions,omitempty"`
}

// InterfaceSection defines one API surface in the specify stage contract.
type InterfaceSection struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

// VerifyCriterion defines one testable acceptance criterion with a category.
type VerifyCriterion struct {
	Category    string `json:"category"`
	Description string `json:"description"`
}

// FileTouch identifies a file expected to be created or modified by this spec.
type FileTouch struct {
	Path       string `json:"path"`
	Purpose    string `json:"purpose"`
	ChangeType string `json:"change_type"`
}

// SpecifyOutput captures the precise contract and verification criteria.
type SpecifyOutput struct {
	Interfaces     []InterfaceSection `json:"interfaces,omitempty"`
	VerifyCriteria []VerifyCriterion  `json:"verify_criteria,omitempty"`
	Invariants     []string           `json:"invariants,omitempty"`
	Touches        []FileTouch        `json:"touches,omitempty"`
}

// DecompositionStrategy identifies how a spec is broken into slices.
type DecompositionStrategy string

// Decomposition strategy values.
const (
	StrategyVerticalSlice DecompositionStrategy = "vertical_slice"
	StrategyLayerCake     DecompositionStrategy = "layer_cake"
	StrategySingleUnit    DecompositionStrategy = "single_unit"
	StrategySteelThread   DecompositionStrategy = "steel_thread"
)

// IsValid reports whether s is a known DecompositionStrategy value.
func (s DecompositionStrategy) IsValid() bool {
	switch s {
	case StrategyVerticalSlice, StrategyLayerCake, StrategySingleUnit, StrategySteelThread:
		return true
	default:
		return false
	}
}

// DecomposeSlice represents one independently deliverable unit of work.
type DecomposeSlice struct {
	ID        string   `json:"id,omitempty"`
	Intent    string   `json:"intent,omitempty"`
	Verify    []string `json:"verify,omitempty"`
	Touches   []string `json:"touches,omitempty"`
	DependsOn []string `json:"depends_on,omitempty"`
}

// DecomposeOutput captures the decomposition strategy and resulting slices.
// Slices carries the full input data used by StoreDecomposeOutput to create
// Slice graph nodes. SliceSlugs is populated after creation and stored as the
// parent spec's decompose_output JSON (Slices data lives in the Slice nodes).
type DecomposeOutput struct {
	Strategy   DecompositionStrategy `json:"strategy,omitempty"`
	Slices     []DecomposeSlice      `json:"slices,omitempty"`      // input: full slice data for creation
	SliceSlugs []string              `json:"slice_slugs,omitempty"` // stored output: slug references to Slice nodes
}

// FindingSeverity indicates how severe a finding is.
type FindingSeverity string

// Finding severity levels.
const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityNote     FindingSeverity = "note"
)

// SafetyCategory is the type for safety flag categories.
type SafetyCategory string

// SafetyFlag marks a concern that may indicate harmful or risky content.
type SafetyFlag struct {
	Category    SafetyCategory  `json:"category,omitempty"`
	Severity    FindingSeverity `json:"severity,omitempty"`
	Description string          `json:"description,omitempty"`
}

// AmendResult holds the minimal fields returned after amending a spec.
type AmendResult struct {
	Slug    string
	Stage   SpecStage
	Version int32
}

// StageWriter handles stage transitions and output storage.
type StageWriter interface {
	TransitionStage(ctx context.Context, slug string, from, to SpecStage) error
	StoreSparkOutput(ctx context.Context, slug string, output *SparkOutput) error
	StoreShapeOutput(ctx context.Context, slug string, output *ShapeOutput) error
	StoreSpecifyOutput(ctx context.Context, slug string, output *SpecifyOutput) error
	StoreDecomposeOutput(ctx context.Context, slug string, output *DecomposeOutput) ([]string, error)
	// StoreSafetyFlags runs inline during stage transitions (not as a separate
	// analytical pass), so it belongs in StageWriter rather than a pass-specific interface.
	StoreSafetyFlags(ctx context.Context, slug string, flags []SafetyFlag) error
}

// AuthoringSpecLifecycle handles authoring-level spec amendments and supersession.
// For lifecycle-level operations (done→amended, superseded edges, abandon),
// see LifecycleBackend in lifecycle.go.
type AuthoringSpecLifecycle interface {
	SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error
	AmendSpec(ctx context.Context, slug, reason string, targetStage SpecStage) (*AmendResult, error)
}

// AuthoringBackend composes all authoring storage operations.
// Implementations satisfy all sub-interfaces.
// All methods accept domain types defined in this package, not protobuf types.
type AuthoringBackend interface {
	StageWriter
	AuthoringSpecLifecycle
}
