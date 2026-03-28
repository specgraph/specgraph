// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_HalfConfiguredAuth(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		wantErr  string
	}{
		{"username only", "admin", "", "username and password must be set together"},
		{"password only", "", "secret", "username and password must be set together"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use an unreachable URI — we expect failure before connection.
			_, err := New(context.Background(), "bolt://localhost:1", WithAuth(tt.username, tt.password))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestUpgradeBoltSchemeToTLS(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"bolt://localhost:7687", "bolt+s://localhost:7687"},
		{"neo4j://localhost:7687", "neo4j+s://localhost:7687"},
		{"bolt+s://localhost:7687", "bolt+s://localhost:7687"},
		{"bolt+ssc://localhost:7687", "bolt+ssc://localhost:7687"},
		{"neo4j+s://localhost:7687", "neo4j+s://localhost:7687"},
		{"neo4j+ssc://localhost:7687", "neo4j+ssc://localhost:7687"},
		{"http://localhost:7687", "http://localhost:7687"}, // unknown scheme unchanged
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, upgradeBoltSchemeToTLS(tt.input))
		})
	}
}
