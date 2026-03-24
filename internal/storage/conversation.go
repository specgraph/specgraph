// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package storage

import (
	"context"
	"time"
)

// ConversationExchange represents a single probe/response from an authoring session.
type ConversationExchange struct {
	Role          string // "probe" or "response"
	Content       string
	Stage         string
	Sequence      int32
	DecisionPoint bool
}

// ConversationLogEntry records the authoring conversation for a single stage completion.
type ConversationLogEntry struct {
	ID            string
	Stage         SpecStage
	Version       int32
	IsAmend       bool
	Exchanges     []ConversationExchange
	ExchangeCount int32
	Date          time.Time
}

// ConversationBackend defines storage operations for conversation logs.
type ConversationBackend interface {
	// RecordConversation stores a conversation log for a spec stage.
	// Links to the most recent ChangeLog via EXPLAINS edge (if one exists).
	// Extends the CONTINUES chain from the previous ConversationLog (if one exists).
	// Returns ErrSpecNotFound if the spec slug does not exist.
	RecordConversation(ctx context.Context, slug string, entry ConversationLogEntry) (*ConversationLogEntry, error)

	// ListConversations returns conversation logs for a spec in narrative chain order.
	// If stage is non-empty, filters to that stage only.
	// Returns an empty slice (not an error) if no conversation logs exist.
	// Returns ErrSpecNotFound if the spec slug does not exist.
	ListConversations(ctx context.Context, slug string, stage string) ([]*ConversationLogEntry, error)
}
