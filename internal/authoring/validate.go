// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package authoring

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
)

const (
	// MaxConversationExchanges is the maximum number of exchanges allowed per call.
	MaxConversationExchanges = 100
	// MaxExchangeContentLen is the maximum character length of a single exchange's content.
	MaxExchangeContentLen = 4096
)

// ValidationError indicates conversation_exchanges failed a structural check.
type ValidationError struct {
	Reason string
}

func (e *ValidationError) Error() string { return "conversation_exchanges: " + e.Reason }

// newValidationError returns a non-empty-reason ValidationError. Using this
// constructor rather than the struct literal ensures error messages are
// always descriptive.
func newValidationError(reason string) *ValidationError {
	if reason == "" {
		reason = "unspecified validation failure"
	}
	return &ValidationError{Reason: reason}
}

// ValidateExchanges enforces the structural invariants from the design doc:
// non-empty, role in {"probe","response"}, non-empty content, stage (when set)
// matches target, sequence strictly increasing.
//
// targetStage is the authoring stage of the call ("shape", "specify", etc.).
// An empty stage field on an exchange is treated as unspecified and accepted.
func ValidateExchanges(exchanges []*specv1.ConversationExchange, targetStage string) error {
	if len(exchanges) == 0 {
		return newValidationError("at least one exchange required")
	}
	if len(exchanges) > MaxConversationExchanges {
		return newValidationError(fmt.Sprintf("exchanges exceed maximum of %d", MaxConversationExchanges))
	}

	var lastSeq int32
	seenAny := false

	for i, ex := range exchanges {
		if ex == nil {
			return newValidationError(fmt.Sprintf("exchange[%d] is nil", i))
		}
		role := ex.GetRole()
		if role == "" {
			return newValidationError(fmt.Sprintf("exchange[%d] missing role", i))
		}
		if role != "probe" && role != "response" {
			return newValidationError(fmt.Sprintf("exchange[%d] role %q must be one of: probe, response", i, role))
		}
		if ex.GetContent() == "" {
			return newValidationError(fmt.Sprintf("exchange[%d] missing content", i))
		}
		if len(ex.GetContent()) > MaxExchangeContentLen {
			return newValidationError(fmt.Sprintf("exchange[%d] content exceeds maximum length of %d characters", i, MaxExchangeContentLen))
		}
		if s := ex.GetStage(); s != "" && s != targetStage {
			return newValidationError(fmt.Sprintf("exchange[%d] stage %q does not match target stage %q", i, s, targetStage))
		}
		seq := ex.GetSequence()
		if seq <= 0 {
			return newValidationError(fmt.Sprintf("exchange[%d] sequence %d must be >= 1", i, seq))
		}
		if seenAny && seq <= lastSeq {
			return newValidationError(fmt.Sprintf("exchange[%d] sequence %d is not strictly greater than previous %d", i, seq, lastSeq))
		}
		lastSeq = seq
		seenAny = true
	}

	return nil
}
