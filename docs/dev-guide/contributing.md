# Contributing

By participating in this project, you agree to abide by the
[code of conduct](./code_of_conduct.md).

## Development Setup

Prerequisites:

- [Go 1.24+](https://golang.org/doc/install)
- Git
- Optional: Docker (for container validation)

Clone the repository and work from the project root:

```sh
git clone git@github.com:sanix-darker/prev.git
cd prev
```

## Build and Test

Use the standard Go toolchain commands during development:

```sh
go build ./...
go test ./...
```

Recommended validation before opening a pull request:

```sh
go test -race ./...
go vet ./...
go test -tags=e2e ./tests/...
```

## Commits

Commit messages should follow the Conventional Commits specification:

- `feat:` for new features
- `fix:` for bug fixes
- `test:` for test-only changes
- `docs:` for documentation changes
- `refactor:` for internal restructuring without behavior changes

Reference: [conventionalcommits.org](https://www.conventionalcommits.org/)

## Pull Requests

1. Create a topic branch.
2. Keep changes scoped and tested.
3. Update docs when behavior or configuration changes.
4. Open the pull request against `main`.
