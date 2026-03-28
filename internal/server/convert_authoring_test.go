// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"testing"
	"time"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ConversationLog ---

func TestConversationLogToProto(t *testing.T) {
	now := time.Date(2026, 3, 28, 12, 0, 0, 0, time.UTC)
	entry := &storage.ConversationLogEntry{
		ID:      "conv-1",
		Stage:   "spark",
		Version: 1,
		IsAmend: false,
		Exchanges: []storage.ConversationExchange{
			{Role: storage.ConversationRoleProbe, Content: "What problem?", Stage: "spark", Sequence: 1, DecisionPoint: false},
			{Role: storage.ConversationRoleResponse, Content: "Auth is broken", Stage: "spark", Sequence: 2, DecisionPoint: true},
		},
		ExchangeCount: 2,
		Date:          now,
	}

	pb := conversationLogToProto(entry)
	require.NotNil(t, pb)
	assert.Equal(t, "conv-1", pb.Id)
	assert.Equal(t, "spark", pb.Stage)
	assert.Equal(t, int32(1), pb.Version)
	assert.False(t, pb.IsAmend)
	assert.Equal(t, int32(2), pb.ExchangeCount)
	assert.Equal(t, now.Unix(), pb.Date.AsTime().Unix())

	require.Len(t, pb.Exchanges, 2)
	assert.Equal(t, "probe", pb.Exchanges[0].Role)
	assert.Equal(t, "What problem?", pb.Exchanges[0].Content)
	assert.Equal(t, int32(1), pb.Exchanges[0].Sequence)
	assert.False(t, pb.Exchanges[0].DecisionPoint)
	assert.Equal(t, "response", pb.Exchanges[1].Role)
	assert.True(t, pb.Exchanges[1].DecisionPoint)
}

func TestConversationLogToProto_Nil(t *testing.T) {
	assert.Nil(t, conversationLogToProto(nil))
}

func TestConversationLogToProto_EmptyExchanges(t *testing.T) {
	entry := &storage.ConversationLogEntry{
		ID:    "conv-empty",
		Stage: "shape",
	}
	pb := conversationLogToProto(entry)
	require.NotNil(t, pb)
	assert.Empty(t, pb.Exchanges)
}

func TestConversationExchangesFromProto(t *testing.T) {
	protos := []*specv1.ConversationExchange{
		{Role: "probe", Content: "What?", Stage: "spark", Sequence: 1, DecisionPoint: false},
		{Role: "response", Content: "This.", Stage: "spark", Sequence: 2, DecisionPoint: true},
	}
	result, err := conversationExchangesFromProto(protos)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, storage.ConversationRoleProbe, result[0].Role)
	assert.Equal(t, "What?", result[0].Content)
	assert.Equal(t, int32(1), result[0].Sequence)
	assert.False(t, result[0].DecisionPoint)
	assert.Equal(t, storage.ConversationRoleResponse, result[1].Role)
	assert.True(t, result[1].DecisionPoint)
}

func TestConversationExchangesFromProto_Empty(t *testing.T) {
	result, err := conversationExchangesFromProto(nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestConversationExchangesFromProto_InvalidRole(t *testing.T) {
	protos := []*specv1.ConversationExchange{
		{Role: "invalid-role", Content: "test"},
	}
	_, err := conversationExchangesFromProto(protos)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid conversation role")
}

// --- Scope sniff ---

func TestScopeSniffStringToProto(t *testing.T) {
	tests := []struct {
		input string
		want  specv1.ScopeSniff
	}{
		{"tiny", specv1.ScopeSniff_SCOPE_SNIFF_TINY},
		{"small", specv1.ScopeSniff_SCOPE_SNIFF_SMALL},
		{"medium", specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM},
		{"large", specv1.ScopeSniff_SCOPE_SNIFF_LARGE},
		{"epic", specv1.ScopeSniff_SCOPE_SNIFF_EPIC},
		{"", specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED},
		{"unknown", specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, scopeSniffStringToProto(tt.input))
		})
	}
}

// --- Spark ---

func TestSparkOutputToProto(t *testing.T) {
	o := &storage.SparkOutput{
		Seed:       "fix login",
		Signal:     "users can't authenticate",
		Questions:  []string{"which provider?", "what scope?"},
		ScopeSniff: "small",
		KillTest:   "users can log in within 2s",
	}
	pb := sparkOutputToProto(o)
	require.NotNil(t, pb)
	assert.Equal(t, "fix login", pb.Seed)
	assert.Equal(t, "users can't authenticate", pb.Signal)
	assert.Equal(t, []string{"which provider?", "what scope?"}, pb.Questions)
	assert.Equal(t, specv1.ScopeSniff_SCOPE_SNIFF_SMALL, pb.ScopeSniff)
	assert.Equal(t, "users can log in within 2s", pb.KillTest)
}

func TestSparkOutputToProto_Nil(t *testing.T) {
	assert.Nil(t, sparkOutputToProto(nil))
}

// --- Shape ---

func TestShapeOutputToProto(t *testing.T) {
	o := &storage.ShapeOutput{
		ScopeIn:        []string{"login flow"},
		ScopeOut:       []string{"registration"},
		Approaches:     []storage.Approach{{Name: "OAuth", Description: "use OAuth2", Tradeoffs: []string{"complexity"}}},
		ChosenApproach: "OAuth",
		Risks:          []string{"token expiry"},
		SuccessMust:    []string{"login works"},
		SuccessShould:  []string{"token refresh"},
		SuccessWont:    []string{"social login"},
		Decisions:      []storage.DecisionInput{{Slug: "use-oauth", Title: "Use OAuth", Body: "yes", Rationale: "standard"}},
	}
	pb := shapeOutputToProto(o)
	require.NotNil(t, pb)
	assert.Equal(t, []string{"login flow"}, pb.ScopeIn)
	assert.Equal(t, []string{"registration"}, pb.ScopeOut)
	assert.Equal(t, "OAuth", pb.ChosenApproach)
	assert.Equal(t, []string{"token expiry"}, pb.Risks)
	assert.Equal(t, []string{"login works"}, pb.SuccessMust)
	assert.Equal(t, []string{"token refresh"}, pb.SuccessShould)
	assert.Equal(t, []string{"social login"}, pb.SuccessWont)

	require.Len(t, pb.Approaches, 1)
	assert.Equal(t, "OAuth", pb.Approaches[0].Name)
	assert.Equal(t, "use OAuth2", pb.Approaches[0].Description)
	assert.Equal(t, []string{"complexity"}, pb.Approaches[0].Tradeoffs)

	require.Len(t, pb.Decisions, 1)
	assert.Equal(t, "use-oauth", pb.Decisions[0].Slug)
	assert.Equal(t, "Use OAuth", pb.Decisions[0].Title)
	assert.Equal(t, "yes", pb.Decisions[0].Decision)
	assert.Equal(t, "standard", pb.Decisions[0].Rationale)
}

func TestShapeOutputToProto_Nil(t *testing.T) {
	assert.Nil(t, shapeOutputToProto(nil))
}

func TestShapeOutputToProto_Empty(t *testing.T) {
	o := &storage.ShapeOutput{}
	pb := shapeOutputToProto(o)
	require.NotNil(t, pb)
	assert.Empty(t, pb.Approaches)
	assert.Empty(t, pb.Decisions)
}

// --- Specify ---

func TestSpecifyOutputToProto(t *testing.T) {
	o := &storage.SpecifyOutput{
		Interfaces: []storage.InterfaceSection{
			{Name: "AuthService", Body: "Login(ctx, creds) -> token"},
		},
		VerifyCriteria: []storage.VerifyCriterion{
			{Category: "auth", Description: "valid creds return 200"},
		},
		Invariants: []string{"sessions are stateless"},
		Touches: []storage.FileTouch{
			{Path: "internal/auth/handler.go", Purpose: "login endpoint", ChangeType: "create"},
		},
	}
	pb := specifyOutputToProto(o)
	require.NotNil(t, pb)

	require.Len(t, pb.Interfaces, 1)
	assert.Equal(t, "AuthService", pb.Interfaces[0].Name)
	assert.Equal(t, "Login(ctx, creds) -> token", pb.Interfaces[0].Body)

	require.Len(t, pb.VerifyCriteria, 1)
	assert.Equal(t, "auth", pb.VerifyCriteria[0].Category)
	assert.Equal(t, "valid creds return 200", pb.VerifyCriteria[0].Description)

	assert.Equal(t, []string{"sessions are stateless"}, pb.Invariants)

	require.Len(t, pb.Touches, 1)
	assert.Equal(t, "internal/auth/handler.go", pb.Touches[0].Path)
	assert.Equal(t, "login endpoint", pb.Touches[0].Purpose)
	assert.Equal(t, "create", pb.Touches[0].ChangeType)
}

func TestSpecifyOutputToProto_Nil(t *testing.T) {
	assert.Nil(t, specifyOutputToProto(nil))
}

// --- Decompose ---

func TestDecomposeStrategyStringToProto(t *testing.T) {
	tests := []struct {
		domain storage.DecompositionStrategy
		proto  specv1.DecompositionStrategy
	}{
		{storage.StrategyVerticalSlice, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE},
		{storage.StrategyLayerCake, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE},
		{storage.StrategySingleUnit, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT},
	}
	for _, tt := range tests {
		t.Run(string(tt.domain), func(t *testing.T) {
			got, err := decomposeStrategyStringToProto(tt.domain)
			require.NoError(t, err)
			assert.Equal(t, tt.proto, got)
		})
	}

	_, err := decomposeStrategyStringToProto("bogus")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown decomposition strategy")
}

func TestDecomposeOutputToProto(t *testing.T) {
	o := &storage.DecomposeOutput{
		Strategy: storage.StrategyVerticalSlice,
		Slices: []storage.DecomposeSlice{
			{ID: "s1", Intent: "auth endpoint", Verify: []string{"login works"}, Touches: []string{"auth.go"}, DependsOn: []string{}},
			{ID: "s2", Intent: "token refresh", Verify: []string{"refresh works"}, Touches: []string{"token.go"}, DependsOn: []string{"s1"}},
		},
		SliceSlugs: []string{"login/s1", "login/s2"},
	}
	pb, err := decomposeOutputToProto(o)
	require.NoError(t, err)
	require.NotNil(t, pb)
	assert.Equal(t, specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE, pb.Strategy)
	assert.Equal(t, []string{"login/s1", "login/s2"}, pb.SliceSlugs)

	require.Len(t, pb.Slices, 2)
	assert.Equal(t, "s1", pb.Slices[0].Id)
	assert.Equal(t, "auth endpoint", pb.Slices[0].Intent)
	assert.Equal(t, []string{"login works"}, pb.Slices[0].Verify)
	assert.Equal(t, []string{"auth.go"}, pb.Slices[0].Touches)
	assert.Empty(t, pb.Slices[0].DependsOn)
	assert.Equal(t, []string{"s1"}, pb.Slices[1].DependsOn)
}

func TestDecomposeOutputToProto_Nil(t *testing.T) {
	got, err := decomposeOutputToProto(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestDecomposeOutputToProto_InvalidStrategy(t *testing.T) {
	o := &storage.DecomposeOutput{Strategy: "bogus"}
	_, err := decomposeOutputToProto(o)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown decomposition strategy")
}

// --- DecompositionStrategy.IsValid ---

func TestDecompositionStrategyIsValid(t *testing.T) {
	assert.True(t, storage.StrategyVerticalSlice.IsValid())
	assert.True(t, storage.StrategyLayerCake.IsValid())
	assert.True(t, storage.StrategySingleUnit.IsValid())
	assert.False(t, storage.DecompositionStrategy("bogus").IsValid())
	assert.False(t, storage.DecompositionStrategy("").IsValid())
}
