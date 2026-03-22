# Structured SpecifyOutput

**Date:** 2026-03-22
**Closes:** spgr-dp9, spgr-6vl

## Problem

`SpecifyOutput` uses flat types: `interface_contract` is a single string,
`verify_criteria` and `invariants` are `repeated string`, `touches` is
`repeated string`. The specify skill produces rich structured content
(multiple API surfaces, categorized criteria, file paths with purpose and
change type) that flattens into plain strings when persisted. Downstream
consumers (execution bundles, analytical passes, graph queries) cannot
navigate the content programmatically.

Discovered during the demo walkthrough where the specify stage produced
excellent structured output that could not survive the proto roundtrip.

## Approach

Replace flat fields with structured sub-messages that are free-form enough
for skill flexibility but defined enough for programmatic navigation.
`invariants` stays `repeated string` since invariants are naturally single
statements.

This is a clean break — no migration of existing data. Acceptable at
0.2.0-dev with no production data.

## Proto Changes

Three new messages and a restructured `SpecifyOutput`:

```protobuf
message InterfaceSection {
  string name = 1;        // surface name, e.g. "WebhookService proto"
  string body = 2;        // free-form contract content
}

message VerifyCriterion {
  string category = 1;    // e.g. "emission", "CRUD", "e2e"
  string description = 2; // the criterion itself
}

message FileTouch {
  string path = 1;        // file/package path
  string purpose = 2;     // what changes and why
  string change_type = 3; // "new", "modify", "delete"
}

// WIRE-BREAK: Fields 1, 2, 4 change type from string/repeated-string to
// repeated-message. This is intentional at 0.2.0-dev (no production data).
// Field numbers are reused rather than reserved because the semantic intent
// of each field is preserved — only the structure changes.
message SpecifyOutput {
  repeated InterfaceSection interfaces = 1;
  repeated VerifyCriterion verify_criteria = 2;
  repeated string invariants = 3;
  repeated FileTouch touches = 4;
}
```

### Design Decisions

- **`invariants` stays `repeated string`:** Invariants are naturally single
  statements ("A spec may have at most one active lease"). No added structure
  needed.
- **Field numbers reused with WIRE-BREAK note:** Clean break at 0.2.0-dev.
  Field numbers 1, 2, 4 change type but preserve semantic intent. Documented
  with a WIRE-BREAK comment in the proto (same pattern as `ShapeOutput`
  `decisions` field 9).
- **`change_type` is a string, not an enum:** Keeps it free-form for skill
  flexibility. Known values: "new", "modify", "delete". The handler does NOT
  validate `change_type` against a fixed set — this is intentionally different
  from `DecompositionStrategy` which uses a typed string with validation.
  Rationale: file changes are descriptive metadata, not behavioral selectors.
- **`InterfaceSection.body` is free-form text:** Skills can put proto
  definitions, Go interfaces, HTTP endpoint tables, or any format. The `name`
  field provides programmatic navigation.

## Domain Type Changes

`internal/storage/authoring.go`:

```go
type InterfaceSection struct {
    Name string `json:"name"`
    Body string `json:"body"`
}

type VerifyCriterion struct {
    Category    string `json:"category"`
    Description string `json:"description"`
}

type FileTouch struct {
    Path       string `json:"path"`
    Purpose    string `json:"purpose"`
    ChangeType string `json:"change_type"`
}

type SpecifyOutput struct {
    Interfaces     []InterfaceSection `json:"interfaces,omitempty"`
    VerifyCriteria []VerifyCriterion  `json:"verify_criteria,omitempty"`
    Invariants     []string           `json:"invariants,omitempty"`
    Touches        []FileTouch        `json:"touches,omitempty"`
}
```

## Handler Changes

### Safety Scanner (`safetyInput`)

`authoring_handler.go` constructs `safetyInput` for the specify stage with:

```go
Text:       msg.Output.GetInterfaceContract(),
Invariants: msg.Output.GetInvariants(),
```

After the change, `GetInterfaceContract()` no longer exists. The safety
scanner text must be derived by concatenating all `interfaces[].body` fields:

```go
var contractText strings.Builder
for _, iface := range msg.Output.GetInterfaces() {
    if contractText.Len() > 0 {
        contractText.WriteString("\n\n")
    }
    contractText.WriteString(iface.GetName() + ":\n" + iface.GetBody())
}
// safetyInput.Text = contractText.String()
```

### Validation

Replace the single `interface_contract` length check with per-element
validation for the new structured fields:

| Field | Validation |
|-------|------------|
| `interfaces` | Max count check (same pattern as other repeated fields). Per-element: `name` required, `body` length <= `maxFieldLen` |
| `verify_criteria` | Per-element: `description` required, `description` length <= `maxFieldLen` |
| `touches` | Max count check. Per-element: `path` required, `purpose` length <= `maxFieldLen` |
| `invariants` | Unchanged (already `repeated string` with per-item check) |

### Prompt Registry

`internal/authoring/prompts.go` has a prompt named `"interface_contract"`
for the specify stage. Rename to `"interfaces"` to match the new field name.
Update the corresponding test assertion in `authoring_handler_test.go`.

## Affected Files

| File | Change |
|------|--------|
| `proto/specgraph/v1/authoring.proto` | New messages + restructured `SpecifyOutput` with WIRE-BREAK comment |
| `gen/specgraph/v1/` | Regenerate with `task proto` |
| `internal/storage/authoring.go` | New domain types, replace `SpecifyOutput` |
| `internal/server/authoring_handler.go` | Update validation, proto-to-domain conversion, safety scanner text derivation |
| `internal/server/authoring_handler_test.go` | Update test fixtures, prompt name assertion |
| `internal/authoring/prompts.go` | Rename `"interface_contract"` prompt to `"interfaces"` |
| `internal/storage/contenthash/` | Update if specify fields are in hash computation |
| `e2e/api/authoring_test.go` | Update `SpecifyOutput` test fixtures |
| `e2e/api/pipeline_test.go` | Update `SpecifyOutput` test fixtures |
| `e2e/api/lifecycle_pipeline_test.go` | Update `SpecifyOutput` test fixtures |
| `e2e/api/errors_test.go` | Update `SpecifyOutput` test fixtures |
| `e2e/cli/testdata/specify-output.json` | Update JSON to new shape |
| `e2e/cli/pipeline_test.go` | Verify CLI test uses updated fixture |
| `plugin/specgraph/skills/specgraph-specify/SKILL.md` | Update JSON schema in persistence section |
| `plugin/specgraph/skills/specgraph-specify/references/specify-output-format.md` | Update format reference |
| `site/docs/concepts/example-spec.md` | Update example spec (canonical example on public site) |

## Example

A realistic SpecifyOutput using the new structured fields:

```json
{
  "interfaces": [
    {
      "name": "WebhookService proto",
      "body": "service WebhookService {\n  rpc Send(SendRequest) returns (SendResponse);\n}"
    },
    {
      "name": "EventBus Go interface",
      "body": "type EventBus interface {\n  Publish(ctx context.Context, event Event) error\n}"
    }
  ],
  "verifyCriteria": [
    { "category": "behavior", "description": "Send with valid payload returns 200 and delivery ID" },
    { "category": "security", "description": "Send without auth token returns 401" },
    { "category": "retry", "description": "Failed delivery retries up to 3 times with backoff" }
  ],
  "invariants": [
    "Each event is delivered at least once",
    "Delivery order within a topic is preserved"
  ],
  "touches": [
    { "path": "internal/server/webhook_handler.go", "purpose": "new handler", "change_type": "new" },
    { "path": "internal/storage/event.go", "purpose": "event domain types", "change_type": "new" },
    { "path": "proto/specgraph/v1/webhook.proto", "purpose": "service definition", "change_type": "new" }
  ]
}
```

## Migration

None. Clean break at 0.2.0-dev. Existing specify_output JSON in Memgraph
will not parse into the new struct. Specs that have already been specified
would need to be re-specified.

## Content Hash Impact

Check whether `contenthash.Spec()` includes specify output fields. If so,
the hash computation needs to accept the new types. Since the output is
stored as JSON in Memgraph, the hash may already operate on the serialized
JSON string (in which case the shape change is transparent).
