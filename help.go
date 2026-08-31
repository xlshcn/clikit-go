package clikit

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Schema describes a command as plain data: what it is, what it accepts and
// what lives under it. `mycli sub --json --help` returns exactly this, which is
// what lets another program (or an agent) drive the CLI without scraping help
// text.
func Schema(cmd *cobra.Command) map[string]any {
	return map[string]any{
		"name":        cmd.Name(),
		"path":        commandPath(cmd),
		"usage":       cmd.UseLine(),
		"description": describe(cmd),
		"aliases":     stringsOrEmpty(cmd.Aliases),
		"options":     flagSchemas(cmd),
		"arguments":   argumentSchema(cmd),
		"subcommands": subcommandSchemas(cmd),
		"examples":    exampleLines(cmd),
	}
}

func describe(cmd *cobra.Command) string {
	if cmd.Long != "" {
		return cmd.Long
	}
	return cmd.Short
}

func flagSchemas(cmd *cobra.Command) []map[string]any {
	schemas := []map[string]any{}
	collect := func(inherited bool) func(*pflag.Flag) {
		return func(flag *pflag.Flag) {
			if flag.Hidden {
				return
			}
			_, required := flag.Annotations[cobra.BashCompOneRequiredFlag]
			schema := map[string]any{
				"name":      flag.Name,
				"type":      flag.Value.Type(),
				"help":      flag.Usage,
				"required":  required,
				"inherited": inherited,
			}
			if flag.Shorthand != "" {
				schema["shorthand"] = flag.Shorthand
			}
			if flag.DefValue != "" {
				schema["default"] = flag.DefValue
			}
			schemas = append(schemas, schema)
		}
	}
	cmd.LocalFlags().VisitAll(collect(false))
	cmd.InheritedFlags().VisitAll(collect(true))
	return schemas
}

// argumentSchema reports what cobra actually knows about positionals: the
// usage line and the completion candidates. cobra has no positional
// declarations to introspect beyond that.
func argumentSchema(cmd *cobra.Command) map[string]any {
	return map[string]any{
		"usage":      strings.TrimSpace(strings.TrimPrefix(cmd.Use, cmd.Name())),
		"valid_args": stringsOrEmpty(cmd.ValidArgs),
	}
}

func subcommandSchemas(cmd *cobra.Command) []map[string]any {
	schemas := []map[string]any{}
	for _, child := range cmd.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		schemas = append(schemas, map[string]any{
			"name":        child.Name(),
			"description": child.Short,
			"aliases":     stringsOrEmpty(child.Aliases),
			"runnable":    child.Runnable(),
		})
	}
	return schemas
}

func exampleLines(cmd *cobra.Command) []string {
	lines := []string{}
	for _, line := range strings.Split(cmd.Example, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

// stringsOrEmpty keeps nil slices out of the JSON, so consumers always get []
// instead of null.
func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
