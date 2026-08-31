package clikit

import (
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/x/term"
)

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

const defaultSpinnerInterval = 80 * time.Millisecond

// Spinner is a waiting indicator for work of unknown duration. On a TTY it
// animates in place; anywhere else it degrades to a single line so logs stay
// readable.
type Spinner struct {
	out      io.Writer
	console  *Console
	interval time.Duration
	enabled  bool
	// enabledOverride records an explicit SpinnerEnabled, which beats detection.
	enabledOverride *bool

	mu      sync.Mutex
	message string

	paused  atomic.Bool
	stop    chan struct{}
	stopped chan struct{}
	running bool
}

// SpinnerOption configures a [Spinner].
type SpinnerOption func(*Spinner)

// SpinnerOutput sets the stream the animation is drawn on (default os.Stderr).
func SpinnerOutput(w io.Writer) SpinnerOption {
	return func(s *Spinner) { s.out = w }
}

// SpinnerConsole makes the spinner print a final success or error line when it
// stops. Without one, stopping just clears the animation.
func SpinnerConsole(c *Console) SpinnerOption {
	return func(s *Spinner) { s.console = c }
}

// SpinnerEnabled forces animation on or off, overriding TTY detection.
func SpinnerEnabled(enabled bool) SpinnerOption {
	return func(s *Spinner) { s.enabledOverride = &enabled }
}

// SpinnerInterval sets the frame duration.
func SpinnerInterval(interval time.Duration) SpinnerOption {
	return func(s *Spinner) { s.interval = interval }
}

// NewSpinner builds a spinner. It does not draw anything until Start.
func NewSpinner(message string, options ...SpinnerOption) *Spinner {
	spinner := &Spinner{out: os.Stderr, message: message, interval: defaultSpinnerInterval}
	for _, option := range options {
		option(spinner)
	}
	spinner.enabled = isTerminal(spinner.out)
	if spinner.enabledOverride != nil {
		spinner.enabled = *spinner.enabledOverride
	}
	return spinner
}

// Wait runs fn behind a spinner on stderr and reports the outcome. It is the
// one-line form:
//
//	err := clikit.Wait("importing prompts", func() error { return importAll(ctx) })
func Wait(message string, fn func() error) error {
	spinner := NewSpinner(message, SpinnerConsole(NewStderrConsole())).Start()
	err := fn()
	spinner.Stop(err)
	return err
}

// Start begins animating and returns the spinner, so it can be chained with a
// deferred Stop.
func (s *Spinner) Start() *Spinner {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return s
	}
	s.running = true
	message := s.message
	s.mu.Unlock()

	pushSpinner(s)
	if !s.enabled {
		_, _ = io.WriteString(s.out, message+"...\n")
		return s
	}
	s.stop = make(chan struct{})
	s.stopped = make(chan struct{})
	go s.animate()
	return s
}

// Update changes the message while the spinner is running.
func (s *Spinner) Update(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

// Stop ends the animation. A non-nil err prints an error line instead of a
// success line when the spinner has a console.
func (s *Spinner) Stop(err error) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	message := s.message
	s.mu.Unlock()

	popSpinner(s)
	if s.stop != nil {
		close(s.stop)
		<-s.stopped
		s.clearLine()
	}
	if s.console == nil {
		return
	}
	if err != nil {
		s.console.Error(message)
		return
	}
	s.console.Success(message)
}

// Pause suspends the draw loop for the duration of fn, so an interactive
// prompt is not overwritten by the next frame. Safe to call on a disabled or
// stopped spinner, where it simply runs fn.
func (s *Spinner) Pause(fn func()) {
	if !s.enabled || s.stop == nil {
		fn()
		return
	}
	s.paused.Store(true)
	s.clearLine()
	defer s.paused.Store(false)
	fn()
}

func (s *Spinner) animate() {
	defer close(s.stopped)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for frame := 0; ; frame++ {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
		}
		if s.paused.Load() {
			continue
		}
		s.mu.Lock()
		message := s.message
		s.mu.Unlock()
		_, _ = io.WriteString(s.out, "\r\x1b[36m"+string(spinnerFrames[frame%len(spinnerFrames)])+"\x1b[0m "+message+"\x1b[K")
	}
}

func (s *Spinner) clearLine() {
	_, _ = io.WriteString(s.out, "\r\x1b[K")
}

// Active spinners form a stack so prompts can pause whichever one is currently
// drawing. Only the innermost matters: it owns the cursor line.
var (
	activeMu      sync.Mutex
	activeSpinner []*Spinner
)

func pushSpinner(s *Spinner) {
	activeMu.Lock()
	defer activeMu.Unlock()
	activeSpinner = append(activeSpinner, s)
}

func popSpinner(s *Spinner) {
	activeMu.Lock()
	defer activeMu.Unlock()
	for index := len(activeSpinner) - 1; index >= 0; index-- {
		if activeSpinner[index] == s {
			activeSpinner = append(activeSpinner[:index], activeSpinner[index+1:]...)
			return
		}
	}
}

// pauseActiveSpinner runs fn with the innermost spinner suspended.
func pauseActiveSpinner(fn func()) {
	activeMu.Lock()
	var current *Spinner
	if len(activeSpinner) > 0 {
		current = activeSpinner[len(activeSpinner)-1]
	}
	activeMu.Unlock()
	if current == nil {
		fn()
		return
	}
	current.Pause(fn)
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(file.Fd())
}
