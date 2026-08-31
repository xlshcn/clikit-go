// Package clikit is a thin layer over [github.com/spf13/cobra] that adds the
// three things a production CLI needs and cobra deliberately leaves out:
//
//   - a machine-readable output mode (--json) covering results, errors and help
//   - typed errors with meaningful process exit codes
//   - a console/spinner/prompt toolkit for the human-facing half
//
// Everything else — the command tree, flag parsing, "did you mean", shell
// completions, man pages, styled help — is cobra and [charm.land/fang/v2].
// clikit does not hide them: App.Root is a plain *cobra.Command, so any cobra
// or fang feature stays available.
//
// A minimal application:
//
//	root := &cobra.Command{Use: "hello", Short: "greet someone"}
//	root.RunE = func(cmd *cobra.Command, args []string) error {
//		clikit.NewConsole(cmd.OutOrStdout()).Success("hi there")
//		return clikit.Emit(cmd, map[string]any{"greeted": true})
//	}
//	clikit.New(root).Run(context.Background())
//
// Run as `hello` it prints a styled success line; run as `hello --json` it
// prints {"ok":true,"command":[],"data":{"greeted":true},"meta":{}} and
// nothing else.
package clikit
