---
name: buf-regen
description: Regenerate protobuf Go code after .proto file changes. Runs buf generate, tidies modules, and verifies build.
---

# buf-regen

Regenerate Go code from protobuf definitions.

## When to use

After any change to files in `proto/specgraph/v1/*.proto`.

## Steps

1. Run `buf lint` to validate proto files
2. Run `buf generate` to regenerate Go code in `gen/`
3. Run `go mod tidy` to update dependencies
4. Run `go build ./gen/...` to verify generated code compiles
5. Report any errors

## Notes

- Generated files go to `gen/specgraph/v1/` (protobuf) and `gen/specgraph/v1/specgraphv1connect/` (ConnectRPC)
- Never hand-edit files in `gen/` — they are overwritten by `buf generate`
- buf STANDARD lint requires unique response type names per RPC (wrap with `GetXxxResponse`, `UpdateXxxResponse`)
