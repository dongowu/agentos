package orchestration

import (
	"context"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// StubPlanner returns a single command.exec action for MVP.
type StubPlanner struct{}

// Plan implements Planner.
func (p *StubPlanner) Plan(ctx context.Context, input PlanInput) (*taskdsl.Plan, error) {
	_ = ctx
	_ = input
	return &taskdsl.Plan{
		Actions: []taskdsl.Action{
			{ID: "action-1", Kind: "command.exec", RuntimeEnv: "default", Payload: map[string]any{"cmd": "echo ok"}},
		},
	}, nil
}
