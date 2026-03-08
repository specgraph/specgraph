// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package drift detects spec-vs-dependency staleness in the graph.
package drift

import (
	"context"
	"errors"
	"fmt"

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
}

// NewEngine creates a new drift detection engine.
func NewEngine(backend Backend) *Engine {
	return &Engine{backend: backend}
}

// validScopes lists the recognized drift scope values.
var validScopes = map[string]bool{
	"":           true,
	"deps":       true,
	"interfaces": true,
	"verify":     true,
}

// Check runs drift detection for a single spec (by slug) or all eligible specs (empty slug).
// The scope parameter filters which drift checks to run: "deps", "interfaces", "verify", or "" (all).
func (e *Engine) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	if !validScopes[scope] {
		return nil, fmt.Errorf("drift: unknown scope %q (valid: deps, interfaces, verify)", scope)
	}
	var specs []*storage.Spec

	if slug != "" {
		spec, err := e.backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("drift: get spec %q: %w", slug, err)
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
			return nil, err
		}
		if len(report.Items) > 0 {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (e *Engine) checkSpec(ctx context.Context, spec *storage.Spec, scope string) (storage.DriftReport, error) {
	report := storage.DriftReport{SpecSlug: spec.Slug}

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
						Severity:     storage.DriftSeverityInfo,
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

	// Interface drift — placeholder
	// Verify drift — placeholder

	return report, nil
}
