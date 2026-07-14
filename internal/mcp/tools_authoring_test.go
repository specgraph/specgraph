// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// authorTool tests
// ---------------------------------------------------------------------------

func TestAuthorTool_Spark(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		spark: func(req *specv1.SparkRequest) (*specv1.SparkResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.NotNil(t, req.GetOutput())
			require.Equal(t, "initial idea", req.GetOutput().GetSeed())
			return &specv1.SparkResponse{
				Output: req.GetOutput(),
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "spark",
		"slug":   "my-spec",
		"output": "seed: initial idea\nsignal: customer pain\n",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "initial idea")
}

func TestAuthorTool_Spark_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "spark",
		"output": "seed: idea\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Spark_MissingOutput(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "spark",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "output")
}

func TestAuthorTool_Spark_InvalidScopeSniff(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	// Invalid enum must be rejected, never silently written as UNSPECIFIED (T-06-01).
	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "spark",
		"slug":   "my-spec",
		"output": "seed: idea\nscope_sniff: gigantic\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid spark output")
	// Sanitized: no raw parser internals leaked (T-06-03).
	require.NotContains(t, result.Content[0].Text, "SCOPE_SNIFF")
}

func TestAuthorTool_Approve(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		approve: func(slug string) (*specv1.ApproveResponse, error) {
			require.Equal(t, "my-spec", slug)
			return &specv1.ApproveResponse{
				Slug:  slug,
				Stage: specv1.AuthoringStage_AUTHORING_STAGE_APPROVED,
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "approve",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "my-spec")
}

func TestAuthorTool_Approve_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "approve",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Amend(t *testing.T) {
	mock := &mockLifecycleService{
		transitionAmend: func(req *specv1.TransitionAmendRequest) (*specv1.TransitionAmendResponse, error) {
			return &specv1.TransitionAmendResponse{
				Spec: &specv1.Spec{Slug: req.GetSlug(), Version: 2},
			}, nil
		},
	}
	c := &Client{Lifecycle: mock}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":         "amend",
		"slug":           "my-spec",
		"reason":         "needs rework",
		"re_entry_stage": "shape",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	// The tool routes to Lifecycle.TransitionAmend with the passed fields.
	require.NotNil(t, mock.amendReq)
	require.Equal(t, "my-spec", mock.amendReq.GetSlug())
	require.Equal(t, "needs rework", mock.amendReq.GetReason())
	require.Equal(t, "shape", mock.amendReq.GetReEntryStage())
	// A next-step hint referencing the re_entry_stage is emitted (D-05).
	var combined string
	for _, ct := range result.Content {
		combined += ct.Text
	}
	require.Contains(t, combined, "my-spec")
	require.Contains(t, combined, "action=shape")
}

func TestAuthorTool_Amend_MissingReEntryStage(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "amend",
		"slug":   "my-spec",
		"reason": "rework",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "re_entry_stage")
}

func TestAuthorTool_Amend_MissingReason(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":         "amend",
		"slug":           "my-spec",
		"re_entry_stage": "shape",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "reason")
}

func TestAuthorTool_Amend_SurfacesHandlerError(t *testing.T) {
	// The handler rejects an invalid re-entry stage; the tool surfaces the
	// connect error (via connectErrResult) rather than re-validating itself.
	// Mock returns a sentinel error (not fmt.Errorf) per AGENTS.md.
	mock := &mockLifecycleService{
		transitionAmend: func(_ *specv1.TransitionAmendRequest) (*specv1.TransitionAmendResponse, error) {
			return nil, storage.ErrSpecNotAmendable
		},
	}
	c := &Client{Lifecycle: mock}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":         "amend",
		"slug":           "my-spec",
		"reason":         "rework",
		"re_entry_stage": "approved",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestAuthorTool_Supersede(t *testing.T) {
	mock := &mockLifecycleService{
		transitionSupersede: func(req *specv1.TransitionSupersedeRequest) (*specv1.TransitionSupersedeResponse, error) {
			return &specv1.TransitionSupersedeResponse{
				OldSpec: &specv1.Spec{Slug: req.GetSlug()},
				NewSpec: &specv1.Spec{Slug: req.GetNewSlug()},
			}, nil
		},
	}
	c := &Client{Lifecycle: mock}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":   "supersede",
		"slug":     "old-spec",
		"new_slug": "new-spec",
		"reason":   "replaced",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.NotNil(t, mock.supersedeReq)
	require.Equal(t, "old-spec", mock.supersedeReq.GetSlug())
	require.Equal(t, "new-spec", mock.supersedeReq.GetNewSlug())
	require.Equal(t, "replaced", mock.supersedeReq.GetReason())
	require.Contains(t, result.Content[0].Text, "old-spec")
}

func TestAuthorTool_Supersede_MissingNewSlug(t *testing.T) {
	c := &Client{Lifecycle: &mockLifecycleService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "supersede",
		"slug":   "old-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "new_slug")
}

func TestAuthorTool_Supersede_SurfacesHandlerError(t *testing.T) {
	// Mock returns a sentinel error (not fmt.Errorf) per AGENTS.md; the tool
	// surfaces it as an error result, asserted via res.IsError not a message.
	mock := &mockLifecycleService{
		transitionSupersede: func(_ *specv1.TransitionSupersedeRequest) (*specv1.TransitionSupersedeResponse, error) {
			return nil, storage.ErrSpecNotDone
		},
	}
	c := &Client{Lifecycle: mock}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":   "supersede",
		"slug":     "old-spec",
		"new_slug": "new-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
}

func TestAuthorTool_UnknownAction(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

func TestAuthorTool_Shape(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		shape: func(req *specv1.ShapeRequest) (*specv1.ShapeResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.NotNil(t, req.GetOutput())
			require.Equal(t, []string{"auth"}, req.GetOutput().GetScopeIn())
			return &specv1.ShapeResponse{
				Output: req.GetOutput(),
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "shape",
		"slug":   "my-spec",
		"output": "scope_in:\n  - auth\nchosen_approach: oauth2\n",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAuthorTool_Shape_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "shape",
		"output": "scope_in:\n  - auth\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Shape_MissingOutput(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "shape",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "output")
}

func TestAuthorTool_Shape_InvalidYAML(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	// scope_in must be a list; a scalar is a type mismatch the parser rejects.
	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "shape",
		"slug":   "my-spec",
		"output": "scope_in: not-a-list\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid shape output")
}

func TestAuthorTool_Specify(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		specify: func(req *specv1.SpecifyRequest) (*specv1.SpecifyResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.NotNil(t, req.GetOutput())
			require.Equal(t, []string{"state is never negative"}, req.GetOutput().GetInvariants())
			return &specv1.SpecifyResponse{
				Output: req.GetOutput(),
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "specify",
		"slug":   "my-spec",
		"output": "invariants:\n  - state is never negative\n",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAuthorTool_Specify_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "specify",
		"output": "invariants:\n  - x\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Specify_MissingOutput(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "specify",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "output")
}

func TestAuthorTool_Specify_InvalidYAML(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	// invariants must be a list; a scalar is a type mismatch the parser rejects.
	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "specify",
		"slug":   "my-spec",
		"output": "invariants: not-a-list\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid specify output")
}

func TestAuthorTool_Decompose(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		decompose: func(req *specv1.DecomposeRequest) (*specv1.DecomposeResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.NotNil(t, req.GetOutput())
			require.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE, req.GetOutput().GetStrategy())
			return &specv1.DecomposeResponse{
				Output: req.GetOutput(),
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "decompose",
		"slug":   "my-spec",
		"output": "strategy: vertical_slice\n",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
}

func TestAuthorTool_Decompose_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "decompose",
		"output": "strategy: vertical_slice\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Decompose_MissingOutput(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "decompose",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "output")
}

func TestAuthorTool_Decompose_InvalidStrategy(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	// Invalid enum must be rejected, never silently written as UNSPECIFIED (T-06-01).
	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "decompose",
		"slug":   "my-spec",
		"output": "strategy: sideways_slice\n",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid decompose output")
	// Sanitized: no raw parser internals leaked (T-06-03).
	require.NotContains(t, result.Content[0].Text, "DECOMPOSITION_STRATEGY")
}

func TestAuthorTool_Shape_MalformedExchanges(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	// Valid output YAML, but a syntactically invalid exchanges JSON string must
	// be rejected at the MCP boundary via parseOptionalExchanges (T-06-03).
	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "shape",
		"slug":      "my-spec",
		"output":    "scope_in:\n  - auth\nchosen_approach: oauth2\n",
		"exchanges": `not valid json {{{`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid exchanges JSON")
}

func TestAuthorTool_Amend_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "amend",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAuthorTool_Supersede_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("author")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "supersede",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

// ---------------------------------------------------------------------------
// conversationTool tests
// ---------------------------------------------------------------------------

func TestConversationTool_List(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		listConversations: func(req *specv1.ListConversationsRequest) (*specv1.ListConversationsResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, "spark", req.GetStage())
			return &specv1.ListConversationsResponse{
				ConversationLogs: []*specv1.ConversationLog{
					{Id: "log-1"},
				},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "list",
		"slug":   "my-spec",
		"stage":  "spark",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "log-1")
}

func TestConversationTool_List_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "list",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestConversationTool_Record(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{
		recordConversation: func(req *specv1.RecordConversationRequest) (*specv1.RecordConversationResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, "spark", req.GetStage())
			require.Len(t, req.GetExchanges(), 1)
			return &specv1.RecordConversationResponse{
				ConversationLog: &specv1.ConversationLog{Id: "log-1"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "record",
		"slug":      "my-spec",
		"stage":     "spark",
		"exchanges": `[{"role":"probe","content":"what is this?"}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "log-1")
}

func TestConversationTool_Record_InvalidJSON(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "record",
		"slug":      "my-spec",
		"stage":     "spark",
		"exchanges": `not valid`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid exchanges JSON")
}

func TestConversationTool_Record_MissingSlug(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "record",
		"stage":     "spark",
		"exchanges": `[{"role":"probe","content":"what?"}]`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestConversationTool_Record_MissingStage(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "record",
		"slug":      "my-spec",
		"exchanges": `[{"role":"probe","content":"what?"}]`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "stage")
}

func TestConversationTool_Record_MissingExchanges(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "record",
		"slug":   "my-spec",
		"stage":  "spark",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "exchanges")
}

func TestConversationTool_UnknownAction(t *testing.T) {
	c := &Client{Authoring: &mockAuthoringService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("conversation")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// analyticalPassTool tests
// ---------------------------------------------------------------------------

func TestAnalyticalPassTool_Run(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		runAnalyticalPass: func(req *specv1.RunAnalyticalPassRequest) (*specv1.RunAnalyticalPassResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, req.GetPassType())
			return &specv1.RunAnalyticalPassResponse{
				PassType:       req.GetPassType(),
				InitialMessage: "run the check",
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "run",
		"slug":      "my-spec",
		"pass_type": "constitution-check",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "run the check")
}

func TestAnalyticalPassTool_Run_MissingSlug(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "run",
		"pass_type": "constitution-check",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAnalyticalPassTool_Run_MissingPassType(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "run",
		"slug":   "my-spec",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "pass_type")
}

func TestAnalyticalPassTool_Run_InvalidPassType(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "run",
		"slug":      "my-spec",
		"pass_type": "not-a-real-pass",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid pass_type")
}

func TestAnalyticalPassTool_Store(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{
		storeFindings: func(req *specv1.StoreFindingsRequest) (*specv1.StoreFindingsResponse, error) {
			require.Equal(t, "my-spec", req.GetSlug())
			require.Equal(t, specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, req.GetPassType())
			require.Len(t, req.GetFindings(), 1)
			return &specv1.StoreFindingsResponse{
				Ids: []string{"finding-1"},
			}, nil
		},
	}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "store",
		"slug":      "my-spec",
		"pass_type": "constitution-check",
		"findings":  `[{"severity":"FINDING_SEVERITY_WARNING","summary":"missing constraint"}]`,
	})
	require.NoError(t, err)
	require.False(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "finding-1")
}

func TestAnalyticalPassTool_Store_InvalidJSON(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "store",
		"slug":      "my-spec",
		"pass_type": "constitution-check",
		"findings":  `not valid`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid findings JSON")
}

func TestAnalyticalPassTool_Store_MissingSlug(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "store",
		"pass_type": "constitution-check",
		"findings":  `[{"severity":"FINDING_SEVERITY_WARNING","summary":"x"}]`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug")
}

func TestAnalyticalPassTool_Store_MissingFindings(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "store",
		"slug":      "my-spec",
		"pass_type": "constitution-check",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "findings")
}

func TestAnalyticalPassTool_Store_MissingPassType(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":   "store",
		"slug":     "my-spec",
		"findings": `[{"severity":"FINDING_SEVERITY_WARNING","summary":"x"}]`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "pass_type")
}

func TestAnalyticalPassTool_Store_InvalidPassType(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action":    "store",
		"slug":      "my-spec",
		"pass_type": "bogus-pass",
		"findings":  `[{"severity":"FINDING_SEVERITY_WARNING","summary":"x"}]`,
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "invalid pass_type")
}

func TestAnalyticalPassTool_UnknownAction(t *testing.T) {
	c := &Client{AnalyticalPass: &mockAnalyticalPassService{}}
	r := NewRegistry()
	RegisterAuthoringTools(r, c)
	tool, ok := r.LookupTool("analytical_pass")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"action": "delete",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "delete")
}

// ---------------------------------------------------------------------------
// author_start_stage tool tests
// ---------------------------------------------------------------------------

func TestAuthoringStartStageTool(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock("test-slug"), defaultGraphMock())
	reg := NewRegistry()
	RegisterAuthoringTools(reg, c)

	tool, ok := reg.LookupTool("author_start_stage")
	require.True(t, ok, "author_start_stage not registered")

	result, err := tool.Handler(context.Background(), map[string]any{
		"stage": "shape",
		"slug":  "test-slug",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected no error result, got %+v", result)
	require.NotEmpty(t, result.Content)
	require.Contains(t, result.Content[0].Text, "# Shape")
	// B.6: The dynamic state block must reflect the slug supplied.
	// defaultSpecMock("test-slug") returns Slug="test-slug", so the composer renders
	// "**Spec test-slug**" in the current state section.
	require.Contains(t, result.Content[0].Text, "**Spec test-slug**")
}

func TestAuthoringStartStageTool_MissingStage(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())
	reg := NewRegistry()
	RegisterAuthoringTools(reg, c)

	tool, ok := reg.LookupTool("author_start_stage")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"slug": "test-slug",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "stage is required")
}

func TestAuthoringStartStageTool_MissingSlugForNonSpark(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())
	reg := NewRegistry()
	RegisterAuthoringTools(reg, c)

	tool, ok := reg.LookupTool("author_start_stage")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"stage": "shape",
	})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Contains(t, result.Content[0].Text, "slug is required")
}

func TestAuthoringStartStageTool_SparkNoSlug(t *testing.T) {
	c := newComposerClient(defaultConstitutionMock(), defaultSpecMock(""), defaultGraphMock())
	reg := NewRegistry()
	RegisterAuthoringTools(reg, c)

	tool, ok := reg.LookupTool("author_start_stage")
	require.True(t, ok)

	result, err := tool.Handler(context.Background(), map[string]any{
		"stage": "spark",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "spark without slug should succeed, got %+v", result)
	require.NotEmpty(t, result.Content)
	require.Contains(t, result.Content[0].Text, "# Persona")
}

// ---------------------------------------------------------------------------
// enum helper tests
// ---------------------------------------------------------------------------

func TestPostureFromString(t *testing.T) {
	require.Equal(t, specv1.Posture_POSTURE_DRIVE, postureFromString("drive"))
	require.Equal(t, specv1.Posture_POSTURE_PARTNER, postureFromString("partner"))
	require.Equal(t, specv1.Posture_POSTURE_SUPPORT, postureFromString("support"))
	require.Equal(t, specv1.Posture_POSTURE_UNSPECIFIED, postureFromString("unknown"))
	require.Equal(t, specv1.Posture_POSTURE_UNSPECIFIED, postureFromString(""))
}
