package orchestration

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

var writePromptPattern = regexp.MustCompile(`(?i)^write\s+(.+?)\s+(?:to|into)\s+(.+)$`)

// PromptPlanner is the non-LLM fallback planner.
// It tries a small set of explicit heuristics first, then falls back to
// wrapping the original prompt as a command action.
type PromptPlanner struct{}

// Plan derives a typed plan from explicit user intent where possible.
func (p *PromptPlanner) Plan(_ context.Context, input PlanInput) (*taskdsl.Plan, error) {
	segments := splitPromptSegments(input.Prompt)
	actions := make([]taskdsl.Action, 0, len(segments))
	for index, segment := range segments {
		action := planActionForSegment(index+1, segment)
		actions = append(actions, action)
	}
	if len(actions) == 0 {
		return fallbackPlan(input), nil
	}
	return &taskdsl.Plan{Actions: actions}, nil
}

func splitPromptSegments(prompt string) []string {
	normalized := strings.TrimSpace(prompt)
	if normalized == "" {
		return nil
	}
	replacer := strings.NewReplacer(" and then ", " then ", " AND THEN ", " then ")
	normalized = replacer.Replace(normalized)
	parts := strings.Split(normalized, " then ")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	if len(segments) == 0 {
		return []string{normalized}
	}
	return segments
}

func planActionForSegment(index int, segment string) taskdsl.Action {
	trimmed := strings.TrimSpace(segment)
	lower := strings.ToLower(trimmed)
	id := fmt.Sprintf("prompt-%d", index)

	if path := parseReadPath(lower, trimmed); path != "" {
		return taskdsl.Action{ID: id, Kind: "file.read", RuntimeEnv: "default", Payload: map[string]any{"path": path}}
	}
	if content, path, ok := parseWritePayload(trimmed); ok {
		return taskdsl.Action{ID: id, Kind: "file.write", RuntimeEnv: "default", Payload: map[string]any{"content": content, "path": path}}
	}
	if requestURL := parseHTTPURL(lower, trimmed); requestURL != "" {
		return taskdsl.Action{ID: id, Kind: "http.request", RuntimeEnv: "default", Payload: map[string]any{"method": "GET", "url": requestURL}}
	}
	return taskdsl.Action{ID: id, Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": trimmed}}
}

func parseReadPath(lower, original string) string {
	for _, prefix := range []string{"read ", "cat ", "show "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(original[len(prefix):])
		}
	}
	return ""
}

func parseWritePayload(original string) (content string, path string, ok bool) {
	matches := writePromptPattern.FindStringSubmatch(strings.TrimSpace(original))
	if len(matches) != 3 {
		return "", "", false
	}
	content = strings.Trim(strings.TrimSpace(matches[1]), `"'`)
	path = strings.TrimSpace(matches[2])
	if content == "" || path == "" {
		return "", "", false
	}
	return content, path, true
}

func parseHTTPURL(lower, original string) string {
	for _, prefix := range []string{"fetch ", "get ", "download ", "request "} {
		if strings.HasPrefix(lower, prefix) {
			candidate := strings.TrimSpace(original[len(prefix):])
			if parsed, err := url.Parse(candidate); err == nil && parsed.Scheme != "" && parsed.Host != "" {
				return candidate
			}
		}
	}
	return ""
}
