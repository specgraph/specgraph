// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/server"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errorScoper is a Scoper that always returns an error.
type errorScoper struct{ err error }

func (e *errorScoper) Scoped(_ context.Context, _ string) (storage.ScopedBackend, error) {
	return nil, e.err
}

func (e *errorScoper) Subscribe(_ storage.ChangeSubscriber) {}

func TestScopeStore_MissingProjectHeader(t *testing.T) {
	scoper := &testScoper{backend: &stubBackend{}}
	_, err := server.ScopeStore(context.Background(), scoper)
	require.Error(t, err)
	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
}

func TestScopeStore_InvalidSlugs(t *testing.T) {
	scoper := &testScoper{backend: &stubBackend{}}
	invalidSlugs := []string{
		"../traversal",
		"has spaces",
		"",
		"A",              // single char
		"UPPERCASE",      // uppercase not allowed
		"has/slash",      // slash not allowed
		"has.dot",        // dot not allowed
		"-starts-hyphen", // starts with hyphen
		"ends-hyphen-",   // ends with hyphen
	}
	for _, slug := range invalidSlugs {
		t.Run(slug, func(t *testing.T) {
			ctx := server.TestInjectProject(context.Background(), slug)
			_, err := server.ScopeStore(ctx, scoper)
			require.Error(t, err, "expected error for slug %q", slug)
			var connectErr *connect.Error
			require.True(t, errors.As(err, &connectErr))
			assert.Equal(t, connect.CodeInvalidArgument, connectErr.Code())
		})
	}
}

func TestScopeStore_ValidSlug(t *testing.T) {
	backend := &stubBackend{}
	scoper := &testScoper{backend: backend}
	validSlugs := []string{
		"my-project",
		"ab",
		"org-frontend",
		"project123",
		"a1b2c3",
	}
	for _, slug := range validSlugs {
		t.Run(slug, func(t *testing.T) {
			ctx := server.TestInjectProject(context.Background(), slug)
			store, err := server.ScopeStore(ctx, scoper)
			require.NoError(t, err)
			assert.NotNil(t, store)
		})
	}
}

func TestScopeStore_ScoperError(t *testing.T) {
	sentinelErr := errors.New("backend unavailable")
	scoper := &errorScoper{err: sentinelErr}
	ctx := server.TestInjectProject(context.Background(), "valid-project")
	_, err := server.ScopeStore(ctx, scoper)
	require.Error(t, err)
	var connectErr *connect.Error
	require.True(t, errors.As(err, &connectErr))
	assert.Equal(t, connect.CodeInternal, connectErr.Code())
}
