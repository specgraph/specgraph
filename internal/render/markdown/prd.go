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

// Renderer implements render.Renderer for Markdown output.
type Renderer struct{}

// NewRenderer creates a Markdown renderer.
func NewRenderer() *Renderer { return &Renderer{} }

// RenderPRD renders a spec's Spark + Shape outputs as a PRD document.
func (r *Renderer) RenderPRD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# PRD: %s\n\n", spec.Slug)
	b.WriteString(metadataTable([][2]string{
		{"Stage", spec.Stage},
		{"Priority", spec.Priority},
	}))

	if spec.Intent != "" {
		fmt.Fprintf(&b, "\n> %s\n\n", spec.Intent)
	}

	// Spark section
	if o := spec.SparkOutput; o != nil {
		if o.Seed != "" {
			b.WriteString(section(2, "Intent", o.Seed))
		}
		if o.Signal != "" || o.KillTest != "" {
			var ctx strings.Builder
			if o.Signal != "" {
				fmt.Fprintf(&ctx, "**Signal:** %s\n\n", o.Signal)
			}
			if o.KillTest != "" {
				fmt.Fprintf(&ctx, "**Kill Test:** %s\n", o.KillTest)
			}
			b.WriteString(section(2, "Context & Signal", ctx.String()))
		}
	}

	// Shape section
	if o := spec.ShapeOutput; o != nil {
		if len(o.ScopeIn) > 0 || len(o.ScopeOut) > 0 {
			var scope strings.Builder
			if len(o.ScopeIn) > 0 {
				scope.WriteString("**In:**\n")
				for _, s := range o.ScopeIn {
					fmt.Fprintf(&scope, "- %s\n", s)
				}
				scope.WriteString("\n")
			}
			if len(o.ScopeOut) > 0 {
				scope.WriteString("**Out:**\n")
				for _, s := range o.ScopeOut {
					fmt.Fprintf(&scope, "- %s\n", s)
				}
			}
			b.WriteString(section(2, "Scope", scope.String()))
		}
		if len(o.Approaches) > 0 {
			var app strings.Builder
			for _, a := range o.Approaches {
				chosen := ""
				if a.Name == o.ChosenApproach {
					chosen = " (chosen)"
				}
				fmt.Fprintf(&app, "### %s%s\n\n", a.Name, chosen)
				if a.Description != "" {
					fmt.Fprintf(&app, "%s\n\n", a.Description)
				}
				for _, t := range a.Tradeoffs {
					fmt.Fprintf(&app, "- %s\n", t)
				}
				app.WriteString("\n")
			}
			b.WriteString(section(2, "Approaches", app.String()))
		}
		if len(o.SuccessMust) > 0 || len(o.SuccessShould) > 0 || len(o.SuccessWont) > 0 {
			var sc strings.Builder
			for _, s := range o.SuccessMust {
				fmt.Fprintf(&sc, "- **MUST:** %s\n", s)
			}
			for _, s := range o.SuccessShould {
				fmt.Fprintf(&sc, "- **SHOULD:** %s\n", s)
			}
			for _, s := range o.SuccessWont {
				fmt.Fprintf(&sc, "- **WON'T:** %s\n", s)
			}
			b.WriteString(section(2, "Success Criteria", sc.String()))
		}
		if len(o.Risks) > 0 {
			var risks strings.Builder
			for _, ri := range o.Risks {
				fmt.Fprintf(&risks, "- %s\n", ri)
			}
			b.WriteString(section(2, "Risks", risks.String()))
		}
	}

	return render.Document{
		Kind:     render.DocumentPRD,
		Title:    fmt.Sprintf("PRD: %s", spec.Slug),
		Body:     []byte(b.String()),
		SpecSlug: spec.Slug,
	}, nil
}
