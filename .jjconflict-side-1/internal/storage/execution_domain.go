// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"fmt"
	"time"
)

// ExecutionEventType classifies execution callback events.
type ExecutionEventType int

// ExecutionEventType values.
const (
	ExecutionEventTypeProgress ExecutionEventType = iota + 1
	ExecutionEventTypeBlocker
	ExecutionEventTypeCompletion
)

// String returns the string representation of the event type.
func (t ExecutionEventType) String() string {
	switch t {
	case ExecutionEventTypeProgress:
		return "progress"
	case ExecutionEventTypeBlocker:
		return "blocker"
	case ExecutionEventTypeCompletion:
		return "completion"
	default:
		return "unknown"
	}
}

// ParseExecutionEventType converts a string to an ExecutionEventType.
func ParseExecutionEventType(s string) (ExecutionEventType, error) {
	switch s {
	case "progress":
		return ExecutionEventTypeProgress, nil
	case "blocker":
		return ExecutionEventTypeBlocker, nil
	case "completion":
		return ExecutionEventTypeCompletion, nil
	default:
		return 0, fmt.Errorf("unknown execution event type: %q", s)
	}
}

// ExecutionEvent records a progress, blocker, or completion event from an agent.
type ExecutionEvent struct {
	ID        string
	SpecSlug  string
	Agent     string
	Type      ExecutionEventType
	Message   string
	CreatedAt time.Time
}

// CallbackConfig holds the endpoint paths for agent callbacks.
type CallbackConfig struct {
	Endpoint   string
	Prime      string
	Progress   string
	Blocker    string
	Completion string
}

// Bundle is a self-contained package for an executing agent.
type Bundle struct {
	Version   int32
	Spec      *Spec
	Decisions []*Decision
	Bootstrap string
	Callbacks *CallbackConfig
}

// PrimeData holds the raw data needed to compose a prime response.
type PrimeData struct {
	Spec         *Spec
	Decisions    []*Decision
	Constitution *Constitution
}
