// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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

// extractSlugFromURI returns the last path segment of a specgraph:// URI.
// e.g. "specgraph://spec/oauth-refresh" → "oauth-refresh"
func extractSlugFromURI(uri string) string {
	parts := strings.Split(strings.TrimPrefix(uri, "specgraph://"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
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
			return nil, fmt.Errorf("get constitution: %w", err)
		}
		return resourceJSON(uri, resp.Msg)
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
		resp, err := c.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{}))
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
// ready specs, and open-findings counts by severity. Each section is
// conditional: if the upstream RPC fails, that section is silently omitted so
// partial connectivity still produces a useful digest.
func primeResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		var b strings.Builder
		b.WriteString("# SpecGraph Session Prime\n\n")

		if conResp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{})); err == nil && conResp.Msg.GetConstitution() != nil {
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

		if listResp, err := c.Spec.ListSpecs(ctx, connect.NewRequest(&specv1.ListSpecsRequest{})); err == nil {
			b.WriteString("## Graph Overview\n\n")
			counts := map[string]int{}
			for _, s := range listResp.Msg.GetSpecs() {
				counts[s.GetStage()]++
			}
			stages := make([]string, 0, len(counts))
			for stage := range counts {
				stages = append(stages, stage)
			}
			sort.Strings(stages)
			for _, stage := range stages {
				fmt.Fprintf(&b, "- %s: %d\n", stage, counts[stage])
			}
			b.WriteString("\n")
		}

		if readyResp, err := c.Graph.GetReady(ctx, connect.NewRequest(&specv1.GetReadyRequest{})); err == nil {
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

		if findingsResp, err := c.AnalyticalPass.ListFindings(ctx, connect.NewRequest(&specv1.ListFindingsRequest{})); err == nil {
			counts := map[string]int{}
			for _, f := range findingsResp.Msg.GetFindings() {
				counts[f.GetSeverity().String()]++
			}
			if len(counts) > 0 {
				b.WriteString("## Open Findings\n\n")
				sevs := make([]string, 0, len(counts))
				for sev := range counts {
					sevs = append(sevs, sev)
				}
				sort.Strings(sevs)
				for _, sev := range sevs {
					fmt.Fprintf(&b, "- %s: %d\n", sev, counts[sev])
				}
				b.WriteString("\nFull at `specgraph://findings`.\n\n")
			}
		}

		return []ResourceContent{{URI: uri, MimeType: "text/markdown", Text: b.String()}}, nil
	}
}

// ---------------------------------------------------------------------------
// RegisterResources adds all 10 MCP resources to the registry.
// ---------------------------------------------------------------------------

// RegisterResources registers all SpecGraph MCP resources into r using the
// provided Client to make RPC calls. Resources are read-only context providers
// for MCP clients.
func RegisterResources(r *Registry, c *Client) {
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
		MimeType:    "application/json",
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
		Handler:     primeResourceHandler(c),
	})
}
