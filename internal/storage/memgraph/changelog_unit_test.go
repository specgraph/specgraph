// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalFieldChanges_Empty(t *testing.T) {
	result, err := marshalFieldChanges(nil)
	require.NoError(t, err)
	assert.Equal(t, "[]", result)
}

func TestMarshalFieldChanges_SingleChange(t *testing.T) {
	changes := []storage.FieldChange{
		{Field: "intent", OldValue: "old", NewValue: "new"},
	}
	result, err := marshalFieldChanges(changes)
	require.NoError(t, err)
	assert.Contains(t, result, `"field":"intent"`)
	assert.Contains(t, result, `"old_value":"old"`)
	assert.Contains(t, result, `"new_value":"new"`)
}

func TestMarshalFieldChanges_MultipleChanges(t *testing.T) {
	changes := []storage.FieldChange{
		{Field: "intent", OldValue: "a", NewValue: "b"},
		{Field: "priority", OldValue: "p1", NewValue: "p2"},
	}
	result, err := marshalFieldChanges(changes)
	require.NoError(t, err)
	assert.Contains(t, result, `"field":"intent"`)
	assert.Contains(t, result, `"field":"priority"`)
}

func TestUnmarshalFieldChanges_Empty(t *testing.T) {
	result, err := unmarshalFieldChanges("")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalFieldChanges_EmptyArray(t *testing.T) {
	result, err := unmarshalFieldChanges("[]")
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestUnmarshalFieldChanges_InvalidJSON(t *testing.T) {
	_, err := unmarshalFieldChanges("{not valid json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal field changes")
}

func TestUnmarshalFieldChanges_Valid(t *testing.T) {
	raw := `[{"field":"intent","old_value":"old","new_value":"new"}]`
	result, err := unmarshalFieldChanges(raw)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "intent", result[0].Field)
	assert.Equal(t, "old", result[0].OldValue)
	assert.Equal(t, "new", result[0].NewValue)
}

func TestMarshalUnmarshalFieldChanges_RoundTrip(t *testing.T) {
	original := []storage.FieldChange{
		{Field: "intent", OldValue: "login", NewValue: "OAuth2 login"},
		{Field: "stage", OldValue: "spark", NewValue: "shape"},
	}
	serialized, err := marshalFieldChanges(original)
	require.NoError(t, err)

	deserialized, err := unmarshalFieldChanges(serialized)
	require.NoError(t, err)
	assert.Equal(t, original, deserialized)
}
