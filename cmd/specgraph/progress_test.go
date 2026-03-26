// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- fake handlers ---

type fakeProgressHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeProgressHandler) GetExecutionEvents(_ context.Context, _ *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	return connect.NewResponse(&specv1.GetExecutionEventsResponse{
		Events: []*specv1.ExecutionEvent{
			{
				Id:        "evt-1",
				SpecSlug:  "my-spec",
				Agent:     "agent-a",
				Type:      specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_PROGRESS,
				Message:   "step completed",
				CreatedAt: timestamppb.Now(),
			},
			{
				Id:        "evt-2",
				SpecSlug:  "my-spec",
				Agent:     "agent-a",
				Type:      specv1.ExecutionEventType_EXECUTION_EVENT_TYPE_COMPLETION,
				CreatedAt: timestamppb.Now(),
			},
		},
	}), nil
}

type fakeProgressEmptyHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeProgressEmptyHandler) GetExecutionEvents(_ context.Context, _ *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	return connect.NewResponse(&specv1.GetExecutionEventsResponse{}), nil
}

type fakeProgressErrorHandler struct {
	specgraphv1connect.UnimplementedExecutionServiceHandler
}

func (fakeProgressErrorHandler) GetExecutionEvents(_ context.Context, _ *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	return nil, connect.NewError(connect.CodeNotFound, nil)
}

// --- tests ---

func TestRunProgress_HappyPath(t *testing.T) {
	startFakeExecutionServer(t, fakeProgressHandler{})

	old := progressLimit
	progressLimit = 20
	t.Cleanup(func() { progressLimit = old })

	err := runProgress(nil, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunProgress_EmptyEvents(t *testing.T) {
	startFakeExecutionServer(t, fakeProgressEmptyHandler{})

	old := progressLimit
	progressLimit = 20
	t.Cleanup(func() { progressLimit = old })

	err := runProgress(nil, []string{"my-spec"})
	require.NoError(t, err)
}

func TestRunProgress_RPCError(t *testing.T) {
	startFakeExecutionServer(t, fakeProgressErrorHandler{})

	old := progressLimit
	progressLimit = 20
	t.Cleanup(func() { progressLimit = old })

	err := runProgress(nil, []string{"my-spec"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "get execution events")
}

func TestRunProgress_ClientError(t *testing.T) {
	setMissingConfig(t)

	err := runProgress(nil, []string{"my-spec"})
	require.Error(t, err)
}

func TestProgressCmd_RequiresSlug(t *testing.T) {
	err := progressCmd.Args(progressCmd, []string{})
	require.Error(t, err)
}
