// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Drift ---

// SYNC: keep in sync with validScopes (internal/driftscope/scope.go)
// and driftScopeToProtoMap (cmd/specgraph/lifecycle.go).
var driftScopeFromProtoMap = map[specv1.DriftScope]string{
	specv1.DriftScope_DRIFT_SCOPE_UNSPECIFIED: "", // all scopes when client omits the field
	specv1.DriftScope_DRIFT_SCOPE_DEPS:        "deps",
	specv1.DriftScope_DRIFT_SCOPE_INTERFACES:  "interfaces",
	specv1.DriftScope_DRIFT_SCOPE_VERIFY:      "verify",
}

func driftScopeFromProto(scope specv1.DriftScope) (string, bool) {
	if s, ok := driftScopeFromProtoMap[scope]; ok {
		return s, true
	}
	return "", false
}

var driftTypeToProtoMap = map[storage.DriftType]specv1.DriftType{
	storage.DriftTypeDependency: specv1.DriftType_DRIFT_TYPE_DEPENDENCY,
	storage.DriftTypeInterface:  specv1.DriftType_DRIFT_TYPE_INTERFACE,
	storage.DriftTypeVerify:     specv1.DriftType_DRIFT_TYPE_VERIFY,
}

var driftSeverityToProtoMap = map[storage.DriftSeverity]specv1.DriftSeverity{
	storage.DriftSeverityHigh:   specv1.DriftSeverity_DRIFT_SEVERITY_HIGH,
	storage.DriftSeverityMedium: specv1.DriftSeverity_DRIFT_SEVERITY_MEDIUM,
	storage.DriftSeverityLow:    specv1.DriftSeverity_DRIFT_SEVERITY_LOW,
	storage.DriftSeverityInfo:   specv1.DriftSeverity_DRIFT_SEVERITY_INFO,
}

func driftReportToProto(r *storage.DriftReport) (*specv1.DriftReport, error) {
	items := make([]*specv1.DriftItem, len(r.Items))
	for i, item := range r.Items {
		dt, ok := driftTypeToProtoMap[item.Type]
		if !ok {
			return nil, fmt.Errorf("driftReportToProto: unknown drift type %q for slug %q", item.Type, r.SpecSlug)
		}
		ds, ok := driftSeverityToProtoMap[item.Severity]
		if !ok {
			return nil, fmt.Errorf("driftReportToProto: unknown drift severity %q for slug %q", item.Severity, r.SpecSlug)
		}
		items[i] = &specv1.DriftItem{
			Type:         dt,
			Severity:     ds,
			Description:  item.Description,
			SpecSlug:     item.SpecSlug,
			UpstreamSlug: item.UpstreamSlug,
			ExpectedHash: item.ExpectedHash,
			ActualHash:   item.ActualHash,
		}
	}
	return &specv1.DriftReport{
		SpecSlug:     r.SpecSlug,
		Items:        items,
		ErrorMessage: r.ErrorMessage,
	}, nil
}

func driftReportsToProto(reports []storage.DriftReport) ([]*specv1.DriftReport, error) {
	result := make([]*specv1.DriftReport, len(reports))
	for i := range reports {
		var err error
		result[i], err = driftReportToProto(&reports[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
