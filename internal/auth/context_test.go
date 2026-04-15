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
