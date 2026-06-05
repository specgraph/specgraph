// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
	syncpkg "github.com/specgraph/specgraph/internal/sync"
)

// SyncHandler implements the ConnectRPC SyncService.
type SyncHandler struct {
	mu       sync.RWMutex
	scoper   storage.Scoper
	adapters map[storage.SyncAdapterType]syncpkg.Adapter
}

var _ specgraphv1connect.SyncServiceHandler = (*SyncHandler)(nil)

// RegisterSyncService registers the SyncService handler on the mux and returns
// the handler so callers can register adapters via RegisterAdapter.
func RegisterSyncService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) *SyncHandler {
	handler := &SyncHandler{
		scoper:   scoper,
		adapters: map[storage.SyncAdapterType]syncpkg.Adapter{},
	}
	path, h := specgraphv1connect.NewSyncServiceHandler(handler, opts...)
	mux.Handle(path, h)
	return handler
}

// RegisterAdapter adds a sync adapter to the handler.
func (h *SyncHandler) RegisterAdapter(adapter syncpkg.Adapter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.adapters[adapter.Name()] = adapter
}

// SyncBeads implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) SyncBeads(ctx context.Context, req *connect.Request[specv1.SyncBeadsRequest]) (*connect.Response[specv1.SyncResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	h.mu.RLock()
	adapter, ok := h.adapters[storage.SyncAdapterBeads]
	h.mu.RUnlock()
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("beads adapter not configured"))
	}
	return h.syncWithAdapter(ctx, store, adapter, req.Msg.Config)
}

// SyncGitHub implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) SyncGitHub(ctx context.Context, req *connect.Request[specv1.SyncGitHubRequest]) (*connect.Response[specv1.SyncResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	h.mu.RLock()
	adapter, ok := h.adapters[storage.SyncAdapterGitHub]
	h.mu.RUnlock()
	if !ok {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("github adapter not configured"))
	}
	return h.syncWithAdapter(ctx, store, adapter, req.Msg.Config)
}

func (h *SyncHandler) syncWithAdapter(ctx context.Context, store storage.ScopedBackend, adapter syncpkg.Adapter, config *specv1.SyncConfig) (*connect.Response[specv1.SyncResponse], error) {
	if err := adapter.Available(ctx); err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "adapter unavailable",
			slog.Any("adapter", adapter.Name()), slog.Any("error", err))
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("sync adapter not available"))
	}

	var stage, priority string
	var dryRun bool
	if config != nil {
		stage = config.FilterStage
		priority = config.FilterPriority
		dryRun = config.DryRun
	}

	specs, err := store.ListSpecs(ctx, stage, priority, 0)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "failed to list specs", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list specs"))
	}

	resp := &specv1.SyncResponse{}
	for _, spec := range specs {
		result := &specv1.SyncResult{SpecSlug: spec.Slug}

		// Check if already synced
		existing, getErr := store.GetSyncMapping(ctx, spec.Slug, adapter.Name())
		if getErr == nil && existing != nil {
			result.ExternalId = existing.ExternalID
			result.State = specv1.SyncState_SYNC_STATE_SYNCED
			result.Message = "already synced"
			resp.Skipped++
			resp.Results = append(resp.Results, result)
			continue
		}
		if getErr != nil && !errors.Is(getErr, storage.ErrSyncMappingNotFound) {
			slog.LogAttrs(ctx, slog.LevelWarn, "failed to check sync state",
				slog.String("spec", spec.Slug), slog.Any("adapter", adapter.Name()), slog.Any("error", getErr))
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = "failed to check sync state"
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		if dryRun {
			result.State = specv1.SyncState_SYNC_STATE_PENDING
			result.Message = "dry run - would sync"
			resp.DryRunCount++
			resp.Results = append(resp.Results, result)
			continue
		}

		externalID, created, pushErr := adapter.FindOrCreate(ctx, spec)
		if pushErr != nil {
			slog.LogAttrs(ctx, slog.LevelWarn, "failed to push spec to adapter",
				slog.String("spec", spec.Slug), slog.Any("adapter", adapter.Name()), slog.Any("error", pushErr))
			result.State = specv1.SyncState_SYNC_STATE_ERROR
			result.Message = "failed to push to adapter"
			resp.Errors++
			resp.Results = append(resp.Results, result)
			continue
		}

		_, createErr := store.CreateSyncMapping(ctx, spec.Slug, adapter.Name(), externalID)
		if createErr != nil {
			if errors.Is(createErr, storage.ErrSyncMappingExists) {
				result.ExternalId = externalID
				result.State = specv1.SyncState_SYNC_STATE_SYNCED
				result.Message = "already synced (concurrent sync detected)"
				resp.Skipped++
				resp.Results = append(resp.Results, result)
				continue
			}
			// Retry once — transient store failures should not orphan external items.
			if ctx.Err() != nil {
				slog.LogAttrs(ctx, slog.LevelError, "sync mapping record failed after push (context cancelled before retry)",
					slog.String("spec", spec.Slug), slog.Any("adapter", adapter.Name()), slog.String("external_id", externalID),
					slog.Any("error", createErr))
				result.ExternalId = externalID
				result.State = specv1.SyncState_SYNC_STATE_ERROR
				result.Message = "pushed to adapter but failed to record mapping - external_id preserved for reconciliation"
				resp.Errors++
				resp.Results = append(resp.Results, result)
				continue
			}
			_, retryErr := store.CreateSyncMapping(ctx, spec.Slug, adapter.Name(), externalID)
			if retryErr != nil {
				if errors.Is(retryErr, storage.ErrSyncMappingExists) {
					// A concurrent sync won the race — treat as skipped.
					result.ExternalId = externalID
					result.State = specv1.SyncState_SYNC_STATE_SYNCED
					result.Message = "already synced (concurrent sync detected)"
					resp.Skipped++
					resp.Results = append(resp.Results, result)
					continue
				}
				slog.LogAttrs(ctx, slog.LevelError, "sync mapping record failed after push (orphaned external item)",
					slog.String("spec", spec.Slug), slog.Any("adapter", adapter.Name()), slog.String("external_id", externalID),
					slog.Any("initial_error", createErr), slog.Any("error", retryErr))
				result.ExternalId = externalID
				result.State = specv1.SyncState_SYNC_STATE_ERROR
				result.Message = "pushed to adapter but failed to record mapping - external_id preserved for reconciliation"
				resp.Errors++
				resp.Results = append(resp.Results, result)
				continue
			}
		}

		result.ExternalId = externalID
		result.State = specv1.SyncState_SYNC_STATE_SYNCED
		if created {
			result.Message = "synced"
		} else {
			result.Message = "synced (recovered existing external item)"
		}
		resp.Synced++
		resp.Results = append(resp.Results, result)
	}

	return connect.NewResponse(resp), nil
}

// GetSyncStatus implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) GetSyncStatus(ctx context.Context, req *connect.Request[specv1.SyncStatusRequest]) (*connect.Response[specv1.SyncStatusResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	adapterFilter, err := syncAdapterFromProto(req.Msg.Adapter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported adapter"))
	}
	mappings, err := store.ListSyncMappings(ctx, adapterFilter, req.Msg.SpecSlug)
	if err != nil {
		slog.LogAttrs(ctx, slog.LevelError, "failed to list sync mappings", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list sync mappings"))
	}

	protoMappings := make([]*specv1.SyncMapping, 0, len(mappings))
	for _, m := range mappings {
		pm, convErr := syncMappingToProto(m)
		if convErr != nil {
			slog.LogAttrs(ctx, slog.LevelError, "failed to convert sync mapping", slog.Any("error", convErr))
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to convert sync mapping"))
		}
		protoMappings = append(protoMappings, pm)
	}
	return connect.NewResponse(&specv1.SyncStatusResponse{Mappings: protoMappings}), nil
}
