// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/client/transport"
)

// FileTokenStore implements the mcp-go transport.TokenStore interface,
// persisting OAuth tokens to a JSON file under ~/.specgraph/.
type FileTokenStore struct {
	path string
}

// NewFileTokenStore creates a token store that reads/writes the given file path.
func NewFileTokenStore(path string) *FileTokenStore {
	return &FileTokenStore{path: path}
}

// GetToken reads the token from disk.
// Returns transport.ErrNoToken if the file does not exist.
func (s *FileTokenStore) GetToken(_ context.Context) (*transport.Token, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, transport.ErrNoToken
		}
		return nil, fmt.Errorf("read token file: %w", err)
	}
	var token transport.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &token, nil
}

// SaveToken writes the token to disk with 0600 permissions.
func (s *FileTokenStore) SaveToken(_ context.Context, token *transport.Token) error {
	data, err := json.Marshal(token) //nolint:gosec // G117 false positive: token is an OAuth struct, not a secret literal
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create token directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}
