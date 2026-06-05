// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/constitution/fetch"
	"github.com/specgraph/specgraph/internal/constitution/hash"
	"github.com/specgraph/specgraph/internal/constitution/load"
	"github.com/specgraph/specgraph/internal/constitution/merge"
	"github.com/specgraph/specgraph/internal/emitter"
	"github.com/specgraph/specgraph/internal/storage"
)

// Fetcher abstracts internal/constitution/fetch.Fetch for testability.
// The handler's default uses the package function; tests inject fakes.
type Fetcher interface {
	Fetch(ctx context.Context, url string) (*fetch.Fetched, error)
}

type defaultFetcher struct{}

func (defaultFetcher) Fetch(ctx context.Context, url string) (*fetch.Fetched, error) {
	f, err := fetch.Fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	return f, nil
}

// ConstitutionHandler implements the ConnectRPC ConstitutionService.
type ConstitutionHandler struct {
	scoper  storage.Scoper
	fetcher Fetcher
}

var _ specgraphv1connect.ConstitutionServiceHandler = (*ConstitutionHandler)(nil)

// RegisterConstitutionService registers the ConstitutionService on the given mux.
func RegisterConstitutionService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	handler := &ConstitutionHandler{scoper: scoper, fetcher: defaultFetcher{}}
	path, h := specgraphv1connect.NewConstitutionServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// NewConstitutionHandlerForTest creates a ConstitutionHandler with an injected Fetcher.
// Exported for use by tests in package server_test; not part of the stable API.
func NewConstitutionHandlerForTest(scoper storage.Scoper, f Fetcher) *ConstitutionHandler {
	return &ConstitutionHandler{scoper: scoper, fetcher: f}
}

// GetConstitution handles the GetConstitution RPC.
func (h *ConstitutionHandler) GetConstitution(ctx context.Context, req *connect.Request[specv1.GetConstitutionRequest]) (*connect.Response[specv1.GetConstitutionResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg

	// Single layer query.
	if msg.Layer != specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		domainLayer, ok := constitutionLayerFromProtoMap[msg.Layer]
		if !ok {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown layer: %s", msg.Layer))
		}
		c, getErr := store.GetConstitutionLayer(ctx, domainLayer)
		if getErr != nil {
			return nil, constitutionError(getErr)
		}
		return connect.NewResponse(&specv1.GetConstitutionResponse{
			Constitution: constitutionToProto(c),
		}), nil
	}

	// Merged query.
	layers, err := store.GetAllLayers(ctx)
	if err != nil {
		return nil, constitutionError(err)
	}
	if len(layers) == 0 {
		return nil, constitutionError(fmt.Errorf("%w", storage.ErrConstitutionNotFound))
	}

	result, mergeErr := merge.Layers(layers)
	if mergeErr != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("merge layers: %w", mergeErr))
	}

	return connect.NewResponse(&specv1.GetConstitutionResponse{
		Constitution: constitutionToProto(result.Constitution),
		Provenance:   provenanceToProto(result.Provenance),
	}), nil
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
	domainConst, parseErr := constitutionFromProto(msg.Constitution)
	if parseErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, parseErr)
	}
	c, err := store.UpdateConstitution(ctx, domainConst)
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

	layers, err := store.GetAllLayers(ctx)
	if err != nil {
		return nil, constitutionError(err)
	}
	if len(layers) == 0 {
		return nil, constitutionError(fmt.Errorf("%w", storage.ErrConstitutionNotFound))
	}
	result, mergeErr := merge.Layers(layers)
	if mergeErr != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("merge layers: %w", mergeErr))
	}
	c := result.Constitution

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

// RefreshConstitutionLayer handles the RefreshConstitutionLayer RPC.
func (h *ConstitutionHandler) RefreshConstitutionLayer(ctx context.Context, req *connect.Request[specv1.RefreshConstitutionLayerRequest]) (*connect.Response[specv1.RefreshConstitutionLayerResponse], error) {
	// 1. Validate layer.
	if req.Msg.Layer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("layer is required"))
	}
	domainLayer, ok := constitutionLayerFromProtoMap[req.Msg.Layer]
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown layer: %s", req.Msg.Layer))
	}

	// 2. Fetch (URL credential sanitization happens inside the fetcher).
	fetched, err := h.fetcher.Fetch(ctx, req.Msg.SourceUrl)
	if err != nil {
		return nil, classifyFetchError(err)
	}

	// 3. Parse.
	parsed, err := load.FromYAML(fetched.Body)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("parse: %w", err))
	}
	// Override any layer field in the YAML with the request's explicit choice.
	parsed.Layer = domainLayer
	parsed.SourceURL = fetched.ResolvedURL

	// 4. Hash for drift comparison.
	newHash, err := hash.Hash(parsed)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("hash: %w", err))
	}
	parsed.SourceHash = newHash

	// 5. Get scoped store.
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}

	// 6. Compare to existing layer.
	prior, priorErr := store.GetConstitutionLayer(ctx, domainLayer)
	var prevHash string
	if priorErr == nil {
		prevHash = prior.SourceHash
	} else if !errors.Is(priorErr, storage.ErrConstitutionNotFound) {
		return nil, constitutionError(priorErr)
	}

	changed := prevHash != newHash

	resp := &specv1.RefreshConstitutionLayerResponse{
		After:              constitutionToProto(parsed),
		PreviousSourceHash: prevHash,
		NewSourceHash:      newHash,
		Changed:            changed,
	}
	if priorErr == nil {
		resp.Before = constitutionToProto(prior)
	}

	// 7. Dry-run or no-change → return without writing.
	if req.Msg.DryRun || !changed {
		return connect.NewResponse(resp), nil
	}

	// 8. Write.
	written, err := store.UpdateConstitution(ctx, parsed)
	if err != nil {
		return nil, constitutionError(err)
	}
	resp.After = constitutionToProto(written)
	return connect.NewResponse(resp), nil
}

// classifyFetchError maps fetch errors to gRPC codes per Section 12 of the design.
// URL/parse/size errors → CodeInvalidArgument.
// All other fetch failures → CodeUnavailable.
//
// Uses string matching against the fetch package's known error message
// patterns. This is fragile but bounded; the fetch package is the only
// producer of these errors.
func classifyFetchError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "embedded credentials"),
		strings.Contains(msg, "credential parameter"),
		strings.Contains(msg, "exceeds"),
		strings.Contains(msg, "invalid URL"),
		strings.Contains(msg, "unsupported scheme"),
		strings.Contains(msg, "unsupported protocol"),
		strings.Contains(msg, "no getter"):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeUnavailable, err)
	}
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
		slog.LogAttrs(context.Background(), slog.LevelError, "constitutionError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
