// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// provenanceBackend is a mock that respects provenance+stage output combinations
// to correctly reflect the born-at-done behaviour for RETROACTIVE/DECLARED specs.
type provenanceBackend struct {
	stubBackend
	mu    sync.Mutex
	specs map[string]*storage.Spec
	seq   int
}

func newProvenanceBackend() *provenanceBackend {
	return &provenanceBackend{specs: make(map[string]*storage.Spec)}
}

func (m *provenanceBackend) CreateSpec(
	_ context.Context,
	slug, intent, priority, complexity string,
	pt storage.SpecProvenanceType,
	_ storage.SpecProvenanceDetail,
	_ *storage.SparkOutput,
	shape *storage.ShapeOutput,
	specify *storage.SpecifyOutput,
	decompose *storage.DecomposeOutput,
) (*storage.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.specs[slug]; exists {
		return nil, fmt.Errorf("spec %q: %w", slug, storage.ErrSpecAlreadyExists)
	}
	m.seq++
	now := time.Now().UTC()

	// Born-at-done when all four stage outputs are provided.
	stage := storage.SpecStageSpark
	if shape != nil && specify != nil && decompose != nil {
		stage = storage.SpecStageDone
	}

	if priority == "" {
		priority = "p2"
	}
	if complexity == "" {
		complexity = "medium"
	}

	spec := &storage.Spec{
		ID:          fmt.Sprintf("spec-%05d", m.seq),
		Slug:        slug,
		Intent:      intent,
		Stage:       stage,
		Priority:    storage.SpecPriority(priority),
		Complexity:  storage.SpecComplexity(complexity),
		Provenance:  pt,
		Version:     1,
		ContentHash: strings.Repeat("a", 32),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.specs[slug] = spec
	return spec, nil
}

func (m *provenanceBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	spec, ok := m.specs[slug]
	if !ok {
		return nil, fmt.Errorf("spec %q: %w", slug, storage.ErrSpecNotFound)
	}
	return spec, nil
}

func setupProvenanceServer(t *testing.T) (specgraphv1connect.SpecServiceClient, specgraphv1connect.ClaimServiceClient, specgraphv1connect.ExecutionServiceClient) {
	t.Helper()
	mb := newProvenanceBackend()
	scoper := &testScoper{backend: mb}
	mux := server.NewMux(scoper)
	server.RegisterClaimService(mux, scoper)
	server.RegisterExecutionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	specClient := specgraphv1connect.NewSpecServiceClient(http.DefaultClient, srv.URL)
	claimClient := specgraphv1connect.NewClaimServiceClient(http.DefaultClient, srv.URL)
	execClient := specgraphv1connect.NewExecutionServiceClient(http.DefaultClient, srv.URL)
	return specClient, claimClient, execClient
}

// minSparkOutput returns a minimal valid SparkOutput for use in tests.
func minSparkOutput() *specv1.SparkOutput {
	return &specv1.SparkOutput{Seed: "seed text"}
}

// minShapeOutput returns a minimal valid ShapeOutput for use in tests.
func minShapeOutput() *specv1.ShapeOutput {
	return &specv1.ShapeOutput{
		SuccessMust: []string{"criterion"},
	}
}

// minSpecifyOutput returns a minimal valid SpecifyOutput for use in tests.
func minSpecifyOutput() *specv1.SpecifyOutput {
	return &specv1.SpecifyOutput{
		Invariants: []string{"must be idempotent"},
	}
}

// minDecomposeOutput returns a minimal valid DecomposeOutput for use in tests.
func minDecomposeOutput() *specv1.DecomposeOutput {
	return &specv1.DecomposeOutput{
		Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
		Slices: []*specv1.DecompositionSlice{
			{Id: "slice-1", Intent: "initial slice"},
		},
	}
}

// --- Happy-path creation tests ---

func TestCreateSpec_AuthoredHappyPath(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	resp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:          "authored-spec",
		Intent:        "An authored spec",
		Priority:      "p1",
		Complexity:    "low",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED,
	}))
	require.NoError(t, err)
	require.Equal(t, "spark", resp.Msg.GetSpec().GetStage())
	require.Equal(t, specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED, resp.Msg.GetSpec().GetProvenanceType())
}

func TestCreateSpec_RetroactiveBornAtDone(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	resp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "retro-spec",
		Intent:         "A retroactive spec born at done",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
		ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url: "https://github.com/org/repo/pull/42",
				Sha: "abc123def456",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.NoError(t, err)
	require.Equal(t, "done", resp.Msg.GetSpec().GetStage())
	require.Equal(t, specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR, resp.Msg.GetSpec().GetProvenanceType())
}

func TestCreateSpec_DeclaredBornAtDone(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	resp, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "declared-spec",
		Intent:         "A declared spec born at done",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
		ProvenanceDetail: &specv1.CreateSpecRequest_Declared{
			Declared: &specv1.DeclaredProvenance{
				DeclaredBy: "platform-team",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.NoError(t, err)
	require.Equal(t, "done", resp.Msg.GetSpec().GetStage())
	require.Equal(t, specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED, resp.Msg.GetSpec().GetProvenanceType())
}

// --- Sentinel rejection tests ---

func TestCreateSpec_AuthoredRequiresSparkOnly(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "authored-with-shape",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_AUTHORED,
		ShapeOutput:    minShapeOutput(),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSpec_RetroactiveRequiresAllOutputs(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	// Missing decompose_output.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "retro-missing-decompose",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
		ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url: "https://github.com/org/repo/pull/1",
				Sha: "deadbeef",
			},
		},
		SparkOutput:   minSparkOutput(),
		ShapeOutput:   minShapeOutput(),
		SpecifyOutput: minSpecifyOutput(),
		// DecomposeOutput intentionally omitted.
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSpec_RetroactiveRequiresPRRef(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	// All outputs provided but empty URL.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "retro-no-url",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
		ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url: "",   // empty — invalid
				Sha: "abc",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSpec_DeclaredRequiresAllOutputs(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	// Missing decompose_output.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "declared-missing-decompose",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
		ProvenanceDetail: &specv1.CreateSpecRequest_Declared{
			Declared: &specv1.DeclaredProvenance{
				DeclaredBy: "platform-team",
			},
		},
		SparkOutput:   minSparkOutput(),
		ShapeOutput:   minShapeOutput(),
		SpecifyOutput: minSpecifyOutput(),
		// DecomposeOutput intentionally omitted.
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSpec_DeclaredRequiresDeclaredBy(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "declared-no-by",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
		ProvenanceDetail: &specv1.CreateSpecRequest_Declared{
			Declared: &specv1.DeclaredProvenance{
				DeclaredBy: "", // empty — invalid
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCreateSpec_ProvenanceMismatch(t *testing.T) {
	specClient, _, _ := setupProvenanceServer(t)
	ctx := context.Background()

	// provenance_type=DECLARED but retroactive_from_pr detail.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "mismatch-spec",
		Intent:         "should be rejected",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
		ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url: "https://github.com/org/repo/pull/99",
				Sha: "deadbeef",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// --- Claim/Completion rejection on non-AUTHORED specs ---

func TestClaim_RejectsRetroactive(t *testing.T) {
	specClient, claimClient, _ := setupProvenanceServer(t)
	ctx := context.Background()

	// Create a RETROACTIVE spec.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "retro-claim-test",
		Intent:         "retroactive spec to test claim rejection",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_RETROACTIVE_FROM_PR,
		ProvenanceDetail: &specv1.CreateSpecRequest_RetroactiveFromPr{
			RetroactiveFromPr: &specv1.RetroactiveFromPrProvenance{
				Url: "https://github.com/org/repo/pull/7",
				Sha: "feedface",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.NoError(t, err)

	// Attempt to claim — must be rejected.
	_, err = claimClient.ClaimSpec(ctx, connect.NewRequest(&specv1.ClaimSpecRequest{
		SpecSlug: "retro-claim-test",
		Agent:    "agent-1",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestCompletion_RejectsDeclared(t *testing.T) {
	specClient, _, execClient := setupProvenanceServer(t)
	ctx := context.Background()

	// Create a DECLARED spec.
	_, err := specClient.CreateSpec(ctx, connect.NewRequest(&specv1.CreateSpecRequest{
		Slug:           "declared-completion-test",
		Intent:         "declared spec to test completion rejection",
		ProvenanceType: specv1.SpecProvenance_SPEC_PROVENANCE_DECLARED,
		ProvenanceDetail: &specv1.CreateSpecRequest_Declared{
			Declared: &specv1.DeclaredProvenance{
				DeclaredBy: "ops-team",
			},
		},
		SparkOutput:     minSparkOutput(),
		ShapeOutput:     minShapeOutput(),
		SpecifyOutput:   minSpecifyOutput(),
		DecomposeOutput: minDecomposeOutput(),
	}))
	require.NoError(t, err)

	// Attempt to report completion — must be rejected.
	_, err = execClient.ReportCompletion(ctx, connect.NewRequest(&specv1.ReportCompletionRequest{
		Slug:  "declared-completion-test",
		Agent: "agent-1",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
