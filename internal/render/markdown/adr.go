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

// RenderADR renders a decision as an ADR in MADR format.
func (r *Renderer) RenderADR(_ context.Context, d *specv1.Decision) (render.Document, error) {
	if d == nil {
		return render.Document{}, fmt.Errorf("decision is nil")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# ADR: %s\n\n", d.Title)

	// Status
	b.WriteString(section(2, "Status", decisionStatusString(d.Status)))

	// Context
	if d.Question != "" {
		b.WriteString(section(2, "Context", d.Question))
	}

	// Decision
	if d.Decision != "" {
		body := d.Decision
		if d.Rationale != "" {
			body += fmt.Sprintf("\n\n**Rationale:** %s", d.Rationale)
		}
		b.WriteString(section(2, "Decision", body))
	}

	// Considered Options (MADR extension)
	if len(d.RejectedAlternatives) > 0 {
		rows := make([][]string, 0, len(d.RejectedAlternatives)+1)
		if d.Decision != "" {
			rows = append(rows, []string{d.Title, "Chosen", d.Rationale})
		}
		for _, ra := range d.RejectedAlternatives {
			rows = append(rows, []string{ra.Option, "Rejected", ra.Reason})
		}
		b.WriteString(section(2, "Considered Options", ItemTable([]string{"Option", "Status", "Reason"}, rows)))
	}

	// Confidence (MADR extension)
	if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED {
		conf := decisionConfidenceName(d.Confidence)
		detail := fmt.Sprintf("**Confidence:** %s", conf)
		if d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
			detail += fmt.Sprintf("\n**Scope:** %s", decisionScopeName(d.Scope))
		}
		b.WriteString(section(2, "Confidence & Scope", detail))
	}

	return render.Document{
		Kind:       render.DocumentADR,
		Title:      fmt.Sprintf("ADR: %s", d.Title),
		Body:       []byte(b.String()),
		SpecSlug:   d.OriginSpec,
		DecisionID: d.Slug,
	}, nil
}
