package llm

import "context"

// Message represents a single chat message with a role and content.
type Message struct {
	Role    string
	Content string
}

// Request is the input to an LLM chat completion call.
type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
}

// Response is the output from an LLM chat completion call.
type Response struct {
	Content string
}

// Provider abstracts LLM chat completion behind a single method.
type Provider interface {
	Chat(ctx context.Context, req Request) (*Response, error)
}
