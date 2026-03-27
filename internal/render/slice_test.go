// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package render_test

import (
	"strings"
	"testing"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/render"
	"github.com/stretchr/testify/require"
)

func TestSliceDetail(t *testing.T) {
	out := render.SliceDetail(&specv1.Slice{
		Slug:       "my-spec/backend-api",
		ParentSlug: "my-spec",
		SliceId:    "backend-api",
		Intent:     "Implement REST endpoints",
		Status:     specv1.SliceStatus_SLICE_STATUS_CLAIMED,
		AssignedTo: "alice",
		Verify:     []string{"all tests pass", "no lint errors"},
		Touches:    []string{"internal/server/", "cmd/specgraph/"},
		DependsOn:  []string{"my-spec/data-model"},
	})
	require.Contains(t, out, "my-spec/backend-api")
	require.Contains(t, out, "Implement REST endpoints")
	require.Contains(t, out, "claimed")
	require.Contains(t, out, "alice")
	require.Contains(t, out, "all tests pass")
	require.Contains(t, out, "my-spec/data-model")
}

func TestSliceDetail_Nil(t *testing.T) {
	require.Empty(t, render.SliceDetail(nil))
}

func TestSliceList(t *testing.T) {
	out := render.SliceList([]*specv1.Slice{
		{Slug: "p/a", SliceId: "a", Intent: "First", Status: specv1.SliceStatus_SLICE_STATUS_OPEN},
		{Slug: "p/b", SliceId: "b", Intent: "Second", Status: specv1.SliceStatus_SLICE_STATUS_DONE},
	})
	require.Contains(t, out, "a")
	require.Contains(t, out, "First")
	require.Contains(t, out, "open")
	require.Contains(t, out, "done")
}

func TestSliceList_Empty(t *testing.T) {
	require.Contains(t, render.SliceList(nil), "No slices")
}

func TestSliceList_EscapesPipeAndNewline(t *testing.T) {
	out := render.SliceList([]*specv1.Slice{
		{Slug: "p/tricky", SliceId: "id|bad", Intent: "line1\nline2", Status: specv1.SliceStatus_SLICE_STATUS_OPEN, AssignedTo: "a|b"},
	})
	// Pipes escaped, newlines replaced — each row is one line.
	require.NotContains(t, out, "| id|bad |")
	require.Contains(t, out, `id\|bad`)
	require.Contains(t, out, "line1 line2")
	require.Contains(t, out, `a\|b`)
	// Table should have exactly 3 lines: header, separator, data row.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
}
