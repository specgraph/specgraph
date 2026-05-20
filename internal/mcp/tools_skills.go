// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/specgraph/specgraph/internal/mcp/skills"
)

// RegisterSkillTools registers the three MCP tools that expose the skills
// catalog to model-side callers: specgraph_skills_list,
// specgraph_skills_get, and specgraph_skills_search. The src argument is
// the live skills.Source built once at server startup (see server.go).
//
// Tool schemas use the existing objectSchema/stringProp/boolProp builders
// from helpers.go; handlers read parameters via stringParam/boolParam.
// No typed-struct args — matches the convention used by every other
// tools_*.go file in this package.
func RegisterSkillTools(r *Registry, src skills.Source) {
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_list",
		Description: "List the available SpecGraph skills (one-line summary per skill).",
		Profile:     ProfileCore,
		Schema:      objectSchema(props{}),
		Handler:     skillsListHandler(src),
	})
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_get",
		Description: "Fetch a single SpecGraph skill's SKILL.md body by name.",
		Profile:     ProfileCore,
		Schema: objectSchema(
			props{"name": stringProp("Skill name (kebab-case, e.g. specgraph-authoring)")},
			"name",
		),
		Handler: skillsGetHandler(src),
	})
	r.AddTool(ToolDef{
		Name:        "specgraph_skills_search",
		Description: "Search the SpecGraph skills catalog by keyword (substring) or RE2 regex.",
		Profile:     ProfileCore,
		Schema: objectSchema(
			props{
				"query": stringProp("Substring (default) or RE2 regex"),
				"regex": boolProp("Treat query as RE2 regex; defaults to false. Must be a JSON boolean, not the string \"true\""),
			},
			"query",
		),
		Handler: skillsSearchHandler(src),
	})
}

func skillsListHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, _ map[string]any) (*ToolResult, error) {
		metas, err := src.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("list skills: %w", err)
		}
		body, err := json.MarshalIndent(metas, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		return textResult(string(body)), nil
	}
}

func skillsGetHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		name := stringParam(params, "name")
		if name == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("name is required"))
		}
		sk, err := src.Get(ctx, name)
		if err != nil {
			if errors.Is(err, skills.ErrNotFound) {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, fmt.Errorf("get %s: %w", name, err)
		}
		return textResult(string(sk.Body)), nil
	}
}

func skillsSearchHandler(src skills.Source) ToolHandler {
	return func(ctx context.Context, params map[string]any) (*ToolResult, error) {
		query := stringParam(params, "query")
		if query == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, skills.ErrInvalidQuery)
		}
		mode := skills.SearchText
		if boolParam(params, "regex") {
			mode = skills.SearchRegex
		}
		metas, err := src.Search(ctx, query, skills.SearchOptions{Mode: mode})
		if err != nil {
			if errors.Is(err, skills.ErrInvalidQuery) {
				return nil, connect.NewError(connect.CodeInvalidArgument, err)
			}
			return nil, fmt.Errorf("search %q: %w", query, err)
		}
		body, err := json.MarshalIndent(metas, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		return textResult(string(body)), nil
	}
}
