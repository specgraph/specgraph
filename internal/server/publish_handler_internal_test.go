// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/publish"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- pageMappingToProto ---

func TestPageMappingToProto(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	m := &storage.PageMapping{
		SpecSlug:     "my-spec",
		DocKind:      storage.DocumentKindPRD,
		DecisionSlug: "",
		PageID:       "page-42",
		PageVersion:  3,
		SpecVersion:  2,
		State:        storage.PublishStateSynced,
		ErrorMessage: "",
		LastSync:     now,
	}
	pb := pageMappingToProto(m)
	require.NotNil(t, pb)
	assert.Equal(t, "my-spec", pb.GetSpecSlug())
	assert.Equal(t, specv1.DocumentKind_DOCUMENT_KIND_PRD, pb.GetDocKind())
	assert.Equal(t, "page-42", pb.GetPageId())
	assert.Equal(t, int32(3), pb.GetPageVersion())
	assert.Equal(t, int32(2), pb.GetSpecVersion())
	assert.Equal(t, specv1.PublishState_PUBLISH_STATE_SYNCED, pb.GetState())
	require.NotNil(t, pb.GetLastSync())
	assert.Equal(t, now.Unix(), pb.GetLastSync().AsTime().Unix())
}

func TestPageMappingToProto_ADR(t *testing.T) {
	m := &storage.PageMapping{
		SpecSlug:     "spec-1",
		DocKind:      storage.DocumentKindADR,
		DecisionSlug: "adr-use-grpc",
		PageID:       "page-adr",
		PageVersion:  1,
		State:        storage.PublishStateDraft,
	}
	pb := pageMappingToProto(m)
	assert.Equal(t, specv1.DocumentKind_DOCUMENT_KIND_ADR, pb.GetDocKind())
	assert.Equal(t, "adr-use-grpc", pb.GetDecisionSlug())
	assert.Equal(t, specv1.PublishState_PUBLISH_STATE_DRAFT, pb.GetState())
}

func TestPageMappingToProto_SDD(t *testing.T) {
	m := &storage.PageMapping{
		SpecSlug:    "spec-2",
		DocKind:     storage.DocumentKindSDD,
		PageID:      "page-sdd",
		PageVersion: 2,
		State:       storage.PublishStateError,
	}
	pb := pageMappingToProto(m)
	assert.Equal(t, specv1.DocumentKind_DOCUMENT_KIND_SDD, pb.GetDocKind())
	assert.Equal(t, specv1.PublishState_PUBLISH_STATE_ERROR, pb.GetState())
}

// --- pageMappingsToProto ---

func TestPageMappingsToProto_Empty(t *testing.T) {
	result := pageMappingsToProto(nil)
	assert.Nil(t, result)

	result = pageMappingsToProto([]*storage.PageMapping{})
	assert.Nil(t, result)
}

func TestPageMappingsToProto_Multiple(t *testing.T) {
	mappings := []*storage.PageMapping{
		{SpecSlug: "s1", DocKind: storage.DocumentKindPRD, PageID: "p1", State: storage.PublishStateSynced},
		{SpecSlug: "s1", DocKind: storage.DocumentKindSDD, PageID: "p2", State: storage.PublishStateSynced},
		{SpecSlug: "s1", DocKind: storage.DocumentKindADR, DecisionSlug: "adr-1", PageID: "p3", State: storage.PublishStateSynced},
	}
	result := pageMappingsToProto(mappings)
	require.Len(t, result, 3)
	assert.Equal(t, "p1", result[0].GetPageId())
	assert.Equal(t, "p2", result[1].GetPageId())
	assert.Equal(t, "p3", result[2].GetPageId())
	assert.Equal(t, "adr-1", result[2].GetDecisionSlug())
}

// --- groupMappingsToEntries ---

func TestGroupMappingsToEntries_Empty(t *testing.T) {
	result := groupMappingsToEntries(nil)
	assert.Nil(t, result)

	result = groupMappingsToEntries([]*storage.PageMapping{})
	assert.Nil(t, result)
}

func TestGroupMappingsToEntries_MixedSameSpec(t *testing.T) {
	now := time.Now().UTC()
	mappings := []*storage.PageMapping{
		{SpecSlug: "spec-a", DocKind: storage.DocumentKindPRD, PageID: "prd-1", State: storage.PublishStateSynced, LastSync: now},
		{SpecSlug: "spec-a", DocKind: storage.DocumentKindSDD, PageID: "sdd-1", State: storage.PublishStateSynced, LastSync: now.Add(-time.Hour)},
		{SpecSlug: "spec-a", DocKind: storage.DocumentKindADR, DecisionSlug: "adr-1", PageID: "adr-1", State: storage.PublishStateSynced, LastSync: now.Add(-2 * time.Hour)},
		{SpecSlug: "spec-a", DocKind: storage.DocumentKindADR, DecisionSlug: "adr-2", PageID: "adr-2", State: storage.PublishStateSynced, LastSync: now.Add(-3 * time.Hour)},
	}
	entries := groupMappingsToEntries(mappings)
	require.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, "spec-a", e.GetSpecSlug())
	require.NotNil(t, e.GetPrd())
	assert.Equal(t, "prd-1", e.GetPrd().GetPageId())
	require.NotNil(t, e.GetSdd())
	assert.Equal(t, "sdd-1", e.GetSdd().GetPageId())
	require.Len(t, e.GetAdrs(), 2)
	// LastSync should be the most recent (now)
	require.NotNil(t, e.GetLastSync())
	assert.Equal(t, now.Unix(), e.GetLastSync().AsTime().Unix())
}

func TestGroupMappingsToEntries_DifferentSpecs(t *testing.T) {
	now := time.Now().UTC()
	mappings := []*storage.PageMapping{
		{SpecSlug: "spec-alpha", DocKind: storage.DocumentKindPRD, PageID: "prd-a", State: storage.PublishStateSynced, LastSync: now},
		{SpecSlug: "spec-beta", DocKind: storage.DocumentKindPRD, PageID: "prd-b", State: storage.PublishStateSynced, LastSync: now},
	}
	entries := groupMappingsToEntries(mappings)
	require.Len(t, entries, 2)

	slugs := map[string]bool{}
	for _, e := range entries {
		slugs[e.GetSpecSlug()] = true
	}
	assert.True(t, slugs["spec-alpha"], "missing spec-alpha entry")
	assert.True(t, slugs["spec-beta"], "missing spec-beta entry")
}

func TestGroupMappingsToEntries_NoLastSync(t *testing.T) {
	// Mapping with zero LastSync should not set entry.LastSync.
	mappings := []*storage.PageMapping{
		{SpecSlug: "spec-a", DocKind: storage.DocumentKindPRD, PageID: "prd-1", State: storage.PublishStateSynced},
	}
	entries := groupMappingsToEntries(mappings)
	require.Len(t, entries, 1)
	assert.Nil(t, entries[0].GetLastSync(), "LastSync should be nil for zero time")
}

// --- feedbackToProto ---

func TestFeedbackToProto(t *testing.T) {
	now := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	f := &publish.Feedback{
		ExternalID: "comment-xyz",
		Author:     "alice",
		Body:       "Great design!",
		Timestamp:  now,
		Kind:       publish.FeedbackInline,
		Stage:      "shape",
		IsQuestion: false,
		ParentID:   "parent-1",
		SpecSlug:   "my-spec",
	}
	pb := feedbackToProto(f)
	require.NotNil(t, pb)
	assert.Equal(t, "comment-xyz", pb.GetExternalId())
	assert.Equal(t, "alice", pb.GetAuthor())
	assert.Equal(t, "Great design!", pb.GetBody())
	assert.Equal(t, specv1.FeedbackKind_FEEDBACK_KIND_INLINE, pb.GetKind())
	assert.Equal(t, "shape", pb.GetStage())
	assert.False(t, pb.GetIsQuestion())
	assert.Equal(t, "parent-1", pb.GetParentId())
	assert.Equal(t, "my-spec", pb.GetSpecSlug())
	require.NotNil(t, pb.GetTimestamp())
	assert.Equal(t, now.Unix(), pb.GetTimestamp().AsTime().Unix())
}

func TestFeedbackToProto_Footer(t *testing.T) {
	f := &publish.Feedback{
		ExternalID: "footer-1",
		Kind:       publish.FeedbackFooter,
		IsQuestion: true,
		Body:       "Is this in scope?",
	}
	pb := feedbackToProto(f)
	assert.Equal(t, specv1.FeedbackKind_FEEDBACK_KIND_FOOTER, pb.GetKind())
	assert.True(t, pb.GetIsQuestion())
}

// --- getLinkedDecisions ---

func TestGetLinkedDecisions_NoEdges(t *testing.T) {
	store := &noEdgeBackend{}
	decisions, err := getLinkedDecisions(context.Background(), store, "my-spec")
	require.NoError(t, err)
	assert.Empty(t, decisions)
}

func TestGetLinkedDecisions_WithDecidedInEdge(t *testing.T) {
	store := &decidedInBackend{
		specSlug: "my-spec",
		edges: []*storage.Edge{
			{FromID: "my-spec", ToID: "adr-use-grpc", EdgeType: storage.EdgeTypeDecidedIn},
		},
		decision: &storage.Decision{
			ID:     "d-1",
			Slug:   "adr-use-grpc",
			Title:  "Use ConnectRPC",
			Status: storage.DecisionStatusAccepted,
		},
	}
	decisions, err := getLinkedDecisions(context.Background(), store, "my-spec")
	require.NoError(t, err)
	require.Len(t, decisions, 1)
	assert.Equal(t, "adr-use-grpc", decisions[0].GetSlug())
	assert.Equal(t, "Use ConnectRPC", decisions[0].GetTitle())
}

func TestGetLinkedDecisions_DecisionNotFound(t *testing.T) {
	// Decision linked but missing from storage — should be skipped (logged as warn).
	store := &decidedInBackend{
		specSlug: "my-spec",
		edges: []*storage.Edge{
			{FromID: "my-spec", ToID: "missing-decision", EdgeType: storage.EdgeTypeDecidedIn},
		},
		notFound: true,
	}
	decisions, err := getLinkedDecisions(context.Background(), store, "my-spec")
	require.NoError(t, err)
	assert.Empty(t, decisions)
}

func TestGetLinkedDecisions_EdgeFromOtherSpec(t *testing.T) {
	// Edge.FromID doesn't match the slug — should be filtered out.
	store := &decidedInBackend{
		specSlug: "my-spec",
		edges: []*storage.Edge{
			{FromID: "other-spec", ToID: "adr-1", EdgeType: storage.EdgeTypeDecidedIn},
		},
	}
	decisions, err := getLinkedDecisions(context.Background(), store, "my-spec")
	require.NoError(t, err)
	assert.Empty(t, decisions)
}

// --- publishError ---

func TestPublishError_SpecNotFound(t *testing.T) {
	err := publishError(storage.ErrSpecNotFound)
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestPublishError_DecisionNotFound(t *testing.T) {
	err := publishError(storage.ErrDecisionNotFound)
	assertConnectCode(t, err, connect.CodeNotFound)
}

func TestPublishError_OtherError(t *testing.T) {
	err := publishError(errors.New("something else"))
	assertConnectCode(t, err, connect.CodeInternal)
}

// --- helper backends for getLinkedDecisions tests ---

type noEdgeBackend struct{ internalStubBackend }

func (b *noEdgeBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	return nil, nil
}
func (b *noEdgeBackend) GetDecision(_ context.Context, _ string) (*storage.Decision, error) {
	return nil, storage.ErrDecisionNotFound
}

type decidedInBackend struct {
	internalStubBackend
	specSlug string
	edges    []*storage.Edge
	decision *storage.Decision
	notFound bool
}

func (b *decidedInBackend) ListEdges(_ context.Context, _ string, _ storage.EdgeType) ([]*storage.Edge, error) {
	return b.edges, nil
}

func (b *decidedInBackend) GetDecision(_ context.Context, slug string) (*storage.Decision, error) {
	if b.notFound {
		return nil, storage.ErrDecisionNotFound
	}
	if b.decision != nil && b.decision.Slug == slug {
		return b.decision, nil
	}
	return nil, storage.ErrDecisionNotFound
}

// internalStubBackend provides no-op implementations for the ScopedBackend interface
// so per-test backends only need to override the methods they use.
type internalStubBackend struct{}

func (internalStubBackend) CreateSpec(context.Context, string, string, string, string) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetSpec(context.Context, string) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListSpecs(context.Context, string, string, int) ([]*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpdateSpec(context.Context, string, *string, *string, *string, *string, *string) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) Close(context.Context) error { return nil }
func (internalStubBackend) AddEdge(context.Context, string, string, storage.EdgeType) (*storage.Edge, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) RemoveEdge(context.Context, string, string, storage.EdgeType) error {
	return errors.New("not implemented")
}
func (internalStubBackend) ListEdges(context.Context, string, storage.EdgeType) ([]*storage.Edge, error) {
	return nil, nil
}
func (internalStubBackend) GetDependencies(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetTransitiveDeps(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetImpact(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetReady(context.Context) ([]storage.NodeRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetCriticalPath(context.Context, string) ([]storage.NodeRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetFullGraph(context.Context) (*storage.FullGraph, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) CreateDecision(context.Context, string, string, string, string, string,
	[]storage.RejectedAlternative, storage.DecisionConfidence,
	[]string, storage.DecisionScope, string, string,
) (*storage.Decision, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetDecision(context.Context, string) (*storage.Decision, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListDecisions(context.Context, storage.DecisionStatus, int) ([]*storage.Decision, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpdateDecision(context.Context, string, int32, *string, *storage.DecisionStatus,
	*string, *string, *string, *string,
	*[]storage.RejectedAlternative, *storage.DecisionConfidence,
	*[]string, *storage.DecisionScope, *string, *string,
) (*storage.Decision, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ClaimSpec(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UnclaimSpec(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) Heartbeat(context.Context, string, string, time.Duration) (*storage.Claim, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetConstitution(context.Context) (*storage.Constitution, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetConstitutionLayer(_ context.Context, _ storage.ConstitutionLayer) (*storage.Constitution, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetAllLayers(_ context.Context) ([]*storage.Constitution, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpdateConstitution(context.Context, *storage.Constitution) (*storage.Constitution, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) TransitionStage(context.Context, string, storage.SpecStage, storage.SpecStage) error {
	return errors.New("not implemented")
}
func (internalStubBackend) StoreSparkOutput(context.Context, string, *storage.SparkOutput) error {
	return errors.New("not implemented")
}
func (internalStubBackend) StoreShapeOutput(context.Context, string, *storage.ShapeOutput) error {
	return errors.New("not implemented")
}
func (internalStubBackend) StoreSpecifyOutput(context.Context, string, *storage.SpecifyOutput) error {
	return errors.New("not implemented")
}
func (internalStubBackend) StoreDecomposeOutput(context.Context, string, *storage.DecomposeOutput) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) StoreSafetyFlags(context.Context, string, []storage.SafetyFlag) error {
	return errors.New("not implemented")
}
func (internalStubBackend) SupersedeSpec(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) AmendSpec(context.Context, string, string, storage.SpecStage) (*storage.AmendResult, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) StoreFindings(context.Context, string, storage.PassType, []storage.AnalyticalFindingInput) ([]string, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListFindings(context.Context, string, storage.PassType) ([]storage.AnalyticalFinding, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GenerateBundle(context.Context, string) (*storage.Bundle, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) RecordProgress(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) RecordBlocker(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) RecordCompletion(context.Context, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) GetExecutionEvents(context.Context, string, int) ([]*storage.ExecutionEvent, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetPrimeData(context.Context, string) (*storage.PrimeData, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ReleaseExpiredClaims(context.Context) (int, error) {
	return 0, errors.New("not implemented")
}
func (internalStubBackend) LifecycleAmendSpec(context.Context, string, string, string) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) LifecycleSupersedeSpec(context.Context, string, string) (*storage.Spec, *storage.Spec, error) {
	return nil, nil, errors.New("not implemented")
}
func (internalStubBackend) LifecycleAbandonSpec(context.Context, string, string) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) LifecycleAcknowledgeDrift(context.Context, string, string, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) GetDependenciesWithEdgeData(context.Context, string) ([]storage.DependencyRef, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) RefreshDependencyHashes(context.Context, string) error {
	return errors.New("not implemented")
}
func (internalStubBackend) CreateSyncMapping(context.Context, string, storage.SyncAdapterType, string) (*storage.SyncMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpdateSyncState(context.Context, string, storage.SyncAdapterType, storage.SyncStateType, string) (*storage.SyncMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetSyncMapping(context.Context, string, storage.SyncAdapterType) (*storage.SyncMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListSyncMappings(context.Context, storage.SyncAdapterType, string) ([]*storage.SyncMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) DeleteSyncMapping(context.Context, string, storage.SyncAdapterType) error {
	return errors.New("not implemented")
}
func (internalStubBackend) GetProject(context.Context, string) (*storage.Project, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) EnsureProject(context.Context, string) (*storage.Project, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpdateProject(context.Context, string, []string, string) (*storage.Project, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListProjects(context.Context) ([]*storage.Project, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) CreateSlice(context.Context, *storage.Slice) error {
	return errors.New("not implemented")
}
func (internalStubBackend) ListSlices(context.Context, string) ([]*storage.Slice, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) GetSlice(context.Context, string) (*storage.Slice, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ClaimSlice(context.Context, string, string) (*storage.Slice, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) CompleteSlice(context.Context, string) (*storage.Slice, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListChanges(context.Context, string, storage.ChangeLogFilter) ([]*storage.ChangeLogEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) RecordConversation(context.Context, string, storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListConversations(context.Context, string, string) ([]*storage.ConversationLogEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListAllFindings(context.Context) ([]*storage.AnalyticalFinding, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListAllChanges(context.Context) ([]*storage.ChangeLogEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListAllConversations(context.Context) ([]*storage.ConversationLogEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) WipeProjectData(context.Context) error {
	return errors.New("not implemented")
}
func (internalStubBackend) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (internalStubBackend) GetSpecAtVersion(_ context.Context, _ string, _ int32) (*storage.Spec, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) UpsertPageMapping(_ context.Context, m *storage.PageMapping) (*storage.PageMapping, error) {
	return m, errors.New("not implemented")
}
func (internalStubBackend) GetPageMapping(_ context.Context, _ string, _ storage.DocumentKind, _ string) (*storage.PageMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) ListPageMappings(_ context.Context, _ string) ([]*storage.PageMapping, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) DeletePageMappings(_ context.Context, _ string) (int, error) {
	return 0, errors.New("not implemented")
}
func (internalStubBackend) StoreFeedback(_ context.Context, entry *storage.FeedbackEntry) (*storage.FeedbackEntry, error) {
	return entry, errors.New("not implemented")
}
func (internalStubBackend) ListFeedback(_ context.Context, _ string, _ string) ([]*storage.FeedbackEntry, error) {
	return nil, errors.New("not implemented")
}
func (internalStubBackend) CountNewFeedback(_ context.Context, _ string) (int, error) {
	return 0, errors.New("not implemented")
}

// --- feedbackToStorage ---

func TestFeedbackToStorage(t *testing.T) {
	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)

	t.Run("inline feedback", func(t *testing.T) {
		f := &publish.Feedback{
			ExternalID: "comment-inline-1",
			Author:     "alice",
			Body:       "This needs revision",
			Timestamp:  ts,
			Kind:       publish.FeedbackInline,
			Stage:      "shape",
			IsQuestion: false,
			ParentID:   "parent-abc",
			SpecSlug:   "my-spec",
		}
		entry := feedbackToStorage(f, "my-spec")
		require.NotNil(t, entry)
		assert.Equal(t, "comment-inline-1", entry.ExternalID)
		assert.Equal(t, "my-spec", entry.SpecSlug)
		assert.Equal(t, "alice", entry.Author)
		assert.Equal(t, "This needs revision", entry.Body)
		assert.Equal(t, ts, entry.Timestamp)
		assert.Equal(t, storage.FeedbackKindInline, entry.Kind)
		assert.Equal(t, "shape", entry.Stage)
		assert.False(t, entry.IsQuestion)
		assert.Equal(t, "parent-abc", entry.ParentID)
	})

	t.Run("footer feedback", func(t *testing.T) {
		f := &publish.Feedback{
			ExternalID: "comment-footer-2",
			Author:     "bob",
			Body:       "Looks good to me!",
			Timestamp:  ts,
			Kind:       publish.FeedbackFooter,
			IsQuestion: true,
			ParentID:   "",
			SpecSlug:   "other-spec",
		}
		entry := feedbackToStorage(f, "other-spec")
		require.NotNil(t, entry)
		assert.Equal(t, storage.FeedbackKindFooter, entry.Kind)
		assert.True(t, entry.IsQuestion)
		assert.Equal(t, "other-spec", entry.SpecSlug)
		assert.Empty(t, entry.Stage)
		assert.Empty(t, entry.ParentID)
	})
}
