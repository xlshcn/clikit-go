# clikit-go

[![CI](https://github.com/xlshcn/clikit-go/actions/workflows/ci.yml/badge.svg)](https://github.com/xlshcn/clikit-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/xlshcn/clikit-go.svg)](https://pkg.go.dev/github.com/xlshcn/clikit-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/xlshcn/clikit-go)](https://goreportcard.com/report/github.com/xlshcn/clikit-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A thin layer over [cobra](https://github.com/spf13/cobra) for CLIs that have two
audiences: a person at a terminal, and a program parsing the output.

Cobra gives you the command tree, flag parsing, "did you mean", completions and
man pages. [fang](https://charm.land/fang) makes its help and errors look good.
clikit adds the three things neither provides:

- **`--json` everywhere** — results, errors *and* help share one envelope, so a
  script or an agent can drive the CLI without scraping help text.
- **Typed errors with real exit codes** — `2` for a bad invocation, `1` for a
  failed run, `130` for an interrupt, and structured details that survive into
  the JSON error object.
- **A console toolkit** — styled lines, tables, spinners and prompts that
  degrade correctly for pipes, CI logs and `NO_COLOR`.

It is deliberately thin: `App.Root` is a plain `*cobra.Command`. Nothing is
wrapped or hidden, so every cobra and fang feature stays available and you can
stop using clikit for any part you would rather do yourself.

## Install

```sh
go get github.com/xlshcn/clikit-go
```

Requires Go 1.25 or newer.

## Quick start

```go
package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/xlshcn/clikit-go"
)

func main() {
	root := &cobra.Command{Use: "greet", Short: "A tiny clikit demonstration"}
	root.AddCommand(&cobra.Command{
		Use:   "hello NAME",
		Short: "Greet someone",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return clikit.Usagef("hello needs a name")
			}
			clikit.NewConsole(cmd.ErrOrStderr()).Success("Hello, " + args[0] + "!")
			return clikit.Emit(cmd, map[string]any{"name": args[0]})
		},
	})
	clikit.New(root, clikit.WithVersion(clikit.BuildVersion())).Run(context.Background())
}
```

For humans:

```console
$ greet hello ada
✓ Hello, ada!

$ greet hello
ERROR  Hello needs a name.          # exit 2
```

For programs:

```console
$ greet hello ada --json
{"ok":true,"command":["hello"],"data":{"name":"ada"},"meta":{}}

$ greet hello --json
{"ok":false,"command":["hello"],"error":{"kind":"usage_error","message":"hello needs a name"},"meta":{}}

$ greet hello --json --help
{"ok":true,"command":["hello"],"data":{"name":"hello","options":[...],"subcommands":[...]},"meta":{}}
```

A complete runnable version lives in [`examples/greet`](examples/greet).

## The parts

### Output: one envelope, two modes

`Emit` is the only call a command needs to be machine-readable. In `--json` mode
it writes the success envelope to stdout; otherwise it does nothing, because the
command already printed for humans.

```go
return clikit.Emit(cmd, results, map[string]any{"took": elapsed.String()})
```

Guard decorative output with `clikit.JSONRequested(cmd)`, or simply write it to
`cmd.ErrOrStderr()` so stdout stays parseable either way.

Every response — success, failure, help — is one shape:

```json
{"ok": true, "command": ["prompts", "import"], "data": {...}, "meta": {}}
{"ok": false, "command": ["prompts", "import"], "error": {"kind": "...", "message": "..."}, "meta": {}}
```

### Errors: exit codes that mean something

```go
return clikit.Usagef("--since must be before --until")     // exit 2
return clikit.Failf("upload failed").With("bundle", name)  // exit 1, detail in JSON
```

`Usagef` is for "you typed it wrong", `Failf` for "it ran and could not
finish" — the distinction shell scripts and CI actually branch on. Flag parse
failures are mapped to `usage_error` automatically. Cancelled contexts and
aborted prompts exit `130`. Anything else exits `1`.

### Console: styled output that degrades

```go
console := clikit.NewConsole(cmd.ErrOrStderr())
console.Heading("Bundles")
console.Table([]string{"name", "version"}, rows)
console.KeyValues("Summary", []clikit.Pair{{Key: "imported", Value: "12"}})
console.Warn("3 bundles unchanged")
console.Success("done")
```

Colour is handled by a colour-profile writer: it downsamples to what the
terminal supports and strips styling entirely for pipes, CI logs and `NO_COLOR`.
Set `console.Compact = true` for tab-separated, border-free output.

### Spinners: including the one thing no library does

```go
err := clikit.Wait("importing prompts", func() error {
	return importAll(ctx)
})
```

Or hold one open and drive it:

```go
spinner := clikit.NewSpinner("importing", clikit.SpinnerConsole(console)).Start()
spinner.Update("importing bundle 3/12")
spinner.Stop(err)
```

On a TTY it animates in place; anywhere else it prints one line and stops, so
logs stay readable. Prompts raised while a spinner runs pause it first, which is
the detail that makes "spinner, then a confirmation question" work at all.

### Prompts: safe in CI by construction

```go
ok, err := clikit.Confirm("Overwrite the live bundle?", false)
name, err := clikit.Ask("Bundle name", "default")
choice, err := clikit.Choose("Environment", []string{"dev", "prod"}, 0)
```

Built on [huh](https://charm.land/huh). Without a TTY each returns its fallback
instead of hanging or failing — except `AskPassword`, which errors, because
silently continuing without a credential is worse than stopping.

## Design notes

**Why `--json` bypasses fang.** In JSON mode clikit runs cobra directly and
renders help and errors itself. fang's job is to make output pretty; here every
byte on stdout has to stay parseable.

**Why `Emit` returns an error.** So a `RunE` can end with
`return clikit.Emit(cmd, data)`. It always returns `nil`.

**Why there are no positional declarations.** Cobra has no positional model to
introspect, so the JSON schema reports the usage line and `ValidArgs` rather
than inventing a parallel one. Use cobra's `Args` validators as usual.

## Prior art

clikit-go is a Go port of an internal Python `clikit`. Where the Go ecosystem
already had a good answer, it uses it rather than reimplementing: cobra for the
command tree, fang for styled help, lipgloss for styling and tables, huh for
prompts.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE).

Every dependency is permissively licensed — MIT, BSD-2, BSD-3, ISC or
Apache-2.0 — and `TestDependencyLicensesArePermissive` fails the build if that
ever stops being true. Two dependencies are Apache-2.0 rather than MIT
([cobra](https://github.com/spf13/cobra) and its Windows-only
[mousetrap](https://github.com/inconshreveable/mousetrap)); neither ships a
NOTICE file, so shipping a binary built on clikit-go only requires including
their license text, as Apache-2.0 asks. Nothing here is copyleft.
