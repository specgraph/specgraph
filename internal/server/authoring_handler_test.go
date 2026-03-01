// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/server"
	"github.com/stretchr/testify/require"
)

func TestAuthoringHandler_GetPrompts(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterAuthoringService(mux, nil, nil)

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := specgraphv1connect.NewAuthoringServiceClient(http.DefaultClient, srv.URL)

	resp, err := client.GetPrompts(context.Background(), connect.NewRequest(&specv1.GetPromptsRequest{
		Stage: "spark",
	}))
	require.NoError(t, err)
	require.NotEmpty(t, resp.Msg.Prompts)

	names := make(map[string]bool)
	for _, p := range resp.Msg.Prompts {
		names[p.Name] = true
	}
	require.True(t, names["seed"])
	require.True(t, names["signal"])
	require.True(t, names["kill_test"])
}
