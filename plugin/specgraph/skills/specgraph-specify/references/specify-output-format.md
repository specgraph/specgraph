# SpecifyOutput JSON Format

The `specgraph specify <slug> --json-file <path>` command expects proto3 JSON
(camelCase field names). Write this to a temp file and pass the path.

**Important:** Use only ASCII characters. No em dashes, curly quotes, or
Unicode punctuation — the proto JSON parser rejects them.

## Schema

```json
{
  "interfaceContract": "The interface contract as a string (API shape, method signatures, etc.)",
  "verifyCriteria": [
    "Testable acceptance criterion as string"
  ],
  "invariants": [
    "Invariant that must hold as string"
  ],
  "touches": [
    "path/to/file/this/spec/will/modify.go"
  ]
}
```

## Field Notes

- `interfaceContract`: Free text describing the public API surface.
- `verifyCriteria`: Each item should be independently testable.
- `invariants`: Properties that must always hold after implementation.
- `touches`: File paths that will be created or modified.

**Note:** `complexity` is a Spec-level field (set at spark time, default "medium"), not part of SpecifyOutput. Use `specgraph update <slug> --complexity high` to change it.
