// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"bytes"
	"strings"
	"text/template" // nosemgrep: go.lang.security.audit.xss.import-text-template.import-text-template -- rendering Markdown, not HTML; html/template would incorrectly escape Markdown content
	"time"

	"github.com/specgraph/specgraph/internal/storage"
)

func decisionDisplayStatus(s storage.DecisionStatus) string {
	raw := string(s)
	const prefix = "DECISION_STATUS_"
	if strings.HasPrefix(raw, prefix) {
		return strings.ToLower(raw[len(prefix):])
	}
	return strings.ToLower(raw)
}

var bundleFuncs = template.FuncMap{
	"decisionStatus": decisionDisplayStatus,
	"now":            func() string { return time.Now().UTC().Format(time.RFC3339) },
	"add1":           func(i int) int { return i + 1 },
	"join":           strings.Join,
}

var bundleTemplate = template.Must(template.New("bundle").Funcs(bundleFuncs).Parse(bundleTemplateText))

const bundleTemplateText = `---
version: {{ .Version }}
slug: {{ .Spec.Slug }}
stage: {{ .Spec.Stage }}
priority: {{ .Spec.Priority }}
content_hash: {{ .Spec.ContentHash }}
generated_at: {{ now }}
---

## What to Build

**Intent:** {{ .Spec.Intent }}
{{ if and .Spec.ShapeOutput (or .Spec.ShapeOutput.ScopeIn .Spec.ShapeOutput.ScopeOut) }}
### Scope
{{ if .Spec.ShapeOutput.ScopeIn }}
**In scope:**
{{ range .Spec.ShapeOutput.ScopeIn }}- **In:** {{ . }}
{{ end }}{{ end }}{{ if .Spec.ShapeOutput.ScopeOut }}
**Out of scope:**
{{ range .Spec.ShapeOutput.ScopeOut }}- **Out:** {{ . }}
{{ end }}{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.VerifyCriteria }}
### Acceptance Criteria
{{ range .Spec.SpecifyOutput.VerifyCriteria }}
- [ ] {{ .Category }}: {{ .Description }}
{{- end }}
{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Invariants }}
### Invariants
{{ range .Spec.SpecifyOutput.Invariants }}
- {{ . }}
{{- end }}
{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Interfaces }}
### Interfaces
{{ range .Spec.SpecifyOutput.Interfaces }}
**{{ .Name }}**

` + "```" + `
{{ .Body }}
` + "```" + `
{{ end }}{{ end }}{{ if and .Spec.SpecifyOutput .Spec.SpecifyOutput.Touches }}
### File Touches

| Path | Purpose | Change |
|------|---------|--------|
{{ range .Spec.SpecifyOutput.Touches }}| ` + "`" + `{{ .Path }}` + "`" + ` | {{ .Purpose }} | {{ .ChangeType }} |
{{ end }}{{ end }}{{ if .Spec.DecomposeOutput }}
## Work Slices

Strategy: ` + "`" + `{{ .Spec.DecomposeOutput.Strategy }}` + "`" + `
{{ range $i, $s := .Spec.DecomposeOutput.Slices }}
### Slice {{ add1 $i }}: {{ $s.Intent }}
{{ if $s.Verify }}
**Verify:**
{{ range $s.Verify }}- {{ . }}
{{ end }}{{ end }}{{ if $s.Touches }}**Touches:**
{{ range $s.Touches }}- ` + "`" + `{{ . }}` + "`" + `
{{ end }}{{ end }}{{ if $s.DependsOn }}**Depends on:** {{ join $s.DependsOn ", " }}
{{ end }}{{ end }}{{ end }}
## How to Work

1. Claim: ` + "`" + `specgraph claim {{ .Spec.Slug }} --agent <your-id>` + "`" + `
2. Progress: ` + "`" + `specgraph report-progress {{ .Spec.Slug }}` + "`" + `
3. Blocker: ` + "`" + `specgraph report-blocker {{ .Spec.Slug }}` + "`" + `
4. Done: ` + "`" + `specgraph report-completion {{ .Spec.Slug }}` + "`" + `

**Current claim:** {{ if .Claim }}{{ .Claim.Agent }} (expires {{ .Claim.LeaseExpires.Format "2006-01-02T15:04:05Z" }}){{ else }}unclaimed{{ end }}
{{ if .Dependencies }}
### Dependencies

| Spec | Stage | Drifted | Note |
|------|-------|---------|------|
{{ range .Dependencies }}| {{ .Slug }} | {{ .Stage }} | {{ if .Drifted }}**yes**{{ else }}no{{ end }} | {{ .Note }} |
{{ end }}{{ end }}
> **Constitution & priming:** ` + "`" + `specgraph prime {{ .Spec.Slug }}` + "`" + `
{{ if .Decisions }}
## Decisions
{{ range .Decisions }}
### {{ .Slug }}: {{ .Title }}

**Status:** {{ decisionStatus .Status }}

**Decision:** {{ .Body }}

**Rationale:** {{ .Rationale }}
{{ end }}{{ end }}{{ if and .Spec.ShapeOutput (or .Spec.ShapeOutput.ChosenApproach .Spec.ShapeOutput.Risks) }}
## Design Context
{{ if .Spec.ShapeOutput.ChosenApproach }}
**Chosen approach:** {{ .Spec.ShapeOutput.ChosenApproach }}
{{ end }}{{ if .Spec.ShapeOutput.Risks }}
**Risks:**
{{ range .Spec.ShapeOutput.Risks }}- {{ . }}
{{ end }}{{ end }}{{ end }}`

func renderBundleMarkdown(b *storage.Bundle) string {
	var buf bytes.Buffer
	if err := bundleTemplate.Execute(&buf, b); err != nil {
		return "# Error rendering bundle\n\n" + err.Error() + "\n"
	}
	return buf.String()
}
