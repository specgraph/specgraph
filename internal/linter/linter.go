// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

// Package linter validates specs using programmatic field checks and graph-consistency rules.
package linter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/specgraph/specgraph/internal/storage"
)

// Backend is the subset of storage needed by the linter.
type Backend = storage.SpecReader

// maxSpecsPerLint limits the number of specs processed in a single lint run.
const maxSpecsPerLint = 10000

// Engine provides a storage-backed implementation of server.SpecLinter,
type Engine struct {
	backend Backend
	logger  *slog.Logger
}

// NewEngine creates a linter engine backed by the given storage.
// If logger is nil, slog.Default() is used.
func NewEngine(backend Backend, logger *slog.Logger) *Engine {
	l := logger
	if l == nil {
		l = slog.Default()
	}
	return &Engine{backend: backend, logger: l}
}

// Lint validates one or all specs, returning lint results for each.
func (e *Engine) Lint(ctx context.Context, slug string) ([]storage.LintResult, error) {
	return lint(ctx, e.backend, e.logger, slug)
}

// lint validates one or all specs. When slug is empty, all specs are linted.
func lint(ctx context.Context, backend Backend, logger *slog.Logger, slug string) ([]storage.LintResult, error) {
	var specs []*storage.Spec

	if slug == "" {
		listed, err := backend.ListSpecs(ctx, "", "", maxSpecsPerLint)
		if err != nil {
			return nil, fmt.Errorf("linter: list specs: %w", err)
		}
		specs = listed
	} else {
		spec, err := backend.GetSpec(ctx, slug)
		if err != nil {
			// %w preserves errors.Is traversal; lifecycle handler depends on
			// errors.Is(err, storage.ErrSpecNotFound) through this chain.
			return nil, fmt.Errorf("linter: get spec %q: %w", slug, err)
		}
		specs = []*storage.Spec{spec}
	}

	results := make([]storage.LintResult, 0, len(specs))
	for _, spec := range specs {
		result, err := lintSpec(ctx, backend, spec)
		if err != nil {
			logger.ErrorContext(ctx, "linter: internal error",
				slog.String("spec_slug", spec.Slug),
				slog.Any("error", err))
			results = append(results, storage.LintResult{
				SpecSlug: spec.Slug,
				Error:    "internal error during lint",
			})
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// lintSpec runs all lint rules against a single spec.
func lintSpec(ctx context.Context, backend Backend, spec *storage.Spec) (storage.LintResult, error) {
	violations := ValidateSchema(spec)

	// Fetch dependencies once for the root spec, reused by both dangling-dep
	// and cycle-detection rules to avoid a redundant storage roundtrip.
	deps, err := backend.GetDependencies(ctx, spec.Slug)
	if err != nil {
		return storage.LintResult{}, fmt.Errorf("linter: fetch dependencies for %q: %w", spec.Slug, err)
	}

	// Rule 1 is schema validation (ValidateSchema above).
	// Rule 2: Edge consistency — dangling dependency references.
	danglingViolations, danglingSet, err := checkDanglingDeps(ctx, backend, spec.Slug, deps)
	if err != nil {
		return storage.LintResult{}, fmt.Errorf("linter: %w", err)
	}
	violations = append(violations, danglingViolations...)

	// Rule 3: Cycle detection.
	cycleViolations, err := detectCycles(ctx, backend, spec.Slug, deps, danglingSet)
	if err != nil {
		return storage.LintResult{}, fmt.Errorf("linter: %w", err)
	}
	violations = append(violations, cycleViolations...)

	return storage.LintResult{
		SpecSlug:   spec.Slug,
		Violations: violations,
		Passed:     len(violations) == 0,
	}, nil
}

// checkDanglingDeps verifies that each dependency target actually exists,
// using pre-fetched dependencies to avoid redundant storage calls.
func checkDanglingDeps(ctx context.Context, backend Backend, _ string, deps []storage.NodeRef) ([]storage.LintViolation, map[string]bool, error) {
	var violations []storage.LintViolation
	danglingSet := make(map[string]bool)
	for _, dep := range deps {
		_, err := backend.GetSpec(ctx, dep.Slug)
		if err != nil {
			if errors.Is(err, storage.ErrSpecNotFound) {
				danglingSet[dep.Slug] = true
				violations = append(violations, storage.LintViolation{
					Rule:     "edge.dangling_ref",
					Severity: storage.LintSeverityError,
					Message:  fmt.Sprintf("dependency %q references nonexistent spec", dep.Slug),
					Location: fmt.Sprintf("dependencies[%s]", dep.Slug),
				})
			} else {
				return nil, nil, fmt.Errorf("verify dependency %q: %w", dep.Slug, err)
			}
		}
	}

	return violations, danglingSet, nil
}

const maxCycleDepth = 1000

// detectCycles uses DFS to find back-edges in the dependency graph.
// rootDeps are the pre-fetched dependencies for the root node, used to avoid
// a redundant storage roundtrip on the first call.
//
// On storage error during traversal, (nil, err) is returned immediately.
// On success, violations contains the full set of detected cycles.
func detectCycles(ctx context.Context, backend Backend, slug string, rootDeps []storage.NodeRef, danglingSet map[string]bool) ([]storage.LintViolation, error) {
	visited := map[string]bool{}
	inStack := map[string]bool{}
	var violations []storage.LintViolation
	// storageErr is captured by the DFS closure to propagate storage errors out
	// of the recursive walk. Once set, the guard at the top of dfs
	// short-circuits remaining branches.
	var storageErr error

	var dfs func(current string, depth int)
	dfs = func(current string, depth int) {
		if storageErr != nil {
			return
		}
		if inStack[current] {
			violations = append(violations, storage.LintViolation{
				Rule:     "graph.cycle",
				Severity: storage.LintSeverityError,
				Message:  fmt.Sprintf("dependency cycle detected involving %q", current),
				Location: current,
			})
			return
		}
		if visited[current] {
			return
		}

		if depth > maxCycleDepth {
			visited[current] = true
			violations = append(violations, storage.LintViolation{
				Rule:     "graph.cycle",
				Severity: storage.LintSeverityWarning,
				Message:  fmt.Sprintf("dependency graph exceeds maximum depth of %d at %q", maxCycleDepth, current),
				Location: current,
			})
			return
		}

		visited[current] = true
		inStack[current] = true

		var deps []storage.NodeRef
		var err error
		// current == slug only on the first call since slug uniqueness is enforced by storage.
		if current == slug && rootDeps != nil {
			deps = rootDeps
		} else {
			deps, err = backend.GetDependencies(ctx, current)
		}
		if err != nil {
			if errors.Is(err, storage.ErrSpecNotFound) {
				// Skip duplicate violation if this slug was already reported as a dangling ref.
				if !danglingSet[current] {
					violations = append(violations, storage.LintViolation{
						Rule:     "graph.missing_node",
						Severity: storage.LintSeverityWarning,
						Message:  fmt.Sprintf("spec %q referenced in dependency graph but not found", current),
						Location: current,
					})
				}
				inStack[current] = false
				return
			}
			storageErr = fmt.Errorf("get dependencies for %q: %w", current, err)
			inStack[current] = false
			return
		}
		for _, dep := range deps {
			dfs(dep.Slug, depth+1)
		}

		inStack[current] = false
	}

	dfs(slug, 0)

	if storageErr != nil {
		return nil, storageErr
	}
	return violations, nil
}
