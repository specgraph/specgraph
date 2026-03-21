// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockConstitutionBackend struct {
	stubBackend
	mu           sync.Mutex
	constitution *storage.Constitution
	version      int32
}

func newMockConstitutionBackend() *mockConstitutionBackend {
	return &mockConstitutionBackend{}
}

func (m *mockConstitutionBackend) GetConstitution(_ context.Context) (*storage.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.constitution == nil {
		return nil, storage.ErrConstitutionNotFound
	}
	return m.constitution, nil
}

func (m *mockConstitutionBackend) UpdateConstitution(_ context.Context, c *storage.Constitution) (*storage.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version++
	c.Version = m.version
	m.constitution = c
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
		},
	}))
	require.NoError(t, err)
	require.NotNil(t, updateResp.Msg.Constitution)
	require.Equal(t, "specgraph", updateResp.Msg.Constitution.Name)
	require.Equal(t, int32(1), updateResp.Msg.Constitution.Version)

	getResp, err := client.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
	require.NoError(t, err)
	require.NotNil(t, getResp.Msg.Constitution)
	require.Equal(t, "specgraph", getResp.Msg.Constitution.Name)
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
