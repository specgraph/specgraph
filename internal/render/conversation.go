// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package render

import (
	"fmt"
	"strings"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// ConversationLog renders a single conversation log as markdown.
func ConversationLog(log *specv1.ConversationLog) string {
	if log == nil || len(log.Exchanges) == 0 {
		return ""
	}
	var b strings.Builder

	amendLabel := ""
	if log.IsAmend {
		amendLabel = ", amend"
	}
	fmt.Fprintf(&b, "### Authoring Conversation (%s, v%d%s)\n\n", log.Stage, log.Version, amendLabel)

	// Group exchanges by sequence number (probe + response pairs).
	type pair struct {
		probe    *specv1.ConversationExchange
		response *specv1.ConversationExchange
	}
	pairs := make(map[int32]*pair)
	var order []int32
	for _, e := range log.Exchanges {
		p, ok := pairs[e.Sequence]
		if !ok {
			p = &pair{}
			pairs[e.Sequence] = p
			order = append(order, e.Sequence)
		}
		if e.Role == "probe" {
			p.probe = e
		} else {
			p.response = e
		}
	}

	for _, seq := range order {
		p := pairs[seq]
		decisionTag := ""
		if p.response != nil && p.response.DecisionPoint {
			decisionTag = " (decision)"
		}
		fmt.Fprintf(&b, "**[%d]%s**\n", seq, decisionTag)
		if p.probe != nil {
			fmt.Fprintf(&b, "> **Probe:** %s\n", p.probe.Content)
		}
		if p.response != nil {
			fmt.Fprintf(&b, "> **User:** %s\n", p.response.Content)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// ConversationLogList renders multiple conversation logs in narrative order.
func ConversationLogList(logs []*specv1.ConversationLog) string {
	if len(logs) == 0 {
		return ""
	}
	var b strings.Builder
	for _, log := range logs {
		b.WriteString(ConversationLog(log))
	}
	return b.String()
}
