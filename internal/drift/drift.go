// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package drift detects spec-vs-dependency staleness in the graph.
package drift

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/specgraph/specgraph/internal/driftscope"
	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the subset of storage needed by the drift engine.
type Backend interface {
	GetSpec(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	GetDependenciesWithEdgeData(ctx context.Context, slug string) ([]storage.DependencyRef, error)
}

// maxSpecsPerCheck limits the number of specs returned per ListSpecs call.
const maxSpecsPerCheck = 10000

// CheckResult holds drift detection results plus metadata.
type CheckResult struct {
	Reports      []storage.DriftReport
	SkippedCount int32 // specs not in done stage (all-specs mode only)
}

// Engine runs drift detection checks.
type Engine struct {
	backend Backend
	logger  *slog.Logger
}

// NewEngine creates a new drift detection engine.
func NewEngine(backend Backend, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{backend: backend, logger: logger}
}

// Check runs drift detection for a single spec (by slug) or all eligible specs (empty slug).
// The scope parameter filters which drift checks to run: "deps", "interfaces", "verify", or "" (all).
func (e *Engine) Check(ctx context.Context, slug, scope string) (*CheckResult, error) {
	if !driftscope.IsValid(scope) {
		return nil, fmt.Errorf("drift: unknown scope %q (valid: deps, interfaces, verify)", scope)
	}
	var specs []*storage.Spec
	var skipped int32

	if slug != "" {
		spec, err := e.backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("drift: get spec %q: %w", slug, err)
		}
		// Only done specs are eligible for drift detection.
		if spec.Stage != storage.SpecStageDone {
			return nil, fmt.Errorf("drift: spec %q stage %q: %w", slug, spec.Stage, storage.ErrSpecIneligibleForDrift)
		}
		specs = []*storage.Spec{spec}
	} else {
		doneSpecs, err := e.backend.ListSpecs(ctx, string(storage.SpecStageDone), "", maxSpecsPerCheck)
		if err != nil {
			return nil, fmt.Errorf("drift: list done specs: %w", err)
		}
		allSpecs, err := e.backend.ListSpecs(ctx, "", "", maxSpecsPerCheck)
		if err != nil {
			return nil, fmt.Errorf("drift: list all specs: %w", err)
		}
		specs = append(specs, doneSpecs...)
		if diff := len(allSpecs) - len(specs); diff > 0 {
			skipped = int32(diff) //nolint:gosec // bounded by maxSpecsPerCheck (10000)
		}
	}

	reports := make([]storage.DriftReport, 0, len(specs))
	for _, spec := range specs {
		report, err := e.checkSpec(ctx, spec, scope)
		if err != nil {
			e.logger.Warn("drift check failed for spec",
				slog.String("slug", spec.Slug),
				slog.Any("error", err))
			reports = append(reports, storage.DriftReport{
				SpecSlug:     spec.Slug,
				Items:        []storage.DriftItem{},
				ErrorMessage: sanitizeDriftError(err),
			})
			continue
		}
		if len(report.Items) > 0 || report.ErrorMessage != "" {
			reports = append(reports, report)
		}
	}
	return &CheckResult{Reports: reports, SkippedCount: skipped}, nil
}

func (e *Engine) checkSpec(ctx context.Context, spec *storage.Spec, scope string) (storage.DriftReport, error) {
	report := storage.DriftReport{SpecSlug: spec.Slug, Items: []storage.DriftItem{}}

	if scope == "" || scope == "deps" {
		deps, err := e.backend.GetDependenciesWithEdgeData(ctx, spec.Slug)
		if err != nil {
			return report, fmt.Errorf("drift: get dependencies for %q: %w", spec.Slug, err)
		}
		for _, dep := range deps {
			upstream, err := e.backend.GetSpec(ctx, dep.Slug)
			if err != nil {
				if errors.Is(err, storage.ErrSpecNotFound) {
					report.Items = append(report.Items, storage.DriftItem{
						Type:         storage.DriftTypeDependency,
						Severity:     storage.DriftSeverityMedium,
						Description:  fmt.Sprintf("dependency %q not found", dep.Slug),
						SpecSlug:     spec.Slug,
						UpstreamSlug: dep.Slug,
					})
					continue
				}
				return report, fmt.Errorf("drift: get upstream spec %q: %w", dep.Slug, err)
			}
			if upstream.ContentHash != dep.ContentHashAtLink {
				report.Items = append(report.Items, storage.DriftItem{
					Type:         storage.DriftTypeDependency,
					Severity:     storage.DriftSeverityMedium,
					Description:  fmt.Sprintf("upstream %q content changed since %q baselined", upstream.Slug, spec.Slug),
					SpecSlug:     spec.Slug,
					UpstreamSlug: upstream.Slug,
					ExpectedHash: dep.ContentHashAtLink,
					ActualHash:   upstream.ContentHash,
				})
			}
		}
	}

	// Interface and verify drift — not yet implemented. Only set ErrorMessage
	// when the client explicitly requests these scopes, so all-specs checks
	// (scope="") don't pollute every report with a not-implemented note.
	switch scope {
	case "interfaces":
		report.ErrorMessage = "interface drift checking not yet implemented"
	case "verify":
		report.ErrorMessage = "verify drift checking not yet implemented"
	}

	return report, nil
}

// sanitizeDriftError maps known error types to safe client-facing messages.
// Unknown errors get a generic message; the real error is already logged.
// sanitizeDriftError maps internal errors to safe user-facing messages.
// Note: the ErrSpecIneligibleForDrift branch is currently unreachable because
// Check() returns that error at the top level (before checkSpec is called), so
// it never propagates through the per-spec error recovery path. The branch is
// kept as a defensive guard for future refactoring.
func sanitizeDriftError(err error) string {
	if errors.Is(err, storage.ErrSpecNotFound) {
		return "dependency not found"
	}
	if errors.Is(err, storage.ErrSpecIneligibleForDrift) {
		return "spec is not eligible for drift checking"
	}
	return "drift check failed"
}
