// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
)

// ---------------------------------------------------------------------------
// mockSpecService
// ---------------------------------------------------------------------------

type mockSpecService struct {
	specgraphv1connect.SpecServiceClient // panics on unimplemented methods
	getSpec     func(slug string) (*specv1.GetSpecResponse, error)
	listSpecs   func() (*specv1.ListSpecsResponse, error)
	createSpec  func(slug, intent string) (*specv1.CreateSpecResponse, error)
	updateSpec  func(req *specv1.UpdateSpecRequest) (*specv1.UpdateSpecResponse, error)
	listChanges func(slug string) (*specv1.ListChangesResponse, error)
	compareVer  func(req *specv1.CompareVersionsRequest) (*specv1.CompareVersionsResponse, error)
}

func (m *mockSpecService) GetSpec(_ context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.GetSpecResponse], error) {
	resp, err := m.getSpec(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) ListSpecs(_ context.Context, _ *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	resp, err := m.listSpecs()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) CreateSpec(_ context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.CreateSpecResponse], error) {
	resp, err := m.createSpec(req.Msg.GetSlug(), req.Msg.GetIntent())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) UpdateSpec(_ context.Context, req *connect.Request[specv1.UpdateSpecRequest]) (*connect.Response[specv1.UpdateSpecResponse], error) {
	resp, err := m.updateSpec(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) ListChanges(_ context.Context, req *connect.Request[specv1.ListChangesRequest]) (*connect.Response[specv1.ListChangesResponse], error) {
	resp, err := m.listChanges(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSpecService) CompareVersions(_ context.Context, req *connect.Request[specv1.CompareVersionsRequest]) (*connect.Response[specv1.CompareVersionsResponse], error) {
	resp, err := m.compareVer(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockDecisionService
// ---------------------------------------------------------------------------

type mockDecisionService struct {
	specgraphv1connect.DecisionServiceClient // panics on unimplemented methods
	getDecision    func(slug string) (*specv1.GetDecisionResponse, error)
	listDecisions  func() (*specv1.ListDecisionsResponse, error)
	createDecision func(req *specv1.CreateDecisionRequest) (*specv1.CreateDecisionResponse, error)
	updateDecision func(req *specv1.UpdateDecisionRequest) (*specv1.UpdateDecisionResponse, error)
}

func (m *mockDecisionService) GetDecision(_ context.Context, req *connect.Request[specv1.GetDecisionRequest]) (*connect.Response[specv1.GetDecisionResponse], error) {
	resp, err := m.getDecision(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockDecisionService) ListDecisions(_ context.Context, _ *connect.Request[specv1.ListDecisionsRequest]) (*connect.Response[specv1.ListDecisionsResponse], error) {
	resp, err := m.listDecisions()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockDecisionService) CreateDecision(_ context.Context, req *connect.Request[specv1.CreateDecisionRequest]) (*connect.Response[specv1.CreateDecisionResponse], error) {
	resp, err := m.createDecision(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockDecisionService) UpdateDecision(_ context.Context, req *connect.Request[specv1.UpdateDecisionRequest]) (*connect.Response[specv1.UpdateDecisionResponse], error) {
	resp, err := m.updateDecision(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockGraphService
// ---------------------------------------------------------------------------

type mockGraphService struct {
	specgraphv1connect.GraphServiceClient // panics on unimplemented methods
	addEdge         func(req *specv1.AddEdgeRequest) (*specv1.AddEdgeResponse, error)
	removeEdge      func(req *specv1.RemoveEdgeRequest) (*specv1.RemoveEdgeResponse, error)
	listEdges       func(slug string) (*specv1.ListEdgesResponse, error)
	getDeps         func(slug string) (*specv1.GetDependenciesResponse, error)
	getTransDeps    func(slug string) (*specv1.GetTransitiveDepsResponse, error)
	getImpact       func(slug string) (*specv1.GetImpactResponse, error)
	getReady        func() (*specv1.GetReadyResponse, error)
	getCriticalPath func(slug string) (*specv1.GetCriticalPathResponse, error)
	getFullGraph    func() (*specv1.GetFullGraphResponse, error)
}

func (m *mockGraphService) AddEdge(_ context.Context, req *connect.Request[specv1.AddEdgeRequest]) (*connect.Response[specv1.AddEdgeResponse], error) {
	resp, err := m.addEdge(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) RemoveEdge(_ context.Context, req *connect.Request[specv1.RemoveEdgeRequest]) (*connect.Response[specv1.RemoveEdgeResponse], error) {
	resp, err := m.removeEdge(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) ListEdges(_ context.Context, req *connect.Request[specv1.ListEdgesRequest]) (*connect.Response[specv1.ListEdgesResponse], error) {
	resp, err := m.listEdges(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetDependencies(_ context.Context, req *connect.Request[specv1.GetDependenciesRequest]) (*connect.Response[specv1.GetDependenciesResponse], error) {
	resp, err := m.getDeps(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetTransitiveDeps(_ context.Context, req *connect.Request[specv1.GetTransitiveDepsRequest]) (*connect.Response[specv1.GetTransitiveDepsResponse], error) {
	resp, err := m.getTransDeps(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetImpact(_ context.Context, req *connect.Request[specv1.GetImpactRequest]) (*connect.Response[specv1.GetImpactResponse], error) {
	resp, err := m.getImpact(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetReady(_ context.Context, _ *connect.Request[specv1.GetReadyRequest]) (*connect.Response[specv1.GetReadyResponse], error) {
	resp, err := m.getReady()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetCriticalPath(_ context.Context, req *connect.Request[specv1.GetCriticalPathRequest]) (*connect.Response[specv1.GetCriticalPathResponse], error) {
	resp, err := m.getCriticalPath(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockGraphService) GetFullGraph(_ context.Context, _ *connect.Request[specv1.GetFullGraphRequest]) (*connect.Response[specv1.GetFullGraphResponse], error) {
	resp, err := m.getFullGraph()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockHealthService
// ---------------------------------------------------------------------------

type mockHealthService struct {
	specgraphv1connect.ServerServiceClient // panics on unimplemented methods
	health func() (*specv1.HealthResponse, error)
}

func (m *mockHealthService) Health(_ context.Context, _ *connect.Request[specv1.HealthRequest]) (*connect.Response[specv1.HealthResponse], error) {
	resp, err := m.health()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockConstitutionService
// ---------------------------------------------------------------------------

type mockConstitutionService struct {
	specgraphv1connect.ConstitutionServiceClient // panics on unimplemented methods
	getConstitution func() (*specv1.GetConstitutionResponse, error)
}

func (m *mockConstitutionService) GetConstitution(_ context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	resp, err := m.getConstitution()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockAnalyticalPassService
// ---------------------------------------------------------------------------

type mockAnalyticalPassService struct {
	specgraphv1connect.AnalyticalPassServiceClient // panics on unimplemented methods
	listFindings func(slug string) (*specv1.ListFindingsResponse, error)
}

func (m *mockAnalyticalPassService) ListFindings(_ context.Context, req *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	resp, err := m.listFindings(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
