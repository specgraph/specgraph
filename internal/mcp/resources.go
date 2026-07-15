// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
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
		Text:     "# SpecGraph Constitution\n\n" + render.ConstitutionEmptyHint + "\n",
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

// errMalformedSkillURI is returned by extractSkillName when the URI is
// structurally invalid (wrong segment count, wrong scheme, or name that
// fails skillvalidate.NameRegex). Distinguished from skills.ErrNotFound
// (which means "URI was well-formed but the name isn't in the catalog")
// so the resource handler can map malformed URIs to
// connect.CodeInvalidArgument and unknown names to connect.CodeNotFound.
var errMalformedSkillURI = errors.New("malformed skill URI")

// extractSkillName parses a specgraph://skills/<name> URI and validates
// the name against skillvalidate.NameRegex. Returns the validated name
// or errMalformedSkillURI wrapped with context. Mapped at the call site
// to connect.CodeInvalidArgument.
//
// Strict by design: rejects subpaths (specgraph://skills/foo/bar),
// trailing slashes, empty names, mixed-case scheme segment, and any
// name failing the kebab-case regex. URL-encoded names are rejected
// (skill names never need encoding by convention).
func extractSkillName(uri string) (string, error) {
	rest := strings.TrimPrefix(uri, "specgraph://")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("%w: %q has wrong segment count", errMalformedSkillURI, uri)
	}
	if parts[0] != "skills" {
		return "", fmt.Errorf("%w: %q is not a skills URI", errMalformedSkillURI, uri)
	}
	if !skillvalidate.NameRegex.MatchString(parts[1]) {
		return "", fmt.Errorf("%w: name %q in %q is not kebab-case ASCII", errMalformedSkillURI, parts[1], uri)
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
// specSpecificPrimeResourceHandler — specgraph://prime/spec/{slug}
// ---------------------------------------------------------------------------

// primeResourceHandler returns a session-priming digest by calling
// ExecutionService.GetPrime with an empty slug (project scope) and
// rendering the returned ProjectView via internal/render. Composition
// lives server-side in the prime.Composer; this handler is a thin
// proto-to-markdown (or proto-to-JSON) translator.
//
// Query params:
//
//	?provenance=true — annotate constitution fields with "(set by: <layer>)"
//	?format=json     — return application/json instead of markdown
func primeResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		opts, format, err := parsePrimeURI(uri)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		resp, err := c.Execution.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{Slug: ""}))
		if err != nil {
			return nil, fmt.Errorf("get prime: %w", err)
		}
		pview := resp.Msg.GetProjectView()
		if pview == nil {
			return nil, fmt.Errorf("prime response missing project view")
		}
		return renderPrimeResource(uri, pview, nil, opts, format)
	}
}

// specSpecificPrimeResourceHandler returns a spec-scoped priming digest by
// calling ExecutionService.GetPrime with the slug parsed from the URI and
// rendering the returned SpecView via internal/render.
//
// Same query params as the project-scope handler.
func specSpecificPrimeResourceHandler(c *Client) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		slug, opts, format, err := parsePrimeSpecURI(uri)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		resp, err := c.Execution.GetPrime(ctx, connect.NewRequest(&specv1.GetPrimeRequest{Slug: slug}))
		if err != nil {
			return nil, fmt.Errorf("get prime: %w", err)
		}
		sview := resp.Msg.GetSpecView()
		if sview == nil {
			return nil, fmt.Errorf("prime response missing spec view for slug %q", slug)
		}
		return renderPrimeResource(uri, nil, sview, opts, format)
	}
}

// parsePrimeURI parses "specgraph://prime" optionally followed by
// "?provenance=true&format=json". Returns the parsed render options
// and format ("json" or "markdown") on success.
func parsePrimeURI(uri string) (render.RenderOpts, string, error) {
	rest := strings.TrimPrefix(uri, "specgraph://prime")
	if rest != "" && !strings.HasPrefix(rest, "?") {
		return render.RenderOpts{}, "", fmt.Errorf("malformed prime URI %q", uri)
	}
	return parsePrimeQuery(strings.TrimPrefix(rest, "?"))
}

// parsePrimeSpecURI parses "specgraph://prime/spec/{slug}" optionally
// followed by a query string. Returns the slug, render options, and
// format.
func parsePrimeSpecURI(uri string) (slug string, opts render.RenderOpts, format string, err error) {
	const prefix = "specgraph://prime/spec/"
	if !strings.HasPrefix(uri, prefix) {
		return "", render.RenderOpts{}, "", fmt.Errorf("malformed prime spec URI %q", uri)
	}
	rest := strings.TrimPrefix(uri, prefix)
	slug = rest
	query := ""
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		slug = rest[:i]
		query = rest[i+1:]
	}
	if slug == "" {
		return "", render.RenderOpts{}, "", fmt.Errorf("malformed prime spec URI %q: empty slug", uri)
	}
	if strings.ContainsAny(slug, "/") {
		return "", render.RenderOpts{}, "", fmt.Errorf("malformed prime spec URI %q: slug must not contain '/'", uri)
	}
	opts, format, err = parsePrimeQuery(query)
	if err != nil {
		return "", render.RenderOpts{}, "", err
	}
	return slug, opts, format, nil
}

// parsePrimeQuery decodes the supported prime resource query parameters.
// Recognises:
//
//	provenance=true (case-insensitive truthy values per strconv.ParseBool)
//	format=json     (anything else falls back to markdown)
func parsePrimeQuery(raw string) (render.RenderOpts, string, error) {
	opts := render.RenderOpts{}
	format := "markdown"
	if raw == "" {
		return opts, format, nil
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return render.RenderOpts{}, "", fmt.Errorf("parse query %q: %w", raw, err)
	}
	if v := values.Get("provenance"); v != "" {
		switch strings.ToLower(v) {
		case "1", "t", "true", "y", "yes":
			opts.ShowProvenance = true
		case "0", "f", "false", "n", "no":
			opts.ShowProvenance = false
		default:
			return render.RenderOpts{}, "", fmt.Errorf("invalid provenance value %q", v)
		}
	}
	if v := values.Get("format"); v != "" {
		switch strings.ToLower(v) {
		case "json":
			format = "json"
		case "markdown", "md":
			format = "markdown"
		default:
			return render.RenderOpts{}, "", fmt.Errorf("invalid format value %q", v)
		}
	}
	return opts, format, nil
}

// renderPrimeResource dispatches rendering based on which view is non-nil
// and the requested format. Exactly one of project or spec must be non-nil.
func renderPrimeResource(uri string, project *specv1.ProjectView, spec *specv1.SpecView, opts render.RenderOpts, format string) ([]ResourceContent, error) {
	var (
		body     string
		mimeType string
	)
	switch format {
	case "json":
		var (
			data []byte
			err  error
		)
		if project != nil {
			data, err = render.RenderProjectJSON(project, opts)
		} else {
			data, err = render.RenderSpecJSON(spec, opts)
		}
		if err != nil {
			return nil, fmt.Errorf("render prime json: %w", err)
		}
		body = string(data)
		mimeType = "application/json"
	default:
		if project != nil {
			body = render.RenderProjectMarkdown(project, opts)
		} else {
			body = render.RenderSpecMarkdown(spec, opts)
		}
		mimeType = "text/markdown"
	}
	return []ResourceContent{{URI: uri, MimeType: mimeType, Text: body}}, nil
}

// ---------------------------------------------------------------------------
// skillsResourceHandler — specgraph://skills/{name}
// ---------------------------------------------------------------------------

// skillsResourceHandler returns the verbatim SKILL.md bytes for a named
// skill. The URI is parsed through extractSkillName (strict — see helper
// docs). Malformed URIs map to connect.CodeInvalidArgument; unknown
// names (URI well-formed but name not in catalog) map to
// connect.CodeNotFound.
func skillsResourceHandler(src skills.Source) ResourceHandler {
	return func(ctx context.Context, uri string) ([]ResourceContent, error) {
		name, err := extractSkillName(uri)
		if err != nil {
			// Malformed URI → CodeInvalidArgument; unknown name → CodeNotFound.
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		sk, err := src.Get(ctx, name)
		if err != nil {
			if errors.Is(err, skills.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, fmt.Errorf("get %s: %w", name, err)
		}
		return []ResourceContent{{
			URI:      uri,
			MimeType: "text/markdown",
			Text:     string(sk.Body),
		}}, nil
	}
}

// ---------------------------------------------------------------------------
// RegisterResources adds all SpecGraph MCP resources to the registry.
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
		Description: "Session-priming digest: constitution summary, graph counts, ready specs, findings summary. Supports ?provenance=true and ?format=json.",
		MimeType:    "text/markdown",
		IsTemplate:  false,
		Handler:     primeResourceHandler(c),
	})
	r.AddResource(ResourceDef{
		URI:         "specgraph://prime/spec/{slug}",
		Name:        "prime-spec",
		Description: "Spec-scoped priming digest: constitution, related decisions, slices, claims, and blockers for a single spec. Supports ?provenance=true and ?format=json.",
		MimeType:    "text/markdown",
		IsTemplate:  true,
		Handler:     specSpecificPrimeResourceHandler(c),
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
