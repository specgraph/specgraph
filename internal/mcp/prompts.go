// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// RegisterPrompts registers all MCP prompt definitions into the registry.
func RegisterPrompts(r *Registry, c *Client) {
	r.AddPrompt(PromptDef{
		Name:        "spark",
		Description: "Generate an initial spec idea from a topic.",
		Arguments: []PromptArgument{
			{Name: "topic", Description: "The topic or problem area to spark a spec idea for.", Required: true},
			{Name: "context", Description: "Optional additional context to guide the spark.", Required: false},
		},
		Handler: sparkPromptHandler(c),
	})

	r.AddPrompt(PromptDef{
		Name:        "shape",
		Description: "Refine a sparked spec into a clear problem statement.",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Slug of the spec to shape.", Required: true},
		},
		Handler: stagePromptHandler(c, "shape"),
	})

	r.AddPrompt(PromptDef{
		Name:        "specify",
		Description: "Add full specification detail to a shaped spec.",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Slug of the spec to specify.", Required: true},
		},
		Handler: stagePromptHandler(c, "specify"),
	})

	r.AddPrompt(PromptDef{
		Name:        "decompose",
		Description: "Break a specified spec into actionable work slices.",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Slug of the spec to decompose.", Required: true},
		},
		Handler: stagePromptHandler(c, "decompose"),
	})

	r.AddPrompt(PromptDef{
		Name:        "constitution_check",
		Description: "Check a spec against the project constitution for compliance.",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Slug of the spec to check.", Required: true},
		},
		Handler: analyticalPromptHandler(c, "constitution-check"),
	})

	r.AddPrompt(PromptDef{
		Name:        "dependency_review",
		Description: "Review the dependency health of a spec and its surroundings.",
		Arguments: []PromptArgument{
			{Name: "spec_slug", Description: "Slug of the spec to review.", Required: true},
		},
		Handler: analyticalPromptHandler(c, "peripheral-vision"),
	})
}

// stagePromptHandler returns a PromptHandler that fetches prompt templates for
// the given authoring stage and returns the first template as a user message.
func stagePromptHandler(c *Client, stage string) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		specSlug := args["spec_slug"]
		if specSlug == "" {
			return nil, fmt.Errorf("spec_slug is required for %s prompt", stage)
		}
		resp, err := c.Authoring.GetPrompts(ctx, connect.NewRequest(&specv1.GetPromptsRequest{
			Stage: authoringStageFromString(stage),
		}))
		if err != nil {
			return nil, fmt.Errorf("get prompts for stage %s: %w", stage, err)
		}
		templates := resp.Msg.GetPrompts()
		var text string
		if len(templates) == 0 {
			text = "No prompt template available for stage: " + stage
		} else {
			text = templates[0].GetTemplate()
		}
		text += "\n\nSpec: " + specSlug
		return &PromptResult{
			Messages: []PromptMessage{
				{Role: "user", Content: text},
			},
		}, nil
	}
}

// sparkPromptHandler returns a PromptHandler for the spark stage that appends
// the topic and optional context to the fetched template.
func sparkPromptHandler(c *Client) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		if args["topic"] == "" {
			return nil, fmt.Errorf("topic is required for spark prompt")
		}
		resp, err := c.Authoring.GetPrompts(ctx, connect.NewRequest(&specv1.GetPromptsRequest{
			Stage: authoringStageFromString("spark"),
		}))
		if err != nil {
			return nil, fmt.Errorf("get prompts for stage spark: %w", err)
		}
		templates := resp.Msg.GetPrompts()
		var text string
		if len(templates) == 0 {
			text = "No prompt template available for stage: spark"
		} else {
			text = templates[0].GetTemplate()
		}
		if topic := args["topic"]; topic != "" {
			text += "\n\nTopic: " + topic
		}
		if ctxVal := args["context"]; ctxVal != "" {
			text += "\nContext: " + ctxVal
		}
		return &PromptResult{
			Messages: []PromptMessage{
				{Role: "user", Content: text},
			},
		}, nil
	}
}

// analyticalPromptHandler returns a PromptHandler that runs an analytical pass
// and returns its prompt template as a user message.
func analyticalPromptHandler(c *Client, passType string) PromptHandler {
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		specSlug := args["spec_slug"]
		if specSlug == "" {
			return nil, fmt.Errorf("spec_slug is required for %s prompt", passType)
		}
		resp, err := c.AnalyticalPass.RunAnalyticalPass(ctx, connect.NewRequest(&specv1.RunAnalyticalPassRequest{
			Slug:     specSlug,
			PassType: passTypeFromString(passType),
		}))
		if err != nil {
			return nil, fmt.Errorf("run analytical pass %s: %w", passType, err)
		}
		return &PromptResult{
			Messages: []PromptMessage{
				{Role: "user", Content: resp.Msg.GetPromptTemplate()},
			},
		}, nil
	}
}
