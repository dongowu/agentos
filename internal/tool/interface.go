// Package tool provides the Tool plugin system for ClawOS.
package tool

import "context"

// Tool is the unified interface for all tool plugins.
type Tool interface {
	Name() string
	Description() string
	Run(ctx context.Context, input map[string]any) (any, error)
}
