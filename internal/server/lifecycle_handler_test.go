// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	getSpec          func(ctx context.Context, slug string) (*storage.Spec, error)
	batchGetSpecs    func(ctx context.Context, slugs []string) (map[string]*storage.Spec, error)
	amendSpec        func(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error)
	supersedeSpec    func(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error)
	abandonSpec      func(ctx context.Context, slug, reason string) (*storage.Spec, error)
	acknowledgeDrift func(ctx context.Context, slug, note string) (*storage.DriftReport, error)
}

func (f *fakeLifecycleBackend) GetSpec(ctx context.Context, slug string) (*storage.Spec, error) {
	if f.getSpec == nil {
		return nil, storage.ErrSpecNotFound
	}
	return f.getSpec(ctx, slug)
}

func (f *fakeLifecycleBackend) BatchGetSpecs(ctx context.Context, slugs []string) (map[string]*storage.Spec, error) {
	if f.batchGetSpecs != nil {
		return f.batchGetSpecs(ctx, slugs)
	}
	// Default: delegate to individual GetSpec calls.
	result := make(map[string]*storage.Spec, len(slugs))
	for _, slug := range slugs {
		spec, err := f.GetSpec(ctx, slug)
		if err != nil {
			if errors.Is(err, storage.ErrSpecNotFound) {
				continue
			}
			return nil, err
		}
		result[spec.Slug] = spec
	}
	return result, nil
}

func (f *fakeLifecycleBackend) LifecycleAmendSpec(ctx context.Context, slug, reason, reEntryStage string) (*storage.Spec, error) {
	if f.amendSpec == nil {
		return nil, errors.New("fakeLifecycleBackend.amendSpec not configured")
	}
	return f.amendSpec(ctx, slug, reason, reEntryStage)
}

func (f *fakeLifecycleBackend) LifecycleSupersedeSpec(ctx context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
	if f.supersedeSpec == nil {
		return nil, nil, errors.New("fakeLifecycleBackend.supersedeSpec not configured")
	}
	return f.supersedeSpec(ctx, oldSlug, newSlug)
}

func (f *fakeLifecycleBackend) LifecycleAbandonSpec(ctx context.Context, slug, reason string) (*storage.Spec, error) {
	if f.abandonSpec == nil {
		return nil, errors.New("fakeLifecycleBackend.abandonSpec not configured")
	}
	return f.abandonSpec(ctx, slug, reason)
}

func (f *fakeLifecycleBackend) LifecycleAcknowledgeDrift(ctx context.Context, slug, note string) (*storage.DriftReport, error) {
	if f.acknowledgeDrift == nil {
		return nil, errors.New("fakeLifecycleBackend.acknowledgeDrift not configured")
	}
	return f.acknowledgeDrift(ctx, slug, note)
}

// fakeDriftChecker implements server.DriftChecker for testing.
type fakeDriftChecker struct {
	check func(ctx context.Context, slug, scope string) ([]storage.DriftReport, error)
}

func (f *fakeDriftChecker) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	return f.check(ctx, slug, scope)
}

// fakeLinter implements server.SpecLinter for testing.
type fakeLinter struct {
	lint func(ctx context.Context, slug string) ([]storage.LintResult, error)
}

func (f *fakeLinter) Lint(ctx context.Context, slug string) ([]storage.LintResult, error) {
	return f.lint(ctx, slug)
}

type lifecycleTestDeps struct {
	store  *fakeLifecycleBackend
	drift  *fakeDriftChecker
	linter *fakeLinter
}

func defaultTestDeps() *lifecycleTestDeps {
	return &lifecycleTestDeps{
		store:  &fakeLifecycleBackend{},
		drift:  &fakeDriftChecker{},
		linter: &fakeLinter{},
	}
}

func newLifecycleClient(t *testing.T, deps *lifecycleTestDeps) specgraphv1connect.LifecycleServiceClient {
	t.Helper()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.store, deps.drift, deps.linter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)
}

func TestLifecycleHandler_Amend(t *testing.T) {
	now := time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC)
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, slug, _, _ string) (*storage.Spec, error) {
		return &storage.Spec{
			Slug:    slug,
			Stage:   storage.SpecStageAmended,
			Version: 2,
			History: []storage.HistoryEntry{
				{Version: 2, Stage: storage.SpecStageAmended, Summary: "amended", Reason: "needs rework", Date: now},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "needs rework",
		ReEntryStage: "shape",
	}))
	require.NoError(t, err)
	s := resp.Msg.GetSpec()
	require.Equal(t, "my-spec", s.GetSlug())
	require.Equal(t, "amended", s.GetStage())
	require.Equal(t, int32(2), s.GetVersion())
	require.Len(t, s.GetHistory(), 1)
	require.Equal(t, "needs rework", s.GetHistory()[0].GetReason())
}

func TestLifecycleHandler_Amend_NotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
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
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotDone
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "in-progress-spec",
		Reason:       "rework",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Amend_InvalidReEntryStage(t *testing.T) {
	deps := defaultTestDeps()
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "rework",
		ReEntryStage: "invalid-stage",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, oldSlug, newSlug string) (*storage.Spec, *storage.Spec, error) {
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
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
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
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, slug, _ string) (*storage.Spec, error) {
		return &storage.Spec{
			Slug:    slug,
			Stage:   storage.SpecStageAbandoned,
			Version: 2,
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "dead-spec",
		Reason: "no longer needed",
	}))
	require.NoError(t, err)
	s := resp.Msg.GetSpec()
	require.Equal(t, "dead-spec", s.GetSlug())
	require.Equal(t, "abandoned", s.GetStage())
}

func TestLifecycleHandler_CheckDrift(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
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
	}
	client := newLifecycleClient(t, deps)

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

func TestLifecycleHandler_CheckDrift_AllSpecs(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, slug, _ string) ([]storage.DriftReport, error) {
		require.Empty(t, slug, "empty slug means check all specs")
		return []storage.DriftReport{
			{SpecSlug: "spec-a"},
			{SpecSlug: "spec-b"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 2)
}

func TestLifecycleHandler_CheckDrift_MergesAcknowledgmentState(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.getSpec = func(_ context.Context, slug string) (*storage.Spec, error) {
		return &storage.Spec{
			Slug:                 slug,
			Stage:                storage.SpecStageDone,
			DriftAcknowledged:    true,
			DriftAcknowledgeNote: "some note",
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{SpecSlug: "my-spec"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 1)
	require.True(t, resp.Msg.Reports[0].Acknowledged)
	require.Equal(t, "some note", resp.Msg.Reports[0].AcknowledgeNote)
}

func TestLifecycleHandler_CheckDrift_GetSpecErrorReturnsUnavailable(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.getSpec = func(_ context.Context, _ string) (*storage.Spec, error) {
		return nil, errors.New("database timeout")
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{SpecSlug: "my-spec"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnavailable, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_SpecDeletedBetweenDriftAndAckMerge(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.getSpec = func(_ context.Context, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotFound
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{SpecSlug: "deleted-spec"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "deleted-spec",
	}))
	require.NoError(t, err, "spec deletion during ack merge should not cause an error")
	require.Len(t, resp.Msg.Reports, 1)
	require.False(t, resp.Msg.Reports[0].Acknowledged, "ack state must not leak from a deleted spec")
	require.True(t, resp.Msg.Reports[0].AckStateUnavailable, "must signal that ack state is unavailable")
}

func TestLifecycleHandler_CheckDrift_BatchGetSpecsErrorReturnsUnavailable(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.batchGetSpecs = func(_ context.Context, _ []string) (map[string]*storage.Spec, error) {
		return nil, errors.New("database timeout")
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{SpecSlug: "spec-a"},
			{SpecSlug: "spec-b"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnavailable, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_AllSpecs_MissingSpecInBatch(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.batchGetSpecs = func(_ context.Context, _ []string) (map[string]*storage.Spec, error) {
		// Return only spec-a, omitting spec-b to trigger AckStateUnavailable.
		return map[string]*storage.Spec{
			"spec-a": {Slug: "spec-a", DriftAcknowledged: true, DriftAcknowledgeNote: "known"},
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{SpecSlug: "spec-a"},
			{SpecSlug: "spec-b"},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 2)

	var specA, specB *specv1.DriftReport
	for _, r := range resp.Msg.Reports {
		switch r.SpecSlug {
		case "spec-a":
			specA = r
		case "spec-b":
			specB = r
		}
	}
	require.NotNil(t, specA)
	require.True(t, specA.Acknowledged)
	require.False(t, specA.AckStateUnavailable)

	require.NotNil(t, specB)
	require.True(t, specB.AckStateUnavailable, "missing spec in batch must have AckStateUnavailable=true")
	require.False(t, specB.Acknowledged)
}

func TestLifecycleHandler_AcknowledgeDrift(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.NoError(t, err)
	require.Equal(t, "my-spec", resp.Msg.Report.SpecSlug)
	require.True(t, resp.Msg.Report.Acknowledged)
	require.Equal(t, "intentional divergence", resp.Msg.Report.AcknowledgeNote)
}

func TestLifecycleHandler_AcknowledgeDrift_RecheckMergesItems(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	deps.drift.check = func(_ context.Context, slug, _ string) ([]storage.DriftReport, error) {
		return []storage.DriftReport{
			{
				SpecSlug: slug,
				Items: []storage.DriftItem{
					{
						Type:        storage.DriftTypeDependency,
						Severity:    storage.DriftSeverityHigh,
						Description: "upstream changed",
						SpecSlug:    slug,
					},
				},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional",
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Report.Acknowledged)
	require.Len(t, resp.Msg.Report.Items, 1)
	require.Equal(t, specv1.DriftType_DRIFT_TYPE_DEPENDENCY, resp.Msg.Report.Items[0].Type)
	require.Equal(t, "upstream changed", resp.Msg.Report.Items[0].Description)
}

func TestLifecycleHandler_Amend_EmptySlug(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_SameSlug(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "same-spec",
		NewSlug: "same-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Supersede_EmptyNewSlug(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "my-spec",
		NewSlug: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestLifecycleHandler_Supersede_NewSpecNotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrNewSpecNotFound
	}
	client := newLifecycleClient(t, deps)
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "missing-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Supersede_NewSpecTerminal(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrNewSpecTerminal
	}
	client := newLifecycleClient(t, deps)
	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "abandoned-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Lint(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, slug string) ([]storage.LintResult, error) {
		return []storage.LintResult{
			{
				SpecSlug: slug,
				Passed:   true,
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, "my-spec", resp.Msg.Results[0].SpecSlug)
	require.True(t, resp.Msg.Results[0].Passed)
}

func TestLifecycleHandler_Lint_WithViolations(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, _ string) ([]storage.LintResult, error) {
		return []storage.LintResult{
			{
				SpecSlug: "bad-spec",
				Passed:   false,
				Violations: []storage.LintViolation{
					{
						Rule:     "schema.enum",
						Severity: storage.LintSeverityError,
						Message:  "invalid stage",
						Location: "stage",
					},
				},
			},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "bad-spec",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Results, 1)
	require.False(t, resp.Msg.Results[0].Passed)
	require.Len(t, resp.Msg.Results[0].Violations, 1)
	require.Equal(t, "schema.enum", resp.Msg.Results[0].Violations[0].Rule)
}

func TestLifecycleHandler_Abandon_Terminal(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "already-abandoned",
		Reason: "try again",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Supersede_Terminal(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "terminal-spec",
		NewSlug: "new-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_Amend_ConcurrentModification(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrConcurrentModification
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "busy-spec",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeAborted, connErr.Code())
}

func TestLifecycleHandler_Amend_TerminalReEntryStage(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	for _, stage := range []string{"amended", "superseded", "abandoned"} {
		_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
			Slug:         "my-spec",
			Reason:       "rework",
			ReEntryStage: stage,
		}))
		require.Error(t, err, "stage %q should be rejected", stage)
		var connErr *connect.Error
		require.ErrorAs(t, err, &connErr)
		require.Equal(t, connect.CodeInvalidArgument, connErr.Code(), "stage %q should return InvalidArgument", stage)
	}
}

func TestLifecycleHandler_Amend_ReEntryDone(t *testing.T) {
	deps := defaultTestDeps()
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "re-enter at done",
		ReEntryStage: "done",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestLifecycleHandler_CheckDrift_Error(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, errors.New("backend unavailable")
	}
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_NilChecker(t *testing.T) {
	deps := defaultTestDeps()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.store, nil, deps.linter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnimplemented, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_InvalidScope(t *testing.T) {
	deps := defaultTestDeps()
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug:  "my-spec",
		Scope: specv1.DriftScope(99),
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_NilChecker(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.store, nil, deps.linter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Report.Items)
}

func TestLifecycleHandler_AcknowledgeDrift_RecheckError(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, errors.New("drift engine down")
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Report.Acknowledged)
	require.True(t, resp.Msg.Report.ItemsStale)
	require.Empty(t, resp.Msg.Report.Items)
}

func TestLifecycleHandler_Lint_NilLinter(t *testing.T) {
	deps := defaultTestDeps()
	mux := http.NewServeMux()
	server.RegisterLifecycleService(mux, deps.store, deps.store, deps.drift, nil)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewLifecycleServiceClient(http.DefaultClient, srv.URL)

	_, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeUnimplemented, connErr.Code())
}

func TestLifecycleHandler_Lint_Error(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, _ string) ([]storage.LintResult, error) {
		return nil, errors.New("db unavailable")
	}
	client := newLifecycleClient(t, deps)

	_, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestLifecycleHandler_Amend_TerminalSpec(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.amendSpec = func(_ context.Context, _, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecTerminal
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "superseded-spec",
		Reason: "rework",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_NotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, _, _ string) (*storage.DriftReport, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "no-such-spec",
		Note: "intentional divergence",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_EmptyNote(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_IneligibleStage(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, _, _ string) (*storage.DriftReport, error) {
		return nil, storage.ErrSpecIneligibleStage
	}
	client := newLifecycleClient(t, deps)

	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "should fail",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_StorageError(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, _, _ string) (*storage.DriftReport, error) {
		return nil, errors.New("db write failed")
	}
	client := newLifecycleClient(t, deps)

	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestLifecycleHandler_AcknowledgeDrift_ConcurrentModification(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, _, _ string) (*storage.DriftReport, error) {
		return nil, storage.ErrConcurrentModification
	}
	client := newLifecycleClient(t, deps)

	_, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "my-spec",
		Note: "intentional divergence",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeAborted, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_IneligibleForDrift(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, fmt.Errorf("drift: spec ineligible: %w", storage.ErrSpecIneligibleForDrift)
	}
	client := newLifecycleClient(t, deps)

	_, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{
		Slug: "my-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeFailedPrecondition, connErr.Code())
}

func TestLifecycleHandler_CheckDrift_AllSpecs_AckMerge(t *testing.T) {
	deps := defaultTestDeps()
	deps.drift.check = func(_ context.Context, slug, _ string) ([]storage.DriftReport, error) {
		require.Empty(t, slug)
		return []storage.DriftReport{
			{SpecSlug: "spec-a"},
			{SpecSlug: "spec-b"},
		}, nil
	}
	deps.store.batchGetSpecs = func(_ context.Context, _ []string) (map[string]*storage.Spec, error) {
		return map[string]*storage.Spec{
			"spec-a": {Slug: "spec-a", Stage: storage.SpecStageDone, DriftAcknowledged: true, DriftAcknowledgeNote: "acked"},
			"spec-b": {Slug: "spec-b", Stage: storage.SpecStageDone},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.CheckDrift(context.Background(), connect.NewRequest(&specv1.DriftCheckRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Reports, 2)
	// Find spec-a and spec-b reports (order not guaranteed).
	var specA, specB *specv1.DriftReport
	for _, r := range resp.Msg.Reports {
		switch r.SpecSlug {
		case "spec-a":
			specA = r
		case "spec-b":
			specB = r
		}
	}
	require.NotNil(t, specA)
	require.True(t, specA.Acknowledged)
	require.Equal(t, "acked", specA.AcknowledgeNote)
	require.NotNil(t, specB)
	require.False(t, specB.Acknowledged)
}

func TestLifecycleHandler_AcknowledgeDrift_AmendedStage(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.acknowledgeDrift = func(_ context.Context, slug, note string) (*storage.DriftReport, error) {
		return &storage.DriftReport{
			SpecSlug:        slug,
			Acknowledged:    true,
			AcknowledgeNote: note,
		}, nil
	}
	deps.drift.check = func(_ context.Context, _, _ string) ([]storage.DriftReport, error) {
		return nil, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.AcknowledgeDrift(context.Background(), connect.NewRequest(&specv1.DriftAcknowledgeRequest{
		Slug: "amended-spec",
		Note: "divergence accepted",
	}))
	require.NoError(t, err)
	require.Equal(t, "amended-spec", resp.Msg.Report.SpecSlug)
	require.True(t, resp.Msg.Report.Acknowledged)
}

func TestLifecycleHandler_Abandon_NotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.abandonSpec = func(_ context.Context, _, _ string) (*storage.Spec, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "no-such-spec",
		Reason: "no longer needed",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Supersede_OldNotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "missing-old-spec",
		NewSlug: "new-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}

func TestLifecycleHandler_Supersede_PreconditionGetSpecFails(t *testing.T) {
	deps := defaultTestDeps()
	// Simulate preconditionError's double-failure path: atomic guard fails,
	// and the subsequent GetSpec re-read also fails with a non-NotFound error.
	deps.store.supersedeSpec = func(_ context.Context, _, _ string) (*storage.Spec, *storage.Spec, error) {
		return nil, nil, fmt.Errorf("supersede spec %q: atomic guard failed and precondition re-read also failed: %w",
			"old-spec", errors.New("connection reset"))
	}
	client := newLifecycleClient(t, deps)

	_, err := client.TransitionSupersede(context.Background(), connect.NewRequest(&specv1.TransitionSupersedeRequest{
		Slug:    "old-spec",
		NewSlug: "new-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInternal, connErr.Code())
}

func TestLifecycleHandler_Amend_EmptyReason(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:         "my-spec",
		Reason:       "",
		ReEntryStage: "shape",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Abandon_EmptyReason(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "dead-spec",
		Reason: "",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Lint_EmptySlug(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, slug string) ([]storage.LintResult, error) {
		require.Empty(t, slug, "empty slug means lint all specs")
		return []storage.LintResult{
			{SpecSlug: "spec-a", Passed: true},
			{SpecSlug: "spec-b", Passed: false, Violations: []storage.LintViolation{
				{Rule: "schema.enum", Severity: storage.LintSeverityError, Message: "bad stage"},
			}},
		}, nil
	}
	client := newLifecycleClient(t, deps)

	resp, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "",
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Results, 2)
}

func TestLifecycleHandler_Amend_ReasonExceedsMaxLen(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	longReason := strings.Repeat("x", 10001)
	_, err := client.TransitionAmend(context.Background(), connect.NewRequest(&specv1.TransitionAmendRequest{
		Slug:   "my-spec",
		Reason: longReason,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Abandon_ReasonExceedsMaxLen(t *testing.T) {
	client := newLifecycleClient(t, defaultTestDeps())
	longReason := strings.Repeat("x", 10001)
	_, err := client.TransitionAbandon(context.Background(), connect.NewRequest(&specv1.TransitionAbandonRequest{
		Slug:   "my-spec",
		Reason: longReason,
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeInvalidArgument, connErr.Code())
}

func TestLifecycleHandler_Lint_SpecNotFound(t *testing.T) {
	deps := defaultTestDeps()
	deps.linter.lint = func(_ context.Context, _ string) ([]storage.LintResult, error) {
		return nil, storage.ErrSpecNotFound
	}
	client := newLifecycleClient(t, deps)

	_, err := client.Lint(context.Background(), connect.NewRequest(&specv1.LintRequest{
		Slug: "no-such-spec",
	}))
	require.Error(t, err)
	var connErr *connect.Error
	require.ErrorAs(t, err, &connErr)
	require.Equal(t, connect.CodeNotFound, connErr.Code())
}
