// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"errors"
)

// Source is the read-only catalog interface for SKILL.md packages.
type Source interface {
	List(ctx context.Context) ([]Meta, error)
	Get(ctx context.Context, name string) (Skill, error)
	Search(ctx context.Context, query string, opts SearchOptions) ([]Meta, error)
}

// Meta is one catalog row: what specgraph_skills_list and
// specgraph_skills_search return per skill.
type Meta struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	// URI is the canonical fetch resource URI for this skill, always
	// equal to "specgraph://skills/" + Name. The derivation is enforced
	// by parseFrontmatter in embedded.go; future Source implementations
	// must honour this invariant or callers that build URIs from Meta.Name
	// directly may diverge from what Meta.URI reports.
	URI string `json:"uri"`
}

// Skill is the full payload returned by Source.Get and by the
// specgraph://skills/<name> resource handler.
type Skill struct {
	Meta
	// Body is the verbatim SKILL.md bytes (frontmatter + content). For
	// the embedded source, this slice points into the //go:embed-loaded
	// memory shared across all Get callers — do NOT mutate it. Callers
	// that need to modify the bytes must Clone (e.g., bytes.Clone(sk.Body))
	// first.
	Body []byte
}

// SearchOptions tune Source.Search. Zero value = case-insensitive
// substring search across name, summary, and body, no result cap.
type SearchOptions struct {
	Mode   SearchMode    // Text (default) or Regex
	Fields []SearchField // empty = all of {Name, Summary, Body}
	Limit  int           // 0 = no cap
}

// SearchMode controls how Source.Search interprets the query.
type SearchMode int

const (
	// SearchText is case-insensitive substring matching (default).
	SearchText SearchMode = iota
	// SearchRegex compiles the query as an RE2 regex.
	SearchRegex
)

// SearchField restricts Source.Search to specific fields.
type SearchField int

const (
	// FieldName scans Meta.Name.
	FieldName SearchField = iota
	// FieldSummary scans Meta.Summary.
	FieldSummary
	// FieldBody scans the SKILL.md body bytes.
	FieldBody
)

// ErrNotFound is returned by Source.Get when the requested name is not
// in the catalog. Mapped to connect.CodeNotFound at the handler boundary.
var ErrNotFound = errors.New("skill not found")

// ErrInvalidQuery is returned by Source.Search when the query is empty
// or, in SearchRegex mode, fails to compile. Mapped to
// connect.CodeInvalidArgument at the handler boundary.
var ErrInvalidQuery = errors.New("invalid query")
