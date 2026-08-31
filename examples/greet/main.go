// Command greet is a runnable tour of clikit: a command tree with a subcommand,
// a typed error, human output and the matching --json envelope.
//
//	go run ./examples/greet hello world
//	go run ./examples/greet hello world --json
//	go run ./examples/greet --json --help
//	go run ./examples/greet hello          # usage error, exit 2
package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xlshcn/clikit-go"
)

func main() {
	root := &cobra.Command{
		Use:   "greet",
		Short: "A tiny clikit demonstration",
	}
	root.AddCommand(helloCommand())
	clikit.New(root, clikit.WithVersion(clikit.BuildVersion())).Run(context.Background())
}

func helloCommand() *cobra.Command {
	var loud bool
	cmd := &cobra.Command{
		Use:     "hello NAME...",
		Short:   "Greet one or more people",
		Example: "greet hello ada grace --loud",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return clikit.Usagef("hello needs at least one name")
			}
			greeting := "Hello, " + strings.Join(args, " and ") + "!"
			if loud {
				greeting = strings.ToUpper(greeting)
			}

			console := clikit.NewConsole(cmd.ErrOrStderr())
			if !clikit.JSONRequested(cmd) {
				console.Heading("Greeting")
				console.Table([]string{"name", "greeted"}, rows(args))
				console.Success(greeting)
			}
			return clikit.Emit(cmd, map[string]any{"greeting": greeting, "names": args})
		},
	}
	cmd.Flags().BoolVar(&loud, "loud", false, "shout the greeting")
	return cmd
}

func rows(names []string) [][]string {
	out := make([][]string, 0, len(names))
	for _, name := range names {
		out = append(out, []string{name, "yes"})
	}
	return out
}
