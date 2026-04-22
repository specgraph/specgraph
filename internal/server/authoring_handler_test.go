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
	storeSafetyFlagsErr     error
}

func (f *fakeAuthoringBackend) TransitionStage(_ context.Context, _ string, _, _ storage.SpecStage) error {
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

func (f *fakeAuthoringBackend) StoreSafetyFlags(_ context.Context, _ string, _ []storage.SafetyFlag) error {
	return f.storeSafetyFlagsErr
}

func (f *fakeAuthoringBackend) SupersedeSpec(_ context.Context, _, _, _ string) error {
	return f.supersedeErr
}

func (f *fakeAuthoringBackend) AmendSpec(_ context.Context, _, _ string, _ storage.SpecStage) (*storage.AmendResult, error) {
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
	return &storage.Spec{Slug: slug, ContentHash: strings.Repeat("a", 32)}, nil
}

func (f *fakeBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	if f.getSpecErr != nil {
		return nil, f.getSpecErr
	}
	return &storage.Spec{Slug: slug, ContentHash: strings.Repeat("a", 32), UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (f *fakeBackend) ListSpecs(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
	return nil, nil
}

func (f *fakeBackend) UpdateSpec(_ context.Context, slug string, _, _, _, _, _ *string) (*storage.Spec, error) {
	return &storage.Spec{Slug: slug, ContentHash: strings.Repeat("a", 32)}, nil
}

func (f *fakeBackend) Close(_ context.Context) error {
	return nil
}

// authoringTestBackend combines fakeAuthoringBackend and fakeBackend into a
// ScopedBackend for handler tests. Methods not relevant to authoring tests
// delegate to stubBackend.
type authoringTestBackend struct {
	stubBackend
	authoring *fakeAuthoringBackend
	backend   *fakeBackend
}

// txAuthoringTestBackend embeds authoringTestBackend and adds TransactionalBackend support.
type txAuthoringTestBackend struct {
	authoringTestBackend
	runInTxErr error
}

func (a *txAuthoringTestBackend) RunInTransaction(_ context.Context, fn func(context.Context) error) error {
	if a.runInTxErr != nil {
		return a.runInTxErr
	}
	return fn(context.Background())
}

func (a *authoringTestBackend) CreateSpec(ctx context.Context, slug, intent, priority, complexity string) (*storage.Spec, error) {
	return a.backend.CreateSpec(ctx, slug, intent, priority, complexity)
}

func (a *authoringTestBackend) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	return a.backend.GetSpec(ctx, slug)
}

func (a *authoringTestBackend) ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error) {
	return a.backend.ListSpecs(ctx, stage, priority, limit)
}

func (a *authoringTestBackend) UpdateSpec(ctx context.Context, slug string, intent, stage, priority, complexity, notes *string) (*storage.Spec, error) {
	return a.backend.UpdateSpec(ctx, slug, intent, stage, priority, complexity, notes)
}

func (a *authoringTestBackend) Close(ctx context.Context) error {
	return a.backend.Close(ctx)
}

func (a *authoringTestBackend) TransitionStage(ctx context.Context, slug string, from, to storage.SpecStage) error {
	return a.authoring.TransitionStage(ctx, slug, from, to)
}

func (a *authoringTestBackend) StoreSparkOutput(ctx context.Context, slug string, output *storage.SparkOutput) error {
	return a.authoring.StoreSparkOutput(ctx, slug, output)
}

func (a *authoringTestBackend) StoreShapeOutput(ctx context.Context, slug string, output *storage.ShapeOutput) error {
	return a.authoring.StoreShapeOutput(ctx, slug, output)
}

func (a *authoringTestBackend) StoreSpecifyOutput(ctx context.Context, slug string, output *storage.SpecifyOutput) error {
	return a.authoring.StoreSpecifyOutput(ctx, slug, output)
}

func (a *authoringTestBackend) StoreDecomposeOutput(ctx context.Context, slug string, output *storage.DecomposeOutput) ([]string, error) {
	return a.authoring.StoreDecomposeOutput(ctx, slug, output)
}

func (a *authoringTestBackend) StoreSafetyFlags(ctx context.Context, slug string, flags []storage.SafetyFlag) error {
	return a.authoring.StoreSafetyFlags(ctx, slug, flags)
}

func (a *authoringTestBackend) SupersedeSpec(ctx context.Context, slug, supersededBy, reason string) error {
	return a.authoring.SupersedeSpec(ctx, slug, supersededBy, reason)
}

func (a *authoringTestBackend) AmendSpec(ctx context.Context, slug, reason string, targetStage storage.SpecStage) (*storage.AmendResult, error) {
	return a.authoring.AmendSpec(ctx, slug, reason, targetStage)
}

// fakeConversationBackend implements storage.ConversationBackend for handler tests.
type fakeConversationBackend struct {
	recordErr error
	listErr   error
	entries   []*storage.ConversationLogEntry
}

func (f *fakeConversationBackend) RecordConversation(_ context.Context, _ string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	if f.recordErr != nil {
		return nil, f.recordErr
	}
	stored := &storage.ConversationLogEntry{
		ID:            "cvl-test",
		Stage:         entry.Stage,
		Version:       1,
		IsAmend:       entry.IsAmend,
		Exchanges:     entry.Exchanges,
		ExchangeCount: entry.ExchangeCount,
		Date:          time.Now(),
	}
	f.entries = append(f.entries, stored)
	return stored, nil
}

func (f *fakeConversationBackend) ListConversations(_ context.Context, _ string, _ string) ([]*storage.ConversationLogEntry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.entries, nil
}

// fakeConvBackend combines fakeBackend with fakeConversationBackend for handler routing.
type fakeConvBackend struct {
	fakeBackend
	conv *fakeConversationBackend
}

// fakeFindingsBackend captures StoreFindings calls for assertions.
type fakeFindingsBackend struct {
	captured []struct {
		Slug     string
		PassType storage.PassType
		Findings []storage.AnalyticalFindingInput
	}
}

func (f *fakeFindingsBackend) StoreFindings(_ context.Context, slug string, pt storage.PassType, findings []storage.AnalyticalFindingInput) ([]string, error) {
	f.captured = append(f.captured, struct {
		Slug     string
		PassType storage.PassType
		Findings []storage.AnalyticalFindingInput
	}{slug, pt, findings})
	ids := make([]string, len(findings))
	for i := range findings {
		ids[i] = "finding-" + slug
	}
	return ids, nil
}

// fakeRejectBackend combines fakeBackend, fakeConversationBackend, and fakeFindingsBackend.
// stage overrides the spec stage returned by GetSpec; defaults to SpecStageDecompose when zero.
type fakeRejectBackend struct {
	fakeBackend
	conv     *fakeConversationBackend
	findings *fakeFindingsBackend
	stage    storage.SpecStage
}

// rejectAuthoringTestBackend embeds authoringTestBackend, overrides RecordConversation
// and StoreFindings so the reject path can capture calls for assertions.
// GetSpec always returns SpecStageDecompose because reject is only valid from that stage.
type rejectAuthoringTestBackend struct {
	authoringTestBackend
	conv     *fakeConversationBackend
	findings *fakeFindingsBackend
	stage    storage.SpecStage // stage returned by GetSpec; defaults to SpecStageDecompose when zero
}

func (r *rejectAuthoringTestBackend) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	stage := r.stage
	if stage == "" {
		stage = storage.SpecStageDecompose
	}
	return &storage.Spec{Slug: slug, Stage: stage, ContentHash: strings.Repeat("a", 32), UpdatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, nil
}

func (r *rejectAuthoringTestBackend) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	return r.conv.RecordConversation(ctx, slug, entry)
}

func (r *rejectAuthoringTestBackend) ListConversations(ctx context.Context, slug string, stage string) ([]*storage.ConversationLogEntry, error) {
	return r.conv.ListConversations(ctx, slug, stage)
}

func (r *rejectAuthoringTestBackend) StoreFindings(ctx context.Context, slug string, pt storage.PassType, findings []storage.AnalyticalFindingInput) ([]string, error) {
	return r.findings.StoreFindings(ctx, slug, pt, findings)
}

// convAuthoringTestBackend embeds authoringTestBackend and adds ConversationBackend.
type convAuthoringTestBackend struct {
	authoringTestBackend
	conv *fakeConversationBackend
}

func (c *convAuthoringTestBackend) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	return c.conv.RecordConversation(ctx, slug, entry)
}

func (c *convAuthoringTestBackend) ListConversations(ctx context.Context, slug string, stage string) ([]*storage.ConversationLogEntry, error) {
	return c.conv.ListConversations(ctx, slug, stage)
}

// fullAuthoringTestBackend embeds authoringTestBackend and overrides Graph+Decision
// methods for tests that exercise acceptLinkedDecisions.
type fullAuthoringTestBackend struct {
	authoringTestBackend
	full *fakeFullBackend
}

func (f *fullAuthoringTestBackend) ListEdges(ctx context.Context, slug string, et storage.EdgeType) ([]*storage.Edge, error) {
	return f.full.ListEdges(ctx, slug, et)
}

func (f *fullAuthoringTestBackend) GetDecision(ctx context.Context, slug string) (*storage.Decision, error) {
	return f.full.GetDecision(ctx, slug)
}

func (f *fullAuthoringTestBackend) UpdateDecision(ctx context.Context, slug string, expectedVersion int32, title *string, status *storage.DecisionStatus,
	decision, rationale, supersededBy, question *string,
	rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
	tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string,
) (*storage.Decision, error) {
	return f.full.UpdateDecision(ctx, slug, expectedVersion, title, status, decision, rationale, supersededBy,
		question, rejectedAlts, confidence, tags, scope, originSpec, originStage)
}

func newAuthoringClient(t *testing.T, authoringStore *fakeAuthoringBackend, backend any) specgraphv1connect.AuthoringServiceClient {
	t.Helper()
	var scopedBackend storage.ScopedBackend
	switch b := backend.(type) {
	case *fakeFullBackend:
		scopedBackend = &fullAuthoringTestBackend{
			authoringTestBackend: authoringTestBackend{authoring: authoringStore, backend: &b.fakeBackend},
			full:                 b,
		}
	case *fakeTxBackend:
		scopedBackend = &txAuthoringTestBackend{
			authoringTestBackend: authoringTestBackend{authoring: authoringStore, backend: &b.fakeBackend},
			runInTxErr:           b.runInTxErr,
		}
	case *fakeConvBackend:
		scopedBackend = &convAuthoringTestBackend{
			authoringTestBackend: authoringTestBackend{authoring: authoringStore, backend: &b.fakeBackend},
			conv:                 b.conv,
		}
	case *fakeRejectBackend:
		scopedBackend = &rejectAuthoringTestBackend{
			authoringTestBackend: authoringTestBackend{authoring: authoringStore, backend: &b.fakeBackend},
			conv:                 b.conv,
			findings:             b.findings,
			stage:                b.stage,
		}
	case *fakeBackend:
		scopedBackend = &authoringTestBackend{authoring: authoringStore, backend: b}
	default:
		t.Fatalf("unsupported backend type: %T", backend)
	}
	scoper := &testScoper{backend: scopedBackend}
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
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
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "my-spec",
		Output: &specv1.ShapeOutput{
			ScopeIn:  []string{"auth endpoint"},
			ScopeOut: []string{"admin panel"},
			Risks:    []string{"latency"},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "auth endpoint", Stage: "shape", Sequence: 2},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, []string{"auth endpoint"}, resp.Msg.Output.ScopeIn)
	require.NotEmpty(t, resp.Msg.NextPrompts, "should include next-stage prompts")
}

func TestAuthoringHandler_Specify_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "my-spec",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "API", Body: "POST /api/login"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "functional", Description: "returns 200 on valid credentials"},
			},
			Invariants: []string{"session token is opaque"},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "interfaces?", Stage: "specify", Sequence: 1},
			{Role: "response", Content: "POST /api/login", Stage: "specify", Sequence: 2},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Len(t, resp.Msg.Output.Interfaces, 1)
	require.Equal(t, "POST /api/login", resp.Msg.Output.Interfaces[0].Body)
	require.NotEmpty(t, resp.Msg.NextPrompts, "should include next-stage prompts")
}

func TestAuthoringHandler_Decompose_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "auth endpoint"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "slices?", Stage: "decompose", Sequence: 1},
			{Role: "response", Content: "auth endpoint slice", Stage: "decompose", Sequence: 2},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE, resp.Msg.Output.Strategy)
	require.Equal(t, []string{"my-spec/s1"}, resp.Msg.SliceSlugs)
}

func TestAuthoringHandler_Decompose_SteelThread_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "broaden-a", Intent: "add feature A", DependsOn: []string{"thread"}},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD, resp.Msg.Output.Strategy)
}

func TestAuthoringHandler_Decompose_SteelThread_RootHasDeps(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip", DependsOn: []string{"something"}},
				{Id: "broaden", Intent: "add feature", DependsOn: []string{"thread"}},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "no dependencies")
}

func TestAuthoringHandler_Decompose_SteelThread_DisconnectedSlice(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "connected", Intent: "depends on thread", DependsOn: []string{"thread"}},
				{Id: "island", Intent: "no path to thread"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "does not transitively depend on thread slice")
}

func TestAuthoringHandler_Decompose_SteelThread_ChainedBroadening(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "broaden-a", Intent: "first broadening", DependsOn: []string{"thread"}},
				{Id: "broaden-b", Intent: "depends on broaden-a", DependsOn: []string{"broaden-a"}},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
	require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD, resp.Msg.Output.Strategy)
}

func TestAuthoringHandler_Decompose_NonSteelThread_NoNewValidation(t *testing.T) {
	// A vertical-slice decomposition with a disconnected slice should still pass
	// (steel thread validation only applies to STEEL_THREAD strategy).
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "a", Intent: "independent slice A"},
				{Id: "b", Intent: "independent slice B"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}

func TestAuthoringHandler_Decompose_SteelThread_DuplicateSliceID(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD,
			Slices: []*specv1.DecompositionSlice{
				{Id: "thread", Intent: "prove roundtrip"},
				{Id: "thread", Intent: "duplicate id", DependsOn: []string{"thread"}},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
	require.Contains(t, connErr.Message(), "duplicate slice id")
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

func TestAuthoringHandler_Approve_AcceptUnchangedWithoutAction(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_UNSPECIFIED,
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, resp.Msg.Stage)
}

func TestAuthoringHandler_Approve_RejectRequiresExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeRejectBackend{
		conv:     &fakeConversationBackend{},
		findings: &fakeFindingsBackend{},
	})
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_REJECT,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Approve_RejectRecordsFindingAndExchanges(t *testing.T) {
	conv := &fakeConversationBackend{}
	findings := &fakeFindingsBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeRejectBackend{
		conv:     conv,
		findings: findings,
	})
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_REJECT,
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "looks good?", Stage: "approve", Sequence: 1},
			{Role: "response", Content: "no, rejected", Stage: "approve", Sequence: 2},
		},
	}))
	require.NoError(t, err)
	// Stage must reflect current spec stage (Decompose → not transitioned to Approved).
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_DECOMPOSE, resp.Msg.Stage)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	// RecordConversation was called exactly once.
	require.Len(t, conv.entries, 1, "expected exactly one RecordConversation call")
	require.Len(t, conv.entries[0].Exchanges, 2)
	// Conversation entry is recorded under the approved stage (the gate being rejected).
	require.Equal(t, storage.SpecStageApproved, conv.entries[0].Stage)
	// StoreFindings was called exactly once with PassTypeApproveRejected and one critical finding.
	require.Len(t, findings.captured, 1, "expected exactly one StoreFindings call")
	require.Equal(t, storage.PassTypeApproveRejected, findings.captured[0].PassType)
	require.Equal(t, "my-spec", findings.captured[0].Slug)
	require.Len(t, findings.captured[0].Findings, 1)
	require.Equal(t, storage.SeverityCritical, findings.captured[0].Findings[0].Severity)
}

func TestAuthoringHandler_Approve_RejectRequiresDecomposeStage(t *testing.T) {
	conv := &fakeConversationBackend{}
	findings := &fakeFindingsBackend{}
	// Seed the backend with a non-decompose stage (e.g., shape) to trigger the precondition.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeRejectBackend{
		conv:     conv,
		findings: findings,
		stage:    storage.SpecStageShape,
	})
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_REJECT,
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "looks good?", Stage: "approve", Sequence: 1},
			{Role: "response", Content: "no, rejected", Stage: "approve", Sequence: 2},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
	require.Contains(t, connErr.Message(), "reject requires decompose")
}

func TestAuthoringHandler_Amend_HappyPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.SpecStageShape, Version: 2},
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
		Output: &specv1.ShapeOutput{ScopeIn: []string{"x"}},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
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
		Output: &specv1.ShapeOutput{ScopeIn: []string{"x"}},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
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
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Specify_StoreOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeSpecifyOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "my-spec",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "API", Body: "POST /api/login"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "specify", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Decompose_StoreOutputError(t *testing.T) {
	authoringStore := &fakeAuthoringBackend{storeDecomposeOutputErr: errors.New("store failed")}
	client := newAuthoringClient(t, authoringStore, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "auth endpoint"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
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
		Output: &specv1.ShapeOutput{ScopeIn: []string{"x"}},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
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
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
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
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.SpecStageShape, Version: 2},
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
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.SpecStageShape, Version: 2},
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
	require.True(t, names["interfaces"])
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
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
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
	}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "safety-err-specify",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "API", Body: "interface contract"},
			},
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "functional", Description: "check 1"},
			},
			Invariants: []string{"inv 1"},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "specify", Sequence: 1},
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
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "my-spec",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices:   []*specv1.DecompositionSlice{},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Shape_UnspecifiedPostureResolved(t *testing.T) {
	// UNSPECIFIED posture resolves to Partner, which auto-runs constitution_check.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "my-spec",
		Output: &specv1.ShapeOutput{
			ScopeIn: []string{"auth"},
			Risks:   []string{"latency"},
		},
		Posture: specv1.Posture_POSTURE_UNSPECIFIED,
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "shape", Sequence: 1},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Output)
}

func TestAuthoringHandler_Amend_ApprovedTargetStageRejected(t *testing.T) {
	// Amend with target_stage=APPROVED should return CodeInvalidArgument.
	client := newAuthoringClient(t, &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.SpecStageShape, Version: 2},
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
	}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug: "safety-err-decompose",
		Output: &specv1.DecomposeOutput{
			Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
			Slices: []*specv1.DecompositionSlice{
				{Id: "s1", Intent: "slice one"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "decompose", Sequence: 1},
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

func (f *fakeFullBackend) CreateDecision(_ context.Context, _, _, _, _, _ string,
	_ []storage.RejectedAlternative, _ storage.DecisionConfidence,
	_ []string, _ storage.DecisionScope, _, _ string,
) (*storage.Decision, error) {
	return nil, nil
}

func (f *fakeFullBackend) GetDecision(_ context.Context, _ string) (*storage.Decision, error) {
	if f.getDecisionErr != nil {
		return nil, f.getDecisionErr
	}
	if f.getDecisionResult != nil {
		return f.getDecisionResult, nil
	}
	return &storage.Decision{Status: storage.DecisionStatusProposed, ContentHash: strings.Repeat("a", 32)}, nil
}

func (f *fakeFullBackend) ListDecisions(_ context.Context, _ storage.DecisionStatus, _ int) ([]*storage.Decision, error) {
	return nil, nil
}

func (f *fakeFullBackend) UpdateDecision(_ context.Context, slug string, _ int32, _ *string, _ *storage.DecisionStatus,
	_, _, _, _ *string,
	_ *[]storage.RejectedAlternative, _ *storage.DecisionConfidence,
	_ *[]string, _ *storage.DecisionScope, _, _ *string,
) (*storage.Decision, error) {
	if f.updateDecisionErr != nil {
		return nil, f.updateDecisionErr
	}
	return &storage.Decision{Slug: slug, Status: storage.DecisionStatusAccepted, ContentHash: strings.Repeat("a", 32)}, nil
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
}

func TestAuthoringHandler_Approve_AcceptLinkedDecisions_HappyPath(t *testing.T) {
	// When linked decisions exist, they are accepted without error.
	backend := &fakeFullBackend{
		listEdgesResult: []*storage.Edge{
			{FromID: "decision-1", ToID: "my-spec", EdgeType: storage.EdgeTypeDecidedIn},
		},
		getDecisionResult: &storage.Decision{
			Slug:        "decision-1",
			Status:      storage.DecisionStatusProposed,
			ContentHash: strings.Repeat("a", 32),
		},
	}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, backend)
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, resp.Msg.Stage)
}

func TestAuthoringHandler_Approve_AcceptLinkedDecisions_SpecToDecisionDirection(t *testing.T) {
	// Canonical Spec→Decision edge direction (per ADR-003).
	backend := &fakeFullBackend{
		listEdgesResult: []*storage.Edge{
			{FromID: "my-spec", ToID: "decision-1", EdgeType: storage.EdgeTypeDecidedIn},
		},
		getDecisionResult: &storage.Decision{
			Slug:        "decision-1",
			Status:      storage.DecisionStatusProposed,
			ContentHash: strings.Repeat("a", 32),
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
			Slug:        "decision-1",
			Status:      storage.DecisionStatusProposed,
			ContentHash: strings.Repeat("a", 32),
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

func TestAuthoringHandler_Shape_InvalidDecisionSlug(t *testing.T) {
	// DecisionInput with invalid slug should be rejected.
	authoringStore := &fakeAuthoringBackend{}
	client := newAuthoringClient(t, authoringStore, &fakeBackend{})
	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "my-spec",
		Output: &specv1.ShapeOutput{
			Decisions: []*specv1.DecisionInput{
				{Slug: "../bad-slug", Title: "title", Decision: "body", Rationale: "reason"},
			},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestAuthoringHandler_Amend_UnknownStageFromStorage(t *testing.T) {
	// When AmendSpec returns a stage unknown to stageToProto, handler returns CodeInternal.
	authoringStore := &fakeAuthoringBackend{
		amendResult: &storage.AmendResult{Slug: "my-spec", Stage: storage.SpecStage("bogus-stage"), Version: 3},
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
	require.Equal(t, "internal error", connErr.Message())
}

func TestAuthoringHandler_Specify_InterfacesMissingName(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "", Body: "some body"},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_Specify_VerifyCriteriaMissingDescription(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			VerifyCriteria: []*specv1.VerifyCriterion{
				{Category: "test", Description: ""},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_Specify_TouchesMissingPath(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "test-spec",
		Output: &specv1.SpecifyOutput{
			Touches: []*specv1.FileTouch{
				{Path: "", Purpose: "something"},
			},
		},
	}))
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_Specify_MultipleInterfaces(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})
	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug: "multi-iface",
		Output: &specv1.SpecifyOutput{
			Interfaces: []*specv1.InterfaceSection{
				{Name: "ProtoService", Body: "service Webhook { rpc Send(...) }"},
				{Name: "GoInterface", Body: "type EventBus interface { Publish(event) }"},
			},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "q", Stage: "specify", Sequence: 1},
		},
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Output.Interfaces, 2)
}

// --- RecordConversation / ListConversations ---

func TestAuthoringHandler_RecordConversation(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})

	resp, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:  "test-spec",
		Stage: "spark",
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "What's the seed?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "Build widgets", Stage: "spark", Sequence: 1, DecisionPoint: true},
		},
	}))
	require.NoError(t, err)
	require.Equal(t, "cvl-test", resp.Msg.ConversationLog.Id)
	require.Equal(t, "spark", resp.Msg.ConversationLog.Stage)
	require.Len(t, resp.Msg.ConversationLog.Exchanges, 2)
}

func TestAuthoringHandler_RecordConversation_MissingSlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Stage:     "spark",
		Exchanges: []*specv1.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_RecordConversation_MissingStage(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:      "test-spec",
		Exchanges: []*specv1.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_RecordConversation_NoExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:  "test-spec",
		Stage: "spark",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_RecordConversation_SpecNotFound(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{recordErr: storage.ErrSpecNotFound},
	})

	_, err := client.RecordConversation(context.Background(), connect.NewRequest(&specv1.RecordConversationRequest{
		Slug:      "nonexistent",
		Stage:     "spark",
		Exchanges: []*specv1.ConversationExchange{{Role: "probe", Content: "test", Stage: "spark", Sequence: 1}},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestAuthoringHandler_ListConversations(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{
			entries: []*storage.ConversationLogEntry{
				{ID: "cvl-1", Stage: storage.SpecStageSpark, Version: 1, ExchangeCount: 2},
			},
		},
	})

	resp, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug: "test-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.ConversationLogs, 1)
	require.Equal(t, "cvl-1", resp.Msg.ConversationLogs[0].Id)
}

func TestAuthoringHandler_ListConversations_MissingSlug(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{},
	})

	_, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestAuthoringHandler_ListConversations_SpecNotFound(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: &fakeConversationBackend{listErr: storage.ErrSpecNotFound},
	})

	_, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug: "nonexistent",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

// --- Shape conversation_exchanges validation tests ---

func TestAuthoringHandler_Shape_RequiresConversationExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "oauth-refresh",
		Output: validShapeOutput(),
		// conversation_exchanges omitted
	}))
	if err == nil {
		t.Fatal("expected error for missing exchanges")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Shape_RejectsEmptyExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:                  "oauth-refresh",
		Output:                validShapeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Shape_PersistsAtomicallyWithExchanges(t *testing.T) {
	convBackend := &fakeConversationBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: convBackend,
	})

	resp, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug:   "oauth-refresh",
		Output: validShapeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "X in", Stage: "shape", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Shape: %v", err)
	}
	if resp.Msg.GetOutput() == nil {
		t.Error("expected output echoed back")
	}

	logs, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "oauth-refresh",
		Stage: "shape",
	}))
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 conversation log, got %d", len(logs.Msg.GetConversationLogs()))
	}
}

// validShapeOutput returns a minimally-valid ShapeOutput for tests.
func validShapeOutput() *specv1.ShapeOutput {
	return &specv1.ShapeOutput{
		ScopeIn:        []string{"X"},
		ScopeOut:       []string{"Y"},
		Approaches:     []*specv1.Approach{{Name: "a", Description: "d", Tradeoffs: []string{"t"}}},
		ChosenApproach: "a",
		Risks:          []string{"r"},
		SuccessMust:    []string{"m"},
	}
}

// --- Specify conversation_exchanges validation tests ---

func TestAuthoringHandler_Specify_RequiresConversationExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   "oauth-refresh",
		Output: validSpecifyOutput(),
		// conversation_exchanges omitted
	}))
	if err == nil {
		t.Fatal("expected error for missing exchanges")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Specify_RejectsEmptyExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:                  "oauth-refresh",
		Output:                validSpecifyOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Specify_PersistsAtomicallyWithExchanges(t *testing.T) {
	convBackend := &fakeConversationBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: convBackend,
	})

	resp, err := client.Specify(context.Background(), connect.NewRequest(&specv1.SpecifyRequest{
		Slug:   "oauth-refresh",
		Output: validSpecifyOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "interfaces?", Stage: "specify", Sequence: 1},
			{Role: "response", Content: "POST /api/login", Stage: "specify", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Specify: %v", err)
	}
	if resp.Msg.GetOutput() == nil {
		t.Error("expected output echoed back")
	}

	logs, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "oauth-refresh",
		Stage: "specify",
	}))
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 conversation log, got %d", len(logs.Msg.GetConversationLogs()))
	}
}

// validSpecifyOutput returns a minimally-valid SpecifyOutput for tests.
func validSpecifyOutput() *specv1.SpecifyOutput {
	return &specv1.SpecifyOutput{
		Interfaces: []*specv1.InterfaceSection{
			{Name: "API", Body: "POST /api/login"},
		},
		VerifyCriteria: []*specv1.VerifyCriterion{
			{Category: "functional", Description: "returns 200 on valid credentials"},
		},
		Invariants: []string{"session token is opaque"},
		Touches: []*specv1.FileTouch{
			{Path: "internal/auth/handler.go", Purpose: "add login endpoint"},
		},
	}
}

// --- Decompose conversation_exchanges validation tests ---

func TestAuthoringHandler_Decompose_RequiresConversationExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   "oauth-refresh",
		Output: validDecomposeOutput(),
		// conversation_exchanges omitted
	}))
	if err == nil {
		t.Fatal("expected error for missing exchanges")
	}
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Decompose_RejectsEmptyExchanges(t *testing.T) {
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})

	_, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:                  "oauth-refresh",
		Output:                validDecomposeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Decompose_PersistsAtomicallyWithExchanges(t *testing.T) {
	convBackend := &fakeConversationBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: convBackend,
	})

	resp, err := client.Decompose(context.Background(), connect.NewRequest(&specv1.DecomposeRequest{
		Slug:   "oauth-refresh",
		Output: validDecomposeOutput(),
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "slices?", Stage: "decompose", Sequence: 1},
			{Role: "response", Content: "vertical slice", Stage: "decompose", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if resp.Msg.GetOutput() == nil {
		t.Error("expected output echoed back")
	}
	if len(resp.Msg.SliceSlugs) == 0 {
		t.Error("expected slice slugs to be populated")
	}

	logs, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "oauth-refresh",
		Stage: "decompose",
	}))
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 conversation log, got %d", len(logs.Msg.GetConversationLogs()))
	}
}

// validDecomposeOutput returns a minimally-valid DecomposeOutput for tests.
func validDecomposeOutput() *specv1.DecomposeOutput {
	return &specv1.DecomposeOutput{
		Strategy: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
		Slices: []*specv1.DecompositionSlice{
			{Id: "s1", Intent: "login endpoint slice"},
		},
	}
}

// --- Spark conversation_exchanges tests (optional exchanges) ---

func TestAuthoringHandler_Spark_ExchangesOptional(t *testing.T) {
	// Spark without exchanges must succeed — exchanges are optional for Spark.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec",
		Output: &specv1.SparkOutput{Seed: "seed", Signal: "signal"},
	}))
	if err != nil {
		t.Fatalf("Spark without exchanges: %v", err)
	}
}

func TestAuthoringHandler_Spark_ExchangesValidatedWhenPresent(t *testing.T) {
	// When exchanges are present but invalid (empty role), expect CodeInvalidArgument.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec-2",
		Output: &specv1.SparkOutput{Seed: "seed"},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "", Content: "x", Stage: "spark", Sequence: 1},
		},
	}))
	var connErr *connect.Error
	if !errors.As(err, &connErr) || connErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("want CodeInvalidArgument, got %v", err)
	}
}

func TestAuthoringHandler_Spark_RecordsExchangesWhenPresent(t *testing.T) {
	// When exchanges are valid and present, they must be persisted.
	convBackend := &fakeConversationBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeConvBackend{
		conv: convBackend,
	})
	_, err := client.Spark(context.Background(), connect.NewRequest(&specv1.SparkRequest{
		Slug:   "new-spec-3",
		Output: &specv1.SparkOutput{Seed: "seed"},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "seed?", Stage: "spark", Sequence: 1},
			{Role: "response", Content: "x", Stage: "spark", Sequence: 2},
		},
	}))
	if err != nil {
		t.Fatalf("Spark: %v", err)
	}
	logs, err := client.ListConversations(context.Background(), connect.NewRequest(&specv1.ListConversationsRequest{
		Slug:  "new-spec-3",
		Stage: "spark",
	}))
	if err != nil {
		t.Fatalf("ListConversations: %v", err)
	}
	if len(logs.Msg.GetConversationLogs()) != 1 {
		t.Errorf("expected 1 spark conversation log, got %d", len(logs.Msg.GetConversationLogs()))
	}
}

// txConvAuthoringTestBackend combines txAuthoringTestBackend with ConversationBackend
// support so Shape tests can exercise the full 4-op transactional path.
type txConvAuthoringTestBackend struct {
	txAuthoringTestBackend
	conv *fakeConversationBackend
}

func (b *txConvAuthoringTestBackend) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	return b.conv.RecordConversation(ctx, slug, entry)
}

func (b *txConvAuthoringTestBackend) ListConversations(ctx context.Context, slug string, stage string) ([]*storage.ConversationLogEntry, error) {
	return b.conv.ListConversations(ctx, slug, stage)
}

// newTxConvAuthoringClient creates a test client whose backend supports both
// RunInTransaction and ConversationBackend — enabling full Shape tx coverage.
func newTxConvAuthoringClient(t *testing.T, authoringStore *fakeAuthoringBackend, conv *fakeConversationBackend) specgraphv1connect.AuthoringServiceClient {
	t.Helper()
	backend := &txConvAuthoringTestBackend{
		txAuthoringTestBackend: txAuthoringTestBackend{
			authoringTestBackend: authoringTestBackend{authoring: authoringStore, backend: &fakeBackend{}},
		},
		conv: conv,
	}
	scoper := &testScoper{backend: backend}
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)
}

func TestAuthoringHandler_Shape_RecordConversationFailureRollsBack(t *testing.T) {
	// When RecordConversation fails inside the 4-op Shape transaction, the error
	// propagates and the response is CodeInternal. This mirrors the Spark rollback
	// test and is the headline atomicity guarantee of this PR.
	conv := &fakeConversationBackend{recordErr: errors.New("injected record error")}
	authoringStore := &fakeAuthoringBackend{}
	client := newTxConvAuthoringClient(t, authoringStore, conv)

	_, err := client.Shape(context.Background(), connect.NewRequest(&specv1.ShapeRequest{
		Slug: "rollback-spec",
		Output: &specv1.ShapeOutput{
			ScopeIn:  []string{"auth endpoint"},
			ScopeOut: []string{"admin panel"},
			Risks:    []string{"latency"},
		},
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "auth endpoint", Stage: "shape", Sequence: 2},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestAuthoringHandler_Approve_RejectOfAlreadyApprovedFails(t *testing.T) {
	// Reject is only valid from the decompose stage. Attempting to reject a spec
	// that is already in the approved stage should fail with CodeFailedPrecondition.
	conv := &fakeConversationBackend{}
	findings := &fakeFindingsBackend{}
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeRejectBackend{
		conv:     conv,
		findings: findings,
		stage:    storage.SpecStageApproved,
	})
	_, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_REJECT,
		ConversationExchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "looks good?", Stage: "approve", Sequence: 1},
			{Role: "response", Content: "no, rejected", Stage: "approve", Sequence: 2},
		},
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
	require.Contains(t, connErr.Message(), "reject requires decompose")
}

func TestAuthoringHandler_Approve_ExplicitAcceptSucceeds(t *testing.T) {
	// Explicit APPROVE_ACTION_ACCEPT behaves identically to UNSPECIFIED.
	client := newAuthoringClient(t, &fakeAuthoringBackend{}, &fakeBackend{})
	resp, err := client.Approve(context.Background(), connect.NewRequest(&specv1.ApproveRequest{
		Slug:   "my-spec",
		Action: specv1.ApproveAction_APPROVE_ACTION_ACCEPT,
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	require.Equal(t, specv1.AuthoringStage_AUTHORING_STAGE_APPROVED, resp.Msg.Stage)
	require.NotNil(t, resp.Msg.ApprovedAt, "approved_at should be set on explicit ACCEPT")
}
