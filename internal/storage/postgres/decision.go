// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/specgraph/specgraph/internal/storage"
	"github.com/specgraph/specgraph/internal/storage/contenthash"
)

// Compile-time interface assertion.
var _ storage.DecisionBackend = (*Store)(nil)

// decisionRejectedAltJSON is the JSON-serializable representation of a rejected alternative.
type decisionRejectedAltJSON struct {
	Option string `json:"option"`
	Reason string `json:"reason"`
}

// marshalRejectedAlts serializes a slice of RejectedAlternative to JSON bytes
// for storage in the rejected_alternatives JSONB column.
func marshalRejectedAlts(alts []storage.RejectedAlternative) ([]byte, error) {
	if len(alts) == 0 {
		return []byte("[]"), nil
	}
	items := make([]decisionRejectedAltJSON, len(alts))
	for i, a := range alts {
		items[i] = decisionRejectedAltJSON{Option: a.Option, Reason: a.Reason}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("postgres: marshal rejected alternatives: %w", err)
	}
	return b, nil
}

// unmarshalRejectedAlts deserializes JSONB bytes into a slice of RejectedAlternative.
func unmarshalRejectedAlts(data []byte) ([]storage.RejectedAlternative, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var items []decisionRejectedAltJSON
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, fmt.Errorf("postgres: unmarshal rejected alternatives: %w", err)
	}
	if len(items) == 0 {
		return nil, nil
	}
	alts := make([]storage.RejectedAlternative, len(items))
	for i, item := range items {
		alts[i] = storage.RejectedAlternative{Option: item.Option, Reason: item.Reason}
	}
	return alts, nil
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

// marshalTagsForDelta serializes a string slice to a JSON string for field-delta comparison.
func marshalTagsForDelta(tags []string) (string, error) {
	if len(tags) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return "", fmt.Errorf("postgres: marshal tags for delta: %w", err)
	}
	return string(b), nil
}

// marshalRejectedAltsForDelta serializes rejected alternatives to a JSON string for field-delta comparison.
func marshalRejectedAltsForDelta(alts []storage.RejectedAlternative) (string, error) {
	if len(alts) == 0 {
		return "[]", nil
	}
	items := make([]decisionRejectedAltJSON, len(alts))
	for i, a := range alts {
		items[i] = decisionRejectedAltJSON{Option: a.Option, Reason: a.Reason}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("postgres: marshal rejected alternatives for delta: %w", err)
	}
	return string(b), nil
}

// normalizeDecisionStatus handles legacy proto-style status values.
func normalizeDecisionStatus(raw string) storage.DecisionStatus {
	switch raw {
	case "DECISION_STATUS_PROPOSED", "DECISION_STATUS_UNSPECIFIED", "":
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

// decisionToFields converts a Decision to DecisionFields for delta computation.
func decisionToFields(d *storage.Decision) (*storage.DecisionFields, error) {
	tags, err := marshalTagsForDelta(d.Tags)
	if err != nil {
		return nil, err
	}
	rejAlts, err := marshalRejectedAltsForDelta(d.RejectedAlternatives)
	if err != nil {
		return nil, err
	}
	return &storage.DecisionFields{
		Title:                d.Title,
		Status:               string(d.Status),
		Body:                 d.Body,
		Rationale:            d.Rationale,
		Question:             d.Question,
		Confidence:           string(d.Confidence),
		Scope:                string(d.Scope),
		Tags:                 tags,
		RejectedAlternatives: rejAlts,
		OriginSpec:           d.OriginSpec,
		OriginStage:          d.OriginStage,
	}, nil
}

// decisionSelectCols is the SELECT column list shared by GetDecision and ListDecisions.
const decisionSelectCols = `d.id, d.slug, d.title, d.status, d.body, d.rationale, d.question, d.superseded_by,
       d.confidence, d.scope, d.origin_spec, d.origin_stage,
       d.tags, d.rejected_alternatives, d.content_hash, d.version, d.created_at, d.updated_at`

// CreateDecision stores a new decision in Postgres and returns it.
// All DB operations run within a single transaction.
func (s *Store) CreateDecision(ctx context.Context, slug, title, body, rationale, question string,
	rejectedAlts []storage.RejectedAlternative, confidence storage.DecisionConfidence,
	tags []string, scope storage.DecisionScope, originSpec, originStage string,
) (*storage.Decision, error) {
	now := s.now()
	initialStatus := string(storage.DecisionStatusProposed)
	ch := contenthash.Decision(title, initialStatus, body, rationale,
		question, string(confidence), string(scope), originSpec, originStage,
		tags, toContentHashAlts(rejectedAlts))

	if tags == nil {
		tags = []string{}
	}
	rejectedAltsJSON, err := marshalRejectedAlts(rejectedAlts)
	if err != nil {
		return nil, err
	}

	var result *storage.Decision
	err = s.RunInTransaction(ctx, func(txCtx context.Context) error {
		decID := newID("dec")
		row := s.queryRow(txCtx,
			`INSERT INTO decisions
				(id, slug, project_slug, title, status, body, rationale, question,
				 superseded_by, confidence, scope, origin_spec, origin_stage,
				 tags, rejected_alternatives, content_hash, version, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '', $9, $10, $11, $12, $13, $14, $15, 1, $16, $16)
			 RETURNING id, slug, title, status, body, rationale, question, superseded_by,
			           confidence, scope, origin_spec, origin_stage,
			           tags, rejected_alternatives, content_hash, version, created_at, updated_at`,
			decID, slug, s.project, title, initialStatus, body, rationale, question,
			string(confidence), string(scope), originSpec, originStage,
			tags, rejectedAltsJSON, ch, now,
		)

		dec, scanErr := scanDecisionRow(row)
		if scanErr != nil {
			var pgErr *pgconn.PgError
			if errors.As(scanErr, &pgErr) && pgErr.Code == "23505" {
				return fmt.Errorf("slug %q: %w", slug, storage.ErrDecisionAlreadyExists)
			}
			return fmt.Errorf("postgres: create decision: scan: %w", scanErr)
		}

		// Initial changelog entry.
		allFields, fieldsErr := decisionToFields(dec)
		if fieldsErr != nil {
			return fieldsErr
		}
		deltas := storage.ComputeDecisionFieldDeltas(&storage.DecisionFields{}, allFields)
		clEntry := &storage.ChangeLogEntry{
			Version:     int32(dec.Version), //nolint:gosec // version is a small monotonic counter; overflow impossible
			Stage:       string(dec.Status),
			ContentHash: dec.ContentHash,
			Checkpoint:  true,
			Summary:     "Decision created",
			Date:        dec.CreatedAt,
		}
		if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
			return clErr
		}

		// BELONGS_TO edge: decision → project.
		_, execErr := s.exec(txCtx,
			`INSERT INTO edges (from_slug, to_slug, edge_type, project_slug)
			 VALUES ($1, $2, 'BELONGS_TO', $3)`,
			slug, s.project, s.project,
		)
		if execErr != nil {
			return fmt.Errorf("postgres: create decision BELONGS_TO edge: %w", execErr)
		}

		result = dec
		return nil
	})
	return result, err
}

// GetDecision retrieves a decision by slug within the store's project.
// Returns storage.ErrDecisionNotFound if no match.
func (s *Store) GetDecision(ctx context.Context, slug string) (*storage.Decision, error) {
	row := s.queryRow(ctx,
		`SELECT `+decisionSelectCols+`
		 FROM decisions d
		 WHERE d.slug = $1 AND d.project_slug = $2`,
		slug, s.project,
	)

	dec, err := scanDecisionRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgres: decision %q: %w", slug, storage.ErrDecisionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: get decision %q: %w", slug, err)
	}
	return dec, nil
}

// ListDecisions returns decisions matching the given filters within the store's project.
// Empty status means no filter. limit=0 means no limit.
func (s *Store) ListDecisions(ctx context.Context, status storage.DecisionStatus, limit int) ([]*storage.Decision, error) {
	var whereClauses []string
	args := []any{s.project}
	whereClauses = append(whereClauses, "d.project_slug = $1")

	if status != "" {
		statuses := []string{string(status)}
		if legacy := legacyDecisionStatus(status); legacy != "" {
			statuses = append(statuses, legacy)
		}
		if status == storage.DecisionStatusProposed {
			statuses = append(statuses, "DECISION_STATUS_UNSPECIFIED", "")
		}
		args = append(args, statuses)
		whereClauses = append(whereClauses, fmt.Sprintf("d.status = ANY($%d)", len(args)))
	}

	q := `SELECT ` + decisionSelectCols + `
	      FROM decisions d
	      WHERE ` + strings.Join(whereClauses, " AND ") + `
	      ORDER BY d.created_at DESC`

	if limit > 0 {
		args = append(args, limit)
		q += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	rows, err := s.query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: list decisions: %w", err)
	}
	defer rows.Close()

	var decisions []*storage.Decision
	for rows.Next() {
		dec, scanErr := scanDecisionFromRows(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		decisions = append(decisions, dec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list decisions: rows: %w", err)
	}
	if decisions == nil {
		decisions = []*storage.Decision{}
	}
	return decisions, nil
}

// UpdateDecision updates a decision by slug. Only non-nil fields are changed.
// If expectedVersion > 0, uses optimistic concurrency: returns ErrConcurrentModification
// if the stored version doesn't match. Pass 0 to skip the version check.
func (s *Store) UpdateDecision(ctx context.Context, slug string, expectedVersion int32,
	title *string, status *storage.DecisionStatus,
	body, rationale, supersededBy, question *string,
	rejectedAlts *[]storage.RejectedAlternative, confidence *storage.DecisionConfidence,
	tags *[]string, scope *storage.DecisionScope, originSpec, originStage *string,
) (*storage.Decision, error) {
	if status != nil && *status == storage.DecisionStatusSuperseded {
		if supersededBy == nil || *supersededBy == "" {
			return nil, storage.ErrSupersededByRequired
		}
	}

	if title == nil && status == nil && body == nil && rationale == nil &&
		supersededBy == nil && question == nil && rejectedAlts == nil &&
		confidence == nil && tags == nil && scope == nil &&
		originSpec == nil && originStage == nil {
		return s.GetDecision(ctx, slug)
	}

	var result *storage.Decision
	err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		current, getErr := s.GetDecision(txCtx, slug)
		if getErr != nil {
			return getErr
		}
		oldFields, fieldsErr := decisionToFields(current)
		if fieldsErr != nil {
			return fieldsErr
		}

		var setClauses []string
		var args []any
		idx := 1

		now := s.now()
		setClauses = append(setClauses, "version = version + 1", fmt.Sprintf("updated_at = $%d", idx))
		args = append(args, now)
		idx++

		if title != nil {
			setClauses = append(setClauses, fmt.Sprintf("title = $%d", idx))
			args = append(args, *title)
			idx++
		}
		if status != nil {
			setClauses = append(setClauses, fmt.Sprintf("status = $%d", idx))
			args = append(args, string(*status))
			idx++
		}
		if body != nil {
			setClauses = append(setClauses, fmt.Sprintf("body = $%d", idx))
			args = append(args, *body)
			idx++
		}
		if rationale != nil {
			setClauses = append(setClauses, fmt.Sprintf("rationale = $%d", idx))
			args = append(args, *rationale)
			idx++
		}
		if supersededBy != nil {
			setClauses = append(setClauses, fmt.Sprintf("superseded_by = $%d", idx))
			args = append(args, *supersededBy)
			idx++
		}
		if question != nil {
			setClauses = append(setClauses, fmt.Sprintf("question = $%d", idx))
			args = append(args, *question)
			idx++
		}
		if rejectedAlts != nil {
			raJSON, marshalErr := marshalRejectedAlts(*rejectedAlts)
			if marshalErr != nil {
				return marshalErr
			}
			setClauses = append(setClauses, fmt.Sprintf("rejected_alternatives = $%d", idx))
			args = append(args, raJSON)
			idx++
		}
		if confidence != nil {
			setClauses = append(setClauses, fmt.Sprintf("confidence = $%d", idx))
			args = append(args, string(*confidence))
			idx++
		}
		if tags != nil {
			setClauses = append(setClauses, fmt.Sprintf("tags = $%d", idx))
			args = append(args, *tags)
			idx++
		}
		if scope != nil {
			setClauses = append(setClauses, fmt.Sprintf("scope = $%d", idx))
			args = append(args, string(*scope))
			idx++
		}
		if originSpec != nil {
			setClauses = append(setClauses, fmt.Sprintf("origin_spec = $%d", idx))
			args = append(args, *originSpec)
			idx++
		}
		if originStage != nil {
			setClauses = append(setClauses, fmt.Sprintf("origin_stage = $%d", idx))
			args = append(args, *originStage)
			idx++
		}

		slugArgIdx := idx
		projArgIdx := idx + 1
		args = append(args, slug, s.project)
		idx += 2

		whereClause := fmt.Sprintf("slug = $%d AND project_slug = $%d", slugArgIdx, projArgIdx)
		if expectedVersion > 0 {
			whereClause += fmt.Sprintf(" AND version = $%d", idx)
			args = append(args, expectedVersion)
		}

		sql := fmt.Sprintf(
			"UPDATE decisions SET %s WHERE %s",
			strings.Join(setClauses, ", "),
			whereClause,
		)

		tag, execErr := s.exec(txCtx, sql, args...)
		if execErr != nil {
			return fmt.Errorf("postgres: update decision: %w", execErr)
		}
		if tag.RowsAffected() == 0 {
			if expectedVersion > 0 {
				return fmt.Errorf("postgres: decision %q version %d: %w", slug, expectedVersion, storage.ErrConcurrentModification)
			}
			return fmt.Errorf("postgres: decision %q: %w", slug, storage.ErrDecisionNotFound)
		}

		dec, getErr2 := s.GetDecision(txCtx, slug)
		if getErr2 != nil {
			return getErr2
		}

		ch := contenthash.Decision(dec.Title, string(dec.Status), dec.Body, dec.Rationale,
			dec.Question, string(dec.Confidence), string(dec.Scope),
			dec.OriginSpec, dec.OriginStage,
			dec.Tags, toContentHashAlts(dec.RejectedAlternatives))

		_, execErr = s.exec(txCtx,
			`UPDATE decisions SET content_hash = $1 WHERE slug = $2 AND project_slug = $3`,
			ch, slug, s.project,
		)
		if execErr != nil {
			return fmt.Errorf("postgres: update decision content_hash: %w", execErr)
		}
		dec.ContentHash = ch

		if ch != current.ContentHash {
			newFields, nfErr := decisionToFields(dec)
			if nfErr != nil {
				return nfErr
			}
			deltas := storage.ComputeDecisionFieldDeltas(oldFields, newFields)
			if len(deltas) > 0 {
				clEntry := &storage.ChangeLogEntry{
					Version:     int32(dec.Version), //nolint:gosec // version is a small monotonic counter; overflow impossible
					Stage:       string(dec.Status),
					ContentHash: ch,
					Summary:     "decision updated",
					Date:        now,
				}
				if clErr := s.createChangeLog(txCtx, slug, clEntry, deltas); clErr != nil {
					return clErr
				}
			}
		}

		result = dec
		return nil
	})
	return result, err
}

// scanDecisionRow reads a Decision from a single pgx.Row.
func scanDecisionRow(row pgx.Row) (*storage.Decision, error) {
	var (
		id              string
		slug            string
		title           string
		statusStr       string
		body            string
		rationale       string
		question        string
		supersededBy    string
		confidence      string
		scope           string
		originSpec      string
		originStage     string
		tags            []string
		rejectedAltJSON []byte
		contentHash     string
		version         int32
		createdAt       time.Time
		updatedAt       time.Time
	)
	if err := row.Scan(
		&id, &slug, &title, &statusStr, &body, &rationale, &question, &supersededBy,
		&confidence, &scope, &originSpec, &originStage,
		&tags, &rejectedAltJSON, &contentHash, &version, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("postgres: scan decision row: %w", err)
	}
	return buildDecision(id, slug, title, statusStr, body, rationale, question, supersededBy,
		confidence, scope, originSpec, originStage,
		tags, rejectedAltJSON, contentHash, version, createdAt, updatedAt)
}

// scanDecisionFromRows reads a Decision from pgx.Rows (multi-row query).
func scanDecisionFromRows(rows pgx.Rows) (*storage.Decision, error) {
	var (
		id              string
		slug            string
		title           string
		statusStr       string
		body            string
		rationale       string
		question        string
		supersededBy    string
		confidence      string
		scope           string
		originSpec      string
		originStage     string
		tags            []string
		rejectedAltJSON []byte
		contentHash     string
		version         int32
		createdAt       time.Time
		updatedAt       time.Time
	)
	if err := rows.Scan(
		&id, &slug, &title, &statusStr, &body, &rationale, &question, &supersededBy,
		&confidence, &scope, &originSpec, &originStage,
		&tags, &rejectedAltJSON, &contentHash, &version, &createdAt, &updatedAt,
	); err != nil {
		return nil, fmt.Errorf("postgres: scan decision: %w", err)
	}
	return buildDecision(id, slug, title, statusStr, body, rationale, question, supersededBy,
		confidence, scope, originSpec, originStage,
		tags, rejectedAltJSON, contentHash, version, createdAt, updatedAt)
}

// buildDecision constructs a Decision from raw scanned values.
func buildDecision(
	id, slug, title, statusStr, body, rationale, question, supersededBy,
	confidence, scope, originSpec, originStage string,
	tags []string, rejectedAltJSON []byte,
	contentHash string, version int32, createdAt, updatedAt time.Time,
) (*storage.Decision, error) {
	rejectedAlts, err := unmarshalRejectedAlts(rejectedAltJSON)
	if err != nil {
		return nil, err
	}

	status := normalizeDecisionStatus(statusStr)
	if statusStr != "" && statusStr != "DECISION_STATUS_UNSPECIFIED" && !status.IsValid() {
		return nil, fmt.Errorf("postgres: unknown decision status %q", statusStr)
	}

	if tags == nil {
		tags = []string{}
	}

	return &storage.Decision{
		ID:                   id,
		Slug:                 slug,
		Title:                title,
		Status:               status,
		Body:                 body,
		Rationale:            rationale,
		SupersededBy:         supersededBy,
		Question:             question,
		RejectedAlternatives: rejectedAlts,
		Confidence:           storage.DecisionConfidence(confidence),
		Tags:                 tags,
		Scope:                storage.DecisionScope(scope),
		OriginSpec:           originSpec,
		OriginStage:          originStage,
		Version:              int(version),
		ContentHash:          contentHash,
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}
