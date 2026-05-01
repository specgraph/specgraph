// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package adf

import (
	"context"
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
)

// RenderADR renders a decision as an ADF document in MADR format.
func (r *Renderer) RenderADR(_ context.Context, d *specv1.Decision) (render.Document, error) {
	if d == nil {
		return render.Document{}, fmt.Errorf("decision is nil")
	}

	doc := NewDocument()

	// Title
	doc.Heading(1, fmt.Sprintf("ADR: %s", d.Title))

	// Status with lozenge
	statusStr := decisionStatusName(d.Status)
	doc.Heading(2, "Status")
	doc.ParagraphNodes(StatusMacro(statusStr, DecisionStatusColor(statusStr)))

	// Context (the question being answered)
	if d.Question != "" {
		doc.Heading(2, "Context")
		doc.Paragraph(d.Question)
	}

	// Decision
	if d.Decision != "" {
		doc.Heading(2, "Decision")
		doc.Paragraph(d.Decision)
		if d.Rationale != "" {
			doc.ParagraphNodes(TextNode("Rationale: ", Bold()), TextNode(d.Rationale))
		}
	}

	// Considered Options (MADR format)
	if len(d.RejectedAlternatives) > 0 || d.Decision != "" {
		doc.Heading(2, "Considered Options")
		rows := []Node{Row(HeaderCell("Option"), HeaderCell("Status"), HeaderCell("Reason"))}
		if d.Decision != "" {
			rows = append(rows, Row(Cell(d.Title), Cell("Chosen"), Cell(d.Rationale)))
		}
		for _, ra := range d.RejectedAlternatives {
			rows = append(rows, Row(Cell(ra.Option), Cell("Rejected"), Cell(ra.Reason)))
		}
		doc.Table(rows...)
	}

	// Confidence & Scope
	if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED ||
		d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
		doc.Heading(2, "Confidence & Scope")
		var parts []string
		if d.Confidence != specv1.DecisionConfidence_DECISION_CONFIDENCE_UNSPECIFIED {
			parts = append(parts, fmt.Sprintf("Confidence: %s", strings.TrimPrefix(d.Confidence.String(), "DECISION_CONFIDENCE_")))
		}
		if d.Scope != specv1.DecisionScope_DECISION_SCOPE_UNSPECIFIED {
			parts = append(parts, fmt.Sprintf("Scope: %s", strings.TrimPrefix(d.Scope.String(), "DECISION_SCOPE_")))
		}
		doc.Panel(PanelInfo, strings.Join(parts, " | "))
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:       render.DocumentADR,
		Title:      fmt.Sprintf("ADR: %s", d.Title),
		Body:       b,
		SpecSlug:   d.OriginSpec,
		DecisionID: d.Slug,
	}, nil
}

func decisionStatusName(s specv1.DecisionStatus) string {
	switch s {
	case specv1.DecisionStatus_DECISION_STATUS_PROPOSED:
		return "proposed"
	case specv1.DecisionStatus_DECISION_STATUS_ACCEPTED:
		return "accepted"
	case specv1.DecisionStatus_DECISION_STATUS_DEPRECATED:
		return "deprecated"
	case specv1.DecisionStatus_DECISION_STATUS_SUPERSEDED:
		return "superseded"
	default:
		return "unknown"
	}
}
