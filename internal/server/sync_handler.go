// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/inject"
	"github.com/specgraph/specgraph/internal/storage"
	syncpkg "github.com/specgraph/specgraph/internal/sync"
)

// SyncHandler implements the ConnectRPC SyncService.
type SyncHandler struct {
	mu                sync.RWMutex
	scoper            storage.Scoper
	adapters          map[storage.SyncAdapterType]syncpkg.Adapter
	allowedOutputRoot string // if set, Inject validates outputDir is within this root
}

var _ specgraphv1connect.SyncServiceHandler = (*SyncHandler)(nil)

// RegisterSyncService registers the SyncService handler on the mux and returns
// the handler so callers can register adapters via RegisterAdapter.
// allowedOutputRoot restricts Inject output_dir to paths within this root;
// pass "" for unrestricted mode (not recommended for production).
func RegisterSyncService(mux *http.ServeMux, scoper storage.Scoper, allowedOutputRoot string, opts ...connect.HandlerOption) *SyncHandler {
	handler := &SyncHandler{
		scoper:            scoper,
		adapters:          map[storage.SyncAdapterType]syncpkg.Adapter{},
		allowedOutputRoot: allowedOutputRoot,
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

// SetAllowedOutputRoot restricts the Inject handler's output_dir to paths
// within the given root directory. If not called, output_dir is unrestricted.
func (h *SyncHandler) SetAllowedOutputRoot(root string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOutputRoot = root
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
		slog.ErrorContext(ctx, "adapter unavailable", "adapter", adapter.Name(), "error", err)
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
		slog.ErrorContext(ctx, "failed to list specs", "error", err)
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
			slog.WarnContext(ctx, "failed to check sync state", "spec", spec.Slug, "adapter", adapter.Name(), "error", getErr)
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
			slog.WarnContext(ctx, "failed to push spec to adapter", "spec", spec.Slug, "adapter", adapter.Name(), "error", pushErr)
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
				slog.ErrorContext(ctx, "sync mapping record failed after push (context cancelled before retry)",
					"spec", spec.Slug, "adapter", adapter.Name(), "external_id", externalID,
					"error", createErr)
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
				slog.ErrorContext(ctx, "sync mapping record failed after push (orphaned external item)",
					"spec", spec.Slug, "adapter", adapter.Name(), "external_id", externalID,
					"initial_error", createErr, "error", retryErr)
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
		slog.ErrorContext(ctx, "failed to list sync mappings", "error", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to list sync mappings"))
	}

	protoMappings := make([]*specv1.SyncMapping, 0, len(mappings))
	for _, m := range mappings {
		pm, convErr := syncMappingToProto(m)
		if convErr != nil {
			slog.ErrorContext(ctx, "failed to convert sync mapping", "error", convErr)
			return nil, connect.NewError(connect.CodeInternal, errors.New("failed to convert sync mapping"))
		}
		protoMappings = append(protoMappings, pm)
	}
	return connect.NewResponse(&specv1.SyncStatusResponse{Mappings: protoMappings}), nil
}

// Inject implements specgraphv1connect.SyncServiceHandler.
func (h *SyncHandler) Inject(ctx context.Context, req *connect.Request[specv1.InjectRequest]) (*connect.Response[specv1.InjectResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	if req.Msg.SpecSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("spec_slug is required"))
	}
	if req.Msg.Tool == specv1.InjectTool_INJECT_TOOL_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("tool is required"))
	}

	spec, err := store.GetSpec(ctx, req.Msg.SpecSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
		}
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to retrieve spec"))
	}

	outputDir := req.Msg.OutputDir
	if outputDir == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output_dir is required"))
	}

	h.mu.RLock()
	allowedRoot := h.allowedOutputRoot
	h.mu.RUnlock()

	absDir, absErr := filepath.Abs(outputDir)
	if absErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid output directory"))
	}
	// Resolve symlinks to prevent escape via symlinked directories.
	realDir, evalErr := filepath.EvalSymlinks(absDir)
	if evalErr != nil {
		if !errors.Is(evalErr, fs.ErrNotExist) {
			slog.WarnContext(ctx, "EvalSymlinks failed for output_dir, falling back to unresolved path",
				"path", absDir, "error", evalErr)
		}
		// Directory may not exist yet — fall back to the unresolved absDir.
		realDir = absDir
	}

	if allowedRoot != "" {
		absRoot := filepath.Clean(allowedRoot)
		// Resolve symlinks on the root too (e.g., /var -> /private/var on macOS).
		if realRoot, rlErr := filepath.EvalSymlinks(absRoot); rlErr == nil {
			absRoot = realRoot
		}
		if !strings.HasPrefix(realDir, absRoot+string(filepath.Separator)) && realDir != absRoot {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("output_dir must be within the allowed root directory"))
		}
	}

	tool, toolErr := injectToolFromProto(req.Msg.Tool)
	if toolErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unsupported inject tool"))
	}

	// Constitution is optional for injection
	var constitution *storage.Constitution
	var warnings []string
	constitution, conErr := store.GetConstitution(ctx)
	if conErr != nil {
		if errors.Is(conErr, storage.ErrConstitutionNotFound) {
			slog.DebugContext(ctx, "no constitution seeded yet")
			constitution = nil
		} else {
			slog.WarnContext(ctx, "failed to load constitution for injection", "error", conErr)
			warnings = append(warnings, "constitution load failed: storage error")
			constitution = nil
		}
	}

	files, err := inject.Inject(spec, constitution, tool, realDir)
	if err != nil {
		slog.ErrorContext(ctx, "failed to inject spec context", "error", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("failed to inject spec context"))
	}

	summary := "Injected spec context for " + spec.Slug
	if len(warnings) > 0 {
		summary += " (warning: " + strings.Join(warnings, "; ") + ")"
	}

	return connect.NewResponse(&specv1.InjectResponse{
		FilesWritten: files,
		Summary:      summary,
		Warnings:     warnings,
	}), nil
}
