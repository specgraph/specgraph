// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package linter

import (
	"context"
	"errors"
	"fmt"

	"github.com/seanb4t/specgraph/internal/storage"
)

// Backend is the subset of storage needed by the linter.
type Backend interface {
	GetSpec(ctx context.Context, slug string) (*storage.Spec, error)
	ListSpecs(ctx context.Context, stage, priority string, limit int) ([]*storage.Spec, error)
	GetDependencies(ctx context.Context, slug string) ([]storage.NodeRef, error)
}

// Lint validates one or all specs. When slug is empty, all specs are linted.
func Lint(ctx context.Context, backend Backend, slug string) ([]storage.LintResult, error) {
	var specs []*storage.Spec

	if slug == "" {
		listed, err := backend.ListSpecs(ctx, "", "", 0)
		if err != nil {
			return nil, fmt.Errorf("linter: list specs: %w", err)
		}
		specs = listed
	} else {
		spec, err := backend.GetSpec(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("linter: get spec %q: %w", slug, err)
		}
		specs = []*storage.Spec{spec}
	}

	results := make([]storage.LintResult, 0, len(specs))
	for _, spec := range specs {
		result := lintSpec(ctx, backend, spec)
		results = append(results, result)
	}

	return results, nil
}

// lintSpec runs all lint rules against a single spec.
func lintSpec(ctx context.Context, backend Backend, spec *storage.Spec) storage.LintResult {
	violations := ValidateSchema(spec)

	// Rule 2: Edge consistency — dangling dependency references.
	violations = append(violations, checkDanglingDeps(ctx, backend, spec.Slug)...)

	// Rule 3: Cycle detection.
	violations = append(violations, detectCycles(ctx, backend, spec.Slug)...)

	return storage.LintResult{
		SpecSlug:   spec.Slug,
		Violations: violations,
		Passed:     len(violations) == 0,
	}
}

// checkDanglingDeps verifies that each dependency target actually exists.
func checkDanglingDeps(ctx context.Context, backend Backend, slug string) []storage.LintViolation {
	deps, err := backend.GetDependencies(ctx, slug)
	if err != nil {
		return []storage.LintViolation{{
			Rule:     "edge.dangling_ref",
			Severity: storage.LintSeverityError,
			Message:  fmt.Sprintf("failed to fetch dependencies for %q: %v", slug, err),
			Location: slug,
		}}
	}

	var violations []storage.LintViolation
	for _, dep := range deps {
		_, err := backend.GetSpec(ctx, dep.Slug)
		if err != nil {
			if errors.Is(err, storage.ErrSpecNotFound) {
				violations = append(violations, storage.LintViolation{
					Rule:     "edge.dangling_ref",
					Severity: storage.LintSeverityError,
					Message:  fmt.Sprintf("dependency %q references nonexistent spec", dep.Slug),
					Location: fmt.Sprintf("dependencies[%s]", dep.Slug),
				})
			} else {
				violations = append(violations, storage.LintViolation{
					Rule:     "edge.dangling_ref",
					Severity: storage.LintSeverityError,
					Message:  fmt.Sprintf("failed to verify dependency %q: %v", dep.Slug, err),
					Location: fmt.Sprintf("dependencies[%s]", dep.Slug),
				})
			}
		}
	}

	return violations
}

// detectCycles uses DFS to find back-edges in the dependency graph.
func detectCycles(ctx context.Context, backend Backend, slug string) []storage.LintViolation {
	visited := map[string]bool{}
	inStack := map[string]bool{}
	var violations []storage.LintViolation

	var dfs func(current string)
	dfs = func(current string) {
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

		visited[current] = true
		inStack[current] = true

		deps, err := backend.GetDependencies(ctx, current)
		if err != nil {
			violations = append(violations, storage.LintViolation{
				Rule:     "graph.cycle",
				Severity: storage.LintSeverityError,
				Message:  fmt.Sprintf("failed to get dependencies for %q: %v", current, err),
				Location: current,
			})
			inStack[current] = false
			return
		}
		for _, dep := range deps {
			dfs(dep.Slug)
		}

		inStack[current] = false
	}

	dfs(slug)

	return violations
}
