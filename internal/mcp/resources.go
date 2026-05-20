// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/specgraph/specgraph/internal/mcp/skills"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/specgraph/specgraph/internal/skillvalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// resourceJSON marshals a proto message to indented JSON and returns it as a
// single-element ResourceContent slice.
func resourceJSON(uri string, msg proto.Message) ([]ResourceContent, error) {
	data, err := protojson.MarshalOptions{Multiline: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal resource %s: %w", uri, err)
	}
	return []ResourceContent{{
		URI:      uri,
		MimeType: "application/json",
		Text:     string(data),
	}}, nil
}

func constitutionEmptyResource(uri string) []ResourceContent {
	return []ResourceContent{{
		URI:      uri,
		MimeType: "text/markdown",
		Text:     "# SpecGraph Constitution\n\n_No constitution configured. Run `specgraph constitution set` to define project ground truth._\n",
	}}
}

func isConstitutionEmptyState(err error) bool {
	if connect.CodeOf(err) == connect.CodeNotFound {
		return true
	}
	return connect.CodeOf(err) == connect.CodeInvalidArgument && strings.Contains(err.Error(), "slug is required")
}

// extractSlugFromURI returns the last path segment of a specgraph:// URI.
// e.g. "specgraph://spec/oauth-refresh" → "oauth-refresh"
func extractSlugFromURI(uri string) string {
	parts := strings.Split(strings.TrimPrefix(uri, "specgraph://"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// extractSkillName parses a specgraph://skills/<name> URI and validates
// the name against skillvalidate.NameRegex. Returns the validated name
// or an error mapped at the call site to connect.CodeNotFound.
//
// Strict by design: rejects subpaths (specgraph://skills/foo/bar),
// trailing slashes, empty names, mixed-case scheme segment, and any
// name failing the kebab-case regex. URL-encoded names are rejected
// (skill names never need encoding by convention).
func extractSkillName(uri string) (string, error) {
	rest := strings.TrimPrefix(uri, "specgraph://")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("malformed skills URI %q", uri)
	}
	if parts[0] != "skills" {
		return "", fmt.Errorf("not a skills URI: %q", uri)
	}
	if !skillvalidate.NameRegex.MatchString(parts[1]) {
		return "", fmt.Errorf("invalid skill name %q in URI", parts[1])
	}
	return parts[1], nil
}

// ---------------------------------------------------------------------------
// specResourceHandler — specgraph://spec/{slug}
// ---------------------------------------------------------------------------

func specResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		slug := extractSlugFromURI(uri)
		resp, err := c.Spec.GetSpec(ctx, connect.NewRequest(&specv1.GetSpecRequest{Slug: slug}))
		if err != nil {
			return nil, fmt.Errorf("get spec %s: %w", slug, err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// specListResourceHandler — specgraph://specs
// ---------------------------------------------------------------------------

func specListResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		if err != nil {
			return nil, fmt.Errorf("list specs: %w", err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// decisionResourceHandler — specgraph://decision/{slug}
// ---------------------------------------------------------------------------

func decisionResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		slug := extractSlugFromURI(uri)
		resp, err := c.Decision.GetDecision(ctx, connect.NewRequest(&specv1.GetDecisionRequest{Slug: slug}))
		if err != nil {
			return nil, fmt.Errorf("get decision %s: %w", slug, err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// constitutionResourceHandler — specgraph://constitution
// ---------------------------------------------------------------------------

func constitutionResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		if err != nil {
			if isConstitutionEmptyState(err) {
				return constitutionEmptyResource(uri), nil
			}
			return nil, fmt.Errorf("get constitution: %w", err)
		}
		if resp.Msg.GetConstitution() == nil {
			return constitutionEmptyResource(uri), nil
		}
		return []ResourceContent{{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     render.Constitution(resp.Msg.GetConstitution()),
		}}, nil
	}
}

// ---------------------------------------------------------------------------
// constitutionLayerResourceHandler — specgraph://constitution/{layer}
// ---------------------------------------------------------------------------

func constitutionLayerResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		layer := extractSlugFromURI(uri)
		enumLayer := constitutionLayerFromString(layer)
		if enumLayer == specv1.ConstitutionLayer_CONSTITUTION_LAYER_UNSPECIFIED {
			return nil, fmt.Errorf("unknown constitution layer %q", layer)
		}
		resp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: enumLayer,
		}))
		if err != nil {
			return nil, fmt.Errorf("get constitution layer %s: %w", layer, err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// graphResourceHandler — specgraph://graph
// ---------------------------------------------------------------------------

func graphResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Graph.GetFullGraph(ctx, connect.NewRequest(&specv1.GetFullGraphRequest{}))
		if err != nil {
			return nil, fmt.Errorf("get full graph: %w", err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// readyResourceHandler — specgraph://graph/ready
// ---------------------------------------------------------------------------

func readyResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		if err != nil {
			return nil, fmt.Errorf("get ready specs: %w", err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// findingsResourceHandler — specgraph://findings
// ---------------------------------------------------------------------------

func findingsResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		resp, err := c.AnalyticalPass.ListProjectFindings(ctx, connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
		if err != nil {
			return nil, fmt.Errorf("list findings: %w", err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// changesResourceHandler — specgraph://spec/{slug}/changes
// ---------------------------------------------------------------------------

func changesResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		// URI pattern: specgraph://spec/{slug}/changes
		// Split path segments after stripping the scheme.
		parts := strings.Split(strings.TrimPrefix(uri, "specgraph://"), "/")
		// parts: ["spec", "{slug}", "changes"] → slug is parts[1]
		slug := ""
		if len(parts) >= 2 {
			slug = parts[1]
		}
		resp, err := c.Spec.ListChanges(ctx, connect.NewRequest(&specv1.ListChangesRequest{Slug: slug}))
		if err != nil {
			return nil, fmt.Errorf("list changes for %s: %w", slug, err)
		}
		return resourceJSON(uri, resp.Msg)
	}
}

// ---------------------------------------------------------------------------
// primeResourceHandler — specgraph://prime
// ---------------------------------------------------------------------------

// primeResourceHandler returns a session-priming digest in markdown. It
// stitches together constitution summary, spec counts by stage, the top 10
// ready specs, and open-findings counts by severity. Each section renders
// its heading unconditionally; RPC failures log a warning and render a
// visible "_(unable to load: ...)_" marker under the heading so partial
// connectivity is observable rather than silently degrading the digest.
func primeResourceHandler(c *Client, src skills.Source) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		var b strings.Builder
		b.WriteString("# SpecGraph Session Prime\n\n")

		conResp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{}))
		switch {
		case err != nil && connect.CodeOf(err) == connect.CodeNotFound:
			// Expected empty state on fresh projects: no constitution defined.
			// Render a heading + hint so the agent knows the slot exists and
			// how to populate it, rather than treating it as an RPC failure.
			b.WriteString("## Constitution\n\n_No constitution configured. Run `specgraph constitution set` to define project ground truth._\n\n")
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "constitution"),
				slog.String("err", err.Error()))
			b.WriteString("## Constitution\n\n_(unable to load: " + err.Error() + ")_\n\n")
		case conResp.Msg.GetConstitution() == nil:
			// No constitution configured — skip section silently. Distinct from RPC failure.
		default:
			con := conResp.Msg.GetConstitution()
			b.WriteString("## Constitution\n\n")
			if con.GetTech() != nil && con.GetTech().GetLanguages() != nil {
				fmt.Fprintf(&b, "Primary language: %s\n\n", con.GetTech().GetLanguages().GetPrimary())
			}
			if cs := con.GetConstraints(); len(cs) > 0 {
				top := cs
				if len(top) > 5 {
					top = top[:5]
				}
				b.WriteString("Top constraints:\n")
				for _, constraint := range top {
					fmt.Fprintf(&b, "- %s\n", constraint)
				}
				b.WriteString("\nFull at `specgraph://constitution`.\n\n")
			}
		}

		listResp, err := c.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{}))
		switch {
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "graph_overview"),
				slog.String("err", err.Error()))
			b.WriteString("## Graph Overview\n\n_(unable to load: " + err.Error() + ")_\n\n")
		case len(listResp.Msg.GetSpecs()) > 0:
			b.WriteString("## Graph Overview\n\n")
			counts := map[string]int{}
			for _, s := range listResp.Msg.GetSpecs() {
				counts[s.GetStage()]++
			}
			// Render in funnel order, then any unexpected stages alphabetically.
			for _, stage := range authoring.AllStages() {
				if n, ok := counts[stage]; ok {
					fmt.Fprintf(&b, "- %s: %d\n", stage, n)
					delete(counts, stage)
				}
			}
			leftover := make([]string, 0, len(counts))
			for stage := range counts {
				leftover = append(leftover, stage)
			}
			sort.Strings(leftover)
			for _, stage := range leftover {
				fmt.Fprintf(&b, "- %s: %d\n", stage, counts[stage])
			}
			b.WriteString("\n")
		}

		readyResp, err := c.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{}))
		switch {
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "ready_to_work"),
				slog.String("err", err.Error()))
			b.WriteString("## Ready to Work\n\n_(unable to load: " + err.Error() + ")_\n\n")
		default:
			ready := readyResp.Msg.GetReady()
			if len(ready) > 0 {
				b.WriteString("## Ready to Work\n\n")
				if len(ready) > 10 {
					ready = ready[:10]
				}
				for _, s := range ready {
					fmt.Fprintf(&b, "- `%s` (%s)\n", s.GetSlug(), s.GetStage())
				}
				b.WriteString("\nFull list at `specgraph://graph/ready`.\n\n")
			}
		}

		findingsResp, err := c.AnalyticalPass.ListProjectFindings(ctx, connect.NewRequest(&specv1.ListProjectFindingsRequest{}))
		switch {
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "open_findings"),
				slog.String("err", err.Error()))
			b.WriteString("## Open Findings\n\n_(unable to load: " + err.Error() + ")_\n\n")
		default:
			counts := map[specv1.FindingSeverity]int{}
			for _, f := range findingsResp.Msg.GetFindings() {
				counts[f.GetSeverity()]++
			}
			if len(counts) > 0 {
				b.WriteString("## Open Findings\n\n")
				sevs := make([]specv1.FindingSeverity, 0, len(counts))
				for sev := range counts {
					sevs = append(sevs, sev)
				}
				sort.Slice(sevs, func(i, j int) bool {
					return severityRank(sevs[i]) < severityRank(sevs[j])
				})
				for _, sev := range sevs {
					fmt.Fprintf(&b, "- %s: %d\n", sev.String(), counts[sev])
				}
				b.WriteString("\nFull at `specgraph://findings`.\n\n")
			}
		}

		metas, err := src.List(ctx)
		switch {
		case err != nil:
			slog.WarnContext(ctx, "prime.section_failed",
				slog.String("section", "skills"),
				slog.String("err", err.Error()))
			b.WriteString("## Skills\n\n_(unable to load: " + err.Error() + ")_\n\n")
		case len(metas) > 0:
			fmt.Fprintf(&b, "## Skills\n\n%d skills exposed via MCP. ", len(metas))
			b.WriteString("Use `specgraph_skills_list` to see the catalog, ")
			b.WriteString("`specgraph_skills_search` to find one by keyword, ")
			b.WriteString("and `specgraph_skills_get` / `specgraph://skills/<name>` ")
			b.WriteString("to fetch a specific skill.\n\n")
		}

		return []ResourceContent{{URI: uri, MimeType: "text/markdown", Text: b.String()}}, nil
	}
}

// severityRank gives a display order for finding severities: critical first,
// warning second, note third, anything else last. Lower rank renders earlier.
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

// ---------------------------------------------------------------------------
// skillsResourceHandler — specgraph://skills/{name}
// ---------------------------------------------------------------------------

// skillsResourceHandler returns the verbatim SKILL.md bytes for a named
// skill. The URI is parsed through extractSkillName (strict — see helper
// docs). Unknown names and malformed URIs map to connect.CodeNotFound.
func skillsResourceHandler(src skills.Source) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		name, err := extractSkillName(uri)
		if err != nil {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		sk, err := src.Get(ctx, name)
		if err != nil {
			if errors.Is(err, skills.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, fmt.Errorf("get skill %s: %w", name, err)
		}
		return []ResourceContent{{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     string(sk.Body),
		}}, nil
	}
}

// ---------------------------------------------------------------------------
// RegisterResources adds all 11 MCP resources to the registry.
// ---------------------------------------------------------------------------

// RegisterResources registers all SpecGraph MCP resources into r using the
// provided Client for RPC calls and skills.Source for the
// specgraph://skills/{name} handler. Resources are read-only context
// providers for MCP clients.
func RegisterResources(r *Registry, c *Client, src skills.Source) {
	r.AddResource(ResourceDef{
		URI:         "specgraph://spec/{slug}",
		Name:        "spec",
		Description: "Full spec content by slug.",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     specResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://specs",
		Name:        "specs",
		Description: "Summary list of all specs.",
		MimeType:    "application/json",
		IsTemplate:  false,
		Handler:     specListResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://decision/{slug}",
		Name:        "decision",
		Description: "Decision content by slug.",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     decisionResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://constitution",
		Name:        "constitution",
		Description: "Merged constitution (all layers).",
		MimeType:    "text/markdown",
		IsTemplate:  false,
		Handler:     constitutionResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://constitution/{layer}",
		Name:        "constitution-layer",
		Description: "Single constitution layer (user, org, project, domain).",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     constitutionLayerResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://graph",
		Name:        "graph",
		Description: "Complete dependency graph (all nodes and edges).",
		MimeType:    "application/json",
		IsTemplate:  false,
		Handler:     graphResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://graph/ready",
		Name:        "graph-ready",
		Description: "Specs with no unmet dependencies (ready to execute).",
		MimeType:    "application/json",
		IsTemplate:  false,
		Handler:     readyResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://findings",
		Name:        "findings",
		Description: "All analytical findings (global, no slug filter).",
		MimeType:    "application/json",
		IsTemplate:  false,
		Handler:     findingsResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://spec/{slug}/changes",
		Name:        "spec-changes",
		Description: "Version history (change log) for a spec.",
		MimeType:    "application/json",
		IsTemplate:  true,
		Handler:     changesResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://prime",
		Name:        "prime",
		Description: "Session-priming digest: constitution summary, graph counts, ready specs, findings summary.",
		MimeType:    "text/markdown",
		IsTemplate:  false,
		Handler:     primeResourceHandler(c, src),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://skills/{name}",
		Name:        "skill",
		Description: "A single SpecGraph SKILL.md package by name (verbatim markdown bytes).",
		MimeType:    "text/markdown",
		IsTemplate:  true,
		Handler:     skillsResourceHandler(src),
	})
}
