// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// fakeAuthoringBackend is a minimal fake implementation of storage.AuthoringBackend for testing.
type fakeAuthoringBackend struct {
	transitionStageErr      error
	storeSparkOutputErr     error
	storeShapeOutputErr     error
	storeSpecifyOutputErr   error
	storeDecomposeOutputErr error
	supersedeErr            error
	amendErr                error
	amendResult             *storage.AmendResult
}

func (f *fakeAuthoringBackend) TransitionStage(_ context.Context, _ string, _, _ storage.AuthoringStage) error {
	return f.transitionStageErr
}

func (f *fakeAuthoringBackend) StoreSparkOutput(_ context.Context, _ string, _ *storage.SparkOutput) error {
	return f.storeSparkOutputErr
}

func (f *fakeAuthoringBackend) StoreShapeOutput(_ context.Context, _ string, _ *storage.ShapeOutput) error {
	return f.storeShapeOutputErr
}

func (f *fakeAuthoringBackend) StoreSpecifyOutput(_ context.Context, _ string, _ *storage.SpecifyOutput) error {
	return f.storeSpecifyOutputErr
}

func (f *fakeAuthoringBackend) StoreDecomposeOutput(_ context.Context, _ string, _ *storage.DecomposeOutput) ([]string, error) {
	return nil, f.storeDecomposeOutputErr
}

func (f *fakeAuthoringBackend) StoreRedTeamFindings(_ context.Context, _ string, _ []storage.RedTeamFinding) error {
	return nil
}

func (f *fakeAuthoringBackend) StorePeripheralVision(_ context.Context, _ string, _ []storage.PeripheralVisionItem) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreConsistencyIssues(_ context.Context, _ string, _ []storage.ConsistencyIssue) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreSimplicityFindings(_ context.Context, _ string, _ []storage.SimplicityFinding) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreSafetyFlags(_ context.Context, _ string, _ []storage.SafetyFlag) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreConstitutionViolations(_ context.Context, _ string, _ []storage.ConstitutionViolation) error {
	return nil
}

func (f *fakeAuthoringBackend) SupersedeSpec(_ context.Context, _, _, _ string) error {
	return f.supersedeErr
}

func (f *fakeAuthoringBackend) AmendSpec(_ context.Context, _, _ string, _ storage.AuthoringStage) (*storage.AmendResult, error) {
	return f.amendResult, f.amendErr
}

// fakeBackend is a minimal fake implementation of storage.Backend for testing.
type fakeBackend struct {
	createSpecErr    error
	createSpecResult *specv1.Spec
}

// fakeTxBackend embeds fakeBackend and implements TransactionalBackend,
// enabling tests that exercise the transactional code path.
type fakeTxBackend struct {
	fakeBackend
	runInTxErr error // when non-nil, RunInTransaction returns this instead of calling fn
}

func (f *fakeTxBackend) RunInTransaction(_ context.Context, fn func(ctx context.Context) error) error {
	if f.runInTxErr != nil {
		return f.runInTxErr
	}
	return fn(context.Background())
}

func (f *fakeBackend) CreateSpec(_ context.Context, slug, _, _, _ string) (*specv1.Spec, error) {
	if f.createSpecErr != nil {
		return nil, f.createSpecErr
	}
	if f.createSpecResult != nil {
		return f.createSpecResult, nil
	}
	return &specv1.Spec{Slug: slug}, nil
}

func (f *fakeBackend) GetSpec(_ context.Context, slug string) (*specv1.Spec, error) {
	return &specv1.Spec{Slug: slug}, nil
}

func (f *fakeBackend) ListSpecs(_ context.Context, _, _ string, _ int) ([]*specv1.Spec, error) {
	return nil, nil
}

func (f *fakeBackend) UpdateSpec(_ context.Context, slug string, _, _, _, _ *string) (*specv1.Spec, error) {
	return &specv1.Spec{Slug: slug}, nil
}

func (f *fakeBackend) Close(_ context.Context) error {
	return nil
}

func newAuthoringClient(t *testing.T, authoringStore storage.AuthoringBackend, backend storage.Backend) specgraphv1connect.AuthoringServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, authoringStore, backend)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)
}

func TestAuthoringHandler_GetPrompts(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, &fakeAuthoringBackend{}, &fakeBackend{})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_SPARK,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)

	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
	}
	require.True(t, names["seed"])
	require.True(t, names["signal"])
	require.True(t, names["kill_test"])
}

func TestAuthoringHandler_Spark_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "",
		Output: &specv1.SparkOutput{Seed: "some intent"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my-spec",
		Output: &specv1.SparkOutput{Seed: "some intent"},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, "some intent", resp.Msg.Output.Seed)
}

func TestAuthoringHandler_Shape_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "my-spec",
		Output: &specv1.ShapeOutput{
			ScopeIn:  []string{"auth endpoint"},
			ScopeOut: []string{"admin panel"},
			Risks:    []string{"latency"},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, []string{"auth endpoint"}, resp.Msg.Output.ScopeIn)
	require.NotEmpty(t, resp.Msg.NextPrompts, "should include next-stage prompts")
}

func TestAuthoringHandler_Specify_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "my-spec",
		Output: &specv1.SpecifyOutput{
			InterfaceContract: "POST /api/login",
			VerifyCriteria:    []string{"returns 200 on valid credentials"},
			Invariants:        []string{"session token is opaque"},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, "POST /api/login", resp.Msg.Output.InterfaceContract)
	require.NotEmpty(t, resp.Msg.NextPrompts, "should include next-stage prompts")
}

func TestAuthoringHandler_Decompose_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "auth endpoint"},
			},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE, resp.Msg.Output.Strategy)
}

func TestAuthoringHandler_Approve_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, resp.Msg.Stage)
	require.NotNil(t, resp.Msg.ApprovedAt, "approved_at timestamp should be set")
}

func TestAuthoringHandler_Amend_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: "shape", Version: 2},
	}, &fakeBackend{})
	resp, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "scope changed",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SHAPE, resp.Msg.Stage)
	require.Equal(t, int32(2), resp.Msg.Version)
}

func TestAuthoringHandler_Shape_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "",
		Output: &specv1.ShapeOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Specify_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   "",
		Output: &specv1.SpecifyOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Decompose_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   "",
		Output: &specv1.DecomposeOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Approve_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Amend_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Supersede_EmptySlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Supersede_EmptySupersedeBy(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         "my-spec",
		SupersededBy: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Supersede_NotFound(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{supersedeErr: storage.ErrSpecNotFound}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         "missing-spec",
		SupersededBy: "new-spec",
		Reason:       "replaced",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestAuthoringHandler_StageError_InvalidTransition(t *testing.T) {
	// Shape with ErrInvalidStageTransition → CodeFailedPrecondition
	authoringStore := &fakeAuthoringBackend{transitionStageErr: storage.ErrInvalidStageTransition}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "my-spec",
		Output: &specv1.ShapeOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestAuthoringHandler_StageError_NotFound(t *testing.T) {
	// Shape with ErrSpecNotFound → CodeNotFound
	authoringStore := &fakeAuthoringBackend{transitionStageErr: storage.ErrSpecNotFound}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "missing-spec",
		Output: &specv1.ShapeOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestAuthoringHandler_Spark_CreateSpecError(t *testing.T) {
	backend := &fakeBackend{createSpecErr: errors.New("db error")}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my-spec",
		Output: &specv1.SparkOutput{Seed: "some intent"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Spark_StoreSparkOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeSparkOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my-spec",
		Output: &specv1.SparkOutput{Seed: "some intent"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Spark_NilOutput(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my-spec",
		Output: nil,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Shape_StoreOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeShapeOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "my-spec",
		Output: &specv1.ShapeOutput{ScopeIn: []string{"auth endpoint"}},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Specify_StoreOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeSpecifyOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   "my-spec",
		Output: &specv1.SpecifyOutput{InterfaceContract: "POST /api/login"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Decompose_StoreOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeDecomposeOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "auth endpoint"},
			},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_GetPrompts_UnspecifiedStage(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, &fakeAuthoringBackend{}, &fakeBackend{})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	_, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_GetPrompts_ApprovedStage(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, &fakeAuthoringBackend{}, &fakeBackend{})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	_, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_StageError_AlreadyApproved(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{transitionStageErr: storage.ErrSpecAlreadyApproved}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "my-spec",
		Output: &specv1.ShapeOutput{},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestAuthoringHandler_Supersede_InvalidSupersedeBy(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         "my-spec",
		SupersededBy: "../escape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_PathTraversalSlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "../../etc/passwd",
		Output: &specv1.SparkOutput{Seed: "test"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_InvalidCharSlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my spec!@#",
		Output: &specv1.SparkOutput{Seed: "test"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_TransactionalPath(t *testing.T) {
	txBackend := &fakeTxBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, txBackend)
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "tx-spec",
		Output: &specv1.SparkOutput{Seed: "transactional test"},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, "transactional test", resp.Msg.Output.Seed)
}

func TestAuthoringHandler_Spark_TransactionalRollback(t *testing.T) {
	// When the authoring store fails inside a transaction, the error propagates.
	txBackend := &fakeTxBackend{}
	authoringStore := &fakeAuthoringBackend{storeSparkOutputErr: errors.New("simulated failure")}
	client := newAuthoringClient(t, authoringStore, txBackend)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "tx-spec",
		Output: &specv1.SparkOutput{Seed: "will fail"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}
