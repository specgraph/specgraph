// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordToSlice(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	rec := &neo4j.Record{
		Values: []any{
			"slc-123",             // id
			"parent/backend",      // slug
			"parent",              // parent_slug
			"backend",             // slice_id
			"Build the API",       // intent
			`["test passes"]`,     // verify_json
			`["internal/server"]`, // touches_json
			`["parent/frontend"]`, // depends_on_json
			"open",                // status
			"",                    // assigned_to
			now,                   // created_at
			now,                   // updated_at
		},
	}

	sl, err := recordToSlice(rec)
	require.NoError(t, err)
	assert.Equal(t, "slc-123", sl.ID)
	assert.Equal(t, "parent/backend", sl.Slug)
	assert.Equal(t, "parent", sl.ParentSlug)
	assert.Equal(t, "backend", sl.SliceID)
	assert.Equal(t, "Build the API", sl.Intent)
	assert.Equal(t, []string{"test passes"}, sl.Verify)
	assert.Equal(t, []string{"internal/server"}, sl.Touches)
	assert.Equal(t, []string{"parent/frontend"}, sl.DependsOn)
	assert.Equal(t, storage.SliceStatusOpen, sl.Status)
	assert.Empty(t, sl.AssignedTo)
	assert.NotZero(t, sl.CreatedAt)
}

func TestRecordToSlice_EmptyJSON(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	rec := &neo4j.Record{
		Values: []any{
			"slc-456", "parent/fe", "parent", "fe", "Frontend",
			"", "", "", // empty verify_json, touches_json, depends_on_json
			"claimed", "alice", now, now,
		},
	}

	sl, err := recordToSlice(rec)
	require.NoError(t, err)
	assert.Nil(t, sl.Verify)
	assert.Nil(t, sl.Touches)
	assert.Nil(t, sl.DependsOn)
	assert.Equal(t, storage.SliceStatusClaimed, sl.Status)
	assert.Equal(t, "alice", sl.AssignedTo)
}

func TestRecordToSlice_InvalidVerifyJSON(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	rec := &neo4j.Record{
		Values: []any{
			"slc-789", "parent/bad", "parent", "bad", "Bad",
			`{invalid`, "", "",
			"open", "", now, now,
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal verify_json")
}

func TestRecordToSlice_InvalidTouchesJSON(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	rec := &neo4j.Record{
		Values: []any{
			"slc-790", "parent/bad2", "parent", "bad2", "Bad",
			`[]`, `{invalid`, "",
			"open", "", now, now,
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal touches_json")
}

func TestRecordToSlice_InvalidDependsOnJSON(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	rec := &neo4j.Record{
		Values: []any{
			"slc-791", "parent/bad3", "parent", "bad3", "Bad",
			`[]`, `[]`, `{invalid`,
			"open", "", now, now,
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal depends_on_json")
}

func TestCreateSlice_NilSlice(t *testing.T) {
	s := &Store{}
	err := s.CreateSlice(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must not be nil")
}

func TestClaimSlice_BlankAssignee(t *testing.T) {
	s := &Store{}
	ctx := context.Background()

	sl, err := s.ClaimSlice(ctx, "any/slug", "")
	require.Error(t, err)
	require.Nil(t, sl)
	assert.Contains(t, err.Error(), "must not be blank")

	sl, err = s.ClaimSlice(ctx, "any/slug", "   ")
	require.Error(t, err)
	require.Nil(t, sl)
	assert.Contains(t, err.Error(), "must not be blank")
}

func TestRecordToSlice_WrongType(t *testing.T) {
	rec := &neo4j.Record{
		Values: []any{
			42, // id should be string, not int — triggers recordString error
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestRecordToSlice_BadTimestamp(t *testing.T) {
	rec := &neo4j.Record{
		Values: []any{
			"slc-999", "parent/ts", "parent", "ts", "Test",
			"", "", "",
			"open", "",
			"not-a-timestamp", // bad created_at
			"2026-03-26T12:00:00.000000000Z",
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_at")
}

func TestRecordToSlice_FieldErrors(t *testing.T) {
	now := "2026-03-26T12:00:00.000000000Z"
	// Base valid record — 12 fields matching recordToSlice column order.
	base := func() []any {
		return []any{
			"slc-err", "parent/err", "parent", "err", "intent",
			`[]`, `[]`, `[]`, // verify, touches, depends_on
			"open", "", now, now,
		}
	}

	// Each test replaces one position with an invalid type to trigger the
	// corresponding recordString/recordStringOptional error branch.
	tests := []struct {
		name string
		pos  int
		val  any
		want string // substring of expected error
	}{
		{"id", 0, 42, "id"},
		{"slug", 1, 42, "slug"},
		{"parent_slug", 2, 42, "parent_slug"},
		{"slice_id", 3, 42, "slice_id"},
		{"intent", 4, 42, "intent"},
		{"verify_json", 5, 42, "verify_json"},
		{"touches_json", 6, 42, "touches_json"},
		{"depends_on_json", 7, 42, "depends_on_json"},
		{"status", 8, 42, "status"},
		{"assigned_to", 9, 42, "assigned_to"},
		{"created_at", 10, 42, "created_at"},
		{"updated_at", 11, 42, "updated_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vals := base()
			vals[tt.pos] = tt.val
			rec := &neo4j.Record{Values: vals}
			_, err := recordToSlice(rec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestRecordToSlice_BadUpdatedAt(t *testing.T) {
	rec := &neo4j.Record{
		Values: []any{
			"slc-ts2", "parent/ts2", "parent", "ts2", "intent",
			"", "", "",
			"open", "",
			"2026-03-26T12:00:00.000000000Z", // valid created_at
			"not-a-timestamp",                 // bad updated_at
		},
	}
	_, err := recordToSlice(rec)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "updated_at")
}
