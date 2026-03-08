package llm

import "context"

// ToolDef describes a tool the LLM can invoke via function calling.
type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any // JSON Schema
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // raw JSON string
}

// Message represents a single chat message with a role and content.
type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall // non-nil when Role == "assistant" and LLM requests tool use
	ToolCallID string     // set when Role == "tool" to correlate with ToolCall.ID
}

// Request is the input to an LLM chat completion call.
type Request struct {
	Model       string
	Messages    []Message
	Temperature float64
	Tools       []ToolDef // when non-empty, enables tool-calling mode
}

// Response is the output from an LLM chat completion call.
type Response struct {
	Content   string
	ToolCalls []ToolCall // non-nil when the LLM wants to invoke tools
}

// Provider abstracts LLM chat completion behind a single method.
type Provider interface {
	Chat(ctx context.Context, req Request) (*Response, error)
}
