---
status: complete
phase: 06-mcp-authoring-self-teaching-path
source: [06-01-SUMMARY.md, 06-02-SUMMARY.md, 06-03-SUMMARY.md, 06-04-SUMMARY.md, 06-05-SUMMARY.md]
started: 2026-07-14T17:30:25Z
updated: 2026-07-14T17:33:40Z
---

## Current Test
<!-- OVERWRITE each test - shows where we are -->

[testing complete]

<!--
Confirmation gate: all 18 deliverables auto-covered by passing tests. Headline
behavior verified LIVE during this UAT under Docker:
- go test -tags e2e ./e2e/api/ --ginkgo.label-filter=MCPOnly -> 4/4 Passed (9.5s)
  logs show spark->shape->specify->decompose->approve via MCP client only.
- go test -tags e2e ./e2e/api/ (full suite) -> 205 Passed (39s), no regressions.
-->


## Tests

### 1. Friendly YAML parses into stage protos (06-01 D1)
expected: Friendly snake_case YAML for each funnel stage parses into the correct stage proto with nested repeated messages
result: pass
source: automated
coverage_id: 06-01-D1

### 2. Invalid enums rejected in loader (06-01 D2)
expected: Invalid enum values (scope_sniff, strategy) are rejected with an error; multi-token strategy values map correctly
result: pass
source: automated
coverage_id: 06-01-D2

### 3. 7 skills rewritten MCP-first (06-02 D1)
expected: 7 embedded SKILL.md canonicals rewritten MCP-first with a uniform gated CLI appendix
result: pass
source: automated
coverage_id: 06-02-D1

### 4. Constitution skill teaches tool get/update (06-02 D2)
expected: specgraph-constitution teaches constitution tool get/update with the inline friendly-YAML write payload
result: pass
source: automated
coverage_id: 06-02-D2

### 5. Authoring skill teaches snake_case output + exchanges (06-02 D3)
expected: specgraph-authoring teaches snake_case friendly-YAML output per stage and the mandatory exchanges JSON array
result: pass
source: automated
coverage_id: 06-02-D3

### 6. Content-level MCP-first regression gate (06-02 D4)
expected: Content-level assertion + full unit suite gate the MCP-first posture against regression
result: pass
source: automated
coverage_id: 06-02-D4

### 7. Prime emits start-here authoring routing (06-03 D1)
expected: writeSkills emits a start-here authoring routing sentence naming specgraph_skills_list and specgraph_skills_get
result: pass
source: automated
coverage_id: 06-03-D1

### 8. Prime empty-states route to constitution tool (06-03 D2)
expected: Project + Spec prime constitution empty-states route to the constitution MCP tool / specgraph-constitution skill
result: pass
source: automated
coverage_id: 06-03-D2

### 9. Constitution empty-resource routes to MCP (06-03 D3)
expected: MCP constitution empty-resource (specgraph://constitution) routes to the MCP tool/skill via shared render.ConstitutionEmptyHint
result: pass
source: automated
coverage_id: 06-03-D3

### 10. constitution tool accepts friendly YAML (06-04 D1)
expected: constitution tool update accepts friendly YAML (layer: project) via constitution/load and persists
result: pass
source: automated
coverage_id: 06-04-D1

### 11. constitution update rejects invalid layer (06-04 D2)
expected: constitution update rejects invalid layer and empty-layer input with a sanitized errResult
result: pass
source: automated
coverage_id: 06-04-D2

### 12. author stages accept friendly YAML (06-04 D3)
expected: author spark/shape/specify/decompose accept friendly snake_case YAML via authoring/load and persist via the funnel
result: pass
source: automated
coverage_id: 06-04-D3

### 13. Invalid enums / malformed exchanges rejected at write (06-04 D4)
expected: Invalid enums (scope_sniff, strategy) return a sanitized errResult; malformed exchanges JSON is rejected at the boundary
result: pass
source: automated
coverage_id: 06-04-D4

### 14. Tool descriptions teach friendly-YAML shapes (06-04 D5)
expected: Agent-facing tool Description + output/exchanges param docs teach the friendly-YAML/JSON shapes
result: pass
source: automated
coverage_id: 06-04-D5

### 15. MCP-client-only authoring reaches persistence (06-05 D1)
expected: MCP-client-only run reads specgraph://prime, authors the constitution to persistence and the spec to approved
result: pass
source: automated
coverage_id: 06-05-D1

### 16. Prime returns empty-state hint on fresh project (06-05 D2)
expected: specgraph://prime returns the empty-state constitution routing hint on a fresh project (order-independent)
result: pass
source: automated
coverage_id: 06-05-D2

### 17. Post-spark stages exercise ValidateExchanges rejection (06-05 D3)
expected: Post-spark stages exercise the real server ValidateExchanges rejection (missing exchanges; exchange missing sequence)
result: pass
source: automated
coverage_id: 06-05-D3

### 18. skills_test includes specgraph-constitution (06-05 D4)
expected: skills_test.go skill-list assertion includes specgraph-constitution (seven skills)
result: pass
source: automated
coverage_id: 06-05-D4

## Summary

total: 18
passed: 18
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

[none yet]
