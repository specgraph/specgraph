// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/spf13/cobra"
)

// conversationFlagHelp documents the --conversation input contract. It calls out
// explicitly that the flag takes a BARE JSON array (not the `conversation record`
// object shape) so operators do not confuse the two formats (review finding #3).
const conversationFlagHelp = "path to a JSON file containing a bare JSON array of ConversationExchange " +
	"objects ([{role,content,stage,sequence,decision_point}, ...]); NOT the `conversation record` " +
	"object shape ({\"exchanges\":[...]}); use - to read from stdin"

// conversationExchangeInput is the CLI JSON structure for a single conversation
// exchange. The --conversation file is a bare JSON ARRAY of these objects (D-05,
// A2) — the same shape the MCP `author` tool accepts — so there is one validation
// contract. This is distinct from conversationRecordInput, which wraps the array
// in an {"exchanges":[...]} object (Pitfall #3).
type conversationExchangeInput struct {
	Role          string `json:"role"`
	Content       string `json:"content"`
	Stage         string `json:"stage"`
	Sequence      int32  `json:"sequence"`
	DecisionPoint bool   `json:"decision_point,omitempty"`
}

// maxConversationBytes bounds the size of a --conversation payload (stdin or
// file) before it is buffered and parsed. Without this cap a multi-gigabyte
// file or piped stream would be fully buffered into process memory, and a file
// with millions of array entries would build millions of proto messages before
// the server rejects the request at the MaxConversationExchanges limit (WR-02).
const maxConversationBytes = 1 << 20 // 1 MiB

// readBoundedConversation reads at most maxConversationBytes+1 bytes from r and
// rejects the input if it exceeds the cap. The +1 lets us distinguish an
// exactly-at-limit payload from an over-limit one.
func readBoundedConversation(r io.Reader, source string) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConversationBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", source, err)
	}
	if len(data) > maxConversationBytes {
		return nil, fmt.Errorf("conversation input from %s exceeds %d bytes", source, maxConversationBytes)
	}
	return data, nil
}

// loadConversationFlag reads a bare JSON array of ConversationExchange objects
// from the given path and maps it to []*specv1.ConversationExchange. When path is
// "-" it reads the array from stdin. An object-shaped payload
// ({"exchanges":[...]}) is rejected because it does not unmarshal into a slice.
//
// The read is bounded to maxConversationBytes to prevent unbounded client-side
// buffering (WR-02).
//
// Security note: path is a CLI-supplied local value and must never be reused in
// a server/network path.
func loadConversationFlag(path string) ([]*specv1.ConversationExchange, error) {
	var input []conversationExchangeInput
	if path == "-" {
		data, err := readBoundedConversation(os.Stdin, "stdin")
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, fmt.Errorf("parse stdin: %w", err)
		}
	} else {
		f, err := os.Open(path) //nolint:gosec // path is CLI-supplied local value (see security note)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		defer func() { _ = f.Close() }()
		data, err := readBoundedConversation(f, path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &input); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	if len(input) > authoring.MaxConversationExchanges {
		return nil, fmt.Errorf("conversation input has %d exchanges, exceeds maximum of %d", len(input), authoring.MaxConversationExchanges)
	}

	exchanges := make([]*specv1.ConversationExchange, len(input))
	for i, e := range input {
		exchanges[i] = &specv1.ConversationExchange{
			Role:          e.Role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	return exchanges, nil
}

// registerConversationFlag registers a --conversation flag on cmd bound to target.
// When required is true it enforces the flag via Cobra's required-flag validation
// (mirroring conversation.go:48-49); a bare bool does nothing without this call
// (review R2 #4).
func registerConversationFlag(cmd *cobra.Command, target *string, required bool) {
	cmd.Flags().StringVar(target, "conversation", "", conversationFlagHelp)
	if required {
		cobra.CheckErr(cmd.MarkFlagRequired("conversation"))
	}
}
