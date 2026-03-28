// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ---------------------------------------------------------------------------
// Spark
// ---------------------------------------------------------------------------

type fakeSparkHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeSparkHandler) Spark(_ context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	return connect.NewResponse(&specv1.SparkResponse{
		Output: &specv1.SparkOutput{Seed: req.Msg.Output.GetSeed()},
	}), nil
}

func TestRunSpark_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeSparkHandler{})
	err := runSpark(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

type fakeSparkWithPromptsHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeSparkWithPromptsHandler) Spark(_ context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	return connect.NewResponse(&specv1.SparkResponse{
		Output: &specv1.SparkOutput{Seed: req.Msg.Output.GetSeed()},
		SafetyFlags: []*specv1.SafetyFlag{
			{
				Category:    specv1.SafetyCategory_SAFETY_CATEGORY_SECURITY,
				Severity:    specv1.FindingSeverity_FINDING_SEVERITY_WARNING,
				Description: "possible credential exposure",
			},
		},
		NextPrompts: []*specv1.PromptTemplate{
			{Name: "scope", Template: "Define scope for {{slug}}"},
		},
	}), nil
}

func TestRunSpark_WithSeedAndPrompts(t *testing.T) {
	startFakeAuthoringServer(t, fakeSparkWithPromptsHandler{})

	old := sparkSeed
	sparkSeed = "test seed idea"
	t.Cleanup(func() { sparkSeed = old })

	err := runSpark(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

type fakeSparkErrorHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeSparkErrorHandler) Spark(context.Context, *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

func TestRunSpark_RPCError(t *testing.T) {
	startFakeAuthoringServer(t, fakeSparkErrorHandler{})
	err := runSpark(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spark:")
}

func TestRunSpark_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSpark(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestSparkCmd_RequiresSlug(t *testing.T) {
	err := sparkCmd.Args(sparkCmd, []string{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Shape
// ---------------------------------------------------------------------------

type fakeShapeHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeShapeHandler) Shape(_ context.Context, _ *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	return connect.NewResponse(&specv1.ShapeResponse{}), nil
}

func TestRunShape_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeShapeHandler{})
	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunShape_WithJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeShapeHandler{})

	path := writeJSONFile(t, `{"scopeIn":["feature A"],"risks":["tight deadline"]}`)
	old := shapeJSONFile
	shapeJSONFile = path
	t.Cleanup(func() { shapeJSONFile = old })

	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunShape_InvalidJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeShapeHandler{})

	path := writeJSONFile(t, `{not valid json}`)
	old := shapeJSONFile
	shapeJSONFile = path
	t.Cleanup(func() { shapeJSONFile = old })

	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shape:")
}

func TestRunShape_MissingJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeShapeHandler{})

	old := shapeJSONFile
	shapeJSONFile = t.TempDir() + "/no-such-file.json"
	t.Cleanup(func() { shapeJSONFile = old })

	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shape:")
}

type fakeShapeErrorHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeShapeErrorHandler) Shape(context.Context, *connect.Request[specv1.ShapeRequest]) (*connect.Response[specv1.ShapeResponse], error) {
	return nil, connect.NewError(connect.CodeInternal, nil)
}

func TestRunShape_RPCError(t *testing.T) {
	startFakeAuthoringServer(t, fakeShapeErrorHandler{})
	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "shape:")
}

func TestRunShape_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runShape(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestShapeCmd_RequiresSlug(t *testing.T) {
	err := shapeCmd.Args(shapeCmd, []string{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Specify
// ---------------------------------------------------------------------------

type fakeSpecifyHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeSpecifyHandler) Specify(_ context.Context, _ *connect.Request[specv1.SpecifyRequest]) (*connect.Response[specv1.SpecifyResponse], error) {
	return connect.NewResponse(&specv1.SpecifyResponse{}), nil
}

func TestRunSpecify_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeSpecifyHandler{})
	err := runSpecify(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunSpecify_WithJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeSpecifyHandler{})

	path := writeJSONFile(t, `{"invariants":["no data loss"]}`)
	old := specifyJSONFile
	specifyJSONFile = path
	t.Cleanup(func() { specifyJSONFile = old })

	err := runSpecify(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunSpecify_InvalidJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeSpecifyHandler{})

	path := writeJSONFile(t, `<<< bad >>>`)
	old := specifyJSONFile
	specifyJSONFile = path
	t.Cleanup(func() { specifyJSONFile = old })

	err := runSpecify(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specify:")
}

func TestRunSpecify_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runSpecify(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestSpecifyCmd_RequiresSlug(t *testing.T) {
	err := specifyCmd.Args(specifyCmd, []string{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Decompose
// ---------------------------------------------------------------------------

type fakeDecomposeHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeDecomposeHandler) Decompose(_ context.Context, _ *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	return connect.NewResponse(&specv1.DecomposeResponse{}), nil
}

func TestRunDecompose_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeDecomposeHandler{})
	err := runDecompose(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

type fakeDecomposeWithSlicesHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeDecomposeWithSlicesHandler) Decompose(_ context.Context, _ *connect.Request[specv1.DecomposeRequest]) (*connect.Response[specv1.DecomposeResponse], error) {
	return connect.NewResponse(&specv1.DecomposeResponse{
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "slice-1", Intent: "implement core logic"},
				{Id: "slice-2", Intent: "add tests"},
			},
		},
	}), nil
}

func TestRunDecompose_WithSlices(t *testing.T) {
	startFakeAuthoringServer(t, fakeDecomposeWithSlicesHandler{})
	err := runDecompose(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunDecompose_InvalidJSONFile(t *testing.T) {
	startFakeAuthoringServer(t, fakeDecomposeHandler{})

	path := writeJSONFile(t, `{broken`)
	old := decomposeJSONFile
	decomposeJSONFile = path
	t.Cleanup(func() { decomposeJSONFile = old })

	err := runDecompose(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decompose:")
}

func TestRunDecompose_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runDecompose(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestDecomposeCmd_RequiresSlug(t *testing.T) {
	err := decomposeCmd.Args(decomposeCmd, []string{})
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// Approve
// ---------------------------------------------------------------------------

type fakeApproveHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeApproveHandler) Approve(_ context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	return connect.NewResponse(&specv1.ApproveResponse{
		Slug:       req.Msg.GetSlug(),
		ApprovedAt: timestamppb.Now(),
	}), nil
}

func TestRunApprove_HappyPath(t *testing.T) {
	startFakeAuthoringServer(t, fakeApproveHandler{})
	err := runApprove(newCmdWithCtx(), []string{"my-spec"})
	require.NoError(t, err)
}

type fakeApproveErrorHandler struct {
	specgraphv1connect.UnimplementedAuthoringServiceHandler
}

func (fakeApproveErrorHandler) Approve(context.Context, *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	return nil, connect.NewError(connect.CodeFailedPrecondition, nil)
}

func TestRunApprove_RPCError(t *testing.T) {
	startFakeAuthoringServer(t, fakeApproveErrorHandler{})
	err := runApprove(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "approve:")
}

func TestRunApprove_ClientError(t *testing.T) {
	setMissingConfig(t)
	err := runApprove(newCmdWithCtx(), []string{"my-spec"})
	require.Error(t, err)
}

func TestApproveCmd_RequiresSlug(t *testing.T) {
	err := approveCmd.Args(approveCmd, []string{})
	require.Error(t, err)
}
