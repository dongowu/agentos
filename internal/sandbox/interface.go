// Package sandbox provides execution sandboxes: browser, shell, docker, wasm.
package sandbox

import "context"

// Sandbox is the execution environment abstraction.
type Sandbox interface {
	// Type returns the sandbox type: browser, shell, docker, wasm.
	Type() string
	// Run executes the given spec and returns output.
	Run(ctx context.Context, spec Spec) (Result, error)
}

// Spec describes what to run.
type Spec struct {
	Type    string            // browser, shell, docker, wasm
	Command string            // for shell
	URL     string            // for browser
	Actions []BrowserAction   // for browser
	Image   string            // for docker
	Payload []byte            // for wasm
}

// BrowserAction is a single browser step: open, click, input, scrape.
type BrowserAction struct {
	Action string // open, click, input, scrape
	Selector string
	Value   string
}

// Result is the execution result.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Output   string // for scrape
}
