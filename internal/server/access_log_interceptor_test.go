// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/reqctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectCode(t *testing.T) {
	assert.Equal(t, "ok", connectCode(nil))
	assert.Equal(t, "unknown", connectCode(errors.New("plain")))
	assert.Equal(t, "invalid_argument",
		connectCode(connect.NewError(connect.CodeInvalidArgument, errors.New("x"))))
}

func TestAccessLogInterceptor_FillsCodeOnSuccess(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())
	ic := AccessLogInterceptor()
	var called bool
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		called = true
		return connect.NewResponse(&struct{}{}), nil
	})
	_, err := ic.WrapUnary(next)(ctx, connect.NewRequest(&struct{}{}))
	require.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, "ok", info.Code)
}

func TestAccessLogInterceptor_CapturesErrorCodeFromInner(t *testing.T) {
	ctx, info := reqctx.NewContext(context.Background())
	ic := AccessLogInterceptor()
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("denied"))
	})
	_, err := ic.WrapUnary(next)(ctx, connect.NewRequest(&struct{}{}))
	require.Error(t, err)
	assert.Equal(t, "permission_denied", info.Code,
		"outermost interceptor must observe an inner reject-without-next code")
}

func TestAccessLogInterceptor_NoCarrierIsNoop(t *testing.T) {
	ic := AccessLogInterceptor()
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		return connect.NewResponse(&struct{}{}), nil
	})
	_, err := ic.WrapUnary(next)(context.Background(), connect.NewRequest(&struct{}{}))
	require.NoError(t, err)
}
