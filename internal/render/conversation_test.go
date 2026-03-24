// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/assert"
)

func TestConversationLog_RendersPairs(t *testing.T) {
	log := &specv1.ConversationLog{
		Stage:   "shape",
		Version: 3,
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "What should be in scope?", Stage: "shape", Sequence: 1},
			{Role: "response", Content: "API and storage, not CLI", Stage: "shape", Sequence: 1, DecisionPoint: true},
		},
	}
	output := render.ConversationLog(log)
	assert.Contains(t, output, "Authoring Conversation (shape, v3)")
	assert.Contains(t, output, "(decision)")
	assert.Contains(t, output, "What should be in scope?")
	assert.Contains(t, output, "API and storage, not CLI")
}

func TestConversationLog_Nil(t *testing.T) {
	assert.Empty(t, render.ConversationLog(nil))
}

func TestConversationLogList_Empty(t *testing.T) {
	assert.Empty(t, render.ConversationLogList(nil))
}

func TestConversationLog_AmendLabel(t *testing.T) {
	log := &specv1.ConversationLog{
		Stage:   "shape",
		Version: 5,
		IsAmend: true,
		Exchanges: []*specv1.ConversationExchange{
			{Role: "probe", Content: "Revised scope?", Stage: "shape", Sequence: 1},
		},
	}
	output := render.ConversationLog(log)
	assert.True(t, strings.Contains(output, "amend"))
}
