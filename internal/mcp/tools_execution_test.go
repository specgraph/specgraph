// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// claimTool tests
// ---------------------------------------------------------------------------

func TestClaimTool_Claim(t *testing.T) {
	c := &Client{Claim: &mockClaimService{
		claimSpec: func(req *specv1.ClaimSpecRequest) (*specv1.ClaimSpecResponse, error) {
			require.Equal(t, "auth-spec", req.GetSpecSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			require.Nil(t, req.GetLeaseDuration())
			return &specv1.ClaimSpecResponse{
				Claim: &specv1.Claim{
					SpecSlug: "auth-spec",
					Agent:    "agent-1",
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "claim",
		"spec_slug": "auth-spec",
		"agent":     "agent-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "auth-spec")
}

func TestClaimTool_Claim_WithLeaseDuration(t *testing.T) {
	c := &Client{Claim: &mockClaimService{
		claimSpec: func(req *specv1.ClaimSpecRequest) (*specv1.ClaimSpecResponse, error) {
			require.NotNil(t, req.GetLeaseDuration())
			require.Equal(t, int64(1800), req.GetLeaseDuration().GetSeconds()) // 30m = 1800s
			return &specv1.ClaimSpecResponse{
				Claim: &specv1.Claim{SpecSlug: "auth-spec", Agent: "agent-1"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":         "claim",
		"spec_slug":      "auth-spec",
		"agent":          "agent-1",
		"lease_duration": "30m",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestClaimTool_Claim_InvalidLeaseDuration(t *testing.T) {
	c := &Client{Claim: &mockClaimService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":         "claim",
		"spec_slug":      "auth-spec",
		"agent":          "agent-1",
		"lease_duration": "not-a-duration",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid lease_duration")
}

func TestClaimTool_Claim_MissingSpecSlug(t *testing.T) {
	c := &Client{Claim: &mockClaimService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "claim",
		"agent":  "agent-1",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "spec_slug")
}

func TestClaimTool_Claim_MissingAgent(t *testing.T) {
	c := &Client{Claim: &mockClaimService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "claim",
		"spec_slug": "auth-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "agent")
}

func TestClaimTool_Unclaim(t *testing.T) {
	c := &Client{Claim: &mockClaimService{
		unclaimSpec: func(req *specv1.UnclaimSpecRequest) (*specv1.UnclaimSpecResponse, error) {
			require.Equal(t, "auth-spec", req.GetSpecSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			return &specv1.UnclaimSpecResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "unclaim",
		"spec_slug": "auth-spec",
		"agent":     "agent-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestClaimTool_Heartbeat(t *testing.T) {
	c := &Client{Claim: &mockClaimService{
		heartbeat: func(req *specv1.HeartbeatRequest) (*specv1.HeartbeatResponse, error) {
			require.Equal(t, "auth-spec", req.GetSpecSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			return &specv1.HeartbeatResponse{
				Claim: &specv1.Claim{SpecSlug: "auth-spec", Agent: "agent-1"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "heartbeat",
		"spec_slug": "auth-spec",
		"agent":     "agent-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestClaimTool_UnknownAction(t *testing.T) {
	c := &Client{Claim: &mockClaimService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("claim")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "frobnicate"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "frobnicate")
}

// ---------------------------------------------------------------------------
// sliceTool tests
// ---------------------------------------------------------------------------

func TestSliceTool_List(t *testing.T) {
	c := &Client{Slice: &mockSliceService{
		listSlices: func(req *specv1.ListSlicesRequest) (*specv1.ListSlicesResponse, error) {
			require.Equal(t, "auth-spec", req.GetParentSlug())
			return &specv1.ListSlicesResponse{
				Slices: []*specv1.Slice{
					{Slug: "auth-spec/slc-01", Intent: "implement auth"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":      "list",
		"parent_slug": "auth-spec",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slc-01")
}

func TestSliceTool_List_MissingParentSlug(t *testing.T) {
	c := &Client{Slice: &mockSliceService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "list"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "parent_slug")
}

func TestSliceTool_Get(t *testing.T) {
	c := &Client{Slice: &mockSliceService{
		getSlice: func(req *specv1.GetSliceRequest) (*specv1.GetSliceResponse, error) {
			require.Equal(t, "auth-spec/slc-01", req.GetSlug())
			return &specv1.GetSliceResponse{
				Slice: &specv1.Slice{
					Slug:   "auth-spec/slc-01",
					Intent: "implement auth",
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "get",
		"slug":   "auth-spec/slc-01",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "implement auth")
}

func TestSliceTool_Get_MissingSlug(t *testing.T) {
	c := &Client{Slice: &mockSliceService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "get"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestSliceTool_Claim(t *testing.T) {
	c := &Client{Slice: &mockSliceService{
		claimSlice: func(req *specv1.ClaimSliceRequest) (*specv1.ClaimSliceResponse, error) {
			require.Equal(t, "auth-spec/slc-01", req.GetSlug())
			require.Equal(t, "agent-1", req.GetAssignee())
			return &specv1.ClaimSliceResponse{
				Slice: &specv1.Slice{Slug: "auth-spec/slc-01", AssignedTo: "agent-1"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":   "claim",
		"slug":     "auth-spec/slc-01",
		"assignee": "agent-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSliceTool_Claim_MissingAssignee(t *testing.T) {
	c := &Client{Slice: &mockSliceService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "claim",
		"slug":   "auth-spec/slc-01",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "assignee")
}

func TestSliceTool_Complete(t *testing.T) {
	c := &Client{Slice: &mockSliceService{
		completeSlice: func(req *specv1.CompleteSliceRequest) (*specv1.CompleteSliceResponse, error) {
			require.Equal(t, "auth-spec/slc-01", req.GetSlug())
			return &specv1.CompleteSliceResponse{
				Slice: &specv1.Slice{Slug: "auth-spec/slc-01", Status: specv1.SliceStatus_SLICE_STATUS_DONE},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "complete",
		"slug":   "auth-spec/slc-01",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestSliceTool_UnknownAction(t *testing.T) {
	c := &Client{Slice: &mockSliceService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("slice")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "invalid"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid")
}

// ---------------------------------------------------------------------------
// bundleTool tests
// ---------------------------------------------------------------------------

func TestBundleTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		generateBundle: func(req *specv1.GenerateBundleRequest) (*specv1.GenerateBundleResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			require.Equal(t, "https://callback.example.com", req.GetEndpoint())
			return &specv1.GenerateBundleResponse{
				Bundle: &specv1.Bundle{
					Version: 1,
					Bootstrap: "## Bootstrap\nYou are working on auth-spec.",
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("bundle")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug":     "auth-spec",
		"endpoint": "https://callback.example.com",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "Bootstrap")
}

func TestBundleTool_MissingSlug(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("bundle")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

// ---------------------------------------------------------------------------
// primeTool tests
// ---------------------------------------------------------------------------

func TestPrimeTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		getPrime: func(req *specv1.GetPrimeRequest) (*specv1.PrimeResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			return &specv1.PrimeResponse{
				ConstitutionSummary: "All code must be tested.",
				ProjectContext:      "Auth service",
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("prime")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug": "auth-spec",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "constitutionSummary")
}

func TestPrimeTool_MissingSlug(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("prime")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

// ---------------------------------------------------------------------------
// reportTool tests
// ---------------------------------------------------------------------------

func TestReportTool_Progress(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		reportProgress: func(req *specv1.ReportProgressRequest) (*specv1.ReportProgressResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			require.Equal(t, "Completed step 1", req.GetMessage())
			return &specv1.ReportProgressResponse{Acknowledged: true}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":  "progress",
		"slug":    "auth-spec",
		"agent":   "agent-1",
		"message": "Completed step 1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "acknowledged")
}

func TestReportTool_Progress_MissingMessage(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "progress",
		"slug":   "auth-spec",
		"agent":  "agent-1",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "message")
}

func TestReportTool_Blocker(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		reportBlocker: func(req *specv1.ReportBlockerRequest) (*specv1.ReportBlockerResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			require.Equal(t, "DB not reachable", req.GetDescription())
			return &specv1.ReportBlockerResponse{Acknowledged: true}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":      "blocker",
		"slug":        "auth-spec",
		"agent":       "agent-1",
		"description": "DB not reachable",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestReportTool_Blocker_MissingDescription(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "blocker",
		"slug":   "auth-spec",
		"agent":  "agent-1",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "description")
}

func TestReportTool_Completion(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		reportCompletion: func(req *specv1.ReportCompletionRequest) (*specv1.ReportCompletionResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			require.Equal(t, "agent-1", req.GetAgent())
			return &specv1.ReportCompletionResponse{Acknowledged: true, NewStage: "done"}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "completion",
		"slug":   "auth-spec",
		"agent":  "agent-1",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "newStage")
}

func TestReportTool_UnknownAction(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("report")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{"action": "explode"})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "explode")
}

// ---------------------------------------------------------------------------
// executionEventsTool tests
// ---------------------------------------------------------------------------

func TestExecutionEventsTool(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		getEvents: func(req *specv1.GetExecutionEventsRequest) (*specv1.GetExecutionEventsResponse, error) {
			require.Equal(t, "auth-spec", req.GetSlug())
			require.Equal(t, uint32(10), req.GetLimit())
			return &specv1.GetExecutionEventsResponse{
				Events: []*specv1.ExecutionEvent{
					{
						Id:      "evt-1",
						Message: "Progress update",
						Type:    specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS,
					},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("execution_events")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug":  "auth-spec",
		"limit": float64(10),
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "evt-1")
}

func TestExecutionEventsTool_NoLimit(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{
		getEvents: func(req *specv1.GetExecutionEventsRequest) (*specv1.GetExecutionEventsResponse, error) {
			require.Equal(t, uint32(0), req.GetLimit()) // 0 = no limit
			return &specv1.GetExecutionEventsResponse{}, nil
		},
	}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("execution_events")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug": "auth-spec",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestExecutionEventsTool_MissingSlug(t *testing.T) {
	c := &Client{Execution: &mockExecutionService{}}
	r := NewRegistry()
	RegisterExecutionTools(r, c)
	tool, ok := r.LookupTool("execution_events")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}
