# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.2] - 2026-08-31

### Fixed

- Enforce exactly one validated envelope on stdout for every JSON execution;
  missing, duplicate, or polluted success output now becomes a contract error.
- Classify wrapped positional validators, required flags, and flag groups as
  usage errors with exit code 2.
- Handle repeated JSON booleans, combined shorthand flags, `--`, invalid JSON
  values, and existing `-j` flags without selecting the wrong output mode or
  panicking.
- Preserve JSON help across repeated executions and return JSON version output
  as an envelope.

### Changed

- JSON success, failure, help, and version envelopes now all use stdout;
  stderr is reserved for diagnostics.
- Added `WithJSONFlag` to configure the JSON flag name and shorthand, and
  `UsageArgs` to adapt Cobra positional validators.

## [0.1.1] - 2026-08-31

### Fixed

- CI: run every matrix step under bash. Windows runners default to PowerShell,
  which split `-coverprofile=coverage.out` at the dot and made `go test` look
  for a package named `.out`. The tests themselves were passing.

### Changed

- Bumped `spf13/pflag` to v1.0.10 and the GitHub Actions to current majors.

## [0.1.0] - 2026-08-31

### Added

- `App` — a thin runner over a cobra command tree, owning argv, the `--json`
  switch and the process exit code.
- `--json` mode covering results (`Emit`), errors and help (`Schema`) under one
  envelope.
- `Error`, `Usagef`, `Failf` and `ExitCode` — typed failures with exit codes 2
  (usage), 1 (runtime) and 130 (interrupted or aborted).
- `Console` — styled lines, tables and key/value output that degrades for
  pipes, CI logs and `NO_COLOR`.
- `Spinner` and `Wait` — a waiting indicator that animates on a TTY, prints one
  line elsewhere, and can be paused so prompts are not overdrawn.
- `Confirm`, `Ask`, `AskPassword` and `Choose` — prompts that fall back to a
  default when stdin is not interactive.
- `BuildVersion` — the version Go embedded in the binary.
- A license audit test that fails the build if any dependency compiled into a
  user binary, on any supported platform, stops being permissively licensed.

[Unreleased]: https://github.com/xlshcn/clikit-go/compare/v0.1.2...HEAD
[0.1.2]: https://github.com/xlshcn/clikit-go/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/xlshcn/clikit-go/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/xlshcn/clikit-go/releases/tag/v0.1.0
