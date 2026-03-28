// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/export"
	"github.com/specgraph/specgraph/internal/storage"
)

// exportHandler implements the ConnectRPC ExportService.
type exportHandler struct {
	scoper     storage.Scoper
	signingKey string
	version    string
	logger     *slog.Logger
}

var _ specgraphv1connect.ExportServiceHandler = (*exportHandler)(nil)

// ExportProject handles the ExportProject RPC.
func (h *exportHandler) ExportProject(ctx context.Context, req *connect.Request[specv1.ExportProjectRequest]) (*connect.Response[specv1.ExportProjectResponse], error) {
	slug := req.Msg.GetProjectSlug()
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("project_slug is required"))
	}
	store, err := h.scoper.Scoped(ctx, slug)
	if err != nil {
		h.logger.ErrorContext(ctx, "ExportProject: scope error", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	engine := export.NewEngine(store, h.signingKey, h.version)
	data, err := engine.Export(ctx, slug)
	if err != nil {
		return nil, h.exportError(ctx, err)
	}

	return connect.NewResponse(&specv1.ExportProjectResponse{Data: data}), nil
}

// ImportProject handles the ImportProject RPC.
func (h *exportHandler) ImportProject(ctx context.Context, req *connect.Request[specv1.ImportProjectRequest]) (*connect.Response[specv1.ImportProjectResponse], error) {
	data := req.Msg.GetData()
	if len(data) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("data is required"))
	}

	// Parse just enough to extract project_slug for scoping.
	var envelope struct {
		ProjectSlug string `json:"project_slug"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid export data: cannot parse project_slug"))
	}
	if envelope.ProjectSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("export data missing project_slug"))
	}

	store, err := h.scoper.Scoped(ctx, envelope.ProjectSlug)
	if err != nil {
		h.logger.ErrorContext(ctx, "ImportProject: scope error", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	engine := export.NewEngine(store, h.signingKey, h.version)
	result, err := engine.Import(ctx, data, req.Msg.GetForce(), req.Msg.GetRequireSignature())
	if err != nil {
		return nil, h.exportError(ctx, err)
	}

	return connect.NewResponse(&specv1.ImportProjectResponse{
		Result: &specv1.ImportResult{
			SpecsCreated:           int32(result.Specs),
			DecisionsCreated:       int32(result.Decisions),
			SlicesCreated:          int32(result.Slices),
			EdgesCreated:           int32(result.Edges),
			FindingsCreated:        int32(result.Findings),
			ChangelogsCreated:      int32(result.ChangeLogs),
			ConversationsCreated:   int32(result.Conversations),
			SyncMappingsCreated:    int32(result.SyncMappings),
			ExecutionEventsCreated: int32(result.ExecEvents),
			Warnings:               result.Warnings,
		},
	}), nil
}

// VerifyExport handles the VerifyExport RPC.
func (h *exportHandler) VerifyExport(ctx context.Context, req *connect.Request[specv1.VerifyExportRequest]) (*connect.Response[specv1.VerifyExportResponse], error) {
	data := req.Msg.GetData()
	if len(data) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("data is required"))
	}

	// Determine project slug: explicit request field, or parse from data.
	slug := req.Msg.GetProjectSlug()
	if slug == "" {
		var envelope struct {
			ProjectSlug string `json:"project_slug"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid export data: cannot parse project_slug"))
		}
		slug = envelope.ProjectSlug
	}
	if slug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("project_slug required (in request or export data)"))
	}

	store, err := h.scoper.Scoped(ctx, slug)
	if err != nil {
		h.logger.ErrorContext(ctx, "VerifyExport: scope error", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	engine := export.NewEngine(store, h.signingKey, h.version)
	result, err := engine.Verify(ctx, data, slug)
	if err != nil {
		return nil, h.exportError(ctx, err)
	}

	diffs := make([]*specv1.EntityDiff, 0, len(result.Diffs))
	for _, d := range result.Diffs {
		diffs = append(diffs, &specv1.EntityDiff{
			EntityType: d.EntityType,
			Matched:    int32(d.Matched),
			Missing:    int32(d.Missing),
			Extra:      int32(d.Extra),
		})
	}

	return connect.NewResponse(&specv1.VerifyExportResponse{
		Match: result.OK,
		Diffs: diffs,
	}), nil
}

// exportError maps errors to sanitized connect error codes.
func (h *exportHandler) exportError(ctx context.Context, err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	h.logger.ErrorContext(ctx, "exportError: internal error", slog.Any("error", err))
	return connect.NewError(connect.CodeInternal, errors.New("internal error"))
}

// RegisterExportService registers the ExportService on the given mux.
func RegisterExportService(mux *http.ServeMux, scoper storage.Scoper, signingKey, version string, opts ...connect.HandlerOption) {
	handler := &exportHandler{scoper: scoper, signingKey: signingKey, version: version, logger: slog.Default()}
	path, h := specgraphv1connect.NewExportServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
