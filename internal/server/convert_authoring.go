// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- ConversationLog ---

// conversationLogToProto converts a storage ConversationLogEntry to a proto ConversationLog.
func conversationLogToProto(entry *storage.ConversationLogEntry) *specv1.ConversationLog {
	if entry == nil {
		return nil
	}
	exchanges := make([]*specv1.ConversationExchange, len(entry.Exchanges))
	for i, e := range entry.Exchanges {
		exchanges[i] = &specv1.ConversationExchange{
			Role:          string(e.Role),
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	return &specv1.ConversationLog{
		Id:            entry.ID,
		Stage:         string(entry.Stage),
		Version:       entry.Version,
		IsAmend:       entry.IsAmend,
		Exchanges:     exchanges,
		ExchangeCount: entry.ExchangeCount,
		Date:          timeToProto(entry.Date),
	}
}

// conversationExchangesFromProto converts proto exchanges to storage domain types.
func conversationExchangesFromProto(exchanges []*specv1.ConversationExchange) ([]storage.ConversationExchange, error) {
	result := make([]storage.ConversationExchange, len(exchanges))
	for i, e := range exchanges {
		role := storage.ConversationRole(e.Role)
		if !role.IsValid() {
			return nil, fmt.Errorf("invalid conversation role: %q", e.Role)
		}
		result[i] = storage.ConversationExchange{
			Role:          role,
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	return result, nil
}

// --- Stage output domain → proto converters ---

var scopeSniffStringToProtoMap = map[string]specv1.ScopeSniff{
	"":       specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED,
	"tiny":   specv1.ScopeSniff_SCOPE_SNIFF_TINY,
	"small":  specv1.ScopeSniff_SCOPE_SNIFF_SMALL,
	"medium": specv1.ScopeSniff_SCOPE_SNIFF_MEDIUM,
	"large":  specv1.ScopeSniff_SCOPE_SNIFF_LARGE,
	"epic":   specv1.ScopeSniff_SCOPE_SNIFF_EPIC,
}

func scopeSniffStringToProto(s string) specv1.ScopeSniff {
	if v, ok := scopeSniffStringToProtoMap[s]; ok {
		return v
	}
	return specv1.ScopeSniff_SCOPE_SNIFF_UNSPECIFIED
}

func sparkOutputToProto(o *storage.SparkOutput) *specv1.SparkOutput {
	if o == nil {
		return nil
	}
	return &specv1.SparkOutput{
		Seed:       o.Seed,
		Signal:     o.Signal,
		Questions:  o.Questions,
		ScopeSniff: scopeSniffStringToProto(o.ScopeSniff),
		KillTest:   o.KillTest,
	}
}

func shapeOutputToProto(o *storage.ShapeOutput) *specv1.ShapeOutput {
	if o == nil {
		return nil
	}
	approaches := make([]*specv1.Approach, len(o.Approaches))
	for i, a := range o.Approaches {
		approaches[i] = &specv1.Approach{
			Name:        a.Name,
			Description: a.Description,
			Tradeoffs:   a.Tradeoffs,
		}
	}
	decisions := make([]*specv1.DecisionInput, len(o.Decisions))
	for i, d := range o.Decisions {
		decisions[i] = &specv1.DecisionInput{
			Slug:      d.Slug,
			Title:     d.Title,
			Decision:  d.Body,
			Rationale: d.Rationale,
		}
	}
	return &specv1.ShapeOutput{
		ScopeIn:        o.ScopeIn,
		ScopeOut:       o.ScopeOut,
		Approaches:     approaches,
		ChosenApproach: o.ChosenApproach,
		Risks:          o.Risks,
		SuccessMust:    o.SuccessMust,
		SuccessShould:  o.SuccessShould,
		SuccessWont:    o.SuccessWont,
		Decisions:      decisions,
	}
}

func specifyOutputToProto(o *storage.SpecifyOutput) *specv1.SpecifyOutput {
	if o == nil {
		return nil
	}
	interfaces := make([]*specv1.InterfaceSection, len(o.Interfaces))
	for i, iface := range o.Interfaces {
		interfaces[i] = &specv1.InterfaceSection{
			Name: iface.Name,
			Body: iface.Body,
		}
	}
	criteria := make([]*specv1.VerifyCriterion, len(o.VerifyCriteria))
	for i, vc := range o.VerifyCriteria {
		criteria[i] = &specv1.VerifyCriterion{
			Category:    vc.Category,
			Description: vc.Description,
		}
	}
	touches := make([]*specv1.FileTouch, len(o.Touches))
	for i, ft := range o.Touches {
		touches[i] = &specv1.FileTouch{
			Path:       ft.Path,
			Purpose:    ft.Purpose,
			ChangeType: ft.ChangeType,
		}
	}
	return &specv1.SpecifyOutput{
		Interfaces:     interfaces,
		VerifyCriteria: criteria,
		Invariants:     o.Invariants,
		Touches:        touches,
	}
}

var decomposeStrategyStringToProtoMap = map[storage.DecompositionStrategy]specv1.DecompositionStrategy{
	storage.StrategyVerticalSlice: specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_VERTICAL_SLICE,
	storage.StrategyLayerCake:     specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_LAYER_CAKE,
	storage.StrategySingleUnit:    specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_SINGLE_UNIT,
}

func decomposeStrategyStringToProto(s storage.DecompositionStrategy) (specv1.DecompositionStrategy, error) {
	if v, ok := decomposeStrategyStringToProtoMap[s]; ok {
		return v, nil
	}
	return specv1.DecompositionStrategy_DECOMPOSITION_STRATEGY_UNSPECIFIED, fmt.Errorf("unknown decomposition strategy: %q", s)
}

func decomposeOutputToProto(o *storage.DecomposeOutput) (*specv1.DecomposeOutput, error) {
	if o == nil {
		return nil, nil
	}
	strategy, err := decomposeStrategyStringToProto(o.Strategy)
	if err != nil {
		return nil, err
	}
	slices := make([]*specv1.DecompositionSlice, len(o.Slices))
	for i, s := range o.Slices {
		slices[i] = &specv1.DecompositionSlice{
			Id:        s.ID,
			Intent:    s.Intent,
			Verify:    s.Verify,
			Touches:   s.Touches,
			DependsOn: s.DependsOn,
		}
	}
	return &specv1.DecomposeOutput{
		Strategy:   strategy,
		Slices:     slices,
		SliceSlugs: o.SliceSlugs,
	}, nil
}
