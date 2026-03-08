// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// fakeLifecycleBackend is a minimal fake implementation of storage.LifecycleBackend for testing.
type fakeLifecycleBackend struct {
	amendSpec        func(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error)
	supersedeSpec    func(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error)
	abandonSpec      func(ctx context.Context, slug, reason string) (*storage.Spec, error)
	checkDrift       func(ctx context.Context, slug, scope string) ([]storage.DriftReport, error)
	acknowledgeDrift func(ctx context.Context, slug, note string) (*storage.DriftReport, error)
}

func (f *fakeLifecycleBackend) AmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	return f.amendSpec(ctx, slug, reason, reEntryStage)
}

func (f *fakeLifecycleBackend) SupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
	return f.supersedeSpec(ctx, oldSlug, newSlug)
}

func (f *fakeLifecycleBackend) AbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	return f.abandonSpec(ctx, slug, reason)
}

func (f *fakeLifecycleBackend) CheckDrift(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	return f.checkDrift(ctx, slug, scope)
}

func (f *fakeLifecycleBackend) AcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	return f.acknowledgeDrift(ctx, slug, note)
}

func newLifecycleClient(t *testing.T, store storage.LifecycleBackend) specgraphv1connect.LifecycleServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, store)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)
}

func TestLifecycleHandler_Amend(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		amendSpec: func(_ context.Context, slug, _, _ string) (*storage.Spec, error) {
			return &storage.Spec{
				Slug:    slug,
				Stage:   storage.SpecStageAmended,
				Version: 2,
				History: []storage.HistoryEntry{
					{Version: 2, Stage: "amended", Summary: "amended", Reason: "needs rework", Date: now},
				},
			}, nil
		},
	})

	resp, err := client.Amend(context.Background(), connect.NewRequest(&specv1.LifecycleAmendRequest{
		Slug:         "my-spec",
		Reason:       "needs rework",
		ReEntryStage: "shape",
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Slug)
	require.Equal(t, "amended", resp.Msg.Stage)
	require.Equal(t, int32(2), resp.Msg.Version)
	require.Len(t, resp.Msg.History, 1)
	require.Equal(t, "needs rework", resp.Msg.History[0].Reason)
}

func TestLifecycleHandler_Amend_NotFound(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		amendSpec: func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
			return nil, storage.ErrSpecNotFound
		},
	})

	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.LifecycleAmendRequest{
		Slug:         "no-such-spec",
		Reason:       "rework",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Amend_NotDone(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		amendSpec: func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
			return nil, storage.ErrSpecNotDone
		},
	})

	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.LifecycleAmendRequest{
		Slug:         "in-progress-spec",
		Reason:       "rework",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Supersede(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		supersedeSpec: func(_ context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
			return &storage.Spec{
					Slug:         oldSlug,
					Stage:        storage.SpecStageSuperseded,
					SupersededBy: newSlug,
					Version:      3,
				}, &storage.Spec{
					Slug:       newSlug,
					Stage:      storage.SpecStageSpark,
					Supersedes: oldSlug,
					Version:    1,
				}, nil
		},
	})

	resp, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.LifecycleSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "new-spec",
	}))
	require.NoError(t, err)
	require.Equal(t, "old-spec", resp.Msg.OldSpec.Slug)
	require.Equal(t, "superseded", resp.Msg.OldSpec.Stage)
	require.Equal(t, "new-spec", resp.Msg.OldSpec.SupersededBy)
	require.Equal(t, "new-spec", resp.Msg.NewSpec.Slug)
	require.Equal(t, "old-spec", resp.Msg.NewSpec.Supersedes)
}

func TestLifecycleHandler_Abandon(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		abandonSpec: func(_ context.Context, slug, _ string) (*storage.Spec, error) {
			return &storage.Spec{
				Slug:    slug,
				Stage:   storage.SpecStageAbandoned,
				Version: 2,
			}, nil
		},
	})

	resp, err := client.Abandon(context.Background(), connect.NewRequest(&specv1.LifecycleAbandonRequest{
		Slug:   "dead-spec",
		Reason: "no longer needed",
	}))
	require.NoError(t, err)
	require.Equal(t, "dead-spec", resp.Msg.Slug)
	require.Equal(t, "abandoned", resp.Msg.Stage)
}

func TestLifecycleHandler_CheckDrift(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		checkDrift: func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
			return []storage.DriftReport{
				{
					SpecSlug: "my-spec",
					Items: []storage.DriftItem{
						{
							Type:            storage.DriftTypeDependency,
							Severity:        storage.DriftSeverityHigh,
							Description:     "upstream changed",
							SpecSlug:        "my-spec",
							UpstreamSlug:    "upstream-spec",
							ExpectedVersion: 2,
							ActualVersion:   3,
						},
					},
				},
			}, nil
		},
	})

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 1)
	require.Equal(t, "my-spec", resp.Msg.Reports[0].SpecSlug)
	require.Len(t, resp.Msg.Reports[0].Items, 1)
	require.Equal(t, specv1.DriftType_DRIFT_TYPE_DEPENDENCY, resp.Msg.Reports[0].Items[0].Type)
	require.Equal(t, specv1.DriftSeverity_DRIFT_SEVERITY_HIGH, resp.Msg.Reports[0].Items[0].Severity)
}

func TestLifecycleHandler_AcknowledgeDrift(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		acknowledgeDrift: func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
			return &storage.DriftReport{
				SpecSlug:        slug,
				Acknowledged:    true,
				AcknowledgeNote: note,
			}, nil
		},
	})

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.SpecSlug)
	require.True(t, resp.Msg.Acknowledged)
	require.Equal(t, "intentional divergence", resp.Msg.AcknowledgeNote)
}

func TestLifecycleHandler_Amend_EmptySlug(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{})
	_, err := client.Amend(context.Background(), connect.NewRequest(&specv1.LifecycleAmendRequest{
		Slug:   "",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_SameSlug(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.LifecycleSupersedeRequest{
		Slug:    "same-spec",
		NewSlug: "same-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_NewSpecNotFound(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{
		supersedeSpec: func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
			return nil, nil, storage.ErrNewSpecNotFound
		},
	})
	_, err := client.Supersede(context.Background(), connect.NewRequest(&specv1.LifecycleSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "missing-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Lint_Unimplemented(t *testing.T) {
	client := newLifecycleClient(t, &fakeLifecycleBackend{})
	_, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnimplemented, connErr.Code())
}
