// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/specgraph/specgraph/internal/auth"
)

func TestEvaluate_EmitsDecisionLog(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	eng, err := auth.NewCedarEngine(context.Background(),
		[]auth.PolicySource{baseSource()}, testActions())
	require.NoError(t, err)

	_, err = eng.Evaluate(context.Background(), auth.EvalRequest{
		Identity: &auth.Identity{UserID: "u1", EffectiveRole: auth.RoleAdmin, Subject: "apikey:k1"},
		Action:   "graph.delete",
		Resource: auth.ResourceRef{Type: "graph"},
	})
	require.NoError(t, err)

	out := buf.String()
	require.True(t, strings.Contains(out, "cedar decision"), "log: %s", out)
	require.Contains(t, out, "graph.delete")
	require.Contains(t, out, "allowed=true")
}
