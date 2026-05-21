// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package export implements project export, import, and verification.
package export

import (
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

// CurrentSchemaVersion is bumped to 2 with the multi-layer constitution
// migration. v2 documents use the Constitutions list field; v1 documents
// (still importable) use the singular Constitution field.
const CurrentSchemaVersion = 2

// Document is the top-level export structure.
type Document struct {
	SchemaVersion    int        `json:"schema_version"`
	ExportedAt       time.Time  `json:"exported_at"`
	SpecGraphVersion string     `json:"specgraph_version"`
	ProjectSlug      string     `json:"project_slug"`
	Data             Data       `json:"data"`
	Signature        *Signature `json:"signature,omitempty"`
}

// Signature holds HMAC verification data.
type Signature struct {
	Algorithm string `json:"algorithm"`
	Digest    string `json:"digest"`
}

// Data contains all exported entities in dependency order.
type Data struct {
	Project *storage.Project `json:"project"`

	// Constitution is the v1 single-layer field. Always nil in v2-emitted
	// documents (omitempty drops it). Populated when importing v1 documents
	// for the legacy single-layer case.
	Constitution *storage.Constitution `json:"constitution,omitempty"`

	// Constitutions is the v2 list of constitution layers in precedence
	// order (user, org, project, domain). Populated by v2 exports; consumed
	// by v2 imports.
	Constitutions []*storage.Constitution `json:"constitutions,omitempty"`

	Specs           []*storage.Spec                 `json:"specs"`
	Decisions       []*storage.Decision             `json:"decisions"`
	Slices          []*storage.Slice                `json:"slices"`
	Edges           []Edge                          `json:"edges"`
	Findings        []*storage.AnalyticalFinding    `json:"findings"`
	ChangeLogs      []*storage.ChangeLogEntry       `json:"changelogs"`
	Conversations   []*storage.ConversationLogEntry `json:"conversations"`
	SyncMappings    []*storage.SyncMapping          `json:"sync_mappings"`
	ExecutionEvents []*storage.ExecutionEvent       `json:"execution_events"`
}

// Edge is the export representation of a graph edge.
type Edge struct {
	FromSlug          string `json:"from_slug"`
	ToSlug            string `json:"to_slug"`
	Type              string `json:"type"`
	ContentHashAtLink string `json:"content_hash_at_link,omitempty"`
}
