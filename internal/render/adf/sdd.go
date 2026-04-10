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

// RenderSDD renders a spec's Specify + Decompose as an ADF SDD document.
func (r *Renderer) RenderSDD(_ context.Context, spec *specv1.Spec) (render.Document, error) {
	if spec == nil {
		return render.Document{}, fmt.Errorf("spec is nil")
	}

	doc := NewDocument()
	doc.Heading(1, fmt.Sprintf("SDD: %s", spec.Slug))

	// Specify sections
	if o := spec.SpecifyOutput; o != nil {
		if len(o.Interfaces) > 0 {
			doc.Heading(2, "Interface Contracts")
			for _, iface := range o.Interfaces {
				doc.Heading(3, iface.Name)
				if iface.Body != "" {
					doc.CodeBlock("", iface.Body)
				}
			}
		}
		if len(o.VerifyCriteria) > 0 {
			doc.Heading(2, "Acceptance Criteria")
			rows := []Node{Row(HeaderCell("Category"), HeaderCell("Description"))}
			for _, vc := range o.VerifyCriteria {
				rows = append(rows, Row(Cell(vc.Category), Cell(vc.Description)))
			}
			doc.Table(rows...)
		}
		if len(o.Invariants) > 0 {
			doc.Heading(2, "Invariants")
			doc.PanelNodes(PanelWarning, Node{
				Type:    TypeBulletList,
				Content: textListItems(o.Invariants),
			})
		}
		if len(o.Touches) > 0 {
			doc.Heading(2, "File Touches")
			rows := []Node{Row(HeaderCell("Path"), HeaderCell("Purpose"), HeaderCell("Action"))}
			for _, ft := range o.Touches {
				rows = append(rows, Row(Cell(ft.Path), Cell(ft.Purpose), Cell(ft.ChangeType)))
			}
			doc.Table(rows...)
		}
	}

	// Decompose sections
	if o := spec.DecomposeOutput; o != nil {
		if strategy := decompositionStrategyName(o.Strategy); strategy != "" {
			doc.Heading(2, "Decomposition Strategy")
			doc.Panel(PanelInfo, strategy)
		}
		if len(o.Slices) > 0 {
			doc.Heading(2, "Slices")
			rows := []Node{Row(HeaderCell("ID"), HeaderCell("Intent"), HeaderCell("Verify"), HeaderCell("Depends On"))}
			for _, s := range o.Slices {
				rows = append(rows, Row(
					Cell(s.Id),
					Cell(s.Intent),
					Cell(strings.Join(s.Verify, "; ")),
					Cell(strings.Join(s.DependsOn, ", ")),
				))
			}
			doc.Table(rows...)
		}
	}

	b, err := doc.JSON()
	if err != nil {
		return render.Document{}, fmt.Errorf("marshal ADF: %w", err)
	}
	return render.Document{
		Kind:     render.DocumentSDD,
		Title:    fmt.Sprintf("SDD: %s", spec.Slug),
		Body:     b,
		SpecSlug: spec.Slug,
	}, nil
}

// textListItems creates ADF list item nodes from strings.
func textListItems(items []string) []Node {
	nodes := make([]Node, len(items))
	for i, item := range items {
		nodes[i] = Node{
			Type: TypeListItem,
			Content: []Node{
				{Type: TypeParagraph, Content: []Node{{Type: TypeText, Text: item}}},
			},
		}
	}
	return nodes
}

func decompositionStrategyName(s specv1.DecompositionStrategy) string {
	switch s {
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE:
		return "vertical_slice"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE:
		return "layer_cake"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT:
		return "single_unit"
	case specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_STEEL_THREAD:
		return "steel_thread"
	default:
		return ""
	}
}
