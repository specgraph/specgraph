// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/seanb4t/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

type mockConstitutionBackend struct {
	mu           sync.Mutex
	constitution *specv1.Constitution
	version      int32
	violations   map[string][]*specv1.Violation
}

func newMockConstitutionBackend() *mockConstitutionBackend {
	return &mockConstitutionBackend{
		violations: make(map[string][]*specv1.Violation),
	}
}

func (m *mockConstitutionBackend) GetConstitution(_ context.Context) (*specv1.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.constitution == nil {
		return nil, storage.ErrConstitutionNotFound
	}
	return m.constitution, nil
}

func (m *mockConstitutionBackend) UpdateConstitution(_ context.Context, c *specv1.Constitution) (*specv1.Constitution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version++
	c.Version = m.version
	m.constitution = c
	return c, nil
}

func (m *mockConstitutionBackend) CheckViolation(_ context.Context, specSlug string) ([]*specv1.Violation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if specSlug == "nonexistent-spec" {
		return nil, fmt.Errorf("spec %q: %w", specSlug, storage.ErrSpecNotFound)
	}
	violations := m.violations[specSlug]
	return violations, nil
}

func setupConstitutionServer(t *testing.T) specgraphv1connect.ConstitutionServiceClient {
	t.Helper()
	mb := newMockConstitutionBackend()
	mux := http.NewServeMux()
	server.RegisterConstitutionService(mux, mb)
	srv := httptest.NewServer(mux)
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

func TestConstitutionHandler_CheckViolation(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	// First set up a constitution
	_, err := client.UpdateConstitution(ctx, connect.NewRequest(&specv1.UpdateConstitutionRequest{
		Constitution: &specv1.Constitution{
			Name:  "specgraph",
			Layer: specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT,
		},
	}))
	require.NoError(t, err)

	// Check a spec that has no violations
	checkResp, err := client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: "my-spec",
	}))
	require.NoError(t, err)
	require.Empty(t, checkResp.Msg.Violations)

	// Check a spec that does not exist
	_, err = client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: "nonexistent-spec",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	// Check with empty slug returns InvalidArgument
	_, err = client.CheckViolation(ctx, connect.NewRequest(&specv1.CheckViolationRequest{
		SpecSlug: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
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
		Format: "claude-md",
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

	// Try to emit with unsupported format
	_, err = client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: "unknown-format",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestConstitutionHandler_EmitEmptyFormat(t *testing.T) {
	client := setupConstitutionServer(t)
	ctx := context.Background()

	_, err := client.EmitToolFiles(ctx, connect.NewRequest(&specv1.EmitToolFilesRequest{
		Format: "",
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}
