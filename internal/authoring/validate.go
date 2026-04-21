// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

// ValidationError indicates conversation_exchanges failed a structural check.
type ValidationError struct {
	Reason string
}

func (e *ValidationError) Error() string { return "conversation_exchanges: " + e.Reason }

// ValidateExchanges enforces the structural invariants from the design doc:
// non-empty, role in {"probe","response"}, non-empty content, stage (when set)
// matches target, sequence strictly increasing.
//
// targetStage is the authoring stage of the call ("shape", "specify", etc.).
// An empty stage field on an exchange is treated as unspecified and accepted.
func ValidateExchanges(exchanges []*specv1.ConversationExchange, targetStage string) error {
	if len(exchanges) == 0 {
		return &ValidationError{Reason: "at least one exchange required"}
	}

	var lastSeq int32
	seenAny := false

	for i, ex := range exchanges {
		if ex == nil {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] is nil", i)}
		}
		role := ex.GetRole()
		if role == "" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] missing role", i)}
		}
		if role != "probe" && role != "response" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] role %q must be one of: probe, response", i, role)}
		}
		if ex.GetContent() == "" {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] missing content", i)}
		}
		if s := ex.GetStage(); s != "" && s != targetStage {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] stage %q does not match target stage %q", i, s, targetStage)}
		}
		seq := ex.GetSequence()
		if seenAny && seq <= lastSeq {
			return &ValidationError{Reason: fmt.Sprintf("exchange[%d] sequence %d is not strictly greater than previous %d", i, seq, lastSeq)}
		}
		lastSeq = seq
		seenAny = true
	}

	return nil
}
