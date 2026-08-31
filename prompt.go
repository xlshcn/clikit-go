package clikit

import (
	"os"

	"charm.land/huh/v2"
	"github.com/charmbracelet/x/term"
)

// The prompts below wrap huh with the two behaviours a CLI needs and a form
// library cannot assume: any running spinner is paused first so it does not
// overdraw the question, and a non-interactive stdin falls back to the default
// instead of failing, which is what keeps these calls safe in CI.

// Confirm asks a yes/no question. Without a TTY it returns fallback.
func Confirm(question string, fallback bool) (bool, error) {
	if !Interactive() {
		return fallback, nil
	}
	answer := fallback
	err := runPrompt(huh.NewConfirm().Title(question).Value(&answer))
	return answer, err
}

// Ask asks for a line of text. Empty input, or no TTY, yields fallback.
func Ask(question, fallback string) (string, error) {
	if !Interactive() {
		return fallback, nil
	}
	answer := ""
	if err := runPrompt(huh.NewInput().Title(question).Placeholder(fallback).Value(&answer)); err != nil {
		return "", err
	}
	if answer == "" {
		return fallback, nil
	}
	return answer, nil
}

// AskPassword asks for a secret without echoing it. It has no fallback: a
// missing TTY is an error, because silently proceeding without a credential is
// worse than stopping.
func AskPassword(question string) (string, error) {
	if !Interactive() {
		return "", Failf("cannot read %q: stdin is not interactive", question)
	}
	secret := ""
	err := runPrompt(huh.NewInput().Title(question).EchoMode(huh.EchoModePassword).Value(&secret))
	return secret, err
}

// Choose offers a single choice from options. fallback is the index selected
// when there is no TTY; pass a negative index to make that an error instead.
func Choose(question string, options []string, fallback int) (string, error) {
	if len(options) == 0 {
		return "", Failf("choose %q: no options given", question)
	}
	if !Interactive() {
		if fallback < 0 || fallback >= len(options) {
			return "", Failf("cannot choose %q: stdin is not interactive", question)
		}
		return options[fallback], nil
	}
	choice := ""
	if fallback >= 0 && fallback < len(options) {
		choice = options[fallback]
	}
	err := runPrompt(huh.NewSelect[string]().Title(question).Options(huh.NewOptions(options...)...).Value(&choice))
	return choice, err
}

// Interactive reports whether stdin is a terminal, i.e. whether prompting can
// succeed at all.
func Interactive() bool {
	return term.IsTerminal(os.Stdin.Fd())
}

func runPrompt(field huh.Field) error {
	var err error
	pauseActiveSpinner(func() { err = huh.Run(field) })
	return err
}
