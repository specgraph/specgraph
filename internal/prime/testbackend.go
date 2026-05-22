// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package prime

import (
	"context"
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// StubBackend is a hand-written test double that satisfies the prime
// Backend interface. Each storage method is backed by an optional
// function field; nil function fields return zero values (or nil
// errors). Exported so handler and MCP tests in other packages can
// reuse it.
type StubBackend struct {
	// Constitution.
	GetMergedConstitutionFn func(ctx context.Context) (*storage.MergedResult, error)
	GetConstitutionLayerFn  func(ctx context.Context, layer storage.ConstitutionLayer) (*storage.Constitution, error)
	GetAllLayersFn          func(ctx context.Context) ([]*storage.Constitution, error)
	UpdateConstitutionFn    func(ctx context.Context, c *storage.Constitution) (*storage.Constitution, error)

	// Specs.
	CreateSpecFn func(ctx context.Context, slug, intent, priority, complexity string,
		provenance storage.SpecProvenanceType, detail storage.SpecProvenanceDetail,
		spark *storage.SparkOutput, shape *storage.ShapeOutput,
		specify *storage.SpecifyOutput, decompose *storage.DecomposeOutput) (*storage.Spec, error)
	GetSpecFn    func(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecsFn  func(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	UpdateSpecFn func(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error)
	CloseFn      func(ctx context.Context) error

	// Graph.
	AddEdgeFn                     func(ctx context.Context, fromSlug, toSlug string, t storage.EdgeType) (*storage.Edge, error)
	RemoveEdgeFn                  func(ctx context.Context, fromSlug, toSlug string, t storage.EdgeType) error
	ListEdgesFn                   func(ctx context.Context, slug string, t storage.EdgeType) ([]*storage.Edge, error)
	GetDependenciesFn             func(ctx context.Context, slug string) ([]storage.NodeRef, error)
	GetTransitiveDepsFn           func(ctx context.Context, slug string) ([]storage.NodeRef, error)
	GetImpactFn                   func(ctx context.Context, slug string) ([]storage.NodeRef, error)
	GetReadyFn                    func(ctx context.Context) ([]storage.NodeRef, error)
	GetCriticalPathFn             func(ctx context.Context, slug string) ([]storage.NodeRef, error)
	GetDependenciesWithEdgeDataFn func(ctx context.Context, slug string) ([]storage.DependencyRef, error)
	RefreshDependencyHashesFn     func(ctx context.Context, slug string) error
	GetFullGraphFn                func(ctx context.Context) (*storage.FullGraph, error)

	// Findings.
	StoreFindingsFn   func(ctx context.Context, slug string, pt storage.PassType, in []storage.AnalyticalFindingInput) ([]string, error)
	ListFindingsFn    func(ctx context.Context, slug string, pt storage.PassType) ([]storage.AnalyticalFinding, error)
	ListAllFindingsFn func(ctx context.Context) ([]*storage.AnalyticalFinding, error)

	// Execution.
	GenerateBundleFn       func(ctx context.Context, slug string) (*storage.Bundle, error)
	RecordProgressFn       func(ctx context.Context, slug, agent, message string) error
	RecordBlockerFn        func(ctx context.Context, slug, agent, description string) error
	RecordCompletionFn     func(ctx context.Context, slug, agent string) error
	GetExecutionEventsFn   func(ctx context.Context, slug string, limit int) ([]*storage.ExecutionEvent, error)
	GetPrimeDataFn         func(ctx context.Context, slug string) (*storage.PrimeData, error)
	ReleaseExpiredClaimsFn func(ctx context.Context) (int, error)

	// Decision.
	CreateDecisionFn func(ctx context.Context, slug, title, body, rationale, question string,
		rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
		tags []string, scope storage.DecisionScope, originSpec, originStage string) (*storage.Decision, error)
	GetDecisionFn    func(ctx context.Context, slug string) (*storage.Decision, error)
	ListDecisionsFn  func(ctx context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error)
	UpdateDecisionFn func(ctx context.Context, slug string, expectedVersion int32, title *string,
		status *storage.DecisionStatus, body, rationale, supersededBy, question *string,
		rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
		tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string) (*storage.Decision, error)

	// Slice.
	CreateSliceFn   func(ctx context.Context, s *storage.Slice) error
	ListSlicesFn    func(ctx context.Context, parentSlug string) ([]*storage.Slice, error)
	GetSliceFn      func(ctx context.Context, slug string) (*storage.Slice, error)
	ClaimSliceFn    func(ctx context.Context, slug, assignee string) (*storage.Slice, error)
	CompleteSliceFn func(ctx context.Context, slug string) (*storage.Slice, error)

	// Claim.
	ClaimSpecFn      func(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*storage.Claim, error)
	UnclaimSpecFn    func(ctx context.Context, slug, agent string) error
	HeartbeatFn      func(ctx context.Context, slug, agent string, extendBy time.Duration) (*storage.Claim, error)
	GetActiveClaimFn func(ctx context.Context, slug string) (*storage.Claim, error)
}

// --- ConstitutionBackend ---

// GetMergedConstitution dispatches to GetMergedConstitutionFn, returning
// a nil MergedResult and no error if the function is unset.
func (s *StubBackend) GetMergedConstitution(ctx context.Context) (*storage.MergedResult, error) {
	if s.GetMergedConstitutionFn != nil {
		return s.GetMergedConstitutionFn(ctx)
	}
	return nil, nil
}

// GetConstitutionLayer dispatches to GetConstitutionLayerFn.
func (s *StubBackend) GetConstitutionLayer(ctx context.Context, layer storage.ConstitutionLayer) (*storage.Constitution, error) {
	if s.GetConstitutionLayerFn != nil {
		return s.GetConstitutionLayerFn(ctx, layer)
	}
	return nil, nil
}

// GetAllLayers dispatches to GetAllLayersFn.
func (s *StubBackend) GetAllLayers(ctx context.Context) ([]*storage.Constitution, error) {
	if s.GetAllLayersFn != nil {
		return s.GetAllLayersFn(ctx)
	}
	return nil, nil
}

// UpdateConstitution dispatches to UpdateConstitutionFn.
func (s *StubBackend) UpdateConstitution(ctx context.Context, c *storage.Constitution) (*storage.Constitution, error) {
	if s.UpdateConstitutionFn != nil {
		return s.UpdateConstitutionFn(ctx, c)
	}
	return nil, nil
}

// --- Backend (specs) ---

// CreateSpec dispatches to CreateSpecFn.
func (s *StubBackend) CreateSpec(ctx context.Context, slug, intent, priority, complexity string,
	provenance storage.SpecProvenanceType, detail storage.SpecProvenanceDetail,
	spark *storage.SparkOutput, shape *storage.ShapeOutput,
	specify *storage.SpecifyOutput, decompose *storage.DecomposeOutput,
) (*storage.Spec, error) {
	if s.CreateSpecFn != nil {
		return s.CreateSpecFn(ctx, slug, intent, priority, complexity, provenance, detail, spark, shape, specify, decompose)
	}
	return nil, nil
}

// GetSpec dispatches to GetSpecFn.
func (s *StubBackend) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	if s.GetSpecFn != nil {
		return s.GetSpecFn(ctx, slug)
	}
	return nil, nil
}

// ListSpecs dispatches to ListSpecsFn.
func (s *StubBackend) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error) {
	if s.ListSpecsFn != nil {
		return s.ListSpecsFn(ctx, stage, priority, limit)
	}
	return nil, nil
}

// UpdateSpec dispatches to UpdateSpecFn.
func (s *StubBackend) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error) {
	if s.UpdateSpecFn != nil {
		return s.UpdateSpecFn(ctx, slug, intent, stage, priority, complexity, notes)
	}
	return nil, nil
}

// Close dispatches to CloseFn.
func (s *StubBackend) Close(ctx context.Context) error {
	if s.CloseFn != nil {
		return s.CloseFn(ctx)
	}
	return nil
}

// --- GraphBackend ---

// AddEdge dispatches to AddEdgeFn.
func (s *StubBackend) AddEdge(ctx context.Context, fromSlug, toSlug string, t storage.EdgeType) (*storage.Edge, error) {
	if s.AddEdgeFn != nil {
		return s.AddEdgeFn(ctx, fromSlug, toSlug, t)
	}
	return nil, nil
}

// RemoveEdge dispatches to RemoveEdgeFn.
func (s *StubBackend) RemoveEdge(ctx context.Context, fromSlug, toSlug string, t storage.EdgeType) error {
	if s.RemoveEdgeFn != nil {
		return s.RemoveEdgeFn(ctx, fromSlug, toSlug, t)
	}
	return nil
}

// ListEdges dispatches to ListEdgesFn.
func (s *StubBackend) ListEdges(ctx context.Context, slug string, t storage.EdgeType) ([]*storage.Edge, error) {
	if s.ListEdgesFn != nil {
		return s.ListEdgesFn(ctx, slug, t)
	}
	return nil, nil
}

// GetDependencies dispatches to GetDependenciesFn.
func (s *StubBackend) GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	if s.GetDependenciesFn != nil {
		return s.GetDependenciesFn(ctx, slug)
	}
	return nil, nil
}

// GetTransitiveDeps dispatches to GetTransitiveDepsFn.
func (s *StubBackend) GetTransitiveDeps(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	if s.GetTransitiveDepsFn != nil {
		return s.GetTransitiveDepsFn(ctx, slug)
	}
	return nil, nil
}

// GetImpact dispatches to GetImpactFn.
func (s *StubBackend) GetImpact(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	if s.GetImpactFn != nil {
		return s.GetImpactFn(ctx, slug)
	}
	return nil, nil
}

// GetReady dispatches to GetReadyFn.
func (s *StubBackend) GetReady(ctx context.Context) ([]storage.NodeRef, error) {
	if s.GetReadyFn != nil {
		return s.GetReadyFn(ctx)
	}
	return nil, nil
}

// GetCriticalPath dispatches to GetCriticalPathFn.
func (s *StubBackend) GetCriticalPath(ctx context.Context, slug string) ([]storage.NodeRef, error) {
	if s.GetCriticalPathFn != nil {
		return s.GetCriticalPathFn(ctx, slug)
	}
	return nil, nil
}

// GetDependenciesWithEdgeData dispatches to GetDependenciesWithEdgeDataFn.
func (s *StubBackend) GetDependenciesWithEdgeData(ctx context.Context, slug string) ([]storage.DependencyRef, error) {
	if s.GetDependenciesWithEdgeDataFn != nil {
		return s.GetDependenciesWithEdgeDataFn(ctx, slug)
	}
	return nil, nil
}

// RefreshDependencyHashes dispatches to RefreshDependencyHashesFn.
func (s *StubBackend) RefreshDependencyHashes(ctx context.Context, slug string) error {
	if s.RefreshDependencyHashesFn != nil {
		return s.RefreshDependencyHashesFn(ctx, slug)
	}
	return nil
}

// GetFullGraph dispatches to GetFullGraphFn.
func (s *StubBackend) GetFullGraph(ctx context.Context) (*storage.FullGraph, error) {
	if s.GetFullGraphFn != nil {
		return s.GetFullGraphFn(ctx)
	}
	return nil, nil
}

// --- FindingsBackend ---

// StoreFindings dispatches to StoreFindingsFn.
func (s *StubBackend) StoreFindings(ctx context.Context, slug string, pt storage.PassType, in []storage.AnalyticalFindingInput) ([]string, error) {
	if s.StoreFindingsFn != nil {
		return s.StoreFindingsFn(ctx, slug, pt, in)
	}
	return nil, nil
}

// ListFindings dispatches to ListFindingsFn.
func (s *StubBackend) ListFindings(ctx context.Context, slug string, pt storage.PassType) ([]storage.AnalyticalFinding, error) {
	if s.ListFindingsFn != nil {
		return s.ListFindingsFn(ctx, slug, pt)
	}
	return nil, nil
}

// ListAllFindings dispatches to ListAllFindingsFn.
func (s *StubBackend) ListAllFindings(ctx context.Context) ([]*storage.AnalyticalFinding, error) {
	if s.ListAllFindingsFn != nil {
		return s.ListAllFindingsFn(ctx)
	}
	return nil, nil
}

// --- ExecutionBackend ---

// GenerateBundle dispatches to GenerateBundleFn.
func (s *StubBackend) GenerateBundle(ctx context.Context, slug string) (*storage.Bundle, error) {
	if s.GenerateBundleFn != nil {
		return s.GenerateBundleFn(ctx, slug)
	}
	return nil, nil
}

// RecordProgress dispatches to RecordProgressFn.
func (s *StubBackend) RecordProgress(ctx context.Context, slug, agent, message string) error {
	if s.RecordProgressFn != nil {
		return s.RecordProgressFn(ctx, slug, agent, message)
	}
	return nil
}

// RecordBlocker dispatches to RecordBlockerFn.
func (s *StubBackend) RecordBlocker(ctx context.Context, slug, agent, description string) error {
	if s.RecordBlockerFn != nil {
		return s.RecordBlockerFn(ctx, slug, agent, description)
	}
	return nil
}

// RecordCompletion dispatches to RecordCompletionFn.
func (s *StubBackend) RecordCompletion(ctx context.Context, slug, agent string) error {
	if s.RecordCompletionFn != nil {
		return s.RecordCompletionFn(ctx, slug, agent)
	}
	return nil
}

// GetExecutionEvents dispatches to GetExecutionEventsFn.
func (s *StubBackend) GetExecutionEvents(ctx context.Context, slug string, limit int) ([]*storage.ExecutionEvent, error) {
	if s.GetExecutionEventsFn != nil {
		return s.GetExecutionEventsFn(ctx, slug, limit)
	}
	return nil, nil
}

// GetPrimeData dispatches to GetPrimeDataFn.
func (s *StubBackend) GetPrimeData(ctx context.Context, slug string) (*storage.PrimeData, error) {
	if s.GetPrimeDataFn != nil {
		return s.GetPrimeDataFn(ctx, slug)
	}
	return nil, nil
}

// ReleaseExpiredClaims dispatches to ReleaseExpiredClaimsFn.
func (s *StubBackend) ReleaseExpiredClaims(ctx context.Context) (int, error) {
	if s.ReleaseExpiredClaimsFn != nil {
		return s.ReleaseExpiredClaimsFn(ctx)
	}
	return 0, nil
}

// --- DecisionBackend ---

// CreateDecision dispatches to CreateDecisionFn.
func (s *StubBackend) CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
	rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
	tags []string, scope storage.DecisionScope, originSpec, originStage string,
) (*storage.Decision, error) {
	if s.CreateDecisionFn != nil {
		return s.CreateDecisionFn(ctx, slug, title, body, rationale, question, rejectedAlts, confidence, tags, scope, originSpec, originStage)
	}
	return nil, nil
}

// GetDecision dispatches to GetDecisionFn.
func (s *StubBackend) GetDecision(ctx context.Context, slug string) (*storage.Decision, error) {
	if s.GetDecisionFn != nil {
		return s.GetDecisionFn(ctx, slug)
	}
	return nil, nil
}

// ListDecisions dispatches to ListDecisionsFn.
func (s *StubBackend) ListDecisions(ctx context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error) {
	if s.ListDecisionsFn != nil {
		return s.ListDecisionsFn(ctx, status, limit)
	}
	return nil, nil
}

// UpdateDecision dispatches to UpdateDecisionFn.
func (s *StubBackend) UpdateDecision(ctx context.Context, slug string, expectedVersion int32, title *string,
	status *storage.DecisionStatus, body, rationale, supersededBy, question *string,
	rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
	tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string,
) (*storage.Decision, error) {
	if s.UpdateDecisionFn != nil {
		return s.UpdateDecisionFn(ctx, slug, expectedVersion, title, status, body, rationale, supersededBy, question,
			rejectedAlts, confidence, tags, scope, originSpec, originStage)
	}
	return nil, nil
}

// --- SliceBackend ---

// CreateSlice dispatches to CreateSliceFn.
func (s *StubBackend) CreateSlice(ctx context.Context, sl *storage.Slice) error {
	if s.CreateSliceFn != nil {
		return s.CreateSliceFn(ctx, sl)
	}
	return nil
}

// ListSlices dispatches to ListSlicesFn.
func (s *StubBackend) ListSlices(ctx context.Context, parentSlug string) ([]*storage.Slice, error) {
	if s.ListSlicesFn != nil {
		return s.ListSlicesFn(ctx, parentSlug)
	}
	return nil, nil
}

// GetSlice dispatches to GetSliceFn.
func (s *StubBackend) GetSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	if s.GetSliceFn != nil {
		return s.GetSliceFn(ctx, slug)
	}
	return nil, nil
}

// ClaimSlice dispatches to ClaimSliceFn.
func (s *StubBackend) ClaimSlice(ctx context.Context, slug, assignee string) (*storage.Slice, error) {
	if s.ClaimSliceFn != nil {
		return s.ClaimSliceFn(ctx, slug, assignee)
	}
	return nil, nil
}

// CompleteSlice dispatches to CompleteSliceFn.
func (s *StubBackend) CompleteSlice(ctx context.Context, slug string) (*storage.Slice, error) {
	if s.CompleteSliceFn != nil {
		return s.CompleteSliceFn(ctx, slug)
	}
	return nil, nil
}

// --- ClaimBackend ---

// ClaimSpec dispatches to ClaimSpecFn.
func (s *StubBackend) ClaimSpec(ctx context.Context, slug, agent string, leaseDuration time.Duration) (*storage.Claim, error) {
	if s.ClaimSpecFn != nil {
		return s.ClaimSpecFn(ctx, slug, agent, leaseDuration)
	}
	return nil, nil
}

// UnclaimSpec dispatches to UnclaimSpecFn.
func (s *StubBackend) UnclaimSpec(ctx context.Context, slug, agent string) error {
	if s.UnclaimSpecFn != nil {
		return s.UnclaimSpecFn(ctx, slug, agent)
	}
	return nil
}

// Heartbeat dispatches to HeartbeatFn.
func (s *StubBackend) Heartbeat(ctx context.Context, slug, agent string, extendBy time.Duration) (*storage.Claim, error) {
	if s.HeartbeatFn != nil {
		return s.HeartbeatFn(ctx, slug, agent, extendBy)
	}
	return nil, nil
}

// GetActiveClaim dispatches to GetActiveClaimFn. Unset returns nil
// (unclaimed) and no error, matching the production semantics on a
// spec that has no active lease.
func (s *StubBackend) GetActiveClaim(ctx context.Context, slug string) (*storage.Claim, error) {
	if s.GetActiveClaimFn != nil {
		return s.GetActiveClaimFn(ctx, slug)
	}
	return nil, nil
}

// Compile-time check that StubBackend satisfies the Backend interface.
var _ Backend = (*StubBackend)(nil)
