package orchestration

import "github.com/agentos/agentos/pkg/taskdsl"

// StubSkillResolver returns the action's RuntimeEnv as the profile.
type StubSkillResolver struct{}

// Resolve implements SkillResolver.
func (s *StubSkillResolver) Resolve(action *taskdsl.Action) (string, error) {
	if action.RuntimeEnv != "" {
		return action.RuntimeEnv, nil
	}
	return "default", nil
}
