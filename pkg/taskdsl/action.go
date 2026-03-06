package taskdsl

// Action is the smallest schedulable unit.
type Action struct {
	ID          string
	Kind        string // command.exec, file.write, browser.step, etc.
	Payload     map[string]any
	RuntimeEnv  string // golang-dev, rust-dev, sui-move, etc.
}
