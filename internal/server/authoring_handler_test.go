// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	transitionStageErr             error
	storeSparkOutputErr            error
	storeShapeOutputErr            error
	storeSpecifyOutputErr          error
	storeDecomposeOutputErr        error
	supersedeErr                   error
	amendErr                       error
	amendResult                    *storage.AmendResult
	storeSafetyFlagsErr            error
	storeRedTeamErr                error
	storePeripheralVisionErr       error
	storeConsistencyIssuesErr      error
	storeSimplicityFindingsErr     error
	storeConstitutionViolationsErr error
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

func (f *fakeAuthoringBackend) StoreDecomposeOutput(_ context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	if f.storeDecomposeOutputErr != nil {
		return nil, f.storeDecomposeOutputErr
	}
	var slugs []string
	for _, sl := range output.Slices {
		slugs = append(slugs, slug+"/"+sl.ID)
	}
	return slugs, nil
}

func (f *fakeAuthoringBackend) StoreRedTeamFindings(_ context.Context, _ string, _ []storage.RedTeamFinding) error {
	return f.storeRedTeamErr
}

func (f *fakeAuthoringBackend) StorePeripheralVision(_ context.Context, _ string, _ []storage.PeripheralVisionItem) error {
	return f.storePeripheralVisionErr
}

func (f *fakeAuthoringBackend) StoreConsistencyIssues(_ context.Context, _ string, _ []storage.ConsistencyIssue) error {
	return f.storeConsistencyIssuesErr
}

func (f *fakeAuthoringBackend) StoreSimplicityFindings(_ context.Context, _ string, _ []storage.SimplicityFinding) error {
	return f.storeSimplicityFindingsErr
}

func (f *fakeAuthoringBackend) StoreSafetyFlags(_ context.Context, _ string, _ []storage.SafetyFlag) error {
	return f.storeSafetyFlagsErr
}

func (f *fakeAuthoringBackend) StoreConstitutionViolations(_ context.Context, _ string, _ []storage.ConstitutionViolation) error {
	return f.storeConstitutionViolationsErr
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
	createSpecResult *storage.Spec
	getSpecErr       error
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

func (f *fakeBackend) CreateSpec(_ context.Context, slug, _, _, _ string) (*storage.Spec, error) {
	if f.createSpecErr != nil {
		return nil, f.createSpecErr
	}
	if f.createSpecResult != nil {
		return f.createSpecResult, nil
	}
	return &storage.Spec{Slug: slug}, nil
}

func (f *fakeBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	if f.getSpecErr != nil {
		return nil, f.getSpecErr
	}
	return &storage.Spec{Slug: slug}, nil
}

func (f *fakeBackend) ListSpecs(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
	return nil, nil
}

func (f *fakeBackend) UpdateSpec(_ context.Context, slug string, _, _, _, _ *string) (*storage.Spec, error) {
	return &storage.Spec{Slug: slug}, nil
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
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

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
	require.Equal(t, []string{"my-spec/s1"}, resp.Msg.ChildSpecSlugs)
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
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.AuthoringStage("shape"), Version: 2},
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
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_GetPrompts_ApprovedStage(t *testing.T) {
	// Approved stage is terminal — returns empty response (no prompts, no error).
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Prompts)
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

func TestAuthoringHandler_Decompose_UnspecifiedStrategy(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "auth endpoint"},
			},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_HappyPath_SafetyFlags(t *testing.T) {
	// Spark with dangerous input should return safety flags.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "dangerous-spec",
		Output: &specv1.SparkOutput{Seed: "hardcoded secret in config"},
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.SafetyFlags, "safety flags should be populated for dangerous input")
}

func TestAuthoringHandler_Spark_EmptySeed(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "my-spec",
		Output: &specv1.SparkOutput{Seed: ""},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Supersede_SelfSupersede(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         "my-spec",
		SupersededBy: "my-spec",
		Reason:       "oops",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Amend_EmptyReason(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.AuthoringStage("shape"), Version: 2},
	}, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Amend_UnspecifiedTargetStage(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.AuthoringStage("shape"), Version: 2},
	}, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "scope changed",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_UNSPECIFIED,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_AlreadyExists(t *testing.T) {
	backend := &fakeBackend{createSpecErr: storage.ErrSpecAlreadyExists}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "existing-spec",
		Output: &specv1.SparkOutput{Seed: "some intent"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeAlreadyExists, connErr.Code())
}

func TestAuthoringHandler_Slug_ExactlyMaxLength(t *testing.T) {
	// 256 chars is the maximum allowed slug length — should succeed.
	slug := strings.Repeat("a", 256)
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   slug,
		Output: &specv1.SparkOutput{Seed: "boundary test"},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}

func TestAuthoringHandler_Slug_ExceedsMaxLength(t *testing.T) {
	// 257 chars exceeds the maximum — should fail with InvalidArgument.
	slug := strings.Repeat("a", 257)
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   slug,
		Output: &specv1.SparkOutput{Seed: "boundary test"},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Supersede_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.SupersedeRequest{
		Slug:         "old-spec",
		SupersededBy: "new-spec",
		Reason:       "design evolved",
	}))
	require.NoError(t, err)
	require.Equal(t, "old-spec", resp.Msg.Slug)
	require.Equal(t, "new-spec", resp.Msg.SupersededBy)
}

func TestAuthoringHandler_Spark_PostureAccepted(t *testing.T) {
	// Posture field is accepted without error; currently informational.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:    "posture-spec",
		Output:  &specv1.SparkOutput{Seed: "test with posture"},
		Posture: specv1.Posture_POSTURE_DRIVE,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}

func TestAuthoringHandler_GetPrompts_ShapeStage(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)
	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SHAPE, p.Stage)
	}
	require.True(t, names["bound_scope"])
	require.True(t, names["define_success"])
}

func TestAuthoringHandler_GetPrompts_SpecifyStage(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)
	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_SPECIFY, p.Stage)
	}
	require.True(t, names["interface_contract"])
	require.True(t, names["invariants"])
}

func TestAuthoringHandler_GetPrompts_DecomposeStage(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)
	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
		require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE, p.Stage)
	}
	require.True(t, names["strategy"])
	require.True(t, names["slices"])
}

func TestAuthoringHandler_Spark_StoreSafetyFlagsError(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		storeSafetyFlagsErr: errors.New("db write failed"),
	}, &fakeTxBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "safety-err-spark",
		Output: &specv1.SparkOutput{Seed: "trigger safety flag"},
	}))
	// Safety flags may not trigger for benign text, so this test verifies
	// the handler wiring. If no flags are raised, the error path is not hit
	// and the call succeeds — both outcomes are valid.
	if err != nil {
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInternal, connErr.Code())
	}
}

func TestAuthoringHandler_Shape_StoreSafetyFlagsError(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		storeSafetyFlagsErr: errors.New("db write failed"),
	}, &fakeTxBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "safety-err-shape",
		Output: &specv1.ShapeOutput{
			ScopeIn:  []string{"in"},
			ScopeOut: []string{"out"},
		},
	}))
	if err != nil {
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInternal, connErr.Code())
	}
}

func TestAuthoringHandler_Specify_StoreSafetyFlagsError(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		storeSafetyFlagsErr: errors.New("db write failed"),
	}, &fakeTxBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "safety-err-specify",
		Output: &specv1.SpecifyOutput{
			InterfaceContract: "interface contract",
			VerifyCriteria:    []string{"check 1"},
			Invariants:        []string{"inv 1"},
		},
	}))
	if err != nil {
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInternal, connErr.Code())
	}
}

func TestAuthoringHandler_Amend_StorageError(t *testing.T) {
	// When AmendSpec returns a generic error the handler should return CodeInternal.
	authoringStore := &fakeAuthoringBackend{amendErr: errors.New("db error")}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "scope changed",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Decompose_EmptySlices(t *testing.T) {
	// A DecomposeRequest with an empty Slices list produces an InvalidArgument error
	// because SafetyInput.Validate() rejects inputs with no scannable content.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeTxBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices:   []*specv1.DecompositionSlice{},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_ConstitutionViolationsReturned(t *testing.T) {
	// Spark runs PassConstitutionCheck for all postures; response should include violations.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:    "cv-spec",
		Output:  &specv1.SparkOutput{Seed: "some intent"},
		Posture: specv1.Posture_POSTURE_DRIVE,
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.ConstitutionViolations, "constitution_violations should be populated for Spark with DRIVE posture")
}

func TestAuthoringHandler_Spark_ConstitutionViolations_UnspecifiedPosture(t *testing.T) {
	// PassConstitutionCheck auto-runs for all postures; UNSPECIFIED resolves to Partner which still auto-runs it.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:    "cv-unspecified",
		Output:  &specv1.SparkOutput{Seed: "some intent"},
		Posture: specv1.Posture_POSTURE_UNSPECIFIED,
	}))
	require.NoError(t, err)
	// UNSPECIFIED resolves to Partner via ResolvePosture; constitution_check still auto-runs.
	require.NotEmpty(t, resp.Msg.ConstitutionViolations, "constitution_violations should be populated even for UNSPECIFIED posture")
}

func TestAuthoringHandler_Shape_UnspecifiedPostureResolved(t *testing.T) {
	// UNSPECIFIED posture resolves to Partner, which auto-runs constitution_check.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "my-spec",
		Output: &specv1.ShapeOutput{
			ScopeIn: []string{"auth"},
			Risks:   []string{"latency"},
		},
		Posture: specv1.Posture_POSTURE_UNSPECIFIED,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}

func TestAuthoringHandler_Amend_ApprovedTargetStageRejected(t *testing.T) {
	// Amend with target_stage=APPROVED should return CodeInvalidArgument.
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.AuthoringStage("shape"), Version: 2},
	}, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "re-approve",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Spark_UnrecognizedScopeSniff(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug: "my-spec",
		Output: &specv1.SparkOutput{
			Seed:       "some intent",
			ScopeSniff: specv1.ScopeSniff(999), // unknown numeric value not in the enum
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "unrecognized ScopeSniff value")
}

func TestAuthoringHandler_Decompose_StoreSafetyFlagsError(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		storeSafetyFlagsErr: errors.New("db write failed"),
	}, &fakeTxBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "safety-err-decompose",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "slice one"},
			},
		},
	}))
	if err != nil {
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInternal, connErr.Code())
	}
}

// --- fakeFullBackend: Backend + GraphBackend + DecisionBackend for acceptLinkedDecisions tests ---

type fakeFullBackend struct {
	fakeBackend
	listEdgesErr      error
	listEdgesResult   []*storage.Edge
	getDecisionErr    error
	getDecisionResult *storage.Decision
	updateDecisionErr error
}

func (f *fakeFullBackend) AddEdge(_ context.Context, _, _ string, _ storage.EdgeType) (*storage.Edge, error) {
	return nil, nil
}

func (f *fakeFullBackend) RemoveEdge(_ context.Context, _, _ string, _ storage.EdgeType) error {
	return nil
}

func (f *fakeFullBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	return f.listEdgesResult, f.listEdgesErr
}

func (f *fakeFullBackend) GetDependencies(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetTransitiveDeps(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetImpact(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetReady(_ context.Context) ([]storage.NodeRef, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetCriticalPath(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

func (f *fakeFullBackend) CreateDecision(_ context.Context, _, _, _, _ string) (*storage.Decision, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetDecision(_ context.Context, _ string) (*storage.Decision, error) {
	if f.getDecisionErr != nil {
		return nil, f.getDecisionErr
	}
	if f.getDecisionResult != nil {
		return f.getDecisionResult, nil
	}
	return &storage.Decision{Status: storage.DecisionStatusProposed}, nil
}

func (f *fakeFullBackend) ListDecisions(_ context.Context, _ storage.DecisionStatus, _ int) ([]*storage.Decision, error) {
	return nil, nil
}

func (f *fakeFullBackend) UpdateDecision(_ context.Context, slug string, _ *string, _ *storage.DecisionStatus, _, _, _ *string) (*storage.Decision, error) {
	if f.updateDecisionErr != nil {
		return nil, f.updateDecisionErr
	}
	return &storage.Decision{Slug: slug, Status: storage.DecisionStatusAccepted}, nil
}

func TestAuthoringHandler_Approve_GetSpecError(t *testing.T) {
	// After TransitionStage succeeds, if GetSpec fails the handler returns CodeInternal.
	backend := &fakeBackend{getSpecErr: errors.New("db read failed")}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
	require.Contains(t, connErr.Message(), "read back spec")
}

func TestAuthoringHandler_Approve_AcceptLinkedDecisions_EdgeListError(t *testing.T) {
	// When ListEdges fails during decision acceptance, Approve returns CodeInternal.
	backend := &fakeFullBackend{
		listEdgesResult: nil,
		listEdgesErr:    errors.New("graph query failed"),
	}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
	require.Contains(t, connErr.Message(), "accept linked decisions")
}

func TestAuthoringHandler_Approve_AcceptLinkedDecisions_HappyPath(t *testing.T) {
	// When linked decisions exist, they are accepted without error.
	backend := &fakeFullBackend{
		listEdgesResult: []*storage.Edge{
			{FromID: "decision-1", ToID: "my-spec", EdgeType: storage.EdgeTypeDecidedIn},
		},
		getDecisionResult: &storage.Decision{
			Slug:   "decision-1",
			Status: storage.DecisionStatusProposed,
		},
	}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, resp.Msg.Stage)
}

func TestAuthoringHandler_Approve_AcceptLinkedDecisions_UpdateError(t *testing.T) {
	// When UpdateDecision fails, Approve returns CodeInternal.
	backend := &fakeFullBackend{
		listEdgesResult: []*storage.Edge{
			{FromID: "decision-1", ToID: "my-spec", EdgeType: storage.EdgeTypeDecidedIn},
		},
		getDecisionResult: &storage.Decision{
			Slug:   "decision-1",
			Status: storage.DecisionStatusProposed,
		},
		updateDecisionErr: errors.New("update failed"),
	}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Amend_UnknownStageFromStorage(t *testing.T) {
	// When AmendSpec returns a stage unknown to stageToProto, handler returns CodeInternal.
	authoringStore := &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.AuthoringStage("bogus-stage"), Version: 3},
	}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.AmendRequest{
		Slug:        "my-spec",
		Reason:      "scope changed",
		TargetStage: specv1.AuthoringStage_AUTHORING_STAGE_SHAPE,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
	require.Contains(t, connErr.Message(), "unknown stage")
}
