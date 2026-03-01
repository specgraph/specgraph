// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/emitter"
	"github.com/seanb4t/specgraph/internal/storage"
)

// ConstitutionHandler implements the ConnectRPC ConstitutionService.
type ConstitutionHandler struct {
	store storage.ConstitutionBackend
}

var _ specgraphv1connect.ConstitutionServiceHandler = (*ConstitutionHandler)(nil)

// RegisterConstitutionService registers the ConstitutionService on the given mux.
func RegisterConstitutionService(mux *http.ServeMux, store storage.ConstitutionBackend) {
	handler := &ConstitutionHandler{store: store}
	path, h := specgraphv1connect.NewConstitutionServiceHandler(handler)
	mux.Handle(path, h)
}

// GetConstitution handles the GetConstitution RPC.
func (h *ConstitutionHandler) GetConstitution(ctx context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	c, err := h.store.GetConstitution(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.GetConstitutionResponse{Constitution: c}), nil
}

// UpdateConstitution handles the UpdateConstitution RPC.
func (h *ConstitutionHandler) UpdateConstitution(ctx context.Context, req *connect.Request[specv1.UpdateConstitutionRequest]) (*connect.Response[specv1.UpdateConstitutionResponse], error) {
	msg := req.Msg
	if msg.Constitution == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("constitution is required"))
	}
	c, err := h.store.UpdateConstitution(ctx, msg.Constitution)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.UpdateConstitutionResponse{Constitution: c}), nil
}

// CheckViolation handles the CheckViolation RPC.
func (h *ConstitutionHandler) CheckViolation(ctx context.Context, req *connect.Request[specv1.CheckViolationRequest]) (*connect.Response[specv1.CheckViolationResponse], error) {
	msg := req.Msg
	if msg.SpecSlug == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("spec_slug is required"))
	}
	violations, err := h.store.CheckViolation(ctx, msg.SpecSlug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.CheckViolationResponse{Violations: violations}), nil
}

// EmitToolFiles handles the EmitToolFiles RPC.
func (h *ConstitutionHandler) EmitToolFiles(ctx context.Context, req *connect.Request[specv1.EmitToolFilesRequest]) (*connect.Response[specv1.EmitToolFilesResponse], error) {
	if req.Msg.Format == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("format is required"))
	}

	c, err := h.store.GetConstitution(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrConstitutionNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	content, filename, err := emitter.Emit(c, req.Msg.Format)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&specv1.EmitToolFilesResponse{
		Content:  content,
		Filename: filename,
	}), nil
}
