package clikit

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"

	"charm.land/fang/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

	fangOptions       []fang.Option
	jsonFlagName      string
	jsonFlagShorthand string
	humanHelp         func(*cobra.Command, []string)
	prepared          bool
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

// WithJSONFlag changes the machine-readable output flag. An empty shorthand
// disables the short form.
func WithJSONFlag(name, shorthand string) Option {
	return func(app *App) {
		if name == "" {
			name = JSONFlag
		}
		app.jsonFlagName, app.jsonFlagShorthand = name, shorthand
	}
}

// New builds an App around an existing cobra command tree.
func New(root *cobra.Command, options ...Option) *App {
	app := &App{
		Root:              root,
		Out:               os.Stdout,
		Err:               os.Stderr,
		jsonFlagName:      JSONFlag,
		jsonFlagShorthand: "j",
	}
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

	cmd, _, _ := a.Root.Find(args)
	if cmd == nil {
		cmd = a.Root
	}
	if !wantsJSON(args, a.jsonFlagName, a.jsonFlagShorthand, cmd.Flags()) {
		return ExitCode(classifyCobraError(cmd, fang.Execute(ctx, a.Root, a.resolvedFangOptions()...)))
	}
	return a.executeJSON(ctx)
}

func (a *App) executeJSON(ctx context.Context) int {
	// JSON mode bypasses fang entirely: its job is to make output pretty, and
	// here every byte on stdout has to stay parseable.
	var output bytes.Buffer
	a.Root.SetOut(&output)
	defer a.Root.SetOut(a.Out)

	silenceErrors, silenceUsage := a.Root.SilenceErrors, a.Root.SilenceUsage
	a.Root.SilenceErrors = true
	a.Root.SilenceUsage = true
	defer func() { a.Root.SilenceErrors, a.Root.SilenceUsage = silenceErrors, silenceUsage }()

	cmd, err := a.Root.ExecuteContextC(context.WithValue(ctx, jsonFlagContextKey{}, a.jsonFlagName))
	if cmd == nil {
		cmd = a.Root
	}
	if err != nil {
		err = classifyCobraError(cmd, err)
		_ = writeJSON(a.Out, errorEnvelope(commandPath(cmd), err))
		return ExitCode(err)
	}

	if version, _ := cmd.Flags().GetBool("version"); version && a.Version != "" {
		output.Reset()
		_ = writeJSON(&output, Envelope{
			OK:      true,
			Command: commandPath(cmd),
			Data:    map[string]any{"version": a.Version},
			Meta:    map[string]any{},
		})
	}
	if err := validateSuccessEnvelope(output.Bytes(), commandPath(cmd)); err != nil {
		contractErr := Failf("JSON contract violation").With("reason", err.Error()).Wrap(err)
		_ = writeJSON(a.Out, errorEnvelope(commandPath(cmd), contractErr))
		return ExitFailure
	}
	if _, err := a.Out.Write(output.Bytes()); err != nil {
		return ExitFailure
	}
	return ExitOK
}

func (a *App) prepare(args []string) {
	a.Root.SetArgs(args)
	a.Root.SetOut(a.Out)
	a.Root.SetErr(a.Err)
	a.Root.Version = a.Version
	jsonFlag := a.Root.Flags().Lookup(a.jsonFlagName)
	if jsonFlag == nil {
		shorthand := a.jsonFlagShorthand
		if len(shorthand) != 1 || shorthandInUse(a.Root, shorthand) {
			shorthand = ""
		}
		a.Root.PersistentFlags().BoolP(a.jsonFlagName, shorthand, false, "emit machine-readable JSON")
		jsonFlag = a.Root.PersistentFlags().Lookup(a.jsonFlagName)
	}
	a.jsonFlagShorthand = jsonFlag.Shorthand
	_ = jsonFlag.Value.Set(jsonFlag.DefValue)
	jsonFlag.Changed = false

	if a.humanHelp == nil {
		a.humanHelp = a.Root.HelpFunc()
	}
	a.Root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !JSONRequested(cmd) {
			a.humanHelp(cmd, args)
			return
		}
		_ = writeJSON(cmd.OutOrStdout(), Envelope{
			OK:      true,
			Command: commandPath(cmd),
			Data:    Schema(cmd),
			Meta:    map[string]any{},
		})
	})

	if a.prepared {
		return
	}
	a.prepared = true

	// Flag parse failures are the user's typo, not a runtime fault: exit 2.
	flagError := a.Root.FlagErrorFunc()
	a.Root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		if flagError != nil {
			err = flagError(cmd, err)
		}
		return asUsageError(cmd, err)
	})
}

func shorthandInUse(cmd *cobra.Command, shorthand string) bool {
	if cmd.LocalFlags().ShorthandLookup(shorthand) != nil || cmd.PersistentFlags().ShorthandLookup(shorthand) != nil {
		return true
	}
	for _, child := range cmd.Commands() {
		if shorthandInUse(child, shorthand) {
			return true
		}
	}
	return false
}

func classifyCobraError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	var commandErr *Error
	if errors.As(err, &commandErr) {
		return err
	}
	for _, validate := range []func() error{cmd.ValidateRequiredFlags, cmd.ValidateFlagGroups} {
		if validationErr := validate(); validationErr != nil && validationErr.Error() == err.Error() {
			return asUsageError(cmd, err)
		}
	}
	return err
}

func asUsageError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	var commandErr *Error
	if errors.As(err, &commandErr) {
		return err
	}
	return Usagef("%s", err.Error()).
		With("help_command", cmd.CommandPath()+" --help").
		Wrap(err)
}

func (a *App) resolvedFangOptions() []fang.Option {
	if a.Version == "" {
		return append([]fang.Option{fang.WithoutVersion()}, a.fangOptions...)
	}
	return append([]fang.Option{fang.WithVersion(a.Version)}, a.fangOptions...)
}

// wantsJSON scans raw argv rather than parsed flags, because the decision has
// to be made before parsing: a flag error must also be reported as JSON.
func wantsJSON(args []string, name, shorthand string, flags *pflag.FlagSet) bool {
	wanted := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--"+name || shorthand != "" && arg == "-"+shorthand {
			wanted = true
			continue
		}
		if value, ok, valid := boolFlagValue(arg, name, shorthand); ok {
			if !valid {
				return true
			}
			wanted = value
			continue
		}
		if shorthand != "" && shortArgRequestsJSON(arg, shorthand, flags) {
			wanted = true
		}
	}
	return wanted
}

func shortArgRequestsJSON(arg, shorthand string, flags *pflag.FlagSet) bool {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return false
	}
	shorts := arg[1:]
	for len(shorts) > 0 {
		if shorts[:1] == shorthand {
			return true
		}
		flag := flags.ShorthandLookup(shorts[:1])
		if flag == nil || flag.NoOptDefVal == "" || len(shorts) > 1 && shorts[1] == '=' {
			return false
		}
		shorts = shorts[1:]
	}
	return false
}

func boolFlagValue(arg, name, shorthand string) (value, matched, valid bool) {
	prefixes := []string{"--" + name + "="}
	if shorthand != "" {
		prefixes = append(prefixes, "-"+shorthand+"=")
	}
	for _, prefix := range prefixes {
		if len(arg) >= len(prefix) && arg[:len(prefix)] == prefix {
			value, err := strconv.ParseBool(arg[len(prefix):])
			return value, true, err == nil
		}
	}
	return false, false, false
}
