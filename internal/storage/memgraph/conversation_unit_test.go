// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"testing"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalExchanges_InvalidRole(t *testing.T) {
	raw := `[{"role":"bogus","content":"hello","stage":"spark","sequence":1}]`
	exchanges, err := unmarshalExchanges(raw)
	require.NoError(t, err)
	require.Len(t, exchanges, 1)
	assert.Equal(t, storage.ConversationRole("bogus"), exchanges[0].Role)
	assert.False(t, exchanges[0].Role.IsValid(), "bogus role should not be valid")
}

func TestUnmarshalExchanges_Empty(t *testing.T) {
	exchanges, err := unmarshalExchanges("")
	require.NoError(t, err)
	assert.Nil(t, exchanges)

	exchanges, err = unmarshalExchanges("[]")
	require.NoError(t, err)
	assert.Nil(t, exchanges)
}

func TestUnmarshalExchanges_InvalidJSON(t *testing.T) {
	_, err := unmarshalExchanges("{not-json}")
	require.Error(t, err)
}

func TestMarshalUnmarshalExchanges_RoundTrip(t *testing.T) {
	input := []storage.ConversationExchange{
		{Role: storage.ConversationRoleProbe, Content: "what?", Stage: "spark", Sequence: 1},
		{Role: storage.ConversationRoleResponse, Content: "this.", Stage: "spark", Sequence: 2, DecisionPoint: true},
	}
	raw, err := marshalExchanges(input)
	require.NoError(t, err)

	output, err := unmarshalExchanges(raw)
	require.NoError(t, err)
	assert.Equal(t, input, output)
}
