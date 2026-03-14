// Package console provides pretty terminal output for the SynapBus server.
package console

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// ANSI color codes
const (
	reset  = "\033[0m"
	green  = "\033[32m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	gray   = "\033[90m"
	bold   = "\033[1m"
	red    = "\033[31m"
)

// Printer writes pretty-formatted console output.
type Printer struct {
	mu  sync.Mutex
	out io.Writer
}

// New creates a Printer writing to stdout.
func New() *Printer {
	return &Printer{out: os.Stdout}
}

// NewWithWriter creates a Printer writing to the given writer (for testing).
func NewWithWriter(w io.Writer) *Printer {
	return &Printer{out: w}
}

// Success prints a green checkmark line: ✓ message
func (p *Printer) Success(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s✓%s %s\n", green, reset, msg)
}

// Arrow prints a cyan arrow line: → message
func (p *Printer) Arrow(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s→%s %s\n", cyan, reset, msg)
}

// Info prints a gray info line.
func (p *Printer) Info(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s%s%s\n", gray, msg, reset)
}

// Warn prints a yellow warning line.
func (p *Printer) Warn(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s⚠ %s%s\n", yellow, msg, reset)
}

// Error prints a red error line.
func (p *Printer) Error(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s✗ %s%s\n", red, msg, reset)
}

// Blank prints an empty line.
func (p *Printer) Blank() {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.out)
}

// AgentConnected prints a formatted agent connection event.
func (p *Printer) AgentConnected(name, clientName, clientVersion string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientName != "" {
		version := ""
		if clientVersion != "" {
			version = " " + clientVersion
		}
		fmt.Fprintf(p.out, "%s→%s Agent %s\"%s\"%s connected %s(%s%s)%s\n",
			cyan, reset, bold, name, reset, gray, clientName, version, reset)
	} else {
		fmt.Fprintf(p.out, "%s→%s Agent %s\"%s\"%s connected\n",
			cyan, reset, bold, name, reset)
	}
}

// AgentDisconnected prints a formatted agent disconnection event.
func (p *Printer) AgentDisconnected(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.out, "%s←%s Agent %s\"%s\"%s disconnected\n",
		yellow, reset, bold, name, reset)
}

// ClientConnected prints an anonymous client connection (no agent identity).
func (p *Printer) ClientConnected(clientName, clientVersion string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientName != "" {
		version := ""
		if clientVersion != "" {
			version = " " + clientVersion
		}
		fmt.Fprintf(p.out, "%s→%s Client connected %s(%s%s)%s\n",
			cyan, reset, gray, clientName, version, reset)
	} else {
		fmt.Fprintf(p.out, "%s→%s Client connected\n", cyan, reset)
	}
}
