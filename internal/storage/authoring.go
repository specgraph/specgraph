// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"
	"fmt"
)

// ErrInvalidStageTransition is returned when a stage transition violates funnel rules.
var ErrInvalidStageTransition = errors.New("invalid stage transition")

// ErrSpecAlreadyApproved is returned when attempting to modify an already-approved spec.
var ErrSpecAlreadyApproved = errors.New("spec is already approved")

// ErrSpecAlreadyExists is returned when creating a spec with a slug that already exists.
var ErrSpecAlreadyExists = errors.New("spec already exists")

// ErrSpecSuperseded is returned when attempting to amend a spec that has been superseded.
var ErrSpecSuperseded = errors.New("spec has been superseded and cannot be amended")

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

// ShapeOutput captures scope boundaries, approaches, and success criteria.
type ShapeOutput struct {
	ScopeIn        []string   `json:"scope_in,omitempty"`
	ScopeOut       []string   `json:"scope_out,omitempty"`
	Approaches     []Approach `json:"approaches,omitempty"`
	ChosenApproach string     `json:"chosen_approach,omitempty"`
	Risks          []string   `json:"risks,omitempty"`
	SuccessMust    []string   `json:"success_must,omitempty"`
	SuccessShould  []string   `json:"success_should,omitempty"`
	SuccessWont    []string   `json:"success_wont,omitempty"`
	Decisions      []string   `json:"decisions,omitempty"` // TODO(ADR-003): promote to Decision graph nodes
}

// SpecifyOutput captures the precise contract and verification criteria.
type SpecifyOutput struct {
	InterfaceContract string   `json:"interface_contract,omitempty"`
	VerifyCriteria    []string `json:"verify_criteria,omitempty"`
	Invariants        []string `json:"invariants,omitempty"`
	Touches           []string `json:"touches,omitempty"`
}

// DecompositionStrategy identifies how a spec is broken into slices.
type DecompositionStrategy string

// Decomposition strategy values.
const (
	StrategyVerticalSlice DecompositionStrategy = "vertical_slice"
	StrategyLayerCake     DecompositionStrategy = "layer_cake"
	StrategySingleUnit    DecompositionStrategy = "single_unit"
)

// validStrategies lists the accepted DecompositionStrategy values.
var validStrategies = map[DecompositionStrategy]bool{
	StrategyVerticalSlice: true,
	StrategyLayerCake:     true,
	StrategySingleUnit:    true,
}

// ValidateStrategy checks whether a DecompositionStrategy is a known value.
func ValidateStrategy(s DecompositionStrategy) error {
	if !validStrategies[s] {
		return fmt.Errorf("unknown decomposition strategy %q", s)
	}
	return nil
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
type DecomposeOutput struct {
	Strategy DecompositionStrategy `json:"strategy,omitempty"`
	Slices   []DecomposeSlice      `json:"slices,omitempty"`
}

// FindingSeverity indicates how severe a finding is.
type FindingSeverity string

// Finding severity levels.
const (
	SeverityCritical FindingSeverity = "critical"
	SeverityWarning  FindingSeverity = "warning"
	SeverityNote     FindingSeverity = "note"
)

// RedTeamFinding records an adversarial challenge to spec correctness or safety.
type RedTeamFinding struct {
	Severity   FindingSeverity `json:"severity,omitempty"`
	Finding    string          `json:"finding,omitempty"`
	Resolution string          `json:"resolution,omitempty"`
}

// PeripheralDisposition indicates how an out-of-scope concern should be handled.
type PeripheralDisposition string

// Peripheral disposition values.
const (
	DispositionAddedToSpec        PeripheralDisposition = "added_to_spec"
	DispositionSeparateSpec       PeripheralDisposition = "separate_spec"
	DispositionNoteForImplementer PeripheralDisposition = "note_for_implementer"
)

// PeripheralVisionItem captures a related concern noticed during authoring.
type PeripheralVisionItem struct {
	Item        string                `json:"item,omitempty"`
	Disposition PeripheralDisposition `json:"disposition,omitempty"`
}

// ConsistencyIssue records a conflict between specs in the graph.
type ConsistencyIssue struct {
	IssueKind     string   `json:"issue_kind,omitempty"`
	Description   string   `json:"description,omitempty"`
	AffectedSpecs []string `json:"affected_specs,omitempty"`
}

// SimplicityFinding highlights where spec or design can be simplified.
type SimplicityFinding struct {
	Area       string `json:"area,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
}

// SafetyCategory is the type for safety flag categories.
type SafetyCategory string

// SafetyFlag marks a concern that may indicate harmful or risky content.
type SafetyFlag struct {
	Category    SafetyCategory  `json:"category,omitempty"`
	Severity    FindingSeverity `json:"severity,omitempty"`
	Description string          `json:"description,omitempty"`
}

// ConstitutionViolation records a conflict with an active constitution constraint.
type ConstitutionViolation struct {
	Constraint string          `json:"constraint,omitempty"`
	Violation  string          `json:"violation,omitempty"`
	Severity   FindingSeverity `json:"severity,omitempty"`
}

// AuthoringStage is a typed stage name for use in storage method signatures.
type AuthoringStage string

// AmendResult holds the minimal fields returned after amending a spec.
type AmendResult struct {
	Slug    string
	Stage   AuthoringStage
	Version int32
}

// StageWriter handles stage transitions and output storage.
type StageWriter interface {
	TransitionStage(ctx context.Context, slug string, from, to AuthoringStage) error
	StoreSparkOutput(ctx context.Context, slug string, output *SparkOutput) error
	StoreShapeOutput(ctx context.Context, slug string, output *ShapeOutput) error
	StoreSpecifyOutput(ctx context.Context, slug string, output *SpecifyOutput) error
	StoreDecomposeOutput(ctx context.Context, slug string, output *DecomposeOutput) ([]string, error)
}

// PassWriter stores analytical pass results.
type PassWriter interface {
	StoreRedTeamFindings(ctx context.Context, slug string, findings []RedTeamFinding) error
	StorePeripheralVision(ctx context.Context, slug string, items []PeripheralVisionItem) error
	StoreConsistencyIssues(ctx context.Context, slug string, issues []ConsistencyIssue) error
	StoreSimplicityFindings(ctx context.Context, slug string, findings []SimplicityFinding) error
	StoreSafetyFlags(ctx context.Context, slug string, flags []SafetyFlag) error
	StoreConstitutionViolations(ctx context.Context, slug string, violations []ConstitutionViolation) error
}

// SpecLifecycle handles spec amendments and supersession.
type SpecLifecycle interface {
	SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error
	AmendSpec(ctx context.Context, slug, reason string, targetStage AuthoringStage) (*AmendResult, error)
}

// AuthoringBackend composes all authoring storage operations.
// Implementations satisfy all sub-interfaces.
// All methods accept domain types defined in this package, not protobuf types.
type AuthoringBackend interface {
	StageWriter
	PassWriter
	SpecLifecycle
}
