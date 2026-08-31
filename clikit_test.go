package clikit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/xlshcn/clikit-go"
)

// run executes a fresh command tree with the given args and returns stdout,
// stderr and the exit code.
func run(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer

	root := &cobra.Command{Use: "demo", Short: "test app"}
	child := &cobra.Command{
		Use:     "echo WORD",
		Short:   "echo a word",
		Example: "demo echo hi",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch {
			case len(args) == 0:
				return clikit.Usagef("echo needs a word")
			case args[0] == "boom":
				return clikit.Failf("exploded").With("word", args[0])
			case args[0] == "bad-detail":
				return clikit.Failf("exploded").With("bad", func() {})
			}
			return clikit.Emit(cmd, map[string]any{"word": args[0]}, map[string]any{"source": "test"})
		},
	}
	child.Flags().Bool("upper", false, "uppercase it")
	root.AddCommand(child)

	code := clikit.New(root, clikit.WithArgs(args...), clikit.WithOutput(&out, &errOut)).Execute(context.Background())
	return out.String(), errOut.String(), code
}

func decode(t *testing.T, payload string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, payload)
	}
	return decoded
}

func TestEmitStaysSilentWithoutJSONFlag(t *testing.T) {
	out, _, code := run(t, "echo", "hi")
	if code != clikit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	if out != "" {
		t.Fatalf("stdout = %q, want empty: human mode must not emit envelopes", out)
	}
}

func TestSuccessEnvelope(t *testing.T) {
	out, _, code := run(t, "echo", "hi", "--json")
	if code != clikit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	envelope := decode(t, out)
	if envelope["ok"] != true {
		t.Errorf("ok = %v, want true", envelope["ok"])
	}
	if got := envelope["command"]; !equalStrings(got, []string{"echo"}) {
		t.Errorf("command = %v, want [echo]", got)
	}
	data, _ := envelope["data"].(map[string]any)
	if data["word"] != "hi" {
		t.Errorf("data.word = %v, want hi", data["word"])
	}
	meta, _ := envelope["meta"].(map[string]any)
	if meta["source"] != "test" {
		t.Errorf("meta.source = %v, want test", meta["source"])
	}
}

func TestErrorEnvelopeAndExitCodes(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantKind string
		wantCode int
	}{
		{"usage error", []string{"echo", "--json"}, "usage_error", clikit.ExitUsage},
		{"execution error", []string{"echo", "boom", "--json"}, "execution_error", clikit.ExitFailure},
		{"unknown flag", []string{"echo", "hi", "--nope", "--json"}, "usage_error", clikit.ExitUsage},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			out, errOut, code := run(t, testCase.args...)
			if code != testCase.wantCode {
				t.Fatalf("exit = %d, want %d", code, testCase.wantCode)
			}
			if errOut != "" {
				t.Errorf("stderr = %q, want empty: envelopes belong on stdout", errOut)
			}
			envelope := decode(t, out)
			if envelope["ok"] != false {
				t.Errorf("ok = %v, want false", envelope["ok"])
			}
			failure, _ := envelope["error"].(map[string]any)
			if failure["kind"] != testCase.wantKind {
				t.Errorf("error.kind = %v, want %s", failure["kind"], testCase.wantKind)
			}
		})
	}
}

func TestErrorDetailsSurviveIntoJSON(t *testing.T) {
	out, _, _ := run(t, "echo", "boom", "--json")
	failure, _ := decode(t, out)["error"].(map[string]any)
	if failure["word"] != "boom" {
		t.Errorf("error.word = %v, want boom", failure["word"])
	}
	out, _, _ = run(t, "echo", "bad-detail", "--json")
	if failure, _ := decode(t, out)["error"].(map[string]any); failure["message"] != "exploded" || failure["bad"] != nil {
		t.Errorf("error = %v, want valid envelope without unencodable detail", failure)
	}
}

func TestJSONContractRejectsMissingDuplicateAndTextOutput(t *testing.T) {
	tests := []struct {
		name string
		run  func(*cobra.Command) error
		want string
	}{
		{"missing Emit", func(*cobra.Command) error { return nil }, "without calling Emit"},
		{"duplicate Emit", func(cmd *cobra.Command) error {
			_ = clikit.Emit(cmd, "first")
			return clikit.Emit(cmd, "second")
		}, "more than one JSON document"},
		{"text output", func(cmd *cobra.Command) error {
			fmt.Fprint(cmd.OutOrStdout(), "noise")
			return clikit.Emit(cmd, "result")
		}, "not an envelope"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
				return testCase.run(cmd)
			}}
			code := clikit.New(root,
				clikit.WithArgs("--json"),
				clikit.WithOutput(&out, &errOut),
			).Execute(context.Background())
			if code != clikit.ExitFailure {
				t.Fatalf("exit = %d, want 1", code)
			}
			failure, _ := decode(t, out.String())["error"].(map[string]any)
			if failure["kind"] != "execution_error" || !strings.Contains(failure["reason"].(string), testCase.want) {
				t.Errorf("error = %v, want contract reason containing %q", failure, testCase.want)
			}
			if errOut.Len() != 0 {
				t.Errorf("stderr = %q, want empty", errOut.String())
			}
		})
	}
}

func TestJSONErrorsAndLogsUseSeparateStreams(t *testing.T) {
	var out, errOut bytes.Buffer
	root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
		fmt.Fprintln(cmd.ErrOrStderr(), "diagnostic")
		return clikit.Failf("failed")
	}}
	code := clikit.New(root,
		clikit.WithArgs("--json"),
		clikit.WithOutput(&out, &errOut),
	).Execute(context.Background())
	if code != clikit.ExitFailure || decode(t, out.String())["ok"] != false {
		t.Fatalf("exit/output = %d/%q, want one failure envelope", code, out.String())
	}
	if errOut.String() != "diagnostic\n" {
		t.Errorf("stderr = %q, want diagnostic log", errOut.String())
	}
}

func TestUsageArgsAndRequiredFlagsExitTwo(t *testing.T) {
	t.Run("positional validator", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{
			Use:  "demo ARG",
			Args: clikit.UsageArgs(cobra.ExactArgs(1)),
			RunE: func(cmd *cobra.Command, _ []string) error { return clikit.Emit(cmd, "ok") },
		}
		code := clikit.New(root,
			clikit.WithArgs("--json"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		failure, _ := decode(t, out.String())["error"].(map[string]any)
		if code != clikit.ExitUsage || failure["kind"] != "usage_error" {
			t.Errorf("exit/error = %d/%v, want usage error", code, failure)
		}
	})

	t.Run("required flag", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
			return clikit.Emit(cmd, "ok")
		}}
		root.Flags().String("name", "", "name")
		if err := root.MarkFlagRequired("name"); err != nil {
			t.Fatal(err)
		}
		code := clikit.New(root,
			clikit.WithArgs("--json"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		failure, _ := decode(t, out.String())["error"].(map[string]any)
		if code != clikit.ExitUsage || failure["kind"] != "usage_error" {
			t.Errorf("exit/error = %d/%v, want usage error", code, failure)
		}
	})

	t.Run("required flag in human mode", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
		root.Flags().String("name", "", "name")
		if err := root.MarkFlagRequired("name"); err != nil {
			t.Fatal(err)
		}
		code := clikit.New(root,
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		if code != clikit.ExitUsage {
			t.Errorf("exit = %d, want 2", code)
		}

		out.Reset()
		errOut.Reset()
		code = clikit.New(root,
			clikit.WithArgs("--help"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		if code != clikit.ExitOK {
			t.Errorf("help exit = %d, want 0", code)
		}
	})

	t.Run("flag group", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
		root.Flags().Bool("left", false, "left")
		root.Flags().Bool("right", false, "right")
		root.MarkFlagsOneRequired("left", "right")
		code := clikit.New(root,
			clikit.WithArgs("--json"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		failure, _ := decode(t, out.String())["error"].(map[string]any)
		if code != clikit.ExitUsage || failure["kind"] != "usage_error" {
			t.Errorf("exit/error = %d/%v, want usage error", code, failure)
		}
	})
}

func TestJSONFlagConfigurationAndArgvBoundaries(t *testing.T) {
	t.Run("occupied shorthand falls back to long flag", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
			return clikit.Emit(cmd, "ok")
		}}
		root.Flags().BoolP("junk", "j", false, "unrelated flag")
		code := clikit.New(root,
			clikit.WithArgs("--json"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		if code != clikit.ExitOK || decode(t, out.String())["ok"] != true {
			t.Fatalf("exit/output = %d/%q, want success envelope", code, out.String())
		}
		if shorthand := root.PersistentFlags().Lookup("json").Shorthand; shorthand != "" {
			t.Errorf("json shorthand = %q, want empty because -j is occupied", shorthand)
		}
	})

	t.Run("custom flag and combined shorthand", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
			return clikit.Emit(cmd, "ok")
		}}
		root.Flags().BoolP("extra", "x", false, "extra")
		code := clikit.New(root,
			clikit.WithJSONFlag("machine", "m"),
			clikit.WithArgs("-xm"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		if code != clikit.ExitOK || decode(t, out.String())["ok"] != true {
			t.Fatalf("exit/output = %d/%q, want success envelope", code, out.String())
		}
	})

	t.Run("short flag value is not a JSON request", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", RunE: func(cmd *cobra.Command, _ []string) error {
			return clikit.Emit(cmd, "ok")
		}}
		root.Flags().StringP("name", "n", "", "name")
		code := clikit.New(root,
			clikit.WithArgs("-njson"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		if code != clikit.ExitOK || out.Len() != 0 {
			t.Errorf("exit/output = %d/%q, want human mode", code, out.String())
		}
	})

	t.Run("last repeated value wins", func(t *testing.T) {
		out, _, code := run(t, "echo", "hi", "--json", "--json=false")
		if code != clikit.ExitOK || out != "" {
			t.Errorf("true then false = %d/%q, want human mode", code, out)
		}
		out, _, code = run(t, "echo", "hi", "--json=false", "--json")
		if code != clikit.ExitOK || decode(t, out)["ok"] != true {
			t.Errorf("false then true = %d/%q, want JSON mode", code, out)
		}
	})

	t.Run("double dash ends flag scanning", func(t *testing.T) {
		out, _, code := run(t, "echo", "hi", "--", "--json")
		if code != clikit.ExitOK || out != "" {
			t.Errorf("exit/output = %d/%q, want human mode", code, out)
		}
	})

	t.Run("invalid JSON value still returns JSON error", func(t *testing.T) {
		for _, arg := range []string{"--json=invalid", "--json="} {
			out, _, code := run(t, "echo", "hi", arg)
			failure, _ := decode(t, out)["error"].(map[string]any)
			if code != clikit.ExitUsage || failure["kind"] != "usage_error" {
				t.Errorf("%s exit/error = %d/%v, want usage envelope", arg, code, failure)
			}
		}
	})
}

func TestCustomCobraHandlersAndJSONVersion(t *testing.T) {
	t.Run("flag handler and repeated help", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo", Run: func(*cobra.Command, []string) {}}
		root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
			return fmt.Errorf("custom: %w", err)
		})

		app := clikit.New(root, clikit.WithArgs("--help"), clikit.WithOutput(&out, &errOut))
		if code := app.Execute(context.Background()); code != clikit.ExitOK || out.Len() == 0 {
			t.Errorf("human help = %d/%q, want help output", code, out.String())
		}

		out.Reset()
		errOut.Reset()
		app.Args = []string{"--json", "--help"}
		if code := app.Execute(context.Background()); code != clikit.ExitOK {
			t.Fatalf("JSON help exit = %d, want 0", code)
		}
		if schema, _ := decode(t, out.String())["data"].(map[string]any); schema["name"] != "demo" {
			t.Errorf("schema = %v, want demo", schema)
		}

		out.Reset()
		errOut.Reset()
		app.Args = []string{"--json", "--unknown"}
		if code := app.Execute(context.Background()); code != clikit.ExitUsage {
			t.Fatalf("exit = %d, want 2", code)
		}
		failure, _ := decode(t, out.String())["error"].(map[string]any)
		if !strings.HasPrefix(failure["message"].(string), "custom: ") {
			t.Errorf("error = %v, want custom flag handler result", failure)
		}
	})

	t.Run("version is an envelope", func(t *testing.T) {
		var out, errOut bytes.Buffer
		root := &cobra.Command{Use: "demo"}
		code := clikit.New(root,
			clikit.WithVersion("v1.2.3"),
			clikit.WithArgs("--json", "--version"),
			clikit.WithOutput(&out, &errOut),
		).Execute(context.Background())
		envelope := decode(t, out.String())
		data, _ := envelope["data"].(map[string]any)
		if code != clikit.ExitOK || data["version"] != "v1.2.3" {
			t.Errorf("exit/data = %d/%v, want version envelope", code, data)
		}
	})
}

func TestJSONHelpIsSchemaNotProse(t *testing.T) {
	out, _, code := run(t, "echo", "--json", "--help")
	if code != clikit.ExitOK {
		t.Fatalf("exit = %d, want 0", code)
	}
	schema, _ := decode(t, out)["data"].(map[string]any)
	if schema["name"] != "echo" {
		t.Errorf("name = %v, want echo", schema["name"])
	}
	options, _ := schema["options"].([]any)
	names := map[string]bool{}
	for _, option := range options {
		entry, _ := option.(map[string]any)
		names[entry["name"].(string)] = true
	}
	for _, want := range []string{"upper", "json", "help"} {
		if !names[want] {
			t.Errorf("option %q missing from schema, got %v", want, names)
		}
	}
	examples, _ := schema["examples"].([]any)
	if len(examples) != 1 || examples[0] != "demo echo hi" {
		t.Errorf("examples = %v, want [demo echo hi]", examples)
	}
}

func TestRootJSONHelpListsSubcommands(t *testing.T) {
	out, _, _ := run(t, "--json", "--help")
	schema, _ := decode(t, out)["data"].(map[string]any)
	subcommands, _ := schema["subcommands"].([]any)
	found := false
	for _, subcommand := range subcommands {
		if entry, _ := subcommand.(map[string]any); entry["name"] == "echo" {
			found = true
		}
	}
	if !found {
		t.Errorf("subcommands = %v, want one named echo", subcommands)
	}
}

func TestExitCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, clikit.ExitOK},
		{"usage", clikit.Usagef("bad"), clikit.ExitUsage},
		{"failure", clikit.Failf("bad"), clikit.ExitFailure},
		{"plain", errors.New("bad"), clikit.ExitFailure},
		{"canceled", context.Canceled, clikit.ExitAbort},
		{"wrapped usage", errors.Join(errors.New("ctx"), clikit.Usagef("bad")), clikit.ExitUsage},
	}
	for _, testCase := range cases {
		if got := clikit.ExitCode(testCase.err); got != testCase.want {
			t.Errorf("ExitCode(%s) = %d, want %d", testCase.name, got, testCase.want)
		}
	}
}

func TestConsoleTableAndCompactMode(t *testing.T) {
	var buffer bytes.Buffer
	console := clikit.NewConsole(&buffer)
	console.Table([]string{"key", "value"}, [][]string{{"a", "1"}})
	if !strings.Contains(buffer.String(), "│") {
		t.Errorf("bordered table missing borders: %q", buffer.String())
	}

	buffer.Reset()
	console.Compact = true
	console.KeyValues("", []clikit.Pair{{Key: "a", Value: "1"}})
	if got := buffer.String(); got != "a: 1\n" {
		t.Errorf("compact key/values = %q, want \"a: 1\\n\"", got)
	}
}

func TestConsoleStripsColorForNonTerminals(t *testing.T) {
	var buffer bytes.Buffer
	clikit.NewConsole(&buffer).Success("done")
	if got := buffer.String(); got != "✓ done\n" {
		t.Errorf("output = %q, want plain text: a buffer is not a terminal", got)
	}
}

func equalStrings(got any, want []string) bool {
	values, ok := got.([]any)
	if !ok || len(values) != len(want) {
		return false
	}
	for index, value := range values {
		if value != want[index] {
			return false
		}
	}
	return true
}
