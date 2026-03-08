package actionbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dongowu/agentos/internal/runtimeclient"
	"github.com/dongowu/agentos/internal/tool"
	_ "github.com/dongowu/agentos/internal/tool/builtin"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

const ControlPlaneWorkerID = "control-plane"

// Bridge executes tool-like actions through the local Go tool registry.
type Bridge struct{}

// New returns a bridge for direct tool-style action execution.
func New() *Bridge { return &Bridge{} }

// CanExecute reports whether the action can run through the local tool registry.
func (b *Bridge) CanExecute(action *taskdsl.Action) bool {
	if action == nil {
		return false
	}
	if action.Kind == "http.request" {
		return true
	}
	return tool.Get(strings.TrimSpace(action.Kind)) != nil
}

// Execute runs a bridgeable action and maps its output into an ExecutionResult.
func (b *Bridge) Execute(ctx context.Context, taskID string, action *taskdsl.Action, sink func(runtimeclient.StreamChunk)) (*runtimeclient.ExecutionResult, error) {
	toolName, payload, err := resolveToolCall(action)
	if err != nil {
		return nil, err
	}
	out, err := tool.Run(ctx, toolName, payload)
	if err != nil {
		return nil, err
	}
	stdout, err := serializeOutput(out)
	if err != nil {
		return nil, err
	}
	result := &runtimeclient.ExecutionResult{ExitCode: 0, Stdout: stdout}
	if sink != nil && len(stdout) > 0 {
		sink(runtimeclient.StreamChunk{TaskID: taskID, ActionID: action.ID, Kind: "stdout", Data: stdout})
	}
	return result, nil
}

func resolveToolCall(action *taskdsl.Action) (string, map[string]any, error) {
	if action == nil {
		return "", nil, fmt.Errorf("action bridge: nil action")
	}
	kind := strings.TrimSpace(action.Kind)
	if kind == "" {
		return "", nil, fmt.Errorf("action bridge: action kind required")
	}
	payload := clonePayload(action.Payload)
	if kind == "http.request" {
		return resolveHTTPRequest(payload)
	}
	if tool.Get(kind) == nil {
		return "", nil, fmt.Errorf("action bridge: unsupported action kind %q", kind)
	}
	return kind, payload, nil
}

func resolveHTTPRequest(payload map[string]any) (string, map[string]any, error) {
	method := "GET"
	if raw, ok := payload["method"]; ok {
		text, ok := raw.(string)
		if !ok {
			return "", nil, fmt.Errorf("action bridge http.request: method must be a string")
		}
		method = strings.ToUpper(strings.TrimSpace(text))
	}
	switch method {
	case "", "GET":
		return "http.get", payload, nil
	case "POST":
		return "http.post", payload, nil
	default:
		return "", nil, fmt.Errorf("action bridge http.request: unsupported method %q", method)
	}
}

func serializeOutput(out any) ([]byte, error) {
	switch value := out.(type) {
	case nil:
		return nil, nil
	case []byte:
		return append([]byte(nil), value...), nil
	case string:
		return []byte(value), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("action bridge: marshal output: %w", err)
		}
		return data, nil
	}
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}
	return cloned
}
