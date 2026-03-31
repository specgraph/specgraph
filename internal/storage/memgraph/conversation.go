// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"

	"github.com/specgraph/specgraph/internal/storage"
)

// Compile-time interface assertion.
var _ storage.ConversationBackend = (*Store)(nil)

// conversationExchangeJSON is the JSON-serializable representation of an exchange.
type conversationExchangeJSON struct {
	Role          string `json:"role"`
	Content       string `json:"content"`
	Stage         string `json:"stage"`
	Sequence      int32  `json:"sequence"`
	DecisionPoint bool   `json:"decision_point,omitempty"`
}

// marshalExchanges serializes exchanges to a JSON string for storage.
func marshalExchanges(exchanges []storage.ConversationExchange) (string, error) {
	items := make([]conversationExchangeJSON, len(exchanges))
	for i, e := range exchanges {
		items[i] = conversationExchangeJSON{
			Role:          string(e.Role),
			Content:       e.Content,
			Stage:         e.Stage,
			Sequence:      e.Sequence,
			DecisionPoint: e.DecisionPoint,
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("marshal exchanges: %w", err)
	}
	return string(b), nil
}

// unmarshalExchanges deserializes exchanges from a JSON string.
func unmarshalExchanges(raw string) ([]storage.ConversationExchange, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []conversationExchangeJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("unmarshal exchanges: %w", err)
	}
	result := make([]storage.ConversationExchange, len(items))
	for i, item := range items {
		result[i] = storage.ConversationExchange{
			Role:          storage.ConversationRole(item.Role),
			Content:       item.Content,
			Stage:         item.Stage,
			Sequence:      item.Sequence,
			DecisionPoint: item.DecisionPoint,
		}
	}
	return result, nil
}

// RecordConversation stores a conversation log for a spec stage.
func (s *Store) RecordConversation(ctx context.Context, slug string, entry storage.ConversationLogEntry) (*storage.ConversationLogEntry, error) { //nolint:gocritic // hugeParam: entry is a value type by interface contract
	var result *storage.ConversationLogEntry

	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// 1. Verify spec exists and get current version.
		specQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			RETURN s.version AS version
		`
		specRecords, specErr := s.executeQuery(txCtx, specQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if specErr != nil {
			return fmt.Errorf("memgraph: record conversation: verify spec: %w", specErr)
		}
		if len(specRecords) == 0 {
			return storage.ErrSpecNotFound
		}
		version, vErr := recordInt64(specRecords[0], 0, "version")
		if vErr != nil {
			return vErr
		}

		// 2. Find the most recent ChangeLog for this stage+version (for EXPLAINS edge).
		changeLogQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:HAS_CHANGE]->(cl:ChangeLog)
			WHERE cl.stage = $stage AND cl.version = $version
			RETURN cl.id AS cl_id
			ORDER BY cl.date DESC
			LIMIT 1
		`
		clParams := mergeParams(s.projectParam(), map[string]any{
			"slug":    slug,
			"stage":   string(entry.Stage),
			"version": version,
		})
		clRecords, clErr := s.executeQuery(txCtx, changeLogQuery, clParams)
		if clErr != nil {
			return fmt.Errorf("memgraph: record conversation: find changelog: %w", clErr)
		}
		var changeLogID string
		if len(clRecords) > 0 {
			changeLogID, _ = recordString(clRecords[0], 0, "cl_id") //nolint:errcheck // best-effort; empty string means no EXPLAINS edge
		}

		// 3. Find the current CONTINUES chain tail.
		// Memgraph does not support pattern expressions in WHERE (e.g., NOT (x)-[:R]->())
		// so we traverse the chain and find nodes with no outgoing CONTINUES edge.
		tailQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			      -[:AUTHORED_VIA]->(first:ConversationLog)
			OPTIONAL MATCH (first)-[:CONTINUES*0..]->(reachable:ConversationLog)
			WITH coalesce(reachable, first) AS candidate
			OPTIONAL MATCH (candidate)-[:CONTINUES]->(next:ConversationLog)
			WITH candidate WHERE next IS NULL
			RETURN candidate.id AS tail_id
		`
		tailRecords, tailErr := s.executeQuery(txCtx, tailQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
		if tailErr != nil {
			return fmt.Errorf("memgraph: record conversation: find tail: %w", tailErr)
		}
		var tailID string
		if len(tailRecords) > 0 {
			tailID, _ = recordString(tailRecords[0], 0, "tail_id") //nolint:errcheck // best-effort; empty string means first log
		}

		// 4. Create the ConversationLog node.
		id := newID("cvl")
		dateStr := s.now()
		exchangesJSON, mErr := marshalExchanges(entry.Exchanges)
		if mErr != nil {
			return mErr
		}

		createQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
			CREATE (cvl:ConversationLog {
				id: $id,
				stage: $stage,
				version: $version,
				is_amend: $is_amend,
				exchanges_json: $exchanges_json,
				exchange_count: $exchange_count,
				date: $date
			})
			RETURN cvl.id AS id
		`
		createParams := mergeParams(s.projectParam(), map[string]any{
			"slug":           slug,
			"id":             id,
			"stage":          string(entry.Stage),
			"version":        version,
			"is_amend":       entry.IsAmend,
			"exchanges_json": exchangesJSON,
			"exchange_count": int64(entry.ExchangeCount),
			"date":           dateStr,
		})
		createRecords, createErr := s.executeQuery(txCtx, createQuery, createParams)
		if createErr != nil {
			return fmt.Errorf("memgraph: record conversation: create node: %w", createErr)
		}
		if len(createRecords) == 0 {
			return fmt.Errorf("memgraph: record conversation: no rows returned from CREATE")
		}

		// 5a. AUTHORED_VIA edge (only if this is the first ConversationLog for this spec).
		if tailID == "" {
			edgeQuery := `
				MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}),
				      (cvl:ConversationLog {id: $cvl_id})
				CREATE (s)-[:AUTHORED_VIA]->(cvl)
			`
			_, edgeErr := s.executeQuery(txCtx, edgeQuery, mergeParams(s.projectParam(), map[string]any{
				"slug":   slug,
				"cvl_id": id,
			}))
			if edgeErr != nil {
				return fmt.Errorf("memgraph: record conversation: create AUTHORED_VIA: %w", edgeErr)
			}
		}

		// 5b. CONTINUES edge (from previous tail to this node).
		if tailID != "" {
			contQuery := `
				MATCH (prev:ConversationLog {id: $tail_id}),
				      (cvl:ConversationLog {id: $cvl_id})
				CREATE (prev)-[:CONTINUES]->(cvl)
			`
			_, contErr := s.executeQuery(txCtx, contQuery, map[string]any{
				"tail_id": tailID,
				"cvl_id":  id,
			})
			if contErr != nil {
				return fmt.Errorf("memgraph: record conversation: create CONTINUES: %w", contErr)
			}
		}

		// 5c. EXPLAINS edge (to the matching ChangeLog, if found).
		if changeLogID != "" {
			explQuery := `
				MATCH (cvl:ConversationLog {id: $cvl_id}),
				      (cl:ChangeLog {id: $cl_id})
				CREATE (cvl)-[:EXPLAINS]->(cl)
			`
			_, explErr := s.executeQuery(txCtx, explQuery, map[string]any{
				"cvl_id": id,
				"cl_id":  changeLogID,
			})
			if explErr != nil {
				return fmt.Errorf("memgraph: record conversation: create EXPLAINS: %w", explErr)
			}
		}

		result = &storage.ConversationLogEntry{
			ID:            id,
			Stage:         entry.Stage,
			Version:       safeInt32(version),
			IsAmend:       entry.IsAmend,
			Exchanges:     entry.Exchanges,
			ExchangeCount: entry.ExchangeCount,
			Date:          s.nowTime(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ListConversations returns conversation logs for a spec in narrative chain order.
func (s *Store) ListConversations(ctx context.Context, slug, stage string) ([]*storage.ConversationLogEntry, error) {
	// Verify spec exists.
	checkQuery := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug}) RETURN s.slug`
	checkRecords, err := s.executeQuery(ctx, checkQuery, mergeParams(s.projectParam(), map[string]any{"slug": slug}))
	if err != nil {
		return nil, fmt.Errorf("memgraph: list conversations: %w", err)
	}
	if len(checkRecords) == 0 {
		return nil, storage.ErrSpecNotFound
	}

	// Fetch conversation logs in chain order.
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec {slug: $slug})
		      -[:AUTHORED_VIA]->(first:ConversationLog)
		OPTIONAL MATCH path = (first)-[:CONTINUES*0..]->(log)
		RETURN log.id AS id,
		       log.stage AS stage,
		       log.version AS version,
		       log.is_amend AS is_amend,
		       log.exchanges_json AS exchanges_json,
		       log.exchange_count AS exchange_count,
		       log.date AS date
		ORDER BY log.date
	`
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, qErr := s.executeQuery(ctx, query, params)
	if qErr != nil {
		return nil, fmt.Errorf("memgraph: list conversations: %w", qErr)
	}

	var entries []*storage.ConversationLogEntry
	for _, rec := range records {
		e, pErr := recordToConversationLogEntry(rec)
		if pErr != nil {
			return nil, pErr
		}
		if stage != "" && string(e.Stage) != stage {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// recordToConversationLogEntry parses a neo4j record into a ConversationLogEntry.
func recordToConversationLogEntry(rec *neo4j.Record) (*storage.ConversationLogEntry, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	stageStr, err := recordString(rec, 1, "stage")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64(rec, 2, "version")
	if err != nil {
		return nil, err
	}
	isAmend, ok := rec.Values[3].(bool)
	if !ok {
		return nil, fmt.Errorf("memgraph: conversation log: expected bool for is_amend, got %T", rec.Values[3])
	}
	exchangesJSON, err := recordString(rec, 4, "exchanges_json")
	if err != nil {
		return nil, err
	}
	exchangeCount, err := recordInt64(rec, 5, "exchange_count")
	if err != nil {
		return nil, err
	}
	dateStr, err := recordString(rec, 6, "date")
	if err != nil {
		return nil, err
	}

	exchanges, uErr := unmarshalExchanges(exchangesJSON)
	if uErr != nil {
		return nil, uErr
	}
	date, tErr := parseRFC3339("date", dateStr)
	if tErr != nil {
		return nil, tErr
	}

	return &storage.ConversationLogEntry{
		ID:            id,
		Stage:         storage.SpecStage(stageStr),
		Version:       safeInt32(version),
		IsAmend:       isAmend,
		Exchanges:     exchanges,
		ExchangeCount: safeInt32(exchangeCount),
		Date:          date,
	}, nil
}

// ListAllConversations returns all conversation logs across all specs in the project.
// SpecSlug is populated from the spec_slug column for each entry.
func (s *Store) ListAllConversations(ctx context.Context) ([]*storage.ConversationLogEntry, error) {
	query := `
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(s:Spec)-[:AUTHORED_VIA]->(first:ConversationLog)
		OPTIONAL MATCH path = (first)-[:CONTINUES*0..]->(log:ConversationLog)
		RETURN log.id, s.slug AS spec_slug, log.stage, log.version, log.is_amend,
		       log.exchanges_json, log.exchange_count, log.date
		ORDER BY s.slug, log.date
	`
	records, err := s.executeQuery(ctx, query, s.projectParam())
	if err != nil {
		return nil, fmt.Errorf("memgraph: list all conversations: %w", err)
	}

	entries := make([]*storage.ConversationLogEntry, 0, len(records))
	for _, rec := range records {
		id, err := recordString(rec, 0, "log.id")
		if err != nil {
			return nil, err
		}
		specSlug, err := recordString(rec, 1, "spec_slug")
		if err != nil {
			return nil, err
		}
		stageStr, err := recordString(rec, 2, "log.stage")
		if err != nil {
			return nil, err
		}
		version, err := recordInt64(rec, 3, "log.version")
		if err != nil {
			return nil, err
		}
		isAmend, ok := rec.Values[4].(bool)
		if !ok {
			return nil, fmt.Errorf("memgraph: conversation log: expected bool for is_amend, got %T", rec.Values[4])
		}
		exchangesJSON, err := recordString(rec, 5, "log.exchanges_json")
		if err != nil {
			return nil, err
		}
		exchangeCount, err := recordInt64(rec, 6, "log.exchange_count")
		if err != nil {
			return nil, err
		}
		dateStr, err := recordString(rec, 7, "log.date")
		if err != nil {
			return nil, err
		}
		exchanges, err := unmarshalExchanges(exchangesJSON)
		if err != nil {
			return nil, err
		}
		date, err := parseRFC3339("log.date", dateStr)
		if err != nil {
			return nil, err
		}

		entries = append(entries, &storage.ConversationLogEntry{
			ID:            id,
			SpecSlug:      specSlug,
			Stage:         storage.SpecStage(stageStr),
			Version:       safeInt32(version),
			IsAmend:       isAmend,
			Exchanges:     exchanges,
			ExchangeCount: safeInt32(exchangeCount),
			Date:          date,
		})
	}
	return entries, nil
}

// EnsureConversationLogIndexes creates indexes on ConversationLog nodes.
// Called from ensureIndexes during Store initialization.
func (s *Store) EnsureConversationLogIndexes(ctx context.Context) error {
	return runDDLStatements(ctx, s.driver, []string{
		"CREATE INDEX ON :ConversationLog(id)",
		"CREATE INDEX ON :ConversationLog(date)",
	})
}
