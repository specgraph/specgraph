// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
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
		resp, err := c.Constitution.GetConstitution(ctx, connect.NewRequest(&specv1.GetConstitutionRequest{
			Layer: constitutionLayerFromString(layer),
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
// RegisterResources adds all 9 MCP resources to the registry.
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
}
