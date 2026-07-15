// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// errRawDB simulates a raw database error that should never be exposed to clients.
var errRawDB = errors.New("postgres: query failed: SELECT * FROM specs WHERE slug = $1; pq: relation \"specs\" does not exist")

// assertSanitized verifies that an error response does not leak internal details.
func assertSanitized(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	msg := err.Error()
	for _, leak := range []string{"postgres:", "SELECT", "pq:", "relation"} {
		require.False(t, strings.Contains(msg, leak),
			"error message should not contain %q, got: %s", leak, msg)
	}
}

// --- errorBackend returns raw DB errors for every method ---

type errorBackend struct {
	stubBackend
}

func (errorBackend) CreateSpec(_ context.Context, _, _, _, _ string, _ storage.SpecProvenanceType, _ storage.SpecProvenanceDetail, _ *storage.SparkOutput, _ *storage.ShapeOutput, _ *storage.SpecifyOutput, _ *storage.DecomposeOutput) (*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) GetSpec(context.Context, string) (*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) ListSpecs(context.Context, string, string, int) ([]*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) UpdateSpec(context.Context, string, *string, *string, *string, *string, *string) (*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) AddEdge(context.Context, string, string, storage.EdgeType) (*storage.Edge, error) {
	return nil, errRawDB
}

func (errorBackend) RemoveEdge(context.Context, string, string, storage.EdgeType) error {
	return errRawDB
}

func (errorBackend) ListEdges(context.Context, string, storage.EdgeType) ([]*storage.Edge, error) {
	return nil, errRawDB
}

func (errorBackend) GetDependencies(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errRawDB
}

func (errorBackend) GetTransitiveDeps(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errRawDB
}

func (errorBackend) GetImpact(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errRawDB
}

func (errorBackend) GetReady(context.Context) ([]storage.NodeRef, error) {
	return nil, errRawDB
}

func (errorBackend) GetCriticalPath(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errRawDB
}

func (errorBackend) GetFullGraph(context.Context) (*storage.FullGraph, error) {
	return nil, errRawDB
}

func (errorBackend) GetMergedConstitution(context.Context) (*storage.MergedResult, error) {
	return nil, errRawDB
}

// GetConstitutionLayer + GetAllLayers overrides preserve the test intent
// of TestConstitutionHandler_ErrorSanitization: every storage path that
// the constitution RPC handler can hit must surface a raw DB error so
// the test verifies sanitization happens before the client sees it.
// Without these, the embedded stubBackend returns errNotImplemented and
// the test exercises a weaker assertion path.
func (errorBackend) GetConstitutionLayer(context.Context, storage.ConstitutionLayer) (*storage.Constitution, error) {
	return nil, errRawDB
}

func (errorBackend) GetAllLayers(context.Context) ([]*storage.Constitution, error) {
	return nil, errRawDB
}

func (errorBackend) UpdateConstitution(context.Context, *storage.Constitution) (*storage.Constitution, error) {
	return nil, errRawDB
}

func (errorBackend) ClaimSpec(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errRawDB
}

func (errorBackend) UnclaimSpec(context.Context, string, string) error {
	return errRawDB
}

func (errorBackend) Heartbeat(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errRawDB
}

func (errorBackend) GetActiveClaim(context.Context, string) (*storage.Claim, error) {
	return nil, errRawDB
}

func (errorBackend) CreateDecision(context.Context, string, string, string, string, string,
	[]storage.RejectedAlternative, storage.DecisionConfidence,
	[]string, storage.DecisionScope, string, string,
) (*storage.Decision, error) {
	return nil, errRawDB
}

func (errorBackend) GetDecision(context.Context, string) (*storage.Decision, error) {
	return nil, errRawDB
}

func (errorBackend) ListDecisions(context.Context, storage.DecisionStatus, int) ([]*storage.Decision, error) {
	return nil, errRawDB
}

func (errorBackend) UpdateDecision(context.Context, string, int32, *string, *storage.DecisionStatus,
	*string, *string, *string, *string,
	*[]storage.RejectedAlternative, *storage.DecisionConfidence,
	*[]string, *storage.DecisionScope, *string, *string,
) (*storage.Decision, error) {
	return nil, errRawDB
}

func (errorBackend) GenerateBundle(context.Context, string) (*storage.Bundle, error) {
	return nil, errRawDB
}

func (errorBackend) RecordProgress(context.Context, string, string, string) error {
	return errRawDB
}

func (errorBackend) RecordBlocker(context.Context, string, string, string) error {
	return errRawDB
}

func (errorBackend) RecordCompletion(context.Context, string, string) error {
	return errRawDB
}

func (errorBackend) GetExecutionEvents(context.Context, string, int) ([]*storage.ExecutionEvent, error) {
	return nil, errRawDB
}

func (errorBackend) GetPrimeData(context.Context, string) (*storage.PrimeData, error) {
	return nil, errRawDB
}

func (errorBackend) StoreFindings(context.Context, string, storage.PassType, []storage.AnalyticalFindingInput) ([]string, error) {
	return nil, errRawDB
}

func (errorBackend) ListFindings(context.Context, string, storage.PassType) ([]storage.AnalyticalFinding, error) {
	return nil, errRawDB
}

func (errorBackend) ListAllFindings(context.Context) ([]*storage.AnalyticalFinding, error) {
	return nil, errRawDB
}

func (errorBackend) LifecycleAmendSpec(context.Context, string, string, string) (*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) LifecycleSupersedeSpec(context.Context, string, string, string) (*storage.Spec, *storage.Spec, error) {
	return nil, nil, errRawDB
}

func (errorBackend) LifecycleAbandonSpec(context.Context, string, string) (*storage.Spec, error) {
	return nil, errRawDB
}

func (errorBackend) LifecycleAcknowledgeDrift(context.Context, string, string, string) error {
	return errRawDB
}

func (errorBackend) ListSlices(context.Context, string) ([]*storage.Slice, error) {
	return nil, errRawDB
}

func (errorBackend) GetSlice(context.Context, string) (*storage.Slice, error) {
	return nil, errRawDB
}

func (errorBackend) ClaimSlice(context.Context, string, string) (*storage.Slice, error) {
	return nil, errRawDB
}

func (errorBackend) CompleteSlice(context.Context, string) (*storage.Slice, error) {
	return nil, errRawDB
}

func (errorBackend) GetProject(context.Context, string) (*storage.Project, error) {
	return nil, errRawDB
}

func (errorBackend) EnsureProject(context.Context, string) (*storage.Project, error) {
	return nil, errRawDB
}

func (errorBackend) UpdateProject(context.Context, string, []string, string) (*storage.Project, error) {
	return nil, errRawDB
}

func (errorBackend) ListProjects(context.Context) ([]*storage.Project, error) {
	return nil, errRawDB
}

// --- SpecService error sanitization tests ---

func TestSpecHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	srv := httptest.NewServer(wrapTestProject(server.NewMux(scoper)))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("CreateSpec", func(t *testing.T) {
		_, err := client.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
			Slug: "test-spec", Intent: "test",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetSpec", func(t *testing.T) {
		_, err := client.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ListSpecs", func(t *testing.T) {
		_, err := client.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("UpdateSpec", func(t *testing.T) {
		_, err := client.UpdateSpec(ctx, connect.NewRequest(&specv1.UpdateSpecRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- GraphService error sanitization tests ---

func TestGraphHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterGraphService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewGraphServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("AddEdge", func(t *testing.T) {
		_, err := client.AddEdge(ctx, connect.NewRequest(&specv1.AddEdgeRequest{
			FromSlug: "spec-a", ToSlug: "spec-b",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("RemoveEdge", func(t *testing.T) {
		_, err := client.RemoveEdge(ctx, connect.NewRequest(&specv1.RemoveEdgeRequest{
			FromSlug: "spec-a", ToSlug: "spec-b",
			EdgeType: specv1.EdgeType_EDGE_TYPE_DEPENDS_ON,
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ListEdges", func(t *testing.T) {
		_, err := client.ListEdges(ctx, connect.NewRequest(&specv1.ListEdgesRequest{
			Slug: "spec-a",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetDependencies", func(t *testing.T) {
		_, err := client.GetDependencies(ctx, connect.NewRequest(&specv1.GetDependenciesRequest{
			Slug: "spec-a",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetTransitiveDeps", func(t *testing.T) {
		_, err := client.GetTransitiveDeps(ctx, connect.NewRequest(&specv1.GetTransitiveDepsRequest{
			Slug: "spec-a",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetImpact", func(t *testing.T) {
		_, err := client.GetImpact(ctx, connect.NewRequest(&specv1.GetImpactRequest{
			Slug: "spec-a",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetReady", func(t *testing.T) {
		_, err := client.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetCriticalPath", func(t *testing.T) {
		_, err := client.GetCriticalPath(ctx, connect.NewRequest(&specv1.GetCriticalPathRequest{
			Slug: "spec-a",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetFullGraph", func(t *testing.T) {
		_, err := client.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- ConstitutionService error sanitization tests ---

func TestConstitutionHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterConstitutionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewConstitutionServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("GetConstitution", func(t *testing.T) {
		_, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("UpdateConstitution", func(t *testing.T) {
		_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
			Constitution: &specv1.Constitution{Name: "test"},
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- ClaimService error sanitization tests ---

func TestClaimHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterClaimService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewClaimServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("ClaimSpec", func(t *testing.T) {
		_, err := client.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
			SpecSlug: "test-spec", Agent: "agent-1",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("UnclaimSpec", func(t *testing.T) {
		_, err := client.UnclaimSpec(ctx, connect.NewRequest(&specv1.UnclaimSpecRequest{
			SpecSlug: "test-spec", Agent: "agent-1",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("Heartbeat", func(t *testing.T) {
		_, err := client.Heartbeat(ctx, connect.NewRequest(&specv1.HeartbeatRequest{
			SpecSlug: "test-spec", Agent: "agent-1",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- ExecutionService error sanitization tests ---

func TestExecutionHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterExecutionService(mux, scoper, nil)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewExecutionServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("GenerateBundle", func(t *testing.T) {
		_, err := client.GenerateBundle(ctx, connect.NewRequest(&specv1.GenerateBundleRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetPrime", func(t *testing.T) {
		_, err := client.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ReportProgress", func(t *testing.T) {
		_, err := client.ReportProgress(ctx, connect.NewRequest(&specv1.ReportProgressRequest{
			Slug: "test-spec", Agent: "agent-1", Message: "working",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ReportBlocker", func(t *testing.T) {
		_, err := client.ReportBlocker(ctx, connect.NewRequest(&specv1.ReportBlockerRequest{
			Slug: "test-spec", Agent: "agent-1", Description: "stuck",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ReportCompletion", func(t *testing.T) {
		_, err := client.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
			Slug: "test-spec", Agent: "agent-1",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetExecutionEvents", func(t *testing.T) {
		_, err := client.GetExecutionEvents(ctx, connect.NewRequest(&specv1.GetExecutionEventsRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- DecisionService error sanitization tests ---

func TestDecisionHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterDecisionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewDecisionServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("CreateDecision", func(t *testing.T) {
		_, err := client.CreateDecision(ctx, connect.NewRequest(&specv1.CreateDecisionRequest{
			Slug: "adr-001", Title: "test", Decision: "do it", Rationale: "because",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetDecision", func(t *testing.T) {
		_, err := client.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{
			Slug: "adr-001",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ListDecisions", func(t *testing.T) {
		_, err := client.ListDecisions(ctx, connect.NewRequest(&specv1.ListDecisionsRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("UpdateDecision", func(t *testing.T) {
		_, err := client.UpdateDecision(ctx, connect.NewRequest(&specv1.UpdateDecisionRequest{
			Slug: "adr-001",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- AnalyticalPassService error sanitization tests ---

func TestAnalyticalPassHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterAnalyticalPassService(mux, scoper, "")
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewAnalyticalPassServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("RunAnalyticalPass", func(t *testing.T) {
		_, err := client.RunAnalyticalPass(ctx, connect.NewRequest(&specv1.RunAnalyticalPassRequest{
			Slug:     "test-spec",
			PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("StoreFindings", func(t *testing.T) {
		_, err := client.StoreFindings(ctx, connect.NewRequest(&specv1.StoreFindingsRequest{
			Slug:     "test-spec",
			PassType: specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK,
			Findings: []*specv1.AnalyticalFindingInput{
				{Summary: "test", Severity: specv1.FindingSeverity_FINDING_SEVERITY_NOTE},
			},
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ListFindings", func(t *testing.T) {
		_, err := client.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ListProjectFindings", func(t *testing.T) {
		_, err := client.ListProjectFindings(ctx, connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- LifecycleService error sanitization tests ---

func TestLifecycleHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, scoper, nil, nil, nil)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("TransitionAmend", func(t *testing.T) {
		_, err := client.TransitionAmend(ctx, connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug: "test-spec", Reason: "needs rework", ReEntryStage: "shape",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("TransitionAbandon", func(t *testing.T) {
		_, err := client.TransitionAbandon(ctx, connect.NewRequest(&specv1.TransitionAbandonRequest{
			Slug: "test-spec", Reason: "no longer needed",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("TransitionSupersede", func(t *testing.T) {
		_, err := client.TransitionSupersede(ctx, connect.NewRequest(&specv1.TransitionSupersedeRequest{
			Slug: "spec-a", NewSlug: "spec-b",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("AcknowledgeDrift", func(t *testing.T) {
		_, err := client.AcknowledgeDrift(ctx, connect.NewRequest(&specv1.DriftAcknowledgeRequest{
			Slug: "test-spec", All: true, Note: "intentional",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}

// --- SliceService error sanitization tests ---

func TestSliceHandler_ErrorSanitization(t *testing.T) {
	scoper := &testScoper{backend: errorBackend{}}
	mux := http.NewServeMux()
	server.RegisterSliceService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSliceServiceClient(http.DefaultClient, srv.URL)
	ctx := context.Background()

	t.Run("ListSlices", func(t *testing.T) {
		_, err := client.ListSlices(ctx, connect.NewRequest(&specv1.ListSlicesRequest{
			ParentSlug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("GetSlice", func(t *testing.T) {
		_, err := client.GetSlice(ctx, connect.NewRequest(&specv1.GetSliceRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("ClaimSlice", func(t *testing.T) {
		_, err := client.ClaimSlice(ctx, connect.NewRequest(&specv1.ClaimSliceRequest{
			Slug: "test-spec", Assignee: "agent-1",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})

	t.Run("CompleteSlice", func(t *testing.T) {
		_, err := client.CompleteSlice(ctx, connect.NewRequest(&specv1.CompleteSliceRequest{
			Slug: "test-spec",
		}))
		require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
		assertSanitized(t, err)
	})
}
