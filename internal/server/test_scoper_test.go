// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
)

const testProject = "test-project"

// testScoper wraps a storage.ScopedBackend as a storage.Scoper.
// Scoped() always returns the same backend regardless of project slug.
type testScoper struct {
	backend storage.ScopedBackend
}

func (s *testScoper) Scoped(_ context.Context, _ string) (storage.ScopedBackend, error) {
	return s.backend, nil
}

// wrapTestProject injects a default X-Specgraph-Project header into incoming
// requests (if not already set), then applies ProjectMiddleware to extract
// it into context. This allows test clients to work without setting the header.
func wrapTestProject(h http.Handler) http.Handler {
	withProject := server.ProjectMiddleware(h)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Specgraph-Project") == "" {
			r.Header.Set("X-Specgraph-Project", testProject)
		}
		withProject.ServeHTTP(w, r)
	})
}

var errNotImplemented = errors.New("not implemented in test")

// stubBackend provides no-op/panic implementations of all storage interfaces.
// Tests embed this and override only the methods they need.
type stubBackend struct{}

// --- Backend ---

func (stubBackend) CreateSpec(context.Context, string, string, string, string) (*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetSpec(context.Context, string) (*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) ListSpecs(context.Context, string, string, int) ([]*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) UpdateSpec(context.Context, string, *string, *string, *string, *string, *string) (*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) Close(context.Context) error { return nil }

// --- GraphBackend ---

func (stubBackend) AddEdge(context.Context, string, string, storage.EdgeType) (*storage.Edge, error) {
	return nil, errNotImplemented
}

func (stubBackend) RemoveEdge(context.Context, string, string, storage.EdgeType) error {
	return errNotImplemented
}

func (stubBackend) ListEdges(context.Context, string, storage.EdgeType) ([]*storage.Edge, error) {
	return nil, nil // return empty for tests that don't use graph operations
}

func (stubBackend) GetDependencies(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetTransitiveDeps(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetImpact(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetReady(context.Context) ([]storage.NodeRef, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetCriticalPath(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errNotImplemented
}

// --- DecisionBackend ---

func (stubBackend) CreateDecision(context.Context, string, string, string, string) (*storage.Decision, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetDecision(context.Context, string) (*storage.Decision, error) {
	return nil, errNotImplemented
}

func (stubBackend) ListDecisions(context.Context, storage.DecisionStatus, int) ([]*storage.Decision, error) {
	return nil, errNotImplemented
}

func (stubBackend) UpdateDecision(context.Context, string, *string, *storage.DecisionStatus, *string, *string, *string) (*storage.Decision, error) {
	return nil, errNotImplemented
}

// --- ClaimBackend ---

func (stubBackend) ClaimSpec(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errNotImplemented
}

func (stubBackend) UnclaimSpec(context.Context, string, string) error { return errNotImplemented }

func (stubBackend) Heartbeat(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errNotImplemented
}

// --- ConstitutionBackend ---

func (stubBackend) GetConstitution(context.Context) (*storage.Constitution, error) {
	return nil, errNotImplemented
}

func (stubBackend) UpdateConstitution(context.Context, *storage.Constitution) (*storage.Constitution, error) {
	return nil, errNotImplemented
}

// --- AuthoringBackend (StageWriter + AuthoringSpecLifecycle) ---

func (stubBackend) TransitionStage(context.Context, string, storage.AuthoringStage, storage.AuthoringStage) error {
	return errNotImplemented
}

func (stubBackend) StoreSparkOutput(context.Context, string, *storage.SparkOutput) error {
	return errNotImplemented
}

func (stubBackend) StoreShapeOutput(context.Context, string, *storage.ShapeOutput) error {
	return errNotImplemented
}

func (stubBackend) StoreSpecifyOutput(context.Context, string, *storage.SpecifyOutput) error {
	return errNotImplemented
}

func (stubBackend) StoreDecomposeOutput(context.Context, string, *storage.DecomposeOutput) ([]string, error) {
	return nil, errNotImplemented
}

func (stubBackend) StoreSafetyFlags(context.Context, string, []storage.SafetyFlag) error {
	return errNotImplemented
}

func (stubBackend) SupersedeSpec(context.Context, string, string, string) error {
	return errNotImplemented
}

func (stubBackend) AmendSpec(context.Context, string, string, storage.AuthoringStage) (*storage.AmendResult, error) {
	return nil, errNotImplemented
}

// --- FindingsBackend ---

func (stubBackend) StoreFindings(context.Context, string, storage.PassType, []storage.AnalyticalFindingInput) ([]string, error) {
	return nil, errNotImplemented
}

func (stubBackend) ListFindings(context.Context, string, storage.PassType) ([]storage.AnalyticalFinding, error) {
	return nil, errNotImplemented
}

// --- ExecutionBackend ---

func (stubBackend) GenerateBundle(context.Context, string) (*storage.Bundle, error) {
	return nil, errNotImplemented
}

func (stubBackend) RecordProgress(context.Context, string, string, string) error {
	return errNotImplemented
}

func (stubBackend) RecordBlocker(context.Context, string, string, string) error {
	return errNotImplemented
}

func (stubBackend) RecordCompletion(context.Context, string, string) error {
	return errNotImplemented
}

func (stubBackend) GetExecutionEvents(context.Context, string, int) ([]*storage.ExecutionEvent, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetPrimeData(context.Context, string) (*storage.PrimeData, error) {
	return nil, errNotImplemented
}

func (stubBackend) ReleaseExpiredClaims(context.Context) (int, error) {
	return 0, errNotImplemented
}

// --- LifecycleBackend ---

func (stubBackend) LifecycleAmendSpec(context.Context, string, string, string) (*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) LifecycleSupersedeSpec(context.Context, string, string) (*storage.Spec, *storage.Spec, error) {
	return nil, nil, errNotImplemented
}

func (stubBackend) LifecycleAbandonSpec(context.Context, string, string) (*storage.Spec, error) {
	return nil, errNotImplemented
}

func (stubBackend) LifecycleAcknowledgeDrift(context.Context, string, string, string) error {
	return errNotImplemented
}

func (stubBackend) GetDependenciesWithEdgeData(context.Context, string) ([]storage.DependencyRef, error) {
	return nil, errNotImplemented
}

func (stubBackend) RefreshDependencyHashes(context.Context, string) error {
	return errNotImplemented
}

// --- SyncBackend ---

func (stubBackend) CreateSyncMapping(context.Context, string, storage.SyncAdapterType, string) (*storage.SyncMapping, error) {
	return nil, errNotImplemented
}

func (stubBackend) UpdateSyncState(context.Context, string, storage.SyncAdapterType, storage.SyncStateType, string) (*storage.SyncMapping, error) {
	return nil, errNotImplemented
}

func (stubBackend) GetSyncMapping(context.Context, string, storage.SyncAdapterType) (*storage.SyncMapping, error) {
	return nil, errNotImplemented
}

func (stubBackend) ListSyncMappings(context.Context, storage.SyncAdapterType, string) ([]*storage.SyncMapping, error) {
	return nil, errNotImplemented
}

func (stubBackend) DeleteSyncMapping(context.Context, string, storage.SyncAdapterType) error {
	return errNotImplemented
}

// --- ProjectBackend ---

func (stubBackend) GetProject(context.Context, string) (*storage.Project, error) {
	return nil, errNotImplemented
}

func (stubBackend) EnsureProject(context.Context, string) (*storage.Project, error) {
	return nil, errNotImplemented
}

func (stubBackend) UpdateProject(context.Context, string, []string, string) (*storage.Project, error) {
	return nil, errNotImplemented
}

func (stubBackend) ListProjects(context.Context) ([]*storage.Project, error) {
	return nil, errNotImplemented
}

// Verify stubBackend satisfies ScopedBackend at compile time.
var _ storage.ScopedBackend = (*stubBackend)(nil)
