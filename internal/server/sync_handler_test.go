// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	gosync "sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	syncpkg "github.com/seanb4t/specgraph/internal/sync"
	"github.com/stretchr/testify/require"
)

type mockSyncBackend struct {
	mu       gosync.Mutex
	mappings map[string]*storage.SyncMapping // key: "slug:adapter"
}

func newMockSyncBackend() *mockSyncBackend {
	return &mockSyncBackend{
		mappings: map[string]*storage.SyncMapping{},
	}
}

func (m *mockSyncBackend) key(slug string, adapter storage.SyncAdapterType) string {
	return fmt.Sprintf("%s:%s", slug, string(adapter))
}

func (m *mockSyncBackend) CreateSyncMapping(_ context.Context, specSlug string, adapter storage.SyncAdapterType, externalID string) (*storage.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	if _, exists := m.mappings[k]; exists {
		return nil, storage.ErrSyncMappingExists
	}
	mapping := &storage.SyncMapping{
		SpecSlug:   specSlug,
		Adapter:    adapter,
		ExternalID: externalID,
		State:      storage.SyncStateSynced,
		LastSync:   time.Now(),
		CreatedAt:  time.Now(),
	}
	m.mappings[k] = mapping
	return mapping, nil
}

func (m *mockSyncBackend) UpdateSyncState(_ context.Context, specSlug string, adapter storage.SyncAdapterType, state storage.SyncStateType, errorMessage string) (*storage.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	mapping, exists := m.mappings[k]
	if !exists {
		return nil, storage.ErrSyncMappingNotFound
	}
	mapping.State = state
	mapping.ErrorMessage = errorMessage
	return mapping, nil
}

func (m *mockSyncBackend) GetSyncMapping(_ context.Context, specSlug string, adapter storage.SyncAdapterType) (*storage.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	mapping, exists := m.mappings[k]
	if !exists {
		return nil, storage.ErrSyncMappingNotFound
	}
	return mapping, nil
}

func (m *mockSyncBackend) ListSyncMappings(_ context.Context, adapter storage.SyncAdapterType, specSlug string) ([]*storage.SyncMapping, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []*storage.SyncMapping
	for _, mapping := range m.mappings {
		if adapter != "" && mapping.Adapter != adapter {
			continue
		}
		if specSlug != "" && mapping.SpecSlug != specSlug {
			continue
		}
		result = append(result, mapping)
	}
	return result, nil
}

func (m *mockSyncBackend) DeleteSyncMapping(_ context.Context, specSlug string, adapter storage.SyncAdapterType) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := m.key(specSlug, adapter)
	delete(m.mappings, k)
	return nil
}

type mockSpecReader struct {
	specs map[string]*storage.Spec
}

func (m *mockSpecReader) GetSpec(_ context.Context, slug string) (*storage.Spec, error) {
	spec, ok := m.specs[slug]
	if !ok {
		return nil, storage.ErrSpecNotFound
	}
	return spec, nil
}

func (m *mockSpecReader) ListSpecs(_ context.Context, stage, priority string, _ int) ([]*storage.Spec, error) {
	var result []*storage.Spec
	for _, spec := range m.specs {
		if stage != "" && string(spec.Stage) != stage {
			continue
		}
		if priority != "" && string(spec.Priority) != priority {
			continue
		}
		result = append(result, spec)
	}
	return result, nil
}

func (m *mockSpecReader) GetDependencies(_ context.Context, _ string) ([]storage.NodeRef, error) {
	return nil, nil
}

var (
	_ storage.SpecReader  = (*mockSpecReader)(nil)
	_ storage.SyncBackend = (*mockSyncBackend)(nil)
)

func setupSyncServer(t *testing.T) specgraphv1connect.SyncServiceClient {
	t.Helper()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID:         "spec-test123",
				Slug:       "test-spec",
				Intent:     "Test spec for sync",
				Stage:      "approved",
				Priority:   "p2",
				Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)
}

func TestSyncHandler_GetSyncStatus_Empty(t *testing.T) {
	client := setupSyncServer(t)
	resp, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Mappings)
}

func TestSyncHandler_GetSyncStatus_WithMappings(t *testing.T) {
	syncStore := newMockSyncBackend()
	syncStore.mappings["spec-a:github"] = &storage.SyncMapping{
		SpecSlug:   "spec-a",
		Adapter:    storage.SyncAdapterGitHub,
		ExternalID: "gh-1",
		State:      storage.SyncStateSynced,
		LastSync:   time.Now(),
		CreatedAt:  time.Now(),
	}

	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, &mockSpecReader{specs: map[string]*storage.Spec{}}, nil, "")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.Mappings, 1)
	require.Equal(t, "gh-1", resp.Msg.Mappings[0].ExternalId)
}

func TestSyncHandler_Inject_SpecNotFound(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug: "nonexistent",
			Tool:     specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestSyncHandler_Inject_MissingSlug(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			Tool: specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSyncHandler_Inject_MissingTool(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug: "test-spec",
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestSyncHandler_Inject_Success(t *testing.T) {
	outputDir := t.TempDir()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID:         "spec-test123",
				Slug:       "test-spec",
				Intent:     "Test spec for sync",
				Stage:      "approved",
				Priority:   "p2",
				Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.SetAllowedOutputRoot(outputDir)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.FilesWritten, 1)
	require.Contains(t, resp.Msg.Summary, "test-spec")
}

func TestSyncHandler_Inject_SuccessCursor(t *testing.T) {
	outputDir := t.TempDir()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID: "spec-test123", Slug: "test-spec", Intent: "Test spec for sync",
				Stage: "approved", Priority: "p2", Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.SetAllowedOutputRoot(outputDir)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CURSOR,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.FilesWritten, 1)
	require.Contains(t, resp.Msg.Summary, "test-spec")
}

func TestSyncHandler_Inject_SuccessAgentsMD(t *testing.T) {
	outputDir := t.TempDir()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID: "spec-test123", Slug: "test-spec", Intent: "Test spec for sync",
				Stage: "approved", Priority: "p2", Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.SetAllowedOutputRoot(outputDir)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_AGENTS_MD,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.FilesWritten, 1)
	require.Contains(t, resp.Msg.Summary, "test-spec")
}

// mockAdapter implements syncpkg.Adapter for testing syncWithAdapter.
type mockAdapter struct {
	name      storage.SyncAdapterType
	pushFn    func(ctx context.Context, spec *storage.Spec) (string, error)
	available bool
}

func (m *mockAdapter) Name() storage.SyncAdapterType { return m.name }
func (m *mockAdapter) Available(_ context.Context) error {
	if !m.available {
		return fmt.Errorf("adapter not available")
	}
	return nil
}

func (m *mockAdapter) Push(ctx context.Context, spec *storage.Spec) (string, error) {
	return m.pushFn(ctx, spec)
}
func (m *mockAdapter) Pull(_ context.Context, _ string) (string, error) { return "", nil }

var _ syncpkg.Adapter = (*mockAdapter)(nil)

func setupSyncServerWithAdapter(t *testing.T, adapter syncpkg.Adapter) specgraphv1connect.SyncServiceClient {
	t.Helper()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID:       "spec-test123",
				Slug:     "test-spec",
				Intent:   "Test spec for sync",
				Stage:    "approved",
				Priority: "p2",
			},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)
}

func TestSyncHandler_SyncBeads_NoAdapter(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestSyncHandler_SyncBeads_Success(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Synced)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, specv1.SyncState_SYNC_STATE_SYNCED, resp.Msg.Results[0].State)
}

func TestSyncHandler_SyncBeads_PushError(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			return "", fmt.Errorf("push failed")
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errors)
	require.Equal(t, "failed to push to adapter", resp.Msg.Results[0].Message)
}

func TestSyncHandler_SyncBeads_DryRun(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			t.Fatal("push should not be called in dry run")
			return "", nil
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{
				DryRun: true,
			},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Synced)
	require.Equal(t, int32(1), resp.Msg.DryRunCount)
	require.Len(t, resp.Msg.Results, 1)
	require.Equal(t, specv1.SyncState_SYNC_STATE_PENDING, resp.Msg.Results[0].State)
}

func TestSyncHandler_SyncBeads_AlreadySynced(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	// Setup with pre-existing mapping
	syncStore := newMockSyncBackend()
	syncStore.mappings["test-spec:beads"] = &storage.SyncMapping{
		SpecSlug:   "test-spec",
		Adapter:    storage.SyncAdapterBeads,
		ExternalID: "beads-existing",
		State:      storage.SyncStateSynced,
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Skipped)
	require.Equal(t, "already synced", resp.Msg.Results[0].Message)
}

func TestSyncHandler_SyncGitHub_NoAdapter(t *testing.T) {
	client := setupSyncServer(t) // no adapters registered
	_, err := client.SyncGitHub(context.Background(),
		connect.NewRequest(&specv1.SyncGitHubRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestSyncHandler_SyncGitHub_Success(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterGitHub,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			return "https://github.com/owner/repo/issues/1", nil
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	resp, err := client.SyncGitHub(context.Background(),
		connect.NewRequest(&specv1.SyncGitHubRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Synced)
}

func TestSyncHandler_GetSyncStatus_InvalidAdapter(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{
			Adapter: specv1.SyncAdapter(99),
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// errorSyncBackend wraps mockSyncBackend to inject errors for specific methods.
type errorSyncBackend struct {
	*mockSyncBackend
	getSyncMappingErr    error
	createSyncMappingErr error
	listSyncMappingsErr  error
}

func (e *errorSyncBackend) GetSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType) (*storage.SyncMapping, error) {
	if e.getSyncMappingErr != nil {
		return nil, e.getSyncMappingErr
	}
	return e.mockSyncBackend.GetSyncMapping(ctx, specSlug, adapter)
}

func (e *errorSyncBackend) ListSyncMappings(ctx context.Context, adapter storage.SyncAdapterType, specSlug string) ([]*storage.SyncMapping, error) {
	if e.listSyncMappingsErr != nil {
		return nil, e.listSyncMappingsErr
	}
	return e.mockSyncBackend.ListSyncMappings(ctx, adapter, specSlug)
}

func (e *errorSyncBackend) CreateSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, externalID string) (*storage.SyncMapping, error) {
	if e.createSyncMappingErr != nil {
		return nil, e.createSyncMappingErr
	}
	return e.mockSyncBackend.CreateSyncMapping(ctx, specSlug, adapter, externalID)
}

func TestSyncHandler_SyncBeads_GetSyncMappingError(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			t.Fatal("push should not be called when GetSyncMapping fails")
			return "", nil
		},
	}
	syncStore := &errorSyncBackend{
		mockSyncBackend:   newMockSyncBackend(),
		getSyncMappingErr: fmt.Errorf("database connection lost"),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errors)
	require.Equal(t, "failed to check sync state", resp.Msg.Results[0].Message)
}

func TestSyncHandler_SyncBeads_CreateSyncMappingError(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	syncStore := &errorSyncBackend{
		mockSyncBackend:      newMockSyncBackend(),
		createSyncMappingErr: fmt.Errorf("disk full"),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Errors)
	require.Contains(t, resp.Msg.Results[0].Message, "pushed to adapter")
	require.NotContains(t, resp.Msg.Results[0].Message, "beads-test-spec", "externalID must not leak into client-visible message")
}

func TestSyncHandler_SyncBeads_AdapterAvailableError(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: false,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			t.Fatal("push should not be called when adapter is unavailable")
			return "", nil
		},
	}
	client := setupSyncServerWithAdapter(t, adapter)
	_, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

type errorSpecReader struct {
	*mockSpecReader
	listSpecsErr error
}

func (e *errorSpecReader) ListSpecs(_ context.Context, _, _ string, _ int) ([]*storage.Spec, error) {
	if e.listSpecsErr != nil {
		return nil, e.listSpecsErr
	}
	return e.mockSpecReader.ListSpecs(context.Background(), "", "", 0)
}

func TestSyncHandler_SyncBeads_ListSpecsError(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			t.Fatal("push should not be called when ListSpecs fails")
			return "", nil
		},
	}
	specStore := &errorSpecReader{
		mockSpecReader: &mockSpecReader{specs: map[string]*storage.Spec{}},
		listSpecsErr:   fmt.Errorf("database unavailable"),
	}
	syncStore := newMockSyncBackend()
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	_, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSyncHandler_GetSyncStatus_ListSyncMappingsError(t *testing.T) {
	syncStore := &errorSyncBackend{
		mockSyncBackend:     newMockSyncBackend(),
		listSyncMappingsErr: fmt.Errorf("storage unavailable"),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	_, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSyncHandler_GetSyncStatus_ConversionFailure(t *testing.T) {
	syncStore := newMockSyncBackend()
	syncStore.mappings["spec-a:invalid"] = &storage.SyncMapping{
		SpecSlug:   "spec-a",
		Adapter:    storage.SyncAdapterType("invalid"),
		ExternalID: "ext-1",
		State:      storage.SyncStateSynced,
		LastSync:   time.Now(),
		CreatedAt:  time.Now(),
	}

	mux := http.NewServeMux()
	server.RegisterSyncService(mux, syncStore, &mockSpecReader{specs: map[string]*storage.Spec{}}, nil, "")
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	_, err := client.GetSyncStatus(context.Background(),
		connect.NewRequest(&specv1.SyncStatusRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestSyncHandler_SyncBeads_CreateSyncMappingExists(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	syncStore := &errorSyncBackend{
		mockSyncBackend:      newMockSyncBackend(),
		createSyncMappingErr: storage.ErrSyncMappingExists,
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Skipped)
	require.Contains(t, resp.Msg.Results[0].Message, "already synced (concurrent sync detected)")
}

func TestSyncHandler_Inject_ConstitutionWarning(t *testing.T) {
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID:         "spec-test123",
				Slug:       "test-spec",
				Intent:     "Test spec",
				Stage:      "approved",
				Priority:   "p2",
				Complexity: "medium",
			},
		},
	}
	conStore := &mockConstitutionStore{err: fmt.Errorf("db connection lost")}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, conStore, "")
	outputDir := t.TempDir()
	handler.SetAllowedOutputRoot(outputDir)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)
	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Contains(t, resp.Msg.Summary, "warning: constitution load failed: storage error")
	require.Len(t, resp.Msg.Warnings, 1)
	require.Equal(t, "constitution load failed: storage error", resp.Msg.Warnings[0])
}

// mockConstitutionStore always returns an error for GetConstitution.
type mockConstitutionStore struct {
	err error
}

func (m *mockConstitutionStore) GetConstitution(_ context.Context) (*storage.Constitution, error) {
	return nil, m.err
}

func (m *mockConstitutionStore) UpdateConstitution(_ context.Context, c *storage.Constitution) (*storage.Constitution, error) {
	return c, nil
}

func (m *mockConstitutionStore) CheckViolation(_ context.Context, _ string) ([]storage.Violation, error) {
	return nil, nil
}

func TestSyncHandler_Inject_ConstitutionNotFound_NoWarning(t *testing.T) {
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID: "spec-test123", Slug: "test-spec", Intent: "Test spec",
				Stage: "approved", Priority: "p2", Complexity: "medium",
			},
		},
	}
	// ErrConstitutionNotFound is a normal state — should NOT produce a warning.
	conStore := &mockConstitutionStore{err: storage.ErrConstitutionNotFound}
	outputDir := t.TempDir()
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, conStore, "")
	handler.SetAllowedOutputRoot(outputDir)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Empty(t, resp.Msg.Warnings, "ErrConstitutionNotFound should not produce warnings")
	require.NotContains(t, resp.Msg.Summary, "warning:")
}

func TestSyncHandler_Inject_OutputDirOutsideRoot(t *testing.T) {
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.SetAllowedOutputRoot(t.TempDir())
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
			OutputDir: "/tmp/evil-path",
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// countingSyncBackend wraps mockSyncBackend to fail CreateSyncMapping N times then succeed.
type countingSyncBackend struct {
	*mockSyncBackend
	createFailCount int
	createErr       error
	calls           int
}

func (c *countingSyncBackend) CreateSyncMapping(ctx context.Context, specSlug string, adapter storage.SyncAdapterType, externalID string) (*storage.SyncMapping, error) {
	c.calls++
	if c.calls <= c.createFailCount {
		return nil, c.createErr
	}
	return c.mockSyncBackend.CreateSyncMapping(ctx, specSlug, adapter, externalID)
}

func TestSyncHandler_SyncBeads_RetrySuccess(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	syncStore := &countingSyncBackend{
		mockSyncBackend: newMockSyncBackend(),
		createFailCount: 1,
		createErr:       fmt.Errorf("transient error"),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(1), resp.Msg.Synced)
	require.Equal(t, "synced", resp.Msg.Results[0].Message)
}

// retryExistsSyncBackend returns a transient error on first CreateSyncMapping call,
// then ErrSyncMappingExists on retry (simulating a concurrent sync winning the race).
type retryExistsSyncBackend struct {
	*mockSyncBackend
	createCalls int
}

func (r *retryExistsSyncBackend) CreateSyncMapping(_ context.Context, _ string, _ storage.SyncAdapterType, _ string) (*storage.SyncMapping, error) {
	r.createCalls++
	if r.createCalls == 1 {
		return nil, fmt.Errorf("transient error")
	}
	return nil, storage.ErrSyncMappingExists
}

func TestSyncHandler_SyncBeads_RetryExistsCountsAsSkipped(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			return "beads-" + spec.Slug, nil
		},
	}
	syncStore := &retryExistsSyncBackend{
		mockSyncBackend: newMockSyncBackend(),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	// First CreateSyncMapping fails with transient error, retry returns ErrSyncMappingExists
	// (concurrent sync won the race). Should count as skipped, not synced.
	require.Equal(t, int32(1), resp.Msg.Skipped)
	require.Contains(t, resp.Msg.Results[0].Message, "already synced (concurrent sync detected)")
}

// cancelledCtxSyncBackend fails on first CreateSyncMapping, then the test verifies
// the context-cancelled path prevents a retry.
type cancelledCtxSyncBackend struct {
	*mockSyncBackend
	createCalls int
}

func (c *cancelledCtxSyncBackend) CreateSyncMapping(_ context.Context, _ string, _ storage.SyncAdapterType, _ string) (*storage.SyncMapping, error) {
	c.createCalls++
	return nil, fmt.Errorf("transient error")
}

func TestSyncHandler_SyncBeads_ContextCancelledBeforeRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, spec *storage.Spec) (string, error) {
			// Cancel the context after push succeeds — handler checks ctx.Err()
			// before retrying CreateSyncMapping.
			cancel()
			return "beads-" + spec.Slug, nil
		},
	}
	syncStore := &cancelledCtxSyncBackend{
		mockSyncBackend: newMockSyncBackend(),
	}
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {ID: "spec-test123", Slug: "test-spec", Stage: "approved"},
		},
	}
	// Call the handler method directly (not via HTTP) to avoid context
	// cancellation affecting the transport layer.
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)

	resp, err := handler.SyncBeads(ctx,
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{},
		}))
	require.NoError(t, err)
	// Context was cancelled after push but before retry — should be an error result
	// with ExternalId preserved for reconciliation.
	require.Equal(t, int32(1), resp.Msg.Errors)
	require.Equal(t, specv1.SyncState_SYNC_STATE_ERROR, resp.Msg.Results[0].State)
	require.NotEmpty(t, resp.Msg.Results[0].ExternalId)
	require.Contains(t, resp.Msg.Results[0].Message, "reconciliation")
}

func TestSyncHandler_Inject_EmptyOutputDir(t *testing.T) {
	client := setupSyncServer(t)
	_, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug: "test-spec",
			Tool:     specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
		}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

// TestSyncHandler_Inject_NoAllowedOutputRoot verifies that when RegisterSyncService
// is called with allowedOutputRoot="" the Inject handler accepts any output_dir
// without path validation (unrestricted mode).
func TestSyncHandler_Inject_NoAllowedOutputRoot(t *testing.T) {
	outputDir := t.TempDir()
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"test-spec": {
				ID:         "spec-test123",
				Slug:       "test-spec",
				Intent:     "Test spec for sync",
				Stage:      "approved",
				Priority:   "p2",
				Complexity: "medium",
			},
		},
	}
	mux := http.NewServeMux()
	// Pass allowedOutputRoot="" — no root restriction.
	server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	// Any output_dir should be accepted when allowedOutputRoot is empty.
	resp, err := client.Inject(context.Background(),
		connect.NewRequest(&specv1.InjectRequest{
			SpecSlug:  "test-spec",
			Tool:      specv1.InjectTool_INJECT_TOOL_CLAUDE_CODE,
			OutputDir: outputDir,
		}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.FilesWritten, 1)
	require.Contains(t, resp.Msg.Summary, "test-spec")
}

// TestSyncHandler_SyncBeads_DryRunCount verifies that syncWithAdapter correctly
// increments dry_run_count (not synced/errors/skipped) for each spec when
// dry_run is true.
func TestSyncHandler_SyncBeads_DryRunCount(t *testing.T) {
	adapter := &mockAdapter{
		name:      storage.SyncAdapterBeads,
		available: true,
		pushFn: func(_ context.Context, _ *storage.Spec) (string, error) {
			t.Fatal("push must not be called in dry run mode")
			return "", nil
		},
	}
	syncStore := newMockSyncBackend()
	specStore := &mockSpecReader{
		specs: map[string]*storage.Spec{
			"spec-alpha": {ID: "spec-alpha-id", Slug: "spec-alpha", Stage: "approved"},
			"spec-beta":  {ID: "spec-beta-id", Slug: "spec-beta", Stage: "approved"},
		},
	}
	mux := http.NewServeMux()
	handler := server.RegisterSyncService(mux, syncStore, specStore, nil, "")
	handler.RegisterAdapter(adapter)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	client := specgraphv1connect.NewSyncServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.SyncBeads(context.Background(),
		connect.NewRequest(&specv1.SyncBeadsRequest{
			Config: &specv1.SyncConfig{
				DryRun: true,
			},
		}))
	require.NoError(t, err)
	require.Equal(t, int32(0), resp.Msg.Synced)
	require.Equal(t, int32(0), resp.Msg.Errors)
	require.Equal(t, int32(0), resp.Msg.Skipped)
	require.Equal(t, int32(2), resp.Msg.DryRunCount)
	require.Len(t, resp.Msg.Results, 2)
	for _, result := range resp.Msg.Results {
		require.Equal(t, specv1.SyncState_SYNC_STATE_PENDING, result.State)
	}
}
