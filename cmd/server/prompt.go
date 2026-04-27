package main

import (
"bufio"
"fmt"
"io"
"os"
"strings"
)

// prompter holds the I/O streams and interactive mode flag.
// Tests inject their own reader; production uses os.Stdin.
type prompter struct {
reader      *bufio.Reader
writer      io.Writer
interactive bool
}

func newPrompter(noInteractive bool) *prompter {
interactive := !noInteractive && isTTY()
return &prompter{
reader:      bufio.NewReader(os.Stdin),
writer:      os.Stdout,
interactive: interactive,
}
}

// newPrompterFromReader creates a prompter with a custom reader (used in tests).
func newPrompterFromReader(r io.Reader, w io.Writer, interactive bool) *prompter {
return &prompter{
reader:      bufio.NewReader(r),
writer:      w,
interactive: interactive,
}
}

// Prompt shows a labelled question with a default value and reads the answer.
// Returns defaultVal if not interactive or if the user hits enter without input.
func (p *prompter) Prompt(label, defaultVal string) string {
if !p.interactive {
return defaultVal
}
if defaultVal != "" {
fmt.Fprintf(p.writer, "  %s [%s]: ", label, defaultVal)
} else {
fmt.Fprintf(p.writer, "  %s: ", label)
}
line, _ := p.reader.ReadString('\n')
line = strings.TrimRight(line, "\r\n")
if line == "" {
return defaultVal
}
return line
}

// PromptSecret shows a prompt for sensitive input (e.g. API keys, passwords).
// Input is visible in the terminal. Returns empty string when not interactive.
func (p *prompter) PromptSecret(label string) string {
if !p.interactive {
return ""
}
fmt.Fprintf(p.writer, "  %s (visible — consider env var instead): ", label)
line, _ := p.reader.ReadString('\n')
return strings.TrimRight(line, "\r\n")
}

// PromptSelect shows a choice with allowed options and a default.
// If the user enters something not in opts, the default is used.
func (p *prompter) PromptSelect(label string, opts []string, defaultVal string) string {
if !p.interactive {
return defaultVal
}
fmt.Fprintf(p.writer, "  %s (%s) [%s]: ", label, strings.Join(opts, "/"), defaultVal)
line, _ := p.reader.ReadString('\n')
line = strings.TrimRight(line, "\r\n")
if line == "" {
return defaultVal
}
for _, o := range opts {
if strings.EqualFold(o, line) {
return strings.ToLower(o)
}
}
fmt.Fprintf(p.writer, "    → %q not recognised, using default %q\n", line, defaultVal)
return defaultVal
}

// PromptBool shows a yes/no question. Accepts y/yes/n/no (case-insensitive).
func (p *prompter) PromptBool(label string, def bool) bool {
if !p.interactive {
return def
}
defStr := "N"
if def {
defStr = "Y"
}
fmt.Fprintf(p.writer, "  %s (y/n) [%s]: ", label, defStr)
line, _ := p.reader.ReadString('\n')
line = strings.TrimRight(line, "\r\n")
if line == "" {
return def
}
switch strings.ToLower(line) {
case "y", "yes":
return true
case "n", "no":
return false
}
return def
}

// section prints a section header when in interactive mode.
func (p *prompter) section(name string) {
if p.interactive {
fmt.Fprintf(p.writer, "\n=== %s ===\n", name)
}
}

// isTTY reports whether stdout is an interactive terminal.
func isTTY() bool {
fi, err := os.Stdout.Stat()
if err != nil {
return false
}
return fi.Mode()&os.ModeCharDevice != 0
}
