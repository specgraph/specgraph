// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package main

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadinessPinger_NotReadyBeforeSet(t *testing.T) {
	p := newReadinessPinger()
	err := p.Ping(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, errStoreNotReady)
}

func TestReadinessPinger_DelegatesAfterSet(t *testing.T) {
	p := newReadinessPinger()

	p.set(&stubPinger{}) // stubPinger is defined in serve_probes_test.go (same package)
	assert.NoError(t, p.Ping(context.Background()))

	want := errors.New("db down")
	p.set(&stubPinger{err: want})
	assert.ErrorIs(t, p.Ping(context.Background()), want)
}
