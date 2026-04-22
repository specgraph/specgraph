// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// cliSyntheticExchanges returns a minimal 2-entry probe/response pair marking
// that the stage was authored via the CLI rather than an LLM-driven MCP session.
// Required-exchanges validation is for capturing LLM reasoning; CLI authoring
// has no such dialogue, so the CLI stamps a synthetic placeholder so the
// server's atomic persist still succeeds. Real MCP clients supply their own
// exchanges and this helper is not used for that path.
func cliSyntheticExchanges(stage string) []*specv1.ConversationExchange {
	return []*specv1.ConversationExchange{
		{Role: "probe", Content: "cli invocation", Stage: stage, Sequence: 1},
		{Role: "response", Content: "authored via specgraph CLI (no interactive dialogue)", Stage: stage, Sequence: 2},
	}
}
