// Copyright 2026 SpecGraph Contributors
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/specgraph/specgraph/internal/mcp/skills"
)

// fakeSource is an in-package test double satisfying skills.Source.
// Used by the handler tests so they don't depend on the embedded
// canonicals (which are exercised separately in internal/mcp/skills).
type fakeSource struct {
	entries []skills.Skill
}

func (f *fakeSource) List(_ context.Context) ([]skills.Meta, error) {
	out := make([]skills.Meta, len(f.entries))
	for i, e := range f.entries {
		out[i] = e.Meta
	}
	return out, nil
}

func (f *fakeSource) Get(_ context.Context, name string) (skills.Skill, error) {
	for _, e := range f.entries {
		if e.Name == name {
			return e, nil
		}
	}
	return skills.Skill{}, skills.ErrNotFound
}

func (f *fakeSource) Search(_ context.Context, query string, opts skills.SearchOptions) ([]skills.Meta, error) {
	if query == "" {
		return nil, skills.ErrInvalidQuery
	}
	var out []skills.Meta
	switch opts.Mode {
	case skills.SearchRegex:
		re, err := regexp.Compile(query)
		if err != nil {
			return nil, skills.ErrInvalidQuery
		}
		for _, e := range f.entries {
			if re.MatchString(e.Name + " " + e.Summary) {
				out = append(out, e.Meta)
			}
		}
	default:
		for _, e := range f.entries {
			if strings.Contains(strings.ToLower(e.Name+" "+e.Summary), strings.ToLower(query)) {
				out = append(out, e.Meta)
			}
		}
	}
	return out, nil
}

func twoSkillFake() *fakeSource {
	return &fakeSource{entries: []skills.Skill{
		{Meta: skills.Meta{Name: "alpha", Summary: "first", URI: "specgraph://skills/alpha"}, Body: []byte("---\nname: alpha\n---\nbody-a")},
		{Meta: skills.Meta{Name: "beta", Summary: "second", URI: "specgraph://skills/beta"}, Body: []byte("---\nname: beta\n---\nbody-b")},
	}}
}

func TestRegisterSkillTools_RegistersThreeTools(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	for _, name := range []string{"specgraph_skills_list", "specgraph_skills_get", "specgraph_skills_search"} {
		if _, ok := r.LookupTool(name); !ok {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestSpecgraphSkillsList_ReturnsCatalog(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_list")

	res, err := def.Handler(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, `"alpha"`) || !strings.Contains(text, `"beta"`) {
		t.Errorf("expected both skill names in output; got %s", text)
	}
}

func TestSpecgraphSkillsGet_KnownAndUnknown(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_get")

	res, err := def.Handler(context.Background(), map[string]any{"name": "alpha"})
	if err != nil {
		t.Fatalf("handler(alpha): %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "body-a") {
		t.Errorf("expected body-a in payload; got %s", res.Content[0].Text)
	}

	_, err = def.Handler(context.Background(), map[string]any{"name": "no-such"})
	if err == nil {
		t.Error("expected error for unknown name")
	}
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connect.CodeOf(err))
	}

	_, err = def.Handler(context.Background(), map[string]any{"name": ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}
}

func TestSpecgraphSkillsSearch_TextAndRegex(t *testing.T) {
	r := NewRegistry()
	RegisterSkillTools(r, twoSkillFake())
	def, _ := r.LookupTool("specgraph_skills_search")

	res, err := def.Handler(context.Background(), map[string]any{"query": "first"})
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "alpha") || strings.Contains(text, "beta") {
		t.Errorf("text search should match alpha only; got %s", text)
	}

	// Regex branch: the tool reads `regex: true` and threads SearchRegex
	// into SearchOptions.Mode. The fake source honours opts.Mode, so this
	// genuinely exercises the bool plumbing through the handler.
	res, err = def.Handler(context.Background(), map[string]any{"query": "^alpha", "regex": true})
	if err != nil {
		t.Fatalf("regex search: %v", err)
	}
	text = res.Content[0].Text
	if !strings.Contains(text, "alpha") || strings.Contains(text, "beta") {
		t.Errorf("regex search should match alpha only; got %s", text)
	}

	_, err = def.Handler(context.Background(), map[string]any{"query": ""})
	if err == nil {
		t.Error("expected error for empty query")
	}
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connect.CodeOf(err))
	}

	// Invalid regex routes the source's ErrInvalidQuery through the
	// handler's connect.CodeInvalidArgument mapping.
	_, err = def.Handler(context.Background(), map[string]any{"query": "[unclosed", "regex": true})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument for invalid regex, got %v", connect.CodeOf(err))
	}
}
