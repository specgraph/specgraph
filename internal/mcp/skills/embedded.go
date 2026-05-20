// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package skills

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/specgraph/specgraph/internal/skillvalidate"
	"gopkg.in/yaml.v3"
)

//go:embed embedded/*/SKILL.md
var embeddedFS embed.FS

type embeddedSource struct {
	byName map[string]Skill
	order  []string // sorted skill names
}

// NewEmbedded loads and validates every embedded SKILL.md once. Returns
// a Source whose List/Get/Search read from the prebuilt in-memory
// catalog. Any malformed skill (missing required frontmatter, summary
// > 120 chars after YAML decode, non-kebab name, invalid YAML) returns
// a precise error and causes `specgraph serve` startup to fail loudly.
func NewEmbedded() (Source, error) {
	src := &embeddedSource{byName: map[string]Skill{}}

	entries, err := fs.ReadDir(embeddedFS, "embedded")
	if err != nil {
		return nil, fmt.Errorf("read embedded skills root: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !skillvalidate.NameRegex.MatchString(name) {
			return nil, fmt.Errorf("skill directory %q is not kebab-case ASCII (regex: %s)",
				name, skillvalidate.NameRegex.String())
		}
		body, err := fs.ReadFile(embeddedFS, path.Join("embedded", name, "SKILL.md"))
		if err != nil {
			return nil, fmt.Errorf("read %s/SKILL.md: %w", name, err)
		}
		meta, err := parseFrontmatter(name, body)
		if err != nil {
			return nil, fmt.Errorf("skill %q: %w", name, err)
		}
		src.byName[name] = Skill{Meta: meta, Body: body}
		src.order = append(src.order, name)
	}
	sort.Strings(src.order)
	return src, nil
}

func (s *embeddedSource) List(_ context.Context) ([]Meta, error) {
	out := make([]Meta, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.byName[name].Meta)
	}
	return out, nil
}

func (s *embeddedSource) Get(_ context.Context, name string) (Skill, error) {
	sk, ok := s.byName[name]
	if !ok {
		return Skill{}, ErrNotFound
	}
	return sk, nil
}

// Search walks the prebuilt catalog in sorted order applying a predicate
// built from SearchOptions. Empty query returns ErrInvalidQuery.
// SearchRegex compiles query as RE2; invalid pattern wraps ErrInvalidQuery.
// Fields defaults to {Name, Summary, Body}. Limit 0 = no cap.
func (s *embeddedSource) Search(_ context.Context, query string, opts SearchOptions) ([]Meta, error) {
	if query == "" {
		return nil, ErrInvalidQuery
	}
	fields := opts.Fields
	if len(fields) == 0 {
		fields = []SearchField{FieldName, FieldSummary, FieldBody}
	}

	matchText := func(haystack, needle string) bool {
		return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
	}

	var rx *regexp.Regexp
	if opts.Mode == SearchRegex {
		var err error
		rx, err = regexp.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidQuery, err)
		}
	}

	matches := func(text string) bool {
		if rx != nil {
			return rx.MatchString(text)
		}
		return matchText(text, query)
	}

	matchesSkill := func(sk Skill) bool {
		for _, f := range fields {
			switch f {
			case FieldName:
				if matches(sk.Name) {
					return true
				}
			case FieldSummary:
				if matches(sk.Summary) {
					return true
				}
			case FieldBody:
				if matches(string(sk.Body)) {
					return true
				}
			}
		}
		return false
	}

	out := make([]Meta, 0, len(s.order))
	for _, name := range s.order {
		sk := s.byName[name]
		if !matchesSkill(sk) {
			continue
		}
		out = append(out, sk.Meta)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

// parseFrontmatter extracts the YAML frontmatter, validates required
// fields (name match, summary present + ≤120 chars decoded), and
// returns the Meta. Body bytes are unmodified.
func parseFrontmatter(dirName string, body []byte) (Meta, error) {
	const fence = "---\n"
	bs := string(body)
	if !strings.HasPrefix(bs, fence) {
		return Meta{}, fmt.Errorf("missing leading YAML frontmatter fence")
	}
	rest := bs[len(fence):]
	end := strings.Index(rest, "\n"+fence[:len(fence)-1])
	if end < 0 {
		return Meta{}, fmt.Errorf("unterminated YAML frontmatter")
	}
	var fm struct {
		Name    string `yaml:"name"`
		Summary string `yaml:"summary"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return Meta{}, fmt.Errorf("parse frontmatter: %w", err)
	}
	if fm.Name != dirName {
		return Meta{}, fmt.Errorf("frontmatter.name=%q must match dirname %q", fm.Name, dirName)
	}
	if !skillvalidate.NameRegex.MatchString(fm.Name) {
		return Meta{}, fmt.Errorf("frontmatter.name=%q is not kebab-case", fm.Name)
	}
	summary := strings.TrimSpace(fm.Summary)
	if summary == "" {
		return Meta{}, fmt.Errorf("frontmatter.summary is required")
	}
	if len([]rune(summary)) > 120 {
		return Meta{}, fmt.Errorf("frontmatter.summary too long (%d > 120 chars after YAML decode)",
			len([]rune(summary)))
	}
	return Meta{
		Name:    fm.Name,
		Summary: summary,
		URI:     "specgraph://skills/" + fm.Name,
	}, nil
}
