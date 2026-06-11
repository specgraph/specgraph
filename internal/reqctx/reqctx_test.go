// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package reqctx_test

import (
	"context"
	"testing"

	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromContext_AbsentReturnsNil(t *testing.T) {
	assert.Nil(t, reqctx.FromContext(context.Background()),
		"no carrier seeded → FromContext must return nil so writers can guard")
}

func TestNewContext_RoundTripAndPointerMutationVisible(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())
	require.NotNil(t, info)

	got := reqctx.FromContext(ctx)
	require.Same(t, info, got, "FromContext must return the same pointer NewContext seeded")
	got.Procedure = "/svc/Method"
	got.Code = "ok"
	got.Identity = "apikey:abc"

	assert.Equal(t, "/svc/Method", info.Procedure)
	assert.Equal(t, "ok", info.Code)
	assert.Equal(t, "apikey:abc", info.Identity)
}
