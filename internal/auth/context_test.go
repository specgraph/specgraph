// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"testing"
)

func TestBearerToken_RoundTrip(t *testing.T) {
	ctx := context.Background()
	token := "test-api-key-12345"

	ctx = WithBearerToken(ctx, token)
	got, ok := BearerTokenFromContext(ctx)

	if !ok {
		t.Fatal("BearerTokenFromContext returned false")
	}
	if got != token {
		t.Errorf("BearerTokenFromContext = %q, want %q", got, token)
	}
}

func TestBearerToken_Missing(t *testing.T) {
	ctx := context.Background()

	got, ok := BearerTokenFromContext(ctx)
	if ok {
		t.Errorf("BearerTokenFromContext returned true for empty context, got %q", got)
	}
}

func TestBearerToken_Empty(t *testing.T) {
	ctx := WithBearerToken(context.Background(), "")

	_, ok := BearerTokenFromContext(ctx)
	if ok {
		t.Error("BearerTokenFromContext returned true for empty token")
	}
}

func TestInteractiveLoginContext(t *testing.T) {
	ctx := context.Background()
	if InteractiveLoginFromContext(ctx) {
		t.Fatal("plain context must not be marked interactive")
	}
	ctx = WithInteractiveLogin(ctx)
	if !InteractiveLoginFromContext(ctx) {
		t.Fatal("marked context must report interactive")
	}
}

func TestMCPRequestContext(t *testing.T) {
	ctx := context.Background()
	if MCPRequestFromContext(ctx) {
		t.Fatal("plain context must not be marked as an MCP request")
	}
	ctx = WithMCPRequest(ctx)
	if !MCPRequestFromContext(ctx) {
		t.Fatal("marked context must report an MCP request")
	}
}

// TestMCPRequestContext_IndependentFromInteractive guards that the two
// per-request markers use distinct keys and never alias each other.
func TestMCPRequestContext_IndependentFromInteractive(t *testing.T) {
	ctx := WithInteractiveLogin(context.Background())
	if MCPRequestFromContext(ctx) {
		t.Fatal("interactive-login marker must not imply the MCP-request marker")
	}
	ctx = WithMCPRequest(context.Background())
	if InteractiveLoginFromContext(ctx) {
		t.Fatal("MCP-request marker must not imply the interactive-login marker")
	}
}
