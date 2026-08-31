package clikit

import (
	"context"
	"io"
	"os"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
)

// App runs a cobra command tree. It is a thin shell: Root stays a plain
// *cobra.Command that you build with ordinary cobra calls, and App only owns
// the process boundary — argv, the --json switch, and the exit code.
type App struct {
	// Root is the command tree. Configure it with cobra directly.
	Root *cobra.Command
	// Version is reported by --version. Empty disables the flag.
	Version string
	// Args overrides os.Args[1:]; useful in tests.
	Args []string
	// Out and Err default to os.Stdout and os.Stderr.
	Out io.Writer
	Err io.Writer

	fangOptions []fang.Option
}

// Option configures an [App].
type Option func(*App)

// WithVersion sets the version string reported by --version.
func WithVersion(version string) Option {
	return func(app *App) { app.Version = version }
}

// WithArgs replaces os.Args[1:], for tests and embedded use.
func WithArgs(args ...string) Option {
	return func(app *App) { app.Args = args }
}

// WithOutput redirects the application's stdout and stderr.
func WithOutput(out, err io.Writer) Option {
	return func(app *App) { app.Out, app.Err = out, err }
}

// WithFang passes options through to fang, which renders help and errors in
// human mode. See [fang.Option] for theming, signal handling and disabling the
// generated manpage or completions.
func WithFang(options ...fang.Option) Option {
	return func(app *App) { app.fangOptions = append(app.fangOptions, options...) }
}

// New builds an App around an existing cobra command tree.
func New(root *cobra.Command, options ...Option) *App {
	app := &App{Root: root, Out: os.Stdout, Err: os.Stderr}
	for _, option := range options {
		option(app)
	}
	return app
}

// Run executes the application and terminates the process with its exit code.
func (a *App) Run(ctx context.Context) {
	os.Exit(a.Execute(ctx))
}

// Execute runs the application and returns the exit code instead of exiting,
// so it can be called from tests.
func (a *App) Execute(ctx context.Context) int {
	args := a.Args
	if args == nil {
		args = os.Args[1:]
	}
	a.prepare(args)

	if !wantsJSON(args) {
		return ExitCode(fang.Execute(ctx, a.Root, a.resolvedFangOptions()...))
	}

	// JSON mode bypasses fang entirely: its job is to make output pretty, and
	// here every byte on stdout has to stay parseable.
	a.Root.SilenceErrors = true
	a.Root.SilenceUsage = true
	a.Root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		_ = writeJSON(a.Out, Envelope{
			OK:      true,
			Command: commandPath(cmd),
			Data:    Schema(cmd),
			Meta:    map[string]any{},
		})
	})

	cmd, err := a.Root.ExecuteContextC(ctx)
	if err == nil {
		return ExitOK
	}
	if cmd == nil {
		cmd = a.Root
	}
	_ = writeJSON(a.Err, errorEnvelope(commandPath(cmd), err))
	return ExitCode(err)
}

func (a *App) prepare(args []string) {
	a.Root.SetArgs(args)
	a.Root.SetOut(a.Out)
	a.Root.SetErr(a.Err)
	if a.Version != "" {
		a.Root.Version = a.Version
	}
	if a.Root.PersistentFlags().Lookup(JSONFlag) == nil {
		a.Root.PersistentFlags().BoolP(JSONFlag, "j", false, "emit machine-readable JSON")
	}
	// Flag parse failures are the user's typo, not a runtime fault: exit 2.
	a.Root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return Usagef("%s", err.Error()).
			With("help_command", cmd.CommandPath()+" --help").
			Wrap(err)
	})
}

func (a *App) resolvedFangOptions() []fang.Option {
	if a.Version == "" {
		return a.fangOptions
	}
	return append([]fang.Option{fang.WithVersion(a.Version)}, a.fangOptions...)
}

// wantsJSON scans raw argv rather than parsed flags, because the decision has
// to be made before parsing: a flag error must also be reported as JSON.
func wantsJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--"+JSONFlag || arg == "-j" {
			return true
		}
		if value, ok := boolFlagValue(arg); ok {
			return value
		}
	}
	return false
}

func boolFlagValue(arg string) (bool, bool) {
	for _, prefix := range []string{"--" + JSONFlag + "=", "-j="} {
		if len(arg) > len(prefix) && arg[:len(prefix)] == prefix {
			return arg[len(prefix):] == "true" || arg[len(prefix):] == "1", true
		}
	}
	return false, false
}
