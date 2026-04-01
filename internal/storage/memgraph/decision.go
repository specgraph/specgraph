// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package memgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v6/neo4j"
	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/contenthash"
)

// rejectedAltJSON is the JSON-serializable representation of a rejected alternative.
type rejectedAltJSON struct {
	Option string `json:"option"`
	Reason string `json:"reason"`
}

// marshalRejectedAlts serializes a slice of RejectedAlternative to a JSON string
// for storage as a property on the Decision node.
func marshalRejectedAlts(alts []storage.RejectedAlternative) string {
	if len(alts) == 0 {
		return "[]"
	}
	items := make([]rejectedAltJSON, len(alts))
	for i, a := range alts {
		items[i] = rejectedAltJSON{Option: a.Option, Reason: a.Reason}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalRejectedAlts deserializes a JSON string into a slice of RejectedAlternative.
func unmarshalRejectedAlts(raw string) ([]storage.RejectedAlternative, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var items []rejectedAltJSON
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("memgraph: unmarshal rejected alternatives: %w", err)
	}
	alts := make([]storage.RejectedAlternative, len(items))
	for i, item := range items {
		alts[i] = storage.RejectedAlternative{Option: item.Option, Reason: item.Reason}
	}
	return alts, nil
}

// marshalTags serializes a string slice to a JSON array string.
func marshalTags(tags []string) string {
	if len(tags) == 0 {
		return "[]"
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// unmarshalTags deserializes a JSON array string into a string slice.
func unmarshalTags(raw string) ([]string, error) {
	if raw == "" || raw == "[]" {
		return nil, nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, fmt.Errorf("memgraph: unmarshal tags: %w", err)
	}
	return tags, nil
}

// recordInt64Optional extracts an int64 from a neo4j record by position,
// returning 0 for nil or missing values (backward compatibility for nodes
// created before the field existed).
func recordInt64Optional(rec *neo4j.Record, pos int, field string) (int64, error) { //nolint:unparam // field kept for error context; currently only "version" but will expand
	if pos >= len(rec.Values) || rec.Values[pos] == nil {
		return 0, nil
	}
	v, ok := rec.Values[pos].(int64)
	if !ok {
		return 0, fmt.Errorf("memgraph: field %q at position %d: expected int64 or nil, got %T", field, pos, rec.Values[pos])
	}
	return v, nil
}

// toContentHashAlts converts storage rejected alternatives to contenthash rejected alternatives.
func toContentHashAlts(alts []storage.RejectedAlternative) []contenthash.RejectedAlt {
	if len(alts) == 0 {
		return nil
	}
	out := make([]contenthash.RejectedAlt, len(alts))
	for i, a := range alts {
		out[i] = contenthash.RejectedAlt{Option: a.Option, Reason: a.Reason}
	}
	return out
}

// decisionReturnCols is the RETURN clause shared by CreateDecision, GetDecision, ListDecisions, and UpdateDecision.
const decisionReturnCols = `d.id, d.slug, d.title, d.status, d.decision, d.rationale,
	       d.superseded_by, d.created_at, d.updated_at, d.content_hash,
	       d.question, d.rejected_alternatives_json, d.confidence,
	       d.tags_json, d.scope, d.origin_spec, d.origin_stage, d.version`

// CreateDecision stores a new decision node in Memgraph.
func (s *Store) CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
	rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
	tags []string, scope storage.DecisionScope, originSpec, originStage string) (*storage.Decision, error) {
	now := s.nowTime()
	id := newID("dec")
	nowStr := now.Format(time.RFC3339)
	initialStatus := string(storage.DecisionStatusProposed)
	ch := contenthash.Decision(title, initialStatus, body, rationale,
		question, string(confidence), string(scope), originSpec, originStage,
		tags, toContentHashAlts(rejectedAlts))

	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})
		CREATE (p)<-[:BELONGS_TO]-(d:Decision {
			id: $id,
			slug: $slug,
			title: $title,
			status: $status,
			decision: $decision,
			rationale: $rationale,
			superseded_by: $superseded_by,
			question: $question,
			rejected_alternatives_json: $rejected_alternatives_json,
			confidence: $confidence,
			tags_json: $tags_json,
			scope: $scope,
			origin_spec: $origin_spec,
			origin_stage: $origin_stage,
			version: $version,
			created_at: $created_at,
			updated_at: $updated_at,
			content_hash: $content_hash
		})
		RETURN %s
	`, decisionReturnCols)
	params := mergeParams(s.projectParam(), map[string]any{
		"id":                         id,
		"slug":                       slug,
		"title":                      title,
		"status":                     initialStatus,
		"decision":                   body,
		"rationale":                  rationale,
		"superseded_by":              "",
		"question":                   question,
		"rejected_alternatives_json": marshalRejectedAlts(rejectedAlts),
		"confidence":                 string(confidence),
		"tags_json":                  marshalTags(tags),
		"scope":                      string(scope),
		"origin_spec":                originSpec,
		"origin_stage":               originStage,
		"version":                    int64(1),
		"created_at":                 nowStr,
		"updated_at":                 nowStr,
		"content_hash":               ch,
	})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: create decision: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: create decision returned no records")
	}

	return recordToDecision(records[0])
}

// GetDecision retrieves a decision by slug.
func (s *Store) GetDecision(ctx context.Context, slug string) (*storage.Decision, error) {
	query := fmt.Sprintf(`
		MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})
		RETURN %s
	`, decisionReturnCols)
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: get decision: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("memgraph: decision %q: %w", slug, storage.ErrDecisionNotFound)
	}

	return recordToDecision(records[0])
}

// ListDecisions returns decisions matching the given filters.
func (s *Store) ListDecisions(ctx context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error) {
	var clauses []string
	params := s.projectParam()

	if status != "" {
		// Match both new lowercase and legacy proto-style values for backward compatibility.
		clauses = append(clauses, "d.status IN $statuses")
		statuses := []string{string(status)}
		if legacy := legacyDecisionStatus(status); legacy != "" {
			statuses = append(statuses, legacy)
		}
		// Include unspecified/empty legacy values when filtering for "proposed",
		// since the read path normalizes DECISION_STATUS_UNSPECIFIED and "" to proposed.
		if status == storage.DecisionStatusProposed {
			statuses = append(statuses, "DECISION_STATUS_UNSPECIFIED", "")
		}
		params["statuses"] = statuses
	}

	query := "MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision)"
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " RETURN " + decisionReturnCols
	query += " ORDER BY d.created_at"
	if limit > 0 {
		query += " LIMIT $limit"
		params["limit"] = int64(limit)
	}

	records, err := s.executeQuery(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("memgraph: list decisions: %w", err)
	}

	decisions := make([]*storage.Decision, 0, len(records))
	for _, rec := range records {
		d, err := recordToDecision(rec)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, d)
	}
	return decisions, nil
}

// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
func (s *Store) UpdateDecision(ctx context.Context, slug string, expectedVersion int32, title *string, status *storage.DecisionStatus,
	body, rationale, supersededBy, question *string,
	rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
	tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string) (*storage.Decision, error) {
	if status != nil && *status == storage.DecisionStatusSuperseded {
		if supersededBy == nil || *supersededBy == "" {
			return nil, storage.ErrSupersededByRequired
		}
	}

	var setClauses []string
	params := mergeParams(s.projectParam(), map[string]any{"slug": slug})

	if title != nil {
		setClauses = append(setClauses, "d.title = $title")
		params["title"] = *title
	}
	if status != nil {
		setClauses = append(setClauses, "d.status = $status")
		params["status"] = string(*status)
	}
	if body != nil {
		setClauses = append(setClauses, "d.decision = $decision")
		params["decision"] = *body
	}
	if rationale != nil {
		setClauses = append(setClauses, "d.rationale = $rationale")
		params["rationale"] = *rationale
	}
	if supersededBy != nil {
		setClauses = append(setClauses, "d.superseded_by = $superseded_by")
		params["superseded_by"] = *supersededBy
	}
	if question != nil {
		setClauses = append(setClauses, "d.question = $question")
		params["question"] = *question
	}
	if rejectedAlts != nil {
		setClauses = append(setClauses, "d.rejected_alternatives_json = $rejected_alternatives_json")
		params["rejected_alternatives_json"] = marshalRejectedAlts(*rejectedAlts)
	}
	if confidence != nil {
		setClauses = append(setClauses, "d.confidence = $confidence")
		params["confidence"] = string(*confidence)
	}
	if tags != nil {
		setClauses = append(setClauses, "d.tags_json = $tags_json")
		params["tags_json"] = marshalTags(*tags)
	}
	if scope != nil {
		setClauses = append(setClauses, "d.scope = $scope")
		params["scope"] = string(*scope)
	}
	if originSpec != nil {
		setClauses = append(setClauses, "d.origin_spec = $origin_spec")
		params["origin_spec"] = *originSpec
	}
	if originStage != nil {
		setClauses = append(setClauses, "d.origin_stage = $origin_stage")
		params["origin_stage"] = *originStage
	}

	if len(setClauses) == 0 {
		return s.GetDecision(ctx, slug)
	}

	nowStr := s.nowTime().Format(time.RFC3339)
	setClauses = append(setClauses, "d.updated_at = $updated_at")
	params["updated_at"] = nowStr
	setClauses = append(setClauses, "d.version = d.version + 1")

	matchClause := `MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})`
	if expectedVersion > 0 {
		matchClause += "\nWHERE d.version = $expected_version"
		params["expected_version"] = int64(expectedVersion)
	}

	query := fmt.Sprintf(`
		%s
		SET %s
		RETURN %s
	`, matchClause, strings.Join(setClauses, ", "), decisionReturnCols)

	var result *storage.Decision
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		// Fetch old state before mutation for field delta computation.
		oldDec, getErr := s.GetDecision(txCtx, slug)
		if getErr != nil {
			return getErr
		}
		oldFields := decisionToFields(oldDec)

		records, qErr := s.executeQuery(txCtx, query, params)
		if qErr != nil {
			return fmt.Errorf("memgraph: update decision: %w", qErr)
		}
		if len(records) == 0 {
			if expectedVersion > 0 {
				return fmt.Errorf("memgraph: decision %q version %d: %w", slug, expectedVersion, storage.ErrConcurrentModification)
			}
			return fmt.Errorf("memgraph: decision %q: %w", slug, storage.ErrDecisionNotFound)
		}

		dec, parseErr := recordToDecision(records[0])
		if parseErr != nil {
			return parseErr
		}
		ch := contenthash.Decision(dec.Title, string(dec.Status), dec.Body, dec.Rationale,
			dec.Question, string(dec.Confidence), string(dec.Scope),
			dec.OriginSpec, dec.OriginStage,
			dec.Tags, toContentHashAlts(dec.RejectedAlternatives))

		hashQuery := `
			MATCH (p:Project {slug: $project})<-[:BELONGS_TO]-(d:Decision {slug: $slug})
			SET d.content_hash = $content_hash
		`
		if _, hashErr := s.executeQuery(txCtx, hashQuery, mergeParams(s.projectParam(), map[string]any{
			"slug":         slug,
			"content_hash": ch,
		})); hashErr != nil {
			return fmt.Errorf("memgraph: update decision content_hash: %w", hashErr)
		}
		dec.ContentHash = ch

		// Compute field deltas and create ChangeLog entry.
		newFields := decisionToFields(dec)
		deltas := storage.ComputeDecisionFieldDeltas(oldFields, newFields)
		if len(deltas) > 0 {
			clEntry := &storage.ChangeLogEntry{
				Version:     int32(dec.Version), //nolint:gosec // version is a small monotonic counter; overflow impossible
				Stage:       string(dec.Status),
				ContentHash: dec.ContentHash,
				Summary:     "decision updated",
				Date:        s.nowTime(),
			}
			if clErr := s.createChangeLog(txCtx, "Decision", slug, clEntry, deltas); clErr != nil {
				return clErr
			}
		}

		result = dec
		return nil
	})
	return result, err
}

// decisionToFields converts a Decision to DecisionFields for delta computation.
func decisionToFields(d *storage.Decision) *storage.DecisionFields {
	return &storage.DecisionFields{
		Title:                d.Title,
		Status:               string(d.Status),
		Body:                 d.Body,
		Rationale:            d.Rationale,
		Question:             d.Question,
		Confidence:           string(d.Confidence),
		Scope:                string(d.Scope),
		Tags:                 marshalTags(d.Tags),
		RejectedAlternatives: marshalRejectedAlts(d.RejectedAlternatives),
		OriginSpec:           d.OriginSpec,
		OriginStage:          d.OriginStage,
	}
}

// legacyDecisionStatus returns the old proto-style status string for backward-compatible queries.
func legacyDecisionStatus(s storage.DecisionStatus) string {
	switch s {
	case storage.DecisionStatusProposed:
		return "DECISION_STATUS_PROPOSED"
	case storage.DecisionStatusAccepted:
		return "DECISION_STATUS_ACCEPTED"
	case storage.DecisionStatusSuperseded:
		return "DECISION_STATUS_SUPERSEDED"
	case storage.DecisionStatusDeprecated:
		return "DECISION_STATUS_DEPRECATED"
	default:
		return ""
	}
}

// normalizeDecisionStatus handles both old proto-style values from existing
// Memgraph data and the new lowercase values. New writes use lowercase;
// this function ensures old data is read correctly.
func normalizeDecisionStatus(raw string) storage.DecisionStatus {
	switch raw {
	case "DECISION_STATUS_PROPOSED":
		return storage.DecisionStatusProposed
	case "DECISION_STATUS_ACCEPTED":
		return storage.DecisionStatusAccepted
	case "DECISION_STATUS_SUPERSEDED":
		return storage.DecisionStatusSuperseded
	case "DECISION_STATUS_DEPRECATED":
		return storage.DecisionStatusDeprecated
	default:
		return storage.DecisionStatus(raw)
	}
}

func recordToDecision(rec *neo4j.Record) (*storage.Decision, error) {
	id, err := recordString(rec, 0, "id")
	if err != nil {
		return nil, err
	}
	slug, err := recordString(rec, 1, "slug")
	if err != nil {
		return nil, err
	}
	title, err := recordString(rec, 2, "title")
	if err != nil {
		return nil, err
	}
	statusStr, err := recordString(rec, 3, "status")
	if err != nil {
		return nil, err
	}
	body, err := recordString(rec, 4, "decision")
	if err != nil {
		return nil, err
	}
	rationale, err := recordString(rec, 5, "rationale")
	if err != nil {
		return nil, err
	}
	supersededBy, err := recordString(rec, 6, "superseded_by")
	if err != nil {
		return nil, err
	}
	createdAtStr, err := recordString(rec, 7, "created_at")
	if err != nil {
		return nil, err
	}
	updatedAtStr, err := recordString(rec, 8, "updated_at")
	if err != nil {
		return nil, err
	}
	contentHash, err := recordStringOptional(rec, 9, "content_hash")
	if err != nil {
		return nil, err
	}
	questionStr, err := recordStringOptional(rec, 10, "question")
	if err != nil {
		return nil, err
	}
	rejectedAltsJSON, err := recordStringOptional(rec, 11, "rejected_alternatives_json")
	if err != nil {
		return nil, err
	}
	confidenceStr, err := recordStringOptional(rec, 12, "confidence")
	if err != nil {
		return nil, err
	}
	tagsJSON, err := recordStringOptional(rec, 13, "tags_json")
	if err != nil {
		return nil, err
	}
	scopeStr, err := recordStringOptional(rec, 14, "scope")
	if err != nil {
		return nil, err
	}
	originSpec, err := recordStringOptional(rec, 15, "origin_spec")
	if err != nil {
		return nil, err
	}
	originStage, err := recordStringOptional(rec, 16, "origin_stage")
	if err != nil {
		return nil, err
	}
	version, err := recordInt64Optional(rec, 17, "version")
	if err != nil {
		return nil, err
	}

	rejectedAlts, err := unmarshalRejectedAlts(rejectedAltsJSON)
	if err != nil {
		return nil, err
	}
	tags, err := unmarshalTags(tagsJSON)
	if err != nil {
		return nil, err
	}

	createdAt, err := parseRFC3339("created_at", createdAtStr)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseRFC3339("updated_at", updatedAtStr)
	if err != nil {
		return nil, err
	}

	var status storage.DecisionStatus
	if statusStr == "DECISION_STATUS_UNSPECIFIED" || statusStr == "" {
		status = storage.DecisionStatusProposed
	} else {
		status = normalizeDecisionStatus(statusStr)
		if !status.IsValid() {
			return nil, fmt.Errorf("memgraph: unknown decision status %q", statusStr)
		}
	}

	return &storage.Decision{
		ID:                   id,
		Slug:                 slug,
		Title:                title,
		Status:               status,
		Body:                 body,
		Rationale:            rationale,
		SupersededBy:         supersededBy,
		Question:             questionStr,
		RejectedAlternatives: rejectedAlts,
		Confidence:           storage.DecisionConfidence(confidenceStr),
		Tags:                 tags,
		Scope:                storage.DecisionScope(scopeStr),
		OriginSpec:           originSpec,
		OriginStage:          originStage,
		Version:              int(version),
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
		ContentHash:          contentHash,
	}, nil
}
