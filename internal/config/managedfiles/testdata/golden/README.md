# Golden fixtures

Captured byte outputs from the deleted-in-PR-B `mcpconfigs/` and
`pointers/` packages, used to prove behavioural parity with
`managedfiles/`.

These fixtures are **immutable** — they cannot be regenerated from
`main` after the PR-B cleanup commit. The capture helper at
`internal/config/managedfiles/internal/captureimpl/` is also deleted
in that commit.

## To regenerate

1. `git checkout <PR-B-pre-cleanup commit SHA>`
2. `go run ./internal/config/managedfiles/internal/captureimpl`
3. Hand-merge the new bytes back to current `main`.

The `task capture-goldens` target also no longer exists after the
cleanup commit — invoke `go run` directly.

## Cases

- `missing-first-init/` — Missing file → first init produces these
  bytes for each managed file. Uses slug `captureslug`, server
  `http://localhost:9090`.
