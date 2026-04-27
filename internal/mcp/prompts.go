// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
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

// stagePromptHandler returns a PromptHandler that uses authoring.Composer to
// assemble a rich composed prompt (persona + orchestration + recording +
// heuristics + stage + dynamic state) for the given authoring stage.
func stagePromptHandler(c *Client, stage string) PromptHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		specSlug := args["spec_slug"]
		if specSlug == "" {
			return nil, fmt.Errorf("spec_slug is required for %s prompt", stage)
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage: authoring.Stage(stage),
			Slug:  specSlug,
		})
		if err != nil {
			return nil, fmt.Errorf("compose %s prompt: %w", stage, err)
		}
		return &PromptResult{
			Messages: []PromptMessage{{Role: "user", Content: result.Body}},
		}, nil
	}
}

// sparkPromptHandler returns a PromptHandler for the spark stage that composes
// a rich prompt and appends the required topic and optional context.
func sparkPromptHandler(c *Client) PromptHandler {
	composer := authoring.NewComposer(&composerBackend{client: c})
	return func(ctx context.Context, args map[string]string) (*PromptResult, error) {
		topic := args["topic"]
		if topic == "" {
			return nil, fmt.Errorf("topic is required for spark prompt")
		}
		result, err := composer.ComposeStagePrompt(ctx, authoring.ComposeInput{
			Stage: authoring.StageSpark,
			// Spark stage has no spec yet (PromptDef declares only topic + context).
			// If mid-stream re-entry into spark with an existing slug becomes a
			// requirement, declare spec_slug as an optional PromptArgument and read
			// it here.
			Slug: "",
		})
		if err != nil {
			return nil, fmt.Errorf("compose spark prompt: %w", err)
		}
		body := result.Body + "\n\n# Topic\n\n" + topic
		if ctxVal := args["context"]; ctxVal != "" {
			body += "\n\n# Additional Context\n\n" + ctxVal
		}
		return &PromptResult{
			Messages: []PromptMessage{{Role: "user", Content: body}},
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
