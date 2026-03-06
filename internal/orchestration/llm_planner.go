package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dongowu/agentos/internal/adapters/llm"
	"github.com/dongowu/agentos/pkg/taskdsl"
)

const systemPrompt = `You are a task planner. Given a user prompt, decompose it into a structured plan.
Respond with ONLY a JSON object matching this schema, no extra text:
{
  "Actions": [
    {
      "ID": "action-1",
      "Kind": "command.exec",
      "RuntimeEnv": "default",
      "Payload": {"cmd": "echo hello"}
    }
  ]
}
Valid action Kinds: command.exec, file.write, file.read, browser.step, http.request.
Each action must have a unique ID, a Kind, a RuntimeEnv, and a Payload map.`

// LLMPlanner implements Planner by calling an LLM provider.
type LLMPlanner struct {
	provider llm.Provider
	model    string
}

// NewLLMPlanner creates a Planner backed by the given LLM provider and model name.
func NewLLMPlanner(provider llm.Provider, model string) *LLMPlanner {
	return &LLMPlanner{provider: provider, model: model}
}

// Plan calls the LLM to decompose the prompt into a structured plan.
// If the LLM response cannot be parsed as a valid plan, it falls back to a
// single command.exec action using the original prompt.
func (p *LLMPlanner) Plan(ctx context.Context, input PlanInput) (*taskdsl.Plan, error) {
	resp, err := p.provider.Chat(ctx, llm.Request{
		Model: p.model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: input.Prompt},
		},
		Temperature: 0.2,
	})
	if err != nil {
		return nil, fmt.Errorf("llm planner: %w", err)
	}

	plan, err := parsePlan(resp.Content)
	if err != nil || len(plan.Actions) == 0 {
		return fallbackPlan(input), nil
	}
	return plan, nil
}

// parsePlan extracts a Plan from the LLM response text, handling optional
// markdown code fences around the JSON.
func parsePlan(content string) (*taskdsl.Plan, error) {
	raw := strings.TrimSpace(content)

	// Strip markdown code fences if present.
	if strings.HasPrefix(raw, "```") {
		// Remove opening fence line.
		if idx := strings.Index(raw, "\n"); idx != -1 {
			raw = raw[idx+1:]
		}
		// Remove closing fence.
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}

	var plan taskdsl.Plan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// fallbackPlan returns a single command.exec action wrapping the original prompt.
func fallbackPlan(input PlanInput) *taskdsl.Plan {
	return &taskdsl.Plan{
		Actions: []taskdsl.Action{
			{
				ID:         "fallback-1",
				Kind:       "command.exec",
				RuntimeEnv: "default",
				Payload:    map[string]any{"cmd": input.Prompt},
			},
		},
	}
}
