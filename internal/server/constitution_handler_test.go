// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/constitution/fetch"
	"github.com/specgraph/specgraph/internal/constitution/merge"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConstitutionBackend struct {
	stubBackend
	mu      sync.Mutex
	layers  map[storage.ConstitutionLayer]*storage.Constitution
	version int32
}

func newMockConstitutionBackend() *mockConstitutionBackend {
	return &mockConstitutionBackend{
		layers: make(map[storage.ConstitutionLayer]*storage.Constitution),
	}
}

func (m *mockConstitutionBackend) GetConstitutionLayer(_ context.Context, layer storage.ConstitutionLayer) (*storage.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.layers[layer]
	if !ok {
		return nil, storage.ErrConstitutionNotFound
	}
	return c, nil
}

func (m *mockConstitutionBackend) GetAllLayers(_ context.Context) ([]*storage.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.layers) == 0 {
		return []*storage.Constitution{}, nil
	}
	// Return layers in a stable precedence order: user, org, project, domain.
	order := []storage.ConstitutionLayer{
		storage.ConstitutionLayerUser,
		storage.ConstitutionLayerOrg,
		storage.ConstitutionLayerProject,
		storage.ConstitutionLayerDomain,
	}
	result := make([]*storage.Constitution, 0, len(m.layers))
	for _, l := range order {
		if c, ok := m.layers[l]; ok {
			result = append(result, c)
		}
	}
	return result, nil
}

func (m *mockConstitutionBackend) GetMergedConstitution(ctx context.Context) (*storage.MergedResult, error) {
	layers, err := m.GetAllLayers(ctx)
	if err != nil {
		return nil, err
	}
	if len(layers) == 0 {
		return nil, storage.ErrConstitutionNotFound
	}
	return merge.Layers(layers)
}

func (m *mockConstitutionBackend) UpdateConstitution(_ context.Context, c *storage.Constitution) (*storage.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version++
	c.Version = m.version
	m.layers[c.Layer] = c
	return c, nil
}

func setupConstitutionServer(t *testing.T) specgraphv1connect.ConstitutionServiceClient {
	t.Helper()
	mb := newMockConstitutionBackend()
	scoper := &testScoper{backend: mb}
	mux := http.NewServeMux()
	server.RegisterConstitutionService(mux, scoper)
	srv := httptest.NewServer(wrapTestProject(mux))
	t.Cleanup(srv.Close)
	return specgraphv1connect.NewConstitutionServiceClient(http.DefaultClient, srv.URL)
}

func TestConstitutionHandler_GetNotFound(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConstitutionHandler_UpdateAndGet(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	updateResp, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Name:  "specgraph",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{Primary: "go"},
			},
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, updateResp.Msg.Constitution)
	require.Equal(t, "specgraph", updateResp.Msg.Constitution.Name)
	require.Equal(t, int32(1), updateResp.Msg.Constitution.Version)

	// GetConstitution with no layer filter merges all layers via the merge engine.
	// Name is not a merged field; verify the response is non-nil and that merged
	// fields (Tech) are present.
	getResp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Constitution)
	require.NotNil(t, getResp.Msg.Constitution.Tech)
	require.Equal(t, "go", getResp.Msg.Constitution.Tech.Languages.Primary)
}

func TestConstitutionHandler_UpdateNilBody(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: nil,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConstitutionHandler_GetByLayer(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	// Store a constitution at the org layer.
	_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Name:  "org-constitution",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{Primary: "go"},
			},
		},
	}))
	require.NoError(t, err)

	// Query with a specific layer filter — exercises the single-layer branch.
	resp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
		Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
	}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Constitution)
	require.Equal(t, specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG, resp.Msg.Constitution.Layer)
	require.NotNil(t, resp.Msg.Constitution.Tech)
	require.Equal(t, "go", resp.Msg.Constitution.Tech.Languages.Primary)

	// Single-layer response has no provenance (no merge performed).
	require.Empty(t, resp.Msg.Provenance)
}

func TestConstitutionHandler_EmitNotFound(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestConstitutionHandler_EmitUnsupportedFormat(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	// First store a constitution
	_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Name:  "test",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		},
	}))
	require.NoError(t, err)

	// Try to emit with unsupported format (out-of-range enum value)
	_, err = client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: specv1.OutputFormat(99),
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConstitutionHandler_EmitUnspecifiedFormat(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: specv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED,
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConstitutionHandler_EmitSuccess(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Name:  "test-project",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
			Tech: &specv1.TechConfig{
				Languages: &specv1.LanguageConfig{
					Primary: "go",
				},
				Frameworks: map[string]string{
					"api": "ConnectRPC",
				},
			},
			Principles: []*specv1.Principle{
				{Statement: "Keep it simple"},
			},
			Constraints: []string{"no global state"},
		},
	}))
	require.NoError(t, err)

	resp, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: specv1.OutputFormat_OUTPUT_FORMAT_CLAUDE_MD,
	}))
	require.NoError(t, err)
	require.Equal(t, "CLAUDE.md", resp.Msg.Filename)
	assert.NotEmpty(t, resp.Msg.Content)
	assert.Contains(t, resp.Msg.Content, "Constitution")
	assert.Contains(t, resp.Msg.Content, "go")
	assert.Contains(t, resp.Msg.Content, "ConnectRPC")
}

// fakeFetcher returns canned bytes or an error.
type fakeFetcher struct {
	body []byte
	err  error
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) (*fetch.Fetched, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &fetch.Fetched{Body: f.body, ResolvedURL: url}, nil
}

// testScoperFor wraps a mockConstitutionBackend as a storage.Scoper.
func testScoperFor(b *mockConstitutionBackend) *testScoper {
	return &testScoper{backend: b}
}

func TestRefreshConstitutionLayer_NewLayer_HashUnset(t *testing.T) {
	backend := newMockConstitutionBackend()
	fake := &fakeFetcher{
		body: []byte("name: imported\nlayer: org\nprinciples:\n  - id: p1\n    statement: from remote\n"),
	}
	handler := server.NewConstitutionHandlerForTest(testScoperFor(backend), fake)
	ctx := server.TestInjectProject(context.Background(), testProject)

	resp, err := handler.RefreshConstitutionLayer(ctx,
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: "https://example.com/c.yaml",
		}))

	require.NoError(t, err)
	assert.True(t, resp.Msg.GetChanged(), "first refresh must be changed=true")
	assert.Nil(t, resp.Msg.GetBefore(), "no prior layer; Before must be nil")
	require.NotNil(t, resp.Msg.GetAfter())
	assert.Equal(t, "imported", resp.Msg.GetAfter().GetName())
	assert.NotEmpty(t, resp.Msg.GetNewSourceHash())
}

func TestRefreshConstitutionLayer_SameContent_NoChange(t *testing.T) {
	backend := newMockConstitutionBackend()
	body := []byte("name: cached\nlayer: org\n")
	fake := &fakeFetcher{body: body}
	handler := server.NewConstitutionHandlerForTest(testScoperFor(backend), fake)
	ctx := server.TestInjectProject(context.Background(), testProject)

	req := &specv1.RefreshConstitutionLayerRequest{
		Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
		SourceUrl: "https://example.com/c.yaml",
	}
	_, err := handler.RefreshConstitutionLayer(ctx, connect.NewRequest(req))
	require.NoError(t, err)

	// Second refresh on identical content — must report unchanged.
	resp, err := handler.RefreshConstitutionLayer(ctx, connect.NewRequest(req))
	require.NoError(t, err)
	assert.False(t, resp.Msg.GetChanged())
	assert.Equal(t, resp.Msg.GetPreviousSourceHash(), resp.Msg.GetNewSourceHash())
}

func TestRefreshConstitutionLayer_DryRun_DoesNotWrite(t *testing.T) {
	backend := newMockConstitutionBackend()
	fake := &fakeFetcher{body: []byte("name: test\nlayer: org\n")}
	handler := server.NewConstitutionHandlerForTest(testScoperFor(backend), fake)
	ctx := server.TestInjectProject(context.Background(), testProject)

	resp, err := handler.RefreshConstitutionLayer(ctx,
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: "https://example.com/c.yaml",
			DryRun:    true,
		}))

	require.NoError(t, err)
	assert.True(t, resp.Msg.GetChanged(), "dry-run still reports changed")

	// Verify nothing was written by checking that GetConstitutionLayer
	// still returns not-found.
	_, err = backend.GetConstitutionLayer(context.Background(), storage.ConstitutionLayerOrg)
	require.Error(t, err, "dry-run must not write to storage")
}

func TestRefreshConstitutionLayer_UnspecifiedLayer(t *testing.T) {
	handler := server.NewConstitutionHandlerForTest(testScoperFor(newMockConstitutionBackend()), &fakeFetcher{body: []byte("name: x\n")})

	_, err := handler.RefreshConstitutionLayer(context.Background(),
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED,
			SourceUrl: "https://example.com/c.yaml",
		}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefreshConstitutionLayer_FetchError(t *testing.T) {
	handler := server.NewConstitutionHandlerForTest(testScoperFor(newMockConstitutionBackend()), &fakeFetcher{err: errors.New("network down")})
	ctx := server.TestInjectProject(context.Background(), testProject)

	_, err := handler.RefreshConstitutionLayer(ctx,
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: "https://example.com/c.yaml",
		}))

	require.Error(t, err)
	// Per Section 12: generic fetch failures map to CodeUnavailable.
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestRefreshConstitutionLayer_MalformedBody(t *testing.T) {
	handler := server.NewConstitutionHandlerForTest(testScoperFor(newMockConstitutionBackend()), &fakeFetcher{body: []byte("name: [unclosed")})
	ctx := server.TestInjectProject(context.Background(), testProject)

	_, err := handler.RefreshConstitutionLayer(ctx,
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: "https://example.com/c.yaml",
		}))

	require.Error(t, err)
	// Parse errors map to CodeInvalidArgument.
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestRefreshConstitutionLayer_URLValidationError(t *testing.T) {
	// Embedded credentials in URL — the real fetcher rejects with a
	// message containing "embedded credentials". The fake here mimics
	// that error so we can verify the handler maps it to InvalidArgument.
	handler := server.NewConstitutionHandlerForTest(testScoperFor(newMockConstitutionBackend()),
		&fakeFetcher{err: errors.New("URL contains embedded credentials; use SPECGRAPH_FETCH_GITHUB_TOKEN env var for authenticated GitHub access")})
	ctx := server.TestInjectProject(context.Background(), testProject)

	_, err := handler.RefreshConstitutionLayer(ctx,
		connect.NewRequest(&specv1.RefreshConstitutionLayerRequest{
			Layer:     specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG,
			SourceUrl: "https://tok@example.com/c.yaml",
		}))

	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
