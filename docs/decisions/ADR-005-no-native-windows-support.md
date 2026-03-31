# ADR-005: No Native Windows Support — WSL Required

- **Status:** Accepted
- **Date:** 2026-03-30
- **Bead:** spgr-n2o

## Context

SpecGraph uses POSIX-specific features in several subsystems:

- **File locking:** `internal/inject/lock_unix.go` uses `syscall.Flock` for advisory locking during tool injection. The Windows stub (`lock_windows.go`) is a no-op that logs a warning — concurrent inject calls are unprotected on native Windows.
- **File permissions:** Credential bootstrap writes `0600` and checks permission bits, which have no meaningful equivalent on native Windows NTFS.
- **Docker integration:** Testcontainers and Docker Compose are used for Memgraph in development and CI. Docker Desktop on Windows adds complexity vs. Docker in WSL.
- **Shell scripts:** Taskfile commands, git hooks (lefthook), and CI scripts assume a POSIX shell.

Maintaining Windows-specific code paths (file locking, permission handling, path separators) adds ongoing cost with minimal benefit — the primary development and deployment environments are Linux and macOS.

## Decision

SpecGraph does not support native Windows. Windows users must use WSL (Windows Subsystem for Linux), which provides full POSIX compatibility.

The `lock_windows.go` file is retained solely for cross-compilation — it allows `go build` to succeed on Windows but does not represent a supported runtime path.

## Consequences

- No Windows-specific CI jobs or testing.
- No Windows-specific code paths beyond compilation stubs.
- Documentation and README should note WSL as a requirement for Windows users.
- `lock_windows.go` stays as-is (no-op with warning log). If a user runs on native Windows, the warning makes the limitation visible.
