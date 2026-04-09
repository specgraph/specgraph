// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/storage"
)

const (
	defaultClaimLease  = 15 * time.Minute
	defaultHeartbeatBy = 5 * time.Minute
)

// ClaimHandler implements the ConnectRPC ClaimService.
type ClaimHandler struct {
	scoper storage.Scoper
}

var _ specgraphv1connect.ClaimServiceHandler = (*ClaimHandler)(nil)

// ClaimSpec handles the ClaimSpec RPC.
func (h *ClaimHandler) ClaimSpec(ctx context.Context, req *connect.Request[specv1.ClaimSpecRequest]) (*connect.Response[specv1.ClaimSpecResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg

	leaseDuration := defaultClaimLease
	if msg.LeaseDuration != nil {
		leaseDuration = msg.LeaseDuration.AsDuration()
	}

	claim, err := store.ClaimSpec(ctx, msg.SpecSlug, msg.Agent, leaseDuration)
	if err != nil {
		return nil, claimError(err)
	}
	return connect.NewResponse(&specv1.ClaimSpecResponse{Claim: claimToProto(claim)}), nil
}

// UnclaimSpec handles the UnclaimSpec RPC.
func (h *ClaimHandler) UnclaimSpec(ctx context.Context, req *connect.Request[specv1.UnclaimSpecRequest]) (*connect.Response[specv1.UnclaimSpecResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	err = store.UnclaimSpec(ctx, msg.SpecSlug, msg.Agent)
	if err != nil {
		return nil, claimError(err)
	}
	return connect.NewResponse(&specv1.UnclaimSpecResponse{}), nil
}

// Heartbeat handles the Heartbeat RPC.
func (h *ClaimHandler) Heartbeat(ctx context.Context, req *connect.Request[specv1.HeartbeatRequest]) (*connect.Response[specv1.HeartbeatResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg

	extendBy := defaultHeartbeatBy
	if msg.ExtendBy != nil {
		extendBy = msg.ExtendBy.AsDuration()
	}

	claim, err := store.Heartbeat(ctx, msg.SpecSlug, msg.Agent, extendBy)
	if err != nil {
		return nil, claimError(err)
	}
	return connect.NewResponse(&specv1.HeartbeatResponse{Claim: claimToProto(claim)}), nil
}

// claimError maps storage errors to sanitized connect error codes.
func claimError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	case errors.Is(err, storage.ErrSpecAlreadyClaimed):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("spec already claimed"))
	case errors.Is(err, storage.ErrNotClaimOwner):
		return connect.NewError(connect.CodePermissionDenied, errors.New("agent does not own the claim"))
	case errors.Is(err, storage.ErrSpecNotClaimed):
		return connect.NewError(connect.CodeNotFound, errors.New("spec is not claimed"))
	default:
		slog.Error("claimError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}

// RegisterClaimService registers the ClaimService on the given mux.
func RegisterClaimService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &ClaimHandler{scoper: scoper}
	path, h := specgraphv1connect.NewClaimServiceHandler(handler, opts...)
	mux.Handle(path, h)
}
