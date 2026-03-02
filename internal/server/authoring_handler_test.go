// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
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
	transitionStageErr    error
	storeSparkOutputErr   error
	storeShapeOutputErr   error
	storeSpecifyOutputErr error
	supersedeErr          error
	amendErr              error
	amendResult           *specv1.Spec
}

func (f *fakeAuthoringBackend) TransitionStage(_ context.Context, _ string, _, _ string) error {
	return f.transitionStageErr
}

func (f *fakeAuthoringBackend) StoreSparkOutput(_ context.Context, _ string, _ *specv1.SparkOutput) error {
	return f.storeSparkOutputErr
}

func (f *fakeAuthoringBackend) StoreShapeOutput(_ context.Context, _ string, _ *specv1.ShapeOutput) error {
	return f.storeShapeOutputErr
}

func (f *fakeAuthoringBackend) StoreSpecifyOutput(_ context.Context, _ string, _ *specv1.SpecifyOutput) error {
	return f.storeSpecifyOutputErr
}

func (f *fakeAuthoringBackend) StoreDecomposeOutput(_ context.Context, _ string, _ *specv1.DecomposeOutput) ([]*specv1.Spec, error) {
	return nil, nil
}

func (f *fakeAuthoringBackend) StoreRedTeamFindings(_ context.Context, _ string, _ []*specv1.RedTeamFinding) error {
	return nil
}

func (f *fakeAuthoringBackend) StorePeripheralVision(_ context.Context, _ string, _ []*specv1.PeripheralVisionItem) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreConsistencyIssues(_ context.Context, _ string, _ []*specv1.ConsistencyIssue) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreSimplicityFindings(_ context.Context, _ string, _ []*specv1.SimplicityFinding) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreSafetyFlags(_ context.Context, _ string, _ []*specv1.SafetyFlag) error {
	return nil
}

func (f *fakeAuthoringBackend) StoreConstitutionViolations(_ context.Context, _ string, _ []*specv1.ConstitutionViolation) error {
	return nil
}

func (f *fakeAuthoringBackend) SupersedeSpec(_ context.Context, _, _, _ string) error {
	return f.supersedeErr
}

func (f *fakeAuthoringBackend) AmendSpec(_ context.Context, _, _, _ string) (*specv1.Spec, error) {
	return f.amendResult, f.amendErr
}

// fakeBackend is a minimal fake implementation of storage.Backend for testing.
type fakeBackend struct {
	createSpecErr    error
	createSpecResult *specv1.Spec
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
	server.RegisterAuthoringService(mux, nil, nil)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: "spark",
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
