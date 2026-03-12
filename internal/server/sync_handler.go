// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/inject"
	"github.com/seanb4t/specgraph/internal/storage"
	syncpkg "github.com/seanb4t/specgraph/internal/sync"
)

// SyncHandler implements the ConnectRPC SyncService.
type SyncHandler struct {
	syncStore         storage.SyncBackend
	specStore         storage.SpecReader
	constitutionStore storage.ConstitutionBackend
	adapters          map[storage.SyncAdapterType]syncpkg.Adapter
}

var _ specgraphv1connect.SyncServiceHandler = (*SyncHandler)(nil)

// RegisterSyncService registers the SyncService handler on the mux and returns
// the handler so callers can register adapters via RegisterAdapter.
// constitutionStore can be nil if constitution injection is not needed.
func RegisterSyncService(mux *http.ServeMux, syncStore storage.SyncBackend, specStore storage.SpecReader, constitutionStore storage.ConstitutionBackend) *SyncHandler {
	handler := &SyncHandler{
		syncStore:         syncStore,
		specStore:         specStore,
		constitutionStore: constitutionStore,
		adapters:          map[storage.SyncAdapterType]syncpkg.Adapter{},
	}
	path, h := specgraphv1connect.NewSyncServiceHandler(handler)
	mux.Handle(path, h)
	return handler
}

// RegisterAdapter adds a sync adapter to the handler.
func (h *SyncHandler) RegisterAdapter(adapter syncpkg.Adapter) {
	h.adapters[adapter.Name()] = adapter
}

// SyncBeads implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) SyncBeads(ctx context.Context, req *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	adapter, ok := h.adapters[storage.SyncAdapterBeads]
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("beads adapter not configured"))
	}
	return h.syncWithAdapter(ctx, adapter, req.Msg.Config)
}

// SyncGitHub implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) SyncGitHub(ctx context.Context, req *connect.Request[specv1.SyncGitHubRequest]) (*connect.Response[specv1.SyncResponse], error) {
	adapter, ok := h.adapters[storage.SyncAdapterGitHub]
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("github adapter not configured"))
	}
	return h.syncWithAdapter(ctx, adapter, req.Msg.Config)
}

func (h *SyncHandler) syncWithAdapter(ctx context.Context, adapter syncpkg.Adapter, config *specv1.SyncConfig) (*connect.Response[specv1.SyncResponse], error) {
	if err := adapter.Available(); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}

	var stage, priority string
	var dryRun bool
	if config != nil {
		stage = config.FilterStage
		priority = config.FilterPriority
		dryRun = config.DryRun
	}

	specs, err := h.specStore.ListSpecs(ctx, stage, priority, 0)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list specs"))
	}

	resp := &specv1.SyncResponse{}
	for _, spec := range specs {
		result := &specv1.SyncResult{SpecSlug: spec.Slug}

		// Check if already synced
		existing, getErr := h.syncStore.GetSyncMapping(ctx, spec.Slug, adapter.Name())
		if getErr == nil && existing != nil {
			result.ExternalId = existing.ExternalID
			result.State = specv1.SyncState_SYNC_STATE_SYNCED
			result.Message = "already synced"
			resp.Skipped++
			resp.Results = append(resp.Results, result)
			continue
		}
		if getErr != nil && !errors.Is(getErr, storage.ErrSyncMappingNotFound) {
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = "failed to check sync state"
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		if dryRun {
			result.State = specv1.SyncState_SYNC_STATE_PENDING
			result.Message = "dry run - would sync"
			resp.Results = append(resp.Results, result)
			continue
		}

		externalID, pushErr := adapter.Push(ctx, spec)
		if pushErr != nil {
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = "failed to push to adapter"
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		_, createErr := h.syncStore.CreateSyncMapping(ctx, spec.Slug, adapter.Name(), externalID)
		if createErr != nil {
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = "failed to record sync mapping"
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		result.ExternalId = externalID
		result.State = specv1.SyncState_SYNC_STATE_SYNCED
		result.Message = "synced"
		resp.Synced++
		resp.Results = append(resp.Results, result)
	}

	return connect.NewResponse(resp), nil
}

// GetSyncStatus implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) GetSyncStatus(ctx context.Context, req *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	adapterFilter, err := syncAdapterFromProto(req.Msg.Adapter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	mappings, err := h.syncStore.ListSyncMappings(ctx, adapterFilter, req.Msg.SpecSlug)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list sync mappings"))
	}

	protoMappings := make([]*specv1.SyncMapping, 0, len(mappings))
	for _, m := range mappings {
		pm, convErr := syncMappingToProto(m)
		if convErr != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to convert sync mapping"))
		}
		protoMappings = append(protoMappings, pm)
	}
	return connect.NewResponse(&specv1.SyncStatusResponse{Mappings: protoMappings}), nil
}

// Inject implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) Inject(ctx context.Context, req *connect.Request[specv1.InjectRequest]) (*connect.Response[specv1.InjectResponse], error) {
	if req.Msg.SpecSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("spec_slug is required"))
	}
	if req.Msg.Tool == specv1.InjectTool_INJECT_TOOL_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tool is required"))
	}

	spec, err := h.specStore.GetSpec(ctx, req.Msg.SpecSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to retrieve spec"))
	}

	outputDir := req.Msg.OutputDir
	if outputDir == "" {
		var wdErr error
		outputDir, wdErr = os.Getwd()
		if wdErr != nil {
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to determine working directory"))
		}
	}

	tool := injectToolFromProto(req.Msg.Tool)

	// Constitution is optional for injection
	var constitution *storage.Constitution
	if h.constitutionStore != nil {
		// Try to load the project constitution; ignore errors (constitution is optional)
		var conErr error
		constitution, conErr = h.constitutionStore.GetConstitution(ctx)
		if conErr != nil {
			slog.WarnContext(ctx, "failed to load constitution for injection", "error", conErr)
		}
	}

	files, err := inject.Inject(spec, constitution, tool, outputDir)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to inject spec context"))
	}

	return connect.NewResponse(&specv1.InjectResponse{
		FilesWritten: files,
		Summary:      "Injected spec context for " + spec.Slug,
	}), nil
}
