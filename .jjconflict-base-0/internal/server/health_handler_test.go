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

func TestHealthHandler(t *testing.T) {
	mux := http.NewServeMux()
	server.RegisterHealthService(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := specgraphv1connect.NewServerServiceClient(http.DefaultClient, srv.URL)
	resp, err := client.Health(context.Background(), connect.NewRequest(&specv1.HealthRequest{}))
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Msg.Status)
	require.Equal(t, "dev", resp.Msg.Version)
	require.NotNil(t, resp.Msg.ServerTime)
}
