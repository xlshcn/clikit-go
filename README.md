# clikit-go

[![CI](https://github.com/xlshcn/clikit-go/actions/workflows/ci.yml/badge.svg)](https://github.com/xlshcn/clikit-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/xlshcn/clikit-go.svg)](https://pkg.go.dev/github.com/xlshcn/clikit-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/xlshcn/clikit-go)](https://goreportcard.com/report/github.com/xlshcn/clikit-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A thin layer over [cobra](https://github.com/spf13/cobra) for command-line tools
that have two audiences: a person at a terminal, and a program parsing the
output.

```sh
go get github.com/xlshcn/clikit-go
```

---

## Why this exists

A CLI usually starts as something a person types. Then a deploy script calls it.
Then CI branches on its exit code. Then an agent tries to discover what it can
do. Each of those readers wants something different, and the usual result is a
tool that serves the first one well and the rest by accident:

- output mixes human decoration into the stream a script has to parse
- every failure exits `1`, so callers cannot tell "you typed it wrong" from "the
  upload failed" without matching on error strings
- the only machine-readable description of the tool is its help text, so
  anything automating it ends up scraping prose
- colour and spinners leak into CI logs as escape-sequence noise

Cobra solves the parsing problem completely, and [fang](https://charm.land/fang)
makes cobra's help and errors look good. Neither addresses the list above,
because neither is meant to: they are about defining and presenting commands,
not about what a command *returns*.

clikit-go fills exactly that gap and nothing else:

| Need | What clikit adds |
| --- | --- |
| A script must parse the output | `--json` mode: results, errors **and** help share one envelope |
| CI must branch on the outcome | Typed errors with real exit codes — `2` bad invocation, `1` failed run, `130` interrupted |
| A program must discover the CLI | `--json --help` returns a command schema, not prose |
| Humans still want good output | Console, tables, spinners and prompts that degrade for pipes, CI and `NO_COLOR` |

It stays deliberately thin. `App.Root` is a plain `*cobra.Command`: nothing is
wrapped, nothing is hidden, every cobra and fang feature remains available, and
you can stop using clikit for any part you would rather do yourself.

### When to use it

Reach for clikit-go when your tool is called by something other than a person —
CI, a Makefile, another service, an agent — and you want that path to be a
designed interface rather than an accident.

### When not to

If your CLI is only ever typed by humans, plain cobra plus fang is already the
right answer and clikit adds nothing you need. If you want a completely
different command model (struct tags, declarative specs), look at
[kong](https://github.com/alecthomas/kong) or
[urfave/cli](https://github.com/urfave/cli) instead — clikit is committed to
cobra's.

### Requirements

Go 1.25 or newer.

---

## Usage

### Quick start

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

The import path ends in `clikit-go` but the package is named `clikit`, so calls
read `clikit.New(...)`. No import alias is needed — Go takes the identifier from
the package clause, not the path — and `goimports` resolves it for you.

That one program serves both audiences. For a person:

```console
$ greet hello ada
✓ Hello, ada!

$ greet hello
ERROR  Hello needs a name.          # exit 2
```

For a program:

```console
$ greet hello ada --json
{"ok":true,"command":["hello"],"data":{"name":"ada"},"meta":{}}

$ greet hello --json
{"ok":false,"command":["hello"],"error":{"kind":"usage_error","message":"hello needs a name"},"meta":{}}

$ greet hello --json --help
{"ok":true,"command":["hello"],"data":{"name":"hello","options":[...],"subcommands":[...]},"meta":{}}
```

A complete runnable version lives in [`examples/greet`](examples/greet):

```sh
go run ./examples/greet hello ada grace --json
```

### Building the application

`clikit.New` takes a cobra command tree you built the usual way, plus options.
`Run` executes it and exits; `Execute` returns the exit code instead, which is
what you want in tests.

```go
app := clikit.New(root,
	clikit.WithVersion(clikit.BuildVersion()),  // --version
	clikit.WithFang(fang.WithoutManpage()),     // pass options through to fang
)
app.Run(ctx)
```

| Option | Effect |
| --- | --- |
| `WithVersion(string)` | Sets the string `--version` reports. Empty disables the flag. |
| `WithArgs(...string)` | Replaces `os.Args[1:]`. For tests and embedding. |
| `WithOutput(out, err)` | Redirects the application's stdout and stderr. |
| `WithFang(...fang.Option)` | Passes through to fang: theming, signals, disabling the generated manpage or completions. |

`BuildVersion` reports whatever Go embedded in the binary — a real module
version for `go install module@v1.2.3`, or a pseudo-version carrying the commit,
date and a `+dirty` marker for `go build` inside a checkout. Prefer a
linker-supplied version when your build sets one.

### Returning results

`Emit` is the only call a command needs to become machine-readable. In `--json`
mode it writes the success envelope to stdout; otherwise it does nothing, on the
assumption that the command already printed for humans. It always returns `nil`,
so a `RunE` can end with it:

```go
return clikit.Emit(cmd, results)
```

Attach anything a caller might want that is not the result itself as meta:

```go
return clikit.Emit(cmd, results, map[string]any{"took": elapsed.String()})
```

Every response — success, failure, help — has one shape:

```json
{"ok": true,  "command": ["prompts", "import"], "data": {...},  "meta": {}}
{"ok": false, "command": ["prompts", "import"], "error": {...}, "meta": {}}
```

`command` is the path with the program name stripped, so a consumer reads
`["prompts","import"]` rather than `["mycli","prompts","import"]`.

Guard decorative output with `clikit.JSONRequested(cmd)`, or simply write it to
`cmd.ErrOrStderr()` so stdout stays parseable either way:

```go
if !clikit.JSONRequested(cmd) {
	console.Heading("Importing")
}
```

### Failing usefully

```go
return clikit.Usagef("--since must be before --until")     // exit 2
return clikit.Failf("upload failed").With("bundle", name)  // exit 1
```

`Usagef` means "you typed it wrong", `Failf` means "it ran and could not
finish" — the distinction shell scripts and CI actually branch on. `With` adds
fields that survive into the JSON error object:

```json
{"ok":false,"command":["upload"],"error":{"kind":"execution_error","message":"upload failed","bundle":"prod"},"meta":{}}
```

| Situation | Exit code |
| --- | --- |
| Success | `0` |
| `Failf`, or any other error | `1` |
| `Usagef`, or a flag parse failure | `2` |
| Cancelled context, or an aborted prompt | `130` |

Flag errors are mapped to `usage_error` automatically, so `mycli --nonsense`
exits `2` without you writing anything. Use `Wrap` to keep an underlying cause
for `errors.Is` and `errors.As`, and `ExitCode(err)` if you need the mapping
yourself.

### Machine-readable help

`mycli sub --json --help` returns the command as data — name, usage,
description, aliases, options (local and inherited, with types, defaults and
whether they are required), arguments, subcommands and examples. That is what
lets another program, or an agent, discover the CLI without scraping help text.
`clikit.Schema(cmd)` produces the same map if you want it elsewhere.

### Console output

```go
console := clikit.NewConsole(cmd.ErrOrStderr())  // or clikit.NewStderrConsole()

console.Heading("Bundles")
console.Table([]string{"name", "version"}, rows)
console.KeyValues("Summary", []clikit.Pair{{Key: "imported", Value: "12"}})
console.Sections([]clikit.Pair{{Key: "Notes", Value: "…"}})

console.Success("done")
console.Warn("3 bundles unchanged")
console.Error("2 bundles failed")
console.Verbose("skipped 4 unchanged files")
console.Debug("resolved endpoint " + endpoint)
console.Printf("%d bundles", count)
```

Colour is handled by a colour-profile writer: it downsamples to what the
terminal supports and strips styling entirely for pipes, CI logs and `NO_COLOR`.
You never have to ask whether you are on a TTY. Set `console.Compact = true` for
tab-separated, border-free output when a table's borders would be noise.

`KeyValues` and `Sections` take an ordered `[]Pair` rather than a map, because
Go map iteration order would shuffle your output between runs.

### Spinners

For work of unknown duration, the one-line form:

```go
err := clikit.Wait("importing prompts", func() error {
	return importAll(ctx)
})
```

Or hold one open and drive it:

```go
spinner := clikit.NewSpinner("importing", clikit.SpinnerConsole(console)).Start()
spinner.Update("importing bundle 3/12")
err := importAll(ctx)
spinner.Stop(err)
```

Deferring the stop needs a closure, because `defer spinner.Stop(err)` would
capture `err` as it is now — nil:

```go
defer func() { spinner.Stop(err) }()
```

On a TTY it animates in place; anywhere else it prints one line and stops, so
logs stay readable. With a `SpinnerConsole` it prints a success or error line on
exit depending on the error passed to `Stop`. `Stop` is safe to call twice, so a
`defer` after an explicit stop will not panic.

Prompts raised while a spinner is running pause it first — the detail that makes
"spinner, then a confirmation question" work at all. `spinner.Pause(fn)` does
the same for anything else that needs the cursor.

### Prompts

```go
ok, err := clikit.Confirm("Overwrite the live bundle?", false)
name, err := clikit.Ask("Bundle name", "default")
secret, err := clikit.AskPassword("API token")
env, err := clikit.Choose("Environment", []string{"dev", "prod"}, 0)
```

Built on [huh](https://charm.land/huh). The last argument is the fallback used
when stdin is not a terminal, so these calls are safe in CI: they return the
fallback instead of hanging or failing. `AskPassword` is the exception — it
errors, because silently continuing without a credential is worse than stopping.
`clikit.Interactive()` reports whether prompting can succeed at all.

### Testing your CLI

`WithArgs` and `WithOutput` make a command tree testable without touching the
process:

```go
var out, errOut bytes.Buffer
code := clikit.New(root,
	clikit.WithArgs("upload", "bundle.yaml", "--json"),
	clikit.WithOutput(&out, &errOut),
).Execute(context.Background())

if code != clikit.ExitOK {
	t.Fatalf("exit %d: %s", code, errOut.String())
}
```

Asserting on the JSON envelope is far more stable than asserting on help text or
styled output.

### API summary

| | |
| --- | --- |
| **Application** | `New`, `App.Run`, `App.Execute`, `WithVersion`, `WithArgs`, `WithOutput`, `WithFang`, `BuildVersion` |
| **Output** | `Emit`, `JSONRequested`, `Envelope`, `Schema`, `JSONFlag` |
| **Errors** | `Usagef`, `Failf`, `Error.With`, `Error.Wrap`, `ExitCode`, `ExitOK`, `ExitFailure`, `ExitUsage`, `ExitAbort` |
| **Console** | `NewConsole`, `NewStderrConsole`, `Console`, `Pair` |
| **Spinner** | `Wait`, `NewSpinner`, `Spinner`, `SpinnerOutput`, `SpinnerConsole`, `SpinnerEnabled`, `SpinnerInterval` |
| **Prompts** | `Confirm`, `Ask`, `AskPassword`, `Choose`, `Interactive` |

Full documentation: [pkg.go.dev](https://pkg.go.dev/github.com/xlshcn/clikit-go).

---

## License

clikit-go is released under the [MIT License](LICENSE). You may use, copy,
modify, merge, publish, distribute, sublicense and sell it, in open-source or
commercial work, provided the copyright notice and permission notice are
included. It is provided as-is, without warranty.

### Dependencies

Every dependency compiled into a binary is permissively licensed. There is no
copyleft anywhere in the tree.

| License | Count | Notable |
| --- | --- | --- |
| MIT | 28 | fang, huh, lipgloss, bubbletea, colorprofile, uniseg |
| BSD-3-Clause | 5 | spf13/pflag, golang.org/x/{sync,sys,text} |
| Apache-2.0 | 2 | spf13/cobra, inconshreveable/mousetrap (Windows only) |

The two Apache-2.0 dependencies remain Apache-2.0; MIT covers only clikit-go's
own code. Neither ships a `NOTICE` file, so the only obligation that travels
with them is including their license text — and that falls on whoever
distributes a compiled binary, not on clikit-go itself, since Go consumers
resolve dependencies for themselves.

This is enforced, not just documented: `TestDependencyLicensesArePermissive`
walks every module that compiles into a binary on Linux, macOS and Windows,
classifies its license from the operative wording, and fails the build on
anything copyleft or unrecognised.

---

## Prior art

clikit-go is a Go port of an internal Python `clikit`, at roughly half the size.
Where the Go ecosystem already had a good answer it uses it rather than
reimplementing: cobra for the command tree, fang for styled help, lipgloss for
styling and tables, huh for prompts. What remains is the part none of them
cover.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Changes are tracked in
[CHANGELOG.md](CHANGELOG.md).
