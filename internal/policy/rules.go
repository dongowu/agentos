package policy

// Rule binds an agent glob pattern to allowed and denied tool/command patterns.
type Rule struct {
	// Agent is a glob pattern matched against the agent name (e.g. "worker-*").
	Agent string

	// Actions defines allow and deny tool patterns for matching agents.
	Actions Actions
}

// Actions holds allow/deny glob patterns. Deny takes precedence over allow.
type Actions struct {
	Allow []string
	Deny  []string
}

// Config holds policy engine configuration loaded at startup.
type Config struct {
	Rules          []Rule
	DefaultAutonomy AutonomyLevel
	RateLimit      RateLimitConfig
}

// RateLimitConfig controls per-agent rate limiting.
type RateLimitConfig struct {
	MaxActionsPerHour int
}
