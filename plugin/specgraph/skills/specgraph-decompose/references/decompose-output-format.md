# DecomposeOutput JSON Format

The `specgraph decompose <slug> --json-file <path>` command expects proto3 JSON
(camelCase field names). Write this to a temp file and pass the path.

**Important:** Use only ASCII characters. No em dashes, curly quotes, or
Unicode punctuation — the proto JSON parser rejects them.

## Schema

```json
{
  "strategy": "DECOMPOSITION_STRATEGY_VERTICAL_SLICE",
  "slices": [
    {
      "id": "kebab-case-slice-id",
      "intent": "What this slice delivers",
      "verify": ["How to verify this slice works"],
      "touches": ["path/to/file.go"],
      "dependsOn": ["id-of-prerequisite-slice"]
    }
  ]
}
```

## Strategy Values

- `DECOMPOSITION_STRATEGY_VERTICAL_SLICE` — Each slice delivers end-to-end value
- `DECOMPOSITION_STRATEGY_LAYER_CAKE` — Split by architectural layer (e.g. storage first, then API, then UI)
- `DECOMPOSITION_STRATEGY_SINGLE_UNIT` — Deliver the entire spec as one unit; no decomposition

## Field Notes

- `slices[].id`: Becomes a child spec slug in the graph.
- `slices[].dependsOn`: References other slice IDs. Creates DEPENDS_ON edges.
- `slices[].verify`: Each item should be independently testable.
- `slices[].touches`: Files, packages, or components this slice is expected to modify.
- Order slices by dependency: independent slices first, dependent ones after.
