package clikit_test

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xlshcn/clikit-go"
)

// safeBuffer guards the animation goroutine's writes against the test's reads.
type safeBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

func TestSpinnerDegradesWithoutTerminal(t *testing.T) {
	var out safeBuffer
	console := clikit.NewConsole(&out)
	spinner := clikit.NewSpinner("loading", clikit.SpinnerOutput(&out), clikit.SpinnerConsole(console)).Start()
	spinner.Stop(nil)

	if got := out.String(); got != "loading...\n✓ loading\n" {
		t.Errorf("output = %q, want a plain line then a success line", got)
	}
}

func TestSpinnerReportsFailure(t *testing.T) {
	var out safeBuffer
	spinner := clikit.NewSpinner("loading",
		clikit.SpinnerOutput(&out),
		clikit.SpinnerConsole(clikit.NewConsole(&out)),
	).Start()
	spinner.Stop(errors.New("nope"))

	if !strings.Contains(out.String(), "✗ loading") {
		t.Errorf("output = %q, want an error line", out.String())
	}
}

// The animated path is exercised under -race: Update, Pause and Stop all touch
// state the draw goroutine reads.
func TestSpinnerAnimatesConcurrently(t *testing.T) {
	var out safeBuffer
	spinner := clikit.NewSpinner("step one",
		clikit.SpinnerOutput(&out),
		clikit.SpinnerEnabled(true),
		clikit.SpinnerInterval(time.Millisecond),
	).Start()

	time.Sleep(10 * time.Millisecond)
	spinner.Update("step two")
	paused := false
	spinner.Pause(func() { paused = true })
	time.Sleep(10 * time.Millisecond)
	spinner.Stop(nil)

	if !paused {
		t.Error("Pause did not run its function")
	}
	if !strings.Contains(out.String(), "step two") {
		t.Errorf("output = %q, want the updated message", out.String())
	}
}

func TestSpinnerStopIsIdempotent(t *testing.T) {
	var out safeBuffer
	spinner := clikit.NewSpinner("loading", clikit.SpinnerOutput(&out)).Start()
	spinner.Stop(nil)
	spinner.Stop(nil) // a deferred Stop after an explicit one must not panic
}

func TestWaitReturnsTheCallbackError(t *testing.T) {
	want := errors.New("failed")
	if got := clikit.Wait("working", func() error { return want }); !errors.Is(got, want) {
		t.Errorf("Wait returned %v, want %v", got, want)
	}
}
