package output

import (
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
)

// Printer writes styled output to stderr (status) and stdout (data).
type Printer struct {
	stderr io.Writer
	stdout io.Writer
}

// New creates a Printer with the given writers.
func New(stderr, stdout io.Writer) *Printer {
	return &Printer{stderr: stderr, stdout: stdout}
}

// Default is the package-level printer using os.Stderr and os.Stdout.
var Default = New(os.Stderr, os.Stdout)

// Status prints a cyan status message to stderr.
func (p *Printer) Status(msg string, args ...any) {
	c := color.New(color.FgCyan)
	_, _ = c.Fprintf(p.stderr, msg+"\n", args...)
}

// Success prints a green "✓" prefixed message to stderr.
func (p *Printer) Success(msg string, args ...any) {
	c := color.New(color.FgGreen)
	_, _ = c.Fprintf(p.stderr, "✓ "+msg+"\n", args...)
}

// Error prints a red "✗" prefixed message to stderr.
func (p *Printer) Error(msg string, args ...any) {
	c := color.New(color.FgRed)
	_, _ = c.Fprintf(p.stderr, "✗ "+msg+"\n", args...)
}

// Warning prints a yellow "!" prefixed message to stderr.
func (p *Printer) Warning(msg string, args ...any) {
	c := color.New(color.FgYellow)
	_, _ = c.Fprintf(p.stderr, "! "+msg+"\n", args...)
}

// KeyValue prints a bold key + value pair to stderr.
func (p *Printer) KeyValue(key, value string) {
	bold := color.New(color.Bold)
	_, _ = bold.Fprintf(p.stderr, "%s", key)
	_, _ = fmt.Fprintf(p.stderr, " = %s\n", value)
}

// Prompt prints a bold message to stderr (no newline).
func (p *Printer) Prompt(msg string) {
	c := color.New(color.Bold)
	_, _ = c.Fprintf(p.stderr, "%s", msg)
}

// Dim prints a faint message to stderr.
func (p *Printer) Dim(msg string, args ...any) {
	c := color.New(color.Faint)
	_, _ = c.Fprintf(p.stderr, msg+"\n", args...)
}

// Stderr returns the stderr writer.
func (p *Printer) Stderr() io.Writer {
	return p.stderr
}

// Data prints uncolored output to stdout (pipeable).
func (p *Printer) Data(msg string, args ...any) {
	_, _ = fmt.Fprintf(p.stdout, msg+"\n", args...)
}
