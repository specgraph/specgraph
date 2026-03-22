# SpecifyOutput JSON Format

The `specgraph specify <slug> --json-file <path>` command expects proto3 JSON
(camelCase field names). Write this to a temp file and pass the path.

**Important:** The file must be valid JSON. Unicode characters are allowed
inside string values, but JSON delimiters must be ASCII -- do not use smart
quotes (\u201c\u201d) as string delimiters. Use standard ASCII `"` (U+0022).

## Schema

````json
{
  "interfaces": [
    {
      "name": "Surface name (e.g. WebhookService proto)",
      "body": "Free-form contract content"
    }
  ],
  "verifyCriteria": [
    {
      "category": "Category grouping (e.g. emission, CRUD, e2e)",
      "description": "Testable acceptance criterion"
    }
  ],
  "invariants": [
    "Invariant that must hold as string"
  ],
  "touches": [
    {
      "path": "path/to/file.go",
      "purpose": "What changes and why",
      "changeType": "new | modify | delete"
    }
  ]
}
````

## Field Notes

- `interfaces`: One entry per API surface. `name` identifies the surface,
  `body` is free-form (proto definitions, Go interfaces, HTTP specs, etc.).
- `verifyCriteria`: Each item should be independently testable. `category`
  groups related criteria.
- `invariants`: Properties that must always hold after implementation.
- `touches`: Files that will be created or modified. `changeType` is
  descriptive ("new", "modify", "delete") but not validated.

**Note:** `complexity` is a Spec-level field (set at spark time, default
"medium"), not part of SpecifyOutput. Use
`specgraph update <slug> --complexity high` to change it.
