// Package policy provides a control-plane policy engine inspired by HiClaw's
// credential isolation and zeroclaw's SecurityPolicy. It evaluates agent
// actions against rules before dispatch and ensures workers never hold
// real API keys.
package policy

import "context"

// AutonomyLevel controls how much human oversight an agent requires.
type AutonomyLevel string

const (
	Supervised     AutonomyLevel = "supervised"
	SemiAutonomous AutonomyLevel = "semi_autonomous"
	Autonomous     AutonomyLevel = "autonomous"
)

// PolicyRequest is submitted by the orchestrator before dispatching an action.
type PolicyRequest struct {
	AgentName string
	ToolName  string
	Command   string
	TenantID  string
}

// PolicyDecision is the engine's verdict on a PolicyRequest.
type PolicyDecision struct {
	Allowed  bool
	Reason   string
	Autonomy AutonomyLevel
}

// PolicyEngine evaluates whether an agent action is permitted.
type PolicyEngine interface {
	Evaluate(ctx context.Context, req PolicyRequest) (*PolicyDecision, error)
}
