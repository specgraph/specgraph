// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/emitter"
	"github.com/specgraph/specgraph/internal/storage"
)

// ConstitutionHandler implements the ConnectRPC ConstitutionService.
type ConstitutionHandler struct {
	scoper storage.Scoper
}

var _ specgraphv1connect.ConstitutionServiceHandler = (*ConstitutionHandler)(nil)

// RegisterConstitutionService registers the ConstitutionService on the given mux.
func RegisterConstitutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &ConstitutionHandler{scoper: scoper}
	path, h := specgraphv1connect.NewConstitutionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// GetConstitution handles the GetConstitution RPC.
func (h *ConstitutionHandler) GetConstitution(ctx context.Context, _ *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	c, err := store.GetConstitution(ctx)
	if err != nil {
		return nil, constitutionError(err)
	}
	return connect.NewResponse(&specv1.GetConstitutionResponse{Constitution: constitutionToProto(c)}), nil
}

// UpdateConstitution handles the UpdateConstitution RPC.
func (h *ConstitutionHandler) UpdateConstitution(ctx context.Context, req *connect.Request[specv1.UpdateConstitutionRequest]) (*connect.Response[specv1.UpdateConstitutionResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if msg.Constitution == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("constitution is required"))
	}
	c, err := store.UpdateConstitution(ctx, constitutionFromProto(msg.Constitution))
	if err != nil {
		return nil, constitutionError(err)
	}
	return connect.NewResponse(&specv1.UpdateConstitutionResponse{Constitution: constitutionToProto(c)}), nil
}

// EmitToolFiles handles the EmitToolFiles RPC.
func (h *ConstitutionHandler) EmitToolFiles(ctx context.Context, req *connect.Request[specv1.EmitToolFilesRequest]) (*connect.Response[specv1.EmitToolFilesResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	if req.Msg.Format == specv1.OutputFormat_OUTPUT_FORMAT_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("format is required"))
	}

	c, err := store.GetConstitution(ctx)
	if err != nil {
		return nil, constitutionError(err)
	}

	formatStr, ok := outputFormatToString[req.Msg.Format]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unsupported format: %s", req.Msg.Format))
	}

	content, filename, err := emitter.Emit(c, formatStr)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	return connect.NewResponse(&specv1.EmitToolFilesResponse{
		Content:  content,
		Filename: filename,
	}), nil
}

// constitutionError maps storage errors to sanitized connect error codes.
func constitutionError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrConstitutionNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("constitution not found"))
	default:
		slog.Error("constitutionError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
