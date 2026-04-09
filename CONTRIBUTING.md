# Contributing to SpecGraph

Thank you for your interest in contributing to SpecGraph! This document
covers the process and requirements for contributing.

## Developer Certificate of Origin (DCO)

This project uses the [Developer Certificate of Origin](DCO) (DCO). All
contributors must sign off on their commits to certify that they have the
right to submit the code under the project's [Apache-2.0 license](LICENSE).

### How to sign off

Add a `Signed-off-by` trailer to every commit:

```bash
git commit -s -m "feat: add new feature"
```

This produces:

```text
feat: add new feature

Signed-off-by: Your Name <your.email@example.com>
```

The email must match your git author email (`git config user.email`).

### GitHub privacy emails

GitHub's noreply email (`username@users.noreply.github.com`) is accepted.
Configure it as your git author email if you prefer not to expose a personal
address:

```bash
git config user.email "username@users.noreply.github.com"
```

### Fixing a missing sign-off

If a commit is missing the sign-off, amend it:

```bash
git commit --amend -s
```

For multiple commits, use interactive rebase:

```bash
git rebase -i HEAD~N --exec "git commit --amend --no-edit -s"
```

## Getting Started

1. Fork the repository
2. Clone your fork
3. Install dev tools: `task tools`
4. Install git hooks: `task hooks:install`
5. Create a branch for your work
6. Make your changes
7. Run quality gates: `task check`
8. Push and open a pull request

## Quality Gates

Before pushing, run:

```bash
task check    # fmt, lint, build, unit tests
```

Before opening a PR:

```bash
task pr-prep  # check + integration + e2e tests (requires Docker)
```

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/).
The `cog` tool validates commit messages via a git hook.

Format: `type(scope): description`

Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `perf`

## Code Style

- Follow existing patterns in the codebase
- All `.go`, `.proto`, `.sh`, `.py` files require SPDX license headers
- Run `task fmt` to auto-format before committing

## Questions?

Open an issue or start a discussion on GitHub.
