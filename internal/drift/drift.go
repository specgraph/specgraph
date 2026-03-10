// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package drift detects spec-vs-dependency staleness in the graph.
package drift

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/seanb4t/specgraph/internal/driftscope"
	"github.com/seanb4t/specgraph/internal/storage"
)

// Backend is the subset of storage needed by the drift engine.
type Backend = storage.SpecReader

// maxSpecsPerCheck limits the number of specs returned per ListSpecs call.
// Note: check-all mode calls ListSpecs twice (done + amended), so the actual
// ceiling is 2 * maxSpecsPerCheck.
const maxSpecsPerCheck = 10000

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
func (e *Engine) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	if !driftscope.IsValid(scope) {
		return nil, fmt.Errorf("drift: unknown scope %q (valid: deps, interfaces, verify)", scope)
	}
	var specs []*storage.Spec

	if slug != "" {
		spec, err := e.backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("drift: get spec %q: %w", slug, err)
		}
		// Only done and amended specs are eligible for drift detection.
		if spec.Stage != storage.SpecStageDone && spec.Stage != storage.SpecStageAmended {
			return nil, fmt.Errorf("drift: spec %q stage %q: %w", slug, spec.Stage, storage.ErrSpecIneligibleForDrift)
		}
		specs = []*storage.Spec{spec}
	} else {
		doneSpecs, err := e.backend.ListSpecs(ctx, string(storage.SpecStageDone), "", maxSpecsPerCheck)
		if err != nil {
			return nil, fmt.Errorf("drift: list done specs: %w", err)
		}
		amendedSpecs, err := e.backend.ListSpecs(ctx, string(storage.SpecStageAmended), "", maxSpecsPerCheck)
		if err != nil {
			return nil, fmt.Errorf("drift: list amended specs: %w", err)
		}
		specs = append(specs, doneSpecs...)
		specs = append(specs, amendedSpecs...)
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
		if len(report.Items) > 0 {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (e *Engine) checkSpec(ctx context.Context, spec *storage.Spec, scope string) (storage.DriftReport, error) {
	report := storage.DriftReport{SpecSlug: spec.Slug, Items: []storage.DriftItem{}}

	// Dependency drift
	if scope == "" || scope == "deps" {
		deps, err := e.backend.GetDependencies(ctx, spec.Slug)
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
			if upstream.UpdatedAt.After(spec.UpdatedAt) {
				report.Items = append(report.Items, storage.DriftItem{
					Type:            storage.DriftTypeDependency,
					Severity:        storage.DriftSeverityMedium,
					Description:     fmt.Sprintf("upstream %q updated after %q", upstream.Slug, spec.Slug),
					SpecSlug:        spec.Slug,
					UpstreamSlug:    upstream.Slug,
					ExpectedVersion: spec.Version,
					ActualVersion:   upstream.Version,
				})
			}
		}
	}

	// Interface drift — not yet implemented. Scope "interfaces" is accepted
	// but returns no items until interface tracking is built.
	// Verify drift — not yet implemented. Scope "verify" is accepted
	// but returns no items until verification checks are built.

	return report, nil
}

// sanitizeDriftError maps known error types to safe client-facing messages.
// Unknown errors get a generic message; the real error is already logged.
func sanitizeDriftError(err error) string {
	if errors.Is(err, storage.ErrSpecNotFound) {
		return "dependency not found"
	}
	if errors.Is(err, storage.ErrSpecIneligibleForDrift) {
		return "spec is not eligible for drift checking"
	}
	return "drift check failed"
}
