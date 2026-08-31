package clikit

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// JSONFlag is the persistent flag name [App] installs to switch a command tree
// into machine-readable mode.
const JSONFlag = "json"

// Envelope is the single shape every --json response takes, success or
// failure, so callers can branch on one field.
type Envelope struct {
	OK      bool           `json:"ok"`
	Command []string       `json:"command"`
	Data    any            `json:"data,omitempty"`
	Error   map[string]any `json:"error,omitempty"`
	Meta    map[string]any `json:"meta"`
}

// Emit publishes a command's result. In --json mode it writes the success
// envelope to the command's stdout; otherwise it does nothing, on the
// assumption that the command already printed for humans.
//
// It always returns nil, so a RunE can end with `return clikit.Emit(cmd, data)`.
func Emit(cmd *cobra.Command, data any, meta ...map[string]any) error {
	if !JSONRequested(cmd) {
		return nil
	}
	return writeJSON(cmd.OutOrStdout(), Envelope{
		OK:      true,
		Command: commandPath(cmd),
		Data:    data,
		Meta:    mergeMeta(meta),
	})
}

// JSONRequested reports whether the caller asked for machine-readable output.
// Commands should consult it before printing anything decorative.
func JSONRequested(cmd *cobra.Command) bool {
	flag := cmd.Flags().Lookup(JSONFlag)
	if flag == nil {
		return false
	}
	return flag.Value.String() == "true"
}

func errorEnvelope(path []string, err error) Envelope {
	return Envelope{OK: false, Command: path, Error: errorPayload(err), Meta: map[string]any{}}
}

func writeJSON(w io.Writer, payload any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(payload)
}

func mergeMeta(metas []map[string]any) map[string]any {
	merged := map[string]any{}
	for _, meta := range metas {
		for key, value := range meta {
			merged[key] = value
		}
	}
	return merged
}

// commandPath is the command's path with the program name stripped, so a
// consumer sees ["prompts","import"] rather than ["nelu","prompts","import"].
func commandPath(cmd *cobra.Command) []string {
	parts := strings.Fields(cmd.CommandPath())
	if len(parts) <= 1 {
		return []string{}
	}
	return parts[1:]
}
