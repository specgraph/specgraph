// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

const (
	defaultClaimLease  = 15 * time.Minute
	defaultHeartbeatBy = 5 * time.Minute
)

// ClaimHandler implements the ConnectRPC ClaimService.
type ClaimHandler struct {
	store storage.ClaimBackend
}

var _ specgraphv1connect.ClaimServiceHandler = (*ClaimHandler)(nil)

// ClaimSpec handles the ClaimSpec RPC.
func (h *ClaimHandler) ClaimSpec(ctx context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.Claim], error) {
	msg := req.Msg

	leaseDuration := defaultClaimLease
	if msg.LeaseDuration != nil {
		leaseDuration = msg.LeaseDuration.AsDuration()
	}

	claim, err := h.store.ClaimSpec(ctx, msg.SpecSlug, msg.Agent, leaseDuration)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, storage.ErrSpecAlreadyClaimed) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(claim), nil
}

// UnclaimSpec handles the UnclaimSpec RPC.
func (h *ClaimHandler) UnclaimSpec(ctx context.Context, req *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	msg := req.Msg
	err := h.store.UnclaimSpec(ctx, msg.SpecSlug, msg.Agent)
	if err != nil {
		if errors.Is(err, storage.ErrNotClaimOwner) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		if errors.Is(err, storage.ErrSpecNotClaimed) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.UnclaimSpecResponse{}), nil
}

// Heartbeat handles the Heartbeat RPC.
func (h *ClaimHandler) Heartbeat(ctx context.Context, req *connect.Request[specv1.HeartbeatRequest]) (*connect.Response[specv1.Claim], error) {
	msg := req.Msg

	extendBy := defaultHeartbeatBy
	if msg.ExtendBy != nil {
		extendBy = msg.ExtendBy.AsDuration()
	}

	claim, err := h.store.Heartbeat(ctx, msg.SpecSlug, msg.Agent, extendBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(claim), nil
}

// RegisterClaimService registers the ClaimService on the given mux.
func RegisterClaimService(mux *http.ServeMux, store storage.ClaimBackend) {
	handler := &ClaimHandler{store: store}
	path, h := specgraphv1connect.NewClaimServiceHandler(handler)
	mux.Handle(path, h)
}
