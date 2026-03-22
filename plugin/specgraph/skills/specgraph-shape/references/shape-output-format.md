# ShapeOutput JSON Format

The `specgraph shape <slug> --json-file <path>` command expects proto3 JSON
(camelCase field names). Write this to a temp file and pass the path.

**Important:** Use only ASCII characters in all string values. No em dashes
(`—`), curly quotes, or other Unicode punctuation — the proto JSON parser
rejects them. Use `--` instead of `—`, and straight quotes only.

## Schema

```json
{
  "scopeIn": ["string — what's included"],
  "scopeOut": ["string — what's excluded"],
  "approaches": [
    {
      "name": "kebab-case-id",
      "description": "What this approach does",
      "tradeoffs": ["Pro or con as a string"]
    }
  ],
  "chosenApproach": "name-of-chosen-approach",
  "risks": ["Risk description as string"],
  "successMust": ["Must-have success criterion"],
  "successShould": ["Should-have criterion"],
  "successWont": ["Explicitly excluded criterion"],
  "decisions": [
    {
      "slug": "kebab-case-decision-slug",
      "title": "Decision title",
      "decision": "What was decided",
      "rationale": "Why this was decided"
    }
  ]
}
```

## Field Notes

- `approaches`: At least 2. Each must have `name`, `description`, `tradeoffs`.
- `chosenApproach`: Must match one of the approach `name` values.
- `decisions`: Each becomes a first-class Decision node in the graph with
  a `DECIDED_IN` edge to the spec. `slug` must be unique.
- `successMust`/`successShould`/`successWont`: Map to MoSCoW priority.
  `successMust` items become acceptance criteria.
