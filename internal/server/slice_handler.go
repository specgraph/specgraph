// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

// SliceHandler implements the ConnectRPC SliceService.
type SliceHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.SliceServiceHandler = (*SliceHandler)(nil)

// ListSlices handles the ListSlices RPC.
func (h *SliceHandler) ListSlices(ctx context.Context, req *connect.Request[specv1.ListSlicesRequest]) (*connect.Response[specv1.ListSlicesResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetParentSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	slices, err := store.ListSlices(ctx, req.Msg.ParentSlug)
	if err != nil {
		h.logger.ErrorContext(ctx, "list slices failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := slicesToProto(slices)
	if err != nil {
		h.logger.ErrorContext(ctx, "slice conversion failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.ListSlicesResponse{Slices: pb}), nil
}

// GetSlice handles the GetSlice RPC.
func (h *SliceHandler) GetSlice(ctx context.Context, req *connect.Request[specv1.GetSliceRequest]) (*connect.Response[specv1.GetSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.GetSlice(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		h.logger.ErrorContext(ctx, "get slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		h.logger.ErrorContext(ctx, "slice conversion failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.GetSliceResponse{Slice: pb}), nil
}

// ClaimSlice handles the ClaimSlice RPC.
func (h *SliceHandler) ClaimSlice(ctx context.Context, req *connect.Request[specv1.ClaimSliceRequest]) (*connect.Response[specv1.ClaimSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	msg := req.Msg
	if err := validateSlug(msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	assignee := strings.TrimSpace(msg.GetAssignee())
	if err := validateRequiredField("assignee", assignee); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.ClaimSlice(ctx, msg.Slug, assignee)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		if errors.Is(err, storage.ErrSliceWrongStatus) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("slice is not in open status"))
		}
		h.logger.ErrorContext(ctx, "claim slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		h.logger.ErrorContext(ctx, "slice conversion failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.ClaimSliceResponse{Slice: pb}), nil
}

// CompleteSlice handles the CompleteSlice RPC.
func (h *SliceHandler) CompleteSlice(ctx context.Context, req *connect.Request[specv1.CompleteSliceRequest]) (*connect.Response[specv1.CompleteSliceResponse], error) {
	store, scopeErr := scopeStore(ctx, h.scoper)
	if scopeErr != nil {
		return nil, scopeErr
	}
	if err := validateSlug(req.Msg.GetSlug()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	s, err := store.CompleteSlice(ctx, req.Msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSliceNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("slice not found"))
		}
		if errors.Is(err, storage.ErrSliceWrongStatus) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("slice is not in claimed status"))
		}
		h.logger.ErrorContext(ctx, "complete slice failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	pb, err := sliceToProto(s)
	if err != nil {
		h.logger.ErrorContext(ctx, "slice conversion failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
	return connect.NewResponse(&specv1.CompleteSliceResponse{Slice: pb}), nil
}

// RegisterSliceService registers the SliceService handler on the mux.
func RegisterSliceService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &SliceHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewSliceServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
