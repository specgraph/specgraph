// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
)

// RenderOpts controls optional decoration of rendered output.
//
//nolint:revive // Public name spelled `RenderOpts` per the spgr-8ar Piece E task contract.
type RenderOpts struct {
	// ShowProvenance, when true, annotates constitution fields with
	// "(set by: <layer>)" markers using the view's ConstitutionProvenance.
	// It also controls whether the JSON variants include the
	// constitution_provenance field.
	ShowProvenance bool
}

// RenderProjectMarkdown returns the markdown digest for a project-scope
// PrimeResponse view. The layout matches the legacy `specgraph://prime`
// resource output byte-for-byte when provenance is disabled — that
// invariant is verified by TestRenderProjectMarkdown_NoProvenance_MatchesExistingLayout.
//
//nolint:revive // Public name per the spgr-8ar Piece E task contract.
func RenderProjectMarkdown(v *specv1.ProjectView, opts RenderOpts) string {
	var b strings.Builder
	b.WriteString("# SpecGraph Session Prime\n\n")
	if v == nil {
		return b.String()
	}

	writeProjectConstitution(&b, v, opts)
	writeGraphOverview(&b, v.GetGraphOverview())
	writeReady(&b, v.GetReady())
	writeFindings(&b, v.GetFindingsBySeverity())
	writeSkills(&b, v.GetSkillsCount())

	return b.String()
}

// RenderSpecMarkdown returns the markdown digest for a spec-scope
// PrimeResponse view.
//
//nolint:revive // Public name per the spgr-8ar Piece E task contract.
func RenderSpecMarkdown(v *specv1.SpecView, opts RenderOpts) string {
	var b strings.Builder
	if v == nil {
		return ""
	}

	spec := v.GetSpec()
	if spec != nil {
		fmt.Fprintf(&b, "# Prime: %s\n\n", spec.GetSlug())
		b.WriteString("## Spec\n\n")
		// Spec messages have no dedicated title field; we surface stage,
		// priority, and a clipped intent. The heading already shows the slug.
		if s := spec.GetStage(); s != "" {
			fmt.Fprintf(&b, "Stage: %s\n", s)
		}
		if p := spec.GetPriority(); p != "" {
			fmt.Fprintf(&b, "Priority: %s\n", p)
		}
		if intent := spec.GetIntent(); intent != "" {
			fmt.Fprintf(&b, "\nIntent: %s\n", truncate(intent, 400))
		}
		b.WriteString("\n")
	}

	writeSpecConstitution(&b, v, opts)

	if decisions := v.GetDecisions(); len(decisions) > 0 {
		b.WriteString("## Decisions\n\n")
		for _, d := range decisions {
			fmt.Fprintf(&b, "- [%s] %s\n", d.GetSlug(), d.GetTitle())
		}
		b.WriteString("\n")
	}

	if slices := v.GetSlices(); len(slices) > 0 {
		b.WriteString("## Slices\n\n")
		for _, s := range slices {
			intent := truncate(s.GetIntent(), 200)
			if intent == "" {
				fmt.Fprintf(&b, "- `%s`\n", s.GetSlug())
			} else {
				fmt.Fprintf(&b, "- `%s` — %s\n", s.GetSlug(), intent)
			}
		}
		b.WriteString("\n")
	}

	if claims := v.GetClaims(); len(claims) > 0 {
		b.WriteString("## Claims\n\n")
		for _, c := range claims {
			agent := c.GetAgent()
			if exp := c.GetLeaseExpires(); exp != nil {
				fmt.Fprintf(&b, "Active claim: %s (expires %s)\n", agent, exp.AsTime().UTC().Format("2006-01-02T15:04:05Z"))
			} else {
				fmt.Fprintf(&b, "Active claim: %s\n", agent)
			}
		}
		b.WriteString("\n")
	}

	if blockers := v.GetBlockers(); len(blockers) > 0 {
		b.WriteString("## Blockers\n\n")
		for _, ev := range blockers {
			ts := ""
			if t := ev.GetCreatedAt(); t != nil {
				ts = t.AsTime().UTC().Format("2006-01-02T15:04:05Z")
			}
			reporter := ev.GetAgent()
			switch {
			case reporter != "" && ts != "":
				fmt.Fprintf(&b, "- %s (reported by %s at %s)\n", ev.GetMessage(), reporter, ts)
			case reporter != "":
				fmt.Fprintf(&b, "- %s (reported by %s)\n", ev.GetMessage(), reporter)
			case ts != "":
				fmt.Fprintf(&b, "- %s (at %s)\n", ev.GetMessage(), ts)
			default:
				fmt.Fprintf(&b, "- %s\n", ev.GetMessage())
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

// RenderProjectJSON returns the JSON serialization of a ProjectView using
// protojson. When opts.ShowProvenance is false, the constitution_provenance
// field is omitted (invariant: callers that don't ask for provenance see
// no key change in the output).
//
//nolint:revive // Public name per the spgr-8ar Piece E task contract.
func RenderProjectJSON(v *specv1.ProjectView, opts RenderOpts) ([]byte, error) {
	if v == nil {
		return marshalProtoJSON(&specv1.ProjectView{})
	}
	if opts.ShowProvenance {
		return marshalProtoJSON(v)
	}
	clone, ok := proto.Clone(v).(*specv1.ProjectView)
	if !ok {
		// Defensive: should be impossible because proto.Clone preserves type.
		return nil, fmt.Errorf("clone ProjectView: unexpected type %T", proto.Clone(v))
	}
	clone.ConstitutionProvenance = nil
	return marshalProtoJSON(clone)
}

// RenderSpecJSON returns the JSON serialization of a SpecView using
// protojson. When opts.ShowProvenance is false, the constitution_provenance
// field is omitted.
//
//nolint:revive // Public name per the spgr-8ar Piece E task contract.
func RenderSpecJSON(v *specv1.SpecView, opts RenderOpts) ([]byte, error) {
	if v == nil {
		return marshalProtoJSON(&specv1.SpecView{})
	}
	if opts.ShowProvenance {
		return marshalProtoJSON(v)
	}
	clone, ok := proto.Clone(v).(*specv1.SpecView)
	if !ok {
		return nil, fmt.Errorf("clone SpecView: unexpected type %T", proto.Clone(v))
	}
	clone.ConstitutionProvenance = nil
	return marshalProtoJSON(clone)
}

// ---------------------------------------------------------------------------
// Section writers — ProjectView
// ---------------------------------------------------------------------------

// writeProjectConstitution renders the Constitution section of a ProjectView
// in the legacy `specgraph://prime` layout. The output is byte-equal to the
// old primeResourceHandler when provenance is disabled and the inputs are
// equivalent (no RPC failures — that path lives in the resource handler,
// not here).
func writeProjectConstitution(b *strings.Builder, v *specv1.ProjectView, opts RenderOpts) {
	con := v.GetConstitution()
	if con == nil {
		// Empty-state hint — matches today's primeResourceHandler exactly.
		b.WriteString("## Constitution\n\n_No constitution configured. Run `specgraph constitution set` to define project ground truth._\n\n")
		return
	}

	b.WriteString("## Constitution\n\n")
	prov := provenanceLookup(v.GetConstitutionProvenance(), opts.ShowProvenance)

	if con.GetTech() != nil && con.GetTech().GetLanguages() != nil {
		primary := con.GetTech().GetLanguages().GetPrimary()
		if primary != "" {
			line := fmt.Sprintf("Primary language: %s", primary)
			if layer := prov["tech_config.languages.primary"]; layer != "" {
				line += " (set by: " + layer + ")"
			}
			fmt.Fprintf(b, "%s\n\n", line)
		}
	}

	if cs := con.GetConstraints(); len(cs) > 0 {
		top := cs
		if len(top) > 5 {
			top = top[:5]
		}
		b.WriteString("Top constraints:\n")
		for _, constraint := range top {
			line := "- " + constraint
			if layer := prov["constraints["+constraint+"]"]; layer != "" {
				line += "  (set by: " + layer + ")"
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\nFull at `specgraph://constitution`.\n\n")
	}
}

func writeGraphOverview(b *strings.Builder, g *specv1.GraphOverview) {
	counts := g.GetCountsByStage()
	if len(counts) == 0 {
		return
	}
	b.WriteString("## Graph Overview\n\n")
	// Render in funnel order, then any unexpected stages alphabetically.
	remaining := make(map[string]int32, len(counts))
	for k, v := range counts {
		remaining[k] = v
	}
	for _, stage := range authoring.AllStages() {
		if n, ok := remaining[stage]; ok {
			fmt.Fprintf(b, "- %s: %d\n", stage, n)
			delete(remaining, stage)
		}
	}
	leftover := make([]string, 0, len(remaining))
	for stage := range remaining {
		leftover = append(leftover, stage)
	}
	sort.Strings(leftover)
	for _, stage := range leftover {
		fmt.Fprintf(b, "- %s: %d\n", stage, remaining[stage])
	}
	b.WriteString("\n")
}

func writeReady(b *strings.Builder, ready []*specv1.Spec) {
	if len(ready) == 0 {
		return
	}
	b.WriteString("## Ready to Work\n\n")
	for _, s := range ready {
		fmt.Fprintf(b, "- `%s` (%s)\n", s.GetSlug(), s.GetStage())
	}
	b.WriteString("\nFull list at `specgraph://graph/ready`.\n\n")
}

func writeFindings(b *strings.Builder, bySev map[int32]int32) {
	if len(bySev) == 0 {
		return
	}
	b.WriteString("## Open Findings\n\n")
	sevs := make([]specv1.FindingSeverity, 0, len(bySev))
	for raw := range bySev {
		sevs = append(sevs, specv1.FindingSeverity(raw))
	}
	sort.Slice(sevs, func(i, j int) bool {
		return severityRank(sevs[i]) < severityRank(sevs[j])
	})
	for _, sev := range sevs {
		fmt.Fprintf(b, "- %s: %d\n", sev.String(), bySev[int32(sev)])
	}
	b.WriteString("\nFull at `specgraph://findings`.\n\n")
}

func writeSkills(b *strings.Builder, count int32) {
	if count <= 0 {
		return
	}
	fmt.Fprintf(b, "## Skills\n\n%d skills exposed via MCP. ", count)
	b.WriteString("Use `specgraph_skills_list` to see the catalog, ")
	b.WriteString("`specgraph_skills_search` to find one by keyword, ")
	b.WriteString("and `specgraph_skills_get` / `specgraph://skills/<name>` ")
	b.WriteString("to fetch a specific skill.\n\n")
}

// ---------------------------------------------------------------------------
// Section writers — SpecView
// ---------------------------------------------------------------------------

// writeSpecConstitution renders the Constitution section for a SpecView,
// following the same layout as the project-scope writer.
func writeSpecConstitution(b *strings.Builder, v *specv1.SpecView, opts RenderOpts) {
	con := v.GetConstitution()
	if con == nil {
		b.WriteString("## Constitution\n\n_No constitution configured. Run `specgraph constitution set` to define project ground truth._\n\n")
		return
	}
	b.WriteString("## Constitution\n\n")
	prov := provenanceLookup(v.GetConstitutionProvenance(), opts.ShowProvenance)

	if con.GetTech() != nil && con.GetTech().GetLanguages() != nil {
		primary := con.GetTech().GetLanguages().GetPrimary()
		if primary != "" {
			line := fmt.Sprintf("Primary language: %s", primary)
			if layer := prov["tech_config.languages.primary"]; layer != "" {
				line += " (set by: " + layer + ")"
			}
			fmt.Fprintf(b, "%s\n\n", line)
		}
	}

	if cs := con.GetConstraints(); len(cs) > 0 {
		top := cs
		if len(top) > 5 {
			top = top[:5]
		}
		b.WriteString("Top constraints:\n")
		for _, constraint := range top {
			line := "- " + constraint
			if layer := prov["constraints["+constraint+"]"]; layer != "" {
				line += "  (set by: " + layer + ")"
			}
			b.WriteString(line + "\n")
		}
		b.WriteString("\nFull at `specgraph://constitution`.\n\n")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// provenanceLookup builds a path → layer-string map from a slice of
// ProvenanceEntry values. Returns an empty (non-nil) map when show is false
// so callers can safely index without nil-checks.
func provenanceLookup(entries []*specv1.ProvenanceEntry, show bool) map[string]string {
	out := map[string]string{}
	if !show {
		return out
	}
	for _, e := range entries {
		if e == nil {
			continue
		}
		out[e.GetPath()] = constitutionLayerString(e.GetLayer())
	}
	return out
}

// severityRank gives a display order for finding severities: critical first,
// warning second, note third, anything else last.
func severityRank(s specv1.FindingSeverity) int {
	switch s {
	case specv1.FindingSeverity_FINDING_SEVERITY_CRITICAL:
		return 0
	case specv1.FindingSeverity_FINDING_SEVERITY_WARNING:
		return 1
	case specv1.FindingSeverity_FINDING_SEVERITY_NOTE:
		return 2
	default:
		return 99
	}
}

// marshalProtoJSON wraps protojson.Marshal with the project's standard
// Multiline option for consistency with cmd/specgraph/output.go.
func marshalProtoJSON(m proto.Message) ([]byte, error) {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return data, nil
}
