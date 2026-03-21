// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/gen/specgraph/v1/specgraphv1connect"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/specgraph/specgraph/internal/storage"
)

//go:embed templates/*.md
var templateFS embed.FS

// AnalyticalPassHandler implements the ConnectRPC AnalyticalPassService.
type AnalyticalPassHandler struct {
	scoper storage.Scoper
	logger *slog.Logger
}

var _ specgraphv1connect.AnalyticalPassServiceHandler = (*AnalyticalPassHandler)(nil)

// RegisterAnalyticalPassService registers the AnalyticalPassService on the given mux.
func RegisterAnalyticalPassService(mux *http.ServeMux, scoper storage.Scoper, opts ...connect.HandlerOption) {
	if scoper == nil {
		panic("RegisterAnalyticalPassService: scoper must not be nil")
	}
	handler := &AnalyticalPassHandler{scoper: scoper, logger: slog.Default()}
	path, h := specgraphv1connect.NewAnalyticalPassServiceHandler(handler, opts...)
	mux.Handle(path, h)
}

// RunAnalyticalPass validates the request, loads the spec and prompt template,
// and returns the template + tool manifest for the caller to execute the pass.
func (h *AnalyticalPassHandler) RunAnalyticalPass(ctx context.Context, req *connect.Request[specv1.RunAnalyticalPassRequest]) (*connect.Response[specv1.RunAnalyticalPassResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if vErr := validateSlug(msg.Slug); vErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, vErr)
	}
	if msg.PassName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("pass_name is required"))
	}
	if !storage.ValidPassType(storage.PassType(msg.PassName)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass_name %q", msg.PassName))
	}

	spec, err := store.GetSpec(ctx, msg.Slug)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	templatePath := fmt.Sprintf("templates/%s.md", msg.PassName)
	tmplBytes, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("no template for pass %q", msg.PassName))
	}

	stage := authoring.Stage(spec.Stage)
	offered := authoring.OfferedPasses(stage, authoring.PostureDrive)

	return connect.NewResponse(&specv1.RunAnalyticalPassResponse{
		PassName:       msg.PassName,
		PromptTemplate: string(tmplBytes),
		Tools:          passToolManifest(msg.Slug),
		InitialMessage: fmt.Sprintf("Run the %s analytical pass on spec %q (stage: %s).", msg.PassName, msg.Slug, spec.Stage),
		OfferedPasses:  offered,
		Stage:          string(spec.Stage),
	}), nil
}

// StoreFindings validates the request, converts proto findings to domain types,
// stores them, and returns the assigned IDs.
func (h *AnalyticalPassHandler) StoreFindings(ctx context.Context, req *connect.Request[specv1.StoreFindingsRequest]) (*connect.Response[specv1.StoreFindingsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if vErr := validateSlug(msg.Slug); vErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, vErr)
	}
	pt := storage.PassType(msg.PassType)
	if !storage.ValidPassType(pt) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass_type %q", msg.PassType))
	}
	domain := make([]storage.AnalyticalFinding, len(msg.Findings))
	for i, f := range msg.Findings {
		if f.Severity == specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("finding[%d]: severity must not be UNSPECIFIED", i))
		}
		domain[i] = storage.AnalyticalFinding{
			Severity:   findingSeverityFromProto(f.Severity),
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
		}
	}

	ids, sErr := store.StoreFindings(ctx, msg.Slug, pt, domain)
	if sErr != nil {
		if errors.Is(sErr, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, sErr)
		}
		return nil, connect.NewError(connect.CodeInternal, sErr)
	}

	return connect.NewResponse(&specv1.StoreFindingsResponse{
		Ids: ids,
	}), nil
}

// ListFindings validates the request and returns findings for the spec,
// optionally filtered by pass type.
func (h *AnalyticalPassHandler) ListFindings(ctx context.Context, req *connect.Request[specv1.ListFindingsRequest]) (*connect.Response[specv1.ListFindingsResponse], error) {
	store, err := scopeStore(ctx, h.scoper)
	if err != nil {
		return nil, err
	}
	msg := req.Msg
	if vErr := validateSlug(msg.Slug); vErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, vErr)
	}

	var pt storage.PassType
	if msg.PassType != "" {
		pt = storage.PassType(msg.PassType)
		if !storage.ValidPassType(pt) {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown pass_type %q", msg.PassType))
		}
	}

	findings, err := store.ListFindings(ctx, msg.Slug, pt)
	if err != nil {
		if errors.Is(err, storage.ErrSpecNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	proto := make([]*specv1.AnalyticalFinding, len(findings))
	for i := range findings {
		f := &findings[i]
		proto[i] = &specv1.AnalyticalFinding{
			Id:         f.ID,
			PassType:   string(f.PassType),
			Severity:   findingSeverityToProto(f.Severity),
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
			Version:    f.Version,
		}
	}

	return connect.NewResponse(&specv1.ListFindingsResponse{
		Findings: proto,
	}), nil
}

// passToolManifest returns the tool references for an analytical pass.
func passToolManifest(slug string) []*specv1.ToolReference {
	return []*specv1.ToolReference{
		{Name: "show_spec", Command: fmt.Sprintf("specgraph show %s --json", slug), Description: "Read the spec's full content including all stage outputs"},
		{Name: "show_constitution", Command: "specgraph constitution show --json", Description: "Read the full project constitution"},
		{Name: "list_deps", Command: fmt.Sprintf("specgraph edges %s --type DEPENDS_ON --json", slug), Description: "List specs this one depends on"},
		{Name: "show_dep", Command: "specgraph show {slug} --json", Description: "Read a specific dependency's full content (replace {slug} with the dependency slug)"},
	}
}

// findingSeverityFromProto converts a proto FindingSeverity to the domain type.
func findingSeverityFromProto(s specv1.FindingSeverity) storage.FindingSeverity {
	switch s {
	case specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL:
		return storage.SeverityCritical
	case specv1.FindingSeverity_FINDING_SEVERITY_WARNING:
		return storage.SeverityWarning
	case specv1.FindingSeverity_FINDING_SEVERITY_NOTE:
		return storage.SeverityNote
	default:
		return storage.SeverityNote
	}
}

// findingSeverityToProto converts a domain FindingSeverity to the proto type.
func findingSeverityToProto(s storage.FindingSeverity) specv1.FindingSeverity {
	switch s {
	case storage.SeverityCritical:
		return specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL
	case storage.SeverityWarning:
		return specv1.FindingSeverity_FINDING_SEVERITY_WARNING
	case storage.SeverityNote:
		return specv1.FindingSeverity_FINDING_SEVERITY_NOTE
	default:
		return specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED
	}
}
