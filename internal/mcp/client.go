// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

// Package mcp implements a Model Context Protocol server that exposes
// SpecGraph's ConnectRPC services as MCP tools, resources, and prompts.
package mcp

import (
	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// Client wraps all ConnectRPC service clients needed by MCP tool handlers.
type Client struct {
	Spec           specgraphv1connect.SpecServiceClient
	Decision       specgraphv1connect.DecisionServiceClient
	Graph          specgraphv1connect.GraphServiceClient
	Claim          specgraphv1connect.ClaimServiceClient
	Constitution   specgraphv1connect.ConstitutionServiceClient
	Authoring      specgraphv1connect.AuthoringServiceClient
	AnalyticalPass specgraphv1connect.AnalyticalPassServiceClient
	Execution      specgraphv1connect.ExecutionServiceClient
	Slice          specgraphv1connect.SliceServiceClient
	Export         specgraphv1connect.ExportServiceClient
	Lifecycle      specgraphv1connect.LifecycleServiceClient
	Sync           specgraphv1connect.SyncServiceClient
	Health         specgraphv1connect.ServerServiceClient
}

// NewClient creates a Client with all service clients pointing at baseURL.
func NewClient(httpClient connect.HTTPClient, baseURL string) *Client {
	return &Client{
		Spec:           specgraphv1connect.NewSpecServiceClient(httpClient, baseURL),
		Decision:       specgraphv1connect.NewDecisionServiceClient(httpClient, baseURL),
		Graph:          specgraphv1connect.NewGraphServiceClient(httpClient, baseURL),
		Claim:          specgraphv1connect.NewClaimServiceClient(httpClient, baseURL),
		Constitution:   specgraphv1connect.NewConstitutionServiceClient(httpClient, baseURL),
		Authoring:      specgraphv1connect.NewAuthoringServiceClient(httpClient, baseURL),
		AnalyticalPass: specgraphv1connect.NewAnalyticalPassServiceClient(httpClient, baseURL),
		Execution:      specgraphv1connect.NewExecutionServiceClient(httpClient, baseURL),
		Slice:          specgraphv1connect.NewSliceServiceClient(httpClient, baseURL),
		Export:         specgraphv1connect.NewExportServiceClient(httpClient, baseURL),
		Lifecycle:      specgraphv1connect.NewLifecycleServiceClient(httpClient, baseURL),
		Sync:           specgraphv1connect.NewSyncServiceClient(httpClient, baseURL),
		Health:         specgraphv1connect.NewServerServiceClient(httpClient, baseURL),
	}
}
