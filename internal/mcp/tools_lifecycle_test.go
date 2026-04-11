// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/base64"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// driftTool tests
// ---------------------------------------------------------------------------

func TestDriftTool_Check(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		checkDrift: func(req *specv1.DriftCheckRequest) (*specv1.DriftCheckResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_DEPS, req.GetScope())
			return &specv1.DriftCheckResponse{
				Reports: []*specv1.DriftReport{
					{SpecSlug: "my-spec"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "check",
		"slug":   "my-spec",
		"scope":  "deps",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-spec")
}

func TestDriftTool_Check_AllSpecs(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		checkDrift: func(req *specv1.DriftCheckRequest) (*specv1.DriftCheckResponse, error) {
			require.Equal(t, "", req.GetSlug())
			return &specv1.DriftCheckResponse{
				SkippedCount: 3,
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "check",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestDriftTool_Acknowledge(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		acknowledgeDrift: func(req *specv1.DriftAcknowledgeRequest) (*specv1.DriftAcknowledgeResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, "upstream changed but we reviewed it", req.GetNote())
			require.True(t, req.GetAll())
			return &specv1.DriftAcknowledgeResponse{
				Report: &specv1.DriftReport{SpecSlug: "my-spec"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "acknowledge",
		"slug":   "my-spec",
		"note":   "upstream changed but we reviewed it",
		"all":    true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-spec")
}

func TestDriftTool_Acknowledge_MissingSlug(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "acknowledge",
		"note":   "some note",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestDriftTool_Acknowledge_MissingNote(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "acknowledge",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "note")
}

func TestDriftTool_UnknownAction(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("drift")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// lintTool tests
// ---------------------------------------------------------------------------

func TestLintTool(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		lint: func(req *specv1.LintRequest) (*specv1.LintResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			return &specv1.LintResponse{
				Results: []*specv1.LintResult{
					{SpecSlug: "my-spec", Passed: true},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("lint")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug": "my-spec",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-spec")
}

func TestLintTool_AllSpecs(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{
		lint: func(req *specv1.LintRequest) (*specv1.LintResponse, error) {
			require.Equal(t, "", req.GetSlug())
			return &specv1.LintResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("lint")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

// ---------------------------------------------------------------------------
// syncTool tests
// ---------------------------------------------------------------------------

func TestSyncTool_Status(t *testing.T) {
	c := &Client{Sync: &mockSyncService{
		getSyncStatus: func() (*specv1.SyncStatusResponse, error) {
			return &specv1.SyncStatusResponse{
				Mappings: []*specv1.SyncMapping{
					{SpecSlug: "my-spec"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("sync")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "status",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-spec")
}

func TestSyncTool_Beads(t *testing.T) {
	c := &Client{Sync: &mockSyncService{
		syncBeads: func() (*specv1.SyncResponse, error) {
			return &specv1.SyncResponse{
				Synced: 5,
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("sync")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "beads",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSyncTool_UnknownAction(t *testing.T) {
	c := &Client{Sync: &mockSyncService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("sync")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// exportTool tests
// ---------------------------------------------------------------------------

func TestExportTool_Export(t *testing.T) {
	exportData := []byte(`{"project":"my-project"}`)
	c := &Client{Export: &mockExportService{
		exportProject: func(projectSlug string) (*specv1.ExportProjectResponse, error) {
			require.Equal(t, "my-project", projectSlug)
			return &specv1.ExportProjectResponse{
				Data: exportData,
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":       "export",
		"project_slug": "my-project",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	// Result should be base64 encoded
	decoded, decErr := base64.StdEncoding.DecodeString(result.Content[0].Text)
	require.NoError(t, decErr)
	require.Equal(t, exportData, decoded)
}

func TestExportTool_Export_MissingSlug(t *testing.T) {
	c := &Client{Export: &mockExportService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "export",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "project_slug")
}

func TestExportTool_Import(t *testing.T) {
	exportData := []byte(`{"project":"my-project"}`)
	encoded := base64.StdEncoding.EncodeToString(exportData)

	c := &Client{Export: &mockExportService{
		importProject: func(req *specv1.ImportProjectRequest) (*specv1.ImportProjectResponse, error) {
			require.Equal(t, exportData, req.GetData())
			require.True(t, req.GetForce())
			return &specv1.ImportProjectResponse{
				Result: &specv1.ImportResult{SpecsCreated: 3},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "import",
		"data":   encoded,
		"force":  true,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestExportTool_Import_InvalidBase64(t *testing.T) {
	c := &Client{Export: &mockExportService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "import",
		"data":   "not valid base64!!!",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid base64")
}

func TestExportTool_Verify(t *testing.T) {
	exportData := []byte(`{"project":"my-project"}`)
	encoded := base64.StdEncoding.EncodeToString(exportData)

	c := &Client{Export: &mockExportService{
		verifyExport: func(req *specv1.VerifyExportRequest) (*specv1.VerifyExportResponse, error) {
			require.Equal(t, exportData, req.GetData())
			return &specv1.VerifyExportResponse{Match: true}, nil
		},
	}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "verify",
		"data":   encoded,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestExportTool_UnknownAction(t *testing.T) {
	c := &Client{Export: &mockExportService{}}
	r := NewRegistry()
	RegisterLifecycleTools(r, c)
	tool, ok := r.LookupTool("export")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// driftScopeFromString enum helper
// ---------------------------------------------------------------------------

func TestDriftScopeFromString(t *testing.T) {
	require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_DEPS, driftScopeFromString("deps"))
	require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_INTERFACES, driftScopeFromString("interfaces"))
	require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_VERIFY, driftScopeFromString("verify"))
	require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED, driftScopeFromString("unknown"))
	require.Equal(t, specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED, driftScopeFromString(""))
}
