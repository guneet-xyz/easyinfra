# Contributing to easyinfra

## Prerequisites

- Go 1.22+
- `helm` CLI
- `kubectl` CLI
- `golangci-lint` (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)
- `goreleaser` (for release testing)

## Build

```sh
make build
# Binary at bin/easyinfra
```

## Test

```sh
make test        # unit + integration tests
make e2e         # end-to-end tests (requires helm)
make cover       # test with coverage report
```

## Lint

```sh
make lint        # golangci-lint
make vet         # go vet
```

## Commit Convention

Use [Conventional Commits](https://www.conventionalcommits.org/):

- `feat(scope): description` — new feature
- `fix(scope): description` — bug fix
- `chore: description` — maintenance
- `docs: description` — documentation
- `test: description` — tests only
- `ci: description` — CI/CD changes

## Workflow

- All changes go directly to `main`
- One commit per logical change
- Run `make test lint` before committing

## Releases

Releases are fully automated:

1. Push a Conventional Commit to `main` (`feat:`, `fix:`, or any commit with `BREAKING CHANGE:` in the body).
2. The `Auto Tag` workflow analyzes commits since the last tag and creates a new `vX.Y.Z` tag (minor for `feat`, patch for `fix`, major for breaking).
3. The new tag triggers `Release`, which runs goreleaser to build binaries and publish a GitHub release.

Commits typed `chore:`, `docs:`, `test:`, `ci:`, `refactor:`, etc. produce **no release**. Add `[skip release]` to a commit message to suppress tagging explicitly.
