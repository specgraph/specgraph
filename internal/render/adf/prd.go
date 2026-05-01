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

// Renderer implements render.Renderer for ADF output.
type Renderer struct{}

// NewRenderer creates an ADF renderer.
func NewRenderer() *Renderer { return &Renderer{} }

// RenderPRD renders a spec's Spark + Shape as an ADF PRD document.
func (r *Renderer) RenderPRD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}

	doc := NewDocument()

	// Title + status lozenge
	doc.Heading(1, fmt.Sprintf("PRD: %s", spec.Slug))
	doc.ParagraphNodes(
		TextNode("Stage: "),
		StatusMacro(spec.Stage, StageColor(spec.Stage)),
	)

	// Page properties
	props := [][2]string{
		{"Slug", spec.Slug},
		{"Stage", spec.Stage},
	}
	if spec.Priority != "" {
		props = append(props, [2]string{"Priority", spec.Priority})
	}
	doc.Raw(PageProperties(props))

	// Intent
	if spec.Intent != "" {
		doc.Blockquote(spec.Intent)
	}

	// Spark sections
	if o := spec.SparkOutput; o != nil {
		if o.Seed != "" {
			doc.Heading(2, "Intent")
			doc.Paragraph(o.Seed)
		}
		if o.Signal != "" || o.KillTest != "" {
			doc.Heading(2, "Context & Signal")
			if o.Signal != "" {
				doc.ParagraphNodes(TextNode("Signal: ", Bold()), TextNode(o.Signal))
			}
			if o.KillTest != "" {
				doc.Panel(PanelWarning, fmt.Sprintf("Kill Test: %s", o.KillTest))
			}
		}
	}

	// Shape sections
	if o := spec.ShapeOutput; o != nil {
		// Scope table
		if len(o.ScopeIn) > 0 || len(o.ScopeOut) > 0 {
			doc.Heading(2, "Scope")
			doc.Table(
				Row(HeaderCell("In"), HeaderCell("Out")),
				Row(
					CellNodes(listParagraph(o.ScopeIn)),
					CellNodes(listParagraph(o.ScopeOut)),
				),
			)
		}

		// Approaches
		if len(o.Approaches) > 0 {
			doc.Heading(2, "Approaches")
			for _, a := range o.Approaches {
				title := a.Name
				if a.Name == o.ChosenApproach {
					title += " (chosen)"
				}
				content := a.Description
				if len(a.Tradeoffs) > 0 {
					content += "\n\nTradeoffs: " + strings.Join(a.Tradeoffs, "; ")
				}
				if a.Name == o.ChosenApproach {
					doc.Heading(3, title)
					doc.Paragraph(content)
				} else {
					doc.Expand(title, content)
				}
			}
		}

		// Success criteria
		if len(o.SuccessMust) > 0 || len(o.SuccessShould) > 0 || len(o.SuccessWont) > 0 {
			doc.Heading(2, "Success Criteria")
			rows := make([]Node, 0)
			rows = append(rows, Row(HeaderCell("Priority"), HeaderCell("Criterion")))
			for _, s := range o.SuccessMust {
				rows = append(rows, Row(Cell("MUST"), Cell(s)))
			}
			for _, s := range o.SuccessShould {
				rows = append(rows, Row(Cell("SHOULD"), Cell(s)))
			}
			for _, s := range o.SuccessWont {
				rows = append(rows, Row(Cell("WON'T"), Cell(s)))
			}
			doc.Table(rows...)
		}

		// Risks
		if len(o.Risks) > 0 {
			doc.Heading(2, "Risks")
			doc.BulletList(o.Risks)
		}
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:     render.DocumentPRD,
		Title:    fmt.Sprintf("PRD: %s", spec.Slug),
		Body:     b,
		SpecSlug: spec.Slug,
	}, nil
}

// listParagraph creates a bullet list node from string items.
// Returns an empty paragraph if items is nil.
func listParagraph(items []string) Node {
	if len(items) == 0 {
		return Node{
			Type:    TypeParagraph,
			Content: []Node{{Type: TypeText, Text: "—"}},
		}
	}
	listItems := make([]Node, len(items))
	for i, item := range items {
		listItems[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{
					Type:    TypeParagraph,
					Content: []Node{{Type: TypeText, Text: item}},
				},
			},
		}
	}
	return Node{
		Type:    TypeBulletList,
		Content: listItems,
	}
}
