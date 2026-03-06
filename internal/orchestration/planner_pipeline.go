package orchestration

import (
	"context"

	"github.com/dongowu/agentos/pkg/taskdsl"
)

// RetryPlanner retries transient planner failures a bounded number of times.
type RetryPlanner struct {
	inner    Planner
	attempts int
}

func NewRetryPlanner(inner Planner, attempts int) *RetryPlanner {
	if attempts < 1 {
		attempts = 1
	}
	return &RetryPlanner{inner: inner, attempts: attempts}
}

func (p *RetryPlanner) Plan(ctx context.Context, input PlanInput) (*taskdsl.Plan, error) {
	var lastErr error
	for attempt := 0; attempt < p.attempts; attempt++ {
		plan, err := p.inner.Plan(ctx, input)
		if err == nil {
			return plan, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

// FallbackPlanner delegates to a secondary planner when the primary planner errors or returns no actions.
type FallbackPlanner struct {
	primary   Planner
	secondary Planner
}

func NewFallbackPlanner(primary, secondary Planner) *FallbackPlanner {
	return &FallbackPlanner{primary: primary, secondary: secondary}
}

func (p *FallbackPlanner) Plan(ctx context.Context, input PlanInput) (*taskdsl.Plan, error) {
	plan, err := p.primary.Plan(ctx, input)
	if err == nil && plan != nil && len(plan.Actions) > 0 {
		return plan, nil
	}
	if p.secondary == nil {
		return plan, err
	}
	return p.secondary.Plan(ctx, input)
}
