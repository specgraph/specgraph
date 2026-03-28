// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

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
	scoper              storage.Scoper
	templateOverrideDir string
}

var _ specgraphv1connect.AnalyticalPassServiceHandler = (*AnalyticalPassHandler)(nil)

// RegisterAnalyticalPassService registers the AnalyticalPassService on the given mux.
// templateOverrideDir, if non-empty, is checked for <pass_type>.md files before
// falling back to the embedded defaults. Typical value: ".specgraph/templates".
func RegisterAnalyticalPassService(mux *http.ServeMux, scoper storage.Scoper, templateOverrideDir string, opts ...connect.HandlerOption) {
	if scoper == nil {
		panic("RegisterAnalyticalPassService: scoper must not be nil")
	}
	handler := &AnalyticalPassHandler{scoper: scoper, templateOverrideDir: templateOverrideDir}
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
	pt, ptErr := passTypeFromProto(msg.PassType)
	if ptErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, ptErr)
	}

	spec, err := store.GetSpec(ctx, msg.Slug)
	if err != nil {
		return nil, analyticalPassError(err)
	}

	tmplBytes, err := h.loadTemplate(pt)
	if err != nil {
		slog.Error("analyticalPassError: template load failed", slog.Any("error", err))
		return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}

	stage := spec.Stage
	offered := authoring.OfferedPasses(stage, authoring.PostureDrive)

	return connect.NewResponse(&specv1.RunAnalyticalPassResponse{
		PassType:       msg.PassType,
		PromptTemplate: string(tmplBytes),
		Tools:          passToolManifest(msg.Slug),
		InitialMessage: fmt.Sprintf("Run the %s analytical pass on spec %q (stage: %s).", pt, msg.Slug, spec.Stage),
		OfferedPasses:  offered,
		Stage:          string(spec.Stage),
	}), nil
}

// loadTemplate returns the prompt template for the given pass type.
// If templateOverrideDir is set and contains a <passType>.md file, that file
// is used. Otherwise falls back to the embedded default.
func (h *AnalyticalPassHandler) loadTemplate(pt storage.PassType) ([]byte, error) {
	fileName := string(pt) + ".md"
	if h.templateOverrideDir != "" {
		overridePath := filepath.Join(h.templateOverrideDir, fileName)
		data, err := os.ReadFile(overridePath)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("read template override %q: %w", overridePath, err)
		}
		// File not found — fall through to embedded default.
	}
	data, err := templateFS.ReadFile("templates/" + fileName)
	if err != nil {
		return nil, fmt.Errorf("load template %q: %w", fileName, err)
	}
	return data, nil
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
	pt, ptErr := passTypeFromProto(msg.PassType)
	if ptErr != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, ptErr)
	}
	if len(msg.Findings) > maxFindingsPerRequest {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("too many findings: %d exceeds maximum of %d", len(msg.Findings), maxFindingsPerRequest))
	}
	domain := make([]storage.AnalyticalFindingInput, len(msg.Findings))
	for i, f := range msg.Findings {
		if f == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("finding[%d]: must not be null", i))
		}
		if err := validateRequiredField("summary", f.Summary); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("finding[%d]: %w", i, err))
		}
		if len(f.Detail) > maxFindingDetailLen {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("finding[%d]: detail exceeds maximum length of %d", i, maxFindingDetailLen))
		}
		sev, convErr := findingSeverityFromProto(f.Severity)
		if convErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("finding[%d]: %w", i, convErr))
		}
		domain[i] = storage.AnalyticalFindingInput{
			Severity:   sev,
			Summary:    f.Summary,
			Detail:     f.Detail,
			Constraint: f.Constraint,
			Resolution: f.Resolution,
		}
	}

	ids, sErr := store.StoreFindings(ctx, msg.Slug, pt, domain)
	if sErr != nil {
		return nil, analyticalPassError(sErr)
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
	if msg.PassType != specv1.PassType_PASS_TYPE_UNSPECIFIED {
		var ptErr error
		pt, ptErr = passTypeFromProto(msg.PassType)
		if ptErr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, ptErr)
		}
	}

	findings, err := store.ListFindings(ctx, msg.Slug, pt)
	if err != nil {
		return nil, analyticalPassError(err)
	}

	proto := make([]*specv1.AnalyticalFinding, len(findings))
	for i := range findings {
		f := &findings[i]
		protoSev, sevErr := findingSeverityToProto(f.Severity)
		if sevErr != nil {
			return nil, analyticalPassError(sevErr)
		}
		protoPassType, ptErr := passTypeToProto(f.PassType)
		if ptErr != nil {
			return nil, analyticalPassError(ptErr)
		}
		proto[i] = &specv1.AnalyticalFinding{
			Id:         f.ID,
			PassType:   protoPassType,
			Severity:   protoSev,
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
		{Name: "show_spec", Command: fmt.Sprintf("specgraph show %q --json", slug), Description: "Read the spec's full content including all stage outputs"},
		{Name: "show_constitution", Command: "specgraph constitution show --json", Description: "Read the full project constitution"},
		{Name: "list_deps", Command: fmt.Sprintf("specgraph edges %q --type DEPENDS_ON --json", slug), Description: "List specs this one depends on"},
		{Name: "show_dep", Command: "specgraph show {slug} --json", Description: "Read a specific dependency's full content (replace {slug} with the dependency slug)"},
	}
}

// findingSeverityFromProto converts a proto FindingSeverity to the domain type.
// Returns an error for UNSPECIFIED or unknown numeric values.
func findingSeverityFromProto(s specv1.FindingSeverity) (storage.FindingSeverity, error) {
	switch s {
	case specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL:
		return storage.SeverityCritical, nil
	case specv1.FindingSeverity_FINDING_SEVERITY_WARNING:
		return storage.SeverityWarning, nil
	case specv1.FindingSeverity_FINDING_SEVERITY_NOTE:
		return storage.SeverityNote, nil
	default:
		return "", fmt.Errorf("invalid severity %d", int32(s))
	}
}

// findingSeverityToProto converts a domain FindingSeverity to the proto type.
// Returns an error for unknown severity values to surface data integrity issues.
func findingSeverityToProto(s storage.FindingSeverity) (specv1.FindingSeverity, error) {
	switch s {
	case storage.SeverityCritical:
		return specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL, nil
	case storage.SeverityWarning:
		return specv1.FindingSeverity_FINDING_SEVERITY_WARNING, nil
	case storage.SeverityNote:
		return specv1.FindingSeverity_FINDING_SEVERITY_NOTE, nil
	default:
		return specv1.FindingSeverity_FINDING_SEVERITY_UNSPECIFIED, fmt.Errorf("unknown severity %q", s)
	}
}

// passTypeFromProto converts a proto PassType to the domain type.
// Returns an error for UNSPECIFIED or unknown numeric values.
func passTypeFromProto(p specv1.PassType) (storage.PassType, error) {
	switch p {
	case specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK:
		return storage.PassTypeConstitutionCheck, nil
	case specv1.PassType_PASS_TYPE_RED_TEAM:
		return storage.PassTypeRedTeam, nil
	case specv1.PassType_PASS_TYPE_PERIPHERAL_VISION:
		return storage.PassTypePeripheralVision, nil
	case specv1.PassType_PASS_TYPE_CONSISTENCY:
		return storage.PassTypeConsistencyCheck, nil
	case specv1.PassType_PASS_TYPE_SIMPLICITY:
		return storage.PassTypeSimplicityCheck, nil
	default:
		return "", fmt.Errorf("invalid pass_type %d", int32(p))
	}
}

// passTypeToProto converts a domain PassType to the proto type.
// Returns an error for unknown pass type values to surface data integrity issues.
func passTypeToProto(p storage.PassType) (specv1.PassType, error) {
	switch p {
	case storage.PassTypeConstitutionCheck:
		return specv1.PassType_PASS_TYPE_CONSTITUTION_CHECK, nil
	case storage.PassTypeRedTeam:
		return specv1.PassType_PASS_TYPE_RED_TEAM, nil
	case storage.PassTypePeripheralVision:
		return specv1.PassType_PASS_TYPE_PERIPHERAL_VISION, nil
	case storage.PassTypeConsistencyCheck:
		return specv1.PassType_PASS_TYPE_CONSISTENCY, nil
	case storage.PassTypeSimplicityCheck:
		return specv1.PassType_PASS_TYPE_SIMPLICITY, nil
	default:
		return specv1.PassType_PASS_TYPE_UNSPECIFIED, fmt.Errorf("unknown pass_type %q", p)
	}
}

// analyticalPassError maps storage errors to sanitized connect error codes.
func analyticalPassError(err error) error {
	var connErr *connect.Error
	if errors.As(err, &connErr) {
		return connErr
	}
	switch {
	case errors.Is(err, storage.ErrSpecNotFound):
		return connect.NewError(connect.CodeNotFound, errors.New("spec not found"))
	default:
		slog.Error("analyticalPassError: internal error", slog.Any("error", err))
		return connect.NewError(connect.CodeInternal, errors.New("internal error"))
	}
}
