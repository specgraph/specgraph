// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package drift detects spec-vs-dependency staleness in the graph.
package drift

import (
	"context"
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"
)

// Backend is the subset of storage needed by the drift engine.
type Backend interface {
	GetSpec(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error)
}

// Engine runs drift detection checks.
type Engine struct {
	backend Backend
}

// NewEngine creates a new drift detection engine.
func NewEngine(backend Backend) *Engine {
	return &Engine{backend: backend}
}

// Check runs drift detection for a single spec (by slug) or all eligible specs (empty slug).
// The scope parameter filters which drift checks to run: "deps", "interfaces", "verify", or "" (all).
func (e *Engine) Check(ctx context.Context, slug, scope string) ([]storage.DriftReport, error) {
	var specs []*storage.Spec

	if slug != "" {
		spec, err := e.backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("drift: get spec %q: %w", slug, err)
		}
		specs = []*storage.Spec{spec}
	} else {
		doneSpecs, err := e.backend.ListSpecs(ctx, string(storage.SpecStageDone), "", 0)
		if err != nil {
			return nil, fmt.Errorf("drift: list done specs: %w", err)
		}
		amendedSpecs, err := e.backend.ListSpecs(ctx, string(storage.SpecStageAmended), "", 0)
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
		reports = append(reports, report)
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
				continue // skip missing deps
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
