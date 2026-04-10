// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package markdown

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderSDD renders a spec's Specify + Decompose outputs as an SDD document.
func (r *Renderer) RenderSDD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# SDD: %s\n\n", spec.Slug)

	// Specify section
	if o := spec.SpecifyOutput; o != nil {
		if len(o.Interfaces) > 0 {
			b.WriteString("## Interface Contracts\n\n")
			for _, iface := range o.Interfaces {
				fmt.Fprintf(&b, "### %s\n\n", iface.Name)
				if iface.Body != "" {
					fmt.Fprintf(&b, "```\n%s\n```\n\n", iface.Body)
				}
			}
		}
		if len(o.VerifyCriteria) > 0 {
			rows := make([][]string, len(o.VerifyCriteria))
			for i, vc := range o.VerifyCriteria {
				rows[i] = []string{vc.Category, vc.Description}
			}
			b.WriteString(section("Acceptance Criteria", ItemTable([]string{"Category", "Description"}, rows)))
		}
		if len(o.Invariants) > 0 {
			var inv strings.Builder
			for _, s := range o.Invariants {
				fmt.Fprintf(&inv, "- %s\n", s)
			}
			b.WriteString(section("Invariants", inv.String()))
		}
		if len(o.Touches) > 0 {
			rows := make([][]string, len(o.Touches))
			for i, ft := range o.Touches {
				rows[i] = []string{ft.Path, ft.Purpose, ft.ChangeType}
			}
			b.WriteString(section("File Touches", ItemTable([]string{"Path", "Purpose", "Action"}, rows)))
		}
	}

	// Decompose section
	if o := spec.DecomposeOutput; o != nil {
		if strategy := decompositionStrategyString(o.Strategy); strategy != "" {
			b.WriteString(section("Decomposition Strategy", strategy))
		}
		if len(o.Slices) > 0 {
			b.WriteString("## Slices\n\n")
			rows := make([][]string, len(o.Slices))
			for i, s := range o.Slices {
				rows[i] = []string{
					s.Id,
					s.Intent,
					strings.Join(s.Verify, "; "),
					strings.Join(s.DependsOn, ", "),
				}
			}
			b.WriteString(ItemTable([]string{"ID", "Intent", "Verify", "Depends On"}, rows))
		}
	}

	return render.Document{
		Kind:     render.DocumentSDD,
		Title:    fmt.Sprintf("SDD: %s", spec.Slug),
		Body:     []byte(b.String()),
		SpecSlug: spec.Slug,
	}, nil
}
