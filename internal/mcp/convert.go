// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
)

// toSDKTool converts a SpecGraph ToolDef to an mcp-go SDK Tool.
// It populates the InputSchema directly from the handler's JSON Schema map,
// which uses the objectSchema/stringProp/etc helpers in helpers.go.
func toSDKTool(def ToolDef) sdkmcp.Tool {
	return sdkmcp.Tool{
		Name:        def.Name,
		Description: def.Description,
		InputSchema: toSDKInputSchema(def.Schema),
	}
}

// toSDKInputSchema converts a map[string]any JSON Schema to an SDK ToolInputSchema.
func toSDKInputSchema(schema map[string]any) sdkmcp.ToolInputSchema {
	is := sdkmcp.ToolInputSchema{
		Type: "object",
	}
	if props, ok := schema["properties"].(map[string]any); ok {
		is.Properties = props
	}
	if is.Properties == nil {
		is.Properties = make(map[string]any)
	}
	if req, ok := schema["required"].([]string); ok {
		is.Required = req
	} else if reqAny, ok := schema["required"].([]any); ok {
		// objectSchema stores required as []string but the type system may
		// round-trip through []any in some contexts.
		for _, v := range reqAny {
			if s, ok := v.(string); ok {
				is.Required = append(is.Required, s)
			}
		}
	}
	return is
}

// fromSDKParams extracts the arguments map from an SDK CallToolRequest.
func fromSDKParams(req sdkmcp.CallToolRequest) map[string]any {
	return req.GetArguments()
}

// toSDKResult converts a SpecGraph ToolResult to an mcp-go CallToolResult.
func toSDKResult(r *ToolResult) *sdkmcp.CallToolResult {
	contents := make([]sdkmcp.Content, 0, len(r.Content))
	for _, c := range r.Content {
		contents = append(contents, sdkmcp.NewTextContent(c.Text))
	}
	return &sdkmcp.CallToolResult{
		Content: contents,
		IsError: r.IsError,
	}
}

// toSDKResource converts a SpecGraph ResourceDef (non-template) to an SDK Resource.
func toSDKResource(def ResourceDef) sdkmcp.Resource {
	opts := []sdkmcp.ResourceOption{
		sdkmcp.WithResourceDescription(def.Description),
	}
	if def.MimeType != "" {
		opts = append(opts, sdkmcp.WithMIMEType(def.MimeType))
	}
	return sdkmcp.NewResource(def.URI, def.Name, opts...)
}

// toSDKResourceTemplate converts a SpecGraph ResourceDef (template) to an SDK ResourceTemplate.
func toSDKResourceTemplate(def ResourceDef) sdkmcp.ResourceTemplate {
	opts := []sdkmcp.ResourceTemplateOption{
		sdkmcp.WithTemplateDescription(def.Description),
	}
	if def.MimeType != "" {
		opts = append(opts, sdkmcp.WithTemplateMIMEType(def.MimeType))
	}
	return sdkmcp.NewResourceTemplate(def.URI, def.Name, opts...)
}

// toSDKResourceContents converts SpecGraph ResourceContent slices to SDK ResourceContents.
func toSDKResourceContents(rcs []ResourceContent) []sdkmcp.ResourceContents {
	out := make([]sdkmcp.ResourceContents, 0, len(rcs))
	for _, rc := range rcs {
		out = append(out, sdkmcp.TextResourceContents{
			URI:      rc.URI,
			MIMEType: rc.MimeType,
			Text:     rc.Text,
		})
	}
	return out
}

// toSDKPrompt converts a SpecGraph PromptDef to an SDK Prompt.
func toSDKPrompt(def PromptDef) sdkmcp.Prompt {
	opts := []sdkmcp.PromptOption{
		sdkmcp.WithPromptDescription(def.Description),
	}
	for _, arg := range def.Arguments {
		argOpts := []sdkmcp.ArgumentOption{}
		if arg.Description != "" {
			argOpts = append(argOpts, sdkmcp.ArgumentDescription(arg.Description))
		}
		if arg.Required {
			argOpts = append(argOpts, sdkmcp.RequiredArgument())
		}
		opts = append(opts, sdkmcp.WithArgument(arg.Name, argOpts...))
	}
	return sdkmcp.NewPrompt(def.Name, opts...)
}

// toSDKPromptResult converts a SpecGraph PromptResult to an SDK GetPromptResult.
func toSDKPromptResult(r *PromptResult) *sdkmcp.GetPromptResult {
	msgs := make([]sdkmcp.PromptMessage, 0, len(r.Messages))
	for _, m := range r.Messages {
		role := sdkmcp.RoleUser
		if m.Role == "assistant" {
			role = sdkmcp.RoleAssistant
		}
		msgs = append(msgs, sdkmcp.NewPromptMessage(role, sdkmcp.NewTextContent(m.Content)))
	}
	return sdkmcp.NewGetPromptResult(r.Description, msgs)
}
