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
	getConstitution    func() (*specv1.GetConstitutionResponse, error)
	updateConstitution func(req *specv1.UpdateConstitutionRequest) (*specv1.UpdateConstitutionResponse, error)
}

func (m *mockConstitutionService) GetConstitution(_ context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	resp, err := m.getConstitution()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockConstitutionService) UpdateConstitution(_ context.Context, req *connect.Request[specv1.UpdateConstitutionRequest]) (*connect.Response[specv1.UpdateConstitutionResponse], error) {
	resp, err := m.updateConstitution(req.Msg)
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
	listFindings      func(slug string) (*specv1.ListFindingsResponse, error)
	runAnalyticalPass func(req *specv1.RunAnalyticalPassRequest) (*specv1.RunAnalyticalPassResponse, error)
	storeFindings     func(req *specv1.StoreFindingsRequest) (*specv1.StoreFindingsResponse, error)
}

func (m *mockAnalyticalPassService) ListFindings(_ context.Context, req *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	resp, err := m.listFindings(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAnalyticalPassService) RunAnalyticalPass(_ context.Context, req *connect.Request[specv1.RunAnalyticalPassRequest]) (*connect.Response[specv1.RunAnalyticalPassResponse], error) {
	if m.runAnalyticalPass == nil {
		panic("mockAnalyticalPassService.RunAnalyticalPass not configured")
	}
	resp, err := m.runAnalyticalPass(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAnalyticalPassService) StoreFindings(_ context.Context, req *connect.Request[specv1.StoreFindingsRequest]) (*connect.Response[specv1.StoreFindingsResponse], error) {
	if m.storeFindings == nil {
		panic("mockAnalyticalPassService.StoreFindings not configured")
	}
	resp, err := m.storeFindings(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockAuthoringService
// ---------------------------------------------------------------------------

type mockAuthoringService struct {
	specgraphv1connect.AuthoringServiceClient // panics on unimplemented methods
	spark              func(req *specv1.SparkRequest) (*specv1.SparkResponse, error)
	approve            func(slug string) (*specv1.ApproveResponse, error)
	amend              func(req *specv1.AmendRequest) (*specv1.AmendResponse, error)
	supersede          func(req *specv1.SupersedeRequest) (*specv1.SupersedeResponse, error)
	recordConversation func(req *specv1.RecordConversationRequest) (*specv1.RecordConversationResponse, error)
	listConversations  func(req *specv1.ListConversationsRequest) (*specv1.ListConversationsResponse, error)
}

func (m *mockAuthoringService) Spark(_ context.Context, req *connect.Request[specv1.SparkRequest]) (*connect.Response[specv1.SparkResponse], error) {
	if m.spark == nil {
		panic("mockAuthoringService.Spark not configured")
	}
	resp, err := m.spark(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) Approve(_ context.Context, req *connect.Request[specv1.ApproveRequest]) (*connect.Response[specv1.ApproveResponse], error) {
	if m.approve == nil {
		panic("mockAuthoringService.Approve not configured")
	}
	resp, err := m.approve(req.Msg.GetSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) Amend(_ context.Context, req *connect.Request[specv1.AmendRequest]) (*connect.Response[specv1.AmendResponse], error) {
	if m.amend == nil {
		panic("mockAuthoringService.Amend not configured")
	}
	resp, err := m.amend(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) Supersede(_ context.Context, req *connect.Request[specv1.SupersedeRequest]) (*connect.Response[specv1.SupersedeResponse], error) {
	if m.supersede == nil {
		panic("mockAuthoringService.Supersede not configured")
	}
	resp, err := m.supersede(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) RecordConversation(_ context.Context, req *connect.Request[specv1.RecordConversationRequest]) (*connect.Response[specv1.RecordConversationResponse], error) {
	if m.recordConversation == nil {
		panic("mockAuthoringService.RecordConversation not configured")
	}
	resp, err := m.recordConversation(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockAuthoringService) ListConversations(_ context.Context, req *connect.Request[specv1.ListConversationsRequest]) (*connect.Response[specv1.ListConversationsResponse], error) {
	if m.listConversations == nil {
		panic("mockAuthoringService.ListConversations not configured")
	}
	resp, err := m.listConversations(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockLifecycleService
// ---------------------------------------------------------------------------

type mockLifecycleService struct {
	specgraphv1connect.LifecycleServiceClient // panics on unimplemented methods
	checkDrift       func(req *specv1.DriftCheckRequest) (*specv1.DriftCheckResponse, error)
	acknowledgeDrift func(req *specv1.DriftAcknowledgeRequest) (*specv1.DriftAcknowledgeResponse, error)
	lint             func(req *specv1.LintRequest) (*specv1.LintResponse, error)
}

func (m *mockLifecycleService) CheckDrift(_ context.Context, req *connect.Request[specv1.DriftCheckRequest]) (*connect.Response[specv1.DriftCheckResponse], error) {
	if m.checkDrift == nil {
		panic("mockLifecycleService.CheckDrift not configured")
	}
	resp, err := m.checkDrift(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockLifecycleService) AcknowledgeDrift(_ context.Context, req *connect.Request[specv1.DriftAcknowledgeRequest]) (*connect.Response[specv1.DriftAcknowledgeResponse], error) {
	if m.acknowledgeDrift == nil {
		panic("mockLifecycleService.AcknowledgeDrift not configured")
	}
	resp, err := m.acknowledgeDrift(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockLifecycleService) Lint(_ context.Context, req *connect.Request[specv1.LintRequest]) (*connect.Response[specv1.LintResponse], error) {
	if m.lint == nil {
		panic("mockLifecycleService.Lint not configured")
	}
	resp, err := m.lint(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockSyncService
// ---------------------------------------------------------------------------

type mockSyncService struct {
	specgraphv1connect.SyncServiceClient // panics on unimplemented methods
	syncBeads     func() (*specv1.SyncResponse, error)
	getSyncStatus func() (*specv1.SyncStatusResponse, error)
}

func (m *mockSyncService) SyncBeads(_ context.Context, _ *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	if m.syncBeads == nil {
		panic("mockSyncService.SyncBeads not configured")
	}
	resp, err := m.syncBeads()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSyncService) GetSyncStatus(_ context.Context, _ *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	if m.getSyncStatus == nil {
		panic("mockSyncService.GetSyncStatus not configured")
	}
	resp, err := m.getSyncStatus()
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockExportService
// ---------------------------------------------------------------------------

type mockExportService struct {
	specgraphv1connect.ExportServiceClient // panics on unimplemented methods
	exportProject func(projectSlug string) (*specv1.ExportProjectResponse, error)
	importProject func(req *specv1.ImportProjectRequest) (*specv1.ImportProjectResponse, error)
	verifyExport  func(req *specv1.VerifyExportRequest) (*specv1.VerifyExportResponse, error)
}

func (m *mockExportService) ExportProject(_ context.Context, req *connect.Request[specv1.ExportProjectRequest]) (*connect.Response[specv1.ExportProjectResponse], error) {
	if m.exportProject == nil {
		panic("mockExportService.ExportProject not configured")
	}
	resp, err := m.exportProject(req.Msg.GetProjectSlug())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExportService) ImportProject(_ context.Context, req *connect.Request[specv1.ImportProjectRequest]) (*connect.Response[specv1.ImportProjectResponse], error) {
	if m.importProject == nil {
		panic("mockExportService.ImportProject not configured")
	}
	resp, err := m.importProject(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExportService) VerifyExport(_ context.Context, req *connect.Request[specv1.VerifyExportRequest]) (*connect.Response[specv1.VerifyExportResponse], error) {
	if m.verifyExport == nil {
		panic("mockExportService.VerifyExport not configured")
	}
	resp, err := m.verifyExport(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockClaimService
// ---------------------------------------------------------------------------

type mockClaimService struct {
	specgraphv1connect.ClaimServiceClient // panics on unimplemented methods
	claimSpec   func(req *specv1.ClaimSpecRequest) (*specv1.ClaimSpecResponse, error)
	unclaimSpec func(req *specv1.UnclaimSpecRequest) (*specv1.UnclaimSpecResponse, error)
	heartbeat   func(req *specv1.HeartbeatRequest) (*specv1.HeartbeatResponse, error)
}

func (m *mockClaimService) ClaimSpec(_ context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.ClaimSpecResponse], error) {
	if m.claimSpec == nil {
		panic("mockClaimService.ClaimSpec not configured")
	}
	resp, err := m.claimSpec(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockClaimService) UnclaimSpec(_ context.Context, req *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	if m.unclaimSpec == nil {
		panic("mockClaimService.UnclaimSpec not configured")
	}
	resp, err := m.unclaimSpec(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockClaimService) Heartbeat(_ context.Context, req *connect.Request[specv1.HeartbeatRequest]) (*connect.Response[specv1.HeartbeatResponse], error) {
	if m.heartbeat == nil {
		panic("mockClaimService.Heartbeat not configured")
	}
	resp, err := m.heartbeat(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockSliceService
// ---------------------------------------------------------------------------

type mockSliceService struct {
	specgraphv1connect.SliceServiceClient // panics on unimplemented methods
	listSlices    func(req *specv1.ListSlicesRequest) (*specv1.ListSlicesResponse, error)
	getSlice      func(req *specv1.GetSliceRequest) (*specv1.GetSliceResponse, error)
	claimSlice    func(req *specv1.ClaimSliceRequest) (*specv1.ClaimSliceResponse, error)
	completeSlice func(req *specv1.CompleteSliceRequest) (*specv1.CompleteSliceResponse, error)
}

func (m *mockSliceService) ListSlices(_ context.Context, req *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	if m.listSlices == nil {
		panic("mockSliceService.ListSlices not configured")
	}
	resp, err := m.listSlices(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) GetSlice(_ context.Context, req *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	if m.getSlice == nil {
		panic("mockSliceService.GetSlice not configured")
	}
	resp, err := m.getSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) ClaimSlice(_ context.Context, req *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	if m.claimSlice == nil {
		panic("mockSliceService.ClaimSlice not configured")
	}
	resp, err := m.claimSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockSliceService) CompleteSlice(_ context.Context, req *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	if m.completeSlice == nil {
		panic("mockSliceService.CompleteSlice not configured")
	}
	resp, err := m.completeSlice(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

// ---------------------------------------------------------------------------
// mockExecutionService
// ---------------------------------------------------------------------------

type mockExecutionService struct {
	specgraphv1connect.ExecutionServiceClient // panics on unimplemented methods
	generateBundle   func(req *specv1.GenerateBundleRequest) (*specv1.GenerateBundleResponse, error)
	getPrime         func(req *specv1.GetPrimeRequest) (*specv1.PrimeResponse, error)
	reportProgress   func(req *specv1.ReportProgressRequest) (*specv1.ReportProgressResponse, error)
	reportBlocker    func(req *specv1.ReportBlockerRequest) (*specv1.ReportBlockerResponse, error)
	reportCompletion func(req *specv1.ReportCompletionRequest) (*specv1.ReportCompletionResponse, error)
	getEvents        func(req *specv1.GetExecutionEventsRequest) (*specv1.GetExecutionEventsResponse, error)
}

func (m *mockExecutionService) GenerateBundle(_ context.Context, req *connect.Request[specv1.GenerateBundleRequest]) (*connect.Response[specv1.GenerateBundleResponse], error) {
	if m.generateBundle == nil {
		panic("mockExecutionService.GenerateBundle not configured")
	}
	resp, err := m.generateBundle(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) GetPrime(_ context.Context, req *connect.Request[specv1.GetPrimeRequest]) (*connect.Response[specv1.PrimeResponse], error) {
	if m.getPrime == nil {
		panic("mockExecutionService.GetPrime not configured")
	}
	resp, err := m.getPrime(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportProgress(_ context.Context, req *connect.Request[specv1.ReportProgressRequest]) (*connect.Response[specv1.ReportProgressResponse], error) {
	if m.reportProgress == nil {
		panic("mockExecutionService.ReportProgress not configured")
	}
	resp, err := m.reportProgress(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportBlocker(_ context.Context, req *connect.Request[specv1.ReportBlockerRequest]) (*connect.Response[specv1.ReportBlockerResponse], error) {
	if m.reportBlocker == nil {
		panic("mockExecutionService.ReportBlocker not configured")
	}
	resp, err := m.reportBlocker(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) ReportCompletion(_ context.Context, req *connect.Request[specv1.ReportCompletionRequest]) (*connect.Response[specv1.ReportCompletionResponse], error) {
	if m.reportCompletion == nil {
		panic("mockExecutionService.ReportCompletion not configured")
	}
	resp, err := m.reportCompletion(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func (m *mockExecutionService) GetExecutionEvents(_ context.Context, req *connect.Request[specv1.GetExecutionEventsRequest]) (*connect.Response[specv1.GetExecutionEventsResponse], error) {
	if m.getEvents == nil {
		panic("mockExecutionService.GetExecutionEvents not configured")
	}
	resp, err := m.getEvents(req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}
