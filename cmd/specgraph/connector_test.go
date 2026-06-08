// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyConnectError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want errClass
	}{
		{"invalid_password 28P01", &pgconn.PgError{Code: "28P01"}, classCredential},
		{"invalid_authorization 28000", &pgconn.PgError{Code: "28000"}, classCredential},
		{"wrapped credential", fmt.Errorf("postgres: verify connectivity: %w", &pgconn.PgError{Code: "28P01"}), classCredential},
		{"connection refused 08006", &pgconn.PgError{Code: "08006"}, classOther},
		{"plain error", errors.New("dial tcp: connection refused"), classOther},
		{"nil-ish wrapped non-pg", fmt.Errorf("boom: %w", errors.New("x")), classOther},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyConnectError(tc.err))
		})
	}
}

func testPolicy() backoffPolicy {
	return backoffPolicy{otherBase: time.Millisecond, otherMax: 4 * time.Millisecond, credInterval: time.Millisecond, credMaxRetry: 5}
}

func noSleep(_ context.Context, _ time.Duration) error { return nil }

func TestRunConnector_SucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	attempt := func(context.Context) (connectResult, error) {
		calls++
		if calls < 3 {
			return connectResult{}, errors.New("connection refused")
		}
		return connectResult{}, nil
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	assert.NoError(t, err)
	assert.Equal(t, 3, calls)
}

func TestRunConnector_CredentialExhaustionIsFatal(t *testing.T) {
	calls := 0
	attempt := func(context.Context) (connectResult, error) {
		calls++
		return connectResult{}, &pgconn.PgError{Code: "28P01"}
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credentials rejected")
	assert.Equal(t, 5, calls)
}

func TestRunConnector_CredentialCounterResetsOnOther(t *testing.T) {
	seq := []error{
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		errors.New("refused"),
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		&pgconn.PgError{Code: "28P01"}, &pgconn.PgError{Code: "28P01"},
		nil,
	}
	i := 0
	attempt := func(context.Context) (connectResult, error) {
		err := seq[i]
		i++
		return connectResult{}, err
	}
	_, err := runConnector(context.Background(), attempt, testPolicy(), noSleep)
	assert.NoError(t, err)
}

func TestRunConnector_ContextCancellationStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempt := func(context.Context) (connectResult, error) {
		return connectResult{}, errors.New("refused")
	}
	_, err := runConnector(ctx, attempt, testPolicy(), noSleep)
	assert.ErrorIs(t, err, context.Canceled)
}
