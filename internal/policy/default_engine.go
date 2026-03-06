package policy

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// dangerousCommands is the blacklist of commands that are always denied.
var dangerousCommands = []string{
	"rm -rf /",
	"rm -rf /*",
	"dd ",
	"mkfs",
	":(){ :|:& };:",
	"> /dev/sda",
	"chmod -R 777 /",
}

// DefaultEngine is the standard PolicyEngine implementation.
// It evaluates rules with deny-takes-precedence semantics,
// enforces a dangerous-command blacklist, and applies per-agent rate limiting.
type DefaultEngine struct {
	rules           []Rule
	defaultAutonomy AutonomyLevel
	rateLimit       int // max actions per hour, 0 = unlimited

	// rate tracking: agentName -> list of timestamps
	mu    sync.Mutex
	usage map[string][]time.Time
}

// NewDefaultEngine creates a DefaultEngine from the given Config.
func NewDefaultEngine(cfg Config) *DefaultEngine {
	autonomy := cfg.DefaultAutonomy
	if autonomy == "" {
		autonomy = Supervised
	}
	return &DefaultEngine{
		rules:           cfg.Rules,
		defaultAutonomy: autonomy,
		rateLimit:       cfg.RateLimit.MaxActionsPerHour,
		usage:           make(map[string][]time.Time),
	}
}

// Evaluate checks the request against the dangerous-command blacklist,
// per-agent rate limits, and configured rules.
func (e *DefaultEngine) Evaluate(_ context.Context, req PolicyRequest) (*PolicyDecision, error) {
	// 1. Dangerous command blacklist
	if reason, blocked := e.checkDangerousCommand(req.Command); blocked {
		return &PolicyDecision{
			Allowed: false,
			Reason:  reason,
			Autonomy: e.defaultAutonomy,
		}, nil
	}

	// 2. Rate limiting
	if e.rateLimit > 0 {
		if !e.allowRate(req.AgentName) {
			return &PolicyDecision{
				Allowed: false,
				Reason:  fmt.Sprintf("rate limit exceeded: max %d actions/hour for agent %q", e.rateLimit, req.AgentName),
				Autonomy: e.defaultAutonomy,
			}, nil
		}
	}

	// 3. Rule evaluation (deny takes precedence)
	return e.evaluateRules(req), nil
}

func (e *DefaultEngine) checkDangerousCommand(cmd string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(cmd))
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lower, strings.ToLower(dangerous)) {
			return fmt.Sprintf("command blocked by blacklist: contains %q", dangerous), true
		}
	}
	return "", false
}

func (e *DefaultEngine) allowRate(agent string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Hour)

	// Prune old entries
	timestamps := e.usage[agent]
	valid := timestamps[:0]
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= e.rateLimit {
		e.usage[agent] = valid
		return false
	}

	e.usage[agent] = append(valid, now)
	return true
}

func (e *DefaultEngine) evaluateRules(req PolicyRequest) *PolicyDecision {
	for _, rule := range e.rules {
		agentMatch, _ := filepath.Match(rule.Agent, req.AgentName)
		if !agentMatch {
			continue
		}

		// Check deny first
		for _, pattern := range rule.Actions.Deny {
			if matched, _ := filepath.Match(pattern, req.ToolName); matched {
				return &PolicyDecision{
					Allowed:  false,
					Reason:   fmt.Sprintf("tool %q denied by pattern %q for agent %q", req.ToolName, pattern, rule.Agent),
					Autonomy: e.defaultAutonomy,
				}
			}
		}

		// Check allow
		if len(rule.Actions.Allow) > 0 {
			for _, pattern := range rule.Actions.Allow {
				if matched, _ := filepath.Match(pattern, req.ToolName); matched {
					return &PolicyDecision{
						Allowed:  true,
						Reason:   fmt.Sprintf("tool %q allowed by pattern %q", req.ToolName, pattern),
						Autonomy: e.defaultAutonomy,
					}
				}
			}
			// Has allow list but no match
			return &PolicyDecision{
				Allowed:  false,
				Reason:   fmt.Sprintf("tool %q not matched by any allow pattern for agent %q", req.ToolName, rule.Agent),
				Autonomy: e.defaultAutonomy,
			}
		}
	}

	// No matching rule: allow by default
	return &PolicyDecision{
		Allowed:  true,
		Reason:   "no matching rule; default allow",
		Autonomy: e.defaultAutonomy,
	}
}
