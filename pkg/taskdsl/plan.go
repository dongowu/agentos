package taskdsl

// Plan is the output of a Planner, an ordered list of actions.
type Plan struct {
	Actions []Action
}
