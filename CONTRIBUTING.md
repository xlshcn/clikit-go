# Contributing

Thanks for taking a look. Issues and pull requests are welcome.

## Getting set up

```sh
git clone https://github.com/xlshcn/clikit-go
cd clikit-go
go test -race ./...
```

That is the whole setup. There is no build step and no code generation.

## Before opening a pull request

```sh
gofmt -l .              # must print nothing
go vet ./...
go test -race ./...
golangci-lint run       # optional locally, CI runs it
```

## Scope

clikit-go is deliberately thin. Before proposing a feature, check whether cobra,
fang, lipgloss or huh already do it — if one of them does, the right change is
usually documentation pointing at it, not a wrapper around it.

Good candidates:

- bugs, especially in the `--json` envelope, exit codes, or non-TTY behaviour
- gaps where the human and machine output paths disagree
- documentation and examples

Please discuss larger additions in an issue first.

## Compatibility

The package follows semantic versioning. Anything exported is public API:
changing or removing it needs a major version. The minimum Go version tracks
the dependencies and may rise in a minor release.

## Commits

Conventional-commit prefixes (`fix:`, `feat:`, `docs:`, `refactor:`) are used
for the changelog, but readable subject lines matter more than the prefix.
