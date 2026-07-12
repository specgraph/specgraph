---
status: testing
phase: 05-ui-project-selector-and-refinements
source: [05-VERIFICATION.md]
started: 2026-07-12T17:05:00Z
updated: 2026-07-12T17:05:00Z
---

## Current Test

number: 1
name: Dark-mode contrast on migrated Spec & Decision detail pages (05-14)
expected: |
  In the running UI, toggle dark mode and open a Spec detail (/spec/[slug]) and a
  Decision detail (/decision/[slug]) page. All body content (headings, meta labels,
  body text, tables, blockquotes, approaches, lifecycle banners, conversation roles,
  spec chips, decision-status badge) is readable with proper contrast in BOTH light
  and dark mode.
awaiting: user response

## Tests

### 1. Dark-mode contrast on migrated Spec & Decision detail pages (post-05-14)
expected: In the running UI, toggle dark mode and open a Spec detail (/spec/[slug]) and a Decision detail (/decision/[slug]) page. All body content (headings, meta labels, body text, tables, blockquotes, approaches, lifecycle banners, conversation roles, spec chips, decision-status badge) is readable with proper contrast in BOTH light and dark mode.
result: [pending]

### 2. D-10 constitution switch re-derivation
expected: Switch between a project WITH a constitution and one WITHOUT. Merged/Layer badges and sections re-derive with no stale prior-project content; the "No constitution found for this project" empty state appears; layer hues correct in light + dark.
result: [pending]

### 3. Project-switch skeleton-on-switch across scoped views
expected: With >1 project available, selecting a different project re-suspends each view (dashboard/graph/spec/decision/constitution) to its Skeleton, then renders the new project's data with correct X-Specgraph-Project scoping; no stale content.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
