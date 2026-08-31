# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/xlshcn/clikit-go/compare/main...HEAD
