// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package mcp

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect"
	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/authoring"
	"github.com/stretchr/testify/require"
)

// TestComposerAdapter_GetSpecSummary_NotFoundMaps verifies that a
// connect.CodeNotFound error from the underlying GetSpec RPC is translated
// into authoring.ErrSpecNotFound. Without this mapping the soft-miss path
// documented on authoring.ErrSpecNotFound would never trigger in production
// (the real server maps storage.ErrSpecNotFound → connect.CodeNotFound).
func TestComposerAdapter_GetSpecSummary_NotFoundMaps(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("spec not found"))
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "missing-slug")
	require.Nil(t, summary)
	require.Error(t, err)
	require.True(t, errors.Is(err, authoring.ErrSpecNotFound),
		"expected errors.Is(err, authoring.ErrSpecNotFound), got %v", err)
}

// TestComposerAdapter_GetSpecSummary_NilSpecMaps verifies the defense-in-depth
// nil-Spec branch: if a backend returns success with no spec payload, the
// adapter still produces ErrSpecNotFound rather than a nil-summary leak.
func TestComposerAdapter_GetSpecSummary_NilSpecMaps(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return &specv1.GetSpecResponse{}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "any-slug")
	require.Nil(t, summary)
	require.True(t, errors.Is(err, authoring.ErrSpecNotFound),
		"expected errors.Is(err, authoring.ErrSpecNotFound), got %v", err)
}

// TestComposerAdapter_GetSpecSummary_OtherErrorWrapped verifies that
// non-NotFound errors are wrapped opaquely (callers should not treat them
// as a soft miss).
func TestComposerAdapter_GetSpecSummary_OtherErrorWrapped(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(_ string) (*specv1.GetSpecResponse, error) {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("boom"))
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "any-slug")
	require.Nil(t, summary)
	require.Error(t, err)
	require.False(t, errors.Is(err, authoring.ErrSpecNotFound),
		"non-NotFound error should not match ErrSpecNotFound, got %v", err)
}

// TestComposerAdapter_GetSpecSummary_Success verifies the happy path: a
// populated Spec produces a SpecSummary with Slug/Intent/Stage copied through.
func TestComposerAdapter_GetSpecSummary_Success(t *testing.T) {
	c := &Client{
		Spec: &mockSpecService{
			getSpec: func(slug string) (*specv1.GetSpecResponse, error) {
				return &specv1.GetSpecResponse{
					Spec: &specv1.Spec{Slug: slug, Intent: "the intent", Stage: "shape"},
				}, nil
			},
		},
	}
	b := &composerBackend{client: c}

	summary, err := b.GetSpecSummary(context.Background(), "my-spec")
	require.NoError(t, err)
	require.NotNil(t, summary)
	require.Equal(t, "my-spec", summary.Slug)
	require.Equal(t, "the intent", summary.Intent)
	require.Equal(t, "shape", summary.Stage)
}
