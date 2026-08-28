// Package iostreams centralises the three standard streams plus everything that
// depends on whether they are attached to a terminal: colour, width, prompting.
package iostreams

import (
	"bytes"
	"io"
	"os"
	"strconv"

	"golang.org/x/term"
)

const defaultWidth = 80

type IOStreams struct {
	In     io.ReadCloser
	Out    io.Writer
	ErrOut io.Writer

	stdinTTY  bool
	stdoutTTY bool
	stderrTTY bool

	colorEnabled bool
	width        int

	// neverPrompt is set by --no-prompt or a non-interactive stdin.
	neverPrompt bool
}

// System builds IOStreams around the process's own stdin/stdout/stderr.
func System() *IOStreams {
	stdoutTTY := isTerminal(os.Stdout)
	stderrTTY := isTerminal(os.Stderr)
	stdinTTY := isTerminal(os.Stdin)

	io := &IOStreams{
		In:        os.Stdin,
		Out:       os.Stdout,
		ErrOut:    os.Stderr,
		stdinTTY:  stdinTTY,
		stdoutTTY: stdoutTTY,
		stderrTTY: stderrTTY,
	}
	io.colorEnabled = envColorEnabled(stdoutTTY)
	io.width = terminalWidth(os.Stdout)
	io.neverPrompt = !stdinTTY || !stdoutTTY
	return io
}

// Test builds IOStreams backed by buffers, for use in tests.
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	return &IOStreams{
		In:          io.NopCloser(in),
		Out:         out,
		ErrOut:      errOut,
		width:       defaultWidth,
		neverPrompt: true,
	}, in, out, errOut
}

func (s *IOStreams) IsStdinTTY() bool  { return s.stdinTTY }
func (s *IOStreams) IsStdoutTTY() bool { return s.stdoutTTY }
func (s *IOStreams) IsStderrTTY() bool { return s.stderrTTY }

func (s *IOStreams) ColorEnabled() bool { return s.colorEnabled }

// SetColorEnabled forces colour on or off, e.g. from a global flag.
func (s *IOStreams) SetColorEnabled(v bool) { s.colorEnabled = v }

func (s *IOStreams) SetStdoutTTY(v bool) { s.stdoutTTY = v }
func (s *IOStreams) SetStdinTTY(v bool)  { s.stdinTTY = v }
func (s *IOStreams) SetStderrTTY(v bool) { s.stderrTTY = v }

func (s *IOStreams) CanPrompt() bool       { return !s.neverPrompt }
func (s *IOStreams) SetNeverPrompt(v bool) { s.neverPrompt = v }

// TerminalWidth reports the usable width of stdout, defaulting to 80 columns.
func (s *IOStreams) TerminalWidth() int {
	if s.width > 0 {
		return s.width
	}
	return defaultWidth
}

func (s *IOStreams) SetTerminalWidth(w int) { s.width = w }

func (s *IOStreams) ColorScheme() *ColorScheme { return NewColorScheme(s.colorEnabled) }

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func terminalWidth(f *os.File) int {
	if w, _, err := term.GetSize(int(f.Fd())); err == nil && w > 0 {
		return w
	}
	if v := os.Getenv("COLUMNS"); v != "" {
		if w, err := strconv.Atoi(v); err == nil && w > 0 {
			return w
		}
	}
	return defaultWidth
}

// envColorEnabled honours the NO_COLOR / CLICOLOR_FORCE conventions.
func envColorEnabled(stdoutTTY bool) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if v := os.Getenv("CLICOLOR_FORCE"); v != "" && v != "0" {
		return true
	}
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return stdoutTTY
}
