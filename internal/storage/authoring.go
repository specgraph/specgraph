// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"errors"

	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
)

// ErrInvalidStageTransition is returned when a stage transition violates funnel rules.
var ErrInvalidStageTransition = errors.New("invalid stage transition")

// ErrSpecAlreadyApproved is returned when attempting to modify an already-approved spec.
var ErrSpecAlreadyApproved = errors.New("spec is already approved")

// AuthoringBackend defines storage operations for the authoring funnel.
type AuthoringBackend interface {
	TransitionStage(ctx context.Context, slug string, from, to string) error
	StoreSparkOutput(ctx context.Context, slug string, output *specv1.SparkOutput) error
	StoreShapeOutput(ctx context.Context, slug string, output *specv1.ShapeOutput) error
	StoreSpecifyOutput(ctx context.Context, slug string, output *specv1.SpecifyOutput) error
	StoreDecomposeOutput(ctx context.Context, slug string, output *specv1.DecomposeOutput) ([]*specv1.Spec, error)
	StoreRedTeamFindings(ctx context.Context, slug string, findings []*specv1.RedTeamFinding) error
	StorePeripheralVision(ctx context.Context, slug string, items []*specv1.PeripheralVisionItem) error
	StoreConsistencyIssues(ctx context.Context, slug string, issues []*specv1.ConsistencyIssue) error
	StoreSimplicityFindings(ctx context.Context, slug string, findings []*specv1.SimplicityFinding) error
	StoreSafetyFlags(ctx context.Context, slug string, flags []*specv1.SafetyFlag) error
	StoreConstitutionViolations(ctx context.Context, slug string, violations []*specv1.ConstitutionViolation) error
	SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error
	AmendSpec(ctx context.Context, slug, reason, targetStage string) (*specv1.Spec, error)
}
