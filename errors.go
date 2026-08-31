package clikit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"charm.land/huh/v2"
	"github.com/spf13/cobra"
)

// Exit codes. They follow the common shell convention: 0 success, 1 runtime
// failure, 2 the caller typed something wrong, 130 interrupted.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
	ExitAbort   = 130
)

// Error is a command failure carrying an exit code and structured details that
// survive into the --json error envelope.
type Error struct {
	Kind    string         // stable machine-readable category, e.g. "usage_error"
	Message string         // human-readable summary
	Code    int            // process exit code
	Details map[string]any // extra fields merged into the JSON error object
	Cause   error
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Unwrap() error { return e.Cause }

// With attaches one detail field and returns the error, for chaining.
func (e *Error) With(key string, value any) *Error {
	if e.Details == nil {
		e.Details = map[string]any{}
	}
	e.Details[key] = value
	return e
}

// Wrap records the underlying cause, keeping kind and exit code.
func (e *Error) Wrap(cause error) *Error {
	e.Cause = cause
	return e
}

// Usagef reports that the invocation itself was wrong: a bad flag, a missing
// argument, an unknown value. Exits 2.
func Usagef(format string, args ...any) *Error {
	return &Error{Kind: "usage_error", Message: fmt.Sprintf(format, args...), Code: ExitUsage}
}

// UsageArgs adapts a cobra positional-argument validator so validation
// failures use the same usage_error and exit code 2 as flag parse failures.
func UsageArgs(validate cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := validate(cmd, args); err != nil {
			return asUsageError(cmd, err)
		}
		return nil
	}
}

// Failf reports that the command ran but could not complete. Exits 1.
func Failf(format string, args ...any) *Error {
	return &Error{Kind: "execution_error", Message: fmt.Sprintf(format, args...), Code: ExitFailure}
}

// ExitCode maps an error to a process exit code: nil is 0, a [*Error] uses its
// own Code, cancellation and aborted prompts are 130, anything else is 1.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var cmdErr *Error
	if errors.As(err, &cmdErr) && cmdErr.Code != 0 {
		return cmdErr.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, huh.ErrUserAborted) {
		return ExitAbort
	}
	return ExitFailure
}

// errorPayload renders any error as the JSON "error" object.
func errorPayload(err error) map[string]any {
	payload := map[string]any{"kind": "execution_error", "message": err.Error()}
	var cmdErr *Error
	if errors.As(err, &cmdErr) {
		payload["kind"] = cmdErr.Kind
		payload["message"] = cmdErr.Message
		for key, value := range cmdErr.Details {
			if value != nil {
				if _, err := json.Marshal(value); err != nil {
					continue
				}
				payload[key] = value
			}
		}
	}
	return payload
}
