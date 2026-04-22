// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"errors"
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

func TestValidateExchanges(t *testing.T) {
	tests := []struct {
		name            string
		exchanges       []*specv1.ConversationExchange
		stage           string
		wantErr         bool
		wantErrContains string
	}{
		{
			name:            "empty rejected",
			exchanges:       nil,
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "at least one exchange",
		},
		{
			name: "missing role rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "", Content: "x", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "role",
		},
		{
			name: "unknown role rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "narrator", Content: "x", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "role",
		},
		{
			name: "missing content rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "content",
		},
		{
			name: "mismatched stage rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "x", Stage: "spark", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "stage",
		},
		{
			name: "sequence zero rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "q1", Stage: "shape", Sequence: 0},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "sequence",
		},
		{
			name: "non-increasing sequence rejected",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "q1", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "r1", Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "sequence",
		},
		{
			name: "valid strictly-increasing accepted",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "what is scope?", Stage: "shape", Sequence: 1},
				{Role: "response", Content: "X and Y in; Z out", Stage: "shape", Sequence: 2},
				{Role: "probe", Content: "risks?", Stage: "shape", Sequence: 3},
				{Role: "response", Content: "none", Stage: "shape", Sequence: 4},
			},
			stage: "shape",
		},
		{
			name: "missing stage field accepted when target provided",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: "q", Stage: "", Sequence: 1},
			},
			stage: "shape",
		},
		{
			name: "too many exchanges",
			exchanges: func() []*specv1.ConversationExchange {
				es := make([]*specv1.ConversationExchange, MaxConversationExchanges+1)
				for i := range es {
					role := "probe"
					if i%2 == 1 {
						role = "response"
					}
					es[i] = &specv1.ConversationExchange{
						Role:     role,
						Content:  "x",
						Stage:    "shape",
						Sequence: int32(i + 1),
					}
				}
				return es
			}(),
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "exceed maximum",
		},
		{
			name: "content too long",
			exchanges: []*specv1.ConversationExchange{
				{Role: "probe", Content: string(make([]byte, MaxExchangeContentLen+1)), Stage: "shape", Sequence: 1},
			},
			stage:           "shape",
			wantErr:         true,
			wantErrContains: "exceeds maximum length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExchanges(tt.exchanges, tt.stage)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExchanges err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && tt.wantErrContains != "" {
				var ve *ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("expected ValidationError, got %T", err)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Errorf("err %q does not contain %q", err.Error(), tt.wantErrContains)
				}
			}
		})
	}
}
