// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// SparkSection renders the spark stage output as markdown.
func SparkSection(o *specv1.SparkOutput) string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Spark\n\n")

	if o.Seed != "" {
		fmt.Fprintf(&b, "> **Seed:** %s\n\n", o.Seed)
	}
	if o.Signal != "" {
		fmt.Fprintf(&b, "> **Signal:** %s\n\n", o.Signal)
	}
	if scope := scopeSniffString(o.ScopeSniff); scope != "" {
		fmt.Fprintf(&b, "**Scope Sniff:** %s\n", scope)
	}
	if o.KillTest != "" {
		fmt.Fprintf(&b, "**Kill Test:** %s\n", o.KillTest)
	}
	if len(o.Questions) > 0 {
		b.WriteString("\n**Questions:**\n")
		for _, q := range o.Questions {
			fmt.Fprintf(&b, "- %s\n", q)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func scopeSniffString(s specv1.ScopeSniff) string {
	switch s {
	case specv1.ScopeSniff_SCOPE_SNIFF_TINY:
		return "tiny"
	case specv1.ScopeSniff_SCOPE_SNIFF_SMALL:
		return "small"
	case specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM:
		return "medium"
	case specv1.ScopeSniff_SCOPE_SNIFF_LARGE:
		return "large"
	case specv1.ScopeSniff_SCOPE_SNIFF_EPIC:
		return "epic"
	default:
		return ""
	}
}

// ShapeSection renders the shape stage output as markdown.
func ShapeSection(o *specv1.ShapeOutput) string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Shape\n\n")

	if len(o.ScopeIn) > 0 {
		b.WriteString("### Scope In\n\n")
		for _, s := range o.ScopeIn {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	if len(o.ScopeOut) > 0 {
		b.WriteString("### Scope Out\n\n")
		for _, s := range o.ScopeOut {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}
	if len(o.Approaches) > 0 {
		b.WriteString("### Approaches\n\n")
		for _, a := range o.Approaches {
			chosen := ""
			if a.Name == o.ChosenApproach {
				chosen = " (chosen)"
			}
			fmt.Fprintf(&b, "**%s%s**", a.Name, chosen)
			if a.Description != "" {
				fmt.Fprintf(&b, " — %s", a.Description)
			}
			b.WriteString("\n")
			for _, t := range a.Tradeoffs {
				fmt.Fprintf(&b, "- %s\n", t)
			}
			b.WriteString("\n")
		}
	}
	if len(o.Risks) > 0 {
		b.WriteString("### Risks\n\n")
		for _, r := range o.Risks {
			fmt.Fprintf(&b, "- %s\n", r)
		}
		b.WriteString("\n")
	}
	if len(o.SuccessMust) > 0 || len(o.SuccessShould) > 0 || len(o.SuccessWont) > 0 {
		b.WriteString("### Success Criteria\n\n")
		for _, s := range o.SuccessMust {
			fmt.Fprintf(&b, "- **MUST:** %s\n", s)
		}
		for _, s := range o.SuccessShould {
			fmt.Fprintf(&b, "- **SHOULD:** %s\n", s)
		}
		for _, s := range o.SuccessWont {
			fmt.Fprintf(&b, "- **WON'T:** %s\n", s)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// SpecifySection renders the specify stage output as markdown.
func SpecifySection(o *specv1.SpecifyOutput) string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Specify\n\n")

	if len(o.Interfaces) > 0 {
		b.WriteString("### Interfaces\n\n")
		for _, iface := range o.Interfaces {
			fmt.Fprintf(&b, "**%s**\n\n", iface.Name)
			if iface.Body != "" {
				fmt.Fprintf(&b, "%s\n\n", iface.Body)
			}
		}
	}
	if len(o.VerifyCriteria) > 0 {
		b.WriteString("### Verify Criteria\n\n")
		rows := make([][]string, len(o.VerifyCriteria))
		for i, vc := range o.VerifyCriteria {
			rows[i] = []string{vc.Category, vc.Description}
		}
		b.WriteString(itemTable([]string{"Category", "Description"}, rows))
		b.WriteString("\n")
	}
	if len(o.Invariants) > 0 {
		b.WriteString("### Invariants\n\n")
		for _, inv := range o.Invariants {
			fmt.Fprintf(&b, "- %s\n", inv)
		}
		b.WriteString("\n")
	}
	if len(o.Touches) > 0 {
		b.WriteString("### File Touches\n\n")
		rows := make([][]string, len(o.Touches))
		for i, ft := range o.Touches {
			rows[i] = []string{ft.Path, ft.Purpose, ft.ChangeType}
		}
		b.WriteString(itemTable([]string{"Path", "Purpose", "Action"}, rows))
		b.WriteString("\n")
	}
	return b.String()
}

func decompositionStrategyString(s specv1.DecompositionStrategy) string {
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

// DecomposeSection renders the decompose stage output as markdown.
func DecomposeSection(o *specv1.DecomposeOutput) string {
	if o == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Decompose\n\n")

	if strategy := decompositionStrategyString(o.Strategy); strategy != "" {
		fmt.Fprintf(&b, "> **Strategy:** %s\n\n", strategy)
	}
	for _, s := range o.Slices {
		fmt.Fprintf(&b, "### %s\n\n", s.Id)
		if s.Intent != "" {
			fmt.Fprintf(&b, "%s\n\n", s.Intent)
		}
		if len(s.Verify) > 0 {
			b.WriteString("**Verify:**\n")
			for _, v := range s.Verify {
				fmt.Fprintf(&b, "- %s\n", v)
			}
			b.WriteString("\n")
		}
		if len(s.Touches) > 0 {
			b.WriteString("**Touches:**\n")
			for _, t := range s.Touches {
				fmt.Fprintf(&b, "- %s\n", t)
			}
			b.WriteString("\n")
		}
		if len(s.DependsOn) > 0 {
			fmt.Fprintf(&b, "**Depends on:** %s\n\n", strings.Join(s.DependsOn, ", "))
		}
	}
	return b.String()
}
