package server

import (
	"context"

	"connectrpc.com/connect"
	specv1 "github.com/seanb4t/specgraph/gen/specgraph/v1"
	"github.com/seanb4t/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/seanb4t/specgraph/internal/storage"
)

// SpecHandler implements the ConnectRPC SpecService using a storage backend.
type SpecHandler struct {
	backend storage.Backend
}

var _ specgraphv1connect.SpecServiceHandler = (*SpecHandler)(nil)

// NewSpecHandler creates a SpecHandler backed by the given storage.Backend.
func NewSpecHandler(backend storage.Backend) *SpecHandler {
	return &SpecHandler{backend: backend}
}

func (h *SpecHandler) CreateSpec(ctx context.Context, req *connect.Request[specv1.CreateSpecRequest]) (*connect.Response[specv1.Spec], error) {
	msg := req.Msg
	priority := msg.Priority
	if priority == "" {
		priority = "p2"
	}
	complexity := msg.Complexity
	if complexity == "" {
		complexity = "medium"
	}

	spec, err := h.backend.CreateSpec(ctx, msg.Slug, msg.Intent, priority, complexity)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *SpecHandler) GetSpec(ctx context.Context, req *connect.Request[specv1.GetSpecRequest]) (*connect.Response[specv1.Spec], error) {
	spec, err := h.backend.GetSpec(ctx, req.Msg.Slug)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	return connect.NewResponse(spec), nil
}

func (h *SpecHandler) ListSpecs(ctx context.Context, req *connect.Request[specv1.ListSpecsRequest]) (*connect.Response[specv1.ListSpecsResponse], error) {
	msg := req.Msg
	limit := int(msg.Limit)
	if limit == 0 {
		limit = 50
	}

	specs, err := h.backend.ListSpecs(ctx, msg.Stage, msg.Priority, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&specv1.ListSpecsResponse{Specs: specs}), nil
}
