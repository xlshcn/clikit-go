package clikit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
			if out != "" {
				t.Errorf("stdout = %q, want empty: failures belong on stderr", out)
			}
			envelope := decode(t, errOut)
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
	_, errOut, _ := run(t, "echo", "boom", "--json")
	failure, _ := decode(t, errOut)["error"].(map[string]any)
	if failure["word"] != "boom" {
		t.Errorf("error.word = %v, want boom", failure["word"])
	}
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
