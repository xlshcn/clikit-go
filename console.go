package clikit

import (
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/charmbracelet/colorprofile"
)

// Console writes human-facing output. Colour is handled by the underlying
// colour-profile writer, which downsamples to whatever the terminal supports
// and strips styling entirely for pipes, CI logs and NO_COLOR — so callers
// never have to ask "is this a TTY".
type Console struct {
	// Compact drops table borders and renders key/value pairs as plain lines.
	Compact bool

	writer *colorprofile.Writer
}

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	verboseStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	debugStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	headingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
)

// NewConsole writes to w. Pass cmd.ErrOrStderr() to keep decorative output off
// stdout, which matters when stdout is being piped into another program.
func NewConsole(w io.Writer) *Console {
	return &Console{writer: colorprofile.NewWriter(w, os.Environ())}
}

// NewStderrConsole is the common case: humans read stderr, machines read stdout.
func NewStderrConsole() *Console { return NewConsole(os.Stderr) }

// Print writes a line verbatim.
func (c *Console) Print(message string) { c.line(message) }

// Printf writes a formatted line verbatim.
func (c *Console) Printf(format string, args ...any) { c.line(fmt.Sprintf(format, args...)) }

// Success marks a completed step.
func (c *Console) Success(message string) { c.line(successStyle.Render("✓ " + message)) }

// Warn marks something the user should notice but that did not stop the run.
func (c *Console) Warn(message string) { c.line(warnStyle.Render("▫ " + message)) }

// Error marks a failure.
func (c *Console) Error(message string) { c.line(errorStyle.Render("✗ " + message)) }

// Verbose writes de-emphasised detail.
func (c *Console) Verbose(message string) { c.line(verboseStyle.Render(message)) }

// Debug writes diagnostic detail.
func (c *Console) Debug(message string) { c.line(debugStyle.Render("DEBUG: " + message)) }

// Heading writes a section title.
func (c *Console) Heading(message string) { c.line(headingStyle.Render(message)) }

// Pair is one ordered key/value row. A slice is used rather than a map so the
// caller controls the order.
type Pair struct {
	Key   string
	Value string
}

// KeyValues renders ordered fields, as a two-column table or, in compact mode,
// as `key: value` lines.
func (c *Console) KeyValues(title string, pairs []Pair) {
	if title != "" {
		c.Heading(title)
	}
	if c.Compact {
		for _, pair := range pairs {
			c.Printf("%s: %s", pair.Key, pair.Value)
		}
		return
	}
	rows := make([][]string, 0, len(pairs))
	for _, pair := range pairs {
		rows = append(rows, []string{pair.Key, pair.Value})
	}
	c.Table([]string{"key", "value"}, rows)
}

// Table renders rows under the given headers. Pass a nil headers slice to omit
// the header row.
func (c *Console) Table(headers []string, rows [][]string) {
	if c.Compact {
		if len(headers) > 0 {
			c.line(strings.Join(headers, "\t"))
		}
		for _, row := range rows {
			c.line(strings.Join(row, "\t"))
		}
		return
	}
	rendered := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(verboseStyle).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headingStyle.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		Headers(headers...).
		Rows(rows...).
		Render()
	c.line(rendered)
}

// Sections writes titled blocks separated by blank lines.
func (c *Console) Sections(sections []Pair) {
	for index, section := range sections {
		if index > 0 {
			c.line("")
		}
		c.Heading(section.Key)
		c.line(section.Value)
	}
}

func (c *Console) line(message string) {
	_, _ = c.writer.WriteString(message + "\n")
}
