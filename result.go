package clikit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// JSONFlag is the default persistent flag name [App] installs to switch a
// command tree into machine-readable mode.
const JSONFlag = "json"

type jsonFlagContextKey struct{}

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
// A RunE can end with `return clikit.Emit(cmd, data)` so encoding errors are
// not discarded.
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
	name := JSONFlag
	if configured, ok := cmd.Context().Value(jsonFlagContextKey{}).(string); ok {
		name = configured
	}
	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		return false
	}
	return flag.Value.String() == "true"
}

func validateSuccessEnvelope(payload []byte, path []string) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("command completed without calling Emit")
		}
		return fmt.Errorf("stdout is not an envelope: %w", err)
	}
	if !envelope.OK || envelope.Meta == nil || !slices.Equal(envelope.Command, path) {
		return errors.New("stdout is not a success envelope for this command")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("command emitted more than one JSON document")
		}
		return fmt.Errorf("stdout contains trailing non-JSON content: %w", err)
	}
	return nil
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
