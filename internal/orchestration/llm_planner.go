package orchestration

import (
	"context"
	"encoding/json"
	"errors"
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

const repairSystemPrompt = `You repair malformed planner output. Return ONLY valid JSON matching this schema:
{
  "Actions": [
    {
      "ID": "action-1",
      "Kind": "command.exec",
      "RuntimeEnv": "default",
      "Payload": {"cmd": "echo hello"}
    }
  ]
}`

var ErrMalformedPlan = errors.New("planner returned malformed plan")

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

	plan, parseErr := parseAndNormalizePlan(resp.Content)
	if parseErr == nil {
		return plan, nil
	}

	repaired, repairErr := p.repairPlan(ctx, input, resp.Content)
	if repairErr == nil {
		return repaired, nil
	}
	return nil, fmt.Errorf("%w: %v", ErrMalformedPlan, parseErr)
}

// parsePlan extracts a Plan from mixed LLM response text.
// It supports fenced JSON, surrounding explanatory text, a full object with
// `Actions`, or a bare action array.
func parsePlan(content string) (*taskdsl.Plan, error) {
	raw := extractJSONPayload(content)
	if raw == "" {
		return nil, fmt.Errorf("no json payload found")
	}
	if strings.HasPrefix(raw, "[") {
		var actions []taskdsl.Action
		if err := json.Unmarshal([]byte(raw), &actions); err != nil {
			return nil, err
		}
		return &taskdsl.Plan{Actions: actions}, nil
	}
	var plan taskdsl.Plan
	if err := json.Unmarshal([]byte(raw), &plan); err == nil {
		return &plan, nil
	}
	var alt struct {
		Actions []taskdsl.Action `json:"actions"`
	}
	if err := json.Unmarshal([]byte(raw), &alt); err != nil {
		return nil, err
	}
	return &taskdsl.Plan{Actions: alt.Actions}, nil
}

func extractJSONPayload(content string) string {
	raw := strings.TrimSpace(content)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "```") {
		if idx := strings.Index(raw, "\n"); idx != -1 {
			raw = raw[idx+1:]
		}
		if idx := strings.LastIndex(raw, "```"); idx != -1 {
			raw = raw[:idx]
		}
		raw = strings.TrimSpace(raw)
	}
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		return raw
	}
	start := strings.IndexAny(raw, "[{")
	if start == -1 {
		return ""
	}
	candidate := strings.TrimSpace(raw[start:])
	if strings.HasPrefix(candidate, "{") {
		if end := strings.LastIndex(candidate, "}"); end != -1 {
			return strings.TrimSpace(candidate[:end+1])
		}
	}
	if strings.HasPrefix(candidate, "[") {
		if end := strings.LastIndex(candidate, "]"); end != -1 {
			return strings.TrimSpace(candidate[:end+1])
		}
	}
	return ""
}

func (p *LLMPlanner) repairPlan(ctx context.Context, input PlanInput, malformedOutput string) (*taskdsl.Plan, error) {
	resp, err := p.provider.Chat(ctx, llm.Request{
		Model: p.model,
		Messages: []llm.Message{
			{Role: "system", Content: repairSystemPrompt},
			{Role: "user", Content: fmt.Sprintf("The previous planner output was malformed for prompt %q. Rewrite it as valid plan JSON only. Malformed output:\n%s", input.Prompt, malformedOutput)},
		},
		Temperature: 0,
	})
	if err != nil {
		return nil, err
	}
	return parseAndNormalizePlan(resp.Content)
}

func parseAndNormalizePlan(content string) (*taskdsl.Plan, error) {
	plan, err := parsePlan(content)
	if err != nil {
		return nil, err
	}
	if len(plan.Actions) == 0 {
		return nil, fmt.Errorf("empty actions")
	}
	normalizePlan(plan)
	if len(plan.Actions) == 0 {
		return nil, fmt.Errorf("empty actions")
	}
	return plan, nil
}

func normalizePlan(plan *taskdsl.Plan) {
	if plan == nil {
		return
	}
	for index := range plan.Actions {
		action := &plan.Actions[index]
		if strings.TrimSpace(action.ID) == "" {
			action.ID = fmt.Sprintf("llm-%d", index+1)
		}
		if strings.TrimSpace(action.RuntimeEnv) == "" {
			action.RuntimeEnv = "default"
		}
	}
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
