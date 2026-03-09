# Agent Loop Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a tool-calling Agent Loop to EngineImpl so the LLM can observe execution results and iterate autonomously (max 10 rounds).

**Architecture:** Extend the existing LLM Provider interface with tool-calling types (ToolDef, ToolCall), update the OpenAI adapter to send/parse function calls, and add a `runAgentLoop` method inside EngineImpl that loops: LLM → tool_calls → execute → feed results back → repeat.

**Tech Stack:** Go 1.24, OpenAI-compatible API (function calling format), existing tool.Tool registry.

---

### Task 1: Extend LLM Provider Types

**Files:**
- Modify: `internal/adapters/llm/provider.go`

**Step 1: Add ToolDef, ToolCall types and extend Message/Request/Response**

```go
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
```

**Step 2: Run existing tests to verify backward compatibility**

Run: `go test ./internal/adapters/llm/... ./internal/orchestration/... -count=1`
Expected: All existing tests PASS (new fields are zero-valued, no behavior change).

**Step 3: Commit**

```bash
git add internal/adapters/llm/provider.go
git commit -m "feat(llm): extend provider types with tool-calling support"
```

---

### Task 2: Update OpenAI Adapter for Tool Calling

**Files:**
- Modify: `internal/adapters/llm/openai/openai.go`
- Modify: `internal/adapters/llm/openai/openai_test.go`

**Step 1: Write failing test for tool-calling request/response**

Add to `openai_test.go`:

```go
func TestClient_Chat_ToolCalling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// Verify tools were sent
		tools, ok := body["tools"].([]any)
		if !ok || len(tools) != 1 {
			t.Fatalf("expected 1 tool, got %v", body["tools"])
		}

		// Return a tool_calls response
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_abc123",
								"type": "function",
								"function": map[string]any{
									"name":      "shell",
									"arguments": `{"cmd":"ls -la"}`,
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: "list files"},
		},
		Tools: []llm.ToolDef{
			{
				Name:        "shell",
				Description: "Execute shell commands",
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "shell" {
		t.Errorf("expected tool name 'shell', got %q", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].ID != "call_abc123" {
		t.Errorf("expected tool call ID 'call_abc123', got %q", resp.ToolCalls[0].ID)
	}
	if resp.ToolCalls[0].Arguments != `{"cmd":"ls -la"}` {
		t.Errorf("unexpected arguments: %q", resp.ToolCalls[0].Arguments)
	}
}

func TestClient_Chat_ToolResultMessages(t *testing.T) {
	var receivedMessages []map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receivedMessages = body["messages"].([]any)

		// Verify tool result message was sent correctly
		msgs := body["messages"].([]any)
		lastMsg := msgs[len(msgs)-1].(map[string]any)
		if lastMsg["role"] != "tool" {
			t.Errorf("expected last message role 'tool', got %q", lastMsg["role"])
		}
		if lastMsg["tool_call_id"] != "call_abc123" {
			t.Errorf("expected tool_call_id 'call_abc123', got %v", lastMsg["tool_call_id"])
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "Here are the files."}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.Chat(context.Background(), llm.Request{
		Model: "gpt-4",
		Messages: []llm.Message{
			{Role: "user", Content: "list files"},
			{Role: "assistant", ToolCalls: []llm.ToolCall{{ID: "call_abc123", Name: "shell", Arguments: `{"cmd":"ls"}`}}},
			{Role: "tool", ToolCallID: "call_abc123", Content: "file1.txt\nfile2.txt"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "Here are the files." {
		t.Errorf("unexpected content: %q", resp.Content)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/adapters/llm/openai/ -run TestClient_Chat_Tool -v`
Expected: FAIL (tools not sent in request, tool_calls not parsed from response).

**Step 3: Update openai.go to support tool calling**

Replace internal types and `Chat` method in `openai.go`:

```go
// chatRequest is the JSON body sent to the completions endpoint.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	Temperature float64         `json:"temperature"`
	Tools       []chatTool      `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatFunctionCall `json:"function"`
}

type chatFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// chatResponse mirrors the OpenAI chat completion response shape.
type chatResponse struct {
	Choices []chatChoice `json:"choices"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

// Chat sends a chat completion request and returns the assistant's response.
func (c *Client) Chat(ctx context.Context, req llm.Request) (*llm.Response, error) {
	messages := make([]chatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msg := chatMessage{Role: m.Role, Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			msg.ToolCalls = append(msg.ToolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: chatFunctionCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		messages[i] = msg
	}

	body := chatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
	}
	for _, td := range req.Tools {
		body.Tools = append(body.Tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  td.Parameters,
			},
		})
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai: send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openai: API returned status %d", httpResp.StatusCode)
	}

	var resp chatResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("openai: empty choices in response")
	}

	choice := resp.Choices[0].Message
	result := &llm.Response{Content: choice.Content}
	for _, tc := range choice.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, llm.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return result, nil
}
```

**Step 4: Run all OpenAI adapter tests**

Run: `go test ./internal/adapters/llm/openai/ -v -count=1`
Expected: ALL PASS (both new tool-calling tests and existing tests).

**Step 5: Commit**

```bash
git add internal/adapters/llm/openai/openai.go internal/adapters/llm/openai/openai_test.go
git commit -m "feat(openai): support tool calling in chat requests and responses"
```

---

### Task 3: Add LoopIteration Event and Task.Result Field

**Files:**
- Modify: `pkg/events/task_events.go`
- Modify: `pkg/taskdsl/task.go`

**Step 1: Add LoopIteration event type**

Append to `task_events.go`:

```go
// LoopIteration is emitted after each agent loop iteration.
type LoopIteration struct {
	TaskID     string    `json:"task_id"`
	Iteration  int       `json:"iteration"`
	ToolCalls  int       `json:"tool_calls"`
	Occurred   time.Time `json:"occurred"`
}
```

**Step 2: Add Result field to Task**

Update `task.go`:

```go
type Task struct {
	ID        string
	Prompt    string
	State     string
	Plan      *Plan
	Result    string // final answer from agent loop
	CreatedAt time.Time
	UpdatedAt time.Time
}
```

**Step 3: Run tests to verify no breakage**

Run: `go test ./pkg/... ./internal/... -count=1`
Expected: ALL PASS (new fields are zero-valued, backward compatible).

**Step 4: Commit**

```bash
git add pkg/events/task_events.go pkg/taskdsl/task.go
git commit -m "feat(taskdsl): add Task.Result field and LoopIteration event"
```

---

### Task 4: Add Tool Definition Builder

**Files:**
- Create: `internal/tool/tooldefs.go`
- Create: `internal/tool/tooldefs_test.go`

**Step 1: Write failing test**

```go
package tool

import (
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
)

type mockSchemaAwareTool struct {
	name   string
	desc   string
	schema map[string]any
}

func (m *mockSchemaAwareTool) Name() string                                          { return m.name }
func (m *mockSchemaAwareTool) Description() string                                   { return m.desc }
func (m *mockSchemaAwareTool) Run(_ context.Context, _ map[string]any) (any, error)  { return nil, nil }
func (m *mockSchemaAwareTool) Schema() map[string]any                                { return m.schema }

func TestBuildToolDefs_WithSchema(t *testing.T) {
	tools := []Tool{
		&mockSchemaAwareTool{
			name:   "shell",
			desc:   "Execute shell commands",
			schema: map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}},
		},
	}
	defs := BuildToolDefs(tools)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Name != "shell" {
		t.Errorf("expected name 'shell', got %q", defs[0].Name)
	}
	if defs[0].Parameters["type"] != "object" {
		t.Errorf("expected schema with type=object, got %v", defs[0].Parameters)
	}
}

func TestBuildToolDefs_WithoutSchema(t *testing.T) {
	tools := []Tool{
		&mockSchemaAwareTool{name: "basic", desc: "A basic tool"},
	}
	// mockSchemaAwareTool returns nil schema
	tools[0].(*mockSchemaAwareTool).schema = nil

	defs := BuildToolDefs(tools)
	if len(defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(defs))
	}
	if defs[0].Parameters["type"] != "object" {
		t.Errorf("expected fallback schema, got %v", defs[0].Parameters)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tool/ -run TestBuildToolDefs -v`
Expected: FAIL (BuildToolDefs not defined).

**Step 3: Write implementation**

Create `internal/tool/tooldefs.go`:

```go
package tool

import "github.com/dongowu/agentos/internal/adapters/llm"

// BuildToolDefs converts a slice of Tools into LLM ToolDef descriptors.
// If a tool implements SchemaAware, its schema is used; otherwise a generic
// object schema is provided as fallback.
func BuildToolDefs(tools []Tool) []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		def := llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  map[string]any{"type": "object"},
		}
		if sa, ok := t.(SchemaAware); ok {
			if s := sa.Schema(); s != nil {
				def.Parameters = s
			}
		}
		defs = append(defs, def)
	}
	return defs
}

// AllTools returns all registered tools as a slice.
func AllTools() []Tool {
	mu.RLock()
	defer mu.RUnlock()
	tools := make([]Tool, 0, len(registry))
	for _, t := range registry {
		tools = append(tools, t)
	}
	return tools
}
```

**Step 4: Run tests**

Run: `go test ./internal/tool/ -run TestBuildToolDefs -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/tool/tooldefs.go internal/tool/tooldefs_test.go
git commit -m "feat(tool): add BuildToolDefs helper for LLM tool-calling"
```

---

### Task 5: Implement Agent Loop in EngineImpl

**Files:**
- Modify: `internal/orchestration/engine_impl.go`
- Modify: `internal/orchestration/engine_impl_test.go`

**Step 1: Write failing test for agent loop**

Add to `engine_impl_test.go`:

```go
// --- mock LLM provider for agent loop ---

type mockLLMProvider struct {
	responses []llm.Response
	calls     int
	requests  []llm.Request
}

func (m *mockLLMProvider) Chat(_ context.Context, req llm.Request) (*llm.Response, error) {
	m.requests = append(m.requests, req)
	idx := m.calls
	m.calls++
	if idx >= len(m.responses) {
		return &llm.Response{Content: "done"}, nil
	}
	return &m.responses[idx], nil
}

// --- mock tool for agent loop ---

type mockTool struct {
	name   string
	desc   string
	result any
	err    error
	calls  int
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.desc }
func (m *mockTool) Run(_ context.Context, _ map[string]any) (any, error) {
	m.calls++
	return m.result, m.err
}
func (m *mockTool) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}}
}

func TestEngineImpl_AgentLoop_SingleToolCall(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// LLM: first call returns tool_call, second call returns final answer
	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "shell", Arguments: `{"cmd":"echo hi"}`}}},
			{Content: "The command output: hi"},
		},
	}

	shellTool := &mockTool{name: "shell", desc: "Execute commands", result: map[string]any{"stdout": "hi", "exit_code": 0}}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "run echo hi")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "The command output: hi" {
		t.Errorf("expected result 'The command output: hi', got %q", task.Result)
	}
	if provider.calls != 2 {
		t.Errorf("expected 2 LLM calls, got %d", provider.calls)
	}
	if shellTool.calls != 1 {
		t.Errorf("expected 1 tool call, got %d", shellTool.calls)
	}
}

func TestEngineImpl_AgentLoop_MaxIterations(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// LLM always returns tool calls, never stops
	infiniteToolCall := llm.Response{
		ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "shell", Arguments: `{"cmd":"echo loop"}`}},
	}
	responses := make([]llm.Response, 20) // more than max
	for i := range responses {
		responses[i] = infiniteToolCall
	}
	provider := &mockLLMProvider{responses: responses}

	shellTool := &mockTool{name: "shell", desc: "Execute commands", result: map[string]any{"stdout": "loop", "exit_code": 0}}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "infinite loop")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Failed) {
		t.Errorf("expected failed, got %s", task.State)
	}
	if provider.calls != 10 {
		t.Errorf("expected exactly 10 LLM calls (max iterations), got %d", provider.calls)
	}
}

func TestEngineImpl_AgentLoop_NoToolCalls_ImmediateAnswer(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// LLM immediately returns a final answer without tool calls
	provider := &mockLLMProvider{
		responses: []llm.Response{
			{Content: "The answer is 42"},
		},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "what is the answer")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "The answer is 42" {
		t.Errorf("expected 'The answer is 42', got %q", task.Result)
	}
	if provider.calls != 1 {
		t.Errorf("expected 1 LLM call, got %d", provider.calls)
	}
}

func TestEngineImpl_AgentLoop_FallbackWithoutProvider(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// No LLM provider set — should fall back to existing plan+execute path
	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "echo hello")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	// Same behavior as before: reaches queued with plan attached
	if task.State != string(Queued) {
		t.Errorf("expected queued (legacy path), got %s", task.State)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/orchestration/ -run TestEngineImpl_AgentLoop -v`
Expected: FAIL (WithLLMProvider, WithTools methods not defined).

**Step 3: Implement agent loop in engine_impl.go**

Add these fields, methods, and the core loop to `engine_impl.go`:

Add to `EngineImpl` struct (after existing fields):

```go
	llmProvider llm.Provider   // nil = use legacy plan+execute path
	llmModel    string
	tools       []tool.Tool    // tools available for agent loop
```

Add import for `"encoding/json"` and `"github.com/dongowu/agentos/internal/adapters/llm"` and `"github.com/dongowu/agentos/internal/tool"`.

Add new methods:

```go
const maxAgentLoopIterations = 10

const agentSystemPrompt = `You are an AI agent running on AgentOS. You have access to tools that you can call to accomplish tasks.

When you need to perform an action, use the available tools. When you have completed the task or have the final answer, respond with your conclusion in plain text without calling any tools.

Be concise and focused. Execute the minimum number of tool calls needed to accomplish the task.`

// WithLLMProvider attaches an LLM provider for agent loop mode.
func (e *EngineImpl) WithLLMProvider(provider llm.Provider, model string) *EngineImpl {
	e.llmProvider = provider
	e.llmModel = model
	return e
}

// WithTools sets the tools available for the agent loop.
func (e *EngineImpl) WithTools(tools []tool.Tool) *EngineImpl {
	e.tools = tools
	return e
}

// runAgentLoop executes the multi-turn tool-calling loop.
func (e *EngineImpl) runAgentLoop(ctx context.Context, task *taskdsl.Task) (*taskdsl.Task, error) {
	if err := e.transition(ctx, task, Planning); err != nil {
		return task, err
	}
	if err := e.transition(ctx, task, Queued); err != nil {
		return task, err
	}
	if err := e.transition(ctx, task, Running); err != nil {
		return task, err
	}

	toolDefs := tool.BuildToolDefs(e.tools)
	toolMap := make(map[string]tool.Tool, len(e.tools))
	for _, t := range e.tools {
		toolMap[t.Name()] = t
	}

	messages := []llm.Message{
		{Role: "system", Content: agentSystemPrompt},
		{Role: "user", Content: task.Prompt},
	}

	for i := 0; i < maxAgentLoopIterations; i++ {
		resp, err := e.llmProvider.Chat(ctx, llm.Request{
			Model:       e.llmModel,
			Messages:    messages,
			Temperature: 0.2,
			Tools:       toolDefs,
		})
		if err != nil {
			_ = e.transition(ctx, task, Failed)
			return task, fmt.Errorf("agent loop iteration %d: %w", i, err)
		}

		_ = e.bus.Publish(ctx, "task.loop.iteration", &events.LoopIteration{
			TaskID:    task.ID,
			Iteration: i + 1,
			ToolCalls: len(resp.ToolCalls),
			Occurred:  time.Now(),
		})

		// No tool calls: LLM is done, this is the final answer.
		if len(resp.ToolCalls) == 0 {
			task.Result = resp.Content
			task.UpdatedAt = time.Now()
			_ = e.repo.Update(ctx, task)
			_ = e.transition(ctx, task, Evaluating)
			_ = e.transition(ctx, task, Succeeded)
			return e.repo.Get(ctx, task.ID)
		}

		// Append assistant message with tool calls.
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute each tool call and collect results.
		for _, tc := range resp.ToolCalls {
			actionID := fmt.Sprintf("loop-%d-%s", i+1, tc.ID)

			_ = e.bus.Publish(ctx, "task.action.dispatched", &events.ActionDispatched{
				TaskID:   task.ID,
				ActionID: actionID,
				Occurred: time.Now(),
			})

			t, ok := toolMap[tc.Name]
			if !ok {
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("error: tool %q not found", tc.Name),
				})
				continue
			}

			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: tc.ID,
					Content:    fmt.Sprintf("error: invalid arguments: %v", err),
				})
				continue
			}

			result, err := t.Run(ctx, args)
			var resultStr string
			if err != nil {
				resultStr = fmt.Sprintf("error: %v", err)
			} else {
				resultBytes, _ := json.Marshal(result)
				resultStr = string(resultBytes)
			}

			_ = e.bus.Publish(ctx, "task.action.completed", &events.ActionCompleted{
				TaskID:   task.ID,
				ActionID: actionID,
				ExitCode: 0,
				Stdout:   resultStr,
				Occurred: time.Now(),
			})

			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: tc.ID,
				Content:    resultStr,
			})
		}
	}

	// Max iterations reached
	task.Result = "agent loop exceeded maximum iterations"
	task.UpdatedAt = time.Now()
	_ = e.repo.Update(ctx, task)
	_ = e.transition(ctx, task, Failed)
	return e.repo.Get(ctx, task.ID)
}
```

Modify `StartTaskWithInput` — add agent loop routing at the beginning of the method body, before the existing logic. Insert immediately after the task creation and `task.created` event publish (after line 92 in current file):

```go
	// Agent loop path: when an LLM provider is configured, use the multi-turn
	// tool-calling loop instead of the legacy plan+execute path.
	if e.llmProvider != nil {
		return e.runAgentLoop(ctx, task)
	}
```

The full insertion point is right after these existing lines:
```go
	_ = e.bus.Publish(ctx, "task.created", &events.TaskCreated{...})

	// --- INSERT HERE ---
	if e.llmProvider != nil {
		return e.runAgentLoop(ctx, task)
	}

	if err := e.transition(ctx, task, Planning); err != nil {
	// ... existing code continues
```

**Step 4: Run all agent loop tests**

Run: `go test ./internal/orchestration/ -run TestEngineImpl_AgentLoop -v`
Expected: ALL 4 tests PASS.

**Step 5: Run ALL orchestration tests to verify no regression**

Run: `go test ./internal/orchestration/ -v -count=1`
Expected: ALL PASS (including existing tests for legacy path).

**Step 6: Commit**

```bash
git add internal/orchestration/engine_impl.go internal/orchestration/engine_impl_test.go
git commit -m "feat(orchestration): implement agent loop with tool-calling support"
```

---

### Task 6: Wire Agent Loop in Bootstrap

**Files:**
- Modify: `internal/bootstrap/bootstrap.go`

**Step 1: Wire LLM provider and tools into EngineImpl**

In `bootstrap.go`, the engine is created at line 227. After the existing `.WithAuditStore(audit)` chain, add:

```go
	// Wire agent loop when LLM is configured.
	llmProv, llmModel := llmProviderFromConfig(cfg)
	if llmProv != nil {
		engine.WithLLMProvider(llmProv, llmModel)
		engine.WithTools(tool.AllTools())
	}
```

Note: `llmProviderFromConfig` already exists (line 56). We just reuse its return values.

Also add import for `"github.com/dongowu/agentos/internal/tool"`.

**Step 2: Run bootstrap test**

Run: `go test ./internal/bootstrap/ -v -count=1`
Expected: PASS.

**Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS.

**Step 4: Commit**

```bash
git add internal/bootstrap/bootstrap.go
git commit -m "feat(bootstrap): wire LLM provider and tools into agent loop"
```

---

### Task 7: Integration Smoke Test

**Files:**
- Create: `internal/orchestration/agent_loop_integration_test.go`

**Step 1: Write integration test that exercises the full loop with mocks**

```go
package orchestration

import (
	"context"
	"testing"

	"github.com/dongowu/agentos/internal/adapters/llm"
	msgmemory "github.com/dongowu/agentos/internal/adapters/messaging/memory"
	persmemory "github.com/dongowu/agentos/internal/adapters/persistence/memory"
	"github.com/dongowu/agentos/internal/tool"
)

func TestAgentLoop_Integration_MultiStepTask(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	// Simulate a 3-step agent interaction:
	// 1. LLM calls shell("echo step1")
	// 2. LLM calls shell("echo step2")
	// 3. LLM returns final answer
	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "shell", Arguments: `{"cmd":"echo step1"}`}}},
			{ToolCalls: []llm.ToolCall{{ID: "c2", Name: "shell", Arguments: `{"cmd":"echo step2"}`}}},
			{Content: "Both steps completed successfully"},
		},
	}

	shellTool := &mockTool{
		name:   "shell",
		desc:   "Execute commands",
		result: map[string]any{"stdout": "ok", "exit_code": 0},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")
	engine.WithTools([]tool.Tool{shellTool})

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "run two steps")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}

	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
	if task.Result != "Both steps completed successfully" {
		t.Errorf("unexpected result: %q", task.Result)
	}
	if provider.calls != 3 {
		t.Errorf("expected 3 LLM calls, got %d", provider.calls)
	}
	if shellTool.calls != 2 {
		t.Errorf("expected 2 tool calls, got %d", shellTool.calls)
	}

	// Verify conversation history: last request should have all prior messages
	lastReq := provider.requests[2]
	// system + user + assistant(tool_call) + tool(result) + assistant(tool_call) + tool(result) = 6
	if len(lastReq.Messages) != 6 {
		t.Errorf("expected 6 messages in final request, got %d", len(lastReq.Messages))
	}
}

func TestAgentLoop_Integration_ToolError_ContinuesLoop(t *testing.T) {
	repo := persmemory.NewTaskRepository()
	bus := msgmemory.NewEventBus()
	planner := &StubPlanner{}

	provider := &mockLLMProvider{
		responses: []llm.Response{
			{ToolCalls: []llm.ToolCall{{ID: "c1", Name: "nonexistent", Arguments: `{}`}}},
			{Content: "Tool not found, but I can still answer"},
		},
	}

	engine := NewEngineImpl(repo, bus, planner, nil, nil, nil, nil)
	engine.WithLLMProvider(provider, "test-model")

	ctx := context.Background()
	task, err := engine.StartTask(ctx, "try bad tool")
	if err != nil {
		t.Fatalf("StartTask: %v", err)
	}
	if task.State != string(Succeeded) {
		t.Errorf("expected succeeded, got %s", task.State)
	}
}
```

**Step 2: Run integration tests**

Run: `go test ./internal/orchestration/ -run TestAgentLoop_Integration -v`
Expected: ALL PASS.

**Step 3: Run full test suite one final time**

Run: `go test ./... -count=1`
Expected: ALL PASS, zero regressions.

**Step 4: Commit**

```bash
git add internal/orchestration/agent_loop_integration_test.go
git commit -m "test(orchestration): add agent loop integration tests"
```

---

## Summary

| Task | Files | What |
|------|-------|------|
| 1 | `provider.go` | Extend LLM types with ToolDef, ToolCall |
| 2 | `openai.go`, `openai_test.go` | OpenAI adapter tool-calling support |
| 3 | `task_events.go`, `task.go` | LoopIteration event + Task.Result field |
| 4 | `tooldefs.go`, `tooldefs_test.go` | Tool → ToolDef conversion helper |
| 5 | `engine_impl.go`, `engine_impl_test.go` | Core agent loop in EngineImpl |
| 6 | `bootstrap.go` | Wire LLM + tools into engine |
| 7 | `agent_loop_integration_test.go` | Multi-step integration tests |
