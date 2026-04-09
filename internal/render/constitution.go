// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// Constitution renders a constitution as markdown.
func Constitution(c *specv1.Constitution) string {
	if c == nil {
		return "No constitution found.\n"
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", c.GetName())

	pairs := [][2]string{
		{"Layer", constitutionLayerString(c.GetLayer())},
		{"Version", fmt.Sprintf("%d", c.GetVersion())},
	}
	if tech := c.GetTech(); tech != nil {
		if langs := tech.GetLanguages(); langs != nil && langs.GetPrimary() != "" {
			pairs = append(pairs, [2]string{"Primary Language", langs.GetPrimary()})
		}
	}
	b.WriteString(metadataTable(pairs))

	if ps := c.GetPrinciples(); len(ps) > 0 {
		b.WriteString("\n## Principles\n\n")
		for _, p := range ps {
			fmt.Fprintf(&b, "- %s\n", p.GetStatement())
		}
	}

	if cs := c.GetConstraints(); len(cs) > 0 {
		b.WriteString("\n## Constraints\n\n")
		for _, ct := range cs {
			fmt.Fprintf(&b, "- %s\n", ct)
		}
	}

	if aps := c.GetAntipatterns(); len(aps) > 0 {
		b.WriteString("\n## Anti-patterns\n\n")
		for _, ap := range aps {
			fmt.Fprintf(&b, "- **%s**: %s\n", ap.GetPattern(), ap.GetWhy())
		}
	}

	if refs := c.GetReferences(); len(refs) > 0 {
		b.WriteString("\n## References\n\n")
		for _, ref := range refs {
			fmt.Fprintf(&b, "- [%s] %s\n", referenceTypeName(ref.GetReferenceType()), ref.GetPath())
		}
	}

	return b.String()
}

func constitutionLayerString(l specv1.ConstitutionLayer) string {
	switch l {
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_USER:
		return "user"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_ORG:
		return "org"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_PROJECT:
		return "project"
	case specv1.ConstitutionLayer_CONSTITUTION_LAYER_DOMAIN:
		return "domain"
	default:
		return "unspecified"
	}
}

func referenceTypeName(rt specv1.ReferenceType) string {
	switch rt {
	case specv1.ReferenceType_REFERENCE_TYPE_ADR:
		return "ADR"
	case specv1.ReferenceType_REFERENCE_TYPE_SPEC:
		return "Spec"
	case specv1.ReferenceType_REFERENCE_TYPE_DOC:
		return "Doc"
	case specv1.ReferenceType_REFERENCE_TYPE_URL:
		return "URL"
	default:
		return "Ref"
	}
}
