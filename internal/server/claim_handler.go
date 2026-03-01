// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// ClaimHandler implements the ConnectRPC ClaimService.
type ClaimHandler struct {
	store storage.ClaimBackend
}

var _ specgraphv1connect.ClaimServiceHandler = (*ClaimHandler)(nil)

// ClaimSpec handles the ClaimSpec RPC.
func (h *ClaimHandler) ClaimSpec(ctx context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.Claim], error) {
	msg := req.Msg
	claim, err := h.store.ClaimSpec(ctx, msg.SpecSlug, msg.Agent, msg.LeaseDuration.AsDuration())
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connect.NewResponse(claim), nil
}

// UnclaimSpec handles the UnclaimSpec RPC.
func (h *ClaimHandler) UnclaimSpec(ctx context.Context, req *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	msg := req.Msg
	err := h.store.UnclaimSpec(ctx, msg.SpecSlug, msg.Agent)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.UnclaimSpecResponse{}), nil
}

// Heartbeat handles the Heartbeat RPC.
func (h *ClaimHandler) Heartbeat(ctx context.Context, req *connect.Request[specv1.HeartbeatRequest]) (*connect.Response[specv1.Claim], error) {
	msg := req.Msg
	claim, err := h.store.Heartbeat(ctx, msg.SpecSlug, msg.Agent, msg.ExtendBy.AsDuration())
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
