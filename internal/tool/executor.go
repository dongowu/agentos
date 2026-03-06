package tool

import "context"

// Run executes a tool by name with the given input.
func Run(ctx context.Context, name string, input map[string]any) (any, error) {
	t := Get(name)
	if t == nil {
		return nil, ErrNotFound{Name: name}
	}
	return t.Run(ctx, input)
}

// ErrNotFound is returned when a tool is not registered.
type ErrNotFound struct {
	Name string
}

func (e ErrNotFound) Error() string {
	return "tool: not found: " + e.Name
}
