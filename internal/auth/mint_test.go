// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateAPIKeySecret_Shape(t *testing.T) {
	secret, phc, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	require.Len(t, secret, apiKeySecretLen, "secret length matches the resolver's parser expectation")
	require.True(t, strings.HasPrefix(phc, "$argon2id$"), "PHC format")
}

func TestGenerateAPIKeySecret_DistinctEachCall(t *testing.T) {
	s1, h1, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	s2, h2, err := GenerateAPIKeySecret()
	require.NoError(t, err)
	require.NotEqual(t, s1, s2)
	require.NotEqual(t, h1, h2)
}

func TestFormatAPIKeyToken(t *testing.T) {
	token := FormatAPIKeyToken("abc12345", "thirtytwocharsecretthirtytwocha0")
	require.True(t, strings.HasPrefix(token, apiKeyPrefix))
	require.Equal(t, apiKeyPrefix+"abc12345_thirtytwocharsecretthirtytwocha0", token)
	require.Equal(t, apiKeyPrefix, APIKeyTokenPrefix())
}
